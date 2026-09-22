package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	falseSuccessProbeChildEnv = "TEST_ONLY_FALSE_SUCCESS_PROBE_CHILD"
	falseSuccessMutationEnv   = "TEST_ONLY_MEMORY_FALSE_SUCCESS"
	brokenFeedbackProbeEnv    = "TEST_ONLY_BROKEN_FEEDBACK_PROBE_CHILD"
	brokenFeedbackMutationEnv = "TEST_ONLY_DROP_TOOL_RESULT_FEEDBACK"
)

// TestIndependentToolE2ERejectsFalseSuccessWithoutWrite proves that the
// independent product assertion, rather than a successful-looking Tool
// receipt, is the success oracle. The fault exists only in this _test.go file.
func TestIndependentToolE2ERejectsFalseSuccessWithoutWrite(t *testing.T) {
	requireFaultSensitivityDatabase(t)

	faultOutput, faultErr := runFaultSensitivityProbe(t, "TestIndependentToolE2EFalseSuccessProbe", map[string]string{
		falseSuccessProbeChildEnv: "1",
		falseSuccessMutationEnv:   "1",
	})
	if faultErr == nil {
		t.Fatalf("false-success mutation unexpectedly passed:\n%s", faultOutput)
	}
	if !strings.Contains(faultOutput, "independent memory product assertion failed") {
		t.Fatalf("false-success probe failed for the wrong reason: %v\n%s", faultErr, faultOutput)
	}

	controlOutput, controlErr := runFaultSensitivityProbe(t, "TestIndependentToolE2EFalseSuccessProbe", map[string]string{
		falseSuccessProbeChildEnv: "1",
	})
	if controlErr != nil {
		t.Fatalf("restored production memory Tool did not pass the same probe: %v\n%s", controlErr, controlOutput)
	}
	t.Log("controlled mutation experiment: test-only memory writer suppression produced a non-zero probe; restored production path passed")
}

// TestIndependentToolE2EFalseSuccessProbe is intentionally inert in the
// ordinary package run. Its parent invokes it in a fresh process so a fatal
// product assertion becomes observable as the required non-zero exit.
func TestIndependentToolE2EFalseSuccessProbe(t *testing.T) {
	if os.Getenv(falseSuccessProbeChildEnv) != "1" {
		return
	}
	fixture := newIndependentToolE2EFixture(t, "false-success-probe")
	app := fixture.app
	if os.Getenv(falseSuccessMutationEnv) == "1" {
		faultApp := newTestApp(t, fixture.repository, nil)
		faultApp.Capabilities = mustCapabilityRegistry(memoryEventCapability{service: testOnlyFalseSuccessMemoryService{delegate: faultApp.Memory}})
		faultApp.ContextResolver = NewAppContextResolver(faultApp)
		app = faultApp
	}

	content := "fault-sensitivity-memory-" + stableDigest(fixture.suffix)[:20]
	request := fixture.request("memory_event", "false-success-write", map[string]any{
		"content": content, "type": "semantic", "confidence": 1.0, "importance": 1.0,
	})
	receipt, err := app.ExecuteTool(fixture.ctx, request)
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("formal memory Tool did not return completed: receipt=%#v err=%v", receipt, err)
	}
	var rows int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx,
		`SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$1 AND content=$2`,
		fixture.fluctlightID, content,
	).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("independent memory product assertion failed: completed receipt had %d durable memory rows", rows)
	}
}

type testOnlyFalseSuccessMemoryService struct {
	delegate memoryCapabilityService
}

func (service testOnlyFalseSuccessMemoryService) prepareMemoryCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	return service.delegate.prepareMemoryCapability(ctx, invocation, resolved)
}

func (testOnlyFalseSuccessMemoryService) applyMemoryCapability(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return testOnlyFalseSuccessMemoryResult(invocation), nil
}

func (testOnlyFalseSuccessMemoryService) applyMemoryCapabilityTx(_ context.Context, _ pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return testOnlyFalseSuccessMemoryResult(invocation), nil
}

func testOnlyFalseSuccessMemoryResult(invocation CapabilityInvocation) CapabilityResult {
	return CapabilityResult{
		CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "memory:test-only-false-success",
		Output: map[string]any{
			"operation": "create", "memory_id": "test_only_missing_memory", "status": "active",
			"revision": 1, "disposition": "applied", "replayed": false,
		},
	}
}

