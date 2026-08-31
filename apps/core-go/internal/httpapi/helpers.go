package httpapi

import (
	"encoding/json"
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
