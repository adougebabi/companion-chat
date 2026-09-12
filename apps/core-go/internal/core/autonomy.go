package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) ProcessDailyReview(ctx context.Context, fluctlightID, localDate string) (map[string]any, error) {
	fluctlight, err := a.DB.GetFluctlight(ctx, fluctlightID, "")
	if err != nil {
		// The internal job is authorized by the workflow intent; re-read without
		// owner filtering for worker execution.
		fluctlight, err = a.readFluctlightByID(ctx, fluctlightID)
		if err != nil {
			return nil, err
		}
	}
	if fluctlight.Status != "active" {
		return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "timezone": stringValue(fluctlight.Identity["timezone"]), "status": "inactive"}, nil
	}
	timezone := stringValue(fluctlight.Identity["timezone"])
	if timezone == "" {
		// Schedule lifecycle uses the same explicit deployment default when a
		// provider omits an optional identity timezone. Without this fallback a
		// pending activation review resolved in UTC while EnsureCurrentDaySchedule
		// created the plan in Asia/Shanghai, so the review could never see its
		// accepted schedule.
		timezone = "Asia/Shanghai"
	}
	location, zoneErr := time.LoadLocation(canonicalTimezone(timezone))
	if zoneErr != nil {
		return nil, zoneErr
	}
	if localDate == "" {
		localDate = time.Now().In(location).Format("2006-01-02")
	}
	releaseReviewLock, acquired, lockErr := a.tryDailyReviewExecutionLock(ctx, fluctlightID, localDate)
	if lockErr != nil {
		return nil, lockErr
	}
	if !acquired {
		return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "timezone": location.String(), "status": "in_progress"}, nil
	}
	defer releaseReviewLock()
	var scheduleReady bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_schedules WHERE fluctlight_id=$1 AND local_date=$2 AND status='accepted')`, fluctlightID, localDate).Scan(&scheduleReady); err != nil {
		return nil, err
	}
	if !scheduleReady {
		return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "timezone": location.String(), "status": "pending", "error_code": "schedule_pending"}, nil
	}
	ownerID, conversationID, err := a.directTarget(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	projection, err := a.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		ConversationID: conversationID, SourceFactID: "daily-review:" + fluctlightID + ":" + localDate,
		MemoryOperation: MemoryForDailyReview, MemoryConversationMode: MemoryConversationExact,
	})
	if err != nil {
		return nil, err
	}
	workflowID := fmt.Sprintf("go-autonomy:%s:%s", fluctlightID, localDate)
	actionID := "autonomy_" + stableDigest(workflowID)
	providerID := "provider_" + stableDigest(actionID)
	var existingStatus, existingType string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,action_type FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&existingStatus, &existingType); err == nil {
		return map[string]any{"action_id": actionID, "action_type": existingType, "local_date": localDate, "timezone": location.String(), "status": existingStatus}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceAutonomy)
	schema := dailyReviewResponseSchema()
	assembly, assembledProjection, err := a.assembleProjectionPrompt(ctx, projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityDailyReviewPolicyInstruction}, jsonString(map[string]any{"local_date": localDate}), definitions, "daily_review_response", schema)
	if err != nil {
		return nil, err
	}
	projection = assembledProjection
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "daily_review"), assembly.Diagnostics)
	completion, err := a.Provider.StructuredAssembledWithToolsSchema(providerCtx, "cognitive_assessment", assembly.Messages, definitions, "daily_review_response", schema, true)
	if err != nil {
		// A daily review is one semantic cognition. Invalid Provider output is
		// retried by its owning workflow with the same durable identity; this call
		// never opens a second Main LLM request with a different tool contract.
		return nil, err
	}
	// A structured fallback with native calls is still a valid tool-only
	// assessment. Do not issue a second no-tools model request here: that would
	// discard schedule/scene changes and turn an optional capability decision
	// into an unrelated retry. An empty fallback with no calls naturally settles
	// as a safe no-op below.
	decision := completion.Structured
	if decision == nil {
		decision = map[string]any{}
	}
	influences, err := freezeDecisionInfluences(decision, projection, false)
	if err != nil {
		return nil, err
	}
	if stringValue(decision["action_type"]) != "no_op" || len(completion.ToolCalls) > 0 {
		if err := requireDecisionInfluences(influences, "daily_review_influences_required"); err != nil {
			return nil, err
		}
	}
	if preference := mapValue(decision["output_preference_decision"]); len(preference) > 0 {
		if normalized, normalizeErr := normalizeOutputPreferenceDecision(preference, stringValue(projection.PersonalityRuntime["active_profile_id"])); normalizeErr == nil {
			decision["output_preference_decision"] = normalized
		}
	}
	toolCalls := append([]CapabilityInvocation(nil), completion.ToolCalls...)
	toolCalls, err = a.bindCapabilityInvocationsToProjection(toolCalls, projection, actionID, "daily-review:"+fluctlightID+":"+localDate, CapabilitySurfaceAutonomy)
	if err != nil {
		return nil, err
	}
	toolCalls, err = a.prepareCapabilityInvocations(ctx, fluctlightID, conversationID, "daily-review:"+fluctlightID+":"+localDate, toolCalls)
	if err != nil {
		return nil, err
	}
	// A thinking-enabled Provider may express a schedule/scene/native update
	// entirely through optional capability calls and omit the JSON sidecar. Keep
	// those calls: `no_op + capability_invocations` is a valid capability-only review and
	// must not be converted into a silent no-op or a no-tools retry.
	composite, err := normalizeCompositeAction(decision, toolCalls, workflowID, "no_op")
	if err != nil {
		return nil, err
	}
	actionType := composite.ActionType
	if actionType == "no_op" && len(composite.ToolCalls) > 0 {
		actionType = "capability"
	}
	if actionType != "proactive_message" && actionType != "moment" && actionType != "capability" && actionType != "no_op" {
		return nil, errors.New("daily_review_decision_invalid")
	}
	if preference := mapValue(decision["output_preference_decision"]); len(preference) > 0 {
		decision["output_preference_decision"] = evaluateOutputPreferenceAction(preference, actionType, composite.ToolCalls, a.capabilityRegistry())
	}
	policySnapshot := map[string]any{}
	if actionType != "no_op" {
		policyDecision, policyErr := a.EvaluateAutonomyPolicy(ctx, fluctlightID, actionType, time.Now().UTC())
		if policyErr != nil {
			return nil, policyErr
		}
		policySnapshot = policyDecision.Snapshot
		if !policyDecision.Allowed {
			return map[string]any{"action_id": actionID, "action_type": actionType, "local_date": localDate, "timezone": location.String(), "status": "blocked", "reason": policyDecision.Reason, "policy_snapshot": policySnapshot}, nil
		}
		policySnapshot["budget_reserved"] = true
	}
	if actionType == "proactive_message" {
		if conversationID == "" {
			return nil, errors.New("proactive_target_invalid")
		}
		if err := validateCompositeOutputCapabilities(composite.ToolCalls, "conversation_message", a.capabilityRegistry()); err != nil {
			return nil, fmt.Errorf("daily_review_output_binding_invalid: %w", err)
		}
	} else if actionType == "moment" {
		if err := validateCompositeOutputCapabilities(composite.ToolCalls, "moment", a.capabilityRegistry()); err != nil {
			return nil, fmt.Errorf("daily_review_output_binding_invalid: %w", err)
		}
	} else if actionType == "capability" {
		// Capability-only reviews have no visible output target. Each registered
		// capability remains optional; the durable capability.action intent records
		// individual results without requiring conversation.reply or moment.publish.
		if len(composite.ToolCalls) == 0 {
			return nil, errors.New("daily_review_capability_calls_empty")
		}
	}
	payload := map[string]any{"capability_runtime_version": CapabilityRuntimePayloadVersion, "conversation_id": conversationID, "response_intent": composite.ResponseIntent, "decision": composite, "source_fact_id": "daily-review:" + fluctlightID + ":" + localDate, "capability_results": []CapabilityResult{}}
	payload["context_reference_version"] = contextReferenceIndexVersion
	payload["context_reference_index"] = projection.ReferenceIndex
	payload["influences"] = decisionInfluenceMaps(influences)
	payload["goal_refs"] = decision["goal_refs"]
	payload["intention_refs"] = decision["intention_refs"]
	if preference := mapValue(decision["output_preference_decision"]); len(preference) > 0 {
		payload["output_preference_decision"] = preference
	}
	if len(composite.ToolCalls) > 0 {
		payload["capability_invocations"] = composite.ToolCalls
		payload["output_bindings"] = composite.OutputBindings
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, projection.ContextRevision, projection.CurrentStateRevision, projection.LifeContextRevision, time.Now().UTC()); err != nil {
			return err
		}
		if actionType != "no_op" {
			if err := reserveAutonomyBudgetTx(ctx, tx, fluctlightID); err != nil {
				return err
			}
		}
		status := "frozen"
		if actionType == "no_op" {
			status = "completed"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_actions (id,fluctlight_id,action_type,payload,policy_snapshot,expected_revisions,status,workflow_id,provider_request_id,created_at,settled_at) VALUES ($1,$2,$3,$4,$5,$6,$7::varchar,$8,$9,now(),CASE WHEN $7::varchar='completed' THEN now() ELSE NULL END)`, actionID, fluctlightID, actionType, jsonBytes(payload), jsonBytes(policySnapshot), jsonBytes(map[string]any{"foundation_revision": projection.ContextRevision, "current_state_revision": projection.CurrentStateRevision, "life_context_revision": projection.LifeContextRevision}), status, workflowID, providerID); err != nil {
			return err
		}
		if status == "frozen" {
			intentType := "autonomy.action"
			intentIDPrefix := "autonomy_daily_intent:"
			if actionType == "capability" {
				intentType = "capability.action"
				intentIDPrefix = "capability_daily_intent:"
			}
			_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction',$3,$4) ON CONFLICT DO NOTHING`, intentIDPrefix+actionID, workflowID, intentType, jsonBytes(map[string]any{"action_id": actionID, "fluctlight_id": fluctlightID, "local_date": localDate}))
			if err != nil {
				return err
			}
			return appendOutboxTx(ctx, tx, "autonomy.action.frozen", "autonomy_action", actionID, fluctlightID, actionID, workflowID, "autonomy-freeze:"+actionID, map[string]any{"action_type": actionType, "workflow_id": workflowID, "intent_type": intentType})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	deliveryStatus := ""
	deliveredMessageID := ""
	if actionType != "no_op" && actionType != "capability" {
		targetKind := "moment"
		if actionType == "proactive_message" {
			targetKind = "conversation_message"
		}
		visible := textFromOutputBinding(composite.ToolCalls, targetKind, a.capabilityRegistry())
		if visible == "" {
			_, _ = a.failAutonomyAction(ctx, actionID, "output_capability_text_missing")
			return nil, fmt.Errorf("daily_review_%s_required", targetKind)
		}
		if _, err := a.DB.Pool().Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(payload,'{text}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(visible)); err != nil {
			return nil, err
		}
		execution, executionErr := a.ProcessAutonomyAction(ctx, actionID)
		if executionErr != nil {
			return nil, executionErr
		}
		deliveryStatus = stringValue(execution["delivery_status"])
		deliveredMessageID = stringValue(execution["message_id"])
	}
	if actionType == "no_op" {
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			payload := map[string]any{"action_id": actionID, "action_type": actionType, "status": "completed", "local_date": localDate, "delivery_status": deliveryStatus, "message_id": deliveredMessageID}
			causality, causalityErr := frozenDecisionCausality(decision)
			if causalityErr != nil {
				return causalityErr
			}
			for key, value := range causality {
				payload[key] = value
			}
			factID, factErr := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", payload, "daily-review-result:"+actionID)
			if factErr != nil {
				return factErr
			}
			if _, factErr := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:daily:"+actionID, "reflection:daily:"+actionID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": actionID})); factErr != nil {
				return factErr
			}
			return appendOutboxTx(ctx, tx, "autonomy.result.recorded", "fluctlight", fluctlightID, fluctlightID, actionID, "daily-review-result:"+actionID, "daily-review-result:"+actionID, payload)
		}); err != nil {
			return nil, err
		}
	}
	resultStatus := "completed"
	if actionType == "capability" {
		// Capability calls are executed by the durable capability.action intent.
		// Returning queued here avoids running them twice and lets their single
		// settlement create the authoritative autonomy.result/reflection facts.
		resultStatus = "queued"
	}
	result := map[string]any{"action_id": actionID, "action_type": actionType, "local_date": localDate, "timezone": location.String(), "status": resultStatus, "owner_actor_id": ownerID}
	if deliveryStatus != "" {
		result["delivery_status"] = deliveryStatus
		result["message_id"] = deliveredMessageID
	}
	return result, nil
}

