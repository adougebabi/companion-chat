package core

import (
	"context"
	"errors"
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

// EnqueueTurnFact records the source observation before any model call. The
// per-Fluctlight sequence and idempotency key are durable, so a client retry
// cannot create a second fact or reorder another Fluctlight's work.
func (a *App) EnqueueTurnFact(ctx context.Context, actorID, fluctlightID, conversationID, turnID, idempotency, text string) (string, error) {
	inboxID := "inbox_" + stableDigest("turn:"+idempotency)
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var existing string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key=$2`, fluctlightID, idempotency).Scan(&existing); err == nil {
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
		payload := map[string]any{"actor_id": actorID, "fluctlight_id": fluctlightID, "conversation_id": conversationID, "turn_id": turnID, "text": text}
		_, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,$3,'conversation.turn',$4,$5,$6,$7,now(),'pending')`, inboxID, fluctlightID, sequence, jsonBytes(payload), turnID, "turn:"+turnID, idempotency)
		if err != nil {
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

func (a *App) CompleteTurnCognition(ctx context.Context, inboxID, frozenID string, realization map[string]any) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_frozen_actions SET status='completed',realization_payload=$2,completed_at=now() WHERE id=$1 AND status='frozen'`, frozenID, jsonBytes(realization)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now() WHERE id=$1`, inboxID)
		return err
	})
}

func (a *App) FailTurnCognition(ctx context.Context, inboxID, frozenID, code string) error {
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.cognition_frozen_actions SET status='failed',error_code=$2 WHERE id=$1 AND status='frozen'`, frozenID, code)
	return err
}

func (a *App) CognitionFactAge(ctx context.Context, inboxID string) (time.Time, error) {
	var t time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT occurred_at FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&t)
	return t, err
}
