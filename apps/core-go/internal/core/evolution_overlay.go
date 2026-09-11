package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const evolutionOverlayPolicyVersion = "persona-evolution.policy.v1"

type EvolutionOverlayKind string

const (
	OverlayPersonality    EvolutionOverlayKind = "personality"
	OverlayBehaviorPolicy EvolutionOverlayKind = "behavior_policy"
)

type EvolutionOverlayValueKind string

const (
	OverlayNumeric     EvolutionOverlayValueKind = "numeric"
	OverlayCategorical EvolutionOverlayValueKind = "categorical"
)

type EvolutionOverlayStatus string

const (
	OverlayActive     EvolutionOverlayStatus = "active"
	OverlaySuperseded EvolutionOverlayStatus = "superseded"
)

var personalityEvolutionNumericPaths = map[string]struct{}{
	"traits.openness": {}, "traits.conscientiousness": {}, "traits.extraversion": {},
	"traits.agreeableness": {}, "traits.emotional_stability": {}, "expression.warmth": {},
	"expression.initiative": {}, "expression.playfulness": {},
}

var behaviorEvolutionCategoricalPaths = map[string]struct{}{
	"communication.tone": {}, "response.style": {}, "initiative.mode": {},
	"conflict.approach": {}, "support.style": {},
}

var forbiddenEvolutionPathPrefixes = []string{
	"identity.", "core_persona.", "owner.", "permissions.", "security.", "provider.",
	"infrastructure.", "character_constraints.", "personality_system.active_profile_id",
}

type PersonaEvolutionState struct {
	FluctlightID   string             `json:"fluctlight_id"`
	ProfileID      string             `json:"profile_id"`
	ProfileRef     string             `json:"profile_ref"`
	Revision       int                `json:"revision"`
	Personality    map[string]any     `json:"personality_baseline"`
	BehaviorPolicy map[string]any     `json:"behavior_policy_baseline"`
	Overlays       []EvolutionOverlay `json:"overlays"`
}

type EvolutionOverlay struct {
	ID                string                    `json:"id"`
	Ref               string                    `json:"ref"`
	FluctlightID      string                    `json:"fluctlight_id"`
	ProfileID         string                    `json:"profile_id"`
	Kind              EvolutionOverlayKind      `json:"kind"`
	FieldPath         string                    `json:"field_path"`
	ValueKind         EvolutionOverlayValueKind `json:"value_kind"`
	SemanticDirection string                    `json:"semantic_direction"`
	RequestedDelta    float64                   `json:"requested_delta,omitempty"`
	AppliedDelta      float64                   `json:"applied_delta,omitempty"`
	BeforeValue       any                       `json:"before_value"`
	AfterValue        any                       `json:"after_value"`
	Confidence        float64                   `json:"confidence"`
	EvidenceRefs      []string                  `json:"evidence_refs"`
	EvidenceWindows   []string                  `json:"evidence_windows"`
	PolicyVersion     string                    `json:"policy_version"`
	BaseRevision      int                       `json:"base_revision"`
	Revision          int                       `json:"revision"`
	Status            EvolutionOverlayStatus    `json:"status"`
	Supersedes        string                    `json:"supersedes,omitempty"`
	RollbackOf        string                    `json:"rollback_of,omitempty"`
	CooldownUntil     time.Time                 `json:"cooldown_until"`
	CreatedAt         time.Time                 `json:"created_at"`
}

type EvolutionOverlayPolicy struct {
	NumericMinConfidence     float64
	CategoricalMinConfidence float64
	NumericMinWindows        int
	CategoricalMinWindows    int
	MaxNumericDelta          float64
	Cooldown                 time.Duration
}

func defaultEvolutionOverlayPolicy(policy EvolutionOverlayPolicy) EvolutionOverlayPolicy {
	if policy.NumericMinConfidence <= 0 {
		policy.NumericMinConfidence = 0.8
	}
	if policy.CategoricalMinConfidence <= 0 {
		policy.CategoricalMinConfidence = 0.9
	}
	if policy.NumericMinWindows <= 0 {
		policy.NumericMinWindows = 2
	}
	if policy.CategoricalMinWindows <= 0 {
		policy.CategoricalMinWindows = 3
	}
	if policy.MaxNumericDelta <= 0 {
		policy.MaxNumericDelta = 0.1
	}
	if policy.Cooldown <= 0 {
		policy.Cooldown = 24 * time.Hour
	}
	return policy
}

type EvolutionOverlayRequest struct {
	ExpectedRevision int
	Candidate        ReflectionOverlayCandidateV2
	EvidenceWindows  []string
	OccurredAt       time.Time
}

