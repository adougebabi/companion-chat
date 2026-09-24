package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func validateWardrobeEventEffect(kind string, raw any) (map[string]any, error) {
	if kind != "wardrobe_gain" && kind != "wardrobe_loss" && kind != "wardrobe_unavailable" {
		if raw != nil {
			return nil, errors.New("wardrobe_effect_requires_wardrobe_event_kind")
		}
		return nil, nil
	}
	effect := mapValue(raw)
	if len(effect) == 0 {
		return nil, errors.New("wardrobe_event_effect_required")
	}
	if kind == "wardrobe_gain" {
		category := strings.TrimSpace(stringValue(effect["category"]))
		slot := strings.TrimSpace(stringValue(effect["slot"]))
		description := strings.TrimSpace(stringValue(effect["description"]))
		ownership := strings.TrimSpace(stringValue(effect["ownership"]))
		if category == "" || slot == "" || description == "" || len([]rune(category)) > 64 || len([]rune(slot)) > 64 || len([]rune(description)) > 512 ||
			(ownership != "owned" && ownership != "borrowed" && ownership != "unknown") {
			return nil, errors.New("wardrobe_gain_effect_invalid")
		}
		return map[string]any{"category": category, "slot": slot, "description": description, "ownership": ownership}, nil
	}
	itemID := strings.TrimSpace(stringValue(effect["item_id"]))
	if itemID == "" || len([]rune(itemID)) > 128 {
		return nil, errors.New("wardrobe_item_effect_invalid")
	}
	return map[string]any{"item_id": itemID}, nil
}

func applyWardrobeEffectFromConfirmedEventTx(ctx context.Context, tx pgx.Tx, fluctlightID, eventID, kind string, effect map[string]any) (map[string]any, error) {
	if len(effect) == 0 {
		return nil, nil
	}
	var storedKind, eventStatus string
	var resultRaw []byte
	if err := tx.QueryRow(ctx, `SELECT kind,status,result FROM public.life_events WHERE id=$1 AND fluctlight_id=$2`, eventID, fluctlightID).Scan(&storedKind, &eventStatus, &resultRaw); err != nil {
		return nil, err
	}
	if storedKind != kind || eventStatus != "confirmed" || stableDigest(jsonString(mapValue(decodeObject(resultRaw)["wardrobe_effect"]))) != stableDigest(jsonString(effect)) {
		return nil, errors.New("wardrobe_event_source_invalid")
	}
	var revision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision); err != nil {
		return nil, err
	}
	output := map[string]any{}
	if kind == "wardrobe_gain" {
		itemID := "wardrobe_" + stableDigest(fluctlightID+"\x1f"+eventID+"\x1fprimary")
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key) VALUES($1,$2,$3,$4,$5,$6,'available','accepted_event',$7,'primary')`, itemID, fluctlightID, effect["category"], effect["slot"], effect["description"], effect["ownership"], eventID); err != nil {
			return nil, err
		}
		output["item_id"] = itemID
	} else {
		itemID := stringValue(effect["item_id"])
		var currentAvailability string
		if err := tx.QueryRow(ctx, `SELECT availability FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 FOR UPDATE`, fluctlightID, itemID).Scan(&currentAvailability); err != nil {
			return nil, err
		}
		if currentAvailability != "available" {
			return nil, errors.New("wardrobe_item_not_available_for_change")
		}
		nextAvailability := "unavailable"
		if kind == "wardrobe_loss" {
			nextAvailability = "lost"
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_items SET availability=$3,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND id=$2`, fluctlightID, itemID, nextAvailability); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, fluctlightID, itemID); err != nil {
			return nil, err
		}
		output["item_id"] = itemID
		output["availability"] = nextAvailability
	}
	newRevision := revision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=$2,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, newRevision); err != nil {
		return nil, err
	}
	output["wardrobe_revision"] = newRevision
	if err := appendOutboxTx(ctx, tx, "wardrobe.event.applied", "fluctlight", fluctlightID, fluctlightID, eventID,
		"wardrobe:"+fluctlightID, "wardrobe-event:"+eventID, map[string]any{"event_id": eventID, "kind": kind, "revision": newRevision}); err != nil {
		return nil, err
	}
	return output, nil
}
