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

// agentCommittedOutcome is the application-facing view of a completed Agent
// run. Tool calls are audit records here: the ADK bridge has already executed
// and committed them before they entered the trace.
type agentCommittedOutcome struct {
	Invocations []CapabilityInvocation
	Results     []CapabilityResult
}

var errAgentFinalContractInvalid = errors.New("agent_final_contract_invalid")

func committedAgentOutcome(trace *ADKCapabilityTrace) (agentCommittedOutcome, error) {
	if trace == nil {
		return agentCommittedOutcome{}, nil
	}
	invocations, results := trace.Snapshot()
	resultByCall := make(map[string]CapabilityResult, len(results))
	for _, result := range results {
		callID := strings.TrimSpace(result.CallID)
		if callID == "" {
			return agentCommittedOutcome{}, errors.New("agent_tool_result_identity_missing")
		}
		if _, duplicate := resultByCall[callID]; duplicate {
			return agentCommittedOutcome{}, errors.New("agent_tool_result_identity_duplicate")
		}
		resultByCall[callID] = result
	}
	for _, invocation := range invocations {
		result, found := resultByCall[strings.TrimSpace(invocation.CallID)]
		if !found || result.CapabilityName != invocation.CapabilityName {
			return agentCommittedOutcome{}, errors.New("agent_tool_result_missing")
		}
	}
	return agentCommittedOutcome{Invocations: invocations, Results: results}, nil
}

func committedConversationReplyResult(results []CapabilityResult) (CapabilityResult, bool) {
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		if result.CapabilityName == "conversation.reply" && result.Status == "completed" && stringValue(mapValue(result.Output)["target_kind"]) == "conversation_message" && strings.TrimSpace(stringValue(mapValue(result.Output)["target_ref"])) != "" {
			return result, true
		}
	}
	return CapabilityResult{}, false
}

func committedConversationReplyResults(results []CapabilityResult) []CapabilityResult {
	replies := make([]CapabilityResult, 0)
	for _, result := range results {
		if result.CapabilityName == "conversation.reply" && result.Status == "completed" && stringValue(mapValue(result.Output)["target_kind"]) == "conversation_message" && strings.TrimSpace(stringValue(mapValue(result.Output)["target_ref"])) != "" {
			replies = append(replies, result)
		}
	}
	return replies
}

func mediaIntentFromCommittedResults(results []CapabilityResult) string {
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		if result.CapabilityName != "media.image.generate" || (result.Status != "accepted" && result.Status != "completed") {
			continue
		}
		if id := strings.TrimSpace(stringValue(mapValue(result.Output)["media_intent_id"])); id != "" {
			return id
		}
	}
	return ""
}

