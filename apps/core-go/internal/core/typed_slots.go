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

var typedSlotSchemas = map[string]struct{}{
	"pressure": {}, "scalar": {}, "categorical": {}, "set": {}, "bounded_object": {},
}

func validateSlotKey(key string) bool {
	return len(key) > 0 && len([]rune(key)) <= 128 && toolNamePattern.MatchString(key)
}

func validateSlotText(value any, max int) (string, error) {
	text := strings.TrimSpace(stringValue(value))
	if text == "" || len([]rune(text)) > max {
		return "", errors.New("slot_text_invalid")
	}
	return text, nil
}

func validateTypedSlotValue(schema string, value any) error {
	if _, ok := typedSlotSchemas[schema]; !ok {
		return errors.New("slot_value_schema_invalid")
	}
	if value == nil {
		return errors.New("slot_value_required")
	}
	switch schema {
	case "pressure":
		object := mapValue(value)
		if len(object) == 0 {
			return errors.New("drive_pressure_invalid")
		}
		for _, field := range []string{"pressure", "salience"} {
			if parsed, ok := numberFloat(object[field]); !ok || parsed < 0 || parsed > 1 {
				return errors.New("drive_pressure_invalid")
			}
		}
		if direction := stringValue(object["direction"]); direction != "" && len([]rune(direction)) > 256 {
			return errors.New("drive_direction_invalid")
		}
	case "scalar":
		switch value.(type) {
		case string, bool, float64, int, json.Number:
		default:
			return errors.New("slot_scalar_invalid")
		}
	case "categorical":
		if object := mapValue(value); len(object) > 0 {
			if selected := stringValue(object["selected"]); selected == "" || len([]rune(selected)) > 512 {
				return errors.New("slot_categorical_invalid")
			}
		} else if selected := stringValue(value); selected == "" || len([]rune(selected)) > 512 {
			return errors.New("slot_categorical_invalid")
		}
	case "set":
		items := arrayValue(value)
		if len(items) == 0 || len(items) > 64 {
			return errors.New("slot_set_invalid")
		}
		for _, item := range items {
			if text := stringValue(item); text == "" || len([]rune(text)) > 512 {
				return errors.New("slot_set_invalid")
			}
		}
	case "bounded_object":
		if len(mapValue(value)) == 0 || len(jsonBytes(value)) > 16000 {
			return errors.New("slot_object_invalid")
		}
	}
	if len(jsonBytes(value)) > 16000 {
		return errors.New("slot_value_too_large")
	}
	return nil
}

func validateSlotCandidate(candidate map[string]any, kind string, allowedEvidence map[string]struct{}) (map[string]any, error) {
	key, err := validateSlotText(candidate["key"], 128)
	if err != nil || !validateSlotKey(key) {
		return nil, errors.New("slot_key_invalid")
	}
	label, err := validateSlotText(candidate["label"], 256)
	if err != nil {
		return nil, errors.New("slot_label_invalid")
	}
	description, err := validateSlotText(candidate["description"], 4000)
	if err != nil {
		return nil, errors.New("slot_description_invalid")
	}
	schema, err := validateSlotText(candidate["value_schema"], 64)
	if err != nil || !isKnownSlotSchema(schema) {
		return nil, errors.New("slot_value_schema_invalid")
	}
	if err := validateTypedSlotValue(schema, candidate["value"]); err != nil {
		return nil, err
	}
	confidence, err := requiredBoundedNumber(candidate["confidence"])
	if err != nil {
		return nil, errors.New("slot_confidence_invalid")
	}
	refs := arrayValue(candidate["evidence_refs"])
	if !validateEvidenceRefs(refs, allowedEvidence) {
		return nil, errors.New("slot_evidence_invalid")
	}
	status := stringValue(candidate["status"])
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "paused" && status != "superseded" {
		return nil, errors.New("slot_status_invalid")
	}
	result := map[string]any{
		"key": key, "label": label, "description": description, "value_schema": schema,
		"value": candidate["value"], "confidence": confidence, "evidence_refs": refs, "status": status,
		"provenance": mapValue(candidate["provenance"]), "update_policy": mapValue(candidate["update_policy"]),
	}
	if supersededBy := stringValue(candidate["superseded_by"]); supersededBy != "" {
		if !validateSlotKey(supersededBy) {
			return nil, errors.New("slot_superseded_by_invalid")
		}
		result["superseded_by"] = supersededBy
	}
	if kind == "drive" {
		result["decay_policy"] = mapValue(candidate["decay_policy"])
	}
	return result, nil
}

