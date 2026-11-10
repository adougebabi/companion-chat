package core

import (
	"context"
	"errors"
	"fmt"
)

const (
	mediaModelSnapshotContractKey = "_media_model_snapshot_contract"
	mediaModelSnapshotsKey        = "_media_model_snapshots"
)

var errMediaModelSnapshotInvalid = errors.New("media_model_snapshot_invalid")

// Durable media intents save a model stage before its first Provider request.
// Older intents cannot prove their original selection; their existing behavior
// is retained and explicitly diagnosed until their recovery contract is migrated.
func (a *App) mediaStageAssignment(ctx context.Context, intent mediaIntent, agent FormalAgentID, variant, role string) (providerAssignment, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return providerAssignment{}, err
	}
	if intent.modelProvenance == nil {
		// Independent non-durable task entry points keep their caller's binding.
		return provider.assignment(ctx, role)
	}
	if len(intent.modelProvenance) == 0 {
		a.recordDiagnosticEvent(ctx, "media.model_binding.legacy_unfrozen", "warn", intent.Owner, intent.ID, "media:"+intent.ID, map[string]any{"agent_id": agent, "task_variant": variant, "reason_code": "legacy_model_binding_unavailable"})
		return provider.assignment(ctx, role)
	}
	if intent.modelProvenance[mediaModelSnapshotContractKey] != workflowModelSnapshotVersion {
		return providerAssignment{}, errMediaModelSnapshotInvalid
	}
	snapshot, found, err := readWorkflowModelSnapshot(intent.modelProvenance, mediaModelSnapshotsKey, variant, agent, variant, role, errMediaModelSnapshotInvalid)
	if err != nil {
		return providerAssignment{}, err
	}
	if found {
		assignment := snapshot.Assignment
		if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
			return providerAssignment{}, fmt.Errorf("media_model_snapshot_restore: %w", err)
		}
		return assignment, nil
	}
	assignment, err := provider.assignment(ctx, role)
	if err != nil {
		return providerAssignment{}, err
	}
	snapshot = newWorkflowModelSnapshot(agent, variant, role, assignment, 0, 0)
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET model_provenance=model_provenance||jsonb_build_object($4::text,COALESCE(model_provenance->$4::text,'{}'::jsonb)||jsonb_build_object($5::text,$6::jsonb)) WHERE id=$1 AND status IN ('pending','running') AND model_provenance->>$2::text=$3 AND NOT (COALESCE(model_provenance->$4::text,'{}'::jsonb) ? $5::text)`, intent.ID, mediaModelSnapshotContractKey, workflowModelSnapshotVersion, mediaModelSnapshotsKey, variant, jsonBytes(snapshot))
	if err != nil {
		return providerAssignment{}, err
	}
	if tag.RowsAffected() != 1 {
		return providerAssignment{}, fmt.Errorf("media_model_snapshot_not_saved: %w", ErrConflict)
	}
	snapshots := mapValue(intent.modelProvenance[mediaModelSnapshotsKey])
	if snapshots == nil {
		snapshots = map[string]any{}
	}
	snapshots[variant] = snapshot
	intent.modelProvenance[mediaModelSnapshotsKey] = snapshots
	return assignment, nil
}