func (a *App) loadCommittedAssistantMessage(ctx context.Context, conversationID, fluctlightID string, result CapabilityResult) (map[string]any, error) {
	messageID := strings.TrimSpace(stringValue(mapValue(result.Output)["target_ref"]))
	if messageID == "" {
		return nil, errors.New("agent_reply_target_missing")
	}
	var id, ownerConversation, authorID, kind, text string
	var sequence int
	var attachments []byte
	var createdAt time.Time
	if err := a.DB.Pool().QueryRow(ctx, `SELECT id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,created_at FROM public.conversation_messages WHERE id=$1`, messageID).Scan(&id, &ownerConversation, &sequence, &authorID, &kind, &text, &attachments, &createdAt); err != nil {
		return nil, err
	}
	if ownerConversation != conversationID || authorID != fluctlightID || kind != "assistant" {
		return nil, ErrUnauthorized
	}
	return map[string]any{
		"id": id, "conversation_id": ownerConversation, "sequence": sequence,
		"author_actor_id": authorID, "kind": kind, "text": text,
		"attachment_refs": decodeArray(attachments), "created_at": createdAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func finalAgentVisibleText(completion ProviderCompletion) string {
	if text := normalizeVisibleReply(stringValue(completion.Structured["visible_text"])); text != "" {
		return text
	}
	if text := normalizeVisibleReply(stringValue(mapValue(completion.Structured["response_plan"])["visible_text"])); text != "" {
		return text
	}
	if completion.Structured != nil {
		return ""
	}
	return normalizeVisibleReply(completion.Text)
}

// publishNaturalAgentReply is the output adapter for an Agent that naturally
// finishes without conversation.reply. It uses the same publication service
// as the formal Tool and therefore shares ownership, idempotency and sequence
// rules without manufacturing a ToolCall.
func (a *App) publishNaturalAgentReply(ctx context.Context, actorID, fluctlightID, conversationID, operationID, correlationID, text string) (map[string]any, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("cognition_visible_text_missing")
	}
	var resource publishedResource
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		resource, err = NewToolPublicationService(a).PublishConversationReplyTx(ctx, tx, ConversationReplyPublication{
			AuthorizationActorID: actorID, FluctlightID: fluctlightID, ConversationID: conversationID,
			OperationID: operationID, CorrelationID: correlationID, Text: text,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return a.loadCommittedAssistantMessage(ctx, conversationID, fluctlightID, CapabilityResult{Output: map[string]any{"target_ref": resource.ID}})
}

// handleTurn is the production conversation caller for the unified Agent
// runtime. It owns input acceptance and final product projection only. Tool
// execution happens inside the Eino loop and the trace is consumed strictly as
// an audit/committed-result channel.
func (a *App) handleTurn(ctx context.Context, actorID, conversationID string, payload map[string]any, callbacks turnCallbacks, claimStream bool) (TurnResult, error) {
	authorizationActorID := firstString(payload["authorization_actor_id"], actorID)
	fluctlightID := strings.TrimSpace(stringValue(payload["fluctlight_id"]))
	text := strings.TrimSpace(stringValue(payload["text"]))
	idempotency := strings.TrimSpace(stringValue(payload["idempotency_key"]))
	turnID := strings.TrimSpace(stringValue(payload["turn_id"]))
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

	claimOwner := ""
	if claimStream {
		claimOwner = "go-stream:" + randomID("claim_")
	}
	claimSettled := !claimStream
	var user map[string]any
	var inboxID string
	var supersededInboxIDs []string
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var existingID, existingText, existingAuthor, existingTurnID, existingSourceFactID, existingCorrelationID string
		var existingSequence int
		var existingAttachments []byte
		var existingCreatedAt time.Time
		err := tx.QueryRow(ctx, `SELECT id,sequence,text,author_actor_id,attachment_refs,created_at,COALESCE(turn_id,''),COALESCE(source_fact_id,''),COALESCE(correlation_id,'') FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, conversationID, idempotency).Scan(&existingID, &existingSequence, &existingText, &existingAuthor, &existingAttachments, &existingCreatedAt, &existingTurnID, &existingSourceFactID, &existingCorrelationID)
		messageExists := err == nil
		if err == nil {
			if existingAuthor != actorID || existingText != text || !jsonEqual(existingAttachments, payload["attachment_refs"]) || (existingTurnID != "" && existingTurnID != turnID) {
				return ErrConflict
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !messageExists {
			var participantCount int
			if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id IN ($2,$3) AND status='active'`, conversationID, actorID, fluctlightID).Scan(&participantCount); err != nil {
				return err
			}
			if participantCount != 2 {
				return errors.New("conversation_not_found")
			}
		}
		var enqueueErr error
		inboxID, supersededInboxIDs, enqueueErr = a.enqueueTurnFactTx(ctx, tx, actorID, fluctlightID, conversationID, turnID, idempotency, text, payload["attachment_refs"], claimOwner)
		if enqueueErr != nil {
			return enqueueErr
		}
		correlationID := "turn:" + turnID
		if messageExists {
			if (existingSourceFactID != "" && existingSourceFactID != inboxID) || (existingCorrelationID != "" && existingCorrelationID != correlationID) {
				return ErrConflict
			}
			if existingSourceFactID == "" || existingTurnID == "" || existingCorrelationID == "" {
				if _, err := tx.Exec(ctx, `UPDATE public.conversation_messages SET turn_id=COALESCE(turn_id,$2),source_fact_id=COALESCE(source_fact_id,$3),correlation_id=COALESCE(correlation_id,$4) WHERE id=$1`, existingID, turnID, inboxID, correlationID); err != nil {
					return err
				}
			}
			user = map[string]any{"id": existingID, "conversation_id": conversationID, "sequence": existingSequence, "author_actor_id": existingAuthor, "kind": "user", "text": existingText, "attachment_refs": decodeArray(existingAttachments), "created_at": existingCreatedAt.UTC().Format(time.RFC3339Nano)}
			return nil
		}
		var sequence int
		if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, conversationID).Scan(&sequence); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=$2 WHERE conversation_id=$1`, conversationID, sequence+1); err != nil {
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
		var createdAt time.Time
		if err := tx.QueryRow(ctx, `INSERT INTO public.conversation_messages (id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,turn_id,source_fact_id,correlation_id) VALUES ($1,$2,$3,$4,'user',$5,$6,$7,$8,$9,$10) RETURNING created_at`, messageID, conversationID, sequence, actorID, text, jsonBytes(attachments), idempotency, turnID, inboxID, correlationID).Scan(&createdAt); err != nil {
			return err
		}
		user = map[string]any{"id": messageID, "conversation_id": conversationID, "sequence": sequence, "author_actor_id": actorID, "kind": "user", "text": text, "attachment_refs": attachments, "created_at": createdAt.UTC().Format(time.RFC3339Nano)}
		return nil
	})
	if err != nil {
		return TurnResult{}, err
	}
	a.cancelSupersededCognitionFacts(ctx, supersededInboxIDs)
	if preemptErr := a.CancelLifecycleForCognition(ctx, fluctlightID, "cognition:"+inboxID); preemptErr != nil {
		a.recordDiagnosticEvent(ctx, "cognition.lifecycle_preemption.degraded", "warning", fluctlightID, inboxID, "turn:"+turnID, map[string]any{"error_code": "lifecycle_preemption_failed"})
	}
	ctx = WithProviderCancellationKey(ctx, inboxID)
	if claimStream {
		defer func() {
			if !claimSettled && claimOwner != "" {
				_ = a.releaseCognitionClaim(ctx, inboxID, claimOwner)
			}
		}()
	}
	if callbacks.onActionResult != nil {
		if err := callbacks.onActionResult(map[string]any{"message": user, "correlation_id": "turn:" + turnID}); err != nil {
			return TurnResult{}, err
		}
	}

	if replayed, replayedAssistants, replayErr := a.replayCommittedAgentTurn(ctx, user, inboxID, fluctlightID, conversationID, turnID); replayErr != nil {
		return TurnResult{}, replayErr
	} else if replayed != nil {
		if err := emitCommittedAssistantMessages(callbacks, replayedAssistants, replayed.CorrelationID); err != nil {
			return TurnResult{}, err
		}
		claimSettled = true
		return *replayed, nil
	}
	if a.cognitionFactSuperseded(ctx, inboxID) {
		return TurnResult{}, errCognitionTurnSuperseded
	}

	var attemptCount int
	_ = a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(attempt_count,0) FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&attemptCount)
	runID := "turn:" + turnID
	if attemptCount > 0 {
		runID = fmt.Sprintf("turn:%s:attempt:%d", turnID, attemptCount)
	}

	run, err := a.RunConversationCognitionAgent(WithProviderCorrelation(ctx, "turn:"+turnID), ConversationCognitionAgentInput{
		AuthorizationActorID: authorizationActorID, SpeakerActorID: actorID, FluctlightID: fluctlightID,
		ConversationID: conversationID, SourceFactID: inboxID, RunID: runID, CurrentInput: text, EnableStreaming: claimStream,
	})
	projection := run.Projection
	outcome, outcomeErr := committedAgentOutcome(run.Trace)
	if outcomeErr != nil {
		return TurnResult{}, outcomeErr
	}
	if err != nil {
		_ = a.failAgentTurnAfterRun(ctx, inboxID, outcome, "agent_run_failed")
		return TurnResult{}, err
	}
	if a.cognitionFactSuperseded(ctx, inboxID) {
		return TurnResult{}, errCognitionTurnSuperseded
	}

	decision := cloneMap(run.Completion.Structured)
	if decision == nil {
		decision = map[string]any{}
	}
	if _, err := freezeDecisionInfluences(decision, projection, false); err != nil {
		_ = a.failAgentTurnAfterRun(ctx, inboxID, outcome, "agent_final_contract_invalid")
		return TurnResult{}, fmt.Errorf("%w: %w", errAgentFinalContractInvalid, err)
	}
	if len(mapValue(decision["appraisal"])) == 0 {
		decision["cognitive_state_transition"] = "not_proposed"
	}
	responsePlan, err := normalizeResponsePlan(decision, inboxID, projection)
	if err != nil {
		_ = a.failAgentTurnAfterRun(ctx, inboxID, outcome, "agent_final_contract_invalid")
		return TurnResult{}, fmt.Errorf("%w: %w", errAgentFinalContractInvalid, err)
	}

	var assistant map[string]any
	assistantMessages := make([]map[string]any, 0)
	if replyResults := committedConversationReplyResults(outcome.Results); len(replyResults) > 0 {
		for _, replyResult := range replyResults {
			message, loadErr := a.loadCommittedAssistantMessage(ctx, conversationID, fluctlightID, replyResult)
			if loadErr != nil {
				err = loadErr
				break
			}
			assistantMessages = append(assistantMessages, message)
		}
		if len(assistantMessages) > 0 {
			assistant = assistantMessages[len(assistantMessages)-1]
		}
	} else if visible := finalAgentVisibleText(run.Completion); visible != "" {
		assistant, err = a.publishNaturalAgentReply(ctx, authorizationActorID, fluctlightID, conversationID, "agent-final:"+turnID, "turn:"+turnID, visible)
		if err == nil {
			assistantMessages = append(assistantMessages, assistant)
		}
	} else if len(outcome.Results) > 0 {
		assistant = map[string]any{}
	} else {
		err = errors.New("cognition_visible_text_missing")
	}
	if err != nil {
		_ = a.failAgentTurnAfterRun(ctx, inboxID, outcome, "agent_output_publication_failed")
		return TurnResult{}, fmt.Errorf("agent_output_publication_failed: %w", err)
	}
	mediaIntentID := mediaIntentFromCommittedResults(outcome.Results)
	if err := a.settleAgentConversationTurn(ctx, inboxID, turnID, fluctlightID, actorID, projection, decision, responsePlan, assistant, mediaIntentID, outcome); err != nil {
		// The reply and every Tool result above are already committed facts. A
		// cognition projection failure must not erase or republish them.
		_ = a.failAgentTurnAfterRun(ctx, inboxID, outcome, "agent_cognition_settlement_failed")
		_ = emitCommittedAssistantMessages(callbacks, assistantMessages, "turn:"+turnID)
		return TurnResult{}, fmt.Errorf("agent_cognition_settlement_failed: %w", err)
	}
	if followupErr := a.scheduleCognitionFollowups(ctx, fluctlightID); followupErr != nil {
		a.recordDiagnosticEvent(ctx, "cognition.followup.degraded", "warning", fluctlightID, inboxID, "turn:"+turnID, map[string]any{"error_code": "followup_schedule_failed"})
	}
	if err := emitCommittedAssistantMessages(callbacks, assistantMessages, "turn:"+turnID); err != nil {
		return TurnResult{}, err
	}
	claimSettled = true
	return TurnResult{UserMessage: user, Assistant: assistant, MediaIntentID: mediaIntentID, TurnID: turnID, CorrelationID: "turn:" + turnID}, nil
}

func emitCommittedAssistantMessages(callbacks turnCallbacks, messages []map[string]any, correlationID string) error {
	for _, message := range messages {
		text := strings.TrimSpace(stringValue(message["text"]))
		if callbacks.onChunk != nil && text != "" {
			if err := callbacks.onChunk(text); err != nil {
				return err
			}
		}
		if callbacks.onActionResult != nil {
			if err := callbacks.onActionResult(map[string]any{"message": message, "correlation_id": correlationID}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *App) replayCommittedAgentTurn(ctx context.Context, user map[string]any, inboxID, fluctlightID, conversationID, turnID string) (*TurnResult, []map[string]any, error) {
	var status, errorCode string
	var payload []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,''),payload FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&status, &errorCode, &payload); err != nil {
		return nil, nil, err
	}
	if status == "failed" {
		var assistantExists bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2)`, conversationID, "assistant:"+turnID).Scan(&assistantExists); err == nil && assistantExists {
			status = "processed"
		} else {
			return nil, nil, errors.New(firstString(errorCode, "agent_turn_failed"))
		}
	}
	if status != "processed" {
		return nil, nil, nil
	}
	result := mapValue(decodeObject(payload)["agent_result"])
	messageIDs := make([]string, 0)
	for _, raw := range arrayValue(result["message_ids"]) {
		if id := strings.TrimSpace(stringValue(raw)); id != "" {
			messageIDs = append(messageIDs, id)
		}
	}
	if len(messageIDs) == 0 {
		if id := strings.TrimSpace(stringValue(result["message_id"])); id != "" {
			messageIDs = append(messageIDs, id)
		}
	}
	if len(messageIDs) == 0 {
		if len(arrayValue(result["capability_results"])) == 0 {
			return nil, nil, errors.New("processed_agent_turn_result_missing")
		}
		return &TurnResult{UserMessage: user, Assistant: map[string]any{}, MediaIntentID: stringValue(result["media_intent_id"]), TurnID: turnID, CorrelationID: "turn:" + turnID}, nil, nil
	}
	assistants := make([]map[string]any, 0, len(messageIDs))
	for _, messageID := range messageIDs {
		assistant, err := a.loadCommittedAssistantMessage(ctx, conversationID, fluctlightID, CapabilityResult{Output: map[string]any{"target_ref": messageID}})
		if err != nil {
			return nil, nil, err
		}
		assistants = append(assistants, assistant)
	}
	return &TurnResult{UserMessage: user, Assistant: assistants[len(assistants)-1], MediaIntentID: stringValue(result["media_intent_id"]), TurnID: turnID, CorrelationID: "turn:" + turnID}, assistants, nil
}

