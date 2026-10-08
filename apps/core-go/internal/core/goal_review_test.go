package core

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestActorGoalReviewWindowUsesLocalDateAndDST(t *testing.T) {
	for _, tc := range []struct {
		at, date string
		hours    time.Duration
	}{
		{"2026-03-08T16:00:00Z", "2026-03-08", 23 * time.Hour},
		{"2026-11-01T16:00:00Z", "2026-11-01", 25 * time.Hour},
	} {
		at, err := time.Parse(time.RFC3339, tc.at)
		if err != nil {
			t.Fatal(err)
		}
		date, start, end, err := actorGoalReviewWindow(at, "America/New_York")
		if err != nil || date != tc.date || end.Sub(start) != tc.hours || start.Location() != time.UTC || end.Location() != time.UTC {
			t.Fatalf("local DST window %s %s %s %v", date, start, end, err)
		}
	}
}

func TestGoalReviewCycleReplayAndLateEvidenceRevision(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"actual expression"})
	seedCognitiveProviderRole(t, f.ctx, f.repository, "review-model-"+f.suffix)
	at := f.app.now()
	queue := func() string {
		t.Helper()
		id := ""
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			var err error
			id, err = queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "default", "test_cycle", at)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		calls++
		snapshot := readProcessingGoalSnapshot(t, f)
		entry := snapshot.Goals[0]
		if len(snapshot.Reviews) != 1 {
			t.Fatalf("review context absent %#v", snapshot.Reviews)
		}
		return fakeProviderResult{Structured: map[string]any{"evaluations": []any{map[string]any{"goal_id": goalID, "expected_revision": entry.Goal.Revision, "criteria_version": entry.Goal.CriteriaVersion, "judgments": []any{map[string]any{"criterion_id": entry.Goal.CriterionIDs[0], "verdict": "unknown", "kind": "communication", "subject": "actor_self", "discourse": "uncertain", "evidence_refs": []any{}, "reason": "没有表达证据"}}, "impact": "needs_evidence", "blocker": "", "wait_condition": "等待双方愿意讨论的时机", "next_step": "", "residual_motivation": "", "review": map[string]any{"reason_category": "no_opportunity", "decision": "wait", "explanation": "本周期无已知适当机会，继续等待，不计努力不足", "evidence_refs": []any{}, "stage_id": "", "feasible_alternative": ""}}}, "plans": []any{}}}
	})}
	id := queue()
	if id == "" {
		t.Fatal("initial review absent")
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, id); err != nil {
		t.Fatal(err)
	}
	if again := queue(); again != "" {
		t.Fatalf("same cycle produced another request %s", again)
	}
	var reviews, revision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*),max(revision) FROM public.goal_reviews WHERE goal_id=$1`, goalID).Scan(&reviews, &revision); err != nil || reviews != 1 || revision != 1 {
		t.Fatalf("cycle duplicated %d %d %v", reviews, revision, err)
	}
	messageID := "late-review-" + f.suffix
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at) VALUES($1,$2,50,$3,'user','晚到的真实对话','[]',$1,$4)`, messageID, f.conversationID, f.ownerID, at.Add(-time.Hour)); err != nil {
			return err
		}
		return f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, messageID, "")
	}); err != nil {
		t.Fatal(err)
	}
	id = latestPendingGoalRequest(t, f)
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*),max(revision) FROM public.goal_reviews WHERE goal_id=$1`, goalID).Scan(&reviews, &revision); err != nil || reviews != 1 || revision != 2 || calls != 2 {
		t.Fatalf("late revision %d %d calls=%d %v", reviews, revision, calls, err)
	}
}

func TestPendingGoalReviewLateSourceFencesOldDecision(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"actual result"})
	var requestID string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		requestID, err = queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "default", "claim_race", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	old, _, err := f.app.claimGoalEvaluation(f.ctx, requestID)
	if err != nil || len(old.Reviews) != 1 {
		t.Fatalf("claim %#v %v", old.Reviews, err)
	}
	messageID := "processing-late-" + f.suffix
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,50,$3,'user','新的真实回应','[]',$1)`, messageID, f.conversationID, f.ownerID); err != nil {
			return err
		}
		return f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, messageID, "")
	}); err != nil {
		t.Fatal(err)
	}
	err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return commitGoalReviewTx(f.ctx, tx, old.Goals[0].Goal, old.Reviews[0], &GoalReviewDecision{ReasonCategory: "no_opportunity", Decision: "wait", Explanation: "old decision"}, map[string]GoalSource{}, "", nil)
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("old review overwrote new source revision: %v", err)
	}
	var newRequest, state string
	var revision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT evaluation_request_id,status,revision FROM public.goal_reviews WHERE goal_id=$1`, goalID).Scan(&newRequest, &state, &revision); err != nil || newRequest == requestID || state != "pending" || revision != 2 {
		t.Fatalf("late source lacks durable replacement %s %s %d %v", newRequest, state, revision, err)
	}
	var intents int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id=$1`, newRequest).Scan(&intents); err != nil || intents != 1 {
		t.Fatalf("replacement not durable %d %v", intents, err)
	}
}

