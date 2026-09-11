package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// CapabilityType describes the policy/audit class of a capability.  It is
// metadata only; execution is always selected through the registry entry.
type CapabilityType string

const (
	CapabilityTypeAction   CapabilityType = "action"
	CapabilityTypeQuery    CapabilityType = "query"
	CapabilityTypeInternal CapabilityType = "internal"
)

// CapabilitySurface identifies a provider-facing catalog.  Callers select a
// surface, never a list of concrete capability names.
type CapabilitySurface string

const (
	CapabilitySurfaceConversation    CapabilitySurface = "conversation"
	CapabilitySurfaceWakeUp          CapabilitySurface = "wake_up"
	CapabilitySurfaceAutonomy        CapabilitySurface = "autonomy"
	CapabilitySurfaceNativeCognition CapabilitySurface = "native_cognition"
	CapabilitySurfaceReflection      CapabilitySurface = "reflection"
)

// CapabilityFailurePolicy controls visible-settlement behavior.  The Runtime
// does not inspect capability names when applying this policy.
type CapabilityFailurePolicy string

const (
	FailurePolicyRequiredForVisibleClaim CapabilityFailurePolicy = "required_for_visible_claim"
	FailurePolicyOptionalInternal        CapabilityFailurePolicy = "optional_internal"
)

type CapabilityExecutionClass string

const (
	CapabilityExecutionPureQuery             CapabilityExecutionClass = "pure_query"
	CapabilityExecutionTransactionalMutation CapabilityExecutionClass = "transactional_mutation"
	CapabilityExecutionDeferredOutput        CapabilityExecutionClass = "deferred_output"
	CapabilityExecutionExternalAsyncIntent   CapabilityExecutionClass = "external_async_intent"
)

func classifyCapabilityExecution(capability Capability, definition CapabilityDefinition) (CapabilityExecutionClass, error) {
	if capability == nil {
		return "", errors.New("capability_execution_missing")
	}
	if definition.CompletionBoundary != "" {
		if definition.OutcomeReferenceField == "" {
			return "", errors.New("capability_async_reference_field_missing")
		}
		return CapabilityExecutionExternalAsyncIntent, nil
	}
	if definition.IsDeferredOutput() {
		if _, ok := capability.(DeferredCapability); !ok {
			return "", errors.New("capability_deferred_output_seam_missing")
		}
		return CapabilityExecutionDeferredOutput, nil
	}
	if _, ok := capability.(TransactionalCapability); ok {
		return CapabilityExecutionTransactionalMutation, nil
	}
	if definition.Type == CapabilityTypeQuery && definition.SideEffectClass == "read_only" {
		return CapabilityExecutionPureQuery, nil
	}
	if definition.SideEffectClass == "native_projection" {
		return "", errors.New("capability_transactional_seam_missing")
	}
	return "", errors.New("capability_execution_class_invalid")
}

// ContextSlot is a typed request for execution context.  A capability should
// declare the smallest set of slots it needs rather than reading App state
// directly.
type ContextSlot string

const (
	SlotCorePersona       ContextSlot = "core_persona"
	SlotCurrentState      ContextSlot = "current_state"
	SlotCurrentLife       ContextSlot = "current_life"
	SlotSchedule          ContextSlot = "schedule"
	SlotVisualIdentity    ContextSlot = "visual_identity"
	SlotAppearance        ContextSlot = "appearance"
	SlotRelationshipScope ContextSlot = "relationship_scope"
	SlotMemoryScope       ContextSlot = "memory_scope"
	SlotAgency            ContextSlot = "agency"
	SlotRecentOutcomes    ContextSlot = "recent_outcomes"
)

// ContextRequest is intentionally small.  It carries resource identity and
// replay metadata; domain payloads remain in the typed CapabilityContext.
type ContextRequest struct {
	FluctlightID   string
	ConversationID string
	SourceFactID   string
	ActionID       string
	CorrelationID  string
	Surface        CapabilitySurface
	SemanticIntent string
	ViewerActorIDs []string
}

