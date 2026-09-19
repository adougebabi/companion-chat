package core

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func TestEinoFactoryUsesOfficialChatModelAndPreservesToolCalls(t *testing.T) {
	var mu sync.Mutex
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"memory.recall","arguments":"{\"intent\":\"recent\"}"}}]}}],"usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14}}`))
	}))
	defer server.Close()

	factory := NewEinoModelFactory(server.Client())
	chat, err := factory.NewChatModel(context.Background(), EinoModelConfig{BaseURL: server.URL, Model: "fake", HTTPClient: server.Client(), MaxCompletionTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	bound, err := chat.WithTools([]*schema.ToolInfo{{Name: "memory.recall", Desc: "Recall bounded memory"}})
	if err != nil {
		t.Fatal(err)
	}
	message, err := bound.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")})
	if err != nil {
		t.Fatal(err)
	}
	if len(message.ToolCalls) != 1 || message.ToolCalls[0].Function.Name != "memory.recall" {
		t.Fatalf("tool calls = %#v", message.ToolCalls)
	}
	if got := payload["model"]; got != "fake" {
		t.Fatalf("model payload = %#v", got)
	}
}

func TestEinoFactoryUsesOfficialEmbedder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer server.Close()
	factory := NewEinoModelFactory(server.Client())
	embedder, err := factory.NewEmbedder(context.Background(), EinoModelConfig{BaseURL: server.URL, Model: "embedding", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := embedder.EmbedStrings(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 1 || len(vectors[0]) != 3 || math.Abs(vectors[0][1]-0.2) > 1e-6 {
		t.Fatalf("vectors = %#v", vectors)
	}
}

func TestStreamWithEinoAggregatesChunksAndPropagatesCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer server.Close()
	p := &ProviderClient{HTTP: server.Client()}
	var chunks []string
	text, err := p.streamWithEino(context.Background(), providerAssignment{BaseURL: server.URL, ModelID: "fake", Timeout: 5 * time.Second, TokenBudget: 64}, []map[string]any{{"role": "user", "content": "hello"}}, "stream-request", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" || len(chunks) != 2 {
		t.Fatalf("text=%q chunks=%#v", text, chunks)
	}
}

type adkFakeChatModel struct {
	mu    sync.Mutex
	calls int
}

type adkErrorChatModel struct{}

func (adkErrorChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("fake_model_failed")
}
func (adkErrorChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("fake_stream_failed")
}
func (m adkErrorChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type adkLoopChatModel struct{}

func (adkLoopChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("", []schema.ToolCall{{ID: "loop-call", Type: "function", Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"loop"}`}}}), nil
}
func (adkLoopChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("", nil)}), nil
}
func (m adkLoopChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *adkFakeChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	for _, message := range input {
		if message != nil && message.Role == schema.Tool {
			return schema.AssistantMessage("tool result received", nil), nil
		}
	}
	return schema.AssistantMessage("", []schema.ToolCall{{ID: "call-1", Type: "function", Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"recent"}`}}}), nil
}

func (m *adkFakeChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("stream", nil)}), nil
}

func (m *adkFakeChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type adkFakeInvoker struct {
	mu    sync.Mutex
	calls []string
}

type adkNoIdentityInvoker struct{}

func (adkNoIdentityInvoker) Execute(context.Context, string, string) (string, error) {
	return `{}`, nil
}

type adkFailingInvoker struct{}

func (adkFailingInvoker) Execute(context.Context, string, string) (string, error) {
	return "", errors.New("fake_tool_failed")
}

func (adkFailingInvoker) ExecuteWithID(ctx context.Context, _ string, capabilityName, argumentsJSON string) (string, error) {
	return adkFailingInvoker{}.Execute(ctx, capabilityName, argumentsJSON)
}

type adkTraceInvoker struct{ trace *ADKCapabilityTrace }

func (i adkTraceInvoker) Execute(_ context.Context, capabilityName string, argumentsJSON string) (string, error) {
	return i.ExecuteWithID(context.Background(), "trace-call", capabilityName, argumentsJSON)
}

func (i adkTraceInvoker) ExecuteWithID(ctx context.Context, callID, capabilityName string, argumentsJSON string) (string, error) {
	i.trace.Invocations = append(i.trace.Invocations, CapabilityInvocation{CallID: callID, CapabilityName: capabilityName, Arguments: json.RawMessage(argumentsJSON), SchemaVersion: CapabilityInvocationSchemaVersion, SourceFactID: "source", ProviderRequestID: "provider:" + callID, Metadata: InvocationMetadata{CorrelationID: providerCorrelation(ctx), Source: "model_tool"}})
	i.trace.Results = append(i.trace.Results, CapabilityResult{CallID: callID, CapabilityName: capabilityName, Status: "completed", Output: map[string]any{"items": []any{"recent"}}})
	return `{"items":["recent"]}`, nil
}

func (i *adkFakeInvoker) Execute(_ context.Context, capabilityName string, argumentsJSON string) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.calls = append(i.calls, capabilityName+":"+argumentsJSON)
	return `{"items":["recent"]}`, nil
}

