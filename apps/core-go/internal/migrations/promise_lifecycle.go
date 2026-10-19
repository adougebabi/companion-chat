package migrations

// A promise is the stable Owner proposal plus links to the existing execution
// authorities. Fulfillment is deliberately not duplicated in this table.
const promiseLifecycleSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.owner_promise_links (
 request_id varchar(128) PRIMARY KEY REFERENCES public.owner_surface_records(id) DEFERRABLE INITIALLY DEFERRED,
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 profile_id varchar(128) NOT NULL,title text NOT NULL,
 window_start timestamptz NOT NULL,window_end timestamptz NOT NULL,timezone varchar(128) NOT NULL,
 proposed_goal text NOT NULL DEFAULT '',proposed_action text NOT NULL DEFAULT '',expected_result text NOT NULL DEFAULT '',
 planning_source_key varchar(256),goal_id varchar(128) REFERENCES public.fluctlight_goals(id),
 commitment_id varchar(128) UNIQUE REFERENCES public.goal_commitments(id),
 intention_id varchar(128) UNIQUE REFERENCES public.fluctlight_intentions(id),
 accepted_at timestamptz,cancelled_at timestamptz,revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(window_end>window_start),CHECK((commitment_id IS NULL)=(intention_id IS NULL))
);
CREATE INDEX IF NOT EXISTS ix_owner_promise_scope ON public.owner_promise_links(owner_actor_id,fluctlight_id,created_at DESC,request_id DESC);
CREATE INDEX IF NOT EXISTS ix_owner_promise_deadline ON public.owner_promise_links(window_end) WHERE commitment_id IS NOT NULL AND cancelled_at IS NULL;
CREATE TABLE IF NOT EXISTS public.owner_promise_history (
 id varchar(128) PRIMARY KEY,request_id varchar(128) NOT NULL REFERENCES public.owner_promise_links(request_id),
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),revision integer NOT NULL,
 operation varchar(32) NOT NULL,source_ref varchar(256) NOT NULL,reason text NOT NULL,
 before_value jsonb,after_value jsonb NOT NULL,occurred_at timestamptz NOT NULL,
 UNIQUE(request_id,revision)
);
CREATE INDEX IF NOT EXISTS ix_owner_promise_history ON public.owner_promise_history(request_id,occurred_at DESC,id DESC);
ALTER TABLE public.goal_commitments ADD COLUMN IF NOT EXISTS commitment_kind varchar(16) NOT NULL DEFAULT 'step';
ALTER TABLE public.goal_commitments ADD COLUMN IF NOT EXISTS owner_promise_id varchar(128) REFERENCES public.owner_surface_records(id) DEFERRABLE INITIALLY DEFERRED;
CREATE UNIQUE INDEX IF NOT EXISTS ux_goal_commitment_owner_promise ON public.goal_commitments(owner_promise_id) WHERE owner_promise_id IS NOT NULL;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.goal_commitments'::regclass AND conname='ck_goal_commitment_promise_kind') THEN
  ALTER TABLE public.goal_commitments ADD CONSTRAINT ck_goal_commitment_promise_kind CHECK((commitment_kind='step' AND owner_promise_id IS NULL) OR (commitment_kind='promise' AND owner_promise_id IS NOT NULL));
 END IF;
END $$;
`
