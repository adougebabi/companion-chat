package core

import (
	"context"
	"errors"
	"strings"
)

// ConversationCognitionAgentInput is the application-facing input for an
// independent conversation cognition run. It deliberately does not require a
// Conversation message, cognition inbox fact, frozen action, or Main lifecycle
// state. Callers provide the authorized business resource and a stable RunID;
// the Agent owns its ContextProjection, prompt, tool catalog, model role and
// final output contract.
type ConversationCognitionAgentInput struct {
	SpeakerActorID       string
	SourceFactID         string
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	RunID                string
	CurrentInput         string
	EnableStreaming      bool
}

// ConversationCognitionAgentResult is a model result, not a conversation
// publication. Native ToolCalls have already executed through ExecuteTool and
// are exposed in Trace for audit/correlation. Reply Tools may already have
// published; this boundary does not publish a separate natural final message.
type ConversationCognitionAgentResult struct {
	Completion  ProviderCompletion
	Projection  ContextProjection
	Diagnostics map[string]any
	Trace       *ADKCapabilityTrace
}

// RunConversationCognitionAgent runs the formal conversation cognition Agent
// independently of Main and chat settlement. It uses the same prompt/context,
// capability registry, Eino loop and output schema as production conversation
// cognition. Natural final publication belongs to the caller; native reply
// Tools commit through the same publication service during the loop.
func (a *App) RunConversationCognitionAgent(ctx context.Context, input ConversationCognitionAgentInput) (ConversationCognitionAgentResult, error) {
	if a == nil || a.DB == nil || a.Provider == nil {
		return ConversationCognitionAgentResult{}, errors.New("conversation_cognition_agent_dependencies_missing")
	}
	actorID := strings.TrimSpace(input.AuthorizationActorID)
	fluctlightID := strings.TrimSpace(input.FluctlightID)
	runID := strings.TrimSpace(input.RunID)
	currentInput := strings.TrimSpace(input.CurrentInput)
	if actorID == "" || fluctlightID == "" {
		return ConversationCognitionAgentResult{}, errors.New("conversation_cognition_agent_authorized_resource_required")
	}
	if runID == "" {
		return ConversationCognitionAgentResult{}, errors.New("conversation_cognition_agent_run_id_required")
	}
	if currentInput == "" {
		return ConversationCognitionAgentResult{}, errors.New("conversation_cognition_agent_input_required")
	}

	conversationID := strings.TrimSpace(input.ConversationID)
	memoryOperation := MemoryForNativeCognition
	memoryMode := MemoryConversationGlobalOnly
	if conversationID != "" {
		memoryOperation = MemoryForConversation
		memoryMode = MemoryConversationExact
	}
	projectionRequest := ContextProjectionRequest{
		AuthorizationActorID:   actorID,
		SpeakerActorID:         firstString(input.SpeakerActorID, actorID),
		SourceFactID:           input.SourceFactID,
		FluctlightID:           fluctlightID,
		ConversationID:         conversationID,
		CurrentUserText:        currentInput,
		MemoryOperation:        memoryOperation,
		MemoryConversationMode: memoryMode,
	}
	projection, err := a.BuildContextProjectionFor(ctx, projectionRequest)
	if err != nil {
		return ConversationCognitionAgentResult{}, err
	}

	definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceConversation)
	if !isMultiPersonalityProjection(projection) {
		definitions = filterPersonaActionCapabilities(definitions)
	}
	schema := cognitiveTurnResponseSchema()
	operationRules := []string{providerContextAuthorityRule, capabilityConversationPolicyInstruction}
	assembly, projection, err := a.assembleProjectionPromptForSurface(
		ctx,
		ProviderContextSurfaceConversationMain,
		projection,
		"cognitive_assessment",
		operationRules,
		currentInput,
		definitions,
		"conversation_turn_response",
		schema,
	)
	if err != nil {
		return ConversationCognitionAgentResult{}, err
	}

	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "cognitive_assessment"), assembly.Diagnostics)
	providerCtx = WithProviderCorrelation(providerCtx, firstString(providerCorrelation(ctx), "conversation-cognition-agent:"+runID))
	providerCtx = a.bindProjectionRefresh(providerCtx, projectionRequest, ProviderContextSurfaceConversationMain, "cognitive_assessment", operationRules, currentInput, definitions, "conversation_turn_response", schema, nil)
	runInput := FormalAgentRunInput{
		Prompt:          PromptAssemblyResult{Messages: assembly.Messages, ResponseFormat: schema},
		Definitions:     definitions,
		SchemaName:      "conversation_turn_response",
		EnableThinking:  structuredThinkingEnabledForSchema("conversation_turn_response"),
		EnableStreaming: input.EnableStreaming,
		Capability: &ADKCapabilityRequest{
			AuthorizationActorID: actorID,
			SubjectActorID:       firstString(input.SpeakerActorID, actorID),
			FluctlightID:         fluctlightID,
			ConversationID:       conversationID,
			SourceFactID:         input.SourceFactID,
			OperationID:          runID,
			CorrelationID:        "conversation-cognition-agent:" + runID,
			Surface:              CapabilitySurfaceConversation,
			Projection:           projection,
		},
	}
	run, err := a.RunFormalAgent(providerCtx, FormalAgentConversationCognition, runInput)
	if run.Projection != nil {
		projection = *run.Projection
	}
	if err != nil {
		return ConversationCognitionAgentResult{Projection: projection, Diagnostics: assembly.Diagnostics, Trace: run.Trace}, err
	}
	return ConversationCognitionAgentResult{
		Completion: run.Completion, Projection: projection,
		Diagnostics: assembly.Diagnostics, Trace: run.Trace,
	}, nil
}

func isMultiPersonalityProjection(projection ContextProjection) bool {
	corePersona := corePersonaData(projection.CorePersona)
	system := mapValue(corePersona["personality_system"])
	if len(system) == 0 {
		system = mapValue(projection.PersonalitySystem)
	}
	return isMultiPersonalitySystem(system)
}

func filterPersonaActionCapabilities(definitions []CapabilityDefinition) []CapabilityDefinition {
	filtered := make([]CapabilityDefinition, 0, len(definitions))
	for _, def := range definitions {
		if def.Name == personaSwitchCapabilityName || def.Name == personaTakeoverCapabilityName {
			continue
		}
		filtered = append(filtered, def)
	}
	return filtered
}
