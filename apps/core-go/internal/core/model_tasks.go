package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
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

type MediaPromptTaskResult struct {
	Prompt          string
	CapturePlan     map[string]any
	CaptureFallback map[string]any
}

func (a *App) RunMediaPromptTask(ctx context.Context, input MediaPromptTaskInput) (string, error) {
	result, err := a.runMediaPromptTaskResult(ctx, input)
	return result.Prompt, err
}
func (a *App) runMediaPromptTaskResult(ctx context.Context, input MediaPromptTaskInput) (MediaPromptTaskResult, error) {
	if _, ok := compactMediaConceptObjectForProvider(input.Intent.Prompt); !ok {
		return MediaPromptTaskResult{}, errors.New("media_prompt_frozen_concept_invalid")
	}
	concept := decodeObject([]byte(input.Intent.Prompt))
	promptIntent := input.Intent
	if hasCurrentCapture(concept) {
		concept = cloneMap(concept)
		delete(concept, "capture_plan_fallback")
		delete(concept, "capture_plan")
		promptIntent.Prompt = jsonString(concept)
	}
	instruction := mediaPromptSystemInstruction(input.Intent)
	prompt := PromptAssemblyResult{}
	if hasCurrentCapture(concept) {
		if err := validateCurrentCaptureSnapshot(concept); err != nil {
			return MediaPromptTaskResult{}, err
		}
		instruction = "Normalize the upstream vague photo instruction into an accurate, physically consistent standard photograph description. Choose framing, pose, expression, lighting, style and capture (mode, camera, angle, mirror, device_visibility). The upstream capture/framing are interpretation hints from the main model, not immutable facts or literal enum strings to copy. Preserve the requested photo meaning while resolving ambiguity, e.g. full-body self-capture may use a full-length mirror. Use the supplied schema for your standardized result. Body, current clothing, used objects and reference images are server-owned snapshot facts; they cannot be supplied or overridden in your response. No prose or extra fields. " + currentCaptureEnumInstruction()
		prompt.ResponseFormat = providerResponseFormatForSchema("media_prompt", "current_capture_plan", currentCapturePlanSchema())
	}
	prompt.Messages = formatProviderMessagesForRole([]map[string]any{{"role": "system", "content": instruction}, {"role": "user", "content": mediaPromptInput(promptIntent)}}, "media_prompt")
	run, err := a.RunFormalAgent(WithProviderScenario(ctx, "media_prompt"), FormalAgentMediaPrompt, FormalAgentRunInput{Prompt: prompt, SchemaName: "media_prompt_text"})
	if err != nil {
		return MediaPromptTaskResult{}, err
	}
	if hasCurrentCapture(concept) {
		var plan map[string]any
		if err := jsonUnmarshal([]byte(run.Completion.Text), &plan); err != nil {
			if structured, ok := parseStructuredCandidates([]string{run.Completion.Text}); ok {
				plan = structured
			} else {
				return MediaPromptTaskResult{}, errors.New("current_capture_plan_invalid")
			}
		}
		// A new model attempt may recover a previous fallback. Do not inherit its
		// override unless this new attempt also requires the configured fallback.
		concept = cloneMap(concept)
		delete(concept, "capture_plan_fallback")
		plan, fallback, err := resolveCurrentCapturePlan(concept, plan)
		if err != nil {
			return MediaPromptTaskResult{}, err
		}
		if len(fallback) > 0 {
			concept["capture_plan_fallback"] = fallback
			if a.DB != nil {
				a.recordDiagnosticEvent(ctx, "media.current_capture.plan_fallback", "info", input.Intent.Owner, input.Intent.ID, providerCorrelation(ctx), fallback)
			}
		}
		rendered, err := renderCurrentCapturePrompt(concept, plan)
		return MediaPromptTaskResult{Prompt: rendered, CapturePlan: plan, CaptureFallback: fallback}, err
	}
	return MediaPromptTaskResult{Prompt: cleanGeneratedMediaPrompt(run.Completion.Text)}, nil
}

