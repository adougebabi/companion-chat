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
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
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

type kevFlakySettings struct {
	mu      sync.Mutex
	config  decision.Config
	version int64
	reads   int
	failAt  int
	failAll bool
}

func (s *kevFlakySettings) Read(context.Context) (decision.Config, int64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	if s.failAll || s.reads == s.failAt {
		return decision.Config{}, 0, "", errors.New("controlled transient settings failure")
	}
	return s.config, s.version, "", nil
}
func TestKevTransientSettingsReadsKeepNamedWindowConsistent(t *testing.T) {
	for _, failAt := range []int{1, 3} {
		for _, choice := range []string{"yes", "no"} {
			t.Run(fmt.Sprintf("read=%d/%s", failAt, choice), func(t *testing.T) {
				service := controlledKevToolService(t, choice)
				settings := &kevFlakySettings{config: service.Settings.(*kevFixedSettings).config, version: 1, failAt: failAt}
				service.Settings = settings
				state := &kevRunSelection{service: service, original: []CapabilityDefinition{(capabilityCatalogCapability{}).Definition(), {Name: "optional"}}, loaded: map[string]bool{}, version: 1}
				loaded, unavailable, err := state.load(context.Background(), []string{"optional"})
				if err != nil || len(loaded) != 1 || len(unavailable) != 0 {
					t.Fatalf("transient read treated as change: %v %v %v", loaded, unavailable, err)
				}
				item, _ := state.availabilityFor(context.Background(), "optional")
				defs, err := state.definitions(context.Background())
				if err != nil || item.Status != "available" || item.Admission != "catalog_fallback" || len(defs) != 2 || !state.permits(context.Background(), "optional") {
					t.Fatalf("loaded/availability/schema/guard drifted: %+v %v %v", item, toolNamesForTest(defs), err)
				}
				for _, record := range service.Store.(*kevMemoryStore).records {
					if record.Reason == "config_changed" {
						t.Fatal("transient read mislabeled in audit")
					}
				}
			})
		}
	}
}
func TestKevObservedRevocationCannotReturnAfterReadFailure(t *testing.T) {
	for _, observe := range []string{"schema", "availability"} {
		t.Run(observe, func(t *testing.T) {
			service := controlledKevToolService(t, "yes")
			config := service.Settings.(*kevFixedSettings).config
			config.Enabled = false
			settings := &kevFlakySettings{config: config, version: 1}
			service.Settings = settings
			state := &kevRunSelection{service: service, original: []CapabilityDefinition{(capabilityCatalogCapability{}).Definition(), {Name: "optional"}}, loaded: map[string]bool{}, version: 1}
			if _, _, err := state.load(context.Background(), []string{"optional"}); err != nil {
				t.Fatal(err)
			}
			settings.mu.Lock()
			settings.version = 2
			settings.mu.Unlock()
			if observe == "schema" {
				_, _ = state.definitions(context.Background())
			} else {
				_, _ = state.availabilityFor(context.Background(), "optional")
			}
			settings.mu.Lock()
			settings.failAll = true
			settings.mu.Unlock()
			defs, err := state.definitions(context.Background())
			item, _ := state.availabilityFor(context.Background(), "optional")
			if err != nil || len(defs) != 1 || !state.permits(context.Background(), "optional") || item.WarningCode != "tool_selection_changed" || item.Status != "available" {
				t.Fatalf("revoked window resurrected: %v %+v %v", toolNamesForTest(defs), item, err)
			}
			settings.mu.Lock()
			settings.failAll = false
			settings.mu.Unlock()
			if _, _, err := state.load(context.Background(), []string{"optional"}); err != nil || !state.permits(context.Background(), "optional") {
				t.Fatal("fresh named request could not establish new window", err)
			}
		})
	}
}

func TestKevFailedAuditDoesNotAdoptUnauditedNo(t *testing.T) {
	service := controlledKevToolService(t, "no")
	service.Store = kevRejectingAudit{}
	state := &kevRunSelection{service: service, original: []CapabilityDefinition{{Name: "optional"}}, loaded: map[string]bool{}, version: 1}
	loaded, unavailable, err := state.load(context.Background(), []string{"optional"})
	if err != nil || len(loaded) != 1 || len(unavailable) != 0 {
		t.Fatalf("unaudited answer adopted: %v %v %v", loaded, unavailable, err)
	}
	item, _ := state.availabilityFor(context.Background(), "optional")
	if item.Admission != "catalog_fallback" {
		t.Fatal("fallback misreported as Kev decision", item)
	}
}

