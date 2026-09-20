package core

import (
	"context"
	"errors"
	"strings"
)

type ProjectionTaskResult struct {
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
	provider, err := a.modelTaskProvider()
	if err != nil {
		return nil, err
	}
	messages := initializationAnalysisMessages(input.Description)
	return provider.Structured(WithProviderScenario(ctx, "initialization"), "initialization", messages)
}

// MediaPromptTaskInput contains only the frozen media facts and retry
// feedback. The task selects the media_prompt instruction and serializes the
// bounded concept; callers cannot inject an arbitrary system message.
type MediaPromptTaskInput struct {
	Intent mediaIntent
}

func (a *App) RunMediaPromptTask(ctx context.Context, input MediaPromptTaskInput) (string, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return "", err
	}
	messages := []map[string]any{
		{"role": "system", "content": mediaPromptInstruction},
		{"role": "user", "content": mediaPromptInput(input.Intent)},
	}
	return provider.Text(WithProviderScenario(ctx, "media_prompt"), "media_prompt", messages)
}

type MediaQualityTaskInput struct {
	Intent      mediaIntent
	ContentType string
	Content     []byte
}

func (a *App) RunMediaQualityTask(ctx context.Context, input MediaQualityTaskInput) (mediaQualityAcceptance, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return mediaQualityAcceptance{}, err
	}
	messages, err := mediaQualityMessages(input.Intent, input.ContentType, input.Content)
	if err != nil {
		return mediaQualityAcceptance{}, err
	}
	value, err := provider.StructuredWithSchema(
		WithProviderScenario(ctx, "media_quality_acceptance"),
		"media_prompt", messages, "media_quality_acceptance_response", mediaQualityAcceptanceResponseSchema(), false,
	)
	if err != nil {
		return mediaQualityAcceptance{}, err
	}
	return normalizeMediaQualityAcceptance(value)
}

type VisualIdentityVisionTaskInput struct {
	CandidateAssetID string
	InputSnapshot    map[string]any
	Constraints      map[string]any
	ImageContent     map[string]any
}

const visualIdentityVisionTaskInstruction = "Inspect the supplied candidate image for visual identity continuity. The required target is one complete CHARACTER PROFILE card on a white minimalist background with an editorial 3:4 vertical layout. It must include BASIC INFORMATION, exactly three MODEL SHEET views (front, side and back), six EXPRESSIONS, OUTFIT BREAKDOWN, ACCESSORIES, DETAIL CLOSE-UP, COLOR PALETTE, CHARACTER INTRODUCTION, KEYWORDS and SIGNATURE. Verify that the same face is preserved across the front, side and back views. If the image is anime, chibi, an unrelated art photo, abstract silhouette, landscape, object-only image, missing a person, missing a required section, or inconsistent across views, report a low identity_match and make that mismatch explicit in observations. Return bounded structured observations only."

func (a *App) RunVisualIdentityVisionTask(ctx context.Context, input VisualIdentityVisionTaskInput) (map[string]any, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return nil, err
	}
	content := []any{map[string]any{"type": "text", "text": jsonString(map[string]any{
		"asset_id": input.CandidateAssetID, "render_intent": "character_design_sheet", "expected_subject": "one_human_character",
		"expected_views": visualIdentityExpectedViews(), "panel_layout": map[string]string{"front": "front_full_body", "side": "side_full_body", "back": "back_full_body"},
		"visual_identity": input.InputSnapshot, "renderer_constraints": input.Constraints,
	})}}
	if input.ImageContent != nil {
		content = append(content, input.ImageContent)
	}
	return provider.StructuredWithSchema(ctx, "visual_identity_vision", []map[string]any{
		{"role": "system", "content": visualIdentityVisionTaskInstruction}, {"role": "user", "content": content},
	}, "visual_identity_vision_response", visualIdentityVisionResponseSchema(), false)
}

