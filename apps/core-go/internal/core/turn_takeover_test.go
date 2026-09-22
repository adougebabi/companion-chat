package core

import (
	"strings"
	"testing"
)

// Visible publication follows the native Tool result. Intermediate structured
// content is neither a second Tool protocol nor a competing reply candidate.
// This replaces the retired response_plan/root/reply-argument precedence and
// candidate-preview fixtures with an end-to-end assertion at the formal Agent
// boundary.
func TestNativeConversationReplyReceiptIsTheVisiblePublicationAuthority(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "native-visible-owner", "native-visible-fluctlight", "native-visible-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(payload map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			return fakeProviderResult{
				// This content is an intermediate model sidecar. It cannot publish or
				// override the native Tool result below.
				Structured: map[string]any{
					"visible_text":  "未提交的中间候选",
					"response_plan": map[string]any{"visible_text": "另一个未提交候选"},
				},
				ToolCalls: []map[string]any{
					nativePersonaToolCall("native-visible-reply", "conversation.reply", map[string]any{"text": "已提交的唯一可见回复"}),
				},
			}
		case 2:
			tools := nativePersonaToolMessages(payload)
			if !strings.Contains(tools, "conversation_message") || !strings.Contains(tools, "completed") {
				t.Fatalf("committed reply receipt missing from final decision: %s", tools)
			}
			return nativePersonaFinal()
		default:
			t.Fatalf("unexpected extra model decision %d", step)
			return fakeProviderResult{Status: 500}
		}
	})

	app := newTestApp(t, repository, router)
	result, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "请回复。", "native-visible-turn", "native-visible-turn-1"))
	if err != nil {
		t.Fatal(err)
	}
	if got := stringValue(result.Assistant["text"]); got != "已提交的唯一可见回复" {
		t.Fatalf("assistant text=%q", got)
	}
	texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "native-visible-turn-1")
	if len(texts) != 1 || texts[0] != "已提交的唯一可见回复" {
		t.Fatalf("published assistant messages=%#v", texts)
	}
	results := nativePersonaTrace(t, ctx, repository, "native-visible-turn")
	receipt := nativePersonaResultByName(t, results, "conversation.reply")
	if firstString(receipt["status"], stringValue(receipt["Status"])) != "completed" {
		t.Fatalf("reply receipt was not committed: %#v", receipt)
	}
	if step != 2 || router.requestCount(workingPersonaMainTurnSchema) != 2 {
		t.Fatalf("native reply path used %d decisions / %d requests", step, router.requestCount(workingPersonaMainTurnSchema))
	}
}
