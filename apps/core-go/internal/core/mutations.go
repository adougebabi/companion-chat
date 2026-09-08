package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type TurnResult struct {
	UserMessage   map[string]any
	Assistant     map[string]any
	MediaIntentID string
	TurnID        string
	CorrelationID string
}

type turnCallbacks struct {
	onActionResult func(map[string]any) error
	onChunk        func(string) error
}

var errCognitionTurnSuperseded = errors.New("cognition_turn_superseded")

func (a *App) AcceptSchedule(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	localDate, err := time.Parse("2006-01-02", stringValue(payload["local_date"]))
	if err != nil {
		return nil, errors.New("schedule_accept_failed")
	}
	timezone := canonicalTimezone(stringValue(payload["timezone"]))
	if timezone == "" {
		return nil, errors.New("schedule_accept_failed")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, errors.New("schedule_timezone_invalid")
	}
	items := arrayValue(payload["items"])
	if len(items) == 0 {
		return nil, errors.New("schedule_accept_failed")
	}
	var expected *int
	if raw, ok := payload["expected_revision"]; ok && raw != nil {
		value := intValue(raw)
		expected = &value
	}
	scheduleID := randomID("schedule_")
	now := time.Now().UTC()
	revision := 1
	location, _ := time.LoadLocation(timezone)
	type scheduleEntry struct {
		item       map[string]any
		start, end time.Time
	}
	entries := make([]scheduleEntry, 0, len(items))
	for _, raw := range items {
		item := mapValue(raw)
		if len(item) == 0 {
			return nil, errors.New("schedule item invalid")
		}
		start, e1 := parseScheduleTime(stringValue(item["start_at"]))
		end, e2 := parseScheduleTime(stringValue(item["end_at"]))
		if e1 != nil || e2 != nil || !end.After(start) {
			return nil, errors.New("schedule item time is invalid")
		}
		if strings.TrimSpace(stringValue(item["activity"])) == "" || strings.TrimSpace(stringValue(item["scene"])) == "" {
			return nil, errors.New("schedule item activity and scene are required")
		}
		for _, field := range []string{"priority", "flexibility", "interruption_cost"} {
			if rawValue, ok := item[field]; ok && rawValue != nil {
				normalizedValue := normalizeScheduleScalar(rawValue)
				item[field] = normalizedValue
				value, ok := numberFloat(normalizedValue)
				if !ok || value < 0 || value > 1 {
					return nil, errors.New("schedule item numeric value invalid")
				}
			}
		}
		entries = append(entries, scheduleEntry{item: item, start: start, end: end})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].start.Before(entries[j].start) })
	dayStart := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 0, 0, 0, 0, location)
	nextDay := dayStart.AddDate(0, 0, 1)
	if !entries[0].start.In(location).Equal(dayStart) || !entries[len(entries)-1].end.In(location).Equal(nextDay) {
		return nil, errors.New("schedule must cover complete local day")
	}
	for index := range entries {
		if index > 0 && !entries[index].start.Equal(entries[index-1].end) {
			return nil, errors.New("schedule items must be contiguous")
		}
		if entries[index].start.In(location).Before(dayStart) || entries[index].end.In(location).After(nextDay) {
			return nil, errors.New("schedule item outside local day")
		}
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fluctlightID+":"+localDate.Format("2006-01-02")); err != nil {
			return err
		}
		var current int
		err := tx.QueryRow(ctx, `SELECT revision FROM public.life_schedules WHERE fluctlight_id=$1 AND local_date=$2 AND status='accepted' ORDER BY revision DESC LIMIT 1`, fluctlightID, localDate).Scan(&current)
		found := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if errors.Is(err, pgx.ErrNoRows) {
			current = 0
		}
		if found && expected == nil {
			return errors.New("schedule acceptance requires expected revision")
		}
		if expected != nil && *expected != current {
			return ErrConflict
		}
		revision = current + 1
		if _, err := tx.Exec(ctx, `UPDATE public.life_schedules SET status='superseded' WHERE fluctlight_id=$1 AND local_date=$2 AND status='accepted'`, fluctlightID, localDate); err != nil {
			return err
		}
		evidence := arrayValue(payload["evidence_refs"])
		if len(evidence) == 0 {
			evidence = []any{"owner:" + actorID}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.life_schedules (id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,generated_at,reschedule_policy) VALUES ($1,$2,$3,$4,'accepted','owner',$5,$6,$7,$8)`, scheduleID, fluctlightID, localDate, timezone, jsonBytes(evidence), revision, now, jsonBytes(payload["reschedule_policy"])); err != nil {
			return err
		}
		var previousID string
		_ = tx.QueryRow(ctx, `SELECT id FROM public.life_schedules WHERE fluctlight_id=$1 AND local_date=$2 AND revision=$3`, fluctlightID, localDate, current).Scan(&previousID)
		if previousID != "" {
			_, _ = tx.Exec(ctx, `UPDATE public.life_schedules SET previous_version_id=$2 WHERE id=$1`, scheduleID, previousID)
		}
		var previousEnd *time.Time
		for _, entry := range entries {
			item := entry.item
			start, end := entry.start, entry.end
			if previousEnd != nil && !start.Equal(*previousEnd) {
				return errors.New("schedule items must be contiguous")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.life_schedule_items (id,schedule_id,start_at,end_at,activity,scene,item_type,status,priority,flexibility,interruption_cost) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, randomID("schedule_item_"), scheduleID, start, end, strings.TrimSpace(stringValue(item["activity"])), strings.TrimSpace(stringValue(item["scene"])), firstString(item["item_type"], "planned"), firstString(item["status"], "planned"), numberString(item["priority"], 0.5), numberString(item["flexibility"], 0.5), numberString(item["interruption_cost"], 0.5)); err != nil {
				return err
			}
			previousEnd = &end
		}
		if previousEnd == nil {
			return errors.New("schedule item time is invalid")
		}
		if err := insertPostScheduleLifecycleIntentsTx(ctx, tx, fluctlightID, localDate, timezone); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "schedule.accepted", "fluctlight", fluctlightID, fluctlightID, scheduleID, "schedule:"+scheduleID, "schedule-outbox:"+scheduleID, map[string]any{"schedule_id": scheduleID, "local_date": localDate.Format("2006-01-02"), "revision": revision})
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": scheduleID, "local_date": localDate.Format("2006-01-02"), "timezone": timezone, "revision": revision, "status": "accepted", "reschedule_policy": payload["reschedule_policy"]}, nil
}

// ReplanSchedule shares the immutable acceptance/CAS path but refuses to
// rewrite a completed interval. Callers provide the completed boundary in
// RFC3339; completed items must be carried forward unchanged by the planner.
func (a *App) ReplanSchedule(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	boundaryValue := stringValue(payload["completed_before"])
	if boundaryValue == "" {
		return a.AcceptSchedule(ctx, actorID, fluctlightID, payload)
	}
	boundary, err := time.Parse(time.RFC3339, boundaryValue)
	if err != nil {
		return nil, errors.New("completed_before_invalid")
	}
	for _, raw := range arrayValue(payload["items"]) {
		item := mapValue(raw)
		start, startErr := parseScheduleTime(stringValue(item["start_at"]))
		end, endErr := parseScheduleTime(stringValue(item["end_at"]))
		if startErr != nil || endErr != nil {
			return nil, errors.New("schedule item time is invalid")
		}
		if start.Before(boundary) && end.After(boundary) {
			return nil, errors.New("schedule replan crosses completed boundary")
		}
	}
	return a.AcceptSchedule(ctx, actorID, fluctlightID, payload)
}

// insertPostScheduleLifecycleIntentsTx releases the cognition-producing
// lifecycle intents only after the current local day's Schedule is accepted.
// Keeping these inserts in the same transaction as schedule acceptance makes
// schedule-first ordering durable across Worker restarts and dispatcher
// retries; an accepted future schedule does not wake the Fluctlight early.
func insertPostScheduleLifecycleIntentsTx(ctx context.Context, tx pgx.Tx, fluctlightID string, localDate time.Time, timezone string) error {
	location, err := time.LoadLocation(canonicalTimezone(timezone))
	if err != nil {
		return fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	if localDate.Format("2006-01-02") != time.Now().In(location).Format("2006-01-02") {
		return nil
	}
	dateValue := localDate.Format("2006-01-02")
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'lifecycle','daily_review.current_day',$3) ON CONFLICT DO NOTHING`, "daily_review_intent:"+fluctlightID+":"+dateValue, "daily_review:"+fluctlightID+":"+dateValue, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "local_date": dateValue})); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'lifecycle','wake_up.current',$3) ON CONFLICT DO NOTHING`, "wake_up_intent:"+fluctlightID, "wake_up:"+fluctlightID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "cycle": 0}))
	return err
}

func parseScheduleTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "T24:") {
		value = strings.Replace(value, "T24:", "T00:", 1)
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, err
		}
		return parsed.Add(24 * time.Hour), nil
	}
	return time.Parse(time.RFC3339, value)
}

func canonicalTimezone(value string) string {
	normalized := strings.TrimSpace(value)
	switch strings.ToLower(normalized) {
	case "utc+8", "utc+08:00", "gmt+8", "gmt+08:00", "china standard time", "cst":
		return "Asia/Shanghai"
	case "utc", "gmt", "z":
		return "UTC"
	default:
		return normalized
	}
}

func (a *App) HandleTurn(ctx context.Context, actorID, conversationID string, payload map[string]any) (TurnResult, error) {
	return a.handleTurn(ctx, actorID, conversationID, payload, turnCallbacks{}, false)
}

// HandleActorTurn is the internal/group-chat entry point. Authorization stays
// anchored to the Owner Human while the persisted message and cognition
// speaker use the actual Actor (including another Fluctlight).
func (a *App) HandleActorTurn(ctx context.Context, ownerActorID, senderActorID, conversationID string, payload map[string]any) (TurnResult, error) {
	copyPayload := cloneMap(payload)
	copyPayload["authorization_actor_id"] = ownerActorID
	return a.handleTurn(ctx, senderActorID, conversationID, copyPayload, turnCallbacks{}, false)
}

func (a *App) handleTurn(ctx context.Context, actorID, conversationID string, payload map[string]any, callbacks turnCallbacks, claimStream bool) (TurnResult, error) {
	authorizationActorID := firstString(payload["authorization_actor_id"], actorID)
	fluctlightID := stringValue(payload["fluctlight_id"])
	text := stringValue(payload["text"])
	idempotency := stringValue(payload["idempotency_key"])
	turnID := stringValue(payload["turn_id"])
	if turnID == "" {
		turnID = "turn_" + stableDigest(conversationID+":"+idempotency)
	}
	if fluctlightID == "" || text == "" || idempotency == "" {
		return TurnResult{}, errors.New("conversation_turn_invalid")
	}
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, authorizationActorID); err != nil {
		return TurnResult{}, err
	}
	if authorizationActorID != actorID {
		if err := a.authorizeActorTurn(ctx, authorizationActorID, actorID, fluctlightID, conversationID); err != nil {
			return TurnResult{}, err
		}
	}
	var user map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var existingID string
		var existingText string
		var existingSequence int
		var existingAuthor string
		var existingAttachments []byte
		err := tx.QueryRow(ctx, `SELECT id,sequence,text,author_actor_id,attachment_refs FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversationID, idempotency).Scan(&existingID, &existingSequence, &existingText, &existingAuthor, &existingAttachments)
		if err == nil {
			if existingAuthor != actorID || existingText != text || !jsonEqual(existingAttachments, payload["attachment_refs"]) {
				return ErrConflict
			}
			user = map[string]any{"id": existingID, "conversation_id": conversationID, "sequence": existingSequence, "author_actor_id": existingAuthor, "kind": "user", "text": existingText, "attachment_refs": decodeArray(existingAttachments)}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var participantCount int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id IN ($2,$3) AND status='active'`, conversationID, actorID, fluctlightID).Scan(&participantCount); err != nil {
			return err
		}
		if participantCount != 2 {
			return errors.New("conversation_not_found")
		}
		var seq int
		if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, conversationID).Scan(&seq); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=$2 WHERE conversation_id=$1`, conversationID, seq+1); err != nil {
			return err
		}
		messageID := turnID
		if !strings.HasPrefix(messageID, "message_") {
			messageID = randomID("message_")
		}
		attachments := payload["attachment_refs"]
		if attachments == nil {
			attachments = []any{}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_messages (id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES ($1,$2,$3,$4,'user',$5,$6,$7)`, messageID, conversationID, seq, actorID, text, jsonBytes(attachments), idempotency); err != nil {
			return err
		}
		user = map[string]any{"id": messageID, "conversation_id": conversationID, "sequence": seq, "author_actor_id": actorID, "kind": "user", "text": text, "attachment_refs": attachments}
		return nil
	})
	if err != nil {
		return TurnResult{}, err
	}
	var inboxID string
	claimOwner := ""
	claimSettled := !claimStream
	if claimStream {
		inboxID, claimOwner, err = a.enqueueTurnFactClaimed(ctx, actorID, fluctlightID, conversationID, turnID, idempotency, text, payload["attachment_refs"])
	} else {
		inboxID, err = a.EnqueueTurnFact(ctx, actorID, fluctlightID, conversationID, turnID, idempotency, text, payload["attachment_refs"])
	}
	if err != nil {
		return TurnResult{}, err
	}
	ctx = WithProviderCancellationKey(ctx, inboxID)
	if claimStream {
		defer func() {
			if claimSettled || claimOwner == "" {
				return
			}
			if releaseErr := a.releaseCognitionClaim(ctx, inboxID, claimOwner); releaseErr != nil {
				slog.Default().Warn("Go Core streaming cognition claim cleanup failed", "inbox_id", inboxID, "turn_id", turnID, "claim_owner", claimOwner, "error", releaseErr)
			}
		}()
	}
	var replayed map[string]any
	var replayedID, replayedText string
	var replayedSequence int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT id,sequence,text FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversationID, "assistant:"+turnID).Scan(&replayedID, &replayedSequence, &replayedText); err == nil {
		replayed = map[string]any{"id": replayedID, "conversation_id": conversationID, "sequence": replayedSequence, "author_actor_id": fluctlightID, "kind": "assistant", "text": replayedText, "attachment_refs": []any{}}
		if mediaIntent, recovered, recoveryErr := a.recoverFrozenTurnAfterAssistant(ctx, inboxID, fluctlightID, conversationID, replayedID, replayedText); recoveryErr != nil {
			return TurnResult{}, recoveryErr
		} else if recovered {
			if callbacks.onActionResult != nil {
				if err := callbacks.onActionResult(map[string]any{"message": user, "correlation_id": "turn:" + turnID}); err != nil {
					return TurnResult{}, err
				}
			}
			if callbacks.onChunk != nil {
				if err := callbacks.onChunk(replayedText); err != nil {
					return TurnResult{}, err
				}
			}
			return TurnResult{UserMessage: user, Assistant: replayed, MediaIntentID: mediaIntent, TurnID: turnID, CorrelationID: "turn:" + turnID}, nil
		}
		if callbacks.onActionResult != nil {
			if err := callbacks.onActionResult(map[string]any{"message": user, "correlation_id": "turn:" + turnID}); err != nil {
				return TurnResult{}, err
			}
		}
		if callbacks.onChunk != nil {
			if err := callbacks.onChunk(replayedText); err != nil {
				return TurnResult{}, err
			}
		}
		return TurnResult{UserMessage: user, Assistant: replayed, TurnID: turnID, CorrelationID: "turn:" + turnID}, nil
	}
	var decision map[string]any
	var action string
	var mediaConcept map[string]any
	var toolCalls []ToolCallV1
	var toolResults []ToolResultV1
	var responsePlan map[string]any
	var composite CompositeActionV1
	toolOnlyNoReply := false
	frozen, frozenFound, err := a.LoadFrozenTurn(ctx, inboxID)
	if err != nil {
		return TurnResult{}, err
	}
	var projection ContextProjection
	if frozenFound && frozen.Status == "frozen" {
		frozenDecision := mapValue(frozen.Payload["decision"])
		if savedProjection, ok := contextProjectionFromValue(frozenDecision["context_projection"]); ok {
			projection = savedProjection
		} else {
			projection, err = a.buildTurnProjection(ctx, authorizationActorID, actorID, fluctlightID, conversationID, inboxID, text)
			if err != nil {
				return TurnResult{}, err
			}
		}
	} else {
		projection, err = a.buildTurnProjection(ctx, authorizationActorID, actorID, fluctlightID, conversationID, inboxID, text)
		if err != nil {
			return TurnResult{}, err
		}
	}
	if frozenFound && frozen.Status == "frozen" {
		action = frozen.ActionType
		decision = mapValue(frozen.Payload["decision"])
		if savedProjection, ok := contextProjectionFromValue(decision["context_projection"]); ok {
			projection = savedProjection
		}
		mediaConcept = mapValue(frozen.Payload["media_concept"])
		toolCalls = toolCallsFromValue(decision["tool_calls"])
		if loaded, ok := compositeActionFromValue(decision["composite_action"]); ok {
			composite = loaded
		} else {
			composite, err = normalizeCompositeAction(decision, toolCalls, inboxID, action)
			if err != nil {
				return TurnResult{}, err
			}
		}
		toolResults = toolResultsFromValue(frozen.Payload["tool_results"])
		responsePlan = mapValue(decision["response_plan"])
		if len(responsePlan) == 0 {
			responsePlan, err = normalizeResponsePlan(decision, inboxID, projection)
			if err != nil {
				return TurnResult{}, err
			}
		}
	} else {
		messages := []map[string]any{{"role": "system", "content": conversationAssessmentInstruction}, {"role": "user", "content": jsonString(map[string]any{"current_message": map[string]any{"sender": compactActorRef(projection.CurrentSpeaker), "content": text}, "text": text, "context": compactCognitionContext(projection)})}}
		messages = withActorRelationshipSystemContext(messages, projection)
		messages = withContextAuthorityInstruction(messages)
		// Moment publication is a Wake-up/autonomy output, not an ordinary
		// interactive reply capability. Keep it registered globally for the
		// Runtime while withholding it from the conversation tool catalog.
		manifests := capabilityManifestsExcept(a.capabilityRegistry(), "moment.publish")
		completion, completionErr := a.Provider.StructuredWithToolsSchema(WithProviderScenario(ctx, "cognitive_assessment"), "cognitive_assessment", messages, manifests, "conversation_turn_response", cognitiveTurnResponseSchema(), true)
		if completionErr != nil {
			if a.cognitionFactSuperseded(ctx, inboxID) {
				return TurnResult{}, errCognitionTurnSuperseded
			}
			return TurnResult{}, completionErr
		}
		if a.cognitionFactSuperseded(ctx, inboxID) {
			return TurnResult{}, errCognitionTurnSuperseded
		}
		decision = completion.Structured
		toolCalls = completion.ToolCalls
		for index := range toolCalls {
			toolCalls[index].SourceFactID = inboxID
		}
		toolCalls = normalizeConversationReplyCalls(toolCalls)
		toolCalls = bindMediaContextToToolCalls(toolCalls, projection)
		if decision == nil {
			decision = map[string]any{}
		}
		// A thinking-enabled Provider may return only an immediate native tool
		// call (for example scene_event) and no JSON sidecar. This is a valid
		// internal cognition outcome: execute the tool and settle the turn as
		// no_op instead of requiring a user-visible reply.
		if completion.StructuredFallback && len(toolCalls) > 0 && len(mapValue(decision["appraisal"])) == 0 {
			decision["appraisal"] = toolOnlyCognitionAppraisal(inboxID)
		}
		toolOnlyNoReply = completion.StructuredFallback && len(toolCalls) > 0 && !hasConversationReplyToolCall(toolCalls) && !toolCallsRequireDeferredOutput(toolCalls, a.capabilityRegistry())
		if toolOnlyNoReply {
			decision["appraisal"] = toolOnlyCognitionAppraisal(inboxID)
			decision["action_type"] = "no_op"
			decision["response_intent"] = ""
		}
		if personalityDecision := mapValue(decision["personality_decision"]); len(personalityDecision) > 0 {
			personalityRuntime, personalityErr := a.applyPersonalityDecision(ctx, fluctlightID, personalityDecision)
			if personalityErr != nil {
				return TurnResult{}, personalityErr
			}
			if len(personalityRuntime) > 0 {
				refreshedProjection, refreshErr := a.buildTurnProjection(ctx, authorizationActorID, actorID, fluctlightID, conversationID, inboxID, text)
				if refreshErr != nil {
					return TurnResult{}, refreshErr
				}
				projection = refreshedProjection
				projection.PersonalityRuntime = personalityRuntime
				decision["personality_runtime"] = personalityRuntime
			}
		}
		responsePlan, err = normalizeResponsePlan(decision, inboxID, projection)
		if err != nil {
			return TurnResult{}, err
		}
		responsePlan["tool_calls"] = toolCalls
		decision["response_plan"] = responsePlan
		decision["context_projection"] = projection
		if len(toolCalls) > 0 {
			decision["tool_calls"] = toolCalls
			action, mediaConcept, err = resolveToolCallAction(toolCalls, toolManifestMap(manifests))
			if err != nil {
				return TurnResult{}, err
			}
			if toolOnlyNoReply {
				action = "no_op"
			}
		} else {
			action, mediaConcept = resolveDecisionAction(decision)
		}
		// A missing conversation.reply is an intentional no-visible-reply
		// decision, not a malformed conversation turn. Immediate native tools
		// such as affect_event may still execute in the no_op path. Deferred
		// output calls without a conversation/Moment target remain deferred and
		// are recorded without turning the text turn into an error.
		if normalizedAction, suppressed := normalizeMissingConversationReplyAction(action, toolCalls); suppressed {
			action = normalizedAction
			mediaConcept = nil
			decision["action_type"] = "no_op"
			decision["response_intent"] = ""
			responsePlan["visible_text"] = ""
			decision["visible_text"] = ""
		}
		if len(mediaConcept) > 0 {
			mediaConcept, _ = alignMediaConceptWithContext(mediaConcept, projection)
		}
		if preferenceDecision := mapValue(responsePlan["output_preference_decision"]); len(preferenceDecision) > 0 {
			responsePlan["output_preference_decision"] = evaluateOutputPreferenceAction(preferenceDecision, action, toolCalls)
		}
		composite, err = normalizeCompositeAction(decision, toolCalls, inboxID, action)
		if err != nil {
			return TurnResult{}, err
		}
		decision["composite_action"] = composite
		if action != "reply" && action != "media_request" && action != "no_op" {
			return TurnResult{}, errors.New("decision_effect_invalid")
		}
		if nested, ok := decision["decision"].(map[string]any); ok {
			for key, value := range nested {
				if _, exists := decision[key]; !exists {
					decision[key] = value
				}
			}
		}
		if a.cognitionFactSuperseded(ctx, inboxID) {
			return TurnResult{}, errCognitionTurnSuperseded
		}
		frozen, err = a.PersistTurnDecision(ctx, inboxID, fluctlightID, conversationID, turnID, action, decision, mediaConcept)
		if err != nil {
			return TurnResult{}, err
		}
	}
	if action != "reply" && action != "media_request" && action != "no_op" {
		return TurnResult{}, errors.New("decision_effect_invalid")
	}
	if action == "no_op" {
		for index := range toolCalls {
			toolCalls[index].ActionID = frozen.ID
		}
		if len(toolCalls) > 0 && !frozenFound {
			if err := a.PersistFrozenToolCalls(ctx, frozen.ID, toolCalls); err != nil {
				return TurnResult{}, err
			}
		}
		if len(toolCalls) > 0 && len(toolResults) == 0 {
			toolResults, err = a.ExecuteToolCalls(ctx, fluctlightID, conversationID, inboxID, toolCalls)
			if err != nil {
				if len(toolResults) > 0 {
					_ = a.PersistToolResults(ctx, frozen.ID, toolResults)
				}
				_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "tool_call_failed")
				return TurnResult{}, err
			}
			if err := a.PersistToolResults(ctx, frozen.ID, toolResults); err != nil {
				return TurnResult{}, err
			}
		}
		if a.cognitionFactSuperseded(ctx, inboxID) {
			return TurnResult{}, errCognitionTurnSuperseded
		}
		if err := a.CompleteTurnCognition(ctx, inboxID, frozen.ID, map[string]any{"status": "no_op", "response_intent": stringValue(responsePlan["response_intent"]), "tool_results": toolResults}); err != nil {
			return TurnResult{}, err
		}
		if callbacks.onActionResult != nil {
			if err := callbacks.onActionResult(map[string]any{"message": user, "correlation_id": "turn:" + turnID}); err != nil {
				return TurnResult{}, err
			}
		}
		claimSettled = true
		return TurnResult{UserMessage: user, Assistant: map[string]any{}, TurnID: turnID, CorrelationID: "turn:" + turnID}, nil
	}
	if action == "media_request" && len(mediaConcept) == 0 {
		mediaConcept = mediaConceptValue(decision["media_request"])
		if len(mediaConcept) == 0 {
			mediaConcept = mediaConceptValue(decision["visual_concept"])
		}
		if len(mediaConcept) == 0 {
			_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "media_concept_invalid")
			return TurnResult{}, errors.New("media_concept_invalid")
		}
	}
	for index := range toolCalls {
		toolCalls[index].ActionID = frozen.ID
	}
	if len(toolCalls) > 0 && !frozenFound {
		if err := a.PersistFrozenToolCalls(ctx, frozen.ID, toolCalls); err != nil {
			return TurnResult{}, err
		}
	}
	if callbacks.onActionResult != nil {
		if err := callbacks.onActionResult(map[string]any{"message": user, "correlation_id": "turn:" + turnID}); err != nil {
			return TurnResult{}, err
		}
	}
	mediaIntent := ""
	if len(toolCalls) > 0 {
		if len(toolResults) == 0 {
			toolResults, err = a.ExecuteToolCalls(ctx, fluctlightID, conversationID, inboxID, toolCalls)
			if err != nil {
				// Preserve the structured failure before settling the frozen action.
				// This keeps native capability diagnostics replayable instead of
				// reducing every executor error to `tool_call_failed`.
				if len(toolResults) > 0 {
					_ = a.PersistToolResults(ctx, frozen.ID, toolResults)
				}
				_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "tool_call_failed")
				return TurnResult{}, err
			}
			if err := a.PersistToolResults(ctx, frozen.ID, toolResults); err != nil {
				return TurnResult{}, err
			}
		}
		if toolCallsRequireDeferredOutput(toolCalls, a.capabilityRegistry()) {
			// External async tools are deliberately deferred until the assistant
			// message exists, so their intent can bind to that concrete output.
			// A replay may already have a completed result; otherwise settlement
			// happens in the message transaction below.
			mediaIntent = mediaIntentIDFromToolResults(toolResults)
		}
	}
	var visible string
	// The conversation cognition call is the single semantic pass. Its
	// visible_text is selected together with the active personality, action and
	// response plan, so it must be sent directly instead of being replaced by a
	// second action_realization request.
	visible = normalizeVisibleReply(firstString(responsePlan["visible_text"], stringValue(decision["visible_text"])))
	if strings.TrimSpace(visible) == "" {
		visible = replyTextFromToolCalls(toolCalls)
	}
	if strings.TrimSpace(visible) == "" {
		if frozenFound || frozen.ID != "" {
			_ = a.FailTurnCognition(ctx, inboxID, frozen.ID, "cognition_visible_text_missing")
		}
		return TurnResult{}, errors.New("cognition_visible_text_missing")
	}
	if callbacks.onChunk != nil {
		if err := callbacks.onChunk(visible); err != nil {
			return TurnResult{}, err
		}
	}
	assistantID := randomID("message_")
	var assistant map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var existingID string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversationID, "assistant:"+turnID).Scan(&existingID); err == nil {
			assistantID = existingID
			assistant = map[string]any{"id": existingID, "conversation_id": conversationID, "sequence": 0, "author_actor_id": fluctlightID, "kind": "assistant", "text": visible, "attachment_refs": []any{}}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		} else {
			var seq int
			if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, conversationID).Scan(&seq); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=$2 WHERE conversation_id=$1`, conversationID, seq+1); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_messages (id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES ($1,$2,$3,$4,'assistant',$5,'[]',$6)`, assistantID, conversationID, seq, fluctlightID, visible, "assistant:"+turnID); err != nil {
				return err
			}
			assistant = map[string]any{"id": assistantID, "conversation_id": conversationID, "sequence": seq, "author_actor_id": fluctlightID, "kind": "assistant", "text": visible, "attachment_refs": []any{}}
		}
		if len(toolCalls) > 0 {
			composite = bindCompositeActionOutput(composite, "conversation_message", assistantID)
			bound := OutputBindingV1{ToolCallID: "", TargetKind: "conversation_message", TargetRef: assistantID}
			settled, settleErr := a.settleDeferredToolCallsTx(ctx, tx, fluctlightID, inboxID, inboxID, toolCalls, toolResults, bound)
			if settleErr != nil {
				return settleErr
			}
			toolResults = settled
			mediaIntent = mediaIntentIDFromToolResults(toolResults)
			if _, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(jsonb_set(payload,'{tool_results}',$2::jsonb,true),'{decision,composite_action}',$3::jsonb,true) WHERE id=$1 AND status='frozen'`, frozen.ID, jsonBytes(toolResults), jsonBytes(composite)); err != nil {
				return err
			}
		}
		if len(toolCalls) == 0 {
			composite = bindCompositeActionOutput(composite, "conversation_message", assistantID)
			if _, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(payload,'{decision,composite_action}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, frozen.ID, jsonBytes(composite)); err != nil {
				return err
			}
		}
		if action == "media_request" && mediaIntent == "" {
			concept := mediaConcept
			if len(concept) == 0 {
				concept = mediaConceptValue(decision["media_request"])
			}
			if len(concept) == 0 {
				concept = mediaConceptValue(decision["visual_concept"])
			}
			if len(concept) == 0 {
				return errors.New("media_concept_invalid")
			}
			legacyIntentID := "media_intent_" + stableDigest(inboxID+":legacy-media")
			legacyWorkflowID := "media_workflow_" + stableDigest(inboxID+":legacy-media")
			legacyRequestID := "media_request_" + stableDigest(inboxID+":legacy-media")
			if err := a.createMediaIntentTargetTx(ctx, tx, fluctlightID, concept, legacyIntentID, legacyWorkflowID, legacyRequestID, "", assistantID, ""); err != nil {
				return err
			}
			mediaIntent = legacyIntentID
		}
		return persistClaimsTx(ctx, tx, fluctlightID, inboxID, responsePlan)
	})
	if err != nil {
		return TurnResult{}, err
	}
	if frozen.ID != "" {
		if err := a.CompleteTurnCognition(ctx, inboxID, frozen.ID, map[string]any{"text": visible, "media_intent_id": mediaIntent, "tool_results": toolResults}); err != nil {
			return TurnResult{}, err
		}
	}
	if a.cognitionFactSuperseded(ctx, inboxID) {
		return TurnResult{}, errCognitionTurnSuperseded
	}
	claimSettled = true
	return TurnResult{UserMessage: user, Assistant: assistant, MediaIntentID: mediaIntent, TurnID: turnID, CorrelationID: "turn:" + turnID}, nil
}

