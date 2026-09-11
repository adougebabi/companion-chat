package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type affectProfileQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (a *App) readAffectProfile(ctx context.Context, fluctlightID string) (AffectProfile, map[string]any, error) {
	return readAffectProfileWith(ctx, a.DB.Pool(), fluctlightID)
}

func (a *App) readAffectProfileTx(ctx context.Context, tx pgx.Tx, fluctlightID string) (AffectProfile, map[string]any, error) {
	return readAffectProfileWith(ctx, tx, fluctlightID)
}

func readAffectProfileWith(ctx context.Context, query affectProfileQuerier, fluctlightID string) (AffectProfile, map[string]any, error) {
	var baselineRaw, decayRaw, regulationRaw, summaryRaw, evidenceRaw []byte
	var revision int
	var policyVersion string
	var updatedAt time.Time
	if err := query.QueryRow(ctx, `SELECT baseline_pad,decay_policy,regulation_policy,emotional_summary,evidence_refs,revision,policy_version,updated_at FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1`, fluctlightID).Scan(&baselineRaw, &decayRaw, &regulationRaw, &summaryRaw, &evidenceRaw, &revision, &policyVersion, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AffectProfile{}, nil, errors.New("affect_profile_missing")
		}
		return AffectProfile{}, nil, err
	}
	baseline := decodeObject(baselineRaw)
	decay := decodeObject(decayRaw)
	regulation := decodeObject(regulationRaw)
	summary := decodeObject(summaryRaw)
	projection := map[string]any{
		"baseline_pad": baseline, "decay_policy": decay, "regulation_policy": regulation,
		"emotional_summary": summary, "evidence_refs": decodeArray(evidenceRaw), "revision": revision,
		"policy_version": strings.TrimSpace(policyVersion), "updated_at": updatedAt.Format(time.RFC3339Nano),
	}
	profile, err := affectProfileFromProjection(projection)
	if err != nil {
		return AffectProfile{}, nil, err
	}
	return profile, projection, nil
}

func affectProfileFromProjection(projection map[string]any) (AffectProfile, error) {
	baseline := mapValue(projection["baseline_pad"])
	decay := mapValue(projection["decay_policy"])
	regulation := mapValue(projection["regulation_policy"])
	summary := mapValue(projection["emotional_summary"])
	profile := defaultAffectProfile()
	profile.Revision = intValue(projection["revision"])
	profile.PolicyVersion = strings.TrimSpace(stringValue(projection["policy_version"]))
	if profile.PolicyVersion != affectReducerPolicyVersion {
		return AffectProfile{}, errors.New("affect_profile_policy_invalid")
	}
	for _, key := range []string{"pleasure", "arousal", "dominance"} {
		value, ok := numberFloat(baseline[key])
		if !ok || value < -1 || value > 1 {
			return AffectProfile{}, errors.New("affect_profile_baseline_invalid")
		}
		profile.BaselinePAD[key] = value
	}
	var err error
	if profile.PADHalfLife, err = affectProfileDuration(decay["pad_half_life_seconds"]); err != nil {
		return AffectProfile{}, err
	}
	if profile.MomentumHalfLife, err = affectProfileDuration(decay["momentum_half_life_seconds"]); err != nil {
		return AffectProfile{}, err
	}
	if profile.MoodHalfLife, err = affectProfileDuration(decay["mood_half_life_seconds"]); err != nil {
		return AffectProfile{}, err
	}
	if raw, present := decay["drive_half_life_seconds"]; present {
		if profile.DriveHalfLife, err = affectProfileDuration(raw); err != nil {
			return AffectProfile{}, err
		}
	}
	strength, ok := numberFloat(regulation["strength"])
	if !ok || strength < 0 || strength > 1 {
		return AffectProfile{}, errors.New("affect_profile_regulation_invalid")
	}
	profile.RegulationStrength = strength
	profile.EmotionalSummary = summary
	return profile, nil
}

func affectProfileDuration(value any) (time.Duration, error) {
	seconds, ok := numberFloat(value)
	if !ok || seconds < 60 || seconds > 30*24*60*60 {
		return 0, errors.New("affect_profile_decay_invalid")
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func compactAffectProfile(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	ref := stringValue(value["ref"])
	if ref == "" {
		return nil
	}
	return map[string]any{"ref": ref, "kind": "affect_profile", "authority": "core_numeric_policy"}
}
