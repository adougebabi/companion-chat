package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDirectConversationMessageIsDurableDuringCognitionAndNoReplyCannotComplete(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "delivery-owner", "delivery-fluctlight", "delivery-conversation"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'delivery regression')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	endpointID := "delivery-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://delivery.invalid','delivery-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'delivery-model','structured_output,tool_calling',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	providerReceived := make(chan struct{})
	releaseProvider := make(chan struct{})
	app := &App{DB: repository}
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(request.Body)
		close(providerReceived)
		select {
		case <-releaseProvider:
		case <-request.Context().Done():
			return nil, request.Context().Err()
		}
		structured := map[string]any{
			"action_type": "reply", "response_intent": "acknowledge the direct message", "visible_text": "",
			"tool_calls": []any{}, "influences": []any{},
			"appraisal": map[string]any{
				"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.5, "social_threat": 0.0,
				"controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.5, "expected_effect": 0.5,
				"evidence_refs": []any{}, "event_kind": "conversation", "direction": "mixed", "drive_signals": []any{},
			},
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(structured), "tool_calls": []any{}}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}
	app.Provider = &ProviderClient{DB: repository, HTTP: providerHTTP}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	type turnOutcome struct {
		result TurnResult
		err    error
	}
	done := make(chan turnOutcome, 1)
	go func() {
		result, err := app.HandleTurn(ctx, ownerID, conversationID, map[string]any{
			"fluctlight_id": fluctlightID, "text": "这条消息必须立即可见", "idempotency_key": "delivery-turn", "turn_id": "delivery-turn-1", "attachment_refs": []any{},
		})
		done <- turnOutcome{result: result, err: err}
	}()
	select {
	case <-providerReceived:
	case <-time.After(5 * time.Second):
		close(releaseProvider)
		t.Fatal("Provider did not receive cognition request")
	}
	var durableUserCount, workflowIntentCount, outboxCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages m JOIN public.cognition_inbox i ON i.id=m.source_fact_id WHERE m.conversation_id=$1 AND m.idempotency_key=$2 AND m.kind='user' AND m.turn_id='delivery-turn-1' AND m.correlation_id='turn:delivery-turn-1' AND i.payload->>'turn_id'=m.turn_id`, conversationID, "delivery-turn").Scan(&durableUserCount); err != nil {
		close(releaseProvider)
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id IN (SELECT 'cognition_intent:' || source_fact_id FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2)`, conversationID, "delivery-turn").Scan(&workflowIntentCount); err != nil {
		close(releaseProvider)
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE idempotency_key='cognition:delivery-turn'`).Scan(&outboxCount); err != nil {
		close(releaseProvider)
		t.Fatal(err)
	}
	if durableUserCount != 1 || workflowIntentCount != 1 || outboxCount != 1 {
		close(releaseProvider)
		t.Fatalf("Provider received cognition before atomic source commit: user=%d workflow=%d outbox=%d", durableUserCount, workflowIntentCount, outboxCount)
	}
	close(releaseProvider)
	var outcome turnOutcome
	select {
	case outcome = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("completed cognition did not settle the direct turn")
	}
	var assistantCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID).Scan(&assistantCount); err != nil {
		t.Fatal(err)
	}
	var inboxStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key='delivery-turn'`, fluctlightID).Scan(&inboxStatus); err != nil {
		t.Fatal(err)
	}
	var completedFrozenCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_frozen_actions WHERE fluctlight_id=$1 AND status='completed'`, fluctlightID).Scan(&completedFrozenCount); err != nil {
		t.Fatal(err)
	}
	if outcome.err == nil || !strings.Contains(outcome.err.Error(), "cognition_visible_text_missing") || assistantCount != 0 || inboxStatus != "pending" || completedFrozenCount != 0 {
		t.Fatalf("no-reply cognition was silently completed: result=%#v err=%v assistant=%d inbox=%s completed_frozen=%d", outcome.result, outcome.err, assistantCount, inboxStatus, completedFrozenCount)
	}
}

func TestDirectConversationStreamsCommittedUserBeforeProviderAndAssistantAfterCommit(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "stream-owner", "stream-fluctlight", "stream-conversation"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'stream regression')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	endpointID := "stream-delivery-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://stream-delivery.invalid','stream-delivery-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'stream-delivery-model','structured_output,tool_calling',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	providerReceived := make(chan struct{})
	releaseProvider := make(chan struct{})
	app := &App{DB: repository}
	app.Provider = &ProviderClient{DB: repository, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(request.Body)
		close(providerReceived)
		select {
		case <-releaseProvider:
		case <-request.Context().Done():
			return nil, request.Context().Err()
		}
		structured := map[string]any{
			"action_type": "reply", "response_intent": "acknowledge", "visible_text": "我收到了。",
			"tool_calls": []any{}, "influences": []any{},
			"appraisal": map[string]any{
				"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.5, "social_threat": 0.0,
				"controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.5, "expected_effect": 0.5,
				"evidence_refs": []any{}, "event_kind": "conversation", "direction": "mixed", "drive_signals": []any{},
			},
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(structured), "tool_calls": []any{}}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	var eventLock sync.Mutex
	events := make([]string, 0, 3)
	done := make(chan error, 1)
	go func() {
		_, err := app.handleTurn(ctx, ownerID, conversationID, map[string]any{
			"fluctlight_id": fluctlightID, "text": "请确认收到", "idempotency_key": "stream-delivery-turn", "turn_id": "stream-delivery-turn-1", "attachment_refs": []any{},
		}, turnCallbacks{
			onActionResult: func(payload map[string]any) error {
				message := mapValue(payload["message"])
				if stringValue(message["created_at"]) == "" {
					return fmt.Errorf("authoritative %s frame omitted created_at", stringValue(message["kind"]))
				}
				var committed int
				if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE id=$1`, stringValue(message["id"])).Scan(&committed); err != nil || committed != 1 {
					return fmt.Errorf("authoritative %s frame preceded commit: count=%d err=%v", stringValue(message["kind"]), committed, err)
				}
				eventLock.Lock()
				defer eventLock.Unlock()
				events = append(events, stringValue(message["kind"]))
				return nil
			},
			onChunk: func(text string) error {
				eventLock.Lock()
				defer eventLock.Unlock()
				events = append(events, "token:"+text)
				return nil
			},
		}, true)
		done <- err
	}()
	select {
	case <-providerReceived:
	case <-time.After(5 * time.Second):
		close(releaseProvider)
		t.Fatal("Provider did not receive stream cognition request")
	}
	eventLock.Lock()
	duringProvider := append([]string(nil), events...)
	eventLock.Unlock()
	if fmt.Sprint(duringProvider) != "[user]" {
		close(releaseProvider)
		t.Fatalf("frames before Provider completion=%v, want committed user frame", duringProvider)
	}
	close(releaseProvider)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	eventLock.Lock()
	finalEvents := append([]string(nil), events...)
	eventLock.Unlock()
	if fmt.Sprint(finalEvents) != "[user token:我收到了。 assistant]" {
		t.Fatalf("stream frame order=%v", finalEvents)
	}
	var linkedMessages, sourceFacts int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*),count(DISTINCT source_fact_id) FROM public.conversation_messages WHERE conversation_id=$1 AND turn_id='stream-delivery-turn-1' AND source_fact_id IS NOT NULL AND correlation_id='turn:stream-delivery-turn-1'`, conversationID).Scan(&linkedMessages, &sourceFacts); err != nil {
		t.Fatal(err)
	}
	if linkedMessages != 2 || sourceFacts != 1 {
		t.Fatalf("turn source links messages=%d facts=%d", linkedMessages, sourceFacts)
	}
}

func TestDirectConversationReplyToolWithoutAppraisalCommitsBothMessages(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "reply-tool-owner", "reply-tool-fluctlight", "reply-tool-conversation"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'reply tool regression')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	endpointID := "reply-tool-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://reply-tool.invalid','reply-tool-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'reply-tool-model','structured_output,tool_calling',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	app.Provider = &ProviderClient{DB: repository, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(request.Body)
		message := map[string]any{
			"content": "", "reasoning_content": "internal reasoning must not become visible",
			"tool_calls": []any{map[string]any{
				"id": "reply-tool-call", "type": "function",
				"function": map[string]any{"name": "conversation.reply", "arguments": jsonString(map[string]any{"text": "我收到你的消息了。"})},
			}},
		}
		response := map[string]any{"choices": []any{map[string]any{"message": message}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	events := make([]string, 0, 3)
	_, err = app.handleTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "在吗？", "idempotency_key": "reply-tool-turn", "turn_id": "reply-tool-turn-1", "attachment_refs": []any{},
	}, turnCallbacks{
		onActionResult: func(payload map[string]any) error {
			events = append(events, stringValue(mapValue(payload["message"])["kind"]))
			return nil
		},
		onChunk: func(text string) error {
			events = append(events, "token:"+text)
			return nil
		},
	}, true)
	var userCount, assistantCount, stateRevisionCount int
	if queryErr := repository.Pool().QueryRow(ctx, `SELECT count(*) FILTER (WHERE kind='user'),count(*) FILTER (WHERE kind='assistant') FROM public.conversation_messages WHERE conversation_id=$1`, conversationID).Scan(&userCount, &assistantCount); queryErr != nil {
		t.Fatal(queryErr)
	}
	if queryErr := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_state_revisions WHERE fluctlight_id=$1 AND source_event_id=(SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key='reply-tool-turn')`, fluctlightID).Scan(&stateRevisionCount); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || userCount != 1 || assistantCount != 1 || stateRevisionCount != 0 || fmt.Sprint(events) != "[user token:我收到你的消息了。 assistant]" {
		t.Fatalf("reply-tool turn err=%v user=%d assistant=%d state_revisions=%d events=%v", err, userCount, assistantCount, stateRevisionCount, events)
	}
}

