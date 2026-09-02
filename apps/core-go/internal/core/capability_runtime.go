package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (a *App) capabilityRegistry() *CapabilityRegistry {
	if a != nil && a.Capabilities != nil {
		return a.Capabilities
	}
	return NewCapabilityRegistry(
		&imageCapabilityExecutor{app: a},
		&sceneCapabilityExecutor{app: a},
		&presenceCapabilityExecutor{app: a},
		&memoryCapabilityExecutor{app: a},
		&capabilityRequestExecutor{app: a},
	)
}

// ExecuteToolCalls is the Runtime-owned capability boundary.  It validates a
// normalized call, then invokes only a registered native or external slot.
// Each executor is replaceable while the Runtime retains domain authority.
func (a *App) ExecuteToolCalls(ctx context.Context, fluctlightID, conversationID, sourceFactID string, calls []ToolCallV1) ([]ToolResultV1, error) {
	if len(calls) == 0 {
		return []ToolResultV1{}, nil
	}
	registry := a.capabilityRegistry()
	manifests := toolManifestMap(registry.Manifests())
	results := make([]ToolResultV1, 0, len(calls))
	for _, call := range normalizeToolCallMetadata(calls, sourceFactID, sourceFactID) {
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
		manifest := manifests[call.Name]
		if manifest.IsDeferredOutput() {
			// External async slots are output-producing capabilities. Their
			// durable intent must be created only after the Composite Action has
			// a concrete target (for example a newly persisted message). The
			// caller settles this deferred result at that boundary.
			result := deferredToolResult(call, "output_target_pending")
			results = append(results, result)
			continue
		}
		result, err := executor.Execute(ctx, fluctlightID, conversationID, sourceFactID, call)
		results = append(results, result)
		if validationErr := result.Validate(call); validationErr != nil {
			return results, validationErr
		}
		if validationErr := manifest.ValidateOutput(result.Output); validationErr != nil {
			return results, validationErr
		}
		if call.ActionID != "" {
			if persistErr := a.persistActionResult(ctx, fluctlightID, call.ActionID, sourceFactID, result); persistErr != nil {
				return results, persistErr
			}
		}
		if err != nil {
			return results, err
		}
	}
	return results, nil
}

func deferredToolResult(call ToolCallV1, reason string) ToolResultV1 {
	return ToolResultV1{
		ToolCallID:        call.ID,
		Name:              call.Name,
		Status:            "deferred",
		Output:            map[string]any{"reason": reason},
		Retryable:         true,
		ProviderRequestID: call.ProviderRequestID,
		CorrelationID:     "tool:" + strings.TrimSpace(call.ID),
		SchemaVersion:     ToolResultSchemaVersion,
	}
}

func normalizeToolCallMetadata(calls []ToolCallV1, sourceFactID, identityScope string) []ToolCallV1 {
	result := make([]ToolCallV1, len(calls))
	copy(result, calls)
	for index := range result {
		if result[index].SourceFactID == "" {
			result[index].SourceFactID = sourceFactID
		}
		if result[index].ProviderRequestID == "" {
			result[index].ProviderRequestID = "provider:" + stableDigest(identityScope+":"+result[index].ID)
		}
		if result[index].SchemaVersion == "" {
			result[index].SchemaVersion = ToolCallSchemaVersion
		}
		if result[index].Sequence < 0 {
			result[index].Sequence = index
		}
	}
	return result
}

type imageCapabilityExecutor struct{ app *App }

func (executor *imageCapabilityExecutor) Manifest() CapabilityManifest {
	return imageCapabilityManifest()
}

func (executor *imageCapabilityExecutor) Execute(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	return executor.app.executeImageToolCall(ctx, fluctlightID, conversationID, sourceFactID, call)
}

// DeferredCapabilityExecutor is the optional second phase for an external
// async slot. Runtime first validates/records the call, then supplies the
// concrete Composite Action binding once the output resource has an ID.
// Implementations own their provider-intent persistence while Runtime owns
// when this phase is allowed to run.
type DeferredCapabilityExecutor interface {
	CapabilityExecutor
	ExecuteDeferredTx(context.Context, pgx.Tx, string, string, string, ToolCallV1, OutputBindingV1) (ToolResultV1, error)
}

func (executor *imageCapabilityExecutor) ExecuteDeferredTx(ctx context.Context, tx pgx.Tx, fluctlightID, sourceFactID, identityScope string, call ToolCallV1, binding OutputBindingV1) (ToolResultV1, error) {
	concept, err := mediaConceptFromToolCall(call)
	if err != nil {
		return failedToolResult(call, "media_concept_invalid", false, err.Error()), err
	}
	manifest := executor.Manifest()
	if !containsStringValue(stringSliceAny(manifest.TargetKinds), binding.TargetKind) {
		return failedToolResult(call, "tool_target_invalid", false, "capability does not support this output target"), errors.New("tool target invalid")
	}
	conversationID, messageID, momentID := "", "", ""
	switch binding.TargetKind {
	case "conversation_message":
		messageID = binding.TargetRef
	case "moment":
		momentID = binding.TargetRef
	default:
		return failedToolResult(call, "tool_target_invalid", false, "unsupported output target"), errors.New("tool target invalid")
	}
	intentID, workflowID, providerRequestID := mediaToolCallIdentity(identityScope, call)
	if err := executor.app.createMediaIntentTargetTx(ctx, tx, fluctlightID, concept, intentID, workflowID, providerRequestID, conversationID, messageID, momentID); err != nil {
		return failedToolResult(call, "media_intent_failed", true, err.Error()), err
	}
	return ToolResultV1{
		ToolCallID:        call.ID,
		Name:              call.Name,
		Status:            "completed",
		Output:            map[string]any{"media_intent_id": intentID, "target_kind": binding.TargetKind, "target_ref": binding.TargetRef},
		Retryable:         false,
		ProviderRequestID: providerRequestID,
		CorrelationID:     "tool:" + call.ID,
		SchemaVersion:     ToolResultSchemaVersion,
	}, nil
}

func stringSliceAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func (a *App) settleDeferredToolCallsTx(ctx context.Context, tx pgx.Tx, fluctlightID, sourceFactID, identityScope string, calls []ToolCallV1, existing []ToolResultV1, binding OutputBindingV1) ([]ToolResultV1, error) {
	if len(calls) == 0 {
		return existing, nil
	}
	registry := a.capabilityRegistry()
	manifests := toolManifestMap(registry.Manifests())
	results := append([]ToolResultV1(nil), existing...)
	for _, call := range normalizeToolCallMetadata(calls, sourceFactID, identityScope) {
		if err := call.Validate(manifests); err != nil {
			return results, err
		}
		if !toolCallNeedsDeferredSettlement(call, registry) {
			continue
		}
		if result, ok := toolResultForCall(results, call.ID); ok && result.Status == "completed" {
			continue
		}
		executor, ok := registry.Lookup(call.Name)
		if !ok {
			return results, fmt.Errorf("capability %q has no executor", call.Name)
		}
		deferred, ok := executor.(DeferredCapabilityExecutor)
		if !ok {
			return results, fmt.Errorf("capability %q cannot bind output target", call.Name)
		}
		callBinding := binding
		callBinding.ToolCallID = call.ID
		result, err := deferred.ExecuteDeferredTx(ctx, tx, fluctlightID, sourceFactID, identityScope, call, callBinding)
		if result.SchemaVersion == "" {
			result.SchemaVersion = ToolResultSchemaVersion
		}
		if validationErr := result.Validate(call); validationErr != nil {
			return results, validationErr
		}
		if validationErr := manifests[call.Name].ValidateOutput(result.Output); validationErr != nil {
			return results, validationErr
		}
		if err != nil {
			return results, err
		}
		results = replaceToolResult(results, result)
	}
	return results, nil
}

func toolCallNeedsDeferredSettlement(call ToolCallV1, registry *CapabilityRegistry) bool {
	if registry == nil {
		return false
	}
	executor, ok := registry.Lookup(call.Name)
	return ok && executor.Manifest().IsDeferredOutput()
}

func toolResultForCall(results []ToolResultV1, callID string) (ToolResultV1, bool) {
	for _, result := range results {
		if result.ToolCallID == callID {
			return result, true
		}
	}
	return ToolResultV1{}, false
}

func replaceToolResult(results []ToolResultV1, replacement ToolResultV1) []ToolResultV1 {
	for index := range results {
		if results[index].ToolCallID == replacement.ToolCallID {
			results[index] = replacement
			return results
		}
	}
	return append(results, replacement)
}

type sceneCapabilityExecutor struct{ app *App }

func (executor *sceneCapabilityExecutor) Manifest() CapabilityManifest {
	return sceneCapabilityManifest()
}

func (executor *sceneCapabilityExecutor) Execute(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	return executor.app.applySceneCapability(ctx, fluctlightID, conversationID, sourceFactID, call)
}

type presenceCapabilityExecutor struct{ app *App }

func (executor *presenceCapabilityExecutor) Manifest() CapabilityManifest {
	return presenceCapabilityManifest()
}

type memoryCapabilityExecutor struct{ app *App }

func (executor *memoryCapabilityExecutor) Manifest() CapabilityManifest {
	return memoryCapabilityManifest()
}

func (executor *memoryCapabilityExecutor) Execute(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	return executor.app.applyMemoryCapability(ctx, fluctlightID, conversationID, sourceFactID, call)
}

func (executor *presenceCapabilityExecutor) Execute(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	return executor.app.applyPresenceCapability(ctx, fluctlightID, conversationID, sourceFactID, call)
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
	data, err := json.Marshal(toolCallArrayValue(value))
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
	if len(calls) == 0 {
		return "", nil, errors.New("at least one capability call is required for this turn")
	}
	action := "reply"
	for _, call := range calls {
		if err := call.Validate(manifests); err != nil {
			return "", nil, err
		}
		// Every registered slot remains a capability of the same reply
		// Composite Action. Whether it is immediate or deferred is determined
		// by its manifest, never by embedding the tool name in an action type.
	}
	return action, nil, nil
}

func toolCallsRequireDeferredOutput(calls []ToolCallV1, registry *CapabilityRegistry) bool {
	if registry == nil {
		return false
	}
	for _, call := range calls {
		if executor, ok := registry.Lookup(call.Name); ok && executor.Manifest().IsDeferredOutput() {
			return true
		}
	}
	return false
}
