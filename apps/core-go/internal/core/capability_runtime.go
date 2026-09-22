package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (a *App) capabilityRegistry() *CapabilityRegistry {
	if a != nil && a.Capabilities != nil {
		return a.Capabilities
	}
	registry, _ := NewCapabilityRegistry(builtinCapabilities(a)...)
	return registry
}

func (a *App) capabilityRuntime() *CapabilityRuntime {
	if a != nil && a.Runtime != nil {
		return a.Runtime
	}
	registry := a.capabilityRegistry()
	if a == nil || a.ContextResolver == nil {
		return nil
	}
	resolver := a.ContextResolver
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		return nil
	}
	if a != nil {
		a.Runtime = runtime
	}
	return runtime
}

func capabilityCatalog(registry *CapabilityRegistry, surface CapabilitySurface) []CapabilityDefinition {
	if registry == nil {
		return nil
	}
	return registry.Catalog(surface)
}

// candidateValidationContext carries the frozen identity of the candidate being
// arbitrated. It is the authority the Judge-time validator compares every
// invocation against; a Provider result cannot widen it. All checks performed
// against it are deterministic and side-effect-free — anything that needs
// network, DB or context-resolver I/O stays on the winner-only Prepare path.
type candidateValidationContext struct {
	FluctlightID   string
	ConversationID string
	SourceFactID   string
	ActionID       string
	Surface        CapabilitySurface
	// ContextSnapshot is the Core-owned snapshot captured before arbitration.
	// It is intentionally separate from CapabilityInvocation.ContextSnapshot:
	// provider supplied invocation fields are not an authority for candidate
	// authorization and are overwritten when the candidate is persisted.
	ContextSnapshot map[string]any
	// Context lets the production call sites preserve cancellation and tracing
	// while keeping the two-argument helper source compatible with existing unit
	// tests. It is never used to resolve ambient state.
	Context context.Context
}

// validateCandidateCapabilityInvocations runs the cheap persistence validator
// and then performs the deterministic authorization checks F-02 requires
// before the Judge is consulted: declared surface authorization, frozen
// identity ownership and declared output target kinds. A candidate that is
// structurally valid but unauthorized must fail here so a takeover by B cannot
// mask A's illegal invocation (request.md:79/163/296).
func (a *App) validateCandidateCapabilityInvocations(invocations []CapabilityInvocation, candidate candidateValidationContext) error {
	ctx := candidate.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return a.validateCandidateCapabilityInvocationsWithContext(ctx, invocations, candidate)
}

func (a *App) validateCandidateCapabilityInvocationsWithContext(ctx context.Context, invocations []CapabilityInvocation, candidate candidateValidationContext) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(invocations) == 0 {
		return nil
	}
	if strings.TrimSpace(candidate.FluctlightID) == "" {
		return fmt.Errorf("%w: candidate fluctlight identity is missing", ErrUnauthorized)
	}
	if strings.TrimSpace(string(candidate.Surface)) == "" {
		return fmt.Errorf("%w: candidate surface is missing", ErrUnauthorized)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: candidate validation context: %v", ErrContextResolve, err)
	}
	registry := a.capabilityRegistry()
	if registry == nil {
		return ErrCapabilityNotFound
	}
	for _, invocation := range invocations {
		if err := validateCandidateCapabilityInvocation(ctx, invocation, candidate, registry); err != nil {
			return err
		}
	}
	return nil
}

