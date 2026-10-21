package core

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestWardrobeStandardMetadataAndBodyCoverage(t *testing.T) {
	for _, slot := range []string{"outer", "upper_body", "underwear", "panties", "lower_body", "socks", "shoes", "accessory"} {
		kind, seasons, err := wardrobeMetadata(map[string]any{"item_kind": "wearable", "slot": slot, "clothing_type": slot, "seasons": []any{"spring", "autumn", "spring"}})
		if err != nil || kind != slot || len(seasons) != 2 {
			t.Fatal(kind, seasons, err)
		}
	}
	if _, _, err := wardrobeMetadata(map[string]any{"item_kind": "wearable", "slot": "upper_body", "clothing_type": "shoes"}); err == nil {
		t.Fatal("mismatched type accepted")
	}
	if _, _, err := wardrobeMetadata(map[string]any{"item_kind": "wearable", "slot": "upper_body", "seasons": []any{1}}); err == nil {
		t.Fatal("numeric season accepted")
	}
	if wardrobeCoversBody([]map[string]any{{"slot": "shoes"}, {"slot": "underwear"}}) || !wardrobeCoversBody([]map[string]any{{"slot": "full_body"}}) || !wardrobeCoversBody([]map[string]any{{"slot": "top"}, {"slot": "lower_body"}}) {
		t.Fatal("body coverage changed")
	}
	if wardrobeSlotKey("accessory", "one") == wardrobeSlotKey("accessory", "two") || wardrobeSlotKey("upper_body", "one") != wardrobeSlotKey("upper_body", "two") {
		t.Fatal("slot cardinality incorrect")
	}
}

