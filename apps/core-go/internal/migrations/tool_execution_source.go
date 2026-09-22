package migrations

// The evidence identifier remains immutable in the command, revision and
// current row. Ownership, actor and conversation references remain enforced.
// Like durable Memory evidence_refs, a direct Tool's evidence can identify an
// authenticated business command without a fabricated cognition inbox row.
const toolExecutionSourceSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.tool_executions (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 capability_name varchar(128) NOT NULL,
 operation_id varchar(256) NOT NULL,
 authorization_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 request_digest varchar(128) NOT NULL,
 result jsonb NOT NULL CHECK (jsonb_typeof(result)='object'),
 committed_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,capability_name,operation_id)
);

ALTER TABLE public.tool_executions ADD COLUMN IF NOT EXISTS agent_id varchar(128) NOT NULL DEFAULT '';
ALTER TABLE public.tool_executions ADD COLUMN IF NOT EXISTS run_id text NOT NULL DEFAULT '';
ALTER TABLE public.tool_executions ADD COLUMN IF NOT EXISTS invocation jsonb NOT NULL DEFAULT '{}';
CREATE INDEX IF NOT EXISTS ix_tool_executions_agent_run ON public.tool_executions(fluctlight_id,agent_id,run_id);
CREATE TABLE IF NOT EXISTS public.agent_runs (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 agent_id varchar(128) NOT NULL,
 run_id text NOT NULL,
 input_digest varchar(128) NOT NULL,
 status varchar(16) NOT NULL CHECK(status IN ('running','completed','failed')),
 error_detail text NOT NULL DEFAULT '',
 result jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(result)='object'),
 started_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 PRIMARY KEY(fluctlight_id,agent_id,run_id)
);
CREATE TABLE IF NOT EXISTS public.tool_policy_reservations (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 reservation_id text NOT NULL,
 agent_id varchar(128) NOT NULL DEFAULT '',
 run_id text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(fluctlight_id,reservation_id)
);
ALTER TABLE public.active_memories
  DROP CONSTRAINT IF EXISTS fk_active_memories_source_fact;
ALTER TABLE public.active_memory_commands
  DROP CONSTRAINT IF EXISTS fk_active_memory_commands_source_fact;
COMMENT ON COLUMN public.active_memories.source_fact_id IS
  'Scoped evidence resource ID: cognition fact or authenticated business command; never fabricate an inbox fact for direct Tool execution';
COMMENT ON COLUMN public.active_memory_commands.source_fact_id IS
  'Immutable evidence resource ID copied from the authorized business command';
`
