package browser

import "net/http"

func validInventoryCommand(method string, body map[string]any) bool {
	if !onlyKeys(body, "expectedRevision", "expectedItemRevision", "reason", "idempotencyKey", "item") || !validateRevisionReason(body) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	if method == http.MethodPost {
		if _, present := body["expectedItemRevision"]; present {
			return false
		}
	}
	if method != http.MethodPost && !validateInteger(body["expectedItemRevision"], 1) {
		return false
	}
	if method == http.MethodDelete {
		_, has := body["item"]
		return !has
	}
	item := objectValue(body["item"])
	if !onlyKeys(item, "itemKind", "item_kind", "category", "slot", "description", "ownership", "availability") {
		return false
	}
	kind := stringValue(item["itemKind"])
	legacyKind := stringValue(item["item_kind"])
	if kind != "" && legacyKind != "" {
		return false
	}
	if kind == "" {
		kind = legacyKind
	}
	if kind != "object" && kind != "wearable" {
		return false
	}
	if !validateString(item["category"], 1, 64) || !validateString(item["description"], 1, 512) || !validateString(item["slot"], 0, 64) {
		return false
	}
	slot := stringValue(item["slot"])
	if (kind == "object" && slot != "") || (kind == "wearable" && slot == "") {
		return false
	}
	ownership, availability := stringValue(item["ownership"]), stringValue(item["availability"])
	return (ownership == "owned" || ownership == "borrowed" || ownership == "unknown") && (availability == "available" || availability == "unavailable" || availability == "lost")
}

func validInventoryAction(body map[string]any) bool {
	if !validateInteger(body["expectedRevision"], 0) || !validateString(body["reason"], 1, 500) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	keys := []string{"kind", "expectedRevision", "reason", "idempotencyKey"}
	switch stringValue(body["kind"]) {
	case "borrow":
		keys = append(keys, "lender", "items")
		if !validateString(body["lender"], 1, 256) || len(array(body["items"])) < 1 || len(array(body["items"])) > 8 {
			return false
		}
		for _, raw := range array(body["items"]) {
			item := objectValue(raw)
			if !onlyKeys(item, "itemKind", "category", "slot", "description") || !validateString(item["category"], 1, 64) || !validateString(item["description"], 1, 512) || !validateString(item["slot"], 0, 64) {
				return false
			}
			kind, slot := stringValue(item["itemKind"]), stringValue(item["slot"])
			if (kind != "wearable" && kind != "object") || (kind == "wearable" && slot == "") || (kind == "object" && slot != "") {
				return false
			}
		}
	case "return":
		keys = append(keys, "itemIds")
		if !isStringArray(body["itemIds"]) || len(array(body["itemIds"])) < 1 || len(array(body["itemIds"])) > 8 {
			return false
		}
	case "remove":
		keys = append(keys, "slots")
		if !isStringArray(body["slots"]) || len(array(body["slots"])) < 1 || len(array(body["slots"])) > 16 {
			return false
		}
	case "outfit":
		keys = append(keys, "outfitId", "expectedOutfitRevision", "profileId")
		if !validateString(body["outfitId"], 1, 128) || !validateInteger(body["expectedOutfitRevision"], 1) || !validateString(body["profileId"], 1, 128) {
			return false
		}
	default:
		return false
	}
	return onlyKeys(body, keys...)
}

func validInventoryOutfitSave(method string, body map[string]any) bool {
	if !onlyKeys(body, "name", "itemIds", "profileId", "expectedRevision", "expectedOutfitRevision", "reason", "idempotencyKey") || !validateRevisionReason(body) || !validateString(body["idempotencyKey"], 1, 256) || !validateString(body["name"], 1, 128) || !validateString(body["profileId"], 1, 128) || !validateInteger(body["expectedOutfitRevision"], 0) || !isStringArray(body["itemIds"]) || len(array(body["itemIds"])) < 1 || len(array(body["itemIds"])) > 16 {
		return false
	}
	if method == http.MethodPost {
		return body["expectedOutfitRevision"] == float64(0)
	}
	return validateInteger(body["expectedOutfitRevision"], 1)
}