type VisualIdentityPatchTaskInput struct {
	CandidateAssetID string
	InputSnapshot    map[string]any
	Constraints      map[string]any
	Vision           map[string]any
}

const visualIdentityPatchTaskInstruction = "Review the candidate against the visual identity and return accepted or regenerate. Acceptance is allowed only for one complete CHARACTER PROFILE card on a white minimalist background with an editorial 3:4 vertical layout, including BASIC INFORMATION, front/side/back MODEL SHEET views, six EXPRESSIONS, OUTFIT BREAKDOWN, ACCESSORIES, DETAIL CLOSE-UP, COLOR PALETTE, CHARACTER INTRODUCTION, KEYWORDS and SIGNATURE. The same face must remain consistent across all three views. Anime, chibi, unrelated art photos, abstract silhouettes, missing sections, missing views or inconsistent facial identity must be decision=regenerate. Preserve the explicit decision and a structured patch."

func (a *App) RunVisualIdentityPatchTask(ctx context.Context, input VisualIdentityPatchTaskInput) (map[string]any, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"stage": "review", "render_intent": "character_design_sheet", "expected_subject": "one_human_character", "expected_views": visualIdentityExpectedViews(),
		"panel_layout":    map[string]string{"front": "front_full_body", "side": "side_full_body", "back": "back_full_body"},
		"visual_identity": input.InputSnapshot, "renderer_constraints": input.Constraints, "vision": input.Vision, "candidate_asset_id": input.CandidateAssetID,
	}
	return provider.StructuredWithSchema(ctx, "visual_identity_patch", []map[string]any{
		{"role": "system", "content": visualIdentityPatchTaskInstruction}, {"role": "user", "content": jsonString(payload)},
	}, "visual_identity_patch_response", visualIdentityPatchResponseSchema(), false)
}

type ConversationSummaryTaskInput struct {
	Messages []ConversationSummarySourceMessage
}

func (a *App) RunConversationSummaryTask(ctx context.Context, input ConversationSummaryTaskInput) (conversationSummaryProviderResponse, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return conversationSummaryProviderResponse{}, err
	}
	providerMessages := conversationSummaryProviderMessages(input.Messages)
	value, err := provider.StructuredWithSchema(
		WithProviderScenario(ctx, "conversation_summary"), "reflection", []map[string]any{
			{"role": "system", "content": conversationSummaryInstruction},
			{"role": "user", "content": jsonString(map[string]any{"source_messages": providerMessages})},
		}, "conversation_summary_v1", conversationSummaryProviderSchema(), false,
	)
	if err != nil {
		return conversationSummaryProviderResponse{}, err
	}
	return decodeConversationSummaryProviderResponse(value)
}

type ScheduleGenerationTaskInput struct {
	LocalDate             string
	Timezone              string
	Identity              map[string]any
	LifeProfile           map[string]any
	CompactOutputReminder bool
}

const scheduleGenerationTaskInstruction = "Return one compact object with items and reschedule_policy. items must contain 8-16 objects covering the complete local day contiguously from 00:00 through the next 00:00 in the supplied timezone. Every item needs start_at, end_at, activity, scene, location, item_type, status, priority, flexibility, interruption_cost. Keep activity, scene, and location each under 80 Chinese characters; use one concrete activity and one concrete scene per item, never combine alternatives with '/', '／', '、', or '或'. Merge adjacent periods with the same activity and scene instead of producing many small segments. priority, flexibility, and interruption_cost are normalized numbers from 0 to 1 (never a 1-10 score). Use RFC3339 timestamps with the supplied timezone. Do not return markdown or foundation fields."

