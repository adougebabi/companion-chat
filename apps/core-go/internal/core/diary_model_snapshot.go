package core

import (
	"context"
	"errors"
	"fmt"
)

const (
	diaryModelSnapshotContractKey = "_diary_model_snapshot_contract"
	diaryModelSnapshotsKey        = "_diary_model_snapshots"
	diaryModelSnapshotVersion     = workflowModelSnapshotVersion
)

var (
	errDiaryModelSnapshotMissing = errors.New("diary_model_snapshot_missing")
	errDiaryModelSnapshotInvalid = errors.New("diary_model_snapshot_invalid")
)

type diaryModelSnapshot = workflowModelSnapshot

func newDiaryModelSnapshot(agent FormalAgentID, variant, role string, assignment providerAssignment, version int64, concurrency int) diaryModelSnapshot {
	return newWorkflowModelSnapshot(agent, variant, role, assignment, version, concurrency)
}

func readDiaryModelSnapshot(provenance map[string]any, stage string, agent FormalAgentID, variant, role string) (diaryModelSnapshot, bool, error) {
	return readWorkflowModelSnapshot(provenance, diaryModelSnapshotsKey, stage, agent, variant, role, errDiaryModelSnapshotInvalid)
}

// The caller owns the existing diary execution lock. A newly selected stage is
// persisted while the attempt is running, before Provider I/O.
func (a *App) diaryStageModel(ctx context.Context, attemptID, stage string, provenance map[string]any, agent FormalAgentID, variant, role string, selectModel func() (diaryModelSnapshot, providerAssignment, error)) (diaryModelSnapshot, providerAssignment, error) {
	snapshot, found, err := readDiaryModelSnapshot(provenance, stage, agent, variant, role)
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
	if provenance[diaryModelSnapshotContractKey] != diaryModelSnapshotVersion {
		// A pre-upgrade running attempt cannot prove which model, if any,
		// received its prior request. Explicit retry creates a new attempt.
		return snapshot, providerAssignment{}, errDiaryModelSnapshotMissing
	}
	snapshot, assignment, err := selectModel()
	if err != nil {
		return snapshot, providerAssignment{}, err
	}
	if _, _, err := readDiaryModelSnapshot(map[string]any{diaryModelSnapshotsKey: map[string]any{stage: snapshot}}, stage, agent, variant, role); err != nil {
		return snapshot, providerAssignment{}, err
	}
	safeAssignment := assignment
	safeAssignment.Secret = ""
	if jsonString(snapshot.Assignment) != jsonString(safeAssignment) {
		return snapshot, providerAssignment{}, errDiaryModelSnapshotInvalid
	}
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_diary_attempts SET model_provenance=model_provenance||jsonb_build_object($2::text,COALESCE(model_provenance->$2::text,'{}'::jsonb)||jsonb_build_object($3::text,$4::jsonb)) WHERE id=$1 AND status='running' AND model_provenance->>$5::text=$6 AND NOT (COALESCE(model_provenance->$2::text,'{}'::jsonb) ? $3::text)`, attemptID, diaryModelSnapshotsKey, stage, jsonBytes(snapshot), diaryModelSnapshotContractKey, diaryModelSnapshotVersion)
	if err != nil {
		return snapshot, providerAssignment{}, err
	}
	if tag.RowsAffected() != 1 {
		return snapshot, providerAssignment{}, fmt.Errorf("diary_model_snapshot_not_saved: %w", ErrConflict)
	}
	snapshots := mapValue(provenance[diaryModelSnapshotsKey])
	if snapshots == nil {
		snapshots = map[string]any{}
	}
	snapshots[stage] = snapshot
	provenance[diaryModelSnapshotsKey] = snapshots
	return snapshot, assignment, nil
}

func publicDiaryModelProvenance(raw []byte) map[string]any {
	provenance := decodeObject(raw)
	delete(provenance, diaryModelSnapshotsKey)
	delete(provenance, diaryModelSnapshotContractKey)
	return provenance
}
