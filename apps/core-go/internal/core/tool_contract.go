package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	CapabilityInvocationSchemaVersion = "fluctlight.capability-invocation.v2"
	conversationReplyCapabilityName   = "conversation.reply"
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
		Name: conversationReplyCapabilityName, Version: "v1", Type: CapabilityTypeAction,
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
		TargetKinds:           []string{"conversation", "conversation_message", "moment", "wake_up"},
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
// the canonical JSON sidecar shape. Native entries use id/name (or the nested
// function object); canonical entries use call_id/capability_name. It
// intentionally rejects prose, missing identifiers, non-object arguments, and
// oversized values at one boundary.
func NormalizeProviderToolCalls(value any, sourceFactID, providerRequestID string) ([]CapabilityInvocation, error) {
	return normalizeProviderToolCalls(value, sourceFactID, providerRequestID, false)
}

// normalizeProviderToolCallsWithDerivedIDs accepts the subset of
// OpenAI-compatible Providers that omit native/sidecar call IDs. The derived
// ID is deterministic for one Provider request and invocation position, so a
// retry can reuse the same idempotency identity without trusting model text as
// an identifier. The strict public helper above remains available for payload
// validation tests and any caller that must reject missing identity outright.
func normalizeProviderToolCallsWithDerivedIDs(value any, sourceFactID, providerRequestID string) ([]CapabilityInvocation, error) {
	return normalizeProviderToolCalls(value, sourceFactID, providerRequestID, true)
}

// normalizeProviderToolCallsIndependently keeps valid native calls when one
// sibling entry is malformed. The strict public normalizer remains fail-closed
// for callers that validate a single envelope; the Provider adapter uses this
// event-channel variant so one bad model entry cannot erase unrelated valid
// calls from the same response.
func normalizeProviderToolCallsIndependently(value any, sourceFactID, providerRequestID string) ([]CapabilityInvocation, error) {
	rawCalls := toolCallArrayValue(value)
	if len(rawCalls) == 0 {
		return []CapabilityInvocation{}, nil
	}
	result := make([]CapabilityInvocation, 0, len(rawCalls))
	var firstErr error
	for index, raw := range rawCalls {
		object := mapValue(raw)
		if len(object) == 0 {
			if firstErr == nil {
				firstErr = newProviderToolCallNormalizationError(index, "item_not_object", fmt.Errorf("tool call %d must be an object", index))
			}
			continue
		}
		// Derive a stable ID using the original array position before handing the
		// single item to the strict validator (which otherwise sees position 0).
		if strings.TrimSpace(stringValue(object["id"])) == "" && strings.TrimSpace(stringValue(object["call_id"])) == "" {
			name := stringValue(object["name"])
			if name == "" {
				name = stringValue(object["capability_name"])
			}
			if function := mapValue(object["function"]); name == "" && len(function) > 0 {
				name = stringValue(function["name"])
			}
			arguments := object["arguments"]
			if arguments == nil {
				arguments = mapValue(object["function"])["arguments"]
			}
			if strings.TrimSpace(providerRequestID) != "" {
				if normalized, err := normalizeToolArguments(arguments); err == nil {
					copyObject := make(map[string]any, len(object)+1)
					for key, value := range object {
						copyObject[key] = value
					}
					copyObject["id"] = derivedProviderToolCallID(providerRequestID, index, name, normalized)
					object = copyObject
				}
			}
		}
		calls, err := normalizeProviderToolCalls([]any{object}, sourceFactID, providerRequestID, false)
		if err != nil {
			if firstErr == nil {
				firstErr = newProviderToolCallNormalizationError(index, "item_invalid", err)
			}
			continue
		}
		for callIndex := range calls {
			calls[callIndex].Sequence = index
		}
		result = append(result, calls...)
	}
	return result, firstErr
}

