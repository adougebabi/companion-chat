package core

import (
	"errors"
	"strings"
	"testing"
)

func TestDirectReplyRetryRequiresTerminalWorkflow(t *testing.T) {
	if !directReplyRetryFailure("agent_final_contract_invalid", "failed") {
		t.Fatal("terminal missing reply should qualify")
	}
	if !directReplyRetryFailure("direct_media_delivery_unresolved", "failed") {
		t.Fatal("terminal delivery check failure should qualify after a committed image")
	}
	for _, status := range []string{"", "pending", "started", "running", "paused", "cancel_requested"} {
		if directReplyRetryFailure("agent_final_contract_invalid", status) {
			t.Errorf("active or unknown workflow %q qualified for recovery", status)
		}
	}
}

func TestDirectReplyRetryUsesExistingMediaWithoutReplayingTurn(t *testing.T) {
	const recovered = "自拍任务已经提交，等图片生成后再发。"
	mainCalls := 0
	router := newFakeProviderRouter().
		on(directImageDiscoverySchema, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"needed_tool": "media.image.generate"}}
		}).
		on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
		}).
		otherwise(func(payload map[string]any) fakeProviderResult {
			mainCalls++
			invoke := func(id, name string, args map[string]any) fakeProviderResult {
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(id, name, args)}}
			}
			reply := func(id, text string) fakeProviderResult {
				return invoke(id, conversationReplyCapabilityName, map[string]any{"text": text, "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}})
			}
			switch mainCalls {
			case 1:
				return reply("retry-prior", "拍好自拍再发给你。")
			case 2:
				return invoke("retry-catalog", capabilityCatalogName, map[string]any{"names": []any{"media.image.generate"}})
			case 3:
				plan := mediaTestPlan("person", "present", "yes", "full")
				plan["capture_method"], plan["viewpoint"], plan["framing"], plan["camera_angle"] = "handheld_selfie", "subject_facing", "full_body", "eye_level"
				return invoke("retry-image", "media.image.generate", map[string]any{"intent": "拍一张当前自拍", "media_plan": plan})
			case 4, 5:
				return fakeProviderResult{Text: "模型输出文字但没有调用原生发布工具。"}
			case 6:
				tools := arrayValue(payload["tools"])
				if len(tools) != 1 || !r6PayloadHasTool(payload, conversationReplyCapabilityName) || !strings.Contains(jsonString(payload["messages"]), "reply_recovery") {
					t.Error("retry did not offer only the native reply tool with the actual media status")
				}
				return reply("retry-recovered", recovered)
			default:
				t.Errorf("unexpected model round %d", mainCalls)
				return fakeProviderResult{Status: 500}
			}
		})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-reply-only-recovery", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id,status,identity_snapshot) VALUES('direct-retry-identity',$1,'active','{"identity":{"name":"Fixture identity"}}')`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	if _, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "能发张自拍吗？", "idempotency_key": "retry-prior", "attachment_refs": []any{}}); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"fluctlight_id": fid, "text": "还没发哦", "turn_id": "retry-user-turn", "idempotency_key": "retry-user-key", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err == nil || !strings.Contains(err.Error(), "conversation_native_output_missing") {
		t.Fatalf("expected an accepted media operation with no native reply, got %v", err)
	}
	// The fixture invokes the activity directly. In production the dispatcher
	// records this terminal state before making the retry available.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',completed_at=now() WHERE intent_id=$1`, "cognition_intent:"+accepted.InboxID); err != nil {
		t.Fatal(err)
	}
	page, err := repo.History(ctx, conversation, owner, nil, 10)
	if err != nil || len(page.Messages) != 3 || !page.Messages[2].TurnReplyRetryable || page.Messages[2].TurnRetryable {
		t.Fatalf("failed turn did not expose only reply recovery: %#v, %v", page.Messages, err)
	}
	if err := app.RetryDirectReply(ctx, "different-owner", conversation, accepted.TurnID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong actor could request recovery: %v", err)
	}
	if _, err := app.AcceptTurn(ctx, owner, conversation, payload); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("ordinary replay reopened the partial turn: status=%s err=%v", status, err)
	}
	for range 2 {
		if err := app.RetryDirectReply(ctx, owner, conversation, accepted.TurnID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err != nil {
		t.Fatal(err)
	}
	if err := app.RetryDirectReply(ctx, owner, conversation, accepted.TurnID); err != nil {
		t.Fatalf("repeating an already completed recovery should report success: %v", err)
	}
	page, err = repo.History(ctx, conversation, owner, nil, 10)
	if err != nil || len(page.Messages) != 4 || page.Messages[2].TurnStatus != "completed" || page.Messages[3].Text != recovered ||
		page.Messages[2].ID != stringValue(accepted.UserMessage["id"]) {
		t.Fatalf("reply recovery did not finish the original turn: %#v, %v", page.Messages, err)
	}
	var intents int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE conversation_id=$1`, conversation).Scan(&intents); err != nil || intents != 1 || mainCalls != 6 {
		t.Fatalf("reply retry produced additional work: images=%d calls=%d err=%v", intents, mainCalls, err)
	}
}
