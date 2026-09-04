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
			messageID, err := appendAssistantTxWithID(ctx, tx, conversationID, fluctlightID, text, "proactive:"+actionID)
			if err != nil {
				return err
			}
			calls := toolCallsFromValue(data["tool_calls"])
			binding := OutputBindingV1{TargetKind: "conversation_message", TargetRef: messageID}
			toolResults := make([]ToolResultV1, 0, len(calls))
			if len(calls) > 0 {
				calls = normalizeToolCallMetadata(calls, firstString(data["source_fact_id"], actionID), actionID)
				if err := validateCompositeOutputCalls(calls, binding.TargetKind, a.capabilityRegistry()); err != nil {
					return err
				}
				toolResults, err = a.settleDeferredToolCallsTx(ctx, tx, fluctlightID, firstString(data["source_fact_id"], actionID), actionID, calls, nil, binding)
				if err != nil {
					return err
				}
				bound := make([]OutputBindingV1, 0, len(calls))
				for _, call := range calls {
					bound = append(bound, OutputBindingV1{ToolCallID: call.ID, TargetKind: binding.TargetKind, TargetRef: binding.TargetRef})
				}
				if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(payload,'{output_bindings}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(bound)); err != nil {
					return err
				}
			}
			command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return ErrConflict
			}
			if err := a.settleWakeUpActionTx(ctx, tx, actionID, fluctlightID, map[string]any{"status": "completed", "action_status": "completed", "message_id": messageID, "tool_results": toolResults}); err != nil {
				return err
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
			calls := toolCallsFromValue(data["tool_calls"])
			binding := OutputBindingV1{TargetKind: "moment", TargetRef: momentID}
			toolResults := make([]ToolResultV1, 0, len(calls))
			if len(calls) > 0 {
				calls = normalizeToolCallMetadata(calls, firstString(data["source_fact_id"], actionID), actionID)
				if err := validateCompositeOutputCalls(calls, binding.TargetKind, a.capabilityRegistry()); err != nil {
					return err
				}
				settled, settleErr := a.settleDeferredToolCallsTx(ctx, tx, fluctlightID, firstString(data["source_fact_id"], actionID), actionID, calls, nil, binding)
				if settleErr != nil {
					return settleErr
				}
				toolResults = settled
				bound := make([]OutputBindingV1, 0, len(calls))
				for _, call := range calls {
					bound = append(bound, OutputBindingV1{ToolCallID: call.ID, TargetKind: binding.TargetKind, TargetRef: binding.TargetRef})
				}
				if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(payload,'{output_bindings}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(bound)); err != nil {
					return err
				}
			}
			command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return ErrConflict
			}
			if err := a.settleWakeUpActionTx(ctx, tx, actionID, fluctlightID, map[string]any{"status": "completed", "action_status": "completed", "moment_id": momentID, "tool_results": toolResults}); err != nil {
				return err
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
			if err := a.settleWakeUpActionTx(ctx, tx, actionID, fluctlightID, map[string]any{"status": "completed", "action_status": "completed", "media_intent_id": intentID}); err != nil {
				return err
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

func (a *App) ProcessCapabilityAction(ctx context.Context, actionID string) (map[string]any, error) {
	var fluctlightID, status string
	var payload []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id,status,payload FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&fluctlightID, &status, &payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status == "completed" || status == "failed" || status == "cancelled" || status == "paused" || status == "deferred" || status == "cancel_requested" {
		return map[string]any{"action_id": actionID, "action_type": "capability", "status": status}, nil
	}
	if status != "frozen" {
		return nil, fmt.Errorf("capability action is not executable: %s", status)
	}
	data := decodeObject(payload)
	calls := toolCallsFromValue(data["tool_calls"])
	if len(calls) == 0 {
		return a.failAutonomyAction(ctx, actionID, "capability_calls_empty")
	}
	var sourceFactID string
	sourceFactID = stringValue(data["source_fact_id"])
	if sourceFactID == "" {
		return a.failAutonomyAction(ctx, actionID, "capability_source_missing")
	}
	for _, call := range calls {
		manifest, ok := toolManifestMap(a.capabilityRegistry().Manifests())[call.Name]
		if !ok {
			return a.failAutonomyAction(ctx, actionID, "capability_unavailable")
		}
		if manifest.IsDeferredOutput() {
			// A capability action without a Composite Action output target cannot
			// safely execute an external async slot. Targeted actions are settled
			// by the proactive_message/moment branches above.
			return a.failAutonomyAction(ctx, actionID, "capability_output_target_missing")
		}
	}
	for index := range calls {
		calls[index].ActionID = actionID
		calls[index].SourceFactID = sourceFactID
	}
	results, err := a.ExecuteToolCalls(ctx, fluctlightID, stringValue(data["conversation_id"]), sourceFactID, calls)
	if err != nil {
		_, _ = a.DB.Pool().Exec(ctx, `UPDATE public.autonomy_actions SET status='failed',error_code=$2,settled_at=now() WHERE id=$1 AND status='frozen'`, actionID, "capability_action_failed")
		_, _ = a.DB.Pool().Exec(ctx, `UPDATE public.cognition_wakeups SET result=result || $2::jsonb WHERE action_id=$1`, actionID, jsonBytes(map[string]any{"status": "failed", "action_status": "failed", "tool_results": results}))
		return nil, err
	}
	result := map[string]any{"status": "completed", "action_status": "completed", "tool_results": results}
	if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		return a.settleWakeUpActionTx(ctx, tx, actionID, fluctlightID, result)
	}); err != nil {
		return nil, err
	}
	return map[string]any{"action_id": actionID, "action_type": "capability", "status": "completed", "tool_results": results}, nil
}

