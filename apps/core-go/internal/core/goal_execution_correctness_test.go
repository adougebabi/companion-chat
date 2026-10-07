package core

import (
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGoalStandardRevisionCannotMixWithEvaluation(t *testing.T) {
	for _, candidate := range []ReflectionGoalCandidateV2{
		{Operation: "complete", SuccessCriteria: []string{"brush", "painting"}},
		{Operation: "update", SuccessCriteria: []string{"painting"}, OutcomeRefs: []string{"outcome:x"}},
		{Operation: "update", DesiredOutcome: "different meaning", Complete: true},
	} {
		if err := validateGoalCandidateSeparation(candidate); err == nil {
			t.Fatalf("mixed request accepted: %#v", candidate)
		}
	}
	if err := validateGoalCandidateSeparation(ReflectionGoalCandidateV2{Operation: "update", SuccessCriteria: []string{"brush", "painting"}}); err != nil {
		t.Fatal(err)
	}
}

func TestGoalStandardRevisionResetsOldProgress(t *testing.T) {
	goal := GoalAuthority{SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + strings.Repeat("a", 32), FluctlightID: "fl", DesiredOutcome: "obtain brush", SuccessCriteria: []string{"brush acquired"}, CriteriaVersion: 1, Motivation: "paint", Scope: "general", Progress: 1, Status: GoalActive, Revision: 3, EvidenceRefs: []string{"source:brush"}}
	next, _, err := ApplyGoalCommand(&goal, GoalCommand{Operation: GoalUpdate, ExpectedRevision: 3, Patch: GoalPatch{SuccessCriteria: []string{"brush acquired", "painting finished"}}, EvidenceRefs: []string{"owner:refine"}, OccurredAt: time.Now().UTC()})
	if err != nil || next.CriteriaVersion != 2 || next.Progress != 0 || !next.NeedsReflection {
		t.Fatalf("next=%#v err=%v", next, err)
	}
	goal.Status = GoalCompleted
	if _, _, err := ApplyGoalCommand(&goal, GoalCommand{Operation: GoalUpdate, ExpectedRevision: 3, Patch: GoalPatch{SuccessCriteria: []string{"new"}}, EvidenceRefs: []string{"owner:refine"}, OccurredAt: time.Now().UTC()}); err == nil {
		t.Fatal("terminal goal silently revised")
	}
}

func TestDueSettlementSeparatesQuerySynchronousAndAsynchronous(t *testing.T) {
	registry := mustCapabilityRegistry(lifeActivityStartCapability{}, lifeActivityAdvanceCapability{}, wardrobeInspectCapability{})
	for _, tc := range []struct {
		name    string
		results []CapabilityResult
		status  string
	}{
		{"no action", nil, "suppressed"},
		{"query", []CapabilityResult{{CapabilityName: wardrobeInspectCapabilityName, Status: "completed"}}, "suppressed"},
		{"synchronous action", []CapabilityResult{{CapabilityName: lifeActivityAdvanceCapabilityName, Status: "completed"}}, "completed"},
		{"async accepted", []CapabilityResult{{CapabilityName: lifeActivityStartCapabilityName, Status: "accepted", Output: map[string]any{"activity_id": "activity-1"}}}, "pending"},
		{"async lacks operation", []CapabilityResult{{CapabilityName: lifeActivityStartCapabilityName, Status: "accepted"}}, "suppressed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := dueActionSettlement(tc.results, registry)
			if status != tc.status {
				t.Fatalf("status=%s want=%s", status, tc.status)
			}
		})
	}
}

func seedNativeDueForGoalExecution(t *testing.T) (independentToolE2EFixture, string, map[string]any) {
	t.Helper()
	fixture := seedWardrobeToolFixture(t)
	intentionID := createQualifiedBootIntention(t, fixture, "execution-correctness")
	if err := withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
		_, err := appendProcessedCognitionFactTx(fixture.ctx, tx, fixture.fluctlightID, "life.event.created", map[string]any{"summary": "opportunity"}, "execution-trigger-"+fixture.suffix)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	due, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID)
	if err != nil || stringValue(due["status"]) != "due" {
		t.Fatalf("due=%#v err=%v", due, err)
	}
	return fixture, intentionID, due
}