func TestWardrobeTagsSeasonsAndMultipleAccessoriesPersist(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	base := "/internal/fluctlights/" + f.fluctlightID + "/inventory"
	revision := func() int {
		t.Helper()
		var value int
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	ids := []any{}
	for index, slot := range []string{"upper_body", "lower_body", "accessory", "accessory"} {
		result, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/items", nil, f.ownerID, map[string]any{"expected_revision": revision(), "reason": "registered test garment", "idempotency_key": randomID("garment_"), "item": map[string]any{"item_kind": "wearable", "category": slot, "slot": slot, "clothing_type": slot, "seasons": []any{"spring", "autumn"}, "description": "衣物 " + string(rune('A'+index)), "ownership": "owned", "availability": "available"}})
		if err != nil {
			t.Fatal(err)
		}
		item := mapValue(result["item"])
		if len(arrayValue(item["seasons"])) != 2 || item["clothing_type"] != slot {
			t.Fatal(item)
		}
		ids = append(ids, item["id"])
	}
	body := map[string]any{"name": "居家搭配", "tags": []any{"在家", "外出"}, "item_ids": ids, "profile_id": "default", "expected_revision": revision(), "expected_outfit_revision": 0, "reason": "save without dressing", "idempotency_key": "tagged-outfit"}
	saved, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/outfits", nil, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	outfit := mapValue(saved["outfit"])
	if len(arrayValue(outfit["items"])) != 4 || len(arrayValue(outfit["tags"])) != 2 {
		t.Fatal(outfit)
	}
	before, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(before) != 2 {
		t.Fatal("save dressed the plan", before, err)
	}
	if _, err = f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/outfits", nil, f.ownerID, body); err != nil {
		t.Fatal("same save failed replay", err)
	}
	_, err = f.app.DispatchOwnerSurface(f.ctx, "PUT", base+"/tags", nil, f.ownerID, map[string]any{"tags": []any{"居家", "外出", "上班"}, "renames": map[string]any{"在家": "居家"}, "expected_revision": revision(), "reason": "rename scene tag", "idempotency_key": "rename-scene-tag"})
	if err != nil {
		t.Fatal(err)
	}
	current, err := f.app.OwnerInventoryOutfit(f.ctx, f.ownerID, f.fluctlightID, stringValue(outfit["id"]))
	if err != nil || intValue(current["revision"]) != 2 || !containsString(decisionServiceRefValues(current["tags"]), "居家") {
		t.Fatal(current, err)
	}
	history, err := f.app.OwnerInventoryOutfitHistory(f.ctx, f.ownerID, f.fluctlightID, stringValue(outfit["id"]), url.Values{})
	if err != nil || len(arrayValue(history["items"])) != 2 {
		t.Fatal(history, err)
	}
	receipt, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "multi-accessory-wear", map[string]any{"mode": "full", "item_ids": ids}))
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatal(receipt, err)
	}
	worn, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 4 {
		t.Fatal(worn, err)
	}
	receipt, err = f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "remove-accessories", map[string]any{"mode": "partial", "item_ids": []any{}, "remove_slots": []any{"accessory"}}))
	if err != nil {
		t.Fatal(receipt, err)
	}
	worn, err = readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 2 {
		t.Fatal(worn, err)
	}
	if _, err = f.app.OwnerInventoryOutfit(f.ctx, f.foreignOwnerID, f.fluctlightID, stringValue(outfit["id"])); err == nil {
		t.Fatal("outfit leaked to another owner")
	}
	// Metadata rename keeps archived members, tags and history without re-saving them.
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_items SET archived_at=now() WHERE fluctlight_id=$1 AND id=$2`, f.fluctlightID, ids[0]); err != nil {
		t.Fatal(err)
	}
	renamed, err := f.app.DispatchOwnerSurface(f.ctx, "PATCH", base+"/outfits/"+stringValue(outfit["id"]), nil, f.ownerID, map[string]any{"name": "新名称", "profile_id": "default", "expected_revision": revision(), "expected_outfit_revision": 2, "reason": "只修改名称", "idempotency_key": "metadata-rename"})
	if err != nil || mapValue(renamed["outfit"])["name"] != "新名称" || len(arrayValue(mapValue(renamed["outfit"])["items"])) != 4 {
		t.Fatal(renamed, err)
	}
	inspected, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeInspectCapabilityName, "outfit-season-facts", map[string]any{"operation": "outfit_detail", "outfit_id": outfit["id"]}))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range arrayValue(mapValue(mapValue(inspected.Result.Output)["outfit"])["items"]) {
		item := mapValue(raw)
		if len(arrayValue(item["seasons"])) != 2 || item["source_verified"] != true || item["ownership"] != "owned" {
			t.Fatal(item)
		}
	}
	f.requireNoModelRuns(t)
}

func TestWardrobePreparationCommitAndExplicitRecoveryKeepOriginalFailure(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var preparation string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		preparation, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "test-authorized-initialization")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var revision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	resource, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	proposal := map[string]any{"items": []any{map[string]any{"key": "shirt", "clothing_type": "upper_body", "description": "轻便上衣", "seasons": []any{"summer"}}, map[string]any{"key": "trousers", "clothing_type": "lower_body", "description": "宽松长裤", "seasons": []any{"summer"}}}, "plans": []any{map[string]any{"name": "家中放松", "tags": []any{"在家"}, "item_refs": []any{"shirt", "trousers"}}}, "reason": "按性格准备初始穿搭"}
	result, err := f.app.commitPreparedWardrobe(f.ctx, preparation, f.fluctlightID, f.ownerID, "default", revision, resource.CurrentRevision, proposal)
	if err != nil || result["status"] != "awaiting_wear" {
		t.Fatal(result, err)
	}
	worn, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 2 {
		t.Fatal("definition generation changed wearing", worn, err)
	}
	if _, err = f.app.commitPreparedWardrobe(f.ctx, preparation, f.fluctlightID, f.ownerID, "default", revision, resource.CurrentRevision, proposal); !errors.Is(err, ErrConflict) {
		t.Fatal("stale duplicate initialization not rejected", err)
	}
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_preparations SET status='failed' WHERE id=$1`, preparation); err != nil {
		t.Fatal(err)
	}
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='failed',last_error='original-failure' WHERE intent_id=$1`, "wardrobe_initialize:"+f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	recovered, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/inventory/preparation", nil, f.ownerID, map[string]any{"expected_revision": revision, "reason": "explicit recovery", "idempotency_key": "recover-wardrobe"})
	if err != nil || recovered["status"] != "awaiting_wear" || recovered["task_id"] == "wardrobe_initialize:"+f.fluctlightID {
		t.Fatal(recovered, err)
	}
	var status, lastError string
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT status,last_error FROM public.platform_workflow_intents WHERE intent_id=$1`, "wardrobe_initialize:"+f.fluctlightID).Scan(&status, &lastError); err != nil || status != "failed" || lastError != "original-failure" {
		t.Fatal("failure history changed", status, lastError, err)
	}
	f.requireNoModelRuns(t)
}

