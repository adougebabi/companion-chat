package core

import (
	"strings"
	"testing"
)

func TestNaturalFinalTextCannotReplaceNativeConversationReply(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "natural-final-owner", "natural-final-fluctlight", "natural-final-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
		result := nativePersonaFinal()
		result.Structured["visible_text"] = "自然 final 回复"
		return result
	})
	app := newTestApp(t, repository, router)
	_, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "你好。", "natural-final-turn", "natural-final-turn-1"))
	if err == nil || !strings.Contains(err.Error(), "conversation_native_output_missing") {
		t.Fatalf("unpublished final bypassed native Reply boundary: %v", err)
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID); count != 0 {
		t.Fatalf("assistant count=%d", count)
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='conversation.reply'`, fluctlightID); count != 0 {
		t.Fatal("fabricated reply receipt", count)
	}
}
