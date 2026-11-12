package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// A restarted worker must resume the same logical direct turn using the model
// selected before its first Provider request, even if configuration changes.
func TestDirectTurnRetryKeepsOriginalModelBinding(t *testing.T) {
	var models []string
	firstAttempt := true
	const fixtureSecret = "isolated-direct-snapshot-secret"
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		messages := jsonString(payload["messages"])
		if strings.Contains(messages, directTurnModelSnapshotContractKey) || strings.Contains(messages, directTurnModelSnapshotKey) || strings.Contains(messages, fixtureSecret) || strings.Contains(messages, "turn-secret") || strings.Contains(messages, "turn.invalid") {
			t.Error("runtime snapshot metadata or credential entered model messages")
		}
		if firstAttempt {
			return fakeProviderResult{Status: 400}
		}
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("snapshot-retry-reply", conversationReplyCapabilityName, map[string]any{"text": "重试完成。", "decision": decision})}}
	})
	transport := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer "+fixtureSecret {
			t.Error("the resolved Provider credential was not restored")
		}
		return router.RoundTrip(request)
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-turn-snapshot", transport)
	key := []byte("0123456789abcdef0123456789abcdef")
	encrypted, err := encryptSecret(key, "turn-secret", fixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	app.Provider.SettingsKey = key
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	role := agentModelBindingKey(FormalAgentConversationCognition, "conversation_turn_response")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-direct-turn-snapshot','model-a',30)`, role); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "direct-turn-snapshot", "idempotency_key": "direct-turn-snapshot", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil {
		t.Fatal(err)
	}
	var sourceFingerprint string
	if err := repo.Pool().QueryRow(ctx, `SELECT public.cognition_source_fingerprint(payload) FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID).Scan(&sourceFingerprint); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err == nil {
		t.Fatal("controlled first attempt unexpectedly succeeded")
	}
	initialRequests := len(models)
	if initialRequests == 0 {
		t.Fatal("first attempt made no model request")
	}
	var afterFingerprint string
	var saved []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT public.cognition_source_fingerprint(payload),payload->'agent_partial'->$2 FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID, directTurnModelSnapshotKey).Scan(&afterFingerprint, &saved); err != nil {
		t.Fatal(err)
	}
	if sourceFingerprint != afterFingerprint || len(saved) == 0 || strings.Contains(string(saved), fixtureSecret) {
		t.Fatal("model snapshot changed the source fact, was missing, or contained a credential")
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='model-b' WHERE role=$1`, role); err != nil {
		t.Fatal(err)
	}
	firstAttempt = false
	// A fresh App discards all process-local bindings before the same turn is
	// explicitly reopened through the normal durable entry point.
	app = newTestApp(t, repo, transport)
	app.Provider.SettingsKey = key
	reopened, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil || reopened.InboxID != accepted.InboxID {
		t.Fatalf("same turn could not be reopened: %#v %v", reopened, err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err != nil {
		t.Fatalf("retry did not complete: models=%v err=%v", models, err)
	}
	if len(models) <= initialRequests || models[0] != "model-a" {
		t.Fatalf("model requests missing: %v", models)
	}
	for _, model := range models[initialRequests:] {
		if model != "model-a" {
			t.Fatalf("same logical turn silently changed model: first=%s retry=%s requests=%s", models[0], model, strings.Join(models, ","))
		}
	}
	var users, replies int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='user'), (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='重试完成。')`, conversation).Scan(&users, &replies); err != nil || users != 1 || replies != 1 {
		t.Fatalf("retry duplicated or lost messages: users=%d replies=%d err=%v", users, replies, err)
	}
	newTurn := map[string]any{"fluctlight_id": fid, "text": "新的一轮", "turn_id": "direct-turn-snapshot-new", "idempotency_key": "direct-turn-snapshot-new", "attachment_refs": []any{}}
	newRequests := len(models)
	if _, err := app.HandleTurn(ctx, owner, conversation, newTurn); err != nil || len(models) <= newRequests || models[newRequests] != "model-b" {
		t.Fatalf("new turn did not use updated binding: models=%v err=%v", models, err)
	}
	t.Logf("M1_04B first=%s retry=%s new_turn=%s fingerprint_preserved=true snapshot_has_secret=false", models[0], models[initialRequests], models[newRequests])
}

func TestDirectTurnMissingOrCorruptSnapshotDoesNotRebind(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt", "marker_missing", "partial_null"} {
		t.Run(kind, func(t *testing.T) {
			requests := 0
			router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
				requests++
				return fakeProviderResult{Status: 400}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "snapshot-"+kind, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			payload := map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "snapshot-" + kind, "idempotency_key": "snapshot-" + kind, "attachment_refs": []any{}}
			accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err == nil || requests == 0 {
				t.Fatalf("controlled first attempt did not reach Provider: requests=%d err=%v", requests, err)
			}
			switch kind {
			case "missing":
				_, err = repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{agent_partial}',(payload->'agent_partial')-$2::text,true) WHERE id=$1`, accepted.InboxID, directTurnModelSnapshotKey)
			case "corrupt":
				_, err = repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,ARRAY['agent_partial',$2::text],'{"version":"invalid"}'::jsonb,true) WHERE id=$1`, accepted.InboxID, directTurnModelSnapshotKey)
			case "marker_missing":
				_, err = repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=payload-$2::text WHERE id=$1`, accepted.InboxID, directTurnModelSnapshotContractKey)
			case "partial_null":
				_, err = repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{agent_partial}','null'::jsonb,true) WHERE id=$1`, accepted.InboxID)
			}
			if err != nil {
				t.Fatal(err)
			}
			beforeRetry := requests
			app = newTestApp(t, repo, router)
			if _, err := app.AcceptTurn(ctx, owner, conversation, payload); err != nil {
				t.Fatal(err)
			}
			_, err = app.ProcessCognitionInbox(ctx, accepted.InboxID)
			want := "direct_turn_model_snapshot_" + map[string]string{"missing": "missing", "corrupt": "invalid", "marker_missing": "invalid", "partial_null": "invalid"}[kind]
			if err == nil || !strings.Contains(err.Error(), want) || requests != beforeRetry {
				t.Fatalf("%s snapshot silently rebound or hit Provider: requests=%d before=%d err=%v", kind, requests, beforeRetry, err)
			}
		})
	}
}