func cleanGeneratedMediaPrompt(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
			text = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		}
	}
	for _, marker := range []string{"提示词：", "提示词:\n", "Prompt:", "prompt:"} {
		if idx := strings.Index(text, marker); idx != -1 {
			after := strings.TrimSpace(text[idx+len(marker):])
			if after != "" {
				text = after
				break
			}
		}
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 1 {
		first := strings.TrimSpace(lines[0])
		if (strings.HasPrefix(first, "这是一条") || strings.HasPrefix(first, "以下是") || strings.HasPrefix(first, "好的")) &&
			(strings.Contains(first, "提示词") || strings.Contains(first, "写真") || strings.HasSuffix(first, "：") || strings.HasSuffix(first, ":")) {
			text = strings.TrimSpace(strings.Join(lines[1:], "\n"))
		}
	}
	return text
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
	messages = formatProviderMessagesForRole(messages, "visual_identity_vision")
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
	Messages        []ConversationSummarySourceMessage
	PreviousSummary string
	ActorFacts      []map[string]any
	MaxRunes        int
	OwnerActorID    string
	FluctlightID    string
}

func (a *App) RunConversationSummaryTask(ctx context.Context, input ConversationSummaryTaskInput) (conversationSummaryProviderResponse, error) {
	if a != nil && a.DB != nil {
		a.recordDiagnosticEvent(ctx, "conversation_summary.thinking_mode", "info", input.FluctlightID, "", providerCorrelation(ctx), map[string]any{"requested_enable_thinking": false, "provider_support": "unverified", "effective_mode": "unverified"})
	}
	providerMessages := conversationSummaryProviderMessages(input.Messages)
	for index, message := range input.Messages {
		alias := "actor:" + stableDigest(message.AuthorActorID)[:12]
		if message.AuthorActorID == input.OwnerActorID {
			alias = "actor_user"
		}
		if message.AuthorActorID == input.FluctlightID {
			alias = "actor_self"
		}
		providerMessages[index]["author"] = alias
	}
	instruction := conversationSummaryInstruction
	if input.MaxRunes > 0 {
		instruction = fmt.Sprintf("以一份有界运行摘要替换旧运行摘要。只合并 previous_runtime_summary 与尚未覆盖的 source_messages，并以 authoritative_actor_facts 中的明确纠正校正旧误判；不把新状态变化改写为过去一直如此。保留主体、明确纠正、意图、承诺、未完成事项与真实结果，计划/推断/失败不可变成已完成事实。重复问候、光线和衣物颜色无新信息时归并。最多%d个字符，不新增事实，不追加摘要块。", input.MaxRunes)
	}
	messages := (&PromptComposer{}).ComposeTaskMessages("reflection", []map[string]any{
		{"role": "system", "content": instruction},
		{"role": "user", "content": jsonString(map[string]any{"previous_runtime_summary": input.PreviousSummary, "authoritative_actor_facts": input.ActorFacts, "source_messages": providerMessages})},
	})
	summarySchema := conversationSummaryProviderSchema()
	if input.MaxRunes > 0 {
		mapValue(mapValue(summarySchema["properties"])["summary"])["maxLength"] = input.MaxRunes
	}
	run, err := a.runFormalStructuredTask(
		WithProviderScenario(ctx, "conversation_summary"), FormalAgentConversationSummary, messages,
		nil, "conversation_summary_v1", summarySchema, false, nil,
	)
	if err != nil {
		return conversationSummaryProviderResponse{}, err
	}
	return decodeConversationSummaryProviderResponse(run.Completion.Structured)
}

func (a *App) RunConversationSegmentTask(ctx context.Context, source []ConversationSummarySourceMessage) (conversationSegmentResponse, error) {
	from, to := source[0].CreatedAt.UTC(), source[len(source)-1].CreatedAt.UTC()
	messages := (&PromptComposer{}).ComposeTaskMessages("reflection", []map[string]any{
		{"role": "system", "content": conversationSummaryInstruction + " 这是按真实时间界定的聊天阶段。summary 概括重要事件与变化，ending_state 说明这段实际怎样结束，open_threads 只列未完成线索，core_events 只列已发生的关键事情。不要逐条复述。"},
		{"role": "user", "content": jsonString(map[string]any{"started_at": from.Format(time.RFC3339Nano), "ended_at": to.Format(time.RFC3339Nano), "source_messages": conversationSummaryProviderMessages(source)})},
	})
	run, err := a.runFormalStructuredTask(WithProviderScenario(ctx, "conversation_segment"), FormalAgentConversationSummary, messages, nil, "conversation_segment_v1", conversationSegmentProviderSchema(), false, nil)
	if err != nil {
		return conversationSegmentResponse{}, err
	}
	var response conversationSegmentResponse
	if err := jsonUnmarshal(jsonBytes(run.Completion.Structured), &response); err != nil {
		return response, err
	}
	return response, nil
}

