package migrations

const worldItemsSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.world_items (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL,
 world_id varchar(128) NOT NULL,
 source_resource_id varchar(128) NOT NULL,
 source_version varchar(64) NOT NULL,
 name text NOT NULL,
 definition jsonb NOT NULL CHECK(jsonb_typeof(definition)='object'),
 available boolean NOT NULL DEFAULT true,
 place_id varchar(128) REFERENCES public.world_places(id),
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 local_overrides jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(local_overrides)='object'),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(world_id,source_resource_id),
 UNIQUE(fluctlight_id,id),
 FOREIGN KEY(fluctlight_id,world_id) REFERENCES public.life_worlds(fluctlight_id,id),
 FOREIGN KEY(source_resource_id,source_version) REFERENCES public.studio_resource_versions(resource_id,version)
);
CREATE INDEX IF NOT EXISTS ix_world_items_page ON public.world_items(world_id,created_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS public.world_item_revisions (
 item_id varchar(128) NOT NULL REFERENCES public.world_items(id),
 revision integer NOT NULL CHECK(revision>0),
 actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 reason text NOT NULL,
 before_value jsonb,
 after_value jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(item_id,revision)
);
ALTER TABLE public.fluctlight_wardrobe_items ADD COLUMN IF NOT EXISTS source_resource_id varchar(128);
ALTER TABLE public.fluctlight_wardrobe_items ADD COLUMN IF NOT EXISTS source_version varchar(64);
ALTER TABLE public.fluctlight_wardrobe_items ADD COLUMN IF NOT EXISTS source_world_item_id varchar(128);
ALTER TABLE public.fluctlight_wardrobe_items ADD COLUMN IF NOT EXISTS source_world_item_revision integer CHECK(source_world_item_revision>0);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='inventory_definition_source_pair') THEN
  ALTER TABLE public.fluctlight_wardrobe_items ADD CONSTRAINT inventory_definition_source_pair CHECK((source_resource_id IS NULL)=(source_version IS NULL) AND (source_world_item_id IS NULL)=(source_world_item_revision IS NULL) AND (source_resource_id IS NULL)=(source_world_item_id IS NULL));
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='inventory_definition_source_fk') THEN
  ALTER TABLE public.fluctlight_wardrobe_items ADD CONSTRAINT inventory_definition_source_fk FOREIGN KEY(source_resource_id,source_version) REFERENCES public.studio_resource_versions(resource_id,version);
  ALTER TABLE public.fluctlight_wardrobe_items ADD CONSTRAINT inventory_world_item_source_fk FOREIGN KEY(fluctlight_id,source_world_item_id) REFERENCES public.world_items(fluctlight_id,id);
 END IF;
END $$;
CREATE INDEX IF NOT EXISTS ix_inventory_definition_source ON public.fluctlight_wardrobe_items(source_resource_id) WHERE source_resource_id IS NOT NULL;
`
