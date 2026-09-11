package core

import (
	"math"
	"testing"
	"time"
)

func TestCanonicalAffectReducerPreservesNegativePADAndMomentum(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	current := map[string]any{
		"pad":      map[string]any{"pleasure": 0.0, "arousal": 0.0, "dominance": 0.0},
		"mood":     map[string]any{"label": "平静", "intensity": 0.0},
		"momentum": map[string]any{"value": 0.0}, "regulation": map[string]any{"stability": 1.0},
		"revision": 0, "last_updated_at": now.Format(time.RFC3339Nano),
	}
	result, _, _ := reduceAffectState(current, defaultAffectProfile(), affectReductionInput{
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

func TestCanonicalAffectReducerClampsBipolarAndUnitFieldsSeparately(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	current := map[string]any{
		"pad":  map[string]any{"pleasure": -0.95, "arousal": 0.95, "dominance": 0.0},
		"mood": map[string]any{"intensity": 0.95}, "momentum": map[string]any{"value": -0.95},
		"regulation": map[string]any{"stability": 0.95}, "revision": 3, "last_updated_at": now.Format(time.RFC3339Nano),
	}
	result, _, _ := reduceAffectState(current, defaultAffectProfile(), affectReductionInput{Requested: map[string]float64{
		"pad.pleasure": -10, "pad.arousal": 10, "momentum.value": -10, "mood.intensity": 10, "regulation.stability": 10,
	}}, now)
	if got := numberOrZero(mapValue(result["pad"])["pleasure"]); got < -1 || got > 1 {
		t.Fatalf("pleasure=%v", got)
	}
	if got := numberOrZero(mapValue(result["pad"])["arousal"]); got < -1 || got > 1 {
		t.Fatalf("arousal=%v", got)
	}
	if got := numberOrZero(mapValue(result["momentum"])["value"]); got < -1 || got > 1 {
		t.Fatalf("momentum=%v", got)
	}
	for _, got := range []float64{numberOrZero(mapValue(result["mood"])["intensity"]), numberOrZero(mapValue(result["regulation"])["stability"])} {
		if got < 0 || got > 1 {
			t.Fatalf("unit field=%v", got)
		}
	}
}

func TestAffectElapsedDecayIsPartitionInvariant(t *testing.T) {
	start := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	profile := defaultAffectProfile()
	profile.PADHalfLife = time.Hour
	profile.MomentumHalfLife = time.Hour
	profile.MoodHalfLife = time.Hour
	current := map[string]any{
		"pad":  map[string]any{"pleasure": 0.8, "arousal": -0.6, "dominance": 0.4},
		"mood": map[string]any{"intensity": 0.8}, "momentum": map[string]any{"value": -0.8, "trend": 0.5},
		"regulation": map[string]any{"stability": 1.0},
		"drives":     []any{map[string]any{"key": "social", "pressure": 0.8, "salience": 0.6, "direction": "increase", "source": "built_in"}},
		"conflicts":  []any{map[string]any{"key": "social", "pressure": 0.4, "source": "opposed_semantic_signals"}},
		"revision":   0, "last_updated_at": start.Format(time.RFC3339Nano),
	}
	direct, _, _ := reduceAffectState(current, profile, affectReductionInput{}, start.Add(2*time.Hour))
	first, _, _ := reduceAffectState(current, profile, affectReductionInput{}, start.Add(time.Hour))
	partitioned, _, _ := reduceAffectState(first, profile, affectReductionInput{}, start.Add(2*time.Hour))
	for _, path := range []struct {
		section string
		field   string
	}{{"pad", "pleasure"}, {"pad", "arousal"}, {"pad", "dominance"}, {"momentum", "value"}, {"momentum", "trend"}, {"mood", "intensity"}} {
		left := numberOrZero(mapValue(direct[path.section])[path.field])
		right := numberOrZero(mapValue(partitioned[path.section])[path.field])
		if math.Abs(left-right) > 1e-9 {
			t.Fatalf("%s.%s direct=%v partitioned=%v", path.section, path.field, left, right)
		}
	}
	findPressure := func(value any, key string) float64 {
		for _, raw := range arrayValue(value) {
			entry := mapValue(raw)
			if stringValue(entry["key"]) == key {
				return numberOrZero(entry["pressure"])
			}
		}
		return 0
	}
	for _, values := range []struct {
		name  string
		left  float64
		right float64
	}{
		{name: "drive.social.pressure", left: findPressure(direct["drives"], "social"), right: findPressure(partitioned["drives"], "social")},
		{name: "conflict.social.pressure", left: findPressure(direct["conflicts"], "social"), right: findPressure(partitioned["conflicts"], "social")},
	} {
		if math.Abs(values.left-values.right) > 1e-9 {
			t.Fatalf("%s direct=%v partitioned=%v", values.name, values.left, values.right)
		}
	}
}

func TestMergeEffectiveDriveStateDropsInactiveTypedSlotAndRestoresBuiltin(t *testing.T) {
	current := []any{
		map[string]any{"key": "social", "pressure": 0.9, "salience": 0.8, "direction": "increase", "source": "typed_slot", "slot_id": "inactive-social", "slot_revision": 3},
		map[string]any{"key": "custom_need", "pressure": 0.7, "salience": 0.6, "direction": "increase", "source": "typed_slot", "slot_id": "inactive-custom", "slot_revision": 2},
	}
	result := mergeEffectiveDriveState(current, []map[string]any{})
	var social map[string]any
	for _, raw := range result {
		drive := mapValue(raw)
		if stringValue(drive["key"]) == "custom_need" {
			t.Fatalf("inactive custom Drive remained behaviorally active: %#v", drive)
		}
		if stringValue(drive["key"]) == "social" {
			social = drive
		}
	}
	if stringValue(social["source"]) != "built_in" || numberOrZero(social["pressure"]) != 0 {
		t.Fatalf("inactive typed Drive did not restore builtin baseline: %#v", social)
	}
}

func TestAffectProfileRejectsUnsupportedReducerVersion(t *testing.T) {
	projection := map[string]any{
		"baseline_pad":      map[string]any{"pleasure": 0.0, "arousal": 0.0, "dominance": 0.0},
		"decay_policy":      map[string]any{"pad_half_life_seconds": 3600, "momentum_half_life_seconds": 1800, "mood_half_life_seconds": 2400, "drive_half_life_seconds": 3000},
		"regulation_policy": map[string]any{"strength": 0.1}, "emotional_summary": map[string]any{},
		"revision": 2, "policy_version": "affect.reducer.future",
	}
	if _, err := affectProfileFromProjection(projection); err == nil {
		t.Fatal("unsupported Affect reducer version was executed by the current policy")
	}
}
