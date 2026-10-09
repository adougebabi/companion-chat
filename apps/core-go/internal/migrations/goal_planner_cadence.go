package migrations

// Additive cadence policy for the independent GoalPlanner. Every event remains
// durable, while only a bounded primary trigger creates model work. Claim-time
// checks in Core repeat the capacity/trigger fence for runs queued by an older
// binary.
const goalPlannerCadenceSchemaSQL = `
CREATE OR REPLACE FUNCTION public.request_goal_planning(owner_id text,source_id text,why text,data jsonb) RETURNS void LANGUAGE plpgsql AS $$
DECLARE event_seq bigint; run_id text; merge_seconds integer; active_count integer; capacity integer;
DECLARE manual_request boolean; review_trigger boolean; primary_trigger boolean;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || owner_id));
 PERFORM pg_advisory_xact_lock(hashtextextended('life-context:' || owner_id,0));
 INSERT INTO public.goal_set_policies(fluctlight_id) VALUES(owner_id) ON CONFLICT DO NOTHING;
 SELECT merge_window_seconds,max_active_goals INTO merge_seconds,capacity FROM public.goal_set_policies WHERE fluctlight_id=owner_id FOR UPDATE;
 INSERT INTO public.goal_planning_events(fluctlight_id,source_key,reason,payload) VALUES(owner_id,source_id,why,COALESCE(data,'{}'::jsonb)) ON CONFLICT DO NOTHING RETURNING seq INTO event_seq;
 IF event_seq IS NULL THEN RETURN; END IF;

 manual_request := why='owner_request';
 review_trigger := why IN ('actor_context_changed','profile_changed');
 primary_trigger := why IN ('goal_lifecycle_changed','schedule_accepted_daily','initialization_completed','auto_planning_enabled','startup_recovery');
 IF NOT (manual_request OR review_trigger OR primary_trigger) THEN RETURN; END IF;
 SELECT count(*) INTO active_count FROM public.fluctlight_goals WHERE fluctlight_id=owner_id AND status='active';
 IF NOT manual_request AND NOT review_trigger AND active_count>=capacity THEN RETURN; END IF;

 run_id := 'goal_planner_' || md5(owner_id || ':' || event_seq::text);
 INSERT INTO public.goal_planning_runs(id,fluctlight_id,watermark,available_at) VALUES(run_id,owner_id,event_seq,now()+merge_seconds*interval '1 second') ON CONFLICT DO NOTHING;
 IF FOUND THEN
  INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at)
   VALUES(run_id,'goal-planner:'||run_id,'lifecycle','goal.plan',jsonb_build_object('fluctlight_id',owner_id),now()+merge_seconds*interval '1 second') ON CONFLICT DO NOTHING;
 END IF;
END $$;
`