func TestDueProviderFailureBeforeFirstToolSettlesAndBacksOff(t *testing.T) {
	fixture, intentionID, due := seedNativeDueForGoalExecution(t)
	seedCognitiveProviderRole(t, fixture.ctx, fixture.repository, "pretool-failure-"+fixture.suffix)
	router := newFakeProviderRouter().on("native_cognition_response", func(_ map[string]any) fakeProviderResult {
		var state string
		if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, due["attempt_id"]).Scan(&state); err != nil || state != "running" {
			t.Fatalf("Provider I/O before durable Attempt: %s %v", state, err)
		}
		return fakeProviderResult{Status: 500}
	})
	fixture.app.Provider.HTTP = &http.Client{Transport: router}
	if _, err := fixture.app.ProcessCognitionInbox(fixture.ctx, stringValue(due["inbox_id"])); err == nil {
		t.Fatal("Provider failure unexpectedly succeeded")
	}
	var intentionState, attemptState, inboxState string
	var retryAt time.Time
	var retryCount int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT i.status,a.status,c.status,i.next_attempt_at,i.retry_count FROM public.fluctlight_intentions i JOIN public.fluctlight_intention_attempts a ON a.attempt_id=i.current_attempt_id JOIN public.cognition_inbox c ON c.id=$2 WHERE i.id=$1`, intentionID, due["inbox_id"]).Scan(&intentionState, &attemptState, &inboxState, &retryAt, &retryCount); err != nil {
		t.Fatal(err)
	}
	if intentionState != "qualified" || attemptState != "failed" || inboxState != "failed" || retryCount != 1 || !retryAt.After(fixture.app.now()) {
		t.Fatalf("failure recovery: intention=%s attempt=%s inbox=%s retry=%d at=%s", intentionState, attemptState, inboxState, retryCount, retryAt)
	}
	pending, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID)
	if err != nil || stringValue(pending["reason_code"]) != "intention_retry_not_due" {
		t.Fatalf("hot retry: %#v %v", pending, err)
	}
	// Rebuild the App to prove authority resides in PostgreSQL, not memory.
	restarted := *fixture.app
	restarted.Clock = fixedClock(retryAt.Add(time.Second))
	if err := withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
		_, err := appendProcessedCognitionFactTx(fixture.ctx, tx, fixture.fluctlightID, "life.event.created", map[string]any{"summary": "new opportunity"}, "restarted-trigger-"+fixture.suffix)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	next, err := restarted.ProcessIntentionTrigger(fixture.ctx, intentionID)
	if err != nil || stringValue(next["status"]) != "due" || stringValue(next["attempt_id"]) == stringValue(due["attempt_id"]) {
		t.Fatalf("restarted retry=%#v %v", next, err)
	}
	var effects int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&effects); err != nil || effects != 0 {
		t.Fatalf("tool-before-failure fabricated effect: %d %v", effects, err)
	}
}

func changeGoalForExecutionTest(t *testing.T, fixture independentToolE2EFixture, intentionID string, operation GoalLifecycleOperation) {
	t.Helper()
	err := withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
		var id string
		var revision int
		if err := lockLifeContextTx(fixture.ctx, tx, fixture.fluctlightID); err != nil {
			return err
		}
		if err := tx.QueryRow(fixture.ctx, `SELECT g.id,g.revision FROM public.fluctlight_goals g JOIN public.fluctlight_intentions i ON i.goal_id=g.id WHERE i.id=$1`, intentionID).Scan(&id, &revision); err != nil {
			return err
		}
		current, err := loadGoalAuthorityTx(fixture.ctx, tx, fixture.fluctlightID, "goal:ctx_"+stableDigest(id), ContextReference{EntityID: id, Revision: revision})
		if err != nil {
			return err
		}
		next, record, err := ApplyGoalCommand(&current, GoalCommand{Operation: operation, ExpectedRevision: revision, EvidenceRefs: []string{"owner:govern"}, OccurredAt: fixture.app.now()})
		if err != nil {
			return err
		}
		_, err = persistGoalAuthorityTx(fixture.ctx, tx, &current, next, record, "test-goal-"+string(operation)+fixture.suffix)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGoalPauseAndCancelPreventOldDueAndDirectActivity(t *testing.T) {
	for _, operation := range []GoalLifecycleOperation{GoalPause, GoalCancel} {
		t.Run(string(operation), func(t *testing.T) {
			fixture, intentionID, _ := seedNativeDueForGoalExecution(t)
			changeGoalForExecutionTest(t, fixture, intentionID, operation)
			trigger, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID)
			if err != nil || stringValue(trigger["status"]) == "due" || stringValue(trigger["status"]) == "activity_started" {
				t.Fatalf("old trigger executed: %#v %v", trigger, err)
			}
			receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "old-start", map[string]any{"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "black boots", "reason": "old due"}))
			if err == nil || receipt.Result.Status == "accepted" {
				t.Fatalf("Goal %s allowed action: %#v %v", operation, receipt, err)
			}
			var effects int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&effects); err != nil || effects != 0 {
				t.Fatalf("effects=%d %v", effects, err)
			}
		})
	}
}

func TestGoalResumeExpiresHeldIntentionWindow(t *testing.T) {
	fixture, intentionID, _ := seedNativeDueForGoalExecution(t)
	changeGoalForExecutionTest(t, fixture, intentionID, GoalPause)
	var expiration time.Time
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT expiration FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&expiration); err != nil {
		t.Fatal(err)
	}
	fixture.app.Clock = fixedClock(expiration.Add(time.Second))
	changeGoalForExecutionTest(t, fixture, intentionID, GoalResume)
	var state string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&state); err != nil || state != "expired" {
		t.Fatalf("expired window resumed: %s %v", state, err)
	}
}

func TestExpiredAttemptLeaseRecoversWithoutReplayingAgent(t *testing.T) {
	fixture, intentionID, due := seedNativeDueForGoalExecution(t)
	if stopped, err := fixture.app.prepareNativeDueAttempt(fixture.ctx, stringValue(due["inbox_id"]), fixture.fluctlightID); err != nil || stopped {
		t.Fatalf("claim failed: %v %v", stopped, err)
	}
	fixture.app.Clock = fixedClock(fixture.app.now().Add(6 * time.Minute))
	if count, err := fixture.app.ReconcileGoalAttempts(fixture.ctx, 10); err != nil || count != 1 {
		t.Fatalf("recovery=%d %v", count, err)
	}
	var state, attempt string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT i.status,a.status FROM public.fluctlight_intentions i JOIN public.fluctlight_intention_attempts a ON a.attempt_id=i.current_attempt_id WHERE i.id=$1`, intentionID).Scan(&state, &attempt); err != nil || state != "qualified" || attempt != "failed" {
		t.Fatalf("crash recovery: %s %s %v", state, attempt, err)
	}
	if count, err := fixture.app.ReconcileGoalAttempts(fixture.ctx, 10); err != nil || count != 0 {
		t.Fatalf("recovery replay=%d %v", count, err)
	}
}

