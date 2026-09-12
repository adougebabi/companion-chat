package core

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type frozenTurn struct {
	ID         string
	InboxID    string
	ActionType string
	Payload    map[string]any
	StateRev   int
	Status     string
	ErrorCode  string
}

const maxNativeCognitionDepth = 1

// ProcessCognitionInbox is the Worker-owned entry point for a committed
// conversation fact. HandleTurn is idempotent on the inbox/message keys, so a
// retry resumes the same fact instead of consuming another turn.
func (a *App) ProcessCognitionInbox(ctx context.Context, inboxID string) (map[string]any, error) {
	if inboxID == "" {
		return nil, errors.New("cognition_inbox_id_required")
	}
	claimOwner := "go-cognition:" + randomID("worker_")
	payload, status, err := a.claimCognitionInbox(ctx, inboxID, claimOwner)
	if err != nil {
		return nil, err
	}
	if status == "processed" {
		return map[string]any{"inbox_id": inboxID, "status": "processed"}, nil
	}
	if status == "failed" {
		return map[string]any{"inbox_id": inboxID, "status": "failed"}, nil
	}
	claimSettled := false
	defer func() {
		if claimSettled {
			return
		}
		if releaseErr := a.releaseCognitionClaim(ctx, inboxID, claimOwner); releaseErr != nil {
			slog.Default().Warn("Go Core cognition claim cleanup failed", "inbox_id", inboxID, "claim_owner", claimOwner, "error", releaseErr)
		}
	}()
	data := decodeObject(payload)
	if strings.HasPrefix(stringValue(data["event_type"]), "life.") {
		if depth := intValue(data["native_cognition_depth"]); depth > maxNativeCognitionDepth {
			if err := a.settleNativeCognitionCycleGuard(ctx, inboxID, depth); err != nil {
				return nil, err
			}
			claimSettled = true
			return map[string]any{"inbox_id": inboxID, "status": "processed", "event_type": data["event_type"], "reason_code": "native_cognition_cycle_guarded"}, nil
		}
		if err := a.ProcessNativeCognitionFact(ctx, inboxID); err != nil {
			return nil, err
		}
		claimSettled = true
		return map[string]any{"inbox_id": inboxID, "status": "processed", "event_type": data["event_type"]}, nil
	}
	result, err := a.HandleTurn(ctx, stringValue(data["actor_id"]), stringValue(data["conversation_id"]), data)
	if err != nil {
		return nil, err
	}
	if err := a.releaseCognitionClaim(ctx, inboxID, claimOwner); err != nil {
		return nil, err
	}
	claimSettled = true
	return map[string]any{"inbox_id": inboxID, "status": "processed", "turn_id": result.TurnID, "assistant_message_id": stringValue(result.Assistant["id"]), "media_intent_id": result.MediaIntentID}, nil
}

func (a *App) settleNativeCognitionCycleGuard(ctx context.Context, inboxID string, depth int) error {
	reflectionDelay := a.reflectionDelay(ctx)
	nextReflectionAt := time.Now().UTC().Add(reflectionDelay)
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var fluctlightID string
		var sequence int
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id,sequence FROM public.cognition_inbox WHERE id=$1 FOR UPDATE`, inboxID).Scan(&fluctlightID, &sequence); err != nil {
			return err
		}
		guard := map[string]any{"status": "guarded", "reason_code": "native_cognition_cycle_guarded", "depth": depth, "max_depth": maxNativeCognitionDepth}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now(),claimed_by=NULL,claimed_at=NULL,payload=jsonb_set(payload,'{cycle_guard}',$2::jsonb,true) WHERE id=$1`, inboxID, jsonBytes(guard)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads SET last_processed_sequence=GREATEST(last_processed_sequence,$2) WHERE fluctlight_id=$1`, fluctlightID, sequence); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at) VALUES($1,$2,'lifecycle','reflection.run',$3,$4) ON CONFLICT DO NOTHING`, "reflection_intent:"+inboxID, "reflection:"+inboxID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": inboxID, "reason_code": "native_cognition_cycle_guarded"}), nextReflectionAt)
		return err
	})
	if err == nil {
		a.scheduleReflectionTrigger(ctx, "reflection_intent:"+inboxID, reflectionDelay)
	}
	return err
}

