package core

import "testing"

func TestNormalizeAppraisalRejectsRawOutOfRangeValues(t *testing.T) {
	_, err := normalizeAppraisal(map[string]any{"relevance": 1.5})
	if err == nil {
		t.Fatal("out-of-range appraisal should be rejected")
	}
}

func TestReduceInternalDynamicsUsesBoundedCoreDelta(t *testing.T) {
	state := map[string]any{
		"pad":        map[string]any{"pleasure": 0.5, "arousal": 0.5, "dominance": 0.5},
		"mood":       map[string]any{"intensity": 0.5},
		"momentum":   map[string]any{"value": 0.5},
		"regulation": map[string]any{"stability": 0.5},
		"revision":   4,
	}
	appraisal := map[string]any{"reward": 1.0, "loss": 0.0, "social_threat": 0.0, "controllability": 1.0, "expected_effect": 1.0}
	result, _, applied := reduceInternalDynamics(state, appraisal)
	if result["revision"] != 5 {
		t.Fatalf("revision = %#v", result["revision"])
	}
	if got := applied["pad.pleasure"]; got != 0.1 {
		t.Fatalf("pleasure delta = %#v, want 0.1", got)
	}
	pad := mapValue(result["pad"])
	if got := pad["pleasure"]; got != 0.6 {
		t.Fatalf("pleasure = %#v, want 0.6", got)
	}
}

func TestValidateTypedSlotValueSupportsArbitraryPreferenceSchemas(t *testing.T) {
	if err := validateTypedSlotValue("categorical", map[string]any{"selected": "quiet_evening"}); err != nil {
		t.Fatal(err)
	}
	if err := validateTypedSlotValue("set", []any{"tea", "coffee"}); err != nil {
		t.Fatal(err)
	}
	if err := validateTypedSlotValue("pressure", map[string]any{"pressure": 0.8, "salience": 0.6, "direction": "toward_completion"}); err != nil {
		t.Fatal(err)
	}
}
