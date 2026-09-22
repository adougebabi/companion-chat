package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestEinoNativeToolCallsRequireFormalIdentityAndObjectArguments(t *testing.T) {
	valid := schema.AssistantMessage("", []schema.ToolCall{{
		ID: "call-native-1", Type: "function",
		Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"recent"}`},
	}})
	calls, err := normalizeEinoNativeToolCalls(valid, "provider:request-1")
	if err != nil {
		t.Fatalf("normalize native calls: %v", err)
	}
	if len(calls) != 1 || calls[0].CallID != "call-native-1" || calls[0].CapabilityName != "memory.recall" {
		t.Fatalf("native call = %#v", calls)
	}

	missingID := schema.AssistantMessage("", []schema.ToolCall{{
		Type:     "function",
		Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"recent"}`},
	}})
	if _, err := normalizeEinoNativeToolCalls(missingID, "provider:request-1"); err == nil {
		t.Fatal("missing native ToolCall ID was accepted")
	}

	malformedArguments := schema.AssistantMessage("", []schema.ToolCall{{
		ID: "call-native-2", Type: "function",
		Function: schema.FunctionCall{Name: "memory.recall", Arguments: `[]`},
	}})
	if _, err := normalizeEinoNativeToolCalls(malformedArguments, "provider:request-1"); err == nil {
		t.Fatal("non-object native ToolCall arguments were accepted")
	}
}

func TestEinoADKNativeToolCallsDoNotReadStructuredSidecar(t *testing.T) {
	message := schema.AssistantMessage(`{"tool_calls":[{"id":"sidecar","name":"conversation.reply","arguments":{"text":"forged"}}]}`, []schema.ToolCall{{
		ID: "call-native-3", Type: "function",
		Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"recent"}`},
	}})
	calls, err := normalizeEinoNativeToolCalls(message, "provider:request-2")
	if err != nil {
		t.Fatalf("normalize native calls: %v", err)
	}
	if len(calls) != 1 || calls[0].CallID != "call-native-3" || calls[0].CapabilityName != "memory.recall" {
		t.Fatalf("sidecar changed native authority: %#v", calls)
	}
}

func TestEinoNativeToolCallNormalizationKeepsValidSiblingAndRejectsMalformedSibling(t *testing.T) {
	message := schema.AssistantMessage("", []schema.ToolCall{
		{ID: "call-valid", Type: "function", Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"recent"}`}},
		{Type: "function", Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"broken"}`}},
	})
	calls, err := normalizeEinoNativeToolCallsIndependently(message, "provider:request-3")
	if err == nil {
		t.Fatal("malformed sibling did not produce a bounded error")
	}
	if len(calls) != 1 || calls[0].CallID != "call-valid" {
		t.Fatalf("valid sibling was not preserved: calls=%#v err=%v", calls, err)
	}
}

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

type adkLoopChatModel struct {
	mu    sync.Mutex
	calls int
}

type adkCancellationChatModel struct{}

type adkTextThenEmptyChatModel struct {
	mu    sync.Mutex
	calls int
}

type adkToolThenErrorChatModel struct {
	mu    sync.Mutex
	calls int
}

type adkScriptedChatModel struct {
	mu        sync.Mutex
	responses []*schema.Message
	inputs    [][]*schema.Message
}

func (m *adkScriptedChatModel) next(input []*schema.Message) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inputs = append(m.inputs, append([]*schema.Message(nil), input...))
	index := len(m.inputs) - 1
	if index >= len(m.responses) {
		return nil, errors.New("unexpected_model_call")
	}
	return m.responses[index], nil
}

func (m *adkScriptedChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return m.next(input)
}

func (m *adkScriptedChatModel) Stream(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	message, err := m.next(input)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{message}), nil
}

func (m *adkScriptedChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type adkResultInvoker struct {
	mu      sync.Mutex
	results map[string]string
	calls   []string
}

func (i *adkResultInvoker) Execute(ctx context.Context, capabilityName string, argumentsJSON string) (string, error) {
	return i.ExecuteWithID(ctx, "missing-formal-id", capabilityName, argumentsJSON)
}

func (i *adkResultInvoker) ExecuteWithID(_ context.Context, callID, _ string, _ string) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.calls = append(i.calls, callID)
	if result, ok := i.results[callID]; ok {
		return result, nil
	}
	return `{"status":"completed"}`, nil
}

func (m *adkToolThenErrorChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.calls++
	call := m.calls
	m.mu.Unlock()
	if call == 1 {
		return schema.AssistantMessage("", []schema.ToolCall{{ID: "tool-then-error", Type: "function", Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"recent"}`}}}), nil
	}
	return nil, errors.New("second_model_call_failed")
}

