package core

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestConversationTurnBudgetFreezesConfiguredTotalAndAttempt(t *testing.T) {
	now := time.Date(2026, 10, 23, 18, 0, 0, 0, time.FixedZone("business", 8*3600))
	budget := conversationTurnBudget(now, 180, 9, 2)
	end, err := time.Parse(time.RFC3339Nano, stringValue(budget["deadline_at"]))
	if err != nil || !end.Equal(now.Add(180*time.Second)) || intValue(budget["attempt"]) != 2 || intValue(decodeObject(jsonBytes(budget))["config_version"]) != 9 {
		t.Fatal(budget, err)
	}
	if intValue(conversationTurnBudget(now, 7200, 1, 0)["timeout_seconds"]) != 3600 {
		t.Fatal("budget must respect the validated total limit")
	}
}
func TestConversationBudgetFencesOldExecutionWithoutChangingStandaloneTools(t *testing.T) {
	old := conversationTurnBudget(time.Now(), 300, 1, 0)
	next := conversationTurnBudget(time.Now(), 300, 1, 1)
	ctx := context.WithValue(context.Background(), conversationBudgetKey{}, conversationBudgetScope{"inbox", old})
	if !conversationBudgetMatches(ctx, "inbox", map[string]any{"reply_budget": decodeObject(jsonBytes(old))}) {
		t.Fatal("JSON roundtrip changed identity")
	}
	if conversationBudgetMatches(ctx, "inbox", map[string]any{"reply_budget": next}) {
		t.Fatal("old execution admitted into explicit retry")
	}
	if !conversationBudgetMatches(context.Background(), "history-fact", map[string]any{"reply_budget": next}) {
		t.Fatal("standalone tools must retain their own evidence authority")
	}
}
func TestConversationCommittedEffectsIncludeSettledCognitionAndPartialTools(t *testing.T) {
	registry, err := NewCapabilityRegistry(builtinCapabilities(nil)...)
	if err != nil {
		t.Fatal(err)
	}
	if !turnHasCommittedEffects(map[string]any{"agent_result": map[string]any{"reply_draft_ref": "draft"}}, registry) {
		t.Fatal("waiting reply already settled cognition")
	}
	if !turnHasCommittedEffects(map[string]any{"agent_partial": map[string]any{"capability_results": []any{map[string]any{"capability_name": "unknown_mutation", "status": "completed"}}}}, registry) {
		t.Fatal("committed mutation is not retryable")
	}
	if turnHasCommittedEffects(map[string]any{"agent_partial": map[string]any{"capability_results": []any{map[string]any{"capability_name": "unknown_mutation", "status": "handled"}}}}, registry) {
		t.Fatal("no-effect handled result is not a committed mutation")
	}
}
func TestConversationDeadlineQueueSweepAndPublicationFence(t *testing.T) {
	ctx, repository, app, owner, fid, conversation := durableReplyFixture(t, "budget-queue")
	body := map[string]any{"fluctlight_id": fid, "text": "budget", "turn_id": "budget-queue", "idempotency_key": "budget-queue"}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, body)
	if err != nil {
		t.Fatal(err)
	}
	past := conversationTurnBudget(app.now().Add(-time.Minute), 1, 1, 0)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{reply_budget}',$2::jsonb,true) WHERE id=$1`, accepted.InboxID, jsonBytes(past)); err != nil {
		t.Fatal(err)
	}
	err = withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, _, err := resolveConversationPublicationSourceTx(ctx, tx, fid, conversation, "turn:budget-queue", app.now)
		return err
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("late reply publication was admitted", err)
	}
	if _, err := app.ExpireDueConversationTurns(ctx, 50); err != nil {
		t.Fatal(err)
	}
	status, code, err := app.readCognitionTurnStatus(ctx, accepted.InboxID)
	if err != nil || status != "failed" || code != "request_timeout" {
		t.Fatal(status, code, err)
	}
	if _, err := app.AcceptTurn(ctx, owner, conversation, body); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	var attempts int
	if err := repository.Pool().QueryRow(ctx, `SELECT payload,attempt_count FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID).Scan(&raw, &attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || intValue(mapValue(decodeObject(raw)["reply_budget"])["attempt"]) != 1 {
		t.Fatal("explicit retry did not get independent activity budget", string(raw), attempts)
	}
	oldCtx := context.WithValue(ctx, conversationBudgetKey{}, conversationBudgetScope{accepted.InboxID, past})
	if _, _, err := app.claimCognitionInbox(oldCtx, accepted.InboxID, "old-worker"); !errors.Is(err, ErrConflict) {
		t.Fatal("old attempt claimed retry", err)
	}
	if err := app.failAgentTurnAfterRun(oldCtx, accepted.InboxID, agentCommittedOutcome{}, "old-failure"); !errors.Is(err, ErrConflict) {
		t.Fatal("old failure contaminated retry", err)
	}
	result, err := app.expireConversationTurn(ctx, accepted.InboxID, past)
	if err != nil || result["status"] != "waiting_for_agent" {
		t.Fatal(result, err)
	}
}
func TestConversationDeadlinePreservesSettledWaitingReply(t *testing.T) {
	ctx, repository, app, owner, fid, conversation := durableReplyFixture(t, "budget-settled")
	body := map[string]any{"fluctlight_id": fid, "text": "wait for media", "turn_id": "budget-settled", "idempotency_key": "budget-settled"}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, body)
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	payload := decodeObject(raw)
	budget := mapValue(payload["reply_budget"])
	payload["agent_result"] = map[string]any{"reply_draft_ref": "already-settled"}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET status='waiting_reply',payload=$2 WHERE id=$1`, accepted.InboxID, jsonBytes(payload)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.expireConversationTurn(ctx, accepted.InboxID, budget); err != nil {
		t.Fatal(err)
	}
	_, code, err := app.readCognitionTurnStatus(ctx, accepted.InboxID)
	if err != nil || code != "agent_partial_committed" {
		t.Fatal(code, err)
	}
	if _, err := app.AcceptTurn(ctx, owner, conversation, body); err != nil {
		t.Fatal(err)
	}
	status, _, err := app.readCognitionTurnStatus(ctx, accepted.InboxID)
	if err != nil || status != "failed" {
		t.Fatal("settled cognition must never run a second time", status, err)
	}
}

