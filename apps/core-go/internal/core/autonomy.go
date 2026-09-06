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
	projection, err := a.BuildContextProjection(ctx, ownerID, fluctlightID, conversationID, "daily-review:"+fluctlightID+":"+localDate, "")
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
	messages := withContextAuthorityInstruction([]map[string]any{
		{"role": "system", "content": dailyReviewInstruction},
		{"role": "user", "content": jsonString(map[string]any{"local_date": localDate, "context": compactCognitionContext(projection)})},
	})
	messages = withActorRelationshipSystemContext(messages, projection)
	completion, err := a.Provider.StructuredWithToolsSchema(WithProviderScenario(ctx, "daily_review"), "cognitive_assessment", messages, a.capabilityRegistry().Manifests(), "daily_review_response", dailyReviewResponseSchema(), true)
	if err != nil {
		// Some mlx-serve responses contain a malformed bookkeeping tool call
		// (for example a scene_event without an id) before they emit any daily
		// review decision. Retry the same request through the no-tools contract;
		// this keeps the retry semantic-free and lets the model produce the
		// required action_type instead of failing the whole lifecycle workflow.
		if retry, retryErr := a.Provider.StructuredWithSchema(ctx, "cognitive_assessment", messages, "daily_review_response", dailyReviewResponseSchema(), false); retryErr == nil {
			completion = ProviderCompletion{Structured: retry}
		} else {
			return nil, err
		}
	}
	if completion.StructuredFallback {
		// Thinking-enabled mlx-serve responses may emit only bookkeeping tool
		// calls (for example scene_event/memory_event) and omit the structured
		// daily-review action. Retry the same evidence window without tools and
		// thinking so the model must choose an explicit action_type. If that
		// transport retry also fails, retain the safe no-op behavior below rather
		// than inferring an external side effect from an unrelated tool call.
		if retry, retryErr := a.Provider.StructuredWithSchema(ctx, "cognitive_assessment", messages, "daily_review_response", dailyReviewResponseSchema(), false); retryErr == nil {
			completion = ProviderCompletion{Structured: retry}
		}
	}
	decision := completion.Structured
	if decision == nil {
		decision = map[string]any{}
	}
	if preference := mapValue(decision["output_preference_decision"]); len(preference) > 0 {
		if normalized, normalizeErr := normalizeOutputPreferenceDecision(preference, stringValue(projection.PersonalityRuntime["active_profile_id"])); normalizeErr == nil {
			decision["output_preference_decision"] = normalized
		}
	}
	toolCalls := completion.ToolCalls
	if completion.StructuredFallback && len(toolCalls) > 0 {
		// A DailyReview native tool call without its action_type cannot be bound
		// safely to a Moment or Owner conversation. Preserve the review as a
		// no-op rather than guessing the output target.
		toolCalls = nil
	}
	toolCalls = bindMediaContextToToolCalls(toolCalls, projection)
	composite, err := normalizeCompositeAction(decision, toolCalls, workflowID, "no_op")
	if err != nil {
		return nil, err
	}
	if composite.ActionType == "no_op" && len(composite.ToolCalls) > 0 {
		return nil, errors.New("daily_review_tool_target_invalid")
	}
	actionType := composite.ActionType
	if actionType != "proactive_message" && actionType != "moment" && actionType != "no_op" {
		return nil, errors.New("daily_review_decision_invalid")
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
	}
	if actionType == "proactive_message" {
		if conversationID == "" {
			return nil, errors.New("proactive_target_invalid")
		}
		if err := validateCompositeOutputCalls(composite.ToolCalls, "conversation_message", a.capabilityRegistry()); err != nil {
			return nil, fmt.Errorf("daily_review_output_binding_invalid: %w", err)
		}
	} else if actionType == "moment" {
		if err := validateCompositeOutputCalls(composite.ToolCalls, "moment", a.capabilityRegistry()); err != nil {
			return nil, fmt.Errorf("daily_review_output_binding_invalid: %w", err)
		}
	}
	payload := map[string]any{"conversation_id": conversationID, "response_intent": composite.ResponseIntent, "decision": composite, "source_fact_id": "daily-review:" + fluctlightID + ":" + localDate}
	if preference := mapValue(decision["output_preference_decision"]); len(preference) > 0 {
		payload["output_preference_decision"] = preference
	}
	if len(composite.ToolCalls) > 0 {
		payload["tool_calls"] = composite.ToolCalls
		payload["output_bindings"] = composite.OutputBindings
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if actionType != "no_op" {
			if err := reserveAutonomyBudgetTx(ctx, tx, fluctlightID); err != nil {
				return err
			}
		}
		status := "frozen"
		if actionType == "no_op" {
			status = "completed"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_actions (id,fluctlight_id,action_type,payload,policy_snapshot,expected_revisions,status,workflow_id,provider_request_id,created_at,settled_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now(),CASE WHEN $7='completed' THEN now() ELSE NULL END)`, actionID, fluctlightID, actionType, jsonBytes(payload), jsonBytes(policySnapshot), jsonBytes(map[string]any{"context_revision": projection.ContextRevision}), status, workflowID, providerID); err != nil {
			return err
		}
		if status == "frozen" {
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','autonomy.action',$3) ON CONFLICT DO NOTHING`, "autonomy_daily_intent:"+actionID, workflowID, jsonBytes(map[string]any{"action_id": actionID, "fluctlight_id": fluctlightID, "local_date": localDate})); err != nil {
				return err
			}
			return appendOutboxTx(ctx, tx, "autonomy.action.frozen", "autonomy_action", actionID, fluctlightID, actionID, workflowID, "autonomy-freeze:"+actionID, map[string]any{"action_type": actionType, "workflow_id": workflowID})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	deliveryStatus := ""
	deliveredMessageID := ""
	if actionType != "no_op" {
		realizationMessages := []map[string]any{{"role": "system", "content": actionRealizationInstruction}, {"role": "user", "content": jsonString(map[string]any{"action_type": actionType, "response_intent": composite.ResponseIntent, "context": compactCognitionContext(projection)})}}
		realizationMessages = withActorRelationshipSystemContext(realizationMessages, projection)
		visible, realizationErr := a.Provider.Text(WithProviderScenario(ctx, "autonomy_reply"), "action_realization", realizationMessages)
		if realizationErr != nil {
			_, _ = a.failAutonomyAction(ctx, actionID, "realization_failed")
			return nil, realizationErr
		}
		visible = normalizeVisibleReply(visible)
		if visible == "" || len([]rune(visible)) > 32000 {
			_, _ = a.failAutonomyAction(ctx, actionID, "realization_empty")
			return nil, errors.New("daily_review_realization_empty")
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
	result := map[string]any{"action_id": actionID, "action_type": actionType, "local_date": localDate, "timezone": location.String(), "status": "completed", "owner_actor_id": ownerID}
	if deliveryStatus != "" {
		result["delivery_status"] = deliveryStatus
		result["message_id"] = deliveredMessageID
	}
	return result, nil
}

func (a *App) agencyProfile(ctx context.Context, fluctlightID string) ([]map[string]any, []map[string]any, error) {
	goals := make([]map[string]any, 0)
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,profile_id,scope,target_actor_id,description,status,importance,urgency,progress FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status <> 'forgotten' ORDER BY created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id, scope, description, status string
		var profileID *string
		var targetActorID *string
		var importance, urgency, progress []byte
		if err := rows.Scan(&id, &profileID, &scope, &targetActorID, &description, &status, &importance, &urgency, &progress); err != nil {
			rows.Close()
			return nil, nil, err
		}
		item := map[string]any{"id": id, "scope": scope, "description": description, "status": status, "importance": jsonNumber(importance), "urgency": jsonNumber(urgency), "progress": jsonNumber(progress)}
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
	intentRows, err := a.DB.Pool().Query(ctx, `SELECT i.id,i.profile_id,i.goal_id,COALESCE(g.description,''),i.action,i.status,i.confidence,i.preferred_time,i.expiration,i.trigger FROM public.fluctlight_intentions i LEFT JOIN public.fluctlight_goals g ON g.id=i.goal_id AND g.fluctlight_id=i.fluctlight_id WHERE i.fluctlight_id=$1 AND i.status NOT IN ('cancelled','completed','expired') AND i.expiration > now() ORDER BY i.created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	for intentRows.Next() {
		var id, goalDescription, action, status string
		var profileID, goalID *string
		var trigger []byte
		var confidence float64
		var preferredTime, expiration *time.Time
		if err := intentRows.Scan(&id, &profileID, &goalID, &goalDescription, &action, &status, &confidence, &preferredTime, &expiration, &trigger); err != nil {
			intentRows.Close()
			return nil, nil, err
		}
		item := map[string]any{"id": id, "goal": goalDescription, "action": action, "status": status, "confidence": confidence}
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
