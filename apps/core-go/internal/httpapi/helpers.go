package httpapi

import (
	"encoding/json"
	"io"
	"strings"
)

func stringValue(value any) string {
	if result, ok := value.(string); ok {
		return strings.TrimSpace(result)
	}
	return ""
}

func firstString(value any, fallback string) string {
	if result := stringValue(value); result != "" {
		return result
	}
	return fallback
}

func mapValue(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}

func arrayValue(value any) []any {
	if result, ok := value.([]any); ok {
		return result
	}
	return []any{}
}

func jsonValue(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}

// decodeObjectBody enforces the Core boundary contract: exactly one JSON
// object, with no trailing values. The caller supplies a bounded reader.
func decodeObjectBody(body io.Reader) (map[string]any, bool) {
	decoder := json.NewDecoder(body)
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false
	}
	return value, true
}
