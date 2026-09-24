package core

import (
	"context"
	"errors"
	"strings"
)

type ProjectionTaskResult struct {
	Trace       *ADKCapabilityTrace
	Completion  ProviderCompletion
	Projection  ContextProjection
	Diagnostics map[string]any
}

func (a *App) modelTaskProvider() (*ProviderClient, error) {
	if a == nil || a.Provider == nil {
		return nil, errors.New("model_task_provider_unavailable")
	}
	return a.Provider, nil
}

func (a *App) runFormalStructuredTask(ctx context.Context, id FormalAgentID, messages []map[string]any, definitions []CapabilityDefinition, schemaName string, schema map[string]any, enableThinking bool, capability *ADKCapabilityRequest) (ADKStructuredTaskResult, error) {
	return a.RunFormalAgent(ctx, id, FormalAgentRunInput{
		Prompt: PromptAssemblyResult{Messages: messages, ResponseFormat: schema}, Definitions: definitions,
		SchemaName: schemaName, EnableThinking: enableThinking, Capability: capability,
	})
}

func formalTaskCapabilityRequest(id FormalAgentID, projection ContextProjection, surface CapabilitySurface) *ADKCapabilityRequest {
	sourceFactID := strings.TrimSpace(projection.SourceFactID)
	operationRoot := sourceFactID
	if operationRoot == "" {
		operationRoot = "formal-agent:" + string(id) + ":" + stableDigest(jsonString(projection))[:24]
	}
	return &ADKCapabilityRequest{
		AuthorizationPolicy:  "autonomy",
		AuthorizationActorID: projection.OwnerActorID, FluctlightID: projection.FluctlightID,
		ConversationID: projection.ConversationID, SourceFactID: sourceFactID,
		ActionID:      "agent_action_" + stableDigest(string(id) + "\x1f" + operationRoot)[:32],
		OperationID:   "agent_run_" + stableDigest(string(id) + "\x1f" + operationRoot)[:32],
		CorrelationID: "agent:" + string(id) + ":" + operationRoot,
		Surface:       surface, Projection: projection,
	}
}

// InitializationTaskInput is the complete business input for the
// initialization task. The task owns the canonical instruction fragments and
// schema; the setup flow only supplies the Owner's description.
type InitializationTaskInput struct {
	Description string
}

func (a *App) RunInitializationTask(ctx context.Context, input InitializationTaskInput) (map[string]any, error) {
	if strings.TrimSpace(input.Description) == "" {
		return nil, errors.New("initialization_description_required")
	}
	messages := initializationAnalysisMessages(input.Description)
	messages = (&PromptComposer{}).ComposeTaskMessages("initialization", messages)
	run, err := a.runFormalStructuredTask(WithProviderScenario(ctx, "initialization"), FormalAgentInitialization, messages, nil, "initialization_response", initializationResponseSchema(), false, nil)
	return run.Completion.Structured, err
}

// MediaPromptTaskInput contains only the frozen media facts and retry
// feedback. The task selects the media_prompt instruction and serializes the
// bounded concept; callers cannot inject an arbitrary system message.
type MediaPromptTaskInput struct {
	Intent mediaIntent
}

func (a *App) RunMediaPromptTask(ctx context.Context, input MediaPromptTaskInput) (string, error) {
	messages := []map[string]any{
		{"role": "system", "content": mediaPromptSystemInstruction(input.Intent)},
		{"role": "user", "content": mediaPromptInput(input.Intent)},
	}
	messages = addVisualIdentityMediaPromptInstruction("media_prompt", messages)
	messages = formatProviderMessagesForRole(messages, "media_prompt")
	run, err := a.RunFormalAgent(WithProviderScenario(ctx, "media_prompt"), FormalAgentMediaPrompt, FormalAgentRunInput{
		Prompt: PromptAssemblyResult{Messages: messages}, SchemaName: "media_prompt_text",
	})
	return run.Completion.Text, err
}

type MediaQualityTaskInput struct {
	Intent      mediaIntent
	ContentType string
	Content     []byte
}

func (a *App) RunMediaQualityTask(ctx context.Context, input MediaQualityTaskInput) (mediaQualityAcceptance, error) {
	messages, err := mediaQualityMessages(input.Intent, input.ContentType, input.Content)
	if err != nil {
		return mediaQualityAcceptance{}, err
	}
	messages = formatProviderMessagesForRole(messages, "media_prompt")
	run, err := a.runFormalStructuredTask(
		WithProviderScenario(ctx, "media_quality_acceptance"),
		FormalAgentMediaQuality, messages, nil, "media_quality_acceptance_response", mediaQualityAcceptanceResponseSchema(), false, nil,
	)
	if err != nil {
		return mediaQualityAcceptance{}, err
	}
	return normalizeMediaQualityAcceptance(run.Completion.Structured)
}

