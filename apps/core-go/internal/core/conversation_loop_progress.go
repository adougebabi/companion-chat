package core

import "context"

// Re-reading the same facts or repeating the same no-effect rejection cannot
// create new business progress. Once exhausted, keep the turn in expression:
// the independent Writer still decides its text from actual facts/history.
func conversationLoopNeedsReply(ctx context.Context) bool {
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Trace == nil {
		return false
	}
	invoker, ok := bridge.Invoker.(*appADKCapabilityInvoker)
	if !ok || invoker.scopeRegistry == nil {
		return false
	}
	invocations, results := bridge.Trace.Snapshot()
	calls := map[string]CapabilityInvocation{}
	for _, call := range invocations {
		calls[call.CallID] = call
	}
	seen := map[string]int{}
	ready := false
	for _, result := range results {
		call, exists := calls[result.CallID]
		if !exists {
			continue
		}
		definition, installed := invoker.scopeRegistry.Definition(result.CapabilityName)
		out := mapValue(result.Output)
		noEffect := result.Status == "handled" && out["effect_applied"] == false
		query := installed && definition.Type == CapabilityTypeQuery && result.Status == "completed" && result.CapabilityName != capabilityCatalogName && result.CapabilityName != capabilityDiscoverName
		if query || noEffect {
			fingerprint := jsonString(map[string]any{"name": result.CapabilityName, "arguments": decodeObject(call.Arguments), "status": result.Status, "output": result.Output})
			seen[fingerprint]++
			if seen[fingerprint] >= 3 {
				ready = true
			}
		} else if installed && definition.Type == CapabilityTypeAction && (result.Status == "completed" || result.Status == "accepted") {
			seen = map[string]int{}
			ready = false
		}
	}
	return ready
}

// Repeated disclosure of an already selected read-only schema should lead to
// that query, not a fabricated answer. This only narrows the next model choice;
// it never synthesizes arguments or executes an action from a directory load.
func conversationLoadedQueryPhase(ctx context.Context) []string {
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Trace == nil {
		return nil
	}
	invoker, ok := bridge.Invoker.(*appADKCapabilityInvoker)
	if !ok || invoker.scopeRegistry == nil {
		return nil
	}
	_, results := bridge.Trace.Snapshot()
	seen := map[string]int{}
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		if result.CapabilityName != capabilityCatalogName && result.CapabilityName != capabilityDiscoverName {
			if result.Status == "completed" || result.Status == "accepted" {
				return nil
			}
			continue
		}
		out := mapValue(result.Output)
		if result.Status != "completed" || len(arrayValue(out["loaded"])) == 0 || len(arrayValue(out["unavailable"])) > 0 {
			continue
		}
		key := jsonString(out["loaded"])
		seen[key]++
		if seen[key] < 2 {
			continue
		}
		var names []string
		for _, raw := range arrayValue(out["loaded"]) {
			name := stringValue(raw)
			definition, found := invoker.scopeRegistry.Definition(name)
			if found && definition.Type == CapabilityTypeQuery && definition.SideEffectClass == "read_only" && name != capabilityCatalogName && name != capabilityDiscoverName {
				names = append(names, name)
			}
		}
		return names
	}
	return nil
}

func conversationClosedActionAttempts(trace *ADKCapabilityTrace) int {
	_, results := trace.Snapshot()
	count := 0
	for _, result := range results {
		if (result.Status == "completed" || result.Status == "accepted") && result.CapabilityName != capabilityCatalogName && result.CapabilityName != capabilityDiscoverName {
			count = 0
		}
		if mapValue(result.Output)["warning_code"] == "turn_action_window_closed" {
			count++
		}
	}
	return count
}
