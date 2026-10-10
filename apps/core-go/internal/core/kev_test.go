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
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

type kevFixedSettings struct{ config decision.Config }

func (s *kevFixedSettings) Read(context.Context) (decision.Config, int64, string, error) {
	return s.config, 1, "", nil
}

type kevMemoryStore struct{ records []decision.Record }

func (s *kevMemoryStore) Save(_ context.Context, r decision.Record) error {
	s.records = append(s.records, r)
	return nil
}

func TestKevContextFiltersOnlyOptionalAndRetainsSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request decision.Request
		_ = json.NewDecoder(r.Body).Decode(&request)
		answers := map[string]any{}
		for id := range request.Questions {
			answers[id] = map[string]any{"type": "choice", "choice": "no", "confidence": 0.5, "probabilities": map[string]float64{"yes": 0.1, "no": 0.8, "unclear": 0.1}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
	}))
	defer server.Close()
	config := decision.DefaultConfig()
	config.Enabled = true
	config.Endpoint = server.URL
	store := &kevMemoryStore{}
	settings := &kevFixedSettings{config}
	ids := 0
	app := &App{Kev: &decision.Service{Settings: settings, Store: store, HTTP: server.Client(), ID: func() string { ids++; return fmt.Sprint(ids) }}}
	input := WorkingMemoryInput{RuntimeFacts: []PromptFragment{{Kind: PromptFragmentRuntimeFact, Required: true, Content: "current facts", SourceRefs: []string{"facts"}}}, RetrievedMemories: []PromptFragment{{Kind: PromptFragmentRetrievedMemory, Content: "unrelated optional memory", SourceRefs: []string{"memory-1"}}}, RecentMessages: []PromptFragment{{Kind: PromptFragmentRecentMessage, Content: "native history"}}}
	selected, err := app.selectKevContext(context.Background(), ContextProjection{FluctlightID: "actor"}, ProviderContextSurfaceConversationMain, "input", input)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.RetrievedMemories) != 0 || len(selected.RuntimeFacts) != 1 || len(selected.RecentMessages) != 1 || len(input.RetrievedMemories) != 1 {
		t.Fatalf("required or source changed: %+v", selected)
	}
	settings.config.Enabled = false
	restored, err := app.selectKevContext(context.Background(), ContextProjection{}, ProviderContextSurfaceConversationMain, "input", input)
	if err != nil || len(restored.RetrievedMemories) != 1 || len(store.records) != 2 {
		t.Fatal("disabled did not restore original source", err)
	}
}

