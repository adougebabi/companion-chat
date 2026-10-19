package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// A catalogue name is descriptive text, never identity evidence. Only an
// explicitly selected active place in this Fluctlight's selected world is valid.
func validateLifePlaceTx(ctx context.Context, tx pgx.Tx, fid, place string) error {
	if place == "" {
		return nil
	}
	if strings.TrimSpace(place) != place || len(place) > 128 {
		return newCapabilityError("life_place_invalid", false, ErrInvalidArguments)
	}
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_places p JOIN public.fluctlight_world_bindings b ON b.world_id=p.world_id WHERE b.fluctlight_id=$1 AND p.id=$2 AND p.status='active')`, fid, place).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return newCapabilityError("life_place_not_available", false, ErrUnauthorized)
	}
	return nil
}

// Continuing an identified scene retains its stable place. Changing only its
// descriptive text cannot drop that identity to bypass availability checks.
func resolveUnscheduledActivityPlace(args, life map[string]any) (string, error) {
	if explicit := strings.TrimSpace(stringValue(args["place_id"])); explicit != "" {
		return explicit, nil
	}
	current := stringValue(life["place_id"])
	if current == "" {
		return "", nil
	}
	for _, field := range []string{"scene", "location"} {
		if proposed := strings.TrimSpace(stringValue(args[field])); proposed != "" && proposed != strings.TrimSpace(stringValue(life[field])) {
			return "", newCapabilityError("life_place_required_for_context_change", false, ErrInvalidArguments)
		}
	}
	return current, nil
}

// All readers consume the same stable identity as Life. A descriptive location
// never establishes a catalogue reference, and switching worlds invalidates an
// otherwise old valid reference rather than moving the character.
func attachStableLifePlaceWith(ctx context.Context, q lifeContextQuerier, fid string, life, schedule map[string]any) error {
	var world string
	var bindingRevision int
	if err := q.QueryRow(ctx, `SELECT world_id,revision FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1`, fid).Scan(&world, &bindingRevision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	life["world_id"], life["world_binding_revision"] = world, bindingRevision
	place := ""
	if life["source"] == "event" {
		if err := q.QueryRow(ctx, `SELECT COALESCE(result->>'place_id','') FROM public.life_events WHERE id=$1 AND fluctlight_id=$2`, life["event_id"], fid).Scan(&place); err != nil {
			return err
		}
	} else if life["source"] == "schedule" {
		for _, raw := range arrayValue(schedule["items"]) {
			item := mapValue(raw)
			if item["id"] == life["schedule_item_id"] {
				place = stringValue(item["place_id"])
				break
			}
		}
	}
	if place == "" {
		return nil
	}
	var valid bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_places WHERE id=$1 AND world_id=$2 AND status='active')`, place, world).Scan(&valid); err != nil {
		return err
	}
	if valid {
		life["place_id"] = place
	}
	return nil
}
