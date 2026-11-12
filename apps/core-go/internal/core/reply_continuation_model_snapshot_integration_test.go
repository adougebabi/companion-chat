package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The media completion is controlled in the isolated database; cognition and
// Writer requests still use the actual native Tool and continuation paths.
func TestReplyContinuationKeepsOriginalCognitionModel(t *testing.T) {
	testReplyContinuationCognitionBinding(t, false)
}

func TestReplyContinuationLegacyCognitionUsesCurrentModelWithoutInventingSnapshot(t *testing.T) {
	testReplyContinuationCognitionBinding(t, true)
}

func testReplyContinuationCognitionBinding(t *testing.T, legacy bool) {
	t.Helper()
	var draftRef, actionRef string
	var mainModels []string
	writerCalls := 0
	resume := false
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		model := stringValue(payload["model"])
		if model == "writer-model" {
			writerCalls++
			if writerCalls == 1 {
				return fakeProviderResult{ToolCalls: []map[string]any{{"id": "writer-image", "type": "function", "function": map[string]any{"name": replyImageRequestName, "arguments": `{"intent":"一张夜景图"}`}}}}
			}
			return fakeProviderResult{Text: "图片暂时不可用，我们继续聊。"}
		}
		mainModels = append(mainModels, model)
		if resume {
			// Stop after observing the resumed physical request. Media completion
			// below is a controlled fixture, not a full publication scenario.
			return fakeProviderResult{Status: 400}
		}
		if !conversationPayloadHasToolResult(payload) {
			args := map[string]any{}
			return fakeProviderResult{ToolCalls: []map[string]any{{"id": fmt.Sprintf("reply-%d", len(mainModels)), "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(args)}}}}
		}
		return fakeProviderResult{Structured: validToolOnlyConversationFinal()}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "cognition-snapshot", router)
	key := []byte("0123456789abcdef0123456789abcdef")
	const fixtureSecret = "turn-secret-value"
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
	persistConversationReplyMode(t, ctx, repo, "fact_only")
	mainRole := agentModelBindingKey(FormalAgentConversationCognition, "conversation_turn_response")
	writerRole := agentModelBindingKey(FormalAgentConversationReply, "private_reply")
	for _, binding := range []struct{ role, model string }{{mainRole, "main-a"}, {writerRole, "writer-model"}} {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-cognition-snapshot',$2,30)`, binding.role, binding.model); err != nil {
			t.Fatal(err)
		}
	}
	turn, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "请发一张夜景图", "turn_id": "cognition-snapshot-turn", "idempotency_key": "cognition-snapshot-turn", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput{AuthorizationActorID: owner, SpeakerActorID: owner, FluctlightID: fid, ConversationID: conversation, SourceFactID: turn.InboxID, RunID: turn.TurnID, CorrelationID: "turn:" + turn.TurnID, CurrentInput: "请发一张夜景图"})
	if err != nil && !strings.Contains(err.Error(), "conversation_native_output_missing") {
		t.Fatalf("initial cognition failed: %v", err)
	}
	var binding []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT id,binding_snapshot FROM public.conversation_reply_drafts WHERE source_fact_id=$1`, turn.InboxID).Scan(&draftRef, &binding); err != nil {
		t.Fatal(err)
	}
	if legacy {
		// Model an old draft that predates cognition snapshots. The test does
		// not assert any knowable historical model for this legacy record.
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_reply_drafts SET binding_snapshot=binding_snapshot-$2 WHERE id=$1`, draftRef, replyCognitionSnapshotKey); err != nil {
			t.Fatal(err)
		}
		if err := repo.Pool().QueryRow(ctx, `SELECT binding_snapshot FROM public.conversation_reply_drafts WHERE id=$1`, draftRef).Scan(&binding); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT ref FROM public.conversation_reply_actions WHERE draft_id=$1`, draftRef).Scan(&actionRef); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE public.conversation_reply_actions SET status='failed',error_code='controlled_failure' WHERE ref=$1`, actionRef); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.conversation_reply_drafts SET status='waiting_media' WHERE id=$1`, draftRef); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='waiting_reply' WHERE id=$1`, turn.InboxID); err != nil {
			return err
		}
		if err := enqueueReplyContinuationTx(ctx, tx, draftRef); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE public.conversation_reply_continuations SET status='started' WHERE draft_id=$1`, draftRef)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='main-b' WHERE role=$1`, mainRole); err != nil {
		t.Fatal(err)
	}
	initialRequests := len(mainModels)
	if initialRequests == 0 {
		t.Fatal("initial cognition request was not observed")
	}
	resume = true
	_, err = app.ProcessReplyContinuation(ctx, draftRef)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("controlled continuation failure not returned: main_models=%v err=%v", mainModels, err)
	}
	if len(mainModels) <= initialRequests {
		t.Fatal("continuation did not make a new cognition request")
	}
	for index, model := range mainModels {
		want := "main-a"
		if legacy && index >= initialRequests {
			want = "main-b"
		}
		if model != want {
			t.Fatalf("unexpected cognition model at request %d: got=%s want=%s all=%v", index, model, want, mainModels)
		}
	}
	var stored map[string]any
	if err := json.Unmarshal(binding, &stored); err != nil || strings.Contains(string(binding), fixtureSecret) {
		t.Fatalf("draft binding could not be read safely: %v", err)
	}
	_, hasSnapshot := stored[replyCognitionSnapshotKey]
	if legacy {
		var currentBinding []byte
		var diagnostics int
		if err := repo.Pool().QueryRow(ctx, `SELECT binding_snapshot,(SELECT count(*) FROM public.diagnostic_events WHERE correlation_id=$1 AND event_type='reply_continuation.model_binding.legacy_unfrozen' AND payload->>'reason_code'='legacy_model_binding_unavailable') FROM public.conversation_reply_drafts WHERE id=$1`, draftRef).Scan(&currentBinding, &diagnostics); err != nil {
			t.Fatal(err)
		}
		_, fabricated := decodeObject(currentBinding)[replyCognitionSnapshotKey]
		if hasSnapshot || fabricated || diagnostics != 1 {
			t.Fatalf("legacy original model was fabricated or not recorded: original_snapshot=%t current_snapshot=%t diagnostics=%d", hasSnapshot, fabricated, diagnostics)
		}
	} else {
		if !hasSnapshot {
			t.Fatal("draft did not persist the original cognition binding")
		}
		var cognition workflowModelSnapshot
		if err := json.Unmarshal(jsonBytes(stored[replyCognitionSnapshotKey]), &cognition); err != nil || cognition.Assignment.ModelID != "main-a" || stringValue(mapValue(stored["assignment"])["ModelID"]) != "writer-model" {
			t.Fatalf("cognition and Writer bindings were not kept distinct: %v", err)
		}
	}
	if writerCalls < 2 {
		t.Fatalf("independent Writer was not used: calls=%d", writerCalls)
	}
	t.Logf("M1_04A legacy=%t initial_model=%s resumed_model=%s initial_requests=%d resumed_requests=%d snapshot_present=%t controlled_provider_http=400", legacy, mainModels[0], mainModels[initialRequests], initialRequests, len(mainModels)-initialRequests, hasSnapshot)
}