func TestDirectTurnLegacyWithoutSnapshotUsesCurrentModelAndRecordsUnknown(t *testing.T) {
	var models []string
	firstAttempt := true
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		if firstAttempt {
			return fakeProviderResult{Status: 400}
		}
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("legacy-reply", conversationReplyCapabilityName, map[string]any{"text": "继续聊。", "decision": decision})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "legacy-direct-snapshot", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	role := agentModelBindingKey(FormalAgentConversationCognition, "conversation_turn_response")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-legacy-direct-snapshot','model-a',30)`, role); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "legacy-direct-snapshot", "idempotency_key": "legacy-direct-snapshot", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=payload-$2::text WHERE id=$1`, accepted.InboxID, directTurnModelSnapshotContractKey); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err == nil || len(models) == 0 {
		t.Fatalf("legacy first attempt did not reach Provider: models=%v err=%v", models, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='model-b' WHERE role=$1`, role); err != nil {
		t.Fatal(err)
	}
	initialRequests := len(models)
	firstAttempt = false
	app = newTestApp(t, repo, router)
	if _, err := app.AcceptTurn(ctx, owner, conversation, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err != nil || len(models) <= initialRequests || models[initialRequests] != "model-b" {
		t.Fatalf("legacy current binding was not used: models=%v err=%v", models, err)
	}
	var fabricated bool
	var diagnostics int
	if err := repo.Pool().QueryRow(ctx, `SELECT COALESCE((payload->'agent_partial') ? $2::text,false),(SELECT count(*) FROM public.diagnostic_events WHERE event_type='direct_turn.model_binding.legacy_unfrozen' AND correlation_id=$3 AND payload->>'reason_code'='legacy_model_binding_unavailable') FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID, directTurnModelSnapshotKey, "turn:legacy-direct-snapshot").Scan(&fabricated, &diagnostics); err != nil || fabricated || diagnostics != 1 {
		t.Fatalf("legacy original model was fabricated or not diagnosed: fabricated=%t diagnostics=%d err=%v", fabricated, diagnostics, err)
	}
	t.Logf("M1_04B legacy_current_model=%s prior_model_known=false snapshot_fabricated=%t", models[initialRequests], fabricated)
}

