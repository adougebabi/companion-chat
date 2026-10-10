package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func assertGoalEvaluationRequestPhase(t *testing.T, payload map[string]any, submissionsRequired bool) {
	t.Helper()
	_, hasFormat := payload["response_format"]
	if submissionsRequired {
		if payload["tool_choice"] != "required" || hasFormat {
			t.Fatalf("uncovered production request allowed summary: tool_choice=%#v response_format=%t", payload["tool_choice"], hasFormat)
		}
		return
	}
	if payload["tool_choice"] != "auto" || !hasFormat {
		t.Fatalf("covered production request did not restore summary: tool_choice=%#v response_format=%t", payload["tool_choice"], hasFormat)
	}
}

func TestGoalEvaluationProductionRequestPolicyReadsDurableRootCoverage(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	createDialogueGoalForClosure(t, f, []string{"actual expression"})
	requestID := latestPendingGoalRequest(t, f)
	snapshot, prior, err := f.app.claimGoalEvaluation(f.ctx, requestID)
	if err != nil || prior != nil {
		t.Fatal(prior, err)
	}
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	policy := goalEvaluationPhysicalRequestPolicy{app: f.app, session: &goalEvaluationSession{binding: binding, runID: requestID}}
	decision, err := policy.DecidePhysicalModelRequest(f.ctx)
	if err != nil || string(decision.ToolChoice) != "forced" || !decision.OmitResponseFormat {
		t.Fatalf("uncovered roots were not forced: decision=%#v err=%v", decision, err)
	}

	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT result FROM public.goal_evaluation_requests WHERE id=$1`, requestID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	result := decodeObject(raw)
	records := goalSubmissionRecords(result)
	goalID := snapshot.Goals[0].GoalID
	records[goalSubmissionKey(goalObjectSubmit, goalID, "stage:any")] = goalSubmissionRecord{Digest: "object-only"}
	result["submissions"] = records
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET result=$2 WHERE id=$1`, requestID, jsonBytes(result)); err != nil {
		t.Fatal(err)
	}
	decision, err = policy.DecidePhysicalModelRequest(f.ctx)
	if err != nil || string(decision.ToolChoice) != "forced" {
		t.Fatalf("object submission incorrectly covered root: decision=%#v err=%v", decision, err)
	}
	trace := &ADKCapabilityTrace{}
	trace.AppendResult(CapabilityResult{CapabilityName: goalEvaluationSubmit, Status: "failed", ErrorCode: "goal_submission_authority_stale"})
	staleCtx := WithADKCapabilityInvoker(f.ctx, &goalRequestPolicyInvoker{}, trace)
	decision, err = policy.DecidePhysicalModelRequest(staleCtx)
	if err != nil || string(decision.ToolChoice) != "allowed" || decision.OmitResponseFormat {
		t.Fatalf("frozen stale rejection could not exit for replacement: decision=%#v err=%v", decision, err)
	}

	for _, entry := range snapshot.Goals {
		records[goalSubmissionKey(goalEvaluationSubmit, entry.GoalID, "")] = goalSubmissionRecord{Digest: "durable-root-" + entry.GoalID, Output: map[string]any{"status": "completed"}}
	}
	result["submissions"] = records
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET result=$2 WHERE id=$1`, requestID, jsonBytes(result)); err != nil {
		t.Fatal(err)
	}
	decision, err = policy.DecidePhysicalModelRequest(f.ctx)
	if err != nil || string(decision.ToolChoice) != "allowed" || decision.OmitResponseFormat {
		t.Fatalf("durable root coverage did not restore final schema: decision=%#v err=%v", decision, err)
	}

	canceled, cancel := context.WithCancel(f.ctx)
	cancel()
	if _, err := policy.DecidePhysicalModelRequest(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was normalized: %v", err)
	}
	staleSnapshot := snapshot
	staleSnapshot.ClaimRevision++
	staleBinding, err := newGoalEvaluationWireBinding(staleSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	stale := goalEvaluationPhysicalRequestPolicy{app: f.app, session: &goalEvaluationSession{binding: staleBinding}}
	if _, err := stale.DecidePhysicalModelRequest(f.ctx); err == nil || !strings.Contains(err.Error(), "goal_evaluation_claim_stale") {
		t.Fatalf("stale claim read durable coverage: %v", err)
	}
}

func nativeGoalRootFixture(t *testing.T, snapshot goalEvaluationSnapshot, goalID, messageID string) map[string]any {
	t.Helper()
	var entry goalEvaluationGoal
	for _, v := range snapshot.Goals {
		if v.GoalID == goalID {
			entry = v
		}
	}
	proof := ""
	for _, s := range snapshot.Sources {
		if s.ID == messageID {
			proof = s.Ref
		}
	}
	if proof == "" || entry.GoalID == "" {
		t.Fatal("missing actual Goal/message")
	}
	wire := goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "completed", NextStep: "目标完成，后续反馈无需重开目标", Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "real sent message"}}}}})
	return mapValue(arrayValue(wire["evaluations"])[0])
}

func TestGoalEvaluationNativePersistenceSurvivesRejectedStageAndReplaysOnce(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	first := createDialogueGoalForClosure(t, f, []string{"实际明确发送表达"})
	other := f
	other.suffix += "_second"
	second := createDialogueGoalForClosure(t, other, []string{"实际明确发送表达"})
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		g, err := loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(first), ContextReference{EntityID: first, Revision: 1})
		if err != nil {
			return err
		}
		_, err = applyGoalPlanTx(f.ctx, tx, g, GoalPlanCandidate{GoalID: first, ExpectedRevision: 1, CriteriaVersion: g.CriteriaVersion, Reason: "current stage", NextStep: "read actual information", Stage: &GoalStagePlan{Operation: "create", Purpose: "confirm information", Strategy: "use real user evidence", Criteria: []string{"用户偏好信息"}, Reason: "current step"}}, "native-stage-seed", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	message := insertGoalBoundaryMessage(t, f)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "native-goal-provider-"+f.suffix)
	var roots []map[string]any
	physical := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(payload map[string]any) fakeProviderResult {
		physical++
		assertGoalEvaluationRequestPhase(t, payload, physical <= 3)
		if physical == 1 {
			snapshot := readProcessingGoalSnapshot(t, f)
			roots = []map[string]any{nativeGoalRootFixture(t, snapshot, first, message), nativeGoalRootFixture(t, snapshot, second, message)}
			binding, err := newGoalEvaluationWireBinding(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			ref := binding.goalRefsByID[first]
			entry := binding.goalsByRef[ref]
			proof := stringValue(arrayValue(mapValue(arrayValue(roots[0]["judgments"])[0])["evidence_refs"])[0])
			stage := entry.Stages[0]
			args := map[string]any{"goal_ref": ref, "object_kind": "stage", "object_ref": binding.stageRefsByID[stage.ID], "completed": true, "reason": "invalid self-report", "judgments": []any{map[string]any{"criterion_ref": binding.criterionRefs["stage\x1f"+stage.ID+"\x1f"+stage.Criteria[0].ID], "criterion_quote": "", "optional_improvement": "", "verdict": "satisfied", "kind": "information", "subject": "actor_self", "discourse": "assertion", "evidence_refs": []string{proof}, "reason": "assistant report"}}}
			return fakeProviderResult{ToolCalls: []map[string]any{{"call_id": "bad-stage", "capability_name": goalObjectSubmit, "arguments": args}}}
		}
		if physical == 2 {
			// The following physical decision must receive real failed feedback.
			if !strings.Contains(jsonString(payload["messages"]), "goal_judgment_self_report_not_business_fact") {
				t.Fatal("no native rejection feedback")
			}
		}
		if physical >= 2 && physical <= 4 {
			index := physical - 2
			if index == 2 {
				index = 0
			}
			return fakeProviderResult{ToolCalls: []map[string]any{{"call_id": []string{"root-one", "root-two", "root-one-replay"}[physical-2], "capability_name": goalEvaluationSubmit, "arguments": roots[index]}}}
		}
		return fakeProviderResult{Structured: map[string]any{"summary": "completed roots; invalid Stage was rejected"}}
	})}
	request := latestPendingGoalRequest(t, f)
	result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(arrayValue(result["goal_outcomes"])) != 2 || physical != 5 {
		t.Fatal(result, physical)
	}
	for _, id := range []string{first, second} {
		var status string
		var count int
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&status); err != nil || status != "completed" {
			t.Fatal(id, status, err)
		}
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evaluations WHERE goal_id=$1 AND stage_id IS NULL AND commitment_id IS NULL`, id).Scan(&count); err != nil || count != 1 {
			t.Fatal("replayed root twice", id, count, err)
		}
	}
	var failures int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name=$2 AND result->>'error_code'='goal_judgment_self_report_not_business_fact'`, f.fluctlightID, goalObjectSubmit).Scan(&failures); err != nil || failures != 1 {
		t.Fatal("rejection receipt lost", failures, err)
	}
}

func TestGoalEvaluationNativePartialRetryPreservesFrozenRefsAndSuccessfulMemo(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	first := createDialogueGoalForClosure(t, f, []string{"实际表达"})
	other := f
	other.suffix += "_other"
	second := createDialogueGoalForClosure(t, other, []string{"实际表达"})
	message := insertGoalBoundaryMessage(t, f)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "native-partial-"+f.suffix)
	physical := 0
	retry := false
	var frozenSecondRef string
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(payload map[string]any) fakeProviderResult {
		physical++
		assertGoalEvaluationRequestPhase(t, payload, physical <= 3)
		if !retry && physical == 2 {
			return fakeProviderResult{Err: errors.New("controlled failure after accepted root")}
		}
		if !retry || physical == 3 {
			snapshot := readProcessingGoalSnapshot(t, f)
			root := nativeGoalRootFixture(t, snapshot, first, message)
			secondRoot := nativeGoalRootFixture(t, snapshot, second, message)
			if !retry {
				frozenSecondRef = stringValue(secondRoot["goal_ref"])
			} else {
				root = secondRoot
				if stringValue(root["goal_ref"]) != frozenSecondRef || !strings.Contains(jsonString(payload["messages"]), "accepted_submissions") {
					t.Fatal("retry renumbered refs or lost accepted state")
				}
			}
			return fakeProviderResult{ToolCalls: []map[string]any{{"call_id": "root-native", "capability_name": goalEvaluationSubmit, "arguments": root}}}
		}
		return fakeProviderResult{Structured: map[string]any{"summary": "retry submitted remaining root"}}
	})}
	request := latestPendingGoalRequest(t, f)
	if result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, request); err == nil || len(arrayValue(result["goal_outcomes"])) != 1 {
		t.Fatal("partial effect lost", result, err)
	}
	var memos, processed int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT jsonb_array_length(result->'assessment_memos') FROM public.goal_evaluation_requests WHERE id=$1`, request).Scan(&memos); err != nil || memos != 1 {
		t.Fatal(memos, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_source_events WHERE source_id=$1 AND processed_at IS NOT NULL`, message).Scan(&processed); err != nil || processed != 0 {
		t.Fatal("unresolved sources consumed", processed, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET available_at=now() WHERE id=$1`, request); err != nil {
		t.Fatal(err)
	}
	retry = true
	if result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, request); err != nil || len(arrayValue(result["goal_outcomes"])) != 2 {
		t.Fatal(result, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evaluations WHERE goal_id=$1`, first).Scan(&count); err != nil || count != 1 {
		t.Fatal("first root applied again", count, err)
	}
}

func TestGoalEvaluationSummaryWithoutNativeSubmissionCannotComplete(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goal := createDialogueGoalForClosure(t, f, []string{"实际表达"})
	insertGoalBoundaryMessage(t, f)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "summary-only-"+f.suffix)
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(payload map[string]any) fakeProviderResult {
		assertGoalEvaluationRequestPhase(t, payload, true)
		return fakeProviderResult{Structured: map[string]any{"summary": "目标完成，等待对方主动反馈读后感"}}
	})}
	_, err := f.app.ProcessGoalEvaluationIntent(f.ctx, latestPendingGoalRequest(t, f))
	if err == nil || err.Error() != "goal_evaluation_native_submission_missing" {
		t.Fatal(err)
	}
	var status string
	var revision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,revision FROM public.fluctlight_goals WHERE id=$1`, goal).Scan(&status, &revision); err != nil || status != "active" || revision != 1 {
		t.Fatal("prose mutated state", status, revision, err)
	}
}

func TestGoalEvaluationStaleReplacementMigratesDeferredReviews(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	first := createDialogueGoalForClosure(t, f, []string{"actual expression"})
	other := f
	other.suffix += "_deferred"
	deferred := createDialogueGoalForClosure(t, other, []string{"another expression"})
	var request string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		request, err = queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "default", "stale-review-test", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, prior, err := f.app.claimGoalEvaluation(f.ctx, request)
	if err != nil || prior != nil {
		t.Fatal(prior, err)
	}
	snapshot.DeferredGoalIDs = []string{deferred}
	result, err := f.app.replaceStaleGoalEvaluation(f.ctx, snapshot, map[string]any{"status": "partial", "unresolved_goals": []string{first}})
	if err != nil {
		t.Fatal(err)
	}
	replacement := stringValue(result["replacement_request_id"])
	if replacement == "" || replacement == request {
		t.Fatal("no fresh request", result)
	}
	var reviews int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_reviews WHERE evaluation_request_id=$1 AND status='pending' AND goal_id=ANY($2::text[])`, replacement, []string{first, deferred}).Scan(&reviews); err != nil || reviews != 2 {
		t.Fatal("deferred reviews lost or superseded", reviews, err)
	}
}
