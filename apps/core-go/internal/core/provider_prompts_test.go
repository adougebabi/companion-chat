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
		{name: "language", value: providerLanguageRule, max: 240, must: []string{"所有自然语言值使用中文", "协议 key", "工具名", "时间戳", "YAML"}},
		{name: "context", value: providerContextAuthorityRule, max: 420, must: []string{"core_persona", "developing_self", "current_state", "context_override.explicit=true"}},
		{name: "wake-up", value: wakeUpAssessmentInstruction, max: 600, must: []string{"attention", "thought", "desire", "agency", "action_type", "media.image.generate", "capability.request", "no_op", "visible text"}},
		{name: "conversation", value: conversationAssessmentInstruction, max: 650, must: []string{"response_plan", "claims", "appraisal", "state_expression", "media_request"}},
		{name: "daily-review", value: dailyReviewInstruction, max: 420, must: []string{"proactive_message", "moment", "no_op", "response_intent"}},
		{name: "reflection", value: reflectionInstruction, max: 650, must: []string{"memory_candidates", "developing_self_candidates", "evidence_refs", "Core Persona"}},
		{name: "native-cognition", value: nativeCognitionInstruction, max: 300, must: []string{"appraisal", "attention", "thought", "desire", "agency"}},
		{name: "realization", value: actionRealizationInstruction, max: 320, must: []string{"core_persona", "developing_self", "current_state", "action_type"}},
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
}
