package core

import "strings"

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

// Only the compiled-persona and roster envelopes are Core bookkeeping.
// Portrait strings and values inside the selected persona are opaque data.
func goalEvaluationPersonaSemantics(value any) any {
	result, ok := copyPromptData(value).(map[string]any)
	if !ok {
		return copyPromptData(value)
	}
	for _, key := range []string{workingPersonaProfileIDKey, "source_revision", "source_hash", "source_hash_prefix", "overlay_revision", "rules_version"} {
		delete(result, key)
	}
	if body := mapValue(result[workingPersonaBodyKey]); body != nil {
		for _, key := range []string{"profile_id", "source_revision", "source_hash", "source_hash_prefix", "overlay_revision", "rules_version"} {
			delete(body, key)
		}
	}
	if system := mapValue(result["personality_system"]); system != nil {
		delete(system, "active_profile_id")
		switch profiles := system["profiles"].(type) {
		case []any:
			for _, profile := range profiles {
				deleteGoalPersonaRosterIDs(mapValue(profile))
			}
		case []map[string]any:
			for _, profile := range profiles {
				deleteGoalPersonaRosterIDs(profile)
			}
		}
	}
	return result
}

func deleteGoalPersonaRosterIDs(profile map[string]any) {
	delete(profile, "id")
	delete(profile, "profile_id")
}