func (a *App) RunConversationDailyEpisodeTask(ctx context.Context, localDate, timezone string, sources []conversationDailySource) (conversationDailyEpisodeResponse, error) {
	segments := make([]map[string]any, 0, len(sources))
	for _, source := range sources {
		segments = append(segments, map[string]any{"started_at": source.StartedAt.UTC().Format(time.RFC3339Nano), "ended_at": source.EndedAt.UTC().Format(time.RFC3339Nano), "summary": source.Summary, "ending_state": source.EndingState, "open_threads": source.OpenThreads, "core_events": source.CoreEvents})
	}
	messages := (&PromptComposer{}).ComposeTaskMessages("reflection", []map[string]any{
		{"role": "system", "content": "将同一当地日的阶段摘要合成一段简洁的对话经历记忆。只写已发生的交流、结束状态和未完成线索；不把愿望、玩笑、推测、模型回复或约定当成已执行事实，不推导稳定偏好或关系身份。按时间脉络叙述，避免逐条流水账。"},
		{"role": "user", "content": jsonString(map[string]any{"local_date": localDate, "timezone": timezone, "segments": segments})},
	})
	run, err := a.runFormalStructuredTask(WithProviderScenario(ctx, "conversation_daily_memory"), FormalAgentConversationSummary, messages, nil, "conversation_daily_memory_v1", conversationDailyEpisodeProviderSchema(), false, nil)
	if err != nil {
		return conversationDailyEpisodeResponse{}, err
	}
	var response conversationDailyEpisodeResponse
	if err := jsonUnmarshal(jsonBytes(run.Completion.Structured), &response); err != nil {
		return response, err
	}
	return response, nil
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

const virtualActivityResultInstruction = "Resolve one already started and elapsed virtual-life activity. The activity is fictional and grants no real purchase, payment, delivery, medical care, or external action. Return completed, failed, or deferred with a concrete short reason grounded in the supplied request and current facts. Do not always choose success. A completed virtual_shopping activity may end without buying anything; include acquired_item only when acquisition is confirmed, with item_kind, category and slot matching the request. Ordinary objects have item_kind=object and no wearing slot. For a bundle, acquired_items must contain every requested member in the supplied order; partial acquisition is a failed result, never whole-bundle success. No acquired item on failure or defer. For completed haircut return the resulting hair_length and optional hair_color/hair_style. For completed hair_dye return hair_color exactly equal to request.desired_hair_color and no hair_length; a different result must be failed or deferred. No body change on failure or defer. Do not infer that a scheduled activity was completed before its not_before time. Return only the specified JSON."

func virtualActivityResultSchema() map[string]any {
	item := shoppingItemSchema()
	return objectSchema(map[string]any{
		"status":         enumStringSchema("completed", "failed", "deferred"),
		"reason":         map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		"acquired_item":  item,
		"acquired_items": map[string]any{"type": "array", "maxItems": 8, "items": item},
		"hair_length":    map[string]any{"type": "string", "maxLength": 128},
		"hair_color":     map[string]any{"type": "string", "maxLength": 128},
		"hair_style":     map[string]any{"type": "string", "maxLength": 128},
	}, []string{"status", "reason"}, false)
}

func (a *App) RunVirtualActivityResultTask(ctx context.Context, input VirtualActivityResultTaskInput) (map[string]any, error) {
	messages := (&PromptComposer{}).ComposeTaskMessages("cognitive_assessment", []map[string]any{
		{"role": "system", "content": virtualActivityResultInstruction},
		{"role": "user", "content": jsonString(virtualActivityModelInput(input))},
	})
	run, err := a.runFormalStructuredTask(WithProviderScenario(ctx, "virtual_activity_result"), FormalAgentVirtualActivityResult, messages,
		nil, "virtual_activity_result", virtualActivityResultSchema(), false, nil)
	return run.Completion.Structured, err
}

func virtualActivityModelInput(input VirtualActivityResultTaskInput) map[string]any {
	outcomes := make([]map[string]any, 0, len(input.RecentOutcomes))
	for _, source := range input.RecentOutcomes {
		outcome := compactStateMap(source, []string{"capability_name", "status", "success_boundary", "error_code", "occurred_at"})
		if observed := scheduleGenerationSemanticValue(source["observed"]); observed != nil {
			outcome["observed"] = observed
		}
		outcomes = append(outcomes, outcome)
	}
	return map[string]any{
		"kind":       input.Kind,
		"request":    compactStateMap(input.Request, []string{"item_kind", "items", "category", "slot", "description", "desired_hair_length", "desired_hair_color", "scene", "activity", "location", "duration_minutes"}),
		"started_at": input.StartedAt, "not_before": input.NotBefore,
		"current_appearance": compactMediaAppearance(input.CurrentAppearance),
		"current_life":       compactMediaLifeContext(input.CurrentLife),
		"recent_outcomes":    outcomes,
	}
}

const scheduleGenerationTaskInstruction = "Return one compact object with items and reschedule_policy. Use only as many intervals as the supplied facts require, no more than 16; cover the local day contiguously from 00:00 through the next 00:00, with explicit free/unplanned/rest intervals where nothing is committed. Identity or occupation alone does not establish a daily class, library visit, uniform, or fixed routine. Preserve supplied recurring commitments as constraints, distinguish an intention from a scheduled action and a completed result, and consider current state, existing activities and recent outcomes. Do not claim an activity happened just because its planned time passed. Every item needs start_at, end_at, activity, scene, location, item_type, status, priority, flexibility, interruption_cost. A sleep interval must have item_type='sleep'; this is the authoritative behavior classification, not a claim that a periodic check wakes the actor. Keep activity, scene, and location each under 80 Chinese characters; use one concrete activity and scene per item, never combine alternatives with '/', '／', '、', or '或'. Merge adjacent equivalent periods. priority, flexibility, and interruption_cost are normalized numbers from 0 to 1. Use RFC3339 timestamps with the supplied timezone. Do not return markdown or foundation fields."

func (a *App) RunScheduleGenerationTask(ctx context.Context, input ScheduleGenerationTaskInput) (map[string]any, error) {
	instruction := scheduleGenerationTaskInstruction
	if input.CompactOutputReminder {
		instruction += " 上一个日程 JSON 不完整。请重新输出完整且紧凑的时段，必须覆盖从 00:00 到次日 00:00；未安排时间用明确空闲时段表示，不能截断，也不要附加解释。"
	}
	messages := (&PromptComposer{}).ComposeTaskMessages("cognitive_assessment", []map[string]any{
		{"role": "system", "content": instruction},
		{"role": "user", "content": jsonString(scheduleGenerationModelInput(input))},
	})
	run, err := a.runFormalStructuredTask(ctx, FormalAgentScheduleGeneration, messages, nil, "schedule_response", scheduleResponseSchema(), false, nil)
	return run.Completion.Structured, err
}

func scheduleGenerationModelInput(input ScheduleGenerationTaskInput) map[string]any {
	goals := make([]map[string]any, 0, len(input.Goals))
	for _, source := range input.Goals {
		goal := compactStateMap(source, []string{"scope", "success_criteria", "motivation", "status", "importance", "urgency", "progress", "deadline"})
		goal["desired_outcome"] = firstString(stringValue(source["desired_outcome"]), stringValue(source["description"]))
		goals = append(goals, goal)
	}
	intentions := make([]map[string]any, 0, len(input.Intentions))
	for _, source := range input.Intentions {
		intention := compactStateMap(source, []string{"expected_outcome", "status", "confidence", "preferred_time", "expiration", "capability_constraints"})
		intention["action_intent"] = firstString(stringValue(source["action_intent"]), stringValue(source["action"]))
		intentions = append(intentions, intention)
	}
	outcomes := make([]map[string]any, 0, len(input.RecentOutcomes))
	for _, source := range input.RecentOutcomes {
		outcome := compactStateMap(source, []string{"capability_name", "status", "success_boundary", "error_code", "occurred_at"})
		for _, key := range []string{"expected", "observed"} {
			if semantic := scheduleGenerationSemanticValue(source[key]); semantic != nil {
				outcome[key] = semantic
			}
		}
		outcomes = append(outcomes, outcome)
	}
	currentLife := compactStateMap(input.CurrentLife, []string{"scene", "activity", "location", "current_time", "timezone", "effective_at", "expires_at"})
	if presence := compactStateMap(input.CurrentLife["presence"], []string{"current_task", "user_presence", "effective_at", "expires_at"}); len(presence) > 0 {
		currentLife["presence"] = presence
	}
	return map[string]any{
		"local_date": input.LocalDate, "timezone": input.Timezone,
		"identity":      scheduleGenerationSemanticValue(input.Identity),
		"life_profile":  scheduleGenerationSemanticValue(input.LifeProfile),
		"current_state": scheduleGenerationSemanticValue(input.CurrentState),
		"current_life":  currentLife,
		"goals":         goals, "intentions": intentions, "recent_outcomes": outcomes,
	}
}

func scheduleGenerationSemanticValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
			if normalized != "status" && (providerMetadataKey(key) || normalized == "profileid" || normalized == "activeprofileid" || normalized == "ref" || strings.HasSuffix(normalized, "revision") || strings.Contains(normalized, "digest") || strings.HasSuffix(normalized, "ref") || strings.HasSuffix(normalized, "refs")) {
				continue
			}
			result[key] = scheduleGenerationSemanticValue(child)
		}
		return result
	case []map[string]any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = scheduleGenerationSemanticValue(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = scheduleGenerationSemanticValue(child)
		}
		return result
	default:
		return value
	}
}

