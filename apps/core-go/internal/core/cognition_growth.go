package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var appraisalFields = []string{"relevance", "goal_congruence", "reward", "loss", "social_threat", "controllability", "responsibility", "relationship_significance", "expected_effect"}

func normalizeAppraisal(value any) (map[string]any, error) {
	appraisal := mapValue(value)
	if len(appraisal) == 0 {
		return nil, errors.New("appraisal_required")
	}
	result := make(map[string]any, len(appraisal))
	for key, raw := range appraisal {
		result[key] = raw
	}
	for _, field := range appraisalFields {
		parsed, err := requiredBoundedNumber(appraisal[field])
		if err != nil {
			return nil, fmt.Errorf("appraisal_%s_invalid", field)
		}
		result[field] = parsed
	}
	if rawRefs, exists := appraisal["evidence_refs"]; exists {
		var refs []any
		switch rawRefs.(type) {
		case []any, []string:
			refs = arrayValue(rawRefs)
		default:
			return nil, errors.New("appraisal_evidence_refs_invalid")
		}
		if len(refs) > 32 {
			return nil, errors.New("appraisal_evidence_refs_invalid")
		}
		for _, ref := range refs {
			if text := stringValue(ref); text == "" || len([]rune(text)) > 256 {
				return nil, errors.New("appraisal_evidence_refs_invalid")
			}
		}
		result["evidence_refs"] = refs
	} else {
		result["evidence_refs"] = []any{}
	}
	if eventKind := stringValue(appraisal["event_kind"]); eventKind != "" {
		result["event_kind"] = eventKind
	}
	if direction := stringValue(appraisal["direction"]); direction != "" {
		result["direction"] = direction
	}
	return result, nil
}

