package browser

import "testing"

func TestMediaMetadataClosedBodyAndDualCAS(t *testing.T) {
	valid := map[string]any{"expectedAssetRevision": float64(1), "expectedMetadataRevision": float64(0), "title": "Sage", "hidden": false, "reason": "Owner library display", "idempotencyKey": "original"}
	if !validMediaMetadata(valid) {
		t.Fatal("valid metadata rejected")
	}
	for _, key := range []string{"expectedAssetRevision", "expectedMetadataRevision", "hidden", "reason", "idempotencyKey"} {
		bad := map[string]any{}
		for k, v := range valid {
			bad[k] = v
		}
		delete(bad, key)
		if validMediaMetadata(bad) {
			t.Fatal("missing explicit field accepted", key)
		}
	}
	valid["ownerActorId"] = "foreign-owner"
	if validMediaMetadata(valid) {
		t.Fatal("forged owner accepted")
	}
}
func TestDisplayAvatarOnlySelectsExplicitImageOrCanonical(t *testing.T) {
	valid := map[string]any{"mode": "asset", "assetId": "real-image", "expectedAssetRevision": float64(1), "expectedRevision": float64(0), "reason": "Display only", "idempotencyKey": "original"}
	if !validDisplayAvatar(valid) {
		t.Fatal("actual asset selection rejected")
	}
	valid["url"] = "https://untrusted.example/image"
	if validDisplayAvatar(valid) {
		t.Fatal("client URL admitted")
	}
	delete(valid, "url")
	valid["mode"] = "canonical"
	if validDisplayAvatar(valid) {
		t.Fatal("canonical mode carried another asset")
	}
	delete(valid, "assetId")
	delete(valid, "expectedAssetRevision")
	if !validDisplayAvatar(valid) {
		t.Fatal("canonical mode rejected")
	}
}
func TestExplicitMediaReferenceBodyRequiresScopePurposeConfirmationAndGrantPair(t *testing.T) {
	body := map[string]any{"assetId": "actual", "expectedAssetRevision": float64(1), "targetScope": "studio_resource", "targetId": "target", "purpose": "person", "confirm": true, "reason": "Owner permits exact use", "idempotencyKey": "same"}
	if !validMediaReferenceGrant("/api/media-reference-grants", body) {
		t.Fatal("explicit reference grant rejected")
	}
	body["confirm"] = false
	if validMediaReferenceGrant("/api/media-reference-grants", body) {
		t.Fatal("grant without confirmation accepted")
	}
	ref := map[string]any{"assetId": "actual", "purpose": "person", "grantId": "grant", "grantRevision": float64(1)}
	if !validStudioReference(map[string]any{"kind": "image_preview", "referenceAsset": ref}) {
		t.Fatal("exact person grant pair rejected")
	}
	delete(ref, "grantRevision")
	if validStudioReference(map[string]any{"kind": "image_preview", "referenceAsset": ref}) {
		t.Fatal("half grant pair accepted")
	}
}
func TestOwnerImageRequestClosedBodyAndSourceRevisions(t *testing.T) {
	body := map[string]any{"prompt": "fictional picture", "reason": "image only", "idempotencyKey": "same"}
	if !validOwnerMediaRequest("/api/fluctlights/role/media-generations", body) {
		t.Fatal("explicit general picture rejected")
	}
	body["placeId"] = "place"
	if validOwnerMediaRequest("/api/fluctlights/role/media-generations", body) {
		t.Fatal("place without revision accepted")
	}
	body["expectedPlaceRevision"] = float64(2)
	body["itemId"] = "item"
	if validOwnerMediaRequest("/api/fluctlights/role/media-generations", body) {
		t.Fatal("item without revision accepted")
	}
	body["expectedItemRevision"] = float64(1)
	if !validOwnerMediaRequest("/api/fluctlights/role/media-generations", body) {
		t.Fatal("paired sources rejected")
	}
	if validOwnerMediaRequest("/api/fluctlights/role/wardrobe/items/item/photos", body) {
		t.Fatal("route photo accepted another item target")
	}
	delete(body, "itemId")
	if !validOwnerMediaRequest("/api/fluctlights/role/wardrobe/items/item/photos", body) {
		t.Fatal("bound photo source rejected")
	}
	body["expectedRevision"] = float64(1)
	if validOwnerMediaRequest("/api/fluctlights/role/wardrobe/items/item/photos", body) {
		t.Fatal("photo admitted a generic Life mutation revision")
	}
}
