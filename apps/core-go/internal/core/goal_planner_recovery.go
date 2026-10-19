package core

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// A single versioned repair trigger per unchanged review snapshot gives the new
// query protocol a fresh bounded run. Old failed/committed runs are never reset.
const goalPlannerReviewRecoveryVersion = "native_query_v2"

const goalPlannerReviewRecoveryCandidatesSQL = `SELECT f.id FROM public.fluctlights f
 JOIN public.goal_set_policies p ON p.fluctlight_id=f.id
 WHERE f.status='active' AND p.auto_planning_enabled
 AND EXISTS(SELECT 1 FROM public.fluctlight_goals g WHERE g.fluctlight_id=f.id AND g.status='active' AND g.context_review_required AND (g.profile_id IS NULL OR g.profile_id=COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=f.id),'default')))
 AND NOT EXISTS(SELECT 1 FROM public.goal_planning_runs r WHERE r.fluctlight_id=f.id AND r.status IN ('pending','processing','retry'))
 ORDER BY f.id LIMIT $1`

func (a *App) repairGoalPlannerContextReviewsTx(ctx context.Context, tx pgx.Tx, limit int) (int, error) {
	rows, err := tx.Query(ctx, goalPlannerReviewRecoveryCandidatesSQL, limit)
	if err != nil {
		return 0, err
	}
	owners := []string{}
	for rows.Next() {
		var owner string
		if err := rows.Scan(&owner); err != nil {
			rows.Close()
			return 0, err
		}
		owners = append(owners, owner)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	repaired := 0
	for _, owner := range owners {
		// Match request_goal_planning's lock order and recheck all admission under it.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, owner); err != nil {
			return repaired, err
		}
		if err := lockLifeContextTx(ctx, tx, owner); err != nil {
			return repaired, err
		}
		policy, err := a.evaluateAutonomyPolicyWithReader(ctx, tx, owner, "capability", a.now(), "", false)
		if err != nil {
			return repaired, err
		}
		if !policy.Allowed {
			continue
		}
		var signature, profile string
		var eligible bool
		// Assessment revisions are bookkeeping, not a new context-review trigger.
		// Include goal semantics and real Actor/profile events, excluding our own
		// recovery event so neither reassessment nor repair resets this budget.
		if err := tx.QueryRow(ctx, `SELECT
   COALESCE((SELECT string_agg(g.id||':'||md5(jsonb_build_array(g.desired_outcome,g.criteria_version,g.target_actor_id,g.deadline,g.deadline_policy,g.motivation,g.scope)::text),',' ORDER BY g.id) FROM public.fluctlight_goals g WHERE g.fluctlight_id=f.id AND g.status='active' AND g.context_review_required AND (g.profile_id IS NULL OR g.profile_id=COALESCE(r.active_profile_id,'default'))),'')
   || ':' || COALESCE((SELECT max(e.seq)::text FROM public.goal_planning_events e WHERE e.fluctlight_id=f.id AND e.reason IN ('actor_context_changed','profile_changed') AND NOT (e.payload ? 'recovery_protocol') AND (COALESCE(e.payload->>'profile_id','')='' OR e.payload->>'profile_id'=COALESCE(r.active_profile_id,'default'))),'0'),
   COALESCE(r.active_profile_id,'default'),
   f.status='active' AND p.auto_planning_enabled
   AND (SELECT count(*) FROM public.fluctlight_goals g WHERE g.fluctlight_id=f.id AND g.status='active')<p.max_active_goals
   AND EXISTS(SELECT 1 FROM public.goal_planner_episode_keys(f.id,COALESCE(r.active_profile_id,'default'),COALESCE((SELECT max(e.seq) FROM public.goal_planning_events e WHERE e.fluctlight_id=f.id),0)))
   AND EXISTS(SELECT 1 FROM public.fluctlight_goals g WHERE g.fluctlight_id=f.id AND g.status='active' AND g.context_review_required AND (g.profile_id IS NULL OR g.profile_id=COALESCE(r.active_profile_id,'default')))
   AND NOT EXISTS(SELECT 1 FROM public.goal_planning_runs x WHERE x.fluctlight_id=f.id AND x.status IN ('pending','processing','retry'))
   AND NOT EXISTS(SELECT 1 FROM public.goal_planning_runs x WHERE x.id=(SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=f.id ORDER BY created_at DESC,id DESC LIMIT 1) AND x.status='failed' AND (COALESCE(x.error_code,x.result->>'reason','')<>'goal_planner_query_results_missing' OR x.result->>'commit_succeeded'='true' OR EXISTS(SELECT 1 FROM public.tool_executions t WHERE t.fluctlight_id=f.id AND t.capability_name='goal_planner.commit' AND t.run_id LIKE x.id||'@%' AND t.result->>'status'='completed')))
   FROM public.fluctlights f JOIN public.goal_set_policies p ON p.fluctlight_id=f.id LEFT JOIN public.fluctlight_personality_runtime r ON r.fluctlight_id=f.id WHERE f.id=$1`, owner).Scan(&signature, &profile, &eligible); err != nil {
			return repaired, err
		}
		if !eligible || signature == ":0" {
			continue
		}
		source := "planner-review-recovery:" + goalPlannerReviewRecoveryVersion + ":" + stableDigest(owner+":"+profile+":"+signature)
		var scheduled bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_planning_events WHERE fluctlight_id=$1 AND source_key=$2 AND payload ? 'recovery_run_id')`, owner, source).Scan(&scheduled); err != nil {
			return repaired, err
		}
		if scheduled {
			continue
		}
		if err := requestGoalPlanningTx(ctx, tx, owner, source, "actor_context_changed", map[string]any{"profile_id": profile, "recovery_protocol": goalPlannerReviewRecoveryVersion}); err != nil {
			return repaired, err
		}
		// Older repair versions could persist this hint without a run. A genuine
		// unspent episode may now schedule it; the hint itself earns no budget.
		// Capture the current watermark, including the real authorizing event.
		if _, err := tx.Exec(ctx, `INSERT INTO public.goal_planning_runs(id,fluctlight_id,watermark,available_at)
 SELECT $2,$1::varchar,COALESCE((SELECT max(seq) FROM public.goal_planning_events WHERE fluctlight_id=$1),0),now()+p.merge_window_seconds*interval '1 second'
 FROM public.goal_set_policies p WHERE p.fluctlight_id=$1
 AND NOT EXISTS(SELECT 1 FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status IN ('pending','processing','retry'))
 AND (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active')<p.max_active_goals
 AND EXISTS(SELECT 1 FROM public.goal_planner_episode_keys($1,$3,COALESCE((SELECT max(seq) FROM public.goal_planning_events WHERE fluctlight_id=$1),0)))
 ON CONFLICT DO NOTHING`, owner, randomID("goal_planner_review_"), profile); err != nil {
			return repaired, err
		}
		// Keep the previous failure's backoff in addition to the configured merge.
		if _, err := tx.Exec(ctx, `UPDATE public.goal_planning_runs n SET available_at=GREATEST(n.available_at,COALESCE((SELECT max(x.available_at) FROM public.goal_planning_runs x WHERE x.fluctlight_id=$1 AND x.status='failed'),n.available_at)) WHERE n.fluctlight_id=$1 AND n.status='pending'`, owner); err != nil {
			return repaired, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at)
 SELECT r.id,'goal-planner:'||r.id,'lifecycle','goal.plan',jsonb_build_object('fluctlight_id',r.fluctlight_id),r.available_at
 FROM public.goal_planning_runs r WHERE r.fluctlight_id=$1 AND r.status='pending'
 ON CONFLICT(intent_id) DO UPDATE SET next_attempt_at=EXCLUDED.next_attempt_at`, owner); err != nil {
			return repaired, err
		}
		// Count only a durably scheduled run and intent. This marker also bounds
		// an unchanged semantic signature after failure without rewriting history.
		tag, err := tx.Exec(ctx, `UPDATE public.goal_planning_events e SET payload=e.payload||jsonb_build_object('recovery_run_id',r.id)
 FROM public.goal_planning_runs r JOIN public.platform_workflow_intents i ON i.intent_id=r.id AND i.intent_type='goal.plan'
 WHERE e.fluctlight_id=$1 AND e.source_key=$2 AND NOT(e.payload ? 'recovery_run_id') AND r.fluctlight_id=e.fluctlight_id AND r.status='pending'`, owner, source)
		if err != nil {
			return repaired, err
		}
		repaired += int(tag.RowsAffected())
	}
	return repaired, nil
}
