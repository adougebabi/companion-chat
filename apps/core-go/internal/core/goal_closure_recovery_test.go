package core

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestGoalEvaluationClaimFencesLateResultsAndRetainsBatchRemainder(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	for i := 0; i < 8; i++ {
		id := randomID("batch-goal-")
		goal, record, err := CreateGoalAuthority(GoalAuthority{EntityID: id, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(id), FluctlightID: f.fluctlightID, DesiredOutcome: "distinct result " + id, SuccessCriteria: []string{"verified result"}, Motivation: "owner request", Scope: "general", Status: GoalCandidate, Revision: 1}, []string{"owner:goal"}, f.app.now())
		if err != nil {
			t.Fatal(err)
		}
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { _, err := persistGoalAuthorityTx(f.ctx, tx, nil, goal, record, id); return err }); err != nil {
			t.Fatal(err)
		}
	}
	seedLegacyGoalActivityForTest(t, f)
	id := latestPendingGoalRequest(t, f)
	snapshot, prior, err := f.app.claimGoalEvaluation(f.ctx, id)
	if err != nil || prior != nil || len(snapshot.Goals) != 6 || len(snapshot.DeferredGoalIDs) != 2 {
		t.Fatalf("claim: goals=%d remaining=%d prior=%#v err=%v", len(snapshot.Goals), len(snapshot.DeferredGoalIDs), prior, err)
	}
	if _, prior, err := f.app.claimGoalEvaluation(f.ctx, id); err != nil || stringValue(prior["status"]) != "deferred" {
		t.Fatalf("double claim: %#v %v", prior, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET claimed_at=now()-interval '6 minutes' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	newer, prior, err := f.app.claimGoalEvaluation(f.ctx, id)
	if err != nil || prior != nil || newer.ClaimRevision != snapshot.ClaimRevision+1 {
		t.Fatalf("lease reclaim: %#v %v", prior, err)
	}
	_ = f.app.failGoalEvaluation(f.ctx, id, snapshot.ClaimRevision, errors.New("late old failure"))
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.goal_evaluation_requests WHERE id=$1`, id).Scan(&status); err != nil || status != "processing" {
		t.Fatalf("late failure crossed fence: %s %v", status, err)
	}
	_ = f.app.failGoalEvaluation(f.ctx, id, newer.ClaimRevision, errors.New("provider unavailable"))
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.goal_evaluation_requests WHERE id=$1`, id).Scan(&status); err != nil || status != "retry" {
		t.Fatalf("failure lost retry state: %s %v", status, err)
	}
	if _, prior, err := f.app.claimGoalEvaluation(f.ctx, id); err != nil || stringValue(prior["status"]) != "deferred" {
		t.Fatalf("backoff bypassed: %#v %v", prior, err)
	}
}

