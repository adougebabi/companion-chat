package core

import (
	"testing"
	"time"
)

func TestConfirmedWardrobeEventsAcquireAndLoseWithoutFabricatingWearing(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	now := time.Now().UTC()
	life := currentLifeForTest(t, fixture.ctx, fixture.app, fixture.fluctlightID, now)
	gainPayload := map[string]any{
		"kind": "wardrobe_gain", "start_at": now.Add(-time.Minute).Format(time.RFC3339Nano), "end_at": now.Add(time.Minute).Format(time.RFC3339Nano),
		"activity": "接受借用外套", "evidence_refs": []any{"owner:accepted-gift"}, "idempotency_key": "borrowed-coat-" + fixture.suffix,
		"expected_life_context_revision": life["context_revision"],
		"wardrobe_effect":                map[string]any{"category": "coat", "slot": "outerwear", "description": "借来的蓝色外套", "ownership": "borrowed"},
	}
	gained, err := fixture.app.CreateLifeEvent(fixture.ctx, fixture.ownerID, fixture.fluctlightID, gainPayload)
	if err != nil {
		t.Fatalf("confirmed gain rejected: %v", err)
	}
	itemID := stringValue(gained["item_id"])
	if itemID == "" {
		t.Fatalf("gain did not create traceable item: %#v", gained)
	}
	replayed, err := fixture.app.CreateLifeEvent(fixture.ctx, fixture.ownerID, fixture.fluctlightID, gainPayload)
	if err != nil || replayed["replayed"] != true || stringValue(replayed["item_id"]) != itemID {
		t.Fatalf("gain replay duplicated or changed item: %#v err=%v", replayed, err)
	}
	var ownership, sourceKind, sourceRef string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT ownership,source_kind,source_ref FROM public.fluctlight_wardrobe_items WHERE id=$1 AND fluctlight_id=$2`, itemID, fixture.fluctlightID).Scan(&ownership, &sourceKind, &sourceRef); err != nil || ownership != "borrowed" || sourceKind != "accepted_event" || sourceRef != stringValue(gained["id"]) {
		t.Fatalf("gain provenance wrong: ownership=%s source=%s/%s err=%v", ownership, sourceKind, sourceRef, err)
	}
	var worn int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, fixture.fluctlightID, itemID).Scan(&worn); err != nil || worn != 0 {
		t.Fatalf("gain automatically wore item: count=%d err=%v", worn, err)
	}
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeWearCapabilityName, "wear-borrowed-coat", map[string]any{"mode": "partial", "item_ids": []string{itemID}})); err != nil {
		t.Fatal(err)
	}
	life = currentLifeForTest(t, fixture.ctx, fixture.app, fixture.fluctlightID, time.Now().UTC())
	lossPayload := map[string]any{
		"kind": "wardrobe_loss", "start_at": now.Add(-time.Second).Format(time.RFC3339Nano), "end_at": now.Add(time.Minute).Format(time.RFC3339Nano),
		"activity": "归还借用外套", "evidence_refs": []any{"owner:accepted-return"}, "idempotency_key": "return-coat-" + fixture.suffix,
		"expected_life_context_revision": life["context_revision"], "wardrobe_effect": map[string]any{"item_id": itemID},
	}
	lost, err := fixture.app.CreateLifeEvent(fixture.ctx, fixture.ownerID, fixture.fluctlightID, lossPayload)
	if err != nil || stringValue(lost["availability"]) != "lost" {
		t.Fatalf("loss result missing: event=%#v err=%v", lost, err)
	}
	var availability string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT availability FROM public.fluctlight_wardrobe_items WHERE id=$1`, itemID).Scan(&availability); err != nil || availability != "lost" {
		t.Fatalf("lost item remained available: availability=%s err=%v", availability, err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, fixture.fluctlightID, itemID).Scan(&worn); err != nil || worn != 0 {
		t.Fatalf("lost item remained worn: count=%d err=%v", worn, err)
	}
	if receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeWearCapabilityName, "rewear-lost-coat", map[string]any{"mode": "partial", "item_ids": []string{itemID}})); err == nil || receipt.Result.ErrorCode != "wardrobe_item_unavailable" {
		t.Fatalf("lost item was wearable: receipt=%#v err=%v", receipt, err)
	}
}
