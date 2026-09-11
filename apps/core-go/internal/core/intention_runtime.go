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

const intentionTriggerWorkflowType = "intention.trigger"

// ProcessIntentionTrigger evaluates one typed Intention trigger against
// authoritative time or the next processed fact. It never interprets prose:
// semantic triggers only mean "ask cognition again because a new fact exists".
func (a *App) ProcessIntentionTrigger(ctx context.Context, intentionID string) (map[string]any, error) {
	intentionID = strings.TrimSpace(intentionID)
	if intentionID == "" {
		return nil, errors.New("intention_trigger_id_required")
	}
	var fluctlightID, ownerActorID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT i.fluctlight_id,f.created_by_actor_id FROM public.fluctlight_intentions i JOIN public.fluctlights f ON f.id=i.fluctlight_id WHERE i.id=$1`, intentionID).Scan(&fluctlightID, &ownerActorID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	result := map[string]any{"intention_id": intentionID, "status": "pending"}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		current, loadErr := loadIntentionAuthorityByIDTx(ctx, tx, intentionID)
		if loadErr != nil {
			return loadErr
		}
		now := time.Now().UTC()
		if !now.Before(current.Expiration) && !intentionStatusTerminal(current.Status) {
			next, record, applyErr := ApplyIntentionCommand(current, IntentionCommand{
				Operation: IntentionExpire, ExpectedRevision: current.Revision,
				EvidenceRefs: current.EvidenceRefs, Reason: "typed trigger expired", OccurredAt: now,
			})
			if applyErr != nil {
				return applyErr
			}
			if _, persistErr := persistIntentionAuthorityTx(ctx, tx, &current, next, record, "intention-expire:"+current.Ref+":"+fmt.Sprint(current.Revision)); persistErr != nil {
				return persistErr
			}
			result["status"] = "expired"
			return nil
		}
		if current.Status != IntentionQualified && current.Status != IntentionDue {
			result["status"] = string(current.Status)
		}
		return nil
	})
	if err != nil || stringValue(result["status"]) != "pending" {
		return result, err
	}
	projection, err := a.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerActorID, SpeakerActorID: ownerActorID, FluctlightID: fluctlightID,
		SourceFactID: "intention-trigger:" + intentionID, MemoryOperation: MemoryForNativeCognition,
		MemoryConversationMode: MemoryConversationGlobalOnly,
	})
	if err != nil {
		return nil, err
	}
	var intentionRef string
	var entry ContextReference
	for ref, candidate := range projection.ReferenceIndex.ByRef {
		if candidate.Kind == ContextReferenceIntention && candidate.EntityID == intentionID {
			intentionRef, entry = ref, candidate
			break
		}
	}
	if intentionRef == "" {
		return map[string]any{"intention_id": intentionID, "status": "inactive"}, nil
	}
	goalRef := strings.TrimSpace(stringValue(decodeObject(entry.Snapshot)["goal_ref"]))
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		current, loadErr := loadIntentionAuthorityTx(ctx, tx, fluctlightID, intentionRef, goalRef, entry)
		if loadErr != nil {
			return loadErr
		}
		if current.Status != IntentionQualified && current.Status != IntentionDue {
			result["status"] = string(current.Status)
			return nil
		}
		now := time.Now().UTC()
		observation := IntentionTriggerObservation{At: now}
		if current.Trigger.Type != IntentionTriggerTime {
			var cursor int
			if cursorErr := tx.QueryRow(ctx, `SELECT trigger_cursor_sequence FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, intentionID, fluctlightID).Scan(&cursor); cursorErr != nil {
				return cursorErr
			}
			var sequence int
			var eventType string
			var payload []byte
			var occurredAt time.Time
			queryErr := tx.QueryRow(ctx, `SELECT sequence,event_type,payload,occurred_at FROM public.cognition_inbox WHERE fluctlight_id=$1 AND status='processed' AND event_type<>$2 AND sequence>$3 ORDER BY sequence LIMIT 1`, fluctlightID, intentionDueFactType, cursor).Scan(&sequence, &eventType, &payload, &occurredAt)
			if errors.Is(queryErr, pgx.ErrNoRows) {
				return nil
			}
			if queryErr != nil {
				return queryErr
			}
			if command, cursorErr := tx.Exec(ctx, `UPDATE public.fluctlight_intentions SET trigger_cursor_sequence=$3 WHERE id=$1 AND fluctlight_id=$2 AND trigger_cursor_sequence=$4`, intentionID, fluctlightID, sequence, cursor); cursorErr != nil || command.RowsAffected() != 1 {
				if cursorErr == nil {
					cursorErr = errors.New("intention_trigger_cursor_conflict")
				}
				return cursorErr
			}
			fact := decodeObject(payload)
			observation.At, observation.EventType = occurredAt.UTC(), eventType
			observation.EventRef = firstString(fact["event_ref"], firstString(fact["ref"], ""))
			observation.NewFactRef = fmt.Sprintf("sequence:%d", sequence)
		}
		due, matched, dueErr := EvaluateIntentionDue(current, observation)
		if dueErr != nil {
			return dueErr
		}
		if !matched {
			return nil
		}
		next, frozenDue, inboxID, replayed, persistErr := persistIntentionDueFactTx(ctx, tx, a, current, due)
		if persistErr != nil {
			return persistErr
		}
		result = map[string]any{
			"intention_id": intentionID, "intention_ref": next.Ref, "goal_ref": next.GoalRef,
			"status": "due", "due_fact_id": frozenDue.ID, "inbox_id": inboxID, "attempt_id": frozenDue.AttemptID, "replayed": replayed,
		}
		return nil
	})
	return result, err
}

