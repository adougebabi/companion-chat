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
	if len(projection.VisualIdentity) > 0 {
		binding["visual_identity"] = cloneMap(projection.VisualIdentity)
	}
	if innerState := cloneMap(projection.InnerState); len(innerState) > 0 {
		binding["inner_state"] = innerState
	}
	if appearance := visualIdentityAppearance(projection); len(appearance) > 0 {
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

func visualIdentityAppearance(projection ContextProjection) map[string]any {
	if identity := mapValue(projection.VisualIdentity["identity_snapshot"]); len(identity) > 0 {
		if profile := mapValue(identity["life_profile"]); len(profile) > 0 {
			if appearance := mapValue(profile["appearance"]); len(appearance) > 0 {
				return appearance
			}
		}
	}
	return mapValue(projection.Identity["appearance"])
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
	if appearance := visualIdentityAppearance(projection); appearance[field] != nil {
		return appearance[field]
	}
	return nil
}

func bindMediaPromptContext(prompt string) string {
	if !strings.Contains(prompt, "context_binding") {
		return prompt
	}
	return "The structured JSON payload below contains an authoritative context_binding snapshot captured during cognition. Preserve its current scene, activity, location, mood, concrete appearance, visual_identity, and renderer constraints (including the resolved chest LoRA weight). Do not replace a classroom or library with a bedroom unless context_override.explicit is true.\n\n" + prompt
}

func withContextAuthorityInstruction(messages []map[string]any) []map[string]any {
	if len(messages) == 0 {
		return messages
	}
	instruction := map[string]any{
		"role":    "system",
		"content": providerContextAuthorityRule,
	}
	return prependSystemMessage(messages, instruction)
}
