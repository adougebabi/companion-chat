package core

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestR10WorldConfigurationCopyIsIndependentAtomicAndReplayable(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	target, foreign := "r10-target", "r10-foreign"
	for _, row := range []struct{ id, owner string }{{target, f.ownerID}, {foreign, f.foreignOwnerID}} {
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'fluctlight','active')`, row.id); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) SELECT $1,$2,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance FROM public.fluctlights WHERE id=$3`, row.id, row.owner, f.fluctlightID); err != nil {
			t.Fatal(err)
		}
	}
	call := func(fid, method, path string, body map[string]any) (map[string]any, error) {
		if body != nil {
			if body["idempotency_key"] == nil {
				body["idempotency_key"] = randomID("r10_")
			}
			body["reason"] = "isolated r10 configuration copy"
		}
		return f.app.DispatchWorldSurface(f.ctx, method, "/internal/fluctlights/"+fid+path, nil, f.ownerID, body)
	}
	must := func(method, path string, body map[string]any) map[string]any {
		t.Helper()
		v, err := call(f.fluctlightID, method, path, body)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	world := must("PUT", "/world", map[string]any{"name": "杭州生活", "background": "保留的世界背景", "expected_revision": 0, "venue_categories": []any{map[string]any{"id": "bookshop", "name": "书店"}}})
	parent := must("POST", "/world/places", map[string]any{"name": "书店", "description": "一楼", "floor": "1F", "venue_category_id": "bookshop"})
	child := must("POST", "/world/places", map[string]any{"name": "阅读区", "parent_place_id": parent["id"], "floor": "2F", "connection_place_ids": []any{parent["id"]}, "furniture": "长桌", "room_items": "已有书架的描述"})
	before := map[string]int{}
	for _, table := range []string{"conversation_messages", "life_events", "memories", "world_npcs", "world_event_runs"} {
		var n int
		if err := f.repository.Pool().QueryRow(f.ctx, "SELECT count(*) FROM public."+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		before[table] = n
	}
	body := map[string]any{"name": "周屿的杭州生活", "source_world_id": world["id"], "source_fluctlight_id": f.fluctlightID, "expected_source_revision": 1, "idempotency_key": "r10-copy"}
	copy, err := call(target, "POST", "/world/catalog", body)
	if err != nil {
		t.Fatal(err)
	}
	if copy["id"] == world["id"] || copy["configuration_copied"] != true || intValue(copy["copied_places"]) != 2 {
		t.Fatal(copy)
	}
	again, err := call(target, "POST", "/world/catalog", body)
	if err != nil || again["id"] != copy["id"] {
		t.Fatal("duplicate copy", again, err)
	}
	var root, leaf string
	var details []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.world_places WHERE world_id=$1 AND name='书店'`, copy["id"]).Scan(&root); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id,details FROM public.world_places WHERE world_id=$1 AND name='阅读区' AND parent_place_id=$2`, copy["id"], root).Scan(&leaf, &details); err != nil {
		t.Fatal(err)
	}
	d := decodeObject(details)
	if root == parent["id"] || leaf == child["id"] || d["floor"] != "2F" || d["furniture"] != "长桌" || stringValue(arrayValue(d["connection_place_ids"])[0]) != root {
		t.Fatal(root, leaf, d)
	}
	var bound int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1`, target).Scan(&bound); err != nil || bound != 0 {
		t.Fatal("copy silently selected world", bound, err)
	}
	for table, n := range before {
		var after int
		if err := f.repository.Pool().QueryRow(f.ctx, "SELECT count(*) FROM public."+table).Scan(&after); err != nil || after != n {
			t.Fatal("copied life history or actors", table, n, after, err)
		}
	}
	invalid := cloneMap(body)
	invalid["idempotency_key"] = "r10-foreign"
	invalid["source_fluctlight_id"] = foreign
	if _, err := call(target, "POST", "/world/catalog", invalid); err == nil {
		t.Fatal("foreign source admitted")
	}
	stale := cloneMap(body)
	stale["idempotency_key"] = "r10-stale"
	stale["expected_source_revision"] = 99
	if _, err := call(target, "POST", "/world/catalog", stale); err == nil {
		t.Fatal("stale source admitted")
	}
	var worlds int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.life_worlds WHERE fluctlight_id=$1`, target).Scan(&worlds); err != nil || worlds != 1 {
		t.Fatal("failed copies left partial worlds", worlds, err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := tx.Exec(f.ctx, `UPDATE public.world_places SET description='独立的新描述' WHERE id=$1`, leaf)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var original string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT description FROM public.world_places WHERE id=$1`, child["id"]).Scan(&original); err != nil || original == "独立的新描述" {
		t.Fatal("shared mutable place", original, err)
	}
}
