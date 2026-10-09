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
	var action, intentionStatus, intentionID string
	var nextRetry *time.Time
	var retryCount int
	var retryReason *string
	var trigger []byte
	err := q.QueryRow(ctx, `SELECT action_intent,status,trigger,id,next_attempt_at,retry_count,retry_reason FROM public.fluctlight_intentions WHERE fluctlight_id=$1 AND goal_id=$2 AND status NOT IN ('completed','cancelled','expired') AND expiration>$3 ORDER BY updated_at DESC,id DESC LIMIT 1`, owner, goalID, at).Scan(&action, &intentionStatus, &trigger, &intentionID, &nextRetry, &retryCount, &retryReason)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		result["next_step"] = action
		result["intention_id"] = intentionID
		if nextRetry != nil {
			result["next_retry_at"] = formatInstant(*nextRetry)
		}
		if retryCount > 0 {
			result["retry_count"] = retryCount
		}
		if retryReason != nil {
			result["retry_reason"] = *retryReason
		}

		result["stage"] = map[string]string{"candidate": "needs_qualification", "qualified": "ready", "due": "due", "in_progress": "executing", "paused": "waiting"}[intentionStatus]
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
	if err := readGoalClosureExecutionWith(ctx, q, owner, goalID, result); err != nil {
		return nil, err
	}
	// A Goal's terminal or explicit paused state overrides an unfinished read
	// model; a delivered message alone cannot complete a relationship Goal.
	if status != "active" && status != "candidate" {
		result["stage"] = status
	}
	var review, blocked bool
	if err := q.QueryRow(ctx, `SELECT context_review_required,EXISTS(SELECT 1 FROM public.goal_dependencies d JOIN public.fluctlight_goals p ON p.id=d.prerequisite_id WHERE d.goal_id=$1 AND p.status<>'completed') FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2`, goalID, owner).Scan(&review, &blocked); err != nil {
		return nil, err
	}
	if status == "active" && (review || blocked) {
		result["stage"] = "blocked"
		result["blocker"] = map[bool]string{true: "Actor背景或关系已变化，等待复核", false: "等待所有前置目标完成"}[review]
	}
	return result, nil
}

