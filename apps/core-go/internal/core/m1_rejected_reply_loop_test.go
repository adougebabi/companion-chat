package core

import (
	"context"
	"fmt"
	"strings"
	"testing"

	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func m1ToolResult(payload map[string]any, callID string) map[string]any {
	for _, raw := range arrayValue(payload["messages"]) {
		message := mapValue(raw)
		if message["role"] == "tool" && message["tool_call_id"] == callID {
			return decodeObject([]byte(stringValue(message["content"])))
		}
	}
	return nil
}

func m1LoadedMemoryTool(result map[string]any) bool {
	output := mapValue(result["output"])
	if result["status"] != "completed" || len(arrayValue(output["unavailable"])) != 0 {
		return false
	}
	for _, name := range arrayValue(output["loaded"]) {
		if stringValue(name) == "memory.recall" {
			return true
		}
	}
	return false
}

// A capability outside this conversation's surface remains unauthorized even
// if the model keeps changing its arguments and native call ID.
func TestM1RepeatedUnauthorizedToolWithChangedArgumentsStops(t *testing.T) {
	calls, denialFeedback := 0, 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if strings.Contains(nativePersonaToolMessages(payload), "capability_not_authorized") {
			denialFeedback++
		}
		tools := arrayValue(payload["tools"])
		if calls >= 4 && len(tools) == 1 && capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(tools[0])["function"])["name"]), conversationReplyCapabilityName) {
			decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("allowed-reply", conversationReplyCapabilityName, map[string]any{"text": "我不能执行这个操作。", "decision": decision})}}
		}
		if calls > 5 {
			// Bound the pre-fix test; this is not a substitute for a loop guard.
			return fakeProviderResult{Status: 400}
		}
		args := map[string]any{"operation": "pause", "reason": fmt.Sprintf("不同参数 %d", calls)}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("denied-tool-%d", calls), "goal.decide", args)}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m1-unauthorized-tool-loop", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	_, runErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "m1-unauthorized-tool-loop", "idempotency_key": "m1-unauthorized-tool-loop", "attachment_refs": []any{}})
	var messages, unauthorizedExecutions int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='我不能执行这个操作。'), (SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='goal.decide')`, conversation, fid).Scan(&messages, &unauthorizedExecutions); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || calls != 4 || denialFeedback != 3 || messages != 1 || unauthorizedExecutions != 0 {
		t.Fatalf("unauthorized repetition was not bounded: model_calls=%d feedback=%d messages=%d unauthorized_executions=%d err=%v", calls, denialFeedback, messages, unauthorizedExecutions, runErr)
	}
}

func TestM1AuthorizedToolStillAvailableAfterTwoDenials(t *testing.T) {
	calls := 0
	denialsConfirmed, catalogLoaded, recallCompleted := false, false, false
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		switch calls {
		case 1, 2:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("denied-%d", calls), "goal.decide", map[string]any{"reason": fmt.Sprintf("不同参数 %d", calls)})}}
		case 3:
			if len(arrayValue(payload["tools"])) <= 1 {
				t.Error("two denials prematurely hid other authorized tools")
			}
			first, second := m1ToolResult(payload, "denied-1"), m1ToolResult(payload, "denied-2")
			denialsConfirmed = first["status"] == "handled" && second["status"] == "handled" && mapValue(first["output"])["warning_code"] == "capability_not_authorized" && mapValue(second["output"])["warning_code"] == "capability_not_authorized"
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("load-memory", capabilityCatalogName, map[string]any{"names": []any{"memory.recall"}})}}
		case 4:
			catalogLoaded = m1LoadedMemoryTool(m1ToolResult(payload, "load-memory"))
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("recall-memory", "memory.recall", map[string]any{"intent": "最近话题"})}}
		case 5:
			recallCompleted = m1ToolResult(payload, "recall-memory")["status"] == "completed"
			decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("allowed-reply", conversationReplyCapabilityName, map[string]any{"text": "我们继续聊。", "decision": decision})}}
		default:
			return fakeProviderResult{Status: 400}
		}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m1-denial-then-authorized", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	_, runErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "m1-denial-then-authorized", "idempotency_key": "m1-denial-then-authorized", "attachment_refs": []any{}})
	var messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='我们继续聊。'`, conversation).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || calls != 5 || !denialsConfirmed || !catalogLoaded || !recallCompleted || messages != 1 {
		t.Fatalf("authorized continuation failed: model_calls=%d denials=%t catalog_loaded=%t recall_completed=%t messages=%d err=%v", calls, denialsConfirmed, catalogLoaded, recallCompleted, messages, runErr)
	}
}

