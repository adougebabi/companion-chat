package migrations

const momentNPCInteractionsSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.moment_npc_interactions (
 id varchar(128) PRIMARY KEY, moment_id varchar(128) NOT NULL REFERENCES public.moments(id),
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id), fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 world_id varchar(128) NOT NULL REFERENCES public.life_worlds(id), npc_id varchar(128) NOT NULL REFERENCES public.world_npcs(id),
 npc_actor_id varchar(128) NOT NULL REFERENCES public.actors(id), npc_revision integer NOT NULL CHECK(npc_revision>0),
 encounter_id varchar(128) NOT NULL REFERENCES public.world_encounters(id), source_event_id varchar(128) NOT NULL REFERENCES public.life_events(id),
 snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object'), response jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(response)='object'),
 status varchar(16) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','completed','skipped','suppressed','failed')),
 source_fact_id varchar(128), attempt integer NOT NULL DEFAULT 1 CHECK(attempt>0), revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 comment_id varchar(128) REFERENCES public.moment_comments(id), error_code text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(moment_id,npc_id), FOREIGN KEY(world_id,npc_id) REFERENCES public.world_npcs(world_id,id)
);
CREATE INDEX IF NOT EXISTS moment_npc_interactions_page ON public.moment_npc_interactions(moment_id,created_at DESC,id DESC);
ALTER TABLE public.moment_comments ADD COLUMN IF NOT EXISTS source_interaction_id varchar(128) REFERENCES public.moment_npc_interactions(id);
CREATE UNIQUE INDEX IF NOT EXISTS moment_comments_source_interaction ON public.moment_comments(source_interaction_id) WHERE source_interaction_id IS NOT NULL;
`