func (a *App) failAgentTurnAfterRun(ctx context.Context, inboxID string, outcome agentCommittedOutcome, code string) error {
	// Cancellation ends model work, not the bounded recording of facts already
	// committed by a Tool. A retry must see failure rather than replay the run.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var fluctlightID, correlationID string
	if err := a.DB.Pool().QueryRow(ctx, `UPDATE public.cognition_inbox SET status='failed',processed_at=now(),claimed_by=NULL,claimed_at=NULL,error_code=$2,payload=jsonb_set(payload,'{agent_partial}',$3::jsonb,true) WHERE id=$1 AND status IN ('pending','claimed') RETURNING fluctlight_id,COALESCE(correlation_id,'')`, inboxID, code, jsonBytes(map[string]any{"capability_invocations": outcome.Invocations, "capability_results": outcome.Results})).Scan(&fluctlightID, &correlationID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		return err
	}
	stage := "agent_run"
	switch strings.TrimSpace(code) {
	case "agent_final_contract_invalid", "native_cognition_final_contract_invalid":
		stage = "final_contract"
	case "agent_output_publication_failed":
		stage = "output_publication"
	case "agent_cognition_settlement_failed", "native_cognition_settlement_failed":
		stage = "settlement"
	}
	a.recordDiagnosticEvent(ctx, "agent.run.termination", "error", fluctlightID, inboxID, correlationID, map[string]any{
		"run_id": firstString(correlationID, inboxID), "stage": stage, "status": "failed", "reason": strings.TrimSpace(code),
		"committed_tool_count": len(outcome.Results),
	})
	return nil
}

