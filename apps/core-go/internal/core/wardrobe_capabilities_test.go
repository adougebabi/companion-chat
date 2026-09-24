package core

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func seedWardrobeToolFixture(t *testing.T) independentToolE2EFixture {
	t.Helper()
	fixture := newIndependentToolE2EFixture(t, "wardrobe")
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'default',0) ON CONFLICT(fluctlight_id) DO NOTHING`, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	foundation := map[string]any{
		"identity": map[string]any{"name": "摇光"},
		"life_profile": map[string]any{
			"appearance": map[string]any{
				"physical_features": map[string]any{"hair_length": "long", "hair_color": "black"},
				"wardrobe_items": []any{
					map[string]any{"category": "shirt", "slot": "top", "description": "白衬衫", "ownership": "unknown", "available": true, "currently_worn": true},
					map[string]any{"category": "accessory", "slot": "accessory", "description": "银项链", "ownership": "owned", "available": true, "currently_worn": true},
					map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴", "ownership": "owned", "available": true, "currently_worn": false},
					map[string]any{"category": "coat", "slot": "outerwear", "description": "旧外套", "ownership": "owned", "available": false, "currently_worn": false},
				},
			},
			"life_habits": []any{"通常穿轻便衣服"},
		},
	}
	if err := withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
		return initializeEffectiveLifeTx(fixture.ctx, tx, fixture.fluctlightID, foundation)
	}); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestWardrobeIndependentToolKeepsItemsAndWearingDistinct(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	list, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeInspectCapabilityName, "list-boots", map[string]any{"operation": "list", "category": "boots", "limit": 1}))
	if err != nil || list.Result.Status != "completed" {
		t.Fatalf("list wardrobe: receipt=%#v err=%v", list, err)
	}
	items := arrayValue(mapValue(list.Result.Output)["items"])
	if len(items) != 1 || stringValue(mapValue(items[0])["description"]) != "黑色短靴" {
		t.Fatalf("boots were not persisted: %#v", list.Result.Output)
	}
	bootsID := stringValue(mapValue(items[0])["id"])
	if mapValue(list.Result.Output)["can_conclude_absent"] != false {
		t.Fatalf("partial initial inventory must not certify absence: %#v", list.Result.Output)
	}
	initial, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeInspectCapabilityName, "initial-wearing", map[string]any{"operation": "wearing"}))
	if err != nil || len(arrayValue(mapValue(initial.Result.Output)["items"])) != 2 {
		t.Fatalf("initial wearing missing: receipt=%#v err=%v", initial, err)
	}
	wearRequest := fixture.request(wardrobeWearCapabilityName, "partial-boots", map[string]any{"mode": "partial", "item_ids": []string{bootsID}})
	worn, err := fixture.app.ExecuteTool(fixture.ctx, wearRequest)
	if err != nil || worn.Result.Status != "completed" {
		t.Fatalf("partial wear: receipt=%#v err=%v", worn, err)
	}
	if got := len(arrayValue(mapValue(worn.Result.Output)["items"])); got != 3 {
		t.Fatalf("partial wear removed unrelated clothes: %#v", worn.Result.Output)
	}
	replayed, err := fixture.app.ExecuteTool(fixture.ctx, wearRequest)
	if err != nil || !replayed.Replayed || intValue(mapValue(replayed.Result.Output)["revision"]) != 1 {
		t.Fatalf("wearing replay duplicated effect: receipt=%#v err=%v", replayed, err)
	}
	changed := wearRequest
	changed.Arguments = jsonBytes(map[string]any{"mode": "full", "item_ids": []string{bootsID}})
	if _, err := fixture.app.ExecuteTool(fixture.ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("same operation with changed payload should conflict: %v", err)
	}
	full, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeWearCapabilityName, "full-boots", map[string]any{"mode": "full", "item_ids": []string{bootsID}}))
	if err != nil || len(arrayValue(mapValue(full.Result.Output)["items"])) != 1 {
		t.Fatalf("full wear did not replace old clothes: receipt=%#v err=%v", full, err)
	}
	refreshed, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeInspectCapabilityName, "wearing-after", map[string]any{"operation": "wearing"}))
	if err != nil || stringValue(mapValue(arrayValue(mapValue(refreshed.Result.Output)["items"])[0])["id"]) != bootsID {
		t.Fatalf("independent later query did not reuse the same item: receipt=%#v err=%v", refreshed, err)
	}
	if mapValue(refreshed.Result.Output)["wearing_state"] != "known" {
		t.Fatalf("wearing state was not marked known: %#v", refreshed.Result.Output)
	}
	var habitRevision int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT revision FROM public.fluctlight_profile_habits WHERE fluctlight_id=$1 AND profile_id='default'`, fixture.fluctlightID).Scan(&habitRevision); err != nil || habitRevision != 0 {
		t.Fatalf("one outfit change must not update habits: revision=%d err=%v", habitRevision, err)
	}
}

