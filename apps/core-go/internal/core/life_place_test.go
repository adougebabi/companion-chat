package core

import (
	"testing"
	"time"
)

func TestLifeStablePlaceUsesExplicitScheduleAndEventIdentity(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	base := "/internal/fluctlights/" + f.fluctlightID
	world := func(method, path string, body map[string]any) map[string]any {
		t.Helper()
		if body != nil {
			body["reason"] = "Stable place integration"
			body["idempotency_key"] = randomID("place_test_")
		}
		v, e := f.app.DispatchWorldSurface(f.ctx, method, path, nil, f.ownerID, body)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	world("PUT", base+"/world", map[string]any{"name": "World A", "expected_revision": 0})
	p1 := world("POST", base+"/world/places", map[string]any{"name": "Same name"})
	p2 := world("POST", base+"/world/places", map[string]any{"name": "Same name"})
	at := f.app.now().UTC()
	life := currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
	plan := fullDaySchedulePayloadForTest(at, "place-schedule", stringValue(life["context_revision"]))
	for _, raw := range arrayValue(plan["items"]) {
		mapValue(raw)["place_id"] = p1["id"]
	}
	if _, err := f.app.AcceptSchedule(f.ctx, f.ownerID, f.fluctlightID, plan); err != nil {
		t.Fatal(err)
	}
	life = currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
	if life["place_id"] != p1["id"] || life["source"] != "schedule" {
		t.Fatal("schedule prediction lost explicit identity", life)
	}
	event := map[string]any{"kind": "visit", "start_at": at.Add(-time.Minute).Format(time.RFC3339Nano), "end_at": at.Add(time.Hour).Format(time.RFC3339Nano), "scene": "indoors", "activity": "reading", "location": "Same name", "place_id": p2["id"], "evidence_refs": []any{"owner:" + f.ownerID}, "expected_life_context_revision": life["context_revision"], "idempotency_key": "place-arrival"}
	result, err := f.app.CreateLifeEvent(f.ctx, f.ownerID, f.fluctlightID, event)
	if err != nil {
		t.Fatal(err)
	}
	life = currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
	if life["place_id"] != p2["id"] || life["source"] != "event" || life["context_revision"] != result["resulting_context_revision"] {
		t.Fatal("event receipt/projection disagree", life, result)
	}
	for i := 0; i < 2; i++ {
		nearby := world("GET", base+"/nearby", nil)
		if nearby["place_id"] != p2["id"] {
			t.Fatal(nearby)
		}
	}
	if _, err := f.app.CreateLifeEvent(f.ctx, f.ownerID, f.fluctlightID, event); err != nil {
		t.Fatal("event replay", err)
	}
	bad := cloneMap(event)
	bad["idempotency_key"] = "bad-place"
	bad["place_id"] = "missing-place"
	bad["expected_life_context_revision"] = life["context_revision"]
	if _, err := f.app.CreateLifeEvent(f.ctx, f.ownerID, f.fluctlightID, bad); err == nil {
		t.Fatal("unavailable place accepted")
	}
	other := world("POST", base+"/world/catalog", map[string]any{"name": "World B"})
	world("PUT", base+"/world/active", map[string]any{"world_id": other["id"], "expected_revision": 1})
	after := currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
	if after["place_id"] != nil || after["context_revision"] == life["context_revision"] {
		t.Fatal("world switch leaked old place or retained stale authority", after)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatal("reads/replay created events", count, err)
	}
}
