package core

import (
	"errors"
	"testing"
)

func TestOwnerOutfitConflictsReportActualCause(t *testing.T) {
	cases := []struct {
		name, code, message, update string
		status                      int
	}{
		{"profile_missing", "outfit_profile_not_found", "指定的人格不存在，请核对人格标识", "", 404},
		{"profile_mismatch", "outfit_profile_mismatch", "指定的人格与当前人格不匹配，请选择当前人格的套装", "", 409},
		{"scope", "outfit_scope_mismatch", "套装不属于当前人格，请选择当前人格或共用的套装", "UPDATE public.fluctlight_wardrobe_outfits SET profile_id='other' WHERE fluctlight_id=$1", 409},
		{"revision", "outfit_revision_conflict", "套装版本已更新，请重新读取套装后重试", "", 409},
		{"wardrobe_revision", "wardrobe_revision_conflict", "衣橱版本已更新，请重新读取库存后重试", "", 409},
		{"archive", "outfit_item_archived", "套装包含已归档的物品，请先恢复或替换该物品；套装定义保持", "UPDATE public.fluctlight_wardrobe_items SET archived_at=now() WHERE fluctlight_id=$1 AND id=$2", 409},
		{"kind", "outfit_item_not_wearable", "套装包含非衣物物品，请核对物品类型；套装定义保持", "UPDATE public.fluctlight_wardrobe_items SET item_kind='object',slot='' WHERE fluctlight_id=$1 AND id=$2", 409},
		{"slot", "outfit_item_slot_mismatch", "套装记录的穿着部位与物品当前部位不符，请更新套装；套装定义保持", "UPDATE public.fluctlight_wardrobe_items SET slot='outerwear' WHERE fluctlight_id=$1 AND id=$2", 409},
		{"availability", "outfit_items_unavailable", "套装包含当前不可用的物品，请先处理库存；套装定义保持", "UPDATE public.fluctlight_wardrobe_items SET availability='unavailable' WHERE fluctlight_id=$1 AND id=$2", 409},
		{"ownership", "outfit_item_not_held", "套装包含当前未持有且未穿着的物品，请先核对持有情况；套装定义保持", "UPDATE public.fluctlight_wardrobe_items SET ownership='unknown' WHERE fluctlight_id=$1 AND id=$2", 409},
		{"source", "outfit_item_source_unverified", "套装包含来源未经核实的物品，请先核实物品来源；套装定义保持", "UPDATE public.fluctlight_wardrobe_items SET source_kind='accepted_event',source_ref='unverified' WHERE fluctlight_id=$1 AND id=$2", 409},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// This fixture creates and later drops its own database, never clearing shared rows.
			f := seedWardrobeToolFixture(t)
			boots := wardrobeItemID(t, f, "黑色短靴")
			base := "/internal/fluctlights/" + f.fluctlightID + "/inventory"
			saved, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/outfits", nil, f.ownerID, map[string]any{"name": "test outfit", "item_ids": []any{boots}, "profile_id": "default", "expected_revision": wardrobeRevision(t, f), "expected_outfit_revision": 0, "reason": "save recorded outfit", "idempotency_key": "save-conflict-fixture"})
			if err != nil {
				t.Fatal(err)
			}
			outfitID := stringValue(mapValue(saved["outfit"])["id"])
			persona := map[string]any{"personality_system": map[string]any{"active_profile_id": "default", "profiles": []any{map[string]any{"id": "default"}, map[string]any{"id": "other"}}}}
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, f.fluctlightID, jsonBytes(persona)); err != nil {
				t.Fatal(err)
			}
			if tc.update != "" {
				args := []any{f.fluctlightID}
				if tc.name != "scope" {
					args = append(args, boots)
				}
				if _, err := f.repository.Pool().Exec(f.ctx, tc.update, args...); err != nil {
					t.Fatal(err)
				}
			}
			revision := wardrobeRevision(t, f)
			body := map[string]any{"kind": "outfit", "outfit_id": outfitID, "profile_id": "default", "expected_revision": revision, "expected_outfit_revision": 1, "reason": "request exact outfit", "idempotency_key": "conflicting-outfit"}
			switch tc.name {
			case "profile_missing":
				body["profile_id"] = "typo"
			case "profile_mismatch":
				body["profile_id"] = "other"
			case "revision":
				body["expected_outfit_revision"] = 2
			case "wardrobe_revision":
				body["expected_revision"] = revision - 1
			}
			before, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/requests", nil, f.ownerID, body)
			var surface *SurfaceError
			if !errors.As(err, &surface) || surface.Status != tc.status || surface.Code != tc.code || surface.Message != tc.message {
				t.Fatalf("want %d %s %q; got %#v (%v)", tc.status, tc.code, tc.message, surface, err)
			}
			// Execution-time revalidation must retain the same cause in its receipt.
			tx, err := f.repository.Pool().Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			c := ownerLifeCapability{app: f.app, owner: f.ownerID, fid: f.fluctlightID, kind: "inventory_outfit", request: body}
			result, runErr := c.ExecuteTx(f.ctx, tx, CapabilityInvocation{}, CapabilityContext{})
			if err := tx.Rollback(f.ctx); err != nil {
				t.Fatal(err)
			}
			if !errors.As(runErr, &surface) || surface.Code != tc.code || surface.Message != tc.message || result.ErrorCode != tc.code || result.Status != "failed" {
				t.Fatalf("execution lost conflict cause: result=%#v err=%v", result, runErr)
			}
			if got := ownerRequestFailureReason(runErr); got != tc.message {
				t.Fatalf("failure feedback lost cause: got %q want %q", got, tc.message)
			}
			after, err := readCurrentWornItems(f.ctx, f.repository.Pool(), f.fluctlightID)
			if err != nil || jsonString(before) != jsonString(after) || wardrobeRevision(t, f) != revision {
				t.Fatal("rejection changed wearing or wardrobe", after, err)
			}
			var requests, facts, members, outfitRevision int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.owner_surface_records WHERE fluctlight_id=$1 AND kind='owner_request'),(SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND event_type=$2),(SELECT count(*) FROM public.fluctlight_wardrobe_outfit_items WHERE fluctlight_id=$1 AND outfit_id=$3),(SELECT revision FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND id=$3)`, f.fluctlightID, ownerLifeRequestFactType, outfitID).Scan(&requests, &facts, &members, &outfitRevision); err != nil || requests != 0 || facts != 0 || members != 1 || outfitRevision != 1 {
				t.Fatal("rejection enqueued work or changed outfit", requests, facts, members, outfitRevision, err)
			}
			f.requireNoModelRuns(t)
		})
	}
}

func TestOwnerOutfitConflictChecksPreserveEligibleMembers(t *testing.T) {
	for _, scenario := range []string{"current_profile", "shared", "borrowed", "unknown_but_worn"} {
		t.Run(scenario, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			description := "黑色短靴"
			if scenario == "unknown_but_worn" {
				description = "白衬衫"
			}
			item := wardrobeItemID(t, f, description)
			base := "/internal/fluctlights/" + f.fluctlightID + "/inventory"
			saved, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/outfits", nil, f.ownerID, map[string]any{"name": "eligible outfit", "item_ids": []any{item}, "profile_id": "default", "expected_revision": wardrobeRevision(t, f), "expected_outfit_revision": 0, "reason": "save eligible outfit", "idempotency_key": "eligible-outfit"})
			if err != nil {
				t.Fatal(err)
			}
			id := stringValue(mapValue(saved["outfit"])["id"])
			if scenario == "shared" {
				_, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_outfits SET profile_id='' WHERE fluctlight_id=$1 AND id=$2`, f.fluctlightID, id)
			} else if scenario == "borrowed" {
				_, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_items SET ownership='borrowed' WHERE fluctlight_id=$1 AND id=$2`, f.fluctlightID, item)
			}
			if err != nil {
				t.Fatal(err)
			}
			admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/requests", nil, f.ownerID, map[string]any{"kind": "outfit", "outfit_id": id, "profile_id": "default", "expected_revision": wardrobeRevision(t, f), "expected_outfit_revision": 1, "reason": "request valid outfit", "idempotency_key": "request-eligible-outfit"})
			request := mapValue(admitted["request"])
			if err != nil || request["status"] != "pending" || !sameStringSet(mapValue(request["payload"])["item_ids"], []string{item}) {
				t.Fatal("eligible outfit rejected or members changed", admitted, err)
			}
			f.requireNoModelRuns(t)
		})
	}
}