type EvolutionOverlayDecision struct {
	Disposition EvolutionDisposition `json:"disposition"`
	ReasonCode  string               `json:"reason_code"`
	Overlay     *EvolutionOverlay    `json:"overlay,omitempty"`
}

func CompileEvolutionOverlay(state PersonaEvolutionState, request EvolutionOverlayRequest, policy EvolutionOverlayPolicy) (EvolutionOverlayDecision, error) {
	if err := validatePersonaEvolutionState(state); err != nil {
		return EvolutionOverlayDecision{}, err
	}
	if request.ExpectedRevision != state.Revision || request.OccurredAt.IsZero() {
		return EvolutionOverlayDecision{}, errors.New("evolution_overlay_revision_conflict")
	}
	candidate := request.Candidate
	path := strings.TrimSpace(candidate.FieldPath)
	if evolutionPathForbidden(path) {
		return EvolutionOverlayDecision{Disposition: EvolutionRejected, ReasonCode: "field_forbidden"}, nil
	}
	if !unitFinite(candidate.Strength) || !unitFinite(candidate.Confidence) || len(candidate.EvidenceRefs) == 0 || validateBoundedRefs(candidate.EvidenceRefs) != nil {
		return EvolutionOverlayDecision{}, errors.New("evolution_overlay_candidate_invalid")
	}
	policy = defaultEvolutionOverlayPolicy(policy)
	windows := mergeStableRefs(request.EvidenceWindows)
	if len(windows) == 0 {
		return EvolutionOverlayDecision{}, errors.New("evolution_overlay_windows_required")
	}
	kind, valueKind, allowed := evolutionPathKind(path)
	if !allowed {
		return EvolutionOverlayDecision{Disposition: EvolutionRejected, ReasonCode: "field_not_allowlisted"}, nil
	}
	threshold, minWindows := policy.NumericMinConfidence, policy.NumericMinWindows
	if valueKind == OverlayCategorical {
		threshold, minWindows = policy.CategoricalMinConfidence, policy.CategoricalMinWindows
	}
	if candidate.Confidence < threshold {
		return EvolutionOverlayDecision{Disposition: EvolutionDeferred, ReasonCode: "confidence_below_threshold"}, nil
	}
	if len(windows) < minWindows {
		return EvolutionOverlayDecision{Disposition: EvolutionDeferred, ReasonCode: "cross_window_evidence_required"}, nil
	}
	for _, overlay := range state.Overlays {
		if overlay.Status == OverlayActive && overlay.Kind == kind && overlay.FieldPath == path && request.OccurredAt.Before(overlay.CooldownUntil) {
			return EvolutionOverlayDecision{Disposition: EvolutionDeferred, ReasonCode: "cooldown_active"}, nil
		}
	}
	effective, err := ComposeEffectivePersona(state)
	if err != nil {
		return EvolutionOverlayDecision{}, err
	}
	var before, after any
	requestedDelta, appliedDelta := 0.0, 0.0
	if valueKind == OverlayNumeric {
		beforeNumber, ok := numberFloat(valueAtEvolutionPath(effective.Personality, path))
		if !ok || !unitFinite(beforeNumber) {
			return EvolutionOverlayDecision{}, errors.New("evolution_overlay_numeric_baseline_invalid")
		}
		sign := 0.0
		switch strings.TrimSpace(candidate.Direction) {
		case "increase", "strengthen", "toward":
			sign = 1
		case "decrease", "weaken", "away":
			sign = -1
		case "maintain":
			return EvolutionOverlayDecision{Disposition: EvolutionNoChange, ReasonCode: "semantic_no_change"}, nil
		default:
			return EvolutionOverlayDecision{}, errors.New("evolution_overlay_direction_invalid")
		}
		requestedDelta = sign * candidate.Strength * candidate.Confidence
		appliedDelta = math.Max(-policy.MaxNumericDelta, math.Min(policy.MaxNumericDelta, requestedDelta))
		before = beforeNumber
		after = clampUnit(beforeNumber + appliedDelta)
		appliedDelta = after.(float64) - beforeNumber
	} else {
		value := strings.TrimSpace(candidate.SemanticValue)
		if value == "" || len([]rune(value)) > 500 {
			return EvolutionOverlayDecision{}, errors.New("evolution_overlay_categorical_value_invalid")
		}
		before = valueAtEvolutionPath(effective.BehaviorPolicy, path)
		if before == value {
			return EvolutionOverlayDecision{Disposition: EvolutionNoChange, ReasonCode: "semantic_no_change"}, nil
		}
		after = value
	}
	identity := stableDigest(strings.Join([]string{state.FluctlightID, state.ProfileID, path, fmt.Sprint(state.Revision + 1), strings.Join(windows, "\x1f")}, "\x1f"))
	overlay := &EvolutionOverlay{
		ID: "evolution_overlay_" + identity, Ref: string(ContextReferenceEvolutionOverlay) + ":ctx_" + identity,
		FluctlightID: state.FluctlightID, ProfileID: state.ProfileID, Kind: kind, FieldPath: path, ValueKind: valueKind,
		SemanticDirection: strings.TrimSpace(candidate.Direction), RequestedDelta: requestedDelta, AppliedDelta: appliedDelta,
		BeforeValue: before, AfterValue: after, Confidence: candidate.Confidence, EvidenceRefs: mergeStableRefs(candidate.EvidenceRefs),
		EvidenceWindows: windows, PolicyVersion: evolutionOverlayPolicyVersion, BaseRevision: state.Revision, Revision: state.Revision + 1,
		Status: OverlayActive, CooldownUntil: request.OccurredAt.UTC().Add(policy.Cooldown), CreatedAt: request.OccurredAt.UTC(),
	}
	return EvolutionOverlayDecision{Disposition: EvolutionAccepted, ReasonCode: "policy_accepted", Overlay: overlay}, nil
}