// TestFormalAgentE2ERejectsBrokenToolResultFeedback proves that the formal
// cognition Agent depends on the role=tool refill. A test-only transport
// removes that message on the wire; the controlled provider then cannot derive
// its final answer from the database-only random memory and the same probe
// exits non-zero.
func TestFormalAgentE2ERejectsBrokenToolResultFeedback(t *testing.T) {
	requireFaultSensitivityDatabase(t)

	faultOutput, faultErr := runFaultSensitivityProbe(t, "TestFormalAgentE2EBrokenToolResultFeedbackProbe", map[string]string{
		brokenFeedbackProbeEnv:    "1",
		brokenFeedbackMutationEnv: "1",
	})
	if faultErr == nil {
		t.Fatalf("broken Tool-result feedback mutation unexpectedly passed:\n%s", faultOutput)
	}
	if !strings.Contains(faultOutput, "formal Agent final answer did not use the database-only Tool result") {
		t.Fatalf("broken-feedback probe failed for the wrong reason: %v\n%s", faultErr, faultOutput)
	}

	controlOutput, controlErr := runFaultSensitivityProbe(t, "TestFormalAgentE2EBrokenToolResultFeedbackProbe", map[string]string{
		brokenFeedbackProbeEnv: "1",
	})
	if controlErr != nil {
		t.Fatalf("restored formal Tool-result feedback did not pass the same probe: %v\n%s", controlErr, controlOutput)
	}
	t.Log("controlled mutation experiment: test-only role=tool removal produced a non-zero probe; restored production Eino refill passed")
}

func TestFormalAgentE2EBrokenToolResultFeedbackProbe(t *testing.T) {
	if os.Getenv(brokenFeedbackProbeEnv) != "1" {
		return
	}
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := stableDigest(t.Name() + fmt.Sprintf("-%d", time.Now().UnixNano()))[:20]
	ownerID := "feedback_owner_" + suffix
	fluctlightID := "feedback_fluctlight_" + suffix
	conversationID := "feedback_conversation_" + suffix
	seedTurnConversation(t, ctx, repository, ownerID, fluctlightID, conversationID)

	secretCategory := "sealed-category-" + suffix
	secret := "database-only-secret-" + stableDigest(suffix + "-secret")[:20]
	provider := &controlledFeedbackProvider{secretCategory: secretCategory, secret: secret}
	server := httptest.NewServer(provider)
	defer server.Close()
	transport := server.Client().Transport
	if os.Getenv(brokenFeedbackMutationEnv) == "1" {
		transport = &testOnlyToolFeedbackDroppingTransport{inner: transport}
	}
	app := newTestApp(t, repository, transport)
	provider.seed = func() error {
		for operationID, content := range map[string]string{
			"feedback-guide-" + suffix:  "archive lookup guide: the second lookup category is " + secretCategory,
			"feedback-secret-" + suffix: "category " + secretCategory + " contains exact token " + secret,
		} {
			receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{
				CapabilityName: "memory_event", OperationID: operationID,
				AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
				EvidenceID: "owner-command-" + stableDigest(operationID)[:20], Surface: CapabilitySurfaceNativeCognition,
				Arguments: jsonBytes(map[string]any{"content": content, "type": "semantic", "confidence": 1.0, "importance": 1.0}),
			})
			if err != nil || receipt.Result.Status != "completed" {
				return fmt.Errorf("seed database-only controlled memory: status=%s err=%v", receipt.Result.Status, err)
			}
		}
		return nil
	}
	endpointID := "feedback-endpoint-" + suffix
	if _, err := repository.Pool().Exec(ctx,
		`INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible',$2,$3,'ready',now())`,
		endpointID, server.URL, "feedback-secret-purpose-"+suffix,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx,
		`INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'controlled-feedback-model','structured_output,tool_calling',4096,30,'{}')`,
		endpointID,
	); err != nil {
		t.Fatal(err)
	}

	result, err := app.RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput{
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		RunID:        "controlled-feedback-mutation-" + suffix,
		CurrentInput: "Use memory.recall to retrieve the archive lookup guide and return the exact token found in the Tool result.",
	})
	if err != nil {
		t.Fatalf("formal independent cognition Agent failed before final assertion: %v", err)
	}
	visible := stringValue(result.Completion.Structured["visible_text"])
	if !strings.Contains(visible, secret) {
		t.Fatalf("formal Agent final answer did not use the database-only Tool result: visible=%q", visible)
	}
	requests, requestIDs := provider.snapshot()
	if provider.seedError() != nil {
		t.Fatalf("controlled provider could not seed the database-only memory after initial prompt capture: %v", provider.seedError())
	}
	if len(requests) != 2 || len(requestIDs) != 2 || requestIDs[0] == "" || requestIDs[0] == requestIDs[1] {
		t.Fatalf("controlled provider physical calls=%d request_ids=%v", len(requests), requestIDs)
	}
	if formalAgentRequestContains(requests[0], secret) || formalAgentRequestContains(requests[0], secretCategory) {
		t.Fatal("database-only answer leaked into the initial formal Agent request")
	}
	if !payloadHasToolResult(requests[1]) || !formalAgentRequestContains(requests[1], secret) {
		t.Fatal("restored control refill did not carry the database-only Tool result")
	}
	if result.Trace == nil {
		t.Fatal("formal Agent omitted its native Tool trace")
	}
	invocations, results := result.Trace.Snapshot()
	if len(invocations) != 1 || len(results) != 1 || results[0].Status != "completed" || invocations[0].ProviderRequestID != requestIDs[0] {
		t.Fatalf("formal Agent identity/result trace is incomplete: invocations=%#v results=%#v", invocations, results)
	}
}

