package core

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

var clothingTypes = map[string]bool{"outer": true, "upper_body": true, "underwear": true, "panties": true, "lower_body": true, "socks": true, "shoes": true, "accessory": true}

func clothingTypeForSlot(slot string) string {
	if clothingTypes[slot] {
		return slot
	}
	return map[string]string{"outerwear": "outer", "top": "upper_body", "bottom": "lower_body", "accessories": "accessory", "head": "accessory", "neck": "accessory", "wrist": "accessory", "hands": "accessory", "feet": "shoes"}[slot]
}
func multipleWardrobeSlot(slot string) bool { return slot == "accessory" || slot == "accessories" }
func wardrobeSlotKey(slot, id string) string {
	if multipleWardrobeSlot(slot) {
		return slot + "\x1f" + id
	}
	return slot
}
func wardrobeStoredSlot(key string) string { return strings.SplitN(key, "\x1f", 2)[0] }

func wardrobeCoversBody(items []map[string]any) bool {
	upper, lower := false, false
	for _, item := range items {
		slot := stringValue(item["slot"])
		if slot == "full_body" {
			upper, lower = true, true
		}
		if clothingTypeForSlot(slot) == "upper_body" {
			upper = true
		}
		if clothingTypeForSlot(slot) == "lower_body" {
			lower = true
		}
	}
	return upper && lower
}

func putWornItemTx(ctx context.Context, tx pgx.Tx, fid, slot, id string) error {
	if !multipleWardrobeSlot(slot) {
		if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND slot=$2 AND item_id<>$3`, fid, slot, id); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id) VALUES($1,$2,$3) ON CONFLICT(fluctlight_id,item_id) DO UPDATE SET slot=EXCLUDED.slot,changed_at=now()`, fid, slot, id)
	return err
}

func wardrobeMetadata(item map[string]any) (string, []string, error) {
	kind, slot := stringValue(item["item_kind"]), stringValue(item["slot"])
	typeName := stringValue(item["clothing_type"])
	if typeName == "" && kind == "wearable" {
		typeName = clothingTypeForSlot(slot)
	}
	if typeName != "" && (!clothingTypes[typeName] || kind != "wearable" || clothingTypeForSlot(slot) != typeName) {
		return "", nil, invalid("clothing type must match the wearing slot")
	}
	seasons := []string{}
	if raw, present := item["seasons"]; present && raw != nil {
		if _, ok := raw.([]any); !ok {
			if _, ok := raw.([]string); !ok {
				return "", nil, invalid("seasons must be an array")
			}
		}
		seen := map[string]bool{}
		for _, season := range stringList(raw) {
			name := stringValue(season)
			if name != "spring" && name != "summer" && name != "autumn" && name != "winter" {
				return "", nil, invalid("unknown clothing season")
			}
			if !seen[name] {
				seasons = append(seasons, name)
				seen[name] = true
			}
		}
	}
	return typeName, seasons, nil
}

func normalizedOutfitTags(raw any) ([]string, error) {
	if raw == nil {
		return []string{}, nil
	}
	var values []string
	switch tags := raw.(type) {
	case []any:
		for _, v := range tags {
			s, ok := v.(string)
			if !ok {
				return nil, invalid("tag must be text")
			}
			values = append(values, s)
		}
	case []string:
		values = tags
	default:
		return nil, invalid("tags must be an array")
	}
	result := []string{}
	seen := map[string]bool{}
	for _, tag := range values {
		tag = strings.TrimSpace(tag)
		if tag == "" || len([]rune(tag)) > 40 {
			return nil, invalid("tag must contain 1–40 characters")
		}
		if !seen[tag] {
			result = append(result, tag)
			seen[tag] = true
		}
	}
	if len(result) > 32 {
		return nil, invalid("at most 32 tags per outfit")
	}
	return result, nil
}

func usableWardrobeCoverage(items []map[string]any) bool {
	usable := []map[string]any{}
	for _, item := range items {
		if item["source_verified"] == true && item["availability"] == "available" && (item["ownership"] == "owned" || item["ownership"] == "borrowed") {
			usable = append(usable, item)
		}
	}
	return wardrobeCoversBody(usable)
}
