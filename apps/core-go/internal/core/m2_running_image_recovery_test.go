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
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// This validates Core's persisted job recovery across OS processes. The model,
// renderer and object store are local controlled HTTP servers, not real services.
func TestImageRunningJobRecoversAcrossProcessesAfterDisable(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "m2-running-image")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','revision-1')`, owner); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repo, "m2-running-provider")
	enableControlledVision(t, ctx, repo)
	var submissions, polls, downloads, uploads, normalizations, qualityChecks atomic.Int32
	var ready atomic.Bool
	var concept map[string]any
	png := ownerMediaPNG(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode controlled Provider request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var response map[string]any
		switch providerWireSchemaName(payload) {
		case "media_prompt_normalized":
			normalizations.Add(1)
			response = mediaTestOutput(concept)
		case "media_quality_acceptance_response":
			qualityChecks.Add(1)
			response = map[string]any{"schema_version": 1, "verdict": "pass", "violations": []any{}, "retry_guidance": "", "observed_facts": map[string]any{"subject_matches": true, "appearance_matches": true, "scene_matches": true, "capture_matches": true, "framing_matches": true}}
		default:
			t.Errorf("unexpected controlled Provider schema %q", providerWireSchemaName(payload))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(response)}}}})
	}))
	defer provider.Close()
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, "m2-running-provider", provider.URL); err != nil {
		t.Fatal(err)
	}
	renderer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/prompt":
			submissions.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"prompt_id":"controlled-running-job"}`)
		case "/history/controlled-running-job":
			polls.Add(1)
			if !ready.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"controlled-running-job":{"outputs":{"save":{"images":[{"filename":"fixture.png","type":"output"}]}}}}`)
		case "/view":
			downloads.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		default:
			http.NotFound(w, r)
		}
	}))
	defer renderer.Close()
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected controlled object-store method %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		uploads.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("ETag", `"controlled-etag"`)
	}))
	defer storage.Close()
	graph := map[string]any{"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.runtime_settings SET value_json=$1 WHERE key='media.comfyui'`, jsonString(map[string]any{"baseUrl": renderer.URL, "workflow": graph})); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m2-running-image", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}
	receipt, err := app.ExecuteTool(ctx, request)
	if err != nil || receipt.Result.Status != "accepted" {
		t.Fatalf("formal Image admission: %#v err=%v", receipt, err)
	}
	intentID := stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
	var status string
	var raw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT status,prompt FROM public.media_intents WHERE id=$1`, intentID).Scan(&status, &raw); err != nil || status != "pending" {
		t.Fatalf("new intent status=%q err=%v", status, err)
	}
	concept = decodeObject(raw)
	dbURL, err := url.Parse(os.Getenv("GO_CORE_TEST_DATABASE_URL"))
	if err != nil || dbURL.Hostname() != "127.0.0.1" || !strings.HasPrefix(repo.Pool().Config().ConnConfig.Database, "lac_core_") {
		t.Fatal("refusing to pass a non-isolated database to child processes")
	}
	dbURL.Path = "/" + repo.Pool().Config().ConnConfig.Database
	childURL := dbURL.String()
	runChild := func(phase string) int {
		t.Helper()
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		childCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		command := exec.CommandContext(childCtx, executable, "-test.run=^TestImageRunningJobRecoveryChild$", "-test.v")
		command.Env = append(os.Environ(), "M204_PHASE="+phase, "M204_DB_URL="+childURL, "M204_INTENT="+intentID, "M204_STORAGE="+storage.URL)
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
		if pid == os.Getpid() {
			t.Fatal("recovery did not use a separate process")
		}
		return pid
	}
	firstPID := runChild("interrupt")
	var jobID string
	var readyAssets int
	if err := repo.Pool().QueryRow(ctx, `SELECT status,COALESCE(provider_job_id,''),(SELECT count(*) FROM public.media_assets WHERE id=$2 AND status='ready') FROM public.media_intents WHERE id=$1`, intentID, "asset_"+intentID).Scan(&status, &jobID, &readyAssets); err != nil || status != "running" || jobID != "controlled-running-job" || readyAssets != 0 {
		t.Fatalf("interrupted state status=%s job=%s ready=%d err=%v", status, jobID, readyAssets, err)
	}
	if _, err := app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": false, "moment": true}}, "expected_versions": map[string]any{}, "idempotency_key": "m2-disable-running-image"}); err != nil {
		t.Fatal(err)
	}
	ready.Store(true)
	secondPID := runChild("resume")
	if firstPID == secondPID {
		t.Fatal("recovery reused the original helper process")
	}
	request.OperationID = "m2-image-after-disable"
	denied, err := app.ExecuteTool(ctx, request)
	var capabilityErr *CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Code != "domain_plugin_disabled" || denied.Result.ErrorCode != "domain_plugin_disabled" {
		t.Fatalf("new Image request was not denied: %#v err=%v", denied, err)
	}
	var intents, assets, messages, references int
	var verdict string
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.media_intents WHERE id=$1 AND status='completed' AND provider_job_id='controlled-running-job'), (SELECT count(*) FROM public.media_assets WHERE id=$2 AND status='ready'), (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$3 AND idempotency_key=$4 AND attachment_refs ? $2), (SELECT count(*) FROM public.media_references WHERE asset_id=$2 AND target_type='conversation' AND target_id=$3), COALESCE((SELECT quality_verdict FROM public.media_intents WHERE id=$1),'')`, intentID, "asset_"+intentID, conversation, "media:"+intentID+":conversation-result").Scan(&intents, &assets, &messages, &references, &verdict); err != nil || intents != 1 || assets != 1 || messages != 1 || references != 1 || verdict != "pass" {
		t.Fatalf("recovered state intents=%d assets=%d messages=%d references=%d verdict=%s err=%v", intents, assets, messages, references, verdict, err)
	}
	if submissions.Load() != 1 || polls.Load() != 2 || downloads.Load() != 1 || uploads.Load() != 1 || normalizations.Load() != 1 || qualityChecks.Load() != 1 {
		t.Fatalf("unexpected controlled service calls: submit=%d poll=%d download=%d upload=%d normalize=%d quality=%d", submissions.Load(), polls.Load(), downloads.Load(), uploads.Load(), normalizations.Load(), qualityChecks.Load())
	}
}

