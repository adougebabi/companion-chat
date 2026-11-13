package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type wakeUpBackoffDecision struct {
	Idle     bool
	Deadline time.Time
}

func wakeUpIdleBackoffDelay(streak int) time.Duration {
	switch streak {
	case 1:
		return 20 * time.Minute
	case 2:
		return 40 * time.Minute
	default:
		return time.Hour
	}
}

func wakeUpBackoffUntil(payload map[string]any) (time.Time, bool) {
	due, err := time.Parse(time.RFC3339Nano, stringValue(payload["idle_backoff_until"]))
	return due.UTC(), err == nil
}

func resetWakeUpBackoff(payload map[string]any) {
	delete(payload, "idle_noop_streak")
	delete(payload, "idle_backoff_until")
}

func wakeUpBackoffForOutcome(ctx context.Context, registry *CapabilityRegistry, projection ContextProjection, outcome agentCommittedOutcome, status, reason string, now time.Time) wakeUpBackoffDecision {
	decision := wakeUpBackoffDecision{}
	if (status != "no_op" && status != "completed") || reason == "sleeping" {
		return decision
	}
	if obligations, _ := ctx.Value(goalExecutionObligationsKey{}).([]map[string]any); len(obligations) > 0 {
		return decision
	}
	for _, result := range outcome.Results {
		if result.Status != "completed" && result.Status != "accepted" {
			return decision
		}
		definition, found := registry.Definition(result.CapabilityName)
		if found && definition.SideEffectClass == "read_only" {
			continue
		}
		if result.CapabilityName == conversationReplyCapabilityName && mapValue(result.Output)["delivery_status"] == "duplicate_suppressed" {
			continue
		}
		return decision
	}
	decision.Idle = true
	consider := func(value any) {
		due, err := time.Parse(time.RFC3339Nano, stringValue(value))
		if err == nil && due.After(now) && (decision.Deadline.IsZero() || due.Before(decision.Deadline)) {
			decision.Deadline = due.UTC()
		}
	}
	for _, raw := range arrayValue(projection.Schedule["items"]) {
		item := mapValue(raw)
		if item["status"] == "cancelled" || item["status"] == "invalidated" || item["status"] == "skipped" {
			continue
		}
		consider(item["start_at"])
		consider(item["end_at"])
	}
	for _, item := range projection.ActiveActivities {
		consider(item["not_before"])
		consider(item["active_until"])
	}
	return decision
}

// Accepted Goal handoffs retain their own review time even when they are not
// yet offered to the current WakeUp. These are persisted typed times, not a
// model's free-text wait condition.
func capWakeUpBackoffForGoalsTx(ctx context.Context, tx pgx.Tx, owner string, now, due time.Time) (time.Time, error) {
	rows, err := tx.Query(ctx, `
		SELECT g.execution_hint
		FROM public.fluctlight_goals g
		WHERE g.fluctlight_id=$1 AND g.status='active' AND NOT g.context_review_required
		  AND g.execution_hint#>>'{execution_handoff,status}'='pending'
		  AND COALESCE((g.execution_hint#>>'{execution_handoff,attempts}')::int,0)<$3
		  AND (g.profile_id IS NULL OR g.profile_id='' OR g.profile_id=COALESCE(
			(SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1),'default'))
		  AND NOT (g.deadline_policy='hard' AND g.deadline IS NOT NULL AND g.deadline<=$2)`, owner, now, goalExecutionReconsiderations)
	if err != nil {
		return time.Time{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return time.Time{}, err
		}
		hint := decodeObject(raw)
		var review time.Time
		for _, value := range []any{mapValue(hint["execution_handoff"])["not_before"], hint["next_review_at"]} {
			if at, err := time.Parse(time.RFC3339Nano, stringValue(value)); err == nil && at.After(review) {
				review = at
			}
		}
		if !review.After(now) {
			review = now.Add(wakeUpFirstIdleDelay)
		}
		if review.Before(due) {
			due = review.UTC()
		}
	}
	return due, rows.Err()
}
