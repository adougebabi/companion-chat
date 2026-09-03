package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
// were created before the periodic wake-up feature existed. Terminal wake-up
// intents for still-live Fluctlights are made retryable, while pending/retry/
// started intents are left untouched so a running Temporal execution cannot be
// duplicated.
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
			  AND i.status IN ('completed', 'failed')`)
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
	for _, field := range []string{"attention", "thought", "desire", "agency"} {
		normalized, err := wakeUpValue(value[field], field)
		if err != nil {
			return nil, err
		}
		result[field] = normalized
	}
	appraisal, err := normalizeAppraisal(value["appraisal"])
	if err != nil {
		return nil, err
	}
	result["appraisal"] = appraisal
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

// ProcessWakeUp performs one complete internal-life cycle. It records the
// model's attention/thought/desire/agency as a private cognition fact, then
// schedules the existing reflection workflow against that fact. External
// effects are frozen only after their capability contract and hard execution
// invariants pass; delivery itself remains owned by a Temporal action workflow.
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
	if fluctlight.Status != "active" && fluctlight.Status != "paused" {
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "status": "inactive", "interval_seconds": settings.IntervalSeconds}, nil
	}
	wakeID := "wake_up_" + stableDigest(fluctlightID+":"+fmt.Sprint(cycle))
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
	projection, err := a.BuildContextProjection(ctx, ownerID, fluctlightID, conversationID, wakeID, "")
	if err != nil {
		return nil, err
	}
	messages := withContextAuthorityInstruction([]map[string]any{
		{"role": "system", "content": "You are evaluating one internal wake-up for a Fluctlight. Return JSON with attention, thought, desire, agency, and action_type. These fields describe the internal cognitive cycle, not visible prose. Keep each stage as a concise summary; do not provide private chain-of-thought or hidden reasoning. Do not invent facts. Choose no_op when no action is wanted. For a visible proactive_message or moment that needs an image, issue the media.image.generate tool call with the complete visual concept; do not return moment_media_request or message_media_request fields. If another installed capability is needed, issue its tool call and use its capability name as action_type; if the needed capability is missing, issue capability.request. Never return visible text; response_intent is optional and must only explain an explicitly proposed action."},
		{"role": "user", "content": jsonString(map[string]any{"wake_up_id": wakeID, "cycle": cycle, "context": compactCognitionContext(projection)})},
	})
	completion, err := a.Provider.StructuredWithToolsSchema(ctx, "cognitive_assessment", messages, a.capabilityRegistry().Manifests(), "wake_up_response", wakeUpResponseSchema(), true)
	if err != nil {
		return nil, err
	}
	assessment := completion.Structured
	toolCalls := completion.ToolCalls
	if completion.StructuredFallback {
		// A native tool call without a structured wake-up assessment has no
		// trustworthy action type or cognitive stages. Preserve the wake-up as a
		// deterministic no-op instead of validating empty fields and retrying the
		// same activity forever. The tool call is intentionally discarded because
		// its target cannot be authorized without the structured action type.
		assessment = fallbackWakeUpAssessment()
		toolCalls = nil
	}
	if !completion.StructuredFallback && wakeUpAssessmentHasNoStages(assessment, toolCalls) {
		// Some thinking-enabled providers emit bookkeeping capability calls while
		// omitting the four internal stages entirely. Those calls cannot authorize
		// an action without a semantic assessment. Persist one explicit, bounded
		// no-op cycle so the durable timer survives instead of failing forever on
		// wake_up_attention_invalid.
		assessment = fallbackWakeUpAssessment()
		toolCalls = nil
	}
	if assessment == nil {
		return nil, errors.New("wake_up_assessment_invalid")
	}
	toolCalls = bindMediaContextToToolCalls(toolCalls, projection)
	assessment, err = normalizeWakeUpAssessment(assessment)
	if err != nil {
		return nil, err
	}
	composite, err := normalizeCompositeAction(assessment, toolCalls, wakeID, stringValue(assessment["action_type"]))
	if err != nil {
		return nil, err
	}
	toolCalls = composite.ToolCalls
	assessment["tool_calls"] = toolCalls
	assessment["output_bindings"] = composite.OutputBindings
	proposedActionType := stringValue(assessment["action_type"])
	actualActionType := proposedActionType
	deferredOutput := hasDeferredOutputToolCalls(toolCalls, a.capabilityRegistry())
	if proposedActionType == "moment" {
		if err := validateCompositeOutputCalls(toolCalls, "moment", a.capabilityRegistry()); err != nil {
			return nil, fmt.Errorf("wake_up_output_binding_invalid: %w", err)
		}
	} else if proposedActionType == "proactive_message" {
		if err := validateCompositeOutputCalls(toolCalls, "conversation_message", a.capabilityRegistry()); err != nil {
			return nil, fmt.Errorf("wake_up_output_binding_invalid: %w", err)
		}
	}
	mediaComposite := deferredOutput && (proposedActionType == "moment" || proposedActionType == "proactive_message")
	result := map[string]any{"status": "no_op"}
	policySnapshot := map[string]any{}
	policyReason := ""
	if len(toolCalls) > 0 && fluctlight.Status == "paused" {
		actualActionType = "no_op"
		result = map[string]any{"status": "blocked", "reason": "fluctlight_paused", "proposed_action_type": proposedActionType}
	} else if len(toolCalls) > 0 && proposedActionType == "no_op" {
		actualActionType = "capability"
		policySnapshot = map[string]any{"mode": "active", "authorization": "capability_manifest"}
		result = map[string]any{"status": "queued", "proposed_action_type": proposedActionType}
	}
	if proposedActionType != "no_op" {
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
			visible, realizationErr := a.Provider.Text(ctx, "action_realization", []map[string]any{
				{"role": "system", "content": "Realize the already-authorized wake-up action as one concise Chinese message. Preserve core_persona as a hard constraint, use developing_self only as soft context, and treat current_state as transient. For proactive_message address the Owner directly; for moment write one concise public Moment. Do not add semantic state or change the action type."},
				{"role": "user", "content": jsonString(map[string]any{"action_type": proposedActionType, "attention": assessment["attention"], "thought": assessment["thought"], "desire": assessment["desire"], "agency": assessment["agency"], "response_intent": assessment["response_intent"], "context": compactCognitionContext(projection)})},
			})
			if realizationErr != nil {
				return nil, realizationErr
			}
			if strings.TrimSpace(visible) == "" || len([]rune(visible)) > 32000 {
				return nil, errors.New("wake_up_realization_empty")
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
	var actionID string
	if actualActionType != "no_op" {
		actionID = "autonomy_wake_" + stableDigest(wakeID)
		result["action_id"] = actionID
	}
	reflectionIntentID := "reflection_intent:wake:" + wakeID
	factID, err := a.persistWakeUp(ctx, wakeID, fluctlightID, cycle, projection.InnerState, assessment, actualActionType, actionID, result, reflectionIntentID, policySnapshot, conversationID, toolCalls)
	if err != nil {
		return nil, err
	}
	toolResults := make([]ToolResultV1, 0)
	_ = factID
	safeResult := make(map[string]any, len(result))
	for key, value := range result {
		if key != "text" {
			safeResult[key] = value
		}
	}
	safeResult["tool_results"] = toolResults
	return map[string]any{"wake_up_id": wakeID, "fluctlight_id": fluctlightID, "cycle": cycle, "status": "completed", "attention": assessment["attention"], "thought": assessment["thought"], "desire": assessment["desire"], "agency": assessment["agency"], "action_type": actualActionType, "action_id": nullableString(actionID), "reflection_intent_id": reflectionIntentID, "result": safeResult, "interval_seconds": settings.IntervalSeconds}, nil
}

func wakeUpAssessmentHasNoStages(assessment map[string]any, toolCalls []ToolCallV1) bool {
	if len(toolCalls) == 0 || assessment == nil {
		return false
	}
	for _, field := range []string{"attention", "thought", "desire", "agency"} {
		switch value := assessment[field].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return false
			}
		case map[string]any:
			if len(value) > 0 {
				return false
			}
		default:
			// Missing, null, or a scalar with no supported semantic shape is
			// treated as an omitted stage only when every stage is omitted.
		}
	}
	return true
}

func fallbackWakeUpAssessment() map[string]any {
	return map[string]any{
		"attention":       "当前没有可处理的内部事件。",
		"thought":         "保持当前状态，等待下一次可靠输入。",
		"desire":          "无明确行动需求。",
		"agency":          "保持静默观察，不发起行动。",
		"appraisal":       map[string]any{"relevance": 0.0, "goal_congruence": 0.0, "reward": 0.0, "loss": 0.0, "social_threat": 0.0, "controllability": 0.0, "responsibility": 0.0, "relationship_significance": 0.0, "expected_effect": 0.0, "evidence_refs": []any{}, "event_kind": "provider_structured_fallback", "direction": "none"},
		"action_type":     "no_op",
		"response_intent": "",
		"evidence_refs":   []any{},
	}
}

func (a *App) persistWakeUp(ctx context.Context, wakeID, fluctlightID string, cycle int, internalDynamics map[string]any, assessment map[string]any, actionType, actionID string, result map[string]any, reflectionIntentID string, policySnapshot map[string]any, conversationID string, toolCalls []ToolCallV1) (string, error) {
	factID := "wake_fact_" + stableDigest(wakeID)
	workflowID := "autonomy_wake:" + wakeID
	appraisalPayload := mapValue(assessment["appraisal"])
	appraisalRefs := arrayValue(appraisalPayload["evidence_refs"])
	if !containsStringValue(appraisalRefs, factID) {
		appraisalRefs = append(appraisalRefs, factID)
	}
	appraisalPayload["evidence_refs"] = appraisalRefs
	payload := map[string]any{
		"event_type": "internal.wake_up", "wake_up_id": wakeID, "fluctlight_id": fluctlightID,
		"cycle": cycle, "appraisal": appraisalPayload, "attention": assessment["attention"], "thought": assessment["thought"],
		"desire": assessment["desire"], "agency": assessment["agency"], "action_type": actionType,
		"response_intent": assessment["response_intent"], "evidence_refs": assessment["evidence_refs"],
	}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
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
		updatedDynamics, err := a.persistCognitiveStagesTx(ctx, tx, fluctlightID, factID, assessment, actionType, actionID)
		if err != nil {
			return err
		}
		internalDynamics = updatedDynamics
		wakeResult := map[string]any{"status": result["status"], "action_id": nullableString(actionID), "conversation_id": nullableString(conversationID)}
		for key, value := range result {
			if key != "text" {
				wakeResult[key] = value
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_wakeups(id,fluctlight_id,cycle,internal_dynamics,attention,thought,desire,agency,action_type,action_id,result,reflection_intent_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT DO NOTHING`, wakeID, fluctlightID, cycle, jsonBytes(internalDynamics), jsonBytes(assessment["attention"]), jsonBytes(assessment["thought"]), jsonBytes(assessment["desire"]), jsonBytes(assessment["agency"]), actionType, nullableString(actionID), jsonBytes(wakeResult), reflectionIntentID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, reflectionIntentID, "reflection:wake:"+wakeID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "wake_up_id": wakeID})); err != nil {
			return err
		}
		if actionID != "" && actionType == "capability" {
			if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_actions(id,fluctlight_id,action_type,payload,policy_snapshot,expected_revisions,status,workflow_id,provider_request_id) VALUES($1,$2,$3,$4,$5,$6,'frozen',$7,$8) ON CONFLICT DO NOTHING`, actionID, fluctlightID, actionType, jsonBytes(map[string]any{"wake_up_id": wakeID, "source_fact_id": factID, "conversation_id": conversationID, "tool_calls": toolCalls}), jsonBytes(policySnapshot), jsonBytes(map[string]any{"context_revision": internalDynamics["revision"]}), workflowID, "provider_wakeup_"+stableDigest(wakeID)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','capability.action',$3) ON CONFLICT DO NOTHING`, "capability_wake_intent:"+wakeID, workflowID, jsonBytes(map[string]any{"action_id": actionID, "fluctlight_id": fluctlightID, "wake_up_id": wakeID, "source_fact_id": factID})); err != nil {
				return err
			}
		} else if actionID != "" {
			visible := stringValue(result["text"])
			// Keep the action strict: a frozen action can never be queued without a
			// visible payload from the realization stage.
			if visible == "" {
				return errors.New("wake_up_action_payload_empty")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_actions(id,fluctlight_id,action_type,payload,policy_snapshot,expected_revisions,status,workflow_id,provider_request_id) VALUES($1,$2,$3,$4,$5,$6,'frozen',$7,$8) ON CONFLICT DO NOTHING`, actionID, fluctlightID, actionType, jsonBytes(map[string]any{"wake_up_id": wakeID, "text": visible, "conversation_id": conversationID, "response_intent": assessment["response_intent"], "tool_calls": toolCalls, "output_bindings": assessment["output_bindings"]}), jsonBytes(policySnapshot), jsonBytes(map[string]any{"context_revision": internalDynamics["revision"]}), workflowID, "provider_wakeup_"+stableDigest(wakeID)); err != nil {
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

func (a *App) persistWakeToolResults(ctx context.Context, wakeID string, results []ToolResultV1) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_wakeups SET result=result || $2::jsonb WHERE id=$1`, wakeID, jsonBytes(map[string]any{"tool_results": results})); err != nil {
			return err
		}
		var fluctlightID string
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id FROM public.cognition_wakeups WHERE id=$1`, wakeID).Scan(&fluctlightID); err != nil {
			return err
		}
		factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "capability.requested", map[string]any{"wake_up_id": wakeID, "tool_results": results}, "capability-result:"+wakeID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:capability:"+wakeID, "reflection:capability:"+wakeID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "wake_up_id": wakeID}))
		return err
	})
}