func isKnownSlotSchema(value string) bool {
	_, ok := typedSlotSchemas[value]
	return ok
}

func (a *App) applyDriveSlotCandidateTx(ctx context.Context, tx pgx.Tx, fluctlightID string, candidate map[string]any, allowedEvidence map[string]struct{}, sourceWindow string, index int) error {
	validated, err := validateSlotCandidate(candidate, "drive", allowedEvidence)
	if err != nil {
		return err
	}
	key := stringValue(validated["key"])
	slotID := "drive_slot_" + stableDigest(fluctlightID+":"+key)
	idempotency := firstString(candidate["idempotency_key"], "reflection:drive:"+sourceWindow+":"+key+":"+fmt.Sprint(index))
	var alreadyApplied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_drive_revisions WHERE idempotency_key=$1)`, idempotency).Scan(&alreadyApplied); err != nil {
		return err
	}
	if alreadyApplied {
		return nil
	}
	var revision int
	var before []byte
	err = tx.QueryRow(ctx, `SELECT revision,value FROM public.fluctlight_drive_slots WHERE id=$1 FOR UPDATE`, slotID).Scan(&revision, &before)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_drive_slots(id,fluctlight_id,key,label,description,value_schema,value,confidence,evidence_refs,provenance,decay_policy,update_policy,status,revision,superseded_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,0,$14)`, slotID, fluctlightID, key, validated["label"], validated["description"], validated["value_schema"], jsonBytes(validated["value"]), jsonBytes(validated["confidence"]), jsonBytes(validated["evidence_refs"]), jsonBytes(validated["provenance"]), jsonBytes(validated["decay_policy"]), jsonBytes(validated["update_policy"]), validated["status"], nullableString(stringValue(validated["superseded_by"]))); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		revision++
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_drive_slots SET label=$2,description=$3,value_schema=$4,value=$5,confidence=$6,evidence_refs=$7,provenance=$8,decay_policy=$9,update_policy=$10,status=$11,revision=$12,superseded_by=$13,updated_at=now() WHERE id=$1`, slotID, validated["label"], validated["description"], validated["value_schema"], jsonBytes(validated["value"]), jsonBytes(validated["confidence"]), jsonBytes(validated["evidence_refs"]), jsonBytes(validated["provenance"]), jsonBytes(validated["decay_policy"]), jsonBytes(validated["update_policy"]), validated["status"], revision, nullableString(stringValue(validated["superseded_by"]))); err != nil {
			return err
		}
	}
	if err := txExecDriveRevision(ctx, tx, slotID, fluctlightID, revision, before, validated, sourceWindow, idempotency); err != nil {
		return err
	}
	return appendOutboxTx(ctx, tx, "drive.revised", "fluctlight", fluctlightID, fluctlightID, sourceWindow, "drive:"+slotID, "drive-revision:"+idempotency, map[string]any{"slot_id": slotID, "key": key, "revision": revision, "aggregate_sequence": revision})
}

func txExecDriveRevision(ctx context.Context, tx pgx.Tx, slotID, fluctlightID string, revision int, before []byte, candidate map[string]any, sourceWindow, idempotency string) error {
	if len(before) == 0 {
		before = []byte("null")
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_drive_revisions(id,slot_id,fluctlight_id,revision,base_revision,candidate_type,before_value,after_value,evidence_refs,source_window,idempotency_key) VALUES($1,$2,$3,$4,$5,'reflection',$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`, "drive_revision_"+stableDigest(idempotency), slotID, fluctlightID, revision, maxInt(0, revision-1), before, jsonBytes(candidate["value"]), jsonBytes(candidate["evidence_refs"]), sourceWindow, idempotency)
	return err
}