func TestTerminalGoalSupersedesPendingReviewWithoutProvider(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"actual result"})
	var requestID string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		requestID, err = queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "default", "terminal", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, goalID, GoalOwnerCommand{Operation: "cancel", ExpectedRevision: 1, IdempotencyKey: "cancel-before-review", Reason: "结束原目标"}); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID)
	if err != nil || result["status"] != "succeeded" {
		t.Fatalf("empty terminal assessment %#v %v", result, err)
	}
	var state string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.goal_reviews WHERE goal_id=$1`, goalID).Scan(&state); err != nil || state != "superseded" {
		t.Fatalf("terminal review suspended forever %s %v", state, err)
	}
}

func TestSoftDeadlineQueuesVisibleReviewWithoutCancellation(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"actual result"})
	past := f.app.now().Add(-time.Hour)
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, goalID, GoalOwnerCommand{Operation: "update", ExpectedRevision: 1, IdempotencyKey: "soft-expired", Reason: "软期限夹具", Deadline: &past}); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		id, err = queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "default", "soft_deadline", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, _, err := f.app.claimGoalEvaluation(f.ctx, id)
	if err != nil || len(snapshot.Reviews) != 1 || !snapshot.Reviews[0].DeadlineOverdue {
		t.Fatalf("overdue soft review absent %#v %v", snapshot.Reviews, err)
	}
	goal := snapshot.Goals[0].Goal
	if goal.Status != GoalActive {
		t.Fatal("soft deadline cancelled Goal")
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return commitGoalReviewTx(f.ctx, tx, goal, snapshot.Reviews[0], &GoalReviewDecision{ReasonCategory: "no_opportunity", Decision: "wait", Explanation: "软期限已过，暂未出现合适机会，保留原目标并复核策略"}, map[string]GoalSource{}, "下次合适机会再复核", nil)
	}); err != nil {
		t.Fatal(err)
	}
	var overdue bool
	var decision, state string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT r.deadline_overdue,r.decision,g.status FROM public.goal_reviews r JOIN public.fluctlight_goals g ON g.id=r.goal_id WHERE g.id=$1`, goalID).Scan(&overdue, &decision, &state); err != nil || !overdue || decision != "wait" || state != "active" {
		t.Fatalf("soft deadline not visible or changed lifecycle %v %s %s %v", overdue, decision, state, err)
	}
}

