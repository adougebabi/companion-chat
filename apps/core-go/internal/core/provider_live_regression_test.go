package core

import (
	"os"
	"strings"
	"testing"
)

// These scenarios deliberately call the configured OpenAI-compatible Provider.
// They are opt-in because they require a live model and can take minutes.
func requireLiveRegressionProvider(t *testing.T) (string, string) {
	t.Helper()
	if value, _ := os.LookupEnv("FLUCTLIGHT_LIVE_PROVIDER_TEST"); strings.TrimSpace(value) != "1" {
		t.Skip("set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider")
	}
	return liveProviderConfig(t)
}

func TestLiveRegressionMultiPersonalityDayNightContract(t *testing.T) {
	baseURL, model := requireLiveRegressionProvider(t)
	description := `创建一个名为“昼夜”的多重人格摇光。她有两个稳定且互不混淆的人格：
“晨星”负责早晨与白天，语气明亮、主动、行动迅速，喜欢安排当天计划；
“夜潮”负责晚上与深夜，语气安静、克制、细腻，喜欢整理一天的记忆。
规则必须明确：早晨和白天由晨星主导，晚上和深夜由夜潮主导；不要按关键词机械切换，必须结合当前时间和场景。
请建立两个目标：完成每日计划、在夜间整理当天经历；建立对应意图。返回完整结构化初始化对象，不要输出解释。`
	schema := initializationResponseSchema()
	messages := composeProviderMessages("initialization", []map[string]any{
		{"role": "system", "content": "Extract the Owner description into the canonical initialization response. Preserve every personality, time-based switching rule, goal and intention. Return JSON only. " + initializationResponseShapeInstruction},
		{"role": "user", "content": description},
	})
	message := privateLiveProviderMessage(t, baseURL, providerChatPayloadWithSchema(model, messages, initializationMinimumOutputReserveTokens, true, nil, "initialization", "initialization_response", schema, false))
	raw, ok := parseStructuredCandidates(providerStructuredCandidates(message))
	if !ok {
		t.Fatalf("multi-personality Provider response was not structured: content=%s reasoning=%s", boundedLiveProviderValue(message["content"]), boundedLiveProviderValue(message["reasoning_content"]))
	}
	persona := mapValue(raw["core_persona"])
	system := mapValue(persona["personality_system"])
	profiles := arrayValue(system["profiles"])
	if stringValue(system["mode"]) != "multiple" || len(profiles) < 2 {
		t.Fatalf("multi-personality contract was not preserved: %s", boundedLiveProviderValue(raw))
	}
	joined := strings.ToLower(jsonString(system))
	for _, marker := range []string{"晨星", "夜潮", "早晨", "白天", "晚上", "深夜"} {
		if !strings.Contains(joined, strings.ToLower(marker)) {
			t.Fatalf("multi-personality response lost switching marker %q: %s", marker, boundedLiveProviderValue(system))
		}
	}
	prepared, err := prepareInitializationResponse(raw)
	if err != nil || !validInitialization(prepared) {
		t.Fatalf("multi-personality response failed Core initialization validation: err=%v response=%s", err, boundedLiveProviderValue(raw))
	}
}

func TestLiveRegressionConversationImageAndReplySameTurn(t *testing.T) {
	baseURL, model := requireLiveRegressionProvider(t)
	definitions := []CapabilityDefinition{conversationReplyCapabilityDefinition(), imageCapabilityDefinition(), affectEventCapabilityDefinition()}
	schema := cognitiveTurnResponseSchema()
	messages := []map[string]any{
		{"role": "system", "content": capabilityConversationPolicyInstruction},
		{"role": "user", "content": "请同时完成两件事：调用图片能力生成一张雨后昏暗卧室里 22 岁中国女性的真实照片，并用 conversation.reply 给我一句正常回复。不能只调用图片，也不能只返回文字。"},
	}
	message := privateLiveProviderMessage(t, baseURL, providerChatPayloadWithSchema(model, messages, 1800, true, definitions, "cognitive_assessment", "conversation_turn_response", schema, true))
	structured, _ := liveProviderStructuredOrFallback(t, message, "conversation_turn_response", schema)
	calls := liveProviderInvocations(t, message, structured)
	assertLiveInvocationPair(t, calls, "same-turn image+reply")
}

func TestLiveRegressionWakeUpProactiveImageAndReplyBundle(t *testing.T) {
	baseURL, model := requireLiveRegressionProvider(t)
	definitions := []CapabilityDefinition{conversationReplyCapabilityDefinition(), imageCapabilityDefinition(), affectEventCapabilityDefinition()}
	schema := wakeUpResponseSchema()
	messages := []map[string]any{
		{"role": "system", "content": capabilityWakeUpPolicyInstruction},
		{"role": "user", "content": "这是一次 Wake-up 主动行动判断。当前没有新的用户消息，但模型可以主动向 actor_user 发起高强度互动。请同时调用 media.image.generate 生成一张真实图片，并调用 conversation.reply 发送一句正常的主动消息；不要只输出 prose，也不要只调用其中一个能力。"},
	}
	message := privateLiveProviderMessage(t, baseURL, providerChatPayloadWithSchema(model, messages, 1800, true, definitions, "cognitive_assessment", "wake_up_response", schema, true))
	structured, fallback := liveProviderStructuredOrFallback(t, message, "wake_up_response", schema)
	calls := liveProviderInvocations(t, message, structured)
	if len(calls) == 0 {
		t.Fatalf("Wake-up Provider returned no capability calls; fallback=%v structured=%s message=%s", fallback, jsonString(structured), boundedLiveProviderValue(message))
	}
	assertLiveInvocationPair(t, calls, "wake-up proactive image+reply")
	if fallback && stringValue(structured["action_type"]) == "" {
		t.Fatalf("Wake-up fallback lost the typed action decision: %s", jsonString(structured))
	}
}

func assertLiveInvocationPair(t *testing.T, calls []CapabilityInvocation, scenario string) {
	t.Helper()
	image, reply := false, false
	for _, call := range calls {
		switch call.CapabilityName {
		case "media.image.generate":
			image = true
		case "conversation.reply":
			reply = strings.TrimSpace(stringValue(decodeObject(call.Arguments)["text"])) != ""
		}
	}
	if !image || !reply {
		t.Fatalf("%s did not produce both executable calls: image=%v reply=%v calls=%s", scenario, image, reply, jsonString(calls))
	}
}