func TestGoalSourceWithdrawalFlagsEndedGoalWithoutModel(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"actual expression"})
	messageID := "source-message-" + f.suffix
	var sourceID int64
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,50,$3,'assistant','a real sent expression','[]',$1)`, messageID, f.conversationID, f.fluctlightID); err != nil {
			return err
		}
		if err := f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, messageID, "default"); err != nil {
			return err
		}
		if err := tx.QueryRow(f.ctx, `SELECT id FROM public.goal_source_events WHERE source_id=$1 AND source_kind='message'`, messageID).Scan(&sourceID); err != nil {
			return err
		}
		if _, err := tx.Exec(f.ctx, `INSERT INTO public.goal_evidence_links(id,fluctlight_id,goal_id,source_event_id,status,reason) VALUES($1,$2,$3,$4,'confirmed','verified source fixture')`, "evidence-"+f.suffix, f.fluctlightID, goalID, sourceID); err != nil {
			return err
		}
		_, err := tx.Exec(f.ctx, `UPDATE public.fluctlight_goals SET status='completed',progress='1' WHERE id=$1`, goalID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.conversation_messages SET text='corrected historical source' WHERE id=$1`, messageID); err != nil {
		t.Fatal(err)
	}
	var flags, withdrawals int
	var state, link string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evidence_review_flags WHERE goal_id=$1`, goalID).Scan(&flags); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_source_events WHERE source_id=$1 AND source_status='withdrawn'`, messageID).Scan(&withdrawals); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.goal_evidence_links WHERE goal_id=$1`, goalID).Scan(&link); err != nil {
		t.Fatal(err)
	}
	if flags != 1 || withdrawals != 1 || state != "completed" || link != "withdrawn" {
		t.Fatalf("withdrawal rewrote history or needed model: flags=%d withdrawn=%d Goal=%s evidence=%s", flags, withdrawals, state, link)
	}
}

func TestGoalObjectPlanCrossDayWindowAndParentLifecycle(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"long result"})
	var goal GoalAuthority
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var revision int
		if err := tx.QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&revision); err != nil {
			return err
		}
		var err error
		goal, err = loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: goalID, Revision: revision})
		if err != nil {
			return err
		}
		start, end := f.app.now().Add(time.Hour), f.app.now().Add(48*time.Hour)
		plan := GoalPlanCandidate{GoalID: goalID, ExpectedRevision: revision, CriteriaVersion: goal.CriteriaVersion, Reason: "one current phase", WaitCondition: "wait for opportunity", Stage: &GoalStagePlan{Operation: "create", Purpose: "learn the response", Strategy: "one suitable question", Criteria: []string{"actual response"}, Reason: "appropriate phase"}, Commitment: &GoalCommitmentPlan{ExpectedResult: "get one response", Criteria: []string{"actual response"}, WindowStart: &start, WindowEnd: &end, OpportunityCondition: "counterpart available"}}
		goal, err = applyGoalPlanTx(f.ctx, tx, goal, plan, "plan-fixture", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	stages, commitments, err := readGoalObjectsWith(f.ctx, f.repository.Pool(), f.fluctlightID, goalID)
	if err != nil || len(stages) != 1 || len(commitments) != 1 || !commitments[0].WindowEnd.After(f.app.now().Add(24*time.Hour)) {
		t.Fatalf("cross-day plan lost: %#v %v", commitments, err)
	}
	intent := IntentionAuthority{EntityID: "unused", FluctlightID: f.fluctlightID, GoalEntityID: goalID, ProfileID: "", StageID: stages[0].ID, CommitmentID: commitments[0].ID}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return requireGoalCommitmentWindowTx(f.ctx, tx, intent, f.app.now()) }); err == nil {
		t.Fatal("future commitment started early")
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		next, record, err := ApplyGoalCommand(&goal, GoalCommand{Operation: GoalPause, ExpectedRevision: goal.Revision, EvidenceRefs: goal.EvidenceRefs, OccurredAt: f.app.now()})
		if err != nil {
			return err
		}
		_, err = persistGoalAuthorityTx(f.ctx, tx, &goal, next, record, "object-parent-pause")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	stages, commitments, err = readGoalObjectsWith(f.ctx, f.repository.Pool(), f.fluctlightID, goalID)
	if err != nil || stages[0].Status != "paused" || commitments[0].Status != "paused" {
		t.Fatalf("parent pause lost objects: %#v %#v %v", stages, commitments, err)
	}
	var revisions int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_object_revisions WHERE goal_id=$1`, goalID).Scan(&revisions); err != nil || revisions < 4 {
		t.Fatalf("object history missing: %d %v", revisions, err)
	}
}

func TestEmptyGoalEvaluationRejectsExpiredClaim(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"actual result"})
	id := latestPendingGoalRequest(t, f)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET status='cancelled' WHERE id=$1`, goalID); err != nil {
		t.Fatal(err)
	}
	old, _, err := f.app.claimGoalEvaluation(f.ctx, id)
	if err != nil || len(old.Goals) != 0 {
		t.Fatalf("empty claim: %#v %v", old, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET claimed_at=now()-interval '6 minutes' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	live, _, err := f.app.claimGoalEvaluation(f.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.settleEmptyGoalEvaluation(f.ctx, old); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale empty result crossed claim fence: %v", err)
	}
	result, err := f.app.settleEmptyGoalEvaluation(f.ctx, live)
	if err != nil || result["reason"] != "no_active_scoped_goal" {
		t.Fatalf("live empty settlement: %#v %v", result, err)
	}
	var status string
	var claimed *time.Time
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,claimed_at FROM public.goal_evaluation_requests WHERE id=$1`, id).Scan(&status, &claimed); err != nil || status != "succeeded" || claimed != nil {
		t.Fatalf("empty lease retained: %s %v %v", status, claimed, err)
	}
}