func TestGoalReviewSeparatesMissedOpportunityAndIneffectiveStrategyThreshold(t *testing.T) {
	for _, category := range []string{"missed_opportunity", "ineffective_attempt"} {
		t.Run(category, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			id := createDialogueGoalForClosure(t, f, []string{"real long term result"})
			at := f.app.now()
			date, start, end, err := actorGoalReviewWindow(at, "Asia/Shanghai")
			if err != nil {
				t.Fatal(err)
			}
			var goal GoalAuthority
			err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				var revision int
				if err := tx.QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&revision); err != nil {
					return err
				}
				var err error
				goal, err = loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(id), ContextReference{EntityID: id, Revision: revision})
				if err != nil {
					return err
				}
				goal, err = applyGoalPlanTx(f.ctx, tx, goal, GoalPlanCandidate{GoalID: id, ExpectedRevision: revision, CriteriaVersion: goal.CriteriaVersion, Reason: "当前策略", NextStep: "一次合适的询问", Stage: &GoalStagePlan{Operation: "create", Purpose: "了解真实期待", Strategy: "克制询问", Criteria: []string{"实际回应"}, Reason: "当前阶段"}}, "threshold-stage", at)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 2; i++ {
				oldStart := start.AddDate(0, 0, -i)
				oldEnd := oldStart.AddDate(0, 0, 1)
				if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_reviews(id,fluctlight_id,goal_id,local_date,timezone,window_start,window_end,status,reason_category,decision,explanation) VALUES($1,$2,$3,$4,'Asia/Shanghai',$5,$6,'succeeded',$7,'continue','prior reviewed cycle fixture')`, fmt.Sprintf("threshold-%s-%d", f.suffix, i), f.fluctlightID, id, oldStart.In(mustGoalReviewLocation(t, "Asia/Shanghai")).Format("2006-01-02"), oldStart, oldEnd, category); err != nil {
					t.Fatal(err)
				}
			}
			review := GoalReviewContext{ID: "threshold-current-" + f.suffix, GoalID: id, LocalDate: date, Timezone: "Asia/Shanghai", WindowStart: start, WindowEnd: end, Revision: 1}
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_reviews(id,fluctlight_id,goal_id,local_date,timezone,window_start,window_end) VALUES($1,$2,$3,$4,$5,$6,$7)`, review.ID, f.fluctlightID, id, date, review.Timezone, start, end); err != nil {
				t.Fatal(err)
			}
			messageID := "threshold-actual-" + f.suffix
			author, kind, text := f.fluctlightID, "assistant", "我已经尝试询问彼此期待，但还没有得到有效回应。"
			if category == "missed_opportunity" {
				author, kind, text = f.ownerID, "user", "今天有空，可以聊聊你的想法。"
			}
			var source GoalSource
			if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at) VALUES($1,$2,50,$3,$4,$5,'[]',$1,$6)`, messageID, f.conversationID, author, kind, text, at); err != nil {
					return err
				}
				if err := f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, messageID, "default"); err != nil {
					return err
				}
				var eventID int64
				if err := tx.QueryRow(f.ctx, `SELECT id FROM public.goal_source_events WHERE source_id=$1 AND source_kind='message'`, messageID).Scan(&eventID); err != nil {
					return err
				}
				var err error
				source, err = readGoalSourceWith(f.ctx, tx, eventID, true)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.goal_reviews WHERE id=$1`, review.ID).Scan(&review.Revision); err != nil {
				t.Fatal(err)
			}
			sources := map[string]GoalSource{source.Ref: source}
			candidate := &GoalReviewDecision{ReasonCategory: category, Decision: "continue", Explanation: "需要更明确的策略", EvidenceRefs: []string{source.Ref}, StageID: goal.CurrentStageID, FeasibleAlternative: "在该事件中直接问一个尊重对方的问题"}
			err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return commitGoalReviewTx(f.ctx, tx, goal, review, candidate, sources, "", nil) })
			expectedCode := "goal_review_concrete_next_step_required"
			if category == "ineffective_attempt" {
				expectedCode = "goal_review_strategy_change_required"
			}
			if err == nil || err.Error() != expectedCode {
				t.Fatalf("threshold should reject vague strategy with %s, got %v", expectedCode, err)
			}
			next := "明确询问彼此期待，并按实际回应重新判断"
			if category == "ineffective_attempt" {
				candidate.Decision = "adjust"
				next = "换用低压力的一次信息询问，不提高调用频率"
			}
			if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				return commitGoalReviewTx(f.ctx, tx, goal, review, candidate, sources, next, nil)
			}); err != nil {
				t.Fatal(err)
			}
			var progress []byte
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT progress FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&progress); err != nil || numberOrZero(jsonNumber(progress)) != 0 {
				t.Fatalf("review counter fabricated progress %s %v", progress, err)
			}
		})
	}
}
func mustGoalReviewLocation(t *testing.T, zone string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatal(err)
	}
	return location
}

func TestPausedGoalReviewCanAbandonWithoutResumingOrCreatingActions(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := createDialogueGoalForClosure(t, f, []string{"双方明确确认"})
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, GoalOwnerCommand{Operation: "pause", ExpectedRevision: 1, IdempotencyKey: "review-pause", Reason: "尊重明确边界"}); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "abandon-review-"+f.suffix)
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		snapshot := readProcessingGoalSnapshot(t, f)
		g := snapshot.Goals[0].Goal
		return fakeProviderResult{Structured: map[string]any{"evaluations": []any{map[string]any{"goal_id": id, "expected_revision": g.Revision, "criteria_version": g.CriteriaVersion, "judgments": []any{}, "impact": "needs_evidence", "blocker": "目标与当前明确边界不再相容", "wait_condition": "", "next_step": "", "residual_motivation": "", "review": map[string]any{"reason_category": "blocked", "decision": "abandon", "explanation": "明确结束该目标，不恢复或生成进一步行动", "evidence_refs": []any{}, "stage_id": "", "feasible_alternative": ""}}}, "plans": []any{}}}
	})}
	var requestID string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		requestID, err = queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "default", "paused_abandon", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID); err != nil {
		t.Fatal(err)
	}
	var state string
	var actions int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&state); err != nil || state != "abandoned" {
		t.Fatalf("review did not abandon paused goal %s %v", state, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_intentions WHERE goal_id=$1`, id).Scan(&actions); err != nil || actions != 0 {
		t.Fatalf("review resumed/created pressure %d %v", actions, err)
	}
}
