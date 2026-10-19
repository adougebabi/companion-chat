package core

import (
	"errors"
	"net/url"
	"testing"
)

func TestInventoryCorrectionsPreserveOutfitDefinitionAndRejectPartialSet(t *testing.T) {
	for _, correction := range []string{"archive", "slot", "kind"} {
		t.Run(correction, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			worn, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
			if err != nil || len(worn) < 2 {
				t.Fatal(worn, err)
			}
			ids := []any{worn[0]["id"], worn[1]["id"]}
			var revision int
			readRevision := func() int {
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&revision); err != nil {
					t.Fatal(err)
				}
				return revision
			}
			base := "/internal/fluctlights/" + f.fluctlightID + "/inventory"
			saved, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/outfits", nil, f.ownerID, map[string]any{"name": "unchanged definition", "item_ids": ids, "profile_id": "default", "expected_revision": readRevision(), "expected_outfit_revision": 0, "reason": "explicit test set", "idempotency_key": "set-before-correction"})
			if err != nil {
				t.Fatal(err)
			}
			outfitID := stringValue(mapValue(saved["outfit"])["id"])
			id := stringValue(ids[0])
			item, err := f.app.OwnerInventoryItem(f.ctx, f.ownerID, f.fluctlightID, id)
			if err != nil {
				t.Fatal(err)
			}
			body := map[string]any{"expected_revision": readRevision(), "expected_item_revision": item["revision"], "reason": "Owner corrects inventory, not outfit", "idempotency_key": "correct-member"}
			method := "DELETE"
			if correction != "archive" {
				method = "PUT"
				value := map[string]any{}
				for _, key := range []string{"item_kind", "category", "slot", "description", "ownership", "availability"} {
					value[key] = item[key]
				}
				if correction == "slot" {
					value["slot"] = "corrected_slot"
				} else {
					value["item_kind"], value["slot"] = "object", ""
				}
				body["item"] = value
			}
			if _, err := f.app.DispatchOwnerSurface(f.ctx, method, base+"/items/"+id, nil, f.ownerID, body); err != nil {
				t.Fatal(err)
			}
			outfit, err := f.app.OwnerInventoryOutfit(f.ctx, f.ownerID, f.fluctlightID, outfitID)
			if err != nil || intValue(outfit["revision"]) != 1 || len(arrayValue(outfit["items"])) != 2 {
				t.Fatal("inventory correction silently rewrote set", outfit, err)
			}
			_, err = f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/requests", nil, f.ownerID, map[string]any{"kind": "outfit", "outfit_id": outfitID, "profile_id": "default", "expected_revision": readRevision(), "expected_outfit_revision": 1, "reason": "must not shrink set to eligible member", "idempotency_key": "reject-partial-set"})
			var surface *SurfaceError
			if !errors.As(err, &surface) || surface.Code != "outfit_items_unavailable" {
				t.Fatal("invalid member did not block complete set", err)
			}
			history, err := f.app.OwnerInventoryOutfitHistory(f.ctx, f.ownerID, f.fluctlightID, outfitID, url.Values{})
			if err != nil || len(arrayValue(history["items"])) != 1 {
				t.Fatal("inventory correction created outfit revision", history, err)
			}
			f.requireNoModelRuns(t)
		})
	}
}

