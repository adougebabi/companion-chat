package core

import (
	"log/slog"
	"sort"
)

// normalizeStructuredShape repairs only fields that are absent or have the
// wrong JSON container shape for the operation schema. Existing values with
// the expected shape are returned unchanged; this is deliberately not a
// second semantic validation pass.
func normalizeStructuredShape(value map[string]any, schema map[string]any) (map[string]any, []string) {
	if value == nil {
		value = map[string]any{}
	}
	root := selectObjectSchema(schema, value)
	properties := mapValue(root["properties"])
	if len(properties) == 0 {
		return value, nil
	}
	result := value
	changed := false
	changedFields := make(map[string]struct{})
	ensureCopy := func() {
		if changed {
			return
		}
		result = make(map[string]any, len(value)+len(properties))
		for key, item := range value {
			result[key] = item
		}
		changed = true
	}
	for key, rawSchema := range properties {
		fieldSchema := mapValue(rawSchema)
		raw, exists := value[key]
		if !exists || raw == nil {
			if schemaHasRequired(root, key) {
				ensureCopy()
				result[key] = emptySchemaValue(fieldSchema)
				changedFields[key] = struct{}{}
			}
			continue
		}
		normalized, fieldChanged := normalizeSchemaValue(raw, fieldSchema)
		if !fieldChanged {
			continue
		}
		ensureCopy()
		result[key] = normalized
		changedFields[key] = struct{}{}
	}
	fields := make([]string, 0, len(changedFields))
	for field := range changedFields {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return result, fields
}

func normalizeProviderStructured(value map[string]any, schemaName string, schema map[string]any) (map[string]any, []string) {
	result, fields := normalizeStructuredShape(value, schema)
	if (schemaName == "wake_up_response" || schemaName == "daily_review_response") && stringValue(result["action_type"]) == "" {
		copy := make(map[string]any, len(result)+1)
		for key, item := range result {
			copy[key] = item
		}
		result = copy
		result["action_type"] = "no_op"
		fields = append(fields, "action_type")
		sort.Strings(fields)
	}
	return result, fields
}

func emptyProviderStructured(schemaName string, schema map[string]any) (map[string]any, []string) {
	return normalizeProviderStructured(map[string]any{}, schemaName, schema)
}

func logStructuredNormalization(role, schemaName string, fields []string, toolCallCount, candidateCount int, fallback bool) {
	if !fallback && len(fields) == 0 {
		return
	}
	slog.Default().Warn("Go Core Provider structured response normalized",
		"role", role,
		"schema", schemaName,
		"fallback", fallback,
		"fields", fields,
		"tool_call_count", toolCallCount,
		"candidate_count", candidateCount,
	)
}

func logToolCallShapeNormalization(role, schemaName, source string, value any) {
	if _, ok := value.(map[string]any); !ok {
		return
	}
	slog.Default().Warn("Go Core Provider tool_calls object normalized to array",
		"role", role,
		"schema", schemaName,
		"source", source,
		"from", "object",
		"to", "array",
	)
}

func selectObjectSchema(schema map[string]any, value map[string]any) map[string]any {
	variants := arrayValue(schema["anyOf"])
	if len(variants) == 0 {
		return schema
	}
	best := map[string]any{}
	bestMatches := -1
	for _, raw := range variants {
		candidate := mapValue(raw)
		if stringValue(candidate["type"]) != "object" {
			continue
		}
		matches := 0
		for _, required := range arrayValue(candidate["required"]) {
			if _, ok := value[stringValue(required)]; ok {
				matches++
			}
		}
		if matches > bestMatches {
			best, bestMatches = candidate, matches
		}
	}
	if bestMatches >= 0 {
		return best
	}
	return schema
}

func normalizeSchemaValue(value any, schema map[string]any) (any, bool) {
	if schema == nil {
		return value, false
	}
	if variants := arrayValue(schema["anyOf"]); len(variants) > 0 {
		for _, raw := range variants {
			candidate := mapValue(raw)
			if schemaValueMatches(value, candidate) {
				return normalizeSchemaValue(value, candidate)
			}
		}
		// If no branch matches, use the first declared branch as the typed empty
		// representation. The field is already malformed, so this changes only
		// that field and leaves every valid sibling untouched.
		return normalizeSchemaValue(value, mapValue(variants[0]))
	}

	switch stringValue(schema["type"]) {
	case "object":
		if object, ok := value.(map[string]any); ok {
			return normalizeSchemaObject(object, schema)
		}
		if isArrayContainer(value) {
			for _, item := range arrayValue(value) {
				if object, ok := item.(map[string]any); ok {
					normalized, _ := normalizeSchemaObject(object, schema)
					return normalized, true
				}
			}
		}
		return map[string]any{}, true
	case "array":
		if isArrayContainer(value) {
			items := arrayValue(value)
			itemSchema := mapValue(schema["items"])
			changed := false
			for index, item := range items {
				normalized, itemChanged := normalizeSchemaValue(item, itemSchema)
				if itemChanged {
					if !changed {
						items = append([]any(nil), items...)
						changed = true
					}
					items[index] = normalized
				}
			}
			if changed {
				return items, true
			}
			return value, false
		}
		if object, ok := value.(map[string]any); ok {
			normalized, _ := normalizeSchemaValue(object, mapValue(schema["items"]))
			return []any{normalized}, true
		}
		return []any{}, true
	case "string":
		if _, ok := value.(string); ok {
			return value, false
		}
		return "", true
	case "number", "integer":
		if isJSONNumber(value) {
			return value, false
		}
		return float64(0), true
	case "boolean":
		if _, ok := value.(bool); ok {
			return value, false
		}
		return false, true
	default:
		return value, false
	}
}

func normalizeSchemaObject(value map[string]any, schema map[string]any) (map[string]any, bool) {
	properties := mapValue(schema["properties"])
	if len(properties) == 0 {
		return value, false
	}
	result := value
	changed := false
	ensureCopy := func() {
		if changed {
			return
		}
		result = make(map[string]any, len(value)+len(properties))
		for key, item := range value {
			result[key] = item
		}
		changed = true
	}
	for key, rawSchema := range properties {
		fieldSchema := mapValue(rawSchema)
		raw, exists := value[key]
		if !exists || raw == nil {
			if schemaHasRequired(schema, key) {
				ensureCopy()
				result[key] = emptySchemaValue(fieldSchema)
			}
			continue
		}
		normalized, fieldChanged := normalizeSchemaValue(raw, fieldSchema)
		if fieldChanged {
			ensureCopy()
			result[key] = normalized
		}
	}
	return result, changed
}

func schemaHasRequired(schema map[string]any, key string) bool {
	for _, raw := range arrayValue(schema["required"]) {
		if stringValue(raw) == key {
			return true
		}
	}
	return false
}

func emptySchemaValue(schema map[string]any) any {
	if variants := arrayValue(schema["anyOf"]); len(variants) > 0 {
		return emptySchemaValue(mapValue(variants[0]))
	}
	switch stringValue(schema["type"]) {
	case "object":
		result := map[string]any{}
		properties := mapValue(schema["properties"])
		for _, raw := range arrayValue(schema["required"]) {
			key := stringValue(raw)
			if key != "" {
				result[key] = emptySchemaValue(mapValue(properties[key]))
			}
		}
		return result
	case "array":
		return []any{}
	case "string":
		return ""
	case "number", "integer":
		return float64(0)
	case "boolean":
		return false
	default:
		return map[string]any{}
	}
}

func schemaValueMatches(value any, schema map[string]any) bool {
	switch stringValue(schema["type"]) {
	case "object":
		if _, ok := value.(map[string]any); ok {
			return true
		}
		if isArrayContainer(value) {
			for _, item := range arrayValue(value) {
				if _, ok := item.(map[string]any); ok {
					return true
				}
			}
		}
		return false
	case "array":
		return isArrayContainer(value)
	case "string":
		_, ok := value.(string)
		return ok
	case "number", "integer":
		return isJSONNumber(value)
	case "boolean":
		_, ok := value.(bool)
		return ok
	default:
		return false
	}
}

func isArrayContainer(value any) bool {
	switch value.(type) {
	case []any, []map[string]any, []string:
		return true
	default:
		return false
	}
}

func isJSONNumber(value any) bool {
	switch value.(type) {
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}
