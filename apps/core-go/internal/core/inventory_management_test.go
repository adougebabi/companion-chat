package core

import (
	"fmt"
	"net/url"
	"testing"
)

func TestOwnerInventoryPagingScopedDetailHistoryAndArchiveRetainsUseEvidence(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "inventory-management")
	var revision int
	if err := repo.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fid).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	path := "/internal/fluctlights/" + fid + "/inventory/items"
	item := map[string]any{"item_kind": "object", "category": "book", "slot": "", "description": "INVENTORY-MANAGEMENT-TEST book", "ownership": "owned", "availability": "available"}
	body := map[string]any{"item": item, "expected_revision": revision, "reason": "Owner explicit known item fixture", "idempotency_key": "add-object"}
	first, err := app.DispatchOwnerSurface(ctx, "POST", path, nil, owner, body)
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(mapValue(first["item"])["id"])
	revision = intValue(first["revision"])
	replay, err := app.DispatchOwnerSurface(ctx, "POST", path, nil, owner, body)
	if err != nil || mapValue(replay["item"])["id"] != id {
		t.Fatal("add replay duplicated inventory", replay, err)
	}
	// Staged current-use fixture; management must retain the real history and
	// distinguish correction from an autonomous stop Tool.
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_item_uses(fluctlight_id,item_id,activity,started_at,source_event_id) VALUES($1,$2,'reading',now(),'explicit-use-fixture')`, fid, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_item_use_events(id,fluctlight_id,item_id,operation,activity,occurred_at,source_ref,item_revision) VALUES('inventory-use-fixture',$1,$2,'start','reading',now(),'explicit-management-fixture',1)`, fid, id); err != nil {
		t.Fatal(err)
	}
	archived, err := app.DispatchOwnerSurface(ctx, "DELETE", path+"/"+id, nil, owner, map[string]any{"expected_revision": revision, "expected_item_revision": 1, "reason": "Archive registration; preserve evidence", "idempotency_key": "archive-object"})
	if err != nil || mapValue(archived["item"])["archived_at"] == nil {
		t.Fatal("archive failed", archived, err)
	}
	var uses, events int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.fluctlight_item_uses WHERE fluctlight_id=$1 AND item_id=$2),(SELECT count(*) FROM public.fluctlight_item_use_events WHERE fluctlight_id=$1 AND item_id=$2)`, fid, id).Scan(&uses, &events); err != nil || uses != 0 || events != 2 {
		t.Fatal("active/history not separated", uses, events, err)
	}
	history, err := app.OwnerInventoryHistory(ctx, owner, fid, id, url.Values{})
	if err != nil || len(arrayValue(history["items"])) < 4 {
		t.Fatal("history missing", history, err)
	}
	after, err := app.OwnerInventoryItem(ctx, owner, fid, id)
	if err != nil || after["use"] != nil || after["availability"] != "unavailable" {
		t.Fatal("archived item still used", after, err)
	}
	current, err := app.OwnerInventoryList(ctx, owner, fid, url.Values{"q": {"INVENTORY-MANAGEMENT"}})
	if err != nil || len(arrayValue(current["items"])) != 0 {
		t.Fatal("archive still current", current, err)
	}
	archive, err := app.OwnerInventoryList(ctx, owner, fid, url.Values{"state": {"archived"}, "q": {"INVENTORY-MANAGEMENT"}})
	if err != nil || len(arrayValue(archive["items"])) != 1 {
		t.Fatal("archive unreadable", archive, err)
	}
	// A later page is independently addressable by id, regardless of active list.
	for i := 0; i < 47; i++ {
		next := cloneMap(item)
		next["description"] = fmt.Sprintf("MULTIPAGE-%03d", i)
		var rev int
		if err := repo.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fid).Scan(&rev); err != nil {
			t.Fatal(err)
		}
		if _, err := app.DispatchOwnerSurface(ctx, "POST", path, nil, owner, map[string]any{"item": next, "expected_revision": rev, "reason": "explicit pagination fixture", "idempotency_key": fmt.Sprint("add-page-", i)}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for {
		page, err := app.OwnerInventoryList(ctx, owner, fid, url.Values{"q": {"MULTIPAGE"}, "limit": {"9"}, "cursor": {cursor}})
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range arrayValue(page["items"]) {
			item := mapValue(raw)
			id := stringValue(item["id"])
			if seen[id] {
				t.Fatal("duplicate item")
			}
			seen[id] = true
			if _, err := app.OwnerInventoryItem(ctx, owner, fid, id); err != nil {
				t.Fatal(err)
			}
		}
		cursor = stringValue(page["next_cursor"])
		if cursor == "" {
			break
		}
	}
	if len(seen) != 47 {
		t.Fatal("truncated inventory", len(seen))
	}
}

func TestLegacyInventorySlotCorrectionCannotDuplicateCurrentWearing(t *testing.T) {
	ctx, _, app, owner, fid, _ := setupDirectPublicationToolTest(t, "inventory-slot-correction")
	id := "owner-slot-fixture"
	item := map[string]any{"id": id, "item_kind": "wearable", "category": "top", "slot": "upper_body", "description": "explicit known garment", "ownership": "unknown", "availability": "available", "worn": true}
	if _, err := app.AddWardrobeItems(ctx, owner, fid, item); err != nil {
		t.Fatal(err)
	}
	item["category"], item["slot"] = "coat", "outer"
	if _, err := app.AddWardrobeItems(ctx, owner, fid, item); err != nil {
		t.Fatal(err)
	}
	detail, err := app.OwnerInventoryItem(ctx, owner, fid, id)
	if err != nil || mapValue(detail["worn"])["slot"] != "outer" || detail["ownership"] != "unknown" {
		t.Fatal("corrected slot/unknown ownership state inconsistent", detail, err)
	}
	var count int
	if err := app.DB.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, fid, id).Scan(&count); err != nil || count != 1 {
		t.Fatal("same item worn twice", count, err)
	}
}
