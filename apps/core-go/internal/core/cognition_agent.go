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
	CorrelationID        string
	CurrentInput         string
	EnableStreaming      bool
	AuthorizationPolicy  string
	Surface              CapabilitySurface
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
// cognition. Only native reply Tools publish during the loop; callers project
// committed receipts and never publish the final cognition text.
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
	var scopeErr error
	ctx, scopeErr = a.withDomainPluginScope(ctx)
	if scopeErr != nil {
		return ConversationCognitionAgentResult{}, scopeErr
	}

	conversationID := strings.TrimSpace(input.ConversationID)
	memoryOperation := MemoryForNativeCognition
	memoryMode := MemoryConversationGlobalOnly
	if conversationID != "" {
		memoryOperation = MemoryForConversation
		memoryMode = MemoryConversationExact
	}
	profile := ""
	if run := replyRunFromContext(ctx); run != nil {
		profile = run.Scope.WorkingProfileID
	}
	projectionRequest := ContextProjectionRequest{WorkingProfileID: profile,
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

	ctx, err = a.withPrivateReplyRun(ctx, ToolExecutionRequest{AuthorizationActorID: actorID, SubjectActorID: firstString(input.SpeakerActorID, actorID),
		FluctlightID: fluctlightID, ConversationID: conversationID, EvidenceID: input.SourceFactID, RunID: runID, OperationID: runID, Surface: CapabilitySurfaceConversation}, currentInput)
	if err != nil {
		return ConversationCognitionAgentResult{}, err
	}
	replyRetry := directReplyRetryFromContext(ctx)
	if replyRetry.intentID != "" {
		registry := a.capabilityRegistryForContext(ctx)
		entry, _ := registry.Lookup(conversationReplyCapabilityName)
		reply, direct := entry.(directMainReplyCapability)
		if !direct {
			return ConversationCognitionAgentResult{}, ErrConflict
		}
		receipt, err := directReplyRetryImage(ctx, a.DB.Pool(), registry, input.SourceFactID, actorID, fluctlightID, conversationID)
		if err != nil {
			return ConversationCognitionAgentResult{}, err
		}
		if receipt.intentID != replyRetry.intentID {
			return ConversationCognitionAgentResult{}, ErrConflict
		}
		reply.replyOnly = true
		view, err := NewCapabilityRegistry(reply)
		if err != nil {
			return ConversationCognitionAgentResult{}, err
		}
		ctx = withCapabilityRunRegistry(context.WithValue(ctx, directReplyRetryContextKey{}, receipt), view)
		stable := providerStableTaskContext(ctx)
		if stable == nil {
			stable = map[string]any{}
		}
		stable["reply_recovery"] = map[string]any{"image_task_status": receipt.status, "image_delivered": receipt.delivered}
		ctx = withProviderStableTaskContext(ctx, stable)
	}
	if run := replyRunFromContext(ctx); run != nil {
		if run.Settings.Mode == "direct" {
			var previewEnabled bool
			ctx, previewEnabled = a.withConversationCandidatePreview(ctx, input.SourceFactID)
			input.EnableStreaming = input.EnableStreaming || previewEnabled
		}
		if main, _ := ctx.Value(directMainConversationKey{}).(bool); main {
			ctx, err = a.withDirectTurnCognitionSnapshot(ctx, input, run.Settings.Mode == "direct")
			if err != nil {
				return ConversationCognitionAgentResult{}, err
			}
		}
	}
	definitions := capabilityCatalog(a.capabilityRegistryForContext(ctx), CapabilitySurfaceConversation)
	if !isMultiPersonalityProjection(projection) {
		definitions = filterPersonaActionCapabilities(definitions)
	}
	schema := cognitiveTurnResponseSchema()
	operationRules := []string{providerContextAuthorityRule, capabilityConversationPolicyInstruction, capabilityLifeConsistencyInstruction}
	surface := ProviderContextSurfaceConversationMain
	var expressionRefs []string
	expressionSelected := false
	imageDiscovery := false
	if replyRun := replyRunFromContext(ctx); replyRun != nil && replyRun.Settings.Mode == "fact_only" {
		surface = ProviderContextSurfaceFactOnlyCognition
		operationRules = []string{factOnlyExecutionBoundary}
		projection = privateTextChatProjection(projection)
	} else if replyRun != nil && replyRun.Settings.Mode == "direct" {
		operationRules = []string{providerContextAuthorityRule, directMainConversationProtocol, factOnlyExecutionBoundary}
		main, _ := ctx.Value(directMainConversationKey{}).(bool)
		if replyRetry.intentID != "" {
			operationRules = []string{providerContextAuthorityRule, directReplyRetryProtocol}
		} else if main && a.directImageDiscoveryNeeded(ctx, projection, currentInput, definitions) {
			imageDiscovery = true
			ctx = context.WithValue(ctx, directImageDiscoveryPromptKey{}, directImageDiscoveryPrompt(projection, currentInput))
		}
		suggestions, suggestionErr := a.freezeDirectExpressionSuggestions(ctx, replyRun)
		if suggestionErr != nil {
			return ConversationCognitionAgentResult{}, suggestionErr
		}
		if len(suggestions) > 0 {
			expressionSelected = true
			for _, suggestion := range suggestions {
				if ref := taskOptionalSourceRef("expression_candidate", suggestion.ID); len(ref) > 0 {
					expressionRefs = append(expressionRefs, ref...)
				}
			}
			stable := providerStableTaskContext(ctx)
			if stable == nil {
				stable = map[string]any{}
			}
			if _, exists := stable["expression_suggestions"]; exists {
				return ConversationCognitionAgentResult{}, errors.New("direct_expression_context_conflict")
			}
			stable["expression_suggestions"] = expressionSuggestionPromptData(suggestions)
			ctx = withProviderStableTaskContext(ctx, stable)
			operationRules = append(operationRules, expressionSuggestionContextRule)
		}
		projection = privateTextChatProjection(projection)
	}
	assembly, projection, err := a.assembleProjectionPromptForSurface(
		ctx,
		surface,
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
	assembly = declareAssembledTaskInputSlots(assembly, PromptSlotTrace{SlotID: "conversation_current_input", Source: "cognition_task_input", SourceRefs: taskOptionalSourceRef("fact", input.SourceFactID), Reason: "current_authorized_input"})
	if expressionSelected {
		assembly = appendAssembledSourceSlots(assembly, "stable_task_context", PromptSlotPosition("stable_task_context"), PromptSlotTrace{SlotID: "expression_suggestions", Source: "reviewed_expression_candidates", SourceRefs: expressionRefs, Reason: "selected_for_current_conversation"})
	}

	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "cognitive_assessment"), replyRunDiagnostics(ctx, assembly.Diagnostics))
	providerCtx = WithProviderCorrelation(providerCtx, firstString(providerCorrelation(ctx), "conversation-cognition-agent:"+runID))
	providerCtx = a.bindProjectionRefresh(providerCtx, projectionRequest, surface, "cognitive_assessment", operationRules, currentInput, definitions, "conversation_turn_response", schema, nil)
	if conversationID != "" {
		providerCtx = withPhysicalModelRequestPolicy(providerCtx, conversationPublicationPhysicalRequestPolicy{directImageDiscovery: imageDiscovery})
	}
	assembly.ResponseFormat = schema
	runInput := FormalAgentRunInput{
		Prompt:          assembly,
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
			CorrelationID:        "conversation-cognition-agent:" + firstString(input.CorrelationID, runID),
			Surface:              replyContinuationSurface(input.Surface, CapabilitySurfaceConversation),
			AuthorizationPolicy:  input.AuthorizationPolicy,
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