func loadIntentionAuthorityByIDTx(ctx context.Context, tx pgx.Tx, intentionID string) (IntentionAuthority, error) {
	var fluctlightID, status string
	var revision int
	var expiration time.Time
	if err := tx.QueryRow(ctx, `SELECT fluctlight_id,status,revision,expiration FROM public.fluctlight_intentions WHERE id=$1 FOR UPDATE`, intentionID).Scan(&fluctlightID, &status, &revision, &expiration); err != nil {
		return IntentionAuthority{}, err
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT snapshot FROM public.fluctlight_intention_revisions WHERE intention_id=$1 ORDER BY base_revision DESC,created_at DESC,id DESC LIMIT 1`, intentionID).Scan(&raw); err != nil {
		return IntentionAuthority{}, err
	}
	var current IntentionAuthority
	if err := json.Unmarshal(raw, &current); err != nil {
		return IntentionAuthority{}, errors.New("intention_revision_snapshot_invalid")
	}
	current.EntityID, current.FluctlightID, current.Status, current.Revision, current.Expiration = intentionID, fluctlightID, IntentionLifecycleStatus(status), revision, expiration.UTC()
	if err := current.Validate(); err != nil {
		return IntentionAuthority{}, err
	}
	return current, nil
}

func syncIntentionTriggerWorkflowTx(ctx context.Context, tx pgx.Tx, intention IntentionAuthority) error {
	prefix := "intention_trigger_intent:" + intention.EntityID + ":"
	if intention.Status != IntentionQualified {
		_, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='cancel_requested',last_error='intention_not_qualified' WHERE intent_type=$1 AND left(intent_id,length($2))=$2 AND status IN ('pending','retry','started')`, intentionTriggerWorkflowType, prefix)
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_intentions SET trigger_cursor_sequence=COALESCE((SELECT last_processed_sequence FROM public.cognition_inbox_heads WHERE fluctlight_id=$2),0) WHERE id=$1 AND fluctlight_id=$2 AND revision=$3`, intention.EntityID, intention.FluctlightID, intention.Revision); err != nil {
		return err
	}
	intentID := prefix + fmt.Sprint(intention.Revision)
	workflowID := "intention-trigger:" + intention.EntityID + ":" + fmt.Sprint(intention.Revision)
	payload := map[string]any{"intent_id": intentID, "fluctlight_id": intention.FluctlightID, "intention_id": intention.EntityID, "intention_ref": intention.Ref, "intention_revision": intention.Revision, "trigger_type": intention.Trigger.Type}
	if intention.Trigger.DueAt != nil {
		payload["due_at"] = intention.Trigger.DueAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle',$3,$4) ON CONFLICT(intent_id) DO NOTHING`, intentID, workflowID, intentionTriggerWorkflowType, jsonBytes(payload))
	return err
}