func TestOwnerOutfitSaveRevisionsReuseToolWriterAndNeverDress(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	before, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	ids := []any{}
	for _, item := range before {
		ids = append(ids, item["id"])
	}
	var wardrobe int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&wardrobe); err != nil {
		t.Fatal(err)
	}
	path := "/internal/fluctlights/" + f.fluctlightID + "/inventory/outfits"
	body := map[string]any{"name": "OWNER-OUTFIT-TEST", "item_ids": ids, "profile_id": "default", "expected_revision": wardrobe, "expected_outfit_revision": 0, "reason": "Save known garment set, not wearing", "idempotency_key": "save-owner-outfit"}
	saved, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	outfit := mapValue(saved["outfit"])
	id := stringValue(outfit["id"])
	if id == "" || outfit["name"] != "OWNER-OUTFIT-TEST" {
		t.Fatal("saved outfit not readable", saved)
	}
	repeated, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || mapValue(repeated["outfit"])["id"] != id {
		t.Fatal("save replay duplicated outfit", repeated, err)
	}
	next := cloneMap(body)
	next["name"], next["expected_revision"], next["expected_outfit_revision"], next["idempotency_key"] = "OWNER-OUTFIT-RENAMED", saved["revision"], 1, "update-owner-outfit"
	updated, err := f.app.DispatchOwnerSurface(f.ctx, "PUT", path+"/"+id, nil, f.ownerID, next)
	if err != nil || intValue(mapValue(updated["outfit"])["revision"]) != 2 {
		t.Fatal("update failed", updated, err)
	}
	history, err := f.app.OwnerInventoryOutfitHistory(f.ctx, f.ownerID, f.fluctlightID, id, url.Values{})
	if err != nil || len(arrayValue(history["items"])) != 2 {
		t.Fatal("outfit history lost", history, err)
	}
	if mapValue(mapValue(arrayValue(history["items"])[0])["owner_audit"])["reason"] != "Save known garment set, not wearing" {
		t.Fatal("Owner reason not readable", history)
	}
	// The pre-existing registered Tool calls the same writer and leaves wearing intact.
	request := f.request(wardrobeOutfitSaveCapabilityName, "save-tool-outfit", map[string]any{"name": "TOOL-OUTFIT-TEST", "item_ids": ids})
	request.Surface = CapabilitySurfaceConversation
	receipt, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatal(receipt, err)
	}
	after, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil || jsonString(before) != jsonString(after) {
		t.Fatal("catalogue save changed actual wearing", after, err)
	}
	f.requireNoModelRuns(t)
}

func TestActualWearingHistorySeparatesActionsOwnerCorrectionsAndCatalogueSaves(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var item string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND category='boots'`, f.fluctlightID).Scan(&item); err != nil {
		t.Fatal(err)
	}
	before, err := f.app.OwnerWearingHistory(f.ctx, f.ownerID, f.fluctlightID, url.Values{})
	if err != nil || len(arrayValue(before["items"])) != 0 {
		t.Fatal("initialization fabricated wearing history", before, err)
	}
	receipt, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "wear-history-boots", map[string]any{"mode": "partial", "item_ids": []any{item}}))
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatal(receipt, err)
	}
	after, err := f.app.OwnerWearingHistory(f.ctx, f.ownerID, f.fluctlightID, url.Values{})
	if err != nil || len(arrayValue(after["items"])) != 1 {
		t.Fatal("actual wearing not recorded", after, err)
	}
	row := mapValue(arrayValue(after["items"])[0])
	if row["source_kind"] != "tool_action" || len(arrayValue(row["after_items"])) != 3 || len(arrayValue(row["before_items"])) != 2 {
		t.Fatal("wear snapshot/source inconsistent", row)
	}
	current, err := f.app.OwnerInventoryItem(f.ctx, f.ownerID, f.fluctlightID, item)
	if err != nil {
		t.Fatal(err)
	}
	var revision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	_, err = f.app.DispatchOwnerSurface(f.ctx, "DELETE", "/internal/fluctlights/"+f.fluctlightID+"/inventory/items/"+item, nil, f.ownerID, map[string]any{"expected_revision": revision, "expected_item_revision": current["revision"], "reason": "Owner归档修正，不是角色脱下行为", "idempotency_key": "wear-history-archive"})
	if err != nil {
		t.Fatal(err)
	}
	history, err := f.app.OwnerWearingHistory(f.ctx, f.ownerID, f.fluctlightID, url.Values{})
	if err != nil || len(arrayValue(history["items"])) != 2 {
		t.Fatal(history, err)
	}
	newest := mapValue(arrayValue(history["items"])[0])
	if newest["source_kind"] != "owner_correction" || mapValue(newest["owner_audit"])["reason"] != "Owner归档修正，不是角色脱下行为" {
		t.Fatal("Owner correction mislabeled as action", newest)
	}
}
