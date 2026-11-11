package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// A reply and a new image request in one model response cannot consume each
// other's outcomes. Neither may take effect until the model has reconsidered
// them in separate native tool rounds.
func TestDirectMediaAndReplySameBatchAreNotCommitted(t *testing.T) {
	for _, replyFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("reply_first_%t", replyFirst), func(t *testing.T) {
			calls := 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
				if calls == 1 {
					reply := nativePersonaToolCall("batch-reply", conversationReplyCapabilityName, map[string]any{"text": "我现在不想拍。", "decision": decision})
					media := nativePersonaToolCall("batch-media", "media.image.generate", map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})
					if replyFirst {
						return fakeProviderResult{ToolCalls: []map[string]any{reply, media}}
					}
					return fakeProviderResult{ToolCalls: []map[string]any{media, reply}}
				}
				seen := map[string]bool{}
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "tool" && strings.Contains(stringValue(message["content"]), "direct_media_reply_separate_turn_required") {
						seen[stringValue(message["tool_call_id"])] = true
					}
				}
				if !seen["batch-reply"] || !seen["batch-media"] {
					t.Errorf("continuation did not receive both rejected native results: %#v", seen)
				}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("final-reply", conversationReplyCapabilityName, map[string]any{"text": "我现在不想拍，改天吧。", "decision": decision})}}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, fmt.Sprintf("direct-batch-%t", replyFirst), router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			payload := map[string]any{"fluctlight_id": fid, "text": "能发张照片吗？", "idempotency_key": "direct-media-batch", "turn_id": "direct-media-batch", "attachment_refs": []any{}}
			result, err := app.HandleTurn(ctx, owner, conversation, payload)
			var messages, media, receipts int
			if queryErr := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'),
				(SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$2),
				(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='media.image.generate')`, conversation, fid).Scan(&messages, &media, &receipts); queryErr != nil {
				t.Fatal(queryErr)
			}
			if err != nil || calls != 2 || stringValue(result.Assistant["text"]) != "我现在不想拍，改天吧。" || messages != 1 || media != 0 || receipts != 0 {
				t.Fatalf("same-batch result=%#v calls=%d messages=%d media=%d media_receipts=%d err=%v", result, calls, messages, media, receipts, err)
			}
			_, err = app.HandleTurn(ctx, owner, conversation, payload)
			if err != nil || calls != 2 {
				t.Fatalf("replay repeated work: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestDirectMediaBatchRecognizesRepeatedCallID(t *testing.T) {
	for _, replyFirst := range []bool{false, true} {
		calls := []schema.ToolCall{
			{ID: "duplicate", Function: schema.FunctionCall{Name: "media.image.generate"}},
			{ID: "duplicate", Function: schema.FunctionCall{Name: conversationReplyCapabilityName}},
		}
		if replyFirst {
			calls[0], calls[1] = calls[1], calls[0]
		}
		refresh := &runtimeContextRefresh{}
		refresh.noteModelToolCalls(1, calls)
		if !refresh.directMediaReplyBatch(1, "duplicate") {
			t.Fatalf("duplicate call ID hid same-batch media/reply, replyFirst=%t", replyFirst)
		}
	}
}

func TestDirectMediaBatchStallCountsRoundsAndResetsOnResult(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	for sequence := uint64(1); sequence <= 3; sequence++ {
		replyID := fmt.Sprintf("reply-%d", sequence)
		mediaID := fmt.Sprintf("media-%d", sequence)
		trace.RecordModelToolCalls("provider", sequence, []schema.ToolCall{
			{ID: replyID, Function: schema.FunctionCall{Name: conversationReplyCapabilityName}},
			{ID: mediaID, Function: schema.FunctionCall{Name: "media.image.generate"}},
		})
		trace.AppendResult(CapabilityResult{CallID: replyID, CapabilityName: conversationReplyCapabilityName, Status: "rejected", ErrorCode: "direct_media_reply_separate_turn_required"})
		trace.AppendResult(CapabilityResult{CallID: mediaID, CapabilityName: "media.image.generate", Status: "rejected", ErrorCode: "direct_media_reply_separate_turn_required"})
		if got := directMediaReplyRejectedBatches(trace); got != int(sequence) {
			t.Fatalf("count=%d after %d physical responses", got, sequence)
		}
	}
	trace.AppendResult(CapabilityResult{CallID: "real-result", CapabilityName: "memory.recall", Status: "completed"})
	if got := directMediaReplyRejectedBatches(trace); got != 0 {
		t.Fatalf("new actual result did not reset stalled batches: %d", got)
	}
}

func TestDirectMediaAndReplyRepeatedBatchStops(t *testing.T) {
	calls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		reply := nativePersonaToolCall(fmt.Sprintf("reply-%d", calls), conversationReplyCapabilityName, map[string]any{"text": "我再想想。", "decision": decision})
		if calls > 3 {
			return fakeProviderResult{ToolCalls: []map[string]any{reply}}
		}
		media := nativePersonaToolCall(fmt.Sprintf("media-%d", calls), "media.image.generate", map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})
		return fakeProviderResult{ToolCalls: []map[string]any{media, reply}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-repeated-batch", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	payload := map[string]any{"fluctlight_id": fid, "text": "给我看看", "idempotency_key": "direct-repeated-batch", "turn_id": "direct-repeated-batch", "attachment_refs": []any{}}
	result, err := app.HandleTurn(ctx, owner, conversation, payload)
	var messages, media int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'),
		(SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$2)`, conversation, fid).Scan(&messages, &media); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err == nil || !strings.Contains(err.Error(), "direct_media_reply_stalled") || calls != 3 || messages != 0 || media != 0 {
		t.Fatalf("repeated batch result=%#v calls=%d messages=%d media=%d err=%v", result, calls, messages, media, err)
	}
}
