package capability

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

func stringValue(value any) string {
	if result, ok := value.(string); ok {
		return strings.TrimSpace(result)
	}
	return ""
}

func mapValue(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}

func arrayValue(value any) []any {
	switch result := value.(type) {
	case []any:
		return result
	case []map[string]any:
		items := make([]any, len(result))
		for index, item := range result {
			items[index] = item
		}
		return items
	case []string:
		items := make([]any, len(result))
		for index, item := range result {
			items[index] = item
		}
		return items
	}
	return []any{}
}

func jsonBytes(value any) []byte {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return encoded
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, val := range value {
		result[key] = val
	}
	return result
}

func intValue(value any) int {
	if number, ok := numberFloat(value); ok {
		return int(number)
	}
	return 0
}

func numberFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case json.Number:
		floatVal, err := typed.Float64()
		return floatVal, err == nil
	}
	return 0, false
}

func stableDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
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

func NormalizeToolArguments(value any) (json.RawMessage, error) {
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
