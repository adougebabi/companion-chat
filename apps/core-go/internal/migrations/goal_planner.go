package migrations

// Additive: no lifecycle/progress/evidence or schedule link is rewritten.
const goalPlannerSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.goal_set_policies (
 fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id),
 max_active_goals integer NOT NULL DEFAULT 5 CHECK(max_active_goals BETWEEN 1 AND 5),
 auto_planning_enabled boolean NOT NULL DEFAULT true,
 ordering_mode text NOT NULL DEFAULT 'automatic' CHECK(ordering_mode IN ('automatic','manual')),
 revision bigint NOT NULL DEFAULT 1, updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE public.goal_set_policies ADD COLUMN IF NOT EXISTS merge_window_seconds integer NOT NULL DEFAULT 2 CHECK(merge_window_seconds BETWEEN 1 AND 60);
ALTER TABLE public.goal_set_policies ADD COLUMN IF NOT EXISTS retry_backoff_seconds integer NOT NULL DEFAULT 60 CHECK(retry_backoff_seconds BETWEEN 5 AND 3600);
ALTER TABLE public.goal_set_policies ADD COLUMN IF NOT EXISTS max_attempts integer NOT NULL DEFAULT 5 CHECK(max_attempts BETWEEN 1 AND 10);
ALTER TABLE public.goal_set_policies ADD COLUMN IF NOT EXISTS lease_seconds integer NOT NULL DEFAULT 300 CHECK(lease_seconds BETWEEN 60 AND 600);
ALTER TABLE public.goal_set_policies ADD COLUMN IF NOT EXISTS candidate_limit integer NOT NULL DEFAULT 20 CHECK(candidate_limit BETWEEN 1 AND 20);
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS effective_order bigint NOT NULL DEFAULT 0;
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS planner_source jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS owner_protected boolean NOT NULL DEFAULT false;
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS context_review_required boolean NOT NULL DEFAULT false;
CREATE TABLE IF NOT EXISTS public.goal_dependencies (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 goal_id varchar(128) NOT NULL REFERENCES public.fluctlight_goals(id),
 prerequisite_id varchar(128) NOT NULL REFERENCES public.fluctlight_goals(id),
 PRIMARY KEY(goal_id,prerequisite_id), CHECK(goal_id<>prerequisite_id)
);
CREATE TABLE IF NOT EXISTS public.goal_initial_source_reviews (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),source_id text NOT NULL,profile_id text,
 raw_source jsonb NOT NULL,status text NOT NULL DEFAULT 'requires_semantic_review',created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,source_id)
);
INSERT INTO public.goal_initial_source_reviews(fluctlight_id,source_id,profile_id,raw_source)
 SELECT f.id,'profile:'||COALESCE(p->>'id','unknown')||':desires',p->>'id',p->'desires'
 FROM public.fluctlights f CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(f.core_persona->'personality_system'->'profiles')='array' THEN f.core_persona->'personality_system'->'profiles' ELSE '[]'::jsonb END) p
 WHERE p ? 'desires' AND p->'desires'<>'[]'::jsonb ON CONFLICT DO NOTHING;