type VisualIdentityVisionTaskInput struct {
	CandidateAssetID string
	InputSnapshot    map[string]any
	Constraints      map[string]any
	ImageContent     map[string]any
}

var visualIdentityVisionTaskInstruction = "Inspect the supplied candidate image for visual identity continuity. The required target is one complete text-free CHARACTER PROFILE card on a white minimalist background with an editorial 3:4 vertical layout. It must contain these visual sections, represented by layout, figures, diagrams, swatches and whitespace rather than rendered text: " + visualIdentityRequiredCardSectionsText + ". Do not require OCR, exact lettering, section headers, titles, labels, numbers or signature characters; generated text is intentionally omitted and incidental unreadable marks must not by themselves lower identity_match. Verify that the same face is preserved across the front, side and back views. If the image is anime, chibi, an unrelated art photo, abstract silhouette, landscape, object-only image, missing a person, missing a required visual section, or inconsistent across views, report a low identity_match and make that mismatch explicit in observations. Return bounded structured observations only."

func (a *App) RunVisualIdentityVisionTask(ctx context.Context, input VisualIdentityVisionTaskInput) (map[string]any, error) {
	content := []any{map[string]any{"type": "text", "text": jsonString(map[string]any{
		"asset_id": input.CandidateAssetID, "render_intent": "character_design_sheet", "expected_subject": "one_human_character",
		"expected_views": visualIdentityExpectedViews(), "panel_layout": map[string]string{"front": "front_full_body", "side": "side_full_body", "back": "back_full_body"},
		"visual_identity": input.InputSnapshot, "renderer_constraints": input.Constraints,
	})}}
	if input.ImageContent != nil {
		content = append(content, input.ImageContent)
	}
	messages := (&PromptComposer{}).ComposeTaskMessages("visual_identity_vision", []map[string]any{
		{"role": "system", "content": visualIdentityVisionTaskInstruction}, {"role": "user", "content": content},
	})
	run, err := a.runFormalStructuredTask(ctx, FormalAgentVisualIdentityVision, messages, nil, "visual_identity_vision_response", visualIdentityVisionResponseSchema(), false, nil)
	return run.Completion.Structured, err
}

type VisualIdentityPatchTaskInput struct {
	CandidateAssetID string
	InputSnapshot    map[string]any
	Constraints      map[string]any
	Vision           map[string]any
}

var visualIdentityPatchTaskInstruction = "Review the candidate against the visual identity and return accepted or regenerate. Acceptance is allowed only for one complete text-free CHARACTER PROFILE card on a white minimalist background with an editorial 3:4 vertical layout, containing these visual sections: " + visualIdentityRequiredCardSectionsText + ". Do not require or score exact lettering, titles, headers, labels, numbers or signature characters; text is intentionally omitted, and incidental unreadable marks must not by themselves require regeneration. Regenerate only when a required visual section, view, person or consistent facial identity is missing, or when the image is anime, chibi, an unrelated art photo or an abstract silhouette. Preserve the explicit decision and a structured patch."

func (a *App) RunVisualIdentityPatchTask(ctx context.Context, input VisualIdentityPatchTaskInput) (map[string]any, error) {
	payload := map[string]any{
		"stage": "review", "render_intent": "character_design_sheet", "expected_subject": "one_human_character", "expected_views": visualIdentityExpectedViews(),
		"panel_layout":    map[string]string{"front": "front_full_body", "side": "side_full_body", "back": "back_full_body"},
		"visual_identity": input.InputSnapshot, "renderer_constraints": input.Constraints, "vision": input.Vision, "candidate_asset_id": input.CandidateAssetID,
	}
	messages := (&PromptComposer{}).ComposeTaskMessages("visual_identity_patch", []map[string]any{
		{"role": "system", "content": visualIdentityPatchTaskInstruction}, {"role": "user", "content": jsonString(payload)},
	})
	run, err := a.runFormalStructuredTask(ctx, FormalAgentVisualIdentityPatch, messages, nil, "visual_identity_patch_response", visualIdentityPatchResponseSchema(), false, nil)
	return run.Completion.Structured, err
}

type ConversationSummaryTaskInput struct {
	Messages []ConversationSummarySourceMessage
}