func (a *App) authorizeActorTurn(ctx context.Context, ownerActorID, senderActorID, fluctlightID, conversationID string) error {
	var actorType, status string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT actor_type,status FROM public.actors WHERE id=$1`, senderActorID).Scan(&actorType, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnauthorized
		}
		return err
	}
	if status != "active" || (actorType != "human" && actorType != "fluctlight") {
		return ErrUnauthorized
	}
	if actorType == "fluctlight" {
		var createdBy string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, senderActorID).Scan(&createdBy); err != nil || createdBy != ownerActorID {
			return ErrUnauthorized
		}
	}
	var participants int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id IN ($2,$3) AND status='active'`, conversationID, senderActorID, fluctlightID).Scan(&participants); err != nil {
		return err
	}
	if participants != 2 {
		return errors.New("conversation_not_found")
	}
	return nil
}

func (a *App) buildTurnProjection(ctx context.Context, authorizationActorID, speakerActorID, fluctlightID, conversationID, sourceFactID, userText string) (ContextProjection, error) {
	if authorizationActorID == speakerActorID {
		return a.BuildContextProjection(ctx, authorizationActorID, fluctlightID, conversationID, sourceFactID, userText)
	}
	projection, err := a.BuildContextProjection(ctx, authorizationActorID, fluctlightID, conversationID, sourceFactID, userText)
	if err != nil {
		return ContextProjection{}, err
	}
	fluctlight, err := a.DB.GetFluctlight(ctx, fluctlightID, authorizationActorID)
	if err != nil {
		return ContextProjection{}, err
	}
	displayName := firstString(fluctlight.Identity["name"], "摇光")
	extraIDs := make([]string, 0, len(projection.Relationships))
	for _, relationship := range projection.Relationships {
		if target := stringValue(relationship["target_actor_id"]); target != "" {
			extraIDs = append(extraIDs, target)
		}
	}
	actors, selfActor, currentSpeaker := a.buildActorProjection(ctx, fluctlightID, speakerActorID, displayName, projection.RecentMessages, extraIDs)
	relationships, err := a.readRelationships(ctx, fluctlightID, authorizationActorID)
	if err != nil {
		return ContextProjection{}, err
	}
	filtered := make([]map[string]any, 0, 1)
	for _, relationship := range relationships {
		if stringValue(relationship["target_actor_id"]) == speakerActorID {
			filtered = append(filtered, relationship)
		}
	}
	projection.Actors = actors
	projection.SelfActor = selfActor
	projection.CurrentSpeaker = currentSpeaker
	projection.Relationships = filtered
	return projection, nil
}

