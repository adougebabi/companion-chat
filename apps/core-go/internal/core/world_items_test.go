package core

import (
	"testing"
)

func TestItemDefinitionRequiresExplicitNonownershipFields(t *testing.T) {
	good := map[string]any{"schema": "item-definition.v1", "item_kind": "object", "category": "book", "slot": "", "description": "A fictional sourced book"}
	if err := validateItemDefinition(good); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ownership", "currently_worn", "source_resource_id", "permanent_harm"} {
		bad := cloneMap(good)
		bad[key] = true
		if validateItemDefinition(bad) == nil {
			t.Fatal("definition grants authority", key)
		}
	}
	for _, key := range []string{"item_kind", "slot", "category", "description"} {
		bad := cloneMap(good)
		delete(bad, key)
		if validateItemDefinition(bad) == nil {
			t.Fatal("implicit field admitted", key)
		}
	}
	request := cloneMap(good)
	request["world_item_id"] = "catalogue-original"
	result := map[string]any{"acquired_item": map[string]any{"item_kind": "object", "category": "book", "slot": "", "description": good["description"], "world_item_id": "forged"}}
	if validateShoppingAcquisition(request, result) == nil {
		t.Fatal("model changed catalogue identity")
	}
	delete(mapValue(result["acquired_item"]), "world_item_id")
	if err := validateShoppingAcquisition(request, result); err != nil {
		t.Fatal("model must not be authority for omitted provenance", err)
	}
}

func TestWorldItemRegistrationAndRealShoppingPreserveTwoSourceDimensions(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	base := "/internal/fluctlights/" + f.fluctlightID
	world, err := f.app.DispatchWorldSurface(f.ctx, "POST", base+"/world/catalog", nil, f.ownerID, map[string]any{"name": "Item world", "reason": "Test directory", "idempotency_key": "item-world"})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := f.app.worldCatalog(f.ctx, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.DispatchWorldSurface(f.ctx, "PUT", base+"/world/active", nil, f.ownerID, map[string]any{"world_id": world["id"], "expected_revision": intValue(mapValue(catalog["active_binding"])["revision"]), "reason": "Select item world", "idempotency_key": "item-world-select"}); err != nil {
		t.Fatal(err)
	}
	definition := map[string]any{"schema": "item-definition.v1", "item_kind": "object", "category": "book", "slot": "", "description": "Catalogue book"}
	r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "item", "name": "Catalogue book", "body": definition, "reason": "Test definition", "idempotency_key": "item-definition"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(r["id"])
	pub, err := f.app.publishWorldResource(f.ctx, f.ownerID, id, map[string]any{"expected_revision": r["revision"], "reason": "Publish item", "idempotency_key": "item-publish"})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"]}, true)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": preview["instance_revision"], "expected_runtime_revision": 0, "world_id": preview["world_id"], "expected_world_revision": preview["world_binding_revision"], "choices": []any{}, "reason": "Register only", "idempotency_key": "item-apply"}
	registered, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, body, false)
	if err != nil || registered["target_kind"] != "world_item" {
		t.Fatal(registered, err)
	}
	itemID := stringValue(registered["id"])
	var owned int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_resource_id=$2`, f.fluctlightID, id).Scan(&owned); err != nil || owned != 0 {
		t.Fatal("registration grants inventory", owned, err)
	}
	started, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityStartCapabilityName, "catalogue-shopping", map[string]any{"kind": "virtual_shopping", "duration_minutes": 15, "world_item_id": itemID, "item_kind": "object", "category": "book", "slot": "", "description": "Catalogue book", "reason": "Actually attempt virtual acquisition"}))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatal(started, err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	// Only the clock boundary is a fixture. Completion is produced by the real
	// registered result Tool and a clearly labelled controlled Provider response.
	forceVirtualActivityDue(t, f, activityID)
	setupVirtualActivityTestProvider(t, f, map[string]any{"status": "completed", "reason": "Selected book acquired virtually", "acquired_item": map[string]any{"item_kind": "object", "category": "book", "slot": "", "description": "Catalogue book"}})
	resolved, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityAdvanceCapabilityName, "catalogue-shopping-result", map[string]any{"activity_id": activityID}))
	if err != nil || resolved.Result.Status != "completed" {
		t.Fatal(resolved, err)
	}
	ownedID := stringValue(mapValue(resolved.Result.Output)["item_id"])
	item, err := f.app.OwnerInventoryItem(f.ctx, f.ownerID, f.fluctlightID, ownedID)
	if err != nil || item["source_kind"] != "purchase_result" || item["source_verified"] != true || item["source_resource_id"] != id || item["source_version"] != pub["published_version"] || item["source_world_item_id"] != itemID || item["worn"] != nil || item["use"] != nil {
		t.Fatal("lost acquisition or definition provenance", item, err)
	}
	replay, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityAdvanceCapabilityName, "catalogue-shopping-result", map[string]any{"activity_id": activityID}))
	if err != nil || !replay.Replayed {
		t.Fatal("resolution replay changed effect", replay, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_resource_id=$2`, f.fluctlightID, id).Scan(&owned); err != nil || owned != 1 {
		t.Fatal("replay granted another concrete item", owned, err)
	}
}
