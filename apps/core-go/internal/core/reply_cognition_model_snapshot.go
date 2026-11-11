package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	replyCognitionSnapshotKey     = "cognition_model_snapshot"
	replyCognitionSnapshotVariant = "conversation_turn_response"
	replyCognitionSnapshotRole    = "cognitive_assessment"
)

var errReplyCognitionSnapshotInvalid = errors.New("reply_cognition_model_snapshot_invalid")

// Only the original conversation cognition Agent can establish this binding;
// WakeUp and independently invoked reply Tools have different model owners.
func replyCognitionSnapshotFromContext(ctx context.Context) (workflowModelSnapshot, bool) {
	route, ok := ctx.Value(agentModelRouteKey{}).(agentModelRoute)
	if !ok || route.agent != FormalAgentConversationCognition || route.variant != replyCognitionSnapshotVariant {
		return workflowModelSnapshot{}, false
	}
	assignment, ok := providerAssignmentSnapshot(ctx, replyCognitionSnapshotRole)
	if !ok {
		return workflowModelSnapshot{}, false
	}
	return newWorkflowModelSnapshot(FormalAgentConversationCognition, replyCognitionSnapshotVariant, replyCognitionSnapshotRole, assignment, 0, 0), true
}

func (a *App) restoreReplyCognitionSnapshot(ctx context.Context, binding map[string]any) (context.Context, bool, error) {
	raw, present := binding[replyCognitionSnapshotKey]
	if !present {
		return ctx, false, nil
	}
	var snapshot workflowModelSnapshot
	if err := json.Unmarshal(jsonBytes(raw), &snapshot); err != nil || snapshot.Version != workflowModelSnapshotVersion || snapshot.Agent != FormalAgentConversationCognition || snapshot.Variant != replyCognitionSnapshotVariant || snapshot.Role != replyCognitionSnapshotRole || (snapshot.Assignment.Role != "generic_llm" && snapshot.Assignment.Role != conversationReplyModelRole) || strings.TrimSpace(snapshot.Assignment.EndpointID) == "" || strings.TrimSpace(snapshot.Assignment.ModelID) == "" || strings.TrimSpace(snapshot.Assignment.BaseURL) == "" || snapshot.Assignment.Timeout <= 0 || snapshot.Assignment.Secret != "" {
		return ctx, true, errReplyCognitionSnapshotInvalid
	}
	assignment := snapshot.Assignment
	if err := a.restoreAssignmentCredential(ctx, &assignment); err != nil {
		return ctx, true, fmt.Errorf("reply_cognition_model_snapshot_restore: %w", err)
	}
	ctx = withAgentModelRoute(ctx, FormalAgentConversationCognition, replyCognitionSnapshotVariant)
	return withFrozenProviderAssignment(ctx, replyCognitionSnapshotRole, assignment), true, nil
}