func TestKevContextAndToolBatchesSendOnlyTheirCandidateState(t *testing.T) {
	requests := []decision.Request{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request decision.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		requests = append(requests, request)
		answers := map[string]any{}
		for id := range request.Questions {
			answers[id] = map[string]any{"type": "choice", "choice": "yes", "confidence": 0.5, "probabilities": map[string]float64{"yes": 0.8, "no": 0.1, "unclear": 0.1}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
	}))
	defer server.Close()
	config := decision.DefaultConfig()
	config.Enabled = true
	config.Endpoint = server.URL
	store := &kevMemoryStore{}
	ids := 0
	app := &App{Kev: &decision.Service{Settings: &kevFixedSettings{config}, Store: store, HTTP: server.Client(), ID: func() string { ids++; return fmt.Sprint(ids) }}}

	input := WorkingMemoryInput{}
	for index := 0; index < 11; index++ {
		input.RetrievedMemories = append(input.RetrievedMemories, PromptFragment{Kind: PromptFragmentRetrievedMemory, Content: fmt.Sprintf("memory-%02d", index), SourceRefs: []string{fmt.Sprintf("source-%02d", index)}})
	}
	selected, err := app.selectKevContext(context.Background(), ContextProjection{FluctlightID: "actor"}, ProviderContextSurfaceConversationMain, "current task", input)
	if err != nil || len(selected.RetrievedMemories) != 11 || len(input.RetrievedMemories) != 11 {
		t.Fatal("context selection changed caller state", err)
	}
	assertBatches := func(offset int, stateKey string) {
		t.Helper()
		seen := map[string]bool{}
		for batch, want := range []int{8, 3} {
			request := requests[offset+batch]
			var state map[string]json.RawMessage
			if err := json.Unmarshal([]byte(request.State), &state); err != nil {
				t.Fatal(err)
			}
			var subset map[string]json.RawMessage
			if err := json.Unmarshal(state[stateKey], &subset); err != nil || len(request.Questions) != want || len(subset) != want {
				t.Fatalf("%s batch %d mismatch: questions=%d state=%d err=%v", stateKey, batch, len(request.Questions), len(subset), err)
			}
			for id := range subset {
				if seen[id] {
					t.Fatalf("%s repeated across physical requests: %s", stateKey, id)
				}
				seen[id] = true
			}
		}
		if len(seen) != 11 {
			t.Fatalf("%s lost batch state: %d", stateKey, len(seen))
		}
	}
	assertBatches(0, "candidates")

	definitions := make([]CapabilityDefinition, 11)
	for index := range definitions {
		definitions[index] = CapabilityDefinition{Name: fmt.Sprintf("test.capability.%02d", index), Version: "v1", Type: CapabilityTypeQuery, Description: fmt.Sprintf("capability-%02d", index), Surfaces: []CapabilitySurface{CapabilitySurfaceConversation}}
	}
	originalNames := make([]string, len(definitions))
	for index := range definitions {
		originalNames[index] = definitions[index].Name
	}
	_, prepared, err := app.prepareKevTools(context.WithValue(context.Background(), kevAssemblySelectionKey{}, true), ADKStructuredTaskInput{AgentID: FormalAgentConversationCognition, Definitions: definitions, Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "current task"}}}, Capability: &ADKCapabilityRequest{Projection: ContextProjection{FluctlightID: "actor"}, Surface: CapabilitySurfaceConversation}})
	if err != nil || len(prepared) != 13 {
		t.Fatal("tool selection failed", err)
	}
	for index := range definitions {
		if definitions[index].Name != originalNames[index] {
			t.Fatal("tool caller definitions mutated")
		}
	}
	if len(requests) != 4 {
		t.Fatalf("expected context 8+3 and tools 8+3, got %d requests", len(requests))
	}
	assertBatches(2, "capabilities")
}

func TestKevToolSelectionAlwaysDisclosesOnlyMandatoryToolsAndOffersEveryOptionalTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request decision.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		answers := map[string]any{}
		for id := range request.Questions {
			answers[id] = map[string]any{"type": "choice", "choice": "no", "confidence": 0.8, "probabilities": map[string]float64{"yes": 0.1, "no": 0.8, "unclear": 0.1}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
	}))
	defer server.Close()
	config := decision.DefaultConfig()
	config.Enabled = true
	config.Endpoint = server.URL
	store := &kevMemoryStore{}
	ids := 0
	app := &App{Kev: &decision.Service{Settings: &kevFixedSettings{config}, Store: store, HTTP: server.Client(), ID: func() string { ids++; return fmt.Sprint(ids) }}}
	definitions := []CapabilityDefinition{
		{Name: conversationReplyCapabilityName, Description: "reply", OutputRole: "conversation_message"},
		{Name: "moment.publish", Description: "moment", OutputRole: "moment"},
		{Name: "media.image.generate", Description: "image", OutputRole: "media"},
		{Name: "visual_identity.initialize", Description: "initialize visual identity"},
		{Name: "reply.like.optional", Description: "another message output", OutputRole: "conversation_message"},
		{Name: "memory.recall", Description: "recall"},
	}
	definitions = appendCapabilityUtilities(app.capabilityRegistry(), CapabilitySurfaceConversation, definitions)
	ctx, prepared, err := app.prepareKevTools(context.WithValue(context.Background(), kevAssemblySelectionKey{}, true), ADKStructuredTaskInput{
		AgentID: FormalAgentConversationCognition, Definitions: definitions,
		Prompt:     PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "current task"}}},
		Capability: &ADKCapabilityRequest{Projection: ContextProjection{FluctlightID: "actor"}, Surface: CapabilitySurfaceConversation},
	})
	if err != nil || len(prepared) != len(definitions) {
		t.Fatalf("prepare tools: count=%d err=%v", len(prepared), err)
	}
	selection := kevSelection(ctx)
	visible, err := selection.definitions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	visibleNames := map[string]bool{}
	for _, definition := range visible {
		visibleNames[definition.Name] = true
	}
	mandatory := formalAgentMandatoryTools(FormalAgentConversationCognition)
	for name := range mandatory {
		if !visibleNames[name] {
			t.Fatalf("mandatory Tool %q was hidden: %#v", name, visibleNames)
		}
	}
	for _, name := range []string{"visual_identity.initialize", "reply.like.optional", "memory.recall"} {
		if visibleNames[name] {
			t.Fatalf("Kev no did not hide optional Tool %q: %#v records=%#v", name, visibleNames, store.records)
		}
	}
	candidates := map[string]bool{}
	for _, record := range store.records {
		candidates[record.CandidateID] = true
	}
	if len(candidates) != 3 || !candidates["visual_identity.initialize"] || !candidates["reply.like.optional"] || !candidates["memory.recall"] {
		t.Fatalf("optional Tool candidates=%#v", candidates)
	}
	for _, name := range []string{capabilityCatalogName, capabilityDiscoverName} {
		if !visibleNames[name] {
			t.Fatalf("catalog utility %q was hidden: %#v", name, visibleNames)
		}
	}
	for name := range mandatory {
		if candidates[name] {
			t.Fatalf("mandatory Tool %q became a Kev candidate", name)
		}
	}
	for _, name := range []string{capabilityCatalogName, capabilityDiscoverName} {
		if candidates[name] {
			t.Fatalf("recovery utility %q became a Kev candidate", name)
		}
	}
}

