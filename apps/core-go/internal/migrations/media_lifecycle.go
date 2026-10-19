package migrations

const mediaLifecycleSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.media_deletions(
 id varchar(128) PRIMARY KEY,
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 asset_id varchar(128) NOT NULL UNIQUE REFERENCES public.media_assets(id),
 snapshot jsonb NOT NULL,
 status varchar(32) NOT NULL CHECK(status IN ('pending','running','failed','deleted')),
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 attempt integer NOT NULL DEFAULT 1 CHECK(attempt>0),
 workflow_id varchar(256) NOT NULL,
 intent_id varchar(128) NOT NULL,
 error_code varchar(128),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 deleted_at timestamptz
);
CREATE INDEX IF NOT EXISTS ix_media_deletions_owner ON public.media_deletions(owner_actor_id,created_at DESC,id DESC);
ALTER TABLE public.studio_resource_materials ADD COLUMN IF NOT EXISTS revision integer NOT NULL DEFAULT 1 CHECK(revision>0);
ALTER TABLE public.studio_resource_materials ADD COLUMN IF NOT EXISTS released_at timestamptz;
CREATE TABLE IF NOT EXISTS public.studio_material_history(
 id varchar(128) PRIMARY KEY,
 material_id varchar(128) NOT NULL REFERENCES public.studio_resource_materials(id),
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 revision integer NOT NULL,
 action varchar(32) NOT NULL CHECK(action IN ('retain','release')),
 before_value jsonb,
 after_value jsonb NOT NULL,
 reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(material_id,revision)
);
`
