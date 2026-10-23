package core

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func defaultAutonomyConfiguration() map[string]any {
	return map[string]any{"mode": "active", "allowed_actions": []any{"proactive_message", "moment", "capability"}, "budget_remaining": float64(100), "quiet_hours": map[string]any{}, "concurrency_limit": 1, "cooldown_seconds": 0, "outreach_paused": false, "notifications_muted": false, "confirm_action_types": []any{}}
}

func readGlobalAutonomyConfiguration(ctx context.Context, reader autonomyPolicyReader) (map[string]any, error) {
	var raw []byte
	err := reader.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.autonomy'`).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	result := defaultAutonomyConfiguration()
	for key, value := range decodeObject(raw) {
		result[key] = value
	}
	quiet := mapValue(result["quiet_hours"])
	if len(quiet) > 0 && quiet["timezone"] == nil {
		quiet["timezone"] = "UTC"
	}
	return result, nil
}

// Inherited configuration changes do not replenish a consumed action budget
// when an unrelated global field is saved. Explicit budget edits are grants.
func applyGlobalAutonomyConfigurationTx(ctx context.Context, tx pgx.Tx, patch map[string]any) (map[string]any, error) {
	before, err := readGlobalAutonomyConfiguration(ctx, tx)
	if err != nil {
		return nil, err
	}
	for key := range patch {
		switch key {
		case "mode", "allowed_actions", "budget_remaining", "quiet_hours", "concurrency_limit", "cooldown_seconds", "outreach_paused", "notifications_muted", "confirm_action_types", "topic_suppression_seconds":
		default:
			return nil, invalid("unknown autonomy configuration field: " + key)
		}
	}
	next := cloneMap(before)
	for key, value := range patch {
		next[key] = value
	}
	if err := validAutonomyConfiguration(next); err != nil {
		return nil, err
	}
	budgetChanged := jsonString(before["budget_remaining"]) != jsonString(next["budget_remaining"])
	_, err = tx.Exec(ctx, `UPDATE public.autonomy_policies SET mode=$1,allowed_actions=$2,quiet_hours=$3,concurrency_limit=$4,cooldown_seconds=$5,outreach_paused=$6,notifications_muted=$7,confirm_action_types=$8,budget_remaining=CASE WHEN $9 THEN $10 ELSE budget_remaining END,revision=revision+1,updated_at=now() WHERE policy_source='inherit'`, next["mode"], jsonBytes(next["allowed_actions"]), jsonBytes(next["quiet_hours"]), next["concurrency_limit"], next["cooldown_seconds"], next["outreach_paused"], next["notifications_muted"], jsonBytes(next["confirm_action_types"]), budgetChanged, stringValueOrNumber(next["budget_remaining"]))
	return next, err
}