type kevReadSequenceSettings struct {
	config decision.Config
	reads  int
}

func (s *kevReadSequenceSettings) Read(context.Context) (decision.Config, int64, string, error) {
	s.reads++
	if s.reads > 1 {
		return decision.Config{}, 0, "", errors.New("settings temporarily unavailable")
	}
	return s.config, 1, "", nil
}

func TestKevToolSelectionFallbackKeepsRecoveryUtilitiesAndToolFreeAgentsStayEmpty(t *testing.T) {
	config := decision.DefaultConfig()
	config.Enabled = true
	settings := &kevReadSequenceSettings{config: config}
	app := &App{Kev: &decision.Service{Settings: settings, Store: &kevMemoryStore{}}}
	input := ADKStructuredTaskInput{
		AgentID:     FormalAgentConversationCognition,
		Definitions: []CapabilityDefinition{{Name: "memory.recall", Description: "recall"}},
		Capability:  &ADKCapabilityRequest{Surface: CapabilitySurfaceConversation},
	}
	_, prepared, err := app.prepareKevTools(context.WithValue(context.Background(), kevAssemblySelectionKey{}, true), input)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, definition := range prepared {
		names[definition.Name] = true
	}
	for _, name := range []string{"memory.recall", capabilityCatalogName, capabilityDiscoverName} {
		if !names[name] {
			t.Fatalf("fallback removed %q: %#v", name, names)
		}
	}

	_, prepared, err = app.prepareKevTools(context.WithValue(context.Background(), kevAssemblySelectionKey{}, true), ADKStructuredTaskInput{AgentID: FormalAgentInitialization})
	if err != nil || len(prepared) != 0 {
		t.Fatalf("tool-free Agent received recovery utilities: prepared=%#v err=%v", prepared, err)
	}
}

