package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const outfitResourceReferencesSQL = `SELECT jsonb_build_object('fluctlight_id',o.fluctlight_id,'profile_id',o.profile_id,'runtime_id',o.id,'running_version',o.source_version,'instance_revision',o.revision,'target_kind','wardrobe_outfit','status','saved','overrides',o.source_overrides) FROM public.fluctlight_wardrobe_outfits o JOIN public.fluctlights f ON f.id=o.fluctlight_id WHERE o.source_resource_id=$1 AND f.created_by_actor_id=$2 ORDER BY o.id`

func validateOutfitDefinition(body map[string]any) error {
	if body["schema"] != "outfit-definition.v1" || !eventKeys(body, "schema", "description", "members") || containsPermanentHarm(body) {
		return invalid("Expected safe outfit-definition.v1")
	}
	description, ok := body["description"].(string)
	if !ok || len([]rune(description)) > 2000 {
		return invalid("Outfit description must be explicit text up to 2000 characters")
	}
	members, ok := body["members"].([]any)
	if !ok || len(members) < 1 || len(members) > 16 {
		return invalid("Outfit requires 1–16 exact published item references")
	}
	seen := map[string]bool{}
	for _, raw := range members {
		m, ok := raw.(map[string]any)
		if !ok || !eventKeys(m, "resource_id", "resource_version") {
			return invalid("Outfit members must be exact item definition references")
		}
		id, version := strings.TrimSpace(stringValue(m["resource_id"])), strings.TrimSpace(stringValue(m["resource_version"]))
		if id == "" || len(id) > 128 || version == "" || len(version) > 64 || seen[id+":"+version] {
			return invalid("Outfit member identity is invalid or repeated")
		}
		seen[id+":"+version] = true
	}
	return nil
}

func outfitPublishedMembers(ctx context.Context, q lifeContextQuerier, owner string, body map[string]any) ([]any, error) {
	if err := validateOutfitDefinition(body); err != nil {
		return nil, err
	}
	seenSlots := map[string]bool{}
	result := []any{}
	for _, raw := range arrayValue(body["members"]) {
		member := mapValue(raw)
		p, err := readWorldPublication(ctx, q, owner, stringValue(member["resource_id"]), stringValue(member["resource_version"]))
		if err != nil {
			return nil, err
		}
		if p.Kind != "item" {
			return nil, invalid("Outfit member must refer to a published item definition")
		}
		if err := validateItemDefinition(p.Body); err != nil {
			return nil, err
		}
		slot := stringValue(p.Body["slot"])
		if p.Body["item_kind"] != "wearable" || seenSlots[slot] {
			return nil, invalid("Outfit members must be wearable with different slots")
		}
		seenSlots[slot] = true
		result = append(result, map[string]any{"resource_id": p.ID, "resource_version": p.Version, "name": p.Name, "slot": slot, "description": p.Body["description"]})
	}
	return result, nil
}

