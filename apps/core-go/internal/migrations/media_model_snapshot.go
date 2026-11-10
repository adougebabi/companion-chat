package migrations

// Old media intents have no provable binding. Only intents created after this
// migration receive the contract marker that permits a new stage snapshot.
const mediaModelSnapshotSQL = `
ALTER TABLE public.media_intents
 ADD COLUMN IF NOT EXISTS model_provenance jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE public.media_intents ALTER COLUMN model_provenance
 SET DEFAULT '{"_media_model_snapshot_contract":"v1"}'::jsonb;
`
