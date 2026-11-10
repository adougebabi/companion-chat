package core

import (
	"context"
	"errors"
	"fmt"
)

const (
	visualModelSnapshotContractKey = "_visual_model_snapshot_contract"
	visualModelSnapshotsKey        = "_visual_model_snapshots"
	visualModelVariant             = "visual_identity_agent_response"
	visualTextModelStage           = "text"
	visualVisionModelStage         = "vision"
)

var errVisualModelSnapshotInvalid = errors.New("visual_identity_model_snapshot_invalid")

// A Visual Identity session may use independent text and vision models. Each
// stage keeps its first binding across checkpoints and process restarts.
func (a *App) visualIdentityStageAssignment(ctx context.Context, state visualIdentityAgentState, stage, role string, configVersion int64) (providerAssignment, int64, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return providerAssignment{}, 0, err
	}
	if len(state.modelProvenance) == 0 {
		a.recordDiagnosticEvent(ctx, "visual_identity.model_binding.legacy_unfrozen", "warn", state.FluctlightID, state.SessionID, "visual_identity:"+state.SessionID, map[string]any{"stage": stage, "reason_code": "legacy_model_binding_unavailable"})
		assignment, err := provider.assignment(ctx, role)
		return assignment, configVersion, err
	}
	if state.modelProvenance[visualModelSnapshotContractKey] != workflowModelSnapshotVersion {
		return providerAssignment{}, 0, errVisualModelSnapshotInvalid
	}
	snapshot, found, err := readWorkflowModelSnapshot(state.modelProvenance, visualModelSnapshotsKey, stage, FormalAgentVisualIdentity, visualModelVariant, role, errVisualModelSnapshotInvalid)
	if err != nil {
		return providerAssignment{}, 0, err
	}
	if found {
		assignment := snapshot.Assignment
		if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
			return providerAssignment{}, 0, fmt.Errorf("visual_identity_model_snapshot_restore: %w", err)
		}
		return assignment, snapshot.ConfigVersion, nil
	}
	assignment, err := provider.assignment(ctx, role)
	if err != nil {
		return providerAssignment{}, 0, err
	}
	snapshot = newWorkflowModelSnapshot(FormalAgentVisualIdentity, visualModelVariant, role, assignment, configVersion, 0)
	if _, _, err := readWorkflowModelSnapshot(map[string]any{visualModelSnapshotsKey: map[string]any{stage: snapshot}}, visualModelSnapshotsKey, stage, FormalAgentVisualIdentity, visualModelVariant, role, errVisualModelSnapshotInvalid); err != nil {
		return providerAssignment{}, 0, err
	}
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET model_provenance=model_provenance||jsonb_build_object($4::text,COALESCE(model_provenance->$4::text,'{}'::jsonb)||jsonb_build_object($5::text,$6::jsonb)),updated_at=now() WHERE id=$1 AND status IN ('queued','running','character_sheet_pending') AND model_provenance->>$2::text=$3 AND NOT (COALESCE(model_provenance->$4::text,'{}'::jsonb) ? $5::text) AND current_attempt=$7`, state.SessionID, visualModelSnapshotContractKey, workflowModelSnapshotVersion, visualModelSnapshotsKey, stage, jsonBytes(snapshot), state.Attempt)
	if err != nil {
		return providerAssignment{}, 0, err
	}
	if tag.RowsAffected() != 1 {
		return providerAssignment{}, 0, fmt.Errorf("visual_identity_model_snapshot_not_saved: %w", ErrConflict)
	}
	return assignment, snapshot.ConfigVersion, nil
}
