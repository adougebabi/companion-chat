package core

import "testing"

func TestNormalizeWakeUpAssessmentKeepsInternalStages(t *testing.T) {
	value, err := normalizeWakeUpAssessment(map[string]any{
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

func TestNormalizeWakeUpAssessmentRejectsInvalidAction(t *testing.T) {
	_, err := normalizeWakeUpAssessment(map[string]any{
		"attention":   "attention",
		"thought":     "thought",
		"desire":      "desire",
		"agency":      "agency",
		"action_type": "send_everything",
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
