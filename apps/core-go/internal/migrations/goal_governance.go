package migrations

const goalGovernanceSchemaSQL = `
ALTER TABLE public.goal_stages ADD COLUMN IF NOT EXISTS dependency_ids jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(dependency_ids)='array');
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS review_policy jsonb NOT NULL DEFAULT '{"missed_opportunity_threshold":3,"ineffective_attempt_threshold":3}';
CREATE TABLE IF NOT EXISTS public.goal_owner_commands (
 fluctlight_id varchar(128) NOT NULL,
 actor_id varchar(128) NOT NULL,
 idempotency_key varchar(128) NOT NULL,
 request_digest varchar(64) NOT NULL,
 result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,actor_id,idempotency_key)
);
ALTER TABLE public.fluctlight_goal_revisions ADD COLUMN IF NOT EXISTS source varchar(32) NOT NULL DEFAULT 'runtime';
CREATE TABLE IF NOT EXISTS public.goal_reviews (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 goal_id varchar(128) NOT NULL,
 evaluation_request_id varchar(128) REFERENCES public.goal_evaluation_requests(id),
 local_date date NOT NULL,
 timezone text NOT NULL,
 window_start timestamptz NOT NULL,
 window_end timestamptz NOT NULL CHECK(window_end>window_start),
 source_watermark bigint NOT NULL DEFAULT 0,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','succeeded','superseded')),
 reason_category text,
 decision text,
 explanation text,
 next_step text,
 next_review_at timestamptz,
 evidence_refs jsonb NOT NULL DEFAULT '[]',
 strategy_version integer NOT NULL DEFAULT 1,
 revision integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(goal_id,local_date,strategy_version),
 FOREIGN KEY(fluctlight_id,goal_id) REFERENCES public.fluctlight_goals(fluctlight_id,id)
);
CREATE INDEX IF NOT EXISTS ix_goal_reviews_request ON public.goal_reviews(evaluation_request_id) WHERE status='pending';

CREATE OR REPLACE FUNCTION public.requeue_goal_review_source() RETURNS trigger LANGUAGE plpgsql AS $review_source$
DECLARE affected record; request_id text; actual_id text;
BEGIN
 IF NEW.source_kind='goal_revision' THEN RETURN NEW; END IF;
 FOR affected IN
  SELECT r.id,r.goal_id,COALESCE(e.profile_id,g.profile_id,'') profile_id
  FROM public.goal_reviews r JOIN public.fluctlight_goals g ON g.id=r.goal_id
  LEFT JOIN public.goal_evaluation_requests e ON e.id=r.evaluation_request_id
  WHERE r.fluctlight_id=NEW.fluctlight_id AND g.status IN ('active','paused')
   AND r.status IN ('pending','succeeded') AND NEW.occurred_at>=r.window_start AND NEW.occurred_at<r.window_end
   AND (NEW.profile_id IS NULL OR g.profile_id IS NULL OR NEW.profile_id=g.profile_id)
  ORDER BY r.id
 LOOP
  UPDATE public.goal_reviews SET status='pending',revision=revision+1,source_watermark=GREATEST(source_watermark,NEW.id),updated_at=now() WHERE id=affected.id;
  request_id:='goal_review_event_'||NEW.id::text||'_'||md5(affected.profile_id);
  INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,goal_ids,reason,status,available_at)
   VALUES(request_id,NEW.fluctlight_id,affected.profile_id,jsonb_build_array(affected.goal_id),'late_review_source','pending',now()+interval '2 seconds')
   ON CONFLICT(fluctlight_id,profile_id) WHERE status='pending' DO UPDATE SET goal_ids=(SELECT COALESCE(jsonb_agg(v),'[]') FROM (SELECT DISTINCT value v FROM jsonb_array_elements(goal_evaluation_requests.goal_ids||EXCLUDED.goal_ids)) x),updated_at=now()
   RETURNING id INTO actual_id;
  UPDATE public.goal_reviews SET evaluation_request_id=actual_id WHERE id=affected.id;
  INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at)
   VALUES(actual_id,'goal-evaluation:'||actual_id,'lifecycle','goal.evaluate',jsonb_build_object('fluctlight_id',NEW.fluctlight_id,'evaluation_id',actual_id,'profile_id',affected.profile_id),now()+interval '2 seconds')
   ON CONFLICT(intent_id) DO NOTHING;
 END LOOP;
 RETURN NEW;
END;
$review_source$;
DROP TRIGGER IF EXISTS trg_goal_review_source ON public.goal_source_events;
CREATE TRIGGER trg_goal_review_source AFTER INSERT ON public.goal_source_events FOR EACH ROW EXECUTE FUNCTION public.requeue_goal_review_source();

CREATE TABLE IF NOT EXISTS public.goal_review_revisions (
 id bigserial PRIMARY KEY,
 review_id varchar(128) NOT NULL REFERENCES public.goal_reviews(id),
 revision integer NOT NULL,
 snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE OR REPLACE FUNCTION public.snapshot_goal_review() RETURNS trigger LANGUAGE plpgsql AS $review_history$
BEGIN INSERT INTO public.goal_review_revisions(review_id,revision,snapshot) VALUES(NEW.id,NEW.revision,to_jsonb(NEW)); RETURN NEW; END;
$review_history$;
DROP TRIGGER IF EXISTS trg_goal_review_history ON public.goal_reviews;
CREATE TRIGGER trg_goal_review_history AFTER INSERT OR UPDATE ON public.goal_reviews FOR EACH ROW EXECUTE FUNCTION public.snapshot_goal_review();
`
