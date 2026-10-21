package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/url"
)

var placeDefaultFields = []string{"name", "description", "alias", "group", "floor", "scene", "atmosphere", "furniture", "layout", "connections", "room_items"}

func placeDefaultsFields(place map[string]any) map[string]any {
	fields := map[string]any{}
	details := mapValue(place["details"])
	for _, key := range placeDefaultFields {
		switch key {
		case "name", "description":
			fields[key] = place[key]
		default:
			if value, ok := details[key]; ok {
				fields[key] = value
			} else {
				fields[key] = ""
			}
		}
	}
	return fields
}
func readDefaultPlaceWith(ctx context.Context, q lifeContextQuerier, fid, world, id string, lock bool) (map[string]any, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF p"
	}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT to_jsonb(p) FROM public.world_places p JOIN public.life_worlds w ON w.id=p.world_id WHERE w.fluctlight_id=$1 AND w.id=$2 AND p.id=$3`+suffix, fid, world, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}
func readPlaceDefaultWith(ctx context.Context, q lifeContextQuerier, place map[string]any, owner, source string) (map[string]any, error) {
	var raw []byte
	var saved map[string]any
	err := q.QueryRow(ctx, `SELECT to_jsonb(d) FROM public.world_place_defaults d WHERE place_id=$1`, place["id"]).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		saved = decodeObject(raw)
	}
	revision := intValue(saved["revision"])
	switch source {
	case "current_saved":
		return map[string]any{"revision": revision, "source_kind": "owner_pinned_saved", "definition": placeDefaultsFields(place), "source_resource_id": nil, "source_version": nil, "semantics": "Owner明确选择的当前已保存字段；不是补造的历史初始值"}, nil
	case "recorded":
		if saved != nil {
			return saved, nil
		}
	case "current_definition":
	default:
		return nil, ErrInvalidArguments
	}
	resource, version := stringValue(place["source_resource_id"]), stringValue(place["source_version"])
	if resource == "" || version == "" {
		if source == "current_definition" {
			return nil, &SurfaceError{Status: 422, Code: "place_definition_source_missing", Message: "该地点没有可验证的已应用定义来源"}
		}
		return nil, nil
	}
	publication, err := readWorldPublication(ctx, q, owner, resource, version)
	if err != nil {
		return nil, err
	}
	if publication.Kind != "place" {
		return nil, invalid("地点来源不是地点定义")
	}
	description, err := boundedWorldText(publication.Body, "description", 8000, false)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{"name": publication.Name, "description": description, "details": publication.Body}
	return map[string]any{"revision": revision, "source_kind": "published_definition", "source_resource_id": resource, "source_version": version, "definition": placeDefaultsFields(fields)}, nil
}
func writePlaceDefaultTx(ctx context.Context, tx pgx.Tx, place map[string]any, definition map[string]any, reason, operation string, before any) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.world_place_defaults(place_id,world_id,source_kind,source_resource_id,source_version,definition) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(place_id) DO UPDATE SET source_kind=EXCLUDED.source_kind,source_resource_id=EXCLUDED.source_resource_id,source_version=EXCLUDED.source_version,definition=EXCLUDED.definition,revision=world_place_defaults.revision+1,updated_at=now()`, place["id"], place["world_id"], definition["source_kind"], definition["source_resource_id"], definition["source_version"], jsonBytes(mapValue(definition["definition"])))
	if err != nil {
		return err
	}
	var revision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.world_place_defaults WHERE place_id=$1`, place["id"]).Scan(&revision); err != nil {
		return err
	}
	source := cloneMap(definition)
	source["revision"] = revision
	return appendPlaceDefaultHistory(ctx, tx, place, source, reason, operation, before, placeDefaultsFields(place))
}
func appendPlaceDefaultHistory(ctx context.Context, tx pgx.Tx, place, source map[string]any, reason, operation string, before, after any) error {
	audit, _ := ctx.Value(worldCommandAuditKey{}).(string)
	var fid, owner string
	if err := tx.QueryRow(ctx, `SELECT w.fluctlight_id,f.created_by_actor_id FROM public.life_worlds w JOIN public.fluctlights f ON f.id=w.fluctlight_id WHERE w.id=$1`, place["world_id"]).Scan(&fid, &owner); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.world_place_default_history(id,world_id,place_id,operation,place_revision,default_revision,before_value,after_value,source,reason,audit_id,owner_actor_id,fluctlight_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, randomID("place_history_"), place["world_id"], place["id"], operation, intValue(place["revision"]), intValue(source["revision"]), jsonBytes(before), jsonBytes(after), jsonBytes(source), reason, nilIfEmpty(audit), owner, fid)
	return err
}
func (a *App) OwnerPlaceDefaults(ctx context.Context, owner, fid, id string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	world := firstString(q.Get("worldId"), q.Get("world_id"))
	place, err := readDefaultPlaceWith(ctx, a.DB.Pool(), fid, world, id, false)
	if err != nil {
		return nil, err
	}
	source := q.Get("source")
	if source == "" {
		source = "recorded"
	}
	baseline, err := readPlaceDefaultWith(ctx, a.DB.Pool(), place, owner, source)
	if err != nil {
		return nil, err
	}
	diff := []any{}
	current := placeDefaultsFields(place)
	if baseline != nil {
		fields := mapValue(baseline["definition"])
		for _, key := range placeDefaultFields {
			if key == "name" {
				continue
			}
			if jsonString(current[key]) != jsonString(fields[key]) {
				diff = append(diff, map[string]any{"field": key, "from": current[key], "to": fields[key]})
			}
		}
	}
	return map[string]any{"place_id": id, "world_id": world, "place_revision": intValue(place["revision"]), "default_revision": intValue(baseline["revision"]), "baseline": baseline, "diff": diff, "source": source, "current": current, "semantics": "仅恢复地点文本、场景与环境描述字段；不移动角色，不改变世界/父地点/连接ID/状态/NPC/日程及历史引用。没有来源的旧地点须先明确设置恢复基准。"}, nil
}
func (a *App) OwnerPlaceDefaultCommand(ctx context.Context, owner, fid, id string, body map[string]any) (map[string]any, error) {
	if !eventKeys(body, "world_id", "expected_revision", "expected_default_revision", "operation", "source", "reason", "idempotency_key") {
		return nil, ErrInvalidArguments
	}
	operation, source := stringValue(body["operation"]), stringValue(body["source"])
	if operation != "restore" && operation != "capture" || operation == "restore" && source != "recorded" || operation == "capture" && !containsString([]string{"current_saved", "current_definition"}, source) {
		return nil, ErrInvalidArguments
	}
	return a.worldCommand(ctx, owner, fid, "place-default:"+id+":"+operation, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		place, err := readDefaultPlaceWith(ctx, tx, fid, stringValue(body["world_id"]), id, true)
		if err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || expected != intValue(place["revision"]) {
			return nil, conflict("place", expected, intValue(place["revision"]))
		}
		baseline, err := readPlaceDefaultWith(ctx, tx, place, owner, source)
		if err != nil {
			return nil, err
		}
		want, ok := parseRevision(body["expected_default_revision"])
		if !ok || want != intValue(baseline["revision"]) {
			return nil, conflict("place_default", want, intValue(baseline["revision"]))
		}
		if baseline == nil {
			return nil, &SurfaceError{Status: 422, Code: "place_default_not_configured", Message: "该地点没有可验证的默认快照，请先明确选择当前已保存字段或已应用定义作为基准"}
		}
		if operation == "capture" {
			if err := writePlaceDefaultTx(ctx, tx, place, baseline, stringValue(body["reason"]), "set_baseline", nil); err != nil {
				return nil, err
			}
			return map[string]any{"id": id, "place_revision": expected, "default_revision": want + 1, "status": "baseline_saved", "life_write": false}, nil
		}
		if intValue(baseline["revision"]) == 0 {
			if err := writePlaceDefaultTx(ctx, tx, place, baseline, stringValue(body["reason"]), "set_baseline", nil); err != nil {
				return nil, err
			}
			baseline["revision"] = 1
		}
		fields := cloneMap(mapValue(baseline["definition"]))
		fields["world_id"] = place["world_id"]
		fields["expected_revision"] = expected
		fields["name"] = place["name"]
		fields["parent_place_id"] = place["parent_place_id"]
		fields["status"] = place["status"]
		result, err := a.writeWorldPlaceTx(context.WithValue(ctx, placeDefaultRestoreKey{}, true), tx, fid, id, "PUT", fields)
		if err != nil {
			return nil, err
		}
		after, err := readDefaultPlaceWith(ctx, tx, fid, stringValue(place["world_id"]), id, false)
		if err != nil {
			return nil, err
		}
		if err := appendPlaceDefaultHistory(ctx, tx, after, baseline, stringValue(body["reason"]), "restore_fields", placeDefaultsFields(place), placeDefaultsFields(after)); err != nil {
			return nil, err
		}
		result["default_revision"] = baseline["revision"]
		result["life_write"] = false
		return result, nil
	})
}
func (a *App) OwnerPlaceDefaultHistory(ctx context.Context, owner, fid, id string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	var available bool
	world := firstString(q.Get("worldId"), q.Get("world_id"))
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_place_default_history WHERE fluctlight_id=$1 AND owner_actor_id=$2 AND world_id=$3 AND place_id=$4) OR EXISTS(SELECT 1 FROM public.world_places p JOIN public.life_worlds w ON w.id=p.world_id WHERE w.fluctlight_id=$1 AND w.id=$3 AND p.id=$4)`, fid, owner, world, id).Scan(&available); err != nil {
		return nil, err
	}
	if !available {
		return nil, ErrNotFound
	}
	scope := stableDigest("place-default-history:" + owner + ":" + fid + ":" + id)
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, ErrDiagnosticsFilterInvalid
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,to_jsonb(h) FROM public.world_place_default_history h WHERE place_id=$1 AND ($2='' OR (created_at,id)<($3,$2)) AND fluctlight_id=$4 AND owner_actor_id=$5 AND world_id=$6 ORDER BY created_at DESC,id DESC LIMIT 31`, id, cursor.ID, cursor.Time, fid, owner, world)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, 30)
}
func matchPlaceDefaults(method, path string) bool {
	p := surfaceParts(path)
	return len(p) >= 6 && p[0] == "fluctlights" && p[2] == "world" && p[3] == "places" && p[5] == "defaults" && (len(p) == 6 && (method == "GET" || method == "POST") || len(p) == 7 && p[6] == "history" && method == "GET")
}
func (a *App) dispatchPlaceDefaults(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	p := surfaceParts(path)
	if method == "POST" {
		return a.OwnerPlaceDefaultCommand(ctx, owner, p[1], p[4], body)
	}
	if len(p) == 7 {
		return a.OwnerPlaceDefaultHistory(ctx, owner, p[1], p[4], q)
	}
	return a.OwnerPlaceDefaults(ctx, owner, p[1], p[4], q)
}

type placeDefaultRestoreKey struct{}

func recordPlaceFieldEditTx(ctx context.Context, tx pgx.Tx, fid, world, id string, before map[string]any, reason string) error {
	if restoring, _ := ctx.Value(placeDefaultRestoreKey{}).(bool); restoring {
		return nil
	}
	after, err := readDefaultPlaceWith(ctx, tx, fid, world, id, false)
	if err != nil {
		return err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT to_jsonb(d) FROM public.world_place_defaults d WHERE place_id=$1`, id).Scan(&raw)
	source := map[string]any{"revision": 0, "state": "unconfigured", "source_kind": "no_verified_default_snapshot"}
	if err == nil {
		source = decodeObject(raw)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if reason == "" {
		reason = "地点字段修改，操作理由见关联审计"
	}
	return appendPlaceDefaultHistory(ctx, tx, after, source, reason, "edit_fields", placeDefaultsFields(before), placeDefaultsFields(after))
}
