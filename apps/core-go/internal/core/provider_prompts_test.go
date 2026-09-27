package core

import (
	"strings"
	"testing"
)

func TestProviderPromptInstructionsStayCompactAndPreserveContracts(t *testing.T) {
	checks := []struct {
		name  string
		value string
		max   int
		must  []string
	}{
		{name: "language", value: providerLanguageRule, max: 100, must: []string{"自然语言内容使用中文", "协议字面量保持原文"}},
		{name: "context", value: providerContextAuthorityRule, max: 520, must: []string{"core_persona", "developing_self", "current_state", "confirmed Event", "inferred Event", "accepted Schedule item", "context_override.explicit=true"}},
		{name: "wake-up", value: capabilityWakeUpPolicyInstruction, max: 700, must: []string{"正式 Agent", "Tool result", "no_op", "accepted"}},
		{name: "conversation", value: capabilityConversationPolicyInstruction, max: 900, must: []string{"正式 Agent", "Tool result", "conversation.reply", "visible_text", "evidence_refs"}},
		{name: "daily-review", value: capabilityDailyReviewPolicyInstruction, max: 700, must: []string{"正式 Agent", "Tool result", "Moment", "accepted"}},
		{name: "reflection", value: reflectionV2Instruction, max: 850, must: []string{"memory_candidates", "relationship_observations", "emotional_summary", "personality_evolution_candidates", "behavior_policy_evolution_candidates", "evidence_refs", "Core Persona"}},
		{name: "native-cognition", value: nativeCognitionInstruction, max: 300, must: []string{"appraisal", "attention", "thought", "desire", "agency"}},
		{name: "realization", value: actionRealizationInstruction, max: 320, must: []string{"core_persona", "developing_self", "current_state", "action_type"}},
		//{name: "media-prompt", value: mediaPromptInstruction, max: 4000, must: []string{"Determine the intended framing before choosing a camera relationship", "body-part or partial-body close-up", "rear-camera phone self-capture", "face or upper-body close-up", "front-camera phone selfie", "full-length mirror", "photographer and the camera/phone used by that photographer must remain outside the image", "If neither framing nor capture relationship is specified", "quality_feedback", "Do not add a human subject"}},
		{name: "media-quality", value: mediaQualityAcceptanceInstruction, max: 1800, must: []string{"strict visual consistency reviewer", "hard, observable consistency", "Do not judge beauty", "verdict pass", "Use retry only", "Use reject", "retry_guidance"}},
	}
	if !strings.Contains(providerRuntimeProtocol, "actor_user") || !strings.Contains(providerRuntimeProtocol, "context reference") || strings.Contains(providerRuntimeProtocol, "除用户明确要求") {
		t.Fatalf("runtime protocol must use actor_user and a concrete context-reference boundary: %s", providerRuntimeProtocol)
	}
	for _, rule := range []string{providerRuntimeProtocol, providerSingleRuntimeProtocol, providerContextAuthorityRule} {
		if !strings.Contains(rule, "actor_self") || !strings.Contains(rule, "actor_user") || !strings.Contains(rule, "scene_event") {
			t.Fatalf("scene authority rule lost actor ownership: %s", rule)
		}
	}
	if description := sceneCapabilityDefinition().Description; !strings.Contains(description, "actor_self") || !strings.Contains(description, "human saying where they are") {
		t.Fatalf("scene Tool does not identify the location subject: %s", description)
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if got := len([]rune(check.value)); got > check.max {
				t.Fatalf("instruction length = %d, want <= %d: %s", got, check.max, check.value)
			}
			for _, expected := range check.must {
				if !strings.Contains(check.value, expected) {
					t.Fatalf("instruction missing %q: %s", expected, check.value)
				}
			}
		})
	}
	for _, forbidden := range []string{"media_prompt", "YAML", "TOON", "JSON key"} {
		if strings.Contains(providerLanguageRule, forbidden) {
			t.Fatalf("transport-only instruction %q leaked into language rule: %s", forbidden, providerLanguageRule)
		}
	}
}

func TestMediaPromptInstructionRestoresCaptureFirstAndIdentityAuthority(t *testing.T) {
	for _, required := range []string{"先确定画面取景", "手持自拍", "镜前自拍", "first_person", "operator_pov", "external_capture", "默认本人前置相机自拍", "视觉身份和当下已知身体"} {
		if !strings.Contains(mediaPromptInstruction, required) {
			t.Fatalf("media instruction lost capture/identity rule %q", required)
		}
	}
	for _, forbidden := range []string{"画面不是普通自拍", "视觉年龄约 20–26", "胸部饱满", "默认生成年轻成年东方女性"} {
		if strings.Contains(mediaPromptInstruction, forbidden) {
			t.Fatalf("media instruction kept conflicting template %q", forbidden)
		}
	}
	if len([]rune(mediaPromptInstruction)) > 2500 {
		t.Fatalf("media instruction grew beyond bounded task contract: %d runes", len([]rune(mediaPromptInstruction)))
	}
}
