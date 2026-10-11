package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

const goalExecutionReconsiderations = 3

func goalExecutionHandoffSettlementEligible(status string) bool {
	return status == "no_op" || status == "completed"
}

// Assessments may update prose, but they cannot erase an accepted execution
// handoff or reset its budget. Terminal Goals no longer need execution.
func preserveGoalExecutionHandoff(next *GoalAuthority, previous map[string]any) {
	if goalStatusTerminal(next.Status) {
		delete(next.ExecutionHint, "execution_handoff")
		return
	}
	if marker := mapValue(previous["execution_handoff"]); len(marker) > 0 {
		next.ExecutionHint["execution_handoff"] = cloneMap(marker)
	}
}

type goalExecutionObligationsKey struct{}

// A handoff is a durable request to reconsider execution, never an action
// extracted from prose. The existing WakeUp Agent owns the semantic decision.
func goalExecutionHandoff(goalID string, hint, previous map[string]any, at time.Time) map[string]any {
	identity := stableDigest(jsonString([]any{goalID, hint["next_step"], hint["wait_condition"], hint["next_review_at"]}))
	prior := mapValue(previous["execution_handoff"])
	if stringValue(prior["plan_id"]) == identity {
		return cloneMap(prior)
	}
	return map[string]any{"plan_id": identity, "status": "pending", "attempts": 0, "created_at": formatInstant(at)}
}

func persistGoalExecutionHandoffTx(ctx context.Context, tx pgx.Tx, next *GoalAuthority, previous map[string]any, at time.Time) error {
	next.ExecutionHint["execution_handoff"] = goalExecutionHandoff(next.EntityID, next.ExecutionHint, previous, at)
	_, err := tx.Exec(ctx, `UPDATE public.fluctlight_goals SET execution_hint=$3 WHERE fluctlight_id=$1 AND id=$2`, next.FluctlightID, next.EntityID, jsonBytes(next.ExecutionHint))
	return err
}

// Recovery is bounded to the visible active set on an existing periodic entry.
// It repairs old text-only plans once; it never resets an exhausted marker.
func (a *App) repairGoalExecutionHandoffs(ctx context.Context, owner string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, owner); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT g.id,g.execution_hint FROM public.fluctlight_goals g
 WHERE g.fluctlight_id=$1 AND g.status='active'
 AND (g.profile_id IS NULL OR g.profile_id='' OR g.profile_id=COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1),'default'))
 AND NOT (g.execution_hint ? 'execution_handoff')
 AND COALESCE(g.execution_hint->>'next_step','')<>''
 ORDER BY g.effective_order,g.id LIMIT 5 FOR UPDATE`, owner)
		if err != nil {
			return err
		}
		goals := []GoalAuthority{}
		for rows.Next() {
			var id string
			var raw []byte
			if err := rows.Scan(&id, &raw); err != nil {
				rows.Close()
				return err
			}
			goals = append(goals, GoalAuthority{EntityID: id, FluctlightID: owner, ExecutionHint: decodeObject(raw)})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for i := range goals {
			if err := persistGoalExecutionHandoffTx(ctx, tx, &goals[i], nil, a.now().UTC()); err != nil {
				return err
			}
		}
		return nil
	})
}

func goalExecutionHandoffDue(hint map[string]any, at time.Time) bool {
	marker := mapValue(hint["execution_handoff"])
	if stringValue(marker["status"]) != "pending" || intValue(marker["attempts"]) >= goalExecutionReconsiderations {
		return false
	}
	for _, value := range []any{marker["not_before"], hint["next_review_at"]} {
		if due, err := time.Parse(time.RFC3339Nano, stringValue(value)); err == nil && at.Before(due) {
			return false
		}
	}
	return true
}

// Open qualified/running/paused intentions already own dispatch/retry. A
// candidate is offered for qualification rather than creating a duplicate.
func (a *App) goalExecutionObligations(ctx context.Context, owner string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT g.id,g.desired_outcome,g.execution_hint,
 CASE WHEN EXISTS(SELECT 1 FROM public.fluctlight_intentions owned WHERE owned.fluctlight_id=g.fluctlight_id AND owned.goal_id=g.id AND owned.status IN ('qualified','due','in_progress','paused') AND owned.expiration>$2) THEN 'owned' ELSE COALESCE((SELECT i.status FROM public.fluctlight_intentions i WHERE i.fluctlight_id=g.fluctlight_id AND i.goal_id=g.id AND i.status NOT IN ('completed','cancelled','expired') AND i.expiration>$2 ORDER BY i.updated_at DESC,i.id DESC LIMIT 1),'') END
 FROM public.fluctlight_goals g WHERE g.fluctlight_id=$1 AND g.status='active' AND NOT g.context_review_required
 AND (g.profile_id IS NULL OR g.profile_id='' OR g.profile_id=COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1),'default'))
 AND NOT EXISTS(SELECT 1 FROM public.goal_dependencies d JOIN public.fluctlight_goals p ON p.id=d.prerequisite_id WHERE d.goal_id=g.id AND (p.status<>'completed' OR p.context_review_required))
 AND NOT (g.deadline_policy='hard' AND g.deadline IS NOT NULL AND g.deadline<=$2)
 ORDER BY g.effective_order,g.id LIMIT 5`, owner, a.now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id, desired, intention string
		var raw []byte
		if err := rows.Scan(&id, &desired, &raw, &intention); err != nil {
			return nil, err
		}
		hint := decodeObject(raw)
		if !goalExecutionNeedsReconsideration(hint, intention, a.now().UTC()) {
			continue
		}
		result = append(result, map[string]any{"_goal_id": id, "goal_ref": "goal:ctx_" + stableDigest(id), "desired_outcome": boundedGoalExecutionText(desired), "next_step": boundedGoalExecutionText(stringValue(hint["next_step"])), "wait_condition": boundedGoalExecutionText(stringValue(hint["wait_condition"])), "plan_id": mapValue(hint["execution_handoff"])["plan_id"], "intention_status": intention, "reconsideration": intValue(mapValue(hint["execution_handoff"])["attempts"]) + 1})
	}
	return result, rows.Err()
}

