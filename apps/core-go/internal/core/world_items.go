package core

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

const itemResourceReferencesSQL = `SELECT jsonb_build_object('fluctlight_id',i.fluctlight_id,'world_id',i.world_id,'runtime_id',i.id,'running_version',i.source_version,'instance_revision',i.revision,'target_kind','world_item','status','registered','available',i.available) FROM public.world_items i JOIN public.life_worlds w ON w.id=i.world_id WHERE i.source_resource_id=$1 AND w.owner_actor_id=$2 ORDER BY i.id`

func validateItemDefinition(body map[string]any) error {
	if body["schema"] != "item-definition.v1" || !eventKeys(body, "schema", "item_kind", "category", "slot", "description") || containsPermanentHarm(body) {
		return invalid("Expected safe item-definition.v1")
	}
	for _, key := range []string{"item_kind", "category", "slot", "description"} {
		if _, ok := body[key].(string); !ok {
			return invalid("Item definition fields must be explicit text")
		}
	}
	if _, err := normalizeShoppingItem(body); err != nil {
		return invalid("Item definition kind/category/slot/description is invalid")
	}
	return nil
}

func readWorldItemWith(ctx context.Context, q lifeContextQuerier, fid, id string, lock bool) (map[string]any, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT to_jsonb(i) FROM public.world_items i WHERE world_id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1) AND id=$2`+suffix, fid, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}

func worldItemValues(p worldPublication) map[string]any {
	return map[string]any{"name": p.Name, "definition": p.Body}
}

func (a *App) applyStudioItem(ctx context.Context, owner, id string, body map[string]any, p worldPublication, preview bool) (map[string]any, error) {
	fid := stringValue(body["fluctlight_id"])
	f, err := a.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		return nil, err
	}
	if err := validateItemDefinition(p.Body); err != nil {
		return nil, err
	}
	read := func(ctx context.Context, q lifeContextQuerier) (map[string]any, string, int, error) {
		var world string
		var binding int
		if err := q.QueryRow(ctx, `SELECT world_id,revision FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1`, fid).Scan(&world, &binding); err != nil {
			return nil, "", 0, err
		}
		var raw []byte
		err := q.QueryRow(ctx, `SELECT to_jsonb(i) FROM public.world_items i WHERE world_id=$1 AND source_resource_id=$2`, world, id).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, world, binding, nil
		}
		return decodeObject(raw), world, binding, err
	}
	current, world, binding, err := read(ctx, a.DB.Pool())
	if err != nil {
		return nil, err
	}
	diffs := func(current map[string]any) []any {
		rows := []any{}
		for _, key := range []string{"name", "definition"} {
			if jsonString(current[key]) != jsonString(worldItemValues(p)[key]) {
				rows = append(rows, map[string]any{"field": key, "from": current[key], "to": worldItemValues(p)[key]})
			}
		}
		return rows
	}
	if preview {
		return map[string]any{"available": true, "target_kind": "world_item", "world_id": world, "world_binding_revision": binding, "diff": diffs(current), "conflicts": []any{}, "instance_revision": f.CurrentRevision, "runtime_revision": intValue(current["revision"]), "runtime_id": current["id"], "semantics": "只注册世界可取得定义，不入库、不穿着；实际持有仍需完成虚拟取得行动"}, nil
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "world_item_apply")
	return a.worldCommand(ctx, owner, fid, "apply-world-item:"+id, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if err := lockFoundationSourceTx(ctx, tx, owner, p); err != nil {
			return nil, err
		}
		var actual int
		if err := tx.QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fid, owner).Scan(&actual); err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_instance_revision"])
		if !ok || actual != expected {
			return nil, conflict("fluctlight", expected, actual)
		}
		current, world, binding, err := read(ctx, tx)
		if err != nil {
			return nil, err
		}
		worldExpected, worldValid := parseRevision(body["expected_world_revision"])
		if stringValue(body["world_id"]) != world || !worldValid || worldExpected != binding {
			return nil, &SurfaceError{Status: 409, Code: "world_binding_conflict", Message: "目标运行世界已变化，请重新核对差异"}
		}
		expected, ok = parseRevision(body["expected_runtime_revision"])
		if !ok || expected != intValue(current["revision"]) {
			return nil, conflict("world_item", expected, intValue(current["revision"]))
		}
		values := worldItemValues(p)
		choices := map[string]string{}
		for _, raw := range arrayValue(body["choices"]) {
			c := mapValue(raw)
			key, decision := stringValue(c["field"]), stringValue(c["decision"])
			if (key != "name" && key != "definition") || choices[key] != "" || (decision != "keep" && decision != "override") {
				return nil, invalid("Invalid item definition choice")
			}
			choices[key] = decision
		}
		if current != nil {
			for _, raw := range diffs(current) {
				key := stringValue(mapValue(raw)["field"])
				switch choices[key] {
				case "keep":
					values[key] = current[key]
				case "override":
				default:
					return nil, &SurfaceError{Status: 409, Code: "apply_choices_required", Message: "请选择如何处理每个物品定义差异"}
				}
			}
		}
		if err := validateItemDefinition(mapValue(values["definition"])); err != nil {
			return nil, err
		}
		runtimeID := stringValue(current["id"])
		if runtimeID == "" {
			runtimeID = randomID("world_item_")
		}
		overrides := map[string]any{}
		for _, key := range []string{"name", "definition"} {
			if jsonString(values[key]) != jsonString(worldItemValues(p)[key]) {
				overrides[key] = values[key]
			}
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `INSERT INTO public.world_items AS i(id,fluctlight_id,world_id,source_resource_id,source_version,name,definition,local_overrides) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(world_id,source_resource_id) DO UPDATE SET source_version=excluded.source_version,name=excluded.name,definition=excluded.definition,local_overrides=excluded.local_overrides,revision=i.revision+1,updated_at=now() RETURNING to_jsonb(i)`, runtimeID, fid, world, id, p.Version, values["name"], jsonBytes(values["definition"]), jsonBytes(overrides)).Scan(&raw); err != nil {
			return nil, err
		}
		saved := decodeObject(raw)
		if err := recordWorldItemRevisionTx(ctx, tx, owner, stringValue(body["reason"]), current, saved); err != nil {
			return nil, err
		}
		return map[string]any{"id": runtimeID, "status": "applied", "applied": true, "target_kind": "world_item", "resource_id": id, "resource_version": p.Version, "instance_revision": actual, "runtime_revision": saved["revision"], "world_id": world, "conflicts": []any{}, "life_write": false}, nil
	})
}

func recordWorldItemRevisionTx(ctx context.Context, tx pgx.Tx, owner, reason string, before, after map[string]any) error {
	var previous any
	if before != nil {
		previous = jsonBytes(before)
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.world_item_revisions(item_id,revision,actor_id,reason,before_value,after_value) VALUES($1,$2,$3,$4,$5,$6)`, after["id"], intValue(after["revision"]), owner, reason, previous, jsonBytes(after))
	return err
}

