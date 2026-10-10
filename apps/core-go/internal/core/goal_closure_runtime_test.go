package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func createDialogueGoalForClosure(t *testing.T, f independentToolE2EFixture, criteria []string) string {
	t.Helper()
	id := "goal-dialogue-" + f.suffix
	goal, record, err := CreateGoalAuthority(GoalAuthority{EntityID: id, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(id), FluctlightID: f.fluctlightID, ProfileID: "", TargetActorID: f.ownerID, DesiredOutcome: "向对方清楚表达自己的心意", SuccessCriteria: criteria, Motivation: "真实表达当前想法", Scope: "relationship", Status: GoalActive, Revision: 1}, []string{"owner:goal"}, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistGoalAuthorityTx(f.ctx, tx, nil, goal, record, "dialogue-goal-create-"+f.suffix)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func readProcessingGoalSnapshot(t *testing.T, f independentToolE2EFixture) goalEvaluationSnapshot {
	t.Helper()
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT snapshot FROM public.goal_evaluation_requests WHERE fluctlight_id=$1 AND status='processing' ORDER BY claimed_at DESC,id DESC LIMIT 1`, f.fluctlightID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var snapshot goalEvaluationSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func latestPendingGoalRequest(t *testing.T, f independentToolE2EFixture) string {
	t.Helper()
	var id string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_evaluation_requests WHERE fluctlight_id=$1 AND status IN ('pending','retry') ORDER BY created_at DESC,id DESC LIMIT 1`, f.fluctlightID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func logGoalClosureEvidence(t *testing.T, f independentToolE2EFixture, goalID, scenario string) {
	t.Helper()
	var criteriaVersion, revision int
	var evaluationID, status string
	var requestID *string
	var refs []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT g.criteria_version,g.revision,g.status,e.id,e.request_id,e.evidence_refs FROM public.fluctlight_goals g JOIN public.goal_evaluations e ON e.goal_id=g.id AND e.stage_id IS NULL AND e.commitment_id IS NULL WHERE g.id=$1 ORDER BY e.created_at DESC,e.id DESC LIMIT 1`, goalID).Scan(&criteriaVersion, &revision, &status, &evaluationID, &requestID, &refs); err != nil {
		t.Fatal(err)
	}
	id := ""
	if requestID != nil {
		id = *requestID
	}
	t.Logf("GOAL_CLOSURE_EVIDENCE scenario=%s goal=%s criteria_version=%d revision=%d status=%s request=%s evaluation=%s evidence_refs=%s", scenario, goalID, criteriaVersion, revision, status, id, evaluationID, refs)
}

func TestFormalConversationWithoutIntentionCompletesOriginalExpressionGoal(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"已实际清楚向对方表达自己的心意"})
	seedCognitiveProviderRole(t, f.ctx, f.repository, "dialogue-goal-model-"+f.suffix)
	assessments := 0
	router := newFakeProviderRouter().on("conversation_turn_response", func(payload map[string]any) fakeProviderResult {
		refs := regexp.MustCompile(`goal:ctx_[a-f0-9]{32}`).FindAllString(jsonString(payload), -1)
		if len(refs) == 0 {
			t.Error("formal conversation omitted served Goal ref")
			return fakeProviderResult{Status: 500}
		}
		ref := refs[0]
		return fakeProviderResult{Structured: map[string]any{"action_type": "reply", "response_intent": "认真表达自己的心意", "visible_text": "我喜欢你，这是我现在想清楚告诉你的心意。", "influences": []any{map[string]any{"ref": ref, "role": "motivates", "confidence": 1.0, "note": "当前表达目标"}}, "goal_event_candidates": []any{map[string]any{"goal_ref": ref, "reason": "本轮正式表达可能满足原成功标准"}}}}
	}).onGoalEvaluation(func(payload map[string]any) fakeProviderResult {
		assessments++
		schema := mapValue(mapValue(payload["response_format"])["json_schema"])
		evaluation := mapValue(mapValue(mapValue(mapValue(schema["schema"])["properties"])["evaluations"])["items"])
		properties := mapValue(evaluation["properties"])
		if _, ok := properties["stage_evaluation"]; ok {
			t.Error("no-stage Goal Provider schema advertised stage_evaluation")
		}
		if _, ok := properties["commitment_evaluations"]; ok {
			t.Error("no-commitment Goal Provider schema advertised commitment_evaluations")
		}
		snapshot := readProcessingGoalSnapshot(t, f)
		var expression GoalSource
		for _, source := range snapshot.Sources {
			if source.Kind == "message" && source.SubjectActorID == f.fluctlightID {
				expression = source
			}
		}
		if !expression.Valid || !expression.CanSupportSuccess || expression.ID == "" {
			t.Fatalf("actual published expression source missing: %#v", snapshot.Sources)
		}
		goal := snapshot.Goals[0]
		output := GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{{GoalID: goal.GoalID, ExpectedRevision: goal.Goal.Revision, CriteriaVersion: goal.Goal.CriteriaVersion, Judgments: []GoalCriterionJudgment{{CriterionID: goal.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{expression.Ref}, Reason: "正式落库的实际表达满足原标准，不要求对方接受"}}, Impact: "completed"}}, Plans: []GoalPlanCandidate{}}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(snapshot, output)}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	turn, err := f.app.HandleTurn(f.ctx, f.ownerID, f.conversationID, map[string]any{"fluctlight_id": f.fluctlightID, "text": "现在请告诉我你的心意。", "idempotency_key": "expression-turn-" + f.suffix})
	if err != nil || stringValue(turn.Assistant["id"]) == "" {
		t.Fatalf("formal turn=%#v %v", turn, err)
	}
	requestID := latestPendingGoalRequest(t, f)
	result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID)
	if err != nil || stringValue(result["status"]) != "succeeded" {
		t.Fatalf("assessment=%#v %v", result, err)
	}
	var state string
	var attempts, intentions, evidence, resolutions int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	for _, query := range []struct {
		sql string
		dst *int
	}{
		{`SELECT count(*) FROM public.fluctlight_intention_attempts WHERE fluctlight_id=$1`, &attempts},
		{`SELECT count(*) FROM public.fluctlight_intentions WHERE fluctlight_id=$1`, &intentions},
		{`SELECT count(*) FROM public.goal_evidence_links l JOIN public.goal_source_events e ON e.id=l.source_event_id WHERE e.fluctlight_id=$1 AND e.source_kind='message' AND l.status='confirmed'`, &evidence},
		{`SELECT count(*) FROM public.goal_resolutions WHERE fluctlight_id=$1 AND residual_motivation='' AND followup_goal_id IS NULL`, &resolutions},
	} {
		if err := f.repository.Pool().QueryRow(f.ctx, query.sql, f.fluctlightID).Scan(query.dst); err != nil {
			t.Fatal(err)
		}
	}
	if state != "completed" || attempts != 0 || intentions != 0 || evidence != 1 || resolutions != 1 {
		t.Fatalf("no-Intention closure: state=%s attempts=%d intentions=%d evidence=%d resolutions=%d", state, attempts, intentions, evidence, resolutions)
	}
	logGoalClosureEvidence(t, f, goalID, "conversation_without_intention")
	goals, _, err := f.app.agencyProfile(f.ctx, f.fluctlightID)
	if err != nil || len(goals) != 1 || stringValue(goals[0]["status"]) != "completed" {
		t.Fatalf("completed Goal lost dynamic authority: %#v %v", goals, err)
	}
	execution := mapValue(goals[0]["execution"])
	if execution["stage"] != "completed" || mapValue(execution["last_result"])["kind"] != "message" || execution["last_attempt"] != nil || mapValue(execution["last_evaluation"])["impact"] != "completed" {
		t.Fatalf("dialogue result disappeared without life activity: %#v", execution)
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID); err != nil || assessments != 1 {
		t.Fatalf("assessment replay called model: %d %v", assessments, err)
	}
}

func TestGoalEvaluationGhostStageFailsBeforeDomainCommit(t *testing.T) {
	for _, mode := range []string{"empty_stage_ref", "null_stage", "empty_commitments", "empty_review_stage_ref"} {
		t.Run(mode, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			goalID := createDialogueGoalForClosure(t, f, []string{"已实际清楚向对方表达自己的心意"})
			seedCognitiveProviderRole(t, f.ctx, f.repository, "ghost-stage-model-"+f.suffix)
			router := newFakeProviderRouter().on("conversation_turn_response", func(payload map[string]any) fakeProviderResult {
				refs := regexp.MustCompile(`goal:ctx_[a-f0-9]{32}`).FindAllString(jsonString(payload), -1)
				if len(refs) == 0 {
					return fakeProviderResult{Status: 500}
				}
				return fakeProviderResult{Structured: map[string]any{"action_type": "reply", "response_intent": "完成真实表达", "visible_text": "我认真地告诉你，我很在意你。", "influences": []any{map[string]any{"ref": refs[0], "role": "motivates", "confidence": 1.0, "note": "当前表达目标"}}, "goal_event_candidates": []any{map[string]any{"goal_ref": refs[0], "reason": "实际表达可能满足目标"}}}}
			}).onGoalEvaluation(func(_ map[string]any) fakeProviderResult {
				snapshot := readProcessingGoalSnapshot(t, f)
				var proof GoalSource
				for _, source := range snapshot.Sources {
					if source.Kind == "message" && source.SubjectActorID == f.fluctlightID {
						proof = source
					}
				}
				entry := snapshot.Goals[0]
				output := GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof.Ref}, Reason: "实际消息"}}, Impact: "completed"}}, Plans: []GoalPlanCandidate{}}
				wire := goalEvaluationProviderFixture(snapshot, output)
				evaluation := mapValue(arrayValue(wire["evaluations"])[0])
				switch mode {
				case "empty_stage_ref":
					evaluation["stage_evaluation"] = map[string]any{"object_ref": "", "judgments": []any{}, "completed": true, "reason": "ghost"}
				case "null_stage":
					evaluation["stage_evaluation"] = nil
				case "empty_commitments":
					evaluation["commitment_evaluations"] = []any{}
				case "empty_review_stage_ref":
					evaluation["review"] = map[string]any{"reason_category": "progressed", "decision": "continue", "explanation": "真实消息满足标准", "evidence_refs": []any{"e1"}, "stage_ref": "", "feasible_alternative": ""}
				}
				return fakeProviderResult{Structured: wire}
			})
			f.app.Provider.HTTP = &http.Client{Transport: router}
			if _, err := f.app.HandleTurn(f.ctx, f.ownerID, f.conversationID, map[string]any{"fluctlight_id": f.fluctlightID, "text": "请告诉我你的心意。", "idempotency_key": "ghost-stage-turn-" + f.suffix}); err != nil {
				t.Fatal(err)
			}
			_, err := f.app.ProcessGoalEvaluationIntent(f.ctx, latestPendingGoalRequest(t, f))
			if err == nil || !strings.Contains(err.Error(), "adk_final_contract_invalid") {
				t.Fatalf("ghost stage error=%v", err)
			}
			var status string
			var evaluations, resolutions int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evaluations WHERE goal_id=$1`, goalID).Scan(&evaluations); err != nil {
				t.Fatal(err)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1`, goalID).Scan(&resolutions); err != nil {
				t.Fatal(err)
			}
			if status != "active" || evaluations != 0 || resolutions != 0 {
				t.Fatalf("ghost stage mutated domain: status=%s evaluations=%d resolutions=%d", status, evaluations, resolutions)
			}
		})
	}
}

func TestGoalEvaluationRejectsQuoteWrongActorAndMissingMutualConfirmation(t *testing.T) {
	goal := GoalAuthority{EntityID: "relation-goal", SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", FluctlightID: "self", TargetActorID: "user", Scope: "relationship", DesiredOutcome: "双方明确确认恋爱关系", SuccessCriteria: []string{"双方明确确认"}, CriterionIDs: []string{"criterion_a"}, CriteriaVersion: 1, Motivation: "双方认可", Status: GoalActive, Revision: 1, EvidenceRefs: []string{"owner:goal"}}
	source := GoalSource{Ref: "source:1", EventID: 1, Kind: "message", ID: "real-message", Version: "v1", FluctlightID: "self", ProfileID: "default", ConversationID: "conversation", SubjectActorID: "self", OccurredAt: time.Now().UTC(), Valid: true, CanSupportSuccess: true, Data: map[string]any{"text": "我喜欢你", "message_kind": "assistant", "participants": []string{"self", "user"}}}
	sources := map[string]GoalSource{source.Ref: source}
	for _, tc := range []struct{ name, subject, discourse string }{
		{"single expression is not acceptance", "both", "assertion"},
		{"quote is not actual expression", "actor_self", "quotation"},
		{"user statement cannot be self expression", "actor_self", "assertion"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := source
			if tc.name == "user statement cannot be self expression" {
				copy.SubjectActorID = "user"
			}
			sources[copy.Ref] = copy
			candidate := GoalEvaluationCandidate{GoalID: goal.EntityID, ExpectedRevision: 1, CriteriaVersion: 1, Impact: "completed", Judgments: []GoalCriterionJudgment{{CriterionID: "criterion_a", Verdict: "satisfied", Kind: "communication", Subject: tc.subject, Discourse: tc.discourse, EvidenceRefs: []string{copy.Ref}, Reason: "candidate"}}}
			if _, _, _, err := ApplyGoalEvaluation(goal, candidate, sources, time.Now()); err == nil {
				t.Fatal("unsupported completion accepted")
			}
		})
	}
}

func runGoalAssessmentFixture(t *testing.T, f independentToolE2EFixture, kind string, completed bool, indexes []int) {
	t.Helper()
	original := f.app.Provider.HTTP.Transport
	goalRouter := newFakeProviderRouter().onGoalEvaluation(func(_ map[string]any) fakeProviderResult {
		snapshot := readProcessingGoalSnapshot(t, f)
		evaluations := []GoalEvaluationCandidate{}
		for _, entry := range snapshot.Goals {
			proof := ""
			for _, s := range snapshot.Sources {
				if s.Valid && s.CanSupportSuccess && ((kind == "acquisition" && s.Kind == "item") || (s.Kind == "outcome" && (kind != "acquisition" || len(arrayValue(s.Data["verified_item_ids"])) > 0))) {
					proof = s.Ref
					break
				}
			}
			if proof == "" {
				t.Error("assessment fixture lacks actual successful source")
				return fakeProviderResult{Status: 500}
			}
			judgments := []GoalCriterionJudgment{}
			for i, id := range entry.Goal.CriterionIDs {
				verdict := "unknown"
				refs := []string{}
				discourse := "uncertain"
				if len(indexes) == 0 || containsIntForGoalFixture(indexes, i) {
					verdict = "satisfied"
					refs = []string{proof}
					discourse = "domain_fact"
				}
				judgments = append(judgments, GoalCriterionJudgment{CriterionID: id, Verdict: verdict, Kind: kind, Subject: "domain", Discourse: discourse, EvidenceRefs: refs, Reason: "controlled assessment of real source"})
			}
			impact, wait := "progressed", "await missing evidence"
			if completed {
				impact, wait = "completed", ""
			}
			candidate := GoalEvaluationCandidate{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Judgments: judgments, Impact: impact, WaitCondition: wait}
			for _, review := range snapshot.Reviews {
				if review.GoalID == entry.GoalID {
					candidate.Review = &GoalReviewDecision{ReasonCategory: "progressed", Decision: "continue", Explanation: "本周期已有真实结果，按原标准继续", EvidenceRefs: []string{proof}}
				}
			}
			evaluations = append(evaluations, candidate)
		}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: evaluations, Plans: []GoalPlanCandidate{}})}
	})
	f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(request.Body)
		request.Body = io.NopCloser(bytes.NewReader(raw))
		payload := decodeObject(raw)
		if providerWireSchemaName(payload) == "goal_evaluation_v1" {
			return goalRouter.RoundTrip(request)
		}
		return original.RoundTrip(request)
	})}
	id := latestPendingGoalRequest(t, f)
	if result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, id); err != nil || stringValue(result["status"]) != "succeeded" {
		t.Fatalf("formal Goal assessment=%#v err=%v", result, err)
	}
}

func containsIntForGoalFixture(values []int, target int) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func TestMutualRelationshipGoalRequiresActualAcceptanceAndFrozenRevision(t *testing.T) {
	for _, tc := range []struct {
		name, response                string
		accepts, editDuringAssessment bool
	}{
		{"open_signal", "我想要女朋友", false, false},
		{"quotation", "朋友说过：我愿意和你交往", false, false},
		{"refusal", "我不愿意和你建立恋爱关系", false, false},
		{"acceptance", "我接受你的心意，我愿意和你建立恋爱关系", true, false},
		{"concurrent_relationship_edit", "我接受你的心意，我愿意和你建立恋爱关系", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			goalID := createDialogueGoalForClosure(t, f, []string{"摇光实际表达", "双方实际确认恋爱关系"})
			relationshipID := "mutual-relationship-" + f.suffix
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,role,metrics,revision) VALUES($1,$2,$3,'{"label":"friend"}','{}',4)`, relationshipID, f.fluctlightID, f.ownerID); err != nil {
				t.Fatal(err)
			}
			seedCognitiveProviderRole(t, f.ctx, f.repository, "mutual-model-"+f.suffix)
			turns, assessments := 0, 0
			f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("conversation_turn_response", func(payload map[string]any) fakeProviderResult {
				turns++
				ref := regexp.MustCompile(`goal:ctx_[a-f0-9]{32}`).FindString(jsonString(payload))
				if ref == "" {
					t.Errorf("mutual Main lacks Goal ref: %s", jsonString(payload))
				}
				text := "我喜欢你，希望与你建立恋爱关系，你可以自由决定。"
				if turns > 1 {
					text = "我听到了你的回应，尊重你的选择。"
				}
				return fakeProviderResult{Structured: map[string]any{"action_type": "reply", "response_intent": "回应实际关系话题", "visible_text": text, "influences": []any{}, "goal_event_candidates": []any{map[string]any{"goal_ref": ref, "reason": "相关真实对话"}}}}
			}).onGoalEvaluation(func(_ map[string]any) fakeProviderResult {
				assessments++
				snapshot := readProcessingGoalSnapshot(t, f)
				entry := snapshot.Goals[0]
				var selfRef, targetRef string
				for _, s := range snapshot.Sources {
					if s.Kind != "message" {
						continue
					}
					if s.SubjectActorID == f.fluctlightID && stringValue(s.Data["text"]) == "我喜欢你，希望与你建立恋爱关系，你可以自由决定。" {
						selfRef = s.Ref
					}
					if s.SubjectActorID == f.ownerID && stringValue(s.Data["text"]) == tc.response {
						targetRef = s.Ref
					}
				}
				if selfRef == "" {
					t.Error("actual expression absent")
				}
				accepted := assessments > 1 && tc.accepts
				judgments := []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{selfRef}, Reason: "正式发出的表达"}}
				verdict, discourse, impact, wait := "unknown", "uncertain", "progressed", "等待对方明确确认，尊重拒绝"
				refs := []string{}
				if accepted {
					verdict, discourse, impact, wait = "satisfied", "assertion", "completed", ""
					refs = []string{selfRef, targetRef}
				}
				judgments = append(judgments, GoalCriterionJudgment{CriterionID: entry.Goal.CriterionIDs[1], Verdict: verdict, Kind: "relationship", Subject: "both", Discourse: discourse, EvidenceRefs: refs, Reason: "双方确认与单方表达分别判断"})
				candidate := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Judgments: judgments, Impact: impact, WaitCondition: wait}
				if accepted {
					candidate.RelationshipConfirmation = &GoalRelationshipConfirmation{TargetActorID: f.ownerID, EvidenceRefs: refs, Label: "恋人"}
					if tc.editDuringAssessment {
						if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.relationships SET revision=revision+1,role='{"label":"owner revised"}' WHERE id=$1`, relationshipID); err != nil {
							t.Fatal(err)
						}
					}
				}
				return fakeProviderResult{Structured: goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}})}
			})}
			for i, text := range []string{"请告诉我你的心意", tc.response} {
				turn, err := f.app.HandleTurn(f.ctx, f.ownerID, f.conversationID, map[string]any{"fluctlight_id": f.fluctlightID, "text": text, "idempotency_key": fmt.Sprintf("mutual-turn-%s-%d", f.suffix, i)})
				if err != nil {
					t.Fatal(err)
				}
				if stringValue(turn.Assistant["id"]) == "" {
					var reason *string
					_ = f.repository.Pool().QueryRow(f.ctx, `SELECT error_code FROM public.cognition_inbox WHERE fluctlight_id=$1 ORDER BY created_at DESC LIMIT 1`, f.fluctlightID).Scan(&reason)
					t.Fatalf("mutual turn not published i=%d reason=%v turn=%#v", i, reason, turn)
				}
				var requestID string
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_evaluation_requests WHERE fluctlight_id=$1 AND status IN ('pending','retry') ORDER BY created_at DESC LIMIT 1`, f.fluctlightID).Scan(&requestID); err != nil {
					var candidates []byte
					_ = f.repository.Pool().QueryRow(f.ctx, `SELECT payload->'goal_event_candidates' FROM public.cognition_assessments WHERE fluctlight_id=$1 ORDER BY created_at DESC LIMIT 1`, f.fluctlightID).Scan(&candidates)
					var cause *string
					_ = f.repository.Pool().QueryRow(f.ctx, `SELECT payload->>'safe_cause' FROM public.diagnostic_events WHERE fluctlight_id=$1 AND event_type='agent.run.termination' ORDER BY created_at DESC LIMIT 1`, f.fluctlightID).Scan(&cause)
					text := ""
					if cause != nil {
						text = *cause
					}
					t.Fatalf("mutual assessment absent i=%d turns=%d assessments=%d candidates=%s cause=%s err=%v", i, turns, assessments, candidates, text, err)
				}
				_, err = f.app.ProcessGoalEvaluationIntent(f.ctx, requestID)
				if i == 1 && tc.editDuringAssessment {
					if !errors.Is(err, ErrConflict) {
						t.Fatalf("stale relationship evaluation: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				var state string
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&state); err != nil {
					t.Fatal(err)
				}
				want := "active"
				if i == 1 && tc.accepts && !tc.editDuringAssessment {
					want = "completed"
				}
				if state != want {
					t.Fatalf("expression/acceptance state=%s want=%s", state, want)
				}
			}
			var revision, resolutions int
			var label string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision,role->>'label' FROM public.relationships WHERE id=$1`, relationshipID).Scan(&revision, &label); err != nil {
				t.Fatal(err)
			}
			wantRevision, wantLabel := 4, "friend"
			if tc.accepts {
				wantRevision = 5
				wantLabel = "恋人"
			}
			if tc.editDuringAssessment {
				wantLabel = "owner revised"
			}
			if revision != wantRevision || label != wantLabel {
				t.Fatalf("relationship overwritten: %d %s", revision, label)
			}
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1`, goalID).Scan(&resolutions); err != nil {
				t.Fatal(err)
			}
			wantResolutions := 0
			if tc.accepts && !tc.editDuringAssessment {
				logGoalClosureEvidence(t, f, goalID, "mutual_relationship_confirmation")
				wantResolutions = 1
			}
			if resolutions != wantResolutions {
				t.Fatalf("resolutions=%d", resolutions)
			}
			if tc.accepts && !tc.editDuringAssessment {
				// A distinct delivery identity after terminal resolution must not
				// repeat relationship mutation or call the assessment model again.
				replayID := "relationship-redelivery-" + f.suffix
				if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,goal_ids,reason,status) VALUES($1,$2,'default',$3,'duplicate_source_delivery','pending')`, replayID, f.fluctlightID, jsonBytes([]string{goalID})); err != nil {
					t.Fatal(err)
				}
				if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, replayID); err != nil {
					t.Fatal(err)
				}
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision,role->>'label' FROM public.relationships WHERE id=$1`, relationshipID).Scan(&revision, &label); err != nil {
					t.Fatal(err)
				}
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1`, goalID).Scan(&resolutions); err != nil {
					t.Fatal(err)
				}
				var evaluatedEvents int
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_id=$1 AND kind='goal.evaluated'`, goalID).Scan(&evaluatedEvents); err != nil {
					t.Fatal(err)
				}
				if revision != wantRevision || label != wantLabel || resolutions != 1 || assessments != 2 || evaluatedEvents != 2 {
					t.Fatalf("distinct redelivery duplicated effects: relation=%d/%s resolutions=%d calls=%d events=%d", revision, label, resolutions, assessments, evaluatedEvents)
				}
			}
		})
	}
}