func (a *App) applyPreferenceSlotCandidateTx(ctx context.Context, tx pgx.Tx, fluctlightID string, candidate map[string]any, allowedEvidence map[string]struct{}, sourceWindow string, index int) error {
	validated, err := validateSlotCandidate(candidate, "preference", allowedEvidence)
	if err != nil {
		return err
	}
	key := stringValue(validated["key"])
	slotID := "preference_slot_" + stableDigest(fluctlightID+":"+key)
	idempotency := firstString(candidate["idempotency_key"], "reflection:preference:"+sourceWindow+":"+key+":"+fmt.Sprint(index))
	var alreadyApplied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_preference_revisions WHERE idempotency_key=$1)`, idempotency).Scan(&alreadyApplied); err != nil {
		return err
	}
	if alreadyApplied {
		return nil
	}
	var revision int
	var before []byte
	err = tx.QueryRow(ctx, `SELECT revision,value FROM public.fluctlight_preference_slots WHERE id=$1 FOR UPDATE`, slotID).Scan(&revision, &before)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_preference_slots(id,fluctlight_id,key,label,description,value_schema,value,confidence,evidence_refs,provenance,update_policy,status,revision,superseded_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,0,$13)`, slotID, fluctlightID, key, validated["label"], validated["description"], validated["value_schema"], jsonBytes(validated["value"]), jsonBytes(validated["confidence"]), jsonBytes(validated["evidence_refs"]), jsonBytes(validated["provenance"]), jsonBytes(validated["update_policy"]), validated["status"], nullableString(stringValue(validated["superseded_by"]))); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		revision++
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_preference_slots SET label=$2,description=$3,value_schema=$4,value=$5,confidence=$6,evidence_refs=$7,provenance=$8,update_policy=$9,status=$10,revision=$11,superseded_by=$12,updated_at=now() WHERE id=$1`, slotID, validated["label"], validated["description"], validated["value_schema"], jsonBytes(validated["value"]), jsonBytes(validated["confidence"]), jsonBytes(validated["evidence_refs"]), jsonBytes(validated["provenance"]), jsonBytes(validated["update_policy"]), validated["status"], revision, nullableString(stringValue(validated["superseded_by"]))); err != nil {
			return err
		}
	}
	if len(before) == 0 {
		before = []byte("null")
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_preference_revisions(id,slot_id,fluctlight_id,revision,base_revision,candidate_type,before_value,after_value,evidence_refs,source_window,idempotency_key) VALUES($1,$2,$3,$4,$5,'reflection',$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`, "preference_revision_"+stableDigest(idempotency), slotID, fluctlightID, revision, maxInt(0, revision-1), before, jsonBytes(validated["value"]), jsonBytes(validated["evidence_refs"]), sourceWindow, idempotency)
	if err != nil {
		return err
	}
	return appendOutboxTx(ctx, tx, "preference.revised", "fluctlight", fluctlightID, fluctlightID, sourceWindow, "preference:"+slotID, "preference-revision:"+idempotency, map[string]any{"slot_id": slotID, "key": key, "revision": revision, "aggregate_sequence": revision})
}

