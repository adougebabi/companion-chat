package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
)

func plannerRecoveryRun(t *testing.T, f independentToolE2EFixture) string {
	t.Helper()
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "commit-recovery"); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 ORDER BY created_at LIMIT 1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=1,claimed_at=now(),mode='apply',profile_id='default' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	return run + "@1"
}

func plannerRecoveryRequest(t *testing.T, f independentToolE2EFixture, run, operation string, args map[string]any) ToolExecutionRequest {
	t.Helper()
	request := f.request(goalPlannerCommit, operation, args)
	request.Surface, request.TargetKind, request.TargetRef = CapabilitySurfaceGoalPlanner, "goal_planning_run", run
	return request
}

func TestGoalPlannerCommitRecoversReviewFlagWithoutClearingRealDependencies(t *testing.T) {
	for _, reviewed := range []bool{false, true} {
		t.Run(map[bool]string{false: "hold_preserved", true: "explicit_review_committed"}[reviewed], func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			goal := plannerOwnerGoal(t, f, "review-me", false)
			parent := plannerOwnerGoal(t, f, "real-prerequisite", false)
			c := plannerCommand(t, f, "seed-real-dependency")
			c.Dependencies = map[string][]string{goal: {parent}}
			if _, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c); err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET context_review_required=true WHERE id=$1`, goal); err != nil {
				t.Fatal(err)
			}
			run := plannerRecoveryRun(t, f)
			s := plannerSnapshot(t, f)
			args := map[string]any{"expected_version": s["revision"], "expected_facts_revision": s["facts_revision"], "reason": "Existing direction reviewed against unchanged constraints", "dependencies": map[string]any{goal: []string{"context_review_required"}}}
			if reviewed {
				args["reviewed_goal_ids"] = []string{goal}
			}
			request := plannerRecoveryRequest(t, f, run, "review-flag", args)
			receipt, err := f.app.ExecuteTool(f.ctx, request)
			out := mapValue(receipt.Result.Output)
			if err != nil || receipt.Result.Status != "completed" || out["commit_succeeded"] != true || len(arrayValue(out["warnings"])) == 0 {
				t.Fatalf("recoverable metadata blocked commit: %#v %v", receipt, err)
			}
			var hold bool
			var deps int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT context_review_required,(SELECT count(*) FROM public.goal_dependencies WHERE goal_id=$1 AND prerequisite_id=$2) FROM public.fluctlight_goals WHERE id=$1`, goal, parent).Scan(&hold, &deps); err != nil || hold == reviewed || deps != 1 {
				t.Fatalf("metadata widened authority or erased prerequisite: hold=%t deps=%d err=%v", hold, deps, err)
			}
			fresh := plannerSnapshot(t, f)
			if intValue(fresh["revision"]) <= intValue(s["revision"]) || intValue(fresh["revision"]) != intValue(out["revision"]) {
				t.Fatal("no durable set commit", s["revision"], fresh["revision"], out["revision"])
			}
			replay, err := f.app.ExecuteTool(f.ctx, request)
			if err != nil || !replay.Replayed || jsonString(replay.Result.Output) != jsonString(out) {
				t.Fatal("recovered receipt replay lost warnings", replay, err)
			}
		})
	}
}

func TestGoalPlannerReviewOnlyAcceptsInitialEmptyCriteria(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return f.app.insertAgency(f.ctx, tx, f.fluctlightID, f.ownerID, []any{map[string]any{"description": "保持真实的共同交流方向", "motivation": "初始化的稳定方向", "scope": "relationship"}}, nil, map[string]struct{}{"default": {}})
	}); err != nil {
		t.Fatal(err)
	}
	goal := "goal_initial_" + f.fluctlightID + "_0"
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET context_review_required=true WHERE id=$1`, goal); err != nil {
		t.Fatal(err)
	}
	run := plannerRecoveryRun(t, f)
	s := plannerSnapshot(t, f)
	request := plannerRecoveryRequest(t, f, run, "empty-criteria-review", map[string]any{"reason": "The existing intent remains valid; criteria still require genuine planning", "expected_version": s["revision"], "expected_facts_revision": s["facts_revision"], "reviewed_goal_ids": []string{goal}})
	receipt, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || receipt.Result.Status != "completed" || mapValue(receipt.Result.Output)["commit_succeeded"] != true {
		t.Fatal("empty legacy criteria blocked context review", receipt, err)
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT to_jsonb(g) FROM public.fluctlight_goals g WHERE id=$1`, goal).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	g := decodeObject(raw)
	if g["context_review_required"] != false || g["needs_reflection"] != true || len(arrayValue(g["success_criteria"])) != 0 || g["status"] != "active" || intValue(g["revision"]) != 1 {
		t.Fatal("review changed initial goal meaning", g)
	}
}

