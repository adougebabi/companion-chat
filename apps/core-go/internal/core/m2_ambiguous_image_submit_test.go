package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The controlled renderer accepts the first request, then drops its response.
// A fresh process must not submit another job when the first outcome is unknown.
func TestImageAmbiguousSubmitDoesNotCreateAnotherRendererJob(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "m2-submit-ambiguity")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','revision-1')`, owner); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repo, "m2-ambiguity-provider")
	var submissions atomic.Int32
	var concept map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || providerWireSchemaName(payload) != "media_prompt_normalized" {
				t.Errorf("unexpected controlled normalizer request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(mediaTestOutput(concept))}}}})
		case r.URL.Path == "/prompt":
			_, _ = io.Copy(io.Discard, r.Body)
			if submissions.Add(1) == 1 {
				// The test server records acceptance before losing the reply.
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				_ = connection.Close()
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"prompt_id":"duplicate-test-job"}`)
		case r.URL.Path == "/history/duplicate-test-job":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, "m2-ambiguity-provider", server.URL); err != nil {
		t.Fatal(err)
	}
	graph := map[string]any{"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.runtime_settings SET value_json=$1 WHERE key='media.comfyui'`, jsonString(map[string]any{"baseUrl": server.URL, "workflow": graph})); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m2-submit-ambiguity", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}
	receipt, err := app.ExecuteTool(ctx, request)
	if err != nil || receipt.Result.Status != "accepted" {
		t.Fatalf("formal Image admission: %#v err=%v", receipt, err)
	}
	intentID := stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
	var raw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, intentID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	concept = decodeObject(raw)
	dbURL, err := url.Parse(os.Getenv("GO_CORE_TEST_DATABASE_URL"))
	if err != nil || dbURL.Hostname() != "127.0.0.1" || !strings.HasPrefix(repo.Pool().Config().ConnConfig.Database, "lac_core_") {
		t.Fatal("refusing to pass a non-isolated database to child processes")
	}
	dbURL.Path = "/" + repo.Pool().Config().ConnConfig.Database
	childURL := dbURL.String()
	runChild := func(phase string) (int, string) {
		t.Helper()
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		childCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		command := exec.CommandContext(childCtx, executable, "-test.run=^TestImageAmbiguousSubmitChild$", "-test.v")
		command.Env = append(os.Environ(), "M205_PHASE="+phase, "M205_DB_URL="+childURL, "M205_INTENT="+intentID)
		var output bytes.Buffer
		command.Stdout, command.Stderr = &output, &output
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		pid := command.Process.Pid
		err = command.Wait()
		redacted := strings.ReplaceAll(output.String(), childURL, "[isolated database URL]")
		if dbURL.User != nil {
			if password, ok := dbURL.User.Password(); ok && password != "" {
				redacted = strings.ReplaceAll(redacted, password, "[database password]")
			}
		}
		t.Logf("child phase=%s pid=%d output:\n%s", phase, pid, redacted)
		if err != nil {
			t.Fatalf("child phase %s failed: %v", phase, err)
		}
		return pid, redacted
	}
	firstPID, firstOutput := runChild("lost_response")
	var jobID string
	if err := repo.Pool().QueryRow(ctx, `SELECT COALESCE(provider_job_id,'') FROM public.media_intents WHERE id=$1`, intentID).Scan(&jobID); err != nil || jobID != "" || submissions.Load() != 1 || !strings.Contains(firstOutput, "EOF") {
		t.Fatalf("lost response fixture did not reach the intended window: job=%q submissions=%d err=%v", jobID, submissions.Load(), err)
	}
	secondPID, secondOutput := runChild("resume")
	if firstPID == secondPID {
		t.Fatal("recovery reused the original process")
	}
	if submissions.Load() != 1 || !strings.Contains(secondOutput, "media_submission_ambiguous") {
		t.Fatalf("unknown submission was retried automatically: renderer_submissions=%d recovery=%s", submissions.Load(), secondOutput)
	}
	var status, workflowID, workflowIntentID string
	var marked bool
	var revision int
	if err := repo.Pool().QueryRow(ctx, `SELECT status,renderer_submission_started,revision,workflow_id FROM public.media_intents WHERE id=$1`, intentID).Scan(&status, &marked, &revision, &workflowID); err != nil || status != mediaSubmissionUnknownStatus || !marked {
		t.Fatalf("ambiguous state status=%s marked=%t err=%v", status, marked, err)
	}
	if err := app.RecordMediaActivityFailure(ctx, intentID, workflowID, "controlled workflow failure"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT status,revision FROM public.media_intents WHERE id=$1`, intentID).Scan(&status, &revision); err != nil || status != mediaSubmissionUnknownStatus {
		t.Fatalf("terminal failure overwrote uncertainty: status=%s err=%v", status, err)
	}
	if err := repo.Pool().QueryRow(ctx, `UPDATE public.platform_workflow_intents SET status='failed' WHERE workflow_id=$1 RETURNING intent_id`, workflowID).Scan(&workflowIntentID); err != nil {
		t.Fatal(err)
	}
	if err := app.ReconcileUncertainMediaSubmission(ctx, workflowIntentID); err != nil {
		t.Fatal(err)
	}
	_, err = app.RetryOwnerDiagnosticMedia(ctx, owner, intentID, map[string]any{"expected_revision": revision, "mode": "resume", "idempotency_key": "m2-ambiguous-resume", "reason": "核对原服务"})
	var surface *SurfaceError
	if !errors.As(err, &surface) || surface.Code != "media_submission_review_required" {
		t.Fatalf("unreviewed resume was not rejected: %v", err)
	}
	_, err = app.RetryOwnerDiagnosticMedia(ctx, owner, intentID, map[string]any{"expected_revision": revision, "mode": "regenerate", "idempotency_key": "m2-ambiguous-unreviewed", "reason": "未确认核对"})
	if !errors.As(err, &surface) || surface.Code != "media_submission_review_required" {
		t.Fatalf("regenerate without an explicit review was not rejected: %v", err)
	}
	manual, err := app.RetryOwnerDiagnosticMedia(ctx, owner, intentID, map[string]any{"expected_revision": revision, "mode": "regenerate", "submission_reviewed": true, "idempotency_key": "m2-ambiguous-manual", "reason": "已人工核对原服务，明确重新生成"})
	if err != nil || manual["status"] != "retry_queued" || submissions.Load() != 1 {
		t.Fatalf("explicit reviewed retry: %#v submissions=%d err=%v", manual, submissions.Load(), err)
	}
	var prior string
	if err := repo.Pool().QueryRow(ctx, `SELECT m.status,m.renderer_submission_started,COALESCE(r.previous->>'status','') FROM public.media_intents m JOIN public.media_recovery_attempts r ON r.media_intent_id=m.id WHERE m.id=$1`, intentID).Scan(&status, &marked, &prior); err != nil || status != "pending" || marked || prior != mediaSubmissionUnknownStatus {
		t.Fatalf("reviewed retry did not preserve the old uncertain state: status=%s marked=%t prior=%s err=%v", status, marked, prior, err)
	}
}

func TestImageAmbiguousSubmitChild(t *testing.T) {
	phase := os.Getenv("M205_PHASE")
	if phase == "" {
		t.Skip("only run by the isolated ambiguous submission parent test")
	}
	dbURL, err := url.Parse(os.Getenv("M205_DB_URL"))
	if err != nil || dbURL.Hostname() != "127.0.0.1" || !strings.HasPrefix(strings.TrimPrefix(dbURL.Path, "/"), "lac_core_") {
		t.Fatal("child refused a non-isolated database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	repo, err := NewPostgresRepository(ctx, dbURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	app := newTestApp(t, repo, nil)
	result, runErr := app.ProcessMediaIntent(ctx, os.Getenv("M205_INTENT"))
	if runErr == nil {
		t.Fatalf("expected no successful result for phase %s: %#v", phase, result)
	}
	t.Logf("phase=%s actual_error=%v", phase, runErr)
}

func TestMediaSubmissionGuardPausesLegacyAndInterruptedWork(t *testing.T) {
	ctx, repo, app, _, fid, conversation := setupDirectPublicationToolTest(t, "m2-legacy-submit")
	for _, sample := range []struct {
		id, status string
		reconcile  bool
	}{{"legacy-no-job", "pending", false}, {"legacy-failed-no-job", "failed", false}, {"interrupted-submit", "submitting", true}, {"reviewed-ready-missing", "reviewed_ready", false}} {
		t.Run(sample.id, func(t *testing.T) {
			workflow := "workflow-" + sample.id
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,conversation_id,status,renderer_submission_started) VALUES($1,$2,'image','image/png','{}',$3,$4,$5,$6,true)`, sample.id, fid, "request-"+sample.id, workflow, conversation, sample.status); err != nil {
				t.Fatal(err)
			}
			workflowIntent := "intent-" + sample.id
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES($1,$2,'media','media.generation',$3,'failed')`, workflowIntent, workflow, jsonBytes(map[string]any{"intent_id": sample.id})); err != nil {
				t.Fatal(err)
			}
			if sample.reconcile {
				if err := app.ReconcileUncertainMediaSubmission(ctx, workflowIntent); err != nil {
					t.Fatal(err)
				}
			}
			_, err := app.ProcessMediaIntent(ctx, sample.id)
			var failure *CapabilityError
			if !errors.As(err, &failure) || failure.Code != mediaSubmissionAmbiguousCode || failure.Retryable {
				t.Fatalf("uncertain old submission did not pause: %v", err)
			}
			var status string
			if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, sample.id).Scan(&status); err != nil || status != mediaSubmissionUnknownStatus {
				t.Fatalf("old submission status=%q err=%v", status, err)
			}
		})
	}
}

func TestStudioAndVisualIdentityKeepAmbiguousMediaPaused(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "m2-studio-ambiguity")
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "受控场景", "body": map[string]any{"description": "空房间"}, "reason": "fixture", "idempotency_key": "m2-studio-resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "image_preview", "name": "受控场景", "body": map[string]any{"description": "空房间"}, "instruction": "空房间", "reason": "fixture", "idempotency_key": "m2-studio-job"})
	if err != nil {
		t.Fatal(err)
	}
	jobID, intentID, workflow := stringValue(job["id"]), "m2-studio-uncertain", "m2-studio-workflow"
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_jobs SET status='running' WHERE id=$1`, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_job_attempts SET status='running',output=$2 WHERE job_id=$1`, jobID, jsonBytes(map[string]any{"media_intent_id": intentID, "workflow_id": workflow})); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,media_scope,studio_owner_actor_id,studio_job_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,renderer_submission_started) VALUES($1,NULL,'studio',$2,$3,'image','image/png','{}','m2-studio-request',$4,'submitting',true)`, intentID, owner, jobID, workflow); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES('m2-studio-media-intent',$1,'media','media.generation',$2,'failed')`, workflow, jsonBytes(map[string]any{"intent_id": intentID})); err != nil {
		t.Fatal(err)
	}
	if err := app.RecordMediaActivityFailure(ctx, intentID, workflow, "controlled lost response"); err != nil {
		t.Fatal(err)
	}
	if err := app.ReconcileStudioMediaTerminal(ctx, "m2-studio-media-intent", "workflow_failed"); err != nil {
		t.Fatal(err)
	}
	result, err := app.ReconcileStudioWorkflowFailure(ctx, "studio_intent:"+jobID+"_attempt_1", "activity heartbeat timeout")
	if err != nil || result.NextIntentID != "" || result.ReasonCode != "studio_media_already_delegated" {
		t.Fatalf("uncertain Studio media caused an automatic new attempt: %#v err=%v", result, err)
	}
	actual, err := app.StudioJob(ctx, owner, jobID)
	if err != nil || actual["status"] != "running" || actual["error_code"] != mediaSubmissionAmbiguousCode || intValue(actual["latest_attempt"]) != 1 {
		t.Fatalf("Studio did not retain its paused uncertainty: %#v err=%v", actual, err)
	}
	var status string
	if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, intentID).Scan(&status); err != nil || status != mediaSubmissionUnknownStatus {
		t.Fatalf("Studio media status=%q err=%v", status, err)
	}
	// Old failed rows have no reliable evidence that the renderer never saw
	// the request. Planner reconciliation must not create a replacement job.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_intents SET status='failed' WHERE id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	legacy, err := app.ReconcileStudioWorkflowFailure(ctx, "studio_intent:"+jobID+"_attempt_1", "activity heartbeat timeout")
	if err != nil || legacy.NextIntentID != "" || legacy.ReasonCode != "studio_media_already_delegated" {
		t.Fatalf("legacy uncertain Studio media caused a new attempt: %#v err=%v", legacy, err)
	}
	if err := app.ReconcileStudioMediaTerminal(ctx, "m2-studio-media-intent", "workflow_failed"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, intentID).Scan(&status); err != nil || status != mediaSubmissionUnknownStatus {
		t.Fatalf("legacy Studio media was not paused: status=%s err=%v", status, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_jobs SET status='failed',error_code='studio_media_generation_failed' WHERE id=$1`, jobID); err != nil {
		t.Fatal(err)
	}
	oldView, err := app.StudioJob(ctx, owner, jobID)
	if err != nil || oldView["submission_uncertain"] != true {
		t.Fatalf("legacy failed Studio job hid the uncertain media: %#v err=%v", oldView, err)
	}
	page, err := app.DispatchStudioJobSurface(ctx, "GET", "/internal/studio/jobs", url.Values{}, owner, nil)
	if err != nil || len(arrayValue(page["items"])) != 1 || mapValue(arrayValue(page["items"])[0])["submission_uncertain"] != true {
		t.Fatalf("Studio job list hid the uncertain media: %#v err=%v", page, err)
	}
	_, err = app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+jobID+"/retry", nil, owner, map[string]any{"expected_revision": oldView["revision"], "reason": "未核对前尝试", "idempotency_key": "m2-studio-unreviewed"})
	var surface *SurfaceError
	if !errors.As(err, &surface) || surface.Code != "media_submission_review_required" {
		t.Fatalf("legacy Studio retry bypassed the review pause: %v", err)
	}
	state := visualIdentityAgentState{SessionStatus: "running", MediaIntentID: "visual-uncertain", MediaStatus: mediaSubmissionUnknownStatus}
	if err := classifyVisualIdentityAgentState(&state); err != nil || state.WaitingStage != "image_"+mediaSubmissionUnknownStatus || state.ActionRequired != "" {
		t.Fatalf("visual identity did not wait on uncertainty: %#v err=%v", state, err)
	}
}

