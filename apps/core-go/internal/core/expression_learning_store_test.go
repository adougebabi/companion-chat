package core

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"
)

func TestExpressionLearningOwnerSourceReviewAndRevocation(t *testing.T) {
	ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 2)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversation, owner, fid); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repo, "expression-learning-provider")
	calls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Structured: map[string]any{"candidates": []any{map[string]any{"source_ref": "s1", "excerpt": "raw-summary", "situation": "讨论进度"}}}}
	})
	app := newTestApp(t, repo, router)
	settings, err := app.OwnerExpressionLearningSettings(ctx, owner, fid)
	if err != nil || settings["learning_enabled"] != false || settings["use_enabled"] != false || intValue(settings["revision"]) != 0 {
		t.Fatalf("unsafe default settings=%#v err=%v", settings, err)
	}
	if _, err := app.ExtractOwnerExpressionCandidates(ctx, owner, fid, map[string]any{"source_message_id": "summary-message-1", "idempotency_key": "expression-disabled"}); err == nil || calls != 0 {
		t.Fatalf("disabled learning reached model: calls=%d err=%v", calls, err)
	}
	change := map[string]any{"learning_enabled": true, "use_enabled": false, "expected_revision": 0, "idempotency_key": "expression-settings"}
	if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, change); err != nil {
		t.Fatal(err)
	}
	if replay, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, change); err != nil || replay["_replay"] != true {
		t.Fatalf("settings replay=%#v err=%v", replay, err)
	}
	if _, err := app.ExtractOwnerExpressionCandidates(ctx, owner, fid, map[string]any{"source_message_id": "summary-message-2", "idempotency_key": "assistant-source"}); err == nil || calls != 0 {
		t.Fatalf("assistant message became a learning source: calls=%d err=%v", calls, err)
	}
	body := map[string]any{"source_message_id": "summary-message-1", "idempotency_key": "expression-extract"}
	result, err := app.ExtractOwnerExpressionCandidates(ctx, owner, fid, body)
	ids := arrayValue(result["candidate_ids"])
	if err != nil || calls != 1 || len(ids) != 1 {
		t.Fatalf("source extraction=%#v calls=%d err=%v", result, calls, err)
	}
	id := stringValue(ids[0])
	if replay, err := app.ExtractOwnerExpressionCandidates(ctx, owner, fid, body); err != nil || replay["_replay"] != true || calls != 1 {
		t.Fatalf("extraction replay=%#v calls=%d err=%v", replay, calls, err)
	}
	list, err := app.OwnerExpressionLearningCandidates(ctx, owner, fid)
	items := arrayValue(list["items"])
	if err != nil || len(items) != 1 || mapValue(items[0])["status"] != "pending" || mapValue(items[0])["source_valid"] != true {
		t.Fatalf("unreviewed candidate=%#v err=%v", list, err)
	}
	if _, err := app.OwnerExpressionLearningCandidates(ctx, "another-owner", fid); err == nil {
		t.Fatal("another owner read expression candidates")
	}
	enable := map[string]any{"expected_revision": 1, "reason": "适合讨论进度", "trigger_text": "进度", "idempotency_key": "expression-enable"}
	enabled, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, id, "enable", enable)
	if err != nil || enabled["status"] != "enabled" || intValue(enabled["revision"]) != 2 {
		t.Fatalf("enable=%#v err=%v", enabled, err)
	}
	if replay, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, id, "enable", enable); err != nil || replay["_replay"] != true {
		t.Fatalf("review replay=%#v err=%v", replay, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='原消息已更改' WHERE id='summary-message-1'`); err != nil {
		t.Fatal(err)
	}
	list, err = app.OwnerExpressionLearningCandidates(ctx, owner, fid)
	if err != nil || mapValue(arrayValue(list["items"])[0])["source_valid"] != false {
		t.Fatalf("changed source was still marked valid: %#v err=%v", list, err)
	}
	if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, id, "disable", map[string]any{"expected_revision": 2, "reason": "暂停使用", "idempotency_key": "expression-disable"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, id, "enable", map[string]any{"expected_revision": 3, "reason": "恢复", "trigger_text": "进度", "idempotency_key": "expression-stale-enable"}); err == nil {
		t.Fatal("changed source was re-enabled")
	}
	revoked, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, id, "revoke", map[string]any{"expected_revision": 3, "reason": "撤销", "idempotency_key": "expression-revoke"})
	if err != nil || revoked["status"] != "revoked" || intValue(revoked["revision"]) != 4 {
		t.Fatalf("revoke=%#v err=%v", revoked, err)
	}
	if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, id, "enable", map[string]any{"expected_revision": 4, "reason": "恢复", "trigger_text": "进度", "idempotency_key": "expression-revoked-enable"}); err == nil {
		t.Fatal("revoked candidate was re-enabled")
	}
}

func TestExpressionLearningOwnerSurfaceRoutes(t *testing.T) {
	ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 2)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversation, owner, fid); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repo, "expression-route-provider")
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"candidates": []any{map[string]any{"source_ref": "s1", "excerpt": "raw-summary", "situation": "讨论进度"}}}}
	})
	app := newTestApp(t, repo, router)
	base := "/internal/fluctlights/" + fid + "/expression-learning"
	call := func(method, path string, body map[string]any) map[string]any {
		t.Helper()
		if !MatchOwnerSurface(method, path) {
			t.Fatalf("route is not registered: %s %s", method, path)
		}
		result, err := app.DispatchOwnerSurface(ctx, method, path, url.Values{}, owner, body)
		if err != nil {
			t.Fatalf("route %s %s: %v", method, path, err)
		}
		return result
	}
	call("PUT", base+"/settings", map[string]any{"learning_enabled": true, "use_enabled": true, "expected_revision": 0, "idempotency_key": "route-settings"})
	sources := arrayValue(call("GET", base+"/sources", nil)["items"])
	if len(sources) != 1 || stringValue(mapValue(sources[0])["id"]) != "summary-message-1" {
		t.Fatalf("source route returned non-human messages: %#v", sources)
	}
	result := call("POST", base+"/extractions", map[string]any{"source_message_id": "summary-message-1", "idempotency_key": "route-extract"})
	id := stringValue(arrayValue(result["candidate_ids"])[0])
	call("POST", base+"/candidates/"+id+"/enable", map[string]any{"expected_revision": 1, "reason": "审核通过", "trigger_text": "进度", "idempotency_key": "route-enable"})
	view := call("GET", base, nil)
	if mapValue(view["settings"])["use_enabled"] != true || mapValue(arrayValue(view["items"])[0])["status"] != "enabled" {
		t.Fatalf("route readback=%#v", view)
	}
	if _, err := app.DispatchOwnerSurface(ctx, "GET", base, url.Values{}, "another-owner", nil); err == nil {
		t.Fatal("foreign owner accessed expression route")
	}
}

func TestExpressionExtractionHonorsLearningSwitchGeneration(t *testing.T) {
	for _, learningToggle := range []bool{false, true} {
		name := "use_switch_only"
		if learningToggle {
			name = "learning_disabled_then_enabled"
		}
		t.Run(name, func(t *testing.T) {
			ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
			seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 2)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversation, owner, fid); err != nil {
				t.Fatal(err)
			}
			seedCognitiveProviderRole(t, ctx, repo, "expression-switch-provider")
			var app *App
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				if learningToggle {
					if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": false, "use_enabled": false, "expected_revision": 1, "idempotency_key": "learning-off"}); err != nil {
						t.Error(err)
					}
					if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": false, "expected_revision": 2, "idempotency_key": "learning-on-again"}); err != nil {
						t.Error(err)
					}
				} else if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": true, "expected_revision": 1, "idempotency_key": "use-on"}); err != nil {
					t.Error(err)
				}
				return fakeProviderResult{Structured: map[string]any{"candidates": []any{map[string]any{"source_ref": "s1", "excerpt": "raw-summary", "situation": "讨论进度"}}}}
			})
			app = newTestApp(t, repo, router)
			if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": false, "expected_revision": 0, "idempotency_key": "learning-on"}); err != nil {
				t.Fatal(err)
			}
			_, err := app.ExtractOwnerExpressionCandidates(ctx, owner, fid, map[string]any{"source_message_id": "summary-message-1", "idempotency_key": "switch-extract"})
			if (err != nil) != learningToggle {
				t.Fatalf("learning toggle=%v extraction error=%v", learningToggle, err)
			}
			var count int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.expression_learning_candidates`).Scan(&count); err != nil || (learningToggle && count != 0) || (!learningToggle && count != 1) {
				t.Fatalf("learning toggle=%v candidates=%d err=%v", learningToggle, count, err)
			}
		})
	}
}

