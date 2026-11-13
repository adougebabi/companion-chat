package core

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

// Repeated physical responses may reuse a native call ID. The original denial
// stays a single authoritative receipt even when its delivery is repeated.
func TestM1ReplayedUnauthorizedCallIDStopsWithoutDuplicateReceipt(t *testing.T) {
	calls, denialFeedback := 0, 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		feedbackMessages := 0
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] != "tool" || message["tool_call_id"] != "same-denied-call" {
				continue
			}
			result := decodeObject([]byte(stringValue(message["content"])))
			if result["status"] == "handled" && mapValue(result["output"])["warning_code"] == "capability_not_authorized" {
				feedbackMessages++
			}
		}
		if calls > 1 && feedbackMessages >= calls-1 {
			denialFeedback++
		}
		tools := arrayValue(payload["tools"])
		if calls >= 4 && len(tools) == 1 && capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(tools[0])["function"])["name"]), conversationReplyCapabilityName) {
			decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("reply-after-replay", conversationReplyCapabilityName, map[string]any{"text": "我不能执行这个操作。", "decision": decision})}}
		}
		if calls > 5 {
			return fakeProviderResult{Status: 400}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("same-denied-call", "goal.decide", map[string]any{"reason": "固定参数"})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m1-replayed-denial", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	const turnID = "m1-replayed-denial"
	_, runErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": turnID, "idempotency_key": turnID, "attachment_refs": []any{}})
	var messages, unauthorizedExecutions int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='我不能执行这个操作。'), (SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='goal.decide')`, conversation, fid).Scan(&messages, &unauthorizedExecutions); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || calls != 4 || denialFeedback != 3 || messages != 1 || unauthorizedExecutions != 0 {
		t.Fatalf("replayed denial was not bounded: model_calls=%d feedback=%d messages=%d unauthorized_executions=%d err=%v", calls, denialFeedback, messages, unauthorizedExecutions, runErr)
	}
	deniedReceipts := 0
	for _, result := range nativePersonaTrace(t, ctx, repo, turnID) {
		if result["capability_name"] == "goal.decide" && result["status"] == "handled" && mapValue(result["output"])["warning_code"] == "capability_not_authorized" {
			deniedReceipts++
		}
	}
	if deniedReceipts != 1 {
		t.Fatalf("replay changed the authoritative receipt count: %d", deniedReceipts)
	}
}

func TestM1ReplayedUnauthorizedCallIDIgnoringReplyWindowStops(t *testing.T) {
	calls, replyOnlyRequests := 0, 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		tools := arrayValue(payload["tools"])
		if len(tools) == 1 && capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(tools[0])["function"])["name"]), conversationReplyCapabilityName) {
			replyOnlyRequests++
		}
		if calls > 7 {
			return fakeProviderResult{Status: 400}
		}
		// Deliberately ignore the reply-only tool selection on every response.
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("ignored-denied-call", "goal.decide", map[string]any{"reason": "固定参数"})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m1-replay-ignores-window", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	_, runErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "m1-replay-ignores-window", "idempotency_key": "m1-replay-ignores-window", "attachment_refs": []any{}})
	var messages, unauthorizedExecutions int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'), (SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='goal.decide')`, conversation, fid).Scan(&messages, &unauthorizedExecutions); err != nil {
		t.Fatal(err)
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "tool_progress_stalled") || calls != 5 || replyOnlyRequests != 2 || messages != 0 || unauthorizedExecutions != 0 {
		t.Fatalf("ignored reply-only window was not bounded: model_calls=%d reply_only=%d messages=%d unauthorized_executions=%d err=%v", calls, replyOnlyRequests, messages, unauthorizedExecutions, runErr)
	}
}

func TestM1ReplayedClosedWindowReceiptStops(t *testing.T) {
	calls, replyOnlyRequests, closedFeedback := 0, 0, 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		tools := arrayValue(payload["tools"])
		if len(tools) == 1 && capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(tools[0])["function"])["name"]), conversationReplyCapabilityName) {
			replyOnlyRequests++
		}
		if result := m1ToolResult(payload, "closed-window-call"); result["status"] == "handled" && mapValue(result["output"])["warning_code"] == "turn_action_window_closed" {
			closedFeedback++
		}
		if calls > 7 {
			return fakeProviderResult{Status: 400}
		}
		id := "first-denied-call"
		if calls >= 4 {
			id = "closed-window-call"
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(id, "goal.decide", map[string]any{"reason": "固定参数"})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m1-replayed-closed-window", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	_, runErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "m1-replayed-closed-window", "idempotency_key": "m1-replayed-closed-window", "attachment_refs": []any{}})
	var messages, unauthorizedExecutions int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'), (SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='goal.decide')`, conversation, fid).Scan(&messages, &unauthorizedExecutions); err != nil {
		t.Fatal(err)
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "tool_progress_stalled") || calls != 5 || replyOnlyRequests != 2 || closedFeedback != 1 || messages != 0 || unauthorizedExecutions != 0 {
		t.Fatalf("replayed closed-window receipt was not bounded: model_calls=%d reply_only=%d closed_feedback=%d messages=%d unauthorized=%d err=%v", calls, replyOnlyRequests, closedFeedback, messages, unauthorizedExecutions, runErr)
	}
}

