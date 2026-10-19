package core

import "testing"

func TestInventoryOwnerProposalQueuesRealFactWithoutAcquisitionOrModelCall(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var revision, count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision,(SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1) FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&revision, &count); err != nil {
		t.Fatal(err)
	}
	envelope, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/inventory/requests", nil, f.ownerID, map[string]any{"kind": "borrow", "expected_revision": revision, "reason": "提出请求而非已借到", "idempotency_key": "request-ordinary-book", "lender": "图书馆", "items": []any{map[string]any{"item_kind": "object", "category": "book", "slot": "", "description": "真实请求待角色决定"}}})
	if err != nil {
		t.Fatal(err)
	}
	request := mapValue(envelope["request"])
	if request["status"] != "pending" || request["kind"] != "inventory_borrow" || stringValue(request["source_fact_id"]) == "" {
		t.Fatal("request not durably pending", request)
	}
	var unchanged bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*)=$2 FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1) AND EXISTS(SELECT 1 FROM public.cognition_inbox WHERE id=$3 AND fluctlight_id=$1) AND EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE payload->>'inbox_id'=$3)`, f.fluctlightID, count, request["source_fact_id"]).Scan(&unchanged); err != nil || !unchanged {
		t.Fatal("API acquired an item or failed to queue fact/task", unchanged, err)
	}
	f.requireNoModelRuns(t)
}

func TestOwnerInventoryActionScopeRejectsTargetChangesAndAllowsOrderIndependence(t *testing.T) {
	tests := []struct {
		name, kind    string
		request, args map[string]any
		want          bool
	}{
		{"return reorder", "inventory_return", map[string]any{"item_ids": []any{"b", "a"}}, map[string]any{"item_ids": []any{"a", "b"}}, true},
		{"return target injection", "inventory_return", map[string]any{"item_ids": []any{"a"}}, map[string]any{"item_ids": []any{"a", "other"}}, false},
		{"remove exact", "inventory_remove", map[string]any{"slots": []any{"outer"}}, map[string]any{"mode": "partial", "item_ids": []any{}, "remove_slots": []any{"outer"}}, true},
		{"remove becomes full", "inventory_remove", map[string]any{"slots": []any{"outer"}}, map[string]any{"mode": "full", "item_ids": []any{}, "remove_slots": []any{"outer"}}, false},
		{"outfit members", "inventory_outfit", map[string]any{"item_ids": []any{"b", "a"}}, map[string]any{"mode": "full", "item_ids": []any{"a", "b"}}, true},
		{"outfit extra removal", "inventory_outfit", map[string]any{"item_ids": []any{"a"}}, map[string]any{"mode": "full", "item_ids": []any{"a"}, "remove_slots": []any{"outer"}}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ownerInventoryArgumentsMatch(test.kind, test.request, test.args); got != test.want {
				t.Fatalf("scope result %v want %v", got, test.want)
			}
		})
	}
}
