package core

import (
	"encoding/json"
	"errors"
	"fmt"
)

const compositeActionSchemaVersion = "fluctlight.composite-action.v1"

// CompositeActionV1 describes one business output. A ConversationTurn or a
// Moment may contain text plus calls to one or more external capability slots;
// the target is represented by OutputBindings rather than by inventing
// target-specific Tool names.
type CompositeActionV1 struct {
	SchemaVersion  string            `json:"schema_version"`
	Kind           string            `json:"kind"`
	ActionType     string            `json:"action_type"`
	ResponseIntent string            `json:"response_intent,omitempty"`
	ToolCalls      []ToolCallV1      `json:"tool_calls"`
	OutputBindings []OutputBindingV1 `json:"output_bindings"`
}

// OutputBindingV1 connects a Tool result to the Composite Action output. The
// reference is resolved by Core after the output resource (message or Moment)
// receives its durable ID.
type OutputBindingV1 struct {
	ToolCallID string `json:"tool_call_id"`
	TargetKind string `json:"target_kind"`
	TargetRef  string `json:"target_ref"`
}

func normalizeCompositeAction(decision map[string]any, providerCalls []ToolCallV1, sourceFactID, defaultActionType string) (CompositeActionV1, error) {
	if decision == nil {
		return CompositeActionV1{}, errors.New("composite_action_missing")
	}
	action := mapValue(decision["action"])
	actionType := firstString(decision["action_type"], firstString(action["action_type"], defaultActionType))
	if actionType == "" {
		actionType = firstString(action["kind"], "")
	}
	responseIntent := firstString(decision["response_intent"], stringValue(action["response_intent"]))
	kind := ""
	switch actionType {
	case "moment":
		kind = "moment"
	case "proactive_message", "reply", "media_request":
		kind = "conversation_turn"
	case "no_op":
		kind = "none"
	default:
		// Capability-driven actions use the Tool Call name as their action type
		// in the existing Wake-up contract. They have no direct output target;
		// the caller may still persist and execute the typed capability slot.
		kind = "none"
	}

	calls := make([]ToolCallV1, 0, len(providerCalls)+len(arrayValue(decision["tool_calls"]))+1)
	seen := make(map[string]struct{})
	appendCall := func(call ToolCallV1) {
		if call.ID == "" {
			return
		}
		if _, exists := seen[call.ID]; exists {
			return
		}
		if call.SourceFactID == "" {
			call.SourceFactID = sourceFactID
		}
		if call.ProviderRequestID == "" {
			call.ProviderRequestID = "provider:" + stableDigest(sourceFactID+":"+call.ID)
		}
		if call.SchemaVersion == "" {
			call.SchemaVersion = ToolCallSchemaVersion
		}
		seen[call.ID] = struct{}{}
		calls = append(calls, call)
	}
	for _, call := range providerCalls {
		appendCall(call)
	}
	for _, call := range toolCallsFromValue(decision["tool_calls"]) {
		appendCall(call)
	}

	// Older DailyReview providers put the visual concept in a structured
	// `moment_media_request` field. Accept it only at this compatibility
	// boundary and immediately normalize it into the same media Tool Call used
	// by ordinary conversation cognition.
	legacyConcept := mediaConceptValue(decision["moment_media_request"])
	hasMediaCall := false
	for _, call := range calls {
		if call.Name == "media.image.generate" {
			hasMediaCall = true
			break
		}
	}
	if len(legacyConcept) > 0 && !hasMediaCall {
		arguments, _ := json.Marshal(map[string]any{"concept": legacyConcept})
		appendCall(ToolCallV1{
			ID:                "legacy-media-" + stableDigest(sourceFactID+":"+jsonString(legacyConcept)),
			Name:              "media.image.generate",
			Arguments:         arguments,
			SourceFactID:      sourceFactID,
			ProviderRequestID: "provider:" + stableDigest(sourceFactID+":legacy-media"),
			SchemaVersion:     ToolCallSchemaVersion,
		})
	}

	bindings := make([]OutputBindingV1, 0, len(calls))
	targetKind := kind
	if kind == "conversation_turn" {
		targetKind = "conversation_message"
	}
	for _, call := range calls {
		if kind == "none" {
			continue
		}
		bindings = append(bindings, OutputBindingV1{ToolCallID: call.ID, TargetKind: targetKind, TargetRef: "primary_output"})
	}
	return CompositeActionV1{
		SchemaVersion:  compositeActionSchemaVersion,
		Kind:           kind,
		ActionType:     actionType,
		ResponseIntent: responseIntent,
		ToolCalls:      calls,
		OutputBindings: bindings,
	}, nil
}

func mediaConceptFromToolCall(call ToolCallV1) (map[string]any, error) {
	if call.Name != "media.image.generate" {
		return nil, fmt.Errorf("composite_media_tool_invalid: %s", call.Name)
	}
	var arguments map[string]any
	if err := json.Unmarshal(call.Arguments, &arguments); err != nil {
		return nil, errors.New("composite_media_arguments_invalid")
	}
	concept := mediaConceptValue(arguments["concept"])
	if len(concept) == 0 {
		return nil, errors.New("composite_media_concept_invalid")
	}
	return concept, nil
}

func mediaToolCallIdentity(scope string, call ToolCallV1) (string, string, string) {
	base := scope + ":" + call.ID
	return "media_intent_" + stableDigest(base), "media_workflow_" + stableDigest(base), "media_request_" + stableDigest(base)
}

// validateCompositeOutputCalls keeps output-producing Tool slots generic. A
// caller may accept any installed asynchronous capability as long as that
// slot explicitly supports the requested target kind and implements the
// deferred binding phase.
func validateCompositeOutputCalls(calls []ToolCallV1, targetKind string, registry *CapabilityRegistry) error {
	if len(calls) == 0 {
		return nil
	}
	if registry == nil {
		return errors.New("capability_registry_required")
	}
	manifests := toolManifestMap(registry.Manifests())
	for _, call := range calls {
		if err := call.Validate(manifests); err != nil {
			return err
		}
		manifest := manifests[call.Name]
		if !manifest.IsDeferredOutput() {
			return fmt.Errorf("capability %q is not an output slot", call.Name)
		}
		if !containsStringValue(stringSliceAny(manifest.TargetKinds), targetKind) {
			return fmt.Errorf("capability %q does not support target %q", call.Name, targetKind)
		}
	}
	return nil
}

func hasDeferredOutputToolCalls(calls []ToolCallV1, registry *CapabilityRegistry) bool {
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

func bindCompositeActionOutput(action CompositeActionV1, targetKind, targetRef string) CompositeActionV1 {
	bound := action
	bound.OutputBindings = make([]OutputBindingV1, 0, len(action.ToolCalls))
	for _, call := range action.ToolCalls {
		bound.OutputBindings = append(bound.OutputBindings, OutputBindingV1{ToolCallID: call.ID, TargetKind: targetKind, TargetRef: targetRef})
	}
	return bound
}

func compositeActionFromValue(value any) (CompositeActionV1, bool) {
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || string(data) == "null" {
		return CompositeActionV1{}, false
	}
	var action CompositeActionV1
	if err := json.Unmarshal(data, &action); err != nil || action.SchemaVersion == "" {
		return CompositeActionV1{}, false
	}
	return action, true
}