func (m *adkToolThenErrorChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("second_stream_call_failed")
}

func (m *adkToolThenErrorChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *adkTextThenEmptyChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.calls++
	call := m.calls
	m.mu.Unlock()
	if call == 1 {
		return schema.AssistantMessage("candidate before tool", []schema.ToolCall{{ID: "reply-tool-1", Type: "function", Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"recent"}`}}}), nil
	}
	return schema.AssistantMessage("", nil), nil
}

func (m *adkTextThenEmptyChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("", nil)}), nil
}

func (m *adkTextThenEmptyChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (adkCancellationChatModel) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (adkCancellationChatModel) Stream(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (m adkCancellationChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *adkLoopChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.calls++
	callID := fmt.Sprintf("loop-call-%d", m.calls)
	m.mu.Unlock()
	return schema.AssistantMessage("", []schema.ToolCall{{ID: callID, Type: "function", Function: schema.FunctionCall{Name: "memory.recall", Arguments: `{"intent":"loop"}`}}}), nil
}
func (*adkLoopChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("", nil)}), nil
}
func (m *adkLoopChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
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

func TestRunADKLoopExecutesCapabilityAndFeedsResultBack(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall bounded memory", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	invoker := &adkFakeInvoker{}
	tools, err := NewADKCapabilityTools(defs, invoker)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunADKLoop(context.Background(), ADKLoopConfig{
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
	if result.FinalMessage.Role != schema.Assistant || result.ToolResults[0].Role != schema.Tool {
		t.Fatalf("tool result became visible final message: final=%#v results=%#v", result.FinalMessage, result.ToolResults)
	}
}

func TestNewADKCapabilityToolsRejectsInvokerWithoutFormalIdentity(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall bounded memory", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	if _, err := NewADKCapabilityTools(defs, adkNoIdentityInvoker{}); err == nil || err.Error() != "adk_tool_call_identity_invoker_required" {
		t.Fatalf("constructor error = %v", err)
	}
}

func TestRunADKLoopContinuesAcrossBusinessFailureForThreeModelDecisions(t *testing.T) {
	model := &adkScriptedChatModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{ID: "business-failed", Type: "function", Function: schema.FunctionCall{Name: "test.tool", Arguments: `{"step":1}`}}}),
		schema.AssistantMessage("", []schema.ToolCall{{ID: "business-recovery", Type: "function", Function: schema.FunctionCall{Name: "test.tool", Arguments: `{"step":2}`}}}),
		schema.AssistantMessage("recovered after tool feedback", nil),
	}}
	invoker := &adkResultInvoker{results: map[string]string{
		"business-failed":   `{"status":"rejected","error_code":"business_rule"}`,
		"business-recovery": `{"status":"completed","output":{"saved":true}}`,
	}}
	tools, err := NewADKCapabilityTools([]CapabilityDefinition{{Name: "test.tool", Description: "controlled tool"}}, invoker)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunADKLoop(context.Background(), ADKLoopConfig{
		Name: "conversation", Description: "native multi-round loop", Model: model, Tools: tools, MaxIterations: 3,
	}, []*schema.Message{schema.UserMessage("recover from the business rejection")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Iterations != 3 || result.FinalMessage == nil || result.FinalMessage.Content != "recovered after tool feedback" {
		t.Fatalf("result = %#v", result)
	}
	if len(result.ToolCalls) != 2 || len(result.ToolResults) != 2 || len(invoker.calls) != 2 {
		t.Fatalf("calls=%#v results=%#v invocations=%#v", result.ToolCalls, result.ToolResults, invoker.calls)
	}
	if result.ToolResults[0].ToolCallID != "business-failed" || !strings.Contains(result.ToolResults[0].Content, "business_rule") || result.ToolResults[1].ToolCallID != "business-recovery" {
		t.Fatalf("tool result association = %#v", result.ToolResults)
	}
	if len(model.inputs) != 3 || !containsToolResult(model.inputs[1], "business-failed", "business_rule") || !containsToolResult(model.inputs[2], "business-recovery", "saved") {
		t.Fatalf("model inputs did not receive ordered tool facts: %#v", model.inputs)
	}
}

func TestRunADKLoopPreservesSameRoundMultipleCallAssociation(t *testing.T) {
	model := &adkScriptedChatModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			{ID: "multi-a", Type: "function", Function: schema.FunctionCall{Name: "test.tool", Arguments: `{"item":"a"}`}},
			{ID: "multi-b", Type: "function", Function: schema.FunctionCall{Name: "test.tool", Arguments: `{"item":"b"}`}},
		}),
		schema.AssistantMessage("both results observed", nil),
	}}
	invoker := &adkResultInvoker{results: map[string]string{
		"multi-a": `{"status":"completed","value":"alpha"}`,
		"multi-b": `{"status":"completed","value":"beta"}`,
	}}
	tools, err := NewADKCapabilityTools([]CapabilityDefinition{{Name: "test.tool", Description: "controlled tool"}}, invoker)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunADKLoop(context.Background(), ADKLoopConfig{
		Name: "conversation", Description: "native parallel-call association", Model: model, Tools: tools, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("run both")})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ToolCalls) != 2 || len(result.ToolResults) != 2 || result.ToolResults[0].ToolCallID != "multi-a" || result.ToolResults[1].ToolCallID != "multi-b" {
		t.Fatalf("result association = %#v", result)
	}
	if len(model.inputs) != 2 || !containsToolResult(model.inputs[1], "multi-a", "alpha") || !containsToolResult(model.inputs[1], "multi-b", "beta") {
		t.Fatalf("second model input = %#v", model.inputs)
	}
}

func TestRunADKLoopStreamingUsesTheSameNativeToolLoop(t *testing.T) {
	model := &adkScriptedChatModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{ID: "stream-tool", Type: "function", Function: schema.FunctionCall{Name: "test.tool", Arguments: `{}`}}}),
		schema.AssistantMessage("streamed final", nil),
	}}
	invoker := &adkResultInvoker{results: map[string]string{"stream-tool": `{"status":"completed"}`}}
	tools, err := NewADKCapabilityTools([]CapabilityDefinition{{Name: "test.tool", Description: "controlled tool"}}, invoker)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunADKLoop(context.Background(), ADKLoopConfig{
		Name: "conversation", Description: "streaming native loop", Model: model, Tools: tools,
		MaxIterations: 2, EnableStreaming: true,
	}, []*schema.Message{schema.UserMessage("stream the loop")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Iterations != 2 || result.FinalMessage == nil || result.FinalMessage.Content != "streamed final" || len(result.ToolCalls) != 1 || len(result.ToolResults) != 1 {
		t.Fatalf("streaming result = %#v", result)
	}
}

func TestProviderFormalAgentStreamingUsesTheSameRunnerStream(t *testing.T) {
	var sawStream bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		sawStream = boolValue(payload["stream"])
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"answer\\\":\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"\\\"streamed\\\"}\",\"finish_reason\":\"stop\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	definition, _ := FormalAgentDefinitionByID(FormalAgentConversationCognition)
	trace := &ADKCapabilityTrace{}
	ctx := WithADKCapabilityInvoker(context.Background(), &adkFakeInvoker{}, trace)
	response, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, EinoModelCall{
		Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 5 * time.Second, TokenBudget: 64},
		Role:       "cognitive_assessment", Scenario: "cognitive_assessment", Messages: []map[string]any{{"role": "user", "content": "stream"}},
		JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: objectSchema(map[string]any{"answer": stringSchema()}, []string{"answer"}, false),
		ProviderRequestID: "formal-stream-request", CorrelationID: "formal-stream-run", Agent: definition, EnableStreaming: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawStream || response.Message == nil || response.Message.Content != `{"answer":"streamed"}` {
		t.Fatalf("stream=%t response=%#v", sawStream, response.Message)
	}
}

func containsToolResult(messages []*schema.Message, callID, content string) bool {
	for _, message := range messages {
		if message != nil && message.Role == schema.Tool && message.ToolCallID == callID && strings.Contains(message.Content, content) {
			return true
		}
	}
	return false
}

func TestRunADKLoopModelFailureHasNoFinalMessage(t *testing.T) {
	_, err := RunADKLoop(context.Background(), ADKLoopConfig{
		Name: "conversation", Description: "test", Model: adkErrorChatModel{}, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("hello")})
	if err == nil || !strings.Contains(err.Error(), "fake_model_failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunADKLoopToolFailureHasNoFinalMessage(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	tools, err := NewADKCapabilityTools(defs, adkFailingInvoker{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunADKLoop(context.Background(), ADKLoopConfig{
		Name: "conversation", Description: "test", Model: &adkFakeChatModel{}, Tools: tools, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("tool failure")})
	if err == nil || !strings.Contains(err.Error(), "fake_tool_failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunADKLoopToolSuccessThenModelErrorPreservesPartialResult(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	trace := &ADKCapabilityTrace{}
	tools, err := NewADKCapabilityTools(defs, adkTraceInvoker{trace: trace})
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunADKLoop(context.Background(), ADKLoopConfig{
		Name: "conversation", Description: "test", Model: &adkToolThenErrorChatModel{}, Tools: tools, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("tool then model failure")})
	if err == nil || !strings.Contains(err.Error(), "second_model_call_failed") {
		t.Fatalf("expected second model failure, got %v", err)
	}
	if len(result.ToolCalls) != 1 || len(result.ToolResults) != 1 || len(result.Messages) < 2 {
		t.Fatalf("partial ADK result lost after model failure: result=%#v", result)
	}
	if len(trace.Invocations) != 1 || len(trace.Results) != 1 || trace.Results[0].Status != "completed" {
		t.Fatalf("tool trace lost after model failure: trace=%#v", trace)
	}
}

func TestRunADKLoopMaxIterationsReturnsErrorWithPartialFacts(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	trace := &ADKCapabilityTrace{}
	tools, err := NewADKCapabilityTools(defs, adkTraceInvoker{trace: trace})
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunADKLoop(context.Background(), ADKLoopConfig{
		Name: "conversation", Description: "test", Model: &adkLoopChatModel{}, Tools: tools, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("loop")})
	if err == nil || !strings.Contains(err.Error(), "max iterations") {
		t.Fatalf("err = %v", err)
	}
	if result.Iterations != 2 || len(result.ToolCalls) != 2 || len(result.ToolResults) != 2 || result.FinalMessage == nil {
		t.Fatalf("partial loop facts = %#v", result)
	}
	if len(trace.Invocations) != 2 || len(trace.Results) != 2 {
		t.Fatalf("partial tool trace = %#v", trace)
	}
}

func TestRunADKLoopCancellationDoesNotFabricateFinalMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunADKLoop(ctx, ADKLoopConfig{
		Name: "background", Description: "cancellation test", Model: adkCancellationChatModel{}, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("cancel")})
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("cancellation err = %v", err)
	}
}

func TestRunADKLoopDoesNotReuseTextBeforeFinalEmptyAssistant(t *testing.T) {
	defs := []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}}
	tools, err := NewADKCapabilityTools(defs, &adkFakeInvoker{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunADKLoop(context.Background(), ADKLoopConfig{
		Name: "conversation", Description: "final empty regression", Model: &adkTextThenEmptyChatModel{}, Tools: tools, MaxIterations: 2,
	}, []*schema.Message{schema.UserMessage("reply")})
	if err == nil || !strings.Contains(err.Error(), "adk_final_message_empty") {
		t.Fatalf("expected final empty failure, got %v", err)
	}
}

func TestADKCapabilityDefinitionsValidateRegistrationWithoutExecutionStageGate(t *testing.T) {
	app := &App{}
	registry, err := NewCapabilityRegistry(builtinCapabilities(app)...)
	if err != nil {
		t.Fatal(err)
	}
	app.Capabilities = registry
	app.Provider = &ProviderClient{}

	conversationOnly, ok := registry.Definition("memory.recall")
	if !ok {
		t.Fatal("memory.recall definition missing")
	}
	// Catalog surface is an assembly default, not an execution permission.
	// An Agent can explicitly install any canonical public Tool and the Tool
	// still validates its actual resource ownership at execution.
	if err := validateADKCapabilityDefinitions(app, []CapabilityDefinition{conversationOnly}, CapabilitySurfaceWakeUp); err != nil {
		t.Fatalf("canonical Tool was gated by calling Agent surface: %v", err)
	}

	internal, ok := registry.Definition(personaSwitchCapabilityName)
	if !ok {
		t.Fatal("internal persona capability missing")
	}
	// Test genuinely private registration independently of persona business visibility.
	internal.InternalOnly = true
	registry.definitions[internal.Name] = internal
	if _, err := app.RunADKStructuredTask(context.Background(), ADKStructuredTaskInput{
		Role: "cognitive_assessment", Scenario: "wake_up", Definitions: []CapabilityDefinition{internal},
		SchemaName: "wake_up_response", Prompt: PromptAssemblyResult{ResponseFormat: wakeUpResponseSchema()}, Capability: &ADKCapabilityRequest{Surface: CapabilitySurfaceWakeUp},
	}); err == nil || !strings.Contains(err.Error(), "internal") {
		t.Fatalf("internal capability was accepted by WakeUp: %v", err)
	}

	unknown := conversationOnly
	unknown.Name = "not.registered"
	if err := validateADKCapabilityDefinitions(app, []CapabilityDefinition{unknown}, CapabilitySurfaceWakeUp); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown capability was accepted: %v", err)
	}

	mismatched := conversationOnly
	mismatched.Description = mismatched.Description + " changed"
	if err := validateADKCapabilityDefinitions(app, []CapabilityDefinition{mismatched}, CapabilitySurfaceConversation); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("mismatched capability was accepted: %v", err)
	}
}

func TestAppADKInvokerRejectsToolCallWithoutPhysicalProviderIdentity(t *testing.T) {
	app := &App{}
	registry, err := NewCapabilityRegistry(builtinCapabilities(app)...)
	if err != nil {
		t.Fatal(err)
	}
	app.Capabilities = registry
	trace := &ADKCapabilityTrace{}
	invoker := newAppADKCapabilityInvoker(app, ADKCapabilityRequest{
		FluctlightID: "fluctlight-1", ConversationID: "conversation-1", SourceFactID: "fact-1", ActionID: "frozen-1",
		Surface: CapabilitySurfaceConversation,
	}, trace)
	identityInvoker, ok := invoker.(ADKCapabilityInvokerWithID)
	if !ok {
		t.Fatal("production invoker does not preserve tool-call identity")
	}
	if _, err := identityInvoker.ExecuteWithID(context.Background(), "provider-call-7", "conversation.reply", `{"text":"hello"}`); err == nil || !strings.Contains(err.Error(), "provider_request_identity_missing") {
		t.Fatalf("missing physical request identity was accepted: %v", err)
	}
	if len(trace.Invocations) != 0 || len(trace.Results) != 0 {
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
		"action_type": "reply", "visible_text": "ADK settled",
		"response_intent": "answer", "influences": []any{},
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
	if stringValue(result.Assistant["text"]) != "deferred" || router.requestCount("conversation_turn_response") != 2 {
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
	if response.Message == nil || len(response.Message.ToolCalls) != 0 || len(response.ExecutedToolCalls) != 1 || response.ExecutedToolCalls[0].ID != "provider-call-1" || len(trace.Invocations) != 1 || trace.Invocations[0].CallID != "provider-call-1" || len(trace.Results) != 1 {
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

func TestProviderADKLoopContinuesAfterDeferredResultsAndPreservesEveryCall(t *testing.T) {
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		mu.Lock()
		requests++
		request := requests
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch request {
		case 1:
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"deferred-1","type":"function","function":{"name":"test.tool","arguments":"{\"step\":1}"}}]}}]}`))
		case 2:
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"deferred-2","type":"function","function":{"name":"test.tool","arguments":"{\"step\":2}"}}]}}]}`))
		default:
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"action_type\":\"no_op\",\"response_mode\":\"final\"}"}}]}`))
		}
	}))
	defer server.Close()

	invoker := &adkResultInvoker{results: map[string]string{
		"deferred-1": `{"status":"deferred","reason":"first_pending"}`,
		"deferred-2": `{"status":"deferred","reason":"second_pending"}`,
	}}
	trace := &ADKCapabilityTrace{}
	ctx := WithADKCapabilityInvoker(context.Background(), invoker, trace)
	p := &ProviderClient{HTTP: server.Client()}
	response, err := p.generateWithEino(ctx, EinoModelCall{
		Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 128},
		Role:       "cognitive_assessment", Scenario: "wake_up",
		Messages:    []map[string]any{{"role": "user", "content": "continue after every result"}},
		Definitions: []CapabilityDefinition{{Name: "test.tool", Description: "controlled deferred tool"}},
		JSONMode:    true, SchemaName: "wake_up_response", ResponseSchema: map[string]any{"type": "object"},
		ProviderRequestID: "provider-deferred-loop", CorrelationID: "wake:deferred-loop",
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	requestCount := requests
	mu.Unlock()
	if requestCount != 3 || response.Message == nil || len(response.Message.ToolCalls) != 0 || len(response.ExecutedToolCalls) != 2 {
		t.Fatalf("requests=%d response=%#v", requestCount, response.Message)
	}
	if response.ExecutedToolCalls[0].ID != "deferred-1" || response.ExecutedToolCalls[1].ID != "deferred-2" {
		t.Fatalf("intermediate calls were altered: %#v", response.ExecutedToolCalls)
	}
	if len(invoker.calls) != 2 || invoker.calls[0] != "deferred-1" || invoker.calls[1] != "deferred-2" {
		t.Fatalf("tool executions = %#v", invoker.calls)
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

func TestADKToolSuccessThenStructuredParseFailureKeepsTrace(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"parse-after-tool-1","type":"function","function":{"name":"memory.recall","arguments":"{\"intent\":\"recent\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write(jsonBytes(map[string]any{"choices": []any{map[string]any{
			"finish_reason": "stop",
			"message":       map[string]any{"role": "assistant", "content": `{"visible_text":`},
		}}}))
	}))
	defer server.Close()

	trace := &ADKCapabilityTrace{}
	ctx := WithADKCapabilityInvoker(
		WithProviderCorrelation(WithProviderScenario(context.Background(), "cognitive_assessment"), "turn:parse-after-tool"),
		adkTraceInvoker{trace: trace}, trace,
	)
	p := &ProviderClient{HTTP: server.Client()}
	response, err := p.generateWithEino(ctx, EinoModelCall{
		Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 128},
		Role:       "cognitive_assessment", Scenario: "cognitive_assessment",
		Messages:    []map[string]any{{"role": "system", "content": "protocol"}, {"role": "user", "content": "find"}},
		Definitions: []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}},
		JSONMode:    true, SchemaName: "conversation_turn_response", ResponseSchema: map[string]any{"type": "object"},
		ProviderRequestID: "provider-parse-after-tool", CorrelationID: "turn:parse-after-tool",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Message == nil || len(trace.Invocations) != 1 || len(trace.Results) != 1 || trace.Invocations[0].CallID != "parse-after-tool-1" || trace.Results[0].Status != "completed" {
		t.Fatalf("tool fact was lost before parse failure: response=%#v trace=%#v", response.Message, trace)
	}
	if parseErr := validateADKStructuredResponse(response.Message, "cognitive_assessment"); parseErr == nil || !strings.Contains(parseErr.Error(), "adk_structured_response_invalid") {
		t.Fatalf("expected model-stage parse failure, got %v", parseErr)
	}
}

