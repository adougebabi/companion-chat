package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

// Exercise the pinned SDK/Eino serialization through the same queued native
// model as production. A map-level check alone misses an omitted properties
// field after conversion into Eino's typed JSON Schema.
func TestNoArgToolPropertiesOnNativeProviderWire(t *testing.T) {
	registry := (&App{}).capabilityRegistry()
	for _, mode := range []string{"all_builtin", "kev_disabled_fallback", "kev_version_fallback", "kev_settings_fallback", "after_discovery"} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", mode, streaming), func(t *testing.T) {
				ctx := context.Background()
				definitions := registry.Definitions()
				config := decision.DefaultConfig()
				var state *kevRunSelection
				if strings.HasPrefix(mode, "kev_") {
					config.Enabled = mode != "kev_disabled_fallback"
					state = &kevRunSelection{service: &decision.Service{Settings: &kevFixedSettings{config}}, original: capabilityCatalog(registry, CapabilitySurfaceConversation), loaded: map[string]bool{}, version: 1}
					if mode == "kev_version_fallback" {
						state.version = 2
					} else if mode == "kev_settings_fallback" {
						state.service.Settings = &kevReadSequenceSettings{config: config, reads: 1}
					}
				} else if mode == "after_discovery" {
					config.Enabled = true
					habit, ok := registry.Definition("habit.inspect")
					if !ok {
						t.Fatal("habit.inspect missing")
					}
					state = &kevRunSelection{service: controlledKevToolService(t, "yes"), original: []CapabilityDefinition{(capabilityCatalogCapability{}).Definition(), habit}, loaded: map[string]bool{}, version: 1}
				}
				if state != nil {
					ctx = context.WithValue(ctx, kevRunSelectionKey{}, state)
					definitions = appendCapabilityUtilities(registry, CapabilitySurfaceConversation, state.original)
					state.original = definitions
				}
				original := jsonString(definitions)
				requests := 0
				client := &http.Client{Transport: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) {
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						return nil, err
					}
					requests++
					if (payload["stream"] == true) != streaming {
						t.Error("physical streaming mode changed")
					}
					tools := arrayValue(payload["tools"])
					want := len(definitions)
					if mode == "after_discovery" && requests == 1 {
						want = 1
					}
					if strings.HasPrefix(mode, "kev_") {
						want = 3
					}
					if len(tools) != want {
						t.Errorf("request %d tool count=%d, want %d", requests, len(tools), want)
					}
					catalogFound := false
					for index, raw := range tools {
						function := mapValue(mapValue(raw)["function"])
						name := stringValue(function["name"])
						parameters := mapValue(function["parameters"])
						properties, ok := parameters["properties"].(map[string]any)
						if !ok || parameters["type"] != "object" {
							t.Errorf("tools[%d] %s parameters.properties must be an object, got %#v", index, name, parameters["properties"])
							return &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":"parameters.properties: Required object"}`)), Request: r}, nil
						}
						if name == capabilityCatalogName {
							catalogFound = true
							if properties["names"] == nil || parameters["additionalProperties"] != false {
								t.Errorf("catalog no-argument contract changed: %#v", parameters)
							}
						}
					}
					if !catalogFound {
						t.Error("capability.catalog disappeared from the physical request")
					}
					message := map[string]any{"role": "assistant", "content": "done"}
					finish := "stop"
					if mode == "after_discovery" && requests < 3 {
						name, args := capabilityCatalogName, `{"names":["habit.inspect"]}`
						if requests == 2 {
							name, args = "habit.inspect", "{}"
						}
						message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprint("no-arg-", requests), "type": "function", "function": map[string]any{"name": name, "arguments": args}}}}
						finish = "tool_calls"
					}
					part := "message"
					if streaming {
						part = "delta"
					}
					body := jsonString(map[string]any{"id": "no-arg-response", "model": "test", "choices": []any{map[string]any{"index": 0, part: message, "finish_reason": finish}}})
					contentType := "application/json"
					if streaming {
						contentType = "text/event-stream"
						body = "data: " + body + "\n\ndata: [DONE]\n\n"
					}
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				})}
				chat, err := NewEinoModelFactory(client).NewChatModel(ctx, EinoModelConfig{APIKey: "test", BaseURL: "http://provider.invalid/v1", Model: "test", HTTPClient: client, Timeout: time.Second})
				if err != nil {
					t.Fatal(err)
				}
				invoker := &kevDiscoveryInvoker{state: state}
				tools, err := NewADKCapabilityTools(definitions, invoker)
				if err != nil {
					t.Fatal(err)
				}
				queued := &queuedToolCallingChatModel{inner: chat, provider: &ProviderClient{HTTP: client}, role: "cognitive_assessment", assignment: providerAssignment{TokenBudget: 4096, ContextWindowTokens: 131072, MaxInputTokens: 98304, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}}
				result, err := RunADKLoop(ctx, ADKLoopConfig{Name: "no-arg-schema-test", Description: "closed no-argument tools survive provider serialization", Model: queued, Tools: tools, MaxIterations: 4, EnableStreaming: streaming}, []*schema.Message{schema.UserMessage("Inspect authorized tools")})
				if err != nil {
					t.Fatal(err)
				}
				wantRequests := 1
				if mode == "after_discovery" {
					wantRequests = 3
					if invoker.executions != 1 || len(result.ToolResults) != 2 {
						t.Fatalf("discovery feedback/execution missing: executions=%d results=%d", invoker.executions, len(result.ToolResults))
					}
				}
				if requests != wantRequests || result.FinalMessage == nil {
					t.Fatalf("requests=%d want=%d final=%v", requests, wantRequests, result.FinalMessage)
				}
				if original != jsonString(definitions) {
					t.Fatal("provider conversion mutated original capability schemas")
				}
				t.Logf("validated %d installed root schemas across %d physical requests", len(definitions), requests)
			})
		}
	}
}

func TestNoArgCatalogSchemaValidationAndCatalogParity(t *testing.T) {
	registry := (&App{}).capabilityRegistry()
	definition, ok := registry.Definition(capabilityCatalogName)
	if !ok {
		t.Fatal("catalog missing")
	}
	invocation := CapabilityInvocation{CallID: "catalog-contract", CapabilityName: capabilityCatalogName, Arguments: json.RawMessage(`{}`), Metadata: InvocationMetadata{Source: "direct", Surface: CapabilitySurfaceConversation}}
	if err := invocation.Validate(definition); err != nil {
		t.Fatal(err)
	}
	invocation.Arguments = json.RawMessage(`{"extra":true}`)
	if err := invocation.Validate(definition); err == nil {
		t.Fatal("no-argument catalog accepted an extra argument")
	}
	invocation.Arguments = json.RawMessage(`{}`)
	catalog := capabilityCatalogCapability{catalog: registry.Catalog}
	result, err := catalog.Execute(context.Background(), invocation, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, raw := range arrayValue(mapValue(result.Output)["items"]) {
		item := mapValue(raw)
		if item["name"] == capabilityCatalogName {
			found = true
			parameters := mapValue(item["parameters"])
			properties, isObject := parameters["properties"].(map[string]any)
			if !isObject || properties == nil || !reflect.DeepEqual(parameters, definition.InputSchema) {
				t.Fatalf("catalog schema disagrees with the native definition: %#v", parameters)
			}
		}
	}
	if !found {
		t.Fatal("catalog missing its authorized no-argument definition")
	}
}