func TestReviewedAmbiguousMediaCanResumeAlreadyReadyAsset(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "m2-reviewed-ready")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','revision-1')`, owner); err != nil {
		t.Fatal(err)
	}
	const intentID, workflow = "m2-reviewed-ready", "m2-reviewed-ready-workflow"
	assetID := "asset_" + intentID
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,conversation_id,status,revision,renderer_submission_started) VALUES($1,$2,'image','image/png','{}','m2-reviewed-request',$3,$4,'failed',1,true)`, intentID, fid, workflow, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES($1,$2,'v1','image','image/png',1,'fixture-digest','fixture','not-read','m2-reviewed-request',$3,'ready')`, assetID, fid, workflow); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES('m2-reviewed-old',$1,'media','media.generation',$2,'failed')`, workflow, jsonBytes(map[string]any{"intent_id": intentID})); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessMediaIntent(ctx, intentID); err == nil || !strings.Contains(err.Error(), mediaSubmissionAmbiguousCode) {
		t.Fatalf("ready asset bypassed the manual pause: %v", err)
	}
	var messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND attachment_refs ? $2`, conversation, assetID).Scan(&messages); err != nil || messages != 0 {
		t.Fatalf("asset was delivered before review: messages=%d err=%v", messages, err)
	}
	var revision int
	if err := repo.Pool().QueryRow(ctx, `SELECT revision FROM public.media_intents WHERE id=$1 AND status='submission_unknown'`, intentID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	result, err := app.RetryOwnerDiagnosticMedia(ctx, owner, intentID, map[string]any{"expected_revision": revision, "mode": "resume", "submission_reviewed": true, "idempotency_key": "m2-reviewed-ready-resume", "reason": "已核对该资产属于原任务"})
	if err != nil || result["status"] != "retry_queued" {
		t.Fatalf("reviewed ready asset could not resume: %#v err=%v", result, err)
	}
	processed, err := app.ProcessMediaIntent(ctx, intentID)
	if err != nil || processed["status"] != "completed" {
		t.Fatalf("reviewed asset was not delivered: %#v err=%v", processed, err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND attachment_refs ? $2`, conversation, assetID).Scan(&messages); err != nil || messages != 1 {
		t.Fatalf("reviewed asset delivery count=%d err=%v", messages, err)
	}
}

func TestDefiniteRendererValidationRejectionCanBeRetried(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "m2-definite-rejection")
	seedCognitiveProviderRole(t, ctx, repo, "m2-rejection-provider")
	var submissions atomic.Int32
	var concept map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(mediaTestOutput(concept))}}}})
		case r.URL.Path == "/prompt":
			if submissions.Add(1) == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"type":"prompt_outputs_failed_validation"},"node_errors":{}}`)
				return
			}
			_, _ = io.WriteString(w, `{"prompt_id":"accepted-after-rejection"}`)
		case r.URL.Path == "/history/accepted-after-rejection":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, "m2-rejection-provider", server.URL); err != nil {
		t.Fatal(err)
	}
	graph := map[string]any{"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.runtime_settings SET value_json=$1 WHERE key='media.comfyui'`, jsonString(map[string]any{"baseUrl": server.URL, "workflow": graph})); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m2-definite-rejection", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}
	receipt, err := app.ExecuteTool(ctx, request)
	if err != nil || receipt.Result.Status != "accepted" {
		t.Fatalf("formal Image admission: %#v err=%v", receipt, err)
	}
	intentID := stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
	var raw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, intentID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	concept = decodeObject(raw)
	if _, err := app.ProcessMediaIntent(ctx, intentID); err == nil || !strings.Contains(err.Error(), "HTTP 400") || strings.Contains(err.Error(), mediaSubmissionAmbiguousCode) {
		t.Fatalf("definite validation rejection was treated as ambiguous: %v", err)
	}
	var status, job string
	var marked bool
	if err := repo.Pool().QueryRow(ctx, `SELECT status,COALESCE(provider_job_id,''),renderer_submission_started FROM public.media_intents WHERE id=$1`, intentID).Scan(&status, &job, &marked); err != nil || status != "pending" || job != "" || marked {
		t.Fatalf("rejected request left an ambiguous fence: status=%s job=%s marked=%t err=%v", status, job, marked, err)
	}
	restarted := newTestApp(t, repo, nil)
	if _, err := restarted.ProcessMediaIntent(ctx, intentID); err == nil || !strings.Contains(err.Error(), "history returned HTTP 503") {
		t.Fatalf("safe retry did not reach the accepted job: %v", err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT status,COALESCE(provider_job_id,''),renderer_submission_started FROM public.media_intents WHERE id=$1`, intentID).Scan(&status, &job, &marked); err != nil || status != "running" || job != "accepted-after-rejection" || !marked || submissions.Load() != 2 {
		t.Fatalf("safe retry state status=%s job=%s marked=%t submissions=%d err=%v", status, job, marked, submissions.Load(), err)
	}
}

func TestOnlyKnownRendererValidationRejectionsClearSubmissionFence(t *testing.T) {
	for _, sample := range []struct {
		status int
		body   string
		want   bool
	}{
		{400, `{"error":{"type":"prompt_outputs_failed_validation"},"node_errors":{}}`, true},
		{400, `{"error":{"type":"invalid_prompt_id"},"node_errors":{}}`, true},
		{400, `{"error":{"type":"unknown_extension_error"},"node_errors":{}}`, false},
		{400, `{"error":{"type":"prompt_outputs_failed_validation"}}`, false},
		{503, `{"error":{"type":"prompt_outputs_failed_validation"},"node_errors":{}}`, false},
		{400, `not-json`, false},
	} {
		if got := isDefiniteComfyValidationRejection(sample.status, []byte(sample.body)); got != sample.want {
			t.Errorf("status=%d body=%s got=%t want=%t", sample.status, sample.body, got, sample.want)
		}
	}
}

func TestLegacyVisualIdentityWithoutJobRemainsUncertain(t *testing.T) {
	for _, stage := range []string{"candidate", "character_sheet"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newVisualIdentityToolFixture(t)
			receipt := fixture.generateCandidate(t, "m2-legacy-visual-"+stage)
			mediaID := stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
			if mediaID == "" {
				t.Fatal("visual media intent missing")
			}
			if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.media_intents SET status='failed',provider_job_id=NULL,renderer_submission_started=true WHERE id=$1`, mediaID); err != nil {
				t.Fatal(err)
			}
			if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.media_intents SET prompt=jsonb_set(prompt::jsonb,'{renderer_constraints}','{"frozen_test_value":true}'::jsonb,true)::text WHERE id=$1`, mediaID); err != nil {
				t.Fatal(err)
			}
			var frozenPrompt, profileID string
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT m.prompt,s.visual_identity_id FROM public.media_intents m JOIN public.fluctlight_visual_identity_sessions s ON s.id=$2 WHERE m.id=$1`, mediaID, fixture.sessionID).Scan(&frozenPrompt, &profileID); err != nil {
				t.Fatal(err)
			}
			if err := fixture.app.refreshVisualIdentityRendererConstraints(fixture.ctx, fixture.fluctlightID, profileID); err != nil {
				t.Fatal(err)
			}
			var afterPrompt string
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, mediaID).Scan(&afterPrompt); err != nil || afterPrompt != frozenPrompt {
				t.Fatalf("uncertain submitted prompt changed during visual refresh: err=%v", err)
			}
			if stage == "character_sheet" {
				if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='character_sheet_pending',character_sheet_media_intent_id=$2 WHERE id=$1`, fixture.sessionID, mediaID); err != nil {
					t.Fatal(err)
				}
			}
			result, err := fixture.app.ProcessVisualIdentity(fixture.ctx, fixture.sessionID)
			wantStage := "image_" + mediaSubmissionUnknownStatus
			flag := "candidate_submission_uncertain"
			if stage == "character_sheet" {
				wantStage = "character_sheet_" + mediaSubmissionUnknownStatus
				flag = "character_sheet_submission_uncertain"
			}
			if err != nil || result["status"] != "waiting" || result["stage"] != wantStage {
				t.Fatalf("legacy visual submission was treated as failed: %#v err=%v", result, err)
			}
			view, err := fixture.app.OwnerVisualIdentity(fixture.ctx, fixture.ownerID, fixture.fluctlightID)
			if err != nil || mapValue(view["session"])[flag] != true {
				t.Fatalf("manual-review status missing from visual management: %#v err=%v", view, err)
			}
			var sessionStatus string
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, fixture.sessionID).Scan(&sessionStatus); err != nil || sessionStatus == "failed" {
				t.Fatalf("uncertain visual session marked failed: %s err=%v", sessionStatus, err)
			}
		})
	}
}

func TestUncertainGeneratedAvatarDoesNotStartAnotherIntent(t *testing.T) {
	for _, status := range []string{"failed", mediaSubmissionUnknownStatus} {
		t.Run(status, func(t *testing.T) {
			fixture, _, avatarID, _ := acceptedVisualReferenceWithDerivedIntents(t)
			if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.media_intents SET status=$2,provider_job_id=NULL,renderer_submission_started=true WHERE id=$1`, avatarID, status); err != nil {
				t.Fatal(err)
			}
			reviews, err := readCurrentVisualAssetReviewsWith(fixture.ctx, fixture.repository.Pool(), fixture.fluctlightID, 1)
			if err != nil || mapValue(reviews["avatar"])["status"] != mediaSubmissionUnknownStatus {
				t.Fatalf("avatar uncertainty hidden: %#v err=%v", reviews, err)
			}
			var before, after int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fixture.fluctlightID).Scan(&before); err != nil {
				t.Fatal(err)
			}
			path := "/internal/fluctlights/" + fixture.fluctlightID + "/display-avatar/generate"
			_, err = fixture.app.DispatchOwnerMediaSurface(fixture.ctx, "POST", path, nil, fixture.ownerID, map[string]any{"expected_visual_revision": 1, "idempotency_key": "m2-uncertain-avatar-" + status})
			var surface *SurfaceError
			if !errors.As(err, &surface) || surface.Code != "media_submission_review_required" {
				t.Fatalf("uncertain avatar allowed another generation: %v", err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fixture.fluctlightID).Scan(&after); err != nil || after != before {
				t.Fatalf("avatar request changed media count: before=%d after=%d err=%v", before, after, err)
			}
		})
	}
}
