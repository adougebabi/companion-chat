package migrations

// Immutable insertion identities preserve snapshot membership even when a
// status update creates a new tuple xmin. pg_current_xact_id returns the top
// level ID (including within a savepoint), as required by pg_visible_in_snapshot.
const diagnosticPaginationSchemaSQL = `
ALTER TABLE public.diagnostic_model_runs ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE public.agent_runs ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
ALTER TABLE public.diagnostic_events ADD COLUMN IF NOT EXISTS recorded_xid xid8 NOT NULL DEFAULT pg_current_xact_id();
CREATE INDEX IF NOT EXISTS ix_diagnostic_model_creation ON public.diagnostic_model_runs(created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS ix_agent_run_creation ON public.agent_runs(started_at DESC,fluctlight_id DESC,agent_id DESC,run_id DESC);
`
