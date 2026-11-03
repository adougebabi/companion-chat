package core

import (
	"errors"
	"fmt"
)

type ownerWardrobeItem struct {
	clothingType string
	seasons      []string
	seasonsSet   bool
	id           string
	itemKind     string
	category     string
	slot         string
	description  string
	ownership    string
	availability string
	worn         bool
}

// Only fixed field names and reasons enter this public error. Never include
// submitted values, storage errors, or provider diagnostics in its details.
func wardrobeItemInputError(index int, field, reason string) *SurfaceError {
	message := map[string]string{
		"required": "不能为空", "invalid": "不合法", "must_be_text": "必须为文本",
		"must_be_array": "必须为数组", "must_be_object": "必须为对象",
		"must_be_boolean": "必须为布尔值", "must_be_empty": "必须为空（普通物品不可设置穿戴位置）",
		"cannot_be_worn": "不能为 true（普通物品不能穿戴）", "slot_mismatch": "必须与 slot（穿戴位置）匹配",
	}[reason]
	message = field + " 字段" + message
	details := map[string]any{"field": field, "reason": reason}
	if index >= 0 {
		details["item_index"] = index + 1
		message = fmt.Sprintf("第 %d 件物品：%s", index+1, message)
	}
	return &SurfaceError{Status: 422, Code: "wardrobe_item_invalid", Message: message, Details: details}
}

func validateWardrobeItemFieldTypes(item map[string]any, index int) error {
	for _, field := range []string{"id", "item_kind", "category", "slot", "description", "ownership", "availability", "clothing_type"} {
		if raw, present := item[field]; present {
			if _, ok := raw.(string); !ok {
				return wardrobeItemInputError(index, field, "must_be_text")
			}
		}
	}
	if raw, present := item["worn"]; present {
		if _, ok := raw.(bool); !ok {
			return wardrobeItemInputError(index, "worn", "must_be_boolean")
		}
	}
	return nil
}

func ownerWardrobeMetadata(item map[string]any, index int) (string, []string, error) {
	clothing, seasons, err := wardrobeMetadata(item)
	var validation *SurfaceError
	if errors.As(err, &validation) {
		switch validation.Message {
		case "clothing type must match the wearing slot":
			err = wardrobeItemInputError(index, "clothing_type", "slot_mismatch")
		case "seasons must be an array":
			err = wardrobeItemInputError(index, "seasons", "must_be_array")
		case "unknown clothing season":
			err = wardrobeItemInputError(index, "seasons", "invalid")
		}
	}
	return clothing, seasons, err
}

func parseOwnerWardrobeItems(payload map[string]any) ([]ownerWardrobeItem, error) {
	var rawItems []any
	if raw, present := payload["items"]; present {
		var ok bool
		rawItems, ok = raw.([]any)
		if !ok {
			return nil, wardrobeItemInputError(-1, "items", "must_be_array")
		}
	} else if len(payload) > 0 {
		rawItems = []any{payload}
	}
	if len(rawItems) == 0 {
		return nil, wardrobeItemInputError(-1, "items", "required")
	}
	items := make([]ownerWardrobeItem, 0, len(rawItems))
	for i, raw := range rawItems {
		m, ok := raw.(map[string]any)
		if !ok || m == nil {
			return nil, wardrobeItemInputError(i, "item", "must_be_object")
		}
		if err := validateWardrobeItemFieldTypes(m, i); err != nil {
			return nil, err
		}
		itemKind := firstString(m["item_kind"], "wearable")
		category, slot, description := stringValue(m["category"]), stringValue(m["slot"]), stringValue(m["description"])
		ownership := firstString(m["ownership"], "owned")
		availability := firstString(m["availability"], "available")
		switch {
		case itemKind != "wearable" && itemKind != "object":
			return nil, wardrobeItemInputError(i, "item_kind", "invalid")
		case category == "":
			return nil, wardrobeItemInputError(i, "category", "required")
		case description == "":
			return nil, wardrobeItemInputError(i, "description", "required")
		case itemKind == "wearable" && slot == "":
			return nil, wardrobeItemInputError(i, "slot", "required")
		case itemKind == "object" && slot != "":
			return nil, wardrobeItemInputError(i, "slot", "must_be_empty")
		case ownership != "owned" && ownership != "borrowed" && ownership != "unknown":
			return nil, wardrobeItemInputError(i, "ownership", "invalid")
		case availability != "available" && availability != "unavailable" && availability != "lost":
			return nil, wardrobeItemInputError(i, "availability", "invalid")
		}
		worn, _ := m["worn"].(bool)
		if itemKind == "object" && worn {
			return nil, wardrobeItemInputError(i, "worn", "cannot_be_worn")
		}
		metadata := cloneMap(m)
		metadata["item_kind"], metadata["slot"] = itemKind, slot
		clothing, seasons, err := ownerWardrobeMetadata(metadata, i)
		if err != nil {
			return nil, err
		}
		id := stringValue(m["id"])
		if id == "" {
			id = "wardrobe_" + randomID("item_")
		}
		_, seasonsSet := m["seasons"]
		items = append(items, ownerWardrobeItem{
			id: id, clothingType: clothing, seasons: seasons, seasonsSet: seasonsSet,
			itemKind: itemKind, category: category, slot: slot, description: description,
			ownership: ownership, availability: availability, worn: worn,
		})
	}
	return items, nil
}