func (a *App) applyStudioOutfit(ctx context.Context, owner, id string, body map[string]any, p worldPublication, preview bool) (map[string]any, error) {
	fid := stringValue(body["fluctlight_id"])
	f, err := a.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		return nil, err
	}
	members, err := outfitPublishedMembers(ctx, a.DB.Pool(), owner, p.Body)
	if err != nil {
		return nil, err
	}
	service := newWardrobeService(a)
	read := func(ctx context.Context, q DBTX) (map[string]any, string, int, error) {
		profile, err := service.effectiveProfileID(ctx, q, fid, "")
		if err != nil {
			return nil, "", 0, err
		}
		var wardrobe int
		if err := q.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fid).Scan(&wardrobe); err != nil {
			return nil, "", 0, err
		}
		var target string
		err = q.QueryRow(ctx, `SELECT id FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND profile_id=$2 AND source_resource_id=$3 ORDER BY updated_at DESC,id DESC LIMIT 1`, fid, profile, id).Scan(&target)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, profile, wardrobe, nil
		}
		if err != nil {
			return nil, "", 0, err
		}
		current, err := readInventoryOutfitWith(ctx, q, fid, target)
		return current, profile, wardrobe, err
	}
	diff := func(current map[string]any) []any {
		rows := []any{}
		if current == nil || current["name"] != p.Name {
			rows = append(rows, map[string]any{"field": "name", "from": current["name"], "to": p.Name})
		}
		rows = append(rows, map[string]any{"field": "members", "from": current["items"], "to": members})
		return rows
	}
	current, profile, wardrobe, err := read(ctx, a.DB.Pool())
	if err != nil {
		return nil, err
	}
	if preview {
		for _, raw := range members {
			m := mapValue(raw)
			candidates, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('item_id',i.id,'description',i.description,'ownership',i.ownership,'availability',i.availability,'exact_source',i.source_resource_id=$3 AND i.source_version=$4,'source_verified',`+inventorySourceVerifiedSQL+`) FROM public.fluctlight_wardrobe_items i WHERE i.fluctlight_id=$1 AND i.item_kind='wearable' AND i.slot=$2 AND i.archived_at IS NULL ORDER BY (i.source_resource_id=$3 AND i.source_version=$4) DESC NULLS LAST,i.id LIMIT 31`, fid, m["slot"], m["resource_id"], m["resource_version"])
			if err != nil {
				return nil, err
			}
			m["more_candidates"] = len(candidates) > 30
			if len(candidates) > 30 {
				candidates = candidates[:30]
			}
			m["candidates"] = candidates
		}
		return map[string]any{"available": true, "target_kind": "wardrobe_outfit", "profile_id": profile, "wardrobe_revision": wardrobe, "runtime_revision": intValue(current["revision"]), "runtime_id": current["id"], "instance_revision": f.CurrentRevision, "members": members, "diff": diff(current), "conflicts": []any{}, "semantics": "为每个定义成员明确选择实际库存服饰，保存当前人格的套装组合；不取得、不穿着。自选同槽替代会记录为局部差异，原取得来源不改写。"}, nil
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "studio_outfit_apply")
	return a.worldCommand(ctx, owner, fid, "apply-outfit:"+id, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if err := lockFoundationSourceTx(ctx, tx, owner, p); err != nil {
			return nil, err
		}
		var foundation int
		if err := tx.QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1 FOR UPDATE`, fid).Scan(&foundation); err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_instance_revision"])
		if !ok || expected != foundation {
			return nil, conflict("fluctlight", expected, foundation)
		}
		current, profile, wardrobe, err := read(ctx, tx)
		if err != nil {
			return nil, err
		}
		if body["profile_id"] != profile {
			return nil, &SurfaceError{Status: 409, Code: "outfit_profile_changed", Message: "目标人格已变化，请重新读取搭配差异"}
		}
		expected, ok = parseRevision(body["expected_wardrobe_revision"])
		if !ok || expected != wardrobe {
			return nil, conflict("wardrobe", expected, wardrobe)
		}
		expectedOutfit, ok := parseRevision(body["expected_runtime_revision"])
		if !ok || expectedOutfit != intValue(current["revision"]) {
			return nil, conflict("outfit", expectedOutfit, intValue(current["revision"]))
		}
		choices := map[string]string{}
		for _, raw := range arrayValue(body["choices"]) {
			c := mapValue(raw)
			key, value := stringValue(c["field"]), stringValue(c["decision"])
			if (key != "name" && key != "members") || choices[key] != "" || (value != "keep" && value != "override") {
				return nil, invalid("Invalid outfit application choice")
			}
			choices[key] = value
		}
		if current != nil {
			for _, raw := range diff(current) {
				if choices[stringValue(mapValue(raw)["field"])] == "" {
					return nil, &SurfaceError{Status: 409, Code: "apply_choices_required", Message: "请明确保留或替换套装名称及成员"}
				}
			}
		}
		name := p.Name
		if choices["name"] == "keep" && current != nil {
			name = stringValue(current["name"])
		}
		ids := []string{}
		if choices["members"] == "keep" && current != nil {
			for _, raw := range arrayValue(current["items"]) {
				ids = append(ids, stringValue(mapValue(raw)["id"]))
			}
		} else {
			mapping := arrayValue(body["member_items"])
			if len(mapping) != len(members) {
				return nil, invalid("Select one real inventory item for each definition member")
			}
			selected := map[string]string{}
			for _, raw := range mapping {
				m := mapValue(raw)
				if !eventKeys(m, "resource_id", "resource_version", "item_id") {
					return nil, invalid("Invalid outfit member mapping")
				}
				key := stringValue(m["resource_id"]) + ":" + stringValue(m["resource_version"])
				if selected[key] != "" {
					return nil, invalid("Repeated outfit mapping")
				}
				selected[key] = stringValue(m["item_id"])
			}
			for _, raw := range members {
				m := mapValue(raw)
				item := selected[stringValue(m["resource_id"])+":"+stringValue(m["resource_version"])]
				var valid bool
				if err := tx.QueryRow(ctx, `SELECT item_kind='wearable' AND slot=$3 AND archived_at IS NULL FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 FOR SHARE`, fid, item, m["slot"]).Scan(&valid); err != nil {
					return nil, err
				}
				if !valid {
					return nil, invalid("Selected inventory item must be a recorded wearable in the definition slot")
				}
				ids = append(ids, item)
			}
		}
		audit, _ := ctx.Value(worldCommandAuditKey{}).(string)
		result, err := service.saveOutfitTx(ctx, tx, wardrobeOutfitWrite{FluctlightID: fid, ProfileID: profile, OutfitID: stringValue(current["id"]), Name: name, ItemIDs: ids, OperationID: "studio-outfit:" + stringValue(body["idempotency_key"]), SourceRef: audit, ExpectedWardrobeRevision: wardrobe, ExpectedOutfitRevision: expectedOutfit, DefinitionSource: &p})
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": result["outfit_id"], "status": "applied", "applied": true, "target_kind": "wardrobe_outfit", "resource_id": p.ID, "resource_version": p.Version, "instance_revision": foundation, "runtime_revision": result["revision"], "wardrobe_revision": result["wardrobe_revision"], "conflicts": []any{}, "life_write": false, "semantics": "已保存有来源的真实库存组合，未取得或穿着"}, nil
	})
}

