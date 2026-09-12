package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// personalityProfileIDs returns the profile identifiers available to runtime
// state and profile-scoped initialization seeds. "default" is a virtual,
// shared profile while initialization has not selected a dominant declared
// profile, including when the Persona declares multiple profiles.
func personalityProfileIDs(corePersona map[string]any) map[string]struct{} {
	result := map[string]struct{}{}
	system := mapValue(corePersona["personality_system"])
	profiles := arrayValue(system["profiles"])
	if strings.TrimSpace(stringValue(system["active_profile_id"])) == "default" {
		result["default"] = struct{}{}
	}
	for _, raw := range profiles {
		if id := strings.TrimSpace(stringValue(mapValue(raw)["id"])); id != "" {
			result[id] = struct{}{}
		}
	}
	return result
}

func normalizeProfileID(value, fallback string) (string, bool) {
	profileID := strings.TrimSpace(value)
	if profileID == "" {
		profileID = strings.TrimSpace(fallback)
	}
	if profileID == "" {
		profileID = "default"
	}
	return profileID, true
}

func (a *App) readPersonalityRuntime(ctx context.Context, fluctlightID, fallback string) (map[string]any, error) {
	var active, previous, reason string
	var revision int
	var switchedAt, cooldownUntil *time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT active_profile_id,COALESCE(previous_profile_id,''),revision,switch_reason,switched_at,cooldown_until FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&active, &previous, &revision, &reason, &switchedAt, &cooldownUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]any{"active_profile_id": firstString(fallback, "default"), "revision": 0}, nil
	}
	if err != nil {
		return nil, err
	}
	result := map[string]any{"active_profile_id": active, "revision": revision, "switch_reason": reason}
	if previous != "" {
		result["previous_profile_id"] = previous
	}
	if switchedAt != nil {
		result["switched_at"] = switchedAt.Format(time.RFC3339Nano)
	}
	if cooldownUntil != nil {
		result["cooldown_until"] = cooldownUntil.Format(time.RFC3339Nano)
	}
	return result, nil
}

const personalityDecisionPlanVersion = "fluctlight.personality-decision-plan.v1"

type personalityDecisionPlan struct {
	SchemaVersion     string     `json:"schema_version"`
	FluctlightID      string     `json:"fluctlight_id"`
	Choice            string     `json:"choice"`
	CurrentProfile    string     `json:"current_profile"`
	TargetProfile     string     `json:"target_profile"`
	PreviousProfile   string     `json:"previous_profile,omitempty"`
	ExpectedRevision  int        `json:"expected_revision"`
	ResultingRevision int        `json:"resulting_revision"`
	RuntimeExists     bool       `json:"runtime_exists"`
	Reason            string     `json:"reason,omitempty"`
	CooldownUntil     *time.Time `json:"cooldown_until,omitempty"`
}

