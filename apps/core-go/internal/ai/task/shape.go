package task

import (
	"sort"
)

// NormalizeStructuredShape repairs only fields that are absent or have the
// wrong JSON container shape for the operation schema. Existing values with
// the expected shape are returned unchanged.
func NormalizeStructuredShape(value map[string]any, schema map[string]any) (map[string]any, []string) {
	if value == nil {
		value = map[string]any{}
	}
	root := SelectObjectSchema(schema, value)
	properties := MapValue(root["properties"])
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
		fieldSchema := MapValue(rawSchema)
		raw, exists := value[key]
		if !exists || raw == nil {
			if SchemaHasRequired(root, key) {
				ensureCopy()
				result[key] = EmptySchemaValue(fieldSchema)
				changedFields[key] = struct{}{}
			}
			continue
		}
		normalized, fieldChanged := NormalizeSchemaValue(raw, fieldSchema)
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

func SelectObjectSchema(schema map[string]any, value map[string]any) map[string]any {
	variants := ArrayValue(schema["anyOf"])
	if len(variants) == 0 {
		return schema
	}
	best := map[string]any{}
	bestMatches := -1
	for _, raw := range variants {
		candidate := MapValue(raw)
		if StringValue(candidate["type"]) != "object" {
			continue
		}
		matches := 0
		for _, required := range ArrayValue(candidate["required"]) {
			if _, ok := value[StringValue(required)]; ok {
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

func NormalizeSchemaValue(value any, schema map[string]any) (any, bool) {
	if schema == nil {
		return value, false
	}
	if variants := ArrayValue(schema["anyOf"]); len(variants) > 0 {
		for _, raw := range variants {
			candidate := MapValue(raw)
			if SchemaValueMatches(value, candidate) {
				return NormalizeSchemaValue(value, candidate)
			}
		}
		return NormalizeSchemaValue(value, MapValue(variants[0]))
	}

	switch StringValue(schema["type"]) {
	case "object":
		if object, ok := value.(map[string]any); ok {
			return NormalizeSchemaObject(object, schema)
		}
		if IsArrayContainer(value) {
			for _, item := range ArrayValue(value) {
				if object, ok := item.(map[string]any); ok {
					normalized, _ := NormalizeSchemaObject(object, schema)
					return normalized, true
				}
			}
		}
		return map[string]any{}, true
	case "array":
		if IsArrayContainer(value) {
			items := ArrayValue(value)
			itemSchema := MapValue(schema["items"])
			changed := false
			for index, item := range items {
				normalized, itemChanged := NormalizeSchemaValue(item, itemSchema)
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
			normalized, _ := NormalizeSchemaValue(object, MapValue(schema["items"]))
			return []any{normalized}, true
		}
		return []any{}, true
	case "string":
		if _, ok := value.(string); ok {
			return value, false
		}
		return "", true
	case "number", "integer":
		if IsJSONNumber(value) {
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

func NormalizeSchemaObject(value map[string]any, schema map[string]any) (map[string]any, bool) {
	properties := MapValue(schema["properties"])
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
		fieldSchema := MapValue(rawSchema)
		raw, exists := value[key]
		if !exists || raw == nil {
			if SchemaHasRequired(schema, key) {
				ensureCopy()
				result[key] = EmptySchemaValue(fieldSchema)
			}
			continue
		}
		normalized, fieldChanged := NormalizeSchemaValue(raw, fieldSchema)
		if fieldChanged {
			ensureCopy()
			result[key] = normalized
		}
	}
	return result, changed
}

func SchemaHasRequired(schema map[string]any, key string) bool {
	for _, raw := range ArrayValue(schema["required"]) {
		if StringValue(raw) == key {
			return true
		}
	}
	return false
}

func EmptySchemaValue(schema map[string]any) any {
	if variants := ArrayValue(schema["anyOf"]); len(variants) > 0 {
		return EmptySchemaValue(MapValue(variants[0]))
	}
	switch StringValue(schema["type"]) {
	case "object":
		result := map[string]any{}
		properties := MapValue(schema["properties"])
		for _, raw := range ArrayValue(schema["required"]) {
			key := StringValue(raw)
			if key != "" {
				result[key] = EmptySchemaValue(MapValue(properties[key]))
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

func SchemaValueMatches(value any, schema map[string]any) bool {
	switch StringValue(schema["type"]) {
	case "object":
		if _, ok := value.(map[string]any); ok {
			return true
		}
		if IsArrayContainer(value) {
			for _, item := range ArrayValue(value) {
				if _, ok := item.(map[string]any); ok {
					return true
				}
			}
		}
		return false
	case "array":
		return IsArrayContainer(value)
	case "string":
		_, ok := value.(string)
		return ok
	case "number", "integer":
		return IsJSONNumber(value)
	case "boolean":
		_, ok := value.(bool)
		return ok
	default:
		return false
	}
}

func IsArrayContainer(value any) bool {
	switch value.(type) {
	case []any, []map[string]any, []string:
		return true
	default:
		return false
	}
}

func IsJSONNumber(value any) bool {
	switch value.(type) {
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func MapValue(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return nil
}

func ArrayValue(value any) []any {
	if a, ok := value.([]any); ok {
		return a
	}
	return nil
}

func StringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
