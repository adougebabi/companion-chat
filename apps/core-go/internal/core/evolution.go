package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (a *App) applySelfModelCandidateTx(ctx context.Context, tx pgx.Tx, fluctlightID string, candidate map[string]any, evidenceRefs []any, sourceWindow string) error {
	category := strings.TrimSpace(stringValue(candidate["category"]))
	claim := strings.TrimSpace(stringValue(candidate["claim"]))
	if category == "" || claim == "" || len([]rune(claim)) > 2000 || len(evidenceRefs) == 0 {
		return errors.New("self_model_candidate_invalid")
	}
	confidence, err := requiredBoundedNumber(candidate["confidence"])
	if err != nil {
		return errors.New("self_model_candidate_confidence_invalid")
	}
	if confidence < 0.7 {
		return errors.New("self_model_candidate_confidence_low")
	}
	evolutionID := "evolution_" + stableDigest(fluctlightID+":"+category+":"+sourceWindow+":"+claim)
	var alreadyApplied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_evolution_revisions WHERE id=$1)`, evolutionID).Scan(&alreadyApplied); err != nil {
		return err
	}
	if alreadyApplied {
		return nil
	}
	var provenance []byte
	var baseRevision int
	if err := tx.QueryRow(ctx, `SELECT provenance,current_revision FROM public.fluctlights WHERE id=$1 FOR UPDATE`, fluctlightID).Scan(&provenance, &baseRevision); err != nil {
		return err
	}
	before := decodeObject(provenance)
	selfModel := mapValue(before["self_model"])
	previous := selfModel[category]
	if previousString := stringValue(previous); previousString == claim || stringValue(mapValue(previous)["claim"]) == claim {
		return nil
	}
	selfModel[category] = map[string]any{"claim": claim, "confidence": confidence, "evidence_refs": evidenceRefs, "source_window": sourceWindow}
	before["self_model"] = selfModel
	newRevision := baseRevision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlights SET provenance=$2,current_revision=$3,updated_at=now() WHERE id=$1 AND current_revision=$4`, fluctlightID, jsonBytes(before), newRevision, baseRevision); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_evolution_revisions(id,fluctlight_id,field,base_revision,revision,candidate_type,before_value,after_value,evidence_refs,source_window,status) VALUES($1,$2,$3,$4,$5,'self_model',$6,$7,$8,$9,'accepted') ON CONFLICT(id) DO NOTHING`, evolutionID, fluctlightID, "self_model."+category, baseRevision, newRevision, jsonBytes(previous), jsonBytes(selfModel[category]), jsonBytes(evidenceRefs), sourceWindow); err != nil {
		return err
	}
	return appendOutboxTx(ctx, tx, "self_model.revised", "fluctlight", fluctlightID, fluctlightID, sourceWindow, "evolution:"+evolutionID, evolutionID, map[string]any{"evolution_id": evolutionID, "field": "self_model." + category, "revision": newRevision, "aggregate_sequence": newRevision})
}

func (a *App) applyPersonalityCandidateTx(ctx context.Context, tx pgx.Tx, fluctlightID string, candidate map[string]any, evidenceRefs []any, sourceWindow string) error {
	trait := strings.TrimSpace(stringValue(candidate["trait"]))
	if trait == "" || len(evidenceRefs) == 0 {
		return errors.New("personality_candidate_invalid")
	}
	value, err := requiredBoundedNumber(candidate["value"])
	if err != nil {
		return errors.New("personality_candidate_value_invalid")
	}
	confidence, err := requiredBoundedNumber(candidate["confidence"])
	if err != nil || confidence < 0.8 {
		return errors.New("personality_candidate_confidence_low")
	}
	evolutionID := "evolution_" + stableDigest(fluctlightID+":personality."+trait+":"+sourceWindow)
	var alreadyApplied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_evolution_revisions WHERE id=$1)`, evolutionID).Scan(&alreadyApplied); err != nil {
		return err
	}
	if alreadyApplied {
		return nil
	}
	var personality []byte
	var baseRevision int
	if err := tx.QueryRow(ctx, `SELECT personality,current_revision FROM public.fluctlights WHERE id=$1 FOR UPDATE`, fluctlightID).Scan(&personality, &baseRevision); err != nil {
		return err
	}
	before := decodeObject(personality)
	previous, previousOK := numberFloat(before[trait])
	if previousOK && (value-previous > 0.1 || previous-value > 0.1) {
		return errors.New("personality_candidate_delta_too_large")
	}
	before[trait] = value
	newRevision := baseRevision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlights SET personality=$2,current_revision=$3,updated_at=now() WHERE id=$1 AND current_revision=$4`, fluctlightID, jsonBytes(before), newRevision, baseRevision); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_evolution_revisions(id,fluctlight_id,field,base_revision,revision,candidate_type,before_value,after_value,evidence_refs,source_window,status) VALUES($1,$2,$3,$4,$5,'personality',$6,$7,$8,$9,'accepted') ON CONFLICT(id) DO NOTHING`, evolutionID, fluctlightID, "personality."+trait, baseRevision, newRevision, jsonBytes(previous), jsonBytes(value), jsonBytes(evidenceRefs), sourceWindow); err != nil {
		return err
	}
	return appendOutboxTx(ctx, tx, "personality.revised", "fluctlight", fluctlightID, fluctlightID, sourceWindow, "evolution:"+evolutionID, evolutionID, map[string]any{"evolution_id": evolutionID, "field": "personality." + trait, "revision": newRevision, "aggregate_sequence": newRevision})
}

