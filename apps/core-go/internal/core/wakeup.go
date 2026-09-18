package core

// StructuredAssembledWithToolsSchema remains the canonical assembled Eino task boundary.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	defaultWakeUpIntervalSeconds = 30 * 60
	minWakeUpIntervalSeconds     = 5 * 60
	maxWakeUpIntervalSeconds     = 24 * 60 * 60
)

// WakeUpSettings controls the durable internal-life timer. The setting is
// intentionally small and owner-editable through product settings so a local
// deployment can trade model cost for more or less frequent self-reflection.
// The workflow still clamps the interval so an accidental value cannot create
// a tight provider loop or make the persona effectively dormant.
type WakeUpSettings struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"interval_seconds"`
}

func defaultWakeUpSettings() WakeUpSettings {
	return WakeUpSettings{Enabled: true, IntervalSeconds: defaultWakeUpIntervalSeconds}
}

func nextWakeUpDue(previousDue, now time.Time, intervalSeconds int) time.Time {
	if intervalSeconds <= 0 {
		intervalSeconds = defaultWakeUpIntervalSeconds
	}
	now = now.UTC()
	interval := time.Duration(intervalSeconds) * time.Second
	if previousDue.IsZero() {
		return now.Add(interval)
	}
	previousDue = previousDue.UTC()
	if previousDue.After(now) {
		return previousDue
	}
	steps := now.Sub(previousDue)/interval + 1
	return previousDue.Add(steps * interval)
}

func (a *App) ensureWakeUpNextDue(ctx context.Context, fluctlightID string, cycle, intervalSeconds int, reason string) (time.Time, error) {
	var nextDue time.Time
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		nextDue, err = updateWakeUpNextDueTx(ctx, tx, fluctlightID, intervalSeconds, a.now().UTC())
		return err
	})
	if err != nil {
		return time.Time{}, err
	}
	correlationID := wakeUpCycleCorrelation(fluctlightID, cycle)
	a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
		Surface: "wake_up", Transition: LifecycleTransitionNextCycleScheduled,
		FluctlightID: fluctlightID, CorrelationID: correlationID,
		IntentID: "wake_up_intent:" + fluctlightID, WorkflowID: "wake_up:" + fluctlightID,
		Stage: "clock", Status: "scheduled", ReasonCode: reason,
		Attempt: cycle, NextDueAt: nextDue,
	})
	return nextDue.UTC(), nil
}

func (a *App) scheduleWakeUpHint(ctx context.Context, fluctlightID string, cycle int, nextDue time.Time) {
	delay := time.Until(nextDue)
	seconds := int((delay + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	if err := a.scheduleWakeUpTrigger(ctx, fluctlightID, seconds); err != nil {
		correlationID := wakeUpCycleCorrelation(fluctlightID, cycle)
		slog.Warn("Go Core WakeUp Redis hint failed",
			"fluctlight_id", fluctlightID,
			"correlation_id", correlationID,
			"error_type", fmt.Sprintf("%T", err),
		)
		a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
			Surface: "wake_up", Transition: LifecycleTransitionFailed, Severity: "warn",
			FluctlightID: fluctlightID, CorrelationID: correlationID,
			IntentID: "wake_up_intent:" + fluctlightID, WorkflowID: "wake_up:" + fluctlightID,
			Stage: "redis_hint", Status: "degraded", ReasonCode: "redis_hint_failed",
			ErrorCategory: "transport", ErrorCode: "redis_hint_failed", Retryable: true,
			Attempt: cycle, NextDueAt: nextDue,
		})
	}
}

func normalizeWakeUpSettings(value map[string]any) WakeUpSettings {
	settings := defaultWakeUpSettings()
	if enabled, ok := value["enabled"].(bool); ok {
		settings.Enabled = enabled
	}
	if raw, ok := value["interval_seconds"]; ok {
		settings.IntervalSeconds = int(numberOrDefault(raw, float64(settings.IntervalSeconds)))
	}
	if settings.IntervalSeconds < minWakeUpIntervalSeconds {
		settings.IntervalSeconds = minWakeUpIntervalSeconds
	}
	if settings.IntervalSeconds > maxWakeUpIntervalSeconds {
		settings.IntervalSeconds = maxWakeUpIntervalSeconds
	}
	return settings
}

func (a *App) readWakeUpSettings(ctx context.Context) (WakeUpSettings, error) {
	settings := defaultWakeUpSettings()
	var raw string
	err := a.DB.Pool().QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.wakeup'`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return settings, fmt.Errorf("product.wakeup setting is invalid: %w", err)
	}
	return normalizeWakeUpSettings(value), nil
}

// EnsureWakeUpIntents repairs the stable clock independently of Schedule.
// Schedule enriches WakeUp context but never owns autonomous-lifecycle liveness.
func (a *App) EnsureWakeUpIntents(ctx context.Context) (int64, error) {
	settings, err := a.readWakeUpSettings(ctx)
	if err != nil {
		return 0, err
	}
	var ensured int64
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		inserted, err := tx.Exec(ctx, `
		INSERT INTO public.platform_workflow_intents(
				intent_id,workflow_id,task_queue,intent_type,payload,status,next_attempt_at
			)
			SELECT
				'wake_up_intent:' || f.id,
				'wake_up:' || f.id,
				'lifecycle',
				'wake_up.current',
				jsonb_build_object('fluctlight_id', f.id, 'cycle', 0),
				'pending',
				now()+($1 * interval '1 second')
			FROM public.fluctlights AS f
			WHERE f.status IN ('active', 'paused')
			ON CONFLICT (intent_id) DO NOTHING`, settings.IntervalSeconds)
		if err != nil {
			return err
		}
		ensured += inserted.RowsAffected()
		requeued, err := tx.Exec(ctx, `
			UPDATE public.platform_workflow_intents AS i
			SET status='retry',next_attempt_at=now(),started_at=NULL,completed_at=NULL,last_error=NULL
			FROM public.fluctlights AS f
			WHERE i.intent_type='wake_up.current'
			  AND i.payload->>'fluctlight_id' = f.id
			  AND f.status IN ('active', 'paused')
			  AND i.status = 'failed'`)
		if err != nil {
			return err
		}
		ensured += requeued.RowsAffected()
		initialized, err := tx.Exec(ctx, `
			UPDATE public.platform_workflow_intents AS i
			SET next_attempt_at=now()+($1 * interval '1 second')
			FROM public.fluctlights AS f
			WHERE i.intent_type='wake_up.current'
			  AND i.payload->>'fluctlight_id'=f.id
			  AND f.status IN ('active','paused')
			  AND i.next_attempt_at IS NULL`, settings.IntervalSeconds)
		if err != nil {
			return err
		}
		ensured += initialized.RowsAffected()
		return nil
	})
	return ensured, err
}

