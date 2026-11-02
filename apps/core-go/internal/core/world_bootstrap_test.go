package core

import (
	"errors"
	"fmt"
	"testing"
)

func TestCharacterAdmissionNeverCreatesWorldAndExplicitDailyCreatesBaseObjects(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-only','world-test') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"blank_slate"} {
		fl, err := f.app.CreateFluctlight(f.ctx, f.ownerID, randomID("no_world_"), "新角色", mode, "", nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := f.app.WorldSnapshot(f.ctx, f.ownerID, fl.ID)
		if err != nil || snapshot["world"] != nil {
			t.Fatal("admission implicitly bound a world", snapshot, err)
		}
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.life_worlds`).Scan(&count); err != nil || count != 0 {
		t.Fatal("admission created a world", count, err)
	}
	path := "/internal/fluctlights/" + f.fluctlightID + "/world"
	body := map[string]any{"name": "日常世界", "creation_preset": "daily", "expected_revision": 0, "reason": "explicit daily choice", "idempotency_key": "explicit-daily"}
	first, err := f.app.DispatchWorldSurface(f.ctx, "PUT", path, nil, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.app.DispatchWorldSurface(f.ctx, "PUT", path, nil, f.ownerID, body)
	if err != nil || again["id"] != first["id"] || again["_replay"] != true {
		t.Fatal("world command lost idempotency", again, err)
	}
	snapshot, err := f.app.WorldSnapshot(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || len(arrayValue(snapshot["places"])) != 8 || len(arrayValue(snapshot["npcs"])) != 2 {
		t.Fatal("base objects missing", snapshot, err)
	}
	for _, raw := range arrayValue(snapshot["npcs"]) {
		npc := mapValue(raw)
		if npc["confirmed_place_id"] != nil || npc["current_presence"] != nil || npc["position_mode"] != "unknown" {
			t.Fatal("configuration invented presence", npc)
		}
	}
	var worlds, relations, encounters int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.life_worlds),(SELECT count(*) FROM public.relationships),(SELECT count(*) FROM public.world_encounters)`).Scan(&worlds, &relations, &encounters); err != nil || worlds != 1 || relations != 0 || encounters != 0 {
		t.Fatal(worlds, relations, encounters, err)
	}
	other := "later-character"
	seedSharedWorldMember(t, f, other, f.ownerID)
	body["idempotency_key"] = "second-daily"
	if _, err := f.app.DispatchWorldSurface(f.ctx, "PUT", "/internal/fluctlights/"+other+"/world", nil, f.ownerID, body); err == nil {
		t.Fatal("created duplicate daily world instead of requiring selection")
	}
	catalog, err := f.app.worldCatalog(f.ctx, other)
	if err != nil || len(arrayValue(catalog["items"])) != 1 || catalog["active_binding"] != nil {
		t.Fatal("later character created or selected a world", catalog, err)
	}
}

func TestExplicitWorldCatalogCreationIncludesBaseObjectsWithoutBinding(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	result, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/world/catalog", nil, f.ownerID, map[string]any{"name": "明确创建的世界", "reason": "explicit create", "idempotency_key": "new-catalog"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.app.worldSnapshot(f.ctx, f.ownerID, f.fluctlightID, stringValue(result["id"]))
	if err != nil || len(arrayValue(snapshot["places"])) != 8 || len(arrayValue(snapshot["npcs"])) != 2 {
		t.Fatal(snapshot, err)
	}
	active, err := f.app.WorldSnapshot(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || active["world"] != nil {
		t.Fatal("catalog creation implicitly selected world", active, err)
	}
}

func TestConcurrentDailyWorldChoicesCreateOnlyOneWorldPerOwner(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedSharedWorldMember(t, f, "concurrent-character", f.ownerID)
	start := make(chan struct{})
	results := make(chan error, 2)
	for i, fid := range []string{f.fluctlightID, "concurrent-character"} {
		go func(i int, fid string) {
			<-start
			_, err := f.app.DispatchWorldSurface(f.ctx, "PUT", "/internal/fluctlights/"+fid+"/world", nil, f.ownerID, map[string]any{"name": "日常世界", "creation_preset": "daily", "expected_revision": 0, "reason": "explicit daily choice", "idempotency_key": fmt.Sprint("daily-", i)})
			results <- err
		}(i, fid)
	}
	close(start)
	success, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else {
			var e *SurfaceError
			if !errors.As(err, &e) || e.Code != "world_choice_required" {
				t.Fatal(err)
			}
			conflicts++
		}
	}
	var worlds int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.life_worlds`).Scan(&worlds); err != nil || worlds != 1 || success != 1 || conflicts != 1 {
		t.Fatal(worlds, success, conflicts, err)
	}
}

func TestWorldCreationRollsBackWhenBaseConfigurationCannotBeMaterialized(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	_, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/world/catalog", nil, f.ownerID, map[string]any{"name": "非法框架", "framework": map[string]any{"schema": "world-framework.v1", "venues": []any{}}, "reason": "atomicity regression", "idempotency_key": "invalid-base"})
	if err == nil {
		t.Fatal("invalid world base accepted")
	}
	var worlds, places, npcs int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.life_worlds),(SELECT count(*) FROM public.world_places),(SELECT count(*) FROM public.world_npcs)`).Scan(&worlds, &places, &npcs); err != nil || worlds != 0 || places != 0 || npcs != 0 {
		t.Fatal("failed creation left empty/partial world", worlds, places, npcs, err)
	}
}
