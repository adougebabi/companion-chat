package core

import (
	"math"
	"sort"
	"strings"
	"time"
)

const affectReducerPolicyVersion = "affect.reducer.v2"

type AffectProfile struct {
	BaselinePAD        map[string]float64
	PADHalfLife        time.Duration
	MomentumHalfLife   time.Duration
	MoodHalfLife       time.Duration
	DriveHalfLife      time.Duration
	RegulationStrength float64
	Revision           int
	PolicyVersion      string
	EmotionalSummary   map[string]any
}

type affectReductionInput struct {
	Requested map[string]float64
	Drives    []driveSemanticSignal
	Label     string
	Source    string
}

type driveSemanticSignal struct {
	Ref          string
	Key          string
	Direction    string
	Strength     float64
	Confidence   float64
	EvidenceRefs []string
}

var builtInDriveDefinitions = []struct {
	Key         string
	Label       string
	Description string
}{
	{Key: "exploration", Label: "探索", Description: "对新信息、体验与理解的需要"},
	{Key: "rest", Label: "休息", Description: "对恢复、安静与降低负荷的需要"},
	{Key: "social", Label: "联结", Description: "对互动、陪伴与关系联结的需要"},
}

func defaultAffectProfile() AffectProfile {
	return AffectProfile{
		BaselinePAD: map[string]float64{"pleasure": 0, "arousal": 0, "dominance": 0},
		PADHalfLife: 6 * time.Hour, MomentumHalfLife: time.Hour, MoodHalfLife: 2 * time.Hour,
		DriveHalfLife:      4 * time.Hour,
		RegulationStrength: 0, Revision: 0, PolicyVersion: affectReducerPolicyVersion,
		EmotionalSummary: map[string]any{},
	}
}

func clampBipolar(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < -1 {
		return -1
	}
	if value > 1 {
		return 1
	}
	return value
}

func exponentialDecay(value, baseline float64, elapsed, halfLife time.Duration) float64 {
	if elapsed <= 0 || halfLife <= 0 {
		return value
	}
	factor := math.Exp2(-float64(elapsed) / float64(halfLife))
	return baseline + (value-baseline)*factor
}