func TestAsyncMissingCallbackKeepsOriginalOperationForReconciliation(t *testing.T) {
	fixture, intentionID, due, activityID, _ := startDueShoppingThroughNative(t)
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_intention_attempts SET deadline=$2 WHERE attempt_id=$1`, due["attempt_id"], fixture.app.now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if count, err := fixture.app.ReconcileGoalAttempts(fixture.ctx, 10); err != nil || count != 1 {
		t.Fatalf("reconcile=%d %v", count, err)
	}
	var state, waitRef, reason string
	var runs, intents int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status,wait_ref,result->>'recovery_reason' FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, due["attempt_id"]).Scan(&state, &waitRef, &reason); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id=$1`, "intention_result_intent:"+activityID).Scan(&intents); err != nil {
		t.Fatal(err)
	}
	if state != "waiting" || waitRef != activityID || reason != "operation_state_requires_reconciliation" || runs != 1 || intents != 1 {
		t.Fatalf("unknown side effect retried/lost: %s %s %s runs=%d intents=%d", state, waitRef, reason, runs, intents)
	}
}

func TestGoalCancelDuringProviderPreventsNativeToolSideEffect(t *testing.T) {
	fixture, intentionID, due := seedNativeDueForGoalExecution(t)
	seedCognitiveProviderRole(t, fixture.ctx, fixture.repository, "race-goal-"+fixture.suffix)
	calls := 0
	router := newFakeProviderRouter().on("native_cognition_response", func(_ map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			changeGoalForExecutionTest(t, fixture, intentionID, GoalCancel)
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("old-goal-start", lifeActivityStartCapabilityName, map[string]any{"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "black boots", "reason": "old plan"})}}
		}
		return fakeProviderResult{Status: 500}
	})
	fixture.app.Provider.HTTP = &http.Client{Transport: router}
	_, _ = fixture.app.ProcessCognitionInbox(fixture.ctx, stringValue(due["inbox_id"]))
	var effects int
	var goalState string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&effects); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT g.status FROM public.fluctlight_goals g JOIN public.fluctlight_intentions i ON i.goal_id=g.id WHERE i.id=$1`, intentionID).Scan(&goalState); err != nil {
		t.Fatal(err)
	}
	if effects != 0 || goalState != "cancelled" {
		t.Fatalf("Goal cancel raced through Tool: effects=%d state=%s", effects, goalState)
	}
}

func TestGoalProgressRejectsStaleVersionsAndRepeatedPoints(t *testing.T) {
	now := time.Now().UTC()
	goal := GoalAuthority{SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + strings.Repeat("a", 32), FluctlightID: "fl", DesiredOutcome: "confirmed relationship", SuccessCriteria: []string{"expression", "explicit acceptance"}, CriteriaVersion: 2, Motivation: "relationship", Scope: "relationship", Status: GoalActive, Revision: 4, EvidenceRefs: []string{"source:goal"}}
	ref := "outcome:ctx_" + strings.Repeat("b", 32)
	outcomes := map[string]ActionOutcome{ref: {Status: ActionOutcomeCompleted, GoalRefs: []string{goal.Ref}}}
	proposal := GoalProgressProposal{ExpectedRevision: 3, CriteriaVersion: 1, GoalRef: goal.Ref, OutcomeRefs: []string{ref}, CriterionIDs: goalCriteriaAtIndexes(goal, []int{0}), Strength: 1, Confidence: 1, OccurredAt: now}
	if _, _, err := ApplyGoalProgress(goal, proposal, outcomes); err == nil {
		t.Fatal("stale standard evaluation applied")
	}
	proposal.ExpectedRevision = 4
	proposal.CriteriaVersion = 2
	next, _, err := ApplyGoalProgress(goal, proposal, outcomes)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		proposal.ExpectedRevision = next.Revision
		next, _, err = ApplyGoalProgress(next, proposal, outcomes)
		if err != nil {
			t.Fatal(err)
		}
	}
	if next.Progress != 0.5 || next.Status != GoalActive {
		t.Fatalf("repeated expression manufactured relationship progress: %#v", next)
	}
}

func TestCriterionIdentitySurvivesReorderAndCopyEdit(t *testing.T) {
	goal, _, err := CreateGoalAuthority(GoalAuthority{EntityID: "goal-identity", SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + strings.Repeat("a", 32), FluctlightID: "fl", DesiredOutcome: "two independently checked conditions", SuccessCriteria: []string{"express intention", "confirm acceptance"}, Motivation: "relationship", Scope: "relationship", Status: GoalActive, Revision: 1}, []string{"owner:goal"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	first := goalCriterionIDs(goal)
	next, _, err := ApplyGoalCommand(&goal, GoalCommand{Operation: GoalUpdate, ExpectedRevision: 1, Patch: GoalPatch{SuccessCriteria: []string{"confirm acceptance", "express intention"}}, EvidenceRefs: []string{"owner:reorder"}, OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if next.CriterionIDs[0] != first[1] || next.CriterionIDs[1] != first[0] {
		t.Fatalf("reorder changed identity: %#v %#v", first, next.CriterionIDs)
	}
	edited, _, err := ApplyGoalCommand(&next, GoalCommand{Operation: GoalUpdate, ExpectedRevision: next.Revision, Patch: GoalPatch{SuccessCriteria: []string{"explicitly confirm acceptance", "express intention"}}, EvidenceRefs: []string{"owner:clarify"}, OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if edited.CriterionIDs[0] != next.CriterionIDs[0] || edited.CriteriaVersion != 3 {
		t.Fatalf("copy edit lost identity/version: %#v", edited)
	}
	duplicate := goal
	duplicate.CriterionIDs = nil
	duplicate.SuccessCriteria = []string{"confirm acceptance", "confirm acceptance"}
	ids := goalCriterionIDs(duplicate)
	if ids[0] == ids[1] {
		t.Fatal("two standards shared one identity")
	}
}

func TestDirectTimeIntentionCannotStartBeforeTrigger(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	intentionID := createQualifiedBootIntention(t, fixture, "future-direct")
	future := fixture.app.now().Add(24 * time.Hour)
	if err := withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
		current, err := loadIntentionAuthorityByIDTx(fixture.ctx, tx, intentionID)
		if err != nil {
			return err
		}
		trigger := TypedIntentionTrigger{Type: IntentionTriggerTime, DueAt: &future}
		next, record, err := ApplyIntentionCommand(current, IntentionCommand{Operation: IntentionUpdate, ExpectedRevision: current.Revision, Patch: IntentionPatch{Trigger: &trigger}, EvidenceRefs: current.EvidenceRefs, Reason: "future time", OccurredAt: fixture.app.now()})
		if err != nil {
			return err
		}
		_, err = persistIntentionAuthorityTx(fixture.ctx, tx, &current, next, record, "future-direct:"+fixture.suffix)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "too-early", map[string]any{"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "black boots", "reason": "not due"}))
	if err == nil || receipt.Result.Status == "accepted" {
		t.Fatalf("time trigger bypassed: %#v %v", receipt, err)
	}
	var attempts int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE intention_id=$1`, intentionID).Scan(&attempts); err != nil || attempts != 0 {
		t.Fatalf("early attempt created: %d %v", attempts, err)
	}
}

