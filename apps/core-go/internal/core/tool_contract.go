package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

const (
	CapabilityInvocationSchemaVersion = "fluctlight.capability-invocation.v2"
	maxToolNameLength                 = 128
	maxToolArgumentsBytes             = 64 << 10
)

var toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// ProviderCompletion is the normalized provider result used by the
// application boundary. Visible text, structured sidecar data, and native
// tool calls share one result instead of being inferred from prose later.
type ProviderCompletion struct {
	Text               string
	Structured         map[string]any
	ToolCalls          []CapabilityInvocation
	DoneSeen           bool
	StructuredFallback bool
}

type CapabilityRegistry struct {
	capabilities map[string]Capability
	definitions  map[string]CapabilityDefinition
}

func NewCapabilityRegistry(entries ...Capability) (*CapabilityRegistry, error) {
	registry := &CapabilityRegistry{capabilities: make(map[string]Capability, len(entries)), definitions: make(map[string]CapabilityDefinition, len(entries))}
	for _, entry := range entries {
		if err := registry.Register(entry); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func NewCapabilityRegistryChecked(entries ...Capability) (*CapabilityRegistry, error) {
	return NewCapabilityRegistry(entries...)
}

// Register adds one canonical Capability. Duplicate names are rejected before
// the definition is made visible to a Provider catalog.
func (registry *CapabilityRegistry) Register(entry Capability) error {
	if registry == nil || entry == nil {
		return errors.New("capability_required")
	}
	value := reflect.ValueOf(entry)
	if (value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface || value.Kind() == reflect.Map || value.Kind() == reflect.Func || value.Kind() == reflect.Slice) && value.IsNil() {
		return errors.New("capability_required")
	}
	if registry.capabilities == nil {
		registry.capabilities = make(map[string]Capability)
	}
	if registry.definitions == nil {
		registry.definitions = make(map[string]CapabilityDefinition)
	}
	definition := entry.Definition()
	if err := definition.Validate(); err != nil {
		return fmt.Errorf("capability_definition_invalid: %w", err)
	}
	declared := entry.RequiredContext()
	if !sameContextSlots(declared, definition.RequiredContext) {
		return fmt.Errorf("capability_definition_context_mismatch: %s", definition.Name)
	}
	if _, exists := registry.capabilities[definition.Name]; exists {
		return fmt.Errorf("capability %q already registered", definition.Name)
	}
	registry.capabilities[definition.Name] = entry
	registry.definitions[definition.Name] = definition
	return nil
}

func sameContextSlots(left, right []ContextSlot) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[ContextSlot]int, len(left))
	for _, slot := range left {
		seen[slot]++
	}
	for _, slot := range right {
		if seen[slot] == 0 {
			return false
		}
		seen[slot]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}

func (registry *CapabilityRegistry) Definitions() []CapabilityDefinition {
	if registry == nil {
		return nil
	}
	result := make([]CapabilityDefinition, 0, len(registry.definitions))
	for _, definition := range registry.definitions {
		result = append(result, definition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (registry *CapabilityRegistry) Lookup(name string) (Capability, bool) {
	if registry == nil {
		return nil, false
	}
	capability, ok := registry.capabilities[name]
	return capability, ok
}

func (registry *CapabilityRegistry) LookupCapability(name string) (Capability, bool) {
	return registry.Lookup(name)
}

func (registry *CapabilityRegistry) RequiredContext(name string) []ContextSlot {
	definition, ok := registry.Definition(name)
	if !ok {
		return nil
	}
	return append([]ContextSlot(nil), definition.RequiredContext...)
}

func (registry *CapabilityRegistry) Definition(name string) (CapabilityDefinition, bool) {
	if registry == nil {
		return CapabilityDefinition{}, false
	}
	definition, ok := registry.definitions[name]
	return definition, ok
}

func (registry *CapabilityRegistry) Catalog(surface CapabilitySurface) []CapabilityDefinition {
	definitions := make([]CapabilityDefinition, 0)
	for _, definition := range registry.Definitions() {
		if definition.SupportsSurface(surface) {
			definitions = append(definitions, definition)
		}
	}
	return definitions
}

// RenderCapabilityTools is the single provider renderer. It intentionally
// receives definitions, never executors or domain services.
func RenderCapabilityTools(definitions []CapabilityDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		if strings.TrimSpace(definition.Name) == "" {
			continue
		}
		parameters := definition.InputSchema
		if parameters == nil {
			parameters = map[string]any{"type": "object", "additionalProperties": false}
		}
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        definition.Name,
				"description": definition.Description,
				"parameters":  parameters,
			},
		})
	}
	return result
}

