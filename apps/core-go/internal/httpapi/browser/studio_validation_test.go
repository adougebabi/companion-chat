package browser

import "testing"

func TestWorldItemConfigurationHasClosedScopeAndExplicitPlace(t *testing.T) {
	body := map[string]any{"expectedRevision": float64(1), "available": true, "placeId": nil, "reason": "Configure catalogue", "idempotencyKey": "key"}
	if !validWorldItemConfig(body) {
		t.Fatal("explicit unrestricted configuration rejected")
	}
	for _, key := range []string{"placeId", "available", "expectedRevision"} {
		value := body[key]
		delete(body, key)
		if validWorldItemConfig(body) {
			t.Fatal("implicit config admitted", key)
		}
		body[key] = value
	}
	body["ownership"] = "owned"
	if validWorldItemConfig(body) {
		t.Fatal("catalogue config grants inventory")
	}
}

func TestStudioDefinitionPreparationRequiresExplicitPurposeAndExactVersion(t *testing.T) {
	b := map[string]any{"resourceId": "resource", "resourceRevision": float64(1), "kind": "assistant", "name": "Template", "body": map[string]any{"schema": "character-template.v1", "description": "Exact source"}, "instruction": "Prepare", "reason": "Explicit parse", "idempotencyKey": "key", "purpose": "foundation_prepare", "resourceVersion": "1"}
	if !validStudioJobBody("/api/studio/jobs", b) {
		t.Fatal("explicit preparation rejected")
	}
	for _, key := range []string{"purpose", "resourceVersion"} {
		saved := b[key]
		delete(b, key)
		if validStudioJobBody("/api/studio/jobs", b) {
			t.Fatal("partial preparation accepted", key)
		}
		b[key] = saved
	}
	b["kind"] = "image_preview"
	if validStudioJobBody("/api/studio/jobs", b) {
		t.Fatal("image can trigger definition parser")
	}
	b["kind"] = "assistant"
	mapped := surfaceCommandBody("/api/studio/jobs", b)
	if mapped["resource_version"] != "1" || mapped["purpose"] != "foundation_prepare" {
		t.Fatal(mapped)
	}
}

func TestStudioDefinitionTransportIsOpaqueForDraftsAndJobs(t *testing.T) {
	for _, path := range []string{"/api/studio/jobs", "/api/studio/resources", "/api/studio/resources/resource-id"} {
		source := map[string]any{"resourceRevision": float64(1), "body": map[string]any{"customField": "kept", "nestedObject": map[string]any{"aKey": "kept"}}, "referenceAsset": map[string]any{"assetId": "asset-id", "purpose": "scene"}}
		mapped := surfaceCommandBody(path, source)
		if mapped["resource_revision"] != float64(1) || mapped["reference_asset"].(map[string]any)["asset_id"] != "asset-id" {
			t.Fatal("transport envelope not normalized", mapped)
		}
		body := mapped["body"].(map[string]any)
		if body["customField"] != "kept" || body["nestedObject"].(map[string]any)["aKey"] != "kept" {
			t.Fatal("definition schema was rewritten", body)
		}
	}
}

func TestStudioBoundaryRequiresActualResourceSnapshotAndClosedCommand(t *testing.T) {
	b := map[string]any{"resourceId": "res-real", "resourceRevision": float64(1), "kind": "assistant", "name": "Draft", "body": map[string]any{"description": "Unchanged JSON"}, "instruction": "Suggest details", "reason": "Owner request", "idempotencyKey": "job-admit"}
	if !validStudioJobBody("/api/studio/jobs", b) {
		t.Fatal("valid frozen draft rejected")
	}
	b["fluctlightId"] = "forged-life-scope"
	if validStudioJobBody("/api/studio/jobs", b) {
		t.Fatal("life scope accepted")
	}
	delete(b, "fluctlightId")
	b["body"] = "not an object"
	if validStudioJobBody("/api/studio/jobs", b) {
		t.Fatal("untyped definition accepted")
	}
	if validStudioJobBody("/api/studio/jobs/id/retry", map[string]any{"expectedRevision": float64(1), "reason": "retry", "idempotencyKey": "retry", "body": map[string]any{}}) {
		t.Fatal("retry altered frozen input")
	}
}

func TestStudioImagePreviewReferenceIsTypedAndFrozen(t *testing.T) {
	b := map[string]any{"resourceId": "resource", "resourceRevision": float64(1), "kind": "image_preview", "name": "Scene", "body": map[string]any{"customField": "keep raw definition"}, "instruction": "Render this scene", "reason": "Owner preview", "idempotencyKey": "image-original"}
	if !validStudioJobBody("/api/studio/jobs", b) {
		t.Fatal("reference-free image rejected")
	}
	b["referenceAsset"] = map[string]any{"assetId": "asset-retained", "purpose": "scene"}
	if !validStudioJobBody("/api/studio/jobs", b) {
		t.Fatal("typed image reference rejected")
	}
	mapped := surfaceCommandBody("/api/studio/jobs", b)
	ref := mapped["reference_asset"].(map[string]any)
	if ref["asset_id"] != "asset-retained" || ref["purpose"] != "scene" || mapped["body"].(map[string]any)["customField"] != "keep raw definition" {
		t.Fatalf("mapping changed frozen source: %#v", mapped)
	}
	for _, kind := range []string{"assistant", "text_preview"} {
		b["kind"] = kind
		if validStudioJobBody("/api/studio/jobs", b) {
			t.Fatal("image reference leaked into text job")
		}
	}
	b["kind"] = "image_preview"
	for _, invalid := range []any{nil, "asset", map[string]any{"assetId": "asset", "purpose": "portrait"}, map[string]any{"assetId": "asset"}, map[string]any{"assetId": "asset", "purpose": "object", "ownerActorId": "forged"}, map[string]any{"assetId": "", "purpose": "animal"}} {
		b["referenceAsset"] = invalid
		if validStudioJobBody("/api/studio/jobs", b) {
			t.Fatalf("invalid reference accepted: %#v", invalid)
		}
	}
}

func TestStudioCatalogueCommandsHaveClosedCASBodies(t *testing.T) {
	b := map[string]any{"expectedRevision": float64(1), "reason": "Owner copy", "idempotencyKey": "original", "source": "published", "sourceVersion": "1", "name": "Independent"}
	if !validStudioCatalogCommand("/api/studio/resources/id/copy", b) {
		t.Fatal("valid copy rejected")
	}
	if validStudioCatalogCommand("/api/studio/resources/id/restore", b) {
		t.Fatal("restore can change definition")
	}
	b["fluctlightId"] = "other"
	if validStudioCatalogCommand("/api/studio/resources/id/copy", b) {
		t.Fatal("copy accepted life target")
	}
}