func numberOrDefault(value any, fallback float64) float64 {
	if parsed, ok := numberFloat(value); ok {
		return parsed
	}
	return fallback
}

func wakeUpValue(value any, field string) (any, error) {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" || len([]rune(text)) > 4000 {
			return nil, fmt.Errorf("wake_up_%s_invalid", field)
		}
		return text, nil
	case map[string]any:
		if len(typed) == 0 || len(jsonBytes(typed)) > 12000 {
			return nil, fmt.Errorf("wake_up_%s_invalid", field)
		}
		return typed, nil
	default:
		return nil, fmt.Errorf("wake_up_%s_invalid", field)
	}
}

func normalizeWakeUpAssessment(value map[string]any) (map[string]any, error) {
	if value == nil {
		return nil, errors.New("wake_up_assessment_invalid")
	}
	result := make(map[string]any, len(value)+1)
	for key, raw := range value {
		result[key] = raw
	}
	actionType := stringValue(value["action_type"])
	if actionType == "" || (actionType != "no_op" && !validateSlotKey(actionType)) {
		return nil, errors.New("wake_up_action_type_invalid")
	}
	result["action_type"] = actionType
	if refs, ok := value["evidence_refs"]; ok {
		var normalizedRefs []any
		switch refs.(type) {
		case []any, []string:
			normalizedRefs = arrayValue(refs)
		default:
			return nil, errors.New("wake_up_evidence_refs_invalid")
		}
		if len(normalizedRefs) > 20 {
			return nil, errors.New("wake_up_evidence_refs_invalid")
		}
		for _, ref := range normalizedRefs {
			if text := stringValue(ref); text == "" || len([]rune(text)) > 256 {
				return nil, errors.New("wake_up_evidence_refs_invalid")
			}
		}
		result["evidence_refs"] = normalizedRefs
	} else {
		result["evidence_refs"] = []any{}
	}
	if intent := stringValue(value["response_intent"]); intent != "" {
		if len([]rune(intent)) > 4000 {
			return nil, errors.New("wake_up_response_intent_invalid")
		}
		result["response_intent"] = intent
	}
	return result, nil
}

func fallbackWakeUpActionWithoutCapability(proposedActionType string) (string, map[string]any) {
	return "no_op", map[string]any{"status": "no_op", "reason": "action_requires_capability_call", "proposed_action_type": proposedActionType}
}

// normalizeWakeUpActionWithoutCapability converts a model-proposed external
// action into a durable no-op before influence and output-binding validation.
// WakeUp may perform internal cognition without selecting a capability, but it
// must never fail the recurring cycle merely because the optional tool call is
// absent.
func normalizeWakeUpActionWithoutCapability(assessment map[string]any, calls []CapabilityInvocation) string {
	proposedActionType := stringValue(assessment["action_type"])
	if proposedActionType == "" || proposedActionType == "no_op" || len(calls) > 0 {
		return ""
	}
	assessment["action_type"] = "no_op"
	return proposedActionType
}

// wakeUpAssessmentFromToolCalls keeps a tool-only wake-up valid. Thinking
// providers are allowed to express the whole wake-up decision through native
// calls and omit the optional JSON sidecar. Output calls still determine the
// product action; any other registered calls remain capability work attached
// to the same wake-up.
func wakeUpAssessmentFromToolCalls(calls []CapabilityInvocation, registries ...*CapabilityRegistry) map[string]any {
	if len(calls) == 0 {
		return nil
	}
	if len(registries) > 0 && registries[0] != nil {
		for _, invocation := range calls {
			definition, ok := registries[0].Definition(invocation.CapabilityName)
			if !ok || (definition.OutputRole != "conversation_message" && definition.OutputRole != "moment") {
				continue
			}
			if text := capabilityInvocationText(invocation); text == "" {
				continue
			}
			if definition.OutputRole == "conversation_message" && containsCapabilityTarget(definition.TargetKinds, "conversation_message") {
				return map[string]any{"action_type": "proactive_message", "response_intent": "通过已注册输出能力向 actor_user 发送主动私聊", "evidence_refs": []any{}}
			}
			if definition.OutputRole == "moment" && containsCapabilityTarget(definition.TargetKinds, "moment") {
				return map[string]any{"action_type": "moment", "response_intent": "通过已注册输出能力发布主动动态", "evidence_refs": []any{}}
			}
		}
	}
	return map[string]any{
		"action_type":     "no_op",
		"response_intent": "执行本次 wake-up 返回的已注册能力",
		"evidence_refs":   []any{},
	}
}

func mergeWakeUpToolCallAssessment(assessment map[string]any, calls []CapabilityInvocation, registry *CapabilityRegistry) map[string]any {
	if assessment == nil {
		assessment = map[string]any{}
	}
	derived := wakeUpAssessmentFromToolCalls(calls, registry)
	if len(derived) == 0 {
		return assessment
	}
	derivedAction := stringValue(derived["action_type"])
	currentAction := stringValue(assessment["action_type"])
	if currentAction == "" || currentAction == "no_op" || derivedAction != "no_op" {
		if derivedAction != "no_op" || currentAction == "" {
			assessment["action_type"] = derivedAction
		}
		if stringValue(assessment["response_intent"]) == "" {
			assessment["response_intent"] = derived["response_intent"]
		}
		if _, exists := assessment["evidence_refs"]; !exists {
			assessment["evidence_refs"] = derived["evidence_refs"]
		}
	}
	return assessment
}

