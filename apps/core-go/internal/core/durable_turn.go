package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AcceptTurn commits the user message and cognition intent without running a
// model in the API process. The Worker owns the resulting pending inbox fact.
func (a *App) AcceptTurn(ctx context.Context, actorID, conversationID string, payload map[string]any) (TurnResult, error) {
	turnActorID := actorID
	turnPayload := payload
	if sender := strings.TrimSpace(stringValue(payload["sender_actor_id"])); sender != "" && sender != actorID {
		turnActorID = sender
		turnPayload = cloneMap(payload)
		turnPayload["authorization_actor_id"] = actorID
	}
	return a.handleTurn(ctx, turnActorID, conversationID, turnPayload, turnCallbacks{acceptOnly: true}, false)
}

// StreamDurableTurn observes committed state only. Closing this HTTP request
// closes its observer; it never cancels the Worker or Provider.
func (a *App) StreamDurableTurn(ctx context.Context, writer http.ResponseWriter, actorID, conversationID string, payload map[string]any) error {
	acceptCtx, cancelAccept := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	accepted, err := a.AcceptTurn(acceptCtx, actorID, conversationID, payload)
	cancelAccept()
	if err != nil {
		return err
	}
	sequence := 0
	writeFrame := func(kind string, framePayload map[string]any) error {
		if err := json.NewEncoder(writer).Encode(map[string]any{"type": kind, "turn_id": accepted.TurnID, "sequence": sequence, "payload": framePayload}); err != nil {
			return err
		}
		sequence++
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}
	if err := writeFrame("action_result", map[string]any{"message": accepted.UserMessage, "correlation_id": accepted.CorrelationID}); err != nil {
		return err
	}
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	lastSequence := 0
	messageIDs := make([]string, 0)
	for {
		status, errorCode, err := a.readCognitionTurnStatus(ctx, accepted.InboxID)
		if err != nil {
			return err
		}
		messages, err := a.committedTurnMessages(ctx, conversationID, accepted.InboxID, lastSequence)
		if err != nil {
			return err
		}
		for _, message := range messages {
			if err := writeFrame("action_result", map[string]any{"message": message, "correlation_id": accepted.CorrelationID}); err != nil {
				return err
			}
			lastSequence = intValue(message["sequence"])
			messageIDs = append(messageIDs, stringValue(message["id"]))
		}
		if status == "processed" || (status == "failed" && (len(messageIDs) > 0 || errorCode == "superseded_by_newer_turn")) {
			return writeFrame("completed", map[string]any{"message_ids": messageIDs})
		}
		if status == "failed" {
			if errorCode == "user_cancelled" {
				return writeFrame("error", map[string]any{"status": "cancelled", "code": "user_cancelled"})
			}
			return writeFrame("error", map[string]any{"status": "failed", "code": streamTurnFailureCodeWithFallback(errors.New(errorCode), "conversation_turn_failed")})
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (a *App) readCognitionTurnStatus(ctx context.Context, inboxID string) (string, string, error) {
	var status, code string
	err := a.DB.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&status, &code)
	return status, code, err
}

func (a *App) committedTurnMessages(ctx context.Context, conversationID, inboxID string, afterSequence int) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,sequence,author_actor_id,kind,text,attachment_refs,created_at,sender_timezone,sender_utc_offset_minutes,sender_sent_at FROM public.conversation_messages WHERE conversation_id=$1 AND source_fact_id=$2 AND kind='assistant' AND sequence>$3 ORDER BY sequence`, conversationID, inboxID, afterSequence)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]map[string]any, 0)
	for rows.Next() {
		var id, author, kind, text string
		var sequence int
		var attachments []byte
		var createdAt time.Time
		var snapshot messageTime
		if err := rows.Scan(&id, &sequence, &author, &kind, &text, &attachments, &createdAt, &snapshot.zone, &snapshot.offset, &snapshot.sentAt); err != nil {
			return nil, err
		}
		message := map[string]any{"id": id, "conversation_id": conversationID, "sequence": sequence, "author_actor_id": author, "kind": kind, "text": text, "attachment_refs": decodeArray(attachments), "created_at": createdAt.UTC().Format(time.RFC3339Nano)}
		snapshot.addTo(message)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

// CancelTurn is an explicit, authorized command. A transport disconnect never
// calls it. The inbox terminal guard fences the late Worker settlement.
func (a *App) CancelTurn(ctx context.Context, actorID, conversationID, turnID string) error {
	if strings.TrimSpace(turnID) == "" {
		return ErrNotFound
	}
	var inboxID, fluctlightID, inboxStatus, workflowID, workflowStatus string
	newlyCancelled := false
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT i.id,i.fluctlight_id,i.status,COALESCE(w.workflow_id,''),COALESCE(w.status,'') FROM public.conversation_messages m JOIN public.conversation_participants p ON p.conversation_id=m.conversation_id AND p.actor_id=$1 AND p.status='active' JOIN public.cognition_inbox i ON i.id=m.source_fact_id LEFT JOIN public.platform_workflow_intents w ON w.intent_id='cognition_intent:'||i.id WHERE m.conversation_id=$2 AND m.turn_id=$3 AND m.kind='user' LIMIT 1 FOR UPDATE OF i`, actorID, conversationID, turnID).Scan(&inboxID, &fluctlightID, &inboxStatus, &workflowID, &workflowStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if inboxStatus == "processed" || inboxStatus == "failed" {
			return nil
		}
		_, err = tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='failed',error_code='user_cancelled',processed_at=now(),claimed_by=NULL,claimed_at=NULL WHERE id=$1 AND status IN ('pending','claimed')`, inboxID)
		if err != nil {
			return err
		}
		_, _ = tx.Exec(ctx, `
			UPDATE public.cognition_inbox
			SET status='failed',error_code='stale_predecessor_cancelled',claimed_by=NULL,claimed_at=NULL,processed_at=now()
			WHERE fluctlight_id=$1
			  AND sequence < (SELECT sequence FROM public.cognition_inbox WHERE id=$2)
			  AND ((status='claimed' AND claimed_at < now()-interval '10 minutes') OR (status='pending' AND attempt_count>=3))`, fluctlightID, inboxID)
		_, err = tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status=CASE WHEN status IN ('pending','retry') AND $2::bool THEN 'cancel_requested' WHEN status IN ('pending','retry') THEN 'cancelled' WHEN status IN ('started','running') THEN 'cancel_requested' ELSE status END,completed_at=CASE WHEN status IN ('pending','retry') AND NOT $2::bool THEN now() ELSE completed_at END,last_error='user_cancelled' WHERE intent_id=$1`, "cognition_intent:"+inboxID, inboxStatus == "claimed")
		newlyCancelled = err == nil
		return err
	})
	if err != nil {
		return err
	}
	if !newlyCancelled {
		return nil
	}
	firstErr := a.RequestProviderCancellation(ctx, inboxID)
	if a.Workflows != nil && (inboxStatus == "claimed" || workflowStatus == "started" || workflowStatus == "running" || workflowStatus == "cancel_requested") && workflowID != "" {
		if err := a.Workflows.Cancel(ctx, normalizedLifecycleWorkflowID(workflowID), "", "user_cancel:"+turnID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := a.scheduleCognitionFollowups(ctx, fluctlightID); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
