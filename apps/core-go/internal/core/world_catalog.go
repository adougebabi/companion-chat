package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (a *App) worldCatalog(ctx context.Context, fid string) (map[string]any, error) {
	items, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',w.id,'fluctlight_id',w.fluctlight_id,'name',w.name,'background',w.background,'rules',w.rules,'flavor',w.flavor,'description',w.description,'item_pool',w.item_pool,'venue_categories',w.venue_categories,'revision',w.revision,'source_resource_id',w.source_resource_id,'source_version',w.source_version,'active',COALESCE(b.world_id=w.id,false)) FROM public.life_worlds w LEFT JOIN public.fluctlight_world_bindings b ON b.fluctlight_id=w.fluctlight_id WHERE w.fluctlight_id=$1 ORDER BY w.created_at,w.id`, fid)
	if err != nil {
		return nil, err
	}
	var binding []byte
	err = a.DB.Pool().QueryRow(ctx, `SELECT jsonb_build_object('world_id',world_id,'revision',revision) FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1`, fid).Scan(&binding)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var active any
	if len(binding) > 0 {
		active = decodeObject(binding)
	}
	return map[string]any{"items": items, "active_binding": active, "next_cursor": nil}, nil
}

func (a *App) writeWorldCatalogTx(ctx context.Context, tx pgx.Tx, fid, method string, p []string, body map[string]any) (map[string]any, error) {
	if len(p) == 4 && method == "POST" {
		body = cloneMap(body)
		body["catalog_create"] = true
		body["world_id"] = ""
		body["expected_revision"] = 0
		return a.createCatalogWorldTx(ctx, tx, fid, body)
	}
	if len(p) != 5 {
		return nil, ErrInvalidArguments
	}
	id := p[4]
	if method == "PUT" {
		body = cloneMap(body)
		body["world_id"] = id
		body["catalog_create"] = true
		return a.writeWorldTx(ctx, tx, fid, body)
	}
	if method != "DELETE" {
		return nil, ErrInvalidArguments
	}
	var rev int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.life_worlds WHERE fluctlight_id=$1 AND id=$2 FOR UPDATE`, fid, id).Scan(&rev); err != nil {
		return nil, err
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok || rev != expected {
		return nil, conflict("world", expected, rev)
	}
	var referenced bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_world_bindings WHERE world_id=$1) OR EXISTS(SELECT 1 FROM public.world_places WHERE world_id=$1) OR EXISTS(SELECT 1 FROM public.world_npcs WHERE world_id=$1) OR EXISTS(SELECT 1 FROM public.world_encounters WHERE world_id=$1) OR EXISTS(SELECT 1 FROM public.world_event_templates WHERE world_id=$1) OR EXISTS(SELECT 1 FROM public.world_items WHERE world_id=$1)`, id).Scan(&referenced); err != nil {
		return nil, err
	}
	if referenced {
		return nil, worldReferenced("World has an active binding, places, NPCs or history")
	}
	_, err := tx.Exec(ctx, `DELETE FROM public.life_worlds WHERE id=$1`, id)
	return map[string]any{"id": id, "status": "deleted"}, err
}

func (a *App) createCatalogWorldTx(ctx context.Context, tx pgx.Tx, fid string, body map[string]any) (map[string]any, error) {
	name, err := boundedWorldText(body, "name", 256, true)
	if err != nil {
		return nil, err
	}
	background, err := boundedWorldText(body, "background", 16000, false)
	if err != nil {
		return nil, err
	}
	rules, err := boundedWorldText(body, "rules", 8000, false)
	if err != nil {
		return nil, err
	}
	id := randomID("world_")
	if _, err = tx.Exec(ctx, `INSERT INTO public.life_worlds(id,fluctlight_id,name,background,rules) VALUES($1,$2,$3,$4,$5)`, id, fid, name, background, rules); err != nil {
		return nil, err
	}
	if err = writeWorldExtrasTx(ctx, tx, id, body); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "revision": 1, "status": "created", "active": false}, nil
}