func (a *App) RunConversationSummaryTask(ctx context.Context, input ConversationSummaryTaskInput) (conversationSummaryProviderResponse, error) {
	providerMessages := conversationSummaryProviderMessages(input.Messages)
	messages := (&PromptComposer{}).ComposeTaskMessages("reflection", []map[string]any{
		{"role": "system", "content": conversationSummaryInstruction},
		{"role": "user", "content": jsonString(map[string]any{"source_messages": providerMessages})},
	})
	run, err := a.runFormalStructuredTask(
		WithProviderScenario(ctx, "conversation_summary"), FormalAgentConversationSummary, messages,
		nil, "conversation_summary_v1", conversationSummaryProviderSchema(), false, nil,
	)
	if err != nil {
		return conversationSummaryProviderResponse{}, err
	}
	return decodeConversationSummaryProviderResponse(run.Completion.Structured)
}

type ScheduleGenerationTaskInput struct {
	LocalDate             string
	Timezone              string
	Identity              map[string]any
	LifeProfile           map[string]any
	CurrentState          map[string]any
	CurrentLife           map[string]any
	Goals                 []map[string]any
	Intentions            []map[string]any
	RecentOutcomes        []map[string]any
	CompactOutputReminder bool
}

type VirtualActivityResultTaskInput struct {
	Kind              string
	Request           map[string]any
	StartedAt         string
	NotBefore         string
	CurrentAppearance map[string]any
	CurrentLife       map[string]any
	RecentOutcomes    []map[string]any
}

const virtualActivityResultInstruction = "Resolve one already started and elapsed virtual-life activity. The activity is fictional and grants no real purchase, payment, delivery, medical care, or external action. Return completed, failed, or deferred with a concrete short reason grounded in the supplied request and current facts. Do not always choose success. For completed virtual_shopping return one acquired_item whose category and slot match the request; no acquired item on failure or defer. For completed haircut return the resulting hair_length and optional hair_color/hair_style; no body change on failure or defer. Do not infer that a scheduled activity was completed before its not_before time. Return only the specified JSON."

func virtualActivityResultSchema() map[string]any {
	item := objectSchema(map[string]any{
		"category":    map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		"slot":        map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		"description": map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
	}, []string{"category", "slot", "description"}, false)
	return objectSchema(map[string]any{
		"status":        enumStringSchema("completed", "failed", "deferred"),
		"reason":        map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		"acquired_item": item,
		"hair_length":   map[string]any{"type": "string", "maxLength": 128},
		"hair_color":    map[string]any{"type": "string", "maxLength": 128},
		"hair_style":    map[string]any{"type": "string", "maxLength": 128},
	}, []string{"status", "reason"}, false)
}

func (a *App) RunVirtualActivityResultTask(ctx context.Context, input VirtualActivityResultTaskInput) (map[string]any, error) {
	messages := (&PromptComposer{}).ComposeTaskMessages("cognitive_assessment", []map[string]any{
		{"role": "system", "content": virtualActivityResultInstruction},
		{"role": "user", "content": jsonString(map[string]any{
			"kind": input.Kind, "request": input.Request, "started_at": input.StartedAt, "not_before": input.NotBefore,
			"current_appearance": input.CurrentAppearance, "current_life": input.CurrentLife, "recent_outcomes": input.RecentOutcomes,
		})},
	})
	run, err := a.runFormalStructuredTask(WithProviderScenario(ctx, "virtual_activity_result"), FormalAgentVirtualActivityResult, messages,
		nil, "virtual_activity_result", virtualActivityResultSchema(), false, nil)
	return run.Completion.Structured, err
}

const scheduleGenerationTaskInstruction = "Return one compact object with items and reschedule_policy. Use only as many intervals as the supplied facts require, no more than 16; cover the local day contiguously from 00:00 through the next 00:00, with explicit free/unplanned/rest intervals where nothing is committed. Identity or occupation alone does not establish a daily class, library visit, uniform, or fixed routine. Preserve supplied recurring commitments as constraints, distinguish an intention from a scheduled action and a completed result, and consider current state, existing activities and recent outcomes. Do not claim an activity happened just because its planned time passed. Every item needs start_at, end_at, activity, scene, location, item_type, status, priority, flexibility, interruption_cost. Keep activity, scene, and location each under 80 Chinese characters; use one concrete activity and scene per item, never combine alternatives with '/', '／', '、', or '或'. Merge adjacent equivalent periods. priority, flexibility, and interruption_cost are normalized numbers from 0 to 1. Use RFC3339 timestamps with the supplied timezone. Do not return markdown or foundation fields."

