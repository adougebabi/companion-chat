package core

import (
	"sort"
	"strings"
)

// The Working Persona is the single, deterministic projection of persona
// semantics that a cognition call may see (design.md 3, R08/R09). It is
// composed from three ordered layers:
//
//  1. Shared Identity   core_persona.identity + core_persona.life_profile
//  2. Profile baseline  personality_system.profiles[subject], falling back to
//     the shared top-level personality / behavioral_policy
//  3. Active overlay    the already-composed EffectivePersona of the subject
//
// It deliberately excludes every other declared profile, the visual assets
// (they reach the Provider through their own projection) and the secrets of
// every layer. It never calls a model and is stable for the same input.
const (
	workingPersonaSharedIdentityKey = "shared_identity"
	workingPersonaBodyKey           = "working_persona"
	workingPersonaSwitchKey         = "persistent_switch"

	workingPersonaProfileIDKey = "profile_id"
	workingPersonaRevisionKey  = "persona_revision"
	workingPersonaOverlayKey   = "overlay_revision"
	workingPersonaVersion      = "working-persona.v1"
)

// Note (open item): the reviewed design lists "secrets 全文" among the excluded
// material, but the Main call's existing contract requires the active profile's
// structured decision inputs — including secrets.information_asymmetry — to
// remain visible (provider_prompt_composer_test.go
// TestComposeProviderMessagesPreservesMultiPersonalityDecisionInputs). Excluding
// them here would silently change Main's decision inputs, so this change
// preserves the current behaviour and leaves the exclusion to a product
// decision instead of forcing it.

// workingPersonaExcludedKeys are semantic assets that reach the Provider
// through a dedicated projection instead of the System Persona.
var workingPersonaExcludedKeys = map[string]struct{}{
	"visual_identity": {},
}

// workingPersonaLifeProfileKeys are the life_profile top-level keys the
// Working Persona needs to make per-turn expression and behaviour
// decisions. Appearance, social background, life habits, recurring
// commitments, relationship seeds and media preferences are large domain
// semantics that reach the model through their own task-scoped projections
// (visual identity, schedule, relationships, media) instead of being
// injected every turn (request.md:216, design.md:401, F-06).
var workingPersonaLifeProfileKeys = map[string]struct{}{
	"preferences":           {},
	"character_constraints": {},
}

// workingPersonaTrace records what the projection included, excluded and kept
// verbatim. It exists so a regression that silently drops descriptive persona
// text is observable rather than invisible (F11).
type workingPersonaTrace struct {
	Version          string   `json:"version"`
	SubjectProfileID string   `json:"subject_profile_id"`
	PersonaRevision  int      `json:"persona_revision"`
	OverlayRevision  int      `json:"overlay_revision"`
	OverlayApplied   bool     `json:"overlay_applied"`
	Included         []string `json:"included"`
	Excluded         []string `json:"excluded"`
	Verbatim         []string `json:"verbatim"`
}

