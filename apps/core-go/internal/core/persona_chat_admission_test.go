package core

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPersonaPreparationChatErrorClassifiesActiveAndTerminalStates(t *testing.T) {
	for state, want := range map[string]string{
		"": "", "completed": "", "pending": "persona_preparation_pending", "started": "persona_preparation_pending", "retry": "persona_preparation_pending",
		"failed": "persona_preparation_failed", "dead_letter": "persona_preparation_failed", "cancelled": "persona_preparation_failed", "terminated": "persona_preparation_failed",
	} {
		err := personaPreparationChatError(state)
		if got := ConversationTurnFailureCode(err); got != want {
			t.Fatalf("state %q code=%q want=%q", state, got, want)
		}
		if want != "" && PersonaPreparationMessage(want) == "" {
			t.Fatalf("state %q has no public explanation", state)
		}
	}
}

func seedChatPreparation(t *testing.T, ctx context.Context, repo *PostgresRepository, fid, status string) string {
	t.Helper()
	id := "chat-preparation:" + fid
	_, err := repo.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,last_error) SELECT $1,$1,'lifecycle','persona.compile',jsonb_build_object('fluctlight_id',id,'expected_revision',current_revision),$3,'persona_compilation_failed' FROM public.fluctlights WHERE id=$2 ON CONFLICT(intent_id) DO UPDATE SET status=excluded.status`, id, fid, status)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPostgresPersonaPreparationRejectsChatBeforeAcceptanceAndRecovers(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := durableReplyFixture(t, "persona-gate")
	payload := map[string]any{"fluctlight_id": fid, "text": "人格准备后再聊", "idempotency_key": "persona-gate", "turn_id": "persona-gate"}
	for _, status := range []string{"pending", "started", "retry", "failed", "dead_letter", "cancelled", "terminated"} {
		seedChatPreparation(t, ctx, repo, fid, status)
		requestCtx, cancel := context.WithTimeout(ctx, time.Second)
		_, err := app.AcceptTurn(requestCtx, owner, conversation, payload)
		cancel()
		want := "persona_preparation_failed"
		if status == "pending" || status == "started" || status == "retry" {
			want = "persona_preparation_pending"
		}
		if ConversationTurnFailureCode(err) != want {
			t.Fatalf("preparation=%s err=%v want=%s", status, err, want)
		}
	}
	var messages, inboxes int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1),(SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$2)`, conversation, fid).Scan(&messages, &inboxes); err != nil || messages != 0 || inboxes != 0 {
		t.Fatalf("rejected chat persisted data: messages=%d inboxes=%d err=%v", messages, inboxes, err)
	}
	seedChatPreparation(t, ctx, repo, fid, "completed")
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil || accepted.InboxID == "" {
		t.Fatalf("recovered preparation did not accept original draft: %#v %v", accepted, err)
	}
	// Recovery does not substitute a new user-message or turn identity.
	again, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil || again.InboxID != accepted.InboxID || again.TurnID != accepted.TurnID {
		t.Fatalf("accepted replay changed identity: %#v %v", again, err)
	}
}

func TestPostgresPersonaFailureClosesAlreadyAcceptedObserverAndHistory(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := durableReplyFixture(t, "persona-observer")
	payload := map[string]any{"fluctlight_id": fid, "text": "原消息保留", "idempotency_key": "persona-observer", "turn_id": "persona-observer"}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil {
		t.Fatal(err)
	}
	seedChatPreparation(t, ctx, repo, fid, "failed")
	response := httptest.NewRecorder()
	observerCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := app.StreamDurableTurn(observerCtx, response, owner, conversation, payload); err != nil {
		t.Fatal("accepted observer waited instead of reporting failure", err)
	}
	frames := strings.Split(strings.TrimSpace(response.Body.String()), "\n")
	if len(frames) != 2 {
		t.Fatalf("expected accepted user plus one terminal error: %s", response.Body.String())
	}
	var terminal map[string]any
	if err := json.Unmarshal([]byte(frames[1]), &terminal); err != nil || terminal["type"] != "error" || mapValue(terminal["payload"])["code"] != "persona_preparation_failed" || mapValue(terminal["payload"])["message"] != PersonaPreparationMessage("persona_preparation_failed") {
		t.Fatalf("terminal=%#v err=%v", terminal, err)
	}
	status, code, err := app.readCognitionTurnStatus(ctx, accepted.InboxID)
	if err != nil || status != "failed" || code != "persona_preparation_failed" {
		t.Fatalf("persisted status=%s code=%s err=%v", status, code, err)
	}
	page, err := repo.History(ctx, conversation, owner, nil, 50)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].TurnErrorCode != "persona_preparation_failed" || page.Messages[0].TurnStatus != "failed" {
		t.Fatalf("failed history=%#v err=%v", page, err)
	}
	seedChatPreparation(t, ctx, repo, fid, "completed")
	retried, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil || retried.InboxID != accepted.InboxID {
		t.Fatalf("recovered accepted turn could not retry in place: %#v %v", retried, err)
	}
}