func TestGoalResumeRechecksCommitmentWindowAndBlocker(t *testing.T) {
	for _, tc := range []struct {
		name, blocker, want string
		expired             bool
	}{
		{"expired", "", "expired", true},
		{"blocked", "counterpart unavailable", "blocked", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			goalID := createDialogueGoalForClosure(t, f, []string{"long result"})
			at := f.app.now()
			end := at.Add(time.Hour)
			err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				var revision int
				if err := tx.QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&revision); err != nil {
					return err
				}
				goal, err := loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: goalID, Revision: revision})
				if err != nil {
					return err
				}
				goal, err = applyGoalPlanTx(f.ctx, tx, goal, GoalPlanCandidate{GoalID: goalID, ExpectedRevision: revision, CriteriaVersion: goal.CriteriaVersion, Reason: "focused next step", WaitCondition: "appropriate opportunity", Stage: &GoalStagePlan{Operation: "create", Purpose: "learn", Strategy: "ask once", Criteria: []string{"response"}, Reason: "current stage"}, Commitment: &GoalCommitmentPlan{ExpectedResult: "response", Criteria: []string{"response"}, WindowEnd: &end, Blocker: tc.blocker}}, "resume-plan", at)
				if err != nil {
					return err
				}
				for _, operation := range []GoalLifecycleOperation{GoalPause, GoalResume} {
					when := at
					if operation == GoalResume && tc.expired {
						when = end.Add(time.Second)
					}
					next, record, err := ApplyGoalCommand(&goal, GoalCommand{Operation: operation, ExpectedRevision: goal.Revision, EvidenceRefs: goal.EvidenceRefs, OccurredAt: when})
					if err != nil {
						return err
					}
					if _, err = persistGoalAuthorityTx(f.ctx, tx, &goal, next, record, "resume-"+string(operation)); err != nil {
						return err
					}
					goal = next
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			stages, commitments, err := readGoalObjectsWith(f.ctx, f.repository.Pool(), f.fluctlightID, goalID)
			if err != nil || len(stages) != 1 || len(commitments) != 1 {
				t.Fatalf("objects: %#v %#v %v", stages, commitments, err)
			}
			if stages[0].Status != "active" || commitments[0].Status != tc.want || commitments[0].Blocker != tc.blocker {
				t.Fatalf("resume lost window/blocker: %#v %#v", stages, commitments)
			}
		})
	}
}

func TestInvalidOptionalFollowupDoesNotRollbackOriginalCompletion(t *testing.T) {
	for _, name := range []string{"missing_motivation", "duplicate", "invalid_definition"} {
		t.Run(name, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			goalID := createDialogueGoalForClosure(t, f, []string{"actual expression"})
			messageID := "followup-source-" + f.suffix
			if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,50,$3,'assistant','我清楚表达自己的心意','[]',$1)`, messageID, f.conversationID, f.fluctlightID); err != nil {
					return err
				}
				return f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, messageID, "default")
			}); err != nil {
				t.Fatal(err)
			}
			requestID := latestPendingGoalRequest(t, f)
			snapshot, _, err := f.app.claimGoalEvaluation(f.ctx, requestID)
			if err != nil {
				t.Fatal(err)
			}
			goal := snapshot.Goals[0].Goal
			sources := map[string]GoalSource{}
			proof := ""
			for _, source := range snapshot.Sources {
				sources[source.Ref] = source
				if source.ID == messageID {
					proof = source.Ref
				}
			}
			if proof == "" {
				t.Fatal("actual message not journaled")
			}
			followup := &GoalFollowupCandidate{DesiredOutcome: "不同的后续目标", SuccessCriteria: []string{"另一个实际结果"}, Motivation: "尚未解决的动机"}
			residual := "尚未解决的动机"
			switch name {
			case "missing_motivation":
				residual = ""
			case "duplicate":
				followup.DesiredOutcome = goal.DesiredOutcome
			case "invalid_definition":
				followup.SuccessCriteria = nil
			}
			candidate := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: goal.Revision, CriteriaVersion: goal.CriteriaVersion, Impact: "completed", ResidualMotivation: residual, Followup: followup, Judgments: []GoalCriterionJudgment{{CriterionID: goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "actual sent expression"}}}
			if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				_, err := f.app.commitGoalEvaluationTx(f.ctx, tx, goal, candidate, sources, requestID)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			var state, disposition string
			var followupID *string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT g.status,r.disposition,r.followup_goal_id FROM public.fluctlight_goals g JOIN public.goal_resolutions r ON r.goal_id=g.id WHERE g.id=$1`, goalID).Scan(&state, &disposition, &followupID); err != nil {
				t.Fatal(err)
			}
			if state != "completed" || disposition != "planner_review_requested" || followupID != nil {
				t.Fatalf("optional candidate blocked completion: %s %s %v", state, disposition, followupID)
			}
		})
	}
}

