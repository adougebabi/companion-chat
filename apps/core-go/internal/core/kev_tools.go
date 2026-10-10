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

const (
	capabilityCatalogName  = "capability.catalog"
	capabilityDiscoverName = "capability.discover"
)

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

// capabilityCatalogCapability exposes the complete authorized installation on
// demand. Its own small schema is always visible on Kev-selected Agents; the
// potentially large business schemas remain outside the prompt until queried
// or explicitly loaded through capability.discover.
type capabilityCatalogCapability struct {
	catalog func(CapabilitySurface) []CapabilityDefinition
}

func capabilityUtilitySurfaces() []CapabilitySurface {
	return []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition}
}

func (c capabilityCatalogCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: capabilityCatalogName, Version: "v1", Type: CapabilityTypeQuery, Description: "List every capability authorized for this Agent, including each purpose and input parameters; use capability.discover to load selected schemas.", InputSchema: objectSchema(nil, nil, false), OutputSchema: openObjectSchema(), Surfaces: capabilityUtilitySurfaces(), SupportsCancel: true, FailurePolicy: FailurePolicyOptionalInternal, SideEffectClass: "read_only", ConcurrencyClass: "parallel", SuccessBoundary: "catalog_read"}
}
func (c capabilityCatalogCapability) RequiredContext() []ContextSlot { return nil }
func (c capabilityCatalogCapability) Execute(ctx context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return CapabilityResult{}, err
	}
	if c.catalog == nil {
		return CapabilityResult{}, errors.New("capability_catalog_unavailable")
	}
	definitions := authorizedDiscoveryDefinitions(ctx, inv.Metadata.Surface, c.catalog)
	items := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		parameters := definition.InputSchema
		if parameters == nil {
			parameters = objectSchema(nil, nil, false)
		}
		items = append(items, map[string]any{"name": definition.Name, "purpose": definition.Description, "parameters": parameters})
	}
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", Output: map[string]any{"items": items, "count": len(items), "run_scoped": kevSelection(ctx) != nil}}, nil
}

func (c capabilityDiscoverCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: capabilityDiscoverName, Version: "v1", Type: CapabilityTypeQuery, Description: "Load selected authorized capability schemas for the next model request; use capability.catalog first when names or parameters are unknown.", InputSchema: objectSchema(map[string]any{"names": map[string]any{"type": "array", "minItems": 1, "maxItems": 8, "items": stringSchema()}}, []string{"names"}, false), OutputSchema: openObjectSchema(), Surfaces: capabilityUtilitySurfaces(), SupportsCancel: true, FailurePolicy: FailurePolicyOptionalInternal, SideEffectClass: "read_only", ConcurrencyClass: "parallel", SuccessBoundary: "catalog_read"}
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
	definitions := authorizedDiscoveryDefinitions(ctx, inv.Metadata.Surface, c.catalog)
	selection := kevSelection(ctx)
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

func authorizedDiscoveryDefinitions(ctx context.Context, surface CapabilitySurface, catalog func(CapabilitySurface) []CapabilityDefinition) []CapabilityDefinition {
	selection := kevSelection(ctx)
	if selection != nil {
		selection.mu.Lock()
		defer selection.mu.Unlock()
		return append([]CapabilityDefinition(nil), selection.original...)
	}
	return catalog(surface)
}

