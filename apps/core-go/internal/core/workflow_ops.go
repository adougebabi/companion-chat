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

// ProcessAutonomyAction settles a previously frozen action exactly once. The
// action row is authoritative; retries never re-run an already settled side
// effect and failed/cancelled actions remain observable instead of being
// silently converted into success.
func (a *App) ProcessAutonomyAction(ctx context.Context, actionID string) (map[string]any, error) {
	var fluctlightID, actionType, status, workflowID, providerRequestID string
	var payload []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id,action_type,status,workflow_id,provider_request_id,payload FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&fluctlightID, &actionType, &status, &workflowID, &providerRequestID, &payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status == "completed" || status == "failed" || status == "cancelled" {
		return map[string]any{"action_id": actionID, "action_type": actionType, "status": status}, nil
	}
	data := decodeObject(payload)
	if actionType == "proactive_message" {
		conversationID := stringValue(data["conversation_id"])
		text := stringValue(data["text"])
		if conversationID == "" || text == "" {
			return a.failAutonomyAction(ctx, actionID, "proactive_target_invalid")
		}
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := appendAssistantTx(ctx, tx, conversationID, fluctlightID, text, "proactive:"+actionID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID); err != nil {
				return err
			}
			return appendOutboxTx(ctx, tx, "autonomy.action.completed", "autonomy_action", actionID, fluctlightID, actionID, "autonomy:"+actionID, "autonomy-outbox:"+actionID, map[string]any{"action_type": actionType, "status": "completed"})
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"action_id": actionID, "action_type": actionType, "status": "completed"}, nil
	}
	if actionType == "moment" {
		text := stringValue(data["text"])
		if text == "" {
			return a.failAutonomyAction(ctx, actionID, "moment_text_invalid")
		}
		momentID := "moment_" + stableDigest(actionID)
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO public.moments(id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES($1,$2,$2,$3,'participants','visible','[]') ON CONFLICT DO NOTHING`, momentID, fluctlightID, text); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID); err != nil {
				return err
			}
			return appendOutboxTx(ctx, tx, "moment.published", "moment", momentID, fluctlightID, actionID, "autonomy:"+actionID, "moment-outbox:"+actionID, map[string]any{"moment_id": momentID, "action_id": actionID})
		}); err != nil {
			return nil, err
		}
		return map[string]any{"action_id": actionID, "action_type": actionType, "status": "completed", "moment_id": momentID}, nil
	}
	if actionType == "media_request" {
		concept := mediaConceptValue(data["media_request"])
		if len(concept) == 0 {
			return a.failAutonomyAction(ctx, actionID, "media_concept_invalid")
		}
		conversationID := stringValue(data["conversation_id"])
		intentID := "media_intent_" + stableDigest(actionID)
		workflowID = "media_workflow_" + stableDigest(actionID)
		providerRequestID = "media_request_" + stableDigest(actionID)
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,conversation_id,status,revision) VALUES($1,$2,'image','image/png',$3,$4,$5,$6,'pending',0) ON CONFLICT DO NOTHING`, intentID, fluctlightID, jsonString(concept), providerRequestID, workflowID, nullableString(conversationID)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3) ON CONFLICT DO NOTHING`, "media_workflow_intent:"+intentID, workflowID, jsonBytes(map[string]any{"intent_id": intentID, "provider_request_id": providerRequestID})); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID); err != nil {
				return err
			}
			return appendOutboxTx(ctx, tx, "media.intent.created", "autonomy_action", actionID, fluctlightID, actionID, "autonomy:"+actionID, "media-outbox:"+actionID, map[string]any{"media_intent_id": intentID})
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"action_id": actionID, "action_type": actionType, "status": "completed", "media_intent_id": intentID}, nil
	}
	return a.failAutonomyAction(ctx, actionID, "unsupported_action_type")
}

func (a *App) failAutonomyAction(ctx context.Context, actionID, code string) (map[string]any, error) {
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.autonomy_actions SET status='failed',error_code=$2,settled_at=now() WHERE id=$1 AND status='frozen'`, actionID, code)
	if err != nil {
		return nil, err
	}
	return map[string]any{"action_id": actionID, "status": "failed", "error_code": code}, nil
}