func TestWardrobePreparationFormalAgentCommitsInventoryThenRealWearAndReplays(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "wardrobe-preparation-controlled")
	var preparation string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		preparation, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "controlled-owner-source")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	router := newFakeProviderRouter().on("wardrobe_preparation_v1", func(payload map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{Structured: map[string]any{"items": []any{
				map[string]any{"key": "top", "description": "青绿宽松上衣", "clothing_type": "upper_body", "seasons": []any{"spring", "autumn"}}, map[string]any{"key": "bottom", "description": "棉质长裤", "clothing_type": "lower_body", "seasons": []any{"spring", "autumn"}}, map[string]any{"key": "scarf", "description": "柔软围巾", "clothing_type": "accessory", "seasons": []any{"autumn"}}, map[string]any{"key": "necklace", "description": "简洁项链", "clothing_type": "accessory", "seasons": []any{}},
			}, "plans": []any{map[string]any{"name": "轻松居家", "tags": []any{"在家", "休息"}, "item_refs": []any{"top", "bottom", "scarf", "necklace"}}}, "wear_item_refs": []any{}, "reason": "根据实际人格准备虚构初始衣物，尚未穿着"}}
		}
		if calls > 2 {
			t.Fatal("initializer repeated model work", calls)
		}
		rows, err := f.repository.Pool().Query(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_ref=$2 ORDER BY id`, f.fluctlightID, "wardrobe-preparation:"+preparation)
		if err != nil {
			t.Fatal(err)
		}
		ids := []any{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if len(ids) != 4 {
			t.Fatal("selection happened before inventory commit", ids)
		}
		return fakeProviderResult{Structured: map[string]any{"items": []any{}, "plans": []any{}, "wear_item_refs": ids, "reason": "当前情境适合这套已登记的搭配"}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	first, err := f.app.ProcessWardrobePreparation(f.ctx, preparation)
	if err != nil || first["status"] != "awaiting_wear" || first["inventory_committed"] != true || calls != 1 {
		t.Fatal(first, calls, err)
	}
	second, err := f.app.ProcessWardrobePreparation(f.ctx, preparation)
	if err != nil || second["status"] != "completed" || calls != 2 {
		t.Fatal(second, calls, err)
	}
	worn, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 4 {
		t.Fatal("actual tool did not persist wear", worn, err)
	}
	state, err := (&App{DB: f.repository}).OwnerWardrobePreparation(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || state["status"] != "completed" || mapValue(state["result"])["wear_receipt"] == nil {
		t.Fatal(state, err)
	}
	if _, err = (&App{DB: f.repository}).ProcessWardrobePreparation(f.ctx, preparation); err != nil || calls != 2 {
		t.Fatal("completed preparation was regenerated", calls, err)
	}
}

func testWorldFrameworkBody() map[string]any {
	return map[string]any{"background": "温和的魔法世界", "rules": "日常生活", "framework": map[string]any{"schema": "world-framework.v1", "venues": []any{map[string]any{"key": " home ", "name": "家", "places": []any{map[string]any{"key": " bedroom ", "name": "卧室", "floor": "二层", "description": "安静卧室", "atmosphere": "温暖", "furniture": []any{"木床", "书桌"}, "layout": "窗朝东"}}}, map[string]any{"key": "shops", "name": "商店", "places": []any{map[string]any{"key": "clothes", "name": "服装店", "description": "售卖旅人服装", "atmosphere": "明亮", "furniture": nil, "layout": "中央通道"}}}}}}
}

func TestWorldFrameworkPublicationMaterializesAndLocksNames(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	base := "/internal/fluctlights/" + f.fluctlightID
	resource, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "world", "name": "魔法日常", "body": testWorldFrameworkBody(), "reason": "world definition", "idempotency_key": "framework-resource"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(resource["id"])
	publication, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/studio/resources/"+id+"/publish", nil, f.ownerID, map[string]any{"expected_revision": resource["revision"], "reason": "publish framework", "idempotency_key": "framework-publish"})
	if err != nil {
		t.Fatal(err)
	}
	fl, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": publication["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 0, "reason": "apply framework", "idempotency_key": "framework-apply"}
	applied, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/studio/resources/"+id+"/apply", nil, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	world := stringValue(applied["id"])
	snapshot, err := f.app.worldSnapshot(f.ctx, f.ownerID, f.fluctlightID, world)
	if err != nil {
		t.Fatal(err)
	}
	if len(arrayValue(mapValue(snapshot["world"])["venue_categories"])) != 2 || len(arrayValue(snapshot["places"])) != 2 {
		t.Fatal(snapshot)
	}
	place := mapValue(arrayValue(snapshot["places"])[0])
	path := base + "/world/places/" + stringValue(place["id"])
	_, err = f.app.DispatchWorldSurface(f.ctx, "PUT", path, nil, f.ownerID, map[string]any{"world_id": world, "name": "改名", "expected_revision": place["revision"], "reason": "forbidden rename", "idempotency_key": "rename-framework-place"})
	var surface *SurfaceError
	if !errors.As(err, &surface) || surface.Code != "place_name_immutable" {
		t.Fatal("canonical name mutable", err)
	}
	_, err = f.app.DispatchWorldSurface(f.ctx, "PUT", path, nil, f.ownerID, map[string]any{"world_id": world, "name": place["name"], "alias": "常用别名", "floor": "三层", "description": "自定义描述", "atmosphere": "宁静", "furniture": "长椅", "layout": "面向花园", "expected_revision": place["revision"], "reason": "local customization", "idempotency_key": "edit-framework-place"})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": publication["published_version"], "runtime_world_id": world}, true)
	if err != nil {
		t.Fatal(err)
	}
	choices := []any{}
	for _, raw := range arrayValue(preview["diff"]) {
		field := stringValue(mapValue(raw)["field"])
		decision := "keep"
		if field == "framework" {
			decision = "override"
		}
		choices = append(choices, map[string]any{"field": field, "decision": decision})
	}
	_, err = f.app.DispatchWorldApply(f.ctx, f.ownerID, id, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": publication["published_version"], "runtime_world_id": world, "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": preview["runtime_revision"], "choices": choices, "reason": "reapply without replacing local details", "idempotency_key": "framework-reapply"}, false)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = f.app.worldSnapshot(f.ctx, f.ownerID, f.fluctlightID, world)
	if err != nil || len(arrayValue(snapshot["places"])) != 2 {
		t.Fatal(snapshot, err)
	}
	found := false
	for _, raw := range arrayValue(snapshot["places"]) {
		p := mapValue(raw)
		if p["id"] == place["id"] {
			found = true
			if p["alias"] != "常用别名" || p["desc"] != "自定义描述" {
				t.Fatal("framework overwrote local edits", p)
			}
		}
	}
	if !found {
		t.Fatal("place identity lost")
	}
	nearby, err := f.app.worldNearby(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || nearby["place_id"] != nil {
		t.Fatal("framework moved character", nearby, err)
	}
	f.requireNoModelRuns(t)
}

func TestWardrobeSelectionFenceRejectsStaleAndReplaysCommittedReceipt(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var boots string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND slot='shoes'`, f.fluctlightID).Scan(&boots); err != nil {
		t.Fatal(err)
	}
	_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	saved := map[string]any{"wear_fence": map[string]any{"revision": 0, "profile_id": "default", "life_revision": life["context_revision"]}}
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1 WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	request := f.request(wardrobeWearCapabilityName, "stale-initial-selection", map[string]any{"mode": "partial", "item_ids": []any{boots}})
	receipt, err := f.app.ExecuteTool(withWardrobeSelectionFence(f.ctx, saved), request)
	if err != nil || receipt.Result.Status != "handled" || mapValue(receipt.Result.Output)["warning_code"] != "wardrobe_selection_stale" {
		t.Fatal(receipt, err)
	}
	worn, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 2 {
		t.Fatal(worn, err)
	}
	saved["wear_fence"].(map[string]any)["revision"] = 1
	request.OperationID = "fresh-initial-selection"
	receipt, err = f.app.ExecuteTool(withWardrobeSelectionFence(f.ctx, saved), request)
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatal(receipt, err)
	}
	// A later state cannot invalidate proof that this exact operation committed.
	saved["wear_fence"].(map[string]any)["profile_id"] = "obsolete-profile"
	replay, err := f.app.ExecuteTool(withWardrobeSelectionFence(f.ctx, saved), request)
	if err != nil || !replay.Replayed {
		t.Fatal(replay, err)
	}
	worn, err = readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 3 {
		t.Fatal(worn, err)
	}
	f.requireNoModelRuns(t)
}