func TestFormalAgentSchemaMatrixRoutesEveryCompleteTaskToRunner(t *testing.T) {
	for _, test := range []struct {
		name   string
		schema string
	}{
		{name: "main", schema: "conversation_turn_response"},
		{name: "takeover", schema: takeoverReplySchemaName},
		{name: "wakeup", schema: "wake_up_response"},
		{name: "daily_review", schema: "daily_review_response"},
		{name: "native", schema: "native_cognition_response"},
		{name: "reflection", schema: "reflection_proposal_v2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := formalAgentForSchema(test.schema); !ok {
				t.Fatalf("schema %q has no formal Agent", test.schema)
			}
		})
	}
}

func TestRunADKStructuredTaskRejectsUnregisteredAgentBeforeProviderIO(t *testing.T) {
	app := &App{Provider: &ProviderClient{}}
	_, err := app.RunADKStructuredTask(context.Background(), ADKStructuredTaskInput{
		Role:       "cognitive_assessment",
		Scenario:   "unknown",
		SchemaName: "unknown_response",
		Prompt:     PromptAssemblyResult{ResponseFormat: map[string]any{"type": "object"}},
	})
	if err == nil || !strings.Contains(err.Error(), "formal_agent_not_registered") {
		t.Fatalf("unregistered task was accepted by formal Agent boundary: %v", err)
	}
}

