package core

import (
	"encoding/json"
	"log/slog"
	"sort"
	"strings"
)

// bindMediaContextToToolCalls freezes the cognition-time context into every
// image capability call. The model remains free to describe the subject,
// pose, camera and style, but scene-like fields are aligned with the
// authoritative context unless the model explicitly marks a context override.
func bindMediaContextToToolCalls(calls []ToolCallV1, projection ContextProjection) []ToolCallV1 {
	if len(calls) == 0 {
		return calls
	}
	result := append([]ToolCallV1(nil), calls...)
	for index, call := range result {
		if call.Name != "media.image.generate" {
			continue
		}
		var arguments map[string]any
		if err := json.Unmarshal(call.Arguments, &arguments); err != nil {
			continue
		}
		concept := mediaConceptValue(arguments["concept"])
		if len(concept) == 0 {
			continue
		}
		aligned, changedFields := alignMediaConceptWithContext(concept, projection)
		arguments["concept"] = aligned
		encoded, err := json.Marshal(arguments)
		if err != nil {
			continue
		}
		result[index].Arguments = json.RawMessage(encoded)
		if len(changedFields) > 0 {
			slog.Default().Warn("Go Core media concept aligned to cognition context",
				"fluctlight_id", projection.FluctlightID,
				"source_fact_id", projection.SourceFactID,
				"context_revision", projection.ContextRevision,
				"fields", changedFields,
			)
		}
	}
	return result
}

func alignMediaConceptWithContext(concept map[string]any, projection ContextProjection) (map[string]any, []string) {
	result := cloneMap(concept)
	binding := map[string]any{
		"source":           "cognition.life_context",
		"context_revision": projection.ContextRevision,
		"life_context":     cloneMap(projection.LifeContext),
	}
	if innerState := cloneMap(projection.InnerState); len(innerState) > 0 {
		binding["inner_state"] = innerState
	}
	if appearance := mapValue(projection.Identity["appearance"]); len(appearance) > 0 {
		binding["appearance"] = cloneMap(appearance)
	}
	result["context_binding"] = binding

	override := mapValue(result["context_override"])
	explicitOverride, _ := override["explicit"].(bool)
	if explicitOverride {
		return result, nil
	}

	changed := make([]string, 0, 5)
	for _, field := range []string{"scene", "activity", "location"} {
		current := projection.LifeContext[field]
		if contextString(current) == "" {
			continue
		}
		if !compatibleContextText(contextString(result[field]), contextString(current)) {
			result[field] = contextString(current)
			changed = append(changed, field)
		}
	}
	// Only align outfit/hair/mood when the current projection actually has a
	// concrete value for that field. Foundation preferences are not treated as
	// the current outfit or hairstyle.
	for _, field := range []string{"outfit", "hair", "mood"} {
		current := currentStateField(projection, field)
		if current == nil {
			continue
		}
		if jsonString(result[field]) != jsonString(current) {
			result[field] = current
			changed = append(changed, field)
		}
	}
	sort.Strings(changed)
	return result, changed
}

func compatibleContextText(candidate, current string) bool {
	candidate = strings.TrimSpace(candidate)
	current = strings.TrimSpace(current)
	if candidate == "" || current == "" {
		return candidate == current
	}
	if candidate == current {
		return true
	}
	// Preserve a concrete elaboration such as "教室（靠窗座位）" while still
	// rejecting ambiguous alternatives such as "教室/图书馆".
	if strings.Contains(candidate, current) && !strings.ContainsAny(candidate, "/／、|或") {
		return true
	}
	return false
}

func contextString(value any) string {
	switch typed := value.(type) {
	case *string:
		if typed == nil {
			return ""
		}
		return *typed
	default:
		return stringValue(value)
	}
}

func currentStateField(projection ContextProjection, field string) any {
	if value := projection.LifeContext[field]; value != nil {
		return value
	}
	if value := projection.InnerState[field]; value != nil {
		return value
	}
	if appearance := mapValue(projection.Identity["appearance"]); appearance[field] != nil {
		return appearance[field]
	}
	return nil
}

func bindMediaPromptContext(prompt string) string {
	if !strings.Contains(prompt, "context_binding") {
		return prompt
	}
	return "The JSON below contains an authoritative context_binding snapshot captured during cognition. Preserve its current scene, activity, location, mood, and concrete appearance. Do not replace a classroom or library with a bedroom unless context_override.explicit is true.\n\n" + prompt
}

func withContextAuthorityInstruction(messages []map[string]any) []map[string]any {
	if len(messages) == 0 {
		return messages
	}
	instruction := map[string]any{
		"role":    "system",
		"content": "The context.life_context and context.inner_state in this request are the authoritative cognition-time snapshot. Keep the decision and every capability argument consistent with the current scene, activity, location, mood, and any concrete appearance fields present in that snapshot. Do not introduce a different room or activity. Only an explicit user request may change context, and that change must be represented with context_override.explicit=true in the affected capability concept.",
	}
	result := make([]map[string]any, 0, len(messages)+1)
	result = append(result, messages[0], instruction)
	result = append(result, messages[1:]...)
	return result
}
