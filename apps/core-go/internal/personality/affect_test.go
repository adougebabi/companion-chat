package personality

import (
	"math"
	"testing"
	"time"
)

func TestClampBipolar(t *testing.T) {
	if ClampBipolar(-2.0) != -1.0 {
		t.Fatalf("expected -1.0, got %v", ClampBipolar(-2.0))
	}
	if ClampBipolar(2.0) != 1.0 {
		t.Fatalf("expected 1.0, got %v", ClampBipolar(2.0))
	}
	if ClampBipolar(0.5) != 0.5 {
		t.Fatalf("expected 0.5, got %v", ClampBipolar(0.5))
	}
	if ClampBipolar(math.NaN()) != 0.0 {
		t.Fatalf("expected 0.0 for NaN, got %v", ClampBipolar(math.NaN()))
	}
}

func TestClampUnit(t *testing.T) {
	if ClampUnit(-0.5) != 0.0 {
		t.Fatalf("expected 0.0, got %v", ClampUnit(-0.5))
	}
	if ClampUnit(1.5) != 1.0 {
		t.Fatalf("expected 1.0, got %v", ClampUnit(1.5))
	}
	if ClampUnit(0.7) != 0.7 {
		t.Fatalf("expected 0.7, got %v", ClampUnit(0.7))
	}
	if ClampUnit(math.NaN()) != 0.0 {
		t.Fatalf("expected 0.0 for NaN, got %v", ClampUnit(math.NaN()))
	}
}

func TestDefaultAffectProfile(t *testing.T) {
	p := DefaultAffectProfile()
	if p.PolicyVersion != AffectReducerPolicyVersion {
		t.Fatalf("unexpected policy version: %v", p.PolicyVersion)
	}
	if p.BaselinePAD["pleasure"] != 0 {
		t.Fatalf("expected 0 baseline pleasure, got %v", p.BaselinePAD["pleasure"])
	}
}

func TestReduceAffectStatePreservesNegativeBipolar(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	current := map[string]any{
		"pad":      map[string]any{"pleasure": 0.0, "arousal": 0.0, "dominance": 0.0},
		"mood":     map[string]any{"label": "平静", "intensity": 0.0},
		"momentum": map[string]any{"value": 0.0}, "regulation": map[string]any{"stability": 1.0},
		"revision": 0, "last_updated_at": now.Format(time.RFC3339Nano),
	}
	result, _, _ := ReduceAffectState(current, DefaultAffectProfile(), AffectReductionInput{
		Requested: map[string]float64{"pad.pleasure": -0.8, "pad.dominance": -0.6, "momentum.value": -0.7, "mood.intensity": 0.4},
		Label:     "难过", Source: "test",
	}, now)
	pad := mapValue(result["pad"])
	momentum := mapValue(result["momentum"])
	if numberOrZero(pad["pleasure"]) >= 0 || numberOrZero(pad["dominance"]) >= 0 || numberOrZero(momentum["value"]) >= 0 {
		t.Fatalf("negative bipolar state was clamped away: pad=%#v momentum=%#v", pad, momentum)
	}
	if intensity := numberOrZero(mapValue(result["mood"])["intensity"]); intensity < 0 || intensity > 1 {
		t.Fatalf("mood intensity left unit range: %v", intensity)
	}
}

func TestProjectAffectStateAt(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	current := map[string]any{
		"pad":      map[string]any{"pleasure": 0.5, "arousal": 0.5, "dominance": 0.5},
		"mood":     map[string]any{"label": "开心", "intensity": 0.5},
		"momentum": map[string]any{"value": 0.2}, "regulation": map[string]any{"stability": 0.8},
		"revision": 1, "last_updated_at": now.Format(time.RFC3339Nano),
	}
	later := now.Add(2 * time.Hour)
	projected := ProjectAffectStateAt(current, DefaultAffectProfile(), later)
	if projected["projected_at"] == "" {
		t.Fatal("projected_at missing")
	}
	if projected["affect_policy_version"] != AffectReducerPolicyVersion {
		t.Fatalf("unexpected version: %v", projected["affect_policy_version"])
	}
}