func (a *App) recoverFrozenTurnAfterAssistant(ctx context.Context, inboxID, fluctlightID, conversationID, assistantID, visible string) (string, bool, error) {
	frozen, found, err := a.LoadFrozenTurn(ctx, inboxID)
	if err != nil || !found || frozen.Status != "frozen" {
		return "", false, err
	}
	decision := mapValue(frozen.Payload["decision"])
	calls := toolCallsFromValue(decision["tool_calls"])
	results := toolResultsFromValue(frozen.Payload["tool_results"])
	action := frozen.ActionType
	composite, ok := compositeActionFromValue(decision["composite_action"])
	if !ok {
		composite, err = normalizeCompositeAction(decision, calls, inboxID, action)
		if err != nil {
			return "", true, err
		}
	}
	mediaIntent := mediaIntentIDFromToolResults(results)
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if len(calls) > 0 {
			composite = bindCompositeActionOutput(composite, "conversation_message", assistantID)
			settled, settleErr := a.settleDeferredToolCallsTx(ctx, tx, fluctlightID, inboxID, inboxID, calls, results, OutputBindingV1{TargetKind: "conversation_message", TargetRef: assistantID})
			if settleErr != nil {
				return settleErr
			}
			results = settled
			mediaIntent = mediaIntentIDFromToolResults(results)
		}
		if action == "media_request" && mediaIntent == "" {
			concept := mapValue(frozen.Payload["media_concept"])
			if len(concept) == 0 {
				concept = mediaConceptValue(decision["media_request"])
			}
			if len(concept) == 0 {
				concept = mediaConceptValue(decision["visual_concept"])
			}
			if len(concept) == 0 {
				return errors.New("media_concept_invalid")
			}
			intentID := "media_intent_" + stableDigest(inboxID+":legacy-media")
			if err := a.createMediaIntentTargetTx(ctx, tx, fluctlightID, concept, intentID, "media_workflow_"+stableDigest(inboxID+":legacy-media"), "media_request_"+stableDigest(inboxID+":legacy-media"), "", assistantID, ""); err != nil {
				return err
			}
			mediaIntent = intentID
		}
		payloadUpdate := `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(jsonb_set(payload,'{tool_results}',$2::jsonb,true),'{decision,composite_action}',$3::jsonb,true) WHERE id=$1 AND status='frozen'`
		_, err := tx.Exec(ctx, payloadUpdate, frozen.ID, jsonBytes(results), jsonBytes(composite))
		return err
	})
	if err != nil {
		return "", true, err
	}
	if err := a.CompleteTurnCognition(ctx, inboxID, frozen.ID, map[string]any{"text": visible, "media_intent_id": mediaIntent, "tool_results": results}); err != nil {
		return "", true, err
	}
	return mediaIntent, true, nil
}

