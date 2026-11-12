package core

import (
	"errors"
	"strings"
	"testing"
)

// A disposable database and controlled Provider exercise actual configuration,
// native Tool execution and publication. They do not prove live model quality.
func TestImageDisabledKeepsDirectChatAndTextMomentAvailable(t *testing.T) {
	mainRequests, momentRequests := 0, 0
	var consumedMomentResult bool
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if stringValue(payload["model"]) == "controlled-moment-model" {
			momentRequests++
			return fakeProviderResult{Text: "今天想记录一件小事。"}
		}
		mainRequests++
		var momentOffered, imageOffered bool
		for _, raw := range arrayValue(payload["tools"]) {
			name := strings.ReplaceAll(stringValue(mapValue(mapValue(raw)["function"])["name"]), ".", "_")
			momentOffered = momentOffered || name == "moment_publish"
			imageOffered = imageOffered || name == "media_image_generate"
		}
		if imageOffered {
			t.Error("disabled Image was exposed in a new main-model request")
		}
		if !payloadHasToolResult(payload) {
			if !momentOffered {
				t.Error("enabled Moment was not offered to the main model")
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("image-off-moment", "moment.publish", map[string]any{})}}
		}
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "tool" && message["tool_call_id"] == "image-off-moment" {
				consumedMomentResult = strings.Contains(stringValue(message["content"]), "今天想记录一件小事。")
			}
		}
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("image-off-reply", conversationReplyCapabilityName, map[string]any{"text": "好，今天一起慢慢来。", "decision": decision})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "image-off-text", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	momentRole := agentModelBindingKey(FormalAgentConversationReply, "moment")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-image-off-text','controlled-moment-model',30)`, momentRole); err != nil {
		t.Fatal(err)
	}
	if _, err := app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": false, "moment": true}}, "expected_versions": map[string]any{}, "idempotency_key": "image-off-settings"}); err != nil {
		t.Fatal(err)
	}
	settings, err := app.ReadSettings(ctx, owner)
	plugins := mapValue(mapValue(settings["values"])[domainPluginsSettingKey])
	if err != nil || plugins["image"] != false || plugins["moment"] != true {
		t.Fatalf("saved plugin settings=%#v err=%v", plugins, err)
	}
	payload := map[string]any{"fluctlight_id": fid, "text": "请发一条文字动态。", "turn_id": "image-off-chat", "idempotency_key": "image-off-chat", "attachment_refs": []any{}}
	turn, err := app.HandleTurn(ctx, owner, conversation, payload)
	if err != nil || stringValue(turn.Assistant["text"]) != "好，今天一起慢慢来。" || !consumedMomentResult || mainRequests != 2 || momentRequests != 1 {
		t.Fatalf("direct chat/Moment failed: main=%d moment=%d result_consumed=%t turn=%#v err=%v", mainRequests, momentRequests, consumedMomentResult, turn.Assistant, err)
	}
	if _, err := app.HandleTurn(ctx, owner, conversation, payload); err != nil || mainRequests != 2 || momentRequests != 1 {
		t.Fatalf("chat replay called models again: main=%d moment=%d err=%v", mainRequests, momentRequests, err)
	}
	independent := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "image-off-independent-moment", AuthorizationActorID: owner, FluctlightID: fid, Arguments: jsonBytes(map[string]any{})}
	first, err := app.ExecuteTool(ctx, independent)
	if err != nil || first.Result.Status != "completed" || stringValue(mapValue(first.Result.Output)["text"]) != "今天想记录一件小事。" || momentRequests != 2 {
		t.Fatalf("independent Moment failed: %#v calls=%d err=%v", first, momentRequests, err)
	}
	again, err := app.ExecuteTool(ctx, independent)
	if err != nil || !toolBoolValue(mapValue(again.Result.Output)["replayed"]) || momentRequests != 2 {
		t.Fatalf("independent Moment replay failed: %#v calls=%d err=%v", again, momentRequests, err)
	}
	image := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "image-off-new", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}
	denied, err := app.ExecuteTool(ctx, image)
	var capabilityErr *CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Code != "domain_plugin_disabled" || denied.Result.ErrorCode != "domain_plugin_disabled" {
		t.Fatalf("disabled Image was not rejected: %#v err=%v", denied, err)
	}
	var moments, images, messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1), (SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1), (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$2 AND kind='assistant')`, fid, conversation).Scan(&moments, &images, &messages); err != nil || moments != 2 || images != 0 || messages != 1 {
		t.Fatalf("unexpected durable effects: moments=%d images=%d messages=%d err=%v", moments, images, messages, err)
	}
}