func TestSupersededConversationTurnCannotCommitLateAssistantOrReviveInbox(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "supersede-owner", "supersede-fluctlight", "supersede-conversation"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'supersede regression')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,2)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	inboxID, frozenID := "superseded-inbox", "superseded-action"
	inboxPayload := map[string]any{"actor_id": ownerID, "conversation_id": conversationID, "turn_id": "superseded-turn", "text": "旧消息"}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,2,0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,error_code,processed_at) VALUES($2,$1,1,'conversation.turn',$3,$2,$2,$2,now(),'failed','superseded_by_newer_turn',now())`, fluctlightID, inboxID, jsonBytes(inboxPayload)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_frozen_actions(id,decision_id,inbox_id,fluctlight_id,action_type,payload,state_revision,provider_request_id,status) VALUES($3,'superseded-decision',$2,$1,'reply',$4,0,'superseded-provider','frozen')`, fluctlightID, inboxID, frozenID, jsonBytes(map[string]any{"decision": map[string]any{"cognitive_state_transition": "not_proposed"}, "capability_results": []CapabilityResult{}})); err != nil {
		t.Fatal(err)
	}
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('late-assistant',$1,2,$2,'assistant','迟到回复','[]','assistant:superseded-turn')`, conversationID, fluctlightID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	_, settleErr := (&App{DB: repository}).completeTurnCognitionTx(ctx, tx, inboxID, frozenID, map[string]any{"text": "迟到回复", "capability_results": []CapabilityResult{}}, time.Now().UTC())
	if settleErr == nil || !errors.Is(settleErr, errCognitionTurnSuperseded) {
		_ = tx.Rollback(ctx)
		t.Fatalf("superseded settlement err=%v", settleErr)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var assistantCount int
	var inboxStatus, inboxError, frozenStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE id='late-assistant'`).Scan(&assistantCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&inboxStatus, &inboxError); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_frozen_actions WHERE id=$1`, frozenID).Scan(&frozenStatus); err != nil {
		t.Fatal(err)
	}
	if assistantCount != 0 || inboxStatus != "failed" || inboxError != "superseded_by_newer_turn" || frozenStatus != "frozen" {
		t.Fatalf("superseded turn revived: assistant=%d inbox=%s/%s frozen=%s", assistantCount, inboxStatus, inboxError, frozenStatus)
	}
}

// Authored in S02 and executed only by the S12 isolated-PostgreSQL gate.
func TestDirectConversationSourceFactConflictRollsBackNewUserMessage(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "atomic-owner", "atomic-fluctlight", "atomic-conversation"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'atomic source')`, []any{conversationID, ownerID}},
		{`INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, []any{conversationID}},
		{`INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, []any{conversationID, ownerID, fluctlightID}},
		{`INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,2,1)`, []any{fluctlightID}},
		{`INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES($1,$2,1,'conversation.turn',$3,'atomic-turn','turn:atomic-turn','atomic-idempotency',now(),'processed',now())`, []any{"inbox_" + stableDigest("turn:atomic-idempotency"), fluctlightID, jsonBytes(map[string]any{"actor_id": ownerID, "fluctlight_id": fluctlightID, "conversation_id": conversationID, "turn_id": "atomic-turn", "text": "different text", "idempotency_key": "atomic-idempotency"})}},
	} {
		if _, err := repository.Pool().Exec(ctx, seed.query, seed.args...); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	_, err := app.HandleTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "new text", "idempotency_key": "atomic-idempotency", "turn_id": "atomic-turn", "attachment_refs": []any{},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("source conflict err=%v", err)
	}
	var messageCount, factCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1`, conversationID).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key='atomic-idempotency'`, fluctlightID).Scan(&factCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 0 || factCount != 1 {
		t.Fatalf("source conflict left message=%d fact=%d", messageCount, factCount)
	}
}
