package core

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestWorldCatalogCreateSelectScopeCycleReferencesAndReplay(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	base := "/internal/fluctlights/" + f.fluctlightID
	call := func(method, path string, body map[string]any) (map[string]any, error) {
		t.Helper()
		if body != nil {
			body["reason"] = "World catalog regression"
			if body["idempotency_key"] == nil {
				body["idempotency_key"] = randomID("catalog_test_")
			}
		}
		return f.app.DispatchWorldSurface(f.ctx, method, path, nil, f.ownerID, body)
	}
	must := func(method, path string, body map[string]any) map[string]any {
		t.Helper()
		v, e := call(method, path, body)
		if e != nil {
			t.Fatal(path, e)
		}
		return v
	}
	rejected := func(method, path string, body map[string]any, code string) {
		t.Helper()
		_, e := call(method, path, body)
		var surface *SurfaceError
		if !errors.As(e, &surface) || surface.Code != code {
			t.Fatal("expected", code, "got", e)
		}
	}
	first := must("PUT", base+"/world", map[string]any{"name": "Same name", "expected_revision": 0, "flavor": "Quiet", "item_pool": "Books and raincoats"})
	catalogBody := map[string]any{"name": "Same name", "description": "Independent", "flavor": "Lively", "item_pool": "Flowers"}
	second := must("POST", base+"/world/catalog", catalogBody)
	replay := must("POST", base+"/world/catalog", catalogBody)
	if replay["id"] != second["id"] || replay["_replay"] != true {
		t.Fatal(replay)
	}
	snap := must("GET", base+"/world", nil)
	if mapValue(snap["world"])["id"] != first["id"] {
		t.Fatal("create changed binding", snap)
	}
	cat := must("GET", base+"/world/catalog", nil)
	if len(arrayValue(cat["items"])) != 2 {
		t.Fatal(cat)
	}
	must("PUT", base+"/world/active", map[string]any{"world_id": second["id"], "expected_revision": 1})
	oldSnapshot := must("GET", base+"/world/catalog/"+stringValue(first["id"]), nil)
	if mapValue(oldSnapshot["world"])["id"] != first["id"] {
		t.Fatal(oldSnapshot)
	}
	parent := must("POST", base+"/world/places", map[string]any{"name": "Room", "description": "A", "alias": "Study", "group": "Home", "floor": "2", "scene": "asset-reference", "atmosphere": "Calm", "furniture": "Desk", "layout": "Window east", "connections": "Unmapped corridor", "room_items": "One lamp"})
	child := must("POST", base+"/world/places", map[string]any{"name": "Room", "parent_place_id": parent["id"]})
	rejected("PUT", base+"/world/places/"+stringValue(parent["id"]), map[string]any{"name": "Room", "parent_place_id": child["id"], "expected_revision": 1}, "invalid_request")
	rejected("DELETE", base+"/world/places/"+stringValue(parent["id"]), map[string]any{"expected_revision": 1}, "resource_referenced")
	must("DELETE", base+"/world/places/"+stringValue(child["id"]), map[string]any{"expected_revision": 1})
	detail := must("GET", base+"/world/places/"+stringValue(parent["id"]), nil)
	if detail["furniture"] != "Desk" || detail["room_items"] != "One lamp" {
		t.Fatal(detail)
	}
	must("PUT", base+"/world/active", map[string]any{"world_id": first["id"], "expected_revision": 2})
	if _, e := call("GET", base+"/world/places/"+stringValue(parent["id"]), nil); e == nil {
		t.Fatal("place escaped active world")
	}
	must("PUT", base+"/world/active", map[string]any{"world_id": second["id"], "expected_revision": 3})
	must("GET", base+"/world/places/"+stringValue(parent["id"]), nil)
	rejected("PUT", base+"/world/active", map[string]any{"world_id": first["id"], "expected_revision": 1}, "revision_conflict")
	rejected("DELETE", base+"/world/catalog/"+stringValue(second["id"]), map[string]any{"expected_revision": 1}, "resource_referenced")
	rejected("PUT", base+"/world", map[string]any{"name": "Bad", "expected_revision": 1, "permanent_harm": true}, "permanent_harm_forbidden")
	must("DELETE", base+"/world/catalog/"+stringValue(first["id"]), map[string]any{"expected_revision": 1})
	nearby := must("GET", base+"/nearby", nil)
	if nearby["place_id"] != nil || len(arrayValue(nearby["actors"])) != 0 {
		t.Fatal("guessed place from text", nearby)
	}
}

