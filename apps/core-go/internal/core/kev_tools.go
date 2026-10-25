package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

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
	mu               sync.Mutex
	service          *decision.Service
	original         []CapabilityDefinition
	loaded           map[string]bool
	revoked          bool
	configKnown      bool
	selectionEnabled bool
	version          int64
	input            string
	scope            decision.Scope
	loadMu           sync.Mutex
	availability     map[string]toolDisclosureAvailability
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
// and admitted by a named catalog request.
type capabilityCatalogCapability struct {
	catalog func(CapabilitySurface) []CapabilityDefinition
}

func capabilityUtilitySurfaces() []CapabilitySurface {
	return []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition}
}

func (c capabilityCatalogCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: capabilityCatalogName, Version: "v1", Type: CapabilityTypeQuery, Description: "Discover authorized query/action tools beyond the initial set. Unknown names: call {} first; use returned names, never guesses. Supply names to load full schemas for the next request; listing/loading never executes an action. Kev recommendations are advisory; installed tools on this surface remain callable. Uninstalled names are reported separately. Historical source_refs are not actionable evidence_refs.", InputSchema: objectSchema(map[string]any{"names": map[string]any{"type": "array", "uniqueItems": true, "items": stringSchema()}}, nil, false), OutputSchema: openObjectSchema(), Surfaces: capabilityUtilitySurfaces(), SupportsCancel: true, FailurePolicy: FailurePolicyOptionalInternal, SideEffectClass: "read_only", ConcurrencyClass: "parallel", SuccessBoundary: "catalog_read"}
}
func (c capabilityCatalogCapability) RequiredContext() []ContextSlot { return nil }
func (c capabilityCatalogCapability) Execute(ctx context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return CapabilityResult{}, err
	}
	if c.catalog == nil {
		return CapabilityResult{}, errors.New("capability_catalog_unavailable")
	}
	catalog := c.catalog
	if registry, ok := scopedCapabilityRegistry(ctx); ok {
		catalog = registry.Catalog
	}
	definitions := nativeDefinitionsForContext(ctx, authorizedDiscoveryDefinitions(ctx, inv.Metadata.Surface, catalog))
	var args struct {
		Names []string `json:"names"`
	}
	if err := json.Unmarshal(inv.Arguments, &args); err != nil && len(inv.Arguments) > 0 {
		return CapabilityResult{}, err
	}
	available := map[string]bool{}
	for _, d := range definitions {
		available[d.Name] = true
	}
	requestedNames := append([]string(nil), args.Names...)
	args.Names = nil
	unavailable := []toolDisclosureAvailability{}
	for _, name := range sortedUniqueStrings(requestedNames) {
		if available[name] {
			args.Names = append(args.Names, name)
		} else {
			unavailable = append(unavailable, toolDisclosureAvailability{Name: name, Status: "unavailable", ErrorCode: "capability_not_installed"})
		}
	}
	loaded := []string{}
	if len(args.Names) > 0 {
		if selection := kevSelection(ctx); selection != nil {
			var err error
			var missing []toolDisclosureAvailability
			loaded, missing, err = selection.load(ctx, args.Names)
			unavailable = append(unavailable, missing...)
			if err != nil {
				return CapabilityResult{}, err
			}
		} else {
			for _, name := range args.Names {
				unavailable = append(unavailable, toolDisclosureAvailability{Name: name, Status: "unavailable", ErrorCode: "tool_run_context_required"})
			}
		}
	}
	admissions := []toolDisclosureAvailability{}
	if selection := kevSelection(ctx); selection != nil {
		for _, name := range args.Names {
			if item, ok := selection.availabilityFor(ctx, name); ok {
				admissions = append(admissions, item)
			}
		}
	}
	items := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		if len(requestedNames) > 0 && !requestedToolName(args.Names, definition.Name) {
			continue
		}
		item := map[string]any{"name": definition.Name, "purpose": definition.Description}
		if len(requestedNames) == 0 {
			item["purpose"] = capabilityDirectoryPurpose(definition)
		}
		// Listing discovers purposes, not every installed business schema.
		// Named disclosure below retains the complete schema and admissions.
		if len(requestedNames) > 0 {
			parameters := definition.InputSchema
			if parameters == nil {
				parameters = objectSchema(nil, nil, false)
			}
			item["parameters"] = parameters
		}
		items = append(items, item)
	}
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", Output: map[string]any{"items": items, "count": len(items), "run_scoped": kevSelection(ctx) != nil, "loaded": loaded, "unavailable": unavailable, "admissions": admissions, "replaces_optional_window": len(args.Names) > 0}}, nil
}