func ApplyEvolutionOverlay(state PersonaEvolutionState, decision EvolutionOverlayDecision) (PersonaEvolutionState, error) {
	if decision.Disposition != EvolutionAccepted || decision.Overlay == nil {
		return state, errors.New("evolution_overlay_not_accepted")
	}
	overlay := *decision.Overlay
	if overlay.FluctlightID != state.FluctlightID || overlay.ProfileID != state.ProfileID || overlay.BaseRevision != state.Revision || overlay.Revision != state.Revision+1 {
		return state, errors.New("evolution_overlay_apply_stale")
	}
	next := clonePersonaEvolutionState(state)
	for index := range next.Overlays {
		current := &next.Overlays[index]
		if current.Status == OverlayActive && current.Kind == overlay.Kind && current.FieldPath == overlay.FieldPath {
			current.Status = OverlaySuperseded
			overlay.Supersedes = current.ID
		}
	}
	next.Overlays = append(next.Overlays, overlay)
	next.Revision = overlay.Revision
	return next, nil
}

type EffectivePersonaSnapshot struct {
	ProfileRef        string         `json:"profile_ref"`
	AuthorityRevision int            `json:"authority_revision"`
	Personality       map[string]any `json:"personality"`
	BehaviorPolicy    map[string]any `json:"behavioral_policy"`
}

func ComposeEffectivePersona(state PersonaEvolutionState) (EffectivePersonaSnapshot, error) {
	if err := validatePersonaEvolutionState(state); err != nil {
		return EffectivePersonaSnapshot{}, err
	}
	personality := deepCloneEvolutionMap(state.Personality)
	behavior := deepCloneEvolutionMap(state.BehaviorPolicy)
	overlays := append([]EvolutionOverlay(nil), state.Overlays...)
	sort.SliceStable(overlays, func(i, j int) bool { return overlays[i].Revision < overlays[j].Revision })
	for _, overlay := range overlays {
		if overlay.Status != OverlayActive || overlay.ProfileID != state.ProfileID || overlay.FluctlightID != state.FluctlightID {
			continue
		}
		if overlay.Kind == OverlayPersonality {
			setEvolutionPath(personality, overlay.FieldPath, overlay.AfterValue)
		} else if overlay.Kind == OverlayBehaviorPolicy {
			setEvolutionPath(behavior, overlay.FieldPath, overlay.AfterValue)
		}
	}
	return EffectivePersonaSnapshot{ProfileRef: state.ProfileRef, AuthorityRevision: state.Revision, Personality: personality, BehaviorPolicy: behavior}, nil
}

type FrozenEffectivePersona struct {
	Digest   string                   `json:"digest"`
	Snapshot EffectivePersonaSnapshot `json:"snapshot"`
}

func FreezeEffectivePersona(snapshot EffectivePersonaSnapshot) (FrozenEffectivePersona, error) {
	if !contextReferencePattern.MatchString(snapshot.ProfileRef) || snapshot.AuthorityRevision < 0 {
		return FrozenEffectivePersona{}, errors.New("effective_persona_snapshot_invalid")
	}
	frozen := EffectivePersonaSnapshot{ProfileRef: snapshot.ProfileRef, AuthorityRevision: snapshot.AuthorityRevision, Personality: deepCloneEvolutionMap(snapshot.Personality), BehaviorPolicy: deepCloneEvolutionMap(snapshot.BehaviorPolicy)}
	return FrozenEffectivePersona{Digest: stableDigest(jsonString(frozen)), Snapshot: frozen}, nil
}

