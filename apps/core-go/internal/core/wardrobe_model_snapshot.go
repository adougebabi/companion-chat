package core

import (
	"context"
	"errors"
	"fmt"
)

const (
	wardrobeModelSnapshotContractKey = "_wardrobe_model_snapshot_contract"
	wardrobeModelSnapshotsKey        = "_wardrobe_model_snapshots"
	wardrobeModelSnapshotStage       = "preparation"
	wardrobeModelVariant             = "wardrobe_preparation_v1"
	wardrobeModelRole                = "cognitive_assessment"
)

var (
	errWardrobeModelSnapshotMissing = errors.New("wardrobe_model_snapshot_missing")
	errWardrobeModelSnapshotInvalid = errors.New("wardrobe_model_snapshot_invalid")
)

// The preparation's existing advisory lock serializes both stages. Persist the
// selected binding before Provider I/O so a restarted worker cannot rebind it.
func (a *App) wardrobePreparationAssignment(ctx context.Context, id string, provenance map[string]any, messages []map[string]any) (providerAssignment, error) {
	snapshot, found, err := readWorkflowModelSnapshot(provenance, wardrobeModelSnapshotsKey, wardrobeModelSnapshotStage, FormalAgentWardrobePreparation, wardrobeModelVariant, wardrobeModelRole, errWardrobeModelSnapshotInvalid)
	if err != nil {
		return providerAssignment{}, err
	}
	if found {
		if _, err := a.modelTaskProvider(); err != nil {
			return providerAssignment{}, err
		}
		assignment := snapshot.Assignment
		if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
			return providerAssignment{}, err
		}
		return assignment, nil
	}
	if provenance[wardrobeModelSnapshotContractKey] != workflowModelSnapshotVersion {
		return providerAssignment{}, errWardrobeModelSnapshotMissing
	}
	provider, err := a.modelTaskProvider()
	if err != nil {
		return providerAssignment{}, err
	}
	assignment, err := provider.assignment(ctx, wardrobeModelRole)
	if err != nil {
		diagnosticCtx := ensureProviderAttemptIdentity(WithProviderScenario(ctx, "wardrobe_preparation"))
		diagnosticMessages := formalPromptMessages(FormalAgentWardrobePreparation, formatProviderMessagesForRole(messages, wardrobeModelRole))
		correlation := firstString(providerCorrelation(ctx), diagnosticCorrelation(diagnosticMessages, ""))
		provider.recordProviderPreflightFailure(diagnosticCtx, providerAssignment{}, wardrobeModelRole, correlation, "assignment", diagnosticMessages, err)
		return providerAssignment{}, err
	}
	snapshot = newWorkflowModelSnapshot(FormalAgentWardrobePreparation, wardrobeModelVariant, wardrobeModelRole, assignment, 0, 0)
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations SET model_provenance=model_provenance||jsonb_build_object($4::text,COALESCE(model_provenance->$4::text,'{}'::jsonb)||jsonb_build_object($5::text,$6::jsonb)),updated_at=now() WHERE id=$1 AND status<>'completed' AND model_provenance->>$2::text=$3 AND NOT (COALESCE(model_provenance->$4::text,'{}'::jsonb) ? $5::text)`, id, wardrobeModelSnapshotContractKey, workflowModelSnapshotVersion, wardrobeModelSnapshotsKey, wardrobeModelSnapshotStage, jsonBytes(snapshot))
	if err != nil {
		return providerAssignment{}, err
	}
	if tag.RowsAffected() != 1 {
		return providerAssignment{}, fmt.Errorf("wardrobe_model_snapshot_not_saved: %w", ErrConflict)
	}
	return assignment, nil
}
