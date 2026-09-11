package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const compositeActionSchemaVersion = "fluctlight.composite-action.v1"

// CompositeActionV1 describes one business output. A ConversationTurn or a
// Moment may contain text plus calls to one or more external capability slots;
// the target is represented by OutputBindings rather than by inventing
// target-specific Tool names.
type CompositeActionV1 struct {
	SchemaVersion  string `json:"schema_version"`
	Kind           string `json:"kind"`
	ActionType     string `json:"action_type"`
	ResponseIntent string `json:"response_intent,omitempty"`
	// ToolCalls is an in-memory projection only. The durable authority is the
	// sibling payload.capability_invocations array; persisting the full calls
	// here would create a second replay source.
	ToolCalls         []CapabilityInvocation `json:"-"`
	CapabilityCallIDs []string               `json:"capability_call_ids,omitempty"`
	OutputBindings    []OutputBindingV1      `json:"output_bindings"`
}

// OutputBindingV1 connects a Tool result to the Composite Action output. The
// reference is resolved by Core after the output resource (message or Moment)
// receives its durable ID.
type OutputBindingV1 struct {
	ToolCallID string `json:"tool_call_id"`
	TargetKind string `json:"target_kind"`
	TargetRef  string `json:"target_ref"`
}

func normalizeCompositeAction(decision map[string]any, providerCalls []CapabilityInvocation, sourceFactID, defaultActionType string) (CompositeActionV1, error) {
	if decision == nil {
		return CompositeActionV1{}, errors.New("composite_action_missing")
	}
	actionType := firstString(decision["action_type"], defaultActionType)
	actionType = normalizeConversationActionType(actionType)
	responseIntent := stringValue(decision["response_intent"])
	kind := ""
	switch actionType {
	case "moment":
		kind = "moment"
	case "proactive_message", "reply":
		kind = "conversation_turn"
	case "no_op":
		kind = "none"
	default:
		// Capability-driven actions use the Tool Call name as their action type
		// in the existing Wake-up contract. They have no direct output target;
		// the caller may still persist and execute the typed capability slot.
		kind = "none"
	}

	calls := make([]CapabilityInvocation, 0, len(providerCalls)+len(arrayValue(decision["capability_invocations"])))
	seen := make(map[string]struct{})
	appendCall := func(invocation CapabilityInvocation) {
		if invocation.CallID == "" {
			return
		}
		if _, exists := seen[invocation.CallID]; exists {
			return
		}
		if invocation.SourceFactID == "" {
			invocation.SourceFactID = sourceFactID
		}
		if invocation.ProviderRequestID == "" {
			invocation.ProviderRequestID = "provider:" + stableDigest(sourceFactID+":"+invocation.CallID)
		}
		if invocation.CapabilityName == "" {
			return
		}
		seen[invocation.CallID] = struct{}{}
		calls = append(calls, invocation)
	}
	for _, invocation := range providerCalls {
		appendCall(invocation)
	}
	decisionInvocations, err := capabilityInvocationsFromValue(decision["capability_invocations"])
	if err != nil {
		return CompositeActionV1{}, err
	}
	for _, invocation := range decisionInvocations {
		appendCall(invocation)
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
		bindings = append(bindings, OutputBindingV1{ToolCallID: call.CallID, TargetKind: targetKind, TargetRef: "primary_output"})
	}
	return CompositeActionV1{
		SchemaVersion:     compositeActionSchemaVersion,
		Kind:              kind,
		ActionType:        actionType,
		ResponseIntent:    responseIntent,
		ToolCalls:         calls,
		CapabilityCallIDs: capabilityCallIDs(calls),
		OutputBindings:    bindings,
	}, nil
}

func capabilityCallIDs(invocations []CapabilityInvocation) []string {
	result := make([]string, 0, len(invocations))
	for _, invocation := range invocations {
		if invocation.CallID != "" {
			result = append(result, invocation.CallID)
		}
	}
	return result
}

func normalizeConversationActionType(value string) string {
	switch strings.TrimSpace(value) {
	case "respond", "send_message":
		return "reply"
	default:
		return strings.TrimSpace(value)
	}
}

// validateCompositeOutputCapabilities keeps output-producing capability slots generic. A
// caller may accept any installed asynchronous capability as long as that
// slot explicitly supports the requested target kind and implements the
// deferred binding phase.
func validateCompositeOutputCapabilities(calls []CapabilityInvocation, targetKind string, registry *CapabilityRegistry) error {
	if len(calls) == 0 {
		return nil
	}
	if registry == nil {
		return errors.New("capability_registry_required")
	}
	for _, invocation := range calls {
		definition, ok := registry.Definition(invocation.CapabilityName)
		if !ok {
			return fmt.Errorf("capability %q is unavailable", invocation.CapabilityName)
		}
		if err := invocation.Validate(definition); err != nil {
			return err
		}
		if !definition.IsDeferredOutput() {
			// Native state/memory/affect capabilities are independent optional
			// calls that may accompany a visible output. They are executed by the
			// action runtime and must not invalidate the reply or media binding.
			continue
		}
		if !containsCapabilityTarget(definition.TargetKinds, targetKind) {
			return fmt.Errorf("capability %q does not support target %q", invocation.CapabilityName, targetKind)
		}
	}
	return nil
}

func hasDeferredOutputCapabilities(calls []CapabilityInvocation, registry *CapabilityRegistry) bool {
	if registry == nil {
		return false
	}
	for _, invocation := range calls {
		if capability, ok := registry.Lookup(invocation.CapabilityName); ok && capability.Definition().IsDeferredOutput() {
			return true
		}
	}
	return false
}

func splitDeferredOutputCapabilities(calls []CapabilityInvocation, registry *CapabilityRegistry) (deferred, immediate []CapabilityInvocation) {
	for _, invocation := range calls {
		if registry != nil {
			if definition, ok := registry.Definition(invocation.CapabilityName); ok && definition.IsDeferredOutput() {
				deferred = append(deferred, invocation)
				continue
			}
		}
		immediate = append(immediate, invocation)
	}
	return deferred, immediate
}

func bindCompositeActionOutput(action CompositeActionV1, targetKind, targetRef string) CompositeActionV1 {
	bound := action
	bound.OutputBindings = make([]OutputBindingV1, 0, len(action.ToolCalls))
	for _, call := range action.ToolCalls {
		bound.OutputBindings = append(bound.OutputBindings, OutputBindingV1{ToolCallID: call.CallID, TargetKind: targetKind, TargetRef: targetRef})
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