// wakeUpDecisionRequiresInfluences keeps output-only decisions usable when a
// thinking Provider omits the optional influence sidecar. State-changing and
// internal capabilities still require at least one Core-owned evidence
// influence; only deferred output capabilities can stand on their own because
// their durable target/result is the action boundary itself.
func wakeUpDecisionRequiresInfluences(actionType string, calls []CapabilityInvocation, registry *CapabilityRegistry) bool {
	if actionType == "no_op" && len(calls) == 0 {
		return false
	}
	if actionType != "no_op" && actionType != "proactive_message" && actionType != "moment" {
		return true
	}
	if registry == nil {
		return true
	}
	for _, invocation := range calls {
		definition, ok := registry.Definition(invocation.CapabilityName)
		if !ok || !definition.IsDeferredOutput() {
			return true
		}
	}
	return false
}

func wakeUpScheduleStatus(schedule map[string]any) string {
	if len(schedule) == 0 {
		return "missing"
	}
	return firstString(schedule["status"], "ready")
}

// ProcessWakeUp performs one bounded proactive-action assessment. Wake-up is
// not a second cognition/reflection pass: it decides whether a Moment, direct
// message, or installed capability should be proposed, records that decision,
// and schedules the existing reflection workflow against the resulting fact.
// External effects are frozen only after their capability contract and hard
// execution invariants pass; delivery itself remains owned by a Temporal
// action workflow.
func (a *App) ProcessWakeUp(ctx context.Context, fluctlightID string, cycle int) (map[string]any, error) {
	if fluctlightID == "" {
		return nil, errors.New("wake_up_fluctlight_id_required")
	}
	if cycle < 0 {
		return nil, errors.New("wake_up_cycle_invalid")
	}
	correlationID := wakeUpCycleCorrelation(fluctlightID, cycle)
	// Check the lifecycle marker before reading or mutating any Wake-up state.
	// Cognition can supersede a cycle while the activity is still between its
	// durable boundaries; returning a terminal cancellation here keeps that
	// cycle from repairing its clock or creating another action.
	if a.lifecycleCancellationRequested(ctx, WakeUpProviderCancellationMarker(fluctlightID, cycle)) {
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": "cancelled", "reason": "superseded_by_cognition"}, nil
	}
	settings, err := a.readWakeUpSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		nextDue, err := a.ensureWakeUpNextDue(ctx, fluctlightID, cycle, settings.IntervalSeconds, "wake_up_disabled")
		if err != nil {
			return nil, err
		}
		a.scheduleWakeUpHint(ctx, fluctlightID, cycle, nextDue)
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": "disabled", "reason": "wake_up_disabled", "interval_seconds": settings.IntervalSeconds, "next_due_at": nextDue.Format(time.RFC3339Nano)}, nil
	}
	fluctlight, err := a.readFluctlightByID(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	if fluctlight.Status == "paused" {
		nextDue, err := a.ensureWakeUpNextDue(ctx, fluctlightID, cycle, settings.IntervalSeconds, "fluctlight_paused")
		if err != nil {
			return nil, err
		}
		a.scheduleWakeUpHint(ctx, fluctlightID, cycle, nextDue)
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": "paused", "reason": "fluctlight_paused", "interval_seconds": settings.IntervalSeconds, "next_due_at": nextDue.Format(time.RFC3339Nano)}, nil
	}
	if fluctlight.Status != "active" {
		nextDue, err := a.ensureWakeUpNextDue(ctx, fluctlightID, cycle, settings.IntervalSeconds, "fluctlight_inactive")
		if err != nil {
			return nil, err
		}
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": "inactive", "reason": "fluctlight_not_active", "interval_seconds": settings.IntervalSeconds, "next_due_at": nextDue.Format(time.RFC3339Nano)}, nil
	}
	ctx = WithProviderExecutionGuard(ctx, a.providerGuardForFluctlight(fluctlightID))
	wakeID := "wake_up_" + stableDigest(fluctlightID+":"+fmt.Sprint(cycle))
	if a.lifecycleCancellationRequested(ctx, WakeUpProviderCancellationMarker(fluctlightID, cycle)) {
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": wakeUpCycleCorrelation(fluctlightID, cycle), "status": "cancelled", "reason": "superseded_by_cognition"}, nil
	}
	frozenActionID := "autonomy_wake_" + stableDigest(wakeID)
	var existingStatus, existingActionType string
	var existingActionID, existingReflectionIntentID *string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,action_type,action_id,reflection_intent_id FROM public.cognition_wakeups WHERE id=$1`, wakeID).Scan(&existingStatus, &existingActionType, &existingActionID, &existingReflectionIntentID); err == nil {
		nextDue, dueErr := a.ensureWakeUpNextDue(ctx, fluctlightID, cycle, settings.IntervalSeconds, "wake_up_replay_repaired")
		if dueErr != nil {
			return nil, dueErr
		}
		a.scheduleWakeUpHint(ctx, fluctlightID, cycle, nextDue)
		if existingReflectionIntentID != nil && strings.TrimSpace(*existingReflectionIntentID) != "" {
			if triggerErr := a.scheduleReflectionTrigger(ctx, fluctlightID, reflectionQuietPeriod); triggerErr != nil {
				return nil, triggerErr
			}
		}
		return map[string]any{"wake_up_id": wakeID, "fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": existingStatus, "reason": "wake_up_replayed", "action_type": existingActionType, "action_id": existingActionID, "reflection_intent_id": existingReflectionIntentID, "interval_seconds": settings.IntervalSeconds, "next_due_at": nextDue.Format(time.RFC3339Nano)}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var ownerID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerID); err != nil {
		return nil, err
	}
	conversationID := ""
	conversationErr := a.DB.Pool().QueryRow(ctx, `SELECT conversation_id FROM public.fluctlight_direct_conversations WHERE owner_actor_id=$1 AND fluctlight_actor_id=$2 ORDER BY created_at LIMIT 1`, ownerID, fluctlightID).Scan(&conversationID)
	if conversationErr != nil && !errors.Is(conversationErr, pgx.ErrNoRows) {
		return nil, conversationErr
	}
	if conversationID == "" {
		// WakeUp is allowed to send a proactive private message before the
		// Owner has opened the chat page. Ensure the Core-owned direct
		// conversation exists instead of demoting a valid conversation.reply
		// invocation into a capability action with no message target.
		conversationID, err = a.EnsureDirectConversation(ctx, ownerID, fluctlightID)
		if err != nil {
			return nil, err
		}
	}
	memoryMode := MemoryConversationExact
	projection, err := a.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		ConversationID: conversationID, SourceFactID: wakeID,
		MemoryOperation: MemoryForWakeUp, MemoryConversationMode: memoryMode,
	})
	if err != nil {
		return nil, err
	}
	visualIdentityActive, err := a.hasActiveVisualIdentity(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	if !visualIdentityActive {
		// The missing identity is an authoritative domain fact. Keep it in the
		// structured wake-up context so the persona can express the need without
		// relying on text matching or a hidden semantic parser.
		if projection.VisualIdentity == nil {
			projection.VisualIdentity = map[string]any{}
		}
		projection.VisualIdentity["missing"] = true
	}
	definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceWakeUp)
	schema := wakeUpResponseSchema()
	wakeUpContext := map[string]any{
		"wake_up_id":      wakeID,
		"cycle":           cycle,
		"schedule_status": wakeUpScheduleStatus(projection.Schedule),
	}
	assembly, assembledProjection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceWakeUp, projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityWakeUpPolicyInstruction}, jsonString(wakeUpContext), definitions, "wake_up_response", schema)
	if err != nil {
		return nil, err
	}
	projection = assembledProjection
	providerCtx := WithPromptDiagnostics(
		WithProviderCancellationKey(
			WithProviderCorrelation(WithProviderScenario(ctx, "wake_up"), correlationID),
			WakeUpProviderCancellationMarker(fluctlightID, cycle),
		),
		assembly.Diagnostics,
	)
	completion, err := a.RunStructuredToolsTask(providerCtx, ModelTask{Kind: ModelTaskStructuredAssessment, Role: "cognitive_assessment", Scenario: "wake_up", SchemaName: "wake_up_response"}, assembly.Messages, definitions, schema, true, structuredThinkingEnabledForSchema("wake_up_response"))
	if err != nil {
		if a.lifecycleCancellationRequested(ctx, WakeUpProviderCancellationMarker(fluctlightID, cycle)) {
			return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": "cancelled", "reason": "superseded_by_cognition"}, nil
		}
		if status, suppressed := providerSuppressionStatus(err); suppressed {
			reason := "fluctlight_not_active"
			if status == "paused" {
				reason = "fluctlight_paused"
			}
			return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": status, "reason": reason, "interval_seconds": settings.IntervalSeconds}, nil
		}
		return nil, err
	}
	if a.lifecycleCancellationRequested(ctx, WakeUpProviderCancellationMarker(fluctlightID, cycle)) {
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": "cancelled", "reason": "superseded_by_cognition"}, nil
	}
	assessment := completion.Structured
	toolCalls := append([]CapabilityInvocation(nil), completion.ToolCalls...)
	// A tool-only completion is a valid wake-up decision. The tools are the
	// model's decision surface; the JSON sidecar is optional metadata and must
	// not be used as a gate that discards an otherwise executable reply/media or
	// native capability call.
	if completion.StructuredFallback || len(assessment) == 0 || (len(toolCalls) > 0 && (stringValue(assessment["action_type"]) == "" || stringValue(assessment["action_type"]) == "no_op")) {
		assessment = mergeWakeUpToolCallAssessment(assessment, toolCalls, a.capabilityRegistry())
	}
	if assessment == nil {
		return nil, errors.New("wake_up_assessment_invalid")
	}
	assessment, err = normalizeWakeUpAssessment(assessment)
	if err != nil {
		return nil, err
	}
	_, err = freezeDecisionInfluences(assessment, projection, false)
	if err != nil {
		return nil, err
	}
	proposedActionWithoutCapability := normalizeWakeUpActionWithoutCapability(assessment, toolCalls)
	capabilityValidationFailures := make([]CapabilityResult, 0)
	// Freeze the invocation metadata and context snapshot before persistence.
	// Capability-local planning/preflight may perform I/O, so it runs from the
	// durable action worker rather than making a transient failure erase this
	// wake-up decision.
	toolCalls, err = a.bindCapabilityInvocationsToProjection(toolCalls, projection, frozenActionID, wakeID, CapabilitySurfaceWakeUp)
	if err != nil {
		if failures := capabilityBatchFailures(err); len(failures) > 0 {
			capabilityValidationFailures = mergeCapabilityResults(capabilityValidationFailures, failures)
		} else {
			return nil, err
		}
	}
	if err := a.validateCapabilityInvocationsForPersistence(toolCalls); err != nil {
		if failures := capabilityBatchFailures(err); len(failures) > 0 {
			capabilityValidationFailures = mergeCapabilityResults(capabilityValidationFailures, failures)
		} else {
			return nil, err
		}
	}
	if len(capabilityValidationFailures) > 0 {
		assessment["capability_results"] = capabilityResultValues(capabilityValidationFailures)
	}
	assessment["action_type"] = canonicalWakeUpActionType(stringValue(assessment["action_type"]), toolCalls, a.capabilityRegistry())
	if preference := mapValue(assessment["output_preference_decision"]); len(preference) > 0 {
		if normalized, normalizeErr := normalizeOutputPreferenceDecision(preference, stringValue(projection.PersonalityRuntime["active_profile_id"])); normalizeErr == nil {
			assessment["output_preference_decision"] = normalized
		}
	}
	// All Provider calls remain in the frozen invocation envelope. Deferred
	// output calls use the concrete output action while internal calls use the
	// generic capability action; no invocation is silently split out or lost.
	visualIdentityToolResults := make([]CapabilityResult, 0)
	composite, err := normalizeCompositeAction(assessment, toolCalls, wakeID, stringValue(assessment["action_type"]))
	if err != nil {
		return nil, err
	}
	toolCalls = composite.ToolCalls
	assessment["capability_invocations"] = toolCalls
	assessment["output_bindings"] = composite.OutputBindings
	proposedActionType := stringValue(assessment["action_type"])
	if preference := mapValue(assessment["output_preference_decision"]); len(preference) > 0 {
		assessment["output_preference_decision"] = evaluateOutputPreferenceAction(preference, proposedActionType, toolCalls, a.capabilityRegistry())
	}
	actualActionType := proposedActionType
	deferredOutput := hasDeferredOutputCapabilities(toolCalls, a.capabilityRegistry())
	if proposedActionType == "moment" {
		if err := validateCompositeOutputCapabilities(toolCalls, "moment", a.capabilityRegistry()); err != nil {
			return nil, fmt.Errorf("wake_up_output_binding_invalid: %w", err)
		}
	} else if proposedActionType == "proactive_message" {
		if err := validateCompositeOutputCapabilities(toolCalls, "conversation_message", a.capabilityRegistry()); err != nil {
			return nil, fmt.Errorf("wake_up_output_binding_invalid: %w", err)
		}
	}
	mediaComposite := deferredOutput && (proposedActionType == "moment" || proposedActionType == "proactive_message")
	result := map[string]any{"status": "no_op"}
	if proposedActionWithoutCapability != "" {
		_, result = fallbackWakeUpActionWithoutCapability(proposedActionWithoutCapability)
	}
	if len(visualIdentityToolResults) > 0 {
		result["visual_identity_capability_results"] = visualIdentityToolResults
	}
	policySnapshot := map[string]any{}
	policyReason := ""
	policyBlocked := false
	policyActionType := proposedActionType
	if policyActionType == "no_op" && len(toolCalls) > 0 {
		// Native scene/schedule/affect/memory calls are optional capabilities,
		// not separate autonomous product actions. Authorize the capability
		// bundle once; do not make the first tool name decide whether the whole
		// wake-up is blocked (for example, schedule.replan is not an
		// `allowed_actions` product output).
		policyActionType = "capability"
	}
	if policyActionType != "no_op" {
		policyDecision, policyErr := a.EvaluateAutonomyPolicy(ctx, fluctlightID, policyActionType, time.Now().UTC())
		if policyErr != nil {
			return nil, policyErr
		}
		policySnapshot = policyDecision.Snapshot
		if !policyDecision.Allowed {
			policyBlocked = true
			policyReason = policyDecision.Reason
			if len(toolCalls) == 0 {
				actualActionType = "no_op"
				result = map[string]any{"status": "blocked", "reason": policyReason, "proposed_action_type": proposedActionType}
			} else {
				// Keep every Provider invocation visible in the durable action even
				// when policy rejects execution. The action worker will settle these
				// calls as explicit failures; never erase a returned tool call.
				actualActionType = "capability"
				assessment["capability_results"] = capabilityFailureResultValues(toolCalls, "policy_"+policyReason, false)
				result = map[string]any{"status": "blocked", "reason": policyReason, "proposed_action_type": proposedActionType}
			}
		}
	}
	if len(toolCalls) > 0 && fluctlight.Status == "paused" {
		actualActionType = "capability"
		assessment["capability_results"] = capabilityFailureResultValues(toolCalls, "fluctlight_paused", false)
		result = map[string]any{"status": "blocked", "reason": "fluctlight_paused", "proposed_action_type": proposedActionType}
	} else if len(toolCalls) > 0 && proposedActionType == "no_op" {
		actualActionType = "capability"
		policySnapshot = map[string]any{"mode": "active", "authorization": "capability_manifest"}
		result = map[string]any{"status": "queued", "proposed_action_type": proposedActionType}
	}
	if proposedActionType != "no_op" && !policyBlocked {
		if fluctlight.Status == "paused" {
			policySnapshot = map[string]any{"mode": "paused", "allowed_actions": []string{}}
			policyReason = "fluctlight_paused"
			actualActionType = "capability"
			assessment["capability_results"] = capabilityFailureResultValues(toolCalls, "fluctlight_paused", false)
			result = map[string]any{"status": "blocked", "reason": policyReason, "proposed_action_type": proposedActionType}
		} else if len(toolCalls) > 0 && !mediaComposite {
			actualActionType = "capability"
			policySnapshot = map[string]any{"mode": "active", "authorization": "capability_manifest"}
			result = map[string]any{"status": "queued", "proposed_action_type": proposedActionType}
		} else if proposedActionType == "proactive_message" && conversationID == "" {
			// A missing delivery target is an execution failure, not permission
			// to discard the Provider invocation. Persist a capability action so
			// the call receives an explicit failed result during settlement.
			actualActionType = "capability"
			assessment["capability_results"] = capabilityFailureResultValues(toolCalls, "proactive_target_invalid", false)
			result = map[string]any{"status": "blocked", "reason": "proactive_target_invalid", "proposed_action_type": proposedActionType}
		} else if proposedActionType == "proactive_message" || proposedActionType == "moment" {
			targetKind := "moment"
			if proposedActionType == "proactive_message" {
				targetKind = "conversation_message"
			}
			visible := textFromOutputBinding(toolCalls, targetKind, a.capabilityRegistry())
			if visible == "" {
				return nil, fmt.Errorf("wake_up_%s_required", targetKind)
			}
			result = map[string]any{"status": "queued"}
			result["text"] = visible
		} else {
			// The assessment role is also used by ordinary conversation turns, so
			// a provider may conservatively return a chat-only action such as
			// "reply" even though this wake-up has no tool call. Preserve the
			// internal cognitive cycle and record the unsupported external choice
			// as a no-op instead of terminating the long-lived timer.
			actualActionType, result = fallbackWakeUpActionWithoutCapability(proposedActionType)
		}
	}
	if len(visualIdentityToolResults) > 0 {
		result["visual_identity_capability_results"] = visualIdentityToolResults
	}
	// A returned Provider invocation must always have a durable action owner.
	// If an earlier guard selected no_op for a non-empty call set, promote the
	// action to the generic capability lane instead of losing the invocation.
	if len(toolCalls) > 0 && actualActionType == "no_op" {
		actualActionType = "capability"
		if _, exists := result["status"]; !exists || stringValue(result["status"]) == "no_op" {
			result["status"] = "queued"
		}
	}
	if a.lifecycleCancellationRequested(ctx, WakeUpProviderCancellationMarker(fluctlightID, cycle)) {
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": "cancelled", "reason": "superseded_by_cognition"}, nil
	}
	var actionID string
	if actualActionType != "no_op" {
		actionID = frozenActionID
		result["action_id"] = actionID
		policySnapshot["budget_reserved"] = true
	}
	reflectionIntentID := "reflection_intent:wake:" + wakeID
	factID, nextDue, err := a.persistWakeUp(ctx, wakeID, fluctlightID, cycle, settings.IntervalSeconds, projection.ContextRevision, projection.CurrentStateRevision, projection.InnerState, projection.LifeContextRevision, assessment, actualActionType, actionID, result, reflectionIntentID, policySnapshot, conversationID, toolCalls)
	if err != nil {
		if errors.Is(err, errLifecycleSupersededByCognition) || a.lifecycleCancellationRequested(ctx, WakeUpProviderCancellationMarker(fluctlightID, cycle)) {
			return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": "cancelled", "reason": "superseded_by_cognition"}, nil
		}
		return nil, err
	}
	a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
		Surface: "wake_up", Transition: LifecycleTransitionNextCycleScheduled,
		FluctlightID: fluctlightID, CorrelationID: wakeUpCycleCorrelation(fluctlightID, cycle),
		IntentID: "wake_up_intent:" + fluctlightID, WorkflowID: "wake_up:" + fluctlightID,
		Stage: "clock", Status: "scheduled", ReasonCode: "wake_up_cycle_completed",
		Attempt: cycle, NextDueAt: nextDue,
	})
	if err := a.scheduleReflectionTrigger(ctx, fluctlightID, reflectionQuietPeriod); err != nil {
		return nil, err
	}
	// Redis expiration is a low-latency hint only. PostgreSQL next_attempt_at and
	// the Worker's due sweep remain the recurrence authority.
	a.scheduleWakeUpHint(ctx, fluctlightID, cycle, nextDue)
	capabilityResults := make([]CapabilityResult, 0)
	_ = factID
	safeResult := make(map[string]any, len(result))
	for key, value := range result {
		if key != "text" {
			safeResult[key] = value
		}
	}
	safeResult["capability_results"] = capabilityResults
	outcomeReason := firstString(result["reason"], "action_queued")
	if actualActionType == "no_op" {
		outcomeReason = firstString(result["reason"], "no_action_selected")
	}
	return map[string]any{"wake_up_id": wakeID, "fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": firstString(result["status"], "no_op"), "reason": outcomeReason, "action_type": actualActionType, "action_id": nullableString(actionID), "reflection_intent_id": reflectionIntentID, "result": safeResult, "interval_seconds": settings.IntervalSeconds, "next_due_at": nextDue.Format(time.RFC3339Nano)}, nil
}

// canonicalWakeUpActionType gives the visible output capability ownership of
// the WakeUp delivery lane. WakeUp's output target is chosen by Core, not by
// sidecar action metadata: a conversation.reply call with text must settle
// against a real conversation message rather than falling into capability.action
// with a wake_up binding.
func canonicalWakeUpActionType(actionType string, calls []CapabilityInvocation, registry *CapabilityRegistry) string {
	canonical := normalizeConversationActionType(actionType)
	if replyTextFromCapabilityInvocations(calls, registry) != "" {
		return "proactive_message"
	}
	return canonical
}

func capabilityInvocationText(invocation CapabilityInvocation) string {
	var args map[string]any
	if json.Unmarshal(invocation.Arguments, &args) != nil {
		return ""
	}
	text := normalizeVisibleReply(stringValue(args["text"]))
	if text == "" || len([]rune(text)) > 32000 {
		return ""
	}
	return text
}

func capabilityFailureResultValues(invocations []CapabilityInvocation, code string, retryable bool) []any {
	result := make([]any, 0, len(invocations))
	for _, invocation := range invocations {
		result = append(result, failedCapabilityResult(invocation, code, retryable))
	}
	return result
}

func textFromOutputBinding(calls []CapabilityInvocation, targetKind string, registry *CapabilityRegistry) string {
	for _, invocation := range calls {
		if registry != nil {
			definition, ok := registry.Definition(invocation.CapabilityName)
			if !ok || definition.OutputRole != targetKind || !containsCapabilityTarget(definition.TargetKinds, targetKind) {
				continue
			}
		}
		if text := capabilityInvocationText(invocation); text != "" {
			return text
		}
	}
	return ""
}

func (a *App) persistWakeUp(ctx context.Context, wakeID, fluctlightID string, cycle, intervalSeconds, foundationRevision, currentStateRevision int, internalDynamics map[string]any, lifeContextRevision string, assessment map[string]any, actionType, actionID string, result map[string]any, reflectionIntentID string, policySnapshot map[string]any, conversationID string, toolCalls []CapabilityInvocation) (string, time.Time, error) {
	factID := "wake_fact_" + stableDigest(wakeID)
	correlationID := wakeUpCycleCorrelation(fluctlightID, cycle)
	var nextDue time.Time
	workflowID := "autonomy_wake:" + wakeID
	payload := map[string]any{
		"event_type": "internal.wake_up", "wake_up_id": wakeID, "fluctlight_id": fluctlightID,
		"cycle": cycle, "action_type": actionType, "correlation_id": correlationID,
		"response_intent": assessment["response_intent"], "evidence_refs": assessment["evidence_refs"],
	}
	causality, err := frozenDecisionCausality(assessment)
	if err != nil {
		return "", time.Time{}, err
	}
	for key, value := range causality {
		payload[key] = value
	}
	if preference := mapValue(assessment["output_preference_decision"]); len(preference) > 0 {
		payload["output_preference_decision"] = preference
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fluctlightID); err != nil {
			return err
		}
		if a.ProviderCancellationRequested(ctx, WakeUpProviderCancellationMarker(fluctlightID, cycle)) {
			return errLifecycleSupersededByCognition
		}
		if superseded, err := lifecycleIntentSupersededTx(ctx, tx); err != nil {
			return err
		} else if superseded {
			return errLifecycleSupersededByCognition
		}
		if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, foundationRevision, currentStateRevision, lifeContextRevision, a.now().UTC()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, wakeID); err != nil {
			return err
		}
		var existing string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.cognition_wakeups WHERE id=$1 FOR UPDATE`, wakeID).Scan(&existing); err == nil {
			nextDue, err = updateWakeUpNextDueTx(ctx, tx, fluctlightID, intervalSeconds, a.now().UTC())
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,1,0) ON CONFLICT DO NOTHING`, fluctlightID); err != nil {
			return err
		}
		var sequence int
		if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.cognition_inbox_heads WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&sequence); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads SET next_sequence=$2,last_processed_sequence=GREATEST(last_processed_sequence,$3) WHERE fluctlight_id=$1`, fluctlightID, sequence+1, sequence); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES($1,$2,$3,'internal.wake_up',$4,$5,$6,$7,now(),'processed',now()) ON CONFLICT DO NOTHING`, factID, fluctlightID, sequence, jsonBytes(payload), wakeID, correlationID, wakeID); err != nil {
			return err
		}
		// Wake-up is an action check, not a cognition/state-growth pass. Keep the
		// current snapshot for audit compatibility and leave Current State alone.
		stagePlaceholder := map[string]any{}
		wakeResult := map[string]any{"status": result["status"], "correlation_id": correlationID, "action_id": nullableString(actionID), "conversation_id": nullableString(conversationID)}
		for key, value := range causality {
			wakeResult[key] = value
		}
		for key, value := range result {
			if key != "text" {
				wakeResult[key] = value
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_wakeups(id,fluctlight_id,cycle,internal_dynamics,attention,thought,desire,agency,action_type,action_id,result,reflection_intent_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT DO NOTHING`, wakeID, fluctlightID, cycle, jsonBytes(internalDynamics), jsonBytes(stagePlaceholder), jsonBytes(stagePlaceholder), jsonBytes(stagePlaceholder), jsonBytes(stagePlaceholder), actionType, nullableString(actionID), jsonBytes(wakeResult), reflectionIntentID); err != nil {
			return err
		}
		if err := insertReflectionIntentWithDelayTx(ctx, tx,
			reflectionIntentID,
			"reflection:wake:"+wakeID,
			map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "wake_up_id": wakeID, "correlation_id": correlationID, "causation_id": factID},
			reflectionQuietPeriod,
		); err != nil {
			return err
		}
		if actionID != "" && actionType == "capability" {
			if err := reserveAutonomyBudgetTx(ctx, tx, fluctlightID); err != nil {
				return err
			}
			actionPayload := map[string]any{"capability_runtime_version": CapabilityRuntimePayloadVersion, "wake_up_id": wakeID, "source_fact_id": factID, "correlation_id": correlationID, "causation_id": factID, "conversation_id": conversationID, "capability_invocations": toolCalls, "capability_results": arrayValue(assessment["capability_results"])}
			actionPayload["context_reference_version"] = contextReferenceIndexVersion
			actionPayload["context_reference_index"] = assessment["context_reference_index"]
			actionPayload["influences"] = assessment["influences"]
			actionPayload["goal_refs"] = assessment["goal_refs"]
			actionPayload["intention_refs"] = assessment["intention_refs"]
			if preference := mapValue(assessment["output_preference_decision"]); len(preference) > 0 {
				actionPayload["output_preference_decision"] = preference
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_actions(id,fluctlight_id,action_type,payload,policy_snapshot,expected_revisions,status,workflow_id,provider_request_id) VALUES($1,$2,$3,$4,$5,$6,'frozen',$7,$8) ON CONFLICT DO NOTHING`, actionID, fluctlightID, actionType, jsonBytes(actionPayload), jsonBytes(policySnapshot), jsonBytes(map[string]any{"foundation_revision": foundationRevision, "life_context_revision": lifeContextRevision, "current_state_revision": currentStateRevision}), workflowID, "provider_wakeup_"+stableDigest(wakeID)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','capability.action',$3) ON CONFLICT DO NOTHING`, "capability_wake_intent:"+wakeID, workflowID, jsonBytes(map[string]any{"action_id": actionID, "fluctlight_id": fluctlightID, "wake_up_id": wakeID, "source_fact_id": factID, "correlation_id": correlationID, "causation_id": factID})); err != nil {
				return err
			}
		} else if actionID != "" {
			if err := reserveAutonomyBudgetTx(ctx, tx, fluctlightID); err != nil {
				return err
			}
			visible := stringValue(result["text"])
			// Keep the action strict: a frozen action can never be queued without a
			// visible payload from the realization stage.
			if visible == "" {
				return errors.New("wake_up_action_payload_empty")
			}
			actionPayload := map[string]any{"capability_runtime_version": CapabilityRuntimePayloadVersion, "wake_up_id": wakeID, "source_fact_id": factID, "correlation_id": correlationID, "causation_id": factID, "text": visible, "conversation_id": conversationID, "response_intent": assessment["response_intent"], "capability_invocations": toolCalls, "capability_results": arrayValue(assessment["capability_results"]), "output_bindings": assessment["output_bindings"]}
			actionPayload["context_reference_version"] = contextReferenceIndexVersion
			actionPayload["context_reference_index"] = assessment["context_reference_index"]
			actionPayload["influences"] = assessment["influences"]
			actionPayload["goal_refs"] = assessment["goal_refs"]
			actionPayload["intention_refs"] = assessment["intention_refs"]
			if preference := mapValue(assessment["output_preference_decision"]); len(preference) > 0 {
				actionPayload["output_preference_decision"] = preference
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_actions(id,fluctlight_id,action_type,payload,policy_snapshot,expected_revisions,status,workflow_id,provider_request_id) VALUES($1,$2,$3,$4,$5,$6,'frozen',$7,$8) ON CONFLICT DO NOTHING`, actionID, fluctlightID, actionType, jsonBytes(actionPayload), jsonBytes(policySnapshot), jsonBytes(map[string]any{"foundation_revision": foundationRevision, "life_context_revision": lifeContextRevision, "current_state_revision": currentStateRevision}), workflowID, "provider_wakeup_"+stableDigest(wakeID)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','autonomy.action',$3) ON CONFLICT DO NOTHING`, "autonomy_wake_intent:"+wakeID, workflowID, jsonBytes(map[string]any{"action_id": actionID, "fluctlight_id": fluctlightID, "wake_up_id": wakeID, "source_fact_id": factID, "correlation_id": correlationID, "causation_id": factID})); err != nil {
				return err
			}
		}
		if err := appendOutboxTx(ctx, tx, "cognition.fact.created", "fluctlight", fluctlightID, fluctlightID, wakeID, correlationID, "wake-fact:"+wakeID, payload); err != nil {
			return err
		}
		nextDue, err = updateWakeUpNextDueTx(ctx, tx, fluctlightID, intervalSeconds, a.now().UTC())
		if err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "wake_up.completed", "fluctlight", fluctlightID, fluctlightID, wakeID, correlationID, "wake-up:"+wakeID, map[string]any{"wake_up_id": wakeID, "cycle": cycle, "action_type": actionType, "reflection_intent_id": reflectionIntentID, "correlation_id": correlationID, "next_due_at": nextDue.Format(time.RFC3339Nano)})
	})
	return factID, nextDue, err
}

