package core

import (
	"errors"
	"net/http"
	"testing"
)

func TestOwnerWardrobeEditFollowsCharacterRevision(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	bootsID := wardrobeItemID(t, fixture, "黑色短靴")
	coatID := wardrobeItemID(t, fixture, "旧外套")
	before := wardrobeRevision(t, fixture)
	worn, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeWearCapabilityName, "character-boots", map[string]any{"mode": "partial", "item_ids": []string{bootsID}}))
	if err != nil || worn.Result.Status != "completed" {
		t.Fatalf("character wear: %#v err=%v", worn, err)
	}
	afterCharacter := wardrobeRevision(t, fixture)
	if afterCharacter != before+1 {
		t.Fatalf("character wear revision %d -> %d", before, afterCharacter)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_wardrobe_states SET wearing_state='unknown' WHERE fluctlight_id=$1`, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	wearEvents := outboxCount(t, fixture, "wardrobe.wearing.changed")

	stale, err := fixture.app.DispatchOwnerSurface(fixture.ctx, http.MethodPost, "/internal/fluctlights/"+fixture.fluctlightID+"/wardrobe/wear", nil, fixture.ownerID, map[string]any{
		"idempotency_key": "owner-stale", "item_id": coatID, "slot": "outerwear", "expected_revision": before, "reason": "所有者修改", "confirm": true,
	})
	var surface *SurfaceError
	if !errors.As(err, &surface) || surface.Code != "revision_conflict" || surface.Status != http.StatusConflict || intValue(surface.Details["current_revision"]) != afterCharacter {
		t.Fatalf("stale owner wear = %#v err=%v", stale, err)
	}
	if got := wardrobeRevision(t, fixture); got != afterCharacter {
		t.Fatalf("stale owner wear changed revision to %d", got)
	}

	saved, err := fixture.app.DispatchOwnerSurface(fixture.ctx, http.MethodPost, "/internal/fluctlights/"+fixture.fluctlightID+"/wardrobe/wear", nil, fixture.ownerID, map[string]any{
		"idempotency_key": "owner-current", "item_id": coatID, "slot": "outerwear", "expected_revision": afterCharacter, "reason": "所有者修改", "confirm": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if intValue(saved["revision"]) != afterCharacter+1 || saved["identity"] != "owner_edit" || saved["actor"] != "owner" || saved["recorded_by"] != "owner" || intValue(saved["owner_revision"]) != 1 {
		t.Fatalf("owner wear = %#v", saved)
	}
	if got := wardrobeRevision(t, fixture); got != afterCharacter+1 {
		t.Fatalf("owner wear revision = %d", got)
	}
	var wornCoat, wearingState string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT item_id FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND slot='outerwear'`, fixture.fluctlightID).Scan(&wornCoat); err != nil || wornCoat != coatID {
		t.Fatalf("owner wear did not land on worn items: %s %v", wornCoat, err)
	}
	var bootsStillWorn int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND slot='shoes' AND item_id=$2`, fixture.fluctlightID, bootsID).Scan(&bootsStillWorn); err != nil || bootsStillWorn != 1 {
		t.Fatalf("owner wear replaced the character's other slot: %d %v", bootsStillWorn, err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT wearing_state FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&wearingState); err != nil || wearingState != "unknown" {
		t.Fatalf("owner wear changed wearing_state to %s (%v)", wearingState, err)
	}
	if got := outboxCount(t, fixture, "wardrobe.wearing.changed"); got != wearEvents {
		t.Fatalf("owner wear emitted wardrobe.wearing.changed: %d -> %d", wearEvents, got)
	}
	var identity, actor, action string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT identity, actor, action FROM public.owner_surface_audits WHERE fluctlight_id=$1 AND action='wear'`, fixture.fluctlightID).Scan(&identity, &actor, &action); err != nil || identity != "owner_edit" || actor != "owner" || action != "wear" {
		t.Fatalf("audit = %s %s %s err=%v", identity, actor, action, err)
	}
	replayed, err := fixture.app.DispatchOwnerSurface(fixture.ctx, http.MethodPost, "/internal/fluctlights/"+fixture.fluctlightID+"/wardrobe/wear", nil, fixture.ownerID, map[string]any{
		"idempotency_key": "owner-current", "item_id": coatID, "slot": "outerwear", "expected_revision": afterCharacter, "reason": "所有者修改", "confirm": true,
	})
	if err != nil || replayed["_replay"] != true || wardrobeRevision(t, fixture) != afterCharacter+1 {
		t.Fatalf("replay = %#v err=%v revision=%d", replayed, err, wardrobeRevision(t, fixture))
	}

	added, err := fixture.app.AddWardrobeItems(fixture.ctx, fixture.ownerID, fixture.fluctlightID, map[string]any{"item_kind": "object", "category": "art_supply", "slot": "", "description": "细头画笔", "ownership": "owned", "availability": "available"})
	if err != nil {
		t.Fatal(err)
	}
	brushID := stringValue(mapValue(arrayValue(added["items"])[0])["id"])
	beforeUse := wardrobeRevision(t, fixture)
	started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(itemUseCapabilityName, "character-brush", map[string]any{"operation": "start", "item_id": brushID, "activity": "画画"}))
	if err != nil || started.Result.Status != "completed" {
		t.Fatalf("character use: %#v err=%v", started, err)
	}
	afterUse := wardrobeRevision(t, fixture)
	if afterUse != beforeUse+1 {
		t.Fatalf("character use revision %d -> %d", beforeUse, afterUse)
	}
	useEvents := outboxCount(t, fixture, "item.use.changed")
	var useEventRows int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_item_use_events WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&useEventRows); err != nil {
		t.Fatal(err)
	}
	_, err = fixture.app.DispatchOwnerSurface(fixture.ctx, http.MethodPost, "/internal/fluctlights/"+fixture.fluctlightID+"/wardrobe/items/"+brushID+"/use", nil, fixture.ownerID, map[string]any{
		"idempotency_key": "owner-use-stale", "active": false, "expected_revision": beforeUse, "reason": "所有者修改", "confirm": true,
	})
	var useConflict *SurfaceError
	if !errors.As(err, &useConflict) || intValue(useConflict.Details["current_revision"]) != afterUse {
		t.Fatalf("stale owner use = %v", err)
	}
	stopped, err := fixture.app.DispatchOwnerSurface(fixture.ctx, http.MethodPost, "/internal/fluctlights/"+fixture.fluctlightID+"/wardrobe/items/"+brushID+"/use", nil, fixture.ownerID, map[string]any{
		"idempotency_key": "owner-use-current", "active": false, "expected_revision": afterUse, "reason": "所有者修改", "confirm": true,
	})
	if err != nil || intValue(stopped["revision"]) != afterUse+1 || stopped["identity"] != "owner_edit" {
		t.Fatalf("owner use = %#v err=%v", stopped, err)
	}
	var stillUsed int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_item_uses WHERE fluctlight_id=$1 AND item_id=$2`, fixture.fluctlightID, brushID).Scan(&stillUsed); err != nil || stillUsed != 0 {
		t.Fatalf("owner use did not clear the real use row: %d %v", stillUsed, err)
	}
	if got := outboxCount(t, fixture, "item.use.changed"); got != useEvents {
		t.Fatalf("owner use emitted item.use.changed: %d -> %d", useEvents, got)
	}
	var eventsAfter int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_item_use_events WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&eventsAfter); err != nil || eventsAfter != useEventRows {
		t.Fatalf("owner use wrote item use events: %d -> %d err=%v", useEventRows, eventsAfter, err)
	}
}

func wardrobeItemID(t *testing.T, fixture independentToolE2EFixture, description string) string {
	t.Helper()
	var id string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND description=$2`, fixture.fluctlightID, description).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func wardrobeRevision(t *testing.T, fixture independentToolE2EFixture) int {
	t.Helper()
	var revision int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	return revision
}

func outboxCount(t *testing.T, fixture independentToolE2EFixture, kind string) int {
	t.Helper()
	var count int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind=$2`, fixture.fluctlightID, kind).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