func TestExpressionExtractionLeaseRenewsAndCancelsAfterTakeover(t *testing.T) {
	ctx, repo, _, fid, _ := seedConversationSummaryAuthority(t)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.expression_learning_extractions(id,fluctlight_id,owner_actor_id,source_message_id,source_fingerprint,source_revision,learning_generation,idempotency_key,request_digest,status,lease_owner,lease_until)
		VALUES('lease-test',$1,'owner','message','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',0,1,'lease-test','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','running','first-owner',now()+interval '1 second')`, fid); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo}
	work, stop := app.maintainExpressionExtractionLease(ctx, "lease-test", "first-owner", 20*time.Millisecond)
	deadline := time.After(3 * time.Second)
	for {
		var renewed bool
		if err := repo.Pool().QueryRow(ctx, `SELECT lease_until>now()+interval '9 minutes' FROM public.expression_learning_extractions WHERE id='lease-test'`).Scan(&renewed); err != nil {
			_ = stop()
			t.Fatal(err)
		}
		if renewed {
			break
		}
		select {
		case <-deadline:
			_ = stop()
			t.Fatal("active extraction lease was not renewed")
		case <-time.After(20 * time.Millisecond):
		}
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.expression_learning_extractions SET lease_owner='replacement' WHERE id='lease-test'`); err != nil {
		_ = stop()
		t.Fatal(err)
	}
	select {
	case <-work.Done():
	case <-time.After(3 * time.Second):
		_ = stop()
		t.Fatal("lost lease did not cancel model context")
	}
	if err := stop(); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("lost lease error=%v", err)
	}
}

