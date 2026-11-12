package core

import (
	"context"
	"strconv"
)

// Re-reading the same facts or repeating the same no-effect rejection cannot
// create new business progress. Once exhausted, keep the turn in expression:
// the independent Writer still decides its text from actual facts/history.
func conversationLoopNeedsReply(ctx context.Context) bool {
	return conversationLoopNeedsReplyBeforeModelSequence(ctx, 0)
}

// While executing a model response, do not treat a denial in that same
// response as feedback the model has already received. The next physical
// request evaluates all results through conversationLoopNeedsReply.
func conversationLoopNeedsReplyBeforeModelSequence(ctx context.Context, currentSequence uint64) bool {
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
	deniedRounds := map[string]map[string]bool{}
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
			arguments := decodeObject(call.Arguments)
			unauthorized := noEffect && stringValue(out["warning_code"]) == "capability_not_authorized"
			if unauthorized {
				if identity, ok := bridge.Trace.ModelIdentity(call.CallID); ok && currentSequence != 0 && identity.ModelCallSequence == currentSequence {
					continue
				}
				// A run/surface denial cannot be repaired by changing arguments.
				// Other handled results retain argument-sensitive recovery.
				arguments = nil
			}
			fingerprint := jsonString(map[string]any{"name": result.CapabilityName, "arguments": arguments, "status": result.Status, "output": result.Output})
			if unauthorized {
				// Multiple calls returned by one physical model response have not
				// yet received feedback and count as one attempt, not a loop.
				round := "call:" + call.CallID
				if identity, ok := bridge.Trace.ModelIdentity(call.CallID); ok && identity.ModelCallSequence != 0 {
					round = "model:" + strconv.FormatUint(identity.ModelCallSequence, 10)
				}
				if deniedRounds[fingerprint] == nil {
					deniedRounds[fingerprint] = map[string]bool{}
				}
				if !deniedRounds[fingerprint][round] {
					deniedRounds[fingerprint][round] = true
					seen[fingerprint]++
				}
			} else {
				seen[fingerprint]++
			}
			if seen[fingerprint] >= 3 {
				ready = true
			}
		} else if installed && definition.Type == CapabilityTypeAction && (result.Status == "completed" || result.Status == "accepted") {
			seen = map[string]int{}
			deniedRounds = map[string]map[string]bool{}
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
