package core

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func insertGoalBoundaryMessage(t *testing.T, f independentToolE2EFixture) string {
	t.Helper()
	id := "boundary-message-" + f.suffix
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,50,$3,'assistant','我已经完成了，也实际向你表达了自己的想法','[]',$1)`, id, f.conversationID, f.fluctlightID); err != nil {
			return err
		}
		return f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, id, "default")
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestGoalAssessmentRejectsMalformedAuthorityWithoutConsumingSource(t *testing.T) {
	for _, tc := range []struct{ name, code string }{
		{"plan_only", "goal_assessment_coverage_missing"},
		{"unknown_no_change", "goal_evaluation_unknown_requires_evidence"},
		{"invalid_impact", "adk_final_contract_invalid"},
		{"assistant_self_report", "goal_judgment_self_report_not_business_fact"},
		{"quotation", "goal_judgment_not_actual_event"},
		{"hypothesis", "goal_judgment_not_actual_event"},
		{"future_plan", "goal_judgment_not_actual_event"},
		{"reported_action", "goal_judgment_not_actual_event"},
		{"unpublished_draft", "goal_judgment_source_invalid"},
		{"invalid_item", "goal_judgment_source_invalid"},
		{"general_goal_relationship_confirmation", "goal_relationship_resolution_scope_invalid"},
		{"unserved_stage", "goal_stage_evaluation_scope_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			goalID := createDialogueGoalForClosure(t, f, []string{"actual expression"})
			if tc.name == "general_goal_relationship_confirmation" {
				if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET scope='general' WHERE id=$1`, goalID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,role,metrics,revision) VALUES($1,$2,$3,'{"label":"friend"}','{}',4)`, "boundary-relation-"+f.suffix, f.fluctlightID, f.ownerID); err != nil {
					t.Fatal(err)
				}
			}
			messageID := insertGoalBoundaryMessage(t, f)
			if tc.name == "unpublished_draft" || tc.name == "invalid_item" {
				kind, id := "message", "unpublished-draft"
				if tc.name == "invalid_item" {
					kind, id = "item", "invalid-item"
				}
				if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
					_, err := recordGoalSourceEventTx(f.ctx, tx, f.fluctlightID, "default", kind, id, "unverified", f.conversationID, f.fluctlightID, "committed", f.app.now())
					return err
				}); err != nil {
					t.Fatal(err)
				}
			}
			seedCognitiveProviderRole(t, f.ctx, f.repository, "boundary-provider-"+f.suffix)
			f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
				snapshot := readProcessingGoalSnapshot(t, f)
				entry := snapshot.Goals[0]
				proof := ""
				for _, source := range snapshot.Sources {
					if source.ID == messageID {
						proof = source.Ref
					}
				}
				if proof == "" {
					t.Fatal("real committed message missing")
				}
				candidate := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "completed", Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "actual message"}}}
				output := GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}}
				switch tc.name {
				case "plan_only":
					output.Evaluations = []GoalEvaluationCandidate{}
					output.Plans = []GoalPlanCandidate{{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Reason: "only plan", WaitCondition: "wait for evidence"}}
				case "unknown_no_change":
					output.Evaluations[0].Impact = "no_change"
					output.Evaluations[0].Judgments[0].Verdict = "unknown"
					output.Evaluations[0].Judgments[0].Discourse = "uncertain"
					output.Evaluations[0].Judgments[0].EvidenceRefs = []string{}
					output.Evaluations[0].WaitCondition = "need evidence"
				case "invalid_impact":
					output.Evaluations[0].Impact = "unrecognized_semantic_output"
				case "quotation", "hypothesis", "future_plan", "reported_action":
					output.Evaluations[0].Judgments[0].Discourse = map[string]string{"quotation": "quotation", "hypothesis": "hypothesis", "future_plan": "plan", "reported_action": "report"}[tc.name]
				case "unpublished_draft", "invalid_item":
					invalidID := "unpublished-draft"
					if tc.name == "invalid_item" {
						invalidID = "invalid-item"
						output.Evaluations[0].Judgments[0].Kind = "acquisition"
						output.Evaluations[0].Judgments[0].Subject = "domain"
					}
					for _, source := range snapshot.Sources {
						if source.ID == invalidID {
							output.Evaluations[0].Judgments[0].EvidenceRefs = []string{source.Ref}
						}
					}
				case "assistant_self_report":
					output.Evaluations[0].Judgments[0].Kind = "semantic"
				case "general_goal_relationship_confirmation":
					output.Evaluations[0].RelationshipConfirmation = &GoalRelationshipConfirmation{TargetActorID: f.ownerID, EvidenceRefs: []string{proof}, Label: "恋人"}
				case "unserved_stage":
					output.Evaluations[0].StageEvaluation = &GoalObjectEvaluation{ID: "unserved-stage", ExpectedRevision: 1, CriteriaVersion: 1, Judgments: []GoalCriterionJudgment{}, Completed: true}
				}
				return fakeProviderResult{Structured: decodeObject(jsonBytes(output))}
			})}
			requestID := latestPendingGoalRequest(t, f)
			_, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
			var status, requestState string
			var revision, consumed, evaluations, resolutions int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,revision FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&status, &revision); err != nil {
				t.Fatal(err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.goal_evaluation_requests WHERE id=$1`, requestID).Scan(&requestState); err != nil {
				t.Fatal(err)
			}
			for _, q := range []struct {
				sql string
				dst *int
			}{
				{`SELECT count(*) FROM public.goal_source_events WHERE source_id=$1 AND processed_at IS NOT NULL`, &consumed},
				{`SELECT count(*) FROM public.goal_evaluations WHERE goal_id=$1`, &evaluations},
				{`SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1`, &resolutions},
			} {
				id := goalID
				if q.dst == &consumed {
					id = messageID
				}
				if err := f.repository.Pool().QueryRow(f.ctx, q.sql, id).Scan(q.dst); err != nil {
					t.Fatal(err)
				}
			}
			if status != "active" || revision != 1 || requestState != "retry" || consumed != 0 || evaluations != 0 || resolutions != 0 {
				t.Fatalf("invalid output mutated authority: %s rev=%d request=%s consumed=%d evaluations=%d resolutions=%d", status, revision, requestState, consumed, evaluations, resolutions)
			}
			if tc.name == "general_goal_relationship_confirmation" {
				var revision int
				var label string
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision,role->>'label' FROM public.relationships WHERE id=$1`, "boundary-relation-"+f.suffix).Scan(&revision, &label); err != nil {
					t.Fatal(err)
				}
				if revision != 4 || label != "friend" {
					t.Fatalf("relationship changed: %d %s", revision, label)
				}
			}
		})
	}
}

func TestGoalCounterEvidencePreservesScopeAndAllowsWithdrawal(t *testing.T) {
	goal := GoalAuthority{EntityID: "goal", FluctlightID: "self", ProfileID: "private", SuccessCriteria: []string{"result"}, CriterionIDs: []string{"criterion"}}
	for _, verdict := range []string{"unknown", "not_satisfied"} {
		for _, name := range []string{"instance", "profile", "binding", "withdrawal"} {
			t.Run(verdict+"/"+name, func(t *testing.T) {
				source := GoalSource{Ref: "source:1", Kind: "message", FluctlightID: "self", ProfileID: "private", Valid: true, GoalIDs: []string{"goal"}, Data: map[string]any{"message_kind": "assistant"}}
				switch name {
				case "instance":
					source.FluctlightID = "other"
				case "profile":
					source.ProfileID = "other"
				case "binding":
					source.GoalIDs = []string{"other"}
				case "withdrawal":
					source.Valid = false
				}
				_, _, _, err := validatedGoalJudgments(goal, []GoalCriterionJudgment{{CriterionID: "criterion", Verdict: verdict, EvidenceRefs: []string{source.Ref}, Reason: "counter-evidence"}}, map[string]GoalSource{source.Ref: source})
				if (err == nil) != (name == "withdrawal") {
					t.Fatalf("scope=%s verdict=%s err=%v", name, verdict, err)
				}
			})
		}
	}
}

func TestDuePersistedSettlementDoesNotHideSuppressionOrFailure(t *testing.T) {
	for _, name := range []string{"duplicate_delivery", "query_only", "no_action", "mixed_action_failure", "policy_failure", "mixed_policy_failure", "synchronous_action"} {
		t.Run(name, func(t *testing.T) {
			f, _, due := seedNativeDueForGoalExecution(t)
			if stopped, err := f.app.prepareNativeDueAttempt(f.ctx, stringValue(due["inbox_id"]), f.fluctlightID); err != nil || stopped {
				t.Fatalf("claim: %v %v", stopped, err)
			}
			var raw []byte
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload->'due_context' FROM public.cognition_inbox WHERE id=$1`, due["inbox_id"]).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			frozen := decodeObject(raw)
			results := []CapabilityResult{{CallID: "success", CapabilityName: lifeActivityAdvanceCapabilityName, Status: "completed"}}
			wantAttempt, wantIntention, wantOutcome := "succeeded", "completed", ActionOutcomeCompleted
			if name == "duplicate_delivery" {
				results = []CapabilityResult{{CallID: "duplicate", CapabilityName: "conversation.reply", Status: "completed", Output: map[string]any{"delivery_status": "duplicate_suppressed"}}}
				wantAttempt, wantIntention, wantOutcome = "suppressed", "qualified", ActionOutcomeSuppressed
			}
			if name == "synchronous_action" || name == "duplicate_delivery" {
				arguments := map[string]any{"text": "actual boundary test expression", "topic_key": "boundary-expression", "purpose": "actual expression once"}
				request := f.request(conversationReplyCapabilityName, "actual-sync-expression", arguments)
				request.Surface = CapabilitySurfaceWakeUp
				request.AuthorizationPolicy = "autonomy"
				receipt, err := f.app.ExecuteTool(f.ctx, request)
				if err != nil || receipt.Result.Status != "completed" {
					t.Fatalf("actual synchronous action: %#v %v", receipt, err)
				}
				if name == "duplicate_delivery" {
					request.OperationID += "-distinct-redelivery"
					receipt, err = f.app.ExecuteTool(f.ctx, request)
					if err != nil || stringValue(mapValue(receipt.Result.Output)["delivery_status"]) != "duplicate_suppressed" {
						t.Fatalf("actual duplicate not suppressed: %#v %v", receipt, err)
					}
				}
				results = []CapabilityResult{receipt.Result}
				var messages int
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='actual boundary test expression'`, f.conversationID).Scan(&messages); err != nil || messages != 1 {
					t.Fatalf("duplicate message side effect: %d %v", messages, err)
				}
			}
			if name == "query_only" || name == "no_action" {
				results = nil
				if name == "query_only" {
					results = []CapabilityResult{{CallID: "query", CapabilityName: wardrobeInspectCapabilityName, Status: "completed"}}
				}
				wantAttempt, wantIntention, wantOutcome = "suppressed", "qualified", ActionOutcomeSuppressed
			}
			if name == "mixed_action_failure" || name == "policy_failure" || name == "mixed_policy_failure" {
				results = append(results, CapabilityResult{CallID: "failed", CapabilityName: lifeActivityAdvanceCapabilityName, Status: "failed", ErrorCode: "transient_failure"})
				wantAttempt, wantIntention, wantOutcome = "failed", "qualified", ActionOutcomeFailed
			}
			if name == "policy_failure" || name == "mixed_policy_failure" {
				results[1].ErrorCode = "policy_denied"
				wantIntention = "paused"
			}
			if name == "mixed_policy_failure" {
				results = append([]CapabilityResult{{CallID: "first-transient", CapabilityName: lifeActivityAdvanceCapabilityName, Status: "failed", ErrorCode: "transient_failure"}}, results...)
			}
			status, reason := dueActionSettlement(results, f.app.capabilityRegistry())
			outcomes, err := buildActionOutcomes("agent_native_"+stableDigest(stringValue(due["inbox_id"])), f.fluctlightID, stringValue(due["inbox_id"]), "no_op", results, map[string]any{"status": status, "reason_code": reason, "goal_refs": []string{stringValue(frozen["goal_ref"])}, "intention_refs": []string{stringValue(frozen["intention_ref"])}, "context_references": frozen["context_references"]}, f.app.capabilityRegistry(), f.app.now())
			if err != nil {
				t.Fatal(err)
			}
			if (name == "policy_failure" || name == "mixed_policy_failure") && (outcomes[0].ErrorCode != "policy_denied" || stringValue(outcomes[0].Observed["status"]) != "failed") {
				t.Fatalf("policy code lost: %#v", outcomes[0])
			}
			for i := 0; i < 2; i++ {
				if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return persistActionOutcomesTx(f.ctx, tx, outcomes) }); err != nil {
					t.Fatal(err)
				}
			}
			var attempt, intention, primary string
			var progress float64
			var retryCount int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT a.status,i.status,o.status,g.progress,i.retry_count FROM public.fluctlight_intention_attempts a JOIN public.fluctlight_intentions i ON i.id=a.intention_id JOIN public.fluctlight_goals g ON g.id=i.goal_id JOIN public.cognition_action_outcomes o ON o.id=a.outcome_id WHERE a.attempt_id=$1`, due["attempt_id"]).Scan(&attempt, &intention, &primary, &progress, &retryCount); err != nil {
				t.Fatal(err)
			}
			if attempt != wantAttempt || intention != wantIntention || primary != string(wantOutcome) || progress != 0 {
				t.Fatalf("attempt=%s intention=%s primary=%s progress=%f retry=%d", attempt, intention, primary, progress, retryCount)
			}
			if name == "policy_failure" || name == "mixed_policy_failure" {
				var retryReason string
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT retry_reason FROM public.fluctlight_intentions WHERE current_attempt_id=$1`, due["attempt_id"]).Scan(&retryReason); err != nil {
					t.Fatal(err)
				}
				if retryReason != "retry_stopped:policy_denied" {
					t.Fatalf("policy failure retried: %s", retryReason)
				}
			}
			if name == "duplicate_delivery" && retryCount != 0 {
				t.Fatalf("suppression counted as failed attempt: %d", retryCount)
			}
		})
	}
}

func TestGoalObjectEvaluationRejectsUnservedAndDuplicateCommitments(t *testing.T) {
	entry := goalEvaluationGoal{Commitments: []GoalCommitment{{ID: "served", Revision: 2, CriteriaVersion: 1}}}
	for _, candidate := range []GoalEvaluationCandidate{
		{CommitmentEvaluations: []GoalObjectEvaluation{{ID: "hidden", ExpectedRevision: 1, CriteriaVersion: 1}}},
		{CommitmentEvaluations: []GoalObjectEvaluation{{ID: "served", ExpectedRevision: 1, CriteriaVersion: 1}}},
		{CommitmentEvaluations: []GoalObjectEvaluation{{ID: "served", ExpectedRevision: 2, CriteriaVersion: 1}, {ID: "served", ExpectedRevision: 2, CriteriaVersion: 1}}},
	} {
		if err := validateGoalObjectEvaluationScope(entry, candidate); err == nil {
			t.Fatal("unserved or repeated object accepted")
		}
	}
	if err := validateGoalObjectEvaluationScope(entry, GoalEvaluationCandidate{CommitmentEvaluations: []GoalObjectEvaluation{{ID: "served", ExpectedRevision: 2, CriteriaVersion: 1}}}); err != nil {
		t.Fatal(err)
	}
}

func TestGoalTerminalLateCallbacksDoNotReviveOrDuplicate(t *testing.T) {
	for _, terminal := range []ActionOutcomeStatus{ActionOutcomeCompleted, ActionOutcomeFailed} {
		t.Run(string(terminal), func(t *testing.T) {
			f, intentionID, due, activityID, _ := startDueShoppingThroughNative(t)
			changeGoalForExecutionTest(t, f, intentionID, GoalCancel)
			observed := map[string]any{"result": "late actual operation result"}
			code := ""
			if terminal == ActionOutcomeFailed {
				code = "operation_failed"
			}
			for i := 0; i < 2; i++ {
				if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
					changed, err := f.app.settleActionOutcomeByExternalRefTx(f.ctx, tx, activityID, terminal, observed, code)
					if err == nil && changed != (i == 0) {
						t.Fatalf("callback replay changed=%v iteration=%d", changed, i)
					}
					return err
				}); err != nil {
					t.Fatal(err)
				}
			}
			var goalState, intentionState, outcomeState string
			var progress float64
			var revision, attempts, resolutions int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT g.status,g.progress,i.status,o.status,o.revision FROM public.fluctlight_goals g JOIN public.fluctlight_intentions i ON i.goal_id=g.id JOIN public.cognition_action_outcomes o ON o.external_ref=$2 WHERE i.id=$1`, intentionID, activityID).Scan(&goalState, &progress, &intentionState, &outcomeState, &revision); err != nil {
				t.Fatal(err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, due["attempt_id"]).Scan(&attempts); err != nil {
				t.Fatal(err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_resolutions WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&resolutions); err != nil {
				t.Fatal(err)
			}
			if goalState != "cancelled" || intentionState != "cancelled" || progress != 0 || outcomeState != string(terminal) || revision != 2 || attempts != 1 || resolutions != 0 {
				t.Fatalf("late callback revived/repeated authority: goal=%s intention=%s outcome=%s rev=%d attempts=%d resolutions=%d", goalState, intentionState, outcomeState, revision, attempts, resolutions)
			}
			conflict := ActionOutcomeFailed
			if terminal == ActionOutcomeFailed {
				conflict = ActionOutcomeCompleted
			}
			err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				_, err := f.app.settleActionOutcomeByExternalRefTx(f.ctx, tx, activityID, conflict, observed, code)
				return err
			})
			if err == nil || !strings.Contains(err.Error(), "action_outcome_async_terminal_conflict") {
				t.Fatalf("conflicting callback accepted: %v", err)
			}
		})
	}
}

