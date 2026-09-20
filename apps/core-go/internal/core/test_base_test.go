package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// readSourceFile finds and reads a source file, looking first in the current directory (internal/core),
// and searching subpackages (internal/*) or repository root if not found directly.
func readSourceFile(t *testing.T, filename string) []byte {
	t.Helper()
	filename = filepath.Clean(filename)
	// 1. Direct relative check
	if data, err := os.ReadFile(filename); err == nil {
		return data
	}
	// 2. Look in ../<subpkg>/filename or ../../...
	candidates := []string{
		filepath.Join(".", filename),
		filepath.Join("..", filename),
		filepath.Join("..", "personality", filename),
		filepath.Join("..", "conversation", filename),
		filepath.Join("..", "capability", filename),
		filepath.Join("..", "workflow", filename),
		filepath.Join("..", "schedule", filename),
		filepath.Join("..", "memory", filename),
		filepath.Join("..", "media", filename),
		filepath.Join("..", "cognition", filename),
		filepath.Join("..", "reflection", filename),
		filepath.Join("..", "lifecontext", filename),
		filepath.Join("..", "ai", "model", filename),
		filepath.Join("..", "ai", "prompt", filename),
		filepath.Join("..", "ai", "agent", filename),
		filepath.Join("..", "ai", "task", filename),
	}
	for _, cand := range candidates {
		if data, err := os.ReadFile(cand); err == nil {
			return data
		}
	}
	// 3. Fallback: walk from internal/
	var found []byte
	_ = filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(path) == filepath.Base(filename) {
			if data, readErr := os.ReadFile(path); readErr == nil {
				found = data
				return fmt.Errorf("found")
			}
		}
		return nil
	})
	if len(found) > 0 {
		return found
	}
	t.Fatalf("readSourceFile: source file %q not found in any candidate directory", filename)
	return nil
}

// This file is the shared deterministic test base (implement.md phase 1). It
// converges the manual &App{...} assembly that used to be copy-pasted across
// integration tests, provides a scripted Provider router keyed by response
// schema, and captures the real wire payload so budget and "unsupported field"
// claims can be asserted against what was actually sent.

// ---------------------------------------------------------------------------
// Scripted Provider
// ---------------------------------------------------------------------------

// fakeProviderResult is one scripted Provider reply.
type fakeProviderResult struct {
	// Structured is written into message.content as JSON.
	Structured map[string]any
	// ToolCalls is written into message.tool_calls.
	ToolCalls []map[string]any
	// Text is written verbatim into message.content when Structured is nil.
	Text string
	// Status overrides the HTTP status (default 200).
	Status int
	// Err forces a transport error instead of a response.
	Err error
}

// fakeProviderScript decides what the Provider returns for one request.
type fakeProviderScript func(payload map[string]any) fakeProviderResult

// fakeProviderRouter routes by the response schema name on the wire, so a test
// can script the Main turn (conversation_turn_response) and the takeover Judge
// (takeover_judgement_response) independently.
type fakeProviderRouter struct {
	mu       sync.Mutex
	requests []map[string]any
	routes   map[string]fakeProviderScript
	fallback fakeProviderScript
	calls    map[string]int
}

func newFakeProviderRouter() *fakeProviderRouter {
	return &fakeProviderRouter{routes: map[string]fakeProviderScript{}, calls: map[string]int{}}
}

// on scripts the response for one schema name.
func (router *fakeProviderRouter) on(schemaName string, script fakeProviderScript) *fakeProviderRouter {
	router.routes[schemaName] = script
	return router
}

// otherwise scripts the response for any schema without an exact route.
func (router *fakeProviderRouter) otherwise(script fakeProviderScript) *fakeProviderRouter {
	router.fallback = script
	return router
}

