package core

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"net/url"
	"testing"
)

// This fixture uses a disposable migrated PostgreSQL database and real domain
// services. It installs no Provider transport and never executes model tasks.
func v3OwnerFixture(t *testing.T, label string) independentToolE2EFixture {
	t.Helper()
	f := newIndependentToolE2EFixture(t, label)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-hash','test-revision')`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	return f
}
func requireV3SurfaceError(t *testing.T, err error, code string) {
	t.Helper()
	var surface *SurfaceError
	if !errors.As(err, &surface) || surface.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func TestV3ResourcePackPostgresRoundTripIntegrityIsolationAndReplay(t *testing.T) {
	f := v3OwnerFixture(t, "v3-pack")
	resource, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "place", "name": "Reading room", "body": map[string]any{"description": "Original saved definition", "alias": "Study", "connection_place_ids": []any{"external-place"}}, "reason": "Owner creates definition", "idempotency_key": "create-place-definition"})
	if err != nil {
		t.Fatal(err)
	}
	rid := stringValue(resource["id"])
	original, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/studio/resources/"+rid, nil, f.ownerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := f.app.ExportOwnerResourcePack(f.ctx, f.ownerID, url.Values{"resourceId": {rid}})
	if err != nil {
		t.Fatal(err)
	}
	if exported["complete"] != true || exported["available"] != true {
		t.Fatal("incomplete public export", exported)
	}
	pack := mapValue(exported["pack"])
	checked, err := f.app.ValidateOwnerResourcePack(f.ctx, f.ownerID, map[string]any{"pack": pack})
	if err != nil {
		t.Fatal(err)
	}
	if checked["valid"] != true || len(arrayValue(checked["external_bindings"])) != 1 {
		t.Fatal("missing binding review", checked)
	}
	input := map[string]any{"pack": pack, "expected_digest": checked["digest"], "reason": "Owner imports verified definition", "idempotency_key": "import-pack"}
	imported, err := f.app.ImportOwnerResourcePack(f.ctx, f.ownerID, input)
	if err != nil {
		t.Fatal(err)
	}
	mapping := arrayValue(imported["resource_versions"])
	if imported["published"] != false || imported["life_write"] != false || len(mapping) != 1 {
		t.Fatal("import claimed runtime effects", imported)
	}
	cloneID := stringValue(mapValue(mapping[0])["resource_id"])
	if cloneID == rid || cloneID == "" {
		t.Fatal("import did not create independent draft", mapping)
	}
	clone, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/studio/resources/"+cloneID, nil, f.ownerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if clone["published_version"] != nil || clone["draft_version"] != "1" || jsonString(clone["body"]) != jsonString(original["body"]) {
		t.Fatal("imported draft/publication mismatch", clone)
	}
	var source []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload->'pack_source' FROM public.owner_surface_records WHERE id=$1`, cloneID).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if decodeObject(source)["source_resource_id"] != rid || decodeObject(source)["pack_digest"] != checked["digest"] {
		t.Fatal("source lost", string(source))
	}
	replay, err := f.app.ImportOwnerResourcePack(f.ctx, f.ownerID, input)
	if err != nil || replay["id"] != imported["id"] || replay["_replay"] != true {
		t.Fatal("original import not replayed", replay, err)
	}
	changed := cloneMap(input)
	changed["reason"] = "different proposal"
	_, err = f.app.ImportOwnerResourcePack(f.ctx, f.ownerID, changed)
	requireV3SurfaceError(t, err, "idempotency_conflict")
	bad := cloneMap(pack)
	bad["created_at"] = "2026-01-01T00:00:00Z"
	_, err = f.app.ValidateOwnerResourcePack(f.ctx, f.ownerID, map[string]any{"pack": bad})
	requireV3SurfaceError(t, err, "resource_pack_invalid")
	if _, err = f.app.OwnerResourcePack(f.ctx, f.foreignOwnerID, stringValue(imported["id"])); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("foreign Owner read pack", err)
	}
	after, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/studio/resources/"+rid, nil, f.ownerID, nil)
	if err != nil || jsonString(original) != jsonString(after) {
		t.Fatal("import changed source", after, err)
	}
	var resources, worlds, places, events int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.owner_surface_records WHERE kind='studio_resource'),(SELECT count(*) FROM public.life_worlds),(SELECT count(*) FROM public.world_places),(SELECT count(*) FROM public.life_events)`).Scan(&resources, &worlds, &places, &events); err != nil || resources != 2 || worlds != 0 || places != 0 || events != 0 {
		t.Fatal("import duplicated or applied definitions", resources, worlds, places, events, err)
	}
	f.requireNoModelRuns(t)
}

