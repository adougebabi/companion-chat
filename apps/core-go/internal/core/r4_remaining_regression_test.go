package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
)

func TestR4BlankPersonaDeduplicatesSharedTraitsWithoutDroppingDistinctFacts(t *testing.T) {
	input := PersonaCompilationInput{ProfileID: "default", CorePersona: map[string]any{
		"identity":          map[string]any{"name": "小桂"},
		"personality":       map[string]any{"openness": 0.5, "agreeableness": 0.5},
		"behavioral_policy": map[string]any{"response_style": "自然"},
	}}
	before := jsonString(input.CorePersona)
	compiled, err := synthesizeBaselineWorkingPersona(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"openness: 0.5", "agreeableness: 0.5", "response_style: 自然"} {
		if strings.Count(compiled.PortraitText, line) != 1 {
			t.Fatalf("duplicate or missing %q: %s", line, compiled.PortraitText)
		}
	}
	if jsonString(input.CorePersona) != before {
		t.Fatal("baseline compilation mutated the source")
	}
	input.CorePersona["personality_system"] = map[string]any{"profiles": []any{map[string]any{"id": "default", "personality": map[string]any{"openness": 0.7}}}}
	compiled, err = synthesizeBaselineWorkingPersona(input)
	if err != nil || !strings.Contains(compiled.PortraitText, "openness: 0.5") || !strings.Contains(compiled.PortraitText, "openness: 0.7") {
		t.Fatal("different facts were merged", compiled, err)
	}
}

func TestR4OutfitProfileNotFoundAndMismatchAreTruthfulAndAtomic(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET core_persona=jsonb_set(core_persona,'{personality_system}','{"active_profile_id":"default","profiles":[{"id":"other"}]}'::jsonb) WHERE id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	revision := wardrobeRevision(t, f)
	for _, test := range []struct {
		profile, code string
		status        int
	}{{"typo", "outfit_profile_not_found", 404}, {"other", "outfit_profile_mismatch", 409}} {
		for _, operation := range []string{"request", "save"} {
			t.Run(operation+"/"+test.profile, func(t *testing.T) {
				body := map[string]any{"expected_revision": revision, "profile_id": test.profile, "reason": "核对人格", "idempotency_key": operation + test.profile}
				path := "/internal/fluctlights/" + f.fluctlightID + "/inventory/requests"
				if operation == "request" {
					body["kind"], body["outfit_id"], body["expected_outfit_revision"] = "outfit", "outfit-does-not-matter", 1
				} else {
					path = "/internal/fluctlights/" + f.fluctlightID + "/inventory/outfits"
					body["name"], body["item_ids"], body["expected_outfit_revision"] = "外出", []any{wardrobeItemID(t, f, "黑色短靴")}, 0
				}
				_, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
				var surface *SurfaceError
				if !errors.As(err, &surface) || surface.Code != test.code || surface.Status != test.status || strings.Contains(surface.Message, "已变化") {
					t.Fatalf("misleading profile rejection: %#v %v", surface, err)
				}
				if wardrobeRevision(t, f) != revision {
					t.Fatal("rejected selector changed wardrobe")
				}
			})
		}
	}
	f.requireNoModelRuns(t)
}