func (a *App) failAutonomyAction(ctx context.Context, actionID, code string) (map[string]any, error) {
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.autonomy_actions SET status='failed',error_code=$2,settled_at=now() WHERE id=$1 AND status IN ('frozen','running')`, actionID, code)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() != 1 {
		return nil, ErrConflict
	}
	_, _ = a.DB.Pool().Exec(ctx, `UPDATE public.cognition_wakeups SET result=result || $2::jsonb WHERE action_id=$1`, actionID, jsonBytes(map[string]any{"status": "failed", "action_status": "failed", "error_code": code}))
	return map[string]any{"action_id": actionID, "status": "failed", "error_code": code}, nil
}

func (a *App) settleWakeUpActionTx(ctx context.Context, tx pgx.Tx, actionID, fluctlightID string, result map[string]any) error {
	if _, err := tx.Exec(ctx, `UPDATE public.cognition_wakeups SET result=result || $2::jsonb WHERE action_id=$1`, actionID, jsonBytes(result)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_action_results(id,fluctlight_id,action_id,source_fact_id,status,output,evidence_refs) VALUES($1,$2,$3,$4,'completed',$5,$6) ON CONFLICT(action_id) DO NOTHING`, "action_result_"+stableDigest(actionID+":settled"), fluctlightID, actionID, actionID, jsonBytes(result), jsonBytes([]string{actionID})); err != nil {
		return err
	}
	factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", map[string]any{"action_id": actionID, "result": result}, "action-result:"+actionID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:result:"+actionID, "reflection:result:"+actionID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": actionID}))
	return err
}

