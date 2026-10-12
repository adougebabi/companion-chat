package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
)

func TestEinoStreamObservationPreservesTerminalMetadataAndUsage(t *testing.T) {
	var result *schema.Message
	var calls int
	observer := newEinoStreamObservation(func(message *schema.Message, err error) {
		calls++
		result = message
		if err != nil {
			t.Fatal(err)
		}
	})
	observer.observe(&schema.Message{Role: schema.Assistant, Content: "complete ", ReasoningContent: "private "}, nil)
	observer.observe(&schema.Message{Role: schema.Assistant, Content: "answer", ReasoningContent: "reasoning", ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 12, CompletionTokens: 3, TotalTokens: 15}}}, nil)
	observer.observe(&schema.Message{ResponseMeta: &schema.ResponseMeta{FinishReason: "length"}}, nil)
	observer.observe(&schema.Message{ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 12, CompletionTokens: 5, TotalTokens: 17, PromptTokenDetails: schema.PromptTokenDetails{CachedTokens: 2}, CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 4}}}}, nil)
	observer.observe(nil, io.EOF)
	observer.observe(nil, io.EOF)
	raw := einoMessageRaw(result)
	if calls != 1 || raw["content"] != "complete answer" || raw["reasoning_content"] != "private reasoning" || raw["finish_reason"] != "length" || intValue(mapValue(raw["usage"])["completion_tokens"]) != 5 || intValue(mapValue(mapValue(raw["usage"])["completion_tokens_details"])["reasoning_tokens"]) != 4 || intValue(mapValue(mapValue(raw["usage"])["prompt_tokens_details"])["cached_tokens"]) != 2 {
		t.Fatalf("terminal aggregation lost: calls=%d raw=%s", calls, jsonString(raw))
	}
}

func TestEinoStreamObservationBoundsOutputAndRetainsTerminalMetadata(t *testing.T) {
	var result *schema.Message
	observer := newEinoStreamObservation(func(m *schema.Message, _ error) { result = m })
	observer.observe(&schema.Message{Role: schema.Assistant, Content: "kept"}, nil)
	observer.observe(&schema.Message{Role: schema.Assistant, Content: strings.Repeat("x", 2*1024*1024)}, nil)
	observer.observe(&schema.Message{ResponseMeta: &schema.ResponseMeta{FinishReason: "stop", Usage: &schema.TokenUsage{CompletionTokens: 17}}}, nil)
	observer.observe(nil, io.EOF)
	if result.Content != "kept" || result.ResponseMeta.FinishReason != "stop" || einoMessageRaw(result)["diagnostic_output_truncated"] != true {
		t.Fatalf("unbounded/lost terminal metadata: %+v", result)
	}
}

func TestEinoStreamObservationBoundsEmptyCallsAndOversizedMetadata(t *testing.T) {
	var result *schema.Message
	observer := newEinoStreamObservation(func(m *schema.Message, _ error) { result = m })
	observer.observe(&schema.Message{Role: schema.Assistant, Content: "kept"}, nil)
	observer.observe(&schema.Message{ToolCalls: make([]schema.ToolCall, 20000)}, nil)
	observer.observe(&schema.Message{ResponseMeta: &schema.ResponseMeta{FinishReason: strings.Repeat("x", 2*1024*1024), Usage: &schema.TokenUsage{CompletionTokens: 17}}}, nil)
	observer.observe(nil, io.EOF)
	if observer.bytes > 2*1024*1024 || result.Content != "kept" || len(result.ToolCalls) != 0 || len(result.ResponseMeta.FinishReason) != 256 || result.ResponseMeta.Usage.CompletionTokens != 17 || result.Extra["diagnostic_output_truncated"] != true {
		t.Fatalf("diagnostic bound lost: bytes=%d message=%+v", observer.bytes, result)
	}
}

