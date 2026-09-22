package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	personaTakeoverCapabilityName = "persona.takeover"
	personaSwitchCapabilityName   = "persona.switch"
)

type personaActionService interface {
	preparePersonaAction(context.Context, CapabilityInvocation) (CapabilityInvocation, error)
	applyPersonaAction(context.Context, CapabilityInvocation) (CapabilityResult, error)
	applyPersonaActionTx(context.Context, pgx.Tx, CapabilityInvocation) (CapabilityResult, error)
}

type personaActionCapability struct {
	name    string
	service personaActionService
}

func (c personaActionCapability) Definition() CapabilityDefinition {
	return personaActionCapabilityDefinition(c.name)
}

func (c personaActionCapability) RequiredContext() []ContextSlot { return nil }

func (c personaActionCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil {
		return invocation, errors.New("persona action service unavailable")
	}
	return c.service.preparePersonaAction(ctx, invocation)
}

func (c personaActionCapability) Execute(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "persona_action_unavailable", true, "persona action service is unavailable"), errors.New("persona action service unavailable")
	}
	return c.service.applyPersonaAction(ctx, invocation)
}

func (c personaActionCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "persona_action_unavailable", true, "persona action service is unavailable"), errors.New("persona action service unavailable")
	}
	return c.service.applyPersonaActionTx(ctx, tx, invocation)
}

func personaActionCapabilityDefinition(name string) CapabilityDefinition {
	return CapabilityDefinition{
		Name: name, Version: "v1", Type: CapabilityTypeAction,
		Description: "Apply a declared persona rule. persona.switch commits the persistent profile for future decisions; persona.takeover authorizes this run to use the returned working persona without changing the persistent profile. Use only declared profile and rule identifiers, consume rejection results, and never claim a switch before it commits.",
		InputSchema: objectSchema(map[string]any{
			"decision":          map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"rule_id":           map[string]any{"type": "string", "maxLength": 256},
			"target_profile_id": map[string]any{"type": "string", "maxLength": 128},
			"source_profile_id": map[string]any{"type": "string", "maxLength": 128},
			"trigger_id":        map[string]any{"type": "string", "maxLength": 256},
			"reason":            map[string]any{"type": "string", "maxLength": 1000},
		}, []string{"decision"}, false),
		OutputSchema:    openObjectSchema(),
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		SideEffectClass: "native_projection", SuccessBoundary: "persona_action_committed", ConcurrencyClass: "transactional",
		SupportsCancel: true, SupportsRetry: true, FailurePolicy: FailurePolicyRequiredForVisibleClaim,
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
	operationID := "policy:" + stableDigest(name+":"+string(arguments)+":"+actionID)
	callID := "policy_" + stableDigest(operationID)
	invocation := normalizeCapabilityInvocationMetadata(CapabilityInvocation{
		CallID: callID, CapabilityName: name, Arguments: arguments,
		SourceFactID: sourceFactID, ActionID: actionID, Sequence: 0,
		Metadata: InvocationMetadata{CorrelationID: "policy:" + actionID, OperationID: operationID, FluctlightID: fluctlightID, ConversationID: conversationID, Surface: CapabilitySurfaceAutonomy, Source: "policy"},
	}, fluctlightID, conversationID, sourceFactID, actionID, 0)
	invocation.Metadata.Source = "policy"
	prepared, _, err := runtime.Prepare(ctx, invocation)
	if err != nil {
		return invocation, failedCapabilityResultDetail(invocation, "persona_action_prepare_failed", false, err.Error()), err
	}
	if a.DB == nil || a.DB.Pool() == nil {
		return prepared, failedCapabilityResultDetail(prepared, "persona_action_database_unavailable", true, "persona action database is unavailable"), errors.New("persona action database unavailable")
	}
	var result CapabilityResult
	execErr := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var txErr error
		result, txErr = runtime.ExecuteTransactional(ctx, tx, prepared)
		return txErr
	})
	return prepared, result, execErr
}

var _ TransactionalCapability = personaActionCapability{}
