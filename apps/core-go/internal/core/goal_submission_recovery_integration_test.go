package core

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// The fixture owns a unique disposable database. These tests deliberately skip
// when GO_CORE_TEST_DATABASE_URL is absent; HTTP/helper tests run independently.
func TestGoalSubmissionRecoveryPersistenceHandledCycleAndReplay(t *testing.T) {
	for _, mode := range []string{"replay_plan", "bad_criterion", "replay_plan_ignores_tools", "bad_criterion_ignores_tools"} {
		t.Run(mode, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			first := createDialogueGoalForClosure(t, f, []string{"实际表达"})
			other := f
			other.suffix += "_unresolved"
			second := createDialogueGoalForClosure(t, other, []string{"实际表达"})
			message := insertGoalBoundaryMessage(t, f)
			seedCognitiveProviderRole(t, f.ctx, f.repository, "recovery-"+f.suffix)
			calls := 0
			var args map[string]any
			var originalDigest string
			f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(payload map[string]any) fakeProviderResult {
				calls++
				if calls > 7 {
					t.Fatal("native loop exceeded bounded physical budget")
				}
				if len(arrayValue(payload["tools"])) == 0 && !strings.HasSuffix(mode, "_ignores_tools") {
					return fakeProviderResult{Structured: map[string]any{"summary": "accepted sibling; unresolved root awaits evidence"}}
				}
				snapshot := readProcessingGoalSnapshot(t, f)
				if calls == 1 {
					return fakeProviderResult{ToolCalls: []map[string]any{{"call_id": "accepted-sibling", "capability_name": goalEvaluationSubmit, "arguments": nativeGoalRootFixture(t, snapshot, first, message)}}}
				}
				binding, _ := newGoalEvaluationWireBinding(snapshot)
				name := goalEvaluationSubmit
				if strings.HasPrefix(mode, "replay_plan") {
					name = goalPlanSubmit
					args = map[string]any{"goal_ref": binding.goalRefsByID[second], "reason": fmt.Sprint("changed-", calls), "next_step": "", "wait_condition": "wait for actual evidence"}
					if calls == 2 {
						originalDigest = stableDigest(jsonString(args))
					}
				} else {
					args = recoveryRoot(binding.goalRefsByID[second])
					args["judgments"] = []any{map[string]any{"criterion_ref": "criterion:missing", "verdict": "unknown", "kind": "semantic", "subject": "actor_self", "discourse": "uncertain", "evidence_refs": []string{}, "reason": "unknown", "criterion_quote": "", "optional_improvement": ""}}
				}
				return fakeProviderResult{ToolCalls: []map[string]any{{"call_id": fmt.Sprint("unresolved-", calls), "capability_name": name, "arguments": args}}}
			})}
			request := latestPendingGoalRequest(t, f)
			result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, request)
			if err != nil || result["disposition"] != "awaiting_evidence" || result["assessment_complete"] != false || len(arrayValue(result["evaluated_goals"])) != 1 || len(arrayValue(result["unresolved_goals"])) != 1 {
				t.Fatal(result, err)
			}
			before := calls
			replay, err := f.app.ProcessGoalEvaluationIntent(f.ctx, request)
			if err != nil || calls != before || jsonString(result) != jsonString(replay) {
				t.Fatal("handled workflow reopened", calls, before, replay, err)
			}
			records := goalSubmissionRecords(result)
			if _, ok := records[goalSubmissionKey(goalEvaluationSubmit, second, "")]; ok {
				t.Fatal("fabricated root accepted")
			}
			if strings.HasPrefix(mode, "replay_plan") && records[goalSubmissionKey(goalPlanSubmit, second, "")].Digest != originalDigest {
				t.Fatal("changed payload overwrote accepted digest", records)
			}
			var count int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evaluations WHERE goal_id=$1`, second).Scan(&count); err != nil || count != 0 {
				t.Fatal("unresolved judgment mutated", count, err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_source_events WHERE source_id=$1 AND processed_at IS NOT NULL`, message).Scan(&count); err != nil || count != 0 {
				t.Fatal("pending source consumed", count, err)
			}
			var raw []byte
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT result FROM public.goal_evaluation_requests WHERE id=$1`, request).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "goal_submission_conflict") {
				t.Fatal("target replay still conflicts")
			}
		})
	}
}

func TestGoalSubmissionRecoveryPersistenceUnknownNoChange(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goal := createDialogueGoalForClosure(t, f, []string{"实际表达"})
	seedCognitiveProviderRole(t, f.ctx, f.repository, "unknown-"+f.suffix)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(payload map[string]any) fakeProviderResult {
		calls++
		if calls > 2 {
			t.Fatal("all unknown required model correction")
		}
		if calls == 1 {
			snapshot := readProcessingGoalSnapshot(t, f)
			binding, _ := newGoalEvaluationWireBinding(snapshot)
			return fakeProviderResult{ToolCalls: []map[string]any{{"call_id": "unknown-root", "capability_name": goalEvaluationSubmit, "arguments": recoveryRoot(binding.goalRefsByID[goal])}}}
		}
		return fakeProviderResult{Structured: map[string]any{"summary": "awaiting evidence; goal remains active"}}
	})}
	result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, latestPendingGoalRequest(t, f))
	if err != nil || result["assessment_complete"] != true || calls != 2 {
		t.Fatal(result, calls, err)
	}
	var impact, status, wait string
	var progress float64
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT e.impact,g.status,g.progress,e.wait_condition FROM public.goal_evaluations e JOIN public.fluctlight_goals g ON g.id=e.goal_id WHERE g.id=$1`, goal).Scan(&impact, &status, &progress, &wait); err != nil || impact != "needs_evidence" || status != "active" || progress != 0 || wait == "" {
		t.Fatal(impact, status, progress, wait, err)
	}
}
