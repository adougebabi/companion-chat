package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	ToolCallSchemaVersion   = "fluctlight.tool-call.v1"
	ToolResultSchemaVersion = "fluctlight.tool-result.v1"
	maxToolNameLength       = 128
	maxToolArgumentsBytes   = 64 << 10
)

var toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// CapabilityManifest is the Runtime-facing declaration for an external
// capability.  The manifest describes the slot and its safety properties; it
// does not grant the provider direct access to domain tables.
type CapabilityManifest struct {
	Name              string
	Version           string
	Description       string
	Parameters        map[string]any
	SideEffectClass   string
	ConcurrencyClass  string
	SupportsCancel    bool
	SupportsRetry     bool
	RequiresPreflight bool
}

// ToolCallV1 is the canonical representation shared by native provider tool
// calls and structured JSON sidecars.  Arguments remain raw JSON so the
// capability owner can validate them against its own schema without a lossy
// map conversion.
type ToolCallV1 struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Arguments         json.RawMessage `json:"arguments"`
	SourceFactID      string          `json:"source_fact_id"`
	ActionID          string          `json:"action_id,omitempty"`
	ProviderRequestID string          `json:"provider_request_id"`
	SchemaVersion     string          `json:"schema_version"`
	Sequence          int             `json:"sequence"`
}

// ToolResultV1 is persisted and optionally fed back into a subsequent model
// step.  Output is intentionally untyped at this common boundary; each
// capability validates its own result before constructing it.
type ToolResultV1 struct {
	ToolCallID        string `json:"tool_call_id"`
	Name              string `json:"name"`
	Status            string `json:"status"`
	Output            any    `json:"output,omitempty"`
	ErrorCode         string `json:"error_code,omitempty"`
	Retryable         bool   `json:"retryable"`
	ProviderRequestID string `json:"provider_request_id,omitempty"`
	CorrelationID     string `json:"correlation_id,omitempty"`
	SchemaVersion     string `json:"schema_version"`
}

// ProviderCompletion is the normalized provider result used by the
// application boundary. Visible text, structured sidecar data, and native
// tool calls share one result instead of being inferred from prose later.
type ProviderCompletion struct {
	Text       string
	Structured map[string]any
	ToolCalls  []ToolCallV1
	DoneSeen   bool
}

// CapabilityExecutor is the narrow plugin seam.  The Runtime owns the
// invocation context and persistence; an executor only implements one
// replaceable external capability.
type CapabilityExecutor interface {
	Manifest() CapabilityManifest
	Execute(context.Context, string, string, string, ToolCallV1) (ToolResultV1, error)
}

type CapabilityRegistry struct {
	executors map[string]CapabilityExecutor
}

func NewCapabilityRegistry(executors ...CapabilityExecutor) *CapabilityRegistry {
	registry := &CapabilityRegistry{executors: make(map[string]CapabilityExecutor, len(executors))}
	for _, executor := range executors {
		if executor == nil {
			continue
		}
		manifest := executor.Manifest()
		if manifest.Name == "" || !toolNamePattern.MatchString(manifest.Name) {
			continue
		}
		registry.executors[manifest.Name] = executor
	}
	return registry
}

func (registry *CapabilityRegistry) Manifests() []CapabilityManifest {
	if registry == nil {
		return nil
	}
	result := make([]CapabilityManifest, 0, len(registry.executors))
	for _, executor := range registry.executors {
		result = append(result, executor.Manifest())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (registry *CapabilityRegistry) Lookup(name string) (CapabilityExecutor, bool) {
	if registry == nil {
		return nil, false
	}
	executor, ok := registry.executors[name]
	return executor, ok
}

// ToolCallPayload converts a manifest into the OpenAI-compatible tools shape.
// The function is kept at the provider boundary so the browser never receives
// provider-specific schemas or hidden capability metadata.
func ToolCallPayload(manifests []CapabilityManifest) []map[string]any {
	result := make([]map[string]any, 0, len(manifests))
	for _, manifest := range manifests {
		if strings.TrimSpace(manifest.Name) == "" {
			continue
		}
		parameters := manifest.Parameters
		if parameters == nil {
			parameters = map[string]any{"type": "object", "additionalProperties": false}
		}
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        manifest.Name,
				"description": manifest.Description,
				"parameters":  parameters,
			},
		})
	}
	return result
}

