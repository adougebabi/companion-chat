package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const goalEvaluationInterval = 30 * time.Minute

func goalEvaluationQuietWindowBypassed(reason string) bool {
	return reason == "owner_reassess"
}

func goalEvaluationAllowsNoNewActivity(reason string) bool {
	if goalEvaluationQuietWindowBypassed(reason) {
		return true
	}
	return reason == "tool_evaluation" || strings.HasPrefix(reason, "goal_review:") || reason == "assessment_batch_remainder" || reason == "assessment_source_remainder"
}

// Goal lifecycle changes that do not create a goal_source_event still count as
// activity when they queue an evaluation (for example, a completed prerequisite).
func noteGoalEvaluationActivityTx(ctx context.Context, tx pgx.Tx, owner string) error {
	if err := lockLifeContextTx(ctx, tx, owner); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.goal_agent_cadence(fluctlight_id,last_activity_at,activity_revision) VALUES($1,now(),1) ON CONFLICT(fluctlight_id) DO UPDATE SET last_activity_at=GREATEST(COALESCE(goal_agent_cadence.last_activity_at,'-infinity'::timestamptz),now()),activity_revision=goal_agent_cadence.activity_revision+1,updated_at=now()`, owner)
	return err
}

// Called under lifecycle/Life and the request row lock, before either a fresh
// snapshot or frozen retry can become processing. Database time owns the fence.
func admitGoalEvaluationCycleTx(ctx context.Context, tx pgx.Tx, owner, request string, available time.Time, reason string, attempts int) (time.Time, bool, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO public.goal_agent_cadence(fluctlight_id) VALUES($1) ON CONFLICT DO NOTHING`, owner); err != nil {
		return time.Time{}, false, err
	}
	var now, next time.Time
	var lastActivity *time.Time
	var activityRevision, evaluatedRevision int64
	var lastRequest string
	if err := tx.QueryRow(ctx, `SELECT now(),GREATEST(evaluation_next_at,$2::timestamptz),last_activity_at,activity_revision,evaluation_activity_revision,COALESCE(evaluation_request_id,'') FROM public.goal_agent_cadence WHERE fluctlight_id=$1 FOR UPDATE`, owner, available).Scan(&now, &next, &lastActivity, &activityRevision, &evaluatedRevision, &lastRequest); err != nil {
		return time.Time{}, false, err
	}
	newActivity := activityRevision > evaluatedRevision
	sameCycleRetry := attempts > 0 && lastRequest == request
	if !newActivity && !sameCycleRetry && !goalEvaluationAllowsNoNewActivity(reason) {
		return time.Time{}, true, nil
	}
	if !goalEvaluationQuietWindowBypassed(reason) && lastActivity != nil {
		quietUntil := lastActivity.Add(goalEvaluationInterval)
		if quietUntil.After(next) {
			next = quietUntil
		}
	}
	if now.Before(next) {
		_, err := tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET available_at=GREATEST(available_at,$2),updated_at=now() WHERE id=$1`, request, next)
		return next, false, err
	}
	consumeActivity := newActivity && !sameCycleRetry
	_, err := tx.Exec(ctx, `UPDATE public.goal_agent_cadence SET evaluation_next_at=now()+interval '30 minutes',evaluation_request_id=$2,evaluation_activity_revision=CASE WHEN $3 THEN GREATEST(evaluation_activity_revision,activity_revision) ELSE evaluation_activity_revision END,updated_at=now() WHERE fluctlight_id=$1`, owner, request, consumeActivity)
	return time.Time{}, false, err
}

func goalPlannerNoModel(status, reason string) map[string]any {
	return map[string]any{"status": status, "decision": "no_change", "reason": reason, "model_admission": false}
}

// This is the last common entry before the native Agent. A prompt/run reference
// cannot create an Owner request: authority comes from the locked claimed row
// and durable events. Each failed episode stays spent, including frozen retries.
func (a *App) admitGoalPlannerModel(ctx context.Context, owner, actor, profile, runRef, mode string) (map[string]any, error) {
	split := strings.LastIndex(runRef, "@")
	if split < 1 {
		return nil, ErrConflict
	}
	revision, err := strconv.Atoi(runRef[split+1:])
	if err != nil {
		return nil, ErrConflict
	}
	id := runRef[:split]
	var prior map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, owner); err != nil {
			return err
		}
		var valid bool
		var watermark int64
		if err := tx.QueryRow(ctx, `SELECT r.watermark,r.status='processing' AND r.claim_revision=$3 AND r.profile_id=$4 AND r.mode=$5 AND f.created_by_actor_id=$6 AND r.claimed_at>now()-p.lease_seconds*interval '1 second' FROM public.goal_planning_runs r JOIN public.fluctlights f ON f.id=r.fluctlight_id JOIN public.goal_set_policies p ON p.fluctlight_id=f.id WHERE r.id=$1 AND r.fluctlight_id=$2 FOR UPDATE OF r`, id, owner, revision, profile, mode, actor).Scan(&watermark, &valid); err != nil {
			return err
		}
		if !valid {
			return ErrConflict
		}
		var count, capacity int
		if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active'),max_active_goals FROM public.goal_set_policies WHERE fluctlight_id=$1`, owner).Scan(&count, &capacity); err != nil {
			return err
		}
		if count >= capacity {
			prior = goalPlannerNoModel("capacity_full", "Active goal capacity is full; all source events are retained")
			return nil
		}
		// The unique episode ledger has no global watermark: hidden private events
		// remain individually eligible after their profile becomes active.
		tag, err := tx.Exec(ctx, `INSERT INTO public.goal_planner_admissions(fluctlight_id,episode_key,run_ref) SELECT $1,episode_key,$4 FROM public.goal_planner_episode_keys($1,$2,$3) ON CONFLICT DO NOTHING`, owner, profile, watermark, runRef)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			prior = goalPlannerNoModel("awaiting_trigger", "Today's automatic episode and visible completion/Owner events have already been admitted; retained context awaits a new completion, local date or explicit Owner request")
		}
		return nil
	})
	return prior, err
}