func (c capabilityDiscoverCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: capabilityDiscoverName, Version: "v1", Type: CapabilityTypeQuery, Description: "Load selected authorized capability schemas for the next model request; use capability.catalog first when names or parameters are unknown.", InputSchema: objectSchema(map[string]any{"names": map[string]any{"type": "array", "minItems": 1, "items": stringSchema()}}, []string{"names"}, false), OutputSchema: openObjectSchema(), Surfaces: capabilityUtilitySurfaces(), SupportsCancel: true, FailurePolicy: FailurePolicyOptionalInternal, SideEffectClass: "read_only", ConcurrencyClass: "parallel", SuccessBoundary: "catalog_read"}
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
	catalog := c.catalog
	if registry, ok := scopedCapabilityRegistry(ctx); ok {
		catalog = registry.Catalog
	}
	definitions := nativeDefinitionsForContext(ctx, authorizedDiscoveryDefinitions(ctx, inv.Metadata.Surface, catalog))
	selection := kevSelection(ctx)
	available := map[string]CapabilityDefinition{}
	items := []map[string]any{}
	for _, d := range definitions {
		available[d.Name] = d
		items = append(items, map[string]any{"name": d.Name, "description": d.Description})
	}
	loaded := []string{}
	validNames := []string{}
	unavailable := []toolDisclosureAvailability{}
	for _, name := range sortedUniqueStrings(args.Names) {
		if _, ok := available[name]; ok {
			validNames = append(validNames, name)
		} else {
			unavailable = append(unavailable, toolDisclosureAvailability{Name: name, Status: "unavailable", ErrorCode: "capability_not_installed"})
		}
	}
	args.Names = validNames
	if selection != nil {
		var err error
		var missing []toolDisclosureAvailability
		loaded, missing, err = selection.load(ctx, args.Names)
		unavailable = append(unavailable, missing...)
		if err != nil {
			return CapabilityResult{}, err
		}
	}

	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", Output: map[string]any{"available": items, "loaded": loaded, "unavailable": unavailable, "run_scoped": selection != nil}}, nil
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
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	if kevToolSelectionBypassed(input.AgentID) || len(input.Definitions) == 0 {
		return context.WithValue(ctx, kevRunSelectionKey{}, (*kevRunSelection)(nil)), input.Definitions, nil
	}
	registry := a.capabilityRegistryForContext(ctx)
	full := appendCapabilityUtilities(registry, firstCapabilitySurface(input.Capability), input.Definitions)
	if input.Capability != nil && input.Capability.Projection.KevTools != nil {
		return context.WithValue(ctx, kevRunSelectionKey{}, input.Capability.Projection.KevTools), full, nil
	}
	if prior := kevSelection(ctx); prior != nil {
		return ctx, full, nil
	}
	service := a.kevService()
	_, version, _, _ := service.Settings.Read(ctx)
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	state := &kevRunSelection{service: service, original: full, loaded: map[string]bool{}, availability: map[string]toolDisclosureAvailability{}, version: version}
	for _, message := range input.Prompt.Messages {
		if stringValue(message["role"]) == "user" {
			state.input = stringValue(message["content"])
		}
	}
	if input.Capability != nil {
		state.scope = a.kevScope(ctx, input.Capability.Projection, string(input.AgentID))
	}
	// The executor remains complete. Disclosure is projected separately into
	// prompt assembly, budgets, diagnostics and each actual model request.
	return context.WithValue(ctx, kevRunSelectionKey{}, state), full, nil
}

func publicBaselineTool(name string) bool {
	return name == conversationReplyCapabilityName || name == capabilityCatalogName || name == "moment.publish"
}

