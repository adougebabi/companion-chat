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
	return conversationNoProgressAttempts(ctx, currentSequence, false) >= 3
}

func conversationNoProgressAttempts(ctx context.Context, currentSequence uint64, physicalRoundsOnly bool) int {
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Trace == nil {
		return 0
	}
	invoker, ok := bridge.Invoker.(*appADKCapabilityInvoker)
	if !ok || invoker.scopeRegistry == nil {
		return 0
	}
	invocations, results := bridge.Trace.Snapshot()
	calls := map[string]CapabilityInvocation{}
	for _, call := range invocations {
		calls[call.CallID] = call
	}
	// Actual action progress invalidates earlier no-progress rounds, including
	// denials that reused an older call ID after that action.
	lastActionIndex := -1
	var lastActionSequence uint64
	for index, result := range results {
		definition, installed := invoker.scopeRegistry.Definition(result.CapabilityName)
		if installed && definition.Type == CapabilityTypeAction && (result.Status == "completed" || result.Status == "accepted") {
			lastActionIndex = index
			if identity, found := bridge.Trace.ModelIdentity(result.CallID); found && identity.ModelCallSequence > lastActionSequence {
				lastActionSequence = identity.ModelCallSequence
			}
		}
	}
	seen := map[string]int{}
	deniedRounds := map[string]map[string]bool{}
	maximum := 0
	for index, result := range results {
		call, exists := calls[result.CallID]
		if !exists {
			continue
		}
		definition, installed := invoker.scopeRegistry.Definition(result.CapabilityName)
		out := mapValue(result.Output)
		noEffect := result.Status == "handled" && out["effect_applied"] == false
		query := installed && definition.Type == CapabilityTypeQuery && result.Status == "completed" && result.CapabilityName != capabilityCatalogName
		if !query && !noEffect {
			continue
		}
		arguments := decodeObject(call.Arguments)
		warning := stringValue(out["warning_code"])
		unauthorized := noEffect && warning == "capability_not_authorized"
		awaitingCatalog := noEffect && warning == "catalog_discovery_pending"
		momentMediaRejected := noEffect && result.CapabilityName == "moment.publish" && (warning == "direct_media_delivery_unverified" || warning == "direct_media_delivery_stale" || warning == "moment_media_delivery_stale")
		roundBounded := unauthorized || awaitingCatalog || momentMediaRejected
		if physicalRoundsOnly && !roundBounded {
			continue
		}
		if roundBounded {
			arguments = nil
		}
		name := result.CapabilityName
		if awaitingCatalog {
			name = capabilityCatalogName
		}
		fingerprint := jsonString(map[string]any{"name": name, "arguments": arguments, "status": result.Status, "output": result.Output})
		if roundBounded {
			sequences := conversationCallSequences(bridge.Trace, bridge.Refresh, call.CallID, result.CapabilityName)
			if deniedRounds[fingerprint] == nil {
				deniedRounds[fingerprint] = map[string]bool{}
			}
			if len(sequences) == 0 && index > lastActionIndex {
				deniedRounds[fingerprint]["call:"+call.CallID] = true
			}
			for _, sequence := range sequences {
				if sequence <= lastActionSequence || currentSequence != 0 && sequence >= currentSequence {
					continue
				}
				deniedRounds[fingerprint]["model:"+strconv.FormatUint(sequence, 10)] = true
			}
			seen[fingerprint] = len(deniedRounds[fingerprint])
		} else if index > lastActionIndex {
			seen[fingerprint]++
		}
		if seen[fingerprint] > maximum {
			maximum = seen[fingerprint]
		}
	}
	return maximum
}

func conversationCallSequences(trace *ADKCapabilityTrace, refresh *runtimeContextRefresh, callID, capabilityName string) []uint64 {
	if sequences := refresh.modelToolCallSequences(callID, capabilityName); len(sequences) > 0 {
		return sequences
	}
	if identity, found := trace.ModelIdentity(callID); found && identity.ModelCallSequence != 0 {
		return []uint64{identity.ModelCallSequence}
	}
	return nil
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
		if result.CapabilityName != capabilityCatalogName {
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
			if found && definition.Type == CapabilityTypeQuery && definition.SideEffectClass == "read_only" && name != capabilityCatalogName {
				names = append(names, name)
			}
		}
		return names
	}
	return nil
}

func conversationClosedActionAttempts(trace *ADKCapabilityTrace, refresh *runtimeContextRefresh) int {
	_, results := trace.Snapshot()
	lastProgressIndex := -1
	var lastProgressSequence uint64
	for index, result := range results {
		if (result.Status == "completed" || result.Status == "accepted") && result.CapabilityName != capabilityCatalogName {
			lastProgressIndex = index
			if identity, found := trace.ModelIdentity(result.CallID); found && identity.ModelCallSequence > lastProgressSequence {
				lastProgressSequence = identity.ModelCallSequence
			}
		}
	}
	rounds := map[string]bool{}
	for index, result := range results {
		if mapValue(result.Output)["warning_code"] != "turn_action_window_closed" {
			continue
		}
		sequences := conversationCallSequences(trace, refresh, result.CallID, result.CapabilityName)
		if len(sequences) == 0 && index > lastProgressIndex {
			rounds["call:"+result.CallID] = true
		}
		for _, sequence := range sequences {
			if sequence > lastProgressSequence {
				rounds["model:"+strconv.FormatUint(sequence, 10)] = true
			}
		}
	}
	return len(rounds)
}
