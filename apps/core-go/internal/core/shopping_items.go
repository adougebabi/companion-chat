package core

import (
	"errors"
	"strings"
)

func shoppingItemSchema() map[string]any {
	return objectSchema(map[string]any{"item_kind": enumStringSchema("wearable", "object"), "category": map[string]any{"type": "string", "minLength": 1, "maxLength": 64}, "slot": map[string]any{"type": "string", "maxLength": 64}, "description": map[string]any{"type": "string", "minLength": 1, "maxLength": 512}}, []string{"category", "description"}, false)
}
func normalizeShoppingItem(item map[string]any) (map[string]any, error) {
	result := map[string]any{"item_kind": firstString(item["item_kind"], "wearable")}
	for _, key := range []string{"category", "slot", "description"} {
		result[key] = strings.TrimSpace(stringValue(item[key]))
	}
	kind, slot := stringValue(result["item_kind"]), stringValue(result["slot"])
	if (kind != "wearable" && kind != "object") || (kind == "wearable" && slot == "") || (kind == "object" && slot != "") || result["category"] == "" || result["description"] == "" || len([]rune(slot)) > 64 || len([]rune(stringValue(result["category"]))) > 64 || len([]rune(stringValue(result["description"]))) > 512 {
		return nil, errors.New("shopping_item_invalid")
	}
	return result, nil
}
func shoppingRequestedItems(request map[string]any) ([]map[string]any, error) {
	var sources []any
	if raw, exists := request["items"]; exists {
		var ok bool
		sources, ok = raw.([]any)
		if !ok || len(sources) < 1 || len(sources) > 8 {
			return nil, errors.New("shopping_bundle_invalid")
		}
		if stringValue(request["category"]) != "" || stringValue(request["description"]) != "" {
			return nil, errors.New("shopping_single_bundle_conflict")
		}
	} else {
		sources = []any{request}
	}
	result := make([]map[string]any, 0, len(sources))
	for _, raw := range sources {
		item, err := normalizeShoppingItem(mapValue(raw))
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}
func acquiredShoppingItems(result map[string]any) []map[string]any {
	if item := mapValue(result["acquired_item"]); len(item) > 0 {
		return []map[string]any{item}
	}
	items := make([]map[string]any, 0)
	for _, raw := range arrayValue(result["acquired_items"]) {
		items = append(items, mapValue(raw))
	}
	return items
}
func validateShoppingAcquisition(request, result map[string]any) error {
	requested, err := shoppingRequestedItems(request)
	if err != nil {
		return err
	}
	acquired := acquiredShoppingItems(result)
	if len(mapValue(result["acquired_item"])) > 0 && len(arrayValue(result["acquired_items"])) > 0 {
		return errors.New("shopping_acquisition_sources_conflict")
	}
	if len(acquired) == 0 {
		return nil
	}
	if len(acquired) != len(requested) {
		return errors.New("shopping_bundle_partial_result")
	}
	for i, item := range acquired {
		normalized, err := normalizeShoppingItem(item)
		if err != nil || normalized["category"] != requested[i]["category"] || normalized["slot"] != requested[i]["slot"] || normalized["item_kind"] != requested[i]["item_kind"] || normalized["description"] != requested[i]["description"] {
			return errors.New("virtual_shopping_item_mismatch")
		}
	}
	return nil
}
