package core

import (
	"errors"
	"strings"
	"testing"
)

// Formal settings, Tool execution and the default direct turn use a disposable
// database. The Provider is controlled and no image renderer is called.
func TestMomentDisabledKeepsDefaultDirectChatAndImageAvailable(t *testing.T) {
	var mainCalls, momentCalls int
	var imageAccepted bool
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if payload["model"] == "m2-retained-moment-model" {
			momentCalls++
			return fakeProviderResult{Text: "这是一条已有的文字动态。"}
		}
		mainCalls++
		if r6PayloadHasTool(payload, "moment.publish") || r6PayloadHasTool(payload, momentInspectCapabilityName) {
			t.Error("disabled Moment was exposed in a new direct request")
		}
		switch mainCalls {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("m2-image-load", capabilityCatalogName, map[string]any{"names": []any{"media.image.generate"}})}}
		case 2:
			if !r6PayloadHasTool(payload, "media.image.generate") {
				t.Error("enabled Image was not offered after discovery")
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("m2-image-request", "media.image.generate", map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}}
		case 3:
			for _, raw := range arrayValue(payload["messages"]) {
				message := mapValue(raw)
				if message["role"] == "tool" && message["tool_call_id"] == "m2-image-request" {
					imageAccepted = strings.Contains(stringValue(message["content"]), "accepted")
				}
			}
			decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("m2-image-reply", conversationReplyCapabilityName, map[string]any{"text": "好，我继续处理。", "decision": decision})}}
		default:
			t.Errorf("unexpected main request %d", mainCalls)
			return fakeProviderResult{Status: 400}
		}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-off", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	role := agentModelBindingKey(FormalAgentConversationReply, "moment")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-m2-moment-off','m2-retained-moment-model',30)`, role); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://renderer.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
		t.Fatal(err)
	}
	moment := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-existing-moment", AuthorizationActorID: owner, FluctlightID: fid, Arguments: jsonBytes(map[string]any{})}
	first, err := app.ExecuteTool(ctx, moment)
	if err != nil || first.Result.Status != "completed" || momentCalls != 1 {
		t.Fatalf("initial Moment: %#v calls=%d err=%v", first, momentCalls, err)
	}
	momentID := stringValue(mapValue(first.Result.Output)["target_ref"])
	if momentID == "" {
		t.Fatal("initial Moment has no committed identity")
	}
	if _, err := app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": true, "moment": false}}, "expected_versions": map[string]any{}, "idempotency_key": "m2-disable-moment"}); err != nil {
		t.Fatal(err)
	}
	settings, err := app.ReadSettings(ctx, owner)
	plugins := mapValue(mapValue(settings["values"])[domainPluginsSettingKey])
	if err != nil || plugins["image"] != true || plugins["moment"] != false {
		t.Fatalf("saved plugin settings=%#v err=%v", plugins, err)
	}
	replayed, err := app.ExecuteTool(ctx, moment)
	if err != nil || !toolBoolValue(mapValue(replayed.Result.Output)["replayed"]) || stringValue(mapValue(replayed.Result.Output)["target_ref"]) != momentID || momentCalls != 1 {
		t.Fatalf("committed Moment replay: %#v calls=%d err=%v", replayed, momentCalls, err)
	}
	moment.OperationID = "m2-new-disabled-moment"
	denied, err := app.ExecuteTool(ctx, moment)
	var capabilityErr *CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Code != "domain_plugin_disabled" || denied.Result.ErrorCode != "domain_plugin_disabled" {
		t.Fatalf("new disabled Moment was not rejected: %#v err=%v", denied, err)
	}
	payload := map[string]any{"fluctlight_id": fid, "text": "请看一看窗外风景。", "turn_id": "m2-moment-off-chat", "idempotency_key": "m2-moment-off-chat", "attachment_refs": []any{}}
	turn, err := app.HandleTurn(ctx, owner, conversation, payload)
	if err != nil || mainCalls != 3 || !imageAccepted || stringValue(turn.Assistant["text"]) != "好，我继续处理。" {
		t.Fatalf("default direct Image turn: %#v main=%d accepted=%t err=%v", turn.Assistant, mainCalls, imageAccepted, err)
	}
	if _, err := app.HandleTurn(ctx, owner, conversation, payload); err != nil || mainCalls != 3 {
		t.Fatalf("direct turn replay repeated model requests: calls=%d err=%v", mainCalls, err)
	}
	image := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m2-moment-off-independent-image", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}
	firstImage, err := app.ExecuteTool(ctx, image)
	if err != nil || firstImage.Result.Status != "accepted" {
		t.Fatalf("independent Image admission: %#v err=%v", firstImage, err)
	}
	imageReplay, err := app.ExecuteTool(ctx, image)
	if err != nil || !imageReplay.Replayed {
		t.Fatalf("independent Image replay: %#v err=%v", imageReplay, err)
	}
	var moments, intents, messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.moments WHERE id=$1 AND owner_fluctlight_id=$2), (SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$2 AND status='pending'), (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$3 AND kind='assistant')`, momentID, fid, conversation).Scan(&moments, &intents, &messages); err != nil || moments != 1 || intents != 2 || messages != 1 {
		t.Fatalf("unexpected durable effects: moments=%d intents=%d messages=%d err=%v", moments, intents, messages, err)
	}
}