func TestM1SameBatchDenialsDoNotBlockAuthorizedTool(t *testing.T) {
	calls := 0
	denials, catalogLoaded := 0, false
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			toolCalls := []map[string]any{}
			for n := 1; n <= 3; n++ {
				toolCalls = append(toolCalls, nativePersonaToolCall(fmt.Sprintf("batch-denied-%d", n), "goal.decide", map[string]any{"reason": fmt.Sprintf("不同参数 %d", n)}))
			}
			toolCalls = append(toolCalls, nativePersonaToolCall("batch-catalog", capabilityCatalogName, map[string]any{"names": []any{"memory.recall"}}))
			return fakeProviderResult{ToolCalls: toolCalls}
		}
		if calls == 2 {
			for _, raw := range arrayValue(payload["messages"]) {
				message := mapValue(raw)
				if message["role"] != "tool" {
					continue
				}
				id := stringValue(message["tool_call_id"])
				result := decodeObject([]byte(stringValue(message["content"])))
				output := mapValue(result["output"])
				if strings.HasPrefix(id, "batch-denied-") && result["status"] == "handled" && output["warning_code"] == "capability_not_authorized" {
					denials++
				}
				if id == "batch-catalog" && result["status"] == "completed" && len(arrayValue(output["unavailable"])) == 0 {
					for _, name := range arrayValue(output["loaded"]) {
						catalogLoaded = catalogLoaded || stringValue(name) == "memory.recall"
					}
				}
			}
			decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("batch-reply", conversationReplyCapabilityName, map[string]any{"text": "我们继续聊。", "decision": decision})}}
		}
		return fakeProviderResult{Status: 400}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m1-same-batch-denial", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	_, runErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "m1-same-batch-denial", "idempotency_key": "m1-same-batch-denial", "attachment_refs": []any{}})
	var messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='我们继续聊。'`, conversation).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || calls != 2 || denials != 3 || !catalogLoaded || messages != 1 {
		t.Fatalf("same-batch valid tool was blocked: model_calls=%d denials=%d catalog_loaded=%t messages=%d err=%v", calls, denials, catalogLoaded, messages, runErr)
	}
}

func TestM1UnauthorizedRepetitionKeepsOtherRecoveryPaths(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	invoker := &appADKCapabilityInvoker{trace: trace, scopeRegistry: mustCapabilityRegistry(builtinCapabilities(nil)...)}
	ctx := withADKCapabilityContext(context.Background(), invoker, trace, nil)
	appendHandled := func(name, code string, n int) {
		id := fmt.Sprintf("%s-%d-%d", name, n, len(trace.Results))
		trace.Append(CapabilityInvocation{CallID: id, CapabilityName: name, Arguments: jsonBytes(map[string]any{"value": n})}, CapabilityResult{CallID: id, CapabilityName: name, Status: "handled", Output: map[string]any{"effect_applied": false, "warning_code": code}})
	}
	// Different argument failures can be repaired by changing arguments.
	for n := 0; n < 3; n++ {
		appendHandled("goal.evaluate", "invalid_arguments", n)
	}
	// A denial of another capability must not count as the same attempt.
	appendHandled("goal.decide", "capability_not_authorized", 1)
	appendHandled("habit.decide", "capability_not_authorized", 2)
	if conversationLoopNeedsReply(ctx) {
		t.Fatal("distinct recoverable failures were merged into a repeated denial")
	}
	appendHandled("goal.decide", "capability_not_authorized", 3)
	appendHandled("goal.decide", "capability_not_authorized", 4)
	if !conversationLoopNeedsReply(ctx) {
		t.Fatal("the same denied capability with different arguments was not bounded")
	}
	trace.Append(CapabilityInvocation{CallID: "real-action", CapabilityName: lifeActivityStartCapabilityName}, CapabilityResult{CallID: "real-action", CapabilityName: lifeActivityStartCapabilityName, Status: "accepted"})
	if conversationLoopNeedsReply(ctx) {
		t.Fatal("an accepted real action did not reopen normal progress")
	}
}