func TestWorldResourceDraftCASConcurrentApplyAndLocalOverrides(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	world := func(method, path string, body map[string]any) (map[string]any, error) {
		if body != nil {
			body["reason"] = "Concurrent world regression"
			body["idempotency_key"] = randomID("world_cas_")
		}
		return f.app.DispatchWorldSurface(f.ctx, method, path, nil, f.ownerID, body)
	}
	base := "/internal/fluctlights/" + f.fluctlightID
	if _, e := world("PUT", base+"/world", map[string]any{"name": "World", "expected_revision": 0}); e != nil {
		t.Fatal(e)
	}
	resource, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "npc", "name": "Definition", "body": map[string]any{"role": "Keeper", "background": "Original"}, "reason": "Test", "idempotency_key": randomID("resource_")})
	if err != nil {
		t.Fatal(err)
	}
	rid := stringValue(resource["id"])
	rpath := "/internal/studio/resources/" + rid
	pub, err := world("POST", rpath+"/publish", map[string]any{"expected_revision": 1})
	if err != nil {
		t.Fatal(err)
	}
	fl, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := world("POST", rpath+"/apply", map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 0})
	if err != nil {
		t.Fatal(err)
	}
	npc := stringValue(applied["id"])
	if _, err = world("PUT", base+"/npcs/"+npc, map[string]any{"name": "Local name", "role": "Keeper", "background": "Original", "status": "active", "enabled": false, "position_mode": "fixed", "routine": []any{}, "expected_revision": 1}); err != nil {
		t.Fatal(err)
	}
	saved, err := f.app.DispatchOwnerSurface(f.ctx, "PUT", rpath, nil, f.ownerID, map[string]any{"name": "New definition", "body": map[string]any{"role": "New role", "background": "New background"}, "expected_version": pub["published_version"], "reason": "Test", "idempotency_key": randomID("resource_")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.DispatchOwnerSurface(f.ctx, "PUT", rpath, nil, f.ownerID, map[string]any{"name": "Stale", "expected_version": pub["published_version"], "reason": "Test", "idempotency_key": randomID("resource_")})
	if err == nil {
		t.Fatal("old published version overwrote current draft")
	}
	pub, err = world("POST", rpath+"/publish", map[string]any{"expected_revision": saved["revision"]})
	if err != nil {
		t.Fatal(err)
	}
	choices := []any{map[string]any{"field": "name", "decision": "override"}, map[string]any{"field": "role", "decision": "override"}, map[string]any{"field": "background", "decision": "override"}}
	result, err := world("POST", rpath+"/apply", map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 2, "choices": choices})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := world("GET", base+"/npcs/"+npc, nil)
	if err != nil || detail["name"] != "Local name" || detail["role"] != "New role" || detail["enabled"] != false || detail["actor_id"] != applied["actor_id"] {
		t.Fatal(detail, result, err)
	}
	// Two concurrent saves from one draft: exactly one commits.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := f.app.DispatchOwnerSurface(f.ctx, "PUT", rpath, nil, f.ownerID, map[string]any{"name": "Concurrent", "expected_version": pub["published_version"], "reason": "Race", "idempotency_key": randomID("resource_race_")})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	successes := 0
	for e := range errs {
		if e == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("concurrent resource saves", successes)
	}
	// The shared definition lock preserves references from concurrent characters.
	if _, err = f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-only-hash','world-test') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	other, err := f.app.CreateFluctlight(f.ctx, f.ownerID, randomID("world_concurrent_other_"), "Other", "blank_slate", "", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = world("PUT", "/internal/fluctlights/"+other.ID+"/world", map[string]any{"name": "Other world", "expected_revision": 0}); err != nil {
		t.Fatal(err)
	}
	applyErrors := make(chan error, 2)
	for _, target := range []struct {
		fid               string
		instance, runtime int
	}{{f.fluctlightID, fl.CurrentRevision, 3}, {other.ID, other.CurrentRevision, 0}} {
		wg.Add(1)
		go func(target struct {
			fid               string
			instance, runtime int
		}) {
			defer wg.Done()
			_, e := world("POST", rpath+"/apply", map[string]any{"fluctlight_id": target.fid, "resource_version": pub["published_version"], "expected_instance_revision": target.instance, "expected_runtime_revision": target.runtime, "choices": choices})
			applyErrors <- e
		}(target)
	}
	wg.Wait()
	close(applyErrors)
	for e := range applyErrors {
		if e != nil {
			t.Fatal(e)
		}
	}
	var refCount int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT jsonb_array_length(payload->'referenced_by') FROM public.owner_surface_records WHERE id=$1`, rid).Scan(&refCount); err != nil || refCount != 2 {
		t.Fatal("shared definition lost references", refCount, err)
	}
	// A real, bounded Event with stable place identity drives nearby; GET is read only.
	place, err := world("POST", base+"/world/places", map[string]any{"name": "Room"})
	if err != nil {
		t.Fatal(err)
	}
	now := f.app.now()
	event := randomID("world_presence_")
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.world_npcs SET enabled=true,home_place_id=$2 WHERE id=$1`, npc, place["id"]); err != nil {
		t.Fatal(err)
	}
	if _, err = f.repository.Pool().Exec(f.ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,status,scene,activity,location,evidence_refs,idempotency_key,request_digest,result) VALUES($1,$2,'test_presence',$3,$4,'confirmed','Room','Test','Ambiguous name','[]',$1,'00000000000000000000000000000000',$5)`, event, f.fluctlightID, now.Add(-time.Minute), now.Add(time.Minute), jsonBytes(map[string]any{"id": event, "revision": 1, "status": "confirmed", "expected_context_revision": "prior", "resulting_context_revision": "current", "replayed": false, "place_id": place["id"], "npc_actor_id": applied["actor_id"]})); err != nil {
		t.Fatal(err)
	}
	nearby, err := world("GET", base+"/nearby", nil)
	if err != nil || nearby["place_id"] != place["id"] || len(arrayValue(nearby["actors"])) != 1 || mapValue(arrayValue(nearby["actors"])[0])["presence"] != "confirmed" {
		t.Fatal(nearby, err)
	}
	var generation int64
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&generation); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = world("GET", base+"/nearby", nil); err != nil {
			t.Fatal(err)
		}
	}
	var after int64
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&after); err != nil || after != generation {
		t.Fatal("nearby GET wrote state", generation, after, err)
	}
}

func TestWorldCatalogUnboundEditsAndHarmPublicationNPCRemoval(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	base := "/internal/fluctlights/" + f.fluctlightID
	call := func(method, path string, body map[string]any) (map[string]any, error) {
		if body != nil {
			body["idempotency_key"] = randomID("catalog_remove_")
			body["reason"] = "Lifecycle and safety test"
		}
		return f.app.DispatchWorldSurface(f.ctx, method, path, nil, f.ownerID, body)
	}
	created, err := call("POST", base+"/world/catalog", map[string]any{"name": "Unselected"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = call("PUT", base+"/world/catalog/"+stringValue(created["id"]), map[string]any{"name": "Edited unselected", "expected_revision": 1}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := call("GET", base+"/world", nil)
	if err != nil || snapshot["world"] != nil {
		t.Fatal("editing unselected world selected it", snapshot, err)
	}
	if _, err = call("PUT", base+"/world/active", map[string]any{"world_id": created["id"], "expected_revision": 0}); err != nil {
		t.Fatal(err)
	}
	resource, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "npc", "name": "Temporary NPC", "body": map[string]any{}, "reason": "Test", "idempotency_key": randomID("world_resource_")})
	if err != nil {
		t.Fatal(err)
	}
	rid := stringValue(resource["id"])
	path := "/internal/studio/resources/" + rid
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.owner_surface_records SET payload=jsonb_set(payload,'{body}','{"permanent_harm":true}') WHERE id=$1`, rid); err != nil {
		t.Fatal(err)
	}
	if _, err = call("POST", path+"/publish", map[string]any{"expected_revision": 1}); err == nil {
		t.Fatal("publisher accepted permanent harm from persisted draft")
	}
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.owner_surface_records SET payload=jsonb_set(payload,'{body}','{}') WHERE id=$1`, rid); err != nil {
		t.Fatal(err)
	}
	pub, err := call("POST", path+"/publish", map[string]any{"expected_revision": 1})
	if err != nil {
		t.Fatal(err)
	}
	fl, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	npc, err := call("POST", path+"/apply", map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = call("DELETE", base+"/npcs/"+stringValue(npc["id"]), map[string]any{"expected_revision": 1}); err != nil {
		t.Fatal("unreferenced NPC removal", err)
	}
	if _, err = call("GET", base+"/npcs/"+stringValue(npc["id"]), nil); err == nil {
		t.Fatal("removed NPC remained visible")
	}
	// An imported/corrupt publication cannot bypass the same guard on apply.
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.studio_resource_versions SET body='{"permanent_harm":true}' WHERE resource_id=$1`, rid); err != nil {
		t.Fatal(err)
	}
	if _, err = call("POST", path+"/apply", map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 0}); err == nil {
		t.Fatal("apply accepted permanent harm publication")
	}
}

