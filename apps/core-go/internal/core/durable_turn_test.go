package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func durableReplyFixture(t *testing.T, suffix string) (context.Context, *PostgresRepository, *App, string, string, string) {
	t.Helper()
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupMixedMediaReplyTurn(t, suffix, fakeProviderResult{ToolCalls: mixedImageReplyToolCalls()})
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	return ctx, repository, app, ownerID, fluctlightID, conversationID
}

func TestDurableBrowserObserverDisconnectDoesNotCancelWorker(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := durableReplyFixture(t, "disconnect")
	payload := map[string]any{"fluctlight_id": fluctlightID, "text": "断线后继续", "idempotency_key": "durable-disconnect", "turn_id": "durable-disconnect-turn", "attachment_refs": []any{}}
	observerCtx, disconnect := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- app.StreamDurableTurn(observerCtx, httptest.NewRecorder(), ownerID, conversationID, payload)
	}()
	inboxID := "inbox_" + stableDigest("turn:durable-disconnect")
	deadline := time.After(5 * time.Second)
	for {
		var count int
		if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("browser turn was not accepted")
		case <-time.After(10 * time.Millisecond):
		}
	}
	disconnect()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("observer disconnect error = %v", err)
	}
	var status string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&status); err != nil || status != "pending" {
		t.Fatalf("inbox after disconnect = %q, err=%v", status, err)
	}
	result, err := app.ProcessCognitionInbox(ctx, inboxID)
	if err != nil || stringValue(result["status"]) != "processed" {
		t.Fatalf("worker result=%#v err=%v", result, err)
	}
	page, err := repository.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 || page.Messages[0].TurnStatus != "completed" || page.Messages[1].Text != "诶？真的要看啊。" {
		t.Fatalf("history after worker = %#v result=%#v", page.Messages, result)
	}
}