func boundedGoalExecutionText(text string) string {
	runes := []rune(text)
	if len(runes) > 240 {
		return string(runes[:240])
	}
	return text
}

// Called after normal authority settlement, in the same transaction. Only the
// exact offered plan is consumed; a newer accepted plan is left pending.
func settleGoalExecutionHandoffsTx(ctx context.Context, tx pgx.Tx, owner, wakeID string, obligations []map[string]any, at time.Time) error {
	if len(obligations) == 0 {
		return nil
	}
	if err := lockLifeContextTx(ctx, tx, owner); err != nil {
		return err
	}
	for _, obligation := range obligations {
		_, err := tx.Exec(ctx, `UPDATE public.fluctlight_goals g SET execution_hint=jsonb_set(g.execution_hint,'{execution_handoff}',(g.execution_hint->'execution_handoff') || jsonb_build_object(
 'attempts',COALESCE((g.execution_hint#>>'{execution_handoff,attempts}')::integer,0)+1,
 'last_wake_id',$4::text,'not_before',$5::text,
 'status',CASE WHEN EXISTS(SELECT 1 FROM public.fluctlight_intentions i WHERE i.fluctlight_id=g.fluctlight_id AND i.goal_id=g.id AND i.status IN ('qualified','due','in_progress','paused') AND i.expiration>$6) THEN 'dispatched'
 WHEN COALESCE((g.execution_hint#>>'{execution_handoff,attempts}')::integer,0)+1 >= $7 THEN 'exhausted' ELSE 'pending' END))
 WHERE g.fluctlight_id=$1 AND g.status='active' AND g.id=$2
 AND g.execution_hint#>>'{execution_handoff,plan_id}'=$3
 AND COALESCE(g.execution_hint#>>'{execution_handoff,last_wake_id}','')<>$4`, owner, obligation["_goal_id"], obligation["plan_id"], wakeID, formatInstant(at.Add(wakeUpFirstIdleDelay)), at, goalExecutionReconsiderations)
		if err != nil {
			return err
		}
	}
	return nil
}

func publicGoalExecutionObligations(values []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		v := cloneMap(value)
		delete(v, "_goal_id")
		result = append(result, v)
	}
	return result
}

// Execution Tools resolve the current scoped projection token, not the stable
// domain/audit Goal ref. Only obligations present in that projection are offered
// and eligible for successful-Wake consumption.
func bindGoalExecutionObligations(values []map[string]any, index ContextReferenceIndex) []map[string]any {
	refs := map[string]string{}
	for ref, entry := range index.ByRef {
		if entry.Kind == ContextReferenceGoal {
			refs[entry.EntityID] = ref
		}
	}
	result := []map[string]any{}
	for _, value := range values {
		if ref := refs[stringValue(value["_goal_id"])]; ref != "" {
			bound := cloneMap(value)
			bound["goal_ref"] = ref
			result = append(result, bound)
		}
	}
	return result
}

func goalExecutionNeedsReconsideration(hint map[string]any, intentionStatus string, at time.Time) bool {
	return goalExecutionHandoffDue(hint, at) && (intentionStatus == "" || intentionStatus == "candidate")
}
