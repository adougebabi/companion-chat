package core

import (
	"testing"
)

// ---------------------------------------------------------------------------
// F-01: 统一回复提案权威（路径 b：根 visible_text 优先，
// conversation.reply 可作为 reply-only fallback）
// ---------------------------------------------------------------------------
//
// request.md:13/188/240-242 要求 Main cognition 不重新增加平行的
// visible_reply 路径。根 decision.visible_text / response_plan.visible_text
// 是优先正式协议；conversation.reply 只在根字段为空时作为 fallback，由
// Core 派生同一个 canonical visible_text。根字段与 reply.text 不一致时
// fail closed，不能静默选择 winner。

// TestRootVisibleTextWithoutReplyInvocationIsAccepted proves the F-01 path b
// legal form: a candidate that carries the root visible_text and no
// conversation.reply invocation is accepted, because conversation.reply is a
// Core-derived execution record, not a model proposal source. The message is
// delivered from the frozen root field, not from a reply argument.
func TestRootVisibleTextWithoutReplyInvocationIsAccepted(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-f01-noreply-owner", "chain-f01-noreply-fluctlight", "chain-f01-noreply-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	candidate := takeoverChainMainResult("根字段唯一的回复", nil)
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(candidate)).
		on(takeoverJudgeSchemaName, takeoverChainJudge(false))
	app := newTestApp(t, repository, router)

	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你好。", "chain-f01-noreply-turn", "chain-f01-noreply-turn-1")); err != nil {
		t.Fatalf("a root-visible-text candidate without a reply invocation must be accepted: %v", err)
	}
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-f01-noreply-turn-1"); len(texts) != 1 || texts[0] != "根字段唯一的回复" {
		t.Fatalf("the root visible_text must be delivered verbatim: %#v", texts)
	}
}

// TestConflictingReplyArgumentFailsClosed proves the F-01 path b contract
// (request.md:240-242): a candidate that proposes a root visible_text and a
// conversation.reply argument with different texts is a conflict the model
// cannot silently resolve. The turn fails closed before the Judge is consulted
// and no message is delivered, so the model can never produce two texts.
func TestConflictingReplyArgumentFailsClosed(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "chain-f01-conflict-owner", "chain-f01-conflict-fluctlight", "chain-f01-conflict-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	candidate := fakeProviderResult{
		Structured: map[string]any{
			"response_mode": "final", "action_type": "reply", "response_intent": "reply",
			"visible_text": "根字段的文本", "tool_calls": []any{}, "influences": []any{},
		},
		ToolCalls: []map[string]any{
			{"id": "a-conflict-reply", "type": "function", "function": map[string]any{"name": "conversation.reply", "arguments": `{"text":"reply 参数的不同文本"}`}},
		},
	}
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(candidate)).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)

	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你好。", "chain-f01-conflict-turn", "chain-f01-conflict-turn-1")); err == nil {
		t.Fatal("a conflicting reply argument must fail closed")
	}
	if count := router.requestCount(takeoverJudgeSchemaName); count != 0 {
		t.Fatalf("a conflicting candidate must fail before the Judge, got %d judge calls", count)
	}
	if count := router.requestCount(takeoverReplySchemaName); count != 0 {
		t.Fatalf("a conflicting candidate must not reach the takeover generation, got %d", count)
	}
	if texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "chain-f01-conflict-turn-1"); len(texts) != 0 {
		t.Fatalf("a conflicting candidate must not be delivered: %#v", texts)
	}
}
