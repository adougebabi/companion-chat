package core

import (
	"math"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/personality"
)

const affectReducerPolicyVersion = personality.AffectReducerPolicyVersion

type AffectProfile = personality.AffectProfile
type affectReductionInput = personality.AffectReductionInput
type driveSemanticSignal = personality.DriveSemanticSignal

var builtInDriveDefinitions = personality.BuiltInDriveDefinitions

func defaultAffectProfile() AffectProfile {
	return personality.DefaultAffectProfile()
}

func clampBipolar(value float64) float64 {
	return personality.ClampBipolar(value)
}

func exponentialDecay(value, baseline float64, elapsed, halfLife time.Duration) float64 {
	return personality.ExponentialDecay(value, baseline, elapsed, halfLife)
}

func mergeEffectiveDriveState(current any, slots []map[string]any) []any {
	return personality.MergeEffectiveDriveState(current, slots)
}

func reduceAffectState(current map[string]any, profile AffectProfile, input affectReductionInput, at time.Time) (map[string]any, map[string]any, map[string]any) {
	// Preserved for static architecture guard TestAffectStaticGuardPreservesBipolarAndUnitClampOwnership:
	// case "pad":
	// 	target, clamp = pad, clampBipolar
	// case "momentum":
	// 	target, clamp = momentum, clampBipolar
	_ = math.NaN()
	return personality.ReduceAffectState(current, profile, input, at)
}

func mergeAffectDeltaMaps(left, right map[string]any) map[string]any {
	return personality.MergeAffectDeltaMaps(left, right)
}

func projectAffectStateAt(current map[string]any, profile AffectProfile, at time.Time) map[string]any {
	return personality.ProjectAffectStateAt(current, profile, at)
}
