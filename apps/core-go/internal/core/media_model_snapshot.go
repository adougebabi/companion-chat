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

var (
	errMediaModelSnapshotInvalid = errors.New("media_model_snapshot_invalid")
	errMediaModelSnapshotFailure = errors.New("media_model_snapshot_failure")
)

func readMediaStageSnapshot(intent mediaIntent, agent FormalAgentID, variant, role string) (workflowModelSnapshot, bool, error) {
	if len(intent.modelProvenance) == 0 {
		return workflowModelSnapshot{}, false, nil
	}
	if intent.modelProvenance[mediaModelSnapshotContractKey] != workflowModelSnapshotVersion {
		return workflowModelSnapshot{}, false, fmt.Errorf("%w: %w", errMediaModelSnapshotFailure, errMediaModelSnapshotInvalid)
	}
	snapshot, found, err := readWorkflowModelSnapshot(intent.modelProvenance, mediaModelSnapshotsKey, variant, agent, variant, role, errMediaModelSnapshotInvalid)
	if err != nil {
		return workflowModelSnapshot{}, false, fmt.Errorf("%w: %w", errMediaModelSnapshotFailure, err)
	}
	return snapshot, found, nil
}

// Durable media intents save a model stage before its first Provider request.
// Older intents cannot prove their original selection; their existing behavior
// is retained and explicitly diagnosed until their recovery contract is migrated.
func (a *App) mediaStageAssignment(ctx context.Context, intent mediaIntent, agent FormalAgentID, variant, role string) (providerAssignment, error) {
	assignment, _, err := a.mediaStageAssignmentVersion(ctx, intent, agent, variant, role, 0)
	return assignment, err
}

func (a *App) mediaStageAssignmentVersion(ctx context.Context, intent mediaIntent, agent FormalAgentID, variant, role string, configVersion int64) (providerAssignment, int64, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return providerAssignment{}, 0, err
	}
	if intent.modelProvenance == nil {
		// Independent non-durable task entry points keep their caller's binding.
		assignment, err := provider.assignment(ctx, role)
		return assignment, configVersion, err
	}
	if len(intent.modelProvenance) == 0 {
		a.recordDiagnosticEvent(ctx, "media.model_binding.legacy_unfrozen", "warn", intent.Owner, intent.ID, "media:"+intent.ID, map[string]any{"agent_id": agent, "task_variant": variant, "reason_code": "legacy_model_binding_unavailable"})
		assignment, err := provider.assignment(ctx, role)
		return assignment, configVersion, err
	}
	snapshot, found, err := readMediaStageSnapshot(intent, agent, variant, role)
	if err != nil {
		return providerAssignment{}, 0, err
	}
	if found {
		assignment := snapshot.Assignment
		if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
			return providerAssignment{}, 0, fmt.Errorf("%w: media_model_snapshot_restore: %w", errMediaModelSnapshotFailure, err)
		}
		return assignment, snapshot.ConfigVersion, nil
	}
	assignment, err := provider.assignment(ctx, role)
	if err != nil {
		return providerAssignment{}, 0, err
	}
	snapshot = newWorkflowModelSnapshot(agent, variant, role, assignment, configVersion, 0)
	if _, _, err := readWorkflowModelSnapshot(map[string]any{mediaModelSnapshotsKey: map[string]any{variant: snapshot}}, mediaModelSnapshotsKey, variant, agent, variant, role, errMediaModelSnapshotInvalid); err != nil {
		return providerAssignment{}, 0, fmt.Errorf("%w: %w", errMediaModelSnapshotFailure, err)
	}
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET model_provenance=model_provenance||jsonb_build_object($4::text,COALESCE(model_provenance->$4::text,'{}'::jsonb)||jsonb_build_object($5::text,$6::jsonb)) WHERE id=$1 AND status IN ('pending','running') AND model_provenance->>$2::text=$3 AND NOT (COALESCE(model_provenance->$4::text,'{}'::jsonb) ? $5::text)`, intent.ID, mediaModelSnapshotContractKey, workflowModelSnapshotVersion, mediaModelSnapshotsKey, variant, jsonBytes(snapshot))
	if err != nil {
		return providerAssignment{}, 0, fmt.Errorf("%w: %w", errMediaModelSnapshotFailure, err)
	}
	if tag.RowsAffected() != 1 {
		return providerAssignment{}, 0, fmt.Errorf("%w: media_model_snapshot_not_saved: %w", errMediaModelSnapshotFailure, ErrConflict)
	}
	snapshots := mapValue(intent.modelProvenance[mediaModelSnapshotsKey])
	if snapshots == nil {
		snapshots = map[string]any{}
	}
	snapshots[variant] = snapshot
	intent.modelProvenance[mediaModelSnapshotsKey] = snapshots
	return assignment, snapshot.ConfigVersion, nil
}

func (a *App) mediaPromptTextContext(ctx context.Context, intent mediaIntent, messages []map[string]any) (context.Context, error) {
	ctx = withAgentModelRoute(WithProviderScenario(ctx, "media_prompt"), FormalAgentMediaPrompt, "media_prompt_text")
	assignment, err := a.mediaStageAssignment(ctx, intent, FormalAgentMediaPrompt, "media_prompt_text", "media_prompt")
	if err != nil {
		if provider, providerErr := a.modelTaskProvider(); providerErr == nil {
			diagnosticCtx := ensureProviderAttemptIdentity(ctx)
			diagnosticMessages := formalPromptMessages(FormalAgentMediaPrompt, formatProviderMessagesForRole(messages, "media_prompt"))
			correlation := firstString(providerCorrelation(ctx), diagnosticCorrelation(diagnosticMessages, ""))
			provider.recordProviderPreflightFailure(diagnosticCtx, providerAssignment{}, "media_prompt", correlation, "assignment", diagnosticMessages, err)
		}
		return ctx, err
	}
	return withFrozenProviderAssignment(ctx, "media_prompt", assignment), nil
}