func TestConversationDeadlineHistoricalPublicationUsesOriginalAdmissionTime(t *testing.T) {
	ctx, repository, app, owner, fid, conversation := durableReplyFixture(t, "budget-historical")
	accepted, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "historical", "turn_id": "budget-historical", "idempotency_key": "budget-historical"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=payload-'reply_budget',occurred_at=$2 WHERE id=$1`, accepted.InboxID, app.now().Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	err = withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, _, err := resolveConversationPublicationSourceTx(ctx, tx, fid, conversation, "turn:budget-historical", app.now)
		return err
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("historical turn published after total deadline", err)
	}
}

type blockedConversationBudgetTransport struct{}

func (blockedConversationBudgetTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	<-request.Context().Done()
	return nil, request.Context().Err()
}
func TestConversationDeadlineCancelsPhysicalProviderAndSettlesOriginalInbox(t *testing.T) {
	ctx, repository, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "budget-provider", blockedConversationBudgetTransport{})
	config := defaultConversationReplySettings()
	config.TimeoutSeconds = 1
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('conversation.reply',$1) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`, jsonString(config)); err != nil {
		t.Fatal(err)
	}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "等待受限的模型回应", "turn_id": "budget-provider", "idempotency_key": "budget-provider"})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	result, err := app.ProcessCognitionInbox(ctx, accepted.InboxID)
	if err != nil || result["status"] != "failed" || time.Since(started) > 5*time.Second {
		t.Fatal(result, err, time.Since(started))
	}
	status, code, err := app.readCognitionTurnStatus(ctx, accepted.InboxID)
	if err != nil || status != "failed" || code != "request_timeout" {
		t.Fatal(status, code, err)
	}
}
