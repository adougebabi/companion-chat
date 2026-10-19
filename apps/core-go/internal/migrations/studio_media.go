package migrations

// Existing formal rows retain their owner and scope. Studio has a real Human
// owner and producer job instead of borrowing a Fluctlight identity.
const studioMediaSchemaSQL = `
ALTER TABLE public.studio_jobs DROP CONSTRAINT IF EXISTS studio_jobs_kind_check;
ALTER TABLE public.studio_jobs ADD CONSTRAINT studio_jobs_kind_check CHECK(kind IN ('assistant','text_preview','image_preview'));
ALTER TABLE public.media_intents ALTER COLUMN owner_fluctlight_id DROP NOT NULL;
ALTER TABLE public.media_assets ALTER COLUMN owner_fluctlight_id DROP NOT NULL;
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS media_scope varchar(16) NOT NULL DEFAULT 'fluctlight';
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS media_scope varchar(16) NOT NULL DEFAULT 'fluctlight';
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS studio_owner_actor_id varchar(128) REFERENCES public.actors(id);
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS studio_owner_actor_id varchar(128) REFERENCES public.actors(id);
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS studio_job_id varchar(128) UNIQUE REFERENCES public.studio_jobs(id);
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS studio_job_id varchar(128) UNIQUE REFERENCES public.studio_jobs(id);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='media_intents_scope_check') THEN
 ALTER TABLE public.media_intents ADD CONSTRAINT media_intents_scope_check CHECK(
 (media_scope='fluctlight' AND owner_fluctlight_id IS NOT NULL AND studio_owner_actor_id IS NULL AND studio_job_id IS NULL) OR
 (media_scope='studio' AND owner_fluctlight_id IS NULL AND studio_owner_actor_id IS NOT NULL AND studio_job_id IS NOT NULL AND conversation_id IS NULL AND message_id IS NULL AND moment_id IS NULL));
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='media_assets_scope_check') THEN
 ALTER TABLE public.media_assets ADD CONSTRAINT media_assets_scope_check CHECK(
 (media_scope='fluctlight' AND owner_fluctlight_id IS NOT NULL AND studio_owner_actor_id IS NULL AND studio_job_id IS NULL) OR
 (media_scope='studio' AND owner_fluctlight_id IS NULL AND studio_owner_actor_id IS NOT NULL AND studio_job_id IS NOT NULL));
 END IF;
END $$;
ALTER TABLE public.studio_resource_materials ALTER COLUMN text DROP NOT NULL;
ALTER TABLE public.studio_resource_materials ADD COLUMN IF NOT EXISTS asset_id varchar(128) REFERENCES public.media_assets(id);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='studio_material_content_check') THEN
 ALTER TABLE public.studio_resource_materials ADD CONSTRAINT studio_material_content_check CHECK((text IS NOT NULL AND asset_id IS NULL) OR (text IS NULL AND asset_id IS NOT NULL));
 END IF;
END $$;
`
