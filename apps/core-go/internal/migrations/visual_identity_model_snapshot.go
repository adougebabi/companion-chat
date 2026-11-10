package migrations

// Old sessions remain explicitly unmarked; a new session can freeze its own
// model stages without pretending that an older session used today's model.
const visualIdentityModelSnapshotSQL = `
ALTER TABLE public.fluctlight_visual_identity_sessions
 ADD COLUMN IF NOT EXISTS model_provenance jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE public.fluctlight_visual_identity_sessions ALTER COLUMN model_provenance
 SET DEFAULT '{"_visual_model_snapshot_contract":"v1"}'::jsonb;
`