func (a *App) createMediaIntent(ctx context.Context, fluctlightID, conversationID string, concept map[string]any) (string, error) {
	id := randomID("media_intent_")
	workflowID := randomID("media_workflow_")
	requestID := randomID("media_request_")
	return id, a.createMediaIntentWithIdentity(ctx, fluctlightID, conversationID, concept, id, workflowID, requestID)
}

func (a *App) createMediaIntentWithIdentity(ctx context.Context, fluctlightID, conversationID string, concept map[string]any, id, workflowID, requestID string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		return a.createMediaIntentTargetTx(ctx, tx, fluctlightID, concept, id, workflowID, requestID, conversationID, "", "")
	})
}

func (a *App) createMediaIntentTarget(ctx context.Context, fluctlightID string, concept map[string]any, id, workflowID, requestID, conversationID, messageID, momentID string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		return a.createMediaIntentTargetTx(ctx, tx, fluctlightID, concept, id, workflowID, requestID, conversationID, messageID, momentID)
	})
}

// createMediaIntentTx is retained for callers that only have the legacy
// conversation-or-moment target shape. New composite actions should use
// createMediaIntentTargetTx so a generated asset can bind directly to a
// concrete message or Moment output.
func (a *App) createMediaIntentTx(ctx context.Context, tx pgx.Tx, fluctlightID string, concept map[string]any, id, workflowID, requestID, conversationID, momentID string) error {
	return a.createMediaIntentTargetTx(ctx, tx, fluctlightID, concept, id, workflowID, requestID, conversationID, "", momentID)
}

