package core

import (
	"errors"
	"fmt"
)

func wardrobeSelectionItemIDs(proposal map[string]any, items []map[string]any) ([]string, error) {
	// Compatibility with models that echo inventory in the generation DTO.
	// Resolve only exact IDs or unique description/type pairs; never create or
	// mutate inventory here. Explicit wear refs remain the selection authority.
	if len(arrayValue(proposal["plans"])) > 0 {
		return nil, errors.New("wardrobe_select_only_created_inventory: plans must be empty during selection")
	}
	aliases := map[string]string{}
	for _, item := range items {
		if id := stringValue(item["id"]); id != "" {
			aliases[id] = id
		}
	}
	echoed := []string{}
	for index, raw := range arrayValue(proposal["items"]) {
		echo := mapValue(raw)
		key := stringValue(echo["key"])
		id := aliases[key]
		// An earlier local alias is not evidence of an inventory ID.
		if id != key {
			id = ""
		}
		if id == "" {
			description, clothingType := stringValue(echo["description"]), stringValue(echo["clothing_type"])
			matches := []string{}
			if description != "" && clothingType != "" {
				for _, item := range items {
					if item["description"] == description && item["clothing_type"] == clothingType {
						matches = append(matches, stringValue(item["id"]))
					}
				}
			}
			if len(matches) == 1 {
				id = matches[0]
			}
		}
		if id == "" || key == "" || aliases[key] != "" && aliases[key] != id {
			return nil, fmt.Errorf("wardrobe_select_only_created_inventory: items[%d] key=%q description=%q is new or ambiguous; use registered_items IDs in wear_item_refs and return empty items/plans", index, key, stringValue(echo["description"]))
		}
		aliases[key] = id
		echoed = append(echoed, id)
	}
	ids := decisionServiceRefValues(proposal["wear_item_refs"])
	if len(ids) == 0 {
		ids = echoed
	}
	for i, ref := range ids {
		if id := aliases[ref]; id != "" {
			ids[i] = id
		}
	}
	ids = decisionServiceRefValues(ids)
	if len(ids) == 0 || len(ids) > 16 {
		return nil, errors.New("wardrobe_wear_selection_missing")
	}
	allowed := map[string]bool{}
	selectedItems := []map[string]any{}
	for _, item := range items {
		if item["source_verified"] == true && item["availability"] == "available" && (item["ownership"] == "owned" || item["ownership"] == "borrowed") {
			allowed[stringValue(item["id"])] = true
		}
	}
	for _, itemID := range ids {
		if !allowed[itemID] {
			return nil, errors.New("wardrobe_wear_selection_unavailable")
		}
		for _, item := range items {
			if item["id"] == itemID {
				selectedItems = append(selectedItems, item)
			}
		}
	}
	if !wardrobeCoversBody(selectedItems) {
		return nil, errors.New("wardrobe_initial_wear_incomplete")
	}
	return ids, nil
}
