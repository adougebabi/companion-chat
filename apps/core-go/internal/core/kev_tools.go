package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/model"
	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

const capabilityDiscoverName = "capability.discover"

type kevRunSelectionKey struct{}
type kevRunSelection struct {
	mu       sync.Mutex
	service  *decision.Service
	original []CapabilityDefinition
	visible  map[string]bool
	loaded   map[string]bool
	version  int64
}

func kevSelection(ctx context.Context) *kevRunSelection {
	value, _ := ctx.Value(kevRunSelectionKey{}).(*kevRunSelection)
	return value
}

// The independent discovery capability receives only a registry reader. It
// reports available tools when used directly; loading is scoped to an Agent.
type capabilityDiscoverCapability struct {
	catalog func(CapabilitySurface) []CapabilityDefinition
}

func (c capabilityDiscoverCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: capabilityDiscoverName, Version: "v1", Type: CapabilityTypeQuery, Description: "Discover permitted capabilities; provide names to load them for the next model request.", InputSchema: objectSchema(map[string]any{"names": map[string]any{"type": "array", "maxItems": 8, "items": stringSchema()}}, nil, false), OutputSchema: openObjectSchema(), Surfaces: []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition}, SupportsCancel: true, FailurePolicy: FailurePolicyOptionalInternal, SideEffectClass: "read_only", ConcurrencyClass: "parallel", SuccessBoundary: "catalog_read"}
}
func (c capabilityDiscoverCapability) RequiredContext() []ContextSlot { return nil }
func (c capabilityDiscoverCapability) Execute(ctx context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return CapabilityResult{}, err
	}
	if c.catalog == nil {
		return CapabilityResult{}, errors.New("capability_catalog_unavailable")
	}
	var args struct {
		Names []string `json:"names"`
	}
	if err := json.Unmarshal(inv.Arguments, &args); err != nil {
		return CapabilityResult{}, err
	}
	var definitions []CapabilityDefinition
	selection := kevSelection(ctx)
	if selection != nil {
		selection.mu.Lock()
		definitions = append([]CapabilityDefinition(nil), selection.original...)
		selection.mu.Unlock()
	} else {
		definitions = c.catalog(inv.Metadata.Surface)
	}
	available := map[string]CapabilityDefinition{}
	items := []map[string]any{}
	for _, d := range definitions {
		available[d.Name] = d
		items = append(items, map[string]any{"name": d.Name, "description": d.Description})
	}
	loaded := []string{}
	// Validate the whole request before mutating any run-local visible set.
	for _, name := range args.Names {
		if _, ok := available[name]; !ok {
			return failedCapabilityResultDetail(inv, "capability_not_authorized", false, "Requested capability is not available in this scope"), nil
		}
	}
	for _, name := range args.Names {
		if _, ok := available[name]; !ok {
			return failedCapabilityResultDetail(inv, "capability_not_authorized", false, "Requested capability is not available in this scope"), nil
		}
		if selection != nil {
			selection.mu.Lock()
			allowed := false
			for _, d := range selection.original {
				if d.Name == name {
					allowed = true
					break
				}
			}
			if allowed {
				selection.loaded[name] = true
				loaded = append(loaded, name)
			}
			selection.mu.Unlock()
			if !allowed {
				return failedCapabilityResultDetail(inv, "capability_not_authorized", false, "Requested capability was not installed for this Agent"), nil
			}
		}
	}
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", Output: map[string]any{"available": items, "loaded": loaded, "run_scoped": selection != nil}}, nil
}