func (a *App) settleAgentConversationTurn(ctx context.Context, inboxID, turnID, fluctlightID, actorID string, projection ContextProjection, decision, responsePlan, assistant map[string]any, mediaIntentID string, outcome agentCommittedOutcome) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var inboxStatus, inboxError string
		if err := tx.QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$1 FOR UPDATE`, inboxID).Scan(&inboxStatus, &inboxError); err != nil {
			return err
		}
		if inboxStatus == "failed" && inboxError == "superseded_by_newer_turn" {
			return errCognitionTurnSuperseded
		}
		if inboxStatus != "pending" && inboxStatus != "claimed" {
			return ErrConflict
		}
		expectedFoundation, expectedState, expectedLife, err := a.agentSettlementAuthorityRevisionsTx(ctx, tx, fluctlightID, projection, outcome)
		if err != nil {
			return err
		}
		if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, expectedFoundation, expectedState, expectedLife, time.Now().UTC()); err != nil {
			return err
		}
		actionType := "reply"
		if strings.TrimSpace(stringValue(assistant["id"])) == "" {
			actionType = "no_op"
		}
		actionID := "agent_turn_" + stableDigest(inboxID)
		if err := a.applyFrozenCognitiveStagesTx(ctx, tx, fluctlightID, inboxID, decision, actionType, actionID, expectedState); err != nil {
			return err
		}
		if err := persistClaimsTx(ctx, tx, fluctlightID, inboxID, responsePlan); err != nil {
			return err
		}
		assessmentID := "assessment_" + stableDigest(inboxID)
		decisionID := "decision_" + stableDigest(inboxID)
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_assessments(id,inbox_id,fluctlight_id,payload,schema_version,model,model_version,prompt_version,correlation_id) VALUES($1,$2,$3,$4,'structured-turn.v2','configured','configured','formal-agent.v1',$5) ON CONFLICT DO NOTHING`, assessmentID, inboxID, fluctlightID, jsonBytes(decision), "turn:"+turnID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_decision_proposals(id,assessment_id,fluctlight_id,action_type,payload,confidence,evidence_refs) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, decisionID, assessmentID, fluctlightID, actionType, jsonBytes(decision), jsonString(decision["confidence"]), jsonBytes(arrayValue(decision["evidence_refs"]))); err != nil {
			return err
		}
		messageIDs := make([]any, 0)
		for _, reply := range committedConversationReplyResults(outcome.Results) {
			if id := strings.TrimSpace(stringValue(mapValue(reply.Output)["target_ref"])); id != "" {
				messageIDs = append(messageIDs, id)
			}
		}
		if len(messageIDs) == 0 {
			if id := strings.TrimSpace(stringValue(assistant["id"])); id != "" {
				messageIDs = append(messageIDs, id)
			}
		}
		resultPayload := map[string]any{
			"message_id": stringValue(assistant["id"]), "message_ids": messageIDs, "media_intent_id": mediaIntentID,
			"capability_invocations": outcome.Invocations, "capability_results": outcome.Results,
		}
		command, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now(),claimed_by=NULL,claimed_at=NULL,error_code=NULL,payload=jsonb_set(payload,'{agent_result}',$2::jsonb,true) WHERE id=$1 AND status IN ('pending','claimed')`, inboxID, jsonBytes(resultPayload))
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads h SET last_processed_sequence=GREATEST(h.last_processed_sequence,i.sequence) FROM public.cognition_inbox i WHERE h.fluctlight_id=i.fluctlight_id AND i.id=$1`, inboxID); err != nil {
			return err
		}
		if err := a.recordRelationshipInteractionTx(ctx, tx, fluctlightID, actorID, len(mapValue(decision["appraisal"])) > 0, replyProfileFromOutcome(outcome, projection)); err != nil {
			return err
		}
		if err := enqueueQuietPeriodReflectionIntentTx(ctx, tx, fluctlightID, inboxID, a.now().UTC().Add(a.reflectionDelay(ctx)), "conversation_quiet_period"); err != nil {
			return err
		}
		outcomes, err := buildActionOutcomes(actionID, fluctlightID, inboxID, actionType, outcome.Results, resultPayload, a.capabilityRegistry())
		if err != nil {
			return err
		}
		if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
			return err
		}
		_, err = appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", map[string]any{"action_id": actionID, "source_fact_id": inboxID, "result": resultPayload, "outcomes": outcomes}, "action-result:"+actionID)
		return err
	})
}

func (a *App) agentSettlementAuthorityRevisionsTx(ctx context.Context, tx pgx.Tx, fluctlightID string, projection ContextProjection, outcome agentCommittedOutcome) (int, int, string, error) {
	foundation, currentState, lifeContext := projection.ContextRevision, projection.CurrentStateRevision, projection.LifeContextRevision
	resultByCall := make(map[string]CapabilityResult, len(outcome.Results))
	for _, result := range outcome.Results {
		resultByCall[strings.TrimSpace(result.CallID)] = result
	}
	for _, invocation := range outcome.Invocations {
		result, found := resultByCall[strings.TrimSpace(invocation.CallID)]
		if !found || (result.Status != "completed" && result.Status != "accepted") {
			continue
		}
		definition, registered := a.capabilityRegistry().Definition(invocation.CapabilityName)
		if !registered {
			return 0, 0, "", ErrCapabilityNotFound
		}
		if definition.Type == CapabilityTypeQuery && definition.SideEffectClass == "read_only" {
			continue
		}
		operationID := strings.TrimSpace(invocation.Metadata.OperationID)
		if operationID == "" {
			return 0, 0, "", errors.New("agent_tool_operation_identity_missing")
		}
		var encoded []byte
		if err := tx.QueryRow(ctx, `SELECT invocation->'authority_revisions' FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name=$2 AND operation_id=$3`, fluctlightID, invocation.CapabilityName, operationID).Scan(&encoded); err != nil {
			return 0, 0, "", err
		}
		var authority ToolAuthorityRevisions
		if err := json.Unmarshal(encoded, &authority); err != nil || strings.TrimSpace(authority.LifeContext) == "" {
			return 0, 0, "", errors.New("agent_tool_authority_receipt_invalid")
		}
		if authority.Before == nil {
			return 0, 0, "", errors.New("agent_tool_authority_receipt_invalid")
		}
		if err := compareCognitionAuthorityRevisions(foundation, currentState, lifeContext, authority.Before.Foundation, authority.Before.CurrentState, authority.Before.LifeContext); err != nil {
			return 0, 0, "", err
		}
		foundation, currentState, lifeContext = authority.Foundation, authority.CurrentState, authority.LifeContext
	}
	return foundation, currentState, lifeContext, nil
}

