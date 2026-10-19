package core

import (
	"net/http"
	"net/url"
	"testing"
)

func TestStudioAssistantCanCompleteIncompleteEventDraftWithoutInventingMissingReferences(t *testing.T) {
	partial := map[string]any{"schema_version": "world-event.v1", "description": "", "effects": []any{}}
	if err := validateStudioDraft("event_template", "New event", partial, true); err != nil {
		t.Fatal("assistant cannot accept incomplete input", err)
	}
	if err := validateStudioDraft("event_template", "New event", partial, false); err == nil {
		t.Fatal("preview admitted incomplete event")
	}
	snapshot := map[string]any{"resource_kind": "event_template", "name": "New event", "body": partial}
	needs := map[string]any{"disposition": "needs_input", "reply": "请选择已有NPC和到访地点", "name": "New event", "body": partial, "warnings": []any{"缺少稳定引用"}}
	if err := validateStudioOutput("assistant", snapshot, needs); err != nil {
		t.Fatal(err)
	}
	needs["body"] = worldEventTestDefinition(map[string]any{"kind": "weather", "scope": "world", "value": "unrequested alternative"})
	if err := validateStudioOutput("assistant", snapshot, needs); err == nil {
		t.Fatal("missing input silently replaced user intent")
	}
}
func TestStudioProposalRequiresVerifiedFrozenReferences(t *testing.T) {
	body := worldEventTestDefinition(map[string]any{"kind": "npc_visit", "scope": "place", "place_id": "place-known", "npc_id": "npc-invented", "value": "visit"})
	snapshot := map[string]any{"resource_kind": "event_template", "allowed_references": []any{map[string]any{"kind": "place", "id": "place-known", "world_id": "world-A"}}}
	out := map[string]any{"disposition": "proposal", "reply": "suggestion", "name": "Visit", "body": body, "warnings": []any{}}
	if validateStudioOutput("assistant", snapshot, out) == nil {
		t.Fatal("invented NPC accepted as a complete proposal")
	}
	snapshot["allowed_references"] = append(arrayValue(snapshot["allowed_references"]), map[string]any{"kind": "npc", "id": "npc-invented", "world_id": "world-A"})
	if err := validateStudioOutput("assistant", snapshot, out); err != nil {
		t.Fatal("explicit verified references rejected", err)
	}
	mapValue(arrayValue(snapshot["allowed_references"])[1])["world_id"] = "world-B"
	if validateStudioOutput("assistant", snapshot, out) == nil {
		t.Fatal("cross-world references accepted as one event")
	}
}

func TestStudioLateModelOutputCannotReportCompletionAfterTerminalFailure(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "studio-terminal-race")
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "Studio race", "body": map[string]any{"description": "a definition"}, "reason": "test", "idempotency_key": "studio-race-resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "assistant", "name": "Studio race", "body": map[string]any{"description": "a definition"}, "instruction": "extend the description", "reason": "test", "idempotency_key": "studio-race-job"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	intent := "studio_intent:" + id + "_attempt_1"
	seedCognitiveProviderRole(t, ctx, repo, "studio-race")
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if err := app.ReconcileStudioWorkflowTerminal(ctx, intent, "cancelled"); err != nil {
			t.Fatal(err)
		}
		return fakeProviderResult{Structured: map[string]any{"disposition": "proposal", "reply": "建议已形成", "name": "Studio race", "body": map[string]any{"description": "generated suggestion"}, "warnings": []any{}}}
	})
	app.Provider.HTTP = &http.Client{Transport: router}
	result, err := app.ProcessStudioGeneration(ctx, intent)
	if err != nil || result["status"] != "failed" {
		t.Fatal("late output reported a false completion", result, err)
	}
	actual, err := app.StudioJob(ctx, owner, id)
	if err != nil || actual["status"] != "failed" || len(mapValue(actual["result"])) != 0 {
		t.Fatal(actual, err)
	}
}

func TestStudioOutputRejectsMalformedDefinitionsAndEmptyPreview(t *testing.T) {
	snapshot := map[string]any{"resource_kind": "event_template"}
	invalid := map[string]any{"disposition": "proposal", "reply": "建议", "name": "事件", "body": map[string]any{"description": "no structured definition"}, "warnings": []any{}}
	if validateStudioOutput("assistant", snapshot, invalid) == nil {
		t.Fatal("unstructured event accepted")
	}
	valid := worldEventTestDefinition(map[string]any{"kind": "weather", "scope": "world", "value": "virtual rain"})
	invalid["body"] = valid
	if err := validateStudioOutput("assistant", snapshot, invalid); err != nil {
		t.Fatal(err)
	}
	valid["permanent_harm"] = true
	if validateStudioOutput("assistant", snapshot, invalid) == nil {
		t.Fatal("permanent harm accepted")
	}
	if validateStudioOutput("text_preview", snapshot, map[string]any{"text": " ", "label": "虚构创作预览·不进入生活"}) == nil {
		t.Fatal("empty preview accepted")
	}
	if validateStudioOutput("text_preview", snapshot, map[string]any{"text": "fiction", "label": "actual life"}) == nil {
		t.Fatal("unlabeled preview accepted")
	}
}

