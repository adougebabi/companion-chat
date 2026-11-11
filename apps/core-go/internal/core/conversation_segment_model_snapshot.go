package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	conversationSegmentModelContractKey  = "_segment_model_snapshot_contract"
	conversationSegmentModelSnapshotsKey = "_segment_model_snapshots"
	conversationSegmentModelStage        = "summary"
	conversationSegmentModelVariant      = "conversation_segment_v1"
	conversationSegmentModelRole         = "reflection"
)

var errConversationSegmentModelSnapshotInvalid = errors.New("conversation_segment_model_snapshot_invalid")

// The returned flag distinguishes new frozen generations from legacy ones,
// which retain their prior live-configuration check after the model call.
func (a *App) conversationSegmentAssignment(ctx context.Context, intentID string, generation int64) (providerAssignment, bool, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return providerAssignment{}, false, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		var payload, raw []byte
		var status string
		var selected bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT w.payload,w.status,s.snapshot,COALESCE(s.selected,false) FROM public.platform_workflow_intents w LEFT JOIN public.conversation_segment_model_snapshots s ON s.intent_id=w.intent_id AND s.source_generation=$2 WHERE w.intent_id=$1 AND w.intent_type='conversation.segment'`, intentID, generation).Scan(&payload, &status, &raw, &selected); err != nil {
			return providerAssignment{}, false, err
		}
		if err := requireConversationSegmentGeneration(payload, generation); err != nil {
			return providerAssignment{}, false, err
		}
		if status != "pending" && status != "retry" && status != "started" && status != "running" {
			return providerAssignment{}, false, ErrConflict
		}
		values := decodeObject(payload)
		contract, present := values[conversationSegmentModelContractKey]
		if !present && raw == nil {
			a.recordDiagnosticEvent(ctx, "conversation_segment.model_binding.legacy_unfrozen", "warn", stringValue(values["fluctlight_id"]), intentID, intentID, map[string]any{"reason_code": "legacy_model_binding_unavailable"})
			assignment, err := provider.assignment(ctx, conversationSegmentModelRole)
			return assignment, false, err
		}
		if contract != workflowModelSnapshotVersion || raw == nil {
			return providerAssignment{}, false, errConversationSegmentModelSnapshotInvalid
		}
		var stored map[string]any
		if err := json.Unmarshal(raw, &stored); err != nil || stored == nil || selected != (len(stored) > 0) {
			return providerAssignment{}, false, errConversationSegmentModelSnapshotInvalid
		}
		if selected {
			provenance := map[string]any{conversationSegmentModelSnapshotsKey: map[string]any{conversationSegmentModelStage: stored}}
			snapshot, _, err := readWorkflowModelSnapshot(provenance, conversationSegmentModelSnapshotsKey, conversationSegmentModelStage, FormalAgentConversationSummary, conversationSegmentModelVariant, conversationSegmentModelRole, errConversationSegmentModelSnapshotInvalid)
			if err != nil {
				return providerAssignment{}, false, err
			}
			assignment := snapshot.Assignment
			if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
				return providerAssignment{}, false, fmt.Errorf("conversation_segment_model_snapshot_restore: %w", err)
			}
			return assignment, true, nil
		}
		assignment, err := provider.assignment(ctx, conversationSegmentModelRole)
		if err != nil {
			return providerAssignment{}, false, err
		}
		snapshot := newWorkflowModelSnapshot(FormalAgentConversationSummary, conversationSegmentModelVariant, conversationSegmentModelRole, assignment, 0, 0)
		provenance := map[string]any{conversationSegmentModelSnapshotsKey: map[string]any{conversationSegmentModelStage: snapshot}}
		if _, _, err := readWorkflowModelSnapshot(provenance, conversationSegmentModelSnapshotsKey, conversationSegmentModelStage, FormalAgentConversationSummary, conversationSegmentModelVariant, conversationSegmentModelRole, errConversationSegmentModelSnapshotInvalid); err != nil {
			return providerAssignment{}, false, err
		}
		tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.conversation_segment_model_snapshots s SET snapshot=$3,selected=true WHERE s.intent_id=$1 AND s.source_generation=$2 AND NOT s.selected AND s.snapshot='{}'::jsonb AND EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id=s.intent_id AND w.intent_type='conversation.segment' AND w.status IN ('pending','retry','started','running') AND COALESCE(w.payload->>'source_generation','0')=$2::text AND w.payload->>$4::text=$5)`, intentID, generation, jsonBytes(snapshot), conversationSegmentModelContractKey, workflowModelSnapshotVersion)
		if err != nil {
			return providerAssignment{}, false, err
		}
		if tag.RowsAffected() == 1 {
			return assignment, true, nil
		}
	}
	return providerAssignment{}, false, fmt.Errorf("conversation_segment_model_snapshot_not_saved: %w", ErrConflict)
}
