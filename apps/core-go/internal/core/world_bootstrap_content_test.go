package core

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestDefaultWorldResidentsHavePersonalNamesAndMatchingPublication(t *testing.T) {
	f := newIndependentToolE2EFixture(t, "named-world-residents")
	result, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/world/catalog", nil, f.ownerID, map[string]any{"name": "居民名字回归", "reason": "explicit create", "idempotency_key": "named-world"})
	if err != nil {
		t.Fatal(err)
	}
	world := stringValue(result["id"])
	rows, err := f.repository.Pool().Query(f.ctx, `SELECT n.name,v.name,n.position_mode,n.role,n.background,v.body->>'background' FROM public.world_npcs n JOIN public.studio_resource_versions v ON v.resource_id=n.source_resource_id AND v.version=n.source_version WHERE n.world_id=$1 ORDER BY n.name`, world)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names := map[string]bool{}
	expectedNames := map[string]bool{"周予安": true, "林晚晴": true}
	for rows.Next() {
		var name, publishedName, positionMode, role, background, publishedBackground string
		if err := rows.Scan(&name, &publishedName, &positionMode, &role, &background, &publishedBackground); err != nil {
			t.Fatal(err)
		}
		if !expectedNames[name] || name != publishedName || names[name] {
			t.Fatalf("resident lacks an individual name matching its immutable publication: name=%q published=%q", name, publishedName)
		}
		if positionMode != "unknown" || role != "居民" || background != publishedBackground {
			t.Fatalf("name seed changed resident authority: mode=%q role=%q background=%q published=%q", positionMode, role, background, publishedBackground)
		}
		names[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("expected two distinct named residents, got %v", names)
	}
}

func TestDefaultWorldPlacesHaveDistinctSceneDescriptions(t *testing.T) {
	f := newIndependentToolE2EFixture(t, "world-place-descriptions")
	result, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/world/catalog", nil, f.ownerID, map[string]any{"name": "地点描述回归", "reason": "explicit create", "idempotency_key": "described-world"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.app.worldSnapshot(f.ctx, f.ownerID, f.fluctlightID, stringValue(result["id"]))
	if err != nil {
		t.Fatal(err)
	}
	places := arrayValue(snapshot["places"])
	if len(places) != 8 {
		t.Fatalf("expected all eight default places, got %d", len(places))
	}
	descriptions := map[string]bool{}
	for _, raw := range places {
		place := mapValue(raw)
		description := strings.TrimSpace(stringValue(place["desc"]))
		if len([]rune(description)) < 20 || descriptions[description] || strings.Contains(description, "尚未提供") {
			t.Fatalf("place %q has an empty, generic or duplicate scene description: %q", place["name"], description)
		}
		descriptions[description] = true
	}
}

func TestWorldBootstrapPreservesCustomFrameworkAndExistingResidents(t *testing.T) {
	f := newIndependentToolE2EFixture(t, "world-custom-base")
	frame := map[string]any{"schema": "world-framework.v1", "venues": []any{
		map[string]any{"key": "custom", "name": "自定场地", "places": []any{
			map[string]any{"key": "quiet", "name": "静室", "description": "用户写下的房间描述。"},
		}},
	}}
	body := map[string]any{"name": "自定义世界", "framework": frame, "reason": "explicit create", "idempotency_key": "custom-world"}
	result, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/world/catalog", nil, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	world := stringValue(result["id"])
	// Existing configuration must remain untouched even if bootstrap is replayed
	// without the original framework. This is not an existing-world repair path.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.world_npcs SET name='用户保留的名字' WHERE world_id=$1 AND name='周予安'`, world); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return f.app.bootstrapWorldTx(f.ctx, tx, f.fluctlightID, world, nil)
	}); err != nil {
		t.Fatal(err)
	}
	var places, npcs, customDescription, preservedName int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.world_places WHERE world_id=$1),(SELECT count(*) FROM public.world_npcs WHERE world_id=$1),(SELECT count(*) FROM public.world_places WHERE world_id=$1 AND name='静室' AND description='用户写下的房间描述。'),(SELECT count(*) FROM public.world_npcs WHERE world_id=$1 AND name='用户保留的名字')`, world).Scan(&places, &npcs, &customDescription, &preservedName); err != nil {
		t.Fatal(err)
	}
	if places != 1 || npcs != 2 || customDescription != 1 || preservedName != 1 {
		t.Fatalf("bootstrap overwrote or supplemented saved configuration: places=%d npcs=%d customDescription=%d preservedName=%d", places, npcs, customDescription, preservedName)
	}
}
