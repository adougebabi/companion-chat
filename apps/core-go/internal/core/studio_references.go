package core

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
)

// New published/job reference graphs and archive share one short Owner fence.
// Acquire it before a parent resource row so mutable historical types cannot
// deadlock two simultaneous publications that reference each other's old item versions.
func lockStudioReferenceGraphTx(ctx context.Context, tx pgx.Tx, owner string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "studio-reference:"+owner)
	return err
}

func requireActiveStudioMemberTx(ctx context.Context, tx pgx.Tx, owner, id string) error {
	record, found, err := (postgresSurfaceTx{tx}).Get(ctx, owner, id)
	if err != nil {
		return err
	}
	if !found || record.Kind != "studio_resource" {
		return ErrNotFound
	}
	return activeStudioResource(record.Status)
}

func studioReferenceIDs(body map[string]any) map[string]string {
	result := map[string]string{}
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				kind := ""
				switch key {
				case "place_id", "home_place_id", "parent_place_id", "place_ids", "connection_place_ids":
					kind = "place"
				case "npc_id", "npc_ids":
					kind = "npc"
				}
				if kind != "" {
					values := []any{child}
					if list, ok := child.([]any); ok {
						values = list
					}
					for _, raw := range values {
						if id, ok := raw.(string); ok && id != "" {
							result[kind+":"+id] = kind
						}
					}
				} else {
					walk(child)
				}
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(body)
	return result
}

// Only explicit draft references are resolved against Owner-owned definition
// catalogues. This does not read private NPC biography or a Life projection.
func freezeStudioReferencesTx(ctx context.Context, tx pgx.Tx, owner string, body map[string]any) ([]any, error) {
	references := studioReferenceIDs(body)
	keys := make([]string, 0, len(references))
	for key := range references {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	allowed := []any{}
	if body["schema"] == "outfit-definition.v1" {
		for _, raw := range arrayValue(body["members"]) {
			m := mapValue(raw)
			if stringValue(m["resource_id"]) == "" || stringValue(m["resource_version"]) == "" {
				continue
			}
			if err := requireActiveStudioMemberTx(ctx, tx, owner, stringValue(m["resource_id"])); err != nil {
				return nil, err
			}
			p, err := readWorldPublication(ctx, tx, owner, stringValue(m["resource_id"]), stringValue(m["resource_version"]))
			if err != nil {
				return nil, err
			}
			if p.Kind != "item" {
				return nil, invalid("Select a published item definition for the outfit")
			}
			if err := validateItemDefinition(p.Body); err != nil {
				return nil, err
			}
			if p.Body["item_kind"] != "wearable" {
				return nil, invalid("Outfit references must be wearable item definitions")
			}
			allowed = append(allowed, map[string]any{"kind": "item_definition", "id": p.ID, "version": p.Version, "name": p.Name, "slot": p.Body["slot"], "item_kind": p.Body["item_kind"], "description": p.Body["description"]})
		}
	}
	for _, key := range keys {
		kind := references[key]
		id := key[len(kind)+1:]
		table := "world_places"
		if kind == "npc" {
			table = "world_npcs"
		}
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT jsonb_build_object('kind',$3::text,'id',p.id,'world_id',p.world_id,'name',p.name) FROM public.`+table+` p JOIN public.life_worlds w ON w.id=p.world_id WHERE p.id=$1 AND w.owner_actor_id=$2 AND p.status='active'`, id, owner, kind).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		allowed = append(allowed, decodeObject(raw))
	}
	return allowed, nil
}
func validateStudioReferenceProposal(snapshot, out map[string]any) error {
	allowed := map[string]map[string]any{}
	for _, raw := range arrayValue(snapshot["allowed_references"]) {
		r := mapValue(raw)
		allowed[stringValue(r["kind"])+":"+stringValue(r["id"])] = r
	}
	world := ""
	if snapshot["resource_kind"] == "outfit" {
		seenSlots := map[string]bool{}
		for _, raw := range arrayValue(mapValue(out["body"])["members"]) {
			m := mapValue(raw)
			ref, ok := allowed["item_definition:"+stringValue(m["resource_id"])]
			if !ok || ref["version"] != m["resource_version"] || ref["item_kind"] != "wearable" {
				return invalid("Outfit proposal must retain exact verified item definition references")
			}
			slot := stringValue(ref["slot"])
			if slot == "" || seenSlots[slot] {
				return invalid("Outfit proposal must use different verified wearing slots")
			}
			seenSlots[slot] = true
		}
	}
	for key := range studioReferenceIDs(mapValue(out["body"])) {
		r, ok := allowed[key]
		if !ok {
			return invalid("Generated stable reference was not verified in the frozen draft; provide the missing selection")
		}
		next := stringValue(r["world_id"])
		if stringValue(snapshot["resource_kind"]) == "event_template" && world != "" && next != world {
			return invalid("An event proposal cannot combine references from different runtime worlds")
		}
		world = next
	}
	return nil
}
