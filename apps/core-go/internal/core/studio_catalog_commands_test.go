package core

import (
	"errors"
	"net/url"
	"testing"
)

func TestStudioCopyArchiveRestoreRetainsPublicationAndProtectsRuntimeReferences(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "world", "name": "Source world", "body": map[string]any{"background": "Original definition", "rules": "No permanent harm"}, "reason": "Test input", "idempotency_key": randomID("source_")})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(r["id"])
	pub, err := f.app.publishWorldResource(f.ctx, f.ownerID, id, map[string]any{"expected_revision": r["revision"], "reason": "Publish test source", "idempotency_key": randomID("publish_")})
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"expected_revision": pub["revision"], "source": "published", "source_version": pub["published_version"], "name": "Independent copy", "reason": "Copy immutable test source", "idempotency_key": randomID("copy_")}
	copied, err := f.app.studioCatalogCommand(f.ctx, f.ownerID, id, "copy", body)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.app.studioCatalogCommand(f.ctx, f.ownerID, id, "copy", body)
	if err != nil || replay["id"] != copied["id"] || replay["_replay"] != true {
		t.Fatal("copy replay created another resource", replay, err)
	}
	if copied["published_version"] != nil || len(arrayValue(copied["referenced_by"])) != 0 || copied["id"] == id || mapValue(copied["body"])["background"] != "Original definition" {
		t.Fatal("copy inherited publication or runtime ownership", copied)
	}
	archived, err := f.app.studioCatalogCommand(f.ctx, f.ownerID, id, "archive", map[string]any{"expected_revision": pub["revision"], "reason": "Archive unused source", "idempotency_key": randomID("archive_")})
	if err != nil || archived["status"] != "archived" {
		t.Fatal(archived, err)
	}
	versions, err := f.app.OwnerStudioVersions(f.ctx, f.ownerID, id, nil)
	if err != nil || len(arrayValue(versions["items"])) != 1 {
		t.Fatal("archive erased history", versions, err)
	}
	if _, err := f.app.OwnerStudioPublication(f.ctx, f.ownerID, id, stringValue(pub["published_version"])); err != nil {
		t.Fatal("archive broke exact history", err)
	}
	list, err := f.app.OwnerStudioResources(f.ctx, f.ownerID, url.Values{"archived": {"true"}})
	if err != nil || len(arrayValue(list["items"])) != 1 || mapValue(arrayValue(list["items"])[0])["id"] != id {
		t.Fatal("archived catalogue failed", list, err)
	}
	_, err = f.app.publishWorldResource(f.ctx, f.ownerID, id, map[string]any{"expected_revision": archived["revision"], "reason": "Do not publish archive", "idempotency_key": randomID("archive_publish_")})
	var blocked *SurfaceError
	if !errors.As(err, &blocked) || blocked.Code != "studio_resource_archived" {
		t.Fatal("archived resource published", err)
	}
	restored, err := f.app.studioCatalogCommand(f.ctx, f.ownerID, id, "restore", map[string]any{"expected_revision": archived["revision"], "reason": "Restore definition", "idempotency_key": randomID("restore_")})
	if err != nil || restored["published_version"] != pub["published_version"] {
		t.Fatal(restored, err)
	}
	fl, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.DispatchWorldApply(f.ctx, f.ownerID, id, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 0, "choices": []any{}, "reason": "Apply exact test world", "idempotency_key": randomID("apply_")}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.studioCatalogCommand(f.ctx, f.ownerID, id, "archive", map[string]any{"expected_revision": restored["revision"], "reason": "Do not archive referenced source", "idempotency_key": randomID("reference_guard_")})
	if !errors.As(err, &blocked) || blocked.Code != "resource_referenced" {
		t.Fatal("runtime source was archived", err)
	}
	if _, err := f.app.OwnerStudioVersions(f.ctx, f.foreignOwnerID, id, nil); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign publication history readable", err)
	}
}
