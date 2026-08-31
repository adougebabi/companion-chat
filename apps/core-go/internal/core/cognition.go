package core

import (
	"context"
	"errors"
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
}

// ProcessCognitionInbox is the Worker-owned entry point for a committed
// conversation fact. HandleTurn is idempotent on the inbox/message keys, so a
// retry resumes the same fact instead of consuming another turn.
func (a *App) ProcessCognitionInbox(ctx context.Context, inboxID string) (map[string]any, error) {
	if inboxID == "" {
		return nil, errors.New("cognition_inbox_id_required")
	}
	payload, status, err := a.claimCognitionInbox(ctx, inboxID)
	if err != nil {
		return nil, err
	}
	if status == "processed" {
		return map[string]any{"inbox_id": inboxID, "status": "processed"}, nil
	}
	if status == "failed" {
		return map[string]any{"inbox_id": inboxID, "status": "failed"}, nil
	}
	data := decodeObject(payload)
	if strings.HasPrefix(stringValue(data["event_type"]), "life.") {
		if err := a.markNativeFactProcessed(ctx, inboxID); err != nil {
			return nil, err
		}
		return map[string]any{"inbox_id": inboxID, "status": "processed", "event_type": data["event_type"]}, nil
	}
	result, err := a.HandleTurn(ctx, stringValue(data["actor_id"]), stringValue(data["conversation_id"]), data)
	if err != nil {
		return nil, err
	}
	return map[string]any{"inbox_id": inboxID, "status": "processed", "turn_id": result.TurnID, "assistant_message_id": stringValue(result.Assistant["id"]), "media_intent_id": result.MediaIntentID}, nil
}

func (a *App) claimCognitionInbox(ctx context.Context, inboxID string) ([]byte, string, error) {
	var payload []byte
	var status, claimedBy string
	var claimedAt *time.Time
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT payload,status,COALESCE(claimed_by,''),claimed_at FROM public.cognition_inbox WHERE id=$1 FOR UPDATE`, inboxID).Scan(&payload, &status, &claimedBy, &claimedAt); err != nil {
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
		_, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='claimed',claimed_by=$2,claimed_at=now(),attempt_count=attempt_count+1 WHERE id=$1`, inboxID, "go-cognition:"+randomID("worker_"))
		return err
	})
	return payload, status, err
}

// EnqueueTurnFact records the source observation before any model call. The
// per-Fluctlight sequence and idempotency key are durable, so a client retry
// cannot create a second fact or reorder another Fluctlight's work.
func (a *App) EnqueueTurnFact(ctx context.Context, actorID, fluctlightID, conversationID, turnID, idempotency, text string, attachmentRefs any) (string, error) {
	inboxID := "inbox_" + stableDigest("turn:"+idempotency)
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var existing, existingText string
		var existingPayload []byte
		if err := tx.QueryRow(ctx, `SELECT id,payload FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key=$2`, fluctlightID, idempotency).Scan(&existing, &existingPayload); err == nil {
			existingText = stringValue(decodeObject(existingPayload)["text"])
			if existingText != text || stringValue(decodeObject(existingPayload)["conversation_id"]) != conversationID || stringValue(decodeObject(existingPayload)["actor_id"]) != actorID {
				return ErrConflict
			}
			inboxID = existing
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
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads SET next_sequence=$2 WHERE fluctlight_id=$1`, fluctlightID, sequence+1); err != nil {
			return err
		}
		if attachmentRefs == nil {
			attachmentRefs = []any{}
		}
		payload := map[string]any{"actor_id": actorID, "fluctlight_id": fluctlightID, "conversation_id": conversationID, "turn_id": turnID, "text": text, "attachment_refs": attachmentRefs, "idempotency_key": idempotency}
		_, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,$3,'conversation.turn',$4,$5,$6,$7,now(),'pending')`, inboxID, fluctlightID, sequence, jsonBytes(payload), turnID, "turn:"+turnID, idempotency)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','cognition.processing',$3) ON CONFLICT DO NOTHING`, "cognition_intent:"+inboxID, "cognition:"+inboxID, jsonBytes(map[string]any{"inbox_id": inboxID, "fluctlight_id": fluctlightID, "conversation_id": conversationID, "turn_id": turnID, "idempotency_key": idempotency})); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "cognition.fact.created", "fluctlight", fluctlightID, fluctlightID, turnID, "turn:"+turnID, "cognition:"+idempotency, payload)
	})
	return inboxID, err
}

