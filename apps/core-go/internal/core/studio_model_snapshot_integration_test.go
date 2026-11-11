package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// The Provider is controlled; PostgreSQL and the production Studio attempt
// transition are real only when the isolated test database is configured.
func TestStudioPostgresAutomaticRetryKeepsFrozenModel(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "studio-snapshot-auto")
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "Snapshot fixture", "body": map[string]any{"description": "original"}, "reason": "test", "idempotency_key": "snapshot-resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "assistant", "name": "Snapshot fixture", "body": map[string]any{"description": "original"}, "instruction": "extend", "reason": "test", "idempotency_key": "snapshot-job"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	seedCognitiveProviderRole(t, ctx, repo, "studio-snapshot-endpoint")
	models := []string{}
	fail := true
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		if fail {
			return fakeProviderResult{Err: context.DeadlineExceeded}
		}
		return fakeProviderResult{Structured: map[string]any{"disposition": "proposal", "reply": "受控建议", "name": "Snapshot fixture", "body": map[string]any{"description": "updated"}, "warnings": []any{}}}
	})}
	first := "studio_intent:" + id + "_attempt_1"
	if _, err := app.ProcessStudioGeneration(ctx, first); err == nil {
		t.Fatal("expected controlled timeout")
	}
	outcome, err := app.ReconcileStudioWorkflowFailure(ctx, first, "provider timeout")
	if err != nil || outcome.Status != "retry" {
		t.Fatalf("automatic retry=%#v %v", outcome, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='changed-model-b' WHERE role='cognitive_assessment'`); err != nil {
		t.Fatal(err)
	}
	fail = false
	if _, err := app.ProcessStudioGeneration(ctx, outcome.NextIntentID); err != nil {
		t.Fatal(err)
	}
	if len(models) < 2 {
		t.Fatalf("missing physical requests: %v", models)
	}
	for _, model := range models {
		if model != "turn-model" {
			t.Fatalf("automatic retry rebound model: %v", models)
		}
	}
	current, err := app.StudioJob(ctx, owner, id)
	if err != nil || current["status"] != "completed" {
		t.Fatalf("studio result=%#v %v", current, err)
	}
	var traced bool
	if err := repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.diagnostic_model_runs WHERE scenario='studio_assistant' AND metrics->'prompt_slots' @> '[{"slot_id":"studio_definition","source":"studio_job_snapshot"},{"slot_id":"studio_instruction","source":"studio_job_snapshot"}]'::jsonb)`).Scan(&traced); err != nil || !traced {
		t.Fatalf("Studio physical source slots missing: %v", err)
	}
	for _, raw := range arrayValue(current["attempts"]) {
		public := jsonString(mapValue(raw)["provenance"])
		if strings.Contains(public, studioModelSnapshotsKey) || strings.Contains(public, "turn-secret") || strings.Contains(public, "turn.invalid") {
			t.Fatalf("private snapshot exposed: %s", public)
		}
	}
}

func TestStudioPostgresLegacyRunningWithoutSnapshotFailsExplicitly(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "studio-legacy-snapshot")
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "Legacy fixture", "body": map[string]any{"description": "original"}, "reason": "test", "idempotency_key": "legacy-resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "assistant", "name": "Legacy fixture", "body": map[string]any{"description": "original"}, "instruction": "extend", "reason": "test", "idempotency_key": "legacy-job"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_job_attempts SET status='running' WHERE job_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_jobs SET status='running' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	result, err := app.ProcessStudioGeneration(ctx, "studio_intent:"+id+"_attempt_1")
	if result != nil || !errors.Is(err, errStudioModelSnapshotMissing) {
		t.Fatalf("legacy Studio task silently rebound: %#v %v", result, err)
	}
	var status, code string
	if err := repo.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.studio_jobs WHERE id=$1`, id).Scan(&status, &code); err != nil || status != "failed" || code != "studio_model_snapshot_missing" {
		t.Fatalf("status=%q code=%q err=%v", status, code, err)
	}
}

func TestStudioPostgresExplicitRetryCanSelectNewModel(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "studio-snapshot-manual")
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "Manual fixture", "body": map[string]any{"description": "original"}, "reason": "test", "idempotency_key": "manual-resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "assistant", "name": "Manual fixture", "body": map[string]any{"description": "original"}, "instruction": "extend", "reason": "test", "idempotency_key": "manual-job"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	seedCognitiveProviderRole(t, ctx, repo, "studio-manual-endpoint")
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult { return fakeProviderResult{Err: context.DeadlineExceeded} })}
	if _, err := app.ProcessStudioGeneration(ctx, "studio_intent:"+id+"_attempt_1"); err == nil {
		t.Fatal("expected timeout")
	}
	current, err := app.StudioJob(ctx, owner, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='manual-model-b' WHERE role='cognitive_assessment'`); err != nil {
		t.Fatal(err)
	}
	retried, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+id+"/retry", nil, owner, map[string]any{"expected_revision": current["revision"], "reason": "updated model", "idempotency_key": "manual-retry"})
	if err != nil || intValue(retried["latest_attempt"]) != 2 {
		t.Fatalf("manual retry=%#v %v", retried, err)
	}
	var model string
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		model = stringValue(payload["model"])
		return fakeProviderResult{Structured: map[string]any{"disposition": "proposal", "reply": "受控建议", "name": "Manual fixture", "body": map[string]any{"description": "updated"}, "warnings": []any{}}}
	})}
	if _, err := app.ProcessStudioGeneration(ctx, "studio_intent:"+id+"_attempt_2"); err != nil || model != "manual-model-b" {
		t.Fatalf("manual retry model=%q err=%v", model, err)
	}
}

func TestStudioPostgresMediaPlanningAutomaticRetryKeepsFrozenModel(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "studio-media-snapshot-auto")
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "Media fixture", "body": map[string]any{"description": "empty room"}, "reason": "test", "idempotency_key": "media-snapshot-resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "image_preview", "name": "Media fixture", "body": map[string]any{"description": "empty room"}, "instruction": "an empty room", "reason": "test", "idempotency_key": "media-snapshot-job"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	seedCognitiveProviderRole(t, ctx, repo, "studio-media-snapshot-endpoint")
	models := []string{}
	fail := true
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		if fail {
			return fakeProviderResult{Err: context.DeadlineExceeded}
		}
		return fakeProviderResult{Structured: mediaTestPlan("landscape", "none", "no", "unknown")}
	})}
	first := "studio_intent:" + id + "_attempt_1"
	if _, err := app.ProcessStudioGeneration(ctx, first); err == nil {
		t.Fatal("expected controlled media-planning timeout")
	}
	outcome, err := app.ReconcileStudioWorkflowFailure(ctx, first, "provider timeout")
	if err != nil || outcome.Status != "retry" {
		t.Fatalf("automatic retry=%#v %v", outcome, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='changed-media-model-b' WHERE role='cognitive_assessment'`); err != nil {
		t.Fatal(err)
	}
	fail = false
	result, err := app.ProcessStudioGeneration(ctx, outcome.NextIntentID)
	if err != nil || stringValue(result["media_intent_id"]) == "" {
		t.Fatalf("media task was not queued: %#v %v", result, err)
	}
	if len(models) < 2 {
		t.Fatalf("missing model requests: %v", models)
	}
	for _, model := range models {
		if model != "turn-model" {
			t.Fatalf("media planning rebound model: %v", models)
		}
	}
}
