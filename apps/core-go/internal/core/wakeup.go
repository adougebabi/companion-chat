package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// EnsureWakeUpIntents repairs the durable entry point for Fluctlights that
// were created before the wake-up debounce feature existed. Failed intents for
// still-live Fluctlights are made retryable; completed intents wait for their
// Redis quiet-period hint instead of being launched immediately at startup.
func (a *App) EnsureWakeUpIntents(ctx context.Context) (int64, error) {
	var ensured int64
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
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
				now()
			FROM public.fluctlights AS f
			WHERE f.status IN ('active', 'paused')
			  AND EXISTS (
				SELECT 1
				FROM public.life_schedules AS s
				WHERE s.fluctlight_id = f.id
				  AND s.status = 'accepted'
				  AND s.local_date = (now() AT TIME ZONE COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai'))::date
			  )
			ON CONFLICT (intent_id) DO NOTHING`)
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
			  AND EXISTS (
				SELECT 1
				FROM public.life_schedules AS s
				WHERE s.fluctlight_id = f.id
				  AND s.status = 'accepted'
				  AND s.local_date = (now() AT TIME ZONE COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai'))::date
			  )
			  AND i.status = 'failed'`)
		if err != nil {
			return err
		}
		ensured += requeued.RowsAffected()
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
			if !ok || (!containsCapabilityTarget(definition.TargetKinds, "conversation_message") && !containsCapabilityTarget(definition.TargetKinds, "moment")) {
				continue
			}
			if text := capabilityInvocationText(invocation); text == "" {
				continue
			}
			if containsCapabilityTarget(definition.TargetKinds, "conversation_message") {
				return map[string]any{"action_type": "proactive_message", "response_intent": "通过已注册输出能力向 actor_user 发送主动私聊", "evidence_refs": []any{}}
			}
			return map[string]any{"action_type": "moment", "response_intent": "通过已注册输出能力发布主动动态", "evidence_refs": []any{}}
		}
	}
	return map[string]any{
		"action_type":     "no_op",
		"response_intent": "执行本次 wake-up 返回的已注册能力",
		"evidence_refs":   []any{},
	}
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
	settings, err := a.readWakeUpSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "status": "disabled", "interval_seconds": settings.IntervalSeconds}, nil
	}
	fluctlight, err := a.readFluctlightByID(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	if fluctlight.Status == "paused" {
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "status": "paused", "reason": "fluctlight_paused", "interval_seconds": settings.IntervalSeconds}, nil
	}
	if fluctlight.Status != "active" {
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "status": "inactive", "interval_seconds": settings.IntervalSeconds}, nil
	}
	ctx = WithProviderExecutionGuard(ctx, a.providerGuardForFluctlight(fluctlightID))
	wakeID := "wake_up_" + stableDigest(fluctlightID+":"+fmt.Sprint(cycle))
	frozenActionID := "autonomy_wake_" + stableDigest(wakeID)
	var existingStatus, existingActionType string
	var existingActionID, existingReflectionIntentID *string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,action_type,action_id,reflection_intent_id FROM public.cognition_wakeups WHERE id=$1`, wakeID).Scan(&existingStatus, &existingActionType, &existingActionID, &existingReflectionIntentID); err == nil {
		return map[string]any{"wake_up_id": wakeID, "fluctlight_id": fluctlightID, "cycle": cycle, "status": existingStatus, "action_type": existingActionType, "action_id": existingActionID, "reflection_intent_id": existingReflectionIntentID, "interval_seconds": settings.IntervalSeconds}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var ownerID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerID); err != nil {
		return nil, err
	}
	conversationID := ""
	_ = a.DB.Pool().QueryRow(ctx, `SELECT conversation_id FROM public.fluctlight_direct_conversations WHERE fluctlight_actor_id=$1 ORDER BY created_at LIMIT 1`, fluctlightID).Scan(&conversationID)
	memoryMode := MemoryConversationGlobalOnly
	if conversationID != "" {
		memoryMode = MemoryConversationExact
	}
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
	assembly, assembledProjection, err := a.assembleProjectionPrompt(ctx, projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityWakeUpPolicyInstruction}, jsonString(map[string]any{"wake_up_id": wakeID, "cycle": cycle}), definitions, "wake_up_response", schema)
	if err != nil {
		return nil, err
	}
	projection = assembledProjection
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "wake_up"), assembly.Diagnostics)
	completion, err := a.Provider.StructuredAssembledWithToolsSchema(providerCtx, "cognitive_assessment", assembly.Messages, definitions, "wake_up_response", schema, true)
	if err != nil {
		if status, suppressed := providerSuppressionStatus(err); suppressed {
			reason := "fluctlight_not_active"
			if status == "paused" {
				reason = "fluctlight_paused"
			}
			return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "status": status, "reason": reason, "interval_seconds": settings.IntervalSeconds}, nil
		}
		return nil, err
	}
	assessment := completion.Structured
	toolCalls := append([]CapabilityInvocation(nil), completion.ToolCalls...)
	// A tool-only completion is a valid wake-up decision. The tools are the
	// model's decision surface; the JSON sidecar is optional metadata and must
	// not be used as a gate that discards an otherwise executable reply/media or
	// native capability call.
	if completion.StructuredFallback || len(assessment) == 0 || (len(toolCalls) > 0 && (stringValue(assessment["action_type"]) == "" || stringValue(assessment["action_type"]) == "no_op")) {
		if derived := wakeUpAssessmentFromToolCalls(toolCalls, a.capabilityRegistry()); derived != nil {
			assessment = derived
		}
	}
	if assessment == nil {
		return nil, errors.New("wake_up_assessment_invalid")
	}
	assessment, err = normalizeWakeUpAssessment(assessment)
	if err != nil {
		return nil, err
	}
	influences, err := freezeDecisionInfluences(assessment, projection, false)
	if err != nil {
		return nil, err
	}
	if stringValue(assessment["action_type"]) != "no_op" || len(toolCalls) > 0 {
		if err := requireDecisionInfluences(influences, "wake_up_influences_required"); err != nil {
			return nil, err
		}
	}
	// Capability-local planning is HOW work and may perform Provider I/O. It
	// starts only after the semantic decision and all cited refs are validated.
	toolCalls, err = a.bindCapabilityInvocationsToProjection(toolCalls, projection, frozenActionID, wakeID, CapabilitySurfaceWakeUp)
	if err != nil {
		return nil, err
	}
	toolCalls, err = a.prepareCapabilityInvocations(ctx, fluctlightID, conversationID, wakeID, toolCalls)
	if err != nil {
		return nil, err
	}
	if preference := mapValue(assessment["output_preference_decision"]); len(preference) > 0 {
		if normalized, normalizeErr := normalizeOutputPreferenceDecision(preference, stringValue(projection.PersonalityRuntime["active_profile_id"])); normalizeErr == nil {
			assessment["output_preference_decision"] = normalized
		}
	}
	// All calls, including WakeUp-only internal capabilities, remain on the
	// same generic capability action. Definition failure policy decides whether
	// an error is required or optional; no concrete name is split out here.
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
			actualActionType = "no_op"
			toolCalls = nil
			result = map[string]any{"status": "blocked", "reason": policyReason, "proposed_action_type": proposedActionType}
		}
	}
	if len(toolCalls) > 0 && fluctlight.Status == "paused" {
		actualActionType = "no_op"
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
			actualActionType = "no_op"
			result = map[string]any{"status": "blocked", "reason": policyReason, "proposed_action_type": proposedActionType}
		} else if len(toolCalls) > 0 && !mediaComposite {
			actualActionType = "capability"
			policySnapshot = map[string]any{"mode": "active", "authorization": "capability_manifest"}
			result = map[string]any{"status": "queued", "proposed_action_type": proposedActionType}
		} else if proposedActionType == "proactive_message" && conversationID == "" {
			actualActionType = "no_op"
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
	var actionID string
	if actualActionType != "no_op" {
		actionID = frozenActionID
		result["action_id"] = actionID
		policySnapshot["budget_reserved"] = true
	}
	reflectionIntentID := "reflection_intent:wake:" + wakeID
	factID, err := a.persistWakeUp(ctx, wakeID, fluctlightID, cycle, projection.ContextRevision, projection.CurrentStateRevision, projection.InnerState, projection.LifeContextRevision, assessment, actualActionType, actionID, result, reflectionIntentID, policySnapshot, conversationID, toolCalls)
	if err != nil {
		return nil, err
	}
	// Redis expiration is a low-latency hint only; the long-lived Temporal
	// workflow remains the durable wake-up timer and recovery authority.
	a.scheduleWakeUpTrigger(ctx, fluctlightID, settings.IntervalSeconds)
	capabilityResults := make([]CapabilityResult, 0)
	_ = factID
	safeResult := make(map[string]any, len(result))
	for key, value := range result {
		if key != "text" {
			safeResult[key] = value
		}
	}
	safeResult["capability_results"] = capabilityResults
	return map[string]any{"wake_up_id": wakeID, "fluctlight_id": fluctlightID, "cycle": cycle, "status": "completed", "action_type": actualActionType, "action_id": nullableString(actionID), "reflection_intent_id": reflectionIntentID, "result": safeResult, "interval_seconds": settings.IntervalSeconds}, nil
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

func (a *App) persistWakeUp(ctx context.Context, wakeID, fluctlightID string, cycle, foundationRevision, currentStateRevision int, internalDynamics map[string]any, lifeContextRevision string, assessment map[string]any, actionType, actionID string, result map[string]any, reflectionIntentID string, policySnapshot map[string]any, conversationID string, toolCalls []CapabilityInvocation) (string, error) {
	factID := "wake_fact_" + stableDigest(wakeID)
	workflowID := "autonomy_wake:" + wakeID
	payload := map[string]any{
		"event_type": "internal.wake_up", "wake_up_id": wakeID, "fluctlight_id": fluctlightID,
		"cycle": cycle, "action_type": actionType,
		"response_intent": assessment["response_intent"], "evidence_refs": assessment["evidence_refs"],
	}
	causality, err := frozenDecisionCausality(assessment)
	if err != nil {
		return "", err
	}
	for key, value := range causality {
		payload[key] = value
	}
	if preference := mapValue(assessment["output_preference_decision"]); len(preference) > 0 {
		payload["output_preference_decision"] = preference
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, foundationRevision, currentStateRevision, lifeContextRevision, time.Now().UTC()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, wakeID); err != nil {
			return err
		}
		var existing string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.cognition_wakeups WHERE id=$1 FOR UPDATE`, wakeID).Scan(&existing); err == nil {
			return nil
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
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES($1,$2,$3,'internal.wake_up',$4,$5,$6,$7,now(),'processed',now()) ON CONFLICT DO NOTHING`, factID, fluctlightID, sequence, jsonBytes(payload), wakeID, wakeID, wakeID); err != nil {
			return err
		}
		// Wake-up is an action check, not a cognition/state-growth pass. Keep the
		// current snapshot for audit compatibility and leave Current State alone.
		stagePlaceholder := map[string]any{}
		wakeResult := map[string]any{"status": result["status"], "action_id": nullableString(actionID), "conversation_id": nullableString(conversationID)}
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
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, reflectionIntentID, "reflection:wake:"+wakeID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "wake_up_id": wakeID})); err != nil {
			return err
		}
		if actionID != "" && actionType == "capability" {
			if err := reserveAutonomyBudgetTx(ctx, tx, fluctlightID); err != nil {
				return err
			}
			actionPayload := map[string]any{"capability_runtime_version": CapabilityRuntimePayloadVersion, "wake_up_id": wakeID, "source_fact_id": factID, "conversation_id": conversationID, "capability_invocations": toolCalls, "capability_results": []CapabilityResult{}}
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
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','capability.action',$3) ON CONFLICT DO NOTHING`, "capability_wake_intent:"+wakeID, workflowID, jsonBytes(map[string]any{"action_id": actionID, "fluctlight_id": fluctlightID, "wake_up_id": wakeID, "source_fact_id": factID})); err != nil {
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
			actionPayload := map[string]any{"capability_runtime_version": CapabilityRuntimePayloadVersion, "wake_up_id": wakeID, "source_fact_id": factID, "text": visible, "conversation_id": conversationID, "response_intent": assessment["response_intent"], "capability_invocations": toolCalls, "capability_results": []CapabilityResult{}, "output_bindings": assessment["output_bindings"]}
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
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','autonomy.action',$3) ON CONFLICT DO NOTHING`, "autonomy_wake_intent:"+wakeID, workflowID, jsonBytes(map[string]any{"action_id": actionID, "fluctlight_id": fluctlightID, "wake_up_id": wakeID})); err != nil {
				return err
			}
		}
		if err := appendOutboxTx(ctx, tx, "cognition.fact.created", "fluctlight", fluctlightID, fluctlightID, wakeID, wakeID, "wake-fact:"+wakeID, payload); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "wake_up.completed", "fluctlight", fluctlightID, fluctlightID, wakeID, wakeID, "wake-up:"+wakeID, map[string]any{"wake_up_id": wakeID, "cycle": cycle, "action_type": actionType, "reflection_intent_id": reflectionIntentID})
	})
	return factID, err
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
		_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:capability:"+wakeID, "reflection:capability:"+wakeID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "wake_up_id": wakeID}))
		return err
	})
}