func normalizeProviderToolCalls(value any, sourceFactID, providerRequestID string, deriveMissingIDs bool) ([]CapabilityInvocation, error) {
	rawCalls := toolCallArrayValue(value)
	if len(rawCalls) == 0 {
		return []CapabilityInvocation{}, nil
	}
	result := make([]CapabilityInvocation, 0, len(rawCalls))
	seen := make(map[string]struct{}, len(rawCalls))
	for index, raw := range rawCalls {
		object := mapValue(raw)
		if len(object) == 0 {
			return nil, newProviderToolCallNormalizationError(index, "item_not_object", fmt.Errorf("tool call %d must be an object", index))
		}
		if kind := stringValue(object["type"]); kind != "" && kind != "function" {
			return nil, newProviderToolCallNormalizationError(index, "unsupported_type", fmt.Errorf("tool call %d type is unsupported", index))
		}
		id := stringValue(object["id"])
		canonicalID := stringValue(object["call_id"])
		if id != "" && canonicalID != "" && id != canonicalID {
			return nil, newProviderToolCallNormalizationError(index, "id_conflict", fmt.Errorf("tool call %d id fields disagree", index))
		}
		if id == "" {
			id = canonicalID
		}
		name := stringValue(object["name"])
		canonicalName := stringValue(object["capability_name"])
		if name != "" && canonicalName != "" && name != canonicalName {
			return nil, newProviderToolCallNormalizationError(index, "name_conflict", fmt.Errorf("tool call %d name fields disagree", index))
		}
		if name == "" {
			name = canonicalName
		}
		arguments := object["arguments"]
		if function := mapValue(object["function"]); len(function) > 0 {
			if name == "" {
				name = stringValue(function["name"])
			}
			if arguments == nil {
				arguments = function["arguments"]
			}
		}
		if name == "" || len(name) > maxToolNameLength || !toolNamePattern.MatchString(name) {
			return nil, newProviderToolCallNormalizationError(index, "name_invalid", fmt.Errorf("tool call %d name is invalid", index))
		}
		var rawArguments []byte
		if id == "" && deriveMissingIDs && strings.TrimSpace(providerRequestID) != "" {
			var argumentsErr error
			rawArguments, argumentsErr = normalizeToolArguments(arguments)
			if argumentsErr != nil {
				reason := "arguments_invalid"
				var typedErr *providerToolArgumentsNormalizationError
				if errors.As(argumentsErr, &typedErr) {
					reason = typedErr.Reason
				}
				return nil, newProviderToolCallNormalizationError(index, reason, fmt.Errorf("tool call at index %d arguments invalid: %w", index, argumentsErr))
			}
			id = derivedProviderToolCallID(providerRequestID, index, name, rawArguments)
		}
		if id == "" {
			return nil, newProviderToolCallNormalizationError(index, "id_required", fmt.Errorf("tool call %d id is required", index))
		}
		if _, exists := seen[id]; exists {
			return nil, newProviderToolCallNormalizationError(index, "duplicate_id", fmt.Errorf("tool call id %q is duplicated", id))
		}
		seen[id] = struct{}{}
		if rawArguments == nil {
			var err error
			rawArguments, err = normalizeToolArguments(arguments)
			if err != nil {
				reason := "arguments_invalid"
				var argumentsErr *providerToolArgumentsNormalizationError
				if errors.As(err, &argumentsErr) {
					reason = argumentsErr.Reason
				}
				return nil, newProviderToolCallNormalizationError(index, reason, fmt.Errorf("tool call %q arguments invalid: %w", id, err))
			}
		}
		result = append(result, CapabilityInvocation{
			CallID: id, CapabilityName: name, SchemaVersion: CapabilityInvocationSchemaVersion,
			Arguments:         rawArguments,
			SourceFactID:      strings.TrimSpace(sourceFactID),
			ProviderRequestID: strings.TrimSpace(providerRequestID),
			Sequence:          index,
			Metadata:          InvocationMetadata{Source: "model_tool"},
		})
	}
	return result, nil
}

func derivedProviderToolCallID(providerRequestID string, index int, name string, arguments []byte) string {
	return "call_derived_" + stableDigest(strings.Join([]string{
		strings.TrimSpace(providerRequestID), strconv.Itoa(index), strings.TrimSpace(name), string(arguments),
	}, "\x1f"))
}

// providerToolCallNormalizationError preserves the stable reason and item
// index needed by bounded Provider diagnostics while retaining the existing
// human-readable error for callers. The diagnostic projection deliberately
// excludes its model-controlled cause details.
type providerToolCallNormalizationError struct {
	Index  int
	Reason string
	Cause  error
}

func newProviderToolCallNormalizationError(index int, reason string, cause error) error {
	return &providerToolCallNormalizationError{Index: index, Reason: reason, Cause: cause}
}

func (e *providerToolCallNormalizationError) Error() string {
	if e == nil || e.Cause == nil {
		return "provider tool call normalization failed"
	}
	return e.Cause.Error()
}

func (e *providerToolCallNormalizationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type providerToolArgumentsNormalizationError struct {
	Reason string
}

func (e *providerToolArgumentsNormalizationError) Error() string {
	if e == nil {
		return "arguments must be bounded valid JSON"
	}
	switch e.Reason {
	case "arguments_required":
		return "arguments are required"
	case "arguments_not_object":
		return "arguments must be a JSON object"
	default:
		return "arguments must be bounded valid JSON"
	}
}

func providerToolArgumentsError(reason string) error {
	return &providerToolArgumentsNormalizationError{Reason: reason}
}

func toolCallArrayValue(value any) []any {
	if object, ok := value.(map[string]any); ok {
		return []any{object}
	}
	return arrayValue(value)
}

func normalizeToolArguments(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, providerToolArgumentsError("arguments_required")
	}
	var data []byte
	if text, ok := value.(string); ok {
		data = []byte(strings.TrimSpace(text))
	} else {
		data = jsonBytes(value)
	}
	if len(data) == 0 {
		return nil, providerToolArgumentsError("arguments_empty")
	}
	if len(data) > maxToolArgumentsBytes {
		return nil, providerToolArgumentsError("arguments_oversized")
	}
	if !json.Valid(data) {
		return nil, providerToolArgumentsError("arguments_invalid_json")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, providerToolArgumentsError("arguments_not_object")
	}
	var object map[string]any
	if err := json.Unmarshal(trimmed, &object); err != nil || object == nil {
		return nil, providerToolArgumentsError("arguments_not_object")
	}
	return json.RawMessage(append([]byte(nil), trimmed...)), nil
}

