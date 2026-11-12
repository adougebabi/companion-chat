package core

import (
	"bytes"
	"context"
	"encoding/json"
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