func (a *App) createMediaIntentTargetTx(ctx context.Context, tx pgx.Tx, fluctlightID string, concept map[string]any, id, workflowID, requestID, conversationID, messageID, momentID string) error {
	if conversationID != "" && messageID != "" {
		return errors.New("media_intent_target_ambiguous")
	}
	if messageID != "" && momentID != "" {
		return errors.New("media_intent_target_ambiguous")
	}
	if messageID != "" {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_messages WHERE id=$1)`, messageID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	if momentID != "" {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.moments WHERE id=$1)`, momentID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents (id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,conversation_id,message_id,moment_id,status,revision) VALUES ($1,$2,'image','image/png',$3,$4,$5,$6,$7,$8,'pending',0) ON CONFLICT (id) DO NOTHING`, id, fluctlightID, jsonString(concept), requestID, workflowID, nullableString(conversationID), nullableString(messageID), nullableString(momentID)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'media','media.generation',$3) ON CONFLICT DO NOTHING`, "media_workflow_intent:"+id, workflowID, jsonBytes(map[string]any{"intent_id": id, "provider_request_id": requestID, "fluctlight_id": fluctlightID})); err != nil {
		return err
	}
	return appendOutboxTx(ctx, tx, "media.intent.created", "media_intent", id, fluctlightID, requestID, "media:"+id, "media-intent:"+id, map[string]any{"intent_id": id, "workflow_id": workflowID, "moment_id": nullableString(momentID), "message_id": nullableString(messageID), "conversation_id": nullableString(conversationID)})
}

