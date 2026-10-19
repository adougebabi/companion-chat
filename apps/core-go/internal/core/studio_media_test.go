package core

import (
	"net/url"
	"testing"
)

// Ready rows here represent an explicit crash-recovery fixture. This test does
// not claim to have called a model, renderer or object store or produced pixels.
func TestStudioMediaReadyRecoveryAuthorizationAndExplicitMaterialRetention(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "studio-media-recovery")
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "Studio media place", "body": map[string]any{"description": "fictional room"}, "reason": "fixture", "idempotency_key": "studio-media-resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "image_preview", "name": "Studio media place", "body": map[string]any{"description": "fictional room"}, "instruction": "a fictional empty room", "reason": "fixture", "idempotency_key": "studio-image-job"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	mid := "studio_media_" + stableDigest(id)
	asset := "asset_" + mid
	workflow := "studio-media-test-workflow"
	concept := map[string]any{"source": "studio_definition", "studio_owner_actor_id": owner, "studio_source": map[string]any{"resource_id": resource["id"], "source_digest": job["source_digest"]}, "intent": "empty fictional room", "media_plan_version": mediaPlanVersion, "media_capture_plan": mediaTestPlan("landscape", "none", "no", "unknown"), "context_binding": map[string]any{}, "explicit_constraints": map[string]any{"self_in_frame": "no"}, "normalized_output": map[string]any{"prompt": "空房间", "declared_human_presence": "none", "self_in_frame": "no", "capture_method": "unspecified", "used_subject_ids": []any{"scene"}, "used_reference_ids": []any{}, "declared_human_count": 0}}
	output := map[string]any{"media_intent_id": mid, "workflow_id": workflow}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_jobs SET status='running' WHERE id=$1;`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_job_attempts SET status='running',output=$2 WHERE job_id=$1`, id, jsonBytes(output)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,media_scope,studio_owner_actor_id,studio_job_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES($1,NULL,'studio',$2,$3,'image','image/png',$4,'fixture-request',$5,'running')`, mid, owner, id, jsonString(concept), workflow); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,media_scope,studio_owner_actor_id,studio_job_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES($1,NULL,'studio',$2,$3,'v1','image','image/png',1,'fixture-digest','private-fixture','not-read-in-this-test','fixture-request',$4,'ready')`, asset, owner, id, workflow); err != nil {
		t.Fatal(err)
	}
	if err := app.AuthorizeAsset(ctx, owner, asset); err == nil {
		t.Fatal("unfinished preview acquired a read grant")
	}
	// Historical/current workflow rows are explicit recovery setup. A late
	// source callback must not take the current workflow's identity from mid.
	for _, entry := range []struct{ id, workflow, status string }{{"studio-old-source", "old-workflow", "failed"}, {"studio-current-source", workflow, "started"}} {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES($1,$2,'media','media.generation',$3,$4)`, entry.id, entry.workflow, jsonBytes(map[string]any{"intent_id": mid}), entry.status); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.RecordMediaActivityFailure(ctx, mid, "go:old-workflow", "old delayed failure"); err != nil {
		t.Fatal(err)
	}
	if err := app.ReconcileStudioMediaTerminal(ctx, "studio-old-source", "old delayed terminal"); err != nil {
		t.Fatal(err)
	}
	stillRunning, err := app.StudioJob(ctx, owner, id)
	if err != nil || stillRunning["status"] != "running" {
		t.Fatal("old workflow affected current attempt", stillRunning, err)
	}
	// Ready is authoritative even when the Workflow failed between the asset
	// commit and product settlement. No renderer or new Provider call is needed.
	if err := app.ReconcileStudioMediaTerminal(ctx, "studio-current-source", "workflow_failed"); err != nil {
		t.Fatal(err)
	}
	result, err := app.ProcessMediaIntent(ctx, mid)
	if err != nil || result["status"] != "completed" {
		t.Fatal(result, err)
	}
	actual, err := app.StudioJob(ctx, owner, id)
	if err != nil || actual["status"] != "completed" || mapValue(actual["result"])["asset_id"] != asset {
		t.Fatal(actual, err)
	}
	if err := app.AuthorizeAsset(ctx, owner, asset); err != nil {
		t.Fatal(err)
	}
	if err := app.AuthorizeAsset(ctx, owner+"-foreign", asset); err == nil {
		t.Fatal("foreign Owner granted Studio asset")
	}
	media, _, err := app.ListBrowserMedia(ctx, owner, fid, "", 100)
	if err != nil || len(media) != 0 {
		t.Fatal("preview entered formal character library", media, err)
	}
	saved, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+id+"/save-material", nil, owner, map[string]any{"expected_revision": actual["revision"], "reason": "explicit retain", "idempotency_key": "studio-retain-image"})
	if err != nil || saved["asset_id"] != asset {
		t.Fatal(saved, err)
	}
	materials, err := app.DispatchStudioJobSurface(ctx, "GET", "/internal/studio/materials", url.Values{"resource_id": {stringValue(resource["id"])}}, owner, nil)
	if err != nil || len(arrayValue(materials["items"])) != 1 {
		t.Fatal(materials, err)
	}
	if _, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+id+"/discard", nil, owner, map[string]any{"expected_revision": actual["revision"], "reason": "discard preview keep retained material", "idempotency_key": "studio-discard-image"}); err != nil {
		t.Fatal(err)
	}
	if err := app.AuthorizeAsset(ctx, owner, asset); err != nil {
		t.Fatal("explicit retained material lost authorization", err)
	}
}

func TestStudioMediaConceptNeverBorrowedTaskActorOrCurrentLife(t *testing.T) {
	concept := map[string]any{"source": "studio_definition", "studio_source": map[string]any{"resource_id": "resource", "source_digest": "digest"}, "media_capture_plan": mediaTestPlan("landscape", "none", "no", "unknown"), "context_binding": map[string]any{}}
	if err := validateStudioMediaConcept(concept); err != nil {
		t.Fatal(err)
	}
	concept["context_binding"] = map[string]any{"appearance": map[string]any{"body": "character private appearance"}}
	if validateStudioMediaConcept(concept) == nil {
		t.Fatal("formal appearance leaked into preview")
	}
}
