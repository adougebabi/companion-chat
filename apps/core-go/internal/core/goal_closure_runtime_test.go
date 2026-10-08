package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
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
	}).on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		assessments++
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
		return fakeProviderResult{Structured: map[string]any{"evaluations": []any{map[string]any{"goal_id": goal.GoalID, "expected_revision": goal.Goal.Revision, "criteria_version": goal.Goal.CriteriaVersion, "judgments": []any{map[string]any{"criterion_id": goal.Goal.CriterionIDs[0], "verdict": "satisfied", "kind": "communication", "subject": "actor_self", "discourse": "assertion", "evidence_refs": []string{expression.Ref}, "reason": "正式落库的实际表达满足原标准，不要求对方接受"}}, "impact": "completed", "blocker": "", "wait_condition": "", "next_step": "", "residual_motivation": ""}}, "plans": []any{}}}
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
	f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(request.Body)
		request.Body = io.NopCloser(bytes.NewReader(raw))
		payload := decodeObject(raw)
		if stringValue(mapValue(mapValue(payload["response_format"])["json_schema"])["name"]) != "goal_evaluation_v1" {
			return original.RoundTrip(request)
		}
		snapshot := readProcessingGoalSnapshot(t, f)
		evaluations := []any{}
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
				return embeddingHTTPResponse(request, 500, "{}"), nil
			}
			judgments := []any{}
			for i, id := range entry.Goal.CriterionIDs {
				verdict := "unknown"
				refs := []string{}
				discourse := "uncertain"
				if len(indexes) == 0 || containsIntForGoalFixture(indexes, i) {
					verdict = "satisfied"
					refs = []string{proof}
					discourse = "domain_fact"
				}
				judgments = append(judgments, map[string]any{"criterion_id": id, "verdict": verdict, "kind": kind, "subject": "domain", "discourse": discourse, "evidence_refs": refs, "reason": "controlled assessment of real source"})
			}
			impact, wait := "progressed", "await missing evidence"
			if completed {
				impact, wait = "completed", ""
			}
			candidate := map[string]any{"goal_id": entry.GoalID, "expected_revision": entry.Goal.Revision, "criteria_version": entry.Goal.CriteriaVersion, "judgments": judgments, "impact": impact, "blocker": "", "wait_condition": wait, "next_step": "", "residual_motivation": ""}
			for _, review := range snapshot.Reviews {
				if review.GoalID == entry.GoalID {
					candidate["review"] = map[string]any{"reason_category": "progressed", "decision": "continue", "explanation": "本周期已有真实结果，按原标准继续", "evidence_refs": []string{proof}, "stage_id": "", "feasible_alternative": ""}
				}
			}
			evaluations = append(evaluations, candidate)
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(map[string]any{"evaluations": evaluations, "plans": []any{}})}}}}
		return embeddingHTTPResponse(request, 200, jsonString(response)), nil
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
			}).on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
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
				judgments := []any{map[string]any{"criterion_id": entry.Goal.CriterionIDs[0], "verdict": "satisfied", "kind": "communication", "subject": "actor_self", "discourse": "assertion", "evidence_refs": []string{selfRef}, "reason": "正式发出的表达"}}
				verdict, discourse, impact, wait := "unknown", "uncertain", "progressed", "等待对方明确确认，尊重拒绝"
				refs := []string{}
				if accepted {
					verdict, discourse, impact, wait = "satisfied", "assertion", "completed", ""
					refs = []string{selfRef, targetRef}
				}
				judgments = append(judgments, map[string]any{"criterion_id": entry.Goal.CriterionIDs[1], "verdict": verdict, "kind": "relationship", "subject": "both", "discourse": discourse, "evidence_refs": refs, "reason": "双方确认与单方表达分别判断"})
				candidate := map[string]any{"goal_id": goalID, "expected_revision": entry.Goal.Revision, "criteria_version": entry.Goal.CriteriaVersion, "judgments": judgments, "impact": impact, "wait_condition": wait, "blocker": "", "next_step": "", "residual_motivation": ""}
				if accepted {
					candidate["relationship_confirmation"] = map[string]any{"target_actor_id": f.ownerID, "evidence_refs": refs, "label": "恋人"}
					if tc.editDuringAssessment {
						if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.relationships SET revision=revision+1,role='{"label":"owner revised"}' WHERE id=$1`, relationshipID); err != nil {
							t.Fatal(err)
						}
					}
				}
				return fakeProviderResult{Structured: map[string]any{"evaluations": []any{candidate}, "plans": []any{}}}
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
