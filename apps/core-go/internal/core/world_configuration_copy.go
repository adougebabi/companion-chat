package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Copy saved configuration in one snapshot. Runtime identities, encounters,
// confirmed positions, executions and memories never cross to the new actor.
func (a *App) copyWorldConfigurationTx(ctx context.Context, tx pgx.Tx, fid string, body map[string]any) (map[string]any, error) {
	sourceID, sourceFID := stringValue(body["source_world_id"]), stringValue(body["source_fluctlight_id"])
	expected, ok := parseRevision(body["expected_source_revision"])
	if !ok || expected < 1 || sourceID == "" || sourceFID == "" {
		return nil, ErrInvalidArguments
	}
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT jsonb_build_object('world',to_jsonb(w),'places',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM public.world_places p WHERE p.world_id=w.id),'[]')) FROM public.life_worlds w JOIN public.fluctlights s ON s.id=$3 AND s.created_by_actor_id=w.owner_actor_id JOIN public.fluctlights target ON target.id=$1 AND target.created_by_actor_id=w.owner_actor_id WHERE w.id=$2`, fid, sourceID, sourceFID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	snapshot := decodeObject(raw)
	source := mapValue(snapshot["world"])
	if intValue(source["revision"]) != expected {
		return nil, conflict("source_world", expected, intValue(source["revision"]))
	}
	definition := map[string]any{}
	for _, key := range []string{"name", "background", "rules", "timezone", "flavor", "description", "item_pool", "venue_categories"} {
		definition[key] = source[key]
	}
	if name := strings.TrimSpace(stringValue(body["name"])); name != "" {
		definition["name"] = name
	}
	result, err := a.createCatalogWorldTx(ctx, tx, fid, definition)
	if err != nil {
		return nil, err
	}
	world := stringValue(result["id"])
	places := arrayValue(snapshot["places"])
	placeIDs := map[string]string{}
	for _, v := range places {
		p := mapValue(v)
		placeIDs[stringValue(p["id"])] = randomID("place_")
	}
	mapped := func(ids map[string]string, value any) any {
		if value == nil || stringValue(value) == "" {
			return nil
		}
		return ids[stringValue(value)]
	}
	// Insert every place before linking hierarchy/connections, so order is irrelevant.
	for _, v := range places {
		p := mapValue(v)
		id := placeIDs[stringValue(p["id"])]
		if _, err := tx.Exec(ctx, `INSERT INTO public.world_places(id,world_id,name,description,status,source_resource_id,source_version,framework_key,details) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'{}')`, id, world, p["name"], p["description"], p["status"], p["source_resource_id"], p["source_version"], p["framework_key"]); err != nil {
			return nil, err
		}
	}
	for _, v := range places {
		p := mapValue(v)
		id := placeIDs[stringValue(p["id"])]
		details := cloneMap(mapValue(p["details"]))
		if refs, ok := details["connection_place_ids"]; ok {
			values := []any{}
			for _, ref := range arrayValue(refs) {
				next := mapped(placeIDs, ref)
				if next == nil || next == "" {
					return nil, invalid("source connection leaves its world")
				}
				values = append(values, next)
			}
			details["connection_place_ids"] = values
		}
		if _, err := tx.Exec(ctx, `UPDATE public.world_places SET parent_place_id=$3,details=$4 WHERE id=$1 AND world_id=$2`, id, world, mapped(placeIDs, p["parent_place_id"]), jsonBytes(details)); err != nil {
			return nil, err
		}
		place, err := readDefaultPlaceWith(ctx, tx, fid, world, id, false)
		if err != nil {
			return nil, err
		}
		baseline := map[string]any{"source_kind": "initial_saved", "definition": placeDefaultsFields(place), "source_resource_id": nil, "source_version": nil}
		if err := writePlaceDefaultTx(ctx, tx, place, baseline, "复用已有世界的配置，建立独立地点基准", "initial_saved", nil); err != nil {
			return nil, err
		}
	}
	result["configuration_copied"] = true
	result["copied_places"] = len(places)
	result["history_preserved"] = true
	return result, nil
}