type ContextIdentity struct {
	FluctlightID   string `json:"fluctlight_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	SourceFactID   string `json:"source_fact_id,omitempty"`
	ActionID       string `json:"action_id,omitempty"`
}

// The value objects deliberately keep the existing Core JSON representations
// at the edge.  They are not a public map[ContextSlot]any bag.
type PersonaContext struct {
	Data map[string]any `json:"data,omitempty"`
}
type CurrentStateContext struct {
	Data map[string]any `json:"data,omitempty"`
}
type CurrentLifeContext struct {
	Data map[string]any `json:"data,omitempty"`
}
type ScheduleContext struct {
	Data map[string]any `json:"data,omitempty"`
}
type VisualIdentityContext struct {
	Data map[string]any `json:"data,omitempty"`
}
type AppearanceContext struct {
	Data map[string]any `json:"data,omitempty"`
}
type RelationshipScope struct {
	Data map[string]any `json:"data,omitempty"`
}

func (scope *RelationshipScope) Allows(actorID string) bool {
	if scope == nil || strings.TrimSpace(actorID) == "" {
		return false
	}
	for _, raw := range arrayValue(scope.Data["authorized_actor_ids"]) {
		if strings.TrimSpace(stringValue(raw)) == strings.TrimSpace(actorID) {
			return true
		}
	}
	return false
}

type MemoryScope struct {
	Data map[string]any `json:"data,omitempty"`
}
type AgencyContext struct {
	Data map[string]any `json:"data,omitempty"`
}
type RecentOutcomesContext struct {
	Data map[string]any `json:"data,omitempty"`
}

// CapabilityContext is the typed context handed to a capability.  Slots not
// requested by the capability remain nil and are never eagerly loaded.
type CapabilityContext struct {
	Identity ContextIdentity        `json:"identity"`
	Persona  *PersonaContext        `json:"persona,omitempty"`
	State    *CurrentStateContext   `json:"state,omitempty"`
	Life     *CurrentLifeContext    `json:"life,omitempty"`
	Schedule *ScheduleContext       `json:"schedule,omitempty"`
	Visual   *VisualIdentityContext `json:"visual,omitempty"`
	Outfit   *AppearanceContext     `json:"appearance,omitempty"`
	Relation *RelationshipScope     `json:"relationship,omitempty"`
	Memory   *MemoryScope           `json:"memory,omitempty"`
	Agency   *AgencyContext         `json:"agency,omitempty"`
	Outcomes *RecentOutcomesContext `json:"outcomes,omitempty"`
	extra    map[ContextSlot]any
}

func (context CapabilityContext) Has(slot ContextSlot) bool {
	_, ok := context.Value(slot)
	return ok
}

func (context CapabilityContext) Value(slot ContextSlot) (any, bool) {
	switch slot {
	case SlotCorePersona:
		return context.Persona, context.Persona != nil
	case SlotCurrentState:
		return context.State, context.State != nil
	case SlotCurrentLife:
		return context.Life, context.Life != nil
	case SlotSchedule:
		return context.Schedule, context.Schedule != nil
	case SlotVisualIdentity:
		return context.Visual, context.Visual != nil
	case SlotAppearance:
		return context.Outfit, context.Outfit != nil
	case SlotRelationshipScope:
		return context.Relation, context.Relation != nil
	case SlotMemoryScope:
		return context.Memory, context.Memory != nil
	case SlotAgency:
		return context.Agency, context.Agency != nil
	case SlotRecentOutcomes:
		return context.Outcomes, context.Outcomes != nil
	default:
		value, ok := context.extra[slot]
		return value, ok
	}
}

// Snapshot returns a bounded, replay-safe representation.  It is a copy and
// can therefore be persisted alongside a frozen action without exposing the
// resolver's internal map.
func (context CapabilityContext) Snapshot() map[string]any {
	result := map[string]any{"identity": map[string]any{
		"fluctlight_id":   context.Identity.FluctlightID,
		"conversation_id": context.Identity.ConversationID,
		"source_fact_id":  context.Identity.SourceFactID,
		"action_id":       context.Identity.ActionID,
	}}
	put := func(key string, value any) {
		if value == nil {
			return
		}
		// Persist the value object's data directly.  Keeping a second {data:}
		// wrapper makes replay consumers observe a different shape than live
		// resolution and silently drops fields such as schedule.local_date.
		switch typed := value.(type) {
		case *PersonaContext:
			if typed == nil {
				return
			}
			value = typed.Data
		case *CurrentStateContext:
			if typed == nil {
				return
			}
			value = typed.Data
		case *CurrentLifeContext:
			if typed == nil {
				return
			}
			value = typed.Data
		case *ScheduleContext:
			if typed == nil {
				return
			}
			value = typed.Data
		case *VisualIdentityContext:
			if typed == nil {
				return
			}
			value = typed.Data
		case *AppearanceContext:
			if typed == nil {
				return
			}
			value = typed.Data
		case *RelationshipScope:
			if typed == nil {
				return
			}
			value = typed.Data
		case *MemoryScope:
			if typed == nil {
				return
			}
			value = typed.Data
		case *AgencyContext:
			if typed == nil {
				return
			}
			value = typed.Data
		case *RecentOutcomesContext:
			if typed == nil {
				return
			}
			value = typed.Data
		}
		data := jsonBytes(value)
		if len(data) > maxToolArgumentsBytes*2 {
			return
		}
		var copyValue any
		if json.Unmarshal(data, &copyValue) == nil {
			result[key] = copyValue
		}
	}
	put(string(SlotCorePersona), context.Persona)
	put(string(SlotCurrentState), context.State)
	put(string(SlotCurrentLife), context.Life)
	put(string(SlotSchedule), context.Schedule)
	put(string(SlotVisualIdentity), context.Visual)
	put(string(SlotAppearance), context.Outfit)
	put(string(SlotRelationshipScope), context.Relation)
	put(string(SlotMemoryScope), context.Memory)
	put(string(SlotAgency), context.Agency)
	put(string(SlotRecentOutcomes), context.Outcomes)
	for slot, value := range context.extra {
		put(string(slot), value)
	}
	return result
}

func capabilityContextSnapshotForSlots(context CapabilityContext, slots []ContextSlot) map[string]any {
	all := context.Snapshot()
	result := map[string]any{"identity": all["identity"]}
	for _, slot := range slots {
		if value, ok := all[string(slot)]; ok && value != nil {
			result[string(slot)] = value
		}
	}
	return result
}

// ContextSnapshotFromProjection preserves only the bounded capability
// context needed for replay. It is deliberately not the full cognition read
// model and never includes prompts, provider metadata, or raw message history.
func ContextSnapshotFromProjection(projection ContextProjection) map[string]any {
	return map[string]any{
		"identity": map[string]any{
			"fluctlight_id":   projection.FluctlightID,
			"conversation_id": projection.ConversationID,
			"source_fact_id":  projection.SourceFactID,
		},
		"core_persona":       boundedSnapshotValue(projection.CorePersona),
		"current_state":      boundedSnapshotValue(currentStateSnapshotFromProjection(projection)),
		"current_life":       boundedSnapshotValue(projection.LifeContext),
		"schedule":           boundedSnapshotValue(projection.Schedule),
		"visual_identity":    boundedSnapshotValue(projection.VisualIdentity),
		"appearance":         boundedSnapshotValue(mapValue(projection.Identity["appearance"])),
		"relationship_scope": boundedSnapshotValue(map[string]any{"authorized_actor_ids": relationshipAuthorizedActorIDsFromProjection(projection), "relationships": projection.Relationships}),
		"memory_scope": boundedSnapshotValue(map[string]any{
			"owner_actor_id": projection.OwnerActorID, "viewer_actor_ids": []any{stringValue(projection.CurrentSpeaker["actor_id"])},
			"conversation_mode": projection.MemoryRetrievalTrace.ConversationMode,
			"active_profile_id": stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"]),
			"memories":          projection.Memories, "retrieval_trace": projection.MemoryRetrievalTrace,
		}),
		"agency":                  boundedSnapshotValue(map[string]any{"goals": projection.Goals, "intentions": projection.Intentions}),
		"recent_outcomes":         boundedSnapshotValue(map[string]any{"outcomes": projection.RecentOutcomes}),
		"context_reference_index": boundedSnapshotValue(projection.ReferenceIndex),
	}
}

func currentStateSnapshotFromProjection(projection ContextProjection) map[string]any {
	result := cloneMap(projection.InnerState)
	if result == nil {
		result = map[string]any{}
	}
	if len(projection.AffectProfile) > 0 {
		result["affect_profile"] = cloneMap(projection.AffectProfile)
	}
	return result
}

func relationshipAuthorizedActorIDsFromProjection(projection ContextProjection) []any {
	result := make([]any, 0)
	seen := make(map[string]struct{})
	add := func(actorID string) {
		actorID = strings.TrimSpace(actorID)
		if actorID == "" || actorID == projection.FluctlightID {
			return
		}
		if _, exists := seen[actorID]; exists {
			return
		}
		seen[actorID] = struct{}{}
		result = append(result, actorID)
	}
	if projection.ConversationID != "" {
		add(stringValue(projection.CurrentSpeaker["actor_id"]))
		return result
	}
	for _, relationship := range projection.Relationships {
		add(stringValue(relationship["target_actor_id"]))
	}
	add(stringValue(projection.CurrentSpeaker["actor_id"]))
	return result
}

func boundedSnapshotValue(value any) any {
	data := jsonBytes(value)
	if len(data) == 0 || len(data) > maxToolArgumentsBytes*2 {
		return nil
	}
	var copyValue any
	if json.Unmarshal(data, &copyValue) != nil {
		return nil
	}
	return copyValue
}

// ContextResolver is the only ambient-context acquisition boundary used by
// the canonical Runtime.
type ContextResolver interface {
	Resolve(context.Context, ContextRequest, []ContextSlot) (CapabilityContext, error)
}

type ContextLoader func(context.Context, ContextRequest) (any, error)

// StaticContextResolver is useful for tests and composition roots.  It loads
// exactly the requested slots and reports a typed missing-slot error when a
// requested loader is not installed.
type StaticContextResolver struct {
	Loaders map[ContextSlot]ContextLoader
}

func NewStaticContextResolver(loaders map[ContextSlot]ContextLoader) *StaticContextResolver {
	copyLoaders := make(map[ContextSlot]ContextLoader, len(loaders))
	for slot, loader := range loaders {
		copyLoaders[slot] = loader
	}
	return &StaticContextResolver{Loaders: copyLoaders}
}

func (resolver *StaticContextResolver) Resolve(ctx context.Context, request ContextRequest, slots []ContextSlot) (CapabilityContext, error) {
	if resolver == nil {
		return CapabilityContext{}, fmt.Errorf("%w: resolver is unavailable", ErrContextResolve)
	}
	if err := ctx.Err(); err != nil {
		return CapabilityContext{}, fmt.Errorf("%w: %v", ErrContextResolve, err)
	}
	result := CapabilityContext{Identity: ContextIdentity{FluctlightID: request.FluctlightID, ConversationID: request.ConversationID, SourceFactID: request.SourceFactID, ActionID: request.ActionID}, extra: make(map[ContextSlot]any)}
	seen := make(map[ContextSlot]struct{}, len(slots))
	for _, slot := range slots {
		if !knownContextSlot(slot) {
			return CapabilityContext{}, fmt.Errorf("%w: unknown slot %q", ErrContextResolve, slot)
		}
		if _, duplicate := seen[slot]; duplicate {
			continue
		}
		seen[slot] = struct{}{}
		loader := resolver.Loaders[slot]
		if loader == nil {
			return CapabilityContext{}, fmt.Errorf("%w: missing slot %q", ErrContextResolve, slot)
		}
		value, err := loader(ctx, request)
		if err != nil {
			return CapabilityContext{}, fmt.Errorf("%w: slot %q: %v", ErrContextResolve, slot, err)
		}
		if err := ctx.Err(); err != nil {
			return CapabilityContext{}, fmt.Errorf("%w: %v", ErrContextResolve, err)
		}
		if err := result.set(slot, value); err != nil {
			return CapabilityContext{}, err
		}
	}
	return result, nil
}

// SnapshotContextResolver rehydrates the bounded snapshot persisted with a
// frozen action. It never reads live ambient state; authorization/revision
// guards still belong to the executing capability.
type SnapshotContextResolver struct {
	Snapshot map[string]any
}

func NewSnapshotContextResolver(snapshot map[string]any) *SnapshotContextResolver {
	return &SnapshotContextResolver{Snapshot: mapValue(boundedSnapshotValue(snapshot))}
}

func (resolver *SnapshotContextResolver) Resolve(ctx context.Context, request ContextRequest, slots []ContextSlot) (CapabilityContext, error) {
	if resolver == nil || resolver.Snapshot == nil {
		return CapabilityContext{}, fmt.Errorf("%w: snapshot is unavailable", ErrContextResolve)
	}
	if err := ctx.Err(); err != nil {
		return CapabilityContext{}, fmt.Errorf("%w: %v", ErrContextResolve, err)
	}
	identity := mapValue(resolver.Snapshot["identity"])
	for key, actual := range map[string]string{
		"fluctlight_id": request.FluctlightID, "conversation_id": request.ConversationID,
		"source_fact_id": request.SourceFactID, "action_id": request.ActionID,
	} {
		if expected := stringValue(identity[key]); expected != "" && actual != "" && expected != actual {
			return CapabilityContext{}, fmt.Errorf("%w: snapshot identity %s does not match request", ErrContextResolve, key)
		}
	}
	result := CapabilityContext{Identity: ContextIdentity{FluctlightID: request.FluctlightID, ConversationID: request.ConversationID, SourceFactID: request.SourceFactID, ActionID: request.ActionID}, extra: make(map[ContextSlot]any)}
	seen := make(map[ContextSlot]struct{}, len(slots))
	for _, slot := range slots {
		if !knownContextSlot(slot) {
			return CapabilityContext{}, fmt.Errorf("%w: unknown slot %q", ErrContextResolve, slot)
		}
		if _, duplicate := seen[slot]; duplicate {
			continue
		}
		seen[slot] = struct{}{}
		value, ok := resolver.Snapshot[string(slot)]
		if !ok {
			return CapabilityContext{}, fmt.Errorf("%w: missing snapshot slot %q", ErrContextResolve, slot)
		}
		if err := result.set(slot, value); err != nil {
			return CapabilityContext{}, err
		}
	}
	return result, nil
}

func (context *CapabilityContext) set(slot ContextSlot, value any) error {
	if !knownContextSlot(slot) {
		return fmt.Errorf("%w: unknown slot %q", ErrContextResolve, slot)
	}
	data, err := contextMapValueForSlot(slot, value)
	if err != nil {
		return fmt.Errorf("%w: slot %q: %v", ErrContextResolve, slot, err)
	}
	switch slot {
	case SlotCorePersona:
		context.Persona = &PersonaContext{Data: data}
	case SlotCurrentState:
		context.State = &CurrentStateContext{Data: data}
	case SlotCurrentLife:
		context.Life = &CurrentLifeContext{Data: data}
	case SlotSchedule:
		context.Schedule = &ScheduleContext{Data: data}
	case SlotVisualIdentity:
		context.Visual = &VisualIdentityContext{Data: data}
	case SlotAppearance:
		context.Outfit = &AppearanceContext{Data: data}
	case SlotRelationshipScope:
		context.Relation = &RelationshipScope{Data: data}
	case SlotMemoryScope:
		context.Memory = &MemoryScope{Data: data}
	case SlotAgency:
		context.Agency = &AgencyContext{Data: data}
	case SlotRecentOutcomes:
		context.Outcomes = &RecentOutcomesContext{Data: data}
	}
	return nil
}

func contextMapValueForSlot(slot ContextSlot, value any) (map[string]any, error) {
	matches := false
	switch value.(type) {
	case *PersonaContext:
		matches = slot == SlotCorePersona
	case *CurrentStateContext:
		matches = slot == SlotCurrentState
	case *CurrentLifeContext:
		matches = slot == SlotCurrentLife
	case *ScheduleContext:
		matches = slot == SlotSchedule
	case *VisualIdentityContext:
		matches = slot == SlotVisualIdentity
	case *AppearanceContext:
		matches = slot == SlotAppearance
	case *RelationshipScope:
		matches = slot == SlotRelationshipScope
	case *MemoryScope:
		matches = slot == SlotMemoryScope
	case *AgencyContext:
		matches = slot == SlotAgency
	case *RecentOutcomesContext:
		matches = slot == SlotRecentOutcomes
	default:
		matches = true
	}
	if !matches {
		return nil, fmt.Errorf("typed context value does not match slot %q", slot)
	}
	return contextMapValue(value)
}

func knownContextSlot(slot ContextSlot) bool {
	switch slot {
	case SlotCorePersona, SlotCurrentState, SlotCurrentLife, SlotSchedule, SlotVisualIdentity, SlotAppearance, SlotRelationshipScope, SlotMemoryScope, SlotAgency, SlotRecentOutcomes:
		return true
	default:
		return false
	}
}

func contextMapValue(value any) (map[string]any, error) {
	if value == nil {
		return nil, errors.New("slot value is missing")
	}
	switch typed := value.(type) {
	case map[string]any:
		if typed == nil {
			return nil, errors.New("slot value is missing")
		}
		// Accept only the explicit snapshot wrapper used by older serialized
		// value objects when it is unambiguous; all live values are raw maps.
		if len(typed) == 1 {
			if nested, ok := typed["data"].(map[string]any); ok {
				return nested, nil
			}
		}
		return typed, nil
	case *PersonaContext:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	case *CurrentStateContext:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	case *CurrentLifeContext:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	case *ScheduleContext:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	case *VisualIdentityContext:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	case *AppearanceContext:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	case *RelationshipScope:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	case *MemoryScope:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	case *AgencyContext:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	case *RecentOutcomesContext:
		if typed == nil || typed.Data == nil {
			return nil, errors.New("slot value is missing")
		}
		return typed.Data, nil
	default:
		return nil, fmt.Errorf("slot value must be an object, got %T", value)
	}
}

// CapabilityDefinition is the canonical provider and runtime contract.
type CapabilityDefinition struct {
	Name                  string
	Version               string
	Type                  CapabilityType
	Description           string
	InputSchema           map[string]any
	OutputSchema          map[string]any
	Surfaces              []CapabilitySurface
	TargetKinds           []string
	OutputRole            string
	SideEffectClass       string
	SuccessBoundary       string
	CompletionBoundary    string
	OutcomeReferenceField string
	ConcurrencyClass      string
	SupportsCancel        bool
	SupportsRetry         bool
	RequiresPreflight     bool
	FailurePolicy         CapabilityFailurePolicy
	RequiredContext       []ContextSlot
	// ProvenanceFields are mechanically filled by the Runtime when a thin
	// provider input omits source/evidence/idempotency fields. They are not
	// provider-facing semantic decisions.
	ProvenanceFields       []string `json:"-"`
	NestedProvenanceObject string   `json:"-"`
}

const CapabilityPreparedPayloadSchemaVersion = "fluctlight.capability-prepared.v1"
const CapabilityRuntimePayloadVersion = "v2"

type CapabilityPreparedPayload struct {
	SchemaVersion string         `json:"schema_version"`
	Data          map[string]any `json:"data,omitempty"`
	Provenance    map[string]any `json:"provenance,omitempty"`
}

func decodeCapabilityPreparedPayload(raw json.RawMessage) (CapabilityPreparedPayload, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return CapabilityPreparedPayload{SchemaVersion: CapabilityPreparedPayloadSchemaVersion, Data: map[string]any{}, Provenance: map[string]any{}}, nil
	}
	if len(raw) > maxToolArgumentsBytes*2 {
		return CapabilityPreparedPayload{}, errors.New("capability prepared payload is too large")
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return CapabilityPreparedPayload{}, errors.New("capability prepared payload must be an object")
	}
	for key := range object {
		switch key {
		case "schema_version", "data", "provenance":
		default:
			return CapabilityPreparedPayload{}, fmt.Errorf("capability prepared payload field %q is not allowed", key)
		}
	}
	if stringValue(object["schema_version"]) != CapabilityPreparedPayloadSchemaVersion {
		return CapabilityPreparedPayload{}, errors.New("capability prepared payload schema version is invalid")
	}
	data := mapValue(object["data"])
	if object["data"] != nil && data == nil {
		return CapabilityPreparedPayload{}, errors.New("capability prepared payload data must be an object")
	}
	provenance := mapValue(object["provenance"])
	if object["provenance"] != nil && provenance == nil {
		return CapabilityPreparedPayload{}, errors.New("capability prepared payload provenance must be an object")
	}
	return CapabilityPreparedPayload{SchemaVersion: CapabilityPreparedPayloadSchemaVersion, Data: cloneMap(data), Provenance: cloneMap(provenance)}, nil
}

func encodeCapabilityPreparedPayload(payload CapabilityPreparedPayload) (json.RawMessage, error) {
	if payload.SchemaVersion == "" {
		payload.SchemaVersion = CapabilityPreparedPayloadSchemaVersion
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(encoded) > maxToolArgumentsBytes*2 {
		return nil, errors.New("capability prepared payload is too large")
	}
	return encoded, nil
}

func withCapabilityPreparedData(invocation CapabilityInvocation, key string, value any) (CapabilityInvocation, error) {
	payload, err := decodeCapabilityPreparedPayload(invocation.PreparedPayload)
	if err != nil {
		return invocation, err
	}
	if payload.Data == nil {
		payload.Data = make(map[string]any)
	}
	bounded := boundedSnapshotValue(value)
	if value != nil && bounded == nil {
		return invocation, errors.New("capability prepared data is invalid or too large")
	}
	payload.Data[key] = bounded
	encoded, err := encodeCapabilityPreparedPayload(payload)
	if err != nil {
		return invocation, err
	}
	invocation.PreparedPayload = encoded
	return invocation, nil
}

func capabilityPreparedData(invocation CapabilityInvocation, key string) (any, bool, error) {
	payload, err := decodeCapabilityPreparedPayload(invocation.PreparedPayload)
	if err != nil {
		return nil, false, err
	}
	value, ok := payload.Data[key]
	return value, ok, nil
}

func augmentCapabilityInvocationProvenance(invocation CapabilityInvocation, definition CapabilityDefinition) (CapabilityInvocation, error) {
	if len(definition.ProvenanceFields) == 0 {
		return invocation, nil
	}
	payload, err := decodeCapabilityPreparedPayload(invocation.PreparedPayload)
	if err != nil {
		return invocation, err
	}
	if payload.Provenance == nil {
		payload.Provenance = make(map[string]any)
	}
	for _, key := range definition.ProvenanceFields {
		if _, present := payload.Provenance[key]; present {
			continue
		}
		switch key {
		case "evidence_refs":
			payload.Provenance[key] = []any{invocation.SourceFactID}
		case "idempotency_key":
			payload.Provenance[key] = "capability:" + invocation.CallID
		}
	}
	encoded, err := encodeCapabilityPreparedPayload(payload)
	if err != nil {
		return invocation, err
	}
	invocation.PreparedPayload = encoded
	return invocation, nil
}

func capabilityExecutionArguments(invocation CapabilityInvocation, definition CapabilityDefinition) (map[string]any, error) {
	var arguments map[string]any
	if err := json.Unmarshal(invocation.Arguments, &arguments); err != nil || arguments == nil {
		return nil, errors.New("capability arguments must be an object")
	}
	payload, err := decodeCapabilityPreparedPayload(invocation.PreparedPayload)
	if err != nil {
		return nil, err
	}
	target := arguments
	if definition.NestedProvenanceObject != "" {
		target = mapValue(arguments[definition.NestedProvenanceObject])
		if target == nil {
			return nil, fmt.Errorf("capability provenance target %q is missing", definition.NestedProvenanceObject)
		}
	}
	for _, key := range definition.ProvenanceFields {
		if value, ok := payload.Provenance[key]; ok {
			target[key] = boundedSnapshotValue(value)
		}
	}
	if definition.NestedProvenanceObject != "" {
		arguments[definition.NestedProvenanceObject] = target
	}
	return arguments, nil
}

func (definition CapabilityDefinition) IsDeferredOutput() bool {
	return definition.SideEffectClass == "external_async" && len(definition.TargetKinds) > 0
}

func (definition CapabilityDefinition) SupportsSurface(surface CapabilitySurface) bool {
	if len(definition.Surfaces) == 0 {
		return true
	}
	for _, candidate := range definition.Surfaces {
		if candidate == surface {
			return true
		}
	}
	return false
}

func containsCapabilityTarget(targets []string, target string) bool {
	for _, candidate := range targets {
		if candidate == target {
			return true
		}
	}
	return false
}

func (definition CapabilityDefinition) Validate() error {
	if strings.TrimSpace(definition.Name) == "" || !toolNamePattern.MatchString(definition.Name) {
		return errors.New("capability_definition_invalid_name")
	}
	if strings.TrimSpace(definition.Version) == "" {
		return errors.New("capability_definition_invalid_version")
	}
	switch definition.Type {
	case CapabilityTypeAction, CapabilityTypeQuery, CapabilityTypeInternal:
	default:
		return errors.New("capability_definition_invalid_type")
	}
	if strings.TrimSpace(definition.Description) == "" || len([]rune(definition.Description)) > 512 {
		return errors.New("capability_definition_invalid_description")
	}
	if definition.CompletionBoundary != "" {
		if strings.TrimSpace(definition.SuccessBoundary) == "" || strings.TrimSpace(definition.OutcomeReferenceField) == "" || len([]rune(definition.CompletionBoundary)) > 128 || len([]rune(definition.OutcomeReferenceField)) > 128 {
			return errors.New("capability_definition_async_outcome_invalid")
		}
		if _, ok := mapValue(definition.OutputSchema["properties"])[definition.OutcomeReferenceField]; !ok {
			return errors.New("capability_definition_async_reference_undeclared")
		}
	} else if definition.OutcomeReferenceField != "" {
		return errors.New("capability_definition_async_outcome_invalid")
	}
	if definition.InputSchema != nil && stringValue(definition.InputSchema["type"]) != "object" {
		return errors.New("capability_definition_input_schema_invalid")
	}
	if err := validateCapabilitySchemaDefinition(definition.InputSchema, nil); err != nil {
		return fmt.Errorf("capability_definition_input_schema_invalid: %w", err)
	}
	if err := validateCapabilitySchemaDefinition(definition.OutputSchema, nil); err != nil {
		return fmt.Errorf("capability_definition_output_schema_invalid: %w", err)
	}
	if definition.FailurePolicy != FailurePolicyRequiredForVisibleClaim && definition.FailurePolicy != FailurePolicyOptionalInternal {
		return errors.New("capability_definition_failure_policy_invalid")
	}
	for _, slot := range definition.RequiredContext {
		if !knownContextSlot(slot) {
			return fmt.Errorf("capability_definition_context_slot_invalid: %q", slot)
		}
	}
	return nil
}

// CapabilityInvocation is the canonical call envelope produced by the
// Provider codec and persisted by the runtime.
type CapabilityInvocation struct {
	CallID            string             `json:"call_id"`
	CapabilityName    string             `json:"capability_name"`
	SchemaVersion     string             `json:"schema_version"`
	Arguments         json.RawMessage    `json:"arguments"`
	PreparedPayload   json.RawMessage    `json:"prepared_payload,omitempty"`
	Intent            string             `json:"intent,omitempty"`
	SourceFactID      string             `json:"source_fact_id"`
	ActionID          string             `json:"action_id,omitempty"`
	ProviderRequestID string             `json:"provider_request_id"`
	Sequence          int                `json:"sequence"`
	Metadata          InvocationMetadata `json:"metadata,omitempty"`
	ContextSnapshot   map[string]any     `json:"context_snapshot,omitempty"`
}

type InvocationMetadata struct {
	CorrelationID  string            `json:"correlation_id,omitempty"`
	Surface        CapabilitySurface `json:"surface,omitempty"`
	OutputBinding  *OutputBindingV1  `json:"output_binding,omitempty"`
	FluctlightID   string            `json:"fluctlight_id,omitempty"`
	ConversationID string            `json:"conversation_id,omitempty"`
}

func (definition CapabilityDefinition) ValidateOutput(output any) error {
	if definition.OutputSchema == nil {
		return nil
	}
	return validateCapabilitySchemaValue(output, definition.OutputSchema)
}

func capabilityInvocationsFromValue(value any) ([]CapabilityInvocation, error) {
	if value == nil {
		return []CapabilityInvocation{}, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("capability invocation payload invalid: %w", err)
	}
	if string(data) == "null" {
		return []CapabilityInvocation{}, nil
	}
	var invocations []CapabilityInvocation
	if err := json.Unmarshal(data, &invocations); err != nil {
		return nil, fmt.Errorf("capability invocation payload invalid: %w", err)
	}
	seen := make(map[string]struct{}, len(invocations))
	for index := range invocations {
		if strings.TrimSpace(invocations[index].CallID) == "" || strings.TrimSpace(invocations[index].CapabilityName) == "" {
			return nil, fmt.Errorf("capability invocation %d identity is missing", index)
		}
		if _, duplicate := seen[invocations[index].CallID]; duplicate {
			return nil, fmt.Errorf("capability invocation %d call id is duplicated", index)
		}
		seen[invocations[index].CallID] = struct{}{}
		if invocations[index].SchemaVersion != CapabilityInvocationSchemaVersion {
			return nil, fmt.Errorf("capability invocation %d schema version is invalid", index)
		}
	}
	return invocations, nil
}

func (invocation CapabilityInvocation) Validate(definition CapabilityDefinition) error {
	if invocation.SchemaVersion != "" && invocation.SchemaVersion != CapabilityInvocationSchemaVersion {
		return fmt.Errorf("%w: invocation schema version", ErrInvalidArguments)
	}
	if strings.TrimSpace(invocation.CallID) == "" || strings.TrimSpace(invocation.CapabilityName) == "" || invocation.CapabilityName != definition.Name {
		return fmt.Errorf("%w: invocation identity", ErrInvalidArguments)
	}
	if strings.TrimSpace(invocation.SourceFactID) == "" {
		return fmt.Errorf("%w: source fact is required", ErrInvalidArguments)
	}
	if strings.TrimSpace(invocation.ProviderRequestID) == "" {
		return fmt.Errorf("%w: provider request is required", ErrInvalidArguments)
	}
	if invocation.Sequence < 0 {
		return fmt.Errorf("%w: sequence is invalid", ErrInvalidArguments)
	}
	arguments, err := normalizeToolArguments(string(invocation.Arguments))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArguments, err)
	}
	var object map[string]any
	if err := json.Unmarshal(arguments, &object); err != nil || object == nil {
		return fmt.Errorf("%w: arguments must be an object", ErrInvalidArguments)
	}
	if err := validateRequiredSchemaFields(object, definition.InputSchema); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArguments, err)
	}
	if len(invocation.PreparedPayload) > 0 {
		if _, err := decodeCapabilityPreparedPayload(invocation.PreparedPayload); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidArguments, err)
		}
	}
	return nil
}

func validateRequiredSchemaFields(object map[string]any, schema map[string]any) error {
	return validateCapabilitySchemaValue(object, schema)
}

func validateCapabilitySchemaDefinition(schema map[string]any, inheritedProperties map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	typeName := stringValue(schema["type"])
	if typeName != "" {
		switch typeName {
		case "object", "array", "string", "number", "integer", "boolean", "null":
		default:
			return fmt.Errorf("unsupported schema type %q", typeName)
		}
	}
	if pattern := stringValue(schema["pattern"]); pattern != "" {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid string pattern: %w", err)
		}
	}
	properties := make(map[string]any, len(inheritedProperties)+len(mapValue(schema["properties"])))
	for key, value := range inheritedProperties {
		properties[key] = value
	}
	for key, value := range mapValue(schema["properties"]) {
		properties[key] = value
	}
	for _, raw := range arrayValue(schema["required"]) {
		key := strings.TrimSpace(stringValue(raw))
		if key == "" {
			return errors.New("required property name is empty")
		}
		if _, ok := properties[key]; !ok {
			return fmt.Errorf("required property %q is not declared", key)
		}
	}
	for key, raw := range mapValue(schema["properties"]) {
		child := mapValue(raw)
		if len(child) == 0 {
			return fmt.Errorf("property %q schema is invalid", key)
		}
		if err := validateCapabilitySchemaDefinition(child, nil); err != nil {
			return fmt.Errorf("property %q: %w", key, err)
		}
	}
	if rawItems, present := schema["items"]; present {
		items := mapValue(rawItems)
		if len(items) == 0 {
			return errors.New("array items schema is invalid")
		}
		if err := validateCapabilitySchemaDefinition(items, nil); err != nil {
			return fmt.Errorf("array items: %w", err)
		}
	}
	for _, keyword := range []string{"anyOf", "oneOf"} {
		if rawAlternatives, present := schema[keyword]; present {
			alternatives := arrayValue(rawAlternatives)
			if len(alternatives) == 0 {
				return fmt.Errorf("%s must contain at least one schema", keyword)
			}
			for index, raw := range alternatives {
				candidate := mapValue(raw)
				if len(candidate) == 0 {
					return fmt.Errorf("%s[%d] schema is invalid", keyword, index)
				}
				if err := validateCapabilitySchemaDefinition(candidate, properties); err != nil {
					return fmt.Errorf("%s[%d]: %w", keyword, index, err)
				}
			}
		}
	}
	if raw, present := schema["additionalProperties"]; present {
		if _, ok := raw.(bool); !ok {
			return errors.New("additionalProperties must be boolean")
		}
	}
	return nil
}

// validateCapabilitySchemaValue implements the bounded JSON-Schema subset used
// by CapabilityDefinition.InputSchema. Provider validation is still the first
// line of defense, but replay and internally produced invocations must enforce
// the same required fields, primitive types, enums and bounds at the Runtime
// boundary as well.
func validateCapabilitySchemaValue(value any, schema map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	typeName := stringValue(schema["type"])
	if typeName == "" {
		// `anyOf` branches in the compact capability schemas intentionally omit
		// a repeated type declaration and contribute only required keys.
		if len(arrayValue(schema["required"])) > 0 || len(mapValue(schema["properties"])) > 0 {
			object, ok := value.(map[string]any)
			if !ok || object == nil {
				return errors.New("value must be an object")
			}
			for _, raw := range arrayValue(schema["required"]) {
				key := stringValue(raw)
				if key != "" {
					child, present := object[key]
					if !present || child == nil {
						return fmt.Errorf("required field %q is missing", key)
					}
				}
			}
			properties := mapValue(schema["properties"])
			for key, raw := range properties {
				child, present := object[key]
				if !present || child == nil {
					continue
				}
				if err := validateCapabilitySchemaValue(child, mapValue(raw)); err != nil {
					return fmt.Errorf("field %q: %w", key, err)
				}
			}
			if additional, present := schema["additionalProperties"].(bool); present && !additional {
				for key := range object {
					if _, declared := properties[key]; !declared {
						return fmt.Errorf("additional property %q is not allowed", key)
					}
				}
			}
		}
	}
	switch typeName {
	case "object":
		object, ok := value.(map[string]any)
		if !ok || object == nil {
			return errors.New("value must be an object")
		}
		for _, raw := range arrayValue(schema["required"]) {
			key := stringValue(raw)
			if key != "" {
				child, present := object[key]
				if !present || child == nil {
					return fmt.Errorf("required field %q is missing", key)
				}
			}
		}
		properties := mapValue(schema["properties"])
		for key, raw := range properties {
			child, present := object[key]
			if !present || child == nil {
				continue
			}
			if err := validateCapabilitySchemaValue(child, mapValue(raw)); err != nil {
				return fmt.Errorf("field %q: %w", key, err)
			}
		}
		if additional, present := schema["additionalProperties"].(bool); present && !additional {
			for key := range object {
				if _, declared := properties[key]; !declared {
					return fmt.Errorf("additional property %q is not allowed", key)
				}
			}
		}
	case "array":
		switch value.(type) {
		case []any, []map[string]any, []string:
		default:
			return errors.New("value must be an array")
		}
		items := arrayValue(value)
		if minItems := intValue(schema["minItems"]); minItems > 0 && len(items) < minItems {
			return fmt.Errorf("array is shorter than minItems %d", minItems)
		}
		if maxItems := intValue(schema["maxItems"]); maxItems > 0 && len(items) > maxItems {
			return fmt.Errorf("array exceeds maxItems %d", maxItems)
		}
		itemSchema := mapValue(schema["items"])
		for index, item := range items {
			if err := validateCapabilitySchemaValue(item, itemSchema); err != nil {
				return fmt.Errorf("item %d: %w", index, err)
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return errors.New("value must be a string")
		}
		if min := intValue(schema["minLength"]); min > 0 && len([]rune(text)) < min {
			return fmt.Errorf("string is shorter than minLength %d", min)
		}
		if max := intValue(schema["maxLength"]); max > 0 && len([]rune(text)) > max {
			return fmt.Errorf("string exceeds maxLength %d", max)
		}
		if pattern := stringValue(schema["pattern"]); pattern != "" {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return errors.New("string schema pattern is invalid")
			}
			if !compiled.MatchString(text) {
				return errors.New("string does not match pattern")
			}
		}
	case "number":
		number, ok := numberFloat(value)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
			return errors.New("value must be a number")
		}
		if minimum, ok := numberFloat(schema["minimum"]); ok && number < minimum {
			return fmt.Errorf("number is below minimum %v", minimum)
		}
		if maximum, ok := numberFloat(schema["maximum"]); ok && number > maximum {
			return fmt.Errorf("number exceeds maximum %v", maximum)
		}
	case "integer":
		number, ok := numberFloat(value)
		if !ok || math.Trunc(number) != number {
			return errors.New("value must be an integer")
		}
		if minimum, ok := numberFloat(schema["minimum"]); ok && number < minimum {
			return fmt.Errorf("integer is below minimum %v", minimum)
		}
		if maximum, ok := numberFloat(schema["maximum"]); ok && number > maximum {
			return fmt.Errorf("integer exceeds maximum %v", maximum)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("value must be a boolean")
		}
	case "null":
		if value != nil {
			return errors.New("value must be null")
		}
	}
	if enum := arrayValue(schema["enum"]); len(enum) > 0 {
		matched := false
		for _, candidate := range enum {
			if reflect.DeepEqual(candidate, value) || stringValue(candidate) != "" && stringValue(candidate) == stringValue(value) {
				matched = true
				break
			}
		}
		if !matched {
			return errors.New("value is not in enum")
		}
	}
	if alternatives := arrayValue(schema["anyOf"]); len(alternatives) > 0 {
		matched := false
		for _, raw := range alternatives {
			if candidate := mapValue(raw); len(candidate) > 0 && validateCapabilitySchemaValue(value, candidate) == nil {
				matched = true
				break
			}
		}
		if !matched {
			return errors.New("value does not match anyOf schema")
		}
	}
	if alternatives := arrayValue(schema["oneOf"]); len(alternatives) > 0 {
		matches := 0
		for _, raw := range alternatives {
			if candidate := mapValue(raw); len(candidate) > 0 && validateCapabilitySchemaValue(value, candidate) == nil {
				matches++
			}
		}
		if matches != 1 {
			return errors.New("value does not match exactly one oneOf schema")
		}
	}
	return nil
}

type CapabilityResult struct {
	CallID            string        `json:"call_id"`
	CapabilityName    string        `json:"capability_name"`
	Status            string        `json:"status"`
	Output            any           `json:"output,omitempty"`
	ErrorCode         string        `json:"error_code,omitempty"`
	Retryable         bool          `json:"retryable"`
	ProviderRequestID string        `json:"provider_request_id,omitempty"`
	CorrelationID     string        `json:"correlation_id,omitempty"`
	Duration          time.Duration `json:"-"`
	RequiredContext   []ContextSlot `json:"required_context,omitempty"`
}

func (result CapabilityResult) Validate(invocation CapabilityInvocation) error {
	if result.CallID != invocation.CallID || result.CapabilityName != invocation.CapabilityName {
		return errors.New("capability result identity invalid")
	}
	switch result.Status {
	case "completed", "failed", "rejected", "deferred":
		return nil
	default:
		return errors.New("capability result status invalid")
	}
}

// Capability is the implementation seam for migrated capabilities.
type Capability interface {
	Definition() CapabilityDefinition
	RequiredContext() []ContextSlot
	Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}

type CapabilityPreflighter interface {
	Preflight(context.Context, CapabilityContext) error
}

type CapabilityPreparer interface {
	Prepare(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityInvocation, error)
}

type DeferredCapability interface {
	Capability
	ExecuteDeferredTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext, OutputBindingV1) (CapabilityResult, error)
}

// TransactionalCapability applies a native mutation inside the caller-owned
// Unit of Work. It must not perform Provider, Redis, object-storage, Temporal,
// or other external I/O while the transaction is open.
type TransactionalCapability interface {
	Capability
	ExecuteTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}

var (
	ErrCapabilityNotFound  = errors.New("capability not found")
	ErrInvalidArguments    = errors.New("invalid capability arguments")
	ErrContextResolve      = errors.New("capability context resolve failed")
	ErrCapabilityExecution = errors.New("capability execution failed")
)

// CapabilityError preserves a bounded domain code and retry policy while
// participating in the ordinary Go errors.Is/errors.As chain.
type CapabilityError struct {
	Code      string
	Retryable bool
	Cause     error
}

func (err *CapabilityError) Error() string {
	if err == nil {
		return ""
	}
	if err.Cause == nil {
		return err.Code
	}
	if strings.TrimSpace(err.Code) == "" {
		return err.Cause.Error()
	}
	return err.Code + ": " + err.Cause.Error()
}

func (err *CapabilityError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func newCapabilityError(code string, retryable bool, cause error) error {
	return &CapabilityError{Code: code, Retryable: retryable, Cause: cause}
}

func capabilityErrorInfo(err error, fallbackCode string, fallbackRetryable bool) (string, bool) {
	var typed *CapabilityError
	if errors.As(err, &typed) {
		code := strings.TrimSpace(typed.Code)
		if code == "" {
			code = fallbackCode
		}
		return code, typed.Retryable
	}
	return fallbackCode, fallbackRetryable
}

// CapabilityRuntime executes canonical Capability implementations.  It is
// intentionally independent from App so a dummy capability can be registered
// and exercised without changing MainAgent or schema switches.
type CapabilityRuntime struct {
	Registry *CapabilityRegistry
	Resolver ContextResolver
	Logger   *slog.Logger
}

func NewCapabilityRuntime(registry *CapabilityRegistry, resolver ContextResolver) (*CapabilityRuntime, error) {
	if registry == nil {
		return nil, ErrCapabilityNotFound
	}
	if resolver == nil {
		return nil, ErrContextResolve
	}
	return &CapabilityRuntime{Registry: registry, Resolver: resolver, Logger: slog.Default()}, nil
}

func (runtime *CapabilityRuntime) Execute(ctx context.Context, invocation CapabilityInvocation) (CapabilityResult, error) {
	started := time.Now()
	if runtime == nil || runtime.Registry == nil {
		return failedCapabilityResult(invocation, "capability_not_found", false), fmt.Errorf("%w: registry is nil", ErrCapabilityNotFound)
	}
	definition, ok := runtime.Registry.Definition(invocation.CapabilityName)
	if !ok {
		return failedCapabilityResult(invocation, "capability_not_found", false), fmt.Errorf("%w: %s", ErrCapabilityNotFound, invocation.CapabilityName)
	}
	capability, ok := runtime.Registry.LookupCapability(invocation.CapabilityName)
	if !ok {
		return failedCapabilityResult(invocation, "capability_not_found", false), fmt.Errorf("%w: %s", ErrCapabilityNotFound, invocation.CapabilityName)
	}
	if _, transactional := capability.(TransactionalCapability); transactional {
		return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
	}
	var err error
	invocation, err = augmentCapabilityInvocationProvenance(invocation, definition)
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), fmt.Errorf("%w: %v", ErrInvalidArguments, err)
	}
	if err := invocation.Validate(definition); err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	if err := ctx.Err(); err != nil {
		return failedCapabilityResult(invocation, "execution_cancelled", true), fmt.Errorf("%w: %v", ErrCapabilityExecution, err)
	}
	request := ContextRequest{FluctlightID: invocation.Metadata.FluctlightID, ConversationID: invocation.Metadata.ConversationID, SourceFactID: invocation.SourceFactID, ActionID: invocation.ActionID, CorrelationID: invocation.Metadata.CorrelationID, Surface: invocation.Metadata.Surface, SemanticIntent: invocation.Intent}
	resolver := runtime.Resolver
	if resolver == nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", false), ErrContextResolve
	}
	if len(invocation.ContextSnapshot) > 0 {
		resolver = NewSnapshotContextResolver(invocation.ContextSnapshot)
	}
	resolved, err := resolver.Resolve(ctx, request, runtime.Registry.RequiredContext(invocation.CapabilityName))
	if err != nil {
		result := failedCapabilityResult(invocation, "context_resolve_failed", true)
		runtime.log(ctx, invocation, definition, result, started, err)
		return result, fmt.Errorf("%w: %v", ErrContextResolve, err)
	}
	if preflight, ok := capability.(CapabilityPreflighter); ok {
		if err := preflight.Preflight(ctx, resolved); err != nil {
			result := failedCapabilityResult(invocation, "context_resolve_failed", true)
			runtime.log(ctx, invocation, definition, result, started, err)
			return result, fmt.Errorf("%w: preflight: %v", ErrContextResolve, err)
		}
	}
	if preparer, ok := capability.(CapabilityPreparer); ok && len(invocation.PreparedPayload) == 0 {
		prepared, prepareErr := preparer.Prepare(ctx, invocation, resolved)
		if prepareErr != nil {
			code, retryable := capabilityErrorInfo(prepareErr, "execution_failed", true)
			result := failedCapabilityResult(invocation, code, retryable)
			runtime.log(ctx, invocation, definition, result, started, prepareErr)
			return result, fmt.Errorf("%w: %w", ErrCapabilityExecution, prepareErr)
		}
		invocation = prepared
	}
	if len(invocation.ContextSnapshot) == 0 && len(definition.RequiredContext) > 0 {
		invocation.ContextSnapshot = capabilityContextSnapshotForSlots(resolved, definition.RequiredContext)
	}
	result, executeErr := capability.Execute(ctx, invocation, resolved)
	if result.CallID != "" && result.CallID != invocation.CallID {
		result = failedCapabilityResult(invocation, "execution_failed", false)
		err := errors.New("capability result call identity mismatch")
		runtime.log(ctx, invocation, definition, result, started, err)
		return result, fmt.Errorf("%w: %v", ErrCapabilityExecution, err)
	}
	if result.CapabilityName != "" && result.CapabilityName != invocation.CapabilityName {
		result = failedCapabilityResult(invocation, "execution_failed", false)
		err := errors.New("capability result name mismatch")
		runtime.log(ctx, invocation, definition, result, started, err)
		return result, fmt.Errorf("%w: %v", ErrCapabilityExecution, err)
	}
	if result.CallID == "" {
		result.CallID = invocation.CallID
	}
	if result.CapabilityName == "" {
		result.CapabilityName = invocation.CapabilityName
	}
	if result.ProviderRequestID == "" {
		result.ProviderRequestID = invocation.ProviderRequestID
	}
	if result.CorrelationID == "" {
		result.CorrelationID = "capability:" + invocation.CallID
	}
	result.RequiredContext = append([]ContextSlot(nil), runtime.Registry.RequiredContext(invocation.CapabilityName)...)
	if executeErr != nil {
		result.Status = "failed"
		if result.ErrorCode == "" {
			result.ErrorCode = "execution_failed"
		}
		runtime.log(ctx, invocation, definition, result, started, executeErr)
		return result, fmt.Errorf("%w: %v", ErrCapabilityExecution, executeErr)
	}
	if result.Status == "" {
		result.Status = "completed"
	}
	if result.Status != "completed" && result.Status != "failed" && result.Status != "rejected" && result.Status != "deferred" {
		result.Status = "failed"
		result.ErrorCode = "execution_failed"
		err = errors.New("capability result status invalid")
		runtime.log(ctx, invocation, definition, result, started, err)
		return result, fmt.Errorf("%w: %v", ErrCapabilityExecution, err)
	}
	if result.Status == "completed" {
		if err := definition.ValidateOutput(result.Output); err != nil {
			result.Status = "failed"
			result.ErrorCode = "execution_failed"
			result.Retryable = false
			runtime.log(ctx, invocation, definition, result, started, err)
			return result, fmt.Errorf("%w: invalid output: %v", ErrCapabilityExecution, err)
		}
	}
	runtime.log(ctx, invocation, definition, result, started, nil)
	return result, nil
}

// Prepare validates and resolves a deferred invocation without executing its
// target side effect. It is the only pre-target path for image/planner
// compilation; callers persist the returned invocation before target binding.
func (runtime *CapabilityRuntime) Prepare(ctx context.Context, invocation CapabilityInvocation) (CapabilityInvocation, CapabilityContext, error) {
	if runtime == nil || runtime.Registry == nil {
		return invocation, CapabilityContext{}, ErrCapabilityNotFound
	}
	definition, ok := runtime.Registry.Definition(invocation.CapabilityName)
	if !ok {
		return invocation, CapabilityContext{}, ErrCapabilityNotFound
	}
	var err error
	invocation, err = augmentCapabilityInvocationProvenance(invocation, definition)
	if err != nil {
		return invocation, CapabilityContext{}, fmt.Errorf("%w: %v", ErrInvalidArguments, err)
	}
	if err := invocation.Validate(definition); err != nil {
		return invocation, CapabilityContext{}, err
	}
	if runtime.Resolver == nil {
		return invocation, CapabilityContext{}, ErrContextResolve
	}
	resolver := runtime.Resolver
	if len(invocation.ContextSnapshot) > 0 {
		resolver = NewSnapshotContextResolver(invocation.ContextSnapshot)
	}
	resolved, err := resolver.Resolve(ctx, ContextRequest{FluctlightID: invocation.Metadata.FluctlightID, ConversationID: invocation.Metadata.ConversationID, SourceFactID: invocation.SourceFactID, ActionID: invocation.ActionID, CorrelationID: invocation.Metadata.CorrelationID, Surface: invocation.Metadata.Surface, SemanticIntent: invocation.Intent}, definition.RequiredContext)
	if err != nil {
		return invocation, CapabilityContext{}, fmt.Errorf("%w: %v", ErrContextResolve, err)
	}
	if preflight, ok := runtime.Registry.capabilities[invocation.CapabilityName].(CapabilityPreflighter); ok {
		if err := preflight.Preflight(ctx, resolved); err != nil {
			return invocation, resolved, fmt.Errorf("%w: preflight: %v", ErrContextResolve, err)
		}
	}
	if preparer, ok := runtime.Registry.capabilities[invocation.CapabilityName].(CapabilityPreparer); ok {
		prepared, err := preparer.Prepare(ctx, invocation, resolved)
		if err != nil {
			return invocation, resolved, fmt.Errorf("%w: %w", ErrCapabilityExecution, err)
		}
		invocation = prepared
		if len(invocation.PreparedPayload) == 0 {
			return invocation, resolved, fmt.Errorf("%w: capability preparer did not produce a frozen payload", ErrCapabilityExecution)
		}
	}
	if len(invocation.ContextSnapshot) == 0 && len(definition.RequiredContext) > 0 {
		invocation.ContextSnapshot = capabilityContextSnapshotForSlots(resolved, definition.RequiredContext)
	}
	if len(invocation.PreparedPayload) == 0 {
		encoded, encodeErr := encodeCapabilityPreparedPayload(CapabilityPreparedPayload{})
		if encodeErr != nil {
			return invocation, resolved, fmt.Errorf("%w: %v", ErrInvalidArguments, encodeErr)
		}
		invocation.PreparedPayload = encoded
	}
	return invocation, resolved, nil
}

func (runtime *CapabilityRuntime) ExecuteMany(ctx context.Context, invocations []CapabilityInvocation) ([]CapabilityResult, error) {
	results := make([]CapabilityResult, 0, len(invocations))
	if runtime == nil || runtime.Registry == nil {
		return results, fmt.Errorf("%w: registry is nil", ErrCapabilityNotFound)
	}
	var firstErr error
	for _, invocation := range invocations {
		result, err := runtime.Execute(ctx, invocation)
		results = append(results, result)
		if err != nil {
			if definition, ok := runtime.Registry.Definition(invocation.CapabilityName); !ok || definition.FailurePolicy == FailurePolicyRequiredForVisibleClaim {
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	return results, firstErr
}

// ExecuteDeferred binds an output-producing canonical capability to a durable
// target. It mirrors Execute's validation/context/error taxonomy while making
// the transaction ownership explicit to the caller.
func (runtime *CapabilityRuntime) ExecuteDeferred(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, binding OutputBindingV1) (CapabilityResult, error) {
	if runtime == nil || runtime.Registry == nil {
		return failedCapabilityResult(invocation, "capability_not_found", false), fmt.Errorf("%w: registry is nil", ErrCapabilityNotFound)
	}
	definition, ok := runtime.Registry.Definition(invocation.CapabilityName)
	if !ok {
		return failedCapabilityResult(invocation, "capability_not_found", false), fmt.Errorf("%w: %s", ErrCapabilityNotFound, invocation.CapabilityName)
	}
	capability, ok := runtime.Registry.LookupCapability(invocation.CapabilityName)
	deferred, ok := capability.(DeferredCapability)
	if !ok {
		return failedCapabilityResult(invocation, "execution_failed", false), fmt.Errorf("%w: capability is not deferred", ErrCapabilityExecution)
	}
	if len(invocation.PreparedPayload) == 0 || (len(definition.RequiredContext) > 0 && len(invocation.ContextSnapshot) == 0) {
		return failedCapabilityResult(invocation, "prepared_invocation_required", false), fmt.Errorf("%w: deferred invocation is not frozen", ErrInvalidArguments)
	}
	var err error
	invocation, err = augmentCapabilityInvocationProvenance(invocation, definition)
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), fmt.Errorf("%w: %v", ErrInvalidArguments, err)
	}
	if err := invocation.Validate(definition); err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	request := ContextRequest{FluctlightID: invocation.Metadata.FluctlightID, ConversationID: invocation.Metadata.ConversationID, SourceFactID: invocation.SourceFactID, ActionID: invocation.ActionID, CorrelationID: invocation.Metadata.CorrelationID, Surface: invocation.Metadata.Surface, SemanticIntent: invocation.Intent}
	resolver := runtime.Resolver
	if resolver == nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", false), ErrContextResolve
	}
	if len(invocation.ContextSnapshot) > 0 {
		resolver = NewSnapshotContextResolver(invocation.ContextSnapshot)
	}
	resolved, err := resolver.Resolve(ctx, request, definition.RequiredContext)
	if err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), fmt.Errorf("%w: %v", ErrContextResolve, err)
	}
	result, executeErr := deferred.ExecuteDeferredTx(ctx, tx, invocation, resolved, binding)
	if result.CallID == "" {
		result.CallID = invocation.CallID
	}
	if result.CapabilityName == "" {
		result.CapabilityName = invocation.CapabilityName
	}
	if executeErr != nil {
		result.Status = "failed"
		if result.ErrorCode == "" {
			result.ErrorCode = "capability_execution_failed"
		}
		result.CallID = invocation.CallID
		result.CapabilityName = invocation.CapabilityName
		if result.ProviderRequestID == "" {
			result.ProviderRequestID = invocation.ProviderRequestID
		}
		return result, fmt.Errorf("%w: %v", ErrCapabilityExecution, executeErr)
	}
	if err := result.Validate(invocation); err != nil {
		return failedCapabilityResult(invocation, "execution_failed", false), fmt.Errorf("%w: %v", ErrCapabilityExecution, err)
	}
	if result.Status == "completed" {
		if err := definition.ValidateOutput(result.Output); err != nil {
			return failedCapabilityResult(invocation, "execution_failed", false), fmt.Errorf("%w: invalid output: %v", ErrCapabilityExecution, err)
		}
	}
	return result, nil
}

func (runtime *CapabilityRuntime) ExecuteTransactional(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation) (CapabilityResult, error) {
	if runtime == nil || runtime.Registry == nil {
		return failedCapabilityResult(invocation, "capability_not_found", false), fmt.Errorf("%w: registry is nil", ErrCapabilityNotFound)
	}
	definition, ok := runtime.Registry.Definition(invocation.CapabilityName)
	if !ok {
		return failedCapabilityResult(invocation, "capability_not_found", false), fmt.Errorf("%w: %s", ErrCapabilityNotFound, invocation.CapabilityName)
	}
	capability, ok := runtime.Registry.LookupCapability(invocation.CapabilityName)
	transactional, ok := capability.(TransactionalCapability)
	if !ok {
		return failedCapabilityResult(invocation, "execution_failed", false), fmt.Errorf("%w: capability is not transactional", ErrCapabilityExecution)
	}
	if len(invocation.PreparedPayload) == 0 || (len(definition.RequiredContext) > 0 && len(invocation.ContextSnapshot) == 0) {
		return failedCapabilityResult(invocation, "prepared_invocation_required", false), fmt.Errorf("%w: transactional invocation is not frozen", ErrInvalidArguments)
	}
	var err error
	invocation, err = augmentCapabilityInvocationProvenance(invocation, definition)
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), fmt.Errorf("%w: %v", ErrInvalidArguments, err)
	}
	if err := invocation.Validate(definition); err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	resolver := runtime.Resolver
	if resolver == nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", false), ErrContextResolve
	}
	if len(invocation.ContextSnapshot) > 0 {
		resolver = NewSnapshotContextResolver(invocation.ContextSnapshot)
	}
	request := ContextRequest{FluctlightID: invocation.Metadata.FluctlightID, ConversationID: invocation.Metadata.ConversationID, SourceFactID: invocation.SourceFactID, ActionID: invocation.ActionID, CorrelationID: invocation.Metadata.CorrelationID, Surface: invocation.Metadata.Surface, SemanticIntent: invocation.Intent}
	resolved, err := resolver.Resolve(ctx, request, definition.RequiredContext)
	if err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), fmt.Errorf("%w: %v", ErrContextResolve, err)
	}
	result, executeErr := transactional.ExecuteTx(ctx, tx, invocation, resolved)
	if result.CallID == "" {
		result.CallID = invocation.CallID
	}
	if result.CapabilityName == "" {
		result.CapabilityName = invocation.CapabilityName
	}
	if result.ProviderRequestID == "" {
		result.ProviderRequestID = invocation.ProviderRequestID
	}
	if result.CorrelationID == "" {
		result.CorrelationID = "capability:" + invocation.CallID
	}
	result.RequiredContext = append([]ContextSlot(nil), definition.RequiredContext...)
	if executeErr != nil {
		result.Status = "failed"
		if result.ErrorCode == "" {
			result.ErrorCode = "capability_execution_failed"
		}
		return result, fmt.Errorf("%w: %w", ErrCapabilityExecution, executeErr)
	}
	if err := result.Validate(invocation); err != nil {
		return failedCapabilityResult(invocation, "execution_failed", false), fmt.Errorf("%w: %v", ErrCapabilityExecution, err)
	}
	if result.Status == "completed" {
		if err := definition.ValidateOutput(result.Output); err != nil {
			return failedCapabilityResultDetail(invocation, "capability_output_invalid", false, err.Error()), fmt.Errorf("%w: invalid output: %v", ErrCapabilityExecution, err)
		}
	}
	return result, nil
}

func failedCapabilityResult(invocation CapabilityInvocation, code string, retryable bool) CapabilityResult {
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "failed", ErrorCode: code, Retryable: retryable, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "capability:" + invocation.CallID}
}

func failedCapabilityResultDetail(invocation CapabilityInvocation, code string, retryable bool, detail string) CapabilityResult {
	result := failedCapabilityResult(invocation, code, retryable)
	if strings.TrimSpace(detail) != "" {
		result.Output = map[string]any{"detail": detail}
	}
	return result
}

func (runtime *CapabilityRuntime) log(ctx context.Context, invocation CapabilityInvocation, definition CapabilityDefinition, result CapabilityResult, started time.Time, err error) {
	if runtime == nil || runtime.Logger == nil {
		return
	}
	duration := time.Since(started)
	level := slog.LevelInfo
	if err != nil || result.Status == "failed" || result.Status == "rejected" {
		level = slog.LevelWarn
	}
	attrs := []any{"capability", definition.Name, "call_id", invocation.CallID, "type", definition.Type, "surface", invocation.Metadata.Surface, "required_context", definition.RequiredContext, "duration_ms", duration.Milliseconds(), "status", result.Status, "retryable", result.Retryable}
	intent := strings.TrimSpace(invocation.Intent)
	attrs = append(attrs, "intent_present", intent != "", "intent_runes", len([]rune(intent)))
	if intent != "" {
		attrs = append(attrs, "intent_digest", stableDigest(intent))
	}
	if result.ErrorCode != "" {
		attrs = append(attrs, "error_code", result.ErrorCode)
	}
	if ctx == nil {
		ctx = context.TODO()
	}
	runtime.Logger.Log(ctx, level, "capability lifecycle", attrs...)
}
