package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Execution is a read model of the shared Goal/Intention, accepted Schedule,
// and committed outcomes. It does not create a competing progress authority.
func readGoalExecutionStateWith(ctx context.Context, q lifeContextQuerier, owner, goalID, status string, at time.Time) (map[string]any, error) {
	result := map[string]any{"stage": status, "next_step": nil, "last_attempt": nil, "last_result": nil}
	if status == "active" || status == "candidate" {
		result["stage"] = "awaiting_next_step"
	}
	var action, intentionStatus string
	var trigger []byte
	err := q.QueryRow(ctx, `SELECT action_intent,status,trigger FROM public.fluctlight_intentions WHERE fluctlight_id=$1 AND goal_id=$2 AND status NOT IN ('completed','cancelled','expired') AND expiration>$3 ORDER BY updated_at DESC,id DESC LIMIT 1`, owner, goalID, at).Scan(&action, &intentionStatus, &trigger)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		result["next_step"] = action
		result["stage"] = map[string]string{"candidate": "needs_qualification", "qualified": "ready", "due": "due", "running": "executing", "paused": "waiting"}[intentionStatus]
		if result["stage"] == "" {
			result["stage"] = intentionStatus
		}
		if due := stringValue(decodeObject(trigger)["due_at"]); intentionStatus == "qualified" && due != "" {
			result["stage"] = "scheduled"
			result["due_at"] = due
		}
	}
	var id, activityStatus string
	var started time.Time
	var resolved *time.Time
	var raw []byte
	err = q.QueryRow(ctx, `SELECT r.id,r.status,r.started_at,r.resolved_at,r.result_json FROM public.fluctlight_life_activity_runs r JOIN public.fluctlight_intentions i ON i.id=r.intention_id AND i.fluctlight_id=r.fluctlight_id WHERE r.fluctlight_id=$1 AND i.goal_id=$2 ORDER BY r.created_at DESC,r.id DESC LIMIT 1`, owner, goalID).Scan(&id, &activityStatus, &started, &resolved, &raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		result["last_attempt"] = map[string]any{"activity_id": id, "status": activityStatus, "started_at": formatInstant(started)}
		if resolved != nil {
			outcome := mapValue(decodeObject(raw)["result"])
			result["last_result"] = map[string]any{"status": outcome["status"], "reason": outcome["reason"], "event_id": stringValue(decodeObject(raw)["event_id"]), "occurred_at": formatInstant(*resolved)}
		} else if activityStatus == "in_progress" {
			result["stage"] = "executing"
		}
	}
	// A Goal's terminal or explicit paused state overrides an unfinished read
	// model; a delivered message alone cannot complete a relationship Goal.
	if status != "active" && status != "candidate" {
		result["stage"] = status
	}
	return result, nil
}