// Uses the existing repair lifecycle and workflow intents. Repair creates durable
// work only when due; it does not run models or hold a lease while waiting.
func (a *App) RepairGoalEvaluationCadence(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, ErrInvalidArguments
	}
	repaired := 0
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT f.id FROM public.fluctlights f JOIN public.goal_agent_cadence c ON c.fluctlight_id=f.id WHERE f.status='active' AND c.last_activity_at+interval '30 minutes'<=now() AND c.evaluation_next_at<=now() AND c.activity_revision>c.evaluation_activity_revision AND EXISTS(SELECT 1 FROM public.fluctlight_goals g WHERE g.fluctlight_id=f.id AND g.status IN ('active','paused') AND (g.profile_id IS NULL OR g.profile_id=COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=f.id),'default'))) AND NOT EXISTS(SELECT 1 FROM public.goal_evaluation_requests r WHERE r.fluctlight_id=f.id AND r.status IN ('pending','processing','retry')) ORDER BY f.id LIMIT $1`, limit)
		if err != nil {
			return err
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, owner := range ids {
			if err := lockLifeContextTx(ctx, tx, owner); err != nil {
				return err
			}
			profile, err := (&intentionService{}).resolveProfile(ctx, tx, owner, "")
			if err != nil {
				return err
			}
			var due bool
			var identity string
			if err := tx.QueryRow(ctx, `SELECT c.last_activity_at+interval '30 minutes'<=now() AND c.evaluation_next_at<=now() AND c.activity_revision>c.evaluation_activity_revision AND NOT EXISTS(SELECT 1 FROM public.goal_evaluation_requests r WHERE r.fluctlight_id=f.id AND r.status IN ('pending','processing','retry')),c.activity_revision::text FROM public.fluctlights f JOIN public.goal_agent_cadence c ON c.fluctlight_id=f.id WHERE f.id=$1`, owner).Scan(&due, &identity); err != nil {
				return err
			}
			if !due {
				continue
			}
			id, err := queueGoalEvaluationTx(ctx, tx, owner, profile, "activity_quiet_due", "activity:"+identity, nil)
			if err != nil {
				return err
			}
			if id != "" {
				repaired++
			}
		}
		// Old direct SQL producers and interrupted dispatch retain their original IDs.
		_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at) SELECT r.id,'goal-evaluation:'||r.id,'lifecycle','goal.evaluate',jsonb_build_object('fluctlight_id',r.fluctlight_id,'evaluation_id',r.id,'profile_id',r.profile_id),r.available_at FROM public.goal_evaluation_requests r JOIN public.fluctlights f ON f.id=r.fluctlight_id AND f.status='active' WHERE r.status IN ('pending','retry') AND r.attempt_count<5 ORDER BY r.available_at,r.id LIMIT $1 ON CONFLICT DO NOTHING`, limit)
		return err
	})
	if err != nil {
		return repaired, fmt.Errorf("repair Goal evaluation cadence: %w", err)
	}
	return repaired, nil
}
