package migrations

// Existing preparations are intentionally left without a contract marker:
// their previously selected model cannot be reconstructed from current settings.
const wardrobeModelSnapshotSQL = `
ALTER TABLE public.fluctlight_wardrobe_preparations
 ADD COLUMN IF NOT EXISTS model_provenance jsonb NOT NULL DEFAULT '{}'::jsonb;
`
