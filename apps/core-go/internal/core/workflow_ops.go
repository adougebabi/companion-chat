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
	if status == "completed" || status == "failed" || status == "cancelled" || status == "paused" || status == "deferred" || status == "cancel_requested" {
		return map[string]any{"action_id": actionID, "action_type": actionType, "status": status}, nil
	}
	if status != "frozen" {
		return nil, fmt.Errorf("autonomy action is not executable: %s", status)
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
			command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return ErrConflict
			}
			return appendOutboxTx(ctx, tx, "autonomy.action.completed", "autonomy_action", actionID, fluctlightID, actionID, "autonomy:"+actionID, "autonomy-outbox:"+actionID, map[string]any{"action_type": actionType, "status": "completed", "aggregate_sequence": 1})
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
			command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return ErrConflict
			}
			return appendOutboxTx(ctx, tx, "moment.published", "moment", momentID, fluctlightID, actionID, "autonomy:"+actionID, "moment-outbox:"+actionID, map[string]any{"moment_id": momentID, "action_id": actionID, "aggregate_sequence": 1})
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
			command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return ErrConflict
			}
			return appendOutboxTx(ctx, tx, "media.intent.created", "autonomy_action", actionID, fluctlightID, actionID, "autonomy:"+actionID, "media-outbox:"+actionID, map[string]any{"media_intent_id": intentID, "aggregate_sequence": 1})
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"action_id": actionID, "action_type": actionType, "status": "completed", "media_intent_id": intentID}, nil
	}
	return a.failAutonomyAction(ctx, actionID, "unsupported_action_type")
}

func (a *App) failAutonomyAction(ctx context.Context, actionID, code string) (map[string]any, error) {
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.autonomy_actions SET status='failed',error_code=$2,settled_at=now() WHERE id=$1 AND status IN ('frozen','running')`, actionID, code)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() != 1 {
		return nil, ErrConflict
	}
	return map[string]any{"action_id": actionID, "status": "failed", "error_code": code}, nil
}

func (a *App) ProcessReflection(ctx context.Context, fluctlightID, correlationID string) (map[string]any, error) {
	if fluctlightID == "" {
		return nil, fmt.Errorf("reflection_fluctlight_id_required")
	}
	var watermark, stateRevision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT watermark,state_revision FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, fluctlightID).Scan(&watermark, &stateRevision); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		_ = a.DB.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision)
	}
	var currentStateRevision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&currentStateRevision); err == nil && currentStateRevision > stateRevision {
		stateRevision = currentStateRevision
	}
	if err := a.claimReflectionWindow(ctx, fluctlightID, watermark, stateRevision); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,sequence,event_type,payload FROM public.cognition_inbox WHERE fluctlight_id=$1 AND sequence>$2 AND status='processed' ORDER BY sequence LIMIT 20`, fluctlightID, watermark)
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	evidence := make([]map[string]any, 0)
	allowedEvidence := make(map[string]struct{})
	toSequence := watermark
	for rows.Next() {
		var id string
		var sequence int
		var typ string
		var payload []byte
		if err := rows.Scan(&id, &sequence, &typ, &payload); err != nil {
			rows.Close()
			_ = a.setReflectionWindowIdle(ctx, fluctlightID)
			return nil, err
		}
		allowedEvidence[id] = struct{}{}
		allowedEvidence[fmt.Sprintf("sequence:%d", sequence)] = struct{}{}
		evidence = append(evidence, map[string]any{"id": id, "sequence": sequence, "event_type": typ, "payload": json.RawMessage(payload)})
		if sequence > toSequence {
			toSequence = sequence
		}
	}
	rows.Close()
	if len(evidence) == 0 {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "no_op", "watermark": watermark}, nil
	}
	var ownerActorID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerActorID); err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	projection, err := a.BuildContextProjection(ctx, ownerActorID, fluctlightID, "", "reflection:"+fluctlightID, "")
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	proposal, err := a.Provider.Structured(ctx, "reflection", []map[string]any{{"role": "system", "content": "Review only the supplied evidence and context. Return JSON with memory_candidates, relationship_candidates, self_model_candidates, and personality_candidates arrays. Every candidate must include complete typed fields, confidence, and evidence_refs that reference an evidence id or sequence:N. Do not invent facts and do not use defaults."}, {"role": "user", "content": jsonString(map[string]any{"from_sequence": watermark + 1, "to_sequence": toSequence, "evidence": evidence, "context": projection})}})
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	if err := validateReflectionProposal(proposal, allowedEvidence); err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	proposalID := "reflection_" + stableDigest(fluctlightID+fmt.Sprint(toSequence))
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_reflection_proposals(id,fluctlight_id,from_sequence,to_sequence,base_state_revision,payload,evidence_refs,correlation_id,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'proposed') ON CONFLICT(id) DO NOTHING`, proposalID, fluctlightID, watermark+1, toSequence, stateRevision, jsonBytes(proposal), jsonBytes(evidence), correlationID); err != nil {
			return err
		}
		if err := a.applyReflectionCandidates(ctx, tx, fluctlightID, proposal, allowedEvidence, proposalID); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `UPDATE public.cognition_reflection_windows SET watermark=$2,state_revision=$3,status='idle',updated_at=now() WHERE fluctlight_id=$1 AND watermark=$4 AND status='running'`, fluctlightID, toSequence, stateRevision, watermark)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			if watermark == 0 {
				_, err = tx.Exec(ctx, `INSERT INTO public.cognition_reflection_windows(fluctlight_id,watermark,state_revision,status) VALUES($1,$2,$3,'idle') ON CONFLICT DO NOTHING`, fluctlightID, toSequence, stateRevision)
				return err
			}
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE public.cognition_reflection_proposals SET status='applied' WHERE id=$1`, proposalID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "applied", "watermark": toSequence, "proposal_id": proposalID}, nil
}

