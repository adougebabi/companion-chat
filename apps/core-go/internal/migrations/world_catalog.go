package migrations

// Additive upgrade: ownership remains on the world, active selection is separate.
// Old textual locations are deliberately not guessed into catalogue identities.
const worldCatalogSchemaSQL = `
ALTER TABLE public.life_worlds DROP CONSTRAINT IF EXISTS life_worlds_fluctlight_id_key;
ALTER TABLE public.life_worlds ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';
ALTER TABLE public.life_worlds ADD COLUMN IF NOT EXISTS flavor text NOT NULL DEFAULT '';
ALTER TABLE public.life_worlds ADD COLUMN IF NOT EXISTS item_pool jsonb NOT NULL DEFAULT '""' CHECK(jsonb_typeof(item_pool) IN ('string','array'));
DO $unique_world$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.life_worlds'::regclass AND conname='life_worlds_owner_identity_key') THEN
  ALTER TABLE public.life_worlds ADD CONSTRAINT life_worlds_owner_identity_key UNIQUE(fluctlight_id,id);
 END IF;
END $unique_world$;
DO $binding$ BEGIN
 IF to_regclass('public.fluctlight_world_bindings') IS NULL THEN
  CREATE TABLE public.fluctlight_world_bindings (
   fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id),
   world_id varchar(128) NOT NULL,
   revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
   FOREIGN KEY(fluctlight_id,world_id) REFERENCES public.life_worlds(fluctlight_id,id)
  );
  INSERT INTO public.fluctlight_world_bindings(fluctlight_id,world_id) SELECT fluctlight_id,id FROM public.life_worlds;
 END IF;
END $binding$;
CREATE UNIQUE INDEX IF NOT EXISTS ix_world_definition_instance ON public.life_worlds(fluctlight_id,source_resource_id) WHERE source_resource_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ix_place_definition_instance ON public.world_places(world_id,source_resource_id) WHERE source_resource_id IS NOT NULL;
ALTER TABLE public.world_places ADD COLUMN IF NOT EXISTS details jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(details)='object');
ALTER TABLE public.world_npcs ADD COLUMN IF NOT EXISTS enabled boolean NOT NULL DEFAULT true;
ALTER TABLE public.world_npcs ADD COLUMN IF NOT EXISTS position_mode varchar(16) NOT NULL DEFAULT 'routine' CHECK(position_mode IN ('routine','fixed','unknown'));
ALTER TABLE public.world_npcs ADD COLUMN IF NOT EXISTS local_overrides jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(local_overrides)='object');
`