func (a *App) claimCognitionInbox(ctx context.Context, inboxID, claimOwner string) ([]byte, string, error) {
	if claimOwner == "" {
		claimOwner = "go-cognition:" + randomID("worker_")
	}
	var payload []byte
	var fluctlightID string
	var sequence int
	var status, claimedBy string
	var claimedAt *time.Time
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id,sequence,payload,status,COALESCE(claimed_by,''),claimed_at FROM public.cognition_inbox WHERE id=$1 FOR UPDATE`, inboxID).Scan(&fluctlightID, &sequence, &payload, &status, &claimedBy, &claimedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status == "processed" || status == "failed" {
			return nil
		}
		if status == "claimed" && claimedBy != "" && claimedAt != nil && time.Since(*claimedAt) < 10*time.Minute {
			return ErrConflict
		}
		// Ordering is enforced by the durable pending/claimed predecessor
		// check below. Do not reject a recoverable turn merely because the head's
		// last_processed_sequence was not advanced by an older synchronous
		// stream request or by a superseded turn; that would strand a frozen
		// action after the request process dies before message settlement.
		var earlierPending bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_inbox WHERE fluctlight_id=$1 AND sequence<$2 AND status IN ('pending','claimed'))`, fluctlightID, sequence).Scan(&earlierPending); err != nil {
			return err
		}
		if earlierPending {
			return ErrConflict
		}
		_, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='claimed',claimed_by=$2,claimed_at=now(),attempt_count=attempt_count+1 WHERE id=$1`, inboxID, claimOwner)
		return err
	})
	return payload, status, err
}

// releaseCognitionClaim makes a failed claim available to the durable retry
// path. It is conditional on the lease owner so a late cleanup from an older
// request cannot clear a newer worker's claim. If an assistant message already
// exists, it is only considered settled when the canonical frozen action has
// also reached completed. An assistant row with a still-frozen action is a
// crash window and must remain retryable.
func (a *App) releaseCognitionClaim(ctx context.Context, inboxID, claimOwner string) error {
	if inboxID == "" || claimOwner == "" {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := a.DB.Pool().Exec(cleanupCtx, `
		UPDATE public.cognition_inbox AS i
		SET status = CASE
			WHEN EXISTS (SELECT 1 FROM public.cognition_frozen_actions f WHERE f.inbox_id=i.id AND f.status='failed') THEN 'failed'
			WHEN EXISTS (
			SELECT 1 FROM public.cognition_frozen_actions f
			WHERE f.inbox_id=i.id AND f.status='completed'
			  AND (f.action_type='no_op' OR EXISTS (
				SELECT 1 FROM public.conversation_messages AS m
				WHERE m.conversation_id = i.payload->>'conversation_id'
				  AND m.idempotency_key = 'assistant:' || (i.payload->>'turn_id')
			  ))
			) THEN 'processed' ELSE 'pending' END,
			claimed_by = NULL,
			claimed_at = NULL,
			processed_at = CASE WHEN (
				EXISTS (
					SELECT 1
					FROM public.conversation_messages AS m
					WHERE m.conversation_id = i.payload->>'conversation_id'
					  AND m.idempotency_key = 'assistant:' || (i.payload->>'turn_id')
				)
				AND EXISTS (SELECT 1 FROM public.cognition_frozen_actions f WHERE f.inbox_id=i.id AND f.status='completed')
			) OR EXISTS (SELECT 1 FROM public.cognition_frozen_actions f WHERE f.inbox_id=i.id AND f.status='completed' AND f.action_type='no_op') THEN COALESCE(i.processed_at, now()) ELSE NULL END,
			error_code = CASE WHEN EXISTS (SELECT 1 FROM public.cognition_frozen_actions f WHERE f.inbox_id=i.id AND f.status='failed') THEN (SELECT error_code FROM public.cognition_frozen_actions f WHERE f.inbox_id=i.id AND f.status='failed' ORDER BY f.frozen_at DESC LIMIT 1) ELSE NULL END
		WHERE i.id = $1
		  AND i.status = 'claimed'
		  AND i.claimed_by = $2`, inboxID, claimOwner)
	return err
}

// EnqueueTurnFact records the source observation before any model call. The
// per-Fluctlight sequence and idempotency key are durable, so a client retry
// cannot create a second fact or reorder another Fluctlight's work.
func (a *App) EnqueueTurnFact(ctx context.Context, actorID, fluctlightID, conversationID, turnID, idempotency, text string, attachmentRefs any) (string, error) {
	return a.enqueueTurnFact(ctx, actorID, fluctlightID, conversationID, turnID, idempotency, text, attachmentRefs, "")
}

// EnqueueTurnFactClaimed is used by the synchronous NDJSON turn path. It
// claims the inbox row in the same transaction that creates it, so the
// background cognition Worker cannot start the same turn concurrently. If the
// HTTP process dies, the claim expires and the Worker can reclaim the fact.
func (a *App) EnqueueTurnFactClaimed(ctx context.Context, actorID, fluctlightID, conversationID, turnID, idempotency, text string, attachmentRefs any) (string, error) {
	inboxID, _, err := a.enqueueTurnFactClaimed(ctx, actorID, fluctlightID, conversationID, turnID, idempotency, text, attachmentRefs)
	return inboxID, err
}

func (a *App) enqueueTurnFactClaimed(ctx context.Context, actorID, fluctlightID, conversationID, turnID, idempotency, text string, attachmentRefs any) (string, string, error) {
	claimOwner := "go-stream:" + randomID("claim_")
	inboxID, err := a.enqueueTurnFact(ctx, actorID, fluctlightID, conversationID, turnID, idempotency, text, attachmentRefs, claimOwner)
	return inboxID, claimOwner, err
}

func (a *App) enqueueTurnFact(ctx context.Context, actorID, fluctlightID, conversationID, turnID, idempotency, text string, attachmentRefs any, claimOwner string) (string, error) {
	var inboxID string
	var supersededIDs []string
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var enqueueErr error
		inboxID, supersededIDs, enqueueErr = a.enqueueTurnFactTx(ctx, tx, actorID, fluctlightID, conversationID, turnID, idempotency, text, attachmentRefs, claimOwner)
		return enqueueErr
	})
	if err == nil {
		a.cancelSupersededCognitionFacts(ctx, supersededIDs)
	}
	return inboxID, err
}

// enqueueTurnFactTx is the transaction-injected authority for a conversation
// observation. Interactive callers compose the source message and this fact in
// one short transaction; background/public wrappers still own a transaction.
func (a *App) enqueueTurnFactTx(ctx context.Context, tx pgx.Tx, actorID, fluctlightID, conversationID, turnID, idempotency, text string, attachmentRefs any, claimOwner string) (string, []string, error) {
	inboxID := "inbox_" + stableDigest("turn:"+idempotency)
	supersededIDs := make([]string, 0)
	var existing, existingText, existingStatus, existingClaimedBy string
	var existingPayload []byte
	var existingClaimedAt *time.Time
	if err := tx.QueryRow(ctx, `SELECT id,payload,status,COALESCE(claimed_by,''),claimed_at FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key=$2 FOR UPDATE`, fluctlightID, idempotency).Scan(&existing, &existingPayload, &existingStatus, &existingClaimedBy, &existingClaimedAt); err == nil {
		existingData := decodeObject(existingPayload)
		existingText = stringValue(existingData["text"])
		if existingText != text || stringValue(existingData["conversation_id"]) != conversationID || stringValue(existingData["actor_id"]) != actorID {
			return "", nil, ErrConflict
		}
		if claimOwner != "" && existingStatus != "processed" && existingStatus != "failed" {
			if existingStatus == "claimed" && existingClaimedBy != "" && existingClaimedAt != nil && time.Since(*existingClaimedAt) < 10*time.Minute {
				var assistantExists bool
				var frozenStatus, frozenActionType string
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2),COALESCE((SELECT status FROM public.cognition_frozen_actions WHERE inbox_id=$3 ORDER BY frozen_at DESC LIMIT 1),''),COALESCE((SELECT action_type FROM public.cognition_frozen_actions WHERE inbox_id=$3 ORDER BY frozen_at DESC LIMIT 1),'')`, stringValue(existingData["conversation_id"]), "assistant:"+stringValue(existingData["turn_id"]), existing).Scan(&assistantExists, &frozenStatus, &frozenActionType); err != nil {
					return "", nil, err
				}
				if (assistantExists && (frozenStatus == "" || frozenStatus == "completed")) || (!assistantExists && frozenActionType == "no_op" && frozenStatus == "completed") {
					if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',claimed_by=NULL,claimed_at=NULL,processed_at=COALESCE(processed_at,now()),error_code=NULL WHERE id=$1 AND status='claimed'`, existing); err != nil {
						return "", nil, err
					}
				} else if assistantExists && frozenStatus == "frozen" {
					// The assistant transaction committed before cognition completion.
					// Transfer the lease so the normal recovery path can settle the same
					// frozen invocations/results and complete the inbox idempotently.
					if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET claimed_by=$2,claimed_at=now() WHERE id=$1 AND status='claimed'`, existing, claimOwner); err != nil {
						return "", nil, err
					}
				} else {
					return "", nil, ErrConflict
				}
			} else if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='claimed',claimed_by=$2,claimed_at=now() WHERE id=$1`, existing, claimOwner); err != nil {
				return "", nil, err
			}
		}
		return existing, supersededIDs, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", nil, err
	}
	rows, err := tx.Query(ctx, `
		UPDATE public.cognition_inbox
		SET status='failed',error_code='superseded_by_newer_turn',processed_at=now(),claimed_by=NULL,claimed_at=NULL
		WHERE fluctlight_id=$1 AND event_type='conversation.turn'
		  AND payload->>'conversation_id'=$2 AND status IN ('pending','claimed')
		RETURNING id`, fluctlightID, conversationID)
	if err != nil {
		return "", nil, err
	}
	for rows.Next() {
		var oldID string
		if err := rows.Scan(&oldID); err != nil {
			rows.Close()
			return "", nil, err
		}
		supersededIDs = append(supersededIDs, oldID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", nil, err
	}
	rows.Close()
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,1,0) ON CONFLICT DO NOTHING`, fluctlightID); err != nil {
		return "", nil, err
	}
	var sequence int
	if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.cognition_inbox_heads WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&sequence); err != nil {
		return "", nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads SET next_sequence=$2 WHERE fluctlight_id=$1`, fluctlightID, sequence+1); err != nil {
		return "", nil, err
	}
	if attachmentRefs == nil {
		attachmentRefs = []any{}
	}
	payload := map[string]any{"actor_id": actorID, "fluctlight_id": fluctlightID, "conversation_id": conversationID, "turn_id": turnID, "text": text, "attachment_refs": attachmentRefs, "idempotency_key": idempotency}
	status := "pending"
	claimedBy := nullableString("")
	var claimedAt any
	if claimOwner != "" {
		status = "claimed"
		claimedBy = claimOwner
		claimedAt = time.Now().UTC()
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,claimed_by,claimed_at) VALUES($1,$2,$3,'conversation.turn',$4,$5,$6,$7,now(),$8,$9,$10)`, inboxID, fluctlightID, sequence, jsonBytes(payload), turnID, "turn:"+turnID, idempotency, status, claimedBy, claimedAt); err != nil {
		return "", nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','cognition.processing',$3) ON CONFLICT DO NOTHING`, "cognition_intent:"+inboxID, "cognition:"+inboxID, jsonBytes(map[string]any{"inbox_id": inboxID, "fluctlight_id": fluctlightID, "conversation_id": conversationID, "turn_id": turnID, "idempotency_key": idempotency})); err != nil {
		return "", nil, err
	}
	if err := appendOutboxTx(ctx, tx, "cognition.fact.created", "fluctlight", fluctlightID, fluctlightID, turnID, "turn:"+turnID, "cognition:"+idempotency, payload); err != nil {
		return "", nil, err
	}
	return inboxID, supersededIDs, nil
}