func (a *App) RunScheduleGenerationTask(ctx context.Context, input ScheduleGenerationTaskInput) (map[string]any, error) {
	instruction := scheduleGenerationTaskInstruction
	if input.CompactOutputReminder {
		instruction += " 上一个日程 JSON 不完整。请重新输出完整且紧凑的时段，必须覆盖从 00:00 到次日 00:00；未安排时间用明确空闲时段表示，不能截断，也不要附加解释。"
	}
	messages := (&PromptComposer{}).ComposeTaskMessages("cognitive_assessment", []map[string]any{
		{"role": "system", "content": instruction},
		{"role": "user", "content": jsonString(map[string]any{"local_date": input.LocalDate, "timezone": input.Timezone, "identity": input.Identity, "life_profile": input.LifeProfile, "current_state": input.CurrentState, "current_life": input.CurrentLife, "goals": input.Goals, "intentions": input.Intentions, "recent_outcomes": input.RecentOutcomes})},
	})
	run, err := a.runFormalStructuredTask(ctx, FormalAgentScheduleGeneration, messages, nil, "schedule_response", scheduleResponseSchema(), false, nil)
	return run.Completion.Structured, err
}

// Projection-backed tasks own selection of context surfaces, operation rules,
// tool catalog and schema. They return the assembled projection so callers can
// validate and persist the final semantic contract; Tool effects in the trace
// are already committed by the formal Agent loop.
type NativeCognitionTaskInput struct {
	EventType  string
	Fact       []byte
	Projection ContextProjection
}

func (a *App) RunNativeCognitionTask(ctx context.Context, input NativeCognitionTaskInput) (ProjectionTaskResult, error) {
	definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceNativeCognition)
	schema := nativeCognitionResponseSchema()
	assembly, projection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceNativeCognition, input.Projection, "cognitive_assessment", []string{providerContextAuthorityRule, nativeCognitionInstruction}, jsonString(map[string]any{"event_type": input.EventType, "fact": compactProviderFact(input.Fact)}), definitions, "native_cognition_response", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "native_cognition"), assembly.Diagnostics)
	run, err := a.runFormalStructuredTask(providerCtx, FormalAgentNativeCognition, assembly.Messages, definitions, "native_cognition_response", schema, true, formalTaskCapabilityRequest(FormalAgentNativeCognition, projection, CapabilitySurfaceNativeCognition))
	return ProjectionTaskResult{Completion: run.Completion, Projection: projection, Diagnostics: assembly.Diagnostics, Trace: run.Trace}, err
}

type DailyReviewTaskInput struct {
	LocalDate  string
	Projection ContextProjection
}

func (a *App) RunDailyReviewTask(ctx context.Context, input DailyReviewTaskInput) (ProjectionTaskResult, error) {
	definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceAutonomy)
	schema := dailyReviewResponseSchema()
	assembly, projection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceDailyReview, input.Projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityDailyReviewPolicyInstruction}, jsonString(map[string]any{"local_date": input.LocalDate}), definitions, "daily_review_response", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "daily_review"), assembly.Diagnostics)
	run, err := a.runFormalStructuredTask(providerCtx, FormalAgentDailyReview, assembly.Messages, definitions, "daily_review_response", schema, true, formalTaskCapabilityRequest(FormalAgentDailyReview, projection, CapabilitySurfaceAutonomy))
	return ProjectionTaskResult{Completion: run.Completion, Projection: projection, Diagnostics: assembly.Diagnostics, Trace: run.Trace}, err
}

type PersistentSwitchTaskInput struct {
	InboxID        string
	CurrentText    string
	CandidateReply string
	ResponseIntent string
	Projection     ContextProjection
}

func (a *App) RunPersistentSwitchTask(ctx context.Context, input PersistentSwitchTaskInput) (ProjectionTaskResult, error) {
	schema := persistentSwitchAssessmentSchema()
	assembly, projection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfacePersistentSwitch, input.Projection, "cognitive_assessment", []string{providerContextAuthorityRule, persistentSwitchAssessmentInstruction}, jsonString(map[string]any{
		"user_text": input.CurrentText, "candidate_reply": input.CandidateReply, "response_intent": input.ResponseIntent,
	}), nil, persistentSwitchAssessmentSchemaName, schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderCorrelation(WithProviderScenario(ctx, "cognitive_assessment"), "persona-switch-after:"+input.InboxID), assembly.Diagnostics)
	run, err := a.runFormalStructuredTask(providerCtx, FormalAgentPersistentSwitch, assembly.Messages, nil, persistentSwitchAssessmentSchemaName, schema, false, nil)
	return ProjectionTaskResult{Completion: run.Completion, Projection: projection, Diagnostics: assembly.Diagnostics, Trace: run.Trace}, err
}