func RollbackEvolutionOverlay(state PersonaEvolutionState, targetID string, expectedRevision int, evidenceRefs []string, now time.Time, cooldown time.Duration) (PersonaEvolutionState, EvolutionOverlay, error) {
	if expectedRevision != state.Revision || now.IsZero() || len(evidenceRefs) == 0 || validateBoundedRefs(evidenceRefs) != nil {
		return state, EvolutionOverlay{}, errors.New("evolution_overlay_rollback_invalid")
	}
	var target *EvolutionOverlay
	for index := range state.Overlays {
		if state.Overlays[index].ID == targetID {
			copyTarget := state.Overlays[index]
			target = &copyTarget
			break
		}
	}
	if target == nil || target.FluctlightID != state.FluctlightID || target.ProfileID != state.ProfileID {
		return state, EvolutionOverlay{}, errors.New("evolution_overlay_rollback_target_invalid")
	}
	effective, err := ComposeEffectivePersona(state)
	if err != nil {
		return state, EvolutionOverlay{}, err
	}
	currentValue := valueAtEvolutionPath(effective.Personality, target.FieldPath)
	if target.Kind == OverlayBehaviorPolicy {
		currentValue = valueAtEvolutionPath(effective.BehaviorPolicy, target.FieldPath)
	}
	if cooldown <= 0 {
		cooldown = 24 * time.Hour
	}
	identity := stableDigest(target.ID + "\x1frollback\x1f" + fmt.Sprint(state.Revision+1))
	rollback := EvolutionOverlay{
		ID: "evolution_overlay_" + identity, Ref: string(ContextReferenceEvolutionOverlay) + ":ctx_" + identity,
		FluctlightID: state.FluctlightID, ProfileID: state.ProfileID, Kind: target.Kind, FieldPath: target.FieldPath, ValueKind: target.ValueKind,
		SemanticDirection: "rollback", BeforeValue: currentValue, AfterValue: target.BeforeValue, Confidence: 1,
		EvidenceRefs: mergeStableRefs(evidenceRefs), EvidenceWindows: []string{"rollback:" + target.ID}, PolicyVersion: evolutionOverlayPolicyVersion,
		BaseRevision: state.Revision, Revision: state.Revision + 1, Status: OverlayActive, RollbackOf: target.ID,
		CooldownUntil: now.UTC().Add(cooldown), CreatedAt: now.UTC(),
	}
	next, err := ApplyEvolutionOverlay(state, EvolutionOverlayDecision{Disposition: EvolutionAccepted, ReasonCode: "rollback", Overlay: &rollback})
	return next, rollback, err
}

func validatePersonaEvolutionState(state PersonaEvolutionState) error {
	if strings.TrimSpace(state.FluctlightID) == "" || strings.TrimSpace(state.ProfileID) == "" || !contextReferencePattern.MatchString(state.ProfileRef) || state.Revision < 0 || state.Personality == nil || state.BehaviorPolicy == nil {
		return errors.New("persona_evolution_state_invalid")
	}
	return nil
}