// ExternalCapabilityManifests is the first deliberately small catalog.  New
// providers can implement the same slot without changing cognition semantics.
// Video/audio/search slots are added only when their executable adapters exist;
// advertising an unavailable capability would make the model contract lie.
func ExternalCapabilityManifests() []CapabilityManifest {
	return []CapabilityManifest{imageCapabilityManifest()}
}

func imageCapabilityManifest() CapabilityManifest {
	return CapabilityManifest{
		Name:              "media.image.generate",
		Version:           "v1",
		Description:       "Request one image generation from the configured media capability.",
		Parameters:        imageCapabilityParameters(),
		SideEffectClass:   "external_async",
		ConcurrencyClass:  "exclusive",
		SupportsCancel:    true,
		SupportsRetry:     true,
		RequiresPreflight: true,
	}
}

func imageCapabilityParameters() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"concept"},
		"properties": map[string]any{
			"concept": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"description":          "The already-frozen visual concept owned by the Fluctlight decision.",
			},
		},
	}
}

// NormalizeProviderToolCalls accepts both OpenAI-compatible native entries and
// the canonical JSON sidecar shape.  It intentionally rejects prose, missing
// identifiers, non-object arguments, and oversized values at one boundary.
func NormalizeProviderToolCalls(value any, sourceFactID, providerRequestID string) ([]ToolCallV1, error) {
	rawCalls := arrayValue(value)
	if len(rawCalls) == 0 {
		return []ToolCallV1{}, nil
	}
	result := make([]ToolCallV1, 0, len(rawCalls))
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
		result = append(result, ToolCallV1{
			ID:                id,
			Name:              name,
			Arguments:         rawArguments,
			SourceFactID:      strings.TrimSpace(sourceFactID),
			ProviderRequestID: strings.TrimSpace(providerRequestID),
			SchemaVersion:     ToolCallSchemaVersion,
			Sequence:          index,
		})
	}
	return result, nil
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

func (call ToolCallV1) Validate(manifests map[string]CapabilityManifest) error {
	if call.SchemaVersion != ToolCallSchemaVersion {
		return errors.New("tool call schema version invalid")
	}
	if call.ID == "" || call.Name == "" || !toolNamePattern.MatchString(call.Name) {
		return errors.New("tool call identity invalid")
	}
	if strings.TrimSpace(call.SourceFactID) == "" {
		return errors.New("tool call source fact is required")
	}
	if strings.TrimSpace(call.ProviderRequestID) == "" {
		return errors.New("tool call provider request is required")
	}
	if call.Sequence < 0 {
		return errors.New("tool call sequence invalid")
	}
	if _, err := normalizeToolArguments(string(call.Arguments)); err != nil {
		return err
	}
	manifest, ok := manifests[call.Name]
	if !ok || manifest.Name == "" {
		return fmt.Errorf("capability %q is unavailable", call.Name)
	}
	return nil
}

func (result ToolResultV1) Validate(call ToolCallV1) error {
	if result.SchemaVersion != ToolResultSchemaVersion {
		return errors.New("tool result schema version invalid")
	}
	if result.ToolCallID == "" || result.ToolCallID != call.ID || result.Name != call.Name {
		return errors.New("tool result identity invalid")
	}
	switch result.Status {
	case "completed", "failed", "rejected", "deferred":
	default:
		return errors.New("tool result status invalid")
	}
	if data, err := json.Marshal(result.Output); err != nil || len(data) > maxToolArgumentsBytes*4 {
		return errors.New("tool result output is too large")
	}
	return nil
}

func toolManifestMap(manifests []CapabilityManifest) map[string]CapabilityManifest {
	result := make(map[string]CapabilityManifest, len(manifests))
	for _, manifest := range manifests {
		result[manifest.Name] = manifest
	}
	return result
}