// providerToolCallNormalizationDiagnostic exposes only bounded shape metadata
// for a failed normalization. It is safe to persist in a diagnostic model run:
// no ID/name values, argument values, user text, or provider response bodies are
// copied into the result.
func providerToolCallNormalizationDiagnostic(value any, source string, err error) map[string]any {
	source = strings.TrimSpace(source)
	if source != "native" && source != "structured" {
		source = "unknown"
	}
	rawCalls := toolCallArrayValue(value)
	result := map[string]any{
		"source":               source,
		"value_shape":          providerToolCallValueShape(value),
		"call_count":           len(rawCalls),
		"failed_item_index":    -1,
		"normalization_reason": "normalization_failed",
	}
	var normalizationErr *providerToolCallNormalizationError
	if !errors.As(err, &normalizationErr) || normalizationErr == nil {
		return result
	}
	result["failed_item_index"] = normalizationErr.Index
	if normalizationErr.Reason != "" {
		result["normalization_reason"] = normalizationErr.Reason
	}
	if normalizationErr.Index < 0 || normalizationErr.Index >= len(rawCalls) {
		return result
	}
	return addProviderToolCallItemDiagnostic(result, rawCalls[normalizationErr.Index])
}

func addProviderToolCallItemDiagnostic(result map[string]any, raw any) map[string]any {
	if result == nil {
		result = map[string]any{}
	}
	object, objectOK := raw.(map[string]any)
	if !objectOK {
		result["item_shape"] = providerToolCallValueShape(raw)
		result["id_present"] = false
		result["id_type"] = providerToolCallValueShape(nil)
		result["type_value"] = ""
		result["name_present"] = false
		result["name_length"] = 0
		result["name_valid"] = false
		result["arguments_present"] = false
		result["arguments_shape"] = providerToolCallValueShape(nil)
		result["arguments_length"] = 0
		return result
	}
	result["item_shape"] = "object"
	idValue, idPresent := object["id"]
	if (!idPresent || idValue == nil) && object["call_id"] != nil {
		idValue, idPresent = object["call_id"], true
	}
	result["id_present"] = idPresent && idValue != nil
	result["id_type"] = providerToolCallValueShape(idValue)
	if typeValue, present := object["type"]; present {
		result["type_value"] = boundedProviderToolCallDiagnosticToken(typeValue)
	} else {
		result["type_value"] = ""
	}

	nameValue, namePresent := object["name"]
	if (!namePresent || nameValue == nil) && object["capability_name"] != nil {
		nameValue, namePresent = object["capability_name"], true
	}
	name := stringValue(nameValue)
	namePresentEffective := namePresent && nameValue != nil
	functionValue, functionPresent := object["function"]
	functionObject, functionOK := functionValue.(map[string]any)
	if functionPresent {
		result["function_shape"] = providerToolCallValueShape(functionValue)
	}
	if name == "" && functionOK {
		if functionName, present := functionObject["name"]; present {
			nameValue = functionName
			name = stringValue(functionName)
			namePresentEffective = present && functionName != nil
		}
	}
	result["name_present"] = namePresentEffective
	result["name_length"] = len([]byte(name))
	result["name_valid"] = name != "" && len([]byte(name)) <= maxToolNameLength && toolNamePattern.MatchString(name)

	argumentsValue, argumentsPresent := object["arguments"]
	if argumentsValue == nil && functionOK {
		argumentsValue, argumentsPresent = functionObject["arguments"]
	}
	result["arguments_present"] = argumentsPresent && argumentsValue != nil
	result["arguments_shape"] = providerToolCallValueShape(argumentsValue)
	result["arguments_length"] = providerToolCallValueLength(argumentsValue)
	return result
}

func providerToolCallValueShape(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return "number"
	case map[string]any:
		return "object"
	case []any, []map[string]any, []string:
		return "array"
	default:
		return "unsupported"
	}
}

func providerToolCallValueLength(value any) int {
	switch typed := value.(type) {
	case string:
		return len([]byte(strings.TrimSpace(typed)))
	case nil:
		return 0
	default:
		return len(jsonBytes(value))
	}
}

func boundedProviderToolCallDiagnosticToken(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if len([]rune(text)) > 64 {
		return "bounded"
	}
	for _, character := range text {
		if (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' && character != '.' && character != '-' {
			return "invalid"
		}
	}
	return text
}

func capabilityDefinitionMap(definitions []CapabilityDefinition) map[string]CapabilityDefinition {
	result := make(map[string]CapabilityDefinition, len(definitions))
	for _, definition := range definitions {
		result[definition.Name] = definition
	}
	return result
}
