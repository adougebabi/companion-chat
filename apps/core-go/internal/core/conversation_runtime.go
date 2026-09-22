package core

import (
	"context"
	"errors"
	"strings"
)

// ConversationRuntime is the single application-facing model boundary for
// phase-two conversation stages. It delegates transport to the first-phase
// Eino/ADK support and never owns a transaction or global run state.
type ConversationRuntime interface {
	RunMain(context.Context, ConversationMainInput) (ConversationRunResult, error)
	RunMainStream(context.Context, ConversationMainInput) (ConversationRunResult, error)
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
	AuthorizationActorID string
	SubjectActorID       string
	FluctlightID         string
	ConversationID       string
	SourceFactID         string
	ActionID             string
	OperationID          string
	Projection           ContextProjection
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
	return r.runMain(ctx, input, false)
}

func (r *conversationRuntime) RunMainStream(ctx context.Context, input ConversationMainInput) (ConversationRunResult, error) {
	return r.runMain(ctx, input, true)
}

func (r *conversationRuntime) runMain(ctx context.Context, input ConversationMainInput, streaming bool) (ConversationRunResult, error) {
	_, err := r.provider()
	if err != nil {
		return ConversationRunResult{}, err
	}
	if len(input.Definitions) > 0 && input.Capability == nil {
		return ConversationRunResult{}, errors.New("conversation_runtime_capability_context_required")
	}
	runInput := FormalAgentRunInput{
		Prompt:      PromptAssemblyResult{Messages: input.Messages, ResponseFormat: input.Schema},
		Definitions: input.Definitions, SchemaName: input.SchemaName,
		EnableThinking: input.EnableThinking, Capability: conversationADKRequest(input.Capability, providerCorrelation(ctx)),
	}
	var result ADKStructuredTaskResult
	if streaming {
		result, err = r.app.RunFormalAgentStream(ctx, FormalAgentConversationCognition, runInput)
	} else {
		result, err = r.app.RunFormalAgent(ctx, FormalAgentConversationCognition, runInput)
	}
	return ConversationRunResult{Completion: result.Completion, Trace: result.Trace}, conversationRuntimeBoundaryError(err)
}

func (r *conversationRuntime) RunTakeoverJudge(ctx context.Context, input TakeoverJudgeInput) (ConversationRunResult, error) {
	_, err := r.provider()
	if err != nil {
		return ConversationRunResult{}, err
	}
	result, err := r.app.RunFormalAgent(WithProviderScenario(ctx, "takeover_judge"), FormalAgentTakeoverJudge, FormalAgentRunInput{
		Prompt: PromptAssemblyResult{Messages: input.Messages, ResponseFormat: input.Schema}, SchemaName: input.SchemaName,
	})
	if err != nil {
		return ConversationRunResult{}, err
	}
	return ConversationRunResult{Completion: result.Completion, Trace: result.Trace}, nil
}

func (r *conversationRuntime) RunTakeoverReply(ctx context.Context, input TakeoverReplyInput) (ConversationRunResult, error) {
	_, err := r.provider()
	if err != nil {
		return ConversationRunResult{}, err
	}
	if len(input.Definitions) > 0 && input.Capability == nil {
		return ConversationRunResult{}, errors.New("conversation_runtime_capability_context_required")
	}
	result, err := r.app.RunFormalAgent(ctx, FormalAgentTakeoverReply, FormalAgentRunInput{
		Prompt:      PromptAssemblyResult{Messages: input.Messages, ResponseFormat: input.Schema},
		Definitions: input.Definitions, SchemaName: input.SchemaName,
		EnableThinking: input.EnableThinking, Capability: conversationADKRequest(input.Capability, providerCorrelation(ctx)),
	})
	return ConversationRunResult{Completion: result.Completion, Trace: result.Trace}, conversationRuntimeBoundaryError(err)
}

func conversationADKRequest(capability *ConversationCapabilityContext, correlationID string) *ADKCapabilityRequest {
	if capability == nil {
		return nil
	}
	return &ADKCapabilityRequest{
		AuthorizationActorID: firstString(capability.AuthorizationActorID, capability.Projection.OwnerActorID),
		SubjectActorID:       firstString(capability.SubjectActorID, capability.Projection.ReferenceIndex.SpeakerActorID),
		FluctlightID:         capability.FluctlightID, ConversationID: capability.ConversationID,
		SourceFactID: capability.SourceFactID, ActionID: capability.ActionID,
		OperationID:   firstString(capability.OperationID, firstString(capability.ActionID, capability.SourceFactID)),
		CorrelationID: correlationID, Surface: CapabilitySurfaceConversation,
		Projection: capability.Projection,
	}
}

func conversationRuntimeBoundaryError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if strings.HasPrefix(message, "adk_capability_") {
		return errors.New("conversation_runtime_" + strings.TrimPrefix(message, "adk_"))
	}
	return err
}