ALTER TABLE public.goal_initial_source_reviews ADD COLUMN IF NOT EXISTS revision integer NOT NULL DEFAULT 1;
ALTER TABLE public.goal_initial_source_reviews ADD COLUMN IF NOT EXISTS decision jsonb NOT NULL DEFAULT '{}';
CREATE TABLE IF NOT EXISTS public.goal_initial_imports (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id), source_id text NOT NULL,
 input_version text NOT NULL, goal_id varchar(128) NOT NULL REFERENCES public.fluctlight_goals(id),
 result text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(fluctlight_id,source_id)
);
CREATE TABLE IF NOT EXISTS public.goal_set_commands (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id), idempotency_key text NOT NULL,
 request_digest text NOT NULL, result jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,idempotency_key)
);
CREATE TABLE IF NOT EXISTS public.goal_planning_events (
 seq bigserial PRIMARY KEY, fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 source_key text NOT NULL, reason text NOT NULL, payload jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,source_key)
);
CREATE TABLE IF NOT EXISTS public.goal_planning_runs (
 id text PRIMARY KEY, fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 watermark bigint NOT NULL, status text NOT NULL DEFAULT 'pending', mode text NOT NULL DEFAULT 'apply',
 claim_revision integer NOT NULL DEFAULT 0, claimed_at timestamptz, attempt_count integer NOT NULL DEFAULT 0,
 available_at timestamptz NOT NULL DEFAULT now()+interval '2 seconds', result jsonb NOT NULL DEFAULT '{}',
 error_code text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE public.goal_planning_events ADD COLUMN IF NOT EXISTS processed_run_id text;
ALTER TABLE public.goal_planning_runs ADD COLUMN IF NOT EXISTS profile_id text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS ix_goal_planning_live ON public.goal_planning_runs(fluctlight_id) WHERE status IN ('pending','processing','retry');
CREATE INDEX IF NOT EXISTS ix_goal_planning_events_owner ON public.goal_planning_events(fluctlight_id,seq);
INSERT INTO public.goal_set_policies(fluctlight_id,auto_planning_enabled)
 SELECT f.id,COALESCE(a.mode,(SELECT value_json::jsonb->>'mode' FROM public.runtime_settings WHERE key='product.autonomy'),'active')='active'
 FROM public.fluctlights f LEFT JOIN public.autonomy_policies a ON a.fluctlight_id=f.id ON CONFLICT DO NOTHING;
WITH ranked AS (SELECT id,row_number() OVER(PARTITION BY fluctlight_id ORDER BY created_at,id) n FROM public.fluctlight_goals)
 UPDATE public.fluctlight_goals g SET effective_order=r.n FROM ranked r WHERE g.id=r.id AND g.effective_order=0;
INSERT INTO public.goal_initial_imports(fluctlight_id,source_id,input_version,goal_id,result)
 SELECT fluctlight_id,id,'legacy-v1',id,'linked_existing' FROM public.fluctlight_goals WHERE id LIKE 'goal_initial_%' ON CONFLICT DO NOTHING;
CREATE OR REPLACE FUNCTION public.guard_goal_set() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE active_count integer; candidate_count integer; capacity integer; candidate_capacity integer;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || NEW.fluctlight_id));
 PERFORM pg_advisory_xact_lock(hashtextextended('life-context:' || NEW.fluctlight_id,0));
 INSERT INTO public.goal_set_policies(fluctlight_id) VALUES(NEW.fluctlight_id) ON CONFLICT DO NOTHING;
 SELECT max_active_goals,candidate_limit INTO capacity,candidate_capacity FROM public.goal_set_policies WHERE fluctlight_id=NEW.fluctlight_id FOR UPDATE;
 IF NEW.status='active' AND (TG_OP='INSERT' OR OLD.status<>'active') THEN
  SELECT count(*) INTO active_count FROM public.fluctlight_goals WHERE fluctlight_id=NEW.fluctlight_id AND status='active' AND id<>NEW.id;
  IF active_count>=capacity THEN RAISE EXCEPTION 'goal_capacity_exceeded' USING ERRCODE='23514'; END IF;
  SELECT COALESCE(max(effective_order),0)+1 INTO NEW.effective_order FROM public.fluctlight_goals WHERE fluctlight_id=NEW.fluctlight_id;
 END IF;
 IF NEW.status='candidate' AND TG_OP='INSERT' THEN
  SELECT count(*) INTO candidate_count FROM public.fluctlight_goals WHERE fluctlight_id=NEW.fluctlight_id AND status='candidate';
  IF candidate_count>=candidate_capacity THEN RAISE EXCEPTION 'goal_candidate_capacity_exceeded' USING ERRCODE='23514'; END IF;
 END IF;
 UPDATE public.goal_set_policies SET revision=revision+1,updated_at=now() WHERE fluctlight_id=NEW.fluctlight_id;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS goal_set_guard ON public.fluctlight_goals;
CREATE TRIGGER goal_set_guard BEFORE INSERT OR UPDATE ON public.fluctlight_goals FOR EACH ROW EXECUTE FUNCTION public.guard_goal_set();
CREATE OR REPLACE FUNCTION public.request_goal_planning(owner_id text,source_id text,why text,data jsonb) RETURNS void LANGUAGE plpgsql AS $$
DECLARE event_seq bigint; run_id text; merge_seconds integer;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || owner_id));
 PERFORM pg_advisory_xact_lock(hashtextextended('life-context:' || owner_id,0));
 SELECT COALESCE((SELECT merge_window_seconds FROM public.goal_set_policies WHERE fluctlight_id=owner_id),2) INTO merge_seconds;
 INSERT INTO public.goal_planning_events(fluctlight_id,source_key,reason,payload) VALUES(owner_id,source_id,why,data) ON CONFLICT DO NOTHING RETURNING seq INTO event_seq;
 IF event_seq IS NULL THEN RETURN; END IF;
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
  PERFORM public.request_goal_planning(NEW.fluctlight_id,'goal:'||NEW.id||':'||NEW.revision::text,'goal_lifecycle_changed',jsonb_build_object('goal_id',NEW.id,'status',NEW.status,'profile_id',COALESCE(NEW.profile_id,'')));
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS goal_planning_change ON public.fluctlight_goals;
CREATE TRIGGER goal_planning_change AFTER UPDATE ON public.fluctlight_goals FOR EACH ROW EXECUTE FUNCTION public.goal_planning_change();
CREATE OR REPLACE FUNCTION public.goal_planning_profile_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' OR NEW.active_profile_id IS DISTINCT FROM OLD.active_profile_id THEN
  PERFORM public.request_goal_planning(NEW.fluctlight_id,'profile:'||NEW.revision::text||':'||NEW.active_profile_id,'profile_changed',jsonb_build_object('profile_id',NEW.active_profile_id));
 END IF;RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS goal_planning_profile_change ON public.fluctlight_personality_runtime;
CREATE TRIGGER goal_planning_profile_change AFTER INSERT OR UPDATE ON public.fluctlight_personality_runtime FOR EACH ROW EXECUTE FUNCTION public.goal_planning_profile_change();
`