type toolDisclosureAvailability struct {
	WarningCode    string `json:"warning_code,omitempty"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	ErrorCode      string `json:"error_code,omitempty"`
	Admission      string `json:"admission,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}

func (s *kevRunSelection) load(ctx context.Context, names []string) ([]string, []toolDisclosureAvailability, error) {
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if len(names) == 0 {
		return []string{}, []toolDisclosureAvailability{}, nil
	}

	s.mu.Lock()
	allowed := map[string]CapabilityDefinition{}
	for _, d := range s.original {
		allowed[d.Name] = d
	}
	s.mu.Unlock()
	validNames := []string{}
	unavailable := []toolDisclosureAvailability{}
	for _, name := range sortedUniqueStrings(names) {
		if _, ok := allowed[name]; ok {
			validNames = append(validNames, name)
		} else {
			unavailable = append(unavailable, toolDisclosureAvailability{Name: name, Status: "unavailable", ErrorCode: "capability_not_installed"})
		}
	}
	names = validNames
	loaded := []string{}
	if len(names) == 0 {
		return loaded, unavailable, nil
	}
	if s.service == nil || s.service.Settings == nil {
		return nil, unavailable, errors.New("kev_selection_service_unavailable")
	}
	config, version, _, readErr := s.service.Settings.Read(ctx)
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	status := map[string]toolDisclosureAvailability{}
	candidates := []decision.Candidate{}
	descriptions := map[string]string{}
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		if publicBaselineTool(name) {
			loaded = append(loaded, name)
			continue
		}
		reason := "disabled"
		if readErr != nil {
			reason = "settings_unavailable"
		}
		status[name] = toolDisclosureAvailability{Name: name, Status: "available", Admission: "catalog_fallback", FallbackReason: reason}
		candidates = append(candidates, decision.Candidate{ID: name, Question: decision.Choice("Should capability " + name + " be disclosed for this explicit catalog request?")})
		descriptions[name] = allowed[name].Description
	}
	if readErr == nil && config.Allows("tools.select") && len(candidates) > 0 {
		selectionCtx := decision.WithFreshStageBudget(ctx, time.Duration(config.BudgetMS)*time.Millisecond)
		results, err := s.service.DecideWithBatchState(selectionCtx, "tools.select", s.scope, candidates, func(batch []decision.Candidate) string {
			selected := map[string]string{}
			for _, candidate := range batch {
				selected[candidate.ID] = descriptions[candidate.ID]
			}
			return jsonString(map[string]any{"input": s.input, "capabilities": selected})
		})
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			results = nil // Keep the bounded explicit request as the original route.
		}
		for index := range results {
			result := &results[index]
			out, err := result.Admit(ctx)
			if err != nil && ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			if err != nil {
				out = decision.Original
			}
			item := status[result.Record.CandidateID]
			item.FallbackReason = firstString(result.Record.Reason, "selector_unavailable")
			applied := "original_used"
			if out == decision.Yes {
				item.Admission = "kev"
				item.FallbackReason = ""
				applied = "applied"
			} else if out == decision.No {
				item.WarningCode = "tool_not_selected"
				item.Admission = "kev"
				item.FallbackReason = ""
				applied = "applied"
			} else if result.Record.Reason == "config_changed" || result.Record.Reason == "discarded_after_disable" {
				item.WarningCode = "tool_selection_changed"
			}
			// Preserve the actual Kev timeout/disabled/guarded/revocation cause.
			if err := result.Finish(ctx, applied, "", ""); err != nil && item.Status == "available" {
				item.Admission = "catalog_fallback"
				item.FallbackReason = "audit_unavailable"
			}
			status[item.Name] = item
		}
	}
	latest, latestVersion, _, latestErr := s.service.Settings.Read(ctx)
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	if latestErr == nil && readErr == nil && (latestVersion != version || latest.Allows("tools.select") != config.Allows("tools.select")) {
		for name, item := range status {
			if item.Status == "denied" {
				continue
			}
			item.WarningCode = "tool_selection_changed"
			status[name] = item
		}
	} else if latestErr != nil {
		for name, item := range status {
			if item.Status == "available" {
				item.Admission = "catalog_fallback"
				item.FallbackReason = "settings_unavailable"
				status[name] = item
			}
		}
	}
	// Revoke old schemas on every named request. The window stays bounded at
	// three baseline entries plus at most eight explicitly requested tools.
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version = version
	s.configKnown = readErr == nil
	s.selectionEnabled = config.Allows("tools.select")
	if readErr != nil && latestErr == nil {
		s.version = latestVersion
		s.configKnown = true
		s.selectionEnabled = latest.Allows("tools.select")
	}
	s.revoked = false
	for _, item := range status {
		if item.WarningCode == "tool_selection_changed" {
			s.revoked = true
		}
	}
	s.loaded = map[string]bool{}
	if s.availability == nil {
		s.availability = map[string]toolDisclosureAvailability{}
	}
	for name, item := range s.availability {
		if !seen[name] && item.Status == "available" {
			item.WarningCode = "tool_selection_window_replaced"
			s.availability[name] = item
		}
	}
	for _, name := range sortedUniqueStrings(names) {
		if publicBaselineTool(name) {
			continue
		}
		item := status[name]
		s.availability[name] = item
		if item.Status == "available" {
			s.loaded[name] = true
			loaded = append(loaded, name)
		} else {
			unavailable = append(unavailable, item)
		}
	}
	return loaded, unavailable, nil
}