func TestHardDeadlineClosesDueWithoutPermanentPending(t *testing.T) {
	fixture, intentionID, due := seedNativeDueForGoalExecution(t)
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_goals SET deadline_policy='hard',deadline=$2 WHERE id=(SELECT goal_id FROM public.fluctlight_intentions WHERE id=$1)`, intentionID, fixture.app.now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if stopped, err := fixture.app.prepareNativeDueAttempt(fixture.ctx, stringValue(due["inbox_id"]), fixture.fluctlightID); err != nil || !stopped {
		t.Fatalf("expired Goal not stopped: %v %v", stopped, err)
	}
	var state string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&state); err != nil || state != "expired" {
		t.Fatalf("permanent due: %s %v", state, err)
	}
	result, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID)
	if err != nil || stringValue(result["status"]) != "expired" {
		t.Fatalf("hard deadline kept polling: %#v %v", result, err)
	}
}

func TestUnknownOperationReconciliationIsBoundedAndCannotResume(t *testing.T) {
	fixture, intentionID, due, _, _ := startDueShoppingThroughNative(t)
	for i := 0; i < 6; i++ {
		if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_intention_attempts SET deadline=$2 WHERE attempt_id=$1`, due["attempt_id"], fixture.app.now().Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.app.ReconcileGoalAttempts(fixture.ctx, 10); err != nil {
			t.Fatal(err)
		}
	}
	var state string
	var deadline *time.Time
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status,deadline FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, due["attempt_id"]).Scan(&state, &deadline); err != nil || state != "needs_reconciliation" || deadline != nil {
		t.Fatalf("unbounded reconciliation: %s %v %v", state, deadline, err)
	}
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "unsafe-resume", map[string]any{"operation": "resume", "intention_id": intentionID, "reason": "unknown result"})); err == nil {
		t.Fatal("unknown side effect was retried")
	}
}