func TestDurableObserverEmitsWorkerCommittedReplyAndTerminal(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := durableReplyFixture(t, "observed-worker")
	payload := map[string]any{"fluctlight_id": fluctlightID, "text": "等待后台回复", "idempotency_key": "durable-observed", "turn_id": "durable-observed-turn", "attachment_refs": []any{}}
	response := httptest.NewRecorder()
	done := make(chan error, 1)
	go func() { done <- app.StreamDurableTurn(ctx, response, ownerID, conversationID, payload) }()
	inboxID := "inbox_" + stableDigest("turn:durable-observed")
	deadline := time.After(5 * time.Second)
	for {
		var count int
		if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("turn was not accepted")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, err := app.ProcessCognitionInbox(ctx, inboxID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("observer did not see Worker completion")
	}
	frames := make([]map[string]any, 0)
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		var frame map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(frames) != 3 || stringValue(frames[0]["type"]) != "action_result" || stringValue(frames[1]["type"]) != "action_result" || stringValue(mapValue(mapValue(frames[1]["payload"])["message"])["text"]) != "诶？真的要看啊。" || stringValue(frames[2]["type"]) != "completed" {
		t.Fatalf("durable observer frames=%#v", frames)
	}
}

func TestExplicitCancelIsRetryableWithoutDuplicateUserMessage(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := durableReplyFixture(t, "cancel")
	payload := map[string]any{"fluctlight_id": fluctlightID, "text": "取消再试", "idempotency_key": "durable-cancel", "turn_id": "durable-cancel-turn", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, ownerID, conversationID, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.CancelTurn(ctx, "unrelated-actor", conversationID, accepted.TurnID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unauthorized cancellation error = %v", err)
	}
	if err := app.CancelTurn(ctx, ownerID, conversationID, accepted.TurnID); err != nil {
		t.Fatal(err)
	}
	var intentStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, "cognition_intent:"+accepted.InboxID).Scan(&intentStatus); err != nil || intentStatus != "cancelled" {
		t.Fatalf("cancelled workflow intent=%q err=%v", intentStatus, err)
	}
	page, err := repository.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].TurnStatus != "cancelled" {
		t.Fatalf("cancelled history=%#v err=%v", page.Messages, err)
	}
	if result, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err != nil || stringValue(result["status"]) != "failed" {
		t.Fatalf("cancelled worker result=%#v err=%v", result, err)
	}
	reopened, err := app.AcceptTurn(ctx, ownerID, conversationID, payload)
	if err != nil || reopened.InboxID != accepted.InboxID || stringValue(reopened.UserMessage["id"]) != stringValue(accepted.UserMessage["id"]) {
		t.Fatalf("reopened=%#v err=%v", reopened, err)
	}
	var retryWorkflowID string
	if err := repository.Pool().QueryRow(ctx, `SELECT status,workflow_id FROM public.platform_workflow_intents WHERE intent_id=$1`, "cognition_intent:"+accepted.InboxID).Scan(&intentStatus, &retryWorkflowID); err != nil || intentStatus != "pending" || retryWorkflowID == "cognition:"+accepted.InboxID {
		t.Fatalf("retried intent=%q/%q err=%v", intentStatus, retryWorkflowID, err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err != nil {
		t.Fatal(err)
	}
	page, err = repository.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil || len(page.Messages) != 2 || page.Messages[0].TurnStatus != "completed" {
		t.Fatalf("retried history=%#v err=%v", page.Messages, err)
	}
	var userCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='user'`, conversationID).Scan(&userCount); err != nil || userCount != 1 {
		t.Fatalf("user messages=%d err=%v", userCount, err)
	}
}

func TestWakeUpSupervisorRepairsStrandedSupersededClock(t *testing.T) {
	ctx, repository, app, _, fluctlightID, _ := durableReplyFixture(t, "wakeup-repair")
	intentID := "wake_up_intent:" + fluctlightID
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='superseded',next_attempt_at=NULL,last_error='superseded_by_cognition' WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	var due time.Time
	if err := repository.Pool().QueryRow(ctx, `SELECT status,next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &due); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || time.Until(due) < 9*time.Minute || time.Until(due) > 11*time.Minute {
		t.Fatalf("repaired WakeUp status=%q due=%s", status, due)
	}
}

func TestAcceptedUserMessageOwnsWakeUpIdleEpochAndAbsolutePhases(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := durableReplyFixture(t, "idle-phases")
	accepted, err := app.AcceptTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "我回来了", "idempotency_key": "idle-phases-first", "turn_id": "idle-phases-turn", "attachment_refs": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	intentID := "wake_up_intent:" + fluctlightID
	var status string
	var payload []byte
	var due time.Time
	readClock := func() map[string]any {
		t.Helper()
		if err := repository.Pool().QueryRow(ctx, `SELECT status,payload,next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &payload, &due); err != nil {
			t.Fatal(err)
		}
		return decodeObject(payload)
	}
	clock := readClock()
	t0, err := time.Parse(time.RFC3339Nano, stringValue(accepted.UserMessage["created_at"]))
	if err != nil {
		t.Fatal(err)
	}
	displayTime := t0
	if err := repository.Pool().QueryRow(ctx, `SELECT created_at FROM public.conversation_messages WHERE id=$1`, stringValue(accepted.UserMessage["id"])).Scan(&t0); err != nil {
		t.Fatal(err)
	}
	if !displayTime.Equal(t0.Truncate(time.Millisecond)) {
		t.Fatal("public timestamp violated millisecond contract")
	}
	if clock["idle_epoch"] != accepted.UserMessage["id"] || clock["idle_phase"] != "first_10m" || !due.Equal(t0.Add(10*time.Minute)) {
		t.Fatalf("accepted user message did not set first idle phase: status=%q payload=%s due=%s t0=%s", status, payload, due, t0)
	}
	if err := app.scheduleWakeUpAfterCognition(ctx, fluctlightID); err != nil {
		t.Fatal(err)
	}
	clock = readClock()
	if status != "completed" || !due.Equal(t0.Add(10*time.Minute)) {
		t.Fatalf("cognition completion shifted last-user idle clock: status=%q payload=%s due=%s", status, payload, due)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='running',payload=jsonb_set(payload,'{cycle}','1'::jsonb) WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := updateWakeUpNextDueTx(ctx, tx, fluctlightID, 1, 1800, t0.Add(10*time.Minute))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	clock = readClock()
	if clock["idle_phase"] != "recurring_10m" || !due.Equal(t0.Add(20*time.Minute)) {
		t.Fatalf("first Wake-up completion did not schedule absolute t+30: %s due=%s", payload, due)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := updateWakeUpNextDueTx(ctx, tx, fluctlightID, 1, 1800, t0.Add(11*time.Minute))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if replay := readClock(); replay["idle_phase"] != "recurring_10m" || !due.Equal(t0.Add(20*time.Minute)) {
		t.Fatalf("duplicate Wake-up completion advanced phase twice: %s due=%s", payload, due)
	}
}

func TestNewUserIdleEpochFencesOldWakeUpFinalAndToolTransactions(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := durableReplyFixture(t, "idle-fence")
	accepted, err := app.AcceptTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "现在聊天", "idempotency_key": "idle-fence-message", "turn_id": "idle-fence-turn", "attachment_refs": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	intentID := "wake_up_intent:" + fluctlightID
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='running',payload=jsonb_set(payload,'{cycle}','1'::jsonb) WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	staleCtx := WithWakeUpCycle(WithWakeUpIdleEpoch(WithLifecycleIntentID(ctx, intentID), "prior-message"), 1)
	beforeModel, err := app.ProcessWakeUp(staleCtx, fluctlightID, 1)
	if err != nil || beforeModel["status"] != "cancelled" {
		t.Fatalf("stale Wake-up started Provider work: %#v %v", beforeModel, err)
	}
	wakeID := "wake_up_" + stableDigest(fluctlightID+":"+jsonString(1))
	settlement, err := app.persistCommittedWakeUp(staleCtx, wakeID, fluctlightID, 1, 1800, ContextProjection{}, map[string]any{"action_type": "no_op"}, agentCommittedOutcome{}, conversationID, "no_op", "no_action_selected")
	if err != nil || settlement["status"] != "cancelled" {
		t.Fatalf("stale Wake-up final settlement was not cancelled: %#v %v", settlement, err)
	}
	called := false
	_, _, _, err = app.executeToolMutation(staleCtx, ToolExecutionRequest{FluctlightID: fluctlightID, Surface: CapabilitySurfaceWakeUp, CapabilityName: "moment.publish", OperationID: "stale-wake-tool"}, func(pgx.Tx) (CapabilityResult, error) {
		called = true
		return CapabilityResult{Status: "completed"}, nil
	})
	if err == nil || called {
		t.Fatalf("stale Wake-up Tool crossed locked execution fence: called=%v err=%v", called, err)
	}
	var wakeFacts int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_wakeups WHERE id=$1`, wakeID).Scan(&wakeFacts); err != nil || wakeFacts != 0 {
		t.Fatalf("stale Wake-up persisted a fact: count=%d err=%v", wakeFacts, err)
	}
	if accepted.UserMessage["id"] == "" {
		t.Fatal("accepted user message identity missing")
	}
}

func TestCommittedReplyDegradationRearmsWakeUp(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := durableReplyFixture(t, "degraded-followup")
	payload := map[string]any{"fluctlight_id": fluctlightID, "text": "提交后结算失败", "idempotency_key": "durable-degraded", "turn_id": "durable-degraded-turn", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, ownerID, conversationID, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=3 WHERE conversation_id=$1`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,turn_id,source_fact_id,correlation_id) VALUES($1,$2,2,$3,'assistant','已提交的回复','[]',$4,$5,$6,$7)`, "degraded-assistant", conversationID, fluctlightID, "assistant:"+accepted.TurnID, accepted.TurnID, accepted.InboxID, accepted.CorrelationID); err != nil {
		t.Fatal(err)
	}
	if err := app.failAgentTurnAfterRun(ctx, accepted.InboxID, agentCommittedOutcome{}, "agent_final_contract_invalid", errors.New("structured output missing required field")); err != nil {
		t.Fatal(err)
	}
	var termination []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload FROM public.diagnostic_events WHERE event_type='agent.run.termination' AND correlation_id=$1 ORDER BY created_at DESC LIMIT 1`, accepted.CorrelationID).Scan(&termination); err != nil {
		t.Fatal(err)
	}
	if payload := decodeObject(termination); payload["safe_cause"] != "structured output missing required field" || payload["stage"] != "final_contract" {
		t.Fatalf("Agent termination cause missing: %#v", payload)
	}
	var wakeUpStatus string
	var nextWakeUp time.Time
	if err := repository.Pool().QueryRow(ctx, `SELECT status,next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+fluctlightID).Scan(&wakeUpStatus, &nextWakeUp); err != nil || wakeUpStatus != "completed" || time.Until(nextWakeUp) < 9*time.Minute || time.Until(nextWakeUp) > 11*time.Minute {
		t.Fatalf("degraded reply WakeUp=%q due=%s err=%v", wakeUpStatus, nextWakeUp, err)
	}
	page, err := repository.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil || len(page.Messages) != 2 || page.Messages[0].TurnStatus != "completed" {
		t.Fatalf("degraded reply history=%#v err=%v", page.Messages, err)
	}
}

func TestProviderFailureExposesRetryableTurnAndReusesUserMessage(t *testing.T) {
	failing := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return embeddingHTTPResponse(request, http.StatusServiceUnavailable, `{"error":"temporary"}`), nil
	})
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupMixedMediaReplyTurnWithTransport(t, "provider-failure", failing)
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"fluctlight_id": fluctlightID, "text": "失败后重试", "idempotency_key": "durable-provider-failure", "turn_id": "durable-provider-failure-turn", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, ownerID, conversationID, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err == nil {
		t.Fatal("Provider failure was accepted as success")
	}
	page, err := repository.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].TurnStatus != "failed" || page.Messages[0].TurnErrorCode == "" {
		t.Fatalf("failed history=%#v err=%v", page.Messages, err)
	}
	reply := []map[string]any{{"id": "call_retry_reply", "type": "function", "function": map[string]any{"name": "conversation.reply", "arguments": jsonString(map[string]any{"text": "重试成功"})}}}
	app.Provider.HTTP = &http.Client{Transport: newConversationToolLoopTransport(fakeProviderResult{ToolCalls: reply})}
	reopened, err := app.AcceptTurn(ctx, ownerID, conversationID, payload)
	if err != nil || reopened.InboxID != accepted.InboxID {
		t.Fatalf("retry acceptance=%#v err=%v", reopened, err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err != nil {
		t.Fatal(err)
	}
	page, err = repository.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil || len(page.Messages) != 2 || page.Messages[0].TurnStatus != "completed" || page.Messages[1].Text != "重试成功" {
		t.Fatalf("retry history=%#v err=%v", page.Messages, err)
	}
}

func TestWorkerPreservesOwnerAuthorizationForActorSender(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := durableReplyFixture(t, "actor-sender")
	senderID := "mixed-sender-actor-sender"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'fluctlight','active')`, senderID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, senderID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'member','active')`, conversationID, senderID); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"fluctlight_id": fluctlightID, "sender_actor_id": senderID, "text": "由另一位 Actor 发出", "idempotency_key": "durable-actor-sender", "turn_id": "durable-actor-sender-turn", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, ownerID, conversationID, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err != nil {
		t.Fatal(err)
	}
	page, err := repository.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil || len(page.Messages) != 2 || page.Messages[0].AuthorActorID != senderID || page.Messages[0].TurnStatus != "completed" {
		t.Fatalf("actor sender history=%#v err=%v", page.Messages, err)
	}
}