func TestR4WardrobeTimeoutRecoveryKeepsInventoryAndClearsStaleFailure(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r4-wardrobe")
	var id string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		id, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "r4")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	resource, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	proposal := map[string]any{"items": []any{
		map[string]any{"key": "top", "clothing_type": "upper_body", "description": "秋日上衣", "seasons": []any{"autumn"}},
		map[string]any{"key": "bottom", "clothing_type": "lower_body", "description": "秋日长裤", "seasons": []any{"autumn"}},
	}, "plans": []any{map[string]any{"name": "居家", "tags": []any{}, "item_refs": []any{"top", "bottom"}}}, "reason": "准备库存"}
	if _, err = f.app.commitPreparedWardrobe(f.ctx, id, f.fluctlightID, f.ownerID, "default", wardrobeRevision(t, f), resource.CurrentRevision, proposal); err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("wardrobe_preparation_v1", func(payload map[string]any) fakeProviderResult {
		calls++
		input := decodeObject([]byte(stringValue(mapValue(arrayValue(payload["messages"])[1])["content"])))
		if input["select_only"] != true {
			t.Fatal("recovery attempted to generate inventory again", input)
		}
		if calls == 1 {
			return fakeProviderResult{Err: context.DeadlineExceeded}
		}
		ids := []any{}
		for _, raw := range arrayValue(input["registered_items"]) {
			item := mapValue(raw)
			if item["description"] == "秋日上衣" || item["description"] == "秋日长裤" {
				ids = append(ids, item["id"])
			}
		}
		return fakeProviderResult{Structured: map[string]any{"items": []any{}, "plans": []any{}, "wear_item_refs": ids, "reason": "选择已登记衣物"}}
	})}
	if _, err = f.app.ProcessWardrobePreparation(f.ctx, id); err == nil {
		t.Fatal("timeout became success")
	}
	failed, err := f.app.OwnerWardrobePreparation(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || failed["status"] != "failed" || mapValue(failed["result"])["inventory_committed"] != true || stringValue(mapValue(failed["result"])["failure_reason"]) == "" {
		t.Fatal(failed, err)
	}
	recovered, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/inventory/preparation", nil, f.ownerID, map[string]any{"expected_revision": wardrobeRevision(t, f), "reason": "恢复选择", "idempotency_key": "r4-recover"})
	if err != nil || recovered["status"] != "awaiting_wear" || mapValue(recovered["result"])["failure_reason"] != nil {
		t.Fatal("recovery retains stale failure", recovered, err)
	}
	result, err := f.app.ProcessWardrobePreparation(f.ctx, id)
	if err != nil || result["status"] != "completed" || result["failure_reason"] != nil || calls != 2 {
		t.Fatal(result, calls, err)
	}
	var count int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_ref=$2`, f.fluctlightID, "wardrobe-preparation:"+id).Scan(&count); err != nil || count != 2 {
		t.Fatal("inventory duplicated", count, err)
	}
}

func TestR4DisconnectRetainsVisibleWorkerFailureAfterReconnect(t *testing.T) {
	ctx, repository, app, owner, fid, conversation := durableReplyFixture(t, "r4-disconnect-failure")
	observer, disconnect := context.WithCancel(ctx)
	payload := map[string]any{"fluctlight_id": fid, "text": "断线后失败也要可见", "idempotency_key": "r4-disconnect", "turn_id": "r4-disconnect-turn"}
	done := make(chan error, 1)
	go func() { done <- app.StreamDurableTurn(observer, httptest.NewRecorder(), owner, conversation, payload) }()
	inbox := "inbox_" + stableDigest("turn:r4-disconnect")
	deadline := time.After(5 * time.Second)
	for {
		var exists bool
		if err := repository.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_inbox WHERE id=$1)`, inbox).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists {
			break
		}
		select {
		case <-deadline:
			t.Fatal("not accepted")
		case <-time.After(10 * time.Millisecond):
		}
	}
	disconnect()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return embeddingHTTPResponse(r, 503, `{"error":"temporary"}`), nil
	})}
	if _, err := app.ProcessCognitionInbox(ctx, inbox); err == nil {
		t.Fatal("worker failure became success")
	}
	page, err := repository.History(ctx, conversation, owner, nil, 50)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].TurnStatus != "failed" || page.Messages[0].TurnErrorCode == "" || page.Messages[0].TurnID != "r4-disconnect-turn" {
		t.Fatal("reconnect hid failure", page, err)
	}
}

func TestR4WorldStarterTemplatesSupportManualTriggerAndSafeUpgrade(t *testing.T) {
	f, _, _, _ := seedNPCEncounterRuntime(t, false)
	items := arrayValue(eventRuntimeCall(t, f, "GET", "", nil)["items"])
	if len(items) != 2 {
		t.Fatal("manual templates missing", items)
	}
	for _, raw := range items {
		item := mapValue(raw)
		if item["enabled"] != false || item["source_resource_id"] == nil || item["generation_id"] != nil || validateWorldEventDefinition(mapValue(item["definition"])) != nil {
			t.Fatal("invalid starter", item)
		}
	}
	id := stringValue(mapValue(items[0])["id"])
	enabled := eventRuntimeCall(t, f, "PUT", "/"+id, map[string]any{"enabled": true, "expected_revision": 1})
	queued := eventRuntimeCall(t, f, "POST", "/"+id+"/runs", map[string]any{"expected_revision": enabled["revision"]})
	if queued["status"] != "queued" {
		t.Fatal(queued)
	}
	if _, err := f.app.RepairWorldEvents(f.ctx, 10); err != nil {
		t.Fatal(err)
	}
	run := eventRuntimeCall(t, f, "GET", "/"+id+"/runs/"+stringValue(queued["id"]), nil)
	if run["status"] != "applied" {
		t.Fatal("manual trigger did not execute", run)
	}
	// Simulate an existing 0102 world whose manual catalogue was empty.
	world := stringValue(queued["world_id"])
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.life_worlds(id,owner_actor_id,timezone,name) VALUES('r4-old-empty-world',$1,'Asia/Shanghai','旧世界')`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.runtime_settings SET value_json='{"generated_concurrency":3,"embedding_concurrency":4,"queue_wait_timeout_seconds":901}' WHERE key='llm.queue'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.alembic_version SET version_num=$1`, migrations.WorldEventStartersPreviousHead); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrations.New(f.repository.Pool()).Apply(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	var count, revision int
	var stillEnabled bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.world_event_templates WHERE world_id='r4-old-empty-world'`).Scan(&count); err != nil || count != 2 {
		t.Fatal("upgrade omitted/duplicated starters", count, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT enabled,revision FROM public.world_event_templates WHERE world_id=$1 AND id=$2`, world, id).Scan(&stillEnabled, &revision); err != nil || !stillEnabled || revision != intValue(enabled["revision"]) {
		t.Fatal("upgrade overwrote owner policy", stillEnabled, revision, err)
	}
	var queueRaw string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT value_json FROM public.runtime_settings WHERE key='llm.queue'`).Scan(&queueRaw); err != nil || intValue(decodeObject([]byte(queueRaw))["generated_concurrency"]) != 3 || intValue(decodeObject([]byte(queueRaw))["queue_wait_timeout_seconds"]) != 901 {
		t.Fatal("new migration replayed old queue defaults", queueRaw, err)
	}
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.world_event_runs WHERE id=$1`, queued["id"]).Scan(&status); err != nil || status != "applied" {
		t.Fatal("upgrade changed existing event history", status, err)
	}
}