func TestProviderWakeUpUsesADKLoopAndPreservesTrace(t *testing.T) {
	var mu sync.Mutex
	requests := make([]map[string]any, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		mu.Lock()
		requests = append(requests, payload)
		count := len(requests)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"wake-provider-call-1","type":"function","function":{"name":"relationship.lookup","arguments":"{\"intent\":\"recent\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"action_type\":\"no_op\",\"response_mode\":\"final\"}"}}]}`))
	}))
	defer server.Close()

	trace := &ADKCapabilityTrace{}
	ctx := WithADKCapabilityInvoker(WithProviderCorrelation(WithProviderScenario(context.Background(), "wake_up"), "wake_up:fl-1:cycle:3"), adkTraceInvoker{trace: trace}, trace)
	p := &ProviderClient{HTTP: server.Client()}
	response, err := p.generateWithEino(ctx, EinoModelCall{
		Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 128},
		Role:       "cognitive_assessment", Scenario: "wake_up",
		Messages:    []map[string]any{{"role": "system", "content": "wake protocol"}, {"role": "user", "content": "wake event"}},
		Definitions: []CapabilityDefinition{{Name: "relationship.lookup", Description: "Read a bounded relationship fact", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}},
		JSONMode:    true, SchemaName: "wake_up_response", ResponseSchema: map[string]any{"type": "object"},
		ProviderRequestID: "provider-wake-request", CorrelationID: "wake_up:fl-1:cycle:3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Message == nil || len(requests) != 2 || len(trace.Invocations) != 1 || trace.Invocations[0].CallID != "wake-provider-call-1" || trace.Invocations[0].Metadata.Source != "model_tool" {
		t.Fatalf("response=%#v requests=%d trace=%#v", response.Message, len(requests), trace)
	}
	mu.Lock()
	defer mu.Unlock()
	var assistantCall, toolResult bool
	for _, item := range arrayValue(requests[1]["messages"]) {
		message := mapValue(item)
		if stringValue(message["role"]) == "assistant" {
			for _, call := range arrayValue(message["tool_calls"]) {
				if stringValue(mapValue(call)["id"]) == "wake-provider-call-1" {
					assistantCall = true
				}
			}
		}
		if stringValue(message["role"]) == "tool" && stringValue(message["tool_call_id"]) == "wake-provider-call-1" && stringValue(message["name"]) == "relationship.lookup" {
			toolResult = true
		}
	}
	if !assistantCall || !toolResult {
		t.Fatalf("wake-up ADK second request lost tool pair: %#v", requests[1]["messages"])
	}
}

func TestAppADKInvokerWakeUpAlsoRequiresPhysicalProviderIdentity(t *testing.T) {
	app := &App{}
	registry, err := NewCapabilityRegistry(builtinCapabilities(app)...)
	if err != nil {
		t.Fatal(err)
	}
	app.Capabilities = registry
	trace := &ADKCapabilityTrace{}
	invoker := newAppADKCapabilityInvoker(app, ADKCapabilityRequest{
		FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "wake-1", ActionID: "action-1",
		CorrelationID: "wake_up:fl-1:cycle:1", Surface: CapabilitySurfaceWakeUp,
	}, trace)
	identityInvoker, ok := invoker.(ADKCapabilityInvokerWithID)
	if !ok {
		t.Fatal("production invoker does not preserve formal tool identity")
	}
	if _, err := identityInvoker.ExecuteWithID(context.Background(), "wake-call-1", "moment.publish", `{"text":"later"}`); err == nil || !strings.Contains(err.Error(), "provider_request_identity_missing") {
		t.Fatalf("missing physical request identity was accepted: %v", err)
	}
	if len(trace.Invocations) != 0 || len(trace.Results) != 0 {
		t.Fatalf("wake-up trace = %#v", trace)
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
	if response.Message == nil || response.Message.Content == "" || len(response.Message.ToolCalls) != 0 || len(response.ExecutedToolCalls) != 1 || response.ExecutedToolCalls[0].ID != "b-provider-call-1" {
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
	if messages[0].Content != "" {
		t.Fatalf("multimodal message retained mutually-exclusive Content field: %q", messages[0].Content)
	}
	if got := *messages[0].UserInputMultiContent[1].Image.URL; got != "data:image/png;base64,abc" {
		t.Fatalf("image URL = %q", got)
	}
}

func TestEinoFactorySerializesMultimodalMessageWithoutContentCollision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode multimodal request: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

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
	chat, err := NewEinoModelFactory(server.Client()).NewChatModel(context.Background(), EinoModelConfig{
		BaseURL: server.URL, Model: "fake", HTTPClient: server.Client(), MaxCompletionTokens: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chat.Generate(context.Background(), messages); err != nil {
		t.Fatalf("multimodal Generate returned serialization error: %v", err)
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
	if len(optional.Messages) < 3 || !strings.Contains(jsonString(optional.Messages), "fact") {
		t.Fatalf("optional slot was unexpectedly removed: %#v", optional.Messages)
	}
	required, err := composer.Compose(context.Background(), PromptCompositionInput{
		System: "protocol", CurrentInput: "now",
		Slots: []PromptSlot{{ID: PromptSlotRuntimeFact, Position: PromptSlotRuntime, Order: 1, Required: true, BudgetTokens: 1, Fragments: []PromptFragment{{Kind: PromptFragmentRuntimeFact, Required: true, Content: map[string]any{"fact": strings.Repeat("x", 200)}}}}},
	})
	if err != nil || !strings.Contains(jsonString(required.Messages), "fact") {
		t.Fatalf("required content was unexpectedly blocked: err=%v messages=%#v", err, required.Messages)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = composer.Compose(cancelled, PromptCompositionInput{System: "protocol", CurrentInput: "now"})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err = %v", err)
	}
}

var _ tool.BaseTool = (*adkCapabilityTool)(nil)