func TestV3PlaceDefaultsPostgresRestoreCASCaptureHistoryAndScope(t *testing.T) {
	f := v3OwnerFixture(t, "v3-defaults")
	base := "/internal/fluctlights/" + f.fluctlightID
	worldCall := func(method, path string, body map[string]any) map[string]any {
		t.Helper()
		if body != nil {
			body["idempotency_key"] = randomID("defaults_")
			body["reason"] = "Owner changes place definition"
		}
		result, err := f.app.DispatchWorldSurface(f.ctx, method, path, nil, f.ownerID, body)
		if err != nil {
			t.Fatal(path, err)
		}
		return result
	}
	world := worldCall("PUT", base+"/world", map[string]any{"name": "World", "expected_revision": 0})
	wid := stringValue(world["id"])
	parent := worldCall("POST", base+"/world/places", map[string]any{"name": "Parent"})
	created := worldCall("POST", base+"/world/places", map[string]any{"name": "Original room", "description": "Original description", "alias": "Study", "group": "Home", "floor": "2", "scene": "Reading", "atmosphere": "Quiet", "furniture": []any{"Desk"}, "layout": "Window east", "connections": "Corridor", "room_items": []any{"Lamp"}, "parent_place_id": parent["id"], "connection_place_ids": []any{parent["id"]}})
	pid := stringValue(created["id"])
	query := url.Values{"worldId": {wid}}
	read := func() map[string]any {
		t.Helper()
		v, err := f.app.OwnerPlaceDefaults(f.ctx, f.ownerID, f.fluctlightID, pid, query)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	first := read()
	baseline := mapValue(first["baseline"])
	if baseline["source_kind"] != "initial_saved" || intValue(first["default_revision"]) != 1 || len(arrayValue(first["diff"])) != 0 {
		t.Fatal("initial saved baseline missing", first)
	}
	worldCall("PUT", base+"/world/places/"+pid, map[string]any{"name": "Changed room", "description": "Changed description", "alias": "Edited", "expected_revision": 1, "parent_place_id": parent["id"], "connection_place_ids": []any{parent["id"]}})
	if len(arrayValue(read()["diff"])) == 0 {
		t.Fatal("changed fields have no preview")
	}
	restore := map[string]any{"world_id": wid, "expected_revision": 2, "expected_default_revision": 1, "operation": "restore", "source": "recorded", "reason": "Owner restores saved definition", "idempotency_key": "restore-place"}
	restored, err := f.app.OwnerPlaceDefaultCommand(f.ctx, f.ownerID, f.fluctlightID, pid, restore)
	if err != nil {
		t.Fatal(err)
	}
	if restored["life_write"] != false || intValue(restored["revision"]) != 3 {
		t.Fatal(restored)
	}
	// A fresh service must read the persisted fields, not a frontend projection.
	fresh := &App{DB: f.repository}
	view, err := fresh.OwnerPlaceDefaults(f.ctx, f.ownerID, f.fluctlightID, pid, query)
	if err != nil || len(arrayValue(view["diff"])) != 0 || jsonString(view["current"]) != jsonString(first["current"]) {
		t.Fatal("restoration did not survive service refresh", view, err)
	}
	var parentID string
	var connections []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT parent_place_id,details->'connection_place_ids' FROM public.world_places WHERE id=$1`, pid).Scan(&parentID, &connections); err != nil || parentID != parent["id"] || string(connections) != jsonString([]any{parent["id"]}) {
		t.Fatal("restore changed topology", parentID, string(connections), err)
	}
	replay, err := fresh.OwnerPlaceDefaultCommand(f.ctx, f.ownerID, f.fluctlightID, pid, restore)
	if err != nil || replay["_replay"] != true || intValue(replay["revision"]) != 3 {
		t.Fatal("restore replay changed version", replay, err)
	}
	stale := cloneMap(restore)
	stale["idempotency_key"] = "stale-place"
	_, err = fresh.OwnerPlaceDefaultCommand(f.ctx, f.ownerID, f.fluctlightID, pid, stale)
	requireV3SurfaceError(t, err, "revision_conflict")
	stale["expected_revision"] = 3
	stale["expected_default_revision"] = 0
	stale["idempotency_key"] = "stale-baseline"
	_, err = fresh.OwnerPlaceDefaultCommand(f.ctx, f.ownerID, f.fluctlightID, pid, stale)
	requireV3SurfaceError(t, err, "revision_conflict")
	history, err := fresh.OwnerPlaceDefaultHistory(f.ctx, f.ownerID, f.fluctlightID, pid, query)
	if err != nil || len(arrayValue(history["items"])) != 3 {
		t.Fatal("history duplicated, missing or rolled forward after conflict", history, err)
	}
	if _, err = fresh.OwnerPlaceDefaults(f.ctx, f.foreignOwnerID, f.fluctlightID, pid, query); err == nil {
		t.Fatal("foreign Owner read place defaults")
	}
	if _, err = fresh.OwnerPlaceDefaults(f.ctx, f.ownerID, f.fluctlightID, pid, url.Values{"worldId": {"wrong-world"}}); !errors.Is(err, ErrNotFound) {
		t.Fatal("place escaped world scope", err)
	}
	// Simulate a pre-feature custom place with no provable historical baseline.
	if _, err = f.repository.Pool().Exec(f.ctx, `DELETE FROM public.world_place_defaults WHERE place_id=$1`, pid); err != nil {
		t.Fatal(err)
	}
	if len(mapValue(read()["baseline"])) != 0 {
		t.Fatal("invented baseline for old custom place")
	}
	capture := map[string]any{"world_id": wid, "expected_revision": 3, "expected_default_revision": 0, "operation": "restore", "source": "recorded", "reason": "Owner reviews old place", "idempotency_key": "missing-default"}
	_, err = fresh.OwnerPlaceDefaultCommand(f.ctx, f.ownerID, f.fluctlightID, pid, capture)
	requireV3SurfaceError(t, err, "place_default_not_configured")
	capture["operation"], capture["source"], capture["idempotency_key"] = "capture", "current_saved", "capture-current"
	result, err := fresh.OwnerPlaceDefaultCommand(f.ctx, f.ownerID, f.fluctlightID, pid, capture)
	if err != nil || intValue(result["default_revision"]) != 1 || read()["place_revision"] != view["place_revision"] {
		t.Fatal("explicit baseline capture failed or changed place", result, err)
	}
	f.requireNoModelRuns(t)
}

func TestV3InventoryArchiveUseInvalidationSQL(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var item string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND category='boots'`, f.fluctlightID).Scan(&item); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_item_uses(fluctlight_id,item_id,activity,started_at,source_event_id) VALUES($1,$2,'testing',now(),'fixture-use')`, f.fluctlightID, item); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return clearUnavailableItemUseTx(f.ctx, tx, f.fluctlightID, item, "owner-archive", f.app.now())
	}); err != nil {
		t.Fatal("archive use invalidation SQL", err)
	}
	var active, history int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.fluctlight_item_uses WHERE item_id=$1),(SELECT count(*) FROM public.fluctlight_item_use_events WHERE item_id=$1 AND operation='stop')`, item).Scan(&active, &history); err != nil || active != 0 || history != 1 {
		t.Fatal("archive lost use history", active, history, err)
	}
}