func updateWakeUpNextDueTx(ctx context.Context, tx pgx.Tx, fluctlightID string, intervalSeconds int, now time.Time) (time.Time, error) {
	// Lock the single durable clock row so a completion and a cognition
	// follow-up cannot overwrite one another silently.  The next Wake-up is
	// measured from this completion boundary, not from an older nominal slot:
	// if a provider call ran late, preserving fixed cadence would make the next
	// Redis key expire almost immediately and defeat the configured quiet time.
	if err := tx.QueryRow(ctx, `
		SELECT 1
		FROM public.platform_workflow_intents
		WHERE intent_type='wake_up.current'
		  AND payload->>'fluctlight_id'=$1
		FOR UPDATE`, fluctlightID).Scan(new(int)); errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrNotFound
	} else if err != nil {
		return time.Time{}, err
	}
	if intervalSeconds <= 0 {
		intervalSeconds = defaultWakeUpIntervalSeconds
	}
	nextDue := now.UTC().Add(time.Duration(intervalSeconds) * time.Second)
	command, err := tx.Exec(ctx, `
		UPDATE public.platform_workflow_intents
		SET next_attempt_at=$2,attempt_count=0
		WHERE intent_type='wake_up.current'
		  AND payload->>'fluctlight_id'=$1`, fluctlightID, nextDue)
	if err != nil {
		return time.Time{}, err
	}
	if command.RowsAffected() != 1 {
		return time.Time{}, errors.New("wake_up_clock_not_written")
	}
	return nextDue, nil
}

