package migrations

const visualBriefSchemaSQL = `
ALTER TABLE public.fluctlight_visual_identity_attempts ADD COLUMN IF NOT EXISTS reviewed_at timestamptz;
CREATE TABLE IF NOT EXISTS public.fluctlight_visual_briefs (
 fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id),
 revision integer NOT NULL CHECK(revision>0),
 appearance text NOT NULL,clothing text NOT NULL,constraints text NOT NULL,"references" text NOT NULL,
 updated_by_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS public.fluctlight_visual_brief_revisions (
 id varchar(128) PRIMARY KEY,fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 revision integer NOT NULL,actor_id varchar(128) NOT NULL REFERENCES public.actors(id),reason text NOT NULL,
 before_value jsonb,after_value jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id,revision)
);
CREATE INDEX IF NOT EXISTS ix_visual_brief_revisions ON public.fluctlight_visual_brief_revisions(fluctlight_id,created_at DESC,id DESC);
`
