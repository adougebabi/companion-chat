package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type personaPreparationModelIntentKey struct{}

const (
	personaPreparationSnapshotsKey = "_persona_preparation_model_snapshots"
	personaPreparationModelStage   = "compile"
	personaPreparationModelVariant = "persona_compilation_response"
	personaPreparationModelRole    = "initialization"
)

var errPersonaPreparationModelSnapshotInvalid = errors.New("persona_preparation_model_snapshot_invalid")

func withPersonaPreparationModelIntent(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, personaPreparationModelIntentKey{}, id)
}

// The intent owns a single model binding for every profile and automatic retry.
// A row created with the intent marks the new contract; legacy intents have no
// row and retain their previous, explicitly diagnosed selection behavior.
func (a *App) personaPreparationAssignment(ctx context.Context, id string) (providerAssignment, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return providerAssignment{}, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		var raw []byte
		var status, fid string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT s.snapshot,w.status,COALESCE(w.payload->>'fluctlight_id','') FROM public.platform_workflow_intents w LEFT JOIN public.persona_preparation_model_snapshots s ON s.intent_id=w.intent_id WHERE w.intent_id=$1 AND w.intent_type=$2`, id, personaPreparationIntent).Scan(&raw, &status, &fid); err != nil {
			return providerAssignment{}, err
		}
		if status != "pending" && status != "started" && status != "retry" {
			return providerAssignment{}, ErrConflict
		}
		if raw == nil {
			a.recordDiagnosticEvent(ctx, "persona_preparation.model_binding.legacy_unfrozen", "warn", fid, id, "persona-preparation:"+id, map[string]any{"reason_code": "legacy_model_binding_unavailable"})
			return provider.assignment(ctx, personaPreparationModelRole)
		}
		var stored map[string]any
		if err := json.Unmarshal(raw, &stored); err != nil || stored == nil {
			return providerAssignment{}, errPersonaPreparationModelSnapshotInvalid
		}
		if len(stored) > 0 {
			provenance := map[string]any{personaPreparationSnapshotsKey: map[string]any{personaPreparationModelStage: stored}}
			snapshot, _, err := readWorkflowModelSnapshot(provenance, personaPreparationSnapshotsKey, personaPreparationModelStage, FormalAgentPersonaCompilation, personaPreparationModelVariant, personaPreparationModelRole, errPersonaPreparationModelSnapshotInvalid)
			if err != nil {
				return providerAssignment{}, err
			}
			assignment := snapshot.Assignment
			if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
				return providerAssignment{}, fmt.Errorf("persona_preparation_model_snapshot_restore: %w", err)
			}
			return assignment, nil
		}
		assignment, err := provider.assignment(ctx, personaPreparationModelRole)
		if err != nil {
			return providerAssignment{}, err
		}
		snapshot := newWorkflowModelSnapshot(FormalAgentPersonaCompilation, personaPreparationModelVariant, personaPreparationModelRole, assignment, 0, 0)
		provenance := map[string]any{personaPreparationSnapshotsKey: map[string]any{personaPreparationModelStage: snapshot}}
		if _, _, err := readWorkflowModelSnapshot(provenance, personaPreparationSnapshotsKey, personaPreparationModelStage, FormalAgentPersonaCompilation, personaPreparationModelVariant, personaPreparationModelRole, errPersonaPreparationModelSnapshotInvalid); err != nil {
			return providerAssignment{}, err
		}
		tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.persona_preparation_model_snapshots s SET snapshot=$2 WHERE s.intent_id=$1 AND s.snapshot='{}'::jsonb AND EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id=s.intent_id AND w.intent_type=$3 AND w.status IN ('pending','started','retry'))`, id, jsonBytes(snapshot), personaPreparationIntent)
		if err != nil {
			return providerAssignment{}, err
		}
		if tag.RowsAffected() == 1 {
			return assignment, nil
		}
		// A concurrent worker may have saved the winner; read it before any I/O.
	}
	return providerAssignment{}, fmt.Errorf("persona_preparation_model_snapshot_not_saved: %w", ErrConflict)
}