func (a *App) LoadFrozenTurn(ctx context.Context, inboxID string) (frozenTurn, bool, error) {
	var result frozenTurn
	var payload []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,inbox_id,action_type,payload,state_revision,status FROM public.cognition_frozen_actions WHERE inbox_id=$1 ORDER BY frozen_at DESC LIMIT 1`, inboxID).Scan(&result.ID, &result.InboxID, &result.ActionType, &payload, &result.StateRev, &result.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return frozenTurn{}, false, nil
	}
	if err != nil {
		return frozenTurn{}, false, err
	}
	result.Payload = decodeObject(payload)
	return result, true, nil
}

func (a *App) PersistTurnDecision(ctx context.Context, inboxID, fluctlightID, conversationID, turnID, action string, decision, concept map[string]any) (frozenTurn, error) {
	if action != "reply" && action != "media_request" {
		return frozenTurn{}, errors.New("decision_effect_invalid")
	}
	assessmentID := "assessment_" + stableDigest(inboxID)
	decisionID := "decision_" + stableDigest(inboxID)
	frozenID := "frozen_" + stableDigest(inboxID)
	payload := map[string]any{"turn_id": turnID, "conversation_id": conversationID, "decision": decision, "media_concept": concept}
	result := frozenTurn{ID: frozenID, InboxID: inboxID, ActionType: action, Payload: payload, Status: "frozen"}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var stateRevision int
		_ = tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision)
		result.StateRev = stateRevision
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_assessments(id,inbox_id,fluctlight_id,payload,schema_version,model,model_version,prompt_version,correlation_id) VALUES($1,$2,$3,$4,'structured-turn.v1','configured','configured','go-core-turn.v1',$5) ON CONFLICT DO NOTHING`, assessmentID, inboxID, fluctlightID, jsonBytes(decision), "turn:"+turnID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_decision_proposals(id,assessment_id,fluctlight_id,action_type,payload,confidence,evidence_refs) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, decisionID, assessmentID, fluctlightID, action, jsonBytes(decision), jsonString(decision["confidence"]), jsonBytes(arrayValue(decision["evidence_refs"]))); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.cognition_frozen_actions(id,decision_id,inbox_id,fluctlight_id,action_type,payload,state_revision,provider_request_id,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'frozen') ON CONFLICT DO NOTHING`, frozenID, decisionID, inboxID, fluctlightID, action, jsonBytes(payload), stateRevision, "provider_turn_"+stableDigest(inboxID))
		return err
	})
	return result, err
}

// PersistToolResults records the result of a frozen capability call before
// realization. A replay can therefore reuse the same external effect instead
// of submitting a second job when the process crashed after the plugin call.
func (a *App) PersistToolResults(ctx context.Context, frozenID string, results []ToolResultV1) error {
	if frozenID == "" {
		return errors.New("frozen_action_id_required")
	}
	commandTag, err := a.DB.Pool().Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(payload,'{tool_results}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, frozenID, jsonBytes(results))
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (a *App) PersistFrozenToolCalls(ctx context.Context, frozenID string, calls []ToolCallV1) error {
	if frozenID == "" {
		return errors.New("frozen_action_id_required")
	}
	commandTag, err := a.DB.Pool().Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(payload,'{decision,tool_calls}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, frozenID, jsonBytes(calls))
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (a *App) CompleteTurnCognition(ctx context.Context, inboxID, frozenID string, realization map[string]any) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var fluctlightID string
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id FROM public.cognition_frozen_actions WHERE id=$1 FOR UPDATE`, frozenID).Scan(&fluctlightID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET status='completed',realization_payload=$2,completed_at=now() WHERE id=$1 AND status='frozen'`, frozenID, jsonBytes(realization)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now() WHERE id=$1`, inboxID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:"+inboxID, "reflection:"+inboxID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": inboxID})); err != nil {
			return err
		}
		return nil
	})
}

func (a *App) FailTurnCognition(ctx context.Context, inboxID, frozenID, code string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET status='failed',error_code=$2 WHERE id=$1 AND status='frozen'`, frozenID, code); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='failed',error_code=$2,processed_at=now() WHERE id=$1 AND status <> 'processed'`, inboxID, code)
		return err
	})
}

func (a *App) CognitionFactAge(ctx context.Context, inboxID string) (time.Time, error) {
	var t time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT occurred_at FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&t)
	return t, err
}