func (a *App) preparePersonalityDecision(ctx context.Context, fluctlightID string, decision map[string]any) (*personalityDecisionPlan, error) {
	if len(decision) == 0 {
		return nil, nil
	}
	choice := firstString(decision["decision"], "keep")
	if choice != "keep" && choice != "switch" {
		return nil, errors.New("personality_decision_invalid")
	}
	// Legacy Fluctlights may predate the personality-runtime row. Their
	// persisted Core Persona still declares the semantic initial profile, so a
	// missing runtime row must fall back to that profile rather than the literal
	// "default". Otherwise a valid LLM decision such as keep(base) is rejected
	// as personality_source_profile_stale before the first chat frame.
	var rawPersona []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&rawPersona); err != nil {
		return nil, err
	}
	corePersona := decodeObject(rawPersona)
	initialProfile := initialPersonalityProfileID(corePersona)
	system := mapValue(corePersona["personality_system"])
	var current, previous string
	var revision int
	var cooldownUntil *time.Time
	runtimeExists := true
	if err := a.DB.Pool().QueryRow(ctx, `SELECT active_profile_id,COALESCE(previous_profile_id,''),revision,cooldown_until FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&current, &previous, &revision, &cooldownUntil); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		runtimeExists = false
		current = initialProfile
	}
	if strings.TrimSpace(current) == "" {
		current = initialProfile
	}
	target := strings.TrimSpace(stringValue(decision["target_profile_id"]))
	from := strings.TrimSpace(stringValue(decision["from_profile_id"]))
	if choice == "switch" && from == "" {
		return nil, errors.New("personality_source_profile_required")
	}
	if choice == "switch" && target == "" {
		return nil, errors.New("personality_target_profile_required")
	}
	if from != "" && from != current {
		// Legacy single-profile personas may not have a personality_system or a
		// runtime row at all. A keep decision from such a persona can still carry
		// a provider-side label such as "base"; there is no declared profile to
		// compare, so it must not be rejected as a stale switch source.
		declaredProfiles := personalityProfileIDs(corePersona)
		hasPersonalitySystem := len(mapValue(corePersona["personality_system"])) > 0
		if choice != "keep" || hasPersonalitySystem || len(declaredProfiles) > 0 {
			return nil, errors.New("personality_source_profile_stale")
		}
		from = current
	}
	if choice == "keep" || target == "" {
		target = current
	}
	if target == "" {
		return nil, errors.New("personality_target_profile_required")
	}
	if cooldownUntil != nil && time.Now().UTC().Before(cooldownUntil.UTC()) && target != current {
		return nil, errors.New("personality_switch_cooldown")
	}
	if target != "default" {
		found := false
		for _, raw := range arrayValue(system["profiles"]) {
			if stringValue(mapValue(raw)["id"]) == target {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("personality_target_profile_not_found")
		}
	}
	triggerID := strings.TrimSpace(stringValue(decision["trigger_id"]))
	if choice == "switch" && triggerID != "" {
		matched := false
		for _, raw := range arrayValue(system["switching"]) {
			if stringValue(mapValue(raw)["id"]) == triggerID {
				matched = true
				break
			}
		}
		if !matched {
			for _, raw := range arrayValue(mapValue(system["switching"])["rules"]) {
				if stringValue(mapValue(raw)["id"]) == triggerID {
					matched = true
					break
				}
			}
		}
		if !matched {
			return nil, errors.New("personality_trigger_not_found")
		}
	}
	if target != current {
		previous = current
	}
	if target != current && triggerID == "" {
		return nil, errors.New("personality_trigger_required")
	}
	reason := strings.TrimSpace(stringValue(decision["reason"]))
	newRevision := revision
	var switchCooldownUntil *time.Time
	if target != current {
		newRevision++
		if seconds, ok := numberFloat(mapValue(system["switching"])["cooldown_seconds"]); ok && seconds > 0 && seconds <= 7*24*60*60 {
			value := time.Now().UTC().Add(time.Duration(seconds * float64(time.Second)))
			switchCooldownUntil = &value
		}
	}
	return &personalityDecisionPlan{
		SchemaVersion: personalityDecisionPlanVersion, FluctlightID: fluctlightID, Choice: choice,
		CurrentProfile: current, TargetProfile: target, PreviousProfile: previous,
		ExpectedRevision: revision, ResultingRevision: newRevision, RuntimeExists: runtimeExists,
		Reason: reason, CooldownUntil: switchCooldownUntil,
	}, nil
}

func personalityDecisionPlanFromValue(value any) (*personalityDecisionPlan, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("personality_decision_plan_invalid")
	}
	var plan personalityDecisionPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, errors.New("personality_decision_plan_invalid")
	}
	if plan.SchemaVersion != personalityDecisionPlanVersion || strings.TrimSpace(plan.FluctlightID) == "" || strings.TrimSpace(plan.CurrentProfile) == "" || strings.TrimSpace(plan.TargetProfile) == "" || plan.ExpectedRevision < 0 || plan.ResultingRevision < plan.ExpectedRevision || plan.ResultingRevision > plan.ExpectedRevision+1 {
		return nil, errors.New("personality_decision_plan_invalid")
	}
	if plan.TargetProfile == plan.CurrentProfile && plan.ResultingRevision != plan.ExpectedRevision {
		return nil, errors.New("personality_decision_plan_invalid")
	}
	if plan.TargetProfile != plan.CurrentProfile && plan.ResultingRevision != plan.ExpectedRevision+1 {
		return nil, errors.New("personality_decision_plan_invalid")
	}
	return &plan, nil
}

func (a *App) applyPersonalityDecisionPlanTx(ctx context.Context, tx pgx.Tx, fluctlightID string, plan *personalityDecisionPlan) (map[string]any, error) {
	if plan == nil {
		return nil, nil
	}
	if plan.FluctlightID != fluctlightID {
		return nil, errors.New("personality_decision_scope_invalid")
	}
	if plan.TargetProfile != plan.CurrentProfile {
		if plan.RuntimeExists {
			commandTag, err := tx.Exec(ctx, `UPDATE public.fluctlight_personality_runtime SET active_profile_id=$2,previous_profile_id=$3,revision=$4,switch_reason=$5,switched_at=now(),cooldown_until=$6,updated_at=now() WHERE fluctlight_id=$1 AND revision=$7 AND active_profile_id=$8`, fluctlightID, plan.TargetProfile, nullableString(plan.PreviousProfile), plan.ResultingRevision, plan.Reason, plan.CooldownUntil, plan.ExpectedRevision, plan.CurrentProfile)
			if err != nil {
				return nil, err
			}
			if commandTag.RowsAffected() != 1 {
				return nil, errors.New("personality_runtime_revision_conflict")
			}
		} else {
			commandTag, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,previous_profile_id,revision,switch_reason,switched_at,cooldown_until,updated_at) VALUES($1,$2,$3,$4,$5,now(),$6,now()) ON CONFLICT DO NOTHING`, fluctlightID, plan.TargetProfile, nullableString(plan.PreviousProfile), plan.ResultingRevision, plan.Reason, plan.CooldownUntil)
			if err != nil {
				return nil, err
			}
			if commandTag.RowsAffected() != 1 {
				return nil, errors.New("personality_runtime_revision_conflict")
			}
		}
	}
	result := map[string]any{"active_profile_id": plan.TargetProfile, "previous_profile_id": plan.PreviousProfile, "revision": plan.ResultingRevision, "switch_reason": plan.Reason}
	if plan.CooldownUntil != nil {
		result["cooldown_until"] = plan.CooldownUntil.Format(time.RFC3339Nano)
	}
	return result, nil
}

func (a *App) applyPersonalityDecision(ctx context.Context, fluctlightID string, decision map[string]any) (map[string]any, error) {
	plan, err := a.preparePersonalityDecision(ctx, fluctlightID, decision)
	if err != nil || plan == nil {
		return nil, err
	}
	var result map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var applyErr error
		result, applyErr = a.applyPersonalityDecisionPlanTx(ctx, tx, fluctlightID, plan)
		return applyErr
	})
	return result, err
}
