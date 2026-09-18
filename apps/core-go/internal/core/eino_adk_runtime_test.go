package core

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
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

type adkFakeChatModel struct {
	mu    sync.Mutex
	calls int
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

type adkTraceInvoker struct{ trace *ADKCapabilityTrace }

func (i adkTraceInvoker) Execute(_ context.Context, capabilityName string, argumentsJSON string) (string, error) {
	callID := "trace-call"
	i.trace.Invocations = append(i.trace.Invocations, CapabilityInvocation{CallID: callID, CapabilityName: capabilityName, Arguments: json.RawMessage(argumentsJSON), SchemaVersion: CapabilityInvocationSchemaVersion, SourceFactID: "source", ProviderRequestID: "provider"})
	i.trace.Results = append(i.trace.Results, CapabilityResult{CallID: callID, CapabilityName: capabilityName, Status: "completed", Output: map[string]any{"items": []any{"recent"}}})
	return `{"items":["recent"]}`, nil
}

func (i *adkFakeInvoker) Execute(_ context.Context, capabilityName string, argumentsJSON string) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.calls = append(i.calls, capabilityName+":"+argumentsJSON)
	return `{"items":["recent"]}`, nil
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

func TestProviderConversationUsesADKLoopAndReturnsCanonicalTrace(t *testing.T) {
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
		JSONMode:    true, SchemaName: "conversation_turn_response", ResponseSchema: map[string]any{"type": "object"}, EnableThinking: true, ProviderRequestID: "provider-request-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Message == nil || len(response.Message.ToolCalls) != 1 || len(trace.Invocations) != 1 || len(trace.Results) != 1 {
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
	secondMessages := arrayValue(requests[1]["messages"])
	foundToolResult := false
	for _, item := range secondMessages {
		if stringValue(mapValue(item)["role"]) == "tool" {
			foundToolResult = true
		}
	}
	if !foundToolResult {
		t.Fatalf("second request did not contain tool result: %#v", secondMessages)
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

var _ tool.BaseTool = (*adkCapabilityTool)(nil)
