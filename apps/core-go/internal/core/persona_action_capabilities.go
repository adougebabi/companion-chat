package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	personaTakeoverCapabilityName = "persona.takeover"
	personaSwitchCapabilityName   = "persona.switch"
)

type personaActionCapability struct{ name string }

func (c personaActionCapability) Definition() CapabilityDefinition {
	return personaActionCapabilityDefinition(c.name)
}

func (c personaActionCapability) RequiredContext() []ContextSlot { return nil }

func (c personaActionCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	var args map[string]any
	if err := json.Unmarshal(invocation.Arguments, &args); err != nil || args == nil {
		return failedCapabilityResult(invocation, "persona_action_arguments_invalid", false), errors.New("persona action arguments invalid")
	}
	if strings.TrimSpace(stringValue(args["decision"])) == "" {
		return failedCapabilityResult(invocation, "persona_action_decision_required", false), errors.New("persona action decision required")
	}
	return CapabilityResult{
		CallID: invocation.CallID, CapabilityName: invocation.CapabilityName,
		Status: "deferred", Retryable: false,
		Output:            map[string]any{"action": invocation.CapabilityName, "decision": stringValue(args["decision"]), "status": "awaiting_domain_commit"},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "persona:" + invocation.CallID,
	}, nil
}

func personaActionCapabilityDefinition(name string) CapabilityDefinition {
	return CapabilityDefinition{
		Name: name, Version: "v1", Type: CapabilityTypeInternal, InternalOnly: true,
		Description: "Internal persona policy action; never exposed in the ordinary model catalog.",
		InputSchema: objectSchema(map[string]any{
			"decision": stringSchema(), "rule_id": stringSchema(), "target_profile_id": stringSchema(),
			"source_profile_id": stringSchema(), "trigger_id": stringSchema(), "reason": stringSchema(),
		}, []string{"decision"}, false),
		OutputSchema:    openObjectSchema(),
		Surfaces:        []CapabilitySurface{CapabilitySurfaceAutonomy},
		SideEffectClass: "policy", SuccessBoundary: "domain_commit_pending",
		SupportsCancel: true, SupportsRetry: false, FailurePolicy: FailurePolicyRequiredForVisibleClaim,
	}
}

func (a *App) executePersonaPolicyAction(ctx context.Context, name string, fluctlightID, conversationID, sourceFactID, actionID string, args map[string]any) (CapabilityInvocation, CapabilityResult, error) {
	if a == nil {
		return CapabilityInvocation{}, CapabilityResult{}, errors.New("persona_action_runtime_unavailable")
	}
	runtime := a.capabilityRuntime()
	if runtime == nil || runtime.Registry == nil {
		return CapabilityInvocation{}, CapabilityResult{}, errors.New("persona_action_runtime_unavailable")
	}
	if _, ok := runtime.Registry.Definition(name); !ok {
		return CapabilityInvocation{}, CapabilityResult{}, fmt.Errorf("persona_action_capability_missing: %s", name)
	}
	arguments, err := json.Marshal(args)
	if err != nil {
		return CapabilityInvocation{}, CapabilityResult{}, err
	}
	callID := "policy_" + stableDigest(name+":"+string(arguments)+":"+actionID)
	invocation := normalizeCapabilityInvocationMetadata(CapabilityInvocation{
		CallID: callID, CapabilityName: name, Arguments: arguments,
		SourceFactID: sourceFactID, ActionID: actionID, Sequence: 0,
		Metadata: InvocationMetadata{CorrelationID: "policy:" + actionID, FluctlightID: fluctlightID, ConversationID: conversationID, Surface: CapabilitySurfaceAutonomy, Source: "policy"},
	}, fluctlightID, conversationID, sourceFactID, actionID, 0)
	invocation.Metadata.Source = "policy"
	result, execErr := runtime.Execute(ctx, invocation)
	return invocation, result, execErr
}