func TestGoalEvaluationBatchRemainderRetainsActualSource(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	for i := 0; i < 8; i++ {
		id := randomID("source-batch-goal-")
		goal, record, err := CreateGoalAuthority(GoalAuthority{EntityID: id, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(id), FluctlightID: f.fluctlightID, DesiredOutcome: "record actual expression " + id, SuccessCriteria: []string{"actual expression"}, Motivation: "owner request", Scope: "general", Status: GoalCandidate, Revision: 1}, []string{"owner:goal"}, f.app.now())
		if err != nil {
			t.Fatal(err)
		}
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { _, err := persistGoalAuthorityTx(f.ctx, tx, nil, goal, record, id); return err }); err != nil {
			t.Fatal(err)
		}
	}
	seedLegacyGoalActivityForTest(t, f)
	messageID := "batch-expression-" + f.suffix
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,50,$3,'assistant','实际表达','[]',$1)`, messageID, f.conversationID, f.fluctlightID); err != nil {
			return err
		}
		return f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, messageID, "default")
	}); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "batch-provider-"+f.suffix)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().onGoalEvaluation(func(_ map[string]any) fakeProviderResult {
		calls++
		snapshot := readProcessingGoalSnapshot(t, f)
		proof := ""
		for _, source := range snapshot.Sources {
			if source.ID == messageID && source.Valid {
				proof = source.Ref
			}
		}
		if proof == "" {
			t.Fatal("deferred batch lost real source")
		}
		candidates := []GoalEvaluationCandidate{}
		for _, entry := range snapshot.Goals {
			candidates = append(candidates, GoalEvaluationCandidate{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "completed", Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "real source remains available across batches"}}})
		}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: candidates, Plans: []GoalPlanCandidate{}})}
	})}
	for i := 0; i < 2; i++ {
		id := latestPendingGoalRequest(t, f)
		if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	var completed, resolutions, unprocessed int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='completed'`, f.fluctlightID).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_resolutions WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&resolutions); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_source_events WHERE source_id=$1 AND processed_at IS NULL`, messageID).Scan(&unprocessed); err != nil {
		t.Fatal(err)
	}
	if completed != 8 || resolutions != 8 || unprocessed != 0 || calls != 2 {
		t.Fatalf("batch closure completed=%d resolutions=%d unprocessed=%d calls=%d", completed, resolutions, unprocessed, calls)
	}
}

func TestDirectParentEvidenceCompletesGoalAndClosesRedundantPlans(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := createDialogueGoalForClosure(t, f, []string{"实际明确表达心意"})
	var goal GoalAuthority
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var revision int
		if err := tx.QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&revision); err != nil {
			return err
		}
		var err error
		goal, err = loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(id), ContextReference{EntityID: id, Revision: revision})
		if err != nil {
			return err
		}
		end := f.app.now().Add(48 * time.Hour)
		goal, err = applyGoalPlanTx(f.ctx, tx, goal, GoalPlanCandidate{GoalID: id, ExpectedRevision: revision, CriteriaVersion: goal.CriteriaVersion, Reason: "原计划", NextStep: "等合适时机表达", Stage: &GoalStagePlan{Operation: "create", Purpose: "寻找时机", Strategy: "等待对话机会", Criteria: []string{"适当机会"}, Reason: "原阶段"}, Commitment: &GoalCommitmentPlan{ExpectedResult: "表达心意", Criteria: []string{"实际表达"}, WindowEnd: &end}}, "redundant-plan", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	stages, commitments, err := readGoalObjectsWith(f.ctx, f.repository.Pool(), f.fluctlightID, id)
	if err != nil {
		t.Fatal(err)
	}
	intentionID := "obsolete-intention-" + f.suffix
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		intent, record, err := CreateIntentionAuthority(IntentionAuthority{EntityID: intentionID, GoalEntityID: id, SchemaVersion: intentionAuthoritySchemaVersion, Ref: "intention:ctx_" + stableDigest(intentionID), FluctlightID: f.fluctlightID, GoalRef: goal.Ref, StageID: stages[0].ID, CommitmentID: commitments[0].ID, ActionIntent: "稍后再表达", ExpectedOutcome: "实际表达", Trigger: TypedIntentionTrigger{Type: IntentionTriggerTime, DueAt: ptrGoalTime(f.app.now().Add(time.Hour))}, Expiration: f.app.now().Add(48 * time.Hour), Confidence: 1, Status: IntentionCandidate, Revision: 1}, []string{"owner:plan"}, f.app.now())
		if err != nil {
			return err
		}
		_, err = persistIntentionAuthorityTx(f.ctx, tx, nil, intent, record, "obsolete-intent")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	messageID := "parent-actual-" + f.suffix
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,50,$3,'assistant','我清楚告诉你，我喜欢你。','[]',$1)`, messageID, f.conversationID, f.fluctlightID); err != nil {
			return err
		}
		return f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, messageID, "default")
	}); err != nil {
		t.Fatal(err)
	}
	requestID := latestPendingGoalRequest(t, f)
	snapshot, _, err := f.app.claimGoalEvaluation(f.ctx, requestID)
	if err != nil {
		t.Fatal(err)
	}
	goal = snapshot.Goals[0].Goal
	sources := map[string]GoalSource{}
	proof := ""
	for _, s := range snapshot.Sources {
		sources[s.Ref] = s
		if s.ID == messageID {
			proof = s.Ref
		}
	}
	candidate := GoalEvaluationCandidate{GoalID: id, ExpectedRevision: goal.Revision, CriteriaVersion: goal.CriteriaVersion, Impact: "completed", Judgments: []GoalCriterionJudgment{{CriterionID: goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "原标准已经实际达成，不重演计划"}}}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.commitGoalEvaluationTx(f.ctx, tx, goal, candidate, sources, requestID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	stages, commitments, err = readGoalObjectsWith(f.ctx, f.repository.Pool(), f.fluctlightID, id)
	if err != nil || stages[0].Status != "cancelled" || commitments[0].Status != "cancelled" {
		t.Fatalf("obsolete objects retained %#v %#v %v", stages, commitments, err)
	}
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatalf("obsolete action still eligible %s %v", status, err)
	}
	var attempts int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&attempts); err != nil || attempts != 0 {
		t.Fatalf("completion fabricated attempts %d %v", attempts, err)
	}
}
func ptrGoalTime(at time.Time) *time.Time { return &at }

