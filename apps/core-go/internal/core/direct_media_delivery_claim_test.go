package core

import "testing"

// A model-generated completion claim is not evidence that a photo was sent.
// This fixture creates no media intent and makes no media ToolCall.
func TestDirectReplyDoesNotPublishUndeliveredPhotoClaim(t *testing.T) {
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		decision := map[string]any{"action_type": "reply", "response_intent": "回复", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("direct-media-claim", conversationReplyCapabilityName, map[string]any{"text": "我已经把照片发给你了。", "decision": decision})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-undelivered-photo-claim", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	_, turnErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "照片呢？", "turn_id": "direct-media-claim", "idempotency_key": "direct-media-claim", "attachment_refs": []any{}})
	var claims, mediaCalls, mediaIntents int
	err := repo.Pool().QueryRow(ctx, `SELECT
		(SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='我已经把照片发给你了。' AND attachment_refs='[]'::jsonb),
		(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='media.image.generate'),
		(SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$2)`, conversation, fid).Scan(&claims, &mediaCalls, &mediaIntents)
	if err != nil {
		t.Fatal(err)
	}
	if claims != 0 || mediaCalls != 0 || mediaIntents != 0 {
		t.Fatalf("unverified delivery claim or unexpected media action: claims=%d media_calls=%d media_intents=%d turn_error=%v", claims, mediaCalls, mediaIntents, turnErr)
	}
}
