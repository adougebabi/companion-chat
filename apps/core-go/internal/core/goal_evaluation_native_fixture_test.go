package core

import "fmt"

// Existing domain fixtures describe judgments using the frozen wire DTO. This
// controlled Provider script emits them as real native HTTP ToolCalls, one call
// per decision, then consumes the returned Tool messages before its summary.
// There is deliberately no corresponding adapter in production.
func (router *fakeProviderRouter) onGoalEvaluation(script fakeProviderScript) *fakeProviderRouter {
	return router.on("goal_evaluation_v1", nativeGoalEvaluationScript(script))
}

func nativeGoalEvaluationScript(script fakeProviderScript) fakeProviderScript {
	var pending []map[string]any
	started := false
	return func(payload map[string]any) fakeProviderResult {
		hasTool := false
		for _, value := range arrayValue(payload["messages"]) {
			if stringValue(mapValue(value)["role"]) == "tool" {
				hasTool = true
			}
		}
		if !hasTool {
			started = false
		}
		if !started {
			first := script(payload)
			if first.Err != nil || first.Status >= 400 || first.Structured == nil {
				return first
			}
			pending = goalNativeFixtureCalls(first.Structured)
			started = true
		}
		if len(pending) == 0 {
			return fakeProviderResult{Structured: map[string]any{"summary": "controlled native submissions finished; consult actual Tool results"}}
		}
		call := pending[0]
		pending = pending[1:]
		return fakeProviderResult{ToolCalls: []map[string]any{call}}
	}
}

func goalNativeFixtureCalls(wire map[string]any) []map[string]any {
	calls := []map[string]any{}
	appendCall := func(name string, args map[string]any) {
		calls = append(calls, map[string]any{"call_id": fmt.Sprintf("goal-native-fixture-%d", len(calls)+1), "capability_name": name, "arguments": args})
	}
	plans := map[string]map[string]any{}
	for _, value := range arrayValue(wire["plans"]) {
		p := mapValue(value)
		plans[stringValue(p["goal_ref"])] = p
	}
	for _, value := range arrayValue(wire["evaluations"]) {
		root := cloneMap(mapValue(value))
		ref := stringValue(root["goal_ref"])
		stage, commitments := mapValue(root["stage_evaluation"]), arrayValue(root["commitment_evaluations"])
		delete(root, "stage_evaluation")
		delete(root, "commitment_evaluations")
		appendObjects := func() {
			for _, value := range commitments {
				object := cloneMap(mapValue(value))
				object["goal_ref"], object["object_kind"] = ref, "commitment"
				appendCall(goalObjectSubmit, object)
			}
			if len(stage) > 0 {
				object := cloneMap(stage)
				object["goal_ref"], object["object_kind"] = ref, "stage"
				appendCall(goalObjectSubmit, object)
			}
		}
		if stringValue(root["impact"]) == "completed" {
			appendCall(goalEvaluationSubmit, root)
			appendObjects()
		} else {
			appendObjects()
			if stringValue(mapValue(root["review"])["decision"]) == "adjust" {
				if plan, ok := plans[ref]; ok {
					appendCall(goalPlanSubmit, plan)
					delete(plans, ref)
				}
			}
			appendCall(goalEvaluationSubmit, root)
		}
		if plan, ok := plans[ref]; ok {
			appendCall(goalPlanSubmit, plan)
			delete(plans, ref)
		}
	}
	for _, value := range arrayValue(wire["plans"]) {
		plan := mapValue(value)
		if _, ok := plans[stringValue(plan["goal_ref"])]; ok {
			appendCall(goalPlanSubmit, plan)
		}
	}
	return calls
}