func (a *App) cancelSupersededCognitionFacts(ctx context.Context, inboxIDs []string) {
	for _, inboxID := range inboxIDs {
		a.cancelCognitionFact(ctx, inboxID)
	}
}

func (a *App) cancelCognitionFact(ctx context.Context, inboxID string) {
	if a == nil || a.Redis == nil || strings.TrimSpace(inboxID) == "" {
		return
	}
	_ = a.Redis.Set(ctx, providerCognitionCancelPrefix+inboxID, "1", 10*time.Minute).Err()
}

func (a *App) cognitionFactSuperseded(ctx context.Context, inboxID string) bool {
	if a == nil || a.DB == nil || strings.TrimSpace(inboxID) == "" {
		return false
	}
	var status, errorCode string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&status, &errorCode); err != nil {
		return false
	}
	return status == "failed" && errorCode == "superseded_by_newer_turn"
}

func (a *App) LoadFrozenTurn(ctx context.Context, inboxID string) (frozenTurn, bool, error) {
	var result frozenTurn
	var payload []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,inbox_id,action_type,payload,state_revision,status,COALESCE(error_code,'') FROM public.cognition_frozen_actions WHERE inbox_id=$1 ORDER BY frozen_at DESC LIMIT 1`, inboxID).Scan(&result.ID, &result.InboxID, &result.ActionType, &payload, &result.StateRev, &result.Status, &result.ErrorCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return frozenTurn{}, false, nil
	}
	if err != nil {
		return frozenTurn{}, false, err
	}
	result.Payload = decodeObject(payload)
	if result.Status == "frozen" {
		if err := validateExecutableCapabilityPayload(result.Payload); err != nil {
			return frozenTurn{}, false, err
		}
	}
	return result, true, nil
}

func validateExecutableCapabilityPayload(payload map[string]any) error {
	if stringValue(payload["capability_runtime_version"]) != CapabilityRuntimePayloadVersion {
		return errors.New("capability_runtime_version_invalid")
	}
	if _, exists := payload["tool_calls"]; exists {
		return errors.New("capability_runtime_dual_authority")
	}
	if _, exists := payload["tool_results"]; exists {
		return errors.New("capability_runtime_dual_authority")
	}
	if decision := mapValue(payload["decision"]); len(decision) > 0 {
		if _, exists := decision["tool_calls"]; exists {
			return errors.New("capability_runtime_dual_authority")
		}
	}
	for _, key := range []string{"capability_invocations", "capability_results"} {
		if _, exists := payload[key]; !exists {
			return errors.New("capability_runtime_envelope_invalid")
		}
		if _, ok := payload[key].([]any); !ok {
			// Values decoded through typed tests may already be Go slices; round-trip
			// through the canonical decoder to distinguish arrays from objects/null.
			if key == "capability_invocations" {
				if _, err := capabilityInvocationsFromValue(payload[key]); err != nil {
					return errors.New("capability_runtime_envelope_invalid")
				}
			} else if _, err := capabilityResultsFromValue(payload[key]); err != nil {
				return errors.New("capability_runtime_envelope_invalid")
			}
		}
	}
	return nil
}

func (a *App) PersistTurnDecision(ctx context.Context, inboxID, fluctlightID, conversationID, turnID, action string, decision map[string]any) (frozenTurn, error) {
	if action != "reply" && action != "no_op" {
		return frozenTurn{}, errors.New("decision_effect_invalid")
	}
	assessmentID := "assessment_" + stableDigest(inboxID)
	decisionID := "decision_" + stableDigest(inboxID)
	frozenID := "frozen_" + stableDigest(inboxID)
	decisionProjection := cloneMap(decision)
	if raw, exists := decisionProjection["personality_transition"]; exists {
		plan, err := personalityDecisionPlanFromValue(raw)
		if err != nil || plan == nil || plan.FluctlightID != fluctlightID {
			return frozenTurn{}, errors.New("personality_decision_plan_invalid")
		}
	}
	delete(decisionProjection, "capability_invocations")
	delete(decisionProjection, "tool_calls")
	payload := map[string]any{"capability_runtime_version": CapabilityRuntimePayloadVersion, "turn_id": turnID, "conversation_id": conversationID, "decision": decisionProjection, "capability_invocations": []CapabilityInvocation{}, "capability_results": []CapabilityResult{}}
	if err := validateFrozenDecisionInfluences(decisionProjection); err != nil {
		return frozenTurn{}, err
	}
	if version := stringValue(decisionProjection["context_reference_version"]); version != "" {
		payload["context_reference_version"] = version
		payload["context_reference_index"] = decisionProjection["context_reference_index"]
		payload["influences"] = decisionProjection["influences"]
		payload["goal_refs"] = decisionProjection["goal_refs"]
		payload["intention_refs"] = decisionProjection["intention_refs"]
	}
	invocations, invocationErr := capabilityInvocationsFromValue(decision["capability_invocations"])
	if invocationErr != nil {
		return frozenTurn{}, invocationErr
	}
	projection, projectionOK := contextProjectionFromValue(decision["context_projection"])
	if projectionOK {
		for index := range invocations {
			invocations[index] = normalizeCapabilityInvocationMetadata(invocations[index], fluctlightID, conversationID, inboxID, inboxID, index)
			invocations[index].ActionID = frozenID
			if definition, found := a.capabilityRegistry().Definition(invocations[index].CapabilityName); found {
				invocations[index].ContextSnapshot = capabilitySnapshotForProjection(projection, definition.RequiredContext, frozenID)
			}
		}
		payload["capability_context_snapshot"] = ContextSnapshotFromProjection(projection)
	}
	for index := range invocations {
		invocations[index] = normalizeCapabilityInvocationMetadata(invocations[index], fluctlightID, conversationID, inboxID, inboxID, index)
		invocations[index].ActionID = frozenID
	}
	payload["capability_invocations"] = invocations
	result := frozenTurn{ID: frozenID, InboxID: inboxID, ActionType: action, Payload: payload, Status: "frozen"}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var stateRevision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision); err != nil {
			return err
		}
		if projectionOK && stateRevision != projection.CurrentStateRevision {
			return ErrConflict
		}
		result.StateRev = stateRevision
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_assessments(id,inbox_id,fluctlight_id,payload,schema_version,model,model_version,prompt_version,correlation_id) VALUES($1,$2,$3,$4,'structured-turn.v1','configured','configured','go-core-turn.v1',$5) ON CONFLICT DO NOTHING`, assessmentID, inboxID, fluctlightID, jsonBytes(decisionProjection), "turn:"+turnID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_decision_proposals(id,assessment_id,fluctlight_id,action_type,payload,confidence,evidence_refs) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, decisionID, assessmentID, fluctlightID, action, jsonBytes(decisionProjection), jsonString(decision["confidence"]), jsonBytes(arrayValue(decision["evidence_refs"]))); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.cognition_frozen_actions(id,decision_id,inbox_id,fluctlight_id,action_type,payload,state_revision,provider_request_id,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'frozen') ON CONFLICT DO NOTHING`, frozenID, decisionID, inboxID, fluctlightID, action, jsonBytes(payload), result.StateRev, "provider_turn_"+stableDigest(inboxID))
		return err
	})
	return result, err
}

// PersistCapabilityResults records the result of a frozen capability call before
// realization. A replay can therefore reuse the same external effect instead
// of submitting a second job when the process crashed after the plugin call.
func (a *App) PersistCapabilityResults(ctx context.Context, frozenID string, results []CapabilityResult) error {
	if frozenID == "" {
		return errors.New("frozen_action_id_required")
	}
	commandTag, err := a.DB.Pool().Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(payload,'{capability_results}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, frozenID, jsonBytes(results))
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (a *App) persistFrozenCapabilityInvocations(ctx context.Context, frozenID string, invocations []CapabilityInvocation) error {
	if frozenID == "" {
		return errors.New("frozen_action_id_required")
	}
	commandTag, err := a.DB.Pool().Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(payload,'{capability_invocations}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, frozenID, jsonBytes(invocations))
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (a *App) persistFrozenCapabilityInvocationsTx(ctx context.Context, tx pgx.Tx, frozenID string, invocations []CapabilityInvocation) error {
	if frozenID == "" {
		return errors.New("frozen_action_id_required")
	}
	commandTag, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(payload,'{capability_invocations}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, frozenID, jsonBytes(invocations))
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func capabilitySnapshotForProjection(projection ContextProjection, slots []ContextSlot, actionID string) map[string]any {
	all := ContextSnapshotFromProjection(projection)
	result := map[string]any{"identity": map[string]any{
		"fluctlight_id": projection.FluctlightID, "conversation_id": projection.ConversationID,
		"source_fact_id": projection.SourceFactID, "action_id": actionID,
	}}
	if referenceIndex, ok := all["context_reference_index"]; ok && referenceIndex != nil {
		result["context_reference_index"] = referenceIndex
	}
	for _, slot := range slots {
		if value, ok := all[string(slot)]; ok && value != nil {
			result[string(slot)] = value
		}
	}
	return result
}

func (a *App) CompleteTurnCognition(ctx context.Context, inboxID, frozenID string, realization map[string]any) error {
	reflectionDelay := a.reflectionDelay(ctx)
	nextReflectionAt := time.Now().UTC().Add(reflectionDelay)
	fluctlightID := ""
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		fluctlightID, err = a.completeTurnCognitionTx(ctx, tx, inboxID, frozenID, realization, nextReflectionAt)
		return err
	})
	if err == nil {
		a.scheduleReflectionTrigger(ctx, "reflection_intent:"+inboxID, reflectionDelay)
		a.scheduleWakeUpTrigger(ctx, fluctlightID, int(reflectionDelay/time.Second))
	}
	return err
}

func frozenResultingStateCausalityTx(ctx context.Context, tx pgx.Tx, fluctlightID, inboxID string, decision map[string]any) (map[string]any, error) {
	var revision int
	var rawState []byte
	if err := tx.QueryRow(ctx, `SELECT resulting_revision,resulting_state FROM public.fluctlight_state_revisions WHERE fluctlight_id=$1 AND source_event_id=$2 ORDER BY resulting_revision DESC LIMIT 1`, fluctlightID, inboxID).Scan(&revision, &rawState); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	result := map[string]any{"resulting_state_revision": revision}
	if strings.TrimSpace(stringValue(decision["context_reference_version"])) == "" {
		return nil, errors.New("frozen_context_reference_version_invalid")
	}
	index, err := contextReferenceIndexFromValue(decision["context_reference_index"])
	if err != nil {
		return nil, err
	}
	fullState := decodeObject(rawState)
	stateSnapshot := map[string]any{}
	for _, key := range []string{"pad", "mood", "momentum", "regulation", "drives", "conflicts", "revision", "last_updated_at", "affect_profile_revision", "affect_policy_version"} {
		if value, exists := fullState[key]; exists {
			stateSnapshot[key] = value
		}
	}
	encoded := jsonBytes(stateSnapshot)
	if len(encoded) == 0 || len(encoded) > maxContextReferenceSnapshotBytes {
		return nil, errors.New("resulting_state_reference_snapshot_invalid")
	}
	scope := index.expectedScope()
	ref := contextReferenceToken(ContextReferenceState, fluctlightID, revision, scope, encoded)
	entry := ContextReference{Ref: ref, Kind: ContextReferenceState, EntityID: fluctlightID, Revision: revision, Scope: scope, Snapshot: encoded}
	references := make(map[string]ContextReference, 1)
	references[ref] = entry
	result["resulting_state_ref"] = ref
	result["resulting_state_references"] = references
	return result, nil
}

// completeTurnCognitionTx is the one transaction-owned settlement used by the
// normal and recovery paths. Callers may compose assistant/native effects and
// claims in the same transaction; this helper never commits or triggers Redis.
func (a *App) completeTurnCognitionTx(ctx context.Context, tx pgx.Tx, inboxID, frozenID string, realization map[string]any, nextReflectionAt time.Time) (string, error) {
	var fluctlightID, actionType string
	var frozenPayload []byte
	if err := tx.QueryRow(ctx, `SELECT fluctlight_id,action_type,payload FROM public.cognition_frozen_actions WHERE id=$1 FOR UPDATE`, frozenID).Scan(&fluctlightID, &actionType, &frozenPayload); err != nil {
		return "", err
	}
	var inboxStatus, inboxError string
	if err := tx.QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$1 FOR UPDATE`, inboxID).Scan(&inboxStatus, &inboxError); err != nil {
		return "", err
	}
	if inboxStatus == "failed" && inboxError == "superseded_by_newer_turn" {
		return "", errCognitionTurnSuperseded
	}
	if inboxStatus != "pending" && inboxStatus != "claimed" {
		return "", ErrConflict
	}
	frozenObject := decodeObject(frozenPayload)
	decision := mapValue(frozenObject["decision"])
	causality, err := frozenDecisionCausality(decision)
	if err != nil {
		return "", err
	}
	resultingState, err := frozenResultingStateCausalityTx(ctx, tx, fluctlightID, inboxID, decision)
	if err != nil {
		return "", err
	}
	if resultingReferences := resultingState["resulting_state_references"]; resultingReferences != nil {
		references, referenceErr := actionOutcomeContextReferences(causality["context_references"])
		if referenceErr != nil {
			return "", referenceErr
		}
		additional, referenceErr := actionOutcomeContextReferences(resultingReferences)
		if referenceErr != nil {
			return "", referenceErr
		}
		for ref, entry := range additional {
			references[ref] = entry
		}
		causality["context_references"] = references
		delete(resultingState, "resulting_state_references")
	}
	for key, value := range resultingState {
		causality[key] = value
	}
	settledRealization := cloneMap(realization)
	for key, value := range causality {
		settledRealization[key] = value
	}
	resultingStateSnapshot := map[string]any{}
	if revision, ok := resultingState["resulting_state_revision"]; ok {
		resultingStateSnapshot["revision"] = revision
	}
	if ref := stringValue(resultingState["resulting_state_ref"]); ref != "" {
		resultingStateSnapshot["ref"] = ref
	}
	command, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET status='completed',payload=jsonb_set(payload,'{resulting_state}',$3::jsonb,true),realization_payload=$2,completed_at=now() WHERE id=$1 AND status='frozen'`, frozenID, jsonBytes(settledRealization), jsonBytes(resultingStateSnapshot))
	if err != nil {
		return "", err
	}
	if command.RowsAffected() != 1 {
		return "", ErrConflict
	}
	command, err = tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now(),claimed_by=NULL,claimed_at=NULL,error_code=NULL WHERE id=$1 AND status IN ('pending','claimed')`, inboxID)
	if err != nil {
		return "", err
	}
	if command.RowsAffected() != 1 {
		return "", ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads h SET last_processed_sequence=GREATEST(h.last_processed_sequence,i.sequence) FROM public.cognition_inbox i WHERE h.fluctlight_id=i.fluctlight_id AND i.id=$1`, inboxID); err != nil {
		return "", err
	}
	var sourceActorID string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(payload->>'actor_id','') FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&sourceActorID); err != nil {
		return "", err
	}
	if sourceActorID != "" {
		var appraisalPayload []byte
		meaningful := false
		if appraisalErr := tx.QueryRow(ctx, `SELECT payload FROM public.cognition_appraisals WHERE source_fact_id=$1`, inboxID).Scan(&appraisalPayload); appraisalErr == nil {
			meaningful = numberOrZero(mapValue(decodeObject(appraisalPayload))["relationship_significance"]) > 0
		}
		if err := a.recordRelationshipInteractionTx(ctx, tx, fluctlightID, sourceActorID, meaningful); err != nil {
			return "", err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at) VALUES($1,$2,'lifecycle','reflection.run',$3,$4) ON CONFLICT DO NOTHING`, "reflection_intent:"+inboxID, "reflection:"+inboxID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": inboxID}), nextReflectionAt); err != nil {
		return "", err
	}
	capabilityResults, err := capabilityResultsFromValue(settledRealization["capability_results"])
	if err != nil {
		return "", err
	}
	outcomes, err := buildActionOutcomes(frozenID, fluctlightID, inboxID, actionType, capabilityResults, settledRealization, a.capabilityRegistry())
	if err != nil {
		return "", err
	}
	if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
		return "", err
	}
	resultFact := map[string]any{"action_id": frozenID, "source_fact_id": inboxID, "result": settledRealization, "outcomes": outcomes}
	for key, value := range causality {
		resultFact[key] = value
	}
	if _, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", resultFact, "action-result:"+frozenID); err != nil {
		return "", err
	}
	if actionType == "reply" {
		if messageID := strings.TrimSpace(stringValue(settledRealization["message_id"])); messageID != "" {
			if err := a.enqueueConversationSummaryIntentTx(ctx, tx, fluctlightID, "", messageID); err != nil {
				return "", err
			}
		}
	}
	return fluctlightID, nil
}