func (a *App) worldItemSurface(ctx context.Context, method string, p []string, query url.Values, owner, fid string, body map[string]any) (map[string]any, error) {
	if method == "GET" {
		if len(p) == 5 {
			return readWorldItemWith(ctx, a.DB.Pool(), fid, p[4], false)
		}
		world, err := worldIDWith(ctx, a.DB.Pool(), fid)
		if err != nil {
			return nil, err
		}
		limit, err := reflectionReadPage(query)
		if err != nil {
			return nil, err
		}
		search := strings.TrimSpace(query.Get("q"))
		if len([]rune(search)) > 200 {
			return nil, invalid("Search is too long")
		}
		scope := stableDigest(jsonString([]any{"world-items", owner, fid, world, search}))
		cursor, err := decodeOwnerReadCursor(query.Get("cursor"), scope)
		if err != nil {
			return nil, err
		}
		rows, err := a.DB.Pool().Query(ctx, `SELECT i.id,i.created_at,to_jsonb(i) FROM public.world_items i WHERE i.world_id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1) AND i.world_id=$2 AND ($3='' OR strpos(lower(i.name||' '||(i.definition->>'description')),lower($3))>0) AND ($4='' OR (i.created_at,i.id)<($5,$4)) ORDER BY i.created_at DESC,i.id DESC LIMIT $6`, fid, world, search, cursor.ID, cursor.Time, limit+1)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		page, err := readGovernanceRows(rows, scope, limit)
		if err == nil {
			page["world_id"] = world
		}
		return page, err
	}
	if method != "PUT" || len(p) != 5 {
		return nil, ErrNotFound
	}
	return a.worldCommand(ctx, owner, fid, "world-item-config:"+p[4], body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if !eventKeys(body, "expected_revision", "available", "place_id", "reason", "idempotency_key") {
			return nil, invalid("Unknown catalogue configuration field")
		}
		before, err := readWorldItemWith(ctx, tx, fid, p[4], true)
		if err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || expected != intValue(before["revision"]) {
			return nil, conflict("world_item", expected, intValue(before["revision"]))
		}
		available, ok := body["available"].(bool)
		if !ok {
			return nil, invalid("available is required")
		}
		if value, present := body["place_id"]; !present {
			return nil, invalid("place_id is required (null means unrestricted)")
		} else if value != nil {
			if _, ok := value.(string); !ok {
				return nil, invalid("place_id must be a stable ID or null")
			}
		}
		place := strings.TrimSpace(stringValue(body["place_id"]))
		if place != "" {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_places WHERE id=$1 AND world_id=$2 AND status='active')`, place, before["world_id"]).Scan(&valid); err != nil {
				return nil, err
			}
			if !valid {
				return nil, invalid("Select a place in this catalogue world")
			}
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `UPDATE public.world_items SET available=$3,place_id=$4,revision=revision+1,updated_at=now() WHERE id=$1 AND world_id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$2) RETURNING to_jsonb(world_items)`, p[4], fid, available, nilIfEmpty(place)).Scan(&raw); err != nil {
			return nil, err
		}
		after := decodeObject(raw)
		if err := recordWorldItemRevisionTx(ctx, tx, owner, stringValue(body["reason"]), before, after); err != nil {
			return nil, err
		}
		return after, nil
	})
}