func (a *App) StreamTurn(ctx context.Context, writer http.ResponseWriter, actorID, conversationID string, payload map[string]any) error {
	turnID := stringValue(payload["turn_id"])
	if turnID == "" {
		turnID = "turn_" + stableDigest(conversationID+":"+stringValue(payload["idempotency_key"]))
	}
	sequence := 0
	started := false
	writeFrame := func(kind string, framePayload map[string]any) error {
		if err := json.NewEncoder(writer).Encode(map[string]any{"type": kind, "turn_id": turnID, "sequence": sequence, "payload": framePayload}); err != nil {
			return err
		}
		sequence++
		started = true
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}
	turnActorID := actorID
	turnPayload := payload
	if sender := strings.TrimSpace(stringValue(payload["sender_actor_id"])); sender != "" && sender != actorID {
		turnActorID = sender
		turnPayload = cloneMap(payload)
		turnPayload["authorization_actor_id"] = actorID
	}
	result, err := a.handleTurn(ctx, turnActorID, conversationID, turnPayload, turnCallbacks{
		onActionResult: func(framePayload map[string]any) error {
			return writeFrame("action_result", framePayload)
		},
		onChunk: func(chunk string) error {
			return writeFrame("token", map[string]any{"text": chunk})
		},
	}, true)
	if err != nil {
		if errors.Is(err, errCognitionTurnSuperseded) {
			return writeFrame("completed", map[string]any{"message_ids": []string{}})
		}
		if !started || ctx.Err() != nil {
			return err
		}
		// Visible output/action_result has already reached the client. A later
		// lifecycle failure (claims, deferred settlement, reflection scheduling,
		// or CompleteTurnCognition) must not turn that successful visible turn
		// into a conversation_turn_failed frame. Keep the failure in server logs;
		// the durable frozen action/inbox remains available for reconciliation.
		slog.Default().Error("Go Core conversation turn lifecycle settlement failed after visible output", "error", err, "turn_id", turnID)
		return writeFrame("completed", map[string]any{"message_ids": []string{}, "status": "settlement_deferred"})
	}
	messageIDs := make([]string, 0, 1)
	if messageID := stringValue(result.Assistant["id"]); messageID != "" {
		messageIDs = append(messageIDs, messageID)
	}
	return writeFrame("completed", map[string]any{"message_ids": messageIDs})
}

