package migrations

const studioVisualStyleSchemaSQL = `
ALTER TABLE public.fluctlight_visual_briefs ADD COLUMN IF NOT EXISTS source_resource_id varchar(128);
ALTER TABLE public.fluctlight_visual_briefs ADD COLUMN IF NOT EXISTS source_version varchar(64);
ALTER TABLE public.fluctlight_visual_briefs ADD COLUMN IF NOT EXISTS source_overrides jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(source_overrides)='object');
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='visual_brief_source_pair') THEN
  ALTER TABLE public.fluctlight_visual_briefs ADD CONSTRAINT visual_brief_source_pair CHECK((source_resource_id IS NULL)=(source_version IS NULL));
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='visual_brief_source_version_fk') THEN
  ALTER TABLE public.fluctlight_visual_briefs ADD CONSTRAINT visual_brief_source_version_fk FOREIGN KEY(source_resource_id,source_version) REFERENCES public.studio_resource_versions(resource_id,version);
 END IF;
END $$;
CREATE INDEX IF NOT EXISTS ix_visual_brief_resource ON public.fluctlight_visual_briefs(source_resource_id) WHERE source_resource_id IS NOT NULL;
`
