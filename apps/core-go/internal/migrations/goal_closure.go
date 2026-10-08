package migrations

const goalClosureSchemaSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS uq_goal_closure_owner_identity ON public.fluctlight_goals(fluctlight_id,id);
ALTER TABLE public.conversation_messages ADD COLUMN IF NOT EXISTS acting_profile_id varchar(128);
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS current_stage_id varchar(128);
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS execution_hint jsonb NOT NULL DEFAULT '{"state":"needs_planning"}';
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS criteria_policy jsonb NOT NULL DEFAULT '{"mode":"all","optional_ids":[]}';
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS stage_id varchar(128);
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS commitment_id varchar(128);
CREATE TABLE IF NOT EXISTS public.goal_stages (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 goal_id varchar(128) NOT NULL,
 profile_id varchar(128),
 purpose text NOT NULL,
 strategy text NOT NULL,
 entry_basis text NOT NULL,
 exit_basis text NOT NULL,
 criteria jsonb NOT NULL CHECK(jsonb_typeof(criteria)='array'),
 criteria_version integer NOT NULL CHECK(criteria_version>0),
 status varchar(32) NOT NULL CHECK(status IN ('active','completed','skipped','paused','cancelled')),
 revision integer NOT NULL CHECK(revision>0),
 reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id,goal_id,id),
 FOREIGN KEY(fluctlight_id,goal_id) REFERENCES public.fluctlight_goals(fluctlight_id,id) DEFERRABLE INITIALLY DEFERRED
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_goal_current_stage ON public.goal_stages(fluctlight_id,goal_id) WHERE status='active';
CREATE TABLE IF NOT EXISTS public.goal_commitments (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 goal_id varchar(128) NOT NULL,
 stage_id varchar(128),
 profile_id varchar(128),
 expected_result text NOT NULL,
 criteria jsonb NOT NULL CHECK(jsonb_typeof(criteria)='array'),
 criteria_version integer NOT NULL CHECK(criteria_version>0),
 window_start timestamptz,
 window_end timestamptz,
 opportunity_condition text NOT NULL,
 blocker text NOT NULL,
 status varchar(32) NOT NULL CHECK(status IN ('active','completed','blocked','paused','expired','cancelled')),
 revision integer NOT NULL CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(window_start IS NULL OR window_end IS NULL OR window_end>window_start),
 UNIQUE(fluctlight_id,goal_id,id),
 FOREIGN KEY(fluctlight_id,goal_id) REFERENCES public.fluctlight_goals(fluctlight_id,id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(fluctlight_id,goal_id,stage_id) REFERENCES public.goal_stages(fluctlight_id,goal_id,id) DEFERRABLE INITIALLY DEFERRED
);
CREATE TABLE IF NOT EXISTS public.goal_source_events (
 id bigserial PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 source_kind varchar(32) NOT NULL CHECK(source_kind IN ('message','outcome','item','activity','actor_fact','goal_revision')),
 source_id varchar(128) NOT NULL,
 source_version varchar(128) NOT NULL,
 profile_id varchar(128),
 conversation_id varchar(128),
 subject_actor_id varchar(128),
 occurred_at timestamptz NOT NULL,
 recorded_at timestamptz NOT NULL DEFAULT now(),
 source_status varchar(32) NOT NULL CHECK(source_status IN ('committed','withdrawn')),
 processed_at timestamptz,
 UNIQUE(fluctlight_id,source_kind,source_id,source_version,source_status)
);
CREATE INDEX IF NOT EXISTS ix_goal_unprocessed_sources ON public.goal_source_events(fluctlight_id,profile_id,id) WHERE processed_at IS NULL;
CREATE TABLE IF NOT EXISTS public.goal_evaluation_requests (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 profile_id varchar(128) NOT NULL DEFAULT '',
 goal_ids jsonb NOT NULL DEFAULT '[]',
 reason text NOT NULL,
 status varchar(32) NOT NULL CHECK(status IN ('pending','processing','succeeded','retry','failed')),
 attempt_count integer NOT NULL DEFAULT 0,
 source_ids jsonb NOT NULL DEFAULT '[]',
 snapshot jsonb NOT NULL DEFAULT '{}',
 result jsonb NOT NULL DEFAULT '{}',
 error_code text,
 available_at timestamptz NOT NULL DEFAULT now(),
 claimed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE public.goal_evaluation_requests ADD COLUMN IF NOT EXISTS claim_revision integer NOT NULL DEFAULT 0;
DROP INDEX IF EXISTS public.uq_goal_pending_evaluation;
CREATE UNIQUE INDEX uq_goal_pending_evaluation ON public.goal_evaluation_requests(fluctlight_id,profile_id) WHERE status='pending';
CREATE UNIQUE INDEX IF NOT EXISTS uq_goal_processing_evaluation ON public.goal_evaluation_requests(fluctlight_id,profile_id) WHERE status='processing';
CREATE TABLE IF NOT EXISTS public.goal_evidence_links (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 goal_id varchar(128) NOT NULL,
 stage_id varchar(128),
 commitment_id varchar(128),
 source_event_id bigint NOT NULL REFERENCES public.goal_source_events(id),
 status varchar(32) NOT NULL CHECK(status IN ('candidate','confirmed','rejected','withdrawn')),
 reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(goal_id,source_event_id),
 FOREIGN KEY(fluctlight_id,goal_id) REFERENCES public.fluctlight_goals(fluctlight_id,id) DEFERRABLE INITIALLY DEFERRED
);
CREATE TABLE IF NOT EXISTS public.goal_evaluations (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 goal_id varchar(128) NOT NULL,
 stage_id varchar(128),
 commitment_id varchar(128),
 request_id varchar(128),
 goal_revision integer NOT NULL,
 criteria_version integer NOT NULL,
 strategy_version varchar(64) NOT NULL,
 impact varchar(32) NOT NULL CHECK(impact IN ('progressed','no_change','blocked','regressed','needs_evidence','completed')),
 judgments jsonb NOT NULL,
 evidence_refs jsonb NOT NULL,
 blocker text NOT NULL,
 wait_condition text NOT NULL,
 next_step text NOT NULL,
 occurred_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(goal_id,id),
 FOREIGN KEY(fluctlight_id,goal_id) REFERENCES public.fluctlight_goals(fluctlight_id,id) DEFERRABLE INITIALLY DEFERRED
);
CREATE TABLE IF NOT EXISTS public.goal_resolutions (
 goal_id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 status varchar(32) NOT NULL,
 criteria_version integer NOT NULL,
 evidence_refs jsonb NOT NULL,
 residual_motivation text NOT NULL,
 disposition text NOT NULL,
 followup_goal_id varchar(128),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS public.goal_evidence_review_flags (
 goal_id varchar(128) NOT NULL,
 source_event_id bigint NOT NULL REFERENCES public.goal_source_events(id),
 reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(goal_id,source_event_id)
);
ALTER TABLE public.fluctlight_intentions DROP CONSTRAINT IF EXISTS fk_intention_goal_stage;
ALTER TABLE public.fluctlight_intentions ADD CONSTRAINT fk_intention_goal_stage FOREIGN KEY(fluctlight_id,goal_id,stage_id) REFERENCES public.goal_stages(fluctlight_id,goal_id,id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE public.fluctlight_intentions DROP CONSTRAINT IF EXISTS fk_intention_goal_commitment;
ALTER TABLE public.fluctlight_intentions ADD CONSTRAINT fk_intention_goal_commitment FOREIGN KEY(fluctlight_id,goal_id,commitment_id) REFERENCES public.goal_commitments(fluctlight_id,goal_id,id) DEFERRABLE INITIALLY DEFERRED;
`

const goalObjectHistorySchemaSQL = `
ALTER TABLE public.goal_stages ADD COLUMN IF NOT EXISTS held_by_goal boolean NOT NULL DEFAULT false;
ALTER TABLE public.goal_commitments ADD COLUMN IF NOT EXISTS held_by_goal boolean NOT NULL DEFAULT false;
CREATE TABLE IF NOT EXISTS public.goal_object_revisions (
 object_kind varchar(16) NOT NULL,
 object_id varchar(128) NOT NULL,
 fluctlight_id varchar(128) NOT NULL,
 goal_id varchar(128) NOT NULL,
 revision integer NOT NULL,
 snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(object_kind,object_id,revision)
);
CREATE OR REPLACE FUNCTION public.record_goal_object_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO public.goal_object_revisions(object_kind,object_id,fluctlight_id,goal_id,revision,snapshot)
  VALUES(TG_ARGV[0],NEW.id,NEW.fluctlight_id,NEW.goal_id,NEW.revision,to_jsonb(NEW)) ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS goal_stage_revision ON public.goal_stages;
CREATE TRIGGER goal_stage_revision AFTER INSERT OR UPDATE ON public.goal_stages FOR EACH ROW EXECUTE FUNCTION public.record_goal_object_revision('stage');
DROP TRIGGER IF EXISTS goal_commitment_revision ON public.goal_commitments;
CREATE TRIGGER goal_commitment_revision AFTER INSERT OR UPDATE ON public.goal_commitments FOR EACH ROW EXECUTE FUNCTION public.record_goal_object_revision('commitment');
`