// Explicitly models pre-0054 excess stock inside a random isolated test DB.
// No production command bypasses capacity. Historical IDs/evidence stay intact.
func seedLegacyGoalActivityForTest(t *testing.T, f independentToolE2EFixture) {
	t.Helper()
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(f.ctx, `ALTER TABLE public.fluctlight_goals DISABLE TRIGGER goal_set_guard`); err != nil {
			return err
		}
		if _, err := tx.Exec(f.ctx, `UPDATE public.fluctlight_goals SET status='active' WHERE fluctlight_id=$1 AND status='candidate'`, f.fluctlightID); err != nil {
			return err
		}
		if _, err := tx.Exec(f.ctx, `ALTER TABLE public.fluctlight_goals ENABLE TRIGGER goal_set_guard`); err != nil {
			return err
		}
		_, err := queueGoalEvaluationTx(f.ctx, tx, f.fluctlightID, "", "legacy_test_snapshot", "legacy-stock", []string{})
		return err
	}); err != nil {
		t.Fatal(err)
	}
}
func createCandidateDialogueGoalForClosure(t *testing.T, f independentToolE2EFixture, criteria []string) string {
	t.Helper()
	id := "goal-dialogue-" + f.suffix
	goal, record, err := CreateGoalAuthority(GoalAuthority{EntityID: id, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(id), FluctlightID: f.fluctlightID, TargetActorID: f.ownerID, DesiredOutcome: "向对方表达 " + id, SuccessCriteria: criteria, Motivation: "历史需求", Scope: "relationship", Status: GoalCandidate, Revision: 1}, []string{"owner:goal"}, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistGoalAuthorityTx(f.ctx, tx, nil, goal, record, "candidate:"+id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id
}
