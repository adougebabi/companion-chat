package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// ConversationRuntime is the single application-facing model boundary for
// phase-two conversation stages. It delegates transport to the first-phase
// Eino/ADK support and never owns a transaction or global run state.
type ConversationRuntime interface {
	RunMain(context.Context, ConversationMainInput) (ConversationRunResult, error)
	RunQueryContinuation(context.Context, QueryContinuationInput) (ConversationRunResult, error)
	RunTakeoverJudge(context.Context, TakeoverJudgeInput) (ConversationRunResult, error)
	RunTakeoverReply(context.Context, TakeoverReplyInput) (ConversationRunResult, error)
}

type conversationRuntime struct {
	app *App
}

var _ ConversationRuntime = (*conversationRuntime)(nil)

func newConversationRuntime(app *App) ConversationRuntime {
	return &conversationRuntime{app: app}
}

type ConversationMainInput struct {
	Role           string
	Messages       []map[string]any
	Definitions    []CapabilityDefinition
	SchemaName     string
	Schema         map[string]any
	EnableThinking bool
	Capability     *ConversationCapabilityContext
}

type QueryContinuationInput struct {
	Role       string
	Messages   []map[string]any
	SchemaName string
	Schema     map[string]any
}

type TakeoverJudgeInput struct {
	Role       string
	Messages   []map[string]any
	SchemaName string
	Schema     map[string]any
}

type TakeoverReplyInput struct {
	Role           string
	Messages       []map[string]any
	Definitions    []CapabilityDefinition
	SchemaName     string
	Schema         map[string]any
	EnableThinking bool
	Capability     *ConversationCapabilityContext
}

// ConversationCapabilityContext is the narrow request-scoped information the
// runtime needs to build the ADK capability bridge. It deliberately contains
// no App, repository, transaction, or mutable global state; the bridge owns
// the actual authorization and execution through CapabilityRuntime.
type ConversationCapabilityContext struct {
	FluctlightID   string
	ConversationID string
	SourceFactID   string
	ActionID       string
	Projection     ContextProjection
}

// ConversationRunResult keeps the normalized ProviderCompletion separate from
// the request-scoped ADK trace. The trace is consumed by the existing frozen
// settlement path and is never a second persistence envelope.
type ConversationRunResult struct {
	Completion ProviderCompletion
	Trace      *ADKCapabilityTrace
}

func (r *conversationRuntime) provider() (*ProviderClient, error) {
	if r == nil || r.app == nil || r.app.Provider == nil {
		return nil, errors.New("conversation_runtime_provider_unavailable")
	}
	return r.app.Provider, nil
}

func (r *conversationRuntime) RunMain(ctx context.Context, input ConversationMainInput) (ConversationRunResult, error) {
	provider, err := r.provider()
	if err != nil {
		return ConversationRunResult{}, err
	}
	ctx, trace, err := r.withCapabilityBridge(ctx, input.Capability, input.Definitions)
	if err != nil {
		return ConversationRunResult{}, err
	}
	if err := r.validateDefinitions(input.Definitions, CapabilitySurfaceConversation); err != nil {
		return ConversationRunResult{Trace: trace}, err
	}
	completion, err := provider.StructuredAssembledWithToolsSchema(ctx, input.Role, input.Messages, input.Definitions, input.SchemaName, input.Schema, input.EnableThinking)
	if err != nil {
		return ConversationRunResult{Trace: trace}, err
	}
	return ConversationRunResult{Completion: completion, Trace: trace}, nil
}

func (r *conversationRuntime) RunQueryContinuation(ctx context.Context, input QueryContinuationInput) (ConversationRunResult, error) {
	provider, err := r.provider()
	if err != nil {
		return ConversationRunResult{}, err
	}
	completion, err := provider.StructuredQueryContinuation(WithProviderScenario(ctx, "query_continuation"), input.Role, input.Messages, input.SchemaName, input.Schema)
	if err != nil {
		return ConversationRunResult{}, err
	}
	return ConversationRunResult{Completion: completion}, nil
}

func (r *conversationRuntime) RunTakeoverJudge(ctx context.Context, input TakeoverJudgeInput) (ConversationRunResult, error) {
	provider, err := r.provider()
	if err != nil {
		return ConversationRunResult{}, err
	}
	completion, err := provider.StructuredAssembledJudgement(WithProviderScenario(ctx, "takeover_judge"), input.Role, input.Messages, input.SchemaName, input.Schema)
	if err != nil {
		return ConversationRunResult{}, err
	}
	return ConversationRunResult{Completion: completion}, nil
}

func (r *conversationRuntime) RunTakeoverReply(ctx context.Context, input TakeoverReplyInput) (ConversationRunResult, error) {
	provider, err := r.provider()
	if err != nil {
		return ConversationRunResult{}, err
	}
	ctx = WithProviderScenario(ctx, "takeover_reply")
	ctx, trace, err := r.withCapabilityBridge(ctx, input.Capability, input.Definitions)
	if err != nil {
		return ConversationRunResult{}, err
	}
	if err := r.validateDefinitions(input.Definitions, CapabilitySurfaceConversation); err != nil {
		return ConversationRunResult{Trace: trace}, err
	}
	completion, err := provider.StructuredAssembledWithToolsSchema(ctx, input.Role, input.Messages, input.Definitions, input.SchemaName, input.Schema, input.EnableThinking)
	if err != nil {
		return ConversationRunResult{Trace: trace}, err
	}
	return ConversationRunResult{Completion: completion, Trace: trace}, nil
}

func (r *conversationRuntime) withCapabilityBridge(ctx context.Context, capability *ConversationCapabilityContext, definitions []CapabilityDefinition) (context.Context, *ADKCapabilityTrace, error) {
	if capability == nil {
		return nil, nil, errors.New("conversation_runtime_capability_context_required")
	}
	trace := &ADKCapabilityTrace{}
	invoker := newAppADKCapabilityInvoker(r.app, capability.FluctlightID, capability.ConversationID, capability.SourceFactID, capability.ActionID, capability.Projection, trace)
	return WithADKCapabilityInvoker(ctx, invoker, trace), trace, nil
}

// validateDefinitions makes the registry the sole authority for model-facing
// tool schemas. The caller may narrow the catalog, but it cannot replace a
// registered definition with a same-named schema whose execution contract is
// different. This check runs before any Provider I/O.
func (r *conversationRuntime) validateDefinitions(definitions []CapabilityDefinition, surface CapabilitySurface) error {
	if len(definitions) == 0 {
		return nil
	}
	if r == nil || r.app == nil {
		return errors.New("conversation_runtime_capability_registry_unavailable")
	}
	registry := r.app.capabilityRegistry()
	if registry == nil {
		return errors.New("conversation_runtime_capability_registry_unavailable")
	}
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		name := definition.Name
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("conversation_runtime_capability_definition_duplicate: %s", name)
		}
		seen[name] = struct{}{}
		canonical, ok := registry.Definition(name)
		if !ok {
			return fmt.Errorf("conversation_runtime_capability_definition_unknown: %s", name)
		}
		if canonical.InternalOnly {
			return fmt.Errorf("conversation_runtime_capability_definition_internal: %s", name)
		}
		if !canonical.SupportsSurface(surface) {
			return fmt.Errorf("conversation_runtime_capability_definition_surface_forbidden: %s:%s", name, surface)
		}
		if !reflect.DeepEqual(canonical, definition) {
			return fmt.Errorf("conversation_runtime_capability_definition_mismatch: %s", name)
		}
	}
	return nil
}