func TestEinoWithToolsSharesPhysicalResponseMetadata(t *testing.T) {
	physical := &schema.Message{ResponseMeta: &schema.ResponseMeta{FinishReason: "length", Usage: &schema.TokenUsage{CompletionTokens: 19, CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 17}}}}
	queued := &queuedToolCallingChatModel{inner: &adkScriptedChatModel{}, lastPhysicalResponse: &atomic.Pointer[schema.Message]{}}
	bound, err := queued.WithTools([]*schema.ToolInfo{{Name: "memory.recall"}})
	if err != nil {
		t.Fatal(err)
	}
	bound.(*queuedToolCallingChatModel).lastPhysicalResponse.Store(physical)
	final := &schema.Message{ResponseMeta: &schema.ResponseMeta{FinishReason: "stop"}}
	preservePhysicalResponseMetadata(final, queued)
	if final.ResponseMeta.FinishReason != "length" || final.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens != 17 {
		t.Fatalf("bound physical metadata lost: %+v", final)
	}
	final.ResponseMeta.Usage.CompletionTokens = 99
	if physical.ResponseMeta.Usage.CompletionTokens != 19 {
		t.Fatal("metadata handoff mutated physical response")
	}
}

func TestEinoStreamObservationPreservesNativeChunksAndTerminalFailures(t *testing.T) {
	for _, failure := range []error{io.EOF, context.Canceled, errors.New("provider_stream_failed")} {
		t.Run(failure.Error(), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			inner, writer := schema.Pipe[*schema.Message](2)
			chunk := &schema.Message{Role: schema.Assistant, Content: "native", ToolCalls: []schema.ToolCall{{ID: "actual-call", Type: "function", Function: schema.FunctionCall{Name: "hidden", Arguments: "{}"}, Extra: map[string]any{"private": strings.Repeat("x", 2*1024*1024)}}}}
			writer.Send(chunk, nil)
			if failure != io.EOF && failure != context.Canceled {
				writer.Send(nil, failure)
			}
			if failure != context.Canceled {
				writer.Close()
			} else {
				defer writer.Close()
			}
			done := make(chan error, 1)
			var count atomic.Int32
			observer := newEinoStreamObservation(func(full *schema.Message, err error) {
				count.Add(1)
				if full == nil || full.Content != "native" || len(full.ToolCalls) != 1 || len(full.ToolCalls[0].Extra) != 0 {
					t.Errorf("bounded diagnostic projection lost content/calls or retained arbitrary Extra: %+v", full)
				}
				done <- err
			})
			reader, err := runProviderQueuedStream(&ProviderClient{generated: newProviderQueue(1)}, ctx, "generic_llm", "observation", 1, "", func(context.Context) (*schema.StreamReader[*schema.Message], error) { return inner, nil }, observer.observe)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			actual, err := reader.Recv()
			if err != nil || actual != chunk || actual.ToolCalls[0].Extra["private"] == nil {
				t.Fatalf("observer altered native chunk: %+v/%v", actual, err)
			}
			if failure == context.Canceled {
				cancel()
			}
			_, err = reader.Recv()
			if failure == io.EOF && !errors.Is(err, io.EOF) || failure != io.EOF && failure != context.Canceled && !errors.Is(err, failure) || failure == context.Canceled && err == nil {
				t.Fatalf("native terminal error changed: %v", err)
			}
			select {
			case observed := <-done:
				if failure == io.EOF && observed != nil || failure != io.EOF && !errors.Is(observed, failure) || count.Load() != 1 {
					t.Fatalf("diagnostic terminal=%v count=%d", observed, count.Load())
				}
			case <-time.After(time.Second):
				t.Fatal("terminal observation missing")
			}
		})
	}
}

