package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// personalityProfileIDs returns the profile identifiers declared by the
// persisted Core Persona. A single-profile persona still has the implicit
// "default" profile so legacy rows can safely remain shared (NULL scope).
func personalityProfileIDs(corePersona map[string]any) map[string]struct{} {
	result := map[string]struct{}{"default": {}}
	system := mapValue(corePersona["personality_system"])
	for _, raw := range arrayValue(system["profiles"]) {
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

func (a *App) applyPersonalityDecision(ctx context.Context, fluctlightID string, decision map[string]any) (map[string]any, error) {
	if len(decision) == 0 {
		return nil, nil
	}
	choice := firstString(decision["decision"], "keep")
	if choice != "keep" && choice != "switch" {
		return nil, errors.New("personality_decision_invalid")
	}
	var current, previous string
	var revision int
	var cooldownUntil *time.Time
	if err := a.DB.Pool().QueryRow(ctx, `SELECT active_profile_id,COALESCE(previous_profile_id,''),revision,cooldown_until FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&current, &previous, &revision, &cooldownUntil); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		current = "default"
	}
	target := strings.TrimSpace(stringValue(decision["target_profile_id"]))
	from := strings.TrimSpace(stringValue(decision["from_profile_id"]))
	if from != "" && from != current {
		return nil, errors.New("personality_source_profile_stale")
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
	var rawPersona []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&rawPersona); err != nil {
		return nil, err
	}
	system := mapValue(decodeObject(rawPersona)["personality_system"])
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
	if triggerID := strings.TrimSpace(stringValue(decision["trigger_id"])); triggerID != "" {
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
	reason := strings.TrimSpace(stringValue(decision["reason"]))
	newRevision := revision
	var switchCooldownUntil *time.Time
	if target != current {
		newRevision++
		if seconds, ok := numberFloat(mapValue(system["switching"])["cooldown_seconds"]); ok && seconds > 0 && seconds <= 7*24*60*60 {
			value := time.Now().UTC().Add(time.Duration(seconds * float64(time.Second)))
			switchCooldownUntil = &value
		}
		if _, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,previous_profile_id,revision,switch_reason,switched_at,cooldown_until,updated_at) VALUES($1,$2,$3,$4,$5,now(),$6,now()) ON CONFLICT(fluctlight_id) DO UPDATE SET active_profile_id=excluded.active_profile_id,previous_profile_id=excluded.previous_profile_id,revision=excluded.revision,switch_reason=excluded.switch_reason,switched_at=excluded.switched_at,cooldown_until=excluded.cooldown_until,updated_at=excluded.updated_at`, fluctlightID, target, nullableString(previous), newRevision, reason, switchCooldownUntil); err != nil {
			return nil, err
		}
	}
	result := map[string]any{"active_profile_id": target, "previous_profile_id": previous, "revision": newRevision, "switch_reason": reason}
	if switchCooldownUntil != nil {
		result["cooldown_until"] = switchCooldownUntil.Format(time.RFC3339Nano)
	}
	return result, nil
}