func TestM1ClosedWindowSameBatchCountsAsOneResponse(t *testing.T) {
	calls := 0
	twoClosedReceipts := false
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		switch calls {
		case 1, 2, 3:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("new-denial-%d", calls), "goal.decide", map[string]any{"reason": fmt.Sprintf("参数 %d", calls)})}}
		case 4:
			return fakeProviderResult{ToolCalls: []map[string]any{
				nativePersonaToolCall("closed-a", "goal.decide", map[string]any{"reason": "甲"}),
				nativePersonaToolCall("closed-b", "goal.decide", map[string]any{"reason": "乙"}),
			}}
		case 5:
			a, b := m1ToolResult(payload, "closed-a"), m1ToolResult(payload, "closed-b")
			twoClosedReceipts = a["status"] == "handled" && b["status"] == "handled" && mapValue(a["output"])["warning_code"] == "turn_action_window_closed" && mapValue(b["output"])["warning_code"] == "turn_action_window_closed"
			decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("reply-after-closed-batch", conversationReplyCapabilityName, map[string]any{"text": "我不能执行这个操作。", "decision": decision})}}
		default:
			return fakeProviderResult{Status: 400}
		}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m1-closed-window-batch", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	_, runErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "m1-closed-window-batch", "idempotency_key": "m1-closed-window-batch", "attachment_refs": []any{}})
	var messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='我不能执行这个操作。'`, conversation).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || calls != 5 || !twoClosedReceipts || messages != 1 {
		t.Fatalf("one response with two closed calls prematurely stopped: model_calls=%d closed_receipts=%t messages=%d err=%v", calls, twoClosedReceipts, messages, runErr)
	}
}

func TestM1ReplayedDenialRestartsAfterRealAction(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	refresh := &runtimeContextRefresh{}
	invoker := &appADKCapabilityInvoker{trace: trace, scopeRegistry: mustCapabilityRegistry(builtinCapabilities(nil)...)}
	ctx := withADKCapabilityContext(context.Background(), invoker, trace, refresh)
	note := func(sequence uint64, id, name string) {
		calls := []schema.ToolCall{{ID: id, Function: schema.FunctionCall{Name: name}}}
		trace.RecordModelToolCalls("provider", sequence, calls)
		refresh.noteModelToolCalls(sequence, calls)
	}
	note(1, "same-denied", "goal.decide")
	trace.Append(CapabilityInvocation{CallID: "same-denied", CapabilityName: "goal.decide"}, CapabilityResult{CallID: "same-denied", CapabilityName: "goal.decide", Status: "handled", Output: map[string]any{"effect_applied": false, "warning_code": "capability_not_authorized"}})
	note(2, "same-denied", "goal.decide")
	note(3, "same-denied", "goal.decide")
	if !conversationLoopNeedsReply(ctx) {
		t.Fatal("three physical denial responses were not counted")
	}
	note(4, "real-action", lifeActivityStartCapabilityName)
	trace.Append(CapabilityInvocation{CallID: "real-action", CapabilityName: lifeActivityStartCapabilityName}, CapabilityResult{CallID: "real-action", CapabilityName: lifeActivityStartCapabilityName, Status: "accepted"})
	note(5, "same-denied", "goal.decide")
	note(6, "same-denied", "goal.decide")
	if conversationLoopNeedsReply(ctx) {
		t.Fatal("the accepted action did not reset earlier denial rounds")
	}
	note(7, "same-denied", "goal.decide")
	if conversationLoopNeedsReplyBeforeModelSequence(ctx, 7) || !conversationLoopNeedsReply(ctx) {
		t.Fatal("current same-batch denial was treated as already delivered feedback")
	}
}

func TestM1ClosedWindowRoundsResetAfterRealProgress(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	refresh := &runtimeContextRefresh{}
	note := func(sequence uint64, calls ...schema.ToolCall) {
		trace.RecordModelToolCalls("provider", sequence, calls)
		refresh.noteModelToolCalls(sequence, calls)
	}
	a := schema.ToolCall{ID: "closed-a", Function: schema.FunctionCall{Name: "goal.decide"}}
	b := schema.ToolCall{ID: "closed-b", Function: schema.FunctionCall{Name: "goal.decide"}}
	note(4, a, b)
	for _, id := range []string{"closed-a", "closed-b"} {
		trace.Append(CapabilityInvocation{CallID: id, CapabilityName: "goal.decide"}, CapabilityResult{CallID: id, CapabilityName: "goal.decide", Status: "handled", Output: map[string]any{"warning_code": "turn_action_window_closed"}})
	}
	if got := conversationClosedActionAttempts(trace, refresh); got != 1 {
		t.Fatalf("same response counted as %d rounds", got)
	}
	note(5, a)
	if got := conversationClosedActionAttempts(trace, refresh); got != 2 {
		t.Fatalf("replayed receipt counted as %d rounds", got)
	}
	query := schema.ToolCall{ID: "real-query", Function: schema.FunctionCall{Name: "memory.recall"}}
	note(6, query)
	trace.Append(CapabilityInvocation{CallID: "real-query", CapabilityName: "memory.recall"}, CapabilityResult{CallID: "real-query", CapabilityName: "memory.recall", Status: "completed"})
	if got := conversationClosedActionAttempts(trace, refresh); got != 0 {
		t.Fatalf("real progress did not reset the prior %d rounds", got)
	}
	note(7, a)
	if got := conversationClosedActionAttempts(trace, refresh); got != 1 {
		t.Fatalf("first replay after progress counted as %d rounds", got)
	}
	note(8, a)
	if got := conversationClosedActionAttempts(trace, refresh); got != 2 {
		t.Fatalf("second replay after progress counted as %d rounds", got)
	}
}

func TestM1ReplayedDenialResetsAfterSameBatchRealAction(t *testing.T) {
	calls := 0
	actionCompleted, replyOnlyAfterNewDenials := false, false
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		denial := nativePersonaToolCall("same-denial-around-action", "goal.decide", map[string]any{"reason": "固定参数"})
		switch calls {
		case 1, 2:
			return fakeProviderResult{ToolCalls: []map[string]any{denial}}
		case 3:
			memory := nativePersonaToolCall("real-memory", "memory_event", map[string]any{"type": "semantic", "content": "用户喜欢喝茶", "confidence": 0.9, "importance": 0.8})
			return fakeProviderResult{ToolCalls: []map[string]any{denial, memory}}
		case 4:
			actionCompleted = m1ToolResult(payload, "real-memory")["status"] == "completed"
			return fakeProviderResult{ToolCalls: []map[string]any{denial}}
		case 5, 6:
			return fakeProviderResult{ToolCalls: []map[string]any{denial}}
		case 7:
			tools := arrayValue(payload["tools"])
			replyOnlyAfterNewDenials = len(tools) == 1 && capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(tools[0])["function"])["name"]), conversationReplyCapabilityName)
			decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("reply-after-real-action", conversationReplyCapabilityName, map[string]any{"text": "我记住了。", "decision": decision})}}
		default:
			return fakeProviderResult{Status: 400}
		}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m1-replay-real-action", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	_, runErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "记住我喜欢喝茶。", "turn_id": "m1-replay-real-action", "idempotency_key": "m1-replay-real-action", "attachment_refs": []any{}})
	var messages, realActions, unauthorizedExecutions int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='我记住了。'), (SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='memory_event' AND result->>'status'='completed'), (SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='goal.decide')`, conversation, fid).Scan(&messages, &realActions, &unauthorizedExecutions); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || calls != 7 || !actionCompleted || !replyOnlyAfterNewDenials || messages != 1 || realActions != 1 || unauthorizedExecutions != 0 {
		t.Fatalf("same-batch real action did not reset denial rounds: model_calls=%d action_completed=%t reply_only=%t messages=%d actions=%d unauthorized=%d err=%v", calls, actionCompleted, replyOnlyAfterNewDenials, messages, realActions, unauthorizedExecutions, runErr)
	}
}

func TestM1RepeatedQueryBatchKeepsReplyOpportunity(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	refresh := &runtimeContextRefresh{}
	invoker := &appADKCapabilityInvoker{trace: trace, scopeRegistry: mustCapabilityRegistry(builtinCapabilities(nil)...)}
	ctx := withADKCapabilityContext(context.Background(), invoker, trace, refresh)
	var calls []schema.ToolCall
	for index := 0; index < 5; index++ {
		id := fmt.Sprintf("query-%d", index)
		calls = append(calls, schema.ToolCall{ID: id, Function: schema.FunctionCall{Name: "moment.inspect"}})
		trace.Append(CapabilityInvocation{CallID: id, CapabilityName: "moment.inspect"}, CapabilityResult{CallID: id, CapabilityName: "moment.inspect", Status: "completed", Output: map[string]any{"items": []any{}}})
	}
	trace.RecordModelToolCalls("provider", 1, calls)
	refresh.noteModelToolCalls(1, calls)
	choice, err := (conversationPublicationPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx)
	if err != nil || len(choice.AllowedToolNames) != 1 || choice.AllowedToolNames[0] != conversationReplyCapabilityName {
		t.Fatalf("first query batch lost its reply opportunity: choice=%+v err=%v", choice, err)
	}
}