func TestWorldDefinitionApplyRetainsUnselectedCatalogueInstance(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	resource, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "world", "name": "Published world", "body": map[string]any{"description": "A published setting", "flavor": "Quiet", "item_pool": "Books"}, "reason": "Definition apply", "idempotency_key": randomID("world_definition_")})
	if err != nil {
		t.Fatal(err)
	}
	path := "/internal/studio/resources/" + stringValue(resource["id"])
	call := func(endpoint string, body map[string]any) map[string]any {
		t.Helper()
		body["reason"] = "Definition apply"
		body["idempotency_key"] = randomID("world_definition_")
		result, err := f.app.DispatchWorldSurface(f.ctx, "POST", endpoint, nil, f.ownerID, body)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	pub := call(path+"/publish", map[string]any{"expected_revision": 1})
	fl, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	first := call(path+"/apply", map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 0})
	second := call(path+"/apply", map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 1})
	if first["id"] != second["id"] {
		t.Fatal("definition upgrade forked world identity", first, second)
	}
	snapshot, err := f.app.WorldSnapshot(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || snapshot["world"] != nil {
		t.Fatal("definition apply implicitly selected world", snapshot, err)
	}
	detail, err := f.app.worldSnapshot(f.ctx, f.ownerID, f.fluctlightID, stringValue(first["id"]))
	if err != nil || mapValue(detail["world"])["description"] != "A published setting" || mapValue(detail["world"])["item_pool"] != "Books" {
		t.Fatal(detail, err)
	}
}