type controlledFeedbackProvider struct {
	secretCategory string
	secret         string
	seed           func() error
	seedOnce       sync.Once
	seedErr        error

	mu         sync.Mutex
	requests   []map[string]any
	requestIDs []string
}

func (provider *controlledFeedbackProvider) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	provider.mu.Lock()
	provider.requests = append(provider.requests, cloneMap(payload))
	provider.requestIDs = append(provider.requestIDs, request.Header.Get("X-Fluctlight-Provider-Request-Id"))
	sequence := len(provider.requests)
	provider.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if sequence == 1 {
		provider.seedOnce.Do(func() {
			if provider.seed != nil {
				provider.seedErr = provider.seed()
			}
		})
		if provider.seedErr != nil {
			http.Error(w, provider.seedErr.Error(), http.StatusInternalServerError)
			return
		}
		response := map[string]any{"choices": []any{map[string]any{
			"finish_reason": "tool_calls",
			"message": map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{
				"id": "controlled-feedback-recall", "type": "function",
				"function": map[string]any{"name": "memory.recall", "arguments": jsonString(map[string]any{"intent": "archive lookup guide"})},
			}}},
		}}}
		_, _ = w.Write(jsonBytes(response))
		return
	}
	answer := "controlled-provider-missing-tool-result"
	if payloadHasToolResult(payload) && formalAgentRequestContains(payload, provider.secret) {
		answer = provider.secret
	}
	final := map[string]any{
		"action_type": "reply", "response_intent": "answer",
		"visible_text": answer, "influences": []any{},
	}
	response := map[string]any{"choices": []any{map[string]any{
		"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(final), "tool_calls": []any{}},
	}}}
	_, _ = w.Write(jsonBytes(response))
}

func (provider *controlledFeedbackProvider) snapshot() ([]map[string]any, []string) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	requests := make([]map[string]any, len(provider.requests))
	for index, request := range provider.requests {
		requests[index] = cloneMap(request)
	}
	return requests, append([]string(nil), provider.requestIDs...)
}

func (provider *controlledFeedbackProvider) seedError() error {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.seedErr
}

type testOnlyToolFeedbackDroppingTransport struct {
	inner http.RoundTripper
}

func (transport *testOnlyToolFeedbackDroppingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		messages := arrayValue(payload["messages"])
		filtered := make([]any, 0, len(messages))
		for _, raw := range messages {
			if stringValue(mapValue(raw)["role"]) != "tool" {
				filtered = append(filtered, raw)
			}
		}
		payload["messages"] = filtered
		body = jsonBytes(payload)
	}
	clone := request.Clone(request.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.Header = request.Header.Clone()
	clone.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	return transport.inner.RoundTrip(clone)
}

func requireFaultSensitivityDatabase(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("GO_CORE_TEST_DATABASE_URL")) == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is required for isolated PostgreSQL mutation evidence")
	}
}

func runFaultSensitivityProbe(t *testing.T, testName string, additions map[string]string) (string, error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^"+testName+"$", "-test.v")
	environment := make([]string, 0, len(os.Environ())+len(additions))
	for _, entry := range os.Environ() {
		name := strings.SplitN(entry, "=", 2)[0]
		if name == falseSuccessProbeChildEnv || name == falseSuccessMutationEnv || name == brokenFeedbackProbeEnv || name == brokenFeedbackMutationEnv {
			continue
		}
		environment = append(environment, entry)
	}
	for name, value := range additions {
		environment = append(environment, name+"="+value)
	}
	command.Env = environment
	output, runErr := command.CombinedOutput()
	return string(output), runErr
}
