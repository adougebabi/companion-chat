package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (a *App) capabilityRegistry() *CapabilityRegistry {
	if a != nil && a.Capabilities != nil {
		return a.Capabilities
	}
	return NewCapabilityRegistry(&imageCapabilityExecutor{app: a})
}

// ExecuteToolCalls is the Runtime-owned capability boundary.  It validates a
// normalized call, then invokes only a registered external slot.  The first
// phase intentionally exposes one real slot (image generation); future video,
// audio, search, and notification adapters can register beside it without
// changing cognition semantics.
func (a *App) ExecuteToolCalls(ctx context.Context, fluctlightID, conversationID, sourceFactID string, calls []ToolCallV1) ([]ToolResultV1, error) {
	if len(calls) == 0 {
		return []ToolResultV1{}, nil
	}
	registry := a.capabilityRegistry()
	manifests := toolManifestMap(registry.Manifests())
	results := make([]ToolResultV1, 0, len(calls))
	for _, call := range calls {
		if call.SourceFactID == "" {
			call.SourceFactID = sourceFactID
		}
		if err := call.Validate(manifests); err != nil {
			result := failedToolResult(call, "tool_call_rejected", false, err.Error())
			results = append(results, result)
			return results, err
		}
		if call.SourceFactID != sourceFactID {
			result := failedToolResult(call, "tool_call_source_invalid", false, "source fact does not match the current turn")
			results = append(results, result)
			return results, errors.New("tool call source fact invalid")
		}
		executor, ok := registry.Lookup(call.Name)
		if !ok {
			result := failedToolResult(call, "tool_capability_unavailable", false, "capability has no Runtime executor")
			results = append(results, result)
			return results, fmt.Errorf("capability %q has no executor", call.Name)
		}
		result, err := executor.Execute(ctx, fluctlightID, conversationID, sourceFactID, call)
		results = append(results, result)
		if validationErr := result.Validate(call); validationErr != nil {
			return results, validationErr
		}
		if err != nil {
			return results, err
		}
	}
	return results, nil
}

type imageCapabilityExecutor struct{ app *App }

func (executor *imageCapabilityExecutor) Manifest() CapabilityManifest {
	return imageCapabilityManifest()
}

func (executor *imageCapabilityExecutor) Execute(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	return executor.app.executeImageToolCall(ctx, fluctlightID, conversationID, sourceFactID, call)
}

func (a *App) executeImageToolCall(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	var arguments map[string]any
	if err := json.Unmarshal(call.Arguments, &arguments); err != nil {
		return failedToolResult(call, "tool_arguments_invalid", false, err.Error()), err
	}
	concept := mapValue(arguments["concept"])
	if len(concept) == 0 {
		return failedToolResult(call, "media_concept_invalid", false, "concept is required"), errors.New("media concept is required")
	}
	// The call id is the idempotency boundary for this external effect.  A
	// retry after a crash reuses the same intent/workflow/provider request IDs.
	base := sourceFactID + ":" + call.ID
	intentID := "media_intent_" + stableDigest(base)
	workflowID := "media_workflow_" + stableDigest(base)
	providerRequestID := "media_request_" + stableDigest(base)
	if err := a.createMediaIntentWithIdentity(ctx, fluctlightID, conversationID, concept, intentID, workflowID, providerRequestID); err != nil {
		return failedToolResult(call, "media_intent_failed", true, err.Error()), err
	}
	return ToolResultV1{
		ToolCallID:        call.ID,
		Name:              call.Name,
		Status:            "completed",
		Output:            map[string]any{"media_intent_id": intentID},
		Retryable:         false,
		ProviderRequestID: providerRequestID,
		CorrelationID:     "tool:" + call.ID,
		SchemaVersion:     ToolResultSchemaVersion,
	}, nil
}

func failedToolResult(call ToolCallV1, code string, retryable bool, detail string) ToolResultV1 {
	return ToolResultV1{
		ToolCallID:        call.ID,
		Name:              call.Name,
		Status:            "failed",
		ErrorCode:         code,
		Retryable:         retryable,
		ProviderRequestID: call.ProviderRequestID,
		CorrelationID:     "tool:" + strings.TrimSpace(call.ID),
		SchemaVersion:     ToolResultSchemaVersion,
		Output:            map[string]any{"detail": detail},
	}
}

func toolCallsFromValue(value any) []ToolCallV1 {
	data, err := json.Marshal(arrayValue(value))
	if err != nil {
		return nil
	}
	var result []ToolCallV1
	if json.Unmarshal(data, &result) != nil {
		return nil
	}
	return result
}

func toolResultsFromValue(value any) []ToolResultV1 {
	data, err := json.Marshal(arrayValue(value))
	if err != nil {
		return nil
	}
	var result []ToolResultV1
	if json.Unmarshal(data, &result) != nil {
		return nil
	}
	return result
}

func mediaIntentIDFromToolResults(results []ToolResultV1) string {
	for _, result := range results {
		if result.Status != "completed" {
			continue
		}
		if output, ok := result.Output.(map[string]any); ok {
			if id := stringValue(output["media_intent_id"]); id != "" {
				return id
			}
		}
	}
	return ""
}

func resolveToolCallAction(calls []ToolCallV1, manifests map[string]CapabilityManifest) (string, map[string]any, error) {
	if len(calls) != 1 {
		return "", nil, errors.New("exactly one capability call is required for this turn")
	}
	call := calls[0]
	if err := call.Validate(manifests); err != nil {
		return "", nil, err
	}
	if call.Name != "media.image.generate" {
		return "", nil, fmt.Errorf("capability %q cannot be used as a conversation action", call.Name)
	}
	var arguments map[string]any
	if err := json.Unmarshal(call.Arguments, &arguments); err != nil {
		return "", nil, errors.New("tool call arguments invalid")
	}
	concept := mapValue(arguments["concept"])
	if len(concept) == 0 {
		return "", nil, errors.New("media concept is required")
	}
	return "media_request", concept, nil
}
