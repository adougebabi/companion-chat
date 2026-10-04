package core

import (
	"testing"
	"time"
)

func TestOrdinaryObjectAcquisitionAndIndependentUseShareOneAuthority(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	at := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)
	f.app.Clock = func() time.Time { return at }
	started, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityStartCapabilityName, "brush-shopping", map[string]any{"kind": "virtual_shopping", "duration_minutes": 15, "item_kind": "object", "category": "art_supply", "description": "细头画笔", "reason": "缺少合适的画笔"}))
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(mapValue(started.Result.Output)["activity_id"])
	setupVirtualActivityTestProvider(t, f, map[string]any{"status": "completed", "reason": "合法获取了画笔", "acquired_item": map[string]any{"item_kind": "object", "category": "art_supply", "description": "细头画笔"}})
	at = at.Add(15 * time.Minute)
	acquired, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityAdvanceCapabilityName, "brush-result", map[string]any{"activity_id": id}))
	if err != nil {
		t.Fatal(err)
	}
	itemID := stringValue(mapValue(acquired.Result.Output)["item_id"])
	if itemID == "" {
		t.Fatal(acquired)
	}
	appearance, _, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, at)
	if err != nil || len(arrayValue(appearance["used_items"])) != 0 {
		t.Fatalf("purchase auto-used object: %#v %v", appearance, err)
	}
	worn, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "brush-wear", map[string]any{"mode": "partial", "item_ids": []any{itemID}}))
	if err == nil || worn.Result.ErrorCode != "item_not_wearable" {
		t.Fatalf("ordinary object forced into clothing slot: %#v %v", worn, err)
	}
	request := f.request(itemUseCapabilityName, "brush-use", map[string]any{"operation": "start", "item_id": itemID, "activity": "画画"})
	used, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || used.Result.Status != "completed" {
		t.Fatalf("use %#v %v", used, err)
	}
	restarted := &App{DB: f.repository, Provider: f.app.Provider, Clock: func() time.Time { return at }}
	replay, err := restarted.ExecuteTool(f.ctx, request)
	if err != nil || !replay.Replayed {
		t.Fatalf("use replay %#v %v", replay, err)
	}
	appearance, _, _, err = restarted.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, at)
	if err != nil || len(arrayValue(appearance["used_items"])) != 1 {
		t.Fatalf("actual use missing: %#v %v", appearance, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_item_use_events WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate event %d %v", count, err)
	}
}
func TestShoppingBundleRejectsPartialSuccessThenAtomicallyAcquiresAllMembers(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	at := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)
	f.app.Clock = func() time.Time { return at }
	members := []any{map[string]any{"category": "shirt", "slot": "top", "description": "黑衬衫"}, map[string]any{"category": "boots", "slot": "shoes", "description": "黑短靴"}}
	started, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityStartCapabilityName, "bundle-start", map[string]any{"kind": "virtual_shopping", "duration_minutes": 15, "items": members, "reason": "购买一套衣物"}))
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(mapValue(started.Result.Output)["activity_id"])
	at = at.Add(15 * time.Minute)
	router := setupVirtualActivityTestProvider(t, f, map[string]any{"status": "completed", "reason": "只买了第一件", "acquired_items": members[:1]})
	request := f.request(lifeActivityAdvanceCapabilityName, "bundle-result", map[string]any{"activity_id": id})
	_, err = f.app.ExecuteTool(f.ctx, request)
	if err == nil {
		t.Fatal("partial bundle falsely completed")
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, f.fluctlightID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial ghost inventory %d %v", count, err)
	}
	router.on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "全部物品获取成功", "acquired_items": members}}
	})
	done, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(arrayValue(mapValue(done.Result.Output)["item_ids"])) != 2 {
		t.Fatal(done.Result.Output)
	}
	repeated, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || !repeated.Replayed {
		t.Fatalf("retry %#v %v", repeated, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, f.fluctlightID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("bundle duplicates %d %v", count, err)
	}
}
