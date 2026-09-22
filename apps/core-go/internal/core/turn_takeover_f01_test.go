package core

import "testing"

func TestNaturalFinalReplyNeedsNoSyntheticConversationReplyTool(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "natural-final-owner", "natural-final-fluctlight", "natural-final-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
		result := nativePersonaFinal()
		result.Structured["visible_text"] = "自然 final 回复"
		return result
	})
	app := newTestApp(t, repository, router)
	result, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "你好。", "natural-final-turn", "natural-final-turn-1"))
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(result.Assistant["text"]) != "自然 final 回复" {
		t.Fatalf("assistant=%#v", result.Assistant)
	}
	if trace := nativePersonaTrace(t, ctx, repository, "natural-final-turn"); len(trace) != 0 {
		t.Fatalf("natural reply fabricated Tool results: %#v", trace)
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID); count != 1 {
		t.Fatalf("assistant count=%d", count)
	}
}
