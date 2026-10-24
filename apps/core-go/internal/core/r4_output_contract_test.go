package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestR4FormalObjectFraming(t *testing.T) {
	for _, text := range []string{"{\"portrait_text\":\"原有事实\"}", "```json\n{\"portrait_text\":\"原有事实\"}\n```", "```\r\n{\"portrait_text\":\"原有事实\"}\r\n```"} {
		value, ok := parseStructuredCandidates(providerStructuredCandidates(map[string]any{"content": text, "reasoning_content": "ignored"}))
		if !ok || value["portrait_text"] != "原有事实" {
			t.Fatal("complete formal object rejected")
		}
	}
	for _, text := range []string{"前文\n```json\n{}\n```", "```json\n{}\n```\n后文", "```json\n{\"x\":", "```json\n{}", "```json\n{} {}\n```", "```text\n{}\n```", `"{\"x\":1}"`, "<tool_use>{}</tool_use>"} {
		if _, ok := parseStructuredCandidates(providerStructuredCandidates(map[string]any{"content": text})); ok {
			t.Fatalf("ambiguous/partial content accepted: %q", text)
		}
	}
	if _, ok := parseStructuredCandidates(providerStructuredCandidates(map[string]any{"reasoning_content": "```json\n{}\n```"})); ok {
		t.Fatal("reasoning became formal output")
	}
}

func TestR4PersonaSchemaAndFaithfulRepair(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var request map[string]any
				_ = json.NewDecoder(r.Body).Decode(&request)
				format := mapValue(request["response_format"])
				if format["type"] != "json_schema" || mapValue(format["json_schema"])["name"] != "persona_compilation_response" {
					t.Errorf("persona schema downgraded: %v", format)
				}
				if request["max_tokens"] != nil || request["max_completion_tokens"] != nil {
					t.Error("token cap restored")
				}
				content := "```json\n{\"portrait_text\":\"完整画像\"}\n```"
				if calls == 1 {
					content = `{"unique_invalid_output":"retain-this-result"}`
				} else {
					found := false
					for _, raw := range arrayValue(request["messages"]) {
						m := mapValue(raw)
						if m["role"] == "assistant" && strings.Contains(stringValue(m["content"]), "retain-this-result") {
							found = true
						}
					}
					if !found || len(arrayValue(request["tools"])) != 0 {
						t.Error("repair lost original output or installed tools")
					}
				}
				choice := map[string]any{"index": 0, "finish_reason": "stop"}
				message := map[string]any{"role": "assistant", "content": content}
				if stream {
					choice["delta"] = message
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"choices": []any{choice}}))
					return
				}
				choice["message"] = message
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{choice}})
			}))
			defer server.Close()
			trace := &ADKCapabilityTrace{}
			ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil)
			p := &ProviderClient{HTTP: server.Client()}
			result, err := p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "initialization", BaseURL: server.URL, ModelID: "controlled", Timeout: time.Second * 3}, Role: "initialization", Scenario: "persona_compilation", SchemaName: "persona_compilation_response", ResponseSchema: personaCompilationResponseSchema(), JSONMode: true, EnableStreaming: stream, Messages: []map[string]any{{"role": "user", "content": "source"}}})
			if err != nil || calls != 2 || !strings.Contains(result.Message.Content, "完整画像") {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
	if providerResponseFormatForSchema("initialization", "initialization_response", initializationResponseSchema())["type"] != "json_object" {
		t.Fatal("complete initialization contract changed")
	}
}

// User diagnostics remain opt-in local evidence, never committed private fixtures.
func TestR4ProvidedDiagnosticsReplay(t *testing.T) {
	root := os.Getenv("GO_CORE_R4_DIAGNOSTICS_DIR")
	if root == "" {
		t.Skip("local r4 diagnostic replay not requested")
	}
	files, err := filepath.Glob(filepath.Join(root, "*.json"))
	if err != nil || len(files) != 5 {
		t.Fatal("expected five supplied diagnostic records", err)
	}
	valid, partial, length := 0, 0, 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		record := decodeObject(raw)
		response := mapValue(record["response"])
		value, ok := parseStructuredCandidates(providerStructuredCandidates(response))
		finish := stringValue(response["finish_reason"])
		message := schema.AssistantMessage(stringValue(response["content"]), nil)
		message.ResponseMeta = &schema.ResponseMeta{FinishReason: finish}
		if finish == "stop" {
			valid++
			if !ok || value["portrait_text"] == nil {
				t.Fatal("supplied complete persona did not parse")
			}
			if err := validateADKFinalContractForSchema(message, "initialization", "persona_compilation_response", personaCompilationResponseSchema(), nil); err != nil {
				t.Fatal(err)
			}
		} else {
			length++
			if !ok {
				partial++
			}
			var truncated *structuredResponseTruncatedError
			if !errors.As(structuredTerminationError(message, "initialization"), &truncated) || truncated.contentComplete != ok {
				t.Fatal("length metadata lost actual content completeness")
			}
		}
	}
	if valid != 2 || length != 3 || partial != 2 {
		t.Fatalf("stop=%d length=%d partial=%d", valid, length, partial)
	}
}

func TestR4WriterDiagnosticsKeepAuthoritativeScope(t *testing.T) {
	var metadata map[string]any
	client := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		metadata = providerPromptDiagnostics(request.Context())
		return embeddingHTTPResponse(request, http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"收到。"}}]}`), nil
	})}
	app := &App{Provider: &ProviderClient{HTTP: client}}
	input := replyContextFixture()
	ctx := WithPromptDiagnostics(context.Background(), map[string]any{"fluctlight_id": "foreign-parent", "conversation_id": "foreign-conversation"})
	_, err := app.RunConversationReplyTask(ctx, ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "m", Timeout: time.Second}, CorrelationID: "writer", Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if metadata["fluctlight_id"] != input.Projection.FluctlightID || metadata["conversation_id"] != input.Projection.ConversationID {
		t.Fatal("Writer diagnostics lost/borrowed identity", metadata)
	}
	if strings.Contains(jsonString(metadata), "foreign-parent") {
		t.Fatal("parent metadata restored")
	}
}

func TestR4SummaryScenariosRemainDistinctAtFormalBoundary(t *testing.T) {
	for _, item := range []struct{ schema, scenario string }{{"conversation_summary_v1", "conversation_summary"}, {"conversation_segment_v1", "conversation_segment"}, {"conversation_daily_memory_v1", "conversation_daily_memory"}} {
		t.Run(item.schema, func(t *testing.T) {
			observed := ""
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				observed = providerScenario(request.Context(), "reflection", "unset")
				return embeddingHTTPResponse(request, http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"ok\":true}"}}]}`), nil
			})}}}
			ctx := withFrozenProviderAssignment(WithProviderScenario(context.Background(), "foreign-parent"), "reflection", providerAssignment{Role: "reflection", BaseURL: "http://controlled.invalid", ModelID: "m", Timeout: time.Second})
			_, err := app.RunFormalAgent(ctx, FormalAgentConversationSummary, FormalAgentRunInput{SchemaName: item.schema, Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "source"}}, ResponseFormat: openObjectSchema()}})
			if err != nil || observed != item.scenario {
				t.Fatalf("scenario=%s want=%s err=%v", observed, item.scenario, err)
			}
		})
	}
}