// Projection-backed tasks own selection of context surfaces, operation rules,
// tool catalog and schema. They return the assembled projection so callers can
// validate and persist the final semantic contract; Tool effects in the trace
// are already committed by the formal Agent loop.
type NativeCognitionTaskInput struct {
	EventType         string
	Fact              []byte
	Projection        ContextProjection
	ProjectionRequest ContextProjectionRequest
}

func (a *App) RunNativeCognitionTask(ctx context.Context, input NativeCognitionTaskInput) (ProjectionTaskResult, error) {
	definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceNativeCognition)
	schema := nativeCognitionResponseSchema()
	if input.ProjectionRequest.FluctlightID != input.Projection.FluctlightID || input.ProjectionRequest.AuthorizationActorID != input.Projection.OwnerActorID {
		return ProjectionTaskResult{}, errors.New("native_cognition_projection_request_invalid")
	}
	operationRules := []string{providerContextAuthorityRule, nativeCognitionInstruction}
	currentInput := jsonString(map[string]any{"event_type": input.EventType, "fact": compactProviderFact(input.Fact)})
	assembly, projection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceNativeCognition, input.Projection, "cognitive_assessment", operationRules, currentInput, definitions, "native_cognition_response", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "native_cognition"), assembly.Diagnostics)
	providerCtx = a.bindProjectionRefresh(providerCtx, input.ProjectionRequest, ProviderContextSurfaceNativeCognition, "cognitive_assessment", operationRules, currentInput, definitions, "native_cognition_response", schema, nil)
	run, err := a.runFormalStructuredTask(providerCtx, FormalAgentNativeCognition, assembly.Messages, definitions, "native_cognition_response", schema, true, formalTaskCapabilityRequest(FormalAgentNativeCognition, projection, CapabilitySurfaceNativeCognition))
	if run.Projection != nil {
		projection = *run.Projection
	}
	return ProjectionTaskResult{Completion: run.Completion, Projection: projection, Diagnostics: assembly.Diagnostics, Trace: run.Trace}, err
}

