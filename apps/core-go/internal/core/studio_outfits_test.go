package core

import (
	"errors"
	"strings"
	"testing"
)

func TestOutfitDefinitionRejectsInventoryAuthorityAndInventedReferences(t *testing.T) {
	body := map[string]any{"schema": "outfit-definition.v1", "description": "A reusable outfit", "members": []any{map[string]any{"resource_id": "published-shirt", "resource_version": "2"}}}
	if err := validateOutfitDefinition(body); err != nil {
		t.Fatal(err)
	}
	bad := cloneMap(body)
	bad["members"] = []any{map[string]any{"item_id": "pretend-owned"}}
	if validateOutfitDefinition(bad) == nil {
		t.Fatal("inventory IDs admitted as definitions")
	}
	snapshot := map[string]any{"resource_kind": "outfit", "allowed_references": []any{map[string]any{"kind": "item_definition", "id": "published-shirt", "version": "2", "item_kind": "wearable", "slot": "top"}}}
	out := map[string]any{"body": body}
	if err := validateStudioReferenceProposal(snapshot, out); err != nil {
		t.Fatal(err)
	}
	changed := cloneMap(body)
	changed["members"] = []any{map[string]any{"resource_id": "published-shirt", "resource_version": "3"}}
	if validateStudioReferenceProposal(snapshot, map[string]any{"body": changed}) == nil {
		t.Fatal("assistant changed the frozen exact version")
	}
	snapshot["allowed_references"] = append(arrayValue(snapshot["allowed_references"]), map[string]any{"kind": "item_definition", "id": "another-shirt", "version": "1", "item_kind": "wearable", "slot": "top"})
	duplicate := cloneMap(body)
	duplicate["members"] = append(arrayValue(duplicate["members"]), map[string]any{"resource_id": "another-shirt", "resource_version": "1"})
	if validateStudioReferenceProposal(snapshot, map[string]any{"body": duplicate}) == nil {
		t.Fatal("assistant proposal accepted duplicate verified slots")
	}
}

