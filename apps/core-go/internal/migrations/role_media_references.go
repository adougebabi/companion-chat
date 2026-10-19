package migrations

const roleMediaReferencesSchemaSQL = `
ALTER TABLE public.media_reference_grants DROP CONSTRAINT IF EXISTS media_reference_grants_target_scope_check;
ALTER TABLE public.media_reference_grants ADD CONSTRAINT media_reference_grants_target_scope_check CHECK(target_scope IN ('studio_resource','fluctlight'));
ALTER TABLE public.media_reference_grants DROP CONSTRAINT IF EXISTS media_reference_grants_target_id_fkey;
ALTER TABLE public.media_reference_grants ADD COLUMN IF NOT EXISTS target_resource_id varchar(128) GENERATED ALWAYS AS (CASE WHEN target_scope='studio_resource' THEN target_id ELSE NULL END) STORED REFERENCES public.owner_surface_records(id);
ALTER TABLE public.media_reference_grants ADD COLUMN IF NOT EXISTS target_fluctlight_id varchar(128) GENERATED ALWAYS AS (CASE WHEN target_scope='fluctlight' THEN target_id ELSE NULL END) STORED REFERENCES public.fluctlights(id);
`
