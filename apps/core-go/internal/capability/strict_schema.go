package capability

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// strictSchemaDroppedKeywords are rejected by OpenAI-compatible strict
// json_schema. Server-side ValidateCapabilitySchemaValue still enforces the
// original schema after nulls are treated as absent.
var strictSchemaDroppedKeywords = []string{
	"minLength", "maxLength", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum",
	"minItems", "maxItems", "minProperties", "maxProperties", "multipleOf",
	"pattern", "format", "default", "uniqueItems", "patternProperties", "unevaluatedProperties",
}

var capabilityWireToCanonical sync.Map

// CapabilityWireName is the provider tool name. Dots are rejected by the
// upstream tool-name pattern, so they become underscores only on the wire.
func CapabilityWireName(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), ".", "_")
}

// RememberCapabilityWireNames records the reverse map for names that were
// actually offered to a provider. Canonical names that contain no dot stay
// unchanged when a model echoes them.
func RememberCapabilityWireNames(definitions []CapabilityDefinition) {
	for _, definition := range definitions {
		name := strings.TrimSpace(definition.Name)
		if name == "" {
			continue
		}
		capabilityWireToCanonical.Store(CapabilityWireName(name), name)
	}
}

// CanonicalCapabilityName maps a provider tool name back to the registered
// capability. Names that were not rendered as wire aliases are returned as-is,
// so existing dotted calls still resolve.
func CanonicalCapabilityName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if value, ok := capabilityWireToCanonical.Load(name); ok {
		if canonical, ok := value.(string); ok && canonical != "" {
			return canonical
		}
	}
	return name
}

// CapabilityNamesMatch reports whether a provider or canonical name refers to
// the same capability.
func CapabilityNamesMatch(actual, canonical string) bool {
	actual = strings.TrimSpace(actual)
	canonical = strings.TrimSpace(canonical)
	return actual == canonical || actual == CapabilityWireName(canonical) || CanonicalCapabilityName(actual) == canonical
}

// StrictModelSchema returns a deep copy that satisfies strict json_schema:
// every object lists all property keys in required, additionalProperties is
// false, optional fields are nullable, and unsupported constraint keywords are
// removed. The caller's schema is not mutated.
func StrictModelSchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}, "required": []any{}}
	}
	return strictifySchema(cloneSchema(schema))
}

func strictifySchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return schema
	}
	if union, ok := strictifyUnion(schema); ok {
		return union
	}
	// Variant constraints stay on the original schema for server validation.
	// The wire object is the closed property set so strict mode does not reject
	// a union of overlapping objects.
	delete(schema, "anyOf")
	delete(schema, "oneOf")
	delete(schema, "allOf")
	delete(schema, "not")
	if items, ok := schema["items"].(map[string]any); ok {
		schema["items"] = strictifySchema(items)
	}
	if prefix, ok := schema["prefixItems"].([]any); ok {
		rewritten := make([]any, len(prefix))
		for index, item := range prefix {
			rewritten[index] = strictifySchema(mapValue(item))
		}
		schema["prefixItems"] = rewritten
	}
	for _, key := range strictSchemaDroppedKeywords {
		delete(schema, key)
	}
	if schemaIsFreeformObject(schema) {
		return strictFreeformObjectSchema()
	}
	properties := mapValue(schema["properties"])
	if stringValue(schema["type"]) == "object" || len(properties) > 0 || schema["additionalProperties"] != nil {
		return strictifyObject(schema, properties)
	}
	if _, ok := schema["type"].([]any); ok {
		schema["type"] = strictTypeList(schema["type"])
	}
	return schema
}

func strictifyUnion(schema map[string]any) (map[string]any, bool) {
	variants := arrayValue(schema["anyOf"])
	keyword := "anyOf"
	if len(variants) == 0 {
		variants = arrayValue(schema["oneOf"])
		keyword = "oneOf"
	}
	if len(variants) == 0 {
		return nil, false
	}
	if len(mapValue(schema["properties"])) > 0 || stringValue(schema["type"]) == "object" {
		return nil, false
	}
	filtered := make([]any, 0, len(variants))
	for _, raw := range variants {
		branch := mapValue(raw)
		if schemaIsFreeformObject(branch) && unionHasConcreteBranch(variants) {
			continue
		}
		filtered = append(filtered, strictifySchema(cloneSchema(branch)))
	}
	if len(filtered) == 0 {
		return strictFreeformObjectSchema(), true
	}
	if collapsed, ok := collapseNullableUnion(filtered); ok {
		return collapsed, true
	}
	if len(filtered) == 1 {
		if only, ok := filtered[0].(map[string]any); ok {
			return only, true
		}
	}
	return map[string]any{keyword: filtered}, true
}

