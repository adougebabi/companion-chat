package migrations

const personaGovernanceSchemaSQL = `
ALTER TABLE public.fluctlight_foundation_revisions ADD COLUMN IF NOT EXISTS base_snapshot jsonb;
ALTER TABLE public.capability_requests ADD COLUMN IF NOT EXISTS revision integer NOT NULL DEFAULT 1 CHECK(revision>0);
CREATE TABLE IF NOT EXISTS public.capability_request_reviews (
 id varchar(128) PRIMARY KEY,
 request_id varchar(128) NOT NULL REFERENCES public.capability_requests(id),
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 revision integer NOT NULL,
 previous_status varchar(32) NOT NULL,
 status varchar(32) NOT NULL,
 note text NOT NULL,
 capability_version varchar(128),
 reviewer_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 before_value jsonb NOT NULL,
 after_value jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(request_id,revision)
);
CREATE INDEX IF NOT EXISTS ix_capability_review_history ON public.capability_request_reviews(request_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS ix_foundation_browse ON public.fluctlight_foundation_revisions(fluctlight_id,created_at DESC,id DESC);
`