func TestGoalEvaluationRequiresConsistentCompletionImpact(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	goal := GoalAuthority{EntityID: "recommendation", SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", FluctlightID: "self", Scope: "general", DesiredOutcome: "根据阅读偏好推荐小说", SuccessCriteria: []string{"具体推荐并说明理由", "可选补充"}, CriterionIDs: []string{"criterion_required", "criterion_optional"}, CriteriaVersion: 1, Motivation: "分享兴趣", Status: GoalActive, Revision: 1, EvidenceRefs: []string{"owner:goal"}}
	source := GoalSource{Ref: "source:1", EventID: 1, Kind: "message", ID: "recommendation", Version: "v1", FluctlightID: "self", SubjectActorID: "self", Valid: true, CanSupportSuccess: true, Data: map[string]any{"text": "你喜欢科幻，推荐特德姜，语言与时间的设定很精彩。", "message_kind": "assistant"}}
	sources := map[string]GoalSource{source.Ref: source}
	for _, mode := range []string{"all", "any"} {
		goal.CriteriaPolicy = map[string]any{"mode": mode, "optional_ids": []any{"criterion_optional"}}
		for _, impact := range []string{"progressed", "no_change", "blocked", "regressed", "needs_evidence", "completed"} {
			t.Run(mode+"/"+impact, func(t *testing.T) {
				candidate := GoalEvaluationCandidate{GoalID: goal.EntityID, ExpectedRevision: 1, CriteriaVersion: 1, Impact: impact, Judgments: []GoalCriterionJudgment{{CriterionID: "criterion_required", Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{source.Ref}, Reason: "正式推荐"}}}
				next, _, _, err := ApplyGoalEvaluation(goal, candidate, sources, at)
				if impact != "completed" {
					if err == nil || err.Error() != "goal_evaluation_completion_impact_mismatch" {
						t.Fatalf("contradictory completion accepted: %v", err)
					}
					return
				}
				if err != nil || next.Status != GoalCompleted || next.Progress != 1 {
					t.Fatalf("valid completion failed: %#v %v", next, err)
				}
				paused := goal
				paused.Status = GoalPaused
				next, _, _, err = ApplyGoalEvaluation(paused, candidate, sources, at)
				if err != nil || next.Status != GoalPaused {
					t.Fatalf("paused lifecycle overridden: %#v %v", next, err)
				}
			})
		}
	}
}