type DailyReviewTaskInput struct {
	LocalDate         string
	Projection        ContextProjection
	ProjectionRequest ContextProjectionRequest
}

func (a *App) RunDailyReviewTask(ctx context.Context, input DailyReviewTaskInput) (ProjectionTaskResult, error) {
	definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceAutonomy)
	schema := dailyReviewResponseSchema()
	if input.ProjectionRequest.FluctlightID != input.Projection.FluctlightID || input.ProjectionRequest.AuthorizationActorID != input.Projection.OwnerActorID {
		return ProjectionTaskResult{}, errors.New("daily_review_projection_request_invalid")
	}
	operationRules := []string{providerContextAuthorityRule, capabilityDailyReviewPolicyInstruction, capabilityLifeConsistencyInstruction}
	currentInput := jsonString(map[string]any{"local_date": input.LocalDate})
	assembly, projection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceDailyReview, input.Projection, "cognitive_assessment", operationRules, currentInput, definitions, "daily_review_response", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "daily_review"), assembly.Diagnostics)
	providerCtx = a.bindProjectionRefresh(providerCtx, input.ProjectionRequest, ProviderContextSurfaceDailyReview, "cognitive_assessment", operationRules, currentInput, definitions, "daily_review_response", schema, nil)
	run, err := a.runFormalStructuredTask(providerCtx, FormalAgentDailyReview, assembly.Messages, definitions, "daily_review_response", schema, true, formalTaskCapabilityRequest(FormalAgentDailyReview, projection, CapabilitySurfaceAutonomy))
	if run.Projection != nil {
		projection = *run.Projection
	}
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
		{"role": "system", "content": scheduleReplanPlannerInstruction(input)},
		{"role": "user", "content": jsonString(scheduleReplanModelInput(input))},
	})
	run, err := a.runFormalStructuredTask(WithProviderScenario(ctx, "schedule_replan_planner"), FormalAgentScheduleReplan, messages, nil, "schedule_replan_plan", schedulePlannerOutputSchemaForInput(input), false, nil)
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