func (a *App) selectWorldTx(ctx context.Context, tx pgx.Tx, fid string, body map[string]any) (map[string]any, error) {
	id := stringValue(body["world_id"])
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_worlds WHERE id=$1 AND fluctlight_id=$2)`, id, fid).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	var rev int
	err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&rev)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok || expected != rev {
		return nil, conflict("world_binding", expected, rev)
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_world_bindings(fluctlight_id,world_id,revision) VALUES($1,$2,1) ON CONFLICT(fluctlight_id) DO UPDATE SET world_id=$2,revision=fluctlight_world_bindings.revision+1`, fid, id)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT public.bump_fluctlight_context_generation_with_source($1,'world_binding','UPDATE',NULL)`, fid); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "world_id": id, "revision": rev + 1, "status": "selected", "life_write": false}, nil
}

func writeWorldExtrasTx(ctx context.Context, tx pgx.Tx, id string, body map[string]any) error {
	for key, limit := range map[string]int{"flavor": 1000, "description": 8000} {
		if _, ok := body[key]; !ok {
			continue
		}
		v, err := boundedWorldText(body, key, limit, false)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE public.life_worlds SET `+key+`=$2 WHERE id=$1`, id, v); err != nil {
			return err
		}
	}
	if pool, ok := body["item_pool"]; ok {
		if !boundedWorldCollectionOrText(pool) {
			return invalid("itemPool must be bounded text or an array")
		}
		if _, err := tx.Exec(ctx, `UPDATE public.life_worlds SET item_pool=$2 WHERE id=$1`, id, jsonBytes(pool)); err != nil {
			return err
		}
	}
	if raw, present := body["venue_categories"]; present {
		categories, ok := raw.([]any)
		if !ok || len(categories) > 32 {
			return invalid("venueCategories must be an array of at most 32 categories")
		}
		known, names := map[string]string{}, map[string]bool{}
		for _, value := range categories {
			category, ok := value.(map[string]any)
			if !ok || len(category) != 2 {
				return invalid("category requires only id and name")
			}
			key, err := boundedWorldText(category, "id", 80, true)
			if err != nil {
				return err
			}
			name, err := boundedWorldText(category, "name", 128, true)
			if err != nil {
				return err
			}
			if known[key] != "" || names[name] {
				return invalid("duplicate venue category")
			}
			known[key], names[name] = name, true
			category["id"], category["name"] = key, name
		}
		var previous []byte
		if err := tx.QueryRow(ctx, `SELECT venue_categories FROM public.life_worlds WHERE id=$1 FOR UPDATE`, id).Scan(&previous); err != nil {
			return err
		}
		for _, value := range decodeArray(previous) {
			category := mapValue(value)
			if known[stringValue(category["id"])] != stringValue(category["name"]) {
				return invalid("existing venue categories cannot be renamed or removed")
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.life_worlds SET venue_categories=$2 WHERE id=$1`, id, jsonBytes(categories)); err != nil {
			return err
		}
	}
	return nil
}

func worldReferenced(message string) error {
	return &SurfaceError{Status: 409, Code: "resource_referenced", Message: message}
}

func validatePlaceDetails(ctx context.Context, q lifeContextQuerier, world, id string, body map[string]any) (map[string]any, error) {
	details := map[string]any{}
	if id != "" {
		var raw []byte
		err := q.QueryRow(ctx, `SELECT details FROM public.world_places WHERE id=$1 AND world_id=$2`, id, world).Scan(&raw)
		if err != nil {
			return nil, err
		}
		details = decodeObject(raw)
	}
	if raw, present := body["venue_category_id"]; present {
		category, ok := raw.(string)
		if !ok {
			return nil, invalid("place venue category must be text")
		}
		category = strings.TrimSpace(category)
		if previous := stringValue(details["venue_category_id"]); previous != "" && previous != category {
			return nil, &SurfaceError{Status: 422, Code: "place_category_immutable", Message: "地点所属场地分类固定，不能改名或移动分类。"}
		}
		if category != "" {
			var exists bool
			if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_worlds w,jsonb_array_elements(w.venue_categories) c WHERE w.id=$1 AND c->>'id'=$2)`, world, category).Scan(&exists); err != nil {
				return nil, err
			}
			if !exists {
				return nil, invalid("place venue category is outside this world")
			}
		}
		details["venue_category_id"] = category
	}
	for key, limit := range map[string]int{"alias": 256, "group": 256, "floor": 128, "scene": 1000, "atmosphere": 2000, "layout": 8000} {
		if _, ok := body[key]; ok {
			v, err := boundedWorldText(body, key, limit, false)
			if err != nil {
				return nil, err
			}
			details[key] = v
		}
	}
	for _, key := range []string{"furniture", "connections", "room_items"} {
		if value, ok := body[key]; ok {
			if value == nil {
				delete(details, key)
				continue
			}
			if !boundedWorldCollectionOrText(value) {
				return nil, invalid(key + " must be bounded text or an array")
			}
			details[key] = value
		}
	}
	if value, ok := body["connection_place_ids"]; ok {
		items, valid := value.([]any)
		if !valid || len(items) > 200 {
			return nil, invalid("connectionPlaceIds must be an array")
		}
		details["connection_place_ids"] = items
	}
	for _, raw := range arrayValue(details["connection_place_ids"]) {
		place, ok := raw.(string)
		if !ok || place == "" || place == id {
			return nil, invalid("connection must reference another place")
		}
		var exists bool
		if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_places WHERE id=$1 AND world_id=$2 AND status='active')`, place, world).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, invalid("connection is outside this world")
		}
	}

	return details, nil
}

func (a *App) worldNearby(ctx context.Context, owner, fid string) (map[string]any, error) {
	snapshot, err := a.WorldSnapshot(ctx, owner, fid)
	if err != nil {
		return nil, err
	}
	current := mapValue(snapshot["current_context"])
	place := stringValue(current["place_id"])
	event := stringValue(current["event_id"])
	actors := []any{}
	confirmedActor := ""
	if place != "" && event != "" {
		err = a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(result->>'npc_actor_id','') FROM public.life_events WHERE id=$1 AND fluctlight_id=$2`, event, fid).Scan(&confirmedActor)
		if err != nil {
			return nil, err
		}
	}
	if place != "" {
		for _, raw := range arrayValue(snapshot["npcs"]) {
			npc := mapValue(raw)
			if npc["status"] != "active" || npc["enabled"] == false {
				continue
			}
			if presence := mapValue(npc["current_presence"]); presence["place_id"] == place {
				actors = append(actors, map[string]any{"actor_id": npc["actor_id"], "npc_id": npc["id"], "name": npc["name"], "npc": true, "full_fluctlight": false, "since": presence["start_at"], "presence": "confirmed", "source": "npc_routine_execution", "place_id": place, "presence_id": presence["id"], "character_presence": current["authority_status"]})
				continue
			}
			if confirmedActor != "" && npc["actor_id"] == confirmedActor {
				presence, source := "predicted", "inferred_event"
				if current["authority_status"] == "confirmed" {
					presence, source = "confirmed", "active_event"
				}
				actors = append(actors, map[string]any{"actor_id": npc["actor_id"], "npc_id": npc["id"], "name": npc["name"], "npc": true, "full_fluctlight": false, "since": current["effective_at"], "presence": presence, "source": source, "event_id": event, "place_id": place})
				continue
			}
			plan := mapValue(npc["planned_position"])
			if plan["place_id"] == place {
				actors = append(actors, map[string]any{"actor_id": npc["actor_id"], "npc_id": npc["id"], "name": npc["name"], "presence": "predicted", "npc": true, "full_fluctlight": false, "since": "", "source": plan["source"], "place_id": place})
			}
		}
	}
	return map[string]any{"place_id": nilIfEmpty(place), "actors": actors, "available": true, "location_known": place != ""}, nil
}