func TestExpressionExtractionRejectsSourceChangedDuringModelCall(t *testing.T) {
	ctx, repo, owner, fid, conversation := seedConversationSummaryAuthority(t)
	seedConversationSummaryMessages(t, ctx, repo, owner, fid, conversation, 1, 2)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversation, owner, fid); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repo, "expression-drift-provider")
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='来源在模型调用中变化' WHERE id='summary-message-1'`); err != nil {
			t.Error(err)
		}
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='raw-summary-message-1' WHERE id='summary-message-1'`); err != nil {
			t.Error(err)
		}
		return fakeProviderResult{Structured: map[string]any{"candidates": []any{map[string]any{"source_ref": "s1", "excerpt": "raw-summary", "situation": "讨论进度"}}}}
	})
	app := newTestApp(t, repo, router)
	if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": false, "expected_revision": 0, "idempotency_key": "expression-drift-settings"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ExtractOwnerExpressionCandidates(ctx, owner, fid, map[string]any{"source_message_id": "summary-message-1", "idempotency_key": "expression-drift"}); err == nil {
		t.Fatal("source drift was accepted")
	}
	var count int
	var status string
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.expression_learning_candidates),status FROM public.expression_learning_extractions WHERE owner_actor_id=$1 AND idempotency_key='expression-drift'`, owner).Scan(&count, &status); err != nil || count != 0 || status != "failed" {
		t.Fatalf("drift left candidate count=%d status=%s err=%v", count, status, err)
	}
	recovery, err := app.OwnerExpressionExtractionStatus(ctx, owner, fid, "expression-drift")
	if err != nil || recovery["status"] != "failed" || recovery["no_effect"] != true {
		t.Fatalf("failed extraction recovery=%#v err=%v", recovery, err)
	}
	if _, err := app.ExtractOwnerExpressionCandidates(ctx, owner, fid, map[string]any{"source_message_id": "summary-message-1", "idempotency_key": "expression-drift"}); err == nil {
		t.Fatal("terminal failed extraction reused the original identity")
	}
	unknown, err := app.OwnerExpressionExtractionStatus(ctx, owner, fid, "not-yet-visible")
	if err != nil || unknown["status"] != "not_found" || unknown["no_effect"] != false {
		t.Fatalf("missing extraction incorrectly proved no admission: %#v err=%v", unknown, err)
	}
}