func toolOnlyCognitionAppraisal(sourceFactID string) map[string]any {
	return map[string]any{
		"relevance": 0.0, "goal_congruence": 0.0, "reward": 0.0, "loss": 0.0,
		"social_threat": 0.0, "controllability": 1.0, "responsibility": 0.0,
		"relationship_significance": 0.0, "expected_effect": 0.0,
		"evidence_refs": []any{sourceFactID}, "event_kind": "tool_only_action", "direction": "none",
	}
}

func hasConversationReplyToolCall(calls []ToolCallV1) bool {
	for _, call := range calls {
		if call.Name != "conversation.reply" {
			continue
		}
		var args map[string]any
		if json.Unmarshal(call.Arguments, &args) == nil && strings.TrimSpace(stringValue(args["text"])) != "" {
			return true
		}
	}
	return false
}

func normalizeMissingConversationReplyAction(action string, calls []ToolCallV1) (string, bool) {
	if hasConversationReplyToolCall(calls) {
		return action, false
	}
	if action == "reply" || action == "media_request" {
		return "no_op", true
	}
	return action, false
}

// normalizeConversationReplyCalls accepts the transitional model behavior
// where the Provider requests the just-installed conversation.reply slot via
// capability.request. The text inside desired_contract is already the model's
// final reply; route it through the canonical reply tool instead of persisting
// a capability proposal or dropping the message.
func normalizeConversationReplyCalls(calls []ToolCallV1) []ToolCallV1 {
	result := make([]ToolCallV1, len(calls))
	copy(result, calls)
	for index := range result {
		if result[index].Name != "capability.request" {
			continue
		}
		var args map[string]any
		if json.Unmarshal(result[index].Arguments, &args) != nil || stringValue(args["capability_key"]) != "conversation.reply" {
			continue
		}
		contract := mapValue(args["desired_contract"])
		text := strings.TrimSpace(stringValue(contract["text"]))
		if text == "" {
			continue
		}
		result[index].Name = "conversation.reply"
		result[index].Arguments = jsonBytes(map[string]any{"text": text})
	}
	return result
}