type ReflectionProposalTaskInput struct {
	Evidence   []map[string]any
	Projection ContextProjection
}

func compactReflectionEvidenceWithReferences(evidence []map[string]any, projection ContextProjection) []map[string]any {
	result := compactReflectionEvidenceV2(evidence)
	outcomeRefsByID := make(map[string]string)
	for ref, entry := range projection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceOutcome && strings.TrimSpace(entry.EntityID) != "" {
			outcomeRefsByID[entry.EntityID] = ref
		}
	}
	for evidenceIndex, source := range evidence {
		if evidenceIndex >= len(result) || strings.TrimSpace(stringValue(source["event_type"])) != "autonomy.result" {
			continue
		}
		payload := mapValue(source["payload"])
		rawOutcomes := arrayValue(payload["outcomes"])
		if len(rawOutcomes) == 0 && payload["outcome"] != nil {
			rawOutcomes = []any{payload["outcome"]}
		}
		providerEnvelope := mapValue(result[evidenceIndex]["action_outcomes"])
		providerOutcomes := arrayValue(providerEnvelope["outcomes"])
		for outcomeIndex, raw := range rawOutcomes {
			if outcomeIndex >= len(providerOutcomes) {
				break
			}
			providerOutcome := mapValue(providerOutcomes[outcomeIndex])
			if ref := outcomeRefsByID[strings.TrimSpace(stringValue(mapValue(raw)["id"]))]; ref != "" {
				providerOutcome["ref"] = ref
			}
		}
	}
	return result
}

func (a *App) RunReflectionProposalTask(ctx context.Context, input ReflectionProposalTaskInput) (ProjectionTaskResult, error) {
	schema := reflectionProposalV2ProviderSchema()
	providerEvidence := compactReflectionEvidenceWithReferences(input.Evidence, input.Projection)
	assembly, projection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceReflection, input.Projection, "reflection", []string{providerContextAuthorityRule, reflectionV2Instruction}, jsonString(map[string]any{"evidence": providerEvidence}), nil, "reflection_proposal_v2", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "reflection"), assembly.Diagnostics)
	run, err := a.runFormalStructuredTask(providerCtx, FormalAgentReflection, assembly.Messages, nil, "reflection_proposal_v2", schema, true, nil)
	return ProjectionTaskResult{Completion: run.Completion, Projection: projection, Diagnostics: assembly.Diagnostics, Trace: run.Trace}, err
}

// RunScheduleReplanTask is the typed task boundary used by the capability
// planner. It owns the planner instruction, factual slots and output schema.
func (a *App) RunScheduleReplanTask(ctx context.Context, input SchedulePlanInput) (map[string]any, error) {
	messages := (&PromptComposer{}).ComposeTaskMessages("cognitive_assessment", []map[string]any{
		{"role": "system", "content": "Return only a complete schedule replacement. Preserve completed history and use the supplied timezone and revision."},
		{"role": "user", "content": jsonString(map[string]any{
			"intent": input.Intent, "schedule": compactScheduleForProvider(input.Schedule), "current_life": compactLifeContext(input.CurrentLife), "agency": compactSchedulePlannerAgency(input.Agency), "timezone": input.Timezone,
		})},
	})
	run, err := a.runFormalStructuredTask(WithProviderScenario(ctx, "schedule_replan_planner"), FormalAgentScheduleReplan, messages, nil, "schedule_replan_plan", schedulePlannerOutputSchema(), false, nil)
	return run.Completion.Structured, err
}

func (a *App) RunEmbeddingTask(ctx context.Context, text string) (string, []float64, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return "", nil, err
	}
	return provider.Embed(WithProviderScenario(ctx, "memory_retrieval"), text)
}

// RunFrozenEmbeddingTask keeps a workflow-pinned assignment while still
// exposing an operation-owned task boundary. It is used by durable embedding
// intents whose endpoint/model tuple must not be re-resolved on retry.
func (a *App) RunFrozenEmbeddingTask(ctx context.Context, text string, assignment providerAssignment) (string, []float64, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return "", nil, err
	}
	return provider.embedWithAssignment(ctx, text, assignment)
}
