package migrations

// Definitions remain Studio resources. Published snapshots are immutable;
// runtime objects and their references are isolated per Fluctlight world.
const worldRuntimeSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.studio_resource_versions (
 resource_id varchar(128) NOT NULL REFERENCES public.owner_surface_records(id),
 version varchar(64) NOT NULL, owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 kind varchar(64) NOT NULL, name text NOT NULL, body jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(resource_id,version)
);
CREATE TABLE IF NOT EXISTS public.life_worlds (
 id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL UNIQUE REFERENCES public.fluctlights(id),
 name text NOT NULL, background text NOT NULL DEFAULT '', rules text NOT NULL DEFAULT '',
 source_resource_id varchar(128), source_version varchar(64), revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(source_resource_id,source_version) REFERENCES public.studio_resource_versions(resource_id,version)
);
CREATE TABLE IF NOT EXISTS public.world_places (
 id varchar(128) PRIMARY KEY, world_id varchar(128) NOT NULL REFERENCES public.life_worlds(id),
 name text NOT NULL, description text NOT NULL DEFAULT '', parent_place_id varchar(128),
 status varchar(16) NOT NULL DEFAULT 'active' CHECK(status IN ('active','archived')),
 source_resource_id varchar(128), source_version varchar(64), revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(world_id,id),
 FOREIGN KEY(world_id,parent_place_id) REFERENCES public.world_places(world_id,id),
 FOREIGN KEY(source_resource_id,source_version) REFERENCES public.studio_resource_versions(resource_id,version)
);
CREATE TABLE IF NOT EXISTS public.world_npcs (
 id varchar(128) PRIMARY KEY, world_id varchar(128) NOT NULL REFERENCES public.life_worlds(id),
 actor_id varchar(128) NOT NULL UNIQUE REFERENCES public.actors(id),
 source_resource_id varchar(128) NOT NULL, source_version varchar(64) NOT NULL,
 name text NOT NULL, role text NOT NULL DEFAULT '', background text NOT NULL DEFAULT '',
 home_place_id varchar(128), routine jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(routine)='array'),
 status varchar(16) NOT NULL DEFAULT 'active' CHECK(status IN ('active','archived')),
 confirmed_place_id varchar(128), confirmed_at timestamptz,
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(world_id,id),
 UNIQUE(world_id,source_resource_id),
 FOREIGN KEY(source_resource_id,source_version) REFERENCES public.studio_resource_versions(resource_id,version),
 FOREIGN KEY(world_id,home_place_id) REFERENCES public.world_places(world_id,id),
 FOREIGN KEY(world_id,confirmed_place_id) REFERENCES public.world_places(world_id,id)
);
CREATE TABLE IF NOT EXISTS public.world_encounters (
 id varchar(128) PRIMARY KEY, world_id varchar(128) NOT NULL REFERENCES public.life_worlds(id),
 npc_id varchar(128) NOT NULL, place_id varchar(128) NOT NULL,
 life_event_id varchar(128) NOT NULL REFERENCES public.life_events(id),
 relationship_id varchar(128) NOT NULL REFERENCES public.relationships(id),
 memory_id varchar(128) REFERENCES public.memories(id),
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 summary text NOT NULL, evidence_refs jsonb NOT NULL CHECK(jsonb_typeof(evidence_refs)='array'),
 occurred_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(world_id,npc_id) REFERENCES public.world_npcs(world_id,id),
 FOREIGN KEY(world_id,place_id) REFERENCES public.world_places(world_id,id)
);
CREATE INDEX IF NOT EXISTS ix_world_encounters_npc ON public.world_encounters(npc_id,occurred_at DESC);
DROP TRIGGER IF EXISTS context_generation_bump ON public.life_worlds;
CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.life_worlds
 FOR EACH ROW EXECUTE FUNCTION public.bump_context_direct_trigger();
CREATE OR REPLACE FUNCTION public.bump_world_context_trigger() RETURNS trigger LANGUAGE plpgsql AS $world_context$
DECLARE v_world text; v_fid text;
BEGIN
 IF TG_OP='DELETE' THEN v_world:=OLD.world_id; ELSE v_world:=NEW.world_id; END IF;
 SELECT fluctlight_id INTO v_fid FROM public.life_worlds WHERE id=v_world;
 PERFORM public.bump_fluctlight_context_generation_with_source(v_fid,TG_TABLE_NAME,TG_OP,NULL);
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END $world_context$;
DROP TRIGGER IF EXISTS context_generation_bump ON public.world_places;
CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.world_places
 FOR EACH ROW EXECUTE FUNCTION public.bump_world_context_trigger();
DROP TRIGGER IF EXISTS context_generation_bump ON public.world_npcs;
CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.world_npcs
 FOR EACH ROW EXECUTE FUNCTION public.bump_world_context_trigger();
`
