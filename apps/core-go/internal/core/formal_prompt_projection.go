package core

import (
	"context"
	"errors"
	"strings"
)

// projectFormalPromptBoundary is the final read-only boundary for every
// formal model task. Surface assemblers may select different data, but all
// tasks enter the Provider with one source map and the same physical budget
// enforcement. It never changes the raw task messages or domain state.
func projectFormalPromptBoundary(ctx context.Context, input ADKStructuredTaskInput) (context.Context, error) {
	if !validAssembledProviderMessages(input.Prompt.Messages) {
		return ctx, errors.New("formal_prompt_projection_invalid")
	}
	mapping := make([]map[string]any, 0, len(input.Prompt.Messages))
	lastUser := -1
	for index, message := range input.Prompt.Messages {
		if stringValue(message["role"]) == "user" && !strings.HasPrefix(stringValue(message["content"]), "[RUNTIME CONTEXT]\n") {
			lastUser = index
		}
	}
	for index, message := range input.Prompt.Messages {
		role := stringValue(message["role"])
		source := "task_data"
		switch {
		case role == "system":
			source = "trusted_task_configuration"
		case role == "user" && strings.HasPrefix(stringValue(message["content"]), "[RUNTIME CONTEXT]\n"):
			source = "scoped_runtime_projection"
		case index == lastUser && input.Role == "initialization":
			source = "owner_description"
		case index == lastUser:
			source = "current_task_input"
		case role == "user" || role == "assistant":
			source = "historical_message"
		}
		mapping = append(mapping, map[string]any{"index": index, "role": role, "source": source, "estimated_tokens": estimateProviderMessageTokens(message)})
	}
	diagnostics := providerPromptDiagnostics(ctx)
	versions := map[string]any{}
	if input.Capability != nil {
		projection := input.Capability.Projection
		versions = map[string]any{
			"foundation": projection.ContextRevision, "current_state": projection.CurrentStateRevision,
			"current_facts": projection.CurrentFactsRevision, "life_context": projection.LifeContextRevision,
			"active_profile": stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"]),
		}
	}
	diagnostics["context_projection"] = map[string]any{
		"role": input.Role, "schema": input.SchemaName, "source_map": mapping,
		"versions": versions, "token_estimate_method": "rune_heuristic",
		"selected": input.Prompt.Trace.Selected, "dropped": input.Prompt.Trace.Dropped,
	}
	return WithPromptDiagnostics(ctx, diagnostics), nil
}