func TestHardDeadlineAcceptsStartedActivityResultWithoutRestart(t *testing.T) {
	f, intentionID, due, activityID, router := startDueShoppingThroughNative(t)
	deadline := f.app.now().Add(time.Minute)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET deadline_policy='hard',deadline=$2 WHERE id=(SELECT goal_id FROM public.fluctlight_intentions WHERE id=$1)`, intentionID, deadline); err != nil {
		t.Fatal(err)
	}
	f.app.Clock = fixedClock(deadline.Add(time.Minute))
	forceVirtualActivityDue(t, f, activityID)
	router.on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "already started operation finished", "acquired_item": map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}}}
	})
	for i := 0; i < 2; i++ {
		resolved, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityAdvanceCapabilityName, "deadline-result", map[string]any{"activity_id": activityID}))
		if err != nil || resolved.Result.Status != "completed" {
			t.Fatalf("late result lost: %#v %v", resolved, err)
		}
	}
	var intentionState, attemptState string
	var runs, items int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT i.status,a.status FROM public.fluctlight_intentions i JOIN public.fluctlight_intention_attempts a ON a.attempt_id=i.current_attempt_id WHERE i.id=$1 AND a.attempt_id=$2`, intentionID, due["attempt_id"]).Scan(&intentionState, &attemptState); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, f.fluctlightID).Scan(&items); err != nil {
		t.Fatal(err)
	}
	if intentionState != "completed" || attemptState != "succeeded" || runs != 1 || items != 1 {
		t.Fatalf("late result not settled once: %s %s runs=%d items=%d", intentionState, attemptState, runs, items)
	}
	trigger, err := f.app.ProcessIntentionTrigger(f.ctx, intentionID)
	if err != nil || stringValue(trigger["status"]) == "due" {
		t.Fatalf("terminal intention restarted: %#v %v", trigger, err)
	}
}