func evolutionPathForbidden(path string) bool {
	path = strings.TrimSpace(path)
	for _, prefix := range forbiddenEvolutionPathPrefixes {
		if path == strings.TrimSuffix(prefix, ".") || strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func evolutionPathKind(path string) (EvolutionOverlayKind, EvolutionOverlayValueKind, bool) {
	if _, ok := personalityEvolutionNumericPaths[path]; ok {
		return OverlayPersonality, OverlayNumeric, true
	}
	if _, ok := behaviorEvolutionCategoricalPaths[path]; ok {
		return OverlayBehaviorPolicy, OverlayCategorical, true
	}
	return "", "", false
}

func valueAtEvolutionPath(root map[string]any, path string) any {
	current := any(root)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

func setEvolutionPath(root map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func deepCloneEvolutionMap(source map[string]any) map[string]any {
	return decodeObject(jsonBytes(source))
}

func clonePersonaEvolutionState(state PersonaEvolutionState) PersonaEvolutionState {
	result := state
	result.Personality = deepCloneEvolutionMap(state.Personality)
	result.BehaviorPolicy = deepCloneEvolutionMap(state.BehaviorPolicy)
	result.Overlays = append([]EvolutionOverlay(nil), state.Overlays...)
	for index := range result.Overlays {
		result.Overlays[index].EvidenceRefs = append([]string(nil), result.Overlays[index].EvidenceRefs...)
		result.Overlays[index].EvidenceWindows = append([]string(nil), result.Overlays[index].EvidenceWindows...)
	}
	return result
}

func (a *App) readEffectivePersonaProjection(ctx context.Context, fluctlight Fluctlight, system, runtime map[string]any) (map[string]any, []map[string]any, error) {
	baseline := personaEvolutionBaseline(fluctlight.ID, fluctlight.Personality, fluctlight.BehavioralPolicy, system, runtime)
	state, err := loadPersonaEvolutionState(ctx, a.DB.Pool(), baseline)
	if err != nil {
		return nil, nil, err
	}
	effective, err := ComposeEffectivePersona(state)
	if err != nil {
		return nil, nil, err
	}
	overlays := make([]map[string]any, 0, len(state.Overlays))
	for _, overlay := range state.Overlays {
		if overlay.Status != OverlayActive {
			continue
		}
		overlays = append(overlays, map[string]any{
			"id": overlay.ID, "ref": overlay.Ref, "kind": overlay.Kind, "field_path": overlay.FieldPath,
			"semantic_direction": overlay.SemanticDirection, "confidence": overlay.Confidence,
			"policy_version": overlay.PolicyVersion, "revision": overlay.Revision,
		})
	}
	return map[string]any{
		"profile_ref": effective.ProfileRef, "authority_revision": effective.AuthorityRevision,
		"personality": effective.Personality, "behavioral_policy": effective.BehaviorPolicy,
	}, overlays, nil
}

func personaEvolutionBaseline(fluctlightID string, fallbackPersonality, fallbackBehavior, system, runtime map[string]any) PersonaEvolutionState {
	profileID := strings.TrimSpace(stringValue(runtime["active_profile_id"]))
	if profileID == "" {
		profileID = strings.TrimSpace(stringValue(system["active_profile_id"]))
	}
	if profileID == "" {
		profileID = "default"
	}
	personality := deepCloneEvolutionMap(fallbackPersonality)
	behavior := deepCloneEvolutionMap(fallbackBehavior)
	for _, raw := range arrayValue(system["profiles"]) {
		profile := mapValue(raw)
		if stringValue(profile["id"]) != profileID {
			continue
		}
		if candidate := mapValue(profile["personality"]); len(candidate) > 0 {
			personality = deepCloneEvolutionMap(candidate)
		}
		if candidate := mapValue(profile["behavioral_policy"]); len(candidate) > 0 {
			behavior = deepCloneEvolutionMap(candidate)
		}
		break
	}
	return PersonaEvolutionState{
		FluctlightID: fluctlightID, ProfileID: profileID,
		ProfileRef:  "personality:ctx_" + stableDigest(fluctlightID+"\x1f"+profileID),
		Personality: normalizeEvolutionPersonalityBaseline(personality), BehaviorPolicy: normalizeEvolutionBehaviorBaseline(behavior),
	}
}

func personaEvolutionBaselineFromProjection(projection ContextProjection) PersonaEvolutionState {
	return personaEvolutionBaseline(projection.FluctlightID, projection.Personality, projection.BehavioralPolicy, projection.PersonalitySystem, projection.PersonalityRuntime)
}

func normalizeEvolutionPersonalityBaseline(value map[string]any) map[string]any {
	result := deepCloneEvolutionMap(value)
	for _, root := range []string{"traits", "expression"} {
		if len(mapValue(result[root])) == 0 {
			result[root] = map[string]any{}
		}
	}
	for path := range personalityEvolutionNumericPaths {
		parts := strings.Split(path, ".")
		nested := mapValue(result[parts[0]])
		if _, exists := nested[parts[1]]; exists {
			continue
		}
		if direct, ok := numberFloat(result[parts[1]]); ok && unitFinite(direct) {
			nested[parts[1]] = direct
		}
	}
	return result
}

func normalizeEvolutionBehaviorBaseline(value map[string]any) map[string]any {
	result := deepCloneEvolutionMap(value)
	aliases := map[string]string{
		"communication.tone": "tone", "response.style": "response_style", "initiative.mode": "initiative",
		"conflict.approach": "conflict_style", "support.style": "support_style",
	}
	for path := range behaviorEvolutionCategoricalPaths {
		parts := strings.Split(path, ".")
		nested := mapValue(result[parts[0]])
		if len(nested) == 0 {
			nested = map[string]any{}
			result[parts[0]] = nested
		}
		if _, exists := nested[parts[1]]; exists {
			continue
		}
		if direct := strings.TrimSpace(stringValue(result[aliases[path]])); direct != "" {
			nested[parts[1]] = direct
		}
	}
	return result
}