func (a *App) RollbackEvolution(ctx context.Context, actorID, evolutionID string) (map[string]any, error) {
	var fluctlightID, field, status string
	var before []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT e.fluctlight_id,e.field,e.before_value,e.status FROM public.fluctlight_evolution_revisions e JOIN public.fluctlights f ON f.id=e.fluctlight_id WHERE e.id=$1 AND f.created_by_actor_id=$2`, evolutionID, actorID).Scan(&fluctlightID, &field, &before, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status != "accepted" {
		return nil, errors.New("evolution_not_rollbackable")
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var currentRevision int
		var personality, provenance []byte
		if err := tx.QueryRow(ctx, `SELECT current_revision,personality,provenance FROM public.fluctlights WHERE id=$1 FOR UPDATE`, fluctlightID).Scan(&currentRevision, &personality, &provenance); err != nil {
			return err
		}
		beforeValue := decodeJSONValue(before)
		newRevision := currentRevision + 1
		if strings.HasPrefix(field, "self_model.") {
			category := strings.TrimPrefix(field, "self_model.")
			state := decodeObject(provenance)
			selfModel := mapValue(state["self_model"])
			selfModel[category] = beforeValue
			state["self_model"] = selfModel
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlights SET provenance=$2,current_revision=$3,updated_at=now() WHERE id=$1 AND current_revision=$4`, fluctlightID, jsonBytes(state), newRevision, currentRevision); err != nil {
				return err
			}
		} else if strings.HasPrefix(field, "personality.") {
			trait := strings.TrimPrefix(field, "personality.")
			state := decodeObject(personality)
			state[trait] = beforeValue
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlights SET personality=$2,current_revision=$3,updated_at=now() WHERE id=$1 AND current_revision=$4`, fluctlightID, jsonBytes(state), newRevision, currentRevision); err != nil {
				return err
			}
		} else {
			return errors.New("evolution_field_unsupported")
		}
		rollbackID := "evolution_rollback_" + stableDigest(evolutionID+":"+fmt.Sprint(newRevision))
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_evolution_revisions(id,fluctlight_id,field,base_revision,revision,candidate_type,before_value,after_value,evidence_refs,source_window,status) VALUES($1,$2,$3,$4,$5,'rollback',$6,$7,'[]',$8,'accepted')`, rollbackID, fluctlightID, field, currentRevision, newRevision, before, before, evolutionID); err != nil {
			return err
		}
		result = map[string]any{"id": rollbackID, "fluctlight_id": fluctlightID, "field": field, "revision": newRevision, "status": "rolled_back", "source_evolution_id": evolutionID}
		return appendOutboxTx(ctx, tx, "evolution.rolled_back", "fluctlight", fluctlightID, fluctlightID, evolutionID, "evolution:"+rollbackID, rollbackID, map[string]any{"evolution_id": rollbackID, "field": field, "revision": newRevision, "aggregate_sequence": newRevision})
	})
	return result, err
}

func validateEvidenceRefs(refs []any, allowed map[string]struct{}) bool {
	if len(refs) == 0 {
		return false
	}
	for _, raw := range refs {
		value := stringValue(raw)
		if value == "" {
			return false
		}
		if len(allowed) > 0 {
			if _, ok := allowed[value]; !ok {
				return false
			}
		}
	}
	return true
}
