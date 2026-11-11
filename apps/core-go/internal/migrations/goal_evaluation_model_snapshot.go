package migrations

// Legacy requests have no provable binding; only requests inserted after this
// migration receive the marker that permits a new per-request snapshot.
const goalEvaluationModelSnapshotSQL = `
ALTER TABLE public.goal_evaluation_requests
 ADD COLUMN IF NOT EXISTS model_provenance jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE public.goal_evaluation_requests
 ADD COLUMN IF NOT EXISTS model_snapshot_required boolean NOT NULL DEFAULT false;
ALTER TABLE public.goal_evaluation_requests ALTER COLUMN model_provenance
 SET DEFAULT '{"_goal_evaluation_model_snapshot_contract":"v1"}'::jsonb;
ALTER TABLE public.goal_evaluation_requests ALTER COLUMN model_snapshot_required SET DEFAULT true;
`