func readGoalClosureExecutionWith(ctx context.Context, q lifeContextQuerier, owner, goalID string, result map[string]any) error {
	var hint []byte
	var stageID *string
	if err := q.QueryRow(ctx, `SELECT execution_hint,current_stage_id FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND id=$2`, owner, goalID).Scan(&hint, &stageID); err != nil {
		return err
	}
	for key, value := range decodeObject(hint) {
		switch key {
		case "blocker", "wait_condition", "next_review_at", "ready_for_settlement", "pending_evaluation_id":
			result[key] = value
		case "next_step":
			if result["next_step"] == nil && stringValue(value) != "" {
				result[key] = value
			}
		case "state":
			if result["stage"] == "awaiting_next_step" && stringValue(value) != "" {
				result["stage"] = value
			}
		}
	}
	if stageID != nil {
		var raw []byte
		err := q.QueryRow(ctx, `SELECT jsonb_build_object('id',id,'purpose',purpose,'strategy',strategy,'status',status,'revision',revision,'criteria_version',criteria_version) FROM public.goal_stages WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3`, *stageID, owner, goalID).Scan(&raw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			result["current_stage"] = decodeObject(raw)
		}
	}
	var commitments []byte
	if err := q.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(v),'[]') FROM (SELECT jsonb_build_object('id',id,'stage_id',stage_id,'expected_result',expected_result,'status',status,'blocker',blocker,'window_start',window_start,'window_end',window_end,'revision',revision) v FROM public.goal_commitments WHERE fluctlight_id=$1 AND goal_id=$2 AND status IN ('active','blocked','paused','expired') ORDER BY updated_at DESC,id DESC LIMIT 4) c`, owner, goalID).Scan(&commitments); err != nil {
		return err
	}
	if list := decodeArray(commitments); len(list) > 0 {
		result["commitments"] = list
	}
	var governance []byte
	if err := q.QueryRow(ctx, `SELECT jsonb_build_object('policy',g.review_policy,'reasons',COALESCE((SELECT jsonb_agg(c.reason_category ORDER BY c.local_date DESC,c.id DESC) FROM (SELECT reason_category,local_date,id FROM public.goal_reviews WHERE goal_id=g.id AND status='succeeded' AND (g.review_policy->>'reset_after' IS NULL OR window_start>=(g.review_policy->>'reset_after')::timestamptz) ORDER BY local_date DESC,id DESC LIMIT 30)c),'[]')) FROM public.fluctlight_goals g WHERE g.fluctlight_id=$1 AND g.id=$2`, owner, goalID).Scan(&governance); err != nil {
		return err
	}
	policyView := decodeObject(governance)
	reasons := decisionServiceRefValues(arrayValue(policyView["reasons"]))
	for _, category := range []string{"missed_opportunity", "ineffective_attempt"} {
		count := 0
		for _, reason := range reasons {
			if reason != category {
				break
			}
			count++
		}
		policyView[category+"_count"] = count
	}
	policy := mapValue(policyView["policy"])
	// Default zero counters convey no current event. Publish governance only
	// after review history or an explicit policy/reset exists; details keep it.
	if len(reasons) > 0 || intValue(policy["missed_opportunity_threshold"]) != 3 || intValue(policy["ineffective_attempt_threshold"]) != 3 || policy["reset_after"] != nil {
		delete(policyView, "reasons")
		result["review_governance"] = policyView
	}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT jsonb_build_object('attempt_id',attempt_id,'status',status,'started_at',started_at,'settled_at',settled_at,'wait_ref',wait_ref,'deadline',deadline,'outcome_id',outcome_id,'error_code',result #>> '{attempt,error_code}') FROM public.fluctlight_intention_attempts WHERE fluctlight_id=$1 AND goal_ref=$2 ORDER BY occurred_at DESC,attempt_id DESC LIMIT 1`, owner, "goal:ctx_"+stableDigest(goalID)).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		result["last_attempt"] = decodeObject(raw)
	}
	err = q.QueryRow(ctx, `SELECT jsonb_build_object('evaluation_id',id,'impact',impact,'criteria_version',criteria_version,'judgments',judgments,'evidence_refs',evidence_refs,'occurred_at',occurred_at) FROM public.goal_evaluations WHERE fluctlight_id=$1 AND goal_id=$2 AND stage_id IS NULL AND commitment_id IS NULL ORDER BY created_at DESC,id DESC LIMIT 1`, owner, goalID).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		result["last_evaluation"] = decodeObject(raw)
	}
	err = q.QueryRow(ctx, `SELECT jsonb_build_object('request_id',id,'status',status,'attempt_count',attempt_count,'error_code',error_code,'available_at',available_at) FROM public.goal_evaluation_requests WHERE fluctlight_id=$1 AND (goal_ids ? $2 OR jsonb_array_length(goal_ids)=0) ORDER BY updated_at DESC,id DESC LIMIT 1`, owner, goalID).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		result["evaluation_request"] = decodeObject(raw)
	}
	// Both linked outcomes and actual dialogue/domain evidence are visible.
	// No Attempt is synthesized for an unbound conversation event.
	err = q.QueryRow(ctx, `SELECT v FROM (
 SELECT jsonb_build_object('outcome_id',id,'kind','outcome','status',status,'capability',capability_name,'error_code',error_code,'occurred_at',occurred_at) v,occurred_at,id::text stable_id FROM public.cognition_action_outcomes WHERE fluctlight_id=$1 AND goal_refs ? $3
 UNION ALL
 SELECT jsonb_build_object('source_id',e.source_id,'source_ref','source:'||e.id,'kind',e.source_kind,'status',l.status,'reason',l.reason,'occurred_at',e.occurred_at),e.occurred_at,e.id::text FROM public.goal_evidence_links l JOIN public.goal_source_events e ON e.id=l.source_event_id WHERE l.fluctlight_id=$1 AND l.goal_id=$2
 ) r ORDER BY occurred_at DESC,stable_id DESC LIMIT 1`, owner, goalID, "goal:ctx_"+stableDigest(goalID)).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		result["last_result"] = decodeObject(raw)
	}
	return nil
}
