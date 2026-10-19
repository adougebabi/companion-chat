package migrations

const placeDefaultsSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.world_place_defaults(
 place_id varchar(128) PRIMARY KEY,world_id varchar(128) NOT NULL,
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 source_kind varchar(32) NOT NULL CHECK(source_kind IN ('initial_saved','published_definition','owner_pinned_saved')),
 source_resource_id varchar(128),source_version varchar(64),definition jsonb NOT NULL CHECK(jsonb_typeof(definition)='object'),
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(world_id,place_id) REFERENCES public.world_places(world_id,id) ON DELETE CASCADE,
 FOREIGN KEY(source_resource_id,source_version) REFERENCES public.studio_resource_versions(resource_id,version),
 CHECK((source_kind='published_definition' AND source_resource_id IS NOT NULL AND source_version IS NOT NULL) OR (source_kind<>'published_definition' AND source_resource_id IS NULL AND source_version IS NULL))
);
CREATE TABLE IF NOT EXISTS public.world_place_default_history(
 id varchar(128) PRIMARY KEY,world_id varchar(128) NOT NULL,place_id varchar(128) NOT NULL,
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 operation varchar(32) NOT NULL CHECK(operation IN ('initial_saved','set_baseline','restore_fields','edit_fields')),
 place_revision integer NOT NULL,default_revision integer NOT NULL,
 before_value jsonb,after_value jsonb NOT NULL,source jsonb NOT NULL,reason text NOT NULL,audit_id varchar(128),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS world_place_default_history_page ON public.world_place_default_history(place_id,created_at DESC,id DESC);
`