func unionHasConcreteBranch(variants []any) bool {
	for _, raw := range variants {
		if !schemaIsFreeformObject(mapValue(raw)) {
			return true
		}
	}
	return false
}

func collapseNullableUnion(variants []any) (map[string]any, bool) {
	nonNull := make([]map[string]any, 0, len(variants))
	sawNull := false
	for _, raw := range variants {
		branch := mapValue(raw)
		if stringValue(branch["type"]) == "null" && len(branch) == 1 {
			sawNull = true
			continue
		}
		nonNull = append(nonNull, branch)
	}
	if !sawNull || len(nonNull) != 1 {
		return nil, false
	}
	return nullableSchema(nonNull[0]), true
}

func strictifyObject(schema map[string]any, properties map[string]any) map[string]any {
	requiredSet := map[string]bool{}
	for _, raw := range arrayValue(schema["required"]) {
		if key := stringValue(raw); key != "" {
			requiredSet[key] = true
		}
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rewritten := make(map[string]any, len(keys))
	required := make([]any, 0, len(keys))
	for _, key := range keys {
		child := strictifySchema(mapValue(properties[key]))
		if !requiredSet[key] {
			child = nullableSchema(child)
		}
		rewritten[key] = child
		required = append(required, key)
	}
	schema["type"] = "object"
	schema["additionalProperties"] = false
	schema["properties"] = rewritten
	schema["required"] = required
	return schema
}

func nullableSchema(schema map[string]any) map[string]any {
	if schemaAllowsNull(schema) {
		return schema
	}
	switch typed := schema["type"].(type) {
	case string:
		if typed == "object" || typed == "array" || len(mapValue(schema["properties"])) > 0 || schema["items"] != nil || len(arrayValue(schema["anyOf"])) > 0 {
			return map[string]any{"anyOf": []any{schema, map[string]any{"type": "null"}}}
		}
		schema["type"] = []any{typed, "null"}
		if enum := arrayValue(schema["enum"]); len(enum) > 0 {
			schema["enum"] = append(append([]any{}, enum...), nil)
		}
		return schema
	case []any:
		if !typeListHasNull(typed) {
			schema["type"] = append(append([]any{}, typed...), "null")
		}
		if enum := arrayValue(schema["enum"]); len(enum) > 0 && !enumHasNull(enum) {
			schema["enum"] = append(append([]any{}, enum...), nil)
		}
		return schema
	default:
		return map[string]any{"anyOf": []any{schema, map[string]any{"type": "null"}}}
	}
}

func typeListHasNull(value any) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if stringValue(item) == "null" {
			return true
		}
	}
	return false
}

func enumHasNull(values []any) bool {
	for _, value := range values {
		if value == nil {
			return true
		}
	}
	return false
}

func strictTypeList(value any) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	return append([]any{}, items...)
}

func schemaIsFreeformObject(schema map[string]any) bool {
	if len(schema) == 0 {
		return false
	}
	if len(arrayValue(schema["anyOf"])) > 0 || len(arrayValue(schema["oneOf"])) > 0 {
		return false
	}
	typeName, _ := schema["type"].(string)
	if typeName != "" && typeName != "object" {
		return false
	}
	if len(mapValue(schema["properties"])) > 0 {
		return false
	}
	additional, present := schema["additionalProperties"]
	if !present {
		return typeName == "object"
	}
	flag, ok := additional.(bool)
	return ok && flag
}

func strictFreeformObjectSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "JSON object encoded as text because strict mode cannot accept an open object.",
		"properties": map[string]any{
			"json": map[string]any{"type": "string", "description": "JSON object text."},
		},
		"required": []any{"json"},
	}
}

func cloneSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return map[string]any{}
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if err := json.Unmarshal(encoded, &cloned); err != nil || cloned == nil {
		return map[string]any{}
	}
	return cloned
}
