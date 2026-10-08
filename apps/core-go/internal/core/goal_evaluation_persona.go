package core

import (
	"encoding/json"
	"strings"
)

// Goal execution is also exposed through the common runtime projection.
// Keep business state and time windows, not storage/version/retry controls.
func goalRuntimeSemantics(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := map[string]any{}
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
			if normalized == "id" || strings.HasSuffix(normalized, "id") || strings.HasSuffix(normalized, "ids") || strings.HasSuffix(normalized, "revision") || strings.HasSuffix(normalized, "version") || strings.HasSuffix(normalized, "ref") || strings.HasSuffix(normalized, "refs") || normalized == "retrycount" || normalized == "attemptcount" || normalized == "updatedat" || normalized == "createdat" {
				continue
			}
			result[key] = goalRuntimeSemantics(child)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			result = append(result, goalRuntimeSemantics(child))
		}
		return result
	default:
		return value
	}
}

// Preserve portrait meaning, including legacy JSON-shaped portrait facts,
// without promoting their profile/storage bookkeeping to the Goal model.
func goalEvaluationPersonaSemantics(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := map[string]any{}
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
			if normalized == "id" || strings.HasSuffix(normalized, "id") || strings.HasSuffix(normalized, "ids") || strings.HasSuffix(normalized, "revision") || strings.HasSuffix(normalized, "version") || normalized == "sourcehash" || normalized == "sourcehashprefix" {
				continue
			}
			result[key] = goalEvaluationPersonaSemantics(child)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			result = append(result, goalEvaluationPersonaSemantics(child))
		}
		return result
	case []string:
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			result = append(result, goalEvaluationPersonaSemantics(child))
		}
		return result
	case string:
		var decoded any
		if json.Unmarshal([]byte(typed), &decoded) == nil {
			switch decoded.(type) {
			case map[string]any, []any:
				return jsonString(goalEvaluationPersonaSemantics(decoded))
			}
		}
		return typed
	default:
		return value
	}
}
