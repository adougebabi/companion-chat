package core

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMediaNormalizerKeepsModelAcrossIntentReload(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repository, "media-snapshot-endpoint")
	settingsKey := []byte("0123456789abcdef0123456789abcdef")
	encrypted, err := encryptSecret(settingsKey, "turn-secret", "test-media-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	role := agentModelBindingKey(FormalAgentMediaPrompt, "media_prompt_normalized")
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'media-snapshot-endpoint','first-model',30)`, role); err != nil {
		t.Fatal(err)
	}
	concept := mediaPreparedConcept(t, "本人在卧室里的自拍", mediaTestPlan("person", "present", "yes", "full"), mediaTestContext("CURRENT"))
	concept["human_count"] = 1
	valid := mediaTestOutput(concept)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES('media-snapshot-intent','owner-1','image','image/png',$1,'media-snapshot-request','media-snapshot-workflow','pending')`, jsonString(concept)); err != nil {
		t.Fatal(err)
	}
	var models []string
	router := newFakeProviderRouter().on("media_prompt_normalized", func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		return fakeProviderResult{Structured: valid}
	})
	app := newTestApp(t, repository, router)
	app.Provider.SettingsKey = settingsKey
	intent, err := app.readMediaIntent(ctx, "media-snapshot-intent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.runMediaPromptTaskResult(ctx, MediaPromptTaskInput{Intent: intent}); err != nil {
		t.Fatal(err)
	}
	var provenance []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT model_provenance FROM public.media_intents WHERE id='media-snapshot-intent'`).Scan(&provenance); err != nil || strings.Contains(string(provenance), "test-media-secret") {
		t.Fatalf("persisted media snapshot exposed a credential or could not be read: %v", err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='next-model' WHERE role=$1`, role); err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, repository, router)
	restarted.Provider.SettingsKey = settingsKey
	intent, err = restarted.readMediaIntent(ctx, "media-snapshot-intent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.runMediaPromptTaskResult(ctx, MediaPromptTaskInput{Intent: intent}); err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "first-model" || models[1] != "first-model" {
		t.Fatalf("media intent changed its normalizer after reload: %v", models)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES('media-snapshot-new','owner-1','image','image/png',$1,'media-snapshot-new-request','media-snapshot-new-workflow','pending')`, jsonString(concept)); err != nil {
		t.Fatal(err)
	}
	newIntent, err := restarted.readMediaIntent(ctx, "media-snapshot-new")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.runMediaPromptTaskResult(ctx, MediaPromptTaskInput{Intent: newIntent}); err != nil {
		t.Fatal(err)
	}
	if len(models) != 3 || models[2] != "next-model" {
		t.Fatalf("new media intent did not use the updated model: %v", models)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.media_intents SET model_provenance='{"_media_model_snapshot_contract":"invalid"}' WHERE id='media-snapshot-new'`); err != nil {
		t.Fatal(err)
	}
	invalidIntent, err := restarted.readMediaIntent(ctx, "media-snapshot-new")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.runMediaPromptTaskResult(ctx, MediaPromptTaskInput{Intent: invalidIntent}); !errors.Is(err, errMediaModelSnapshotInvalid) || len(models) != 3 {
		t.Fatalf("invalid snapshot silently selected a model: models=%v err=%v", models, err)
	}
}

func TestMediaNormalizerSnapshotSurvivesProcessRecovery(t *testing.T) {
	ctx, repository, app, _, fid, _ := setupDirectPublicationToolTest(t, "media-snapshot-process")
	seedCognitiveProviderRole(t, ctx, repository, "media-process-snapshot-endpoint")
	role := agentModelBindingKey(FormalAgentMediaPrompt, "media_prompt_normalized")
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'media-process-snapshot-endpoint','first-model',30)`, role); err != nil {
		t.Fatal(err)
	}
	comfy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer comfy.Close()
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.runtime_settings SET value_json=$1 WHERE key='media.comfyui'`, jsonString(map[string]any{"baseUrl": comfy.URL, "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
		t.Fatal(err)
	}
	concept := mediaPreparedConcept(t, "本人在卧室里的自拍", mediaTestPlan("person", "present", "yes", "full"), mediaTestContext("CURRENT"))
	concept["human_count"] = 1
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES('media-process-snapshot',$1,'image','image/png',$2,'media-process-request','media-process-workflow','pending')`, fid, jsonString(concept)); err != nil {
		t.Fatal(err)
	}
	var models []string
	router := newFakeProviderRouter().on("media_prompt_normalized", func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		if len(models) == 1 {
			return fakeProviderResult{Status: http.StatusBadRequest}
		}
		return fakeProviderResult{Structured: mediaTestOutput(concept)}
	})
	app.Provider.HTTP = &http.Client{Transport: router}
	if _, err := app.ProcessMediaIntent(ctx, "media-process-snapshot"); err == nil || len(models) != 1 {
		t.Fatalf("expected the first Provider attempt to fail: models=%v err=%v", models, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='next-model' WHERE role=$1`, role); err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, repository, router)
	if _, err := restarted.ProcessMediaIntent(ctx, "media-process-snapshot"); err == nil {
		t.Fatal("controlled media processing unexpectedly completed")
	}
	if len(models) != 2 || models[0] != "first-model" || models[1] != "first-model" {
		t.Fatalf("media process recovery changed its normalizer: %v", models)
	}
	var jobID string
	if err := repository.Pool().QueryRow(ctx, `SELECT COALESCE(provider_job_id,'') FROM public.media_intents WHERE id='media-process-snapshot'`).Scan(&jobID); err != nil || jobID != "" {
		t.Fatalf("failed controlled work created a renderer job: job=%q err=%v", jobID, err)
	}
}