func TestDirectTurnRetryKeepsModelWhenReplyModeChanges(t *testing.T) {
	var models []string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		return fakeProviderResult{Status: 400}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "snapshot-mode-change", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	role := agentModelBindingKey(FormalAgentConversationCognition, "conversation_turn_response")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-snapshot-mode-change','model-a',30)`, role); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "snapshot-mode-change", "idempotency_key": "snapshot-mode-change", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err == nil || len(models) == 0 {
		t.Fatalf("initial request missing: models=%v err=%v", models, err)
	}
	initialRequests := len(models)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='model-b' WHERE role=$1`, role); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "fact_only")
	app = newTestApp(t, repo, router)
	if _, err := app.AcceptTurn(ctx, owner, conversation, payload); err != nil {
		t.Fatal(err)
	}
	_, retryErr := app.ProcessCognitionInbox(ctx, accepted.InboxID)
	if retryErr == nil || len(models) <= initialRequests || models[initialRequests] != "model-a" {
		t.Fatalf("mode change bypassed frozen cognition model: models=%v err=%v", models, retryErr)
	}
}

func TestDirectTurnStaleSnapshotClaimDoesNotFailNewOwner(t *testing.T) {
	requests := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		requests++
		return fakeProviderResult{Status: 400}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "snapshot-stale-claim", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	payload := map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "snapshot-stale-claim", "idempotency_key": "snapshot-stale-claim", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a claim already transferred to a newer worker while an older
	// execution still holds its original claim token.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET status='claimed',claimed_by='new-worker',claimed_at=now() WHERE id=$1`, accepted.InboxID); err != nil {
		t.Fatal(err)
	}
	ctx = context.WithValue(ctx, directTurnClaimKey{}, directTurnClaim{InboxID: accepted.InboxID, Owner: "old-worker"})
	_, runErr := app.HandleTurn(ctx, owner, conversation, payload)
	var status, claimant string
	if err := repo.Pool().QueryRow(ctx, `SELECT status,COALESCE(claimed_by,'') FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID).Scan(&status, &claimant); err != nil {
		t.Fatal(err)
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "direct_turn_snapshot_stale") || status != "claimed" || claimant != "new-worker" || requests != 0 {
		t.Fatalf("stale worker damaged new claim: status=%s claimant=%s requests=%d err=%v", status, claimant, requests, runErr)
	}
}

func TestDirectTurnSnapshotDoesNotSendRotatedCredentialToOldDestination(t *testing.T) {
	requests := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		requests++
		return fakeProviderResult{Status: 400}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "snapshot-destination-change", router)
	key := []byte("0123456789abcdef0123456789abcdef")
	encrypted, err := encryptSecret(key, "turn-secret", "isolated-destination-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	app.Provider.SettingsKey = key
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	payload := map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "snapshot-destination-change", "idempotency_key": "snapshot-destination-change", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err == nil || requests == 0 {
		t.Fatalf("initial Provider attempt missing: requests=%d err=%v", requests, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url='http://changed.invalid' WHERE id='mixed-endpoint-snapshot-destination-change'`); err != nil {
		t.Fatal(err)
	}
	rotated, err := encryptSecret(key, "turn-secret", "isolated-rotated-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.setting_secrets SET ciphertext=$1,nonce=$2 WHERE purpose='turn-secret'`, rotated.ciphertext, rotated.nonce); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.configuration_revisions(resource_key,revision) VALUES('secret:turn-secret',1) ON CONFLICT(resource_key) DO UPDATE SET revision=configuration_revisions.revision+1`); err != nil {
		t.Fatal(err)
	}
	beforeRetry := requests
	app = newTestApp(t, repo, router)
	app.Provider.SettingsKey = key
	if _, err := app.AcceptTurn(ctx, owner, conversation, payload); err != nil {
		t.Fatal(err)
	}
	_, runErr := app.ProcessCognitionInbox(ctx, accepted.InboxID)
	if runErr == nil || !strings.Contains(runErr.Error(), "provider_snapshot_credential_destination_changed") || requests != beforeRetry {
		t.Fatalf("rotated credential could reach the old destination: requests=%d before=%d err=%v", requests, beforeRetry, runErr)
	}
}
