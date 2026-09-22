package core

import (
	"testing"
)

func TestNormalizeWakeUpAssessmentPreservesOptionalLegacyFields(t *testing.T) {
	value, err := normalizeWakeUpAssessment(map[string]any{
		"appraisal": map[string]any{
			"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.0,
			"social_threat": 0.0, "controllability": 0.5, "responsibility": 0.5,
			"relationship_significance": 0.5, "expected_effect": 0.5,
		},
		"attention":   "我注意到今天的节奏发生了变化",
		"thought":     map[string]any{"summary": "需要重新整理优先级"},
		"desire":      "保持清醒并完成重要的事",
		"agency":      map[string]any{"decision": "先观察"},
		"action_type": "no_op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if value["action_type"] != "no_op" || value["attention"] == nil || value["thought"] == nil {
		t.Fatalf("normalized wake-up = %#v", value)
	}
	if refs, ok := value["evidence_refs"].([]any); !ok || len(refs) != 0 {
		t.Fatalf("evidence refs = %#v", value["evidence_refs"])
	}
}

func TestNormalizeWakeUpAssessmentAcceptsActionOnlyDecision(t *testing.T) {
	value, err := normalizeWakeUpAssessment(map[string]any{
		"action_type":     "moment",
		"response_intent": "发布一条简短动态",
		"evidence_refs":   []any{"life_context.scene"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if value["action_type"] != "moment" || stringValue(value["response_intent"]) == "" {
		t.Fatalf("normalized action-only wake-up = %#v", value)
	}
	if _, ok := value["appraisal"]; ok {
		t.Fatalf("wake-up action decision must not synthesize appraisal: %#v", value)
	}
}

func TestNormalizeWakeUpAssessmentCanonicalizesMomentPublishAlias(t *testing.T) {
	value, err := normalizeWakeUpAssessment(map[string]any{
		"action_type":     "moment_publish",
		"response_intent": "发布一条简短动态",
	})
	if err != nil {
		t.Fatal(err)
	}
	if value["action_type"] != "moment" {
		t.Fatalf("action type = %#v, want moment", value["action_type"])
	}
}

func TestNormalizeWakeUpAssessmentRejectsInvalidAction(t *testing.T) {
	_, err := normalizeWakeUpAssessment(map[string]any{
		"appraisal": map[string]any{
			"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.0,
			"social_threat": 0.0, "controllability": 0.5, "responsibility": 0.5,
			"relationship_significance": 0.5, "expected_effect": 0.5,
		},
		"attention":   "attention",
		"thought":     "thought",
		"desire":      "desire",
		"agency":      "agency",
		"action_type": "send everything",
	})
	if err == nil {
		t.Fatal("invalid wake-up action should be rejected")
	}
}

func TestNormalizeWakeUpSettingsClampsInterval(t *testing.T) {
	settings := normalizeWakeUpSettings(map[string]any{"enabled": false, "interval_seconds": 1})
	if settings.Enabled || settings.IntervalSeconds != minWakeUpIntervalSeconds {
		t.Fatalf("settings = %#v", settings)
	}
	settings = normalizeWakeUpSettings(map[string]any{"interval_seconds": maxWakeUpIntervalSeconds + 1})
	if settings.IntervalSeconds != maxWakeUpIntervalSeconds {
		t.Fatalf("maximum settings = %#v", settings)
	}
}