func TestWardrobeCompleteInventoryRejectsExtraStarterStock(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key) VALUES('complete-body',$1,'dress','full_body','已有连衣裙','owned','available','initialization','fixture-authorized','complete-body')`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	fl, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	proposal := map[string]any{"items": []any{map[string]any{"key": "unneeded", "clothing_type": "upper_body", "description": "不应生成的上衣"}}}
	_, err = f.app.commitPreparedWardrobe(f.ctx, "test-no-extra-stock", f.fluctlightID, f.ownerID, "default", 0, fl.CurrentRevision, proposal)
	if err == nil || err.Error() != "wardrobe_starter_inventory_not_required" {
		t.Fatal(err)
	}
	var count int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 5 {
		t.Fatal(count, err)
	}
	f.requireNoModelRuns(t)
}

func TestWardrobeRemovedProfileRetiresOnlyUncommittedSelection(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var preparation string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		preparation, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "profile-removal-test")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	saved := map[string]any{"inventory_committed": true, "wear_operation": "never-executed-old-profile", "wear_profile_id": "removed-profile", "wear_item_ids": []any{"not-used"}, "wear_fence": map[string]any{"profile_id": "removed-profile"}}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_preparations SET result=$2 WHERE id=$1`, preparation, jsonBytes(saved)); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.finishWardrobePreparationWear(f.ctx, preparation, f.fluctlightID, f.ownerID, "default", saved)
	if err != nil || result["status"] != "awaiting_wear" {
		t.Fatal(result, err)
	}
	state, err := f.app.OwnerWardrobePreparation(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	next := mapValue(state["result"])
	if next["inventory_committed"] != true || next["wear_operation"] != nil || next["wear_fence"] != nil || mapValue(next["last_unapplied_selection"])["operation_id"] != "never-executed-old-profile" {
		t.Fatal(state)
	}
	var effects int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND operation_id='never-executed-old-profile'`, f.fluctlightID).Scan(&effects); err != nil || effects != 0 {
		t.Fatal(effects, err)
	}
	f.requireNoModelRuns(t)
}

func TestWardrobeRemovedProfileStillReplaysExistingWearEffect(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var preparation, boots string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		preparation, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "receipt-profile-test")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET core_persona='{"personality_system":{"profiles":[{"id":"default"},{"id":"former"}]}}'::jsonb WHERE id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND slot='shoes'`, f.fluctlightID).Scan(&boots); err != nil {
		t.Fatal(err)
	}
	saved := map[string]any{"inventory_committed": true, "wear_operation": "committed-former-profile", "wear_profile_id": "former", "wear_item_ids": []any{boots}}
	request := ToolExecutionRequest{CapabilityName: wardrobeWearCapabilityName, OperationID: "committed-former-profile", AuthorizationActorID: f.ownerID, SubjectActorID: f.ownerID, FluctlightID: f.fluctlightID, WorkingProfileID: "former", EvidenceID: "wardrobe-preparation:" + preparation, CorrelationID: "wardrobe-preparation:" + preparation, Surface: CapabilitySurfaceNativeCognition, Arguments: jsonBytes(map[string]any{"mode": "full", "item_ids": []any{boots}})}
	first, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || first.Result.Status != "completed" {
		t.Fatal(first, err)
	}
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET core_persona='{}'::jsonb WHERE id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.finishWardrobePreparationWear(f.ctx, preparation, f.fluctlightID, f.ownerID, "default", saved)
	if err != nil || result["status"] != "completed" {
		t.Fatal(result, err)
	}
	receipt, ok := result["wear_receipt"].(ToolExecutionReceipt)
	if !ok || !receipt.Replayed {
		t.Fatal(result)
	}
	worn, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 1 || worn[0]["id"] != boots {
		t.Fatal(worn, err)
	}
	f.requireNoModelRuns(t)
}

func TestWardrobePreparationCannotCommitAfterCharacterPause(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	fl, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET status='paused' WHERE id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.app.commitPreparedWardrobe(f.ctx, "paused-test", f.fluctlightID, f.ownerID, "default", 0, fl.CurrentRevision, map[string]any{}); !errors.Is(err, ErrConflict) {
		t.Fatal("late generation bypassed pause", err)
	}
	var boots string
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND slot='shoes'`, f.fluctlightID).Scan(&boots); err != nil {
		t.Fatal(err)
	}
	saved := map[string]any{"wear_fence": map[string]any{"revision": 0, "profile_id": "default", "life_revision": life["context_revision"]}}
	receipt, err := f.app.ExecuteTool(withWardrobeSelectionFence(f.ctx, saved), f.request(wardrobeWearCapabilityName, "paused-selection", map[string]any{"mode": "full", "item_ids": []any{boots}}))
	if err != nil || receipt.Result.Status != "handled" || mapValue(receipt.Result.Output)["effect_applied"] != false {
		t.Fatal(receipt, err)
	}
	worn, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 2 {
		t.Fatal(worn, err)
	}
	f.requireNoModelRuns(t)
}