// Invoked only by the parent test in separate processes with an isolated DB.
func TestImageRunningJobRecoveryChild(t *testing.T) {
	phase := os.Getenv("M204_PHASE")
	if phase == "" {
		t.Skip("only run by the isolated recovery parent test")
	}
	if phase != "interrupt" && phase != "resume" {
		t.Fatal("invalid child phase")
	}
	dbURL, err := url.Parse(os.Getenv("M204_DB_URL"))
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
	storageURL, err := url.Parse(os.Getenv("M204_STORAGE"))
	if err != nil || storageURL.Scheme != "http" || storageURL.Hostname() != "127.0.0.1" {
		t.Fatal("child refused a non-local object store")
	}
	app.Storage, err = minio.New(storageURL.Host, &minio.Options{Secure: false, Creds: credentials.NewStaticV4("controlled", "controlled", ""), Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	app.S3Bucket = "m2-recovery-fixture"
	result, err := app.ProcessMediaIntent(ctx, os.Getenv("M204_INTENT"))
	if phase == "interrupt" {
		if err == nil || !strings.Contains(err.Error(), "history returned HTTP 503") {
			t.Fatalf("expected controlled interruption after job submission: result=%#v err=%v", result, err)
		}
		t.Log("controlled renderer history failed after submission, as intended")
		return
	}
	if err != nil || result["status"] != "completed" || result["quality_verdict"] != "pass" {
		t.Fatalf("resume result=%#v err=%v", result, err)
	}
	result, err = app.ProcessMediaIntent(ctx, os.Getenv("M204_INTENT"))
	if err != nil || result["status"] != "completed" {
		t.Fatalf("replay result=%#v err=%v", result, err)
	}
	t.Log("resume and replay completed once; child pid=" + strconv.Itoa(os.Getpid()))
}
