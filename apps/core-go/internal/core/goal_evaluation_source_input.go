package core

import (
	"slices"
)

func renderedGoalEvaluationProviderInput(snapshot goalEvaluationSnapshot) (string, error) {
	_, stable, current, err := goalEvaluationWireInput(snapshot)
	if err != nil {
		return "", err
	}
	return renderGoalEvaluationProviderInput(stable, current), nil
}

func renderGoalEvaluationProviderInput(stable, current map[string]any) string {
	return jsonString(goalEvaluationProviderPacket{StableDefinitions: stable, Current: current})
}

func goalEvaluationFactInput(fact map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"subject_actor_id", "actor_id", "attribute", "value_json", "value", "epistemic_kind", "status", "valid_from", "effective_at", "valid_until", "transition_kind"} {
		if value, ok := fact[key]; ok {
			result[key] = value
		}
	}
	return result
}

func compactGoalEvaluationSourceData(source GoalSource) map[string]any {
	if source.Kind == "actor_fact" {
		return map[string]any{"fact": goalEvaluationFactInput(mapValue(source.Data["fact"]))}
	}
	if source.Kind != "outcome" {
		result := cloneMap(source.Data)
		delete(result, "context_authority")
		return result
	}
	result := cloneMap(source.Data)
	capability := stringValue(source.Data["capability"])
	observed := mapValue(source.Data["observed"])
	switch capability {
	case goalEvaluationSubmit, goalObjectSubmit, goalPlanSubmit, "goal.inspect", "goal.decide", "goal.evaluate", "goal.review":
		// The current selected Goal/criteria are already present separately.
		result["observed"] = map[string]any{"status": source.Data["status"]}
	case "actor.inspect", "actor.fact.record":
		// Tool receipts use actor_id/value/effective_at; database sources use
		// subject_actor_id/value_json/valid_from. Preserve both semantic shapes.
		view := goalEvaluationFactInput(observed)
		for _, key := range []string{"history", "timezone_semantics", "operation"} {
			if v, ok := observed[key]; ok {
				view[key] = v
			}
		}
		if values, ok := observed["facts"]; ok {
			facts := []any{}
			for _, value := range arrayValue(values) {
				facts = append(facts, goalEvaluationFactInput(mapValue(value)))
			}
			view["facts"] = facts
		}
		if value, ok := observed["fact"]; ok {
			view["fact"] = goalEvaluationFactInput(mapValue(value))
		}
		result["observed"] = view
	}
	return result
}

func admittedGoalEvaluationSources(snapshot goalEvaluationSnapshot) []GoalSource {
	if snapshot.ProviderSourceIDs == nil {
		return snapshot.Sources
	}
	selected := []GoalSource{}
	for _, source := range snapshot.Sources {
		if slices.Contains(snapshot.ProviderSourceIDs, source.EventID) {
			selected = append(selected, source)
		}
	}
	return selected
}

func interleaveGoalRecoveryLanes(linked, unlinked []int64) []int64 {
	result := make([]int64, 0, len(linked)+len(unlinked))
	for i := 0; i < max(len(linked), len(unlinked)); i++ {
		if i < len(linked) {
			result = append(result, linked[i])
		}
		if i < len(unlinked) {
			result = append(result, unlinked[i])
		}
	}
	return result
}

// All sources in the authorized frozen snapshot reach the Provider. Token
// estimates cannot exclude goal definitions, required evidence or query results.
func admitGoalEvaluationSourceInput(snapshot goalEvaluationSnapshot) (goalEvaluationSnapshot, error) {
	snapshot.ProviderSourceIDs = make([]int64, 0, len(snapshot.Sources))
	for _, source := range snapshot.Sources {
		snapshot.ProviderSourceIDs = append(snapshot.ProviderSourceIDs, source.EventID)
	}
	if _, err := renderedGoalEvaluationProviderInput(snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}
