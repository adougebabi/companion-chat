package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	runtimeSummaryModelContractKey  = "_runtime_summary_model_snapshot_contract"
	runtimeSummaryModelSnapshotsKey = "_runtime_summary_model_snapshots"
	runtimeSummaryModelStage        = "summary"
	runtimeSummaryModelVariant      = "conversation_summary_v1"
	runtimeSummaryModelRole         = "reflection"
)

var errRuntimeSummaryModelSnapshotInvalid = errors.New("runtime_summary_model_snapshot_invalid")

func (a *App) runtimeSummaryAssignment(ctx context.Context, intentID string) (providerAssignment, bool, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return providerAssignment{}, false, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		var payload, raw []byte
		var status string
		var selected bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT w.payload,w.status,s.snapshot,COALESCE(s.selected,false) FROM public.platform_workflow_intents w LEFT JOIN public.runtime_summary_model_snapshots s ON s.intent_id=w.intent_id WHERE w.intent_id=$1 AND w.intent_type='conversation.summary'`, intentID).Scan(&payload, &status, &raw, &selected); err != nil {
			return providerAssignment{}, false, err
		}
		if status != "pending" && status != "retry" && status != "started" && status != "running" {
			return providerAssignment{}, false, ErrConflict
		}
		values := decodeObject(payload)
		contract, present := values[runtimeSummaryModelContractKey]
		if !present && raw == nil {
			a.recordDiagnosticEvent(ctx, "runtime_summary.model_binding.legacy_unfrozen", "warn", stringValue(values["fluctlight_id"]), intentID, intentID, map[string]any{"reason_code": "legacy_model_binding_unavailable"})
			assignment, err := provider.assignment(ctx, runtimeSummaryModelRole)
			return assignment, false, err
		}
		if contract != workflowModelSnapshotVersion || raw == nil {
			return providerAssignment{}, false, errRuntimeSummaryModelSnapshotInvalid
		}
		var stored map[string]any
		if err := json.Unmarshal(raw, &stored); err != nil || stored == nil || selected != (len(stored) > 0) {
			return providerAssignment{}, false, errRuntimeSummaryModelSnapshotInvalid
		}
		if selected {
			provenance := map[string]any{runtimeSummaryModelSnapshotsKey: map[string]any{runtimeSummaryModelStage: stored}}
			snapshot, _, err := readWorkflowModelSnapshot(provenance, runtimeSummaryModelSnapshotsKey, runtimeSummaryModelStage, FormalAgentConversationSummary, runtimeSummaryModelVariant, runtimeSummaryModelRole, errRuntimeSummaryModelSnapshotInvalid)
			if err != nil {
				return providerAssignment{}, false, err
			}
			assignment := snapshot.Assignment
			if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
				return providerAssignment{}, false, fmt.Errorf("runtime_summary_model_snapshot_restore: %w", err)
			}
			return assignment, true, nil
		}
		assignment, err := provider.assignment(ctx, runtimeSummaryModelRole)
		if err != nil {
			return providerAssignment{}, false, err
		}
		snapshot := newWorkflowModelSnapshot(FormalAgentConversationSummary, runtimeSummaryModelVariant, runtimeSummaryModelRole, assignment, 0, 0)
		provenance := map[string]any{runtimeSummaryModelSnapshotsKey: map[string]any{runtimeSummaryModelStage: snapshot}}
		if _, _, err := readWorkflowModelSnapshot(provenance, runtimeSummaryModelSnapshotsKey, runtimeSummaryModelStage, FormalAgentConversationSummary, runtimeSummaryModelVariant, runtimeSummaryModelRole, errRuntimeSummaryModelSnapshotInvalid); err != nil {
			return providerAssignment{}, false, err
		}
		tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.runtime_summary_model_snapshots s SET snapshot=$2,selected=true WHERE s.intent_id=$1 AND NOT s.selected AND s.snapshot='{}'::jsonb AND EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id=s.intent_id AND w.intent_type='conversation.summary' AND w.status IN ('pending','retry','started','running') AND w.payload->>$3::text=$4)`, intentID, jsonBytes(snapshot), runtimeSummaryModelContractKey, workflowModelSnapshotVersion)
		if err != nil {
			return providerAssignment{}, false, err
		}
		if tag.RowsAffected() == 1 {
			return assignment, true, nil
		}
	}
	return providerAssignment{}, false, fmt.Errorf("runtime_summary_model_snapshot_not_saved: %w", ErrConflict)
}
