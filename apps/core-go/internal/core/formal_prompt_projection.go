package core

import (
	"context"
	"errors"
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
		if stringValue(message["role"]) == "user" {
			lastUser = index
		}
	}
	for index, message := range input.Prompt.Messages {
		role := stringValue(message["role"])
		source := "task_data"
		origin := promptMessageOrigin{}
		if len(input.Prompt.messageOrigins) == len(input.Prompt.Messages) {
			origin = input.Prompt.messageOrigins[index]
		}
		switch {
		case role == "system":
			source = "trusted_task_configuration"
		case role == "tool":
			source = "tool_result"
		case role == "assistant" && message["tool_calls"] != nil:
			source = "agent_tool_call"
		case origin.Source == "current_task_input" && input.Role == "initialization":
			source = "owner_description"
		case origin.Source != "":
			source = origin.Source
		case index == lastUser && input.Role == "initialization":
			source = "owner_description"
		case index == lastUser:
			source = "current_task_input"
		case role == "user" || role == "assistant":
			source = "historical_message"
		}
		item := map[string]any{"index": index, "role": role, "source": source, "estimated_tokens": estimateProviderMessageTokens(message)}
		if len(origin.SourceRefs) > 0 {
			item["source_refs"] = append([]string(nil), origin.SourceRefs...)
		}
		if origin.Reason != "" {
			item["reason"] = origin.Reason
		}
		mapping = append(mapping, item)
	}
	diagnostics := providerPromptDiagnostics(ctx)
	versions := map[string]any{}
	if input.Capability != nil {
		// The validated invocation is the scope authority. Do not infer a role
		// from correlation text or depend on a surface's optional trace fields.
		diagnostics["fluctlight_id"] = input.Capability.FluctlightID
		diagnostics["conversation_id"] = input.Capability.ConversationID
		projection := input.Capability.Projection
		versions = map[string]any{
			"foundation": projection.ContextRevision, "current_state": projection.CurrentStateRevision,
			"current_facts": projection.CurrentFactsRevision, "life_context": projection.LifeContextRevision,
			"active_profile": stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"]),
		}
	}
	diagnostics["context_projection"] = map[string]any{
		"agent_id": input.AgentID, "task_variant": firstString(input.TaskVariant, input.SchemaName), "role": input.Role, "schema": input.SchemaName, "source_map": mapping,
		"versions": versions, "token_estimate_method": "rune_heuristic",
		"selected": input.Prompt.Trace.Selected, "dropped": input.Prompt.Trace.Dropped,
	}
	return WithPromptDiagnostics(ctx, diagnostics), nil
}