func insertReflectionIntentTx(ctx context.Context, tx pgx.Tx, intentID, workflowID string, payload map[string]any) error {
	return insertReflectionIntentWithDelayTx(ctx, tx, intentID, workflowID, payload, 0)
}

// insertReflectionIntentWithDelayTx keeps the historical configured interval
// for independent action/outcome producers while allowing the cognition and
// Wake-up chains to opt into their explicit ten-minute debounce contract.
// A zero delay means "use the product Wake-up setting" for those independent
// producers; a positive delay is measured from this LLM completion boundary.
func insertReflectionIntentWithDelayTx(ctx context.Context, tx pgx.Tx, intentID, workflowID string, payload map[string]any, delay time.Duration) error {
	if delay <= 0 {
		settings := defaultWakeUpSettings()
		var raw string
		err := tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.wakeup'`).Scan(&raw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			var value map[string]any
			if json.Unmarshal([]byte(raw), &value) != nil {
				return errors.New("product_wakeup_setting_invalid")
			}
			settings = normalizeWakeUpSettings(value)
		}
		delay = time.Duration(settings.IntervalSeconds) * time.Second
	}
	nextReflectionAt := time.Now().UTC().Add(delay)
	command, err := tx.Exec(ctx, `
		INSERT INTO public.platform_workflow_intents(
			intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at
		) VALUES($1,$2,'lifecycle','reflection.run',$3,$4)
		ON CONFLICT(intent_id) DO UPDATE SET
			next_attempt_at=CASE
				WHEN public.platform_workflow_intents.status IN ('pending','retry')
				THEN GREATEST(public.platform_workflow_intents.next_attempt_at,excluded.next_attempt_at)
				ELSE public.platform_workflow_intents.next_attempt_at
			END`, intentID, workflowID, jsonBytes(payload), nextReflectionAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return errors.New("reflection_intent_not_written")
	}
	return nil
}

func (a *App) persistWakeCapabilityResults(ctx context.Context, wakeID string, results []CapabilityResult) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_wakeups SET result=result || $2::jsonb WHERE id=$1`, wakeID, jsonBytes(map[string]any{"capability_results": results})); err != nil {
			return err
		}
		var fluctlightID string
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id FROM public.cognition_wakeups WHERE id=$1`, wakeID).Scan(&fluctlightID); err != nil {
			return err
		}
		factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "capability.requested", map[string]any{"wake_up_id": wakeID, "capability_results": results}, "capability-result:"+wakeID)
		if err != nil {
			return err
		}
		return insertReflectionIntentWithDelayTx(ctx, tx,
			"reflection_intent:capability:"+wakeID,
			"reflection:capability:"+wakeID,
			map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "wake_up_id": wakeID},
			reflectionQuietPeriod,
		)
	})
}