func (a *App) ProcessReflection(ctx context.Context, fluctlightID, correlationID string) (map[string]any, error) {
	if fluctlightID == "" {
		return nil, fmt.Errorf("reflection_fluctlight_id_required")
	}
	var watermark, stateRevision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT watermark,state_revision FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, fluctlightID).Scan(&watermark, &stateRevision); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT sequence,event_type,payload FROM public.cognition_inbox WHERE fluctlight_id=$1 AND sequence>$2 AND status='processed' ORDER BY sequence LIMIT 20`, fluctlightID, watermark)
	if err != nil {
		return nil, err
	}
	evidence := make([]map[string]any, 0)
	toSequence := watermark
	for rows.Next() {
		var sequence int
		var typ string
		var payload []byte
		if err := rows.Scan(&sequence, &typ, &payload); err != nil {
			rows.Close()
			return nil, err
		}
		evidence = append(evidence, map[string]any{"sequence": sequence, "event_type": typ, "payload": json.RawMessage(payload)})
		if sequence > toSequence {
			toSequence = sequence
		}
	}
	rows.Close()
	if len(evidence) == 0 {
		return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "no_op", "watermark": watermark}, nil
	}
	proposal, err := a.Provider.Structured(ctx, "reflection", []map[string]any{{"role": "system", "content": "Review the supplied evidence. Return JSON with memory_candidates and relationship_candidates arrays. Every memory candidate must include type, content, confidence, importance, emotional_significance, visibility, and evidence_refs. Do not invent facts outside evidence."}, {"role": "user", "content": jsonString(map[string]any{"from_sequence": watermark + 1, "to_sequence": toSequence, "evidence": evidence})}})
	if err != nil {
		return nil, err
	}
	if !validReflectionProposal(proposal) {
		return nil, errors.New("reflection_proposal_invalid")
	}
	proposalID := "reflection_" + stableDigest(fluctlightID+fmt.Sprint(toSequence))
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_reflection_proposals(id,fluctlight_id,from_sequence,to_sequence,base_state_revision,payload,evidence_refs,correlation_id,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'applied') ON CONFLICT(id) DO NOTHING`, proposalID, fluctlightID, watermark+1, toSequence, stateRevision, jsonBytes(proposal), jsonBytes(evidence), correlationID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.cognition_reflection_windows(fluctlight_id,watermark,state_revision,status) VALUES($1,$2,$3,'idle') ON CONFLICT(fluctlight_id) DO UPDATE SET watermark=$2,state_revision=$3,status='idle',updated_at=now() WHERE public.cognition_reflection_windows.watermark=$4`, fluctlightID, toSequence, stateRevision, watermark)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "applied", "watermark": toSequence, "proposal_id": proposalID}, nil
}

func validReflectionProposal(value map[string]any) bool {
	for _, key := range []string{"memory_candidates", "relationship_candidates"} {
		if raw, ok := value[key]; ok && raw != nil {
			if _, ok := raw.([]any); !ok {
				return false
			}
		}
	}
	return true
}

func (a *App) ProcessMemoryEmbedding(ctx context.Context, memoryID string) (map[string]any, error) {
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id_required")
	}
	var content string
	var revision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT content,revision FROM public.memories WHERE id=$1 AND status NOT IN ('forgotten','superseded')`, memoryID).Scan(&content, &revision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]any{"memory_id": memoryID, "status": "not_found"}, nil
		}
		return nil, err
	}
	model, vector, err := a.Provider.Embed(ctx, content)
	if err != nil {
		_, _ = a.DB.Pool().Exec(ctx, `UPDATE public.memory_embeddings SET status='failed',error_code=$2 WHERE memory_id=$1 AND memory_revision=$3`, memoryID, err.Error(), revision)
		return nil, err
	}
	encoded := make([]string, len(vector))
	for i, v := range vector {
		encoded[i] = fmt.Sprintf("%g", v)
	}
	embeddingID := "embedding_" + stableDigest(memoryID+fmt.Sprint(revision))
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='stale' WHERE memory_id=$1 AND memory_revision<>$2 AND status IN ('ready','completed')`, memoryID, revision); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.memory_embeddings(id,memory_id,memory_revision,model_id,dimensions,embedding,embedding_vector,status,embedded_at) VALUES($1,$2,$3,$4,$5,$6,$7::vector,'ready',now()) ON CONFLICT(id) DO UPDATE SET status='ready',embedding=excluded.embedding,embedding_vector=excluded.embedding_vector,embedded_at=now(),error_code=NULL`, embeddingID, memoryID, revision, model, len(vector), jsonBytes(vector), "["+strings.Join(encoded, ",")+"]")
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"memory_id": memoryID, "status": "ready", "revision": revision, "dimensions": len(vector), "model_id": model}, nil
}

// EnsureCurrentDaySchedule ensures that the persona has an LLM-generated,
// fully validated Schedule for its current local day. The Provider owns the
// semantic plan; Go only normalizes the structural JSON shape, validates the
// timezone/day coverage and commits the accepted projection transactionally.
func (a *App) EnsureCurrentDaySchedule(ctx context.Context, fluctlightID string) (map[string]any, error) {
	var timezone, ownerID string
	var identity, lifeProfile []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id,COALESCE(identity->>'timezone','Asia/Shanghai'),identity,life_profile FROM public.fluctlights WHERE id=$1 AND status <> 'retired'`, fluctlightID).Scan(&ownerID, &timezone, &identity, &lifeProfile); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	localDate := time.Now().In(location).Format("2006-01-02")
	var scheduleID string
	err = a.DB.Pool().QueryRow(ctx, `SELECT id FROM public.life_schedules WHERE fluctlight_id=$1 AND local_date=$2 AND status='accepted' ORDER BY revision DESC LIMIT 1`, fluctlightID, localDate).Scan(&scheduleID)
	if errors.Is(err, pgx.ErrNoRows) {
		generated, generateErr := a.generateInitialSchedule(ctx, ownerID, fluctlightID, localDate, timezone, decodeObject(identity), decodeObject(lifeProfile))
		if generateErr != nil {
			// Provider outage/invalid structured output is a durable pending
			// state. The workflow owns the bounded retry/continue-as-new loop;
			// it must not turn a transient model failure into a terminal intent.
			return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "timezone": timezone, "status": "pending", "error_code": "schedule_generation_failed"}, nil
		}
		return generated, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "schedule_id": scheduleID, "status": "ready"}, nil
}