func (a *App) claimReflectionWindow(ctx context.Context, fluctlightID string, watermark, stateRevision int) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var status string
		var updatedAt time.Time
		err := tx.QueryRow(ctx, `SELECT status,updated_at FROM public.cognition_reflection_windows WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&status, &updatedAt)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && status == "running" && time.Since(updatedAt) < 15*time.Minute {
			return ErrConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `INSERT INTO public.cognition_reflection_windows(fluctlight_id,watermark,state_revision,status) VALUES($1,$2,$3,'running')`, fluctlightID, watermark, stateRevision)
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.cognition_reflection_windows SET status='running',state_revision=$2,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, stateRevision)
		return err
	})
}

func (a *App) setReflectionWindowIdle(ctx context.Context, fluctlightID string) error {
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.cognition_reflection_windows SET status='idle',updated_at=now() WHERE fluctlight_id=$1 AND status='running'`, fluctlightID)
	return err
}

func validateReflectionProposal(value map[string]any, allowedEvidence map[string]struct{}) error {
	for _, key := range []string{"memory_candidates", "relationship_candidates", "self_model_candidates", "personality_candidates"} {
		raw, ok := value[key]
		if !ok || raw == nil {
			continue
		}
		items, ok := raw.([]any)
		if !ok {
			return errors.New("reflection_candidates_invalid")
		}
		for _, entry := range items {
			item := mapValue(entry)
			if len(item) == 0 {
				return errors.New("reflection_candidate_invalid")
			}
			refs := arrayValue(item["evidence_refs"])
			if !validateEvidenceRefs(refs, allowedEvidence) {
				return errors.New("reflection_evidence_invalid")
			}
			if key == "memory_candidates" {
				if stringValue(item["content"]) == "" || stringValue(item["type"]) == "" {
					return errors.New("reflection_memory_fields_invalid")
				}
				if _, ok := validMemoryTypes[stringValue(item["type"])]; !ok {
					return errors.New("reflection_memory_type_invalid")
				}
				for _, field := range []string{"confidence", "importance", "emotional_significance"} {
					if _, err := requiredBoundedNumber(item[field]); err != nil {
						return errors.New("reflection_memory_numeric_invalid")
					}
				}
				if _, ok := validMemoryVisibility[firstString(item["visibility"], "")]; !ok {
					return errors.New("reflection_memory_visibility_invalid")
				}
			}
			if key == "relationship_candidates" && (stringValue(item["target_actor_id"]) == "" || stringValue(item["trend"]) == "") {
				return errors.New("reflection_relationship_fields_invalid")
			}
			if key == "self_model_candidates" {
				if stringValue(item["category"]) == "" || stringValue(item["claim"]) == "" {
					return errors.New("reflection_self_model_fields_invalid")
				}
				if _, err := requiredBoundedNumber(item["confidence"]); err != nil {
					return errors.New("reflection_self_model_confidence_invalid")
				}
			}
			if key == "personality_candidates" {
				if stringValue(item["trait"]) == "" {
					return errors.New("reflection_personality_fields_invalid")
				}
				if _, err := requiredBoundedNumber(item["value"]); err != nil {
					return errors.New("reflection_personality_value_invalid")
				}
				if confidence, err := requiredBoundedNumber(item["confidence"]); err != nil || confidence < 0.8 {
					return errors.New("reflection_personality_confidence_low")
				}
			}
		}
	}
	return nil
}