func (a *App) FailTurnCognition(ctx context.Context, inboxID, frozenID, code string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var fluctlightID, actionType string
		var rawPayload []byte
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id,action_type,payload FROM public.cognition_frozen_actions WHERE id=$1 FOR UPDATE`, frozenID).Scan(&fluctlightID, &actionType, &rawPayload); err != nil {
			return err
		}
		payload := decodeObject(rawPayload)
		causality, err := frozenDecisionCausality(mapValue(payload["decision"]))
		if err != nil {
			return err
		}
		settlement := map[string]any{"status": "failed", "action_status": "failed", "error_code": code}
		for key, value := range causality {
			settlement[key] = value
		}
		capabilityResults, resultsErr := capabilityResultsFromValue(payload["capability_results"])
		if resultsErr != nil {
			capabilityResults = nil
			settlement["reason_code"] = "capability_results_invalid"
		}
		outcomes, outcomeErr := buildActionOutcomes(frozenID, fluctlightID, inboxID, actionType, capabilityResults, settlement, a.capabilityRegistry())
		if outcomeErr != nil {
			outcomes, outcomeErr = buildActionOutcomes(frozenID, fluctlightID, inboxID, actionType, nil, settlement, a.capabilityRegistry())
		}
		if outcomeErr != nil {
			return outcomeErr
		}
		if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET status='failed',error_code=$2 WHERE id=$1 AND status='frozen'`, frozenID, code)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		if _, err = tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='failed',error_code=$2,processed_at=now() WHERE id=$1 AND status <> 'processed'`, inboxID, code); err != nil {
			return err
		}
		factPayload := map[string]any{"action_id": frozenID, "source_fact_id": inboxID, "result": settlement, "outcomes": outcomes}
		for key, value := range causality {
			factPayload[key] = value
		}
		factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", factPayload, "action-result:"+frozenID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:result:"+frozenID, "reflection:result:"+frozenID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": frozenID})); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "autonomy.result.recorded", "fluctlight", fluctlightID, fluctlightID, frozenID, "action-result:"+frozenID, "action-result:"+frozenID, factPayload)
	})
}

func (a *App) CognitionFactAge(ctx context.Context, inboxID string) (time.Time, error) {
	var t time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT occurred_at FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&t)
	return t, err
}
