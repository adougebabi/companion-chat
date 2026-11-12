package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type r1ModelObservation struct {
	Phase         string `json:"phase"`
	Task          string `json:"task"`
	RequestModel  string `json:"request_model"`
	ResponseModel string `json:"response_model,omitempty"`
	HTTPStatus    int    `json:"http_status,omitempty"`
	ResponseFile  string `json:"response_file,omitempty"`
	TransportErr  string `json:"transport_error,omitempty"`
}

// r1LiveTransport forwards real requests and records only model and response
// metadata. The raw synthetic response is retained privately for failed runs;
// request messages, headers and credentials are never recorded here.
type r1LiveTransport struct {
	inner       http.RoundTripper
	host        string
	evidenceDir string
	mu          sync.Mutex
	phase       string
	task        string
	observed    []r1ModelObservation
	writeErr    error
}

func (s *r1LiveTransport) begin(phase, task string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase, s.task = phase, task
	return len(s.observed)
}

func (s *r1LiveTransport) since(index int) ([]r1ModelObservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]r1ModelObservation(nil), s.observed[index:]...), s.writeErr
}

func (s *r1LiveTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme != "http" || request.URL.Host != s.host {
		return nil, errors.New("r1_provider_destination_not_loopback_fixture")
	}
	if !strings.HasSuffix(request.URL.Path, "/chat/completions") {
		return s.inner.RoundTrip(request)
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	s.mu.Lock()
	observation := r1ModelObservation{Phase: s.phase, Task: s.task, RequestModel: payload.Model}
	s.mu.Unlock()
	response, err := s.inner.RoundTrip(request)
	if err != nil {
		observation.TransportErr = fmt.Sprintf("%T", err)
		s.mu.Lock()
		s.observed = append(s.observed, observation)
		s.mu.Unlock()
		return nil, err
	}
	responseBody, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		observation.HTTPStatus = response.StatusCode
		observation.TransportErr = "response_body_read_" + fmt.Sprintf("%T", readErr)
		s.mu.Lock()
		s.observed = append(s.observed, observation)
		s.mu.Unlock()
		return nil, readErr
	}
	response.Body = io.NopCloser(bytes.NewReader(responseBody))
	observation.HTTPStatus = response.StatusCode
	var providerResponse struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(responseBody, &providerResponse) == nil {
		observation.ResponseModel = providerResponse.Model
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := fmt.Sprintf("%s-%s-%03d.json", observation.Phase, observation.Task, len(s.observed)+1)
	path := filepath.Join(s.evidenceDir, name)
	file, fileErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if fileErr == nil {
		_, fileErr = file.Write(responseBody)
		if closeErr := file.Close(); fileErr == nil {
			fileErr = closeErr
		}
	}
	if fileErr != nil {
		s.writeErr = fileErr
	} else {
		observation.ResponseFile = path
	}
	s.observed = append(s.observed, observation)
	return response, nil
}

func r1LoopbackProvider(t *testing.T) (string, string, string, string) {
	t.Helper()
	if os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST") != "1" {
		t.Skip("set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to run the real R1 model test")
	}
	base := strings.TrimRight(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_URL"), "/")
	primary := os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_MODEL")
	secondary := os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_SECONDARY_MODEL")
	evidenceDir := os.Getenv("FLUCTLIGHT_R1_EVIDENCE_DIR")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || net.ParseIP(parsed.Hostname()) == nil || !net.ParseIP(parsed.Hostname()).IsLoopback() {
		t.Fatal("R1 requires an explicit loopback-only HTTP Provider URL")
	}
	if primary != "llmfan46/G4-MeroMero-26B-A4B-it-uncensored-heretic-4bit-mxfp4" || secondary != "Huihui-Qwen3.8-27B-3.8bpw" || evidenceDir == "" || !filepath.IsAbs(evidenceDir) {
		t.Fatal("R1 requires the two confirmed model IDs and an absolute private evidence directory")
	}
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("R1 requires an explicitly isolated PostgreSQL URL; a database SKIP is not acceptance")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil || net.ParseIP(config.ConnConfig.Host) == nil || !net.ParseIP(config.ConnConfig.Host).IsLoopback() || len(config.ConnConfig.Fallbacks) != 0 {
		t.Fatal("R1 requires a loopback-only isolated PostgreSQL endpoint without fallback hosts")
	}
	if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return base, primary, secondary, evidenceDir
}

func r1Versions(t *testing.T, ctx context.Context, app *App, owner string) map[string]any {
	t.Helper()
	saved, err := app.ReadSettings(ctx, owner)
	if err != nil {
		t.Fatalf("R1 test setup: read settings: %v", err)
	}
	return mapValue(saved["versions"])
}

func r1SaveBinding(t *testing.T, ctx context.Context, app *App, owner, endpoint, role, model, key string) {
	t.Helper()
	payload := map[string]any{"role": role, "expected_versions": r1Versions(t, ctx, app, owner), "idempotency_key": key}
	if model == "" {
		payload["clear_override"] = true
	} else {
		payload["endpoint_id"], payload["model_id"], payload["timeout_seconds"] = endpoint, model, 720
	}
	if err := app.ConfigureProviderRole(ctx, owner, payload); err != nil {
		t.Fatalf("R1 test setup: save %s: %v", role, err)
	}
}

func r1Route(t *testing.T, ctx context.Context, app *App, owner string, agent FormalAgentID, variant, role, expectedModel, expectedSource string) providerAssignment {
	t.Helper()
	key := agentModelBindingKey(agent, variant)
	catalog, err := app.OwnerConfigurationCatalog(ctx, owner)
	if err != nil {
		t.Fatalf("R1 readback: catalog: %v", err)
	}
	var route map[string]any
	for _, item := range arrayValue(catalog["agent_model_routes"]) {
		if value := mapValue(item); value["key"] == key {
			route = value
			break
		}
	}
	assignment, err := app.Provider.assignment(withAgentModelRoute(ctx, agent, variant), role)
	if err != nil {
		t.Fatalf("R1 readback: assignment: %v", err)
	}
	if route == nil || route["model_id"] != expectedModel || route["source"] != expectedSource || assignment.ModelID != expectedModel || assignment.SourceBindingRole != route["effective_binding"] {
		t.Fatalf("R1 routing mismatch: task=%s variant=%s catalog_model=%v catalog_source=%v assignment_model=%s binding=%s", agent, variant, route["model_id"], route["source"], assignment.ModelID, assignment.SourceBindingRole)
	}
	versions := r1Versions(t, ctx, app, owner)
	revision, ok := parseRevision(versions["binding:"+assignment.SourceBindingRole])
	if !ok || int64(revision) != assignment.BindingRevision {
		t.Fatalf("R1 configuration version mismatch: task=%s binding=%s readback=%v assignment=%d", agent, assignment.SourceBindingRole, versions["binding:"+assignment.SourceBindingRole], assignment.BindingRevision)
	}
	return assignment
}

func r1ReportCall(t *testing.T, observer *r1LiveTransport, start int, round int, phase, task string, assignment providerAssignment, contract string, taskErr error) {
	t.Helper()
	requests, err := observer.since(start)
	if err != nil {
		t.Fatalf("R1 test evidence write failed: %v", err)
	}
	if len(requests) == 0 {
		t.Fatalf("R1 no physical request: round=%d phase=%s task=%s error=%v", round, phase, task, taskErr)
	}
	for _, request := range requests {
		encoded, _ := json.Marshal(map[string]any{"round": round, "phase": phase, "task": task,
			"binding_source": assignment.SourceBindingRole, "binding_version": assignment.BindingRevision,
			"request_model": request.RequestModel, "response_model": request.ResponseModel,
			"http_status": request.HTTPStatus, "transport_error": request.TransportErr,
			"contract": contract, "response_file": request.ResponseFile})
		t.Logf("R1_OBSERVATION %s", encoded)
		if request.Phase != phase || request.Task != task || request.RequestModel != assignment.ModelID {
			t.Fatalf("R1 production route mismatch: phase=%s task=%s request_model=%s expected=%s", phase, task, request.RequestModel, assignment.ModelID)
		}
		if request.ResponseModel == "" {
			t.Fatalf("R1 Provider response model not observed: phase=%s task=%s; backend model identity remains unverified", phase, task)
		}
		if request.ResponseModel != assignment.ModelID {
			t.Fatalf("R1 Provider reported a different model: phase=%s task=%s response_model=%s expected=%s", phase, task, request.ResponseModel, assignment.ModelID)
		}
	}
	if taskErr != nil {
		t.Fatalf("R1 task output/provider failure: phase=%s task=%s error=%v", phase, task, taskErr)
	}
	if contract != "pass" {
		t.Fatalf("R1 task output contract failed: phase=%s task=%s", phase, task)
	}
}

func TestLiveT02VariantOverrideIsolation(t *testing.T) {
	base, primary, secondary, evidenceDir := r1LoopbackProvider(t)
	parsed, _ := url.Parse(base)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	for round := 1; round <= 2; round++ {
		round := round
		ok := t.Run(fmt.Sprintf("round_%d", round), func(t *testing.T) {
			seedCtx, repository := isolatedCoreTestRepository(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			owner, fluctlight, conversation := fmt.Sprintf("r1-owner-%d", round), fmt.Sprintf("r1-fluctlight-%d", round), fmt.Sprintf("r1-conversation-%d", round)
			seedTurnConversation(t, seedCtx, repository, owner, fluctlight, conversation)
			if _, err := repository.Pool().Exec(seedCtx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
				t.Fatal(err)
			}
			roundDir := filepath.Join(evidenceDir, fmt.Sprintf("round-%d", round))
			if err := os.Mkdir(roundDir, 0o700); err != nil {
				t.Fatal(err)
			}
			observer := &r1LiveTransport{inner: transport, host: parsed.Host, evidenceDir: roundDir}
			app := newTestApp(t, repository, observer)
			endpoint := fmt.Sprintf("r1-endpoint-%d", round)
			endpointBody := map[string]any{"endpoint_id": endpoint, "kind": "openai_compatible", "base_url": base, "secret_purpose": "r1-local-no-secret", "expected_versions": r1Versions(t, ctx, app, owner), "idempotency_key": "r1-endpoint"}
			if err := app.ConfigureProviderEndpoint(ctx, owner, endpointBody); err != nil {
				t.Fatalf("R1 test setup: save endpoint: %v", err)
			}
			r1SaveBinding(t, ctx, app, owner, endpoint, "generic_llm", primary, "r1-default-generic")
			r1SaveBinding(t, ctx, app, owner, endpoint, conversationReplyModelRole, primary, "r1-default-conversation")
			projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, FluctlightID: fluctlight, ConversationID: conversation, MemoryOperation: MemoryForReflection, MemoryConversationMode: MemoryConversationExact})
			if err != nil {
				t.Fatalf("R1 test setup: reflection projection: %v", err)
			}
			variant := "reflection_proposal_v2"
			variantKey := agentModelBindingKey(FormalAgentReflection, variant)
			if variantKey != "agent:reflection:reflection_proposal_v2" {
				t.Fatalf("R1 target variant is not the registered production key: %q", variantKey)
			}
			for _, phase := range []string{"A1", "A2", "A3"} {
				model, source := primary, "default"
				if phase == "A2" {
					r1SaveBinding(t, ctx, app, owner, endpoint, variantKey, secondary, "r1-variant-set")
					model, source = secondary, "variant"
				} else if phase == "A3" {
					r1SaveBinding(t, ctx, app, owner, endpoint, variantKey, "", "r1-variant-clear")
				}
				target := r1Route(t, ctx, app, owner, FormalAgentReflection, variant, "reflection", model, source)
				control := r1Route(t, ctx, app, owner, FormalAgentExpressionLearning, expressionLearningSchema, "cognitive_assessment", primary, "default")
				versions := r1Versions(t, ctx, app, owner)
				t.Logf("R1_CONFIG round=%d phase=%s variant_version=%v target_binding=%s target_version=%d control_binding=%s control_version=%d", round, phase, versions["binding:"+variantKey], target.SourceBindingRole, target.BindingRevision, control.SourceBindingRole, control.BindingRevision)
				start := observer.begin(phase, "reflection")
				result, taskErr := app.RunReflectionProposalTask(ctx, ReflectionProposalTaskInput{Evidence: []map[string]any{{"sequence": 1, "event_type": "conversation.turn", "occurred_at": time.Now().UTC(), "payload": map[string]any{"text": "Owner asked for a concise review of tonight's release plan."}}}, Projection: projection})
				contract := "fail"
				if taskErr == nil {
					taskErr = validateReflectionProviderCompletion(result.Completion)
				}
				if taskErr == nil {
					taskErr = validateCapabilitySchemaValue(result.Completion.Structured, reflectionProposalV2ProviderSchema())
				}
				if taskErr == nil {
					_, taskErr = DecodeReflectionProposalV2(jsonBytes(normalizeReflectionProposalV2Header(result.Completion.Structured)))
				}
				if taskErr == nil {
					contract = "pass"
				}
				r1ReportCall(t, observer, start, round, phase, "reflection", target, contract, taskErr)
				start = observer.begin(phase, "expression_learning")
				proposals, taskErr := app.runExpressionLearningTask(ctx, []expressionLearningSource{{SourceID: fmt.Sprintf("r1-source-%d-%s", round, phase), Text: "慢慢来呀，别着急。"}})
				contract = "fail"
				if taskErr == nil {
					contract = "pass"
				}
				t.Logf("R1_CONTROL_RESULT round=%d phase=%s candidates=%d contract=%s", round, phase, len(proposals), contract)
				r1ReportCall(t, observer, start, round, phase, "expression_learning", control, contract, taskErr)
			}
			var messages int
			if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&messages); err != nil || messages != 0 {
				t.Fatalf("R1 fixture unexpectedly published messages: count=%d error=%v", messages, err)
			}
		})
		if !ok {
			t.FailNow()
		}
	}
}