func (a *App) RunScheduleGenerationTask(ctx context.Context, input ScheduleGenerationTaskInput) (map[string]any, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return nil, err
	}
	instruction := scheduleGenerationTaskInstruction
	if input.CompactOutputReminder {
		instruction += " 上一个日程 JSON 不完整。请重新输出完整且紧凑的 8-16 个时段，必须覆盖从 00:00 到次日 00:00，不能截断，也不要附加解释。"
	}
	return provider.StructuredWithSchema(ctx, "cognitive_assessment", []map[string]any{
		{"role": "system", "content": instruction},
		{"role": "user", "content": jsonString(map[string]any{"local_date": input.LocalDate, "timezone": input.Timezone, "identity": input.Identity, "life_profile": input.LifeProfile})},
	}, "schedule_response", scheduleResponseSchema(), false)
}

// Projection-backed tasks own selection of context surfaces, operation rules,
// tool catalog and schema. They return the assembled projection because the
// caller still owns domain validation and settlement after model execution.
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
	provider, err := a.modelTaskProvider()
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "native_cognition"), assembly.Diagnostics)
	completion, err := provider.StructuredAssembledWithToolsSchema(providerCtx, "cognitive_assessment", assembly.Messages, definitions, "native_cognition_response", schema, true)
	return ProjectionTaskResult{Completion: completion, Projection: projection, Diagnostics: assembly.Diagnostics}, err
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
	provider, err := a.modelTaskProvider()
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "daily_review"), assembly.Diagnostics)
	completion, err := provider.StructuredAssembledWithToolsSchema(providerCtx, "cognitive_assessment", assembly.Messages, definitions, "daily_review_response", schema, true)
	return ProjectionTaskResult{Completion: completion, Projection: projection, Diagnostics: assembly.Diagnostics}, err
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
	provider, err := a.modelTaskProvider()
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderCorrelation(WithProviderScenario(ctx, "cognitive_assessment"), "persona-switch-after:"+input.InboxID), assembly.Diagnostics)
	completion, err := provider.StructuredAssembledWithToolsSchema(providerCtx, "cognitive_assessment", assembly.Messages, nil, persistentSwitchAssessmentSchemaName, schema, false)
	return ProjectionTaskResult{Completion: completion, Projection: projection, Diagnostics: assembly.Diagnostics}, err
}

type ReflectionProposalTaskInput struct {
	Evidence   []map[string]any
	Projection ContextProjection
}

func (a *App) RunReflectionProposalTask(ctx context.Context, input ReflectionProposalTaskInput) (ProjectionTaskResult, error) {
	schema := reflectionProposalV2ProviderSchema()
	providerEvidence := compactReflectionEvidenceV2(input.Evidence)
	assembly, projection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceReflection, input.Projection, "reflection", []string{providerContextAuthorityRule, reflectionV2Instruction}, jsonString(map[string]any{"evidence": providerEvidence}), nil, "reflection_proposal_v2", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	provider, err := a.modelTaskProvider()
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "reflection"), assembly.Diagnostics)
	completion, err := provider.StructuredAssembledWithToolsSchema(providerCtx, "reflection", assembly.Messages, nil, "reflection_proposal_v2", schema, true)
	return ProjectionTaskResult{Completion: completion, Projection: projection, Diagnostics: assembly.Diagnostics}, err
}

// RunScheduleReplanTask is the typed task boundary used by the capability
// planner. It owns the planner instruction, factual slots and output schema.
func (a *App) RunScheduleReplanTask(ctx context.Context, input SchedulePlanInput) (map[string]any, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return nil, err
	}
	return provider.StructuredWithSchema(WithProviderScenario(ctx, "schedule_replan_planner"), "cognitive_assessment", []map[string]any{
		{"role": "system", "content": "Return only a complete schedule replacement. Preserve completed history and use the supplied timezone and revision."},
		{"role": "user", "content": jsonString(map[string]any{
			"intent": input.Intent, "schedule": compactScheduleForProvider(input.Schedule), "current_life": compactLifeContext(input.CurrentLife), "agency": compactSchedulePlannerAgency(input.Agency), "timezone": input.Timezone,
		})},
	}, "schedule_replan_plan", schedulePlannerOutputSchema(), false)
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