func (a *App) prepareKevTools(ctx context.Context, input ADKStructuredTaskInput) (context.Context, []CapabilityDefinition, error) {
	if input.Capability != nil && input.Capability.Projection.KevTools != nil {
		state := input.Capability.Projection.KevTools
		discovery, _ := a.capabilityRegistry().Definition(capabilityDiscoverName)
		return context.WithValue(ctx, kevRunSelectionKey{}, state), append(append([]CapabilityDefinition(nil), state.original...), discovery), nil
	}
	if !kevAssemblySelection(ctx) {
		return ctx, input.Definitions, ctx.Err()
	}
	service := a.kevService()
	if len(input.Definitions) == 0 || !service.Enabled(ctx, "tools.select") {
		return ctx, input.Definitions, ctx.Err()
	}
	discovery, ok := a.capabilityRegistry().Definition(capabilityDiscoverName)
	if !ok || !discovery.SupportsSurface(firstCapabilitySurface(input.Capability)) {
		return ctx, input.Definitions, nil
	}
	config, version, _, err := service.Settings.Read(ctx)
	if err != nil {
		return ctx, input.Definitions, nil
	}
	_ = config
	state := &kevRunSelection{service: service, original: append([]CapabilityDefinition(nil), input.Definitions...), visible: map[string]bool{}, loaded: map[string]bool{}, version: version}
	current := ""
	for _, m := range input.Prompt.Messages {
		if stringValue(m["role"]) == "user" {
			current = stringValue(m["content"])
		}
	}
	candidates := []decision.Candidate{}
	summaries := map[string]string{}
	for _, d := range input.Definitions {
		// Visible publication exits remain available. Dependency data continues
		// to be resolved by the existing ContextResolver at actual execution.
		state.visible[d.Name] = true
		if d.OutputRole == "conversation_message" || d.Name == conversationReplyCapabilityName {
			continue
		}
		candidates = append(candidates, decision.Candidate{ID: d.Name, Question: decision.Choice("Should capability " + d.Name + " be disclosed for the current task? Multiple capabilities may be needed.")})
		summaries[d.Name] = d.Description
	}
	projection := input.Capability.Projection
	results, err := service.DecideWithBatchState(ctx, "tools.select", a.kevScope(ctx, projection, string(input.AgentID)), candidates, func(batch []decision.Candidate) string {
		batchSummaries := make(map[string]string, len(batch))
		for _, candidate := range batch {
			batchSummaries[candidate.ID] = summaries[candidate.ID]
		}
		return jsonString(map[string]any{"input": current, "capabilities": batchSummaries})
	})
	if err != nil {
		return ctx, nil, err
	}
	for index := range results {
		r := &results[index]
		out, err := r.Admit(ctx)
		if err != nil {
			return ctx, nil, err
		}
		state.visible[r.Record.CandidateID] = out != decision.No
		status := "applied"
		if out == decision.Original {
			status = "original_used"
		}
		_ = r.Finish(ctx, status, "", "")
	}
	definitions := append(append([]CapabilityDefinition(nil), input.Definitions...), discovery)
	return context.WithValue(ctx, kevRunSelectionKey{}, state), definitions, nil
}

func capabilitySurfaceForProviderSurface(surface ProviderContextSurface) CapabilitySurface {
	switch surface {
	case ProviderContextSurfaceWakeUp:
		return CapabilitySurfaceWakeUp
	case ProviderContextSurfaceNativeCognition, ProviderContextSurfaceDailyReview:
		return CapabilitySurfaceNativeCognition
	case ProviderContextSurfaceReflection:
		return CapabilitySurfaceReflection
	default:
		return CapabilitySurfaceConversation
	}
}
func (s *kevRunSelection) definitions(ctx context.Context) ([]CapabilityDefinition, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c, version, _, err := s.service.Settings.Read(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil || version != s.version || !c.Allows("tools.select") {
		return append([]CapabilityDefinition(nil), s.original...), nil
	}
	out := []CapabilityDefinition{}
	for _, d := range s.original {
		if s.visible[d.Name] || s.loaded[d.Name] {
			out = append(out, d)
		}
	}
	out = append(out, (capabilityDiscoverCapability{}).Definition())
	return out, nil
}
func (s *kevRunSelection) permits(ctx context.Context, name string) bool {
	defs, err := s.definitions(ctx)
	if err != nil {
		return false
	}
	for _, d := range defs {
		if d.Name == name {
			return true
		}
	}
	return false
}

func (m *queuedToolCallingChatModel) kevModelOptions(ctx context.Context, opts []model.Option) (*queuedToolCallingChatModel, []model.Option, error) {
	selection := kevSelection(ctx)
	if selection == nil {
		return m, opts, nil
	}
	definitions, err := selection.definitions(ctx)
	if err != nil {
		return m, nil, err
	}
	infos, err := aiagent.CapabilityToolInfos(definitions)
	if err != nil {
		return m, nil, err
	}
	copyModel := *m
	copyModel.definitions = definitions
	return &copyModel, append(append([]model.Option(nil), opts...), model.WithTools(infos)), nil
}

// Kept separately from the Registry catalog so disabled assembly has precisely
// its pre-integration definitions, including ordering and serialized schemas.
func withoutKevDiscovery(definitions []CapabilityDefinition) []CapabilityDefinition {
	out := make([]CapabilityDefinition, 0, len(definitions))
	for _, d := range definitions {
		if strings.TrimSpace(d.Name) != capabilityDiscoverName {
			out = append(out, d)
		}
	}
	return out
}

type kevAssemblySelectionKey struct{}

func kevAssemblySelection(ctx context.Context) bool {
	value, _ := ctx.Value(kevAssemblySelectionKey{}).(bool)
	return value
}