// ProcessWakeUp runs the registered Wake-up Agent and records the already
// committed Tool outcome. It never creates a second autonomy/capability action
// that could replay the Agent's ToolCalls.
func (a *App) ProcessWakeUp(ctx context.Context, fluctlightID string, cycle int) (map[string]any, error) {
	if strings.TrimSpace(fluctlightID) == "" {
		return nil, errors.New("wake_up_fluctlight_id_required")
	}
	if cycle < 0 {
		return nil, errors.New("wake_up_cycle_invalid")
	}
	correlationID := wakeUpCycleCorrelation(fluctlightID, cycle)
	cancelled := func() map[string]any {
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": "cancelled", "reason": "superseded_by_cognition"}
	}
	if a.lifecycleCancellationRequested(ctx, WakeUpProviderCancellationMarker(fluctlightID, cycle)) {
		return cancelled(), nil
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
	if fluctlight.Status != "active" {
		reason, status := "fluctlight_not_active", "inactive"
		if fluctlight.Status == "paused" {
			reason, status = "fluctlight_paused", "paused"
		}
		nextDue, err := a.ensureWakeUpNextDue(ctx, fluctlightID, cycle, settings.IntervalSeconds, reason)
		if err != nil {
			return nil, err
		}
		a.scheduleWakeUpHint(ctx, fluctlightID, cycle, nextDue)
		return map[string]any{"fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": status, "reason": reason, "interval_seconds": settings.IntervalSeconds, "next_due_at": nextDue.Format(time.RFC3339Nano)}, nil
	}
	ctx = WithProviderExecutionGuard(ctx, a.providerGuardForFluctlight(fluctlightID))
	wakeID := "wake_up_" + stableDigest(fluctlightID+":"+jsonString(cycle))
	var replayStatus, replayAction string
	var replayResult []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,action_type,result FROM public.cognition_wakeups WHERE id=$1`, wakeID).Scan(&replayStatus, &replayAction, &replayResult); err == nil {
		nextDue, dueErr := a.ensureWakeUpNextDue(ctx, fluctlightID, cycle, settings.IntervalSeconds, "wake_up_replay_repaired")
		if dueErr != nil {
			return nil, dueErr
		}
		a.scheduleWakeUpHint(ctx, fluctlightID, cycle, nextDue)
		return map[string]any{"wake_up_id": wakeID, "fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": replayStatus, "reason": "wake_up_replayed", "action_type": replayAction, "result": decodeObject(replayResult), "interval_seconds": settings.IntervalSeconds, "next_due_at": nextDue.Format(time.RFC3339Nano)}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var ownerID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerID); err != nil {
		return nil, err
	}
	conversationID, err := a.EnsureDirectConversation(ctx, ownerID, fluctlightID)
	if err != nil {
		return nil, err
	}
	projection, err := a.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		ConversationID: conversationID, SourceFactID: wakeID,
		MemoryOperation: MemoryForWakeUp, MemoryConversationMode: MemoryConversationExact,
	})
	if err != nil {
		return nil, err
	}
	if active, identityErr := a.hasActiveVisualIdentity(ctx, fluctlightID); identityErr != nil {
		return nil, identityErr
	} else if !active {
		if projection.VisualIdentity == nil {
			projection.VisualIdentity = map[string]any{}
		}
		projection.VisualIdentity["missing"] = true
	}
	policy, err := a.EvaluateAutonomyPolicy(ctx, fluctlightID, "capability", a.now().UTC())
	if err != nil {
		return nil, err
	}
	_ = policy // Effect authorization is rechecked atomically by each Tool; internal cognition can continue.
	definitions := capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceWakeUp)
	schema := wakeUpResponseSchema()
	assembly, assembledProjection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceWakeUp, projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityWakeUpPolicyInstruction}, jsonString(map[string]any{"wake_up_id": wakeID, "cycle": cycle, "schedule_status": wakeUpScheduleStatus(projection.Schedule)}), definitions, "wake_up_response", schema)
	if err != nil {
		return nil, err
	}
	projection = assembledProjection
	providerCtx := WithPromptDiagnostics(WithProviderCancellationKey(WithProviderCorrelation(WithProviderScenario(ctx, "wake_up"), correlationID), WakeUpProviderCancellationMarker(fluctlightID, cycle)), assembly.Diagnostics)
	run, runErr := a.RunFormalAgent(providerCtx, FormalAgentWakeUp, FormalAgentRunInput{
		Prompt: PromptAssemblyResult{Messages: assembly.Messages, ResponseFormat: schema}, Definitions: definitions,
		SchemaName: "wake_up_response", EnableThinking: structuredThinkingEnabledForSchema("wake_up_response"),
		Capability: &ADKCapabilityRequest{
			AuthorizationPolicy:  "autonomy",
			AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
			SourceFactID: wakeID, ActionID: "agent_wake_" + stableDigest(wakeID), OperationID: "wake:" + wakeID,
			CorrelationID: correlationID, Surface: CapabilitySurfaceWakeUp, Projection: projection,
		},
	})
	outcome, outcomeErr := committedAgentOutcome(run.Trace)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	if runErr != nil {
		// Tool facts remain committed. Record the failed Agent cycle so a retry
		// does not replay the complete run.
		_, persistErr := a.persistCommittedWakeUp(ctx, wakeID, fluctlightID, cycle, settings.IntervalSeconds, projection, map[string]any{"action_type": "no_op", "response_intent": "agent_failed", "influences": []any{}}, outcome, conversationID, "failed", "agent_run_failed")
		if persistErr != nil {
			return nil, persistErr
		}
		return nil, runErr
	}
	assessment, err := normalizeWakeUpAssessment(run.Completion.Structured)
	if err != nil {
		return nil, err
	}
	if _, err := freezeDecisionInfluences(assessment, projection, false); err != nil {
		return nil, err
	}
	status, reason := "no_op", "no_action_selected"
	if len(outcome.Results) > 0 {
		status, reason = "completed", "agent_tools_committed"
	}
	if a.lifecycleCancellationRequested(ctx, WakeUpProviderCancellationMarker(fluctlightID, cycle)) {
		status, reason = "cancelled", "superseded_by_cognition"
	}
	return a.persistCommittedWakeUp(ctx, wakeID, fluctlightID, cycle, settings.IntervalSeconds, projection, assessment, outcome, conversationID, status, reason)
}

func (a *App) persistCommittedWakeUp(ctx context.Context, wakeID, fluctlightID string, cycle, intervalSeconds int, projection ContextProjection, assessment map[string]any, outcome agentCommittedOutcome, conversationID, status, reason string) (map[string]any, error) {
	correlationID := wakeUpCycleCorrelation(fluctlightID, cycle)
	actionType := normalizeConversationActionType(stringValue(assessment["action_type"]))
	if len(outcome.Results) > 0 {
		actionType = "capability"
		for _, result := range outcome.Results {
			if result.CapabilityName == "conversation.reply" && result.Status == "completed" {
				actionType = "proactive_message"
			}
			if result.CapabilityName == "moment.publish" && result.Status == "completed" {
				actionType = "moment"
			}
		}
	}
	if actionType == "reply" || actionType == "" {
		actionType = "no_op"
	}
	reflectionIntentID := "reflection_intent:wake:" + wakeID
	factID := "wake_fact_" + stableDigest(wakeID)
	result := map[string]any{"status": status, "reason": reason, "conversation_id": conversationID, "capability_invocations": outcome.Invocations, "capability_results": outcome.Results}
	var nextDue time.Time
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if status != "failed" && status != "cancelled" {
			foundation, currentState, lifeContext, err := a.agentSettlementAuthorityRevisionsTx(ctx, tx, fluctlightID, projection, outcome)
			if err != nil {
				return err
			}
			if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, foundation, currentState, lifeContext, time.Now().UTC()); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fluctlightID); err != nil {
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
		payload := map[string]any{"event_type": "internal.wake_up", "wake_up_id": wakeID, "cycle": cycle, "action_type": actionType, "correlation_id": correlationID}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES($1,$2,$3,'internal.wake_up',$4,$5,$6,$7,now(),'processed',now()) ON CONFLICT DO NOTHING`, factID, fluctlightID, sequence, jsonBytes(payload), wakeID, correlationID, wakeID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_wakeups(id,fluctlight_id,cycle,internal_dynamics,attention,thought,desire,agency,action_type,action_id,result,reflection_intent_id,status) VALUES($1,$2,$3,$4,'{}','{}','{}','{}',$5,NULL,$6,$7,$8) ON CONFLICT DO NOTHING`, wakeID, fluctlightID, cycle, jsonBytes(projection.InnerState), actionType, jsonBytes(result), reflectionIntentID, status); err != nil {
			return err
		}
		if err := insertReflectionIntentWithDelayTx(ctx, tx, reflectionIntentID, "reflection:wake:"+wakeID, map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "wake_up_id": wakeID, "correlation_id": correlationID, "causation_id": factID}, reflectionQuietPeriod); err != nil {
			return err
		}
		actionID := "agent_wake_" + stableDigest(wakeID)
		outcomes, err := buildActionOutcomes(actionID, fluctlightID, factID, actionType, outcome.Results, result, a.capabilityRegistry())
		if err != nil {
			return err
		}
		if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
			return err
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
	if err != nil {
		return nil, err
	}
	a.scheduleWakeUpHint(ctx, fluctlightID, cycle, nextDue)
	_ = a.scheduleReflectionTrigger(ctx, fluctlightID, reflectionQuietPeriod)
	return map[string]any{"wake_up_id": wakeID, "fluctlight_id": fluctlightID, "cycle": cycle, "correlation_id": correlationID, "status": status, "reason": reason, "action_type": actionType, "reflection_intent_id": reflectionIntentID, "result": result, "interval_seconds": intervalSeconds, "next_due_at": nextDue.Format(time.RFC3339Nano)}, nil
}

// ProcessNativeCognitionFact consumes the final contract and the Tool trace
// produced by the registered native-cognition Agent. No ToolCall is prepared
// or executed again after the Agent returns.
// bindIntentionDueFact replaces the pre-transition reference with the exact
// due revision visible in this Agent projection. The durable fact remains the
// audit source; neither a Provider-supplied ref nor a guessed token is trusted.
func (a *App) bindIntentionDueFact(ctx context.Context, fluctlightID string, payload []byte, projection ContextProjection) ([]byte, map[string]any, error) {
	fact := decodeObject(payload)
	candidate := mapValue(fact["candidate"])
	intentionID := strings.TrimSpace(stringValue(fact["source_fact_id"]))
	attemptID := strings.TrimSpace(stringValue(candidate["attempt_id"]))
	revision := intValue(candidate["intention_revision"])
	if intentionID == "" || attemptID == "" || revision <= 0 || !validAgencyReference(stringValue(candidate["intention_ref"]), ContextReferenceIntention) || !validAgencyReference(stringValue(candidate["goal_ref"]), ContextReferenceGoal) {
		return nil, nil, errors.New("intention_due_fact_identity_invalid")
	}
	var goalID, currentAttempt, status string
	var currentRevision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(goal_id,''),COALESCE(current_attempt_id,''),status,revision FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2`, intentionID, fluctlightID).Scan(&goalID, &currentAttempt, &status, &currentRevision); err != nil {
		return nil, nil, err
	}
	if goalID == "" || currentAttempt != attemptID || status != string(IntentionDue) || currentRevision != revision {
		return nil, nil, errors.New("intention_due_fact_stale")
	}
	goalRef, intentionRef := "", ""
	for ref, entry := range projection.ReferenceIndex.ByRef {
		switch {
		case entry.Kind == ContextReferenceGoal && entry.EntityID == goalID:
			goalRef = ref
		case entry.Kind == ContextReferenceIntention && entry.EntityID == intentionID && entry.Revision == revision:
			intentionRef = ref
		}
	}
	if goalRef == "" || intentionRef == "" || goalRef != stringValue(candidate["goal_ref"]) || stringValue(decodeObject(projection.ReferenceIndex.ByRef[intentionRef].Snapshot)["goal_ref"]) != goalRef {
		return nil, nil, errors.New("intention_due_fact_reference_stale")
	}
	candidate["goal_ref"], candidate["intention_ref"] = goalRef, intentionRef
	fact["candidate"] = candidate
	return jsonBytes(fact), fact, nil
}

