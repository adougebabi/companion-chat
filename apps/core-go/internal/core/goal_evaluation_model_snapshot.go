package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	goalEvaluationModelSnapshotContractKey = "_goal_evaluation_model_snapshot_contract"
	goalEvaluationModelSnapshotsKey        = "_goal_evaluation_model_snapshots"
	goalEvaluationModelSelectedKey         = "_goal_evaluation_model_selected"
	goalEvaluationModelStage               = "evaluation"
	goalEvaluationModelVariant             = "goal_evaluation_v1"
	goalEvaluationModelRole                = "cognitive_assessment"
)

var errGoalEvaluationModelSnapshotInvalid = errors.New("goal_evaluation_model_snapshot_invalid")

// The request owns the model binding across claims. A stale claim cannot save
// a binding or continue with a candidate that lost the persistence race.
func (a *App) goalEvaluationAssignment(ctx context.Context, request goalEvaluationSnapshot) (providerAssignment, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return providerAssignment{}, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		var raw, resultRaw []byte
		var status string
		var claim int
		var started, snapshotRequired bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT model_provenance,status,claim_revision,result,EXISTS(SELECT 1 FROM public.agent_runs ar WHERE ar.fluctlight_id=$2 AND ar.agent_id=$3 AND strpos(ar.run_id,$1::text||':claim:')=1),model_snapshot_required FROM public.goal_evaluation_requests WHERE id=$1`, request.RequestID, request.FluctlightID, FormalAgentGoalEvaluation).Scan(&raw, &status, &claim, &resultRaw, &started, &snapshotRequired); err != nil {
			return providerAssignment{}, err
		}
		if status != "processing" || claim != request.ClaimRevision {
			return providerAssignment{}, ErrConflict
		}
		var provenance map[string]any
		if err := json.Unmarshal(raw, &provenance); err != nil || provenance == nil {
			return providerAssignment{}, errGoalEvaluationModelSnapshotInvalid
		}
		if !snapshotRequired && len(provenance) == 0 {
			a.recordDiagnosticEvent(ctx, "goal_evaluation.model_binding.legacy_unfrozen", "warn", request.FluctlightID, request.RequestID, request.RequestID, map[string]any{"reason_code": "legacy_model_binding_unavailable"})
			return provider.assignment(ctx, goalEvaluationModelRole)
		}
		if !snapshotRequired || provenance[goalEvaluationModelSnapshotContractKey] != workflowModelSnapshotVersion {
			return providerAssignment{}, errGoalEvaluationModelSnapshotInvalid
		}
		stored, found, err := readWorkflowModelSnapshot(provenance, goalEvaluationModelSnapshotsKey, goalEvaluationModelStage, FormalAgentGoalEvaluation, goalEvaluationModelVariant, goalEvaluationModelRole, errGoalEvaluationModelSnapshotInvalid)
		if err != nil {
			return providerAssignment{}, err
		}
		selected := false
		if rawSelected, exists := provenance[goalEvaluationModelSelectedKey]; exists {
			var valid bool
			selected, valid = rawSelected.(bool)
			if !valid || (found && !selected) {
				return providerAssignment{}, errGoalEvaluationModelSnapshotInvalid
			}
		}
		if !found && (selected || started || len(goalSubmissionRecords(decodeObject(resultRaw))) > 0) {
			return providerAssignment{}, errGoalEvaluationModelSnapshotInvalid
		}
		if found {
			assignment := stored.Assignment
			if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
				return providerAssignment{}, fmt.Errorf("goal_evaluation_model_snapshot_restore: %w", err)
			}
			return assignment, nil
		}
		assignment, err := provider.assignment(ctx, goalEvaluationModelRole)
		if err != nil {
			return providerAssignment{}, err
		}
		stored = newWorkflowModelSnapshot(FormalAgentGoalEvaluation, goalEvaluationModelVariant, goalEvaluationModelRole, assignment, 0, 0)
		check := map[string]any{goalEvaluationModelSnapshotsKey: map[string]any{goalEvaluationModelStage: stored}}
		if _, _, err := readWorkflowModelSnapshot(check, goalEvaluationModelSnapshotsKey, goalEvaluationModelStage, FormalAgentGoalEvaluation, goalEvaluationModelVariant, goalEvaluationModelRole, errGoalEvaluationModelSnapshotInvalid); err != nil {
			return providerAssignment{}, err
		}
		tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.goal_evaluation_requests SET model_provenance=model_provenance||jsonb_build_object($4::text,COALESCE(model_provenance->$4::text,'{}'::jsonb)||jsonb_build_object($5::text,$6::jsonb),$8::text,true) WHERE id=$1 AND status='processing' AND claim_revision=$2 AND model_snapshot_required AND model_provenance->>$3::text=$7 AND NOT (COALESCE(model_provenance->$4::text,'{}'::jsonb) ? $5::text)`, request.RequestID, request.ClaimRevision, goalEvaluationModelSnapshotContractKey, goalEvaluationModelSnapshotsKey, goalEvaluationModelStage, jsonBytes(stored), workflowModelSnapshotVersion, goalEvaluationModelSelectedKey)
		if err != nil {
			return providerAssignment{}, err
		}
		if tag.RowsAffected() == 1 {
			return assignment, nil
		}
		// Another caller may have persisted the winner for this same claim.
	}
	return providerAssignment{}, fmt.Errorf("goal_evaluation_model_snapshot_not_saved: %w", ErrConflict)
}