func TestKevPhysicalRequestFallbackKeepsRecoveryUtilities(t *testing.T) {
	for _, cause := range []string{"disabled", "version_changed", "settings_error"} {
		t.Run(cause, func(t *testing.T) {
			config := decision.DefaultConfig()
			config.Enabled = cause != "disabled"
			service := &decision.Service{Settings: &kevFixedSettings{config}}
			version := int64(1)
			if cause == "version_changed" {
				version = 2
			}
			if cause == "settings_error" {
				service.Settings = &kevReadSequenceSettings{config: config, reads: 1}
			}
			selection := &kevRunSelection{service: service, original: []CapabilityDefinition{{Name: "memory.recall"}}, visible: map[string]bool{}, loaded: map[string]bool{}, version: version}
			definitions, err := selection.definitions(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			names := map[string]bool{}
			for _, definition := range definitions {
				names[definition.Name] = true
			}
			for _, name := range []string{"memory.recall", capabilityCatalogName, capabilityDiscoverName} {
				if !names[name] || !selection.permits(context.Background(), name) {
					t.Fatalf("fallback removed %s: %#v", name, names)
				}
			}
		})
	}
}

func TestCapabilityCatalogReturnsRunScopedPurposesAndParametersWithoutLoading(t *testing.T) {
	definition := CapabilityDefinition{Name: "allowed", Description: "Read the allowed fact.", InputSchema: objectSchema(map[string]any{"id": stringSchema()}, []string{"id"}, false)}
	state := &kevRunSelection{original: []CapabilityDefinition{definition}, loaded: map[string]bool{}}
	ctx := context.WithValue(context.Background(), kevRunSelectionKey{}, state)
	capability := capabilityCatalogCapability{catalog: func(CapabilitySurface) []CapabilityDefinition {
		t.Fatal("run-scoped catalog read ambient surface catalog")
		return nil
	}}
	result, err := capability.Execute(ctx, CapabilityInvocation{CallID: "catalog", CapabilityName: capabilityCatalogName}, CapabilityContext{})
	items := arrayValue(mapValue(result.Output)["items"])
	if err != nil || len(items) != 1 || stringValue(mapValue(items[0])["name"]) != "allowed" || stringValue(mapValue(items[0])["purpose"]) != definition.Description || len(mapValue(mapValue(items[0])["parameters"])) == 0 {
		t.Fatalf("catalog result=%#v err=%v", result, err)
	}
	if len(state.loaded) != 0 {
		t.Fatalf("catalog query loaded schemas: %#v", state.loaded)
	}
}

func TestKevDedicatedAgentCatalogsBypassSelection(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	config := decision.DefaultConfig()
	config.Enabled = true
	config.Endpoint = server.URL
	app := &App{Kev: &decision.Service{Settings: &kevFixedSettings{config}, Store: &kevMemoryStore{}, HTTP: server.Client()}}
	definitions := []CapabilityDefinition{{Name: "dedicated.tool", Description: "dedicated"}}
	for _, agentID := range []FormalAgentID{FormalAgentGoalPlanner, FormalAgentGoalEvaluation, FormalAgentVisualIdentity, FormalAgentVisualIdentityVision, FormalAgentVisualIdentityPatch} {
		ctx, prepared, err := app.prepareKevTools(context.WithValue(context.Background(), kevAssemblySelectionKey{}, true), ADKStructuredTaskInput{AgentID: agentID, Definitions: definitions})
		if err != nil || kevSelection(ctx) != nil || len(prepared) != 1 || prepared[0].Name != "dedicated.tool" {
			t.Fatalf("agent %s catalog passed through Kev: prepared=%#v err=%v", agentID, prepared, err)
		}
	}
	if requests != 0 {
		t.Fatalf("dedicated catalogs made %d Kev requests", requests)
	}
}

func TestKevSpoolIsDurableBoundedAndContainsRaw(t *testing.T) {
	store := &kevStore{directory: t.TempDir()}
	at := time.Now().UTC()
	record := decision.Record{ID: "record", RequestID: "request", QuestionID: "q_0001", Point: "tools.select", RawResponse: `{"answers":{"q_0001":{"choice":"yes"}}}`, StartedAt: at, CompletedAt: at, RecordedAt: at, Outcome: decision.Yes}
	if err := store.Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(store.directory, "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var restored decision.Record
	if err := json.Unmarshal(raw, &restored); err != nil || restored.RawResponse != record.RawResponse {
		t.Fatal(restored, err)
	}
	f, err := os.OpenFile(filepath.Join(store.directory, "decisions.jsonl"), os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(0); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(strings.Repeat(string(raw), kevSpoolLimit/len(raw))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := store.Save(context.Background(), record); err == nil {
		t.Fatal("full journal accepted an unrecorded result")
	}
}

func TestKevSpoolRepairsOnlyIncompleteTerminalAppend(t *testing.T) {
	store := &kevStore{directory: t.TempDir()}
	ctx := context.Background()
	record := decision.Record{ID: "one", RawResponse: "complete original output"}
	if err := store.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.directory, "decisions.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`{"id":"incomplete`)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	record.ID = "two"
	if err := store.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("valid rows lost: %s", data)
	}
	for _, line := range lines {
		var decoded decision.Record
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatal(err)
		}
	}
	quarantine, err := os.ReadFile(filepath.Join(store.directory, "incomplete-tail.json"))
	if err != nil || !strings.Contains(string(quarantine), "incomplete") {
		t.Fatal("terminal fragment was not retained", err)
	}
}

func TestKevDiscoveryNeverWidensInstalledSetOrPartiallyLoads(t *testing.T) {
	state := &kevRunSelection{original: []CapabilityDefinition{{Name: "allowed", Description: "authorized"}}, loaded: map[string]bool{}}
	ctx := context.WithValue(context.Background(), kevRunSelectionKey{}, state)
	capability := capabilityDiscoverCapability{catalog: func(CapabilitySurface) []CapabilityDefinition {
		t.Fatal("run-scoped discovery read ambient surface catalog")
		return nil
	}}
	result, err := capability.Execute(ctx, CapabilityInvocation{CallID: "call", CapabilityName: capabilityDiscoverName, Arguments: json.RawMessage(`{"names":[]}`)}, CapabilityContext{})
	if err != nil || len(arrayValue(mapValue(result.Output)["available"])) != 1 {
		t.Fatal(result, err)
	}
	result, err = capability.Execute(ctx, CapabilityInvocation{CallID: "call", CapabilityName: capabilityDiscoverName, Arguments: json.RawMessage(`{"names":["allowed","foreign"]}`)}, CapabilityContext{})
	if err != nil || result.ErrorCode != "capability_not_authorized" || len(state.loaded) != 0 {
		t.Fatal("discovery widened/partially loaded", result, err)
	}
}

type kevDiscoveryInvoker struct {
	state      *kevRunSelection
	executions int
}

type kevControlledPersonaDomain struct {
	commits int
	request ToolExecutionRequest
}

func (d *kevControlledPersonaDomain) preparePersonalityDecision(_ context.Context, owner string, _ map[string]any) (*personalityDecisionPlan, error) {
	return &personalityDecisionPlan{ExpectedRevision: 0, FluctlightID: owner}, nil
}
func (d *kevControlledPersonaDomain) ExecuteTool(_ context.Context, r ToolExecutionRequest) (ToolExecutionReceipt, error) {
	d.commits++
	d.request = r
	return ToolExecutionReceipt{ExecutionCallID: "direct-receipt", Result: CapabilityResult{Status: "completed", Output: map[string]any{"disposition": "applied"}}}, nil
}
func TestKevPersonaAdmissionDropsEntireBatchOrBlocksSwitch(t *testing.T) {
	for _, choice := range []string{"yes", "no", "unclear"} {
		t.Run(choice, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request decision.Request
				_ = json.NewDecoder(r.Body).Decode(&request)
				answers := map[string]any{}
				for id := range request.Questions {
					p := map[string]float64{"yes": 0.1, "no": 0.1, "unclear": 0.1}
					p[choice] = 0.8
					answers[id] = map[string]any{"type": "choice", "choice": choice, "confidence": 0.4, "probabilities": p}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
			}))
			defer server.Close()
			config := decision.DefaultConfig()
			config.Enabled = true
			config.Endpoint = server.URL
			ids := 0
			store := &kevMemoryStore{}
			app := &App{Kev: &decision.Service{Settings: &kevFixedSettings{config}, Store: store, HTTP: server.Client(), ID: func() string { ids++; return fmt.Sprint(ids) }}}
			persona := multiProfileTestPersona()
			mapValue(persona["personality_system"])["switching"] = map[string]any{"rules": []any{map[string]any{"id": "test-rule", "condition": "The user explicitly asks for Spark", "source_profile_id": "twilight", "target_profile_id": "spark", "priority": 5}}}
			domain := &kevControlledPersonaDomain{}
			gate := &kevPersonaAdmission{app: app, domain: domain, denied: map[string]bool{}, request: ADKCapabilityRequest{AuthorizationActorID: "owner", FluctlightID: "actor", SourceFactID: "event", Surface: CapabilitySurfaceConversation, Projection: ContextProjection{FluctlightID: "actor", CorePersona: persona, PersonalityRuntime: map[string]any{"active_profile_id": "twilight", "revision": 0}, CurrentUserText: "Please let Spark respond"}}}
			refresh := &runtimeContextRefresh{}
			ctx := withADKCapabilityContext(context.Background(), &adkFakeInvoker{}, &ADKCapabilityTrace{}, refresh)
			ctx = context.WithValue(ctx, kevPersonaKey{}, gate)
			batch := schema.AssistantMessage("A proposal", []schema.ToolCall{{ID: "reply", Function: schema.FunctionCall{Name: conversationReplyCapabilityName, Arguments: `{"text":"A reply"}`}}, {ID: "switch", Function: schema.FunctionCall{Name: personaSwitchCapabilityName, Arguments: `{"trigger_id":"switch:test-rule","target_profile_id":"spark"}`}}})
			candidate, regenerate, err := gate.inspect(ctx, batch)
			if err != nil {
				t.Fatal(err)
			}
			if choice == "yes" {
				if !regenerate || candidate != nil || domain.commits != 1 || domain.request.NativeToolCallID != "" || domain.request.ProviderRequestID != "" || !refresh.dirty {
					t.Fatalf("old A batch escaped or native identity fabricated: candidate=%+v regenerate=%t commits=%d", candidate, regenerate, domain.commits)
				}
				if rejection := kevRejectNativeTool(ctx, personaSwitchCapabilityName, json.RawMessage(`{"trigger_id":"switch:test-rule"}`)); rejection != "kev_persona_already_switched" {
					t.Fatal(rejection)
				}
			} else {
				if regenerate || candidate != batch || domain.commits != 0 {
					t.Fatal("no/abstention committed a switch")
				}
				rejection := kevRejectNativeTool(ctx, personaSwitchCapabilityName, json.RawMessage(`{"trigger_id":"switch:test-rule"}`))
				if choice == "no" && rejection != "kev_persona_condition_false" {
					t.Fatal("no was overridden", rejection)
				}
				if choice == "unclear" && rejection != "" {
					t.Fatal("abstention blocked original switch", rejection)
				}
			}
			if len(batch.ToolCalls) != 2 || batch.Content != "A proposal" {
				t.Fatal("original diagnostic proposal was mutated")
			}
		})
	}
}
func TestKevPersonaRequiresInstalledToolAndRefresh(t *testing.T) {
	if kevPersonaInstalled([]CapabilityDefinition{{Name: "memory.recall"}}) || !kevPersonaInstalled([]CapabilityDefinition{{Name: personaSwitchCapabilityName}}) {
		t.Fatal("Agent tool installation authority lost")
	}
	c := decision.DefaultConfig()
	c.Enabled = true
	app := &App{Kev: &decision.Service{Settings: &kevFixedSettings{c}}}
	domain := &kevControlledPersonaDomain{}
	gate := &kevPersonaAdmission{app: app, domain: domain}
	message := schema.AssistantMessage("proposal", nil)
	result, regen, err := gate.inspect(context.Background(), message)
	if err != nil || regen || result != message || domain.commits != 0 {
		t.Fatal("missing refresh committed business state", err)
	}
}

func (i *kevDiscoveryInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "call", name, args)
}
func (i *kevDiscoveryInvoker) ExecuteWithID(ctx context.Context, id, name, args string) (string, error) {
	if name == capabilityDiscoverName {
		capability := capabilityDiscoverCapability{catalog: func(CapabilitySurface) []CapabilityDefinition { return i.state.original }}
		result, err := capability.Execute(ctx, CapabilityInvocation{CallID: id, CapabilityName: name, Arguments: json.RawMessage(args), Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation}}, CapabilityContext{})
		return jsonString(result), err
	}
	i.executions++
	return `{"status":"completed","output":{"actual":"hidden-service-result"}}`, nil
}
func TestKevNativeLoopDiscoversAndExecutesHiddenTool(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			var mu sync.Mutex
			requests := [][]string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				names := []string{}
				for _, raw := range arrayValue(body["tools"]) {
					names = append(names, stringValue(mapValue(mapValue(raw)["function"])["name"]))
				}
				mu.Lock()
				requests = append(requests, names)
				n := len(requests)
				mu.Unlock()
				message := map[string]any{"role": "assistant", "content": "done"}
				finish := "stop"
				if n < 3 {
					name := capabilityDiscoverName
					args := `{"names":["hidden"]}`
					if n == 2 {
						name = "hidden"
						args = "{}"
					}
					message = map[string]any{"role": "assistant", "tool_calls": []map[string]any{{"index": 0, "id": fmt.Sprint("call_", n), "type": "function", "function": map[string]any{"name": name, "arguments": args}}}}
					finish = "tool_calls"
				}
				if body["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					encoded := jsonString(map[string]any{"id": "response", "model": "test", "choices": []map[string]any{{"index": 0, "delta": message, "finish_reason": finish}}})
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", encoded)
				} else {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"id": "response", "model": "test", "choices": []map[string]any{{"index": 0, "message": message, "finish_reason": finish}}})
				}
			}))
			defer server.Close()
			config := decision.DefaultConfig()
			config.Enabled = true
			state := &kevRunSelection{service: &decision.Service{Settings: &kevFixedSettings{config}}, original: []CapabilityDefinition{{Name: "hidden", Version: "v1", Type: CapabilityTypeQuery, Description: "Read hidden service", Surfaces: []CapabilitySurface{CapabilitySurfaceConversation}}}, visible: map[string]bool{}, loaded: map[string]bool{}, version: 1}
			ctx := context.WithValue(context.Background(), kevRunSelectionKey{}, state)
			chat, err := NewEinoModelFactory(server.Client()).NewChatModel(ctx, EinoModelConfig{APIKey: "test", BaseURL: server.URL + "/v1", Model: "test", HTTPClient: server.Client(), Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			invoker := &kevDiscoveryInvoker{state: state}
			defs := append(append([]CapabilityDefinition(nil), state.original...), (capabilityCatalogCapability{}).Definition(), (capabilityDiscoverCapability{}).Definition())
			tools, err := NewADKCapabilityTools(defs, invoker)
			if err != nil {
				t.Fatal(err)
			}
			model := &queuedToolCallingChatModel{inner: chat, provider: &ProviderClient{HTTP: server.Client()}, role: "cognitive_assessment", assignment: providerAssignment{TokenBudget: 4096, ContextWindowTokens: 131072, MaxInputTokens: 98304, PromptBudgetPolicyVersion: "prompt-budget.v1"}}
			result, err := RunADKLoop(ctx, ADKLoopConfig{Name: "kev-test", Description: "test actual feedback", Model: model, Tools: tools, MaxIterations: 4, EnableStreaming: streaming}, []*schema.Message{schema.UserMessage("Load hidden and call it")})
			if err != nil {
				t.Fatal(err)
			}
			if invoker.executions != 1 || len(result.ToolResults) != 2 || len(requests) != 3 || !phase8EqualStrings(requests[0], []string{capabilityCatalogName, capabilityDiscoverName}) || !phase8EqualStrings(requests[1], []string{"hidden", capabilityCatalogName, capabilityDiscoverName}) {
				t.Fatalf("no actual growth/feedback: wire=%v executions=%d results=%d", requests, invoker.executions, len(result.ToolResults))
			}
		})
	}
}

func TestKevDiscoveryUsesIndependentPureQueryBoundary(t *testing.T) {
	for _, implementation := range []Capability{capabilityCatalogCapability{}, capabilityDiscoverCapability{}} {
		class, err := classifyCapabilityExecution(implementation, implementation.Definition())
		if err != nil || class != CapabilityExecutionPureQuery {
			t.Fatal("catalog utility cannot execute through actual Tool boundary", implementation.Definition().Name, class, err)
		}
	}
}

type kevReplyTestInvoker struct{ arguments []string }

func (i *kevReplyTestInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "", name, args)
}
func (i *kevReplyTestInvoker) ExecuteWithID(_ context.Context, _, _, args string) (string, error) {
	i.arguments = append(i.arguments, args)
	return `{"status":"completed"}`, nil
}

func TestKevNativePersonaLoopExecutesOnlyRegeneratedB(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			kevServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request decision.Request
				_ = json.NewDecoder(r.Body).Decode(&request)
				answers := map[string]any{}
				for id := range request.Questions {
					answers[id] = map[string]any{"type": "choice", "choice": "yes", "confidence": 0.4, "probabilities": map[string]float64{"yes": 0.8, "no": 0.1, "unclear": 0.1}}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
			}))
			defer kevServer.Close()
			calls := 0
			modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				calls++
				message := map[string]any{"role": "assistant", "content": "done"}
				finish := "stop"
				if calls < 3 {
					owner := "A"
					if calls == 2 {
						owner = "B"
					}
					message = map[string]any{"role": "assistant", "tool_calls": []map[string]any{{"index": 0, "id": "reply-" + owner, "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(map[string]any{"text": owner + " reply"})}}}}
					finish = "tool_calls"
				}
				if body["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"id": "response", "model": "test", "choices": []map[string]any{{"index": 0, "delta": message, "finish_reason": finish}}}))
				} else {
					_ = json.NewEncoder(w).Encode(map[string]any{"id": "response", "model": "test", "choices": []map[string]any{{"index": 0, "message": message, "finish_reason": finish}}})
				}
			}))
			defer modelServer.Close()
			config := decision.DefaultConfig()
			config.Enabled = true
			config.Endpoint = kevServer.URL
			ids := 0
			application := &App{Kev: &decision.Service{Settings: &kevFixedSettings{config}, Store: &kevMemoryStore{}, HTTP: kevServer.Client(), ID: func() string { ids++; return fmt.Sprint(ids) }}}
			persona := multiProfileTestPersona()
			mapValue(persona["personality_system"])["switching"] = map[string]any{"rules": []any{map[string]any{"id": "test-rule", "condition": "The user requests Spark", "source_profile_id": "twilight", "target_profile_id": "spark"}}}
			domain := &kevControlledPersonaDomain{}
			gate := &kevPersonaAdmission{app: application, domain: domain, denied: map[string]bool{}, request: ADKCapabilityRequest{AuthorizationActorID: "owner", FluctlightID: "actor", SourceFactID: "event", Projection: ContextProjection{FluctlightID: "actor", CorePersona: persona, PersonalityRuntime: map[string]any{"active_profile_id": "twilight", "revision": 0}, CurrentUserText: "Spark please"}}}
			invoker := &kevReplyTestInvoker{}
			trace := &ADKCapabilityTrace{}
			refresh := &runtimeContextRefresh{refresh: func(context.Context) (modelContextRefreshContent, error) {
				return modelContextRefreshContent{System: "B persona", Runtime: "[RUNTIME CONTEXT]\n{}\n[/RUNTIME CONTEXT]"}, nil
			}}
			ctx := withADKCapabilityContext(context.Background(), invoker, trace, refresh)
			ctx = context.WithValue(ctx, kevPersonaKey{}, gate)
			chat, err := NewEinoModelFactory(modelServer.Client()).NewChatModel(ctx, EinoModelConfig{APIKey: "test", BaseURL: modelServer.URL + "/v1", Model: "test", HTTPClient: modelServer.Client(), Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			definitions := []CapabilityDefinition{{Name: conversationReplyCapabilityName, Version: "v1", Description: "publish actual reply", InputSchema: objectSchema(map[string]any{"text": stringSchema()}, []string{"text"}, false)}}
			tools, err := NewADKCapabilityTools(definitions, invoker)
			if err != nil {
				t.Fatal(err)
			}
			model := &queuedToolCallingChatModel{inner: chat, provider: &ProviderClient{HTTP: modelServer.Client()}, role: "cognitive_assessment", assignment: providerAssignment{TokenBudget: 4096, ContextWindowTokens: 131072, MaxInputTokens: 98304, PromptBudgetPolicyVersion: "prompt-budget.v1"}}
			result, err := RunADKLoop(ctx, ADKLoopConfig{Name: "persona-test", Description: "before side effects", Model: model, Tools: tools, MaxIterations: 4, EnableStreaming: streaming}, []*schema.Message{schema.SystemMessage("A persona"), schema.UserMessage("[RUNTIME CONTEXT]\n{}\n[/RUNTIME CONTEXT]"), schema.UserMessage("Spark please")})
			if err != nil {
				t.Fatal(err)
			}
			if domain.commits != 1 || len(invoker.arguments) != 1 || !strings.Contains(invoker.arguments[0], "B reply") || len(result.ToolCalls) != 1 || result.ToolCalls[0].ID != "reply-B" || calls != 3 {
				t.Fatalf("A batch reached loop/publication: commits=%d args=%v calls=%d trace=%v", domain.commits, invoker.arguments, calls, result.ToolCalls)
			}
		})
	}
}