func (a *App) ProcessReflection(ctx context.Context, fluctlightID, correlationID string) (map[string]any, error) {
	// Reflection is the evidence-windowed learning pass over processed
	// cognition facts. A wake-up may create the fact that feeds this window,
	// but reflection never substitutes for the periodic wake-up trigger.
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
	for _, memory := range projection.Memories {
		if memoryID := stringValue(memory["id"]); memoryID != "" {
			allowedEvidence[memoryID] = struct{}{}
			allowedEvidence["memory:"+memoryID] = struct{}{}
		}
	}
	proposal, err := a.Provider.Structured(WithProviderScenario(ctx, "reflection"), "reflection", []map[string]any{{"role": "system", "content": reflectionInstruction}, {"role": "user", "content": jsonString(map[string]any{"from_sequence": watermark + 1, "to_sequence": toSequence, "evidence": evidence, "context": compactCognitionContext(projection)})}})
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	normalizedProposal := normalizeReflectionProposal(proposal)
	filteredProposal := filterReflectionEvidence(normalizedProposal, allowedEvidence)
	for _, key := range []string{"memory_candidates", "relationship_candidates", "developing_self_candidates", "drive_candidates", "preference_candidates", "trigger_candidates"} {
		beforeCount := len(arrayValue(normalizedProposal[key]))
		afterCount := len(arrayValue(filteredProposal[key]))
		if beforeCount > afterCount {
			a.recordDiagnosticEvent(ctx, "developing_self.candidate.rejected", "warn", fluctlightID, "reflection:"+fluctlightID, correlationID, map[string]any{"reason_code": "evidence_invalid_or_outside_window", "candidate_type": key, "dropped": beforeCount - afterCount})
		}
	}
	proposal = filteredProposal
	if err := validateReflectionProposal(proposal, allowedEvidence); err != nil {
		a.recordDiagnosticEvent(ctx, "developing_self.candidate.rejected", "warn", fluctlightID, "reflection:"+fluctlightID, correlationID, map[string]any{"reason_code": err.Error(), "proposal": proposal})
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	proposalID := "reflection_" + stableDigest(fluctlightID+fmt.Sprint(toSequence))
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var latestStateRevision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1 FOR SHARE`, fluctlightID).Scan(&latestStateRevision); err != nil {
			return err
		}
		if latestStateRevision != stateRevision {
			return ErrConflict
		}
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

// normalizeReflectionProposal keeps the reflection boundary tolerant of
// common provider aliases while remaining fail-closed for incomplete
// candidates. A malformed optional candidate is omitted; valid candidates in
// the same proposal can still advance the watermark and be applied.
func normalizeReflectionProposal(value map[string]any) map[string]any {
	result := make(map[string]any, 7)
	for _, key := range []string{"memory_candidates", "relationship_candidates", "developing_self_candidates", "drive_candidates", "preference_candidates", "trigger_candidates"} {
		rawCandidates := value[key]
		if rawCandidates == nil {
			switch key {
			case "drive_candidates":
				rawCandidates = value["drive_recalibration_candidates"]
			case "preference_candidates":
				rawCandidates = value["preference_revision_candidates"]
			case "trigger_candidates":
				rawCandidates = value["future_trigger_candidates"]
			}
		}
		items := make([]any, 0)
		for _, raw := range arrayValue(rawCandidates) {
			item := mapValue(raw)
			if len(item) == 0 {
				continue
			}
			normalized := make(map[string]any, len(item)+2)
			for field, fieldValue := range item {
				normalized[field] = fieldValue
			}
			switch key {
			case "memory_candidates":
				if stringValue(normalized["type"]) == "" {
					switch stringValue(normalized["memory_type"]) {
					case "user_preference":
						normalized["type"] = "semantic"
					case "context", "interaction_pattern":
						normalized["type"] = "episodic"
					}
				}
				if stringValue(normalized["visibility"]) == "" {
					switch stringValue(normalized["scope"]) {
					case "conversation", "owner":
						normalized["visibility"] = "owner"
					case "private":
						normalized["visibility"] = "private"
					case "participants":
						normalized["visibility"] = "participants"
					}
				}
				if stringValue(normalized["visibility"]) == "" {
					normalized["visibility"] = "private"
				}
				if normalized["emotional_significance"] == nil {
					normalized["emotional_significance"] = 0.0
				}
				if stringValue(normalized["content"]) == "" || stringValue(normalized["type"]) == "" || normalized["importance"] == nil {
					continue
				}
			case "relationship_candidates":
				if stringValue(normalized["target_actor_id"]) == "" {
					normalized["target_actor_id"] = normalized["counterparty_id"]
				}
				if stringValue(normalized["trend"]) == "" || stringValue(normalized["target_actor_id"]) == "" {
					continue
				}
			case "developing_self_candidates":
				if stringValue(normalized["category"]) == "" {
					normalized["category"] = firstString(normalized["dimension"], firstString(normalized["type"], ""))
				}
				if stringValue(normalized["claim"]) == "" {
					normalized["claim"] = firstString(normalized["summary"], firstString(normalized["content"], ""))
				}
				if stringValue(normalized["claim"]) == "" || normalized["value"] == nil {
					continue
				}
			case "drive_candidates", "preference_candidates":
				if stringValue(normalized["key"]) == "" && stringValue(normalized["slot_key"]) != "" {
					normalized["key"] = normalized["slot_key"]
				}
				if stringValue(normalized["value_schema"]) == "" && stringValue(normalized["schema"]) != "" {
					normalized["value_schema"] = normalized["schema"]
				}
				if stringValue(normalized["label"]) == "" {
					normalized["label"] = normalized["name"]
				}
				if stringValue(normalized["description"]) == "" {
					normalized["description"] = normalized["meaning"]
				}
				if stringValue(normalized["key"]) == "" || normalized["value"] == nil {
					continue
				}
			case "trigger_candidates":
				if stringValue(normalized["key"]) == "" {
					normalized["key"] = normalized["trigger_key"]
				}
				if normalized["value"] == nil {
					normalized["value"] = normalized["trigger"]
				}
				if stringValue(normalized["key"]) == "" || normalized["value"] == nil {
					continue
				}
			}
			items = append(items, normalized)
		}
		result[key] = items
	}
	return result
}

func filterReflectionEvidence(proposal map[string]any, allowed map[string]struct{}) map[string]any {
	for _, key := range []string{"memory_candidates", "relationship_candidates", "developing_self_candidates", "drive_candidates", "preference_candidates", "trigger_candidates"} {
		filtered := make([]any, 0)
		for _, raw := range arrayValue(proposal[key]) {
			item := mapValue(raw)
			refs := arrayValue(item["evidence_refs"])
			validRefs := make([]any, 0, len(refs))
			for _, ref := range refs {
				if _, ok := allowed[stringValue(ref)]; ok {
					validRefs = append(validRefs, ref)
				}
			}
			if len(validRefs) == 0 {
				continue
			}
			item["evidence_refs"] = validRefs
			filtered = append(filtered, item)
		}
		proposal[key] = filtered
	}
	return proposal
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
	for _, key := range []string{"memory_candidates", "relationship_candidates", "developing_self_candidates", "drive_candidates", "preference_candidates", "trigger_candidates"} {
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
			if key == "developing_self_candidates" {
				if stringValue(item["category"]) == "" || !validateDevelopingSelfCategory(stringValue(item["category"])) || stringValue(item["claim"]) == "" || item["value"] == nil {
					return errors.New("reflection_developing_self_fields_invalid")
				}
				if _, err := requiredBoundedNumber(item["confidence"]); err != nil {
					return errors.New("reflection_developing_self_confidence_invalid")
				}
				if _, err := normalizeDevelopingSelfClaim(item, "reflection"); err != nil {
					return err
				}
			}
			if key == "drive_candidates" {
				if _, err := validateSlotCandidate(item, "drive", allowedEvidence); err != nil {
					return fmt.Errorf("reflection_drive_slot_invalid: %w", err)
				}
			}
			if key == "preference_candidates" {
				if _, err := validateSlotCandidate(item, "preference", allowedEvidence); err != nil {
					return fmt.Errorf("reflection_preference_slot_invalid: %w", err)
				}
			}
			if key == "trigger_candidates" {
				if !validateSlotKey(stringValue(item["key"])) || len(jsonBytes(item["value"])) == 0 || len(jsonBytes(item["value"])) > 16000 {
					return errors.New("reflection_trigger_candidate_invalid")
				}
				if _, err := requiredBoundedNumber(item["confidence"]); err != nil {
					return errors.New("reflection_trigger_confidence_invalid")
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
	for _, raw := range arrayValue(proposal["developing_self_candidates"]) {
		refs := arrayValue(mapValue(raw)["evidence_refs"])
		if err := a.applyDevelopingSelfCandidateTx(ctx, tx, fluctlightID, mapValue(raw), refs, allowedEvidence, sourceWindow); err != nil {
			return err
		}
	}
	for index, raw := range arrayValue(proposal["drive_candidates"]) {
		if err := a.applyDriveSlotCandidateTx(ctx, tx, fluctlightID, mapValue(raw), allowedEvidence, sourceWindow, index); err != nil {
			return err
		}
	}
	for index, raw := range arrayValue(proposal["preference_candidates"]) {
		if err := a.applyPreferenceSlotCandidateTx(ctx, tx, fluctlightID, mapValue(raw), allowedEvidence, sourceWindow, index); err != nil {
			return err
		}
	}
	for index, raw := range arrayValue(proposal["trigger_candidates"]) {
		if err := a.applyTriggerPreferenceCandidateTx(ctx, tx, fluctlightID, mapValue(raw), allowedEvidence, sourceWindow, index); err != nil {
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
