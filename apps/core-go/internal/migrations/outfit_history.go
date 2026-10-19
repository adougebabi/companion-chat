package migrations

const outfitHistorySchemaSQL = `
CREATE TABLE IF NOT EXISTS public.fluctlight_wardrobe_outfit_revisions (
 id varchar(128) PRIMARY KEY,fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 outfit_id varchar(128) NOT NULL REFERENCES public.fluctlight_wardrobe_outfits(id),
 profile_id varchar(128) NOT NULL,revision integer NOT NULL,source_ref varchar(256) NOT NULL,
 before_value jsonb,after_value jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(outfit_id,revision)
);
CREATE INDEX IF NOT EXISTS ix_outfit_revisions ON public.fluctlight_wardrobe_outfit_revisions(fluctlight_id,outfit_id,created_at DESC,id DESC);

CREATE TABLE IF NOT EXISTS public.fluctlight_wearing_history (
 id varchar(128) PRIMARY KEY,fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 wardrobe_revision integer NOT NULL,source_kind varchar(32) NOT NULL,source_ref varchar(256) NOT NULL,
 before_items jsonb NOT NULL,after_items jsonb NOT NULL,occurred_at timestamptz NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_wearing_history_role ON public.fluctlight_wearing_history(fluctlight_id,created_at DESC,id DESC);
`
