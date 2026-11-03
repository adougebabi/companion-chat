package core

import (
	"errors"
	"strings"
	"testing"
)

func TestOwnerWardrobeItemInputReportsSafeFieldReasons(t *testing.T) {
	valid := map[string]any{"category": "coat", "slot": "outer", "description": "外套"}
	for _, tc := range []struct {
		name, field, reason string
		payload             map[string]any
	}{
		{"missing category", "category", "required", map[string]any{"slot": "outer", "description": "外套"}},
		{"missing description", "description", "required", map[string]any{"category": "coat", "slot": "outer"}},
		{"missing slot", "slot", "required", map[string]any{"category": "coat", "description": "外套"}},
		{"empty items", "items", "required", map[string]any{"items": []any{}}},
		{"no items", "items", "required", map[string]any{}},
		{"items type", "items", "must_be_array", map[string]any{"items": "http://private.invalid/secret"}},
		{"item type", "item", "must_be_object", map[string]any{"items": []any{"private-value"}}},
		{"null item", "item", "must_be_object", map[string]any{"items": []any{nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOwnerWardrobeItems(tc.payload)
			assertWardrobeItemInputError(t, err, tc.field, tc.reason)
		})
	}
	for _, tc := range []struct {
		field, reason string
		value         any
	}{
		{"item_kind", "invalid", "http://private.invalid/secret"},
		{"category", "must_be_text", 123},
		{"description", "required", "  "},
		{"slot", "must_be_text", false},
		{"ownership", "invalid", "postgres://secret@internal/private"},
		{"availability", "invalid", "SQLSTATE 23505 private"},
		{"ownership", "must_be_text", 1},
		{"worn", "must_be_boolean", "true"},
		{"clothing_type", "slot_mismatch", "shoes"},
		{"clothing_type", "must_be_text", 1},
		{"seasons", "must_be_array", "summer"},
		{"seasons", "invalid", []any{"private-value"}},
		{"seasons", "invalid", []any{42}},
	} {
		t.Run(tc.field+"/"+tc.reason, func(t *testing.T) {
			item := cloneMap(valid)
			item[tc.field] = tc.value
			_, err := parseOwnerWardrobeItems(map[string]any{"items": []any{valid, item}})
			validation := assertWardrobeItemInputError(t, err, tc.field, tc.reason)
			if validation.Details["item_index"] != 2 || !strings.HasPrefix(validation.Message, "第 2 件物品：") {
				t.Fatalf("lost batch position: %#v", validation)
			}
		})
	}
	for _, tc := range []struct{ field, reason string }{{"slot", "must_be_empty"}, {"worn", "cannot_be_worn"}} {
		item := map[string]any{"item_kind": "object", "category": "tool", "description": "画笔"}
		if tc.field == "slot" {
			item["slot"] = "shoes"
		} else {
			item["worn"] = true
		}
		_, err := parseOwnerWardrobeItems(item)
		assertWardrobeItemInputError(t, err, tc.field, tc.reason)
	}
}

func assertWardrobeItemInputError(t *testing.T, err error, field, reason string) *SurfaceError {
	t.Helper()
	var validation *SurfaceError
	if !errors.As(err, &validation) || validation.Status != 422 || validation.Code != "wardrobe_item_invalid" || validation.Details["field"] != field || validation.Details["reason"] != reason {
		t.Fatalf("unexpected validation: %#v (%v)", validation, err)
	}
	if !strings.Contains(validation.Message, field+" 字段") {
		t.Fatalf("missing field: %s", validation.Message)
	}
	for _, secret := range []string{"http://", "postgres://", "SQLSTATE", "private", "secret"} {
		if strings.Contains(validation.Error(), secret) {
			t.Fatalf("validation leaked submitted value: %s", validation.Error())
		}
	}
	return validation
}

func TestOwnerWardrobeItemInputPreservesDefaultsAndMetadata(t *testing.T) {
	items, err := parseOwnerWardrobeItems(map[string]any{"items": []any{
		map[string]any{"id": " coat ", "category": " coat ", "slot": "outer", "description": " 外套 ", "worn": true, "seasons": []any{"spring", "spring", "winter"}},
		map[string]any{"item_kind": "object", "category": "tool", "description": "画笔"},
	}})
	if err != nil || len(items) != 2 {
		t.Fatalf("valid items rejected: %#v, %v", items, err)
	}
	coat, object := items[0], items[1]
	if coat.id != "coat" || coat.category != "coat" || coat.description != "外套" || coat.itemKind != "wearable" || coat.ownership != "owned" || coat.availability != "available" || !coat.worn || coat.clothingType != "outer" || !coat.seasonsSet || strings.Join(coat.seasons, ",") != "spring,winter" {
		t.Fatalf("wearable semantics changed: %#v", coat)
	}
	if object.id == "" || object.slot != "" || object.worn || object.clothingType != "" || object.seasonsSet {
		t.Fatalf("object semantics changed: %#v", object)
	}
}
