package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestWardrobeBorrowWearReturnUsesExistingInventoryAuthority(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	request := f.request(wardrobeBorrowCapabilityName, "borrow-shop-shirt", map[string]any{
		"lender": "服装店", "reason": "店员允许临时试穿",
		"items": []any{map[string]any{"category": "shirt", "slot": "top", "description": "借用蓝衬衫"}},
	})
	borrowed, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || borrowed.Result.Status != "completed" {
		t.Fatalf("borrow: %#v err=%v", borrowed, err)
	}
	var startedAt, endedAt, expiresAt time.Time
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT start_at,end_at,expires_at FROM public.life_events WHERE fluctlight_id=$1 AND kind='wardrobe_gain' ORDER BY created_at DESC LIMIT 1`, f.fluctlightID).Scan(&startedAt, &endedAt, &expiresAt); err != nil || !endedAt.After(startedAt) || !expiresAt.Equal(endedAt) {
		t.Fatalf("borrow Event timestamps invalid: start=%v end=%v expiry=%v err=%v", startedAt, endedAt, expiresAt, err)
	}
	item := mapValue(arrayValue(mapValue(borrowed.Result.Output)["items"])[0])
	id := stringValue(item["item_id"])
	if id == "" || item["ownership"] != "borrowed" || item["availability"] != "available" {
		t.Fatalf("borrowed item identity/state missing: %#v", item)
	}
	var verified bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT `+inventorySourceVerifiedSQL+` FROM public.fluctlight_wardrobe_items i WHERE i.fluctlight_id=$1 AND i.id=$2`, f.fluctlightID, id).Scan(&verified); err != nil || !verified {
		t.Fatalf("borrow has no valid Event source: verified=%v err=%v", verified, err)
	}
	var worn int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, f.fluctlightID, id).Scan(&worn); err != nil || worn != 0 {
		t.Fatalf("borrow automatically wore clothing: count=%d err=%v", worn, err)
	}
	replayed, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || !replayed.Replayed || stringValue(mapValue(arrayValue(mapValue(replayed.Result.Output)["items"])[0])["item_id"]) != id {
		t.Fatalf("borrow replay duplicated item: %#v err=%v", replayed, err)
	}
	wear, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "wear-shop-shirt", map[string]any{"mode": "partial", "item_ids": []string{id}}))
	if err != nil || len(arrayValue(mapValue(wear.Result.Output)["items"])) != 2 {
		t.Fatalf("partial replacement lost unrelated accessory: %#v err=%v", wear, err)
	}
	// A batch failure must not partially return its first item or remove wearing.
	failed, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeReturnCapabilityName, "bad-return-batch", map[string]any{"reason": "归还", "item_ids": []string{id, "wardrobe_missing"}}))
	if err == nil || failed.Result.ErrorCode != "wardrobe_item_not_found" {
		t.Fatalf("missing borrowed item accepted: %#v err=%v", failed, err)
	}
	var availability string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT availability FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2`, f.fluctlightID, id).Scan(&availability); err != nil || availability != "available" {
		t.Fatalf("failed return partially committed: availability=%s err=%v", availability, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, f.fluctlightID, id).Scan(&worn); err != nil || worn != 1 {
		t.Fatalf("failed return removed wearing: count=%d err=%v", worn, err)
	}
	returnRequest := f.request(wardrobeReturnCapabilityName, "return-shop-shirt", map[string]any{"reason": "试穿结束归还店员", "item_ids": []string{id}})
	returned, err := f.app.ExecuteTool(f.ctx, returnRequest)
	if err != nil || mapValue(arrayValue(mapValue(returned.Result.Output)["items"])[0])["availability"] != "unavailable" {
		t.Fatalf("return: %#v err=%v", returned, err)
	}
	replayed, err = f.app.ExecuteTool(f.ctx, returnRequest)
	if err != nil || !replayed.Replayed {
		t.Fatalf("return replay: %#v err=%v", replayed, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, f.fluctlightID, id).Scan(&worn); err != nil || worn != 0 {
		t.Fatalf("returned shirt remains worn: count=%d err=%v", worn, err)
	}
	if receipt, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "rewear-returned-shirt", map[string]any{"mode": "partial", "item_ids": []string{id}})); err == nil || receipt.Result.ErrorCode != "wardrobe_item_unavailable" {
		t.Fatalf("returned item still wearable: %#v err=%v", receipt, err)
	}
}

func TestBorrowingPersistenceFailureSeparatesPermanentSQLFromTransientErrors(t *testing.T) {
	invocation := CapabilityInvocation{CallID: "loan", CapabilityName: wardrobeBorrowCapabilityName}
	for _, test := range []struct {
		name      string
		err       error
		retryable bool
	}{
		{"timestamp type", &pgconn.PgError{Code: "42804"}, false},
		{"wrapped missing column", fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "42703"}), false},
		{"serialization", &pgconn.PgError{Code: "40001"}, true},
		{"connection", &pgconn.PgError{Code: "08006"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := borrowingPersistenceFailure(invocation, "borrowing_event_failed", test.err)
			if result.Status != "failed" || result.ErrorCode != "borrowing_event_failed" || result.Retryable != test.retryable {
				t.Fatalf("wrong persistence failure classification: %#v", result)
			}
			if !test.retryable {
				visible := modelFacingToolResult(ToolExecutionReceipt{Result: result}, wardrobeBorrowDefinition(false))
				if !strings.Contains(stringValue(mapValue(visible["output"])["detail"]), "物品状态没有改变") {
					t.Fatalf("model does not see unsuccessful atomic outcome: %#v", visible)
				}
			}
		})
	}
}

func TestWardrobeReturnRejectsOwnedItemsAndForeignOwner(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	list, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeInspectCapabilityName, "list-owned-boots", map[string]any{"operation": "list", "category": "boots"}))
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(mapValue(arrayValue(mapValue(list.Result.Output)["items"])[0])["id"])
	request := f.request(wardrobeReturnCapabilityName, "return-owned-boots", map[string]any{"reason": "归还", "item_ids": []string{id}})
	if receipt, err := f.app.ExecuteTool(f.ctx, request); err == nil || receipt.Result.ErrorCode != "borrowed_item_not_returnable" {
		t.Fatalf("owned item returned: %#v err=%v", receipt, err)
	}
	request.AuthorizationActorID = f.foreignOwnerID
	if _, err := f.app.ExecuteTool(f.ctx, request); err == nil {
		t.Fatal("foreign Owner returned clothing")
	}
}

func TestWardrobePartialConflictExplainsReplacementCorrection(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	list, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeInspectCapabilityName, "list-conflict-boots", map[string]any{"operation": "list", "category": "boots"}))
	if err != nil {
		t.Fatal(err)
	}
	item := mapValue(arrayValue(mapValue(list.Result.Output)["items"])[0])
	id, slot := stringValue(item["id"]), stringValue(item["slot"])
	conflict, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "conflicting-replacement", map[string]any{"mode": "partial", "item_ids": []string{id}, "remove_slots": []string{slot}}))
	visible := modelFacingToolResult(conflict, wardrobeWearDefinition())
	if err == nil || conflict.Result.ErrorCode != "wardrobe_slot_conflict" || !strings.Contains(stringValue(mapValue(visible["output"])["detail"]), "Remove the overlapping slot") {
		t.Fatalf("model has no actionable correction: %#v err=%v", visible, err)
	}
	corrected, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "corrected-replacement", map[string]any{"mode": "partial", "item_ids": []string{id}}))
	if err != nil || corrected.Result.Status != "completed" {
		t.Fatalf("corrected partial failed: %#v err=%v", corrected, err)
	}
}
