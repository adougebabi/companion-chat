package migrations

const mediaReferenceGrantsSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.media_reference_grants(
 id varchar(128) PRIMARY KEY,
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 asset_id varchar(128) NOT NULL REFERENCES public.media_assets(id),
 asset_version varchar(128) NOT NULL,
 asset_revision integer NOT NULL CHECK(asset_revision>0),
 sha256 varchar(128) NOT NULL,
 byte_size integer NOT NULL CHECK(byte_size>0 AND byte_size<=33554432),
 mime_type varchar(128) NOT NULL,
 target_scope varchar(32) NOT NULL CHECK(target_scope='studio_resource'),
 target_id varchar(128) NOT NULL REFERENCES public.owner_surface_records(id),
 purpose varchar(32) NOT NULL CHECK(purpose IN ('scene','animal','object','person')),
 provenance varchar(64) NOT NULL CHECK(provenance='owner_explicit_unverified_content'),
 status varchar(32) NOT NULL CHECK(status IN ('active','revoked')),
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_media_grants_owner_page ON public.media_reference_grants(owner_actor_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS ix_media_grants_target ON public.media_reference_grants(owner_actor_id,target_scope,target_id,status);
CREATE TABLE IF NOT EXISTS public.media_reference_grant_history(
 id varchar(128) PRIMARY KEY,
 grant_id varchar(128) NOT NULL REFERENCES public.media_reference_grants(id),
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 revision integer NOT NULL,
 before_value jsonb,
 after_value jsonb NOT NULL,
 reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(grant_id,revision)
);
`
