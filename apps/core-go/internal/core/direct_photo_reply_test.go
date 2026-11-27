package core

import (
	"fmt"
	"strings"
	"testing"
)

func TestDirectPhotoPublicationCorrectionPublishes(t *testing.T) {
	const finalReply = "我先想想怎么拍。"
	calls := 0
	router := newFakeProviderRouter().
		otherwise(func(payload map[string]any) fakeProviderResult {
			calls++
			influences := []any{}
			switch calls {
			case 2:
				return fakeProviderResult{Text: "这条候选没有调用工具。"}
			case 3:
				checkDirectImageReplySchema(t, payload)
				if !strings.Contains(jsonString(payload["messages"]), "private_reply_not_published") {
					t.Error("the correction must give the model the native reply tool")
				}
				influences = append(influences, map[string]any{"ref": "cr_ffffffffffff", "role": "grounds", "confidence": 1, "note": "无效引用"})
			default:
				if calls != 1 {
					t.Error("native correction did not finish the original turn")
					return fakeProviderResult{}
				}
			}
			call := nativePersonaToolCall(fmt.Sprintf("reply-correction-%d", calls), conversationReplyCapabilityName, map[string]any{
				"text": finalReply, "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": influences},
			})
			return fakeProviderResult{ToolCalls: []map[string]any{call}}
		})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-photo-correction", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	for index, input := range []string{"能发张自拍吗？", "你觉得怎么样？"} {
		result, err := app.HandleTurn(ctx, owner, conversation, map[string]any{
			"fluctlight_id": fid, "text": input, "idempotency_key": fmt.Sprintf("catalog-correction-turn-%d", index), "attachment_refs": []any{},
		})
		if err != nil || stringValue(result.Assistant["text"]) != finalReply {
			t.Fatalf("turn %d did not publish the model's native reply: %v", index, err)
		}
	}
	var images int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE conversation_id=$1`, conversation).Scan(&images); err != nil || images != 0 || calls != 3 {
		t.Fatalf("unexpected picture action or continuation: calls=%d images=%d err=%v", calls, images, err)
	}
}

func checkDirectImageReplySchema(t *testing.T, payload map[string]any) {
	t.Helper()
	tools := arrayValue(payload["tools"])
	if len(tools) != 1 || !r6PayloadHasTool(payload, conversationReplyCapabilityName) {
		t.Fatal("expected only the native private reply")
	}
	parameters := mapValue(mapValue(mapValue(tools[0])["function"])["parameters"])
	decision := mapValue(mapValue(parameters["properties"])["decision"])
	properties := mapValue(decision["properties"])
	if len(properties) != 3 || properties["action_type"] == nil || properties["response_intent"] == nil || properties["influences"] == nil {
		t.Fatalf("image reply received the wrong decision schema: %v", properties)
	}
}
