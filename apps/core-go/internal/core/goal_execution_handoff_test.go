package core

import (
	"testing"
	"time"
)

func TestGoalExecutionHandoffSettlementRequiresSuccessfulWake(t *testing.T) {
	for _, status := range []string{"failed", "cancelled", "sleeping", "deferred", "no_op", "completed"} {
		if got := goalExecutionHandoffSettlementEligible(status); got != (status == "no_op" || status == "completed") {
			t.Fatalf("status %s consumed=%t", status, got)
		}
	}
}

func TestGoalExecutionHandoffUsesCurrentProjectionRefsOnly(t *testing.T) {
	values := []map[string]any{{"_goal_id": "goal-a", "goal_ref": "stable-a"}, {"_goal_id": "hidden-goal", "goal_ref": "stable-hidden"}}
	index := ContextReferenceIndex{ByRef: map[string]ContextReference{"current-scoped-ref": {Kind: ContextReferenceGoal, EntityID: "goal-a"}}}
	bound := bindGoalExecutionObligations(values, index)
	if len(bound) != 1 || bound[0]["goal_ref"] != "current-scoped-ref" || values[0]["goal_ref"] != "stable-a" {
		t.Fatal("handoff exposed stale or unadmitted Goal ref", bound, values)
	}
}

func TestGoalExecutionHandoffKeepsSamePlanBudgetAndTypedWait(t *testing.T) {
	at := time.Date(2026, 10, 11, 2, 0, 0, 0, time.UTC)
	hint := map[string]any{"next_step": "do an authorized activity", "wait_condition": "model prose is not a clock"}
	marker := goalExecutionHandoff("goal-a", hint, nil, at)
	hint["execution_handoff"] = marker
	if !goalExecutionHandoffDue(hint, at) {
		t.Fatal("ordinary plan cannot be reconsidered without a user message")
	}
	marker["attempts"] = goalExecutionReconsiderations
	marker["status"] = "exhausted"
	other := cloneMap(hint)
	other["reason"] = "new evaluation explanation"
	other["execution_handoff"] = goalExecutionHandoff("goal-a", other, hint, at.Add(time.Hour))
	if goalExecutionHandoffDue(other, at.Add(time.Hour)) {
		t.Fatal("replayed plan reset exhausted budget")
	}
	if stringValue(goalExecutionHandoff("goal-b", hint, nil, at)["plan_id"]) == stringValue(marker["plan_id"]) {
		t.Fatal("different goals shared a handoff identity")
	}
	other["next_step"] = "a different approved next step"
	other["execution_handoff"] = goalExecutionHandoff("goal-a", other, hint, at)
	other["next_review_at"] = formatInstant(at.Add(time.Hour))
	if goalExecutionHandoffDue(other, at) || !goalExecutionHandoffDue(other, at.Add(time.Hour)) {
		t.Fatal("real review time ignored")
	}
	mapValue(other["execution_handoff"])["not_before"] = formatInstant(at.Add(2 * time.Hour))
	if goalExecutionHandoffDue(other, at.Add(time.Hour)) {
		t.Fatal("bounded retry backoff ignored")
	}
}

func TestGoalExecutionHandoffQualifiesCandidateWithoutDuplicatingDispatch(t *testing.T) {
	at := time.Now().UTC()
	hint := map[string]any{"next_step": "authorized action"}
	hint["execution_handoff"] = goalExecutionHandoff("g", hint, nil, at)
	for _, status := range []string{"", "candidate", "qualified", "due", "in_progress", "paused"} {
		if got := goalExecutionNeedsReconsideration(hint, status, at); got != (status == "" || status == "candidate") {
			t.Fatalf("status=%s reconsideration=%t", status, got)
		}
	}
	packet := publicGoalExecutionObligations([]map[string]any{{"_goal_id": "private-row", "goal_ref": "goal:ctx_x", "next_step": "action"}})
	if _, ok := packet[0]["_goal_id"]; ok {
		t.Fatal("private database identity exposed")
	}
}

func TestGoalEvaluationOutcomesRequireActualGoalAssociation(t *testing.T) {
	input := goalEvaluationSnapshot{Goals: []goalEvaluationGoal{{GoalID: "boots", Goal: GoalAuthority{Ref: "goal:ctx_" + stableDigest("boots")}}}}
	values := []map[string]any{
		{"status": "failed", "error_code": "unrelated_provider_failure", "goal_refs": []any{"goal:ctx_" + stableDigest("other")}},
		{"status": "failed", "error_code": "unbound"},
		{"status": "completed", "goal_refs": []any{"goal:ctx_" + stableDigest("boots")}},
	}
	got := goalEvaluationScopedOutcomes(input, values)
	if len(got) != 1 || got[0]["status"] != "completed" {
		t.Fatalf("unrelated failures attributed to goal: %#v", got)
	}
	if len(values) != 3 {
		t.Fatal("input mutated")
	}
}

func TestGoalAssessmentPreservesHandoffBudgetUntilTerminal(t *testing.T) {
	previous := map[string]any{"execution_handoff": map[string]any{"plan_id": "accepted-plan", "attempts": 2, "status": "pending"}}
	for _, status := range []GoalLifecycleStatus{GoalActive, GoalPaused, GoalCompleted, GoalCancelled, GoalAbandoned} {
		next := GoalAuthority{Status: status, ExecutionHint: map[string]any{"next_step": "updated assessment prose"}}
		preserveGoalExecutionHandoff(&next, previous)
		marker := mapValue(next.ExecutionHint["execution_handoff"])
		if goalStatusTerminal(status) {
			if len(marker) != 0 {
				t.Fatalf("terminal kept execution: %#v", marker)
			}
			continue
		}
		if marker["plan_id"] != "accepted-plan" || intValue(marker["attempts"]) != 2 {
			t.Fatalf("assessment erased or reset handoff: %#v", marker)
		}
		marker["attempts"] = 0
		if intValue(mapValue(previous["execution_handoff"])["attempts"]) != 2 {
			t.Fatal("assessment mutated prior budget")
		}
	}
}