func (router *fakeProviderRouter) RoundTrip(request *http.Request) (*http.Response, error) {
	router.mu.Lock()
	defer router.mu.Unlock()

	var body []byte
	if request.Body != nil {
		read, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		body = read
	}
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	router.requests = append(router.requests, payload)

	schemaName := providerWireSchemaName(payload)
	script := router.routes[schemaName]
	if script == nil {
		script = router.fallback
	}
	if script == nil {
		return embeddingHTTPResponse(request, http.StatusOK, `{"choices":[{"message":{"content":"{}","tool_calls":[]}}]}`), nil
	}
	router.calls[schemaName]++
	result := script(payload)
	if result.Err != nil {
		return nil, result.Err
	}
	status := result.Status
	if status == 0 {
		status = http.StatusOK
	}
	content := result.Text
	if result.Structured != nil {
		content = jsonString(result.Structured)
	}
	toolCalls := result.ToolCalls
	if toolCalls == nil {
		toolCalls = []map[string]any{}
	}
	toolCalls = fakeProviderNativeToolCalls(toolCalls)
	message := map[string]any{"role": "assistant", "content": content, "tool_calls": toolCalls}
	envelope := map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": message}}}
	return embeddingHTTPResponse(request, status, string(jsonBytes(envelope))), nil
}

// fakeProviderNativeToolCalls keeps the scripted router's public input
// convenient: tests may use the canonical call_id/capability_name/arguments
// sidecar shape, while the response still travels over the OpenAI-compatible
// native tool_calls wire that Eino's decoder understands.
func fakeProviderNativeToolCalls(calls []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		if len(call) == 0 {
			result = append(result, call)
			continue
		}
		if len(mapValue(call["function"])) > 0 {
			result = append(result, call)
			continue
		}
		name := firstString(call["capability_name"], firstString(call["name"], ""))
		arguments := call["arguments"]
		if text := stringValue(arguments); text != "" {
			arguments = text
		} else if arguments != nil {
			arguments = jsonString(arguments)
		}
		result = append(result, map[string]any{
			"id":       firstString(call["call_id"], stringValue(call["id"])),
			"type":     firstString(call["type"], "function"),
			"function": map[string]any{"name": name, "arguments": arguments},
		})
	}
	return result
}

// payloads returns every captured request payload whose schema matches.
func (router *fakeProviderRouter) payloads(schemaName string) []map[string]any {
	router.mu.Lock()
	defer router.mu.Unlock()
	result := make([]map[string]any, 0, len(router.requests))
	for _, payload := range router.requests {
		if providerWireSchemaName(payload) == schemaName {
			result = append(result, payload)
		}
	}
	return result
}

// requestCount reports how many requests matched one schema name. It only counts
// requests that matched a script, so a test that wants to prove "no model call
// happened" must script every role the turn can reach (which also makes the
// forbidden count authoritative).
func (router *fakeProviderRouter) requestCount(schemaName string) int {
	router.mu.Lock()
	defer router.mu.Unlock()
	return router.calls[schemaName]
}

// logicalRequestCount separates one logical ADK stage from its physical model
// rounds. A follow-up request that carries a tool result is still part of the
// same stage; requestCount remains available for physical-attempt and wire
// order assertions.
func (router *fakeProviderRouter) logicalRequestCount(schemaName string) int {
	router.mu.Lock()
	defer router.mu.Unlock()
	count := 0
	for _, payload := range router.requests {
		if providerWireSchemaName(payload) != schemaName || payloadHasToolResult(payload) {
			continue
		}
		count++
	}
	return count
}

func payloadHasToolResult(payload map[string]any) bool {
	for _, raw := range arrayValue(payload["messages"]) {
		if stringValue(mapValue(raw)["role"]) == "tool" {
			return true
		}
	}
	return false
}

// totalRequests reports every HTTP round trip the router saw, scripted or not.
// It is the physical-attempt counter that the logical-stage counters are
// compared against (F10).
func (router *fakeProviderRouter) totalRequests() int {
	router.mu.Lock()
	defer router.mu.Unlock()
	return len(router.requests)
}

// schemaSequence reports the response-schema name of every HTTP round trip in
// the order it was issued. A per-role total cannot express ORDER or repetition;
// the budget contract is "A, then optionally a Judge, then optionally B, and
// never a stage twice", so the sequence is what has to be asserted (F10).
func (router *fakeProviderRouter) schemaSequence() []string {
	router.mu.Lock()
	defer router.mu.Unlock()
	result := make([]string, 0, len(router.requests))
	for _, payload := range router.requests {
		result = append(result, providerWireSchemaName(payload))
	}
	return result
}