func (i *adkFakeInvoker) ExecuteWithID(ctx context.Context, callID, capabilityName string, argumentsJSON string) (string, error) {
	if strings.TrimSpace(callID) == "" {
		return "", errors.New("missing_call_id")
	}
	return i.Execute(ctx, capabilityName, argumentsJSON)
}

func TestRunADKConversationExecutesCapabilityAndFeedsResultBack(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall bounded memory", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	invoker := &adkFakeInvoker{}
	tools, err := NewADKCapabilityTools(defs, invoker)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunADKConversation(context.Background(), ADKConversationConfig{
		Name: "conversation", Description: "Direct conversation runtime", Instruction: "Use the installed capabilities.",
		Model: &adkFakeChatModel{}, Tools: tools, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("find recent memory")})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalMessage == nil || result.FinalMessage.Content != "tool result received" {
		t.Fatalf("final = %#v", result.FinalMessage)
	}
	if len(result.ToolCalls) != 1 || len(result.ToolResults) != 1 || len(invoker.calls) != 1 {
		t.Fatalf("ADK trace calls=%#v results=%#v invocations=%#v", result.ToolCalls, result.ToolResults, invoker.calls)
	}
}

func TestNewADKCapabilityToolsRejectsInvokerWithoutFormalIdentity(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall bounded memory", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	if _, err := NewADKCapabilityTools(defs, adkNoIdentityInvoker{}); err == nil || err.Error() != "adk_tool_call_identity_invoker_required" {
		t.Fatalf("constructor error = %v", err)
	}
}

func TestRunADKConversationRejectsIterationLimitAboveTwo(t *testing.T) {
	_, err := RunADKConversation(context.Background(), ADKConversationConfig{
		Name: "conversation", Description: "test", Model: &adkFakeChatModel{}, MaxIterations: 3,
	}, []*schema.Message{schema.UserMessage("hello")})
	if err == nil || err.Error() != "adk_iteration_limit_invalid" {
		t.Fatalf("err = %v", err)
	}
}

func TestRunADKConversationModelFailureHasNoFinalMessage(t *testing.T) {
	_, err := RunADKConversation(context.Background(), ADKConversationConfig{
		Name: "conversation", Description: "test", Model: adkErrorChatModel{}, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("hello")})
	if err == nil || !strings.Contains(err.Error(), "fake_model_failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunADKConversationToolFailureHasNoFinalMessage(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	tools, err := NewADKCapabilityTools(defs, adkFailingInvoker{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunADKConversation(context.Background(), ADKConversationConfig{
		Name: "conversation", Description: "test", Model: &adkFakeChatModel{}, Tools: tools, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("tool failure")})
	if err == nil || !strings.Contains(err.Error(), "fake_tool_failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunADKConversationStopsLoopAtTwoGenerations(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	tools, err := NewADKCapabilityTools(defs, &adkFakeInvoker{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunADKConversation(context.Background(), ADKConversationConfig{
		Name: "conversation", Description: "test", Model: adkLoopChatModel{}, Tools: tools, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("loop")})
	if err == nil || !strings.Contains(err.Error(), "max iterations") {
		t.Fatalf("err = %v", err)
	}
}

func TestAppADKInvokerPreservesFormalToolCallIDAndSource(t *testing.T) {
	app := &App{}
	registry, err := NewCapabilityRegistry(builtinCapabilities(app)...)
	if err != nil {
		t.Fatal(err)
	}
	app.Capabilities = registry
	trace := &ADKCapabilityTrace{}
	invoker := newAppADKCapabilityInvoker(app, "fluctlight-1", "conversation-1", "fact-1", "frozen-1", ContextProjection{}, trace)
	identityInvoker, ok := invoker.(ADKCapabilityInvokerWithID)
	if !ok {
		t.Fatal("production invoker does not preserve tool-call identity")
	}
	if _, err := identityInvoker.ExecuteWithID(context.Background(), "provider-call-7", "conversation.reply", `{"text":"hello"}`); err != nil {
		t.Fatal(err)
	}
	if len(trace.Invocations) != 1 || trace.Invocations[0].CallID != "provider-call-7" || trace.Invocations[0].Metadata.Source != "model_tool" {
		t.Fatalf("trace = %#v", trace)
	}
}