func TestLegacyMediaNormalizerReportsUnfrozenBinding(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repository, "media-legacy-snapshot-endpoint")
	concept := mediaPreparedConcept(t, "窗外的风景", mediaTestPlan("landscape", "none", "no", "unknown"), mediaTestContext("CURRENT"))
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,model_provenance) VALUES('legacy-media-model','owner-1','image','image/png',$1,'legacy-media-request','legacy-media-workflow','pending','{}')`, jsonString(concept)); err != nil {
		t.Fatal(err)
	}
	router := newFakeProviderRouter().on("media_prompt_normalized", func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: mediaTestOutput(concept)}
	})
	app := newTestApp(t, repository, router)
	intent, err := app.readMediaIntent(ctx, "legacy-media-model")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.runMediaPromptTaskResult(ctx, MediaPromptTaskInput{Intent: intent}); err != nil {
		t.Fatal(err)
	}
	var count int
	err = repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.diagnostic_events WHERE correlation_id='media:legacy-media-model' AND event_type='media.model_binding.legacy_unfrozen' AND payload->>'task_variant'='media_prompt_normalized'`).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("legacy unfrozen model use was not diagnosed: count=%d err=%v", count, err)
	}
}

func TestMediaModelProvenanceIsExcludedFromDiagnosticsAndRecovery(t *testing.T) {
	ctx, repository, app, owner, fid, _ := setupDirectPublicationToolTest(t, "media-snapshot-privacy")
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'controlled','test')`, owner); err != nil {
		t.Fatal(err)
	}
	const intentID = "media-snapshot-private-intent"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,revision,model_provenance) VALUES($1,$2,'image','image/png','{}','private-request','private-workflow','failed',1,'{"private_marker":"snapshot-private-value"}')`, intentID, fid); err != nil {
		t.Fatal(err)
	}
	result, err := app.RetryOwnerDiagnosticMedia(ctx, owner, intentID, map[string]any{"expected_revision": 1, "mode": "resume", "reason": "恢复测试", "idempotency_key": "media-snapshot-private-recovery"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(jsonString(result), "snapshot-private-value") {
		t.Fatal("recovery response exposed private model provenance")
	}
	attemptID := stringValue(result["id"])
	var previous []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT previous FROM public.media_recovery_attempts WHERE id=$1`, attemptID).Scan(&previous); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(previous), "model_provenance") || strings.Contains(string(previous), "snapshot-private-value") {
		t.Fatal("recovery history stored private model provenance")
	}
	// Old audit rows may already contain an internal field; the read projection
	// must also exclude it without discarding the rest of the recovery history.
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.media_recovery_attempts SET previous=previous||'{"model_provenance":{"private_marker":"snapshot-private-value"}}'::jsonb WHERE id=$1`, attemptID); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ kind, key string }{{"media-prompts", intentID}, {"media-recoveries", attemptID}} {
		source, err := ownerDiagnosticSource(test.kind)
		if err != nil {
			t.Fatal(err)
		}
		var raw []byte
		if err := repository.Pool().QueryRow(ctx, `SELECT data FROM (`+source+`) AS source WHERE key=$2`, owner, test.key).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "model_provenance") || strings.Contains(string(raw), "snapshot-private-value") {
			t.Fatalf("%s exposed private model provenance", test.kind)
		}
		if test.kind == "media-recoveries" && mapValue(decodeObject(raw)["previous"])["id"] != intentID {
			t.Fatal("redaction removed the recovery history")
		}
	}
}