// CapabilityToolSchemaStats reports the provider-visible serialized size. The
// current Provider envelope does not expose tokenizer usage, so callers should
// report bytes/chars rather than inventing token counts.
func CapabilityToolSchemaStats(definitions []CapabilityDefinition) (bytes int, chars int) {
	data, err := json.Marshal(RenderCapabilityTools(definitions))
	if err != nil {
		return 0, 0
	}
	return len(data), len([]rune(string(data)))
}

func conversationReplyCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "conversation.reply", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Deliver the final user-visible text for the current conversation turn.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotCurrentLife},
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required":   []any{"text"},
			"properties": map[string]any{"text": map[string]any{"type": "string", "minLength": 1, "maxLength": 32000}},
		},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"text", "target_kind", "target_ref"},
			"properties": map[string]any{
				"text":        map[string]any{"type": "string", "minLength": 1, "maxLength": 32000},
				"target_kind": map[string]any{"type": "string", "enum": []any{"conversation_message"}},
				"target_ref":  map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			},
		},
		TargetKinds: []string{"conversation_message"}, OutputRole: "conversation_message", SideEffectClass: "external_async", SuccessBoundary: "visible_output_committed", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true,
	}
}

func momentPublishCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "moment.publish", Version: "v1", Type: CapabilityTypeAction,
		Description:   "Publish the final text of one Fluctlight Moment to the shared feed.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy},
		FailurePolicy: FailurePolicyRequiredForVisibleClaim,
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"text", "target_kind", "target_ref"},
			"properties": map[string]any{
				"text":        map[string]any{"type": "string"},
				"target_kind": map[string]any{"type": "string"},
				"target_ref":  map[string]any{"type": "string"},
			},
		},
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required":   []any{"text"},
			"properties": map[string]any{"text": map[string]any{"type": "string", "minLength": 1, "maxLength": 32000}},
		},
		TargetKinds: []string{"moment"}, OutputRole: "moment", SideEffectClass: "external_async", SuccessBoundary: "visible_output_committed", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true,
	}
}

func imageCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name:          "media.image.generate",
		Version:       "v1",
		Type:          CapabilityTypeAction,
		Description:   "Request one image generation from the configured media capability.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyRequiredForVisibleClaim,
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required":   []any{"intent"},
			"properties": map[string]any{"intent": map[string]any{"type": "string", "minLength": 1, "maxLength": 4000}},
		},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"media_intent_id", "target_kind", "target_ref"},
			"properties": map[string]any{
				"media_intent_id": map[string]any{"type": "string"},
				"target_kind":     map[string]any{"type": "string"},
				"target_ref":      map[string]any{"type": "string"},
			},
		},
		TargetKinds:           []string{"conversation_message", "moment", "wake_up"},
		OutputRole:            "media",
		SideEffectClass:       "external_async",
		SuccessBoundary:       "durable_media_intent_created",
		CompletionBoundary:    "final_media_asset_ready",
		OutcomeReferenceField: "media_intent_id",
		ConcurrencyClass:      "exclusive",
		SupportsCancel:        true,
		SupportsRetry:         true,
		RequiresPreflight:     true,
		RequiredContext:       []ContextSlot{SlotVisualIdentity, SlotCurrentLife, SlotAppearance, SlotCurrentState},
	}
}

func visualIdentityInitializeCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name:            "visual_identity.initialize",
		Version:         "v1",
		Type:            CapabilityTypeInternal,
		Description:     "Initialize the Fluctlight's durable Visual Identity workflow during a WakeUp cycle.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyOptionalInternal,
		RequiredContext: []ContextSlot{SlotCorePersona, SlotVisualIdentity},
		InputSchema:     map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"session_id", "status"},
			"properties": map[string]any{
				"session_id": map[string]any{"type": "string"},
				"status":     map[string]any{"type": "string"},
			},
		},
		SideEffectClass: "native_projection", SuccessBoundary: "durable_workflow_intent_created", CompletionBoundary: "visual_identity_ready", OutcomeReferenceField: "session_id", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true, RequiresPreflight: false,
	}
}

// NormalizeProviderToolCalls accepts both OpenAI-compatible native entries and
// the canonical JSON sidecar shape.  It intentionally rejects prose, missing
// identifiers, non-object arguments, and oversized values at one boundary.
func NormalizeProviderToolCalls(value any, sourceFactID, providerRequestID string) ([]CapabilityInvocation, error) {
	rawCalls := toolCallArrayValue(value)
	if len(rawCalls) == 0 {
		return []CapabilityInvocation{}, nil
	}
	result := make([]CapabilityInvocation, 0, len(rawCalls))
	seen := make(map[string]struct{}, len(rawCalls))
	for index, raw := range rawCalls {
		object := mapValue(raw)
		if len(object) == 0 {
			return nil, fmt.Errorf("tool call %d must be an object", index)
		}
		if kind := stringValue(object["type"]); kind != "" && kind != "function" {
			return nil, fmt.Errorf("tool call %d type is unsupported", index)
		}
		id := stringValue(object["id"])
		name := stringValue(object["name"])
		arguments := object["arguments"]
		if function := mapValue(object["function"]); len(function) > 0 {
			if name == "" {
				name = stringValue(function["name"])
			}
			if arguments == nil {
				arguments = function["arguments"]
			}
		}
		if id == "" {
			return nil, fmt.Errorf("tool call %d id is required", index)
		}
		if name == "" || len(name) > maxToolNameLength || !toolNamePattern.MatchString(name) {
			return nil, fmt.Errorf("tool call %d name is invalid", index)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("tool call id %q is duplicated", id)
		}
		seen[id] = struct{}{}
		rawArguments, err := normalizeToolArguments(arguments)
		if err != nil {
			return nil, fmt.Errorf("tool call %q arguments invalid: %w", id, err)
		}
		result = append(result, CapabilityInvocation{
			CallID: id, CapabilityName: name, SchemaVersion: CapabilityInvocationSchemaVersion,
			Arguments:         rawArguments,
			SourceFactID:      strings.TrimSpace(sourceFactID),
			ProviderRequestID: strings.TrimSpace(providerRequestID),
			Sequence:          index,
		})
	}
	return result, nil
}

func toolCallArrayValue(value any) []any {
	if object, ok := value.(map[string]any); ok {
		return []any{object}
	}
	return arrayValue(value)
}

func normalizeToolArguments(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, errors.New("arguments are required")
	}
	var data []byte
	if text, ok := value.(string); ok {
		data = []byte(strings.TrimSpace(text))
	} else {
		data = jsonBytes(value)
	}
	if len(data) == 0 || len(data) > maxToolArgumentsBytes || !json.Valid(data) {
		return nil, errors.New("arguments must be bounded valid JSON")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, errors.New("arguments must be a JSON object")
	}
	var object map[string]any
	if err := json.Unmarshal(trimmed, &object); err != nil || object == nil {
		return nil, errors.New("arguments must be a JSON object")
	}
	return json.RawMessage(append([]byte(nil), trimmed...)), nil
}

func capabilityDefinitionMap(definitions []CapabilityDefinition) map[string]CapabilityDefinition {
	result := make(map[string]CapabilityDefinition, len(definitions))
	for _, definition := range definitions {
		result[definition.Name] = definition
	}
	return result
}
