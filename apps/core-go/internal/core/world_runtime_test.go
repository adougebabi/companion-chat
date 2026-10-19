package core

import (
	"testing"
	"time"
)

func TestWorldNPCPersistenceIsolationEncounterAndSource(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	command := func(method, path string, body map[string]any) map[string]any {
		t.Helper()
		if body != nil {
			body["idempotency_key"] = randomID("world_test_")
			body["reason"] = "isolated world integration test"
		}
		result, err := f.app.DispatchWorldSurface(f.ctx, method, path, nil, f.ownerID, body)
		if err != nil {
			t.Fatal(path, err)
		}
		return result
	}
	base := "/internal/fluctlights/" + f.fluctlightID
	before, err := f.app.WorldSnapshot(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || before["world"] != nil {
		t.Fatal(before, err)
	}
	command("PUT", base+"/world", map[string]any{"name": "Test world", "background": "Test setting", "rules": "No permanent harm", "expected_revision": 0})
	place := command("POST", base+"/world/places", map[string]any{"name": "Test bookshop", "description": "Scoped place", "expected_revision": 0})
	resource, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "npc", "name": "Bookshop keeper", "body": map[string]any{"role": "keeper", "background": "Published background"}, "reason": "Create a definition", "idempotency_key": randomID("definition_")})
	if err != nil {
		t.Fatal(err)
	}
	rid := stringValue(resource["id"])
	published := command("POST", "/internal/studio/resources/"+rid+"/publish", map[string]any{"expected_revision": intValue(resource["revision"])})
	fl, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	var relationshipsBefore int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.relationships WHERE owner_fluctlight_id=$1`, f.fluctlightID).Scan(&relationshipsBefore); err != nil {
		t.Fatal(err)
	}
	applied := command("POST", "/internal/studio/resources/"+rid+"/apply", map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": published["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 0, "home_place_id": place["id"], "choices": []any{}})
	if applied["applied"] != true {
		t.Fatal(applied)
	}
	npcID := stringValue(applied["id"])
	npc := command("GET", base+"/npcs/"+npcID, nil)
	if npc["id"] == npc["actor_id"] || npc["id"] == rid || npc["full_fluctlight"] != false {
		t.Fatal("NPC identities conflated", npc)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.relationships WHERE owner_fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != relationshipsBefore {
		t.Fatal("creation invented acquaintance", count, err)
	}
	_, contextBefore, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		command("GET", base+"/world", nil)
		command("GET", base+"/world/places/"+stringValue(place["id"]), nil)
	}
	_, contextAfter, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, f.app.now())
	if err != nil || contextBefore["location"] != contextAfter["location"] || contextBefore["context_revision"] != contextAfter["context_revision"] {
		t.Fatal("browsing changed life", contextBefore, contextAfter, err)
	}
	now := f.app.now().UTC()
	encounter := map[string]any{"confirmed": true, "summary": "Met the keeper at the bookshop", "start_at": now.Add(-10 * time.Minute).Format(time.RFC3339Nano), "end_at": now.Add(-5 * time.Minute).Format(time.RFC3339Nano), "place_id": place["id"], "evidence_refs": []any{f.conversationID}, "expected_life_context_revision": contextAfter["context_revision"], "idempotency_key": randomID("encounter_test_"), "reason": "Owner confirms this test encounter occurred"}
	recorded, err := f.app.DispatchWorldSurface(f.ctx, "POST", base+"/npcs/"+npcID+"/encounters", nil, f.ownerID, encounter)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.app.DispatchWorldSurface(f.ctx, "POST", base+"/npcs/"+npcID+"/encounters", nil, f.ownerID, encounter)
	if err != nil || replay["id"] != recorded["id"] || replay["_replay"] != true {
		t.Fatal("encounter replay", replay, err)
	}
	npc = command("GET", base+"/npcs/"+npcID, nil)
	if len(arrayValue(npc["encounters"])) != 1 || len(arrayValue(mapValue(npc["relationship_context"])["relationships"])) != 1 {
		t.Fatal(npc)
	}
	if _, err := f.app.DispatchWorldSurface(f.ctx, "DELETE", base+"/npcs/"+npcID, nil, f.ownerID, map[string]any{"expected_revision": npc["revision"], "reason": "Reference guard", "idempotency_key": randomID("npc_delete_")}); err == nil {
		t.Fatal("deleted referenced NPC")
	}
	var linked bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.world_encounters e JOIN public.memories m ON m.id=e.memory_id JOIN public.life_events l ON l.id=e.life_event_id JOIN public.relationships r ON r.id=e.relationship_id WHERE e.id=$1 AND m.event_refs ? l.id AND r.target_actor_id=$2)`, recorded["id"], npc["actor_id"]).Scan(&linked); err != nil || !linked {
		t.Fatal("event, memory and relationship lost provenance", linked, err)
	}
	if _, err := f.app.WorldSnapshot(f.ctx, f.foreignOwnerID, f.fluctlightID); err == nil {
		t.Fatal("foreign owner read world")
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-only-hash','world-test') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	other, err := f.app.CreateFluctlight(f.ctx, f.ownerID, randomID("world_other_"), "Other", "blank_slate", "", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	command("PUT", "/internal/fluctlights/"+other.ID+"/world", map[string]any{"name": "Other world", "expected_revision": 0})
	otherApply := command("POST", "/internal/studio/resources/"+rid+"/apply", map[string]any{"fluctlight_id": other.ID, "resource_version": published["published_version"], "expected_instance_revision": other.CurrentRevision, "expected_runtime_revision": 0, "choices": []any{}})
	if otherApply["id"] == npcID || otherApply["actor_id"] == npc["actor_id"] {
		t.Fatal("template instances share identity", otherApply)
	}
	wrong := map[string]any{"name": "Cross-world", "description": "must reject", "parent_place_id": place["id"], "expected_revision": 0, "idempotency_key": randomID("invalid_place_"), "reason": "scope test"}
	if _, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/fluctlights/"+other.ID+"/world/places", nil, f.ownerID, wrong); err == nil {
		t.Fatal("cross-world place accepted")
	}
}

func TestNPCPlannedPositionNeverClaimsConfirmation(t *testing.T) {
	zone, _ := time.LoadLocation("Asia/Shanghai")
	npc := map[string]any{"home_place_id": "home", "routine": []any{map[string]any{"day": "weekday", "start": "22:00", "end": "06:00", "place_id": "shop", "activity": "Night shift"}}}
	predicted := npcPlannedPosition(npc, time.Date(2026, 10, 17, 2, 0, 0, 0, zone))
	if predicted["place_id"] != "shop" || predicted["source"] != "schedule_prediction" {
		t.Fatal(predicted)
	}
	initial := npcPlannedPosition(npc, time.Date(2026, 10, 17, 15, 0, 0, 0, zone))
	if initial["source"] != "initial_place" {
		t.Fatal(initial)
	}
}
