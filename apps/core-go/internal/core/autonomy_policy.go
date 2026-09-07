package core

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AutonomyPolicyDecision is the Core-owned authorization result used by every
// autonomous entry point. The model may propose semantics, but it cannot
// bypass this policy snapshot.
type AutonomyPolicyDecision struct {
	Allowed  bool
	Reason   string
	Snapshot map[string]any
}

func (a *App) EvaluateAutonomyPolicy(ctx context.Context, fluctlightID, actionType string, now time.Time) (AutonomyPolicyDecision, error) {
	return a.evaluateAutonomyPolicyWithBudget(ctx, fluctlightID, actionType, now, "", false)
}

func (a *App) evaluateAutonomyPolicy(ctx context.Context, fluctlightID, actionType string, now time.Time, excludeActionID string) (AutonomyPolicyDecision, error) {
	return a.evaluateAutonomyPolicyWithBudget(ctx, fluctlightID, actionType, now, excludeActionID, false)
}

func (a *App) evaluateAutonomyPolicyAllowReserved(ctx context.Context, fluctlightID, actionType string, now time.Time, excludeActionID string) (AutonomyPolicyDecision, error) {
	return a.evaluateAutonomyPolicyWithBudget(ctx, fluctlightID, actionType, now, excludeActionID, true)
}

func (a *App) evaluateAutonomyPolicyWithBudget(ctx context.Context, fluctlightID, actionType string, now time.Time, excludeActionID string, budgetReserved bool) (AutonomyPolicyDecision, error) {
	if strings.TrimSpace(actionType) == "" || actionType == "no_op" {
		return AutonomyPolicyDecision{Allowed: true, Snapshot: map[string]any{"mode": "active", "action_type": actionType}}, nil
	}
	var mode, budgetText string
	var allowedRaw, quietRaw []byte
	var cooldown *time.Time
	var concurrency, revision int
	err := a.DB.Pool().QueryRow(ctx, `SELECT mode,allowed_actions,budget_remaining,quiet_hours,cooldown_until,concurrency_limit,revision FROM public.autonomy_policies WHERE fluctlight_id=$1`, fluctlightID).Scan(&mode, &allowedRaw, &budgetText, &quietRaw, &cooldown, &concurrency, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		mode, budgetText, concurrency = "active", "100", 1
		allowedRaw, quietRaw = jsonBytes([]string{"proactive_message", "moment", "media.image.generate", "capability"}), jsonBytes(map[string]any{})
		var settingRaw string
		if settingErr := a.DB.Pool().QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.autonomy'`).Scan(&settingRaw); settingErr == nil {
			var setting map[string]any
			if json.Unmarshal([]byte(settingRaw), &setting) == nil {
				mode = firstString(setting["mode"], mode)
				if rawBudget := setting["budget_remaining"]; rawBudget != nil {
					budgetText = numberString(rawBudget, 100)
				}
				if rawAllowed := arrayValue(setting["allowed_actions"]); len(rawAllowed) > 0 {
					allowedRaw = jsonBytes(rawAllowed)
				}
				if rawQuiet := mapValue(setting["quiet_hours"]); len(rawQuiet) > 0 {
					quietRaw = jsonBytes(rawQuiet)
				}
			}
		}
	} else if err != nil {
		return AutonomyPolicyDecision{}, err
	}
	allowed := decodeArray(allowedRaw)
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, raw := range allowed {
		allowedSet[strings.TrimSpace(stringValue(raw))] = struct{}{}
	}
	if _, ok := allowedSet[actionType]; !ok {
		if actionType == "media_request" {
			if _, ok = allowedSet["media.image.generate"]; !ok {
				return autonomyDenied(mode, "action_not_allowed", mode, allowed, budgetText, quietRaw, cooldown, concurrency, revision), nil
			}
		} else if actionType == "capability" {
			if _, ok = allowedSet["capability"]; !ok {
				return autonomyDenied(mode, "action_not_allowed", mode, allowed, budgetText, quietRaw, cooldown, concurrency, revision), nil
			}
		} else {
			return autonomyDenied(mode, "action_not_allowed", mode, allowed, budgetText, quietRaw, cooldown, concurrency, revision), nil
		}
	}
	if mode != "active" {
		return autonomyDenied(mode, "autonomy_mode_blocked", mode, allowed, budgetText, quietRaw, cooldown, concurrency, revision), nil
	}
	if cooldown != nil && now.Before(cooldown.UTC()) {
		return autonomyDenied(mode, "cooldown_active", mode, allowed, budgetText, quietRaw, cooldown, concurrency, revision), nil
	}
	budget, parseErr := strconv.ParseFloat(strings.TrimSpace(budgetText), 64)
	if !budgetReserved && parseErr == nil && budget <= 0 {
		return autonomyDenied(mode, "budget_exhausted", mode, allowed, budgetText, quietRaw, cooldown, concurrency, revision), nil
	}
	if concurrency < 1 {
		concurrency = 1
	}
	var active int
	var activeErr error
	if excludeActionID == "" {
		activeErr = a.DB.Pool().QueryRow(ctx, `SELECT count(*) FROM public.autonomy_actions WHERE fluctlight_id=$1 AND status IN ('frozen','running')`, fluctlightID).Scan(&active)
	} else {
		activeErr = a.DB.Pool().QueryRow(ctx, `SELECT count(*) FROM public.autonomy_actions WHERE fluctlight_id=$1 AND id<>$2 AND status IN ('frozen','running')`, fluctlightID, excludeActionID).Scan(&active)
	}
	if activeErr != nil {
		return AutonomyPolicyDecision{}, activeErr
	}
	if active >= concurrency {
		return autonomyDenied(mode, "concurrency_limit", mode, allowed, budgetText, quietRaw, cooldown, concurrency, revision), nil
	}
	if quietHoursActive(quietRaw, now) {
		return autonomyDenied(mode, "quiet_hours", mode, allowed, budgetText, quietRaw, cooldown, concurrency, revision), nil
	}
	snapshot := autonomySnapshot(mode, actionType, allowed, budgetText, quietRaw, cooldown, concurrency, revision)
	return AutonomyPolicyDecision{Allowed: true, Snapshot: snapshot}, nil
}

