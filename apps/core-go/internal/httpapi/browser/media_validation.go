package browser

import "strings"

func validMediaMetadata(body map[string]any) bool {
	if !onlyKeys(body, "expectedAssetRevision", "expectedMetadataRevision", "title", "hidden", "reason", "idempotencyKey") || !validateInteger(body["expectedAssetRevision"], 1) || !validateInteger(body["expectedMetadataRevision"], 0) || !validateString(body["title"], 0, 200) || !validateString(body["reason"], 1, 1000) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	_, ok := body["hidden"].(bool)
	return ok
}

func validDisplayAvatar(body map[string]any) bool {
	if !onlyKeys(body, "mode", "assetId", "expectedAssetRevision", "expectedRevision", "reason", "idempotencyKey") || !validateInteger(body["expectedRevision"], 0) || !validateString(body["reason"], 1, 1000) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	if body["mode"] == "asset" {
		return validateString(body["assetId"], 1, 128) && validateInteger(body["expectedAssetRevision"], 1)
	}
	_, hasID := body["assetId"]
	_, hasRevision := body["expectedAssetRevision"]
	return body["mode"] == "canonical" && !hasID && !hasRevision
}

func validMediaReferenceGrant(path string, b map[string]any) bool {
	if strings.HasSuffix(path, "/revoke") {
		return onlyKeys(b, "expectedRevision", "reason", "idempotencyKey") && validateInteger(b["expectedRevision"], 1) && validateString(b["reason"], 1, 1000) && validateString(b["idempotencyKey"], 1, 256)
	}
	return onlyKeys(b, "assetId", "expectedAssetRevision", "targetScope", "targetId", "purpose", "confirm", "reason", "idempotencyKey") && validateString(b["assetId"], 1, 128) && validateInteger(b["expectedAssetRevision"], 1) && (b["targetScope"] == "studio_resource" || b["targetScope"] == "fluctlight") && validateString(b["targetId"], 1, 128) && (b["purpose"] == "scene" || b["purpose"] == "animal" || b["purpose"] == "object" || b["purpose"] == "person") && b["confirm"] == true && validateString(b["reason"], 1, 1000) && validateString(b["idempotencyKey"], 1, 256)
}
func validOwnerMediaRequest(path string, b map[string]any) bool {
	if !onlyKeys(b, "prompt", "referenceAsset", "placeId", "expectedPlaceRevision", "itemId", "expectedItemRevision", "reason", "idempotencyKey") || !validateString(b["prompt"], 1, 2000) || !validateString(b["reason"], 1, 500) || !validateString(b["idempotencyKey"], 1, 256) {
		return false
	}
	if raw, present := b["referenceAsset"]; present {
		reference, ok := raw.(map[string]any)
		if !ok || !validExplicitMediaReference(reference) {
			return false
		}
	}
	if place, present := b["placeId"]; present && (!validateString(place, 1, 128) || !validateInteger(b["expectedPlaceRevision"], 1)) {
		return false
	}
	if b["placeId"] == nil && b["expectedPlaceRevision"] != nil {
		return false
	}
	if strings.HasSuffix(path, "/photos") {
		_, present := b["itemId"]
		return !present && validateInteger(b["expectedItemRevision"], 1)
	}
	if item, present := b["itemId"]; present {
		return validateString(item, 1, 128) && validateInteger(b["expectedItemRevision"], 1)
	}
	_, present := b["expectedItemRevision"]
	return !present
}

func validExplicitMediaReference(ref map[string]any) bool {
	return onlyKeys(ref, "assetId", "purpose", "grantId", "grantRevision") && validateString(ref["assetId"], 1, 128) && validateString(ref["grantId"], 1, 128) && validateInteger(ref["grantRevision"], 1) && (ref["purpose"] == "scene" || ref["purpose"] == "animal" || ref["purpose"] == "object" || ref["purpose"] == "person")
}

func validMediaDelete(b map[string]any) bool {
	return onlyKeys(b, "expectedAssetRevision", "confirm", "reason", "idempotencyKey") && validateInteger(b["expectedAssetRevision"], 1) && b["confirm"] == true && validateString(b["reason"], 1, 1000) && validateString(b["idempotencyKey"], 1, 256)
}
func validMediaDeletionRetry(b map[string]any) bool {
	return onlyKeys(b, "expectedRevision", "reason", "idempotencyKey") && validateInteger(b["expectedRevision"], 1) && validateString(b["reason"], 1, 1000) && validateString(b["idempotencyKey"], 1, 256)
}