func TestADKReasoningOnlyPreservesActualFinishReasonAndUsage(t *testing.T) {
	for _, mode := range []struct{ streaming, split bool }{{false, false}, {true, false}, {true, true}} {
		streaming, split := mode.streaming, mode.split
		for _, finish := range []string{"stop", "length", ""} {
			t.Run(fmt.Sprintf("stream=%t/split=%t/%s", streaming, split, finish), func(t *testing.T) {
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					usage := map[string]any{"prompt_tokens": 16191, "completion_tokens": 193, "total_tokens": 16384, "completion_tokens_details": map[string]any{"reasoning_tokens": 193}, "prompt_tokens_details": map[string]any{"cached_tokens": 64}}
					if body["stream"] == true {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprintf(w, "data: %s\n\n", jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "reasoning_content": "private reasoning"}}}}))
						if split {
							fmt.Fprintf(w, "data: %s\n\n", jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}}))
							fmt.Fprintf(w, "data: %s\n\n", jsonString(map[string]any{"choices": []any{}, "usage": usage}))
						} else {
							fmt.Fprintf(w, "data: %s\n\n", jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}, "usage": usage}))
						}
						fmt.Fprint(w, "data: [DONE]\n\n")
					} else {
						_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "", "reasoning_content": "private reasoning"}, "finish_reason": finish}}, "usage": usage})
					}
				}))
				defer server.Close()
				ctx := withADKCapabilityContext(context.Background(), &adkResultInvoker{}, &ADKCapabilityTrace{}, nil)
				_, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{BaseURL: server.URL, ModelID: "fixture", Timeout: time.Second, TokenBudget: 40960, ContextWindowTokens: 131072, MaxInputTokens: 65536, PromptBudgetPolicyVersion: "prompt-budget.v1"}, Role: "cognitive_assessment", Scenario: "conversation", SchemaName: "conversation_turn_response", Messages: []map[string]any{{"role": "user", "content": "reply"}}, Definitions: []CapabilityDefinition{{Name: "hidden", Version: "v1", Description: "actual effect", InputSchema: objectSchema(map[string]any{}, nil, false)}}, EnableThinking: true, EnableStreaming: streaming})
				var empty *aiagent.EmptyFinalMessageError
				if !errors.As(err, &empty) || calls.Load() != 1 {
					t.Fatalf("reasoning-only retried/published or cause lost: calls=%d err=%v", calls.Load(), err)
				}
				stage, code := classifyAgentRunFailure(fmt.Errorf("wrapped: %w", err))
				if stage != "model_output" || code != "adk_final_message_empty" || adkTerminationReason(err) != code {
					t.Fatalf("empty output classification: %s/%s", stage, code)
				}
				raw := einoMessageRaw(empty.Message)
				if raw["finish_reason"] != finish || raw["content"] != "" || raw["reasoning_content"] != "private reasoning" || intValue(mapValue(raw["usage"])["completion_tokens"]) != 193 || intValue(mapValue(mapValue(raw["usage"])["prompt_tokens_details"])["cached_tokens"]) != 64 {
					t.Fatalf("actual terminal metadata lost: %s", jsonString(raw))
				}
				if split {
					// The pinned SDK drops this optional detail on a usage-only frame.
					if mapValue(raw["usage"])["completion_tokens_details"] != nil {
						t.Fatal("unavailable split-frame reasoning usage was fabricated")
					}
				} else if intValue(mapValue(mapValue(raw["usage"])["completion_tokens_details"])["reasoning_tokens"]) != 193 {
					t.Fatalf("available reasoning usage lost: %s", jsonString(raw))
				}
			})
		}
	}
}

func TestFinalRepairContextRetainsRefsAndTraceWithoutAdmissionOrRefresh(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	codec := &providerContextRefCodec{}
	refresh := &runtimeContextRefresh{dirty: true}
	ctx := withADKCapabilityContextRefs(context.Background(), &adkResultInvoker{}, trace, refresh, codec)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, &kevRunSelection{})
	ctx = context.WithValue(ctx, kevPersonaKey{}, &kevPersonaAdmission{})
	ctx = withPhysicalModelRequestPolicy(ctx, physicalModelRequestPolicyFunc(func(context.Context) (physicalModelRequestDecision, error) {
		t.Fatal("business policy ran during repair")
		return physicalModelRequestDecision{}, nil
	}))
	repair := finalRepairContext(ctx)
	bridge, ok := adkCapabilityContext(repair)
	if !ok || bridge.Refs != codec || bridge.Trace != trace || bridge.Refresh != nil || kevSelection(repair) != nil || kevPersona(repair) != nil {
		t.Fatal("repair context leaked gates/refresh or lost original refs/audit")
	}
	format := map[string]any{"type": "json_schema"}
	queued := &queuedToolCallingChatModel{responseFormat: format}
	effective, _, output, err := queued.physicalRequestOptions(repair, nil)
	if err != nil || len(effective.definitions) != 0 || jsonString(output) != jsonString(format) {
		t.Fatalf("repair effective budget/catalog/schema: %+v/%v/%v", effective, output, err)
	}
	if kevSelection(ctx) == nil || kevPersona(ctx) == nil {
		t.Fatal("repair mutated original task context")
	}
}