// The validated output below is an explicit crash fixture, not model evidence.
func TestStudioFrozenJobReplayRecoveryDiscardAndMaterialIsolation(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "studio-job-recovery")
	app.Provider = nil
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "Studio test place", "body": map[string]any{"description": "Owner definition"}, "reason": "test fixture", "idempotency_key": "studio-resource"})
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "text_preview", "name": "Unsaved name", "body": map[string]any{"description": "Unsaved draft"}, "instruction": "Write isolated fiction", "reason": "explicit test", "idempotency_key": "studio-admit"}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, body)
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	replay, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, body)
	if err != nil || replay["id"] != id {
		t.Fatal(replay, err)
	}
	body["instruction"] = "changed"
	if _, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, body); err == nil {
		t.Fatal("changed idempotent input accepted")
	}
	if _, err := app.StudioJob(ctx, owner+"-other", id); err == nil {
		t.Fatal("foreign Owner read allowed")
	}
	var aid, intent string
	if err := repo.Pool().QueryRow(ctx, `SELECT a.id,i.intent_id FROM public.studio_job_attempts a JOIN public.platform_workflow_intents i ON i.payload->>'studio_attempt_id'=a.id WHERE a.job_id=$1`, id).Scan(&aid, &intent); err != nil {
		t.Fatal(err)
	}
	output := map[string]any{"text": "An explicitly fictional test preview.", "label": "虚构创作预览·不进入生活"}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_job_attempts SET output=$2,status='running' WHERE id=$1`, aid, jsonBytes(output)); err != nil {
		t.Fatal(err)
	}
	if err := app.ReconcileStudioWorkflowTerminal(ctx, intent, "failed"); err != nil {
		t.Fatal(err)
	}
	failed, err := app.StudioJob(ctx, owner, id)
	if err != nil || failed["status"] != "failed" {
		t.Fatal(failed, err)
	}
	retried, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+id+"/retry", nil, owner, map[string]any{"expected_revision": failed["revision"], "reason": "recover completed stage", "idempotency_key": "studio-retry"})
	if err != nil {
		t.Fatal(err)
	}
	if intValue(retried["latest_attempt"]) != 2 {
		t.Fatal(retried)
	}
	intent = "studio_intent:" + id + "_attempt_2"
	if _, err := app.ProcessStudioGeneration(ctx, intent); err != nil {
		t.Fatal("completed model stage should not require Provider", err)
	}
	completed, err := app.StudioJob(ctx, owner, id)
	if err != nil || completed["status"] != "completed" {
		t.Fatal(completed, err)
	}
	if _, err := app.ProcessStudioGeneration(ctx, intent); err != nil {
		t.Fatal(err)
	}
	saved, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+id+"/save-material", nil, owner, map[string]any{"expected_revision": completed["revision"], "reason": "save definition material", "idempotency_key": "studio-material"})
	if err != nil {
		t.Fatal(err)
	}
	ms, err := app.DispatchStudioJobSurface(ctx, "GET", "/internal/studio/materials", url.Values{"resource_id": {stringValue(resource["id"])}}, owner, nil)
	if err != nil || len(arrayValue(ms["items"])) != 1 {
		t.Fatal(ms, err)
	}
	if saved["resource_id"] != resource["id"] {
		t.Fatal(saved)
	}
	current, err := app.DispatchOwnerSurface(ctx, "GET", "/internal/studio/resources/"+stringValue(resource["id"]), nil, owner, nil)
	if err != nil || current["name"] != "Studio test place" || intValue(current["revision"]) != 1 {
		t.Fatal("generation overwrote canonical definition", current, err)
	}
	discarded, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+id+"/discard", nil, owner, map[string]any{"expected_revision": completed["revision"], "reason": "discard result", "idempotency_key": "studio-discard"})
	if err != nil || discarded["status"] != "discarded" {
		t.Fatal(discarded, err)
	}
	if _, err := app.ProcessStudioGeneration(ctx, intent); err != nil {
		t.Fatal(err)
	}
	after, err := app.StudioJob(ctx, owner, id)
	if err != nil || after["status"] != "discarded" || len(mapValue(after["result"])) != 0 {
		t.Fatal("late replay republished discarded output", after, err)
	}
}