func TestGoalCancellationSerializesWithWaitingToolTransaction(t *testing.T) {
	fixture, intentionID, _ := seedNativeDueForGoalExecution(t)
	tx, err := fixture.repository.Pool().Begin(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(fixture.ctx)
	if err := lockLifeContextTx(fixture.ctx, tx, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	var goalID string
	var revision int
	if err := tx.QueryRow(fixture.ctx, `SELECT g.id,g.revision FROM public.fluctlight_goals g JOIN public.fluctlight_intentions i ON i.goal_id=g.id WHERE i.id=$1`, intentionID).Scan(&goalID, &revision); err != nil {
		t.Fatal(err)
	}
	current, err := loadGoalAuthorityTx(fixture.ctx, tx, fixture.fluctlightID, "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: goalID, Revision: revision})
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan ToolExecutionReceipt, 1)
	go func() {
		receipt, _ := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "competing-start", map[string]any{"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "black boots", "reason": "concurrent start"}))
		finished <- receipt
	}()
	until := time.Now().Add(3 * time.Second)
	blocked := false
	for time.Now().Before(until) {
		if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory')`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("Tool did not reach the transaction admission barrier")
	}
	next, record, err := ApplyGoalCommand(&current, GoalCommand{Operation: GoalCancel, ExpectedRevision: revision, EvidenceRefs: []string{"owner:cancel"}, OccurredAt: fixture.app.now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persistGoalAuthorityTx(fixture.ctx, tx, &current, next, record, "concurrent-goal-cancel:"+fixture.suffix); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(fixture.ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case receipt := <-finished:
		if receipt.Result.Status == "accepted" {
			t.Fatal("waiting Tool started after Goal cancellation")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("admission deadlocked")
	}
	var effects int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&effects); err != nil || effects != 0 {
		t.Fatalf("cancel/start effects=%d err=%v", effects, err)
	}
}

// Database recovery fixture: the media authority/owned ready asset has committed,
// while its callback has not. This is not real media-provider quality evidence.
func TestReconcileReadsReadyMediaAuthorityWithoutReissuingOperation(t *testing.T) {
	fixture, intentionID, due := seedNativeDueForGoalExecution(t)
	inboxID := stringValue(due["inbox_id"])
	if _, err := fixture.app.prepareNativeDueAttempt(fixture.ctx, inboxID, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT result FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, due["attempt_id"]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	frozen := decodeObject(raw)
	mediaID, assetID := "reconcile-media-"+fixture.suffix, "reconcile-asset-"+fixture.suffix
	providerID := "reconcile-provider-" + fixture.suffix
	if err := withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(fixture.ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES($1,$2,'image','image/png','{}',$3,$4,'completed')`, mediaID, fixture.fluctlightID, providerID, "reconcile-workflow-"+fixture.suffix); err != nil {
			return err
		}
		if _, err := tx.Exec(fixture.ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) VALUES($1,$2,'v1','image','image/png',1,$3,'test-only','test-only',$4,$5,'ready',now())`, assetID, fixture.fluctlightID, strings.Repeat("a", 64), providerID, "reconcile-workflow-"+fixture.suffix); err != nil {
			return err
		}
		result := CapabilityResult{CallID: "actual-media-callback-fixture", CapabilityName: "media.image.generate", Status: "accepted", Output: map[string]any{"media_intent_id": mediaID}}
		outcomes, err := buildActionOutcomes("agent_native_"+stableDigest(inboxID), fixture.fluctlightID, inboxID, "capability", []CapabilityResult{result}, map[string]any{"status": "pending", "goal_refs": []string{stringValue(frozen["goal_ref"])}, "intention_refs": []string{stringValue(frozen["intention_ref"])}, "context_references": frozen["context_references"]}, fixture.app.capabilityRegistry(), fixture.app.now())
		if err != nil {
			return err
		}
		return persistActionOutcomesTx(fixture.ctx, tx, outcomes)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_intention_attempts SET deadline=$2 WHERE attempt_id=$1`, due["attempt_id"], fixture.app.now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if count, err := fixture.app.ReconcileGoalAttempts(fixture.ctx, 10); err != nil || count != 1 {
		t.Fatalf("media reconciliation=%d %v", count, err)
	}
	var attempt, intention, goal string
	var progress float64
	var intents int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT a.status,i.status,g.status,g.progress FROM public.fluctlight_intention_attempts a JOIN public.fluctlight_intentions i ON i.current_attempt_id=a.attempt_id JOIN public.fluctlight_goals g ON g.id=i.goal_id WHERE i.id=$1`, intentionID).Scan(&attempt, &intention, &goal, &progress); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.media_intents WHERE id=$1`, mediaID).Scan(&intents); err != nil {
		t.Fatal(err)
	}
	if attempt != "succeeded" || intention != "completed" || goal != "active" || progress != 0 || intents != 1 {
		t.Fatalf("operation/Goal conflated or reissued: %s %s %s progress=%v intents=%d", attempt, intention, goal, progress, intents)
	}
}
