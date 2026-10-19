package browser

import "strings"

func validWorldItemConfig(b map[string]any) bool {
	_, available := b["available"].(bool)
	place, present := b["placeId"]
	return onlyKeys(b, "expectedRevision", "available", "placeId", "reason", "idempotencyKey") && validateInteger(b["expectedRevision"], 1) && available && present && (place == nil || validateString(place, 1, 128)) && validateString(b["reason"], 1, 1000) && validateString(b["idempotencyKey"], 1, 256)
}

func validStudioJobBody(path string, b map[string]any) bool {
	if path == "/api/studio/jobs" {
		_, ok := b["body"].(map[string]any)
		return onlyKeys(b, "resourceId", "resourceRevision", "kind", "name", "body", "instruction", "reason", "idempotencyKey", "referenceAsset", "purpose", "resourceVersion") && validStudioPreparation(b) && validStudioReference(b) && ok && validateString(b["resourceId"], 1, 128) && validateInteger(b["resourceRevision"], 1) && (b["kind"] == "assistant" || b["kind"] == "text_preview" || b["kind"] == "image_preview") && validateString(b["name"], 1, 200) && validateString(b["instruction"], 1, 4000) && validateString(b["reason"], 1, 1000) && validateString(b["idempotencyKey"], 1, 256)
	}
	if strings.HasPrefix(path, "/api/studio/jobs/") || strings.HasPrefix(path, "/api/studio/materials/") {
		return onlyKeys(b, "expectedRevision", "reason", "idempotencyKey") && validateInteger(b["expectedRevision"], 1) && validateString(b["reason"], 1, 1000) && validateString(b["idempotencyKey"], 1, 256)
	}
	return true
}

func validStudioPreparation(b map[string]any) bool {
	_, purpose := b["purpose"]
	_, version := b["resourceVersion"]
	if !purpose && !version {
		return true
	}
	return b["purpose"] == "foundation_prepare" && validateString(b["resourceVersion"], 1, 64) && b["kind"] == "assistant" && b["referenceAsset"] == nil
}

// Reference authorization and source compatibility remain Core-owned.
func validStudioReference(b map[string]any) bool {
	value, present := b["referenceAsset"]
	if !present {
		return true
	}
	ref, ok := value.(map[string]any)
	if ok && (ref["grantId"] != nil || ref["grantRevision"] != nil) {
		return b["kind"] == "image_preview" && validExplicitMediaReference(ref)
	}
	return b["kind"] == "image_preview" && ok && onlyKeys(ref, "assetId", "purpose") && validateString(ref["assetId"], 1, 128) && (ref["purpose"] == "scene" || ref["purpose"] == "animal" || ref["purpose"] == "object")
}

func validStudioCatalogCommand(path string, b map[string]any) bool {
	if !validateInteger(b["expectedRevision"], 1) || !validateString(b["reason"], 1, 1000) || !validateString(b["idempotencyKey"], 1, 256) {
		return false
	}
	if strings.HasSuffix(path, "/copy") {
		return onlyKeys(b, "expectedRevision", "reason", "idempotencyKey", "name", "source", "sourceVersion") && validateString(b["name"], 1, 200) && (b["source"] == "draft" || b["source"] == "published") && validateString(b["sourceVersion"], 1, 64)
	}
	return onlyKeys(b, "expectedRevision", "reason", "idempotencyKey")
}
