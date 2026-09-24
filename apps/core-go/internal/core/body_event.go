package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func validateBodyEventEffect(kind string, raw any) (map[string]any, error) {
	effect := mapValue(raw)
	if kind != "body_injury" && kind != "body_recovery" {
		if raw != nil {
			return nil, errors.New("body_effect_requires_body_event_kind")
		}
		return nil, nil
	}
	if len(effect) == 0 {
		return nil, errors.New("body_effect_required")
	}
	if kind == "body_injury" {
		part := strings.TrimSpace(stringValue(effect["body_part"]))
		description := strings.TrimSpace(stringValue(effect["description"]))
		severity := strings.TrimSpace(stringValue(effect["severity"]))
		impact := strings.TrimSpace(stringValue(effect["impact"]))
		if part == "" || len([]rune(part)) > 128 || description == "" || len([]rune(description)) > 500 ||
			(severity != "minor" && severity != "moderate" && severity != "significant") ||
			(impact != "pain" && impact != "mobility_limited" && impact != "none") {
			return nil, errors.New("body_injury_effect_invalid")
		}
		return map[string]any{"operation": "injury", "body_part": part, "description": description, "severity": severity, "impact": impact}, nil
	}
	injuryEventID := strings.TrimSpace(stringValue(effect["injury_event_id"]))
	if injuryEventID == "" || len([]rune(injuryEventID)) > 128 {
		return nil, errors.New("body_recovery_effect_invalid")
	}
	return map[string]any{"operation": "recover", "injury_event_id": injuryEventID}, nil
}

func applyBodyEffectFromConfirmedEventTx(ctx context.Context, tx pgx.Tx, fluctlightID, eventID, kind string, effect map[string]any) (int, error) {
	if len(effect) == 0 {
		return 0, nil
	}
	var storedKind, status string
	var resultRaw []byte
	if err := tx.QueryRow(ctx, `SELECT kind,status,result FROM public.life_events WHERE id=$1 AND fluctlight_id=$2`, eventID, fluctlightID).Scan(&storedKind, &status, &resultRaw); err != nil {
		return 0, err
	}
	storedEffect := mapValue(decodeObject(resultRaw)["body_effect"])
	if storedKind != kind || status != "confirmed" || stableDigest(jsonString(storedEffect)) != stableDigest(jsonString(effect)) {
		return 0, errors.New("body_event_source_invalid")
	}
	var revision int
	var stateRaw []byte
	if err := tx.QueryRow(ctx, `SELECT revision,state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision, &stateRaw); err != nil {
		return 0, err
	}
	fields := decodeObject(stateRaw)
	injuries := append([]any(nil), arrayValue(mapValue(fields["injuries"])["value"])...)
	if kind == "body_injury" {
		injuries = append(injuries, map[string]any{"id": eventID, "body_part": effect["body_part"], "description": effect["description"],
			"severity": effect["severity"], "impact": effect["impact"], "status": "active"})
	} else {
		found := false
		for index, raw := range injuries {
			injury := cloneMap(mapValue(raw))
			if stringValue(injury["id"]) != stringValue(effect["injury_event_id"]) {
				continue
			}
			if stringValue(injury["status"]) != "active" {
				return 0, errors.New("body_injury_already_resolved")
			}
			injury["status"] = "recovered"
			injury["recovery_event_id"] = eventID
			injuries[index] = injury
			found = true
			break
		}
		if !found {
			return 0, errors.New("body_injury_source_not_found")
		}
	}
	fields["injuries"] = map[string]any{"status": "known", "value": injuries}
	newRevision := revision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_appearance_states SET revision=$2,state_json=$3,source_kind='accepted_event',source_ref=$4,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, newRevision, jsonBytes(fields), eventID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_appearance_revisions(fluctlight_id,revision,state_json,source_kind,source_ref) VALUES($1,$2,$3,'accepted_event',$4)`, fluctlightID, newRevision, jsonBytes(fields), eventID); err != nil {
		return 0, err
	}
	if err := appendOutboxTx(ctx, tx, "appearance.updated", "fluctlight", fluctlightID, fluctlightID, eventID,
		"appearance:"+fluctlightID, "appearance-event:"+eventID, map[string]any{"event_id": eventID, "revision": newRevision, "change": kind}); err != nil {
		return 0, err
	}
	return newRevision, nil
}