func validateCandidateCapabilityInvocation(ctx context.Context, invocation CapabilityInvocation, candidate candidateValidationContext, registry *CapabilityRegistry) error {
	definition, ok := registry.Definition(invocation.CapabilityName)
	if !ok {
		return fmt.Errorf("%w: %s", ErrCapabilityNotFound, invocation.CapabilityName)
	}
	if err := invocation.Validate(definition); err != nil {
		return err
	}
	if definition.InternalOnly {
		return fmt.Errorf("%w: capability %s is internal-only", ErrUnauthorized, invocation.CapabilityName)
	}
	// The frozen candidate surface is the only authority. A provider may omit
	// metadata (Core fills that default), but it may not claim another
	// already-authorized surface to widen the call's permissions.
	if declaredSurface := invocation.Metadata.Surface; declaredSurface != "" && declaredSurface != candidate.Surface {
		return fmt.Errorf("%w: capability %s declares surface %s outside candidate surface %s", ErrUnauthorized, invocation.CapabilityName, declaredSurface, candidate.Surface)
	}
	if !definition.SupportsSurface(candidate.Surface) {
		return fmt.Errorf("%w: capability %s is not authorized for surface %s", ErrUnauthorized, invocation.CapabilityName, candidate.Surface)
	}
	if owner := strings.TrimSpace(invocation.Metadata.FluctlightID); owner != "" && owner != strings.TrimSpace(candidate.FluctlightID) {
		return fmt.Errorf("%w: capability %s targets fluctlight %s outside the candidate %s", ErrUnauthorized, invocation.CapabilityName, owner, candidate.FluctlightID)
	}
	if cid := strings.TrimSpace(invocation.Metadata.ConversationID); cid != "" && cid != strings.TrimSpace(candidate.ConversationID) {
		return fmt.Errorf("%w: capability %s targets conversation %s outside the candidate %s", ErrUnauthorized, invocation.CapabilityName, cid, candidate.ConversationID)
	}
	if sourceFactID := strings.TrimSpace(invocation.SourceFactID); sourceFactID != "" && strings.TrimSpace(candidate.SourceFactID) != "" && sourceFactID != strings.TrimSpace(candidate.SourceFactID) {
		return fmt.Errorf("%w: capability %s targets source fact %s outside the candidate %s", ErrUnauthorized, invocation.CapabilityName, sourceFactID, candidate.SourceFactID)
	}
	if actionID := strings.TrimSpace(invocation.ActionID); actionID != "" && strings.TrimSpace(candidate.ActionID) != "" && actionID != strings.TrimSpace(candidate.ActionID) {
		return fmt.Errorf("%w: capability %s targets action %s outside the candidate %s", ErrUnauthorized, invocation.CapabilityName, actionID, candidate.ActionID)
	}
	if binding := invocation.Metadata.OutputBinding; binding != nil {
		if len(definition.TargetKinds) == 0 || !targetKindAuthorizes(definition.TargetKinds, binding.TargetKind) {
			return fmt.Errorf("%w: capability %s output target kind %s is not declared", ErrUnauthorized, invocation.CapabilityName, binding.TargetKind)
		}
	}

	resolved := CapabilityContext{Identity: ContextIdentity{
		FluctlightID: strings.TrimSpace(candidate.FluctlightID), ConversationID: strings.TrimSpace(candidate.ConversationID),
		SourceFactID: strings.TrimSpace(candidate.SourceFactID), ActionID: strings.TrimSpace(candidate.ActionID),
	}, extra: make(map[ContextSlot]any)}
	if len(definition.RequiredContext) > 0 {
		if len(candidate.ContextSnapshot) == 0 {
			return fmt.Errorf("%w: capability %s requires a frozen context snapshot", ErrContextResolve, invocation.CapabilityName)
		}
		if err := validateCandidateSnapshotIdentity(candidate.ContextSnapshot, candidate); err != nil {
			return fmt.Errorf("%w: capability %s: %v", ErrContextResolve, invocation.CapabilityName, err)
		}
		resolver := NewSnapshotContextResolver(candidate.ContextSnapshot)
		var resolveErr error
		resolved, resolveErr = resolver.Resolve(ctx, ContextRequest{
			FluctlightID: strings.TrimSpace(candidate.FluctlightID), ConversationID: strings.TrimSpace(candidate.ConversationID),
			SourceFactID: strings.TrimSpace(candidate.SourceFactID), ActionID: strings.TrimSpace(candidate.ActionID), Surface: candidate.Surface,
		}, definition.RequiredContext)
		if resolveErr != nil {
			return fmt.Errorf("%w: capability %s: %v", ErrContextResolve, invocation.CapabilityName, resolveErr)
		}
	} else if len(candidate.ContextSnapshot) > 0 {
		// A contextful snapshot is still checked when supplied to a contextless
		// capability so a custom hook cannot receive a forged identity.
		if err := validateCandidateSnapshotIdentity(candidate.ContextSnapshot, candidate); err != nil {
			return fmt.Errorf("%w: capability %s: %v", ErrContextResolve, invocation.CapabilityName, err)
		}
	}
	if validator, ok := registry.LookupCapability(invocation.CapabilityName); ok {
		if hook, implements := validator.(CapabilityCandidateValidator); implements {
			if err := hook.ValidateCandidate(ctx, invocation, resolved); err != nil {
				return fmt.Errorf("capability %s candidate validation: %w", invocation.CapabilityName, err)
			}
		}
	}
	return nil
}