func TestGoalSourceWithdrawalReevaluatesActiveAndPreservesRealResolution(t *testing.T) {
	for _, ended := range []bool{false, true} {
		name := "active"
		if ended {
			name = "completed"
		}
		t.Run(name, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			criteria := []string{"actual expression"}
			if !ended {
				criteria = append(criteria, "another actual result")
			}
			goalID := createDialogueGoalForClosure(t, f, criteria)
			messageID := insertGoalBoundaryMessage(t, f)
			seedCognitiveProviderRole(t, f.ctx, f.repository, "withdrawal-boundary-"+f.suffix)
			f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
				snapshot := readProcessingGoalSnapshot(t, f)
				entry := snapshot.Goals[0]
				var proof GoalSource
				for _, source := range snapshot.Sources {
					if source.ID == messageID {
						proof = source
						if source.Valid {
							break
						}
					}
				}
				if proof.Ref == "" {
					t.Fatal("withdrawal lost original source")
				}
				judgments := []GoalCriterionJudgment{}
				for i, id := range entry.Goal.CriterionIDs {
					j := GoalCriterionJudgment{CriterionID: id, Verdict: "unknown", Kind: "communication", Subject: "actor_self", Discourse: "uncertain", EvidenceRefs: []string{}, Reason: "no current proof"}
					if i == 0 {
						j.EvidenceRefs = []string{proof.Ref}
						if proof.Valid {
							j.Verdict, j.Discourse = "satisfied", "assertion"
						}
					}
					judgments = append(judgments, j)
				}
				impact, wait := "progressed", "wait for remaining evidence"
				if !proof.Valid {
					impact = "regressed"
				} else if ended {
					impact, wait = "completed", ""
				}
				return fakeProviderResult{Structured: decodeObject(jsonBytes(GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Judgments: judgments, Impact: impact, WaitCondition: wait}}, Plans: []GoalPlanCandidate{}}))}
			})}
			requestID := latestPendingGoalRequest(t, f)
			if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID); err != nil {
				t.Fatal(err)
			}
			var beforeRevision int
			var beforeProgress float64
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision,progress FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&beforeRevision, &beforeProgress); err != nil {
				t.Fatal(err)
			}
			if beforeProgress == 0 {
				t.Fatal("test did not establish real evidence progress")
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.conversation_messages SET text='corrected source' WHERE id=$1`, messageID); err != nil {
				t.Fatal(err)
			}
			if !ended {
				id := latestPendingGoalRequest(t, f)
				var workflows int
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='goal.evaluate'`, id).Scan(&workflows); err != nil || workflows != 1 {
					t.Fatalf("withdrawal not durable: %d %v", workflows, err)
				}
				if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, id); err != nil {
					t.Fatal(err)
				}
			}
			var status string
			var revision, confirmed, withdrawn, resolutions, flags int
			var progress float64
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,revision,progress FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&status, &revision, &progress); err != nil {
				t.Fatal(err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FILTER (WHERE status='confirmed'),count(*) FILTER (WHERE status='withdrawn') FROM public.goal_evidence_links WHERE goal_id=$1`, goalID).Scan(&confirmed, &withdrawn); err != nil {
				t.Fatal(err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1`, goalID).Scan(&resolutions); err != nil {
				t.Fatal(err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evidence_review_flags WHERE goal_id=$1`, goalID).Scan(&flags); err != nil {
				t.Fatal(err)
			}
			if confirmed != 0 || withdrawn == 0 {
				t.Fatalf("stale evidence still confirmed: %d withdrawn=%d", confirmed, withdrawn)
			}
			if ended {
				if status != "completed" || revision != beforeRevision || progress != 1 || resolutions != 1 || flags == 0 {
					t.Fatalf("ended history rewritten: %s rev=%d progress=%f resolutions=%d flags=%d", status, revision, progress, resolutions, flags)
				}
			} else if status != "active" || revision <= beforeRevision || progress != 0 || resolutions != 0 {
				t.Fatalf("active authority not reevaluated: %s rev=%d progress=%f resolutions=%d", status, revision, progress, resolutions)
			}
		})
	}
}

func TestGoalPausedDueRetainsVisibleSuppressionReason(t *testing.T) {
	f, intentionID, due := seedNativeDueForGoalExecution(t)
	changeGoalForExecutionTest(t, f, intentionID, GoalPause)
	for i := 0; i < 2; i++ {
		stopped, err := f.app.prepareNativeDueAttempt(f.ctx, stringValue(due["inbox_id"]), f.fluctlightID)
		if err != nil || !stopped {
			t.Fatalf("paused due admitted: %v %v", stopped, err)
		}
	}
	var reason, attemptReason, state string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT c.payload->'agent_result'->>'reason_code',a.result->>'reason',a.status FROM public.cognition_inbox c JOIN public.fluctlight_intention_attempts a ON a.attempt_id=$2 WHERE c.id=$1`, due["inbox_id"], due["attempt_id"]).Scan(&reason, &attemptReason, &state); err != nil {
		t.Fatal(err)
	}
	if reason != "goal_not_active" || attemptReason != reason || state != "suppressed" {
		t.Fatalf("suppression lost authoritative cause: %s %s %s", reason, attemptReason, state)
	}
}

func TestStaleDueRetainsVisibleSuppressionReason(t *testing.T) {
	f, _, due := seedNativeDueForGoalExecution(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{candidate,intention_revision}','-1'::jsonb) WHERE id=$1`, due["inbox_id"]); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		stopped, err := f.app.prepareNativeDueAttempt(f.ctx, stringValue(due["inbox_id"]), f.fluctlightID)
		if err != nil || !stopped {
			t.Fatalf("stale due admitted: %v %v", stopped, err)
		}
	}
	var reason, attemptReason string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT c.payload->'agent_result'->>'reason_code',a.result->>'reason' FROM public.cognition_inbox c JOIN public.fluctlight_intention_attempts a ON a.attempt_id=$2 WHERE c.id=$1`, due["inbox_id"], due["attempt_id"]).Scan(&reason, &attemptReason); err != nil {
		t.Fatal(err)
	}
	if reason != "intention_due_stale" || attemptReason != reason {
		t.Fatalf("stale cause lost: %s %s", reason, attemptReason)
	}
}

func TestExpiredCommitmentIncludesTimelyProofInFrozenAssessment(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"a later result beyond this commitment"})
	end := f.app.now().Add(time.Hour)
	start := f.app.now().Add(-time.Hour)
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		goal, err := loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: goalID, Revision: 1})
		if err != nil {
			return err
		}
		_, err = applyGoalPlanTx(f.ctx, tx, goal, GoalPlanCandidate{GoalID: goalID, ExpectedRevision: 1, CriteriaVersion: goal.CriteriaVersion, Reason: "bounded commitment", WaitCondition: "await response", Commitment: &GoalCommitmentPlan{ExpectedResult: "actually express", Criteria: []string{"actual expression"}, WindowStart: &start, WindowEnd: &end}}, "timely-commitment", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	messageID := insertGoalBoundaryMessage(t, f)
	f.app.Clock = fixedClock(end.Add(time.Minute))
	seedCognitiveProviderRole(t, f.ctx, f.repository, "timely-commitment-provider-"+f.suffix)
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		snapshot := readProcessingGoalSnapshot(t, f)
		entry := snapshot.Goals[0]
		if len(entry.Commitments) != 1 || entry.Commitments[0].Status != "expired" {
			t.Fatalf("timely proof omitted expired commitment: %#v", entry.Commitments)
		}
		c := entry.Commitments[0]
		proof := ""
		for _, source := range snapshot.Sources {
			if source.ID == messageID && source.Valid {
				proof = source.Ref
			}
		}
		if proof == "" {
			t.Fatal("actual timely proof missing")
		}
		candidate := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Judgments: []GoalCriterionJudgment{}, Impact: "needs_evidence", WaitCondition: "await parent result", CommitmentEvaluations: []GoalObjectEvaluation{{ID: c.ID, ExpectedRevision: c.Revision, CriteriaVersion: c.CriteriaVersion, Completed: true, Reason: "timely message, late assessment", Judgments: []GoalCriterionJudgment{{CriterionID: c.Criteria[0].ID, Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "actual sent message within window"}}}}}
		return fakeProviderResult{Structured: decodeObject(jsonBytes(GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}}))}
	})}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, latestPendingGoalRequest(t, f)); err != nil {
		t.Fatal(err)
	}
	var status, goalState string
	var progress float64
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT c.status,g.status,g.progress FROM public.goal_commitments c JOIN public.fluctlight_goals g ON g.id=c.goal_id WHERE c.goal_id=$1`, goalID).Scan(&status, &goalState, &progress); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || goalState != "active" || progress != 0 {
		t.Fatalf("commitment polluted parent progress: %s %s %f", status, goalState, progress)
	}
}

