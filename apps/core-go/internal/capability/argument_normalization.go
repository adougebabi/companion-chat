package capability

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// NormalizeModelArguments repairs representation only. It never manufactures
// required business values or accepts an authority envelope from model input.
// Direct owner/governance commands continue to use strict validation.
func NormalizeModelArguments(raw json.RawMessage, schema map[string]any) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, errors.New("arguments must be a JSON object")
	}
	object, ok := value.(map[string]any)
	if !ok || object == nil {
		return nil, errors.New("arguments must be a JSON object")
	}
	for _, key := range []string{"prepared_payload", "context_snapshot", "authorization_actor_id", "authorization_policy", "metadata"} {
		if _, present := object[key]; present {
			return nil, errors.New("model arguments cannot supply an authority envelope")
		}
	}
	value = normalizeModelValue(object, schema)
	if err := ValidateCapabilitySchemaValue(value, schema); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func schemaAllowsNull(schema map[string]any) bool {
	if schema["type"] == "null" {
		return true
	}
	for _, kind := range arrayValue(schema["type"]) {
		if kind == "null" {
			return true
		}
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		for _, raw := range arrayValue(schema[key]) {
			if schemaAllowsNull(mapValue(raw)) {
				return true
			}
		}
	}
	return false
}
func normalizeModelValue(value any, schema map[string]any) any {
	if object, ok := value.(map[string]any); ok && (schema["type"] == "object" || len(mapValue(schema["properties"])) > 0) {
		properties := mapValue(schema["properties"])
		required := map[string]bool{}
		for _, raw := range arrayValue(schema["required"]) {
			required[stringValue(raw)] = true
		}
		for key := range object {
			if _, known := properties[key]; !known && schema["additionalProperties"] == false {
				delete(object, key)
			}
		}
		for key, raw := range properties {
			childSchema := mapValue(raw)
			child, present := object[key]
			if present && child == nil && !required[key] && !schemaAllowsNull(childSchema) {
				delete(object, key)
				present = false
			}
			if !present {
				if fallback, exists := childSchema["default"]; exists {
					object[key] = fallback
				}
				continue
			}
			if child != nil {
				object[key] = normalizeModelValue(child, childSchema)
			}
		}
		return object
	}
	kind, _ := schema["type"].(string)
	if kind == "array" {
		items, ok := value.([]any)
		if !ok {
			if _, object := value.(map[string]any); !object && value != nil {
				items = []any{value}
				ok = true
			}
		}
		if ok {
			for index, item := range items {
				items[index] = normalizeModelValue(item, mapValue(schema["items"]))
			}
			return items
		}
	}
	if text, ok := value.(string); ok {
		trimmed := strings.TrimSpace(text)
		switch kind {
		case "boolean":
			if trimmed == "true" {
				return true
			}
			if trimmed == "false" {
				return false
			}
		case "number", "integer":
			if number, err := strconv.ParseFloat(trimmed, 64); err == nil {
				return number
			}
		case "string":
			var match string
			count := 0
			for _, raw := range arrayValue(schema["enum"]) {
				if candidate, yes := raw.(string); yes && strings.EqualFold(trimmed, candidate) {
					match = candidate
					count++
				}
			}
			if count == 1 {
				return match
			}
		}
	}
	return value
}