func TestHandleTurnProductionADKToolLoopSettlesOneAssistant(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "adk-e2e-owner", "adk-e2e-fluctlight", "adk-e2e-conversation"
	seedTurnConversation(t, ctx, repository, ownerID, fluctlightID, conversationID)
	seedCognitiveProviderRole(t, ctx, repository, "adk-e2e-endpoint")
	callCount := 0
	finalDecision := map[string]any{
		"action_type": "reply", "response_mode": "final", "visible_text": "ADK settled",
		"response_intent": "answer", "capability_invocations": []any{}, "influences": []any{},
		"appraisal": map[string]any{"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.5, "social_threat": 0.0, "controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.5, "expected_effect": 0.5, "evidence_refs": []any{}, "event_kind": "conversation", "direction": "mixed", "drive_signals": []any{}},
	}
	router := newFakeProviderRouter().on("conversation_turn_response", func(_ map[string]any) fakeProviderResult {
		callCount++
		if callCount == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{{"id": "adk-production-call", "type": "function", "function": map[string]any{"name": "conversation.reply", "arguments": `{"text":"deferred"}`}}}}
		}
		return fakeProviderResult{Structured: finalDecision}
	})
	app := newTestApp(t, repository, router)
	result, err := app.HandleTurn(ctx, ownerID, conversationID, map[string]any{"fluctlight_id": fluctlightID, "text": "hello", "idempotency_key": "adk-e2e-idempotency", "turn_id": "adk-e2e-turn", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(result.Assistant["text"]) != "ADK settled" || router.requestCount("conversation_turn_response") != 2 {
		t.Fatalf("result=%#v requests=%d", result, router.requestCount("conversation_turn_response"))
	}
	var assistantCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID).Scan(&assistantCount); err != nil {
		t.Fatal(err)
	}
	if assistantCount != 1 {
		t.Fatalf("assistant count = %d", assistantCount)
	}
	second := router.payloads("conversation_turn_response")
	if len(second) != 2 {
		t.Fatalf("payload count = %d", len(second))
	}
	toolResultFound := false
	for _, message := range arrayValue(second[1]["messages"]) {
		if stringValue(mapValue(message)["role"]) == "tool" && stringValue(mapValue(message)["tool_call_id"]) == "adk-production-call" {
			toolResultFound = true
		}
	}
	if !toolResultFound {
		t.Fatalf("production ADK second request missing matching tool result: %#v", second[1]["messages"])
	}
}