func TestExplicitCancelStopsRunningProviderAndFencesLateReply(t *testing.T) {
	started := make(chan struct{})
	var signalOnce sync.Once
	blocking := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		signalOnce.Do(func() { close(started) })
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupMixedMediaReplyTurnWithTransport(t, "running-cancel", blocking)
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer redisServer.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer redisClient.Close()
	app.Redis = redisClient
	app.Provider.redis = redisClient
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"fluctlight_id": fluctlightID, "text": "停止正在运行的模型", "idempotency_key": "durable-running-cancel", "turn_id": "durable-running-cancel-turn", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, ownerID, conversationID, payload)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, processErr := app.ProcessCognitionInbox(ctx, accepted.InboxID); done <- processErr }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("Provider did not start")
	}
	if err := app.CancelTurn(ctx, ownerID, conversationID, accepted.TurnID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AcceptTurn(ctx, ownerID, conversationID, payload); !errors.Is(err, ErrConflict) {
		t.Fatalf("retry before cancellation settlement error = %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Provider did not stop after explicit cancellation")
	}
	page, err := repository.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].TurnStatus != "cancelled" || page.Messages[0].TurnRetryable {
		t.Fatalf("history after running cancellation=%#v err=%v", page.Messages, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='cancelled',completed_at=now() WHERE intent_id=$1 AND status='cancel_requested'`, "cognition_intent:"+accepted.InboxID); err != nil {
		t.Fatal(err)
	}
	page, err = repository.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil || !page.Messages[0].TurnRetryable {
		t.Fatalf("retry readiness after terminal cancellation=%#v err=%v", page.Messages, err)
	}
}
