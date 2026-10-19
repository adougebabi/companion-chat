package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/url"
	"sort"
	"strings"
)

func inventoryRequestKind(value string) string {
	return map[string]string{"borrow": "inventory_borrow", "return": "inventory_return", "remove": "inventory_remove", "outfit": "inventory_outfit"}[value]
}
func (s *OwnerSurface) inventoryRequest(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	kind := inventoryRequestKind(stringValue(body["kind"]))
	if kind == "" {
		return nil, invalid("unknown inventory action")
	}
	allowed := map[string]bool{"kind": true, "expected_revision": true, "reason": true, "idempotency_key": true}
	specific := map[string][]string{"inventory_borrow": {"lender", "items"}, "inventory_return": {"item_ids"}, "inventory_remove": {"slots"}, "inventory_outfit": {"outfit_id", "expected_outfit_revision", "profile_id"}}[kind]
	for _, key := range specific {
		allowed[key] = true
	}
	for key := range body {
		if !allowed[key] {
			return nil, invalid("unexpected inventory action field")
		}
	}
	requestBody := cloneMap(body)
	requestBody["title"] = map[string]string{"inventory_borrow": "借入物品请求", "inventory_return": "归还借入物品请求", "inventory_remove": "脱下指定部位请求", "inventory_outfit": "整套换装请求"}[kind]
	request, err := s.insertRequest(ctx, tx, owner, op.fid, kind, requestBody)
	if err != nil {
		return nil, err
	}
	return withStatus(202, map[string]any{"request": request.Payload, "audit_id": request.Audit}), nil
}
func (a *App) validateOwnerInventoryRequestTx(ctx context.Context, tx pgx.Tx, owner, fid, kind string, body map[string]any) error {
	expected, ok := parseRevision(body["expected_revision"])
	if !ok {
		return invalid("wardrobe revision required")
	}
	var current int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&current); err != nil {
		return err
	}
	if current != expected {
		return conflict("wardrobe", expected, current)
	}
	switch kind {
	case "inventory_borrow":
		lender := strings.TrimSpace(stringValue(body["lender"]))
		members := arrayValue(body["items"])
		if lender == "" || len([]rune(lender)) > 256 || len(members) < 1 || len(members) > 8 {
			return invalid("lender and 1–8 explicit loan items required")
		}
		seen := map[string]bool{}
		for _, raw := range members {
			item := mapValue(raw)
			for key := range item {
				if key != "item_kind" && key != "category" && key != "slot" && key != "description" {
					return invalid("loan item fields invalid")
				}
			}
			for _, key := range []string{"item_kind", "category", "slot", "description"} {
				if _, ok := item[key].(string); !ok {
					return invalid("loan item fields must be explicit text")
				}
			}
			effect, err := validateWardrobeEventEffect("wardrobe_gain", map[string]any{"item_kind": item["item_kind"], "category": item["category"], "slot": item["slot"], "description": item["description"], "ownership": "borrowed"})
			if err != nil {
				return invalid("invalid loan item")
			}
			identity := stableDigest(jsonString(effect))
			if seen[identity] {
				return invalid("duplicate loan item")
			}
			seen[identity] = true
		}
	case "inventory_return":
		ids := decisionServiceRefValues(body["item_ids"])
		if len(ids) < 1 || len(ids) > 8 || len(ids) != len(arrayValue(body["item_ids"])) {
			return invalid("1–8 distinct item IDs required")
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				return invalid("duplicate return item")
			}
			seen[id] = true
			var eligible bool
			err := tx.QueryRow(ctx, `SELECT i.ownership='borrowed' AND i.availability='available' AND i.archived_at IS NULL AND `+inventorySourceVerifiedSQL+` FROM public.fluctlight_wardrobe_items i WHERE i.fluctlight_id=$1 AND i.id=$2`, fid, id).Scan(&eligible)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return err
			}
			if !eligible {
				return &SurfaceError{Status: 409, Code: "borrowed_item_not_returnable", Message: "只能归还当前有可靠来源、可用且借入的物品"}
			}
		}
	case "inventory_remove":
		slots := decisionServiceRefValues(body["slots"])
		if len(slots) < 1 || len(slots) > 16 || len(slots) != len(arrayValue(body["slots"])) {
			return invalid("1–16 wearable slots required")
		}
		seen := map[string]bool{}
		for _, slot := range slots {
			if seen[slot] || len(slot) > 64 {
				return invalid("invalid or duplicate slot")
			}
			seen[slot] = true
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND slot=$2)`, fid, slot).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return &SurfaceError{Status: 409, Code: "wearing_slot_changed", Message: "选中部位当前没有穿着记录，请重新读取"}
			}
		}
	case "inventory_outfit":
		profile, err := newWardrobeService(a).effectiveProfileID(ctx, tx, fid, "")
		if err != nil {
			return err
		}
		if body["profile_id"] != profile {
			return &SurfaceError{Status: 409, Code: "outfit_profile_changed", Message: "当前人格已变化，请重新选择套装"}
		}
		expected, ok := parseRevision(body["expected_outfit_revision"])
		if !ok || expected < 1 {
			return invalid("outfit revision required")
		}
		var revision int
		var scope string
		err = tx.QueryRow(ctx, `SELECT revision,profile_id FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND id=$2 FOR SHARE`, fid, body["outfit_id"]).Scan(&revision, &scope)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if (scope != "" && scope != profile) || revision != expected {
			return &SurfaceError{Status: 409, Code: "outfit_revision_conflict", Message: "套装或人格范围已变化，请重新读取"}
		}
		rows, err := tx.Query(ctx, `SELECT i.id,r.slot=i.slot AND i.item_kind='wearable' AND i.availability='available' AND i.archived_at IS NULL AND (i.ownership IN ('owned','borrowed') OR EXISTS(SELECT 1 FROM public.fluctlight_worn_items w WHERE w.fluctlight_id=i.fluctlight_id AND w.item_id=i.id)) AND `+inventorySourceVerifiedSQL+` FROM public.fluctlight_wardrobe_outfit_items r JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=r.fluctlight_id AND i.id=r.item_id WHERE r.fluctlight_id=$1 AND r.outfit_id=$2 ORDER BY r.slot`, fid, body["outfit_id"])
		if err != nil {
			return err
		}
		ids := []any{}
		for rows.Next() {
			var id string
			var eligible bool
			if err := rows.Scan(&id, &eligible); err != nil {
				rows.Close()
				return err
			}
			if !eligible {
				rows.Close()
				return &SurfaceError{Status: 409, Code: "outfit_items_unavailable", Message: "套装包含当前不可穿的物品，请先处理库存；套装定义保持"}
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(ids) == 0 || len(ids) > 16 {
			return invalid("outfit has no usable recorded members")
		}
		body["item_ids"] = ids
	}
	return nil
}
func ownerInventoryArgumentsMatch(kind string, request, args map[string]any) bool {
	switch kind {
	case "inventory_borrow":
		if strings.TrimSpace(stringValue(args["lender"])) != strings.TrimSpace(stringValue(request["lender"])) {
			return false
		}
		actual, want := arrayValue(args["items"]), arrayValue(request["items"])
		if len(actual) != len(want) {
			return false
		}
		normalize := func(items []any) []string {
			result := []string{}
			for _, raw := range items {
				item := mapValue(raw)
				result = append(result, jsonString(map[string]any{"item_kind": firstString(item["item_kind"], "wearable"), "category": strings.TrimSpace(stringValue(item["category"])), "slot": strings.TrimSpace(stringValue(item["slot"])), "description": strings.TrimSpace(stringValue(item["description"]))}))
			}
			return result
		}
		return sameStringSet(stringSliceAny(normalize(actual)), func() []string { v := normalize(want); sort.Strings(v); return v }())
	case "inventory_return":
		return sameStringSet(args["item_ids"], decisionServiceRefValues(request["item_ids"]))
	case "inventory_remove":
		return args["mode"] == "partial" && len(arrayValue(args["item_ids"])) == 0 && sameStringSet(args["remove_slots"], decisionServiceRefValues(request["slots"]))
	case "inventory_outfit":
		return args["mode"] == "full" && len(arrayValue(args["remove_slots"])) == 0 && sameStringSet(args["item_ids"], decisionServiceRefValues(request["item_ids"]))
	}
	return false
}
func (a *App) OwnerInventoryOutfits(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	profile, err := newWardrobeService(a).effectiveProfileID(ctx, a.DB.Pool(), fid, "")
	if err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"inventory-outfits", owner, fid, profile}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid outfit cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT o.id,o.created_at,to_jsonb(o)||jsonb_build_object('items',(SELECT COALESCE(jsonb_agg(to_jsonb(i)||jsonb_build_object('saved_slot',r.slot,'definition_slot_matches',r.slot=i.slot) ORDER BY r.slot),'[]'::jsonb) FROM public.fluctlight_wardrobe_outfit_items r JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=r.fluctlight_id AND i.id=r.item_id WHERE r.fluctlight_id=o.fluctlight_id AND r.outfit_id=o.id)) FROM public.fluctlight_wardrobe_outfits o WHERE o.fluctlight_id=$1 AND (o.profile_id='' OR o.profile_id=$2) AND ($3='' OR (o.created_at,o.id)<($4,$3)) ORDER BY o.created_at DESC,o.id DESC LIMIT $5`, fid, profile, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	page, err := readGovernanceRows(rows, scope, limit)
	rows.Close()
	if err != nil {
		return nil, err
	}
	page["profile_id"] = profile
	return page, nil
}

func (a *App) OwnerInventoryOutfit(ctx context.Context, owner, fid, id string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	profile, err := newWardrobeService(a).effectiveProfileID(ctx, a.DB.Pool(), fid, "")
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = a.DB.Pool().QueryRow(ctx, `SELECT to_jsonb(o)||jsonb_build_object('active_profile_id',$3::text,'items',(SELECT COALESCE(jsonb_agg(to_jsonb(i)||jsonb_build_object('saved_slot',r.slot,'definition_slot_matches',r.slot=i.slot) ORDER BY r.slot),'[]'::jsonb) FROM public.fluctlight_wardrobe_outfit_items r JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=r.fluctlight_id AND i.id=r.item_id WHERE r.fluctlight_id=o.fluctlight_id AND r.outfit_id=o.id)) FROM public.fluctlight_wardrobe_outfits o WHERE o.fluctlight_id=$1 AND o.id=$2 AND (o.profile_id='' OR o.profile_id=$3)`, fid, id, profile).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}