func TestActualQueryCanCompleteInformationGoalWithoutBusinessAction(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	intentionID := createQualifiedBootIntention(t, f, "information-query")
	var goalID string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT goal_id FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&goalID); err != nil {
		t.Fatal(err)
	}
	// Seed an information-only criterion before the first due event/Attempt.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET desired_outcome='了解当前衣柜的实际物品',success_criteria='["实际查询当前衣柜"]' WHERE id=$1`, goalID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := appendProcessedCognitionFactTx(f.ctx, tx, f.fluctlightID, "life.event.created", map[string]any{"summary": "actual query opportunity"}, "information-trigger-"+f.suffix)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	due, err := f.app.ProcessIntentionTrigger(f.ctx, intentionID)
	if err != nil || stringValue(due["status"]) != "due" {
		t.Fatalf("due query: %#v %v", due, err)
	}
	projection, err := f.app.BuildContextProjection(f.ctx, f.ownerID, f.fluctlightID, "", stringValue(due["inbox_id"]), "")
	if err != nil {
		t.Fatal(err)
	}
	for ref, entry := range projection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceIntention && entry.EntityID == intentionID {
			due["intention_ref"] = ref
		}
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "information-provider-"+f.suffix)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("native_cognition_response", func(_ map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("actual-query", wardrobeInspectCapabilityName, map[string]any{"operation": "list"})}}
		}
		final := nativePersonaFinal()
		delete(final.Structured, "action_type")
		delete(final.Structured, "response_intent")
		for _, key := range []string{"attention", "thought", "desire", "agency"} {
			final.Structured[key] = "actual query returned information; no business action"
		}
		final.Structured["influences"] = []any{map[string]any{"ref": due["goal_ref"], "role": "motivates", "confidence": 1.0, "note": "information goal"}, map[string]any{"ref": due["intention_ref"], "role": "grounds", "confidence": 1.0, "note": "actual query"}}
		return final
	}).on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		snapshot := readProcessingGoalSnapshot(t, f)
		entry := snapshot.Goals[0]
		proof := ""
		for _, source := range snapshot.Sources {
			if source.Kind == "outcome" && source.Valid && source.CanSupportSuccess && stringValue(source.Data["capability"]) == wardrobeInspectCapabilityName {
				proof = source.Ref
			}
		}
		if proof == "" {
			t.Fatal("actual query receipt absent from authoritative sources")
		}
		candidate := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "completed", Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "information", Subject: "domain", Discourse: "domain_fact", EvidenceRefs: []string{proof}, Reason: "actual query returned current inventory"}}}
		return fakeProviderResult{Structured: decodeObject(jsonBytes(GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}}))}
	})}
	if _, err := f.app.ProcessCognitionInbox(f.ctx, stringValue(due["inbox_id"])); err != nil {
		t.Fatal(err)
	}
	var attemptStatus string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, due["attempt_id"]).Scan(&attemptStatus); err != nil || attemptStatus != "suppressed" {
		t.Fatalf("query counted as action: %s %v", attemptStatus, err)
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, latestPendingGoalRequest(t, f)); err != nil {
		t.Fatal(err)
	}
	var status string
	var attempts, activityRuns int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&activityRuns); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || attempts != 1 || activityRuns != 0 {
		t.Fatalf("query invented business action: %s attempts=%d activities=%d", status, attempts, activityRuns)
	}
}