func validAutonomyConfiguration(value map[string]any) error {
	mode := stringValue(value["mode"])
	if mode != "active" && mode != "paused" && mode != "muted" {
		return invalid("invalid autonomy mode")
	}
	for _, key := range []string{"allowed_actions", "confirm_action_types"} {
		raw, ok := value[key].([]any)
		if !ok {
			return invalid(key + " must be an array")
		}
		seen := map[string]bool{}
		for _, v := range raw {
			name, ok := v.(string)
			if !ok || (name != "proactive_message" && name != "moment" && name != "capability" && name != "media_request") || seen[name] {
				return invalid("invalid " + key)
			}
			seen[name] = true
		}
	}
	for _, key := range []string{"outreach_paused", "notifications_muted"} {
		if _, ok := value[key].(bool); !ok {
			return invalid(key + " must be boolean")
		}
	}
	for key, max := range map[string]int{"concurrency_limit": 32, "cooldown_seconds": 604800} {
		n, ok := intValueExact(value[key])
		if !ok || n < 0 || n > max || (key == "concurrency_limit" && n == 0) {
			return invalid("invalid " + key)
		}
	}
	budget, err := strconv.ParseFloat(stringValueOrNumber(value["budget_remaining"]), 64)
	if err != nil || math.IsNaN(budget) || math.IsInf(budget, 0) || budget < 0 || budget > 1000000 {
		return invalid("invalid action budget")
	}
	quiet, ok := value["quiet_hours"].(map[string]any)
	if !ok {
		return invalid("quiet_hours must be an object")
	}
	if len(quiet) > 0 {
		if len(quiet) != 3 && len(quiet) != 4 {
			return invalid("quiet_hours requires start, end and IANA timezone")
		}
		for key := range quiet {
			if key != "start" && key != "end" && key != "timezone" && key != "days" {
				return invalid("unknown quiet hours field")
			}
		}
		if raw, exists := quiet["days"]; exists {
			days, ok := raw.([]any)
			if !ok || len(days) == 0 || len(days) > 7 {
				return invalid("quiet hours days must contain weekdays 0–6")
			}
			seen := map[int]bool{}
			for _, rawDay := range days {
				day, ok := intValueExact(rawDay)
				if !ok || day < 0 || day > 6 || seen[day] {
					return invalid("quiet hours days must contain unique weekdays 0–6")
				}
				seen[day] = true
			}
		}
		for _, key := range []string{"start", "end"} {
			clock := stringValue(quiet[key])
			if len(clock) != 5 {
				return invalid("invalid quiet hour")
			}
			if _, err := time.Parse("15:04", clock); err != nil {
				return invalid("invalid quiet hour")
			}
		}
		if _, err := time.LoadLocation(stringValue(quiet["timezone"])); err != nil {
			return invalid("invalid quiet hours timezone")
		}
	}
	return nil
}

func stringValueOrNumber(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return jsonString(value)
}

func ensureAutonomyPolicyTx(ctx context.Context, tx pgx.Tx, fid string) error {
	config, err := readGlobalAutonomyConfiguration(ctx, tx)
	if err != nil {
		return err
	}
	// Legacy global quiet hours predate explicit timezone. They retain their
	// original clock interpretation until the Owner saves an explicit timezone.
	_, err = tx.Exec(ctx, `INSERT INTO public.autonomy_policies(fluctlight_id,mode,allowed_actions,budget_remaining,quiet_hours,concurrency_limit,revision,policy_source,outreach_paused,notifications_muted,confirm_action_types,cooldown_seconds) VALUES($1,$2,$3,$4,$5,$6,0,'inherit',$7,$8,$9,$10) ON CONFLICT DO NOTHING`, fid, config["mode"], jsonBytes(config["allowed_actions"]), stringValueOrNumber(config["budget_remaining"]), jsonBytes(config["quiet_hours"]), config["concurrency_limit"], config["outreach_paused"], config["notifications_muted"], jsonBytes(config["confirm_action_types"]), config["cooldown_seconds"])
	return err
}

func readAutonomyPolicyWith(ctx context.Context, reader autonomyPolicyReader, fid string) (map[string]any, error) {
	var raw []byte
	err := reader.QueryRow(ctx, `SELECT to_jsonb(p) FROM public.autonomy_policies p WHERE fluctlight_id=$1`, fid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		v, err := readGlobalAutonomyConfiguration(ctx, reader)
		if err != nil {
			return nil, err
		}
		v["policy_source"], v["revision"], v["fluctlight_id"] = "inherit", 0, fid
		return v, nil
	}
	if err != nil {
		return nil, err
	}
	result := decodeObject(raw)
	n, err := strconv.ParseFloat(stringValue(result["budget_remaining"]), 64)
	if err != nil {
		return nil, errors.New("autonomy_budget_corrupt")
	}
	result["budget_remaining"] = n
	return result, nil
}

