package core

// StructuredAssembledWithToolsSchema remains the canonical assembled Eino task boundary.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var appraisalFields = []string{"relevance", "goal_congruence", "reward", "loss", "social_threat", "controllability", "responsibility", "relationship_significance", "expected_effect"}

func normalizeAppraisal(value any) (map[string]any, error) {
	appraisal := mapValue(value)
	if len(appraisal) == 0 {
		return nil, errors.New("appraisal_required")
	}
	allowed := map[string]struct{}{"evidence_refs": {}, "event_kind": {}, "direction": {}, "drive_signals": {}}
	for _, field := range appraisalFields {
		allowed[field] = struct{}{}
	}
	for key := range appraisal {
		if _, ok := allowed[key]; !ok {
			return nil, fmt.Errorf("appraisal_field_%s_forbidden", key)
		}
	}
	result := make(map[string]any, len(allowed))
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
	if driveSignals, exists := appraisal["drive_signals"]; exists {
		result["drive_signals"] = driveSignals
	}
	return result, nil
}

func clampGrowth(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < -0.1 {
		return -0.1
	}
	if value > 0.1 {
		return 0.1
	}
	return value
}

func clampUnit(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func reduceInternalDynamics(current map[string]any, appraisal map[string]any) (map[string]any, map[string]any, map[string]any) {
	return reduceInternalDynamicsWithProfile(current, appraisal, defaultAffectProfile(), nil, time.Now().UTC())
}

func reduceInternalDynamicsWithProfile(current map[string]any, appraisal map[string]any, profile AffectProfile, drives []driveSemanticSignal, at time.Time) (map[string]any, map[string]any, map[string]any) {
	reward := numberOrZero(appraisal["reward"])
	loss := numberOrZero(appraisal["loss"])
	threat := numberOrZero(appraisal["social_threat"])
	controllability := numberOrZero(appraisal["controllability"])
	expectedEffect := numberOrZero(appraisal["expected_effect"])
	requested := map[string]float64{
		"pad.pleasure":         reward - 0.5 - loss,
		"pad.arousal":          threat + (expectedEffect-0.5)*0.25,
		"pad.dominance":        controllability - 0.5 - threat*0.5,
		"mood.intensity":       (absFloat(reward-0.5) + loss + threat) / 2,
		"momentum.value":       expectedEffect - 0.5,
		"regulation.stability": controllability - 0.5,
	}
	return reduceAffectState(current, profile, affectReductionInput{Requested: requested, Drives: drives, Source: "appraisal"}, at)
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

func (a *App) persistCognitiveStagesTx(ctx context.Context, tx pgx.Tx, fluctlightID, sourceFactID string, assessment map[string]any, actionType, actionID string, expectedRevision int) (map[string]any, error) {
	stages := cognitiveStagePayload(assessment)
	if err := validateFrozenDecisionInfluences(stages); err != nil {
		return nil, err
	}
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
	causality, err := frozenDecisionCausality(stages)
	if err != nil {
		return nil, err
	}
	for key, value := range causality {
		appraisal[key] = value
	}
	driveSignals, err := frozenDecisionDriveSignals(stages)
	if err != nil {
		return nil, err
	}
	if version := stringValue(stages["context_reference_version"]); version != "" {
		appraisal["context_reference_version"] = version
		appraisal["context_reference_index"] = stages["context_reference_index"]
	}
	var currentRevision int
	var pad, mood, momentum, regulation, drives, conflicts []byte
	var lastUpdated time.Time
	if err := tx.QueryRow(ctx, `SELECT revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at FROM public.fluctlight_inner_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&currentRevision, &pad, &mood, &momentum, &regulation, &drives, &conflicts, &lastUpdated); err != nil {
		return nil, err
	}
	if expectedRevision >= 0 && currentRevision != expectedRevision {
		return nil, ErrConflict
	}
	current := map[string]any{"pad": decodeObject(pad), "mood": decodeObject(mood), "momentum": decodeObject(momentum), "regulation": decodeObject(regulation), "drives": decodeArray(drives), "conflicts": decodeArray(conflicts), "revision": currentRevision, "last_updated_at": lastUpdated.Format(time.RFC3339Nano)}
	var liveProfileRevision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&liveProfileRevision); err != nil {
		return nil, err
	}
	profile, profileProjection, err := a.readAffectProfileTx(ctx, tx, fluctlightID)
	if err != nil {
		return nil, err
	}
	if projection, ok := contextProjectionFromValue(stages["context_projection"]); ok && len(projection.AffectProfile) > 0 {
		if intValue(projection.AffectProfile["revision"]) != liveProfileRevision {
			return nil, newCapabilityError("affect_profile_revision_conflict", false, ErrConflict)
		}
		profile, err = affectProfileFromProjection(projection.AffectProfile)
		if err != nil {
			return nil, err
		}
		profileProjection = cloneMap(projection.AffectProfile)
	}
	current["affect_profile"] = profileProjection
	transitionAt := time.Now().UTC()
	resulting, requested, applied := reduceInternalDynamicsWithProfile(current, appraisal, profile, driveSignals, transitionAt)
	newRevision := currentRevision + 1
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_inner_states SET revision=$2,pad=$3,mood=$4,momentum=$5,regulation=$6,drives=$7,conflicts=$8,last_updated_at=$9 WHERE fluctlight_id=$1 AND revision=$10`, fluctlightID, newRevision, jsonBytes(resulting["pad"]), jsonBytes(resulting["mood"]), jsonBytes(resulting["momentum"]), jsonBytes(resulting["regulation"]), jsonBytes(resulting["drives"]), jsonBytes(resulting["conflicts"]), transitionAt, currentRevision)
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
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_internal_dynamics(id,fluctlight_id,source_fact_id,previous_state,resulting_state,requested_delta,applied_delta,policy_version,model_version,evidence_refs,status,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'configured',$9,'applied',$10) ON CONFLICT DO NOTHING`, dynamicsID, fluctlightID, sourceFactID, jsonBytes(current), jsonBytes(resulting), jsonBytes(requested), jsonBytes(applied), profile.PolicyVersion, jsonBytes(arrayValue(appraisal["evidence_refs"])), newRevision); err != nil {
		return nil, err
	}
	stateRevisionID := "state_revision_" + stableDigest(sourceFactID)
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_state_revisions(id,fluctlight_id,source_event_id,expected_revision,resulting_revision,previous_state,resulting_state,requested_delta,applied_delta,result,reason_code,policy_version,model_version,evidence_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'applied','cognitive_growth',$10,'configured',$11,$12) ON CONFLICT DO NOTHING`, stateRevisionID, fluctlightID, sourceFactID, currentRevision, newRevision, jsonBytes(current), jsonBytes(resulting), jsonBytes(requested), jsonBytes(applied), profile.PolicyVersion, jsonBytes(arrayValue(appraisal["evidence_refs"])), "state:"+sourceFactID); err != nil {
		return nil, err
	}
	return resulting, nil
}

func (a *App) applyFrozenCognitiveStagesTx(ctx context.Context, tx pgx.Tx, fluctlightID, sourceFactID string, assessment map[string]any, actionType, actionID string, expectedRevision int) error {
	if stringValue(cognitiveStagePayload(assessment)["cognitive_state_transition"]) == "not_proposed" {
		return nil
	}
	var existing bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_appraisals WHERE source_fact_id=$1)`, sourceFactID).Scan(&existing); err != nil {
		return err
	}
	if existing {
		// Historical frozen turns may already have committed their appraisal
		// before the unified settlement boundary existed. Preserve that audit row
		// without applying a second state revision during recovery.
		return nil
	}
	_, err := a.persistCognitiveStagesTx(ctx, tx, fluctlightID, sourceFactID, assessment, actionType, actionID, expectedRevision)
	return err
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

// normalizeCognitiveStages validates the optional semantic sidecar that
// accompanies native capability calls. A Provider may legally return a
// capability-only native decision: in that case the sidecar is empty and Core
// must preserve the tool invocation without manufacturing appraisal/state.
func normalizeCognitiveStages(value map[string]any, allowCapabilityOnly bool) (map[string]any, bool, error) {
	stages := cognitiveStagePayload(value)
	semanticFieldsPresent := false
	for _, field := range []string{"attention", "thought", "desire", "agency"} {
		if text := strings.TrimSpace(stringValue(stages[field])); text != "" || len(mapValue(stages[field])) > 0 {
			semanticFieldsPresent = true
			break
		}
	}
	if allowCapabilityOnly && !semanticFieldsPresent && len(mapValue(stages["appraisal"])) == 0 {
		return stages, false, nil
	}
	for _, field := range []string{"attention", "thought", "desire", "agency"} {
		if _, err := wakeUpValue(stages[field], field); err != nil {
			return nil, false, err
		}
	}
	appraisal, err := normalizeAppraisal(stages["appraisal"])
	if err != nil {
		return nil, false, err
	}
	stages["appraisal"] = appraisal
	return stages, true, nil
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
