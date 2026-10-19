package migrations

const ownerMediaSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.owner_media_uploads (
 id varchar(128) PRIMARY KEY,
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 idempotency_key varchar(256) NOT NULL,
 request_digest varchar(128) NOT NULL,
 asset_id varchar(128) NOT NULL UNIQUE,
 filename text NOT NULL,
 kind varchar(32) NOT NULL CHECK(kind IN ('image','video','audio')),
 mime_type varchar(128) NOT NULL,
 byte_size integer NOT NULL CHECK(byte_size>0 AND byte_size<=67108864),
 sha256 varchar(128) NOT NULL,
 bucket varchar(256) NOT NULL,
 object_key text NOT NULL,
 storage_scope varchar(128) NOT NULL,
 status varchar(32) NOT NULL CHECK(status IN ('uploading','ready','failed')),
 error_code varchar(128),
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(owner_actor_id,idempotency_key)
);
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS upload_owner_actor_id varchar(128) REFERENCES public.actors(id);
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS owner_upload_id varchar(128) UNIQUE REFERENCES public.owner_media_uploads(id);
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS revision integer NOT NULL DEFAULT 1 CHECK(revision>0);
ALTER TABLE public.media_assets ALTER COLUMN provider_request_id DROP NOT NULL;
ALTER TABLE public.media_assets ALTER COLUMN workflow_id DROP NOT NULL;
ALTER TABLE public.media_assets DROP CONSTRAINT IF EXISTS media_assets_scope_check;
ALTER TABLE public.media_assets ADD CONSTRAINT media_assets_scope_check CHECK(
 (media_scope='fluctlight' AND owner_fluctlight_id IS NOT NULL AND studio_owner_actor_id IS NULL AND studio_job_id IS NULL AND upload_owner_actor_id IS NULL AND owner_upload_id IS NULL AND provider_request_id IS NOT NULL AND workflow_id IS NOT NULL) OR
 (media_scope='studio' AND owner_fluctlight_id IS NULL AND studio_owner_actor_id IS NOT NULL AND studio_job_id IS NOT NULL AND upload_owner_actor_id IS NULL AND owner_upload_id IS NULL AND provider_request_id IS NOT NULL AND workflow_id IS NOT NULL) OR
 (media_scope='owner_upload' AND owner_fluctlight_id IS NULL AND studio_owner_actor_id IS NULL AND studio_job_id IS NULL AND upload_owner_actor_id IS NOT NULL AND owner_upload_id IS NOT NULL AND provider_request_id IS NULL AND workflow_id IS NULL)
);
CREATE INDEX IF NOT EXISTS ix_owner_uploads_page ON public.owner_media_uploads(owner_actor_id,created_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS public.media_asset_annotations (
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 asset_id varchar(128) NOT NULL REFERENCES public.media_assets(id),
 title text NOT NULL DEFAULT '',
 hidden boolean NOT NULL DEFAULT false,
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(owner_actor_id,asset_id)
);
`