func (a *App) prepareKevTools(ctx context.Context, input ADKStructuredTaskInput) (context.Context, []CapabilityDefinition, error) {
	if kevToolSelectionBypassed(input.AgentID) {
		return ctx, input.Definitions, ctx.Err()
	}
	// A naturally tool-free typed task has no capability surface to recover.
	// firstCapabilitySurface intentionally defaults nil requests to conversation
	// for legacy validation, so handle this boundary before adding utilities.
	if len(input.Definitions) == 0 {
		return ctx, nil, ctx.Err()
	}
	if input.Capability != nil && input.Capability.Projection.KevTools != nil {
		state := input.Capability.Projection.KevTools
		return context.WithValue(ctx, kevRunSelectionKey{}, state), appendCapabilityUtilities(a.capabilityRegistry(), input.Capability.Surface, state.original), nil
	}
	surface := firstCapabilitySurface(input.Capability)
	withUtilities := appendCapabilityUtilities(a.capabilityRegistry(), surface, input.Definitions)
	businessDefinitions := withoutKevDiscovery(input.Definitions)
	if !kevAssemblySelection(ctx) {
		return ctx, withUtilities, ctx.Err()
	}
	service := a.kevService()
	if len(businessDefinitions) == 0 || !service.Enabled(ctx, "tools.select") {
		return ctx, withUtilities, ctx.Err()
	}
	if !capabilityUtilitiesInstalled(a.capabilityRegistry(), surface) {
		return ctx, input.Definitions, nil
	}
	config, version, _, err := service.Settings.Read(ctx)
	if err != nil {
		return ctx, withUtilities, nil
	}
	_ = config
	state := &kevRunSelection{service: service, original: append([]CapabilityDefinition(nil), businessDefinitions...), visible: map[string]bool{}, loaded: map[string]bool{}, version: version}
	current := ""
	for _, m := range input.Prompt.Messages {
		if stringValue(m["role"]) == "user" {
			current = stringValue(m["content"])
		}
	}
	candidates := []decision.Candidate{}
	summaries := map[string]string{}
	mandatory := formalAgentMandatoryTools(input.AgentID)
	for _, d := range businessDefinitions {
		// Every installed Tool starts from the original visible set so fallback is
		// exactly the pre-Kev catalog. Each Agent owns its mandatory intersection;
		// every remaining installed Tool is its own independent Kev candidate.
		state.visible[d.Name] = true
		if mandatory[d.Name] {
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
	definitions := appendCapabilityUtilities(a.capabilityRegistry(), surface, input.Definitions)
	return context.WithValue(ctx, kevRunSelectionKey{}, state), definitions, nil
}

func formalAgentMandatoryTools(agentID FormalAgentID) map[string]bool {
	definition, ok := FormalAgentDefinitionByID(agentID)
	if !ok {
		return map[string]bool{}
	}
	result := make(map[string]bool, len(definition.MandatoryTools))
	for _, name := range definition.MandatoryTools {
		result[name] = true
	}
	return result
}

func capabilityUtilitiesInstalled(registry *CapabilityRegistry, surface CapabilitySurface) bool {
	for _, name := range []string{capabilityCatalogName, capabilityDiscoverName} {
		definition, ok := registry.Definition(name)
		if !ok || !definition.SupportsSurface(surface) {
			return false
		}
	}
	return true
}

func appendCapabilityUtilities(registry *CapabilityRegistry, surface CapabilitySurface, definitions []CapabilityDefinition) []CapabilityDefinition {
	result := append([]CapabilityDefinition(nil), definitions...)
	seen := make(map[string]bool, len(result))
	for _, definition := range result {
		seen[definition.Name] = true
	}
	for _, name := range []string{capabilityCatalogName, capabilityDiscoverName} {
		definition, ok := registry.Definition(name)
		if ok && definition.SupportsSurface(surface) && !seen[name] {
			result = append(result, definition)
		}
	}
	return result
}

func kevToolSelectionBypassed(agentID FormalAgentID) bool {
	switch agentID {
	case FormalAgentGoalPlanner, FormalAgentGoalEvaluation,
		FormalAgentVisualIdentity, FormalAgentVisualIdentityVision, FormalAgentVisualIdentityPatch:
		return true
	default:
		return false
	}
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
		out := append([]CapabilityDefinition(nil), s.original...)
		return append(out, (capabilityCatalogCapability{}).Definition(), (capabilityDiscoverCapability{}).Definition()), nil
	}
	out := []CapabilityDefinition{}
	for _, d := range s.original {
		if s.visible[d.Name] || s.loaded[d.Name] {
			out = append(out, d)
		}
	}
	out = append(out, (capabilityCatalogCapability{}).Definition(), (capabilityDiscoverCapability{}).Definition())
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
		name := strings.TrimSpace(d.Name)
		if name != capabilityDiscoverName && name != capabilityCatalogName {
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