// validateCandidateSnapshotIdentity makes the frozen snapshot binding strict
// enough for Judge-time authorization. SnapshotContextResolver intentionally
// permits legacy identity omissions for replay compatibility; a candidate
// validator cannot do that because an omitted ID would let a provider attach a
// capability to a different turn while retaining otherwise valid slot data.
func validateCandidateSnapshotIdentity(snapshot map[string]any, candidate candidateValidationContext) error {
	if len(snapshot) == 0 {
		return errors.New("snapshot is unavailable")
	}
	identity := mapValue(snapshot["identity"])
	if len(identity) == 0 {
		return errors.New("snapshot identity is missing")
	}
	checks := []struct {
		key      string
		expected string
	}{
		{"fluctlight_id", strings.TrimSpace(candidate.FluctlightID)},
		{"conversation_id", strings.TrimSpace(candidate.ConversationID)},
		{"source_fact_id", strings.TrimSpace(candidate.SourceFactID)},
	}
	for _, check := range checks {
		if check.expected == "" {
			continue
		}
		actual := strings.TrimSpace(stringValue(identity[check.key]))
		if actual == "" {
			return fmt.Errorf("snapshot identity %s is missing", check.key)
		}
		if actual != check.expected {
			return fmt.Errorf("snapshot identity %s does not match candidate", check.key)
		}
	}
	if expectedAction := strings.TrimSpace(candidate.ActionID); expectedAction != "" {
		if actualAction := strings.TrimSpace(stringValue(identity["action_id"])); actualAction != "" && actualAction != expectedAction {
			return errors.New("snapshot identity action_id does not match candidate")
		}
	}
	return nil
}

func targetKindAuthorizes(declared []string, kind string) bool {
	for _, allowed := range declared {
		if allowed == kind {
			return true
		}
	}
	return false
}

func (a *App) bindCapabilityInvocationsToProjection(invocations []CapabilityInvocation, projection ContextProjection, actionID, sourceFactID string, surface CapabilitySurface) ([]CapabilityInvocation, error) {
	bound := append([]CapabilityInvocation(nil), invocations...)
	registry := a.capabilityRegistry()
	for index := range bound {
		bound[index] = normalizeCapabilityInvocationMetadata(bound[index], projection.FluctlightID, projection.ConversationID, sourceFactID, sourceFactID, index)
		bound[index].ActionID = actionID
		bound[index].Metadata.Surface = surface
		definition, ok := registry.Definition(bound[index].CapabilityName)
		if !ok {
			return bound, fmt.Errorf("%w: %s", ErrCapabilityNotFound, bound[index].CapabilityName)
		}
		bound[index].ContextSnapshot = capabilitySnapshotForProjection(projection, definition.RequiredContext, actionID)
	}
	return bound, nil
}

func normalizeCapabilityInvocationMetadata(invocation CapabilityInvocation, fluctlightID, conversationID, sourceFactID, identityScope string, sequence int) CapabilityInvocation {
	if invocation.CallID == "" {
		return invocation
	}
	if invocation.SourceFactID == "" {
		invocation.SourceFactID = sourceFactID
	}
	if invocation.ProviderRequestID == "" {
		invocation.ProviderRequestID = "provider:" + stableDigest(identityScope+":"+invocation.CallID)
	}
	if invocation.Sequence < 0 {
		invocation.Sequence = sequence
	}
	if invocation.Metadata.FluctlightID == "" {
		invocation.Metadata.FluctlightID = fluctlightID
	}
	if invocation.Metadata.ConversationID == "" {
		invocation.Metadata.ConversationID = conversationID
	}
	if invocation.Metadata.Surface == "" {
		invocation.Metadata.Surface = CapabilitySurfaceConversation
	}
	if invocation.Intent == "" {
		var args map[string]any
		if json.Unmarshal(invocation.Arguments, &args) == nil {
			invocation.Intent = stringValue(args["intent"])
		}
	}
	if invocation.SchemaVersion == "" {
		invocation.SchemaVersion = CapabilityInvocationSchemaVersion
	}
	return invocation
}

func stringSliceAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func capabilityResultForCall(results []CapabilityResult, callID string) (CapabilityResult, bool) {
	for _, result := range results {
		if result.CallID == callID {
			return result, true
		}
	}
	return CapabilityResult{}, false
}

func replaceCapabilityResult(results []CapabilityResult, replacement CapabilityResult) []CapabilityResult {
	for index := range results {
		if results[index].CallID == replacement.CallID {
			results[index] = replacement
			return results
		}
	}
	return append(results, replacement)
}

func capabilityResultsFromValue(value any) ([]CapabilityResult, error) {
	if value == nil {
		return []CapabilityResult{}, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("capability result payload invalid: %w", err)
	}
	if string(data) == "null" {
		return []CapabilityResult{}, nil
	}
	var results []CapabilityResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("capability result payload invalid: %w", err)
	}
	seen := make(map[string]struct{}, len(results))
	for index := range results {
		if strings.TrimSpace(results[index].CallID) == "" || strings.TrimSpace(results[index].CapabilityName) == "" {
			return nil, fmt.Errorf("capability result %d identity is missing", index)
		}
		if _, duplicate := seen[results[index].CallID]; duplicate {
			return nil, fmt.Errorf("capability result %d call id is duplicated", index)
		}
		seen[results[index].CallID] = struct{}{}
		switch results[index].Status {
		case "completed", "accepted", "failed", "rejected", "deferred":
		default:
			return nil, fmt.Errorf("capability result %d status is invalid", index)
		}
	}
	return results, nil
}

func hasConversationReplyCapability(invocations []CapabilityInvocation, registry *CapabilityRegistry) bool {
	for _, invocation := range invocations {
		if isConversationReplyInvocation(invocation, registry) {
			return true
		}
	}
	return false
}

func replyTextFromCapabilityInvocations(invocations []CapabilityInvocation, registry *CapabilityRegistry) string {
	for _, invocation := range invocations {
		if !isConversationReplyInvocation(invocation, registry) {
			continue
		}
		var args map[string]any
		if json.Unmarshal(invocation.Arguments, &args) != nil {
			continue
		}
		if text := normalizeVisibleReply(stringValue(args["text"])); text != "" {
			return text
		}
	}
	return ""
}

// isConversationReplyInvocation identifies the one built-in visible-output
// capability by its canonical name. A normal runtime registry still verifies
// the declared output role; when a partially assembled registry is used (for
// example while recovering a diagnostic/frozen payload), the canonical name is
// enough to preserve the reply text instead of letting unrelated structured
// fields turn a valid private message into an empty response.
func isConversationReplyInvocation(invocation CapabilityInvocation, registry *CapabilityRegistry) bool {
	if strings.TrimSpace(invocation.CapabilityName) != conversationReplyCapabilityName {
		return false
	}
	if registry == nil {
		return true
	}
	definition, ok := registry.Definition(invocation.CapabilityName)
	if !ok {
		return true
	}
	return definition.OutputRole == "conversation_message"
}

func resolveCapabilityAction(invocations []CapabilityInvocation, definitions map[string]CapabilityDefinition) (string, error) {
	if len(invocations) == 0 {
		return "", errors.New("at least one capability invocation is required")
	}
	for _, invocation := range invocations {
		definition, ok := definitions[invocation.CapabilityName]
		if !ok {
			continue
		}
		if invocation.Metadata.OutputBinding != nil && invocation.Metadata.OutputBinding.TargetKind == "conversation_message" {
			return "reply", nil
		}
		if definition.OutputRole == "conversation_message" {
			return "reply", nil
		}
	}
	return "no_op", nil
}