func clampGrowth(value float64) float64 {
	if value < -0.1 {
		return -0.1
	}
	if value > 0.1 {
		return 0.1
	}
	return value
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func reduceInternalDynamics(current map[string]any, appraisal map[string]any) (map[string]any, map[string]any, map[string]any) {
	result := map[string]any{}
	for key, value := range current {
		result[key] = value
	}
	pad := cloneMap(mapValue(current["pad"]))
	mood := cloneMap(mapValue(current["mood"]))
	momentum := cloneMap(mapValue(current["momentum"]))
	regulation := cloneMap(mapValue(current["regulation"]))
	for key, value := range pad {
		if number, ok := numberFloat(value); ok {
			pad[key] = clampUnit(number)
		}
	}
	for key, value := range mood {
		if number, ok := numberFloat(value); ok {
			mood[key] = clampUnit(number)
		}
	}
	for key, value := range momentum {
		if number, ok := numberFloat(value); ok {
			momentum[key] = clampUnit(number)
		}
	}
	for key, value := range regulation {
		if number, ok := numberFloat(value); ok {
			regulation[key] = clampUnit(number)
		}
	}
	reward := numberOrZero(appraisal["reward"])
	loss := numberOrZero(appraisal["loss"])
	threat := numberOrZero(appraisal["social_threat"])
	controllability := numberOrZero(appraisal["controllability"])
	expectedEffect := numberOrZero(appraisal["expected_effect"])
	requested := map[string]any{
		"pad.pleasure":         reward - 0.5 - loss,
		"pad.arousal":          threat + (expectedEffect-0.5)*0.25,
		"pad.dominance":        controllability - 0.5 - threat*0.5,
		"mood.intensity":       (absFloat(reward-0.5) + loss + threat) / 2,
		"momentum.value":       expectedEffect - 0.5,
		"regulation.stability": controllability - 0.5,
	}
	applied := make(map[string]any, len(requested))
	apply := func(target map[string]any, key, auditKey string, delta float64) {
		before := numberOrZero(target[key])
		bounded := clampGrowth(delta)
		target[key] = clampUnit(before + bounded)
		applied[auditKey] = bounded
	}
	apply(pad, "pleasure", "pad.pleasure", numberOrZero(requested["pad.pleasure"]))
	apply(pad, "arousal", "pad.arousal", numberOrZero(requested["pad.arousal"]))
	apply(pad, "dominance", "pad.dominance", numberOrZero(requested["pad.dominance"]))
	apply(mood, "intensity", "mood.intensity", numberOrZero(requested["mood.intensity"]))
	apply(momentum, "value", "momentum.value", numberOrZero(requested["momentum.value"]))
	apply(regulation, "stability", "regulation.stability", numberOrZero(requested["regulation.stability"]))
	result["pad"] = pad
	result["mood"] = mood
	result["momentum"] = momentum
	result["regulation"] = regulation
	result["revision"] = int(numberOrZero(current["revision"])) + 1
	result["last_updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return result, requested, applied
}

func numberOrZero(value any) float64 {
	parsed, _ := numberFloat(value)
	return parsed
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func cloneMap(value map[string]any) map[string]any {
	return decodeObject(jsonBytes(value))
}

func (a *App) persistCognitiveStagesTx(ctx context.Context, tx pgx.Tx, fluctlightID, sourceFactID string, assessment map[string]any, actionType, actionID string) (map[string]any, error) {
	stages := cognitiveStagePayload(assessment)
	appraisal, err := normalizeAppraisal(stages["appraisal"])
	if err != nil {
		return nil, err
	}
	refs := arrayValue(appraisal["evidence_refs"])
	if !containsStringValue(refs, sourceFactID) {
		if len(refs) >= 32 {
			refs = refs[:31]
		}
		refs = append(refs, sourceFactID)
	}
	appraisal["evidence_refs"] = refs
	var currentRevision int
	var pad, mood, momentum, regulation, drives, conflicts []byte
	var lastUpdated time.Time
	if err := tx.QueryRow(ctx, `SELECT revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at FROM public.fluctlight_inner_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&currentRevision, &pad, &mood, &momentum, &regulation, &drives, &conflicts, &lastUpdated); err != nil {
		return nil, err
	}
	current := map[string]any{"pad": decodeObject(pad), "mood": decodeObject(mood), "momentum": decodeObject(momentum), "regulation": decodeObject(regulation), "drives": decodeArray(drives), "conflicts": decodeArray(conflicts), "revision": currentRevision, "last_updated_at": lastUpdated.Format(time.RFC3339Nano)}
	resulting, requested, applied := reduceInternalDynamics(current, appraisal)
	newRevision := currentRevision + 1
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_inner_states SET revision=$2,pad=$3,mood=$4,momentum=$5,regulation=$6,last_updated_at=now() WHERE fluctlight_id=$1 AND revision=$7`, fluctlightID, newRevision, jsonBytes(resulting["pad"]), jsonBytes(resulting["mood"]), jsonBytes(resulting["momentum"]), jsonBytes(resulting["regulation"]), currentRevision)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() != 1 {
		return nil, ErrConflict
	}
	appraisalID := "appraisal_" + stableDigest(sourceFactID)
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_appraisals(id,fluctlight_id,source_fact_id,payload,schema_version,model,model_version,prompt_version,evidence_refs,status,revision) VALUES($1,$2,$3,$4,'fluctlight.appraisal.v1','configured','configured','growth.v1',$5,'accepted',$6) ON CONFLICT DO NOTHING`, appraisalID, fluctlightID, sourceFactID, jsonBytes(appraisal), jsonBytes(arrayValue(appraisal["evidence_refs"])), newRevision); err != nil {
		return nil, err
	}
	focusID := "focus_" + stableDigest(sourceFactID)
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_focus_cycles(id,fluctlight_id,source_fact_id,appraisal_id,attention,thought,desire,agency,action_type,action_id,status,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'frozen',$11) ON CONFLICT DO NOTHING`, focusID, fluctlightID, sourceFactID, appraisalID, jsonBytes(stages["attention"]), jsonBytes(stages["thought"]), jsonBytes(stages["desire"]), jsonBytes(stages["agency"]), actionType, nullableString(actionID), newRevision); err != nil {
		return nil, err
	}
	dynamicsID := "dynamics_" + stableDigest(sourceFactID)
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_internal_dynamics(id,fluctlight_id,source_fact_id,previous_state,resulting_state,requested_delta,applied_delta,policy_version,model_version,evidence_refs,status,revision) VALUES($1,$2,$3,$4,$5,$6,$7,'growth.reducer.v1','configured',$8,'applied',$9) ON CONFLICT DO NOTHING`, dynamicsID, fluctlightID, sourceFactID, jsonBytes(current), jsonBytes(resulting), jsonBytes(requested), jsonBytes(applied), jsonBytes(arrayValue(appraisal["evidence_refs"])), newRevision); err != nil {
		return nil, err
	}
	stateRevisionID := "state_revision_" + stableDigest(sourceFactID)
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_state_revisions(id,fluctlight_id,source_event_id,expected_revision,resulting_revision,previous_state,resulting_state,requested_delta,applied_delta,result,reason_code,policy_version,model_version,evidence_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'applied','cognitive_growth','growth.reducer.v1','configured',$10,$11) ON CONFLICT DO NOTHING`, stateRevisionID, fluctlightID, sourceFactID, currentRevision, newRevision, jsonBytes(current), jsonBytes(resulting), jsonBytes(requested), jsonBytes(applied), jsonBytes(arrayValue(appraisal["evidence_refs"])), "state:"+sourceFactID); err != nil {
		return nil, err
	}
	return resulting, nil
}

func cognitiveStagePayload(value map[string]any) map[string]any {
	result := make(map[string]any, len(value)+5)
	if nested := mapValue(value["assessment"]); len(nested) > 0 {
		for key, item := range nested {
			result[key] = item
		}
	}
	for key, item := range value {
		result[key] = item
	}
	return result
}

func normalizeCognitiveStages(value map[string]any) (map[string]any, error) {
	stages := cognitiveStagePayload(value)
	for _, field := range []string{"attention", "thought", "desire", "agency"} {
		if _, err := wakeUpValue(stages[field], field); err != nil {
			return nil, err
		}
	}
	appraisal, err := normalizeAppraisal(stages["appraisal"])
	if err != nil {
		return nil, err
	}
	stages["appraisal"] = appraisal
	return stages, nil
}

func (a *App) ProcessNativeCognitionFact(ctx context.Context, inboxID string) error {
	var fluctlightID, eventType string
	var payload []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id,event_type,payload FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&fluctlightID, &eventType, &payload); err != nil {
		return err
	}
	var existing bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_appraisals WHERE source_fact_id=$1)`, inboxID).Scan(&existing); err != nil {
		return err
	}
	if existing {
		return nil
	}
	var ownerID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerID); err != nil {
		return err
	}
	projection, err := a.BuildContextProjection(ctx, ownerID, fluctlightID, "", inboxID, "")
	if err != nil {
		return err
	}
	completion, err := a.Provider.StructuredWithToolsSchema(ctx, "cognitive_assessment", []map[string]any{
		{"role": "system", "content": nativeCognitionInstruction},
		{"role": "user", "content": jsonString(map[string]any{"event_type": eventType, "fact": json.RawMessage(payload), "context": compactCognitionContext(projection)})},
	}, a.capabilityRegistry().Manifests(), "native_cognition_response", nativeCognitionResponseSchema(), true)
	if err != nil {
		return err
	}
	stages, err := normalizeCognitiveStages(completion.Structured)
	if err != nil {
		return err
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		_, err := a.persistCognitiveStagesTx(ctx, tx, fluctlightID, inboxID, stages, "no_op", "")
		return err
	})
}

func (a *App) persistActionResult(ctx context.Context, fluctlightID, actionID, sourceFactID string, result ToolResultV1) error {
	resultID := "action_result_" + stableDigest(actionID+":"+result.ToolCallID)
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_action_results WHERE action_id=$1)`, actionID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_action_results(id,fluctlight_id,action_id,source_fact_id,status,output,error_code,evidence_refs) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(action_id) DO NOTHING`, resultID, fluctlightID, actionID, sourceFactID, result.Status, jsonBytes(result.Output), nullableString(result.ErrorCode), jsonBytes([]any{sourceFactID})); err != nil {
			return err
		}
		payload := map[string]any{"action_id": actionID, "source_fact_id": sourceFactID, "result": result}
		factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", payload, "action-result:"+actionID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:action:"+actionID, "reflection:action:"+actionID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": actionID})); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "autonomy.result.recorded", "fluctlight", fluctlightID, fluctlightID, actionID, "action-result:"+actionID, "action-result:"+actionID, payload)
	})
}

func appendProcessedCognitionFactTx(ctx context.Context, tx pgx.Tx, fluctlightID, eventType string, payload map[string]any, idempotency string) (string, error) {
	factID := "fact_" + stableDigest(fluctlightID+":"+idempotency)
	var existing string
	if err := tx.QueryRow(ctx, `SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key=$2 FOR UPDATE`, fluctlightID, idempotency).Scan(&existing); err == nil {
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,1,0) ON CONFLICT DO NOTHING`, fluctlightID); err != nil {
		return "", err
	}
	var sequence int
	if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.cognition_inbox_heads WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&sequence); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads SET next_sequence=$2,last_processed_sequence=GREATEST(last_processed_sequence,$3) WHERE fluctlight_id=$1`, fluctlightID, sequence+1, sequence); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,processed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now(),'processed',now())`, factID, fluctlightID, sequence, eventType, jsonBytes(payload), idempotency, eventType+":"+idempotency, idempotency); err != nil {
		return "", err
	}
	return factID, nil
}