func (a *App) applyTriggerPreferenceCandidateTx(ctx context.Context, tx pgx.Tx, fluctlightID string, candidate map[string]any, allowedEvidence map[string]struct{}, sourceWindow string, index int) error {
	key := strings.TrimSpace(stringValue(candidate["key"]))
	if !validateSlotKey(key) {
		return errors.New("trigger_key_invalid")
	}
	value := candidate["value"]
	if len(jsonBytes(value)) == 0 || len(jsonBytes(value)) > 16000 || !validateEvidenceRefs(arrayValue(candidate["evidence_refs"]), allowedEvidence) {
		return errors.New("trigger_candidate_invalid")
	}
	confidence, err := requiredBoundedNumber(candidate["confidence"])
	if err != nil {
		return errors.New("trigger_confidence_invalid")
	}
	idempotency := firstString(candidate["idempotency_key"], "reflection:trigger:"+sourceWindow+":"+key+":"+fmt.Sprint(index))
	evolutionID := "trigger_evolution_" + stableDigest(idempotency)
	var alreadyApplied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_evolution_revisions WHERE id=$1)`, evolutionID).Scan(&alreadyApplied); err != nil {
		return err
	}
	if alreadyApplied {
		return nil
	}
	var revision int
	var before []byte
	slotID := "trigger_preference_" + stableDigest(fluctlightID+":"+key)
	err = tx.QueryRow(ctx, `SELECT revision,value FROM public.fluctlight_trigger_preferences WHERE id=$1 FOR UPDATE`, slotID).Scan(&revision, &before)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_trigger_preferences(id,fluctlight_id,key,trigger_schema,value,confidence,evidence_refs,source_window,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,0)`, slotID, fluctlightID, key, firstString(candidate["trigger_schema"], "semantic.trigger.v1"), jsonBytes(value), jsonBytes(confidence), jsonBytes(arrayValue(candidate["evidence_refs"])), sourceWindow)
	} else if err == nil {
		revision++
		_, err = tx.Exec(ctx, `UPDATE public.fluctlight_trigger_preferences SET trigger_schema=$2,value=$3,confidence=$4,evidence_refs=$5,source_window=$6,revision=$7,status='active',updated_at=now() WHERE id=$1`, slotID, firstString(candidate["trigger_schema"], "semantic.trigger.v1"), jsonBytes(value), jsonBytes(confidence), jsonBytes(arrayValue(candidate["evidence_refs"])), sourceWindow, revision)
	}
	if err != nil {
		return err
	}
	if len(before) == 0 {
		before = []byte("null")
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_evolution_revisions(id,fluctlight_id,field,base_revision,revision,candidate_type,before_value,after_value,evidence_refs,source_window,status) VALUES($1,$2,$3,$4,$5,'trigger_preference',$6,$7,$8,$9,'accepted') ON CONFLICT DO NOTHING`, evolutionID, fluctlightID, "trigger."+key, maxInt(0, revision-1), revision, before, jsonBytes(value), jsonBytes(arrayValue(candidate["evidence_refs"])), sourceWindow)
	if err != nil {
		return err
	}
	return appendOutboxTx(ctx, tx, "trigger.preference.revised", "fluctlight", fluctlightID, fluctlightID, sourceWindow, "trigger:"+slotID, "trigger-revision:"+idempotency, map[string]any{"key": key, "revision": revision, "aggregate_sequence": revision})
}

func (a *App) readDriveSlots(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,key,label,description,value_schema,value,confidence,evidence_refs,provenance,decay_policy,update_policy,status,revision,updated_at FROM public.fluctlight_drive_slots WHERE fluctlight_id=$1 AND status='active' ORDER BY updated_at DESC,key LIMIT 200`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, key, label, description, schema, status string
		var value, confidence, refs, provenance, decayPolicy, updatePolicy []byte
		var revision int
		var updated time.Time
		if err := rows.Scan(&id, &key, &label, &description, &schema, &value, &confidence, &refs, &provenance, &decayPolicy, &updatePolicy, &status, &revision, &updated); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "key": key, "label": label, "description": description, "value_schema": schema, "value": decodeJSONValue(value), "confidence": decodeJSONValue(confidence), "evidence_refs": decodeArray(refs), "provenance": decodeObject(provenance), "decay_policy": decodeObject(decayPolicy), "update_policy": decodeObject(updatePolicy), "status": status, "revision": revision, "updated_at": updated.Format(time.RFC3339Nano)})
	}
	return result, rows.Err()
}

func (a *App) readPreferenceSlots(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,key,label,description,value_schema,value,confidence,evidence_refs,provenance,update_policy,status,revision,updated_at FROM public.fluctlight_preference_slots WHERE fluctlight_id=$1 AND status='active' ORDER BY updated_at DESC,key LIMIT 200`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, key, label, description, schema, status string
		var value, confidence, refs, provenance, updatePolicy []byte
		var revision int
		var updated time.Time
		if err := rows.Scan(&id, &key, &label, &description, &schema, &value, &confidence, &refs, &provenance, &updatePolicy, &status, &revision, &updated); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "key": key, "label": label, "description": description, "value_schema": schema, "value": decodeJSONValue(value), "confidence": decodeJSONValue(confidence), "evidence_refs": decodeArray(refs), "provenance": decodeObject(provenance), "update_policy": decodeObject(updatePolicy), "status": status, "revision": revision, "updated_at": updated.Format(time.RFC3339Nano)})
	}
	return result, rows.Err()
}

func (a *App) readTriggerPreferences(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,key,trigger_schema,value,confidence,evidence_refs,source_window,status,revision,updated_at FROM public.fluctlight_trigger_preferences WHERE fluctlight_id=$1 AND status='active' ORDER BY updated_at DESC,key LIMIT 200`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, key, schema, sourceWindow, status string
		var value, confidence, refs []byte
		var revision int
		var updated time.Time
		if err := rows.Scan(&id, &key, &schema, &value, &confidence, &refs, &sourceWindow, &status, &revision, &updated); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "key": key, "trigger_schema": schema, "value": decodeJSONValue(value), "confidence": decodeJSONValue(confidence), "evidence_refs": decodeArray(refs), "source_window": sourceWindow, "status": status, "revision": revision, "updated_at": updated.Format(time.RFC3339Nano)})
	}
	return result, rows.Err()
}
