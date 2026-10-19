package migrations

const worldEncounterRuntimeSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.world_npc_runtime_clocks(npc_id varchar(128) PRIMARY KEY REFERENCES public.world_npcs(id) ON DELETE CASCADE,world_id varchar(128) NOT NULL REFERENCES public.life_worlds(id) ON DELETE CASCADE,last_tick_at timestamptz NOT NULL,next_attempt_at timestamptz,error_code varchar(128),last_failure_at timestamptz,failure_count integer NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS public.world_npc_presence_runs (
 id varchar(128) PRIMARY KEY,world_id varchar(128) NOT NULL REFERENCES public.life_worlds(id),
 npc_id varchar(128) NOT NULL,actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 place_id varchar(128) NOT NULL,definition_revision integer NOT NULL,
 source_kind varchar(32) NOT NULL,source_ref varchar(256) NOT NULL,
 planned_start timestamptz NOT NULL,planned_end timestamptz NOT NULL,
 start_at timestamptz NOT NULL,end_at timestamptz NOT NULL,ended_at timestamptz,
 status varchar(16) NOT NULL CHECK(status IN ('active','ended','cancelled')),
 activity text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(end_at>start_at),UNIQUE(npc_id,source_ref),
 FOREIGN KEY(world_id,npc_id) REFERENCES public.world_npcs(world_id,id),
 FOREIGN KEY(world_id,place_id) REFERENCES public.world_places(world_id,id)
);
CREATE INDEX IF NOT EXISTS ix_npc_presence_active ON public.world_npc_presence_runs(world_id,place_id,end_at) WHERE status='active';
CREATE TABLE IF NOT EXISTS public.world_npc_opportunities (
 id varchar(128) PRIMARY KEY,world_id varchar(128) NOT NULL REFERENCES public.life_worlds(id),
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),profile_id varchar(128) NOT NULL,
 npc_id varchar(128) NOT NULL,place_id varchar(128) NOT NULL,
 npc_presence_id varchar(128) NOT NULL REFERENCES public.world_npc_presence_runs(id),npc_revision integer NOT NULL,
 source_event_id varchar(128) NOT NULL REFERENCES public.life_events(id),source_event_revision integer NOT NULL,
 character_presence_source varchar(32) NOT NULL CHECK(character_presence_source IN ('confirmed_event','active_activity')),
 source_activity_id varchar(128) REFERENCES public.fluctlight_life_activity_runs(id),source_activity_revision integer,
 CHECK((source_activity_id IS NULL)=(source_activity_revision IS NULL)),
 source_fact_id varchar(128),expected_life_revision varchar(128) NOT NULL,
 status varchar(16) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','deferred','contacted','declined','passed','expired','failed')),
 attempt_count integer NOT NULL DEFAULT 0,available_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,cooldown_until timestamptz NOT NULL,
 result jsonb NOT NULL DEFAULT '{}',npc_response jsonb,error_code varchar(128),revision integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id,profile_id,npc_presence_id,source_event_id,source_event_revision),
 FOREIGN KEY(world_id,npc_id) REFERENCES public.world_npcs(world_id,id),
 FOREIGN KEY(world_id,place_id) REFERENCES public.world_places(world_id,id)
);
CREATE INDEX IF NOT EXISTS ix_npc_opportunity_recovery ON public.world_npc_opportunities(available_at,id) WHERE status IN ('pending','deferred');
CREATE INDEX IF NOT EXISTS ix_npc_opportunity_pair ON public.world_npc_opportunities(fluctlight_id,world_id,npc_id,cooldown_until DESC);
ALTER TABLE public.world_encounters ALTER COLUMN relationship_id DROP NOT NULL;
ALTER TABLE public.world_encounters ADD COLUMN IF NOT EXISTS source_kind varchar(32) NOT NULL DEFAULT 'owner_confirmed_encounter';
ALTER TABLE public.world_encounters ADD COLUMN IF NOT EXISTS opportunity_id varchar(128) UNIQUE REFERENCES public.world_npc_opportunities(id);
DROP TRIGGER IF EXISTS context_generation_bump ON public.world_npc_presence_runs;
CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.world_npc_presence_runs FOR EACH ROW EXECUTE FUNCTION public.bump_world_context_trigger();
`