// unattributedRequests counts HTTP round trips whose schema never matched a
// script. A physical attempt that maps to no logical stage would otherwise be
// silently invisible to the per-stage counters.
func (router *fakeProviderRouter) unattributedRequests() int {
	router.mu.Lock()
	defer router.mu.Unlock()
	total := 0
	for _, payload := range router.requests {
		if _, scripted := router.routes[providerWireSchemaName(payload)]; !scripted && router.fallback == nil {
			total++
		}
	}
	return total
}

func providerWireSchemaName(payload map[string]any) string {
	responseFormat := mapValue(payload["response_format"])
	return stringValue(mapValue(responseFormat["json_schema"])["name"])
}

// ---------------------------------------------------------------------------
// Wire capture
// ---------------------------------------------------------------------------

// wireCaptureTransport records the exact JSON body of every Provider request
// before delegating to the inner transport.
type wireCaptureTransport struct {
	inner http.RoundTripper

	mu      sync.Mutex
	raw     [][]byte
	decoded []map[string]any
}

func captureProviderWirePayload(inner http.RoundTripper) *wireCaptureTransport {
	return &wireCaptureTransport{inner: inner}
}

func (capture *wireCaptureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var body []byte
	if request.Body != nil {
		read, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		body = read
		request.Body = io.NopCloser(bytes.NewReader(body))
	}
	capture.mu.Lock()
	capture.raw = append(capture.raw, body)
	var decoded map[string]any
	if json.Unmarshal(body, &decoded) == nil {
		capture.decoded = append(capture.decoded, decoded)
	}
	capture.mu.Unlock()
	return capture.inner.RoundTrip(request)
}

// payloadsForSchema returns the captured decoded payloads for one schema name.
func (capture *wireCaptureTransport) payloadsForSchema(schemaName string) []map[string]any {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	result := make([]map[string]any, 0, len(capture.decoded))
	for _, payload := range capture.decoded {
		if providerWireSchemaName(payload) == schemaName {
			result = append(result, payload)
		}
	}
	return result
}

// count returns how many request bodies were captured.
func (capture *wireCaptureTransport) count() int {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return len(capture.raw)
}

// ---------------------------------------------------------------------------
// App assembly and clock
// ---------------------------------------------------------------------------

// newTestApp converges the canonical App wiring used by integration tests. A nil
// transport leaves the Provider without an HTTP client, which is what tests that
// never reach the model want.
func newTestApp(t *testing.T, repository *PostgresRepository, transport http.RoundTripper) *App {
	t.Helper()
	app := &App{DB: repository}
	app.Provider = &ProviderClient{DB: repository}
	if transport != nil {
		app.Provider.HTTP = &http.Client{Transport: transport}
	}
	app.SchedulePlanner = providerSchedulePlanner{provider: app.Provider, runner: app}
	if repository != nil && repository.Pool() != nil {
		app.Schedule = NewScheduleService(repository.Pool(), app.SchedulePlanner, app)
		app.Memory = NewMemoryService(repository.Pool(), app)
		app.Media = NewMediaService(repository.Pool(), nil, "", app)
		app.Cognition = NewCognitionService(repository.Pool(), app)
		app.Reflection = NewReflectionService(repository.Pool(), app)
		app.LifeContext = NewLifeContextService(repository.Pool(), app)
	}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	return app
}

// fixedClock pins the application clock so settlement ordering is deterministic.
func fixedClock(instant time.Time) func() time.Time {
	return func() time.Time { return instant }
}

// seedTurnConversation prepares everything a full HandleTurn call needs: actors,
// fluctlight lifecycle rows, a conversation with a head and both participants.
func seedTurnConversation(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID string) {
	t.Helper()
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'turn regression')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
}

// seedCognitiveProviderRole registers the model binding the Main turn resolves
// through. The Judge reuses the same binding, so one row is enough.
func seedCognitiveProviderRole(t *testing.T, ctx context.Context, repository *PostgresRepository, endpointID string) {
	t.Helper()
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://turn.invalid','turn-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'turn-model','structured_output,tool_calling',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
}