func TestOutfitNewPublicationAndJobRejectArchivedOrNonwearableMembers(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	create := func(kind, name, key string, body map[string]any) map[string]any {
		t.Helper()
		r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": kind, "name": name, "body": body, "reason": "Explicit reference test", "idempotency_key": key})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	garment := create("item", "Archived garment", "archived-garment", map[string]any{"schema": "item-definition.v1", "item_kind": "wearable", "category": "top", "slot": "top", "description": "Published garment"})
	pub, err := f.app.publishWorldResource(f.ctx, f.ownerID, stringValue(garment["id"]), map[string]any{"expected_revision": garment["revision"], "reason": "Publish garment", "idempotency_key": "archive-garment-publish"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.studioCatalogCommand(f.ctx, f.ownerID, stringValue(garment["id"]), "archive", map[string]any{"expected_revision": pub["revision"], "reason": "Archive unused garment", "idempotency_key": "archive-garment"}); err != nil {
		t.Fatal(err)
	}
	definition := map[string]any{"schema": "outfit-definition.v1", "description": "New reference cannot revive archived catalogue availability", "members": []any{map[string]any{"resource_id": garment["id"], "resource_version": pub["published_version"]}}}
	r := create("outfit", "New outfit", "archived-member-outfit", definition)
	var rejected *SurfaceError
	if _, err := f.app.publishWorldResource(f.ctx, f.ownerID, stringValue(r["id"]), map[string]any{"expected_revision": r["revision"], "reason": "Reject archived member", "idempotency_key": "archived-member-publish"}); !errors.As(err, &rejected) || rejected.Code != "studio_resource_archived" {
		t.Fatal("new publication must reject the archived member explicitly", err)
	}
	if _, err := f.app.DispatchStudioJobSurface(f.ctx, "POST", "/internal/studio/jobs", nil, f.ownerID, map[string]any{"resource_id": r["id"], "resource_revision": r["revision"], "kind": "image_preview", "name": r["name"], "body": definition, "instruction": "Preview definitions", "reason": "Reject archived reference", "idempotency_key": "archived-member-image"}); !errors.As(err, &rejected) || rejected.Code != "studio_resource_archived" {
		t.Fatal("new image task must reject archived member explicitly", err)
	}
	// Immutable history stays readable for an already saved origin.
	if _, err := outfitPublishedMembers(f.ctx, f.repository.Pool(), f.ownerID, definition); err != nil {
		t.Fatal("archive broke historical exact source resolution", err)
	}
	object := create("item", "Object cannot be clothing", "object-definition", map[string]any{"schema": "item-definition.v1", "item_kind": "object", "category": "book", "slot": "", "description": "A book"})
	objectPub, err := f.app.publishWorldResource(f.ctx, f.ownerID, stringValue(object["id"]), map[string]any{"expected_revision": object["revision"], "reason": "Publish ordinary object", "idempotency_key": "object-publish"})
	if err != nil {
		t.Fatal(err)
	}
	objectDefinition := map[string]any{"schema": "outfit-definition.v1", "description": "Reject nonwearable member at image admission", "members": []any{map[string]any{"resource_id": object["id"], "resource_version": objectPub["published_version"]}}}
	objectOutfit := create("outfit", "Invalid object outfit", "object-outfit", objectDefinition)
	if _, err := f.app.DispatchStudioJobSurface(f.ctx, "POST", "/internal/studio/jobs", nil, f.ownerID, map[string]any{"resource_id": objectOutfit["id"], "resource_revision": objectOutfit["revision"], "kind": "image_preview", "name": objectOutfit["name"], "body": objectDefinition, "instruction": "Preview", "reason": "Reject ordinary object as outfit", "idempotency_key": "object-outfit-image"}); !errors.As(err, &rejected) || !strings.Contains(rejected.Message, "wearable") {
		t.Fatal("image admission must reject nonwearable source explicitly", err)
	}
}

func TestStudioOutfitMediaNormalizationRetainsVerifiedGarmentSemantics(t *testing.T) {
	members := []any{map[string]any{"kind": "item_definition", "id": "garment-source", "version": "2", "slot": "top", "description": "sage linen shirt with wooden buttons"}}
	concept := map[string]any{"source": "studio_definition", "studio_source": map[string]any{"resource_id": "outfit-source", "resource_kind": "outfit", "source_digest": "digest", "verified_members": members}, "intent": "flat laid garments without people", "media_plan_version": mediaPlanVersion, "media_capture_plan": mediaTestPlan("object", "none", "no", "unknown"), "explicit_constraints": map[string]any{"self_in_frame": "no"}, "context_binding": map[string]any{}}
	packet, err := mediaNormalizationInput(concept)
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(mapValue(packet["studio_definition_source"])["verified_members"]) != jsonString(members) {
		t.Fatal("exact garment semantics dropped before model", packet)
	}
}

func TestPublishedOutfitMapsRealInventoryAndReusesOriginalSaveConsumer(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var held, category, slot, description string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id,category,slot,description FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND item_kind='wearable' AND archived_at IS NULL ORDER BY id LIMIT 1`, f.fluctlightID).Scan(&held, &category, &slot, &description); err != nil {
		t.Fatal(err)
	}
	publish := func(kind, name, key string, definition map[string]any) map[string]any {
		t.Helper()
		r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": kind, "name": name, "body": definition, "reason": "Test definition", "idempotency_key": key})
		if err != nil {
			t.Fatal(err)
		}
		p, err := f.app.publishWorldResource(f.ctx, f.ownerID, stringValue(r["id"]), map[string]any{"expected_revision": r["revision"], "reason": "Publish exact definition", "idempotency_key": key + "-publish"})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	item := publish("item", "Source garment", "outfit-source-item", map[string]any{"schema": "item-definition.v1", "item_kind": "wearable", "category": category, "slot": slot, "description": description})
	definition := map[string]any{"schema": "outfit-definition.v1", "description": "Explicit reusable combination", "members": []any{map[string]any{"resource_id": item["id"], "resource_version": item["published_version"]}}}
	if _, err := outfitPublishedMembers(f.ctx, f.repository.Pool(), "other-owner", definition); err == nil {
		t.Fatal("another Owner can resolve member definitions")
	}
	source := publish("outfit", "Published combination", "outfit-source", definition)
	id := stringValue(source["id"])
	preview, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": source["published_version"]}, true)
	if err != nil || preview["target_kind"] != "wardrobe_outfit" {
		t.Fatal(preview, err)
	}
	before, bRev, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	command := map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": source["published_version"], "expected_instance_revision": preview["instance_revision"], "expected_runtime_revision": preview["runtime_revision"], "profile_id": preview["profile_id"], "expected_wardrobe_revision": preview["wardrobe_revision"], "member_items": []any{map[string]any{"resource_id": item["id"], "resource_version": item["published_version"], "item_id": held}}, "choices": []any{}, "reason": "Explicit actual inventory substitution", "idempotency_key": "outfit-apply"}
	applied, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, command, false)
	if err != nil || applied["applied"] != true {
		t.Fatal(applied, err)
	}
	outfitID := stringValue(applied["id"])
	saved, err := f.app.OwnerInventoryOutfit(f.ctx, f.ownerID, f.fluctlightID, outfitID)
	if err != nil || saved["source_resource_id"] != id || saved["source_version"] != source["published_version"] || len(arrayValue(mapValue(saved["source_overrides"])["members"])) != 1 {
		t.Fatal("origin or explicit legacy-item substitution lost", saved, err)
	}
	after, afterBody, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
	if err != nil || bRev != afterBody || jsonString(before["worn_items"]) != jsonString(after["worn_items"]) {
		t.Fatal("definition application dressed or changed body", err)
	}
	// The pre-existing save Tool consumes and refines this exact sourced outfit;
	// it must retain origin and recompute the local override, not copy business state.
	refined, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeOutfitSaveCapabilityName, "refine-sourced-outfit", map[string]any{"outfit_id": outfitID, "name": "Local refinement", "item_ids": []string{held}}))
	if err != nil || refined.Result.Status != "completed" {
		t.Fatal(refined, err)
	}
	saved, err = f.app.OwnerInventoryOutfit(f.ctx, f.ownerID, f.fluctlightID, outfitID)
	if err != nil || saved["source_resource_id"] != id || mapValue(saved["source_overrides"])["name"] != "Local refinement" {
		t.Fatal(saved, err)
	}
	replay, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, command, false)
	if err != nil || replay["id"] != outfitID {
		t.Fatal("application replay created another outfit", replay, err)
	}
	saved, err = f.app.OwnerInventoryOutfit(f.ctx, f.ownerID, f.fluctlightID, outfitID)
	if err != nil || saved["name"] != "Local refinement" {
		t.Fatal("old apply replay overwrote later refinement", saved, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND source_resource_id=$2`, f.fluctlightID, id).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