func mergeEffectiveDriveState(current any, slots []map[string]any) []any {
	byKey := make(map[string]map[string]any)
	activeSlotIDs := make(map[string]struct{}, len(slots))
	if slots != nil {
		for _, slot := range slots {
			if stringValue(slot["value_schema"]) == "pressure" {
				if id := strings.TrimSpace(stringValue(slot["id"])); id != "" {
					activeSlotIDs[id] = struct{}{}
				}
			}
		}
	}
	for _, raw := range arrayValue(current) {
		entry := cloneMap(mapValue(raw))
		key := strings.TrimSpace(stringValue(entry["key"]))
		if key == "" {
			continue
		}
		if slots != nil && stringValue(entry["source"]) == "typed_slot" {
			if _, active := activeSlotIDs[stringValue(entry["slot_id"])]; !active {
				continue
			}
		}
		entry["pressure"] = clampUnit(numberOrZero(entry["pressure"]))
		entry["salience"] = clampUnit(numberOrZero(entry["salience"]))
		byKey[key] = entry
	}
	for _, definition := range builtInDriveDefinitions {
		if _, exists := byKey[definition.Key]; exists {
			continue
		}
		byKey[definition.Key] = map[string]any{
			"key": definition.Key, "label": definition.Label, "description": definition.Description,
			"pressure": 0.0, "salience": 0.0, "direction": "stable", "source": "built_in",
		}
	}
	for _, slot := range slots {
		if stringValue(slot["value_schema"]) != "pressure" {
			continue
		}
		key := strings.TrimSpace(stringValue(slot["key"]))
		value := mapValue(slot["value"])
		if key == "" || len(value) == 0 {
			continue
		}
		revision := intValue(slot["revision"])
		entry, exists := byKey[key]
		if !exists || stringValue(entry["source"]) != "typed_slot" || intValue(entry["slot_revision"]) != revision {
			entry = map[string]any{
				"key": key, "pressure": clampUnit(numberOrZero(value["pressure"])), "salience": clampUnit(numberOrZero(value["salience"])),
				"direction": firstString(value["direction"], "stable"), "source": "typed_slot", "slot_id": slot["id"], "slot_revision": revision,
			}
		}
		entry["label"] = slot["label"]
		entry["description"] = slot["description"]
		entry["confidence"] = slot["confidence"]
		byKey[key] = entry
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]any, 0, len(keys))
	for _, key := range keys {
		result = append(result, byKey[key])
	}
	return result
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func reduceAffectState(current map[string]any, profile AffectProfile, input affectReductionInput, at time.Time) (map[string]any, map[string]any, map[string]any) {
	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}
	result := cloneMap(current)
	pad := cloneMap(mapValue(current["pad"]))
	mood := cloneMap(mapValue(current["mood"]))
	momentum := cloneMap(mapValue(current["momentum"]))
	regulation := cloneMap(mapValue(current["regulation"]))
	if pad == nil {
		pad = map[string]any{}
	}
	if mood == nil {
		mood = map[string]any{}
	}
	if momentum == nil {
		momentum = map[string]any{}
	}
	if regulation == nil {
		regulation = map[string]any{}
	}
	lastUpdated := at
	if raw := stringValue(current["last_updated_at"]); raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil && !parsed.After(at) {
			lastUpdated = parsed
		}
	}
	elapsed := at.Sub(lastUpdated)
	requested := make(map[string]any, len(input.Requested)+5)
	applied := make(map[string]any, len(input.Requested)+5)

	for _, field := range []string{"pleasure", "arousal", "dominance"} {
		before := clampBipolar(numberOrZero(pad[field]))
		baseline := clampBipolar(profile.BaselinePAD[field])
		after := clampBipolar(exponentialDecay(before, baseline, elapsed, profile.PADHalfLife))
		pad[field] = after
		if after != before {
			requested["decay.pad."+field] = after - before
			applied["decay.pad."+field] = after - before
		}
	}
	for key, value := range momentum {
		if number, ok := numberFloat(value); ok && (key == "value" || key == "trend" || strings.HasSuffix(key, "_momentum")) {
			before := clampBipolar(number)
			after := clampBipolar(exponentialDecay(before, 0, elapsed, profile.MomentumHalfLife))
			momentum[key] = after
			if after != before {
				requested["decay.momentum."+key] = after - before
				applied["decay.momentum."+key] = after - before
			}
		}
	}
	beforeIntensity := clampUnit(numberOrZero(mood["intensity"]))
	afterIntensity := clampUnit(exponentialDecay(beforeIntensity, 0, elapsed, profile.MoodHalfLife))
	mood["intensity"] = afterIntensity
	if afterIntensity != beforeIntensity {
		requested["decay.mood.intensity"] = afterIntensity - beforeIntensity
		applied["decay.mood.intensity"] = afterIntensity - beforeIntensity
	}
	for key, value := range regulation {
		if number, ok := numberFloat(value); ok {
			regulation[key] = clampUnit(number)
		}
	}
	regulationScale := 1 - clampUnit(profile.RegulationStrength)*clampUnit(numberOrZero(regulation["stability"]))*0.5

	for path, rawDelta := range input.Requested {
		requested[path] = rawDelta
		parts := strings.SplitN(path, ".", 2)
		if len(parts) != 2 {
			continue
		}
		delta := clampGrowth(rawDelta) * regulationScale
		var target map[string]any
		var clamp func(float64) float64
		switch parts[0] {
		case "pad":
			target, clamp = pad, clampBipolar
		case "momentum":
			target, clamp = momentum, clampBipolar
		case "mood":
			target, clamp = mood, clampUnit
		case "regulation":
			target, clamp = regulation, clampUnit
		default:
			continue
		}
		before := numberOrZero(target[parts[1]])
		after := clamp(before + delta)
		target[parts[1]] = after
		applied[path] = after - before
	}
	drives := mergeEffectiveDriveState(result["drives"], nil)
	driveByKey := make(map[string]map[string]any, len(drives))
	for _, raw := range drives {
		entry := mapValue(raw)
		key := stringValue(entry["key"])
		beforePressure := clampUnit(numberOrZero(entry["pressure"]))
		afterPressure := clampUnit(exponentialDecay(beforePressure, 0, elapsed, profile.DriveHalfLife))
		entry["pressure"] = afterPressure
		beforeSalience := clampUnit(numberOrZero(entry["salience"]))
		afterSalience := clampUnit(exponentialDecay(beforeSalience, 0, elapsed, profile.DriveHalfLife))
		entry["salience"] = afterSalience
		if afterPressure != beforePressure {
			requested["decay.drive."+key+".pressure"] = afterPressure - beforePressure
			applied["decay.drive."+key+".pressure"] = afterPressure - beforePressure
		}
		if afterSalience != beforeSalience {
			requested["decay.drive."+key+".salience"] = afterSalience - beforeSalience
			applied["decay.drive."+key+".salience"] = afterSalience - beforeSalience
		}
		driveByKey[key] = entry
	}
	positive := make(map[string]float64)
	negative := make(map[string]float64)
	for _, signal := range input.Drives {
		entry := driveByKey[signal.Key]
		if entry == nil {
			continue
		}
		magnitude := clampUnit(signal.Strength) * clampUnit(signal.Confidence)
		rawDelta := magnitude
		if signal.Direction == "decrease" {
			rawDelta = -rawDelta
			negative[signal.Key] += magnitude
		} else {
			positive[signal.Key] += magnitude
		}
		path := "drive." + signal.Key + ".pressure"
		if previous, ok := numberFloat(requested[path]); ok {
			requested[path] = previous + rawDelta
		} else {
			requested[path] = rawDelta
		}
		before := clampUnit(numberOrZero(entry["pressure"]))
		after := clampUnit(before + clampGrowth(rawDelta)*regulationScale)
		entry["pressure"] = after
		if previous, ok := numberFloat(applied[path]); ok {
			applied[path] = previous + after - before
		} else {
			applied[path] = after - before
		}
		entry["salience"] = maxFloat(clampUnit(numberOrZero(entry["salience"])), magnitude)
		entry["direction"] = signal.Direction
	}
	conflictByKey := make(map[string]map[string]any)
	for _, raw := range arrayValue(current["conflicts"]) {
		entry := cloneMap(mapValue(raw))
		key := strings.TrimSpace(stringValue(entry["key"]))
		if key == "" {
			continue
		}
		before := clampUnit(numberOrZero(entry["pressure"]))
		after := clampUnit(exponentialDecay(before, 0, elapsed, profile.DriveHalfLife))
		entry["pressure"] = after
		if after != before {
			requested["decay.conflict."+key+".pressure"] = after - before
			applied["decay.conflict."+key+".pressure"] = after - before
		}
		if after > 0 {
			conflictByKey[key] = entry
		}
	}
	for key, increased := range positive {
		if decreased := negative[key]; decreased > 0 {
			conflictByKey[key] = map[string]any{"key": key, "pressure": clampUnit(math.Min(increased, decreased)), "source": "opposed_semantic_signals"}
		}
	}
	conflictKeys := make([]string, 0, len(conflictByKey))
	for key := range conflictByKey {
		conflictKeys = append(conflictKeys, key)
	}
	sort.Strings(conflictKeys)
	conflicts := make([]any, 0, len(conflictKeys))
	for _, key := range conflictKeys {
		conflicts = append(conflicts, conflictByKey[key])
	}
	if label := strings.TrimSpace(input.Label); label != "" {
		mood["label"] = label
	}
	if source := strings.TrimSpace(input.Source); source != "" {
		mood["source"] = source
	}
	result["pad"] = pad
	result["mood"] = mood
	result["momentum"] = momentum
	result["regulation"] = regulation
	result["drives"] = drives
	result["conflicts"] = conflicts
	result["revision"] = int(numberOrZero(current["revision"])) + 1
	result["last_updated_at"] = at.Format(time.RFC3339Nano)
	result["affect_profile_revision"] = profile.Revision
	result["affect_policy_version"] = firstString(profile.PolicyVersion, affectReducerPolicyVersion)
	return result, requested, applied
}

func mergeAffectDeltaMaps(left, right map[string]any) map[string]any {
	result := cloneMap(left)
	if result == nil {
		result = map[string]any{}
	}
	for key, value := range right {
		if rightNumber, ok := numberFloat(value); ok {
			if leftNumber, exists := numberFloat(result[key]); exists {
				result[key] = leftNumber + rightNumber
				continue
			}
		}
		result[key] = value
	}
	return result
}

func projectAffectStateAt(current map[string]any, profile AffectProfile, at time.Time) map[string]any {
	projected, _, _ := reduceAffectState(current, profile, affectReductionInput{}, at)
	projected["revision"] = int(numberOrZero(current["revision"]))
	projected["last_updated_at"] = current["last_updated_at"]
	projected["projected_at"] = at.UTC().Format(time.RFC3339Nano)
	projected["affect_profile_revision"] = profile.Revision
	projected["affect_policy_version"] = firstString(profile.PolicyVersion, affectReducerPolicyVersion)
	return projected
}