// projectWorkingPersona composes the Working Persona for one subject profile.
// The subject defaults to the runtime active profile so callers that already
// resolved a turn scope do not have to pass it twice.
func projectWorkingPersona(projection ContextProjection, subjectProfileID string) (map[string]any, workingPersonaTrace) {
	// The projection carries the Core Persona inside its {authority,data}
	// envelope, so the declared semantics are read from the unwrapped value.
	corePersona := corePersonaData(projection.CorePersona)
	system := mapValue(corePersona["personality_system"])
	if len(system) == 0 {
		system = mapValue(projection.PersonalitySystem)
	}
	activeProfileID := strings.TrimSpace(stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"]))
	subject := strings.TrimSpace(subjectProfileID)
	if subject == "" {
		subject = activeProfileID
	}
	if subject == "" {
		subject = strings.TrimSpace(initialPersonalityProfileID(corePersona))
	}
	if subject == "" {
		subject = "default"
	}

	profile := map[string]any{}
	for _, raw := range arrayValue(system["profiles"]) {
		candidate := mapValue(raw)
		if strings.TrimSpace(stringValue(candidate["id"])) == subject {
			profile = candidate
			break
		}
	}

	// The profile-local baseline wins over the shared top-level one.
	personality := mapValue(projection.Personality)
	if candidate := mapValue(profile["personality"]); len(candidate) > 0 {
		personality = candidate
	}
	behavior := mapValue(projection.BehavioralPolicy)
	if candidate := mapValue(profile["behavioral_policy"]); len(candidate) > 0 {
		behavior = candidate
	}

	trace := workingPersonaTrace{
		Version: workingPersonaVersion, SubjectProfileID: subject,
		PersonaRevision: intValue(mapValue(projection.PersonalityRuntime)["revision"]),
		OverlayRevision: intValue(mapValue(projection.EffectivePersona)["authority_revision"]),
	}
	// The projection's EffectivePersona is the composed snapshot of the active
	// profile, so an overlay applies to that profile. A projection that has
	// been re-scoped for a non-active subject — the Takeover Judge control
	// view of the handover candidate (F-05) — carries the candidate's composed
	// overlay with an explicit profile_id, and that overlay applies to its
	// declared subject instead of the persistent active profile.
	overlay := projection.EffectivePersona
	overlayAppliesToSubject := subject == activeProfileID
	if len(overlay) > 0 && !overlayAppliesToSubject {
		overlayAppliesToSubject = strings.TrimSpace(stringValue(overlay["profile_id"])) == subject
	}
	if len(overlay) > 0 && overlayAppliesToSubject {
		if composed := mapValue(overlay["personality"]); len(composed) > 0 {
			personality = composed
			trace.OverlayApplied = true
		}
		if composed := mapValue(overlay["behavioral_policy"]); len(composed) > 0 {
			behavior = composed
			trace.OverlayApplied = true
		}
	}

	// One canonical shape for every layer. The declared baseline stores traits
	// flat (openness: 0.5) while the composed overlay stores them nested
	// (traits.openness). Rendering whichever layer happened to win would show
	// the model two different shapes for the same trait, so the nested form is
	// always materialised through the same reconciler the evolution layer uses.
	// The copy is additive: flat keys are kept for compatibility and a
	// non-object traits value keeps its prose verbatim instead of becoming an
	// empty object (R11/F11).
	if len(personality) > 0 {
		personality = normalizeEvolutionPersonalityBaseline(personality)
		// The reconciler materialises the canonical carriers even when they stay
		// empty. An empty carrier is pure noise in the persona payload, so it is
		// dropped again; anything with content (numbers or preserved prose) stays.
		for _, root := range []string{"traits", "expression"} {
			if len(mapValue(personality[root])) == 0 {
				delete(personality, root)
			}
		}
	}

	shared := map[string]any{}
	if identity := mapValue(corePersona["identity"]); len(identity) > 0 {
		if filtered := filterCorePersonaValue(identity); len(filtered) > 0 {
			shared["identity"] = filtered
		}
	}
	if lifeProfile := mapValue(corePersona["life_profile"]); len(lifeProfile) > 0 {
		if filtered := filterWorkingPersonaLifeProfile(lifeProfile); len(filtered) > 0 {
			shared["life_profile"] = filtered
		}
	}

	body := map[string]any{}
	if len(personality) > 0 {
		body["personality"] = personality
	}
	if len(behavior) > 0 {
		body["behavioral_policy"] = behavior
	}
	// The subject profile's remaining declared keys (voice, body_language,
	// behavior_loops, scenario_behavior, intimacy_progression, output
	// preferences, fears, desires, ...) are decision inputs the Main call
	// already requires. Every other profile's keys are dropped instead.
	for _, key := range workingPersonaSortedKeys(profile) {
		if personaSwitchKeyMatched(key, []string{"id", "profile_id", "name"}) {
			trace.Excluded = append(trace.Excluded, key)
			continue
		}
		if _, excluded := workingPersonaExcludedKeys[strings.ToLower(strings.TrimSpace(key))]; excluded {
			trace.Excluded = append(trace.Excluded, key)
			continue
		}
		if key == "personality" || key == "behavioral_policy" {
			continue
		}
		body[key] = profile[key]
	}
	body = filterWorkingPersonaBody(body)
	for _, key := range workingPersonaSortedKeys(shared) {
		trace.Included = append(trace.Included, workingPersonaSharedIdentityKey+"."+key)
	}
	for _, key := range workingPersonaSortedKeys(body) {
		trace.Included = append(trace.Included, workingPersonaBodyKey+"."+key)
	}

	result := map[string]any{workingPersonaProfileIDKey: subject}
	if len(shared) > 0 {
		result[workingPersonaSharedIdentityKey] = shared
	}
	if len(body) > 0 {
		body[workingPersonaProfileIDKey] = subject
		result[workingPersonaBodyKey] = body
		recordWorkingPersonaDescriptivePaths(body, "", &trace)
	}
	if trace.PersonaRevision > 0 {
		result[workingPersonaRevisionKey] = trace.PersonaRevision
	}
	if trace.OverlayRevision > 0 {
		result[workingPersonaOverlayKey] = trace.OverlayRevision
	}
	sort.Strings(trace.Included)
	sort.Strings(trace.Excluded)
	sort.Strings(trace.Verbatim)
	return result, trace
}

// filterWorkingPersonaLifeProfile keeps only the life_profile keys the
// Working Persona needs to make per-turn expression and behaviour
// decisions, and applies the shared semantic filter to each kept key.
// Appearance, social background, life habits, recurring commitments,
// relationship seeds and media preferences are large domain semantics that
// reach the model through their own task-scoped projections instead of
// being injected every turn (request.md:216, design.md:401, F-06).
func filterWorkingPersonaLifeProfile(lifeProfile map[string]any) map[string]any {
	if len(lifeProfile) == 0 {
		return nil
	}
	filtered := make(map[string]any)
	for _, key := range workingPersonaSortedKeys(lifeProfile) {
		if _, ok := workingPersonaLifeProfileKeys[strings.ToLower(strings.TrimSpace(key))]; !ok {
			continue
		}
		child := lifeProfile[key]
		if nested := mapValue(child); len(nested) > 0 {
			if value := filterCorePersonaValue(nested); len(value) > 0 {
				filtered[key] = value
			}
			continue
		}
		if list, ok := child.([]any); ok {
			filtered[key] = filterCorePersonaList(list)
			continue
		}
		filtered[key] = child
	}
	return filtered
}

// filterWorkingPersonaBody applies the shared semantic allowlist to one layer
// of the Working Persona without collapsing empty declared objects.
func filterWorkingPersonaBody(body map[string]any) map[string]any {
	if len(body) == 0 {
		return nil
	}
	filtered := make(map[string]any, len(body))
	for _, key := range workingPersonaSortedKeys(body) {
		child := body[key]
		if nested := mapValue(child); len(nested) > 0 {
			if value := filterCorePersonaValue(nested); len(value) > 0 {
				filtered[key] = value
			}
			continue
		}
		if list, ok := child.([]any); ok {
			filtered[key] = filterCorePersonaList(list)
			continue
		}
		filtered[key] = child
	}
	return filtered
}

// recordWorkingPersonaDescriptivePaths records every descriptive string leaf so
// a test can assert prose traits survived verbatim instead of being replaced by
// an invented number (F11).
func recordWorkingPersonaDescriptivePaths(value map[string]any, prefix string, trace *workingPersonaTrace) {
	for _, key := range workingPersonaSortedKeys(value) {
		child := value[key]
		path := prefix + key
		if nested := mapValue(child); len(nested) > 0 {
			recordWorkingPersonaDescriptivePaths(nested, path+".", trace)
			continue
		}
		if text, ok := child.(string); ok && strings.TrimSpace(text) != "" {
			trace.Verbatim = append(trace.Verbatim, path)
		}
	}
}

func workingPersonaSortedKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// workingPersonaMainTurnSchema is the only cognition schema whose call is
// allowed to propose a persistent dominant-profile switch. WakeUp, Reflection,
// daily review and native cognition share the same persona projection but must
// never receive the persistent-switch section (design.md 0.3, user decision).
const workingPersonaMainTurnSchema = "conversation_turn_response"

// systemPersonaForProjection assembles the bundle handed to filterCorePersona:
// the Working Persona, the identifier-only roster and the shared extensions,
// plus the authorized persistent-switch section for the interactive Main turn.
func systemPersonaForProjection(projection ContextProjection, schemaName string) map[string]any {
	corePersona := corePersonaData(projection.CorePersona)
	working, _ := projectWorkingPersona(projection, "")
	bundle := make(map[string]any, len(working)+3)
	for key, value := range working {
		bundle[key] = value
	}
	if extensions := mapValue(corePersona["extensions"]); len(extensions) > 0 {
		bundle["extensions"] = extensions
	}
	system := mapValue(corePersona["personality_system"])
	if len(system) == 0 {
		system = mapValue(projection.PersonalitySystem)
	}
	if len(system) > 0 {
		bundle["personality_system"] = system
	}
	if strings.TrimSpace(schemaName) != workingPersonaMainTurnSchema {
		return bundle
	}
	scope := resolveTurnPersonaScope(projection)
	normalization := normalizePersonaSwitchRules(corePersona, projection.PersonalityRuntime, scope.ActiveProfileID)
	grant := resolvePersistentSwitchGrant(scope, persistentSwitchGrantScenarioMain, normalization.Rules)
	if section := persistentSwitchPromptSection(scope, grant, normalization.Rules); len(section) > 0 {
		bundle[workingPersonaSwitchKey] = section
	}
	return bundle
}
