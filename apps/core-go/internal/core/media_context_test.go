package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestImageCapabilityPrepareConsumesResolvedContext(t *testing.T) {
	capability := imageGenerateCapability{}
	invocation := CapabilityInvocation{CallID: "image-1", CapabilityName: "media.image.generate", Arguments: []byte(`{"intent":"portrait"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	resolved := CapabilityContext{
		Visual: &VisualIdentityContext{Data: map[string]any{"asset_id": "visual-1"}},
		Life:   &CurrentLifeContext{Data: map[string]any{"scene": "studio"}},
		Outfit: &AppearanceContext{Data: map[string]any{"hair": "long"}},
		State:  &CurrentStateContext{Data: map[string]any{"mood": map[string]any{"label": "calm"}}},
	}
	prepared, err := capability.Prepare(nil, invocation, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if string(prepared.Arguments) != string(invocation.Arguments) {
		t.Fatalf("provider arguments changed during prepare: before=%s after=%s", invocation.Arguments, prepared.Arguments)
	}
	rawConcept, found, decodeErr := capabilityPreparedData(prepared, "media_concept")
	if decodeErr != nil || !found {
		t.Fatalf("prepared image payload missing: found=%v err=%v payload=%s", found, decodeErr, prepared.PreparedPayload)
	}
	concept := mapValue(rawConcept)
	binding := mapValue(concept["context_binding"])
	if stringValue(concept["intent"]) != "portrait" || stringValue(mapValue(binding["current_life"])["scene"]) != "studio" || stringValue(mapValue(binding["appearance"])["hair"]) != "long" {
		t.Fatalf("prepared image payload lost context: %#v", concept)
	}
}

func TestImageIntentAndCanonicalContextReachMediaPromptInput(t *testing.T) {
	capability := imageGenerateCapability{}
	invocation := CapabilityInvocation{CallID: "image-2", CapabilityName: "media.image.generate", Arguments: []byte(`{"intent":"在窗边读书"}`), SourceFactID: "fact-2", ProviderRequestID: "provider-2"}
	resolved := CapabilityContext{
		Visual: &VisualIdentityContext{Data: map[string]any{"status": "active", "renderer_constraints": map[string]any{"chest_cup": "B"}}},
		Life:   &CurrentLifeContext{Data: map[string]any{"scene": "窗边", "activity": "阅读", "location": "客厅"}},
		Outfit: &AppearanceContext{Data: map[string]any{"outfit": "针织衫"}},
		State:  &CurrentStateContext{Data: map[string]any{"mood": map[string]any{"label": "平静", "intensity": 0.4}, "pad": map[string]any{"pleasure": 0.3, "arousal": 0.1, "dominance": 0.2}}},
	}
	prepared, err := capability.Prepare(context.Background(), invocation, resolved)
	if err != nil {
		t.Fatal(err)
	}
	rawConcept, found, err := capabilityPreparedData(prepared, "media_concept")
	if err != nil || !found {
		t.Fatalf("prepared concept missing: found=%v err=%v", found, err)
	}
	promptInput := mediaPromptInput(mediaIntent{Prompt: jsonString(rawConcept)})
	var providerConcept map[string]any
	if err := json.Unmarshal([]byte(promptInput), &providerConcept); err != nil {
		t.Fatalf("media prompt input is not JSON: %v: %s", err, promptInput)
	}
	binding := mapValue(providerConcept["context_binding"])
	if stringValue(providerConcept["intent"]) != "在窗边读书" ||
		stringValue(mapValue(binding["current_life"])["scene"]) != "窗边" ||
		stringValue(mapValue(mapValue(binding["current_state"])["mood"])["label"]) != "平静" ||
		stringValue(mapValue(binding["appearance"])["outfit"]) != "针织衫" ||
		stringValue(mapValue(mapValue(binding["visual_identity"])["renderer_constraints"])["chest_cup"]) != "B" {
		t.Fatalf("canonical media intent/context was lost: %s", promptInput)
	}
	for _, legacy := range []string{"life_context", "inner_state"} {
		if _, found := binding[legacy]; found {
			t.Fatalf("legacy media context key %q remains: %s", legacy, promptInput)
		}
	}
}

func TestWithContextAuthorityInstructionKeepsUserMessageLast(t *testing.T) {
	messages := withContextAuthorityInstruction([]map[string]any{
		{"role": "system", "content": "decide"},
		{"role": "user", "content": "current request"},
	})
	if len(messages) != 2 || stringValue(messages[0]["role"]) != "system" || stringValue(messages[1]["role"]) != "user" {
		t.Fatalf("messages = %#v", messages)
	}
	if !strings.Contains(stringValue(messages[0]["content"]), "context.current_state") || !strings.Contains(stringValue(messages[0]["content"]), "life_context.current_time") || !strings.Contains(stringValue(messages[0]["content"]), "decide") {
		t.Fatalf("authority instruction = %#v", messages[0])
	}
}

func TestWithChineseOutputInstructionExcludesMediaPromptRole(t *testing.T) {
	messages := []map[string]any{
		{"role": "system", "content": "规则"},
		{"role": "user", "content": "内容"},
	}
	localized := withChineseOutputInstruction("cognitive_assessment", messages)
	if len(localized) != 2 || stringValue(localized[0]["role"]) != "system" || stringValue(localized[1]["role"]) != "user" {
		t.Fatalf("localized messages = %#v", localized)
	}
	if !strings.Contains(stringValue(localized[0]["content"]), "自然语言内容使用中文") || !strings.Contains(stringValue(localized[0]["content"]), "规则") {
		t.Fatalf("language instruction = %#v", localized[0])
	}
	media := withChineseOutputInstruction("media_prompt", messages)
	if len(media) != len(messages) {
		t.Fatalf("media prompt messages were changed: %#v", media)
	}
}

func TestWithChineseOutputInstructionMovesLateSystemMessagesToFront(t *testing.T) {
	localized := withChineseOutputInstruction("cognitive_assessment", []map[string]any{
		{"role": "user", "content": "先出现的用户消息"},
		{"role": "system", "content": "迟到的系统规则"},
		{"role": "assistant", "content": "历史回复"},
	})
	if len(localized) != 3 || stringValue(localized[0]["role"]) != "system" || stringValue(localized[1]["role"]) != "user" || stringValue(localized[2]["role"]) != "assistant" {
		t.Fatalf("localized messages = %#v", localized)
	}
	if !strings.Contains(stringValue(localized[0]["content"]), "迟到的系统规则") {
		t.Fatalf("late system rule was lost: %#v", localized[0])
	}
}

func TestSystemInstructionMergesToExactlyOneSystemMessage(t *testing.T) {
	localized := withChineseOutputInstruction("cognitive_assessment", withContextAuthorityInstruction([]map[string]any{
		{"role": "system", "content": "operation rules"},
		{"role": "user", "content": "request"},
		{"role": "system", "content": "late rules"},
	}))
	systemCount := 0
	for _, message := range localized {
		if stringValue(message["role"]) == "system" {
			systemCount++
		}
	}
	if systemCount != 1 || stringValue(localized[0]["role"]) != "system" {
		t.Fatalf("system messages = %#v", localized)
	}
	for _, expected := range []string{"operation rules", "late rules", "context.current_state", "自然语言内容使用中文"} {
		if !strings.Contains(stringValue(localized[0]["content"]), expected) {
			t.Fatalf("merged system content missing %q: %s", expected, localized[0]["content"])
		}
	}
}
