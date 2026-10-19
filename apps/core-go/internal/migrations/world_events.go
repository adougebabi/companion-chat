package migrations

const worldEventsSchemaSQL = `
ALTER TABLE public.life_events ADD COLUMN IF NOT EXISTS context_scope varchar(16) NOT NULL DEFAULT 'life' CHECK(context_scope IN ('life','ambient'));
CREATE TABLE IF NOT EXISTS public.world_event_templates (
 id varchar(128) PRIMARY KEY,world_id varchar(128) NOT NULL REFERENCES public.life_worlds(id),
 source_resource_id varchar(128) NOT NULL REFERENCES public.owner_surface_records(id),source_version varchar(32) NOT NULL,
 name varchar(256) NOT NULL,definition jsonb NOT NULL CHECK(jsonb_typeof(definition)='object'),
 enabled boolean NOT NULL DEFAULT false,revision integer NOT NULL DEFAULT 1 CHECK(revision>0),
 next_check_at timestamptz NOT NULL,last_triggered_at timestamptz,last_checked_at timestamptz,error_code varchar(128),
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(world_id,source_resource_id),UNIQUE(world_id,id)
);
CREATE TABLE IF NOT EXISTS public.world_event_runs (
 id varchar(128) PRIMARY KEY,world_id varchar(128) NOT NULL,template_id varchar(128) NOT NULL,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 template_revision integer NOT NULL,definition jsonb NOT NULL CHECK(jsonb_typeof(definition)='object'),
 trigger_kind varchar(16) NOT NULL CHECK(trigger_kind IN ('manual','automatic')),
 source_key varchar(256) NOT NULL UNIQUE,status varchar(16) NOT NULL CHECK(status IN ('queued','applied','filtered','failed')),
 due_at timestamptz NOT NULL,expires_at timestamptz NOT NULL,started_at timestamptz,end_at timestamptz,
 attempt_count integer NOT NULL DEFAULT 0,revision integer NOT NULL DEFAULT 1,
 life_event_id varchar(128) REFERENCES public.life_events(id),result jsonb NOT NULL DEFAULT '{}',error_code varchar(128),
 recovery_status varchar(16) NOT NULL DEFAULT 'not_required' CHECK(recovery_status IN ('not_required','pending','completed','failed')),
 recovery_result jsonb NOT NULL DEFAULT '{}',recovery_next_at timestamptz,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(expires_at>due_at),FOREIGN KEY(world_id,template_id) REFERENCES public.world_event_templates(world_id,id)
);
CREATE INDEX IF NOT EXISTS ix_world_event_due ON public.world_event_runs(due_at,id) WHERE status='queued';
CREATE INDEX IF NOT EXISTS ix_world_event_history ON public.world_event_runs(template_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS ix_world_event_recovery ON public.world_event_runs(end_at,id) WHERE recovery_status='pending';
CREATE OR REPLACE VIEW public.world_npc_effective_presence_sources AS
 SELECT p.* FROM public.world_npc_presence_runs p
 WHERE p.source_kind<>'world_event_execution' OR EXISTS(
 SELECT 1 FROM public.world_event_runs r JOIN public.life_events e ON e.id=r.life_event_id
 WHERE r.id=p.source_ref AND r.world_id=p.world_id AND r.status='applied' AND e.status='confirmed');
DROP TRIGGER IF EXISTS context_generation_bump ON public.world_event_runs;
CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.world_event_runs FOR EACH ROW EXECUTE FUNCTION public.bump_world_context_trigger();
`
