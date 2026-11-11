package migrations

const runtimeSummaryModelSnapshotSQL = `
CREATE TABLE IF NOT EXISTS public.runtime_summary_model_snapshots (
 intent_id varchar(128) PRIMARY KEY REFERENCES public.platform_workflow_intents(intent_id) ON DELETE CASCADE,
 snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
 selected boolean NOT NULL DEFAULT false
);
`