func (a *App) OwnerAutonomyPolicy(ctx context.Context, owner, fid string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	result, err := readAutonomyPolicyWith(ctx, a.DB.Pool(), fid)
	if err != nil {
		return nil, err
	}
	result["global_policy"], err = readGlobalAutonomyConfiguration(ctx, a.DB.Pool())
	if err != nil {
		return nil, err
	}
	result["runtime_effect"] = "enforced"
	result["budget_unit"] = "autonomous_run"
	result["confirmation_semantics"] = "requires_explicit_owner_command"
	// These historical monetary preferences are not action-budget grants.
	var legacy []byte
	err = a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='autonomy_policy' AND resource_key='current'`, owner, fid).Scan(&legacy)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	result["legacy_preferences"] = decodeObject(legacy)
	return result, nil
}

func (a *App) UpdateOwnerAutonomyPolicy(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("autonomy_policy_transaction_required")
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok || strings.TrimSpace(stringValue(body["reason"])) == "" {
		return nil, invalid("version and reason required")
	}
	source := stringValue(body["policy_source"])
	if source != "inherit" && source != "override" {
		return nil, invalid("policy_source must be inherit or override")
	}
	for key := range body {
		switch key {
		case "policy_source", "mode", "allowed_actions", "quiet_hours", "budget_remaining", "concurrency_limit", "cooldown_seconds", "outreach_paused", "notifications_muted", "confirm_action_types", "expected_revision", "reason", "idempotency_key":
		default:
			return nil, invalid("unknown autonomy policy field: " + key)
		}
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fid); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "tool-policy:"+fid); err != nil {
		return nil, err
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	if err := ensureAutonomyPolicyTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT revision FROM public.autonomy_policies WHERE fluctlight_id=$1 FOR UPDATE`, fid); err != nil {
		return nil, err
	}
	before, err := readAutonomyPolicyWith(ctx, tx, fid)
	if err != nil {
		return nil, err
	}
	revision := intValue(before["revision"])
	if revision != expected {
		return nil, &SurfaceError{Status: 409, Code: "autonomy_policy_conflict", Message: "自治策略或实际预算已变化，请核对最新值后重新提交", Details: map[string]any{"current_revision": revision}}
	}
	next := cloneMap(before)
	if source == "inherit" {
		next, err = readGlobalAutonomyConfiguration(ctx, tx)
		if err != nil {
			return nil, err
		}
		next["budget_remaining"] = before["budget_remaining"]
	} else {
		for _, key := range []string{"mode", "allowed_actions", "quiet_hours", "concurrency_limit", "cooldown_seconds", "confirm_action_types", "outreach_paused", "notifications_muted"} {
			if value, exists := body[key]; exists {
				next[key] = value
			}
		}
	}
	if value, exists := body["budget_remaining"]; exists {
		next["budget_remaining"] = value
	}
	if err := validAutonomyConfiguration(next); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE public.autonomy_policies SET mode=$2,allowed_actions=$3,budget_remaining=$4,quiet_hours=$5,concurrency_limit=$6,cooldown_seconds=$7,outreach_paused=$8,notifications_muted=$9,confirm_action_types=$10,policy_source=$11,revision=revision+1,updated_at=$12 WHERE fluctlight_id=$1`, fid, next["mode"], jsonBytes(next["allowed_actions"]), stringValueOrNumber(next["budget_remaining"]), jsonBytes(next["quiet_hours"]), next["concurrency_limit"], next["cooldown_seconds"], next["outreach_paused"], next["notifications_muted"], jsonBytes(next["confirm_action_types"]), source, a.now().UTC())
	if err != nil {
		return nil, err
	}
	next["policy_source"], next["revision"], next["fluctlight_id"], next["runtime_effect"] = source, revision+1, fid, "enforced"
	id := randomID("autonomy_policy_")
	_, err = tx.Exec(ctx, `INSERT INTO public.autonomy_policy_history(id,fluctlight_id,actor_id,from_revision,to_revision,reason,before_policy,after_policy) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, fid, owner, revision, revision+1, body["reason"], jsonBytes(before), jsonBytes(next))
	if err != nil {
		return nil, err
	}
	next["audit_id"] = id
	return next, nil
}