func boundedWorldCollectionOrText(value any) bool {
	switch v := value.(type) {
	case string:
		return len([]rune(v)) <= 16000
	case []any:
		return len(v) <= 200 && len(jsonBytes(v)) <= 32000
	}
	return false
}

func worldPlaceReferenced(ctx context.Context, q lifeContextQuerier, world, id string) (bool, error) {
	var used bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_items WHERE world_id=$1 AND place_id=$2) OR EXISTS(SELECT 1 FROM public.world_npcs WHERE world_id=$1 AND (home_place_id=$2 OR confirmed_place_id=$2 OR EXISTS(SELECT 1 FROM jsonb_array_elements(routine) r WHERE r->>'place_id'=$2))) OR EXISTS(SELECT 1 FROM public.world_places WHERE world_id=$1 AND (parent_place_id=$2 OR details->'connection_place_ids' ? $2)) OR EXISTS(SELECT 1 FROM public.world_encounters WHERE place_id=$2) OR EXISTS(SELECT 1 FROM public.world_npc_presence_runs WHERE world_id=$1 AND place_id=$2) OR EXISTS(SELECT 1 FROM public.world_npc_opportunities WHERE world_id=$1 AND place_id=$2) OR EXISTS(SELECT 1 FROM public.world_event_templates t WHERE t.world_id=$1 AND (t.definition->'condition'->'place_ids' ? $2 OR EXISTS(SELECT 1 FROM jsonb_array_elements(t.definition->'effects') e WHERE e->>'place_id'=$2))) OR EXISTS(SELECT 1 FROM public.world_event_runs r WHERE r.world_id=$1 AND (r.definition->'condition'->'place_ids' ? $2 OR EXISTS(SELECT 1 FROM jsonb_array_elements(r.definition->'effects') e WHERE e->>'place_id'=$2))) OR EXISTS(SELECT 1 FROM public.life_events WHERE result->>'place_id'=$2) OR EXISTS(SELECT 1 FROM public.life_schedule_items WHERE place_id=$2 OR action_plan->>'place_id'=$2)`, world, id).Scan(&used)
	return used, err
}
