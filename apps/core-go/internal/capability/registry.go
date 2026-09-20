package capability

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

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
		if !definition.InternalOnly && definition.SupportsSurface(surface) {
			definitions = append(definitions, definition)
		}
	}
	return definitions
}

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

func CapabilityToolSchemaStats(definitions []CapabilityDefinition) (bytes int, chars int) {
	data, err := json.Marshal(RenderCapabilityTools(definitions))
	if err != nil {
		return 0, 0
	}
	return len(data), len([]rune(string(data)))
}
