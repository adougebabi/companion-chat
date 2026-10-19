package core

import (
	"context"
	"strings"
	"time"
)

// Owner projections contain compiled public persona text and bounded ledger
// metadata only, never assessments, provider payloads, or hidden reasoning.
func (a *App) readOwnerPersonaProjection(ctx context.Context, fid string, foundation map[string]any, revision int, runtime map[string]any) (map[string]any, error) {
	profile := firstString(runtime["active_profile_id"], initialPersonalityProfileID(foundation))
	baseline := personaEvolutionBaseline(fid, mapValue(foundation["personality"]), mapValue(foundation["behavioral_policy"]), mapValue(foundation["personality_system"]), runtime)
	state, err := loadPersonaEvolutionState(ctx, a.DB.Pool(), baseline)
	if err != nil {
		return nil, err
	}
	effective, err := ComposeEffectivePersona(state)
	if err != nil {
		return nil, err
	}
	overlay := portraitOverlayRevision(state)
	compiled, err := loadCompiledWorkingPersonaVersion(ctx, a.DB.Pool(), fid, profile, revision, foundation, map[string]any{"portrait_overlay_revision": overlay, "personality": effective.Personality, "behavioral_policy": effective.BehaviorPolicy})
	result := map[string]any{"profile_id": profile, "foundation_revision": revision, "overlay_revision": overlay, "status": "ready", "working_persona": nil}
	if err != nil {
		switch {
		case err.Error() == "working_persona_missing":
			result["status"] = "missing"
		case strings.HasPrefix(err.Error(), "working_persona_version_mismatch"):
			result["status"] = "stale"
		case err.Error() == "working_persona_payload_invalid":
			result["status"] = "invalid"
		default:
			return nil, err
		}
	} else {
		result["working_persona"] = renderCompiledWorkingPersona(compiled)
		result["rules_version"] = compiled.RulesVersion
		result["source_hash"] = compiled.SourceHash
	}
	targets := []any{}
	seen := map[string]bool{}
	for _, rule := range ownerPersonaSwitchRules(foundation, profile, "") {
		if !seen[rule.TargetProfileID] && rule.TargetProfileID != profile {
			targets = append(targets, map[string]any{"rule_id": rule.RuleID, "target_profile_id": rule.TargetProfileID})
			seen[rule.TargetProfileID] = true
		}
	}
	result["owner_switch_targets"] = targets
	result["owner_switch_status"] = "assessment_required"
	return result, nil
}

func (a *App) readPersonaSwitchHistory(ctx context.Context, fid string) ([]map[string]any, bool, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT aggregate_id,occurred_at,payload FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind='persona.switch.committed' ORDER BY occurred_at DESC,id DESC LIMIT 101`, fid)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&id, &at, &raw); err != nil {
			return nil, false, err
		}
		payload := decodeObject(raw)
		output := mapValue(mapValue(payload["result"])["output"])
		item := map[string]any{"audit_id": id, "occurred_at": at.UTC().Format(time.RFC3339Nano)}
		for _, key := range []string{"active_profile_id", "previous_profile_id", "revision", "switch_reason", "cooldown_until", "disposition"} {
			if value, ok := output[key]; ok {
				item[key] = value
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(items) > 100
	if more {
		items = items[:100]
	}
	return items, more, nil
}