func TestKevStableDisabledNamedCatalogLoadsRequestedTools(t *testing.T) {
	for _, globalDisabled := range []bool{true, false} {
		t.Run(fmt.Sprint(globalDisabled), func(t *testing.T) {
			service := controlledKevToolService(t, "yes")
			settings := service.Settings.(*kevFixedSettings)
			settings.config.Enabled = !globalDisabled
			settings.config.Points["tools.select"] = false
			original := []CapabilityDefinition{(capabilityCatalogCapability{}).Definition(), {Name: conversationReplyCapabilityName}, {Name: "moment.publish"}}
			names := []string{conversationReplyCapabilityName, "moment.publish"}
			for i := 0; i < 6; i++ {
				name := fmt.Sprintf("requested.%d", i)
				original = append(original, CapabilityDefinition{Name: name})
				names = append(names, name)
			}
			original = append(original, CapabilityDefinition{Name: "unrequested"})
			state := &kevRunSelection{service: service, original: original, loaded: map[string]bool{}, version: 1}
			loaded, unavailable, err := state.load(context.Background(), names)
			if err != nil || len(loaded) != 8 || len(unavailable) != 0 {
				t.Fatalf("stable config misclassified as changed: loaded=%v unavailable=%v err=%v", loaded, unavailable, err)
			}
			defs, err := state.definitions(context.Background())
			if err != nil || len(defs) != 9 || !state.permits(context.Background(), "unrequested") {
				t.Fatalf("explicit window not preserved/bounded: %v %v", toolNamesForTest(defs), err)
			}
			if len(service.Store.(*kevMemoryStore).records) != 0 {
				t.Fatal("disabled selector called Kev")
			}
		})
	}
}

func (kevRejectingAudit) Save(context.Context, decision.Record) error {
	return errors.New("controlled audit failure")
}

func TestKevNamedCatalogAdmissionHasBoundedFallbackAndRespectsNo(t *testing.T) {
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
			if scenario == "no" {
				if len(loaded) != 1 || len(unavailable) != 0 || !state.permits(ctx, "optional") {
					t.Fatalf("explicit No bypassed: %v %v", loaded, unavailable)
				}
			} else {
				if len(loaded) != 1 || len(unavailable) != 0 || !state.permits(ctx, "optional") {
					t.Fatalf("explicit original window lost: %v %v", loaded, unavailable)
				}
				item, _ := state.availabilityFor(ctx, "optional")
				if scenario != "yes" && (item.Admission != "catalog_fallback" || item.FallbackReason == "") {
					t.Fatalf("fallback not disclosed truthfully: %+v", item)
				}
			}
			defs, _ := state.definitions(ctx)
			want := 4

			if len(defs) != want {
				t.Fatalf("window unbounded: %v", toolNamesForTest(defs))
			}
			if scenario == "yes" {
				service.Settings.(*kevFixedSettings).config.Enabled = false
				if !state.permits(ctx, "optional") {
					t.Fatal("revoked disclosure blocked installed execution")
				}
				item, _ := state.availabilityFor(ctx, "optional")
				if item.WarningCode != "tool_selection_changed" || item.Status != "available" {
					t.Fatalf("actual revocation mislabeled: %+v", item)
				}
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
	if !state.permits(context.Background(), "optional.0") {
		t.Fatal("replaced disclosure blocked installed execution")
	}
	if loaded, missing, err := state.load(context.Background(), []string{"optional.0", "foreign"}); err != nil || len(loaded) != 1 || len(missing) != 1 || missing[0].Name != "foreign" {
		t.Fatal("mixed installed subset not recovered", loaded, missing, err)
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
			want := []string{capabilitycontract.CapabilityWireName(capabilityCatalogName), capabilitycontract.CapabilityWireName(conversationReplyCapabilityName), capabilitycontract.CapabilityWireName("moment.publish")}
			sort.Strings(want)
			if !reflect.DeepEqual(seen, want) {
				t.Fatalf("first physical request disclosed %d tools; want only %v; actual=%v", len(seen), want, seen)
			}

		})
	}
}