func (a *App) applyReflectionCandidates(ctx context.Context, tx pgx.Tx, fluctlightID string, proposal map[string]any, allowedEvidence map[string]struct{}, sourceWindow string) error {
	for _, raw := range arrayValue(proposal["memory_candidates"]) {
		item := mapValue(raw)
		if !validateEvidenceRefs(arrayValue(item["evidence_refs"]), allowedEvidence) {
			return errors.New("reflection_memory_evidence_invalid")
		}
		if stringValue(item["idempotency_key"]) == "" {
			item["idempotency_key"] = "reflection:" + sourceWindow + ":" + stableDigest(stringValue(item["content"]))
		}
		record, err := normalizeMemoryRecord(fluctlightID, item)
		if err != nil {
			return err
		}
		if _, err := recordMemoryTx(ctx, tx, record, fluctlightID); err != nil {
			return err
		}
	}
	for index, raw := range arrayValue(proposal["relationship_candidates"]) {
		item := mapValue(raw)
		target := stringValue(item["target_actor_id"])
		revisionKey := "reflection:" + fluctlightID + ":" + sourceWindow + ":" + fmt.Sprint(index)
		var alreadyApplied bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.relationship_revisions WHERE idempotency_key=$1)`, revisionKey).Scan(&alreadyApplied); err != nil {
			return err
		}
		if alreadyApplied {
			continue
		}
		var relationshipID string
		var revision int
		if err := tx.QueryRow(ctx, `SELECT id,revision FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 FOR UPDATE`, fluctlightID, target).Scan(&relationshipID, &revision); errors.Is(err, pgx.ErrNoRows) {
			relationshipID = "relationship_" + stableDigest(fluctlightID+":"+target)
			if _, err := tx.Exec(ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,metrics,trend,summary,emotional_association,revision) VALUES($1,$2,$3,$4,$5,$6,$7,0) ON CONFLICT DO NOTHING`, relationshipID, fluctlightID, target, jsonBytes(mapValue(item["metrics"])), stringValue(item["trend"]), nullableString(stringValue(item["summary"])), jsonBytes(mapValue(item["emotional_association"]))); err != nil {
				return err
			}
			revision = 0
		} else if err != nil {
			return err
		} else {
			revision++
			if _, err := tx.Exec(ctx, `UPDATE public.relationships SET metrics=$2,trend=$3,summary=$4,emotional_association=$5,revision=$6,updated_at=now() WHERE id=$1`, relationshipID, jsonBytes(mapValue(item["metrics"])), stringValue(item["trend"]), nullableString(stringValue(item["summary"])), jsonBytes(mapValue(item["emotional_association"])), revision); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.relationship_revisions(id,relationship_id,revision,base_revision,metrics,trend,summary,emotional_association,evidence_refs,actor_id,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT DO NOTHING`, "relationship_revision_"+stableDigest(fluctlightID+":"+target+":"+fmt.Sprint(index)+":"+fmt.Sprint(revision)), relationshipID, revision, maxInt(0, revision-1), jsonBytes(mapValue(item["metrics"])), stringValue(item["trend"]), nullableString(stringValue(item["summary"])), jsonBytes(mapValue(item["emotional_association"])), jsonBytes(arrayValue(item["evidence_refs"])), fluctlightID, revisionKey); err != nil {
			return err
		}
	}
	for _, raw := range arrayValue(proposal["self_model_candidates"]) {
		refs := arrayValue(mapValue(raw)["evidence_refs"])
		if len(refs) < 3 {
			continue
		}
		if err := a.applySelfModelCandidateTx(ctx, tx, fluctlightID, mapValue(raw), refs, sourceWindow); err != nil {
			return err
		}
	}
	for _, raw := range arrayValue(proposal["personality_candidates"]) {
		refs := arrayValue(mapValue(raw)["evidence_refs"])
		if len(refs) < 3 {
			continue
		}
		if err := a.applyPersonalityCandidateTx(ctx, tx, fluctlightID, mapValue(raw), refs, sourceWindow); err != nil {
			return err
		}
	}
	return nil
}

func boundedNumber(value any, fallback float64) float64 {
	parsed, ok := numberFloat(value)
	if !ok || parsed < 0 || parsed > 1 {
		return fallback
	}
	return parsed
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *App) ProcessMemoryEmbedding(ctx context.Context, memoryID string) (map[string]any, error) {
	return a.ProcessMemoryEmbeddingAt(ctx, memoryID, 0)
}

func (a *App) ProcessMemoryEmbeddingAt(ctx context.Context, memoryID string, requestedRevision int) (map[string]any, error) {
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
	if requestedRevision > 0 && revision != requestedRevision {
		return map[string]any{"memory_id": memoryID, "status": "stale", "revision": revision, "requested_revision": requestedRevision}, nil
	}
	assignment, assignmentErr := a.Provider.assignment(ctx, "embedding")
	if assignmentErr != nil {
		return nil, assignmentErr
	}
	model, vector, err := a.Provider.Embed(ctx, content)
	if err != nil {
		_, _ = a.DB.Pool().Exec(ctx, `INSERT INTO public.memory_embeddings(id,memory_id,memory_revision,model_id,dimensions,embedding,status,error_code) VALUES($1,$2,$3,$4,0,'[]','failed',$5) ON CONFLICT(id) DO UPDATE SET status='failed',error_code=excluded.error_code`, "embedding_"+stableDigest(memoryID+fmt.Sprint(revision)+assignment.ModelID), memoryID, revision, assignment.ModelID, "provider_or_persistence_failure")
		return nil, err
	}
	encoded := make([]string, len(vector))
	for i, v := range vector {
		encoded[i] = fmt.Sprintf("%g", v)
	}
	embeddingID := "embedding_" + stableDigest(memoryID+fmt.Sprint(revision)+":"+model)
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var currentRevision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.memories WHERE id=$1 FOR UPDATE`, memoryID).Scan(&currentRevision); err != nil {
			return err
		}
		if currentRevision != revision || (requestedRevision > 0 && currentRevision != requestedRevision) {
			return errors.New("memory_embedding_stale")
		}
		var existingDimension int
		if err := tx.QueryRow(ctx, `SELECT dimensions FROM public.memory_embeddings WHERE memory_id=$1 AND model_id=$2 AND status IN ('ready','completed') ORDER BY created_at DESC LIMIT 1`, memoryID, model).Scan(&existingDimension); err == nil && existingDimension != len(vector) {
			return errors.New("memory_embedding_dimension_mismatch")
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='stale' WHERE memory_id=$1 AND memory_revision<>$2 AND status <> 'stale'`, memoryID, revision); err != nil {
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
	timezone = canonicalTimezone(timezone)
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