func TestProviderConversationUsesADKLoopAndReturnsCanonicalTrace(t *testing.T) {
	var mu sync.Mutex
	requests := make([]map[string]any, 0, 2)
	requestIDs := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		mu.Lock()
		requests = append(requests, payload)
		requestIDs = append(requestIDs, r.Header.Get("Idempotency-Key"))
		count := len(requests)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"provider-call-1","type":"function","function":{"name":"memory.recall","arguments":"{\"intent\":\"recent\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"visible_text\":\"found\",\"response_mode\":\"final\",\"capability_invocations\":[]}"}}]}`))
	}))
	defer server.Close()

	trace := &ADKCapabilityTrace{}
	invoker := adkTraceInvoker{trace: trace}
	ctx := WithADKCapabilityInvoker(context.Background(), invoker, trace)
	p := &ProviderClient{HTTP: server.Client()}
	response, err := p.generateWithEino(ctx, EinoModelCall{
		Assignment:  providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 128},
		Messages:    []map[string]any{{"role": "system", "content": "protocol"}, {"role": "user", "content": "find"}},
		Definitions: []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}},
		JSONMode:    true, SchemaName: "conversation_turn_response", ResponseSchema: map[string]any{"type": "object"}, EnableThinking: true, ProviderRequestID: "provider-request-1", CorrelationID: "turn:adk-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Message == nil || len(response.Message.ToolCalls) != 1 || response.Message.ToolCalls[0].ID != "provider-call-1" || len(trace.Invocations) != 1 || trace.Invocations[0].CallID != "provider-call-1" || len(trace.Results) != 1 {
		t.Fatalf("response=%#v trace=%#v", response.Message, trace)
	}
	if response.Message.Content == "" || response.Message.Content == "{}" {
		t.Fatalf("final content = %q", response.Message.Content)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("ADK requests = %d", len(requests))
	}
	if requestIDs[0] == "" || requestIDs[0] == requestIDs[1] {
		t.Fatalf("ADK request identities = %#v", requestIDs)
	}
	secondMessages := arrayValue(requests[1]["messages"])
	foundToolResult := false
	foundMatchingAssistantCall := false
	for _, item := range secondMessages {
		message := mapValue(item)
		if stringValue(message["role"]) == "tool" && stringValue(message["tool_call_id"]) == "provider-call-1" {
			foundToolResult = true
		}
		if stringValue(message["role"]) == "assistant" {
			for _, call := range arrayValue(message["tool_calls"]) {
				if stringValue(mapValue(call)["id"]) == "provider-call-1" {
					foundMatchingAssistantCall = true
				}
			}
		}
	}
	if !foundToolResult || !foundMatchingAssistantCall {
		t.Fatalf("second request did not contain tool result: %#v", secondMessages)
	}
}

func TestProviderConversationWithoutToolsStillUsesADKRunner(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		defer r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"visible_text\":\"direct\",\"response_mode\":\"final\"}"}}]}`))
	}))
	defer server.Close()

	trace := &ADKCapabilityTrace{}
	ctx := WithADKCapabilityInvoker(context.Background(), adkTraceInvoker{trace: trace}, trace)
	p := &ProviderClient{HTTP: server.Client()}
	response, err := p.generateWithEino(ctx, EinoModelCall{
		Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 128},
		Role:       "cognitive_assessment", Scenario: "cognitive_assessment",
		Messages: []map[string]any{{"role": "user", "content": "hello"}},
		JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: map[string]any{"type": "object"},
		ProviderRequestID: "provider-no-tool", CorrelationID: "turn:no-tool",
	})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || response.Message == nil || response.Message.Content == "" {
		t.Fatalf("requests=%d response=%#v", requests, response.Message)
	}
	if len(trace.Invocations) != 0 || len(trace.Results) != 0 {
		t.Fatalf("unexpected no-tool trace: %#v", trace)
	}
}