func TestGoalPlannerCommitInvalidOrderHasActionableReceiptAndRollsBackReview(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goal := plannerOwnerGoal(t, f, "review-order", false)
	plannerOwnerGoal(t, f, "other-active", false)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET context_review_required=true WHERE id=$1`, goal); err != nil {
		t.Fatal(err)
	}
	run := plannerRecoveryRun(t, f)
	s := plannerSnapshot(t, f)
	request := plannerRecoveryRequest(t, f, run, "incomplete-order", map[string]any{"reason": "review", "expected_version": s["revision"], "expected_facts_revision": s["facts_revision"], "reviewed_goal_ids": []string{goal}, "order": []string{goal}})
	receipt, err := f.app.ExecuteTool(f.ctx, request)
	requireHandledToolNoEffect(t, receipt, err, "goal_set_order_incomplete")
	if stringValue(mapValue(receipt.Result.Output)["detail"]) == "" {
		t.Fatal("needs_input lost correction", receipt)
	}
	var hold bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT context_review_required FROM public.fluctlight_goals WHERE id=$1`, goal).Scan(&hold); err != nil || !hold || intValue(plannerSnapshot(t, f)["revision"]) != intValue(s["revision"]) {
		t.Fatal("handled invalid order committed review", hold, err)
	}
	trace := &ADKCapabilityTrace{}
	trace.AppendResult(receipt.Result)
	invoker := &appADKCapabilityInvoker{app: f.app, trace: trace, request: ADKCapabilityRequest{FluctlightID: f.fluctlightID, SourceFactID: run, CorrelationID: "handled-planner", Surface: CapabilitySurfaceGoalPlanner}}
	invoker.recordADKToolDiagnostic(f.ctx, "adk.tool.result", receipt.Result.CallID, goalPlannerCommit, "handled", "", jsonString(request.Arguments))
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload FROM public.diagnostic_events WHERE fluctlight_id=$1 AND correlation_id='handled-planner' AND event_type='adk.tool.result'`, f.fluctlightID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if out := decodeObject(raw); out["warning_code"] != "goal_set_order_incomplete" || out["disposition"] != "needs_input" || out["effect_applied"] != false || out["mutation_committed"] != false {
		t.Fatal("handled diagnostics lost the actual cause", out)
	}
}

func TestGoalPlannerCommitForeignDependencyStillRollsBack(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goal := plannerOwnerGoal(t, f, "review-foreign", false)
	run := plannerRecoveryRun(t, f)
	s := plannerSnapshot(t, f)
	request := plannerRecoveryRequest(t, f, run, "foreign-dependency", map[string]any{"reason": "review", "expected_version": s["revision"], "expected_facts_revision": s["facts_revision"], "reviewed_goal_ids": []string{goal}, "dependencies": map[string]any{goal: []string{"context_review_required", "goal-foreign"}}})
	receipt, err := f.app.ExecuteTool(f.ctx, request)
	if !errors.Is(err, ErrUnauthorized) || receipt.Result.Status == "completed" || intValue(plannerSnapshot(t, f)["revision"]) != intValue(s["revision"]) {
		t.Fatal("metadata repair bypassed resource authority", receipt, err)
	}
}

func TestGoalDependencyDatabaseFailureRetainsItsCause(t *testing.T) {
	err := validateGoalDependencyTx(context.Background(), &plannerDependencyErrorTx{}, "owner", "default", "goal", "parent")
	if errors.Is(err, ErrUnauthorized) || !errors.Is(err, errPlannerTestDatabaseFailure) {
		t.Fatal("database failure mislabeled authorization", err)
	}
}

var errPlannerTestDatabaseFailure = errors.New("controlled database unavailable")

type plannerDependencyErrorTx struct{ pgx.Tx }
type plannerDependencyErrorRow struct{}

func (*plannerDependencyErrorTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return plannerDependencyErrorRow{}
}
func (plannerDependencyErrorRow) Scan(...any) error { return errPlannerTestDatabaseFailure }

// Keep JSON import tied to the actual model schema boundary, not a mock DTO.
func TestGoalPlannerReviewSchemaPreservesReviewOnlyArguments(t *testing.T) {
	args := json.RawMessage(`{"reason":"existing intent remains valid","reviewed_goal_ids":["goal-in-scope"],"dependencies":{"goal-in-scope":["context_review_required"]}}`)
	got, err := normalizeNativeModelArguments(args, (goalPlannerCapability{name: goalPlannerCommit}).Definition())
	if err != nil || len(arrayValue(decodeObject(got)["reviewed_goal_ids"])) != 1 {
		t.Fatal(string(got), err)
	}
}

func TestGoalPlannerMetadataBindsInitialRequestSnapshot(t *testing.T) {
	for _, scenario := range []string{"initial", "later_consumed_query", "same_batch_query", "explicit_stale", "missing_identity"} {
		t.Run(scenario, func(t *testing.T) {
			trace := &ADKCapabilityTrace{}
			if scenario != "missing_identity" {
				trace.RecordModelToolCalls("commit-request", 2, []schema.ToolCall{{ID: "commit"}})
			}
			if scenario == "later_consumed_query" || scenario == "same_batch_query" {
				seq := uint64(1)
				if scenario == "same_batch_query" {
					seq = 2
				}
				trace.RecordModelToolCalls("snapshot-request", seq, []schema.ToolCall{{ID: "snapshot"}})
				trace.AppendInvocation(CapabilityInvocation{CallID: "snapshot", CapabilityName: goalPlannerQuery, Arguments: json.RawMessage(`{"section":"snapshot"}`)})
				trace.AppendResult(CapabilityResult{CallID: "snapshot", CapabilityName: goalPlannerQuery, Status: "completed", Output: map[string]any{"revision": 5, "facts_revision": "facts_gen_38"}})
			}
			ctx := WithADKCapabilityInvoker(context.Background(), &goalRequestPolicyInvoker{}, trace)
			initial := map[string]any{"revision": 4, "facts_revision": "facts_gen_37"}
			ctx = withGoalPlannerInitialSnapshot(ctx, initial)
			initial["revision"] = 900 // Binder stores immutable fence values.
			args := map[string]any{"reason": "existing intent remains valid", "reviewed_goal_ids": []string{"goal-in-scope"}}
			if scenario == "explicit_stale" {
				args["expected_version"], args["expected_facts_revision"] = 1, "facts_gen_1"
			}
			before := jsonString(args)
			got := bindGoalPlannerMetadata(ctx, CapabilityInvocation{CallID: "commit", Metadata: InvocationMetadata{OperationID: "trusted-op"}}, args)
			version, facts := 4, "facts_gen_37"
			if scenario == "later_consumed_query" {
				version, facts = 5, "facts_gen_38"
			} else if scenario == "explicit_stale" {
				version, facts = 1, "facts_gen_1"
			} else if scenario == "missing_identity" {
				version, facts = 0, ""
			}
			if intValue(got["expected_version"]) != version || stringValue(got["expected_facts_revision"]) != facts || jsonString(args) != before {
				t.Fatal("unconsumed/rebound fence or mutated input", got)
			}
		})
	}
}

func TestGoalPlannerDependencyRepairDoesNotInferReview(t *testing.T) {
	c := GoalSetCommand{Dependencies: map[string][]string{"goal": {"context_review_required", "parent"}, "flag-only": {"context_review_required"}, "explicit-clear": {}, "context_review_required": {"goal"}}}
	before := jsonString(c)
	got, warnings := normalizeGoalPlannerDependencies(c)
	if len(warnings) != 1 || len(got.ReviewedGoalIDs) != 0 || jsonString(c) != before || len(got.Dependencies) != 2 || len(got.Dependencies["goal"]) != 1 || got.Dependencies["goal"][0] != "parent" {
		t.Fatal("metadata repair inferred review or lost real intent", got, warnings)
	}
	if parents, ok := got.Dependencies["explicit-clear"]; !ok || len(parents) != 0 {
		t.Fatal("intentional dependency clear lost", got)
	}
}

func TestGoalPlannerHandledInputIsNotSummarizedAsPolicyBlock(t *testing.T) {
	inv := CapabilityInvocation{CallID: "commit", CapabilityName: goalPlannerCommit}
	handled := handledCapabilityResult(inv, "goal_set_order_incomplete", "needs_input", nil)
	result := map[string]any{"decision": "blocked_by_policy", "reason": "model inferred policy", "review_condition": "authorize resource"}
	if !reconcileGoalPlannerHandledSummary(result, 0, []CapabilityResult{handled}) || result["decision"] != "awaiting_facts" || stringValue(result["review_condition"]) == "authorize resource" {
		t.Fatal("input recovery remained a policy block", result)
	}
	warnings := goalPlannerToolWarnings([]CapabilityResult{handled})
	if len(warnings) != 1 || mapValue(warnings[0])["warning_code"] != "goal_set_order_incomplete" {
		t.Fatal(warnings)
	}
	queryFailure := failedCapabilityResult(CapabilityInvocation{CallID: "query", CapabilityName: goalPlannerQuery}, "actual_database_failure", false)
	for _, receipts := range [][]CapabilityResult{{handled, failedCapabilityResult(inv, "actual_policy_refusal", false)}, {goalToolResult(inv, map[string]any{"commit_succeeded": true})}, {handled, queryFailure}, {queryFailure, handled}} {
		original := map[string]any{"decision": "blocked_by_policy"}
		if reconcileGoalPlannerHandledSummary(original, 0, receipts) || original["decision"] != "blocked_by_policy" {
			t.Fatal("real refusal or commit relabeled", original)
		}
	}
	failed := map[string]any{"decision": "failed", "reason": "actual database failure"}
	if reconcileGoalPlannerHandledSummary(failed, 0, []CapabilityResult{handled, queryFailure}) || failed["decision"] != "failed" || failed["reason"] != "actual database failure" {
		t.Fatal("later query failure was hidden", failed)
	}
	if reconcileGoalPlannerHandledSummary(result, 1, []CapabilityResult{handled}) {
		t.Fatal("committed run became no-effect")
	}
}

func TestGoalPlannerNativeReviewCommitsWithOmittedTechnicalFields(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goal := plannerOwnerGoal(t, f, "native-review", false)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET context_review_required=true WHERE id=$1`, goal); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "recovery-model-"+f.suffix)
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "native-recovery"); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 LIMIT 1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET available_at=now()-interval '1 second' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	calls := 0
	var commitOutput map[string]any
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_planner_v1", func(payload map[string]any) fakeProviderResult {
		calls++
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] != "tool" {
				continue
			}
			result := decodeObject([]byte(stringValue(message["content"])))
			out := mapValue(result["output"])
			if len(out) == 0 {
				out = mapValue(mapValue(result["result"])["output"])
			}
			if out["commit_succeeded"] == true {
				commitOutput = out
			}
		}
		name, args := goalPlannerQuery, map[string]any{"section": "goal", "goal_id": goal}
		switch calls {
		case 1:
		case 2:
			name, args = goalPlannerCommit, map[string]any{"reason": "Actual goal details preserve the existing valid direction", "reviewed_goal_ids": []string{goal}, "dependencies": map[string]any{goal: []string{"context_review_required"}}}
		case 3:
			if commitOutput == nil {
				t.Fatal("native recovery did not actually commit", payload["messages"])
			}
			args = map[string]any{"section": "snapshot"}
		default:
			return fakeProviderResult{Structured: map[string]any{"decision": "applied", "reason": "Existing goal review committed and reread", "review_condition": "Actual evidence still required", "suggestions": []any{}}}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprint("review-call-", calls), name, args)}}
	})}
	result, err := f.app.ProcessGoalPlanningIntent(f.ctx, run)
	if err != nil || result["decision"] != "applied" || calls != 4 || len(arrayValue(result["tool_warnings"])) != 1 {
		t.Fatal("native review did not complete", result, err, calls)
	}
	var hold bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT context_review_required FROM public.fluctlight_goals WHERE id=$1`, goal).Scan(&hold); err != nil || hold {
		t.Fatal("model summary differed from durable effect", hold, err)
	}
	var diag []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload FROM public.diagnostic_events WHERE fluctlight_id=$1 AND event_type='adk.tool.result' AND payload->>'capability'=$2 ORDER BY created_at DESC LIMIT 1`, f.fluctlightID, goalPlannerCommit).Scan(&diag); err != nil || len(arrayValue(decodeObject(diag)["warnings"])) != 1 {
		t.Fatal("recovery warning missing from Tool diagnostic", string(diag), err)
	}
	if replay, err := f.app.ProcessGoalPlanningIntent(f.ctx, run); err != nil || replay["decision"] != "applied" || calls != 4 {
		t.Fatal("recovery replay called model again", replay, err, calls)
	}
}
