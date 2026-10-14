package core

import "context"

type physicalRequestObservationKey struct{}
type physicalRequestObservation func(context.Context, map[string]any)

func withPhysicalRequestObservation(ctx context.Context, provider *ProviderClient, role string) context.Context {
	if _, ok := ctx.Value(physicalRequestObservationKey{}).(physicalRequestObservation); ok {
		return ctx
	}
	return context.WithValue(ctx, physicalRequestObservationKey{}, physicalRequestObservation(func(callCtx context.Context, body map[string]any) {
		if provider == nil || provider.DB == nil || provider.DB.Pool() == nil {
			return
		}
		diag := providerPromptDiagnostics(callCtx)
		payload := cloneMap(body)
		payload["run_id"] = diag["run_id"]
		payload["model_call_id"] = diag["model_call_id"]
		payload["role"] = role
		payload["phase"] = factOnlyRequestPhase(callCtx)
		if role == conversationReplyModelRole {
			payload["phase"] = "reply"
		}
		payload["context_view"] = diag["context_view"]
		payload["module_estimates"] = physicalRequestModuleEstimates(diag)
		provider.runtimeSupport().RecordDiagnosticEvent(callCtx, "adk.model.effective_request", "info", stringValue(diag["fluctlight_id"]), "", providerCorrelation(callCtx), payload)
	}))
}

func observePhysicalRequest(ctx context.Context, body map[string]any) {
	if observe, ok := ctx.Value(physicalRequestObservationKey{}).(physicalRequestObservation); ok && observe != nil {
		observe(ctx, physicalRequestConfiguration(body))
	}
}

// This allowlist receives the final serialized body, but never exports messages,
// credential headers, reasoning, freeform extra fields or response-schema text.
func physicalRequestConfiguration(body map[string]any) map[string]any {
	result := map[string]any{"source": "serialized_request_payload", "usage": "provider_response_only"}
	for _, key := range []string{"model", "temperature", "top_p", "max_tokens", "max_completion_tokens", "enable_thinking", "stream"} {
		if value, ok := body[key]; ok {
			result[key] = value
		}
	}
	if choice, ok := body["tool_choice"].(string); ok {
		result["tool_choice"] = choice
	}
	format := mapValue(body["response_format"])
	result["response_format_present"] = len(format) > 0
	if kind := stringValue(format["type"]); kind != "" {
		result["response_format_type"] = kind
	}
	tools := arrayValue(body["tools"])
	result["tool_count"] = len(tools)
	if len(tools) > 0 {
		result["tool_schema_digest"] = stableDigest(jsonString(tools))
	}
	return result
}

func physicalRequestModuleEstimates(diag map[string]any) map[string]any {
	sections := mapValue(mapValue(diag["prompt_budget"])["section_tokens"])
	if len(sections) == 0 {
		return map[string]any{"available": false, "count_mode": "estimated_logical_sections_not_actual_usage"}
	}
	value := func(key string) int { return intValue(sections[key]) }
	protocol, persona := value("protocol"), value("working_persona")
	if protocol == 0 && persona == 0 {
		protocol = value("system")
	}
	identity := value("runtime.self_actor") + value("runtime.current_speaker") + value("runtime.actor_background") + value("runtime.current_state") + value("runtime.time_view")
	goals := value("runtime.goals")
	return map[string]any{"available": true, "count_mode": "estimated_logical_sections_not_actual_usage", "protocol": protocol, "persona": persona,
		"identity_current_state": identity, "goals": goals,
		"facts_memory": value("active_memory") + value("resident_memory") + value("retrieved_memory") + value("conversation_summary"),
		"history":      value("recent"), "current_input": value("current_input"), "tools_schema": value("tools") + value("response_schema"),
		"other_runtime": max(0, value("runtime_facts")-identity-goals)}
}