func TestProviderTakeoverReplyUsesADKLoopAndPreservesTrace(t *testing.T) {
	var mu sync.Mutex
	requests := make([]map[string]any, 0, 2)
	requestIDs := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		mu.Lock()
		requests = append(requests, payload)
		requestIDs = append(requestIDs, r.Header.Get("Idempotency-Key"))
		count := len(requests)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"b-provider-call-1","type":"function","function":{"name":"memory.recall","arguments":"{\"intent\":\"recent\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"visible_text\":\"takeover found\",\"response_mode\":\"final\"}"}}]}`))
	}))
	defer server.Close()

	trace := &ADKCapabilityTrace{}
	invoker := adkTraceInvoker{trace: trace}
	ctx := WithADKCapabilityInvoker(WithProviderCorrelation(WithProviderScenario(context.Background(), "takeover_reply"), "takeover-reply:frozen-1"), invoker, trace)
	p := &ProviderClient{HTTP: server.Client()}
	response, err := p.generateWithEino(ctx, EinoModelCall{
		Assignment:  providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 128},
		Role:        "cognitive_assessment",
		Scenario:    "takeover_reply",
		Messages:    []map[string]any{{"role": "system", "content": "takeover protocol"}, {"role": "user", "content": "reply"}},
		Definitions: []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}},
		JSONMode:    true, SchemaName: takeoverReplySchemaName, ResponseSchema: map[string]any{"type": "object"},
		ProviderRequestID: "provider-takeover-request", CorrelationID: "takeover-reply:frozen-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Message == nil || response.Message.Content == "" || len(response.Message.ToolCalls) != 1 || response.Message.ToolCalls[0].ID != "b-provider-call-1" {
		t.Fatalf("response=%#v", response.Message)
	}
	if len(trace.Invocations) != 1 || trace.Invocations[0].CallID != "b-provider-call-1" || trace.Invocations[0].Metadata.Source != "model_tool" || trace.Invocations[0].Metadata.CorrelationID != "takeover-reply:frozen-1" || trace.Invocations[0].ProviderRequestID == "" || len(trace.Results) != 1 || trace.Results[0].CallID != "b-provider-call-1" {
		t.Fatalf("trace=%#v", trace)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 || requestIDs[0] == "" || requestIDs[0] == requestIDs[1] {
		t.Fatalf("ADK requests=%d identities=%#v", len(requests), requestIDs)
	}
	secondMessages := arrayValue(requests[1]["messages"])
	var assistantCall, toolResult bool
	for _, item := range secondMessages {
		message := mapValue(item)
		switch stringValue(message["role"]) {
		case "assistant":
			for _, call := range arrayValue(message["tool_calls"]) {
				if stringValue(mapValue(call)["id"]) == "b-provider-call-1" {
					assistantCall = true
				}
			}
		case "tool":
			if stringValue(message["tool_call_id"]) == "b-provider-call-1" && stringValue(message["name"]) == "memory.recall" {
				toolResult = true
			}
		}
	}
	if !assistantCall || !toolResult {
		t.Fatalf("takeover ADK second request lost call/result pair: %#v", secondMessages)
	}
}

func TestPromptComposerUsesExplicitSlotsAndPreservesWholeTurns(t *testing.T) {
	composer, err := NewPromptComposer(DefaultPromptBudgetPolicy(4096))
	if err != nil {
		t.Fatal(err)
	}
	result, err := composer.Compose(context.Background(), PromptCompositionInput{
		System: "stable protocol", CurrentInput: "new input",
		Slots: []PromptSlot{
			{ID: PromptSlotRecentTurns, Position: PromptSlotRecent, Order: 2, Fragments: []PromptFragment{
				{Kind: PromptFragmentRecentMessage, GroupKey: "turn-1", Content: map[string]any{"role": "user", "content": "old input"}},
				{Kind: PromptFragmentRecentMessage, GroupKey: "turn-1", Content: map[string]any{"role": "assistant", "content": "old answer"}},
			}},
			{ID: PromptSlotCurrent, Position: PromptSlotCurrentInput, Order: 3, Required: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 4 || result.Messages[0].Role != schema.System || result.Messages[len(result.Messages)-1].Content != "new input" {
		t.Fatalf("messages = %#v", result.Messages)
	}
}

func TestProviderMessagesToEinoPreservesMultimodalParts(t *testing.T) {
	messages, err := providerMessagesToEino([]map[string]any{{
		"role": "user",
		"content": []any{
			map[string]any{"type": "text", "text": "inspect"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,abc", "detail": "high"}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || len(messages[0].UserInputMultiContent) != 2 || messages[0].UserInputMultiContent[1].Image == nil {
		t.Fatalf("messages = %#v", messages)
	}
	if got := *messages[0].UserInputMultiContent[1].Image.URL; got != "data:image/png;base64,abc" {
		t.Fatalf("image URL = %q", got)
	}
}

func TestProviderMessagesToEinoRejectsUnknownMultimodalPart(t *testing.T) {
	_, err := providerMessagesToEino([]map[string]any{{"role": "user", "content": []any{map[string]any{"type": "audio"}}}})
	if err == nil || !strings.Contains(err.Error(), "unsupported_part_type") {
		t.Fatalf("err = %v", err)
	}
}

func TestPromptComposerSlotBudgetAndCancellationFailClosed(t *testing.T) {
	composer, err := NewPromptComposer(DefaultPromptBudgetPolicy(4096))
	if err != nil {
		t.Fatal(err)
	}
	optional, err := composer.Compose(context.Background(), PromptCompositionInput{
		System: "protocol", CurrentInput: "now",
		Slots: []PromptSlot{{ID: PromptSlotRuntimeFact, Position: PromptSlotRuntime, Order: 1, BudgetTokens: 1, Fragments: []PromptFragment{{Kind: PromptFragmentRuntimeFact, Content: map[string]any{"fact": strings.Repeat("x", 200)}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(optional.Messages) != 2 || strings.Contains(optional.Messages[1].Content, "fact") {
		t.Fatalf("optional slot was not dropped: %#v", optional.Messages)
	}
	_, err = composer.Compose(context.Background(), PromptCompositionInput{
		System: "protocol", CurrentInput: "now",
		Slots: []PromptSlot{{ID: PromptSlotRuntimeFact, Position: PromptSlotRuntime, Order: 1, Required: true, BudgetTokens: 1, Fragments: []PromptFragment{{Kind: PromptFragmentRuntimeFact, Required: true, Content: map[string]any{"fact": strings.Repeat("x", 200)}}}}},
	})
	if err == nil || !strings.Contains(err.Error(), "prompt_required_budget_exceeded") {
		t.Fatalf("required overflow err = %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = composer.Compose(cancelled, PromptCompositionInput{System: "protocol", CurrentInput: "now"})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err = %v", err)
	}
}

var _ tool.BaseTool = (*adkCapabilityTool)(nil)
