package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const worldFrameworkInstruction = `
世界定义必须包括主题背景（background）、世界规则（rules）和 framework={schema:'world-framework.v1',venues:[{key,name,places:[{key,name,alias,floor,description,atmosphere,furniture,layout}]}]}。
这是“世界→固定场地分类→具体地点”的定义框架。场地分类如家、商店、公共设施，分类不是可进入的地点；具体地点如卧室、浴室、客厅、服装店、小吃店。按用户世界主题（如赛博朋克、魔法中世纪）选择合适名称和环境，不强行套现代城市。补出约3–6个分类、合计8–16个基础地点，说明具体点的楼层、氛围、固定陈设和布局，各字段简短清楚。key是本定义内部局部键，不是运行place_id；不要生成运行ID、在场、NPC接触、媒体URL或生活事实。局部类别名和地点正式名创建后固定，可使用alias作别名。保留已有局部key和正式名，扩展只补缺少地点。未知字段可留空，不因缺少运行地点引用而返回needs_input；此处并不引用现有运行地点。`

func validateWorldFramework(body map[string]any) error {
	raw, present := body["framework"]
	if !present || raw == nil {
		return nil
	} // old shell definitions remain readable
	frame, ok := raw.(map[string]any)
	if !ok || frame["schema"] != "world-framework.v1" {
		return invalid("world framework schema must be world-framework.v1")
	}
	venues, ok := frame["venues"].([]any)
	if !ok || len(venues) == 0 || len(venues) > 32 {
		return invalid("world framework needs venue categories")
	}
	categories := map[string]bool{}
	places := map[string]bool{}
	total := 0
	for _, raw := range venues {
		venue := mapValue(raw)
		key, name := strings.TrimSpace(stringValue(venue["key"])), strings.TrimSpace(stringValue(venue["name"]))
		if key == "" || name == "" || len([]rune(key)) > 80 || len([]rune(name)) > 128 || categories[key] {
			return invalid("world framework category identity invalid")
		}
		categories[key] = true
		points, ok := venue["places"].([]any)
		if !ok {
			return invalid("world framework places must be an array")
		}
		for _, raw := range points {
			point := mapValue(raw)
			key := strings.TrimSpace(stringValue(point["key"]))
			name := strings.TrimSpace(stringValue(point["name"]))
			identity := jsonString([]string{strings.TrimSpace(stringValue(venue["key"])), key})
			if key == "" || name == "" || len([]rune(key)) > 80 || len([]rune(name)) > 256 || places[identity] {
				return invalid("world framework place identity invalid")
			}
			places[identity] = true
			total++
			for field, limit := range map[string]int{"description": 8000, "alias": 256, "floor": 128, "atmosphere": 2000, "layout": 8000} {
				if _, err := boundedWorldText(point, field, limit, false); err != nil {
					return err
				}
			}
			if value, ok := point["furniture"]; ok && value != nil && !boundedWorldCollectionOrText(value) {
				return invalid("world framework furniture invalid")
			}
			for _, forbidden := range []string{"id", "place_id", "world_id", "parent_place_id", "connection_place_ids", "npc_id"} {
				if point[forbidden] != nil {
					return invalid("framework uses definition-local keys, not runtime IDs")
				}
			}
		}
	}
	if total == 0 || total > 200 {
		return invalid("world framework needs 1–200 concrete places")
	}
	return nil
}

// The exact published source is expanded only inside the owning world command.
// Reapplying appends missing local keys and never renames/replaces existing
// categories, runtime place IDs, local descriptions or current Life facts.
func (a *App) materializeWorldFrameworkTx(ctx context.Context, tx pgx.Tx, fid, world string, p worldPublication) (map[string]any, error) {
	if err := validateWorldFramework(p.Body); err != nil {
		return nil, err
	}
	frame := mapValue(p.Body["framework"])
	if len(frame) == 0 {
		return map[string]any{"created_places": 0}, nil
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT venue_categories FROM public.life_worlds WHERE id=$1 AND owner_actor_id=(SELECT created_by_actor_id FROM public.fluctlights WHERE id=$2) FOR UPDATE`, world, fid).Scan(&raw); err != nil {
		return nil, err
	}
	var categories []any
	if err := json.Unmarshal(raw, &categories); err != nil {
		return nil, err
	}
	if categories == nil {
		return nil, errors.New("world venue categories are not an array")
	}
	known := map[string]string{}
	for _, v := range categories {
		row := mapValue(v)
		known[stringValue(row["id"])] = stringValue(row["name"])
	}
	warnings := []string{}
	for _, v := range arrayValue(frame["venues"]) {
		venue := mapValue(v)
		key, name := strings.TrimSpace(stringValue(venue["key"])), strings.TrimSpace(stringValue(venue["name"]))
		if saved, ok := known[key]; ok {
			if saved != name {
				warnings = append(warnings, "保留原场地分类名称："+saved)
			}
			continue
		}
		known[key] = name
		categories = append(categories, map[string]any{"id": key, "name": name})
	}
	if _, err := tx.Exec(ctx, `UPDATE public.life_worlds SET venue_categories=$2 WHERE id=$1`, world, jsonBytes(categories)); err != nil {
		return nil, err
	}
	created := 0
	for _, v := range arrayValue(frame["venues"]) {
		venue := mapValue(v)
		category := strings.TrimSpace(stringValue(venue["key"]))
		for _, pointRaw := range arrayValue(venue["places"]) {
			point := mapValue(pointRaw)
			key := jsonString([]string{category, strings.TrimSpace(stringValue(point["key"]))})
			var existing string
			err := tx.QueryRow(ctx, `SELECT id FROM public.world_places WHERE world_id=$1 AND framework_key=$2`, world, key).Scan(&existing)
			if err == nil {
				continue
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
			fields := cloneMap(point)
			for _, field := range []string{"furniture", "connections", "room_items"} {
				if value, present := fields[field]; present && value == nil {
					delete(fields, field)
				}
			}
			delete(fields, "key")
			fields["world_id"] = world
			fields["venue_category_id"] = category
			result, err := a.writeWorldPlaceTx(ctx, tx, fid, "", "POST", fields)
			if err != nil {
				return nil, err
			}
			source := map[string]any{"resource_id": p.ID, "version": p.Version, "category_key": category, "place_key": strings.TrimSpace(stringValue(point["key"]))}
			if _, err = tx.Exec(ctx, `UPDATE public.world_places SET framework_key=$2,details=details||jsonb_build_object('framework_source',$3::jsonb) WHERE id=$1`, result["id"], key, jsonBytes(source)); err != nil {
				return nil, err
			}
			created++
		}
	}
	return map[string]any{"created_places": created, "venue_categories": categories, "warnings": warnings}, nil
}

func (a *App) worldApplyTarget(ctx context.Context, q lifeContextQuerier, fid string, p worldPublication, body map[string]any) (map[string]any, error) {
	if p.Kind != "world" || stringValue(body["runtime_world_id"]) == "" {
		return a.worldApplyCurrent(ctx, q, fid, p)
	}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT to_jsonb(w) FROM public.life_worlds w WHERE id=$1 AND owner_actor_id=(SELECT created_by_actor_id FROM public.fluctlights WHERE id=$2)`, body["runtime_world_id"], fid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}
