package migrations

const studioFoundationSchemaSQL = `
ALTER TABLE public.fluctlight_foundation_revisions ADD COLUMN IF NOT EXISTS source_resource_id varchar(128);
ALTER TABLE public.fluctlight_foundation_revisions ADD COLUMN IF NOT EXISTS source_version varchar(64);
ALTER TABLE public.fluctlight_foundation_revisions ADD COLUMN IF NOT EXISTS source_choices jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(source_choices)='array');
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='foundation_source_pair') THEN
  ALTER TABLE public.fluctlight_foundation_revisions ADD CONSTRAINT foundation_source_pair CHECK((source_resource_id IS NULL)=(source_version IS NULL));
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='foundation_source_version_fk') THEN
  ALTER TABLE public.fluctlight_foundation_revisions ADD CONSTRAINT foundation_source_version_fk FOREIGN KEY(source_resource_id,source_version) REFERENCES public.studio_resource_versions(resource_id,version);
 END IF;
END $$;
CREATE INDEX IF NOT EXISTS ix_foundation_source_resource ON public.fluctlight_foundation_revisions(source_resource_id,status) WHERE source_resource_id IS NOT NULL;
`
