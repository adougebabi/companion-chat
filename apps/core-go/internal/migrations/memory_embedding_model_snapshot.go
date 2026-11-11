package migrations

// Existing intents retain an empty contract; new intents create the marker
// and a snapshot row in the same transaction that records the Memory change.
const memoryEmbeddingModelSnapshotSQL = `
ALTER TABLE public.platform_workflow_intents
 ADD COLUMN IF NOT EXISTS embedding_snapshot_contract text NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS public.memory_embedding_model_snapshots (
 intent_id varchar(128) PRIMARY KEY REFERENCES public.platform_workflow_intents(intent_id) ON DELETE CASCADE,
 snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
 selected boolean NOT NULL DEFAULT false
);
`
