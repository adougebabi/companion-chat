package migrations

// Operational admission only: no Goal/evidence/history rows are rewritten.
// Installed after all released bundles on both upgrade and head repair.
const goalAgentCadenceSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.goal_agent_cadence (
 fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id),
 evaluation_next_at timestamptz NOT NULL DEFAULT '-infinity',
 evaluation_request_id text,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS public.goal_planner_admissions (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 episode_key text NOT NULL,
 run_ref text NOT NULL,
 admitted_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,episode_key)
);
-- Preserve the most recent pre-upgrade cycle as a conservative fence. A head
-- rerun never resets an existing operational clock.
INSERT INTO public.goal_agent_cadence(fluctlight_id,evaluation_next_at)
 SELECT r.fluctlight_id,max(COALESCE(r.claimed_at,r.updated_at))+interval '30 minutes'
 FROM public.goal_evaluation_requests r JOIN public.fluctlights f ON f.id=r.fluctlight_id
 WHERE r.attempt_count>0 GROUP BY r.fluctlight_id ON CONFLICT DO NOTHING;
INSERT INTO public.goal_planner_admissions(fluctlight_id,episode_key,run_ref,admitted_at)
 SELECT r.fluctlight_id,'daily:'||(COALESCE(r.claimed_at,r.created_at) AT TIME ZONE COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai'))::date::text,r.id||'@'||r.claim_revision::text,COALESCE(r.claimed_at,r.created_at)
 FROM public.goal_planning_runs r JOIN public.fluctlights f ON f.id=r.fluctlight_id
 WHERE r.attempt_count>0 ON CONFLICT DO NOTHING;
INSERT INTO public.goal_planner_admissions(fluctlight_id,episode_key,run_ref,admitted_at)
 SELECT r.fluctlight_id,'event:'||e.seq::text,r.id||'@'||r.claim_revision::text,COALESCE(r.claimed_at,r.created_at)
 FROM public.goal_planning_runs r JOIN public.goal_planning_events e ON e.fluctlight_id=r.fluctlight_id AND e.seq<=r.watermark
 WHERE r.attempt_count>0 AND (COALESCE(e.payload->>'profile_id','')='' OR e.payload->>'profile_id'=r.profile_id)
 AND ((e.reason='owner_request' AND e.source_key LIKE 'owner:%') OR (e.reason='goal_lifecycle_changed' AND e.payload->>'status'='completed')) ON CONFLICT DO NOTHING;
CREATE OR REPLACE FUNCTION public.goal_planner_local_day(owner_id text) RETURNS text LANGUAGE sql STABLE AS $$
 SELECT (now() AT TIME ZONE COALESCE(NULLIF(identity->>'timezone',''),'Asia/Shanghai'))::date::text
 FROM public.fluctlights WHERE id=owner_id
$$;
CREATE OR REPLACE FUNCTION public.goal_planner_episode_keys(owner_id text,profile text,watermark bigint)
RETURNS TABLE(episode_key text) LANGUAGE sql STABLE AS $$
 WITH candidates AS (
  SELECT 'daily:'||public.goal_planner_local_day(owner_id) AS k
  WHERE EXISTS(SELECT 1 FROM public.goal_planning_events e JOIN public.fluctlights f ON f.id=e.fluctlight_id
   WHERE e.fluctlight_id=owner_id AND e.processed_run_id IS NULL AND e.seq<=watermark
    AND (COALESCE(e.payload->>'profile_id','')='' OR e.payload->>'profile_id'=profile)
    AND e.reason IN ('schedule_accepted_daily','daily_cadence_due','initialization_completed','auto_planning_enabled','startup_recovery')
    AND COALESCE(NULLIF(e.payload->>'local_date',''),(e.created_at AT TIME ZONE COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai'))::date::text)=public.goal_planner_local_day(owner_id))
  UNION ALL
  SELECT 'event:'||e.seq::text FROM public.goal_planning_events e
  WHERE e.fluctlight_id=owner_id AND e.processed_run_id IS NULL AND e.seq<=watermark
   AND (COALESCE(e.payload->>'profile_id','')='' OR e.payload->>'profile_id'=profile)
   AND ((e.reason='owner_request' AND e.source_key LIKE 'owner:%') OR
    (e.reason='goal_lifecycle_changed' AND e.payload->>'status'='completed'
     AND EXISTS(SELECT 1 FROM public.fluctlight_goals g WHERE g.fluctlight_id=owner_id AND g.id=e.payload->>'goal_id' AND g.status='completed')))
 ) SELECT k FROM candidates WHERE NOT EXISTS(
  SELECT 1 FROM public.goal_planner_admissions a WHERE a.fluctlight_id=owner_id AND a.episode_key=k)
$$;
CREATE OR REPLACE FUNCTION public.request_goal_planning(owner_id text,source_id text,why text,data jsonb) RETURNS void LANGUAGE plpgsql AS $$
DECLARE event_seq bigint; run_id text; merge_seconds integer; capacity integer; profile text;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || owner_id));
 PERFORM pg_advisory_xact_lock(hashtextextended('life-context:' || owner_id,0));
 INSERT INTO public.goal_set_policies(fluctlight_id) VALUES(owner_id) ON CONFLICT DO NOTHING;
 SELECT merge_window_seconds,max_active_goals INTO merge_seconds,capacity FROM public.goal_set_policies WHERE fluctlight_id=owner_id FOR UPDATE;
 INSERT INTO public.goal_planning_events(fluctlight_id,source_key,reason,payload) VALUES(owner_id,source_id,why,COALESCE(data,'{}'::jsonb)) ON CONFLICT DO NOTHING RETURNING seq INTO event_seq;
 IF event_seq IS NULL THEN RETURN; END IF;
 IF (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=owner_id AND status='active')>=capacity THEN RETURN; END IF;
 SELECT COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=owner_id),'default') INTO profile;
 IF COALESCE(data->>'profile_id','')<>'' AND data->>'profile_id'<>profile THEN RETURN; END IF;
 IF NOT EXISTS(SELECT 1 FROM public.goal_planner_episode_keys(owner_id,profile,event_seq)) THEN RETURN; END IF;
 run_id := 'goal_planner_' || md5(owner_id || ':' || event_seq::text);
 INSERT INTO public.goal_planning_runs(id,fluctlight_id,watermark,available_at) VALUES(run_id,owner_id,event_seq,now()+merge_seconds*interval '1 second') ON CONFLICT DO NOTHING;
 IF FOUND THEN
  INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at)
   VALUES(run_id,'goal-planner:'||run_id,'lifecycle','goal.plan',jsonb_build_object('fluctlight_id',owner_id),now()+merge_seconds*interval '1 second') ON CONFLICT DO NOTHING;
 END IF;
END $$;
CREATE OR REPLACE FUNCTION public.goal_planning_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF COALESCE(current_setting('fluctlight.goal_planner_run',true),'')='' AND TG_OP='UPDATE' AND NEW.status<>OLD.status AND NEW.status IN ('completed','paused','cancelled','abandoned') THEN
  -- Retain lifecycle context; only completed events earn an immediate episode.
  PERFORM public.request_goal_planning(NEW.fluctlight_id,'goal:'||NEW.id||':'||NEW.revision::text,'goal_lifecycle_changed',jsonb_build_object('goal_id',NEW.id,'status',NEW.status,'profile_id',COALESCE(NEW.profile_id,'')));
 END IF;
 RETURN NEW;
END $$;
`