func reserveAutonomyBudgetTx(ctx context.Context, tx pgx.Tx, fluctlightID string) error {
	var budgetText string
	var revision int
	err := tx.QueryRow(ctx, `SELECT budget_remaining,revision FROM public.autonomy_policies WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&budgetText, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		budgetText = "100"
		revision = 0
		var settingRaw string
		if settingErr := tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.autonomy'`).Scan(&settingRaw); settingErr == nil {
			var setting map[string]any
			if json.Unmarshal([]byte(settingRaw), &setting) == nil && setting["budget_remaining"] != nil {
				budgetText = numberString(setting["budget_remaining"], 100)
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_policies(fluctlight_id,mode,allowed_actions,budget_remaining,quiet_hours,concurrency_limit,revision) VALUES($1,'active',$2,$3,'{}',1,0) ON CONFLICT DO NOTHING`, fluctlightID, jsonBytes([]string{"proactive_message", "moment", "media.image.generate", "capability"}), budgetText); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT budget_remaining,revision FROM public.autonomy_policies WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&budgetText, &revision); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	budget, err := strconv.ParseFloat(strings.TrimSpace(budgetText), 64)
	if err != nil || budget <= 0 {
		return errors.New("autonomy_budget_exhausted")
	}
	budget -= 1
	_, err = tx.Exec(ctx, `UPDATE public.autonomy_policies SET budget_remaining=$2,revision=$3,updated_at=now() WHERE fluctlight_id=$1 AND revision=$4`, fluctlightID, strconv.FormatFloat(budget, 'f', -1, 64), revision+1, revision)
	return err
}

func autonomyDenied(mode, reason, snapshotMode string, allowed []any, budget string, quiet []byte, cooldown *time.Time, concurrency, revision int) AutonomyPolicyDecision {
	snapshot := autonomySnapshot(snapshotMode, "", allowed, budget, quiet, cooldown, concurrency, revision)
	snapshot["denied_reason"] = reason
	return AutonomyPolicyDecision{Allowed: false, Reason: reason, Snapshot: snapshot}
}

func autonomySnapshot(mode, actionType string, allowed []any, budget string, quiet []byte, cooldown *time.Time, concurrency, revision int) map[string]any {
	snapshot := map[string]any{"mode": mode, "action_type": actionType, "allowed_actions": allowed, "budget_remaining": budget, "quiet_hours": decodeObject(quiet), "concurrency_limit": concurrency, "revision": revision}
	if cooldown != nil {
		snapshot["cooldown_until"] = cooldown.Format(time.RFC3339Nano)
	}
	return snapshot
}

func quietHoursActive(raw []byte, now time.Time) bool {
	value := decodeObject(raw)
	start, end := stringValue(value["start"]), stringValue(value["end"])
	if start == "" || end == "" {
		return false
	}
	parse := func(value string) (int, bool) {
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return 0, false
		}
		h, hErr := strconv.Atoi(parts[0])
		m, mErr := strconv.Atoi(parts[1])
		if hErr != nil || mErr != nil || h < 0 || h > 23 || m < 0 || m > 59 {
			return 0, false
		}
		return h*60 + m, true
	}
	startMin, startOK := parse(start)
	endMin, endOK := parse(end)
	if !startOK || !endOK {
		return false
	}
	nowMin := now.Hour()*60 + now.Minute()
	if startMin <= endMin {
		return nowMin >= startMin && nowMin < endMin
	}
	return nowMin >= startMin || nowMin < endMin
}
