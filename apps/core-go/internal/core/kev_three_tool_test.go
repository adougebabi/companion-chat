package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

func toolNamesForTest(definitions []CapabilityDefinition) []string {
	names := []string{}
	for _, d := range definitions {
		names = append(names, d.Name)
	}
	sort.Strings(names)
	return names
}

type kevRejectingAudit struct{}

func (kevRejectingAudit) Save(context.Context, decision.Record) error {
	return errors.New("controlled audit failure")
}

func TestKevNamedCatalogAdmissionIsBoundedAndFailsClosed(t *testing.T) {
	for _, scenario := range []string{"yes", "no", "unclear", "disabled", "audit_failed", "settings_failed"} {
		t.Run(scenario, func(t *testing.T) {
			choice := scenario
			if choice != "no" && choice != "unclear" {
				choice = "yes"
			}
			service := controlledKevToolService(t, choice)
			if scenario == "disabled" {
				service.Settings.(*kevFixedSettings).config.Enabled = false
			}
			if scenario == "audit_failed" {
				service.Store = kevRejectingAudit{}
			}
			if scenario == "settings_failed" {
				service.Settings = &kevReadSequenceSettings{reads: 1}
			}
			original := []CapabilityDefinition{(capabilityCatalogCapability{}).Definition(), {Name: conversationReplyCapabilityName}, {Name: "moment.publish"}, {Name: "optional"}}
			state := &kevRunSelection{service: service, original: original, loaded: map[string]bool{}, version: 1}
			ctx := decision.WithStageBudget(context.Background(), -time.Minute)
			loaded, unavailable, err := state.load(ctx, []string{"optional"})
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "yes" {
				if len(loaded) != 1 || len(unavailable) != 0 || !state.permits(ctx, "optional") {
					t.Fatalf("fresh catalog stage failed: %v %v", loaded, unavailable)
				}
				service.Settings.(*kevFixedSettings).config.Enabled = false
				if state.permits(ctx, "optional") {
					t.Fatal("disabled optional tool remained executable")
				}
			} else if len(loaded) != 0 || len(unavailable) != 1 || state.permits(ctx, "optional") {
				t.Fatalf("failed admission leaked: %v %v", loaded, unavailable)
			}
			defs, _ := state.definitions(ctx)
			if len(defs) != 3 {
				t.Fatalf("fallback grew beyond baseline: %v", toolNamesForTest(defs))
			}
		})
	}
	service := controlledKevToolService(t, "yes")
	original := []CapabilityDefinition{(capabilityCatalogCapability{}).Definition(), {Name: conversationReplyCapabilityName}, {Name: "moment.publish"}}
	for i := 0; i < 16; i++ {
		original = append(original, CapabilityDefinition{Name: fmt.Sprintf("optional.%d", i)})
	}
	state := &kevRunSelection{service: service, original: original, loaded: map[string]bool{}, version: 1}
	for offset := 0; offset < 16; offset += 8 {
		names := []string{}
		for i := offset; i < offset+8; i++ {
			names = append(names, fmt.Sprintf("optional.%d", i))
		}
		if _, _, err := state.load(context.Background(), names); err != nil {
			t.Fatal(err)
		}
		defs, _ := state.definitions(context.Background())
		if len(defs) != 11 {
			t.Fatalf("optional window accumulated: %d", len(defs))
		}
	}
	if state.permits(context.Background(), "optional.0") {
		t.Fatal("replaced window still executable")
	}
	if _, _, err := state.load(context.Background(), []string{"optional.0", "foreign"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("foreign request accepted", err)
	}
	if !state.permits(context.Background(), "optional.8") {
		t.Fatal("foreign request partially changed visibility")
	}
}

func controlledKevToolService(t *testing.T, choice string) *decision.Service {
	t.Helper()
	config := decision.DefaultConfig()
	config.Enabled = true
	config.Endpoint = "http://controlled-kev.invalid"
	ids := 0
	return &decision.Service{Settings: &kevFixedSettings{config}, Store: &kevMemoryStore{}, ID: func() string { ids++; return fmt.Sprint(ids) }, HTTP: &http.Client{Transport: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) {
		var request decision.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return nil, err
		}
		answers := map[string]any{}
		for id := range request.Questions {
			probabilities := map[string]float64{"yes": 0.1, "no": 0.1, "unclear": 0.1}
			probabilities[choice] = 0.8
			answers[id] = map[string]any{"type": "choice", "choice": choice, "confidence": 0.8, "probabilities": probabilities}
		}
		return embeddingHTTPResponse(r, 200, jsonString(map[string]any{"answers": answers})), nil
	})}}
}

func TestKevThreeToolBaselineOnActualWireWhenDisabled(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			config := decision.DefaultConfig()
			app := &App{Kev: &decision.Service{Settings: &kevFixedSettings{config}, Store: &kevMemoryStore{}}}
			definitions := capabilityCatalog(app.capabilityRegistry(), CapabilitySurfaceConversation)
			ctx, executors, err := app.prepareKevTools(context.Background(), ADKStructuredTaskInput{AgentID: FormalAgentConversationCognition, Definitions: definitions, Capability: &ADKCapabilityRequest{Surface: CapabilitySurfaceConversation, Projection: ContextProjection{FluctlightID: "self"}}})
			if err != nil {
				t.Fatal(err)
			}
			seen := []string{}
			provider := &ProviderClient{HTTP: &http.Client{Transport: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					return nil, err
				}
				for _, raw := range arrayValue(payload["tools"]) {
					seen = append(seen, stringValue(mapValue(mapValue(raw)["function"])["name"]))
				}
				key := "message"
				if streaming {
					key = "delta"
				}
				encoded := jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, key: map[string]any{"role": "assistant", "content": "controlled result"}, "finish_reason": "stop"}}})
				if streaming {
					encoded = "data: " + encoded + "\n\ndata: [DONE]\n\n"
				}
				response := embeddingHTTPResponse(r, 200, encoded)
				if streaming {
					response.Header.Set("Content-Type", "text/event-stream")
				}
				return response, nil
			})}}
			assignment := providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "controlled-model", Timeout: time.Second, TokenBudget: 1024, ContextWindowTokens: 131072, MaxInputTokens: 65536, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
			inner, err := NewEinoModelFactory(provider.HTTP).NewChatModel(ctx, EinoModelConfig{BaseURL: assignment.BaseURL, Model: assignment.ModelID, HTTPClient: provider.HTTP, MaxCompletionTokens: 1024})
			if err != nil {
				t.Fatal(err)
			}
			m := &queuedToolCallingChatModel{inner: inner, provider: provider, role: "cognitive_assessment", assignment: assignment, definitions: executors, correlationID: "three-tools"}
			infos, err := aiagent.CapabilityToolInfos(executors)
			if err != nil {
				t.Fatal(err)
			}
			if streaming {
				reader, err := m.Stream(ctx, []*schema.Message{schema.UserMessage("ordinary conversation")}, model.WithTools(infos))
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				for {
					_, err := reader.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
				}
			} else if _, err := m.Generate(ctx, []*schema.Message{schema.UserMessage("ordinary conversation")}, model.WithTools(infos)); err != nil {
				t.Fatal(err)
			}
			sort.Strings(seen)
			want := []string{capabilityCatalogName, conversationReplyCapabilityName, "moment.publish"}
			sort.Strings(want)
			if !reflect.DeepEqual(seen, want) {
				t.Fatalf("first physical request disclosed %d tools; want only %v; actual=%v", len(seen), want, seen)
			}

		})
	}
}
