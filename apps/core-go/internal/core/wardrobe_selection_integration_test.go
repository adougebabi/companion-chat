package core

import (
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Requires the isolated PostgreSQL test database. All model I/O is controlled.
func TestWardrobeSelectionRepeatedInventoryCompletesRealWear(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "wardrobe-echo-controlled")
	var preparation string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		preparation, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "echo-test")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	router := newFakeProviderRouter().on("wardrobe_preparation_v1", func(payload map[string]any) fakeProviderResult {
		calls++
		messages := arrayValue(payload["messages"])
		input := decodeObject([]byte(stringValue(mapValue(messages[1])["content"])))
		if input["select_only"] != true {
			return fakeProviderResult{Structured: map[string]any{
				"items": []any{map[string]any{"key": "top", "description": "回归上衣", "clothing_type": "upper_body", "seasons": []any{}}, map[string]any{"key": "bottom", "description": "回归长裤", "clothing_type": "lower_body", "seasons": []any{}}},
				"plans": []any{map[string]any{"name": "日常", "tags": []any{"通用"}, "item_refs": []any{"top", "bottom"}}}, "wear_item_refs": []any{}, "reason": "准备初始库存",
			}}
		}
		echoes, refs := []any{}, []any{}
		for _, raw := range arrayValue(input["registered_items"]) {
			item := mapValue(raw)
			if item["description"] != "回归上衣" && item["description"] != "回归长裤" {
				continue
			}
			echoes = append(echoes, map[string]any{"key": item["id"], "description": item["description"], "clothing_type": item["clothing_type"], "seasons": []any{}})
			refs = append(refs, item["id"])
		}
		if len(refs) != 2 {
			t.Fatal("selection did not receive committed inventory", refs)
		}
		return fakeProviderResult{Structured: map[string]any{"items": echoes, "plans": []any{}, "wear_item_refs": refs, "reason": "重复列出已有衣物作为选择"}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	first, err := f.app.ProcessWardrobePreparation(f.ctx, preparation)
	if err != nil || first["inventory_committed"] != true || first["status"] != "awaiting_wear" {
		t.Fatal(first, err)
	}
	// Same fake selection on every attempt: the old implementation rejects it
	// three times; the fixed initializer must finish on the first selection.
	var second map[string]any
	for attempt := 0; attempt < 3; attempt++ {
		second, err = f.app.ProcessWardrobePreparation(f.ctx, preparation)
		if err == nil && second["status"] == "completed" {
			break
		}
	}
	if err != nil || second["status"] != "completed" || calls != 2 {
		t.Fatalf("selection did not complete immediately: result=%v calls=%d err=%v", second, calls, err)
	}
	worn, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || len(worn) != 2 {
		t.Fatal("wear Tool did not persist selected inventory", worn, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_ref=$2`, f.fluctlightID, "wardrobe-preparation:"+preparation).Scan(&count); err != nil || count != 2 {
		t.Fatal("selection duplicated inventory", count, err)
	}
	public, err := f.app.OwnerWardrobePreparation(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || public["status"] != "completed" || mapValue(public["result"])["wear_receipt"] == nil {
		t.Fatal(public, err)
	}
	if _, err := f.app.ProcessWardrobePreparation(f.ctx, preparation); err != nil || calls != 2 {
		t.Fatal("replay repeated model work", calls, err)
	}
}
