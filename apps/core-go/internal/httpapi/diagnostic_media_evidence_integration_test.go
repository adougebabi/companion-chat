package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	workerflow "github.com/fluctlight/local-ai-companion/apps/core-go/internal/workflow"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// Formal Tool + actual HTTP + real Temporal/Dispatcher + authenticated Owner
// reads. Exported data is replay evidence, never a hand-authored failed task.
func TestDiagnosticMediaWorkflowExportsAuthoritativeRecords(t *testing.T) {
	if os.Getenv("FLUCTLIGHT_DIAGNOSTIC_MEDIA_EVIDENCE") != "1" {
		t.Skip("task-owned diagnostic media evidence opt-in required")
	}
	address := os.Getenv("FLUCTLIGHT_TEMPORAL_TEST_ADDRESS")
	host, _, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		t.Fatal("task-owned loopback Temporal required")
	}
	database, err := url.Parse(os.Getenv("GO_CORE_TEST_DATABASE_URL"))
	if err != nil || net.ParseIP(database.Hostname()) == nil || !net.ParseIP(database.Hostname()).IsLoopback() {
		t.Fatal("task-owned loopback PostgreSQL required")
	}
	ctx, repo, server := m4IsolatedHTTPFixture(t)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('diagnostic-media-role','fluctlight','active');
INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('diagnostic-media-role','human-1','blank_slate','active','{"personality_system":{"active_profile_id":"default"}}','{"name":"隔离媒体诊断角色","timezone":"Asia/Shanghai"}','{}','{}','{}','{}');
INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES('diagnostic-media-role',0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now());
INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES('diagnostic-media-role');
INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES('diagnostic-media-conversation','human-1','Isolated diagnostic evidence');
INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES('diagnostic-media-conversation',1);
INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES('diagnostic-media-conversation','human-1','owner','active'),('diagnostic-media-conversation','diagnostic-media-role','member','active')`); err != nil {
		t.Fatal(err)
	}
	var requests, rendererRequests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			_, _ = w.Write([]byte(`{"data":[{"id":"diagnostic-controlled-model"}]}`))
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected provider route: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		requests.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["max_tokens"] != nil || body["max_completion_tokens"] != nil {
			t.Error("unexpected application token cap")
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Controlled diagnostic Provider rejected this request","type":"invalid_request"}}`))
	}))
	defer provider.Close()
	renderer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rendererRequests.Add(1); w.WriteHeader(500) }))
	defer renderer.Close()
	app, err := core.NewApp(repo, base64.StdEncoding.EncodeToString(make([]byte, 32)), "service-key", "http://127.0.0.1:1", "us-east-1", "fixture", "fixture", "fixture", false)
	if err != nil {
		t.Fatal(err)
	}
	app.DiagnosticDebugReadEnabled = true
	app.Provider.HTTP = provider.Client()
	server.app = app
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status) VALUES('diagnostic-controlled-endpoint','openai_compatible',$1,'diagnostic-no-auth','ready');`, provider.URL+"/v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds,required_capabilities) VALUES('generic_llm','diagnostic-controlled-endpoint','diagnostic-controlled-model',10,'structured_output,tool_calling')`); err != nil {
		t.Fatal(err)
	}
	graph := map[string]any{"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}}
	setting, _ := json.Marshal(map[string]any{"baseUrl": renderer.URL, "workflow": graph})
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, setting); err != nil {
		t.Fatal(err)
	}
	c, err := client.Dial(client.Options{HostPort: address, Namespace: "default"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	app.SetWorkflowRuntime(workerflow.NewTemporalRuntime(c, "default", "diagnostic-evidence"))
	workerflow.Configure(app)
	defer workerflow.Configure(nil)
	scope := fmt.Sprint(time.Now().UnixNano())
	admit := func(operation string) (string, string) {
		t.Helper()
		arguments, _ := json.Marshal(map[string]any{"intent": "隔离回归：窗外无人物风景", "media_plan": map[string]any{"primary_subject": "landscape", "human_presence": "none", "self_in_frame": "no", "visible_scope": "unknown", "capture_method": "unspecified", "viewpoint": "unspecified", "framing": "unspecified", "camera_angle": "unspecified"}})
		receipt, err := app.ExecuteTool(ctx, core.ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: operation + "-" + scope, AuthorizationActorID: "human-1", FluctlightID: "diagnostic-media-role", ConversationID: "diagnostic-media-conversation", TargetKind: "conversation", TargetRef: "diagnostic-media-conversation", Arguments: arguments})
		if err != nil || receipt.Result.Status != "accepted" {
			t.Fatalf("formal image admission: %#v %v", receipt, err)
		}
		output := receipt.Result.Output.(map[string]any)
		return output["media_intent_id"].(string), output["task_id"].(string)
	}
	mediaID, taskKey := admit("diagnostic-failed-image")
	var wf, queue string
	if err := repo.Pool().QueryRow(ctx, `SELECT workflow_id,task_queue FROM public.platform_workflow_intents WHERE payload->>'intent_id'=$1`, mediaID).Scan(&wf, &queue); err != nil {
		t.Fatal(err)
	}
	w := worker.New(c, queue, worker.Options{})
	w.RegisterWorkflow(workerflow.MediaWorkflow)
	w.RegisterActivity(workerflow.ProcessMediaActivity)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	d := workerflow.Dispatcher{App: app, Client: c}
	if n, err := d.DispatchOnce(ctx, 1); err != nil || n != 1 {
		t.Fatal("dispatch", n, err)
	}
	temporalID := wf
	if !strings.HasPrefix(temporalID, "go:") {
		temporalID = "go:" + temporalID
	}
	if err := c.GetWorkflow(ctx, temporalID, "").Get(ctx, nil); err == nil {
		t.Fatal("controlled Provider failure unexpectedly succeeded")
	}
	if _, err := d.ReconcileOnce(ctx, 100); err != nil {
		t.Fatal("real terminal reconciliation", err)
	}
	_, emptyKey := admit("diagnostic-unexecuted-image")
	handler := server.Handler()
	read := func(path string) map[string]any {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(&http.Cookie{Name: "fluctlight_session", Value: "session"})
		request.Header.Set(serviceKeyHeader, "service-key")
		request.Header.Set(humanSessionHeader, "session")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("Owner read %s status=%d body=%s", path, response.Code, response.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	cases := []map[string]any{}
	for index, key := range []string{taskKey, emptyKey} {
		task := read("/api/diagnostics/tasks/" + url.PathEscape(key))
		correlation := task["correlation_id"].(string)
		models := read("/api/diagnostic-records/model-runs?correlationId=" + url.QueryEscape(correlation))
		items := models["items"].([]any)
		if index == 0 && (task["status"] != "failed" || len(items) != int(requests.Load()) || len(items) == 0 || len(task["steps"].([]any)) == 0) {
			t.Fatalf("failed evidence not authoritative: status=%v records=%d http=%d", task["status"], len(items), requests.Load())
		}
		if index == 1 && (len(items) != 0 || task["status"] != "pending") {
			t.Fatal("unexecuted control fabricated model calls/state")
		}
		entry := map[string]any{"task": task, "models": models, "workflow": read("/api/workflow-controls/" + url.PathEscape(task["workflow_id"].(string))), "media": read("/api/diagnostic-records/media-prompts?correlationId=" + url.QueryEscape(correlation))}
		cases = append(cases, entry)
	}
	if rendererRequests.Load() != 0 {
		t.Fatal("Provider failure reached renderer")
	}
	data := map[string]any{"source": "isolated_real_core_temporal_controlled_http", "owner": "human-1", "fluctlight_id": "diagnostic-media-role", "provider_requests": requests.Load(), "renderer_requests": rendererRequests.Load(), "cases": cases, "workflows": read("/api/diagnostic-records/workflow-intents"), "pending": read("/api/pending?kind=failed_task"), "pending_count": read("/api/pending/count")}
	if path := os.Getenv("FLUCTLIGHT_DIAGNOSTIC_MEDIA_EXPORT"); path != "" {
		encoded, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("REAL_DIAGNOSTIC_MEDIA http_requests=%d model_records=%d failed_task=%s empty_task=%s renderer_requests=%d", requests.Load(), len(cases[0]["models"].(map[string]any)["items"].([]any)), taskKey, emptyKey, rendererRequests.Load())
}
