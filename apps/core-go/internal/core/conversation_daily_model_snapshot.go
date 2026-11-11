package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	conversationDailyModelContractKey  = "_daily_memory_model_snapshot_contract"
	conversationDailyModelSnapshotsKey = "_daily_memory_model_snapshots"
	conversationDailyModelStage        = "episode"
	conversationDailyModelVariant      = "conversation_daily_memory_v1"
	conversationDailyModelRole         = "reflection"
)

var errConversationDailyModelSnapshotInvalid = errors.New("conversation_daily_memory_model_snapshot_invalid")

// The intent and generation own one binding; the separate row avoids changing
// the payload used by the Temporal dispatch and recovery comparisons.
func (a *App) conversationDailyAssignment(ctx context.Context, intentID string, generation int64) (providerAssignment, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return providerAssignment{}, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		var payload, raw []byte
		var status string
		var selected bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT w.payload,w.status,s.snapshot,COALESCE(s.selected,false) FROM public.platform_workflow_intents w LEFT JOIN public.conversation_daily_model_snapshots s ON s.intent_id=w.intent_id AND s.source_generation=$2 WHERE w.intent_id=$1 AND w.intent_type='conversation.daily_memory'`, intentID, generation).Scan(&payload, &status, &raw, &selected); err != nil {
			return providerAssignment{}, err
		}
		if err := requireConversationDailyGeneration(payload, generation); err != nil {
			return providerAssignment{}, err
		}
		if status != "pending" && status != "retry" && status != "started" && status != "running" {
			return providerAssignment{}, ErrConflict
		}
		values := decodeObject(payload)
		contract, present := values[conversationDailyModelContractKey]
		if !present && raw == nil {
			a.recordDiagnosticEvent(ctx, "conversation_daily_memory.model_binding.legacy_unfrozen", "warn", stringValue(values["fluctlight_id"]), intentID, intentID, map[string]any{"reason_code": "legacy_model_binding_unavailable"})
			return provider.assignment(ctx, conversationDailyModelRole)
		}
		if contract != workflowModelSnapshotVersion || raw == nil {
			return providerAssignment{}, errConversationDailyModelSnapshotInvalid
		}
		var stored map[string]any
		if err := json.Unmarshal(raw, &stored); err != nil || stored == nil {
			return providerAssignment{}, errConversationDailyModelSnapshotInvalid
		}
		if selected != (len(stored) > 0) {
			return providerAssignment{}, errConversationDailyModelSnapshotInvalid
		}
		if len(stored) > 0 {
			provenance := map[string]any{conversationDailyModelSnapshotsKey: map[string]any{conversationDailyModelStage: stored}}
			snapshot, _, err := readWorkflowModelSnapshot(provenance, conversationDailyModelSnapshotsKey, conversationDailyModelStage, FormalAgentConversationSummary, conversationDailyModelVariant, conversationDailyModelRole, errConversationDailyModelSnapshotInvalid)
			if err != nil {
				return providerAssignment{}, err
			}
			assignment := snapshot.Assignment
			if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
				return providerAssignment{}, fmt.Errorf("conversation_daily_memory_model_snapshot_restore: %w", err)
			}
			return assignment, nil
		}
		assignment, err := provider.assignment(ctx, conversationDailyModelRole)
		if err != nil {
			return providerAssignment{}, err
		}
		snapshot := newWorkflowModelSnapshot(FormalAgentConversationSummary, conversationDailyModelVariant, conversationDailyModelRole, assignment, 0, 0)
		provenance := map[string]any{conversationDailyModelSnapshotsKey: map[string]any{conversationDailyModelStage: snapshot}}
		if _, _, err := readWorkflowModelSnapshot(provenance, conversationDailyModelSnapshotsKey, conversationDailyModelStage, FormalAgentConversationSummary, conversationDailyModelVariant, conversationDailyModelRole, errConversationDailyModelSnapshotInvalid); err != nil {
			return providerAssignment{}, err
		}
		tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.conversation_daily_model_snapshots s SET snapshot=$3,selected=true WHERE s.intent_id=$1 AND s.source_generation=$2 AND NOT s.selected AND s.snapshot='{}'::jsonb AND EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id=s.intent_id AND w.intent_type='conversation.daily_memory' AND w.status IN ('pending','retry','started','running') AND COALESCE(w.payload->>'source_generation','0')=$2::text AND w.payload->>$4::text=$5)`, intentID, generation, jsonBytes(snapshot), conversationDailyModelContractKey, workflowModelSnapshotVersion)
		if err != nil {
			return providerAssignment{}, err
		}
		if tag.RowsAffected() == 1 {
			return assignment, nil
		}
		// A concurrent selector may have saved the winner for this generation.
	}
	return providerAssignment{}, fmt.Errorf("conversation_daily_memory_model_snapshot_not_saved: %w", ErrConflict)
}
