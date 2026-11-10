package core

import (
	"context"
	"errors"
	"fmt"
)

const (
	studioModelSnapshotContractKey = "_studio_model_snapshot_contract"
	studioModelSnapshotsKey        = "_studio_model_snapshots"
)

var (
	errStudioModelSnapshotMissing = errors.New("studio_model_snapshot_missing")
	errStudioModelSnapshotInvalid = errors.New("studio_model_snapshot_invalid")
)

// Studio retains its existing execution lock and attempt identity. This
// snapshot only fixes the model used by the current validated model stage.
func (a *App) studioStageModel(ctx context.Context, attemptID, stage string, provenance map[string]any, agent FormalAgentID, variant, role string, selectModel func() (workflowModelSnapshot, providerAssignment, error)) (workflowModelSnapshot, providerAssignment, error) {
	snapshot, found, err := readWorkflowModelSnapshot(provenance, studioModelSnapshotsKey, stage, agent, variant, role, errStudioModelSnapshotInvalid)
	if err != nil {
		return snapshot, providerAssignment{}, err
	}
	if found {
		if a == nil || a.Provider == nil {
			return snapshot, providerAssignment{}, errors.New("model_task_provider_unavailable")
		}
		assignment := snapshot.Assignment
		if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
			return snapshot, providerAssignment{}, err
		}
		return snapshot, assignment, nil
	}
	if provenance[studioModelSnapshotContractKey] != workflowModelSnapshotVersion {
		return snapshot, providerAssignment{}, errStudioModelSnapshotMissing
	}
	snapshot, assignment, err := selectModel()
	if err != nil {
		return snapshot, providerAssignment{}, err
	}
	if _, _, err := readWorkflowModelSnapshot(map[string]any{studioModelSnapshotsKey: map[string]any{stage: snapshot}}, studioModelSnapshotsKey, stage, agent, variant, role, errStudioModelSnapshotInvalid); err != nil {
		return snapshot, providerAssignment{}, err
	}
	safe := assignment
	safe.Secret = ""
	if jsonString(snapshot.Assignment) != jsonString(safe) {
		return snapshot, providerAssignment{}, errStudioModelSnapshotInvalid
	}
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.studio_job_attempts a SET provenance=a.provenance||jsonb_build_object($2::text,COALESCE(a.provenance->$2::text,'{}'::jsonb)||jsonb_build_object($3::text,$4::jsonb)) FROM public.studio_jobs j WHERE a.id=$1 AND j.id=a.job_id AND j.status='running' AND j.latest_attempt=a.attempt AND a.status='running' AND a.provenance->>$5::text=$6 AND NOT (COALESCE(a.provenance->$2::text,'{}'::jsonb) ? $3::text)`, attemptID, studioModelSnapshotsKey, stage, jsonBytes(snapshot), studioModelSnapshotContractKey, workflowModelSnapshotVersion)
	if err != nil {
		return snapshot, providerAssignment{}, err
	}
	if tag.RowsAffected() != 1 {
		return snapshot, providerAssignment{}, fmt.Errorf("studio_model_snapshot_not_saved: %w", ErrConflict)
	}
	snapshots := mapValue(provenance[studioModelSnapshotsKey])
	snapshots[stage] = snapshot
	provenance[studioModelSnapshotsKey] = snapshots
	return snapshot, assignment, nil
}
