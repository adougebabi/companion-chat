package core

import (
	"errors"
	"slices"
)

// Leave room below the existing 16384 current-input ceiling. This bounds only
// the provider view; the complete source/CAS snapshot remains durable.
const goalEvaluationSourceInputBudget = 12000

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
		return source.Data
	}
	result := cloneMap(source.Data)
	capability := stringValue(source.Data["capability"])
	observed := mapValue(source.Data["observed"])
	switch capability {
	case "goal.inspect", "goal.decide", "goal.evaluate", "goal.review":
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

func admitGoalEvaluationSourceInput(snapshot goalEvaluationSnapshot) (goalEvaluationSnapshot, error) {
	mandatory := map[string]bool{}
	for _, goal := range snapshot.Goals {
		for _, judgment := range goal.CurrentJudgments {
			for _, ref := range judgment.EvidenceRefs {
				mandatory[ref] = true
			}
		}
	}
	sources := append([]GoalSource(nil), snapshot.Sources...)
	priority := func(source GoalSource) int {
		if mandatory[source.Ref] {
			return 0
		}
		if source.Kind == "message" {
			return 1
		}
		if source.Valid && source.CanSupportSuccess {
			return 2
		}
		return 3
	}
	slices.SortStableFunc(sources, func(a, b GoalSource) int {
		if priority(a) != priority(b) {
			return priority(a) - priority(b)
		}
		if !a.RecordedAt.Equal(b.RecordedAt) {
			if a.RecordedAt.After(b.RecordedAt) {
				return -1
			}
			return 1
		}
		if a.EventID > b.EventID {
			return -1
		}
		if a.EventID < b.EventID {
			return 1
		}
		return 0
	})
	snapshot.ProviderSourceIDs = []int64{}
	rendered, err := renderedGoalEvaluationProviderInput(snapshot)
	if err != nil {
		return snapshot, err
	}
	if EstimatePromptTokens(rendered) > goalEvaluationSourceInputBudget {
		return snapshot, errors.New("goal_evaluation_goal_input_budget_exceeded")
	}
	for _, source := range sources {
		snapshot.ProviderSourceIDs = append(snapshot.ProviderSourceIDs, source.EventID)
		rendered, err = renderedGoalEvaluationProviderInput(snapshot)
		if err != nil {
			return snapshot, err
		}
		if EstimatePromptTokens(rendered) > goalEvaluationSourceInputBudget {
			snapshot.ProviderSourceIDs = snapshot.ProviderSourceIDs[:len(snapshot.ProviderSourceIDs)-1]
			if mandatory[source.Ref] {
				return snapshot, errors.New("goal_evaluation_required_evidence_budget_exceeded")
			}
		}
	}
	if len(snapshot.Sources) > 0 && len(snapshot.ProviderSourceIDs) == 0 {
		return snapshot, errors.New("goal_evaluation_source_input_budget_exceeded")
	}
	return snapshot, nil
}
