package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func expireOwnerRetryForTest(t *testing.T, f independentToolE2EFixture, id string) {
	t.Helper()
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.owner_surface_records SET payload=jsonb_set(payload,'{retry_at}',to_jsonb('2000-01-01T00:00:00Z'::text)) WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerImageAssessmentTimeoutRetriesThenCreatesExactlyOneImageIntent(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "owner-timeout-retry")
	seedCognitiveProviderRole(t, ctx, repo, "owner-timeout-retry")
	admitted, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/"+fid+"/media-generations", nil, owner, map[string]any{"prompt": "窗边阅读的插画", "reason": "明确图片请求", "idempotency_key": "image-timeout-retry"})
	if err != nil {
		t.Fatal(err)
	}
	record := mapValue(admitted["request"])
	id, inbox := stringValue(record["id"]), stringValue(record["source_fact_id"])
	render := stringValue(mapValue(record["payload"])["_render_intent"])
	calls := 0
	fail := true
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if strings.Contains(jsonString(payload), "_automatic_retry_count") || strings.Contains(jsonString(payload), "retry_at") {
			t.Error("Core retry metadata leaked into model input")
		}
		if fail {
			return fakeProviderResult{Err: context.DeadlineExceeded}
		}
		if payloadHasToolResult(payload) {
			return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "已受理图片生成"}}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("retry-image", "media.image.generate", map[string]any{"intent": render, "media_plan": mediaTestPlan("object", "none", "no", "unknown")})}}
	})}
	first, err := app.ProcessCognitionInbox(ctx, inbox)
	if err != nil || first["status"] != "retry_scheduled" || first["mutation_committed"] != false || intValue(first["retry_after_seconds"]) != 15 {
		t.Fatal(first, err)
	}
	initialCalls := calls
	early, err := app.ProcessCognitionInbox(ctx, inbox)
	if err != nil || early["status"] != "retry_scheduled" || calls != initialCalls {
		t.Fatal("retry ignored backoff", early, calls, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.owner_surface_records SET payload=jsonb_set(payload,'{retry_at}',to_jsonb('2000-01-01T00:00:00Z'::text)) WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	fail = false
	second, err := app.ProcessCognitionInbox(ctx, inbox)
	if err != nil || second["status"] != "accepted" {
		t.Fatal(second, err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, inbox); err != nil {
		t.Fatal(err)
	}
	var images int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE owner_request_id=$1`, id).Scan(&images); err != nil || images != 1 {
		t.Fatal("retry duplicated image effect", images, err)
	}
}

func TestOwnerAssessmentTransientRetriesExhaustToTerminalState(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "owner-retry-exhausted")
	boots := wardrobeItemID(t, f, "黑色短靴")
	admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/wardrobe/wear", nil, f.ownerID, map[string]any{"item_id": boots, "slot": "shoes", "reason": "请换上靴子", "idempotency_key": "exhausted"})
	if err != nil {
		t.Fatal(err)
	}
	rec := mapValue(admitted["request"])
	id, inbox := stringValue(rec["id"]), stringValue(rec["source_fact_id"])
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult { return fakeProviderResult{Err: context.DeadlineExceeded} })}
	for attempt := 0; attempt < 3; attempt++ {
		result, err := f.app.ProcessCognitionInbox(f.ctx, inbox)
		if err != nil {
			t.Fatal(err)
		}
		if attempt < 2 && result["status"] != "retry_scheduled" || attempt == 2 && result["status"] != "failed" {
			t.Fatal(attempt, result)
		}
		if attempt < 2 {
			want := 15
			if attempt == 1 {
				want = 60
			}
			if intValue(result["retry_after_seconds"]) != want {
				t.Fatal(result)
			}
			expireOwnerRetryForTest(t, f, id)
		}
	}
	var status string
	var attempts int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,attempt_count FROM public.cognition_inbox WHERE id=$1`, inbox).Scan(&status, &attempts); err != nil || status != "processed" || attempts != 3 {
		t.Fatal(status, attempts, err)
	}
}

func TestOwnerStaleOutfitRequestReassessesCurrentCASAndExecutesAutomatically(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "owner-stale-retry")
	boots := wardrobeItemID(t, f, "黑色短靴")
	original := wardrobeRevision(t, f)
	admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/wardrobe/wear", nil, f.ownerID, map[string]any{"item_id": boots, "slot": "shoes", "expected_revision": original, "reason": "请换上靴子", "idempotency_key": "stale-outfit"})
	if err != nil {
		t.Fatal(err)
	}
	rec := mapValue(admitted["request"])
	id, inbox := stringValue(rec["id"]), stringValue(rec["source_fact_id"])
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1 WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	failAfterStale := false
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if failAfterStale {
			return fakeProviderResult{Err: context.DeadlineExceeded}
		}
		if !r6PayloadHasTool(payload, wardrobeWearCapabilityName) {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("discover-wear", capabilityCatalogName, map[string]any{"names": []string{wardrobeWearCapabilityName}})}}
		}
		if !payloadHasToolResult(payload) || !strings.Contains(jsonString(payload), "retry-wear") {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("retry-wear", wardrobeWearCapabilityName, map[string]any{"mode": "partial", "item_ids": []string{boots}})}}
		}
		return fakeProviderResult{Structured: map[string]any{"decision": "deferred", "reason": "如果资料失效则等待重新判断"}}
	})}
	first, err := f.app.ProcessCognitionInbox(f.ctx, inbox)
	if err != nil || first["status"] != "retry_scheduled" || wardrobeRevision(t, f) != original+1 {
		t.Fatal("stale attempt must not mutate", first, err)
	}
	expireOwnerRetryForTest(t, f, id)
	failAfterStale = true
	intermediate, err := f.app.ProcessCognitionInbox(f.ctx, inbox)
	if err != nil || intermediate["status"] != "retry_scheduled" || wardrobeRevision(t, f) != original+1 {
		t.Fatal("transient retry changed state", intermediate, err)
	}
	expireOwnerRetryForTest(t, f, id)
	failAfterStale = false
	result, err := f.app.ProcessCognitionInbox(f.ctx, inbox)
	if err != nil || result["status"] != "completed" || result["mutation_committed"] != true || wardrobeRevision(t, f) != original+2 {
		t.Fatal("fresh reassessment did not execute", result, err)
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload->'payload' FROM public.owner_surface_records WHERE id=$1`, id).Scan(&raw); err != nil || intValue(decodeObject(raw)["expected_revision"]) != original {
		t.Fatal("original Owner CAS overwritten", string(raw), err)
	}
}
