package migrations

// The row is created only with a new persona preparation intent. Existing
// intents are not backfilled with today's model configuration.
const personaPreparationModelSnapshotSQL = `
CREATE TABLE IF NOT EXISTS public.persona_preparation_model_snapshots (
 intent_id varchar(128) PRIMARY KEY REFERENCES public.platform_workflow_intents(intent_id) ON DELETE CASCADE,
 snapshot jsonb NOT NULL DEFAULT '{}'::jsonb
);
`