func (a *App) ProcessNativeCognitionFact(ctx context.Context, inboxID string) error {
	var fluctlightID, eventType, status, errorCode string
	var payload []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id,event_type,payload,status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&fluctlightID, &eventType, &payload, &status, &errorCode); err != nil {
		return err
	}
	if status == "processed" {
		return nil
	}
	if status == "failed" {
		return errors.New(firstString(errorCode, "native_cognition_failed"))
	}
	factPayload := decodeObject(payload)
	if depth := intValue(factPayload["native_cognition_depth"]); nativeCognitionCycleGuarded(depth) {
		return a.settleNativeCognitionCycleGuard(ctx, inboxID, depth)
	}
	if preemptErr := a.CancelLifecycleForCognition(ctx, fluctlightID, "cognition:"+inboxID); preemptErr != nil {
		a.recordDiagnosticEvent(ctx, "native_cognition.lifecycle_preemption.degraded", "warning", fluctlightID, inboxID, "native-cognition:"+inboxID, map[string]any{"error_code": "lifecycle_preemption_failed"})
	}
	var ownerID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerID); err != nil {
		return err
	}
	projection, err := a.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		SourceFactID: inboxID, MemoryOperation: MemoryForNativeCognition,
		MemoryConversationMode: MemoryConversationGlobalOnly,
		MemoryCues:             []MemoryQueryCue{{Kind: "native_event_type", Text: eventType}, {Kind: "native_fact", Text: jsonString(compactProviderFact(payload))}},
	})
	if err != nil {
		return err
	}
	// A due fact is enqueued before the Intention's due revision is projected.
	// Bind its persisted entity/attempt to the current, scoped reference index
	// before showing the fact to the model or checking its influences.
	if eventType == intentionDueFactType {
		payload, factPayload, err = a.bindIntentionDueFact(ctx, fluctlightID, payload, projection)
		if err != nil {
			return err
		}
		// The model may commit an activity Tool and fail before final settlement.
		// Freeze the scoped, Core-owned due references before any Tool can run.
		candidate := mapValue(factPayload["candidate"])
		goalRef, intentionRef := stringValue(candidate["goal_ref"]), stringValue(candidate["intention_ref"])
		frozen := map[string]any{"intention_id": factPayload["source_fact_id"], "attempt_id": candidate["attempt_id"],
			"intention_revision": candidate["intention_revision"], "goal_ref": goalRef, "intention_ref": intentionRef,
			"context_references": map[string]ContextReference{goalRef: projection.ReferenceIndex.ByRef[goalRef], intentionRef: projection.ReferenceIndex.ByRef[intentionRef]}}
		command, freezeErr := a.DB.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{due_context}',$2::jsonb,true) WHERE id=$1 AND fluctlight_id=$3 AND status IN ('pending','claimed') AND payload->>'source_fact_id'=$4`, inboxID, jsonBytes(frozen), fluctlightID, stringValue(factPayload["source_fact_id"]))
		if freezeErr != nil {
			return freezeErr
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
	}
	providerCtx := WithProviderCorrelation(WithProviderScenario(ctx, "native_cognition"), "native-cognition:"+inboxID)
	taskResult, runErr := a.RunNativeCognitionTask(providerCtx, NativeCognitionTaskInput{EventType: eventType, Fact: payload, Projection: projection})
	outcome, outcomeErr := committedAgentOutcome(taskResult.Trace)
	if outcomeErr != nil {
		return outcomeErr
	}
	if runErr != nil {
		_ = a.failAgentTurnAfterRun(ctx, inboxID, outcome, "native_cognition_agent_failed")
		return runErr
	}
	projection = taskResult.Projection
	stages, semanticStages, err := normalizeCognitiveStages(taskResult.Completion.Structured, len(outcome.Invocations) > 0)
	if err != nil {
		_ = a.failAgentTurnAfterRun(ctx, inboxID, outcome, "native_cognition_final_contract_invalid")
		return err
	}
	if _, err := freezeDecisionInfluences(stages, projection, false); err != nil {
		return err
	}
	causality, err := frozenDecisionCausality(stages)
	if err != nil {
		return err
	}
	if !semanticStages {
		stages["cognitive_state_transition"] = "not_proposed"
	}
	if eventType == intentionDueFactType {
		dueFact := mapValue(factPayload["candidate"])
		goalRef := strings.TrimSpace(stringValue(dueFact["goal_ref"]))
		intentionRef := strings.TrimSpace(stringValue(dueFact["intention_ref"]))
		if goalRef == "" || intentionRef == "" || !containsString(decisionServiceRefValues(stages["goal_refs"]), goalRef) || !containsString(decisionServiceRefValues(stages["intention_refs"]), intentionRef) {
			return errors.New("intention_due_service_influences_required")
		}
	}
	settlement := map[string]any{"status": "completed", "capability_invocations": outcome.Invocations, "capability_results": outcome.Results,
		"goal_refs": stages["goal_refs"], "intention_refs": stages["intention_refs"], "context_references": causality["context_references"]}
	if eventType == intentionDueFactType {
		if len(outcome.Invocations) == 0 {
			settlement["status"], settlement["reason_code"] = "suppressed", "intention_deferred"
		} else {
			// A ToolCall may have queried facts, arranged an activity, or even
			// published a reply. None of those alone prove the desired outcome.
			// The activity/result owner settles the Intention after its own
			// completion boundary; do not mark the primary ActionOutcome done.
			settlement["status"], settlement["reason_code"] = "pending", "intention_awaits_verified_result"
		}
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		foundation, currentRevision, lifeContext, err := a.agentSettlementAuthorityRevisionsTx(ctx, tx, fluctlightID, projection, outcome)
		if err != nil {
			return err
		}
		if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, foundation, currentRevision, lifeContext, time.Now().UTC()); err != nil {
			return err
		}
		actionID := "agent_native_" + stableDigest(inboxID)
		if err := a.applyFrozenCognitiveStagesTx(ctx, tx, fluctlightID, inboxID, stages, "no_op", actionID, currentRevision); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now(),claimed_by=NULL,claimed_at=NULL,error_code=NULL,payload=jsonb_set(payload,'{agent_result}',$2::jsonb,true) WHERE id=$1 AND status IN ('pending','claimed')`, inboxID, jsonBytes(settlement))
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads h SET last_processed_sequence=GREATEST(h.last_processed_sequence,i.sequence) FROM public.cognition_inbox i WHERE h.fluctlight_id=i.fluctlight_id AND i.id=$1`, inboxID); err != nil {
			return err
		}
		outcomes, err := buildActionOutcomes(actionID, fluctlightID, inboxID, "no_op", outcome.Results, settlement, a.capabilityRegistry())
		if err != nil {
			return err
		}
		allOutcomes := outcomes
		if eventType == intentionDueFactType {
			// The start Tool already committed its pending outcomes alongside its
			// activity. Keep that frozen causality, even if the activity resolved
			// while the model was producing the final contract.
			remaining := make([]ActionOutcome, 0, len(outcomes))
			for _, candidate := range outcomes {
				var existingID, existingRef string
				lookupErr := tx.QueryRow(ctx, `SELECT id,COALESCE(external_ref,'') FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id=$2`, actionID, candidate.CallID).Scan(&existingID, &existingRef)
				if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
					return lookupErr
				}
				if lookupErr == nil {
					if existingID != candidate.ID || (candidate.ExternalRef != "" && candidate.ExternalRef != existingRef) {
						return errors.New("native_due_outcome_conflict")
					}
					continue
				}
				remaining = append(remaining, candidate)
			}
			outcomes = remaining
		}
		if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
			return err
		}
		_, err = appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", map[string]any{"action_id": actionID, "source_fact_id": inboxID, "result": settlement, "outcomes": allOutcomes}, "action-result:"+actionID)
		return err
	})
	if err != nil {
		_ = a.failAgentTurnAfterRun(ctx, inboxID, outcome, "native_cognition_settlement_failed")
		return err
	}
	if followupErr := a.scheduleCognitionFollowups(ctx, fluctlightID); followupErr != nil {
		a.recordDiagnosticEvent(ctx, "native_cognition.followup.degraded", "warning", fluctlightID, inboxID, "native-cognition:"+inboxID, map[string]any{"error_code": "followup_schedule_failed"})
	}
	return nil
}