func (a *App) tryDailyReviewExecutionLock(ctx context.Context, fluctlightID, localDate string) (func(), bool, error) {
	connection, err := a.DB.Pool().Acquire(ctx)
	if err != nil {
		return func() {}, false, err
	}
	key := "daily-review:" + strings.TrimSpace(fluctlightID) + ":" + strings.TrimSpace(localDate)
	var acquired bool
	if err := connection.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, key).Scan(&acquired); err != nil {
		connection.Release()
		return func() {}, false, err
	}
	if !acquired {
		connection.Release()
		return func() {}, false, nil
	}
	release := func() {
		var unlocked bool
		_ = connection.QueryRow(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, key).Scan(&unlocked)
		connection.Release()
	}
	return release, true, nil
}

func (a *App) agencyProfile(ctx context.Context, fluctlightID string) ([]map[string]any, []map[string]any, error) {
	goals := make([]map[string]any, 0)
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,profile_id,scope,target_actor_id,description,desired_outcome,success_criteria,motivation,needs_reflection,status,importance,urgency,progress,deadline,evidence_refs,revision FROM public.fluctlight_goals WHERE fluctlight_id=$1 ORDER BY created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id, scope, description, desiredOutcome, motivation, status string
		var profileID *string
		var targetActorID *string
		var successCriteria, importance, urgency, progress []byte
		var needsReflection bool
		var deadline *time.Time
		var evidenceRefs []byte
		var revision int
		if err := rows.Scan(&id, &profileID, &scope, &targetActorID, &description, &desiredOutcome, &successCriteria, &motivation, &needsReflection, &status, &importance, &urgency, &progress, &deadline, &evidenceRefs, &revision); err != nil {
			rows.Close()
			return nil, nil, err
		}
		item := map[string]any{"id": id, "scope": scope, "description": description, "desired_outcome": desiredOutcome, "success_criteria": decodeArray(successCriteria), "motivation": motivation, "needs_reflection": needsReflection, "status": status, "importance": jsonNumber(importance), "urgency": jsonNumber(urgency), "progress": jsonNumber(progress), "evidence_refs": decodeArray(evidenceRefs), "revision": revision}
		if deadline != nil {
			item["deadline"] = deadline.Format(time.RFC3339Nano)
		}
		if profileID != nil && strings.TrimSpace(*profileID) != "" {
			item["profile_id"] = *profileID
		}
		if targetActorID != nil {
			item["target_actor_id"] = *targetActorID
		}
		goals = append(goals, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()
	intentions := make([]map[string]any, 0)
	intentRows, err := a.DB.Pool().Query(ctx, `SELECT i.id,i.profile_id,i.goal_id,COALESCE(g.desired_outcome,''),i.action_intent,i.expected_outcome,i.capability_constraints,i.status,i.confidence,i.preferred_time,i.expiration,i.trigger,i.evidence_refs,i.revision,COALESCE(i.current_attempt_id,'') FROM public.fluctlight_intentions i LEFT JOIN public.fluctlight_goals g ON g.id=i.goal_id AND g.fluctlight_id=i.fluctlight_id WHERE i.fluctlight_id=$1 AND i.status NOT IN ('cancelled','completed','expired') AND i.expiration > now() ORDER BY i.created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	for intentRows.Next() {
		var id, goalDescription, action, expectedOutcome, status, currentAttemptID string
		var profileID, goalID *string
		var capabilityConstraints, trigger, evidenceRefs []byte
		var confidence float64
		var revision int
		var preferredTime, expiration *time.Time
		if err := intentRows.Scan(&id, &profileID, &goalID, &goalDescription, &action, &expectedOutcome, &capabilityConstraints, &status, &confidence, &preferredTime, &expiration, &trigger, &evidenceRefs, &revision, &currentAttemptID); err != nil {
			intentRows.Close()
			return nil, nil, err
		}
		item := map[string]any{"id": id, "goal": goalDescription, "action": action, "action_intent": action, "expected_outcome": expectedOutcome, "capability_constraints": decodeArray(capabilityConstraints), "status": status, "confidence": confidence, "trigger": decodeObject(trigger), "evidence_refs": decodeArray(evidenceRefs), "revision": revision}
		if currentAttemptID != "" {
			item["current_attempt_id"] = currentAttemptID
		}
		if profileID != nil && strings.TrimSpace(*profileID) != "" {
			item["profile_id"] = *profileID
		}
		if goalID != nil && strings.TrimSpace(*goalID) != "" {
			item["goal_id"] = *goalID
		}
		triggerValue := decodeObject(trigger)
		if target := strings.TrimSpace(stringValue(triggerValue["target_actor_id"])); target != "" {
			item["target_actor_id"] = target
		}
		if preferredTime != nil {
			item["preferred_time"] = preferredTime.Format(time.RFC3339)
		}
		if expiration != nil {
			item["expiration"] = expiration.Format(time.RFC3339)
		}
		intentions = append(intentions, item)
	}
	if err := intentRows.Err(); err != nil {
		intentRows.Close()
		return nil, nil, err
	}
	intentRows.Close()
	return goals, intentions, nil
}

func (a *App) directTarget(ctx context.Context, fluctlightID string) (string, string, error) {
	var owner, conversation string
	err := a.DB.Pool().QueryRow(ctx, `SELECT owner_actor_id,conversation_id FROM public.fluctlight_direct_conversations WHERE fluctlight_actor_id=$1`, fluctlightID).Scan(&owner, &conversation)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return owner, conversation, err
}
func (a *App) readFluctlightByID(ctx context.Context, id string) (Fluctlight, error) {
	var f Fluctlight
	var i, p, b, l, pr []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,identity,personality,behavioral_policy,life_profile,provenance,status,current_revision FROM public.fluctlights WHERE id=$1`, id).Scan(&f.ID, &i, &p, &b, &l, &pr, &f.Status, &f.CurrentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, ErrNotFound
	}
	if err != nil {
		return f, err
	}
	f.Identity = decodeObject(i)
	f.Personality = decodeObject(p)
	f.BehavioralPolicy = decodeObject(b)
	f.LifeProfile = decodeObject(l)
	f.Provenance = decodeObject(pr)
	return f, nil
}
func appendAssistantTx(ctx context.Context, tx pgx.Tx, conversationID, actorID, text, idempotency string) error {
	_, err := appendAssistantTxWithID(ctx, tx, conversationID, actorID, text, idempotency)
	return err
}

func appendAssistantTxWithID(ctx context.Context, tx pgx.Tx, conversationID, actorID, text, idempotency string) (string, error) {
	var existing string
	if err := tx.QueryRow(ctx, `SELECT id FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversationID, idempotency).Scan(&existing); err == nil {
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	var seq int
	if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, conversationID).Scan(&seq); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=$2 WHERE conversation_id=$1`, conversationID, seq+1); err != nil {
		return "", err
	}
	messageID := randomID("message_")
	if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_messages (id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES ($1,$2,$3,$4,'assistant',$5,'[]',$6)`, messageID, conversationID, seq, actorID, text, idempotency); err != nil {
		return "", err
	}
	return messageID, nil
}
func stableDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:32]
}