func TestWardrobeIndependentToolRejectsUnavailableAndForeignOwner(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	list, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeInspectCapabilityName, "list-coats", map[string]any{"operation": "list", "category": "coat"}))
	if err != nil {
		t.Fatal(err)
	}
	coatID := stringValue(mapValue(arrayValue(mapValue(list.Result.Output)["items"])[0])["id"])
	unavailable, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeWearCapabilityName, "wear-unavailable", map[string]any{"mode": "partial", "item_ids": []string{coatID}}))
	if err == nil || unavailable.Result.ErrorCode != "wardrobe_item_unavailable" {
		t.Fatalf("unavailable item was worn: receipt=%#v err=%v", unavailable, err)
	}
	foreign := fixture.request(wardrobeInspectCapabilityName, "foreign-query", map[string]any{"operation": "list"})
	foreign.AuthorizationActorID = fixture.foreignOwnerID
	if _, err := fixture.app.ExecuteTool(fixture.ctx, foreign); err == nil {
		t.Fatal("foreign Owner inspected wardrobe")
	}
	missing, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeInspectCapabilityName, "missing-item", map[string]any{"operation": "detail", "item_id": "wardrobe_missing"}))
	if err == nil || missing.Result.ErrorCode != "wardrobe_item_not_found" {
		t.Fatalf("missing item result was not explicit: receipt=%#v err=%v", missing, err)
	}
}

func TestWardrobePreparedWearRejectsConcurrentRevision(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	capability := wardrobeWearCapability{service: newWardrobeService(fixture.app)}
	invocation := CapabilityInvocation{CallID: "stale-wear", CapabilityName: wardrobeWearCapabilityName, SchemaVersion: CapabilityInvocationSchemaVersion,
		Arguments: jsonBytes(map[string]any{"mode": "full", "item_ids": []string{}}),
		Metadata:  InvocationMetadata{FluctlightID: fixture.fluctlightID, AuthorizationActorID: fixture.ownerID, OperationID: "stale-wear"}}
	prepared, err := capability.Prepare(context.Background(), invocation, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeWearCapabilityName, "concurrent-wear", map[string]any{"mode": "full", "item_ids": []string{}})); err != nil {
		t.Fatal(err)
	}
	err = withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
		_, err := capability.ExecuteTx(fixture.ctx, tx, prepared, CapabilityContext{})
		return err
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale prepared wear silently replaced a newer decision: %v", err)
	}
}

func TestWardrobeSavedOutfitReferencesExistingItemWithoutChangingWearing(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	list, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeInspectCapabilityName, "outfit-list-boots", map[string]any{"operation": "list", "category": "boots"}))
	if err != nil {
		t.Fatal(err)
	}
	bootsID := stringValue(mapValue(arrayValue(mapValue(list.Result.Output)["items"])[0])["id"])
	saved, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeOutfitSaveCapabilityName, "save-boots-outfit", map[string]any{"name": "周末短靴", "item_ids": []string{bootsID}}))
	if err != nil || saved.Result.Status != "completed" {
		t.Fatalf("save outfit: receipt=%#v err=%v", saved, err)
	}
	outfitID := stringValue(mapValue(saved.Result.Output)["outfit_id"])
	detail, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeInspectCapabilityName, "read-boots-outfit", map[string]any{"operation": "outfit_detail", "outfit_id": outfitID}))
	if err != nil || stringValue(mapValue(arrayValue(mapValue(mapValue(detail.Result.Output)["outfit"])["items"])[0])["id"]) != bootsID {
		t.Fatalf("outfit did not reference existing item: receipt=%#v err=%v", detail, err)
	}
	var totalItems, wornCount int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&totalItems); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&wornCount); err != nil {
		t.Fatal(err)
	}
	if totalItems != 4 || wornCount != 2 {
		t.Fatalf("saving outfit created clothes or changed current wearing: items=%d worn=%d", totalItems, wornCount)
	}
}