// ProcessDailyReview records the registered Agent's committed Tool results in
// the autonomy ledger. There is no follow-up autonomy.action or
// capability.action intent because those calls already ran in the Agent loop.
func (a *App) ProcessDailyReview(ctx context.Context, fluctlightID, localDate string) (map[string]any, error) {
	fluctlight, err := a.readFluctlightByID(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	if fluctlight.Status != "active" {
		return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "timezone": stringValue(fluctlight.Identity["timezone"]), "status": "inactive"}, nil
	}
	timezone := firstString(fluctlight.Identity["timezone"], "Asia/Shanghai")
	location, err := time.LoadLocation(canonicalTimezone(timezone))
	if err != nil {
		return nil, err
	}
	if localDate == "" {
		localDate = a.now().In(location).Format("2006-01-02")
	}
	release, acquired, err := a.tryDailyReviewExecutionLock(ctx, fluctlightID, localDate)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "timezone": location.String(), "status": "in_progress"}, nil
	}
	defer release()
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
	sourceFactID := "daily-review:" + fluctlightID + ":" + localDate
	projection, err := a.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		ConversationID: conversationID, SourceFactID: sourceFactID,
		MemoryOperation: MemoryForDailyReview, MemoryConversationMode: MemoryConversationExact,
	})
	if err != nil {
		return nil, err
	}
	workflowID := "go-autonomy:" + fluctlightID + ":" + localDate
	actionID := "autonomy_" + stableDigest(workflowID)
	var existingStatus, existingType string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,action_type FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&existingStatus, &existingType); err == nil {
		return map[string]any{"action_id": actionID, "action_type": existingType, "local_date": localDate, "timezone": location.String(), "status": existingStatus}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	policy, err := a.EvaluateAutonomyPolicy(ctx, fluctlightID, "capability", a.now().UTC())
	if err != nil {
		return nil, err
	}
	taskResult, runErr := a.RunDailyReviewTask(ctx, DailyReviewTaskInput{LocalDate: localDate, Projection: projection})
	outcome, outcomeErr := committedAgentOutcome(taskResult.Trace)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	decision := cloneMap(taskResult.Completion.Structured)
	if decision == nil {
		decision = map[string]any{}
	}
	if runErr == nil {
		if _, err := freezeDecisionInfluences(decision, taskResult.Projection, false); err != nil {
			return nil, err
		}
	}
	actionType := "no_op"
	status := "completed"
	if len(outcome.Results) > 0 {
		actionType = "capability"
		for _, result := range outcome.Results {
			if result.CapabilityName == "conversation.reply" && result.Status == "completed" {
				actionType = "proactive_message"
			}
			if result.CapabilityName == "moment.publish" && result.Status == "completed" {
				actionType = "moment"
			}
		}
	}
	if runErr != nil {
		status = "failed"
	}
	payload := map[string]any{
		"conversation_id": conversationID, "source_fact_id": sourceFactID,
		"decision": decision, "capability_invocations": outcome.Invocations, "capability_results": outcome.Results,
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if status != "failed" {
			foundation, currentState, lifeContext, err := a.agentSettlementAuthorityRevisionsTx(ctx, tx, fluctlightID, taskResult.Projection, outcome)
			if err != nil {
				return err
			}
			if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, foundation, currentState, lifeContext, time.Now().UTC()); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_actions(id,fluctlight_id,action_type,payload,policy_snapshot,expected_revisions,status,workflow_id,provider_request_id,created_at,settled_at,error_code) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,now(),now(),$10)`, actionID, fluctlightID, actionType, jsonBytes(payload), jsonBytes(policy.Snapshot), jsonBytes(map[string]any{"foundation_revision": taskResult.Projection.ContextRevision, "current_state_revision": taskResult.Projection.CurrentStateRevision, "life_context_revision": taskResult.Projection.LifeContextRevision}), status, workflowID, "formal_agent_daily_"+stableDigest(actionID), nullableString(map[bool]string{true: "agent_run_failed"}[runErr != nil])); err != nil {
			return err
		}
		outcomes, err := buildActionOutcomes(actionID, fluctlightID, sourceFactID, actionType, outcome.Results, payload, a.capabilityRegistry())
		if err != nil {
			return err
		}
		if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
			return err
		}
		factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", map[string]any{"action_id": actionID, "action_type": actionType, "status": status, "local_date": localDate, "capability_results": outcome.Results, "outcomes": outcomes}, "daily-review-result:"+actionID)
		if err != nil {
			return err
		}
		if err := insertReflectionIntentTx(ctx, tx, "reflection_intent:daily:"+actionID, "reflection:daily:"+actionID, map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": actionID}); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "autonomy.result.recorded", "fluctlight", fluctlightID, fluctlightID, actionID, "daily-review-result:"+actionID, "daily-review-result:"+actionID, payload)
	})
	if err != nil {
		return nil, err
	}
	if runErr != nil {
		return nil, runErr
	}
	return map[string]any{"action_id": actionID, "action_type": actionType, "local_date": localDate, "timezone": location.String(), "status": status, "owner_actor_id": ownerID, "capability_results": outcome.Results}, nil
}