func (s *kevRunSelection) availabilityFor(ctx context.Context, name string) (toolDisclosureAvailability, bool) {
	config, version, _, err := s.service.Settings.Read(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revalidateWindowLocked(config, version, err)
	item, ok := s.availability[name]
	if ok && item.Status == "available" {
		if s.revoked {
			item.WarningCode = "tool_selection_changed"
		} else if err != nil {
			item.Admission = "catalog_fallback"
			item.FallbackReason = "settings_unavailable"
		}
	}
	return item, ok
}

// Caller holds mu. Observed revocation is sticky for this named window.
// A restored first readable snapshot anchors an explicitly requested fallback;
// an unknown zero revision from a failed read is not a configuration change.
func (s *kevRunSelection) revalidateWindowLocked(config decision.Config, version int64, err error) {
	if err != nil || s.revoked {
		return
	}
	if !s.configKnown {
		s.version = version
		s.configKnown = true
		s.selectionEnabled = config.Allows("tools.select")
		return
	}
	if version == s.version && config.Allows("tools.select") == s.selectionEnabled {
		return
	}
	s.revoked = true
	s.loaded = map[string]bool{}
	for name, item := range s.availability {
		if item.Status == "available" {
			item.WarningCode = "tool_selection_changed"
			s.availability[name] = item
		}
	}
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
	for _, name := range []string{capabilityCatalogName} {
		definition, ok := registry.Definition(name)
		if !ok || !definition.SupportsSurface(surface) {
			return false
		}
	}
	return true
}

func appendCapabilityUtilities(registry *CapabilityRegistry, surface CapabilitySurface, definitions []CapabilityDefinition) []CapabilityDefinition {
	result := []CapabilityDefinition{}
	for _, d := range definitions {
		if d.Name != capabilityDiscoverName {
			result = append(result, d)
		}
	}
	seen := make(map[string]bool, len(result))
	for _, definition := range result {
		seen[definition.Name] = true
	}
	for _, name := range []string{capabilityCatalogName} {
		definition, ok := registry.Definition(name)
		if ok && definition.SupportsSurface(surface) && !seen[name] {
			result = append(result, definition)
		}
	}
	return result
}

func kevToolSelectionBypassed(agentID FormalAgentID) bool {
	switch agentID {
	case FormalAgentConversationReply, FormalAgentGoalPlanner, FormalAgentGoalEvaluation,
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
	case ProviderContextSurfaceGoalEvaluation:
		return CapabilitySurfaceGoalEvaluation
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
	config, version, _, err := s.service.Settings.Read(ctx)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revalidateWindowLocked(config, version, err)
	out := []CapabilityDefinition{}
	for _, d := range s.original {
		if d.Name == capabilityDiscoverName {
			continue
		}
		item := s.availability[d.Name]
		active := !s.revoked && (item.Admission == "catalog_fallback" || err != nil && item.Admission != "" || err == nil && config.Allows("tools.select"))
		if publicBaselineTool(d.Name) || active && s.loaded[d.Name] {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *kevRunSelection) permits(ctx context.Context, name string) bool {
	if ctx.Err() != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, definition := range s.original {
		if definition.Name == name {
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
	copyModel.toolInfos = infos
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

func requestedToolName(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}