func refreshOutfitDefinitionSourceTx(ctx context.Context, tx pgx.Tx, fid, id string, source *worldPublication) error {
	if source != nil {
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_outfits SET source_resource_id=$3,source_version=$4 WHERE fluctlight_id=$1 AND id=$2`, fid, id, source.ID, source.Version); err != nil {
			return err
		}
	}
	saved, err := readInventoryOutfitWith(ctx, tx, fid, id)
	if err != nil {
		return err
	}
	sourceID := stringValue(saved["source_resource_id"])
	if sourceID == "" {
		return nil
	}
	var owner string
	if err := tx.QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fid).Scan(&owner); err != nil {
		return err
	}
	p, err := readWorldPublication(ctx, tx, owner, sourceID, stringValue(saved["source_version"]))
	if err != nil {
		return err
	}
	if p.Kind != "outfit" {
		return invalid("Saved outfit source must be an outfit definition")
	}
	members, err := outfitPublishedMembers(ctx, tx, owner, p.Body)
	if err != nil {
		return err
	}
	bySlot := map[string]map[string]any{}
	for _, raw := range members {
		m := mapValue(raw)
		bySlot[stringValue(m["slot"])] = m
	}
	overrides := map[string]any{}
	if saved["name"] != p.Name {
		overrides["name"] = saved["name"]
	}
	different := len(arrayValue(saved["items"])) != len(members)
	for _, raw := range arrayValue(saved["items"]) {
		item := mapValue(raw)
		m := bySlot[stringValue(item["slot"])]
		different = different || m == nil || item["source_resource_id"] != m["resource_id"] || item["source_version"] != m["resource_version"]
	}
	if different {
		overrides["members"] = saved["items"]
	}
	_, err = tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_outfits SET source_overrides=$3 WHERE fluctlight_id=$1 AND id=$2`, fid, id, jsonBytes(overrides))
	return err
}