func replyTextFromToolCalls(calls []ToolCallV1) string {
	for _, call := range calls {
		if call.Name != "conversation.reply" {
			continue
		}
		var args map[string]any
		if json.Unmarshal(call.Arguments, &args) == nil {
			if text := strings.TrimSpace(stringValue(args["text"])); text != "" {
				return normalizeVisibleReply(text)
			}
		}
	}
	return ""
}

func resolveDecisionAction(decision map[string]any) (string, map[string]any) {
	action := firstString(decision["action_type"], "")
	concept := mediaConceptValue(decision["media_request"])
	if len(concept) == 0 {
		concept = mediaConceptValue(decision["visual_concept"])
	}
	if nested, ok := decision["decision"].(map[string]any); ok {
		if action == "" {
			action = firstString(nested["action_type"], "")
		}
		if len(concept) == 0 {
			concept = mediaConceptValue(nested["media_request"])
		}
		if effects, ok := nested["effects"].([]any); ok {
			for _, raw := range effects {
				effect := mapValue(raw)
				kind := firstString(effect["action_type"], firstString(effect["type"], ""))
				if kind == "reply" {
					if action == "" {
						action = "reply"
					}
					continue
				}
				if kind == "media_request" {
					action = "media_request"
					if len(concept) == 0 {
						concept = mediaConceptValue(effect["payload"])
						if len(concept) == 0 {
							concept = mediaConceptValue(effect["visual_concept"])
						}
					}
				}
				if kind == "moment" || kind == "proactive_message" {
					action = kind
				}
			}
		}
	}
	if action == "" {
		action = firstString(decision["action"], "")
	}
	if plan := mapValue(decision["response_plan"]); len(plan) > 0 {
		if action == "" {
			action = firstString(plan["action_type"], "")
		}
		if len(concept) == 0 {
			concept = mediaConceptValue(plan["media_request"])
		}
	}
	return normalizeConversationActionType(action), concept
}

func mediaConceptValue(value any) map[string]any {
	if object := mapValue(value); len(object) > 0 {
		return object
	}
	if text := stringValue(value); text != "" {
		return map[string]any{"visual_concept": text}
	}
	return map[string]any{}
}

func jsonString(value any) string { data, _ := json.Marshal(value); return string(data) }
func intValue(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	}
	return 0
}
func numberString(value any, fallback float64) string {
	if v, ok := value.(float64); ok {
		return fmt.Sprintf("%g", v)
	}
	if v, ok := value.(int); ok {
		return fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("%g", fallback)
}

func numberFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		parsed, err := v.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func normalizeScheduleScalar(value any) any {
	if numeric, ok := numberFloat(value); ok {
		return numeric
	}
	switch strings.ToLower(strings.TrimSpace(stringValue(value))) {
	case "very_high", "very high", "urgent":
		return 1.0
	case "high":
		return 0.9
	case "medium", "normal":
		return 0.5
	case "low":
		return 0.1
	case "very_low", "very low":
		return 0.0
	case "none":
		return 0.0
	default:
		return value
	}
}
func firstString(value any, fallback string) string {
	if v, ok := value.(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func jsonEqual(raw []byte, value any) bool {
	var left any
	if len(raw) == 0 {
		raw = []byte("[]")
	}
	if json.Unmarshal(raw, &left) != nil {
		return false
	}
	right := value
	if right == nil {
		right = []any{}
	}
	return jsonString(left) == jsonString(right)
}
