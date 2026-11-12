package core

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// A disposable database and controlled native Provider verify the formal
// direct reply and Goal worker entry; this does not claim live model or
// Temporal execution. The test advances only its own quiet-window timestamps.
func TestDirectReplyGoalCandidateReachesActualSettlement(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	criterion := "已实际清楚向对方表达自己的心意"
	goalID := createDialogueGoalForClosure(t, f, []string{criterion})
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, f.ownerID, f.fluctlightID, f.conversationID); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, f.ctx, f.repository, "direct")
	seedCognitiveProviderRole(t, f.ctx, f.repository, "m2-direct-goal-"+f.suffix)
	var mainCalls, assessments int
	var messageID string
	router := newFakeProviderRouter().onGoalEvaluation(func(payload map[string]any) fakeProviderResult {
		assessments++
		snapshot := readProcessingGoalSnapshot(t, f)
		if len(snapshot.Goals) != 1 || snapshot.Goals[0].GoalID != goalID || len(snapshot.Goals[0].Goal.SuccessCriteria) != 1 || snapshot.Goals[0].Goal.SuccessCriteria[0] != criterion {
			t.Fatalf("evaluation lost original Goal conditions: %#v", snapshot.Goals)
		}
		var expression GoalSource
		for _, source := range snapshot.Sources {
			if source.Kind == "message" && source.ID == messageID && source.SubjectActorID == f.fluctlightID {
				expression = source
			}
		}
		if !expression.Valid || !expression.CanSupportSuccess || expression.ID == "" || !strings.Contains(jsonString(expression.Data), "我很在意你") {
			t.Fatalf("actual committed reply was not available as Goal evidence: %#v", snapshot.Sources)
		}
		goal := snapshot.Goals[0]
		output := GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{{GoalID: goal.GoalID, ExpectedRevision: goal.Goal.Revision, CriteriaVersion: goal.Goal.CriteriaVersion, Judgments: []GoalCriterionJudgment{{CriterionID: goal.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{expression.Ref}, Reason: "正式消息是已提交的实际表达，满足原成功条件"}}, Impact: "completed"}}, Plans: []GoalPlanCandidate{}}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(snapshot, output)}
	}).otherwise(func(payload map[string]any) fakeProviderResult {
		mainCalls++
		goalRef := regexp.MustCompile(`goal:ctx_[a-f0-9]{32}`).FindString(jsonString(payload))
		if goalRef == "" {
			t.Error("direct main request omitted its actual Goal reference")
			return fakeProviderResult{Status: http.StatusBadRequest}
		}
		decision := map[string]any{"action_type": "reply", "response_intent": "表达自己的心意", "influences": []any{map[string]any{"ref": goalRef, "role": "motivates", "confidence": 1.0, "note": "当前表达目标"}}, "goal_event_candidates": []any{map[string]any{"goal_ref": goalRef, "reason": "正式表达可能满足目标条件"}}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("m2-goal-reply", conversationReplyCapabilityName, map[string]any{"text": "我很在意你，想认真告诉你。", "decision": decision})}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	payload := map[string]any{"fluctlight_id": f.fluctlightID, "text": "你现在想对我说什么？", "turn_id": "m2-direct-goal", "idempotency_key": "m2-direct-goal", "attachment_refs": []any{}}
	turn, err := f.app.HandleTurn(f.ctx, f.ownerID, f.conversationID, payload)
	messageID = stringValue(turn.Assistant["id"])
	if err != nil || messageID == "" || mainCalls != 1 {
		t.Fatalf("direct Goal reply: %#v main=%d err=%v", turn.Assistant, mainCalls, err)
	}
	var requestID, state string
	var candidates, workflows, resolutions int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT g.status,
		(SELECT count(*) FROM public.goal_evidence_links l JOIN public.goal_source_events e ON e.id=l.source_event_id WHERE l.goal_id=$1 AND e.source_id=$2 AND e.source_kind='message' AND e.source_status='committed' AND l.status='candidate'),
		(SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1),
		(SELECT count(*) FROM public.goal_evaluation_requests r JOIN public.platform_workflow_intents i ON i.intent_id=r.id AND i.intent_type='goal.evaluate' WHERE r.fluctlight_id=$3 AND r.reason='conversation_candidate' AND r.status='pending' AND r.goal_ids ? $1)
		FROM public.fluctlight_goals g WHERE g.id=$1`, goalID, messageID, f.fluctlightID).Scan(&state, &candidates, &resolutions, &workflows); err != nil || state != "active" || candidates != 1 || resolutions != 0 || workflows != 1 {
		t.Fatalf("candidate was not safely persisted before evaluation: state=%s candidates=%d resolutions=%d workflows=%d err=%v", state, candidates, resolutions, workflows, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_evaluation_requests WHERE fluctlight_id=$1 AND reason='conversation_candidate' AND status='pending' AND goal_ids ? $2`, f.fluctlightID, goalID).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	deferred, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID)
	if err != nil || deferred["status"] != "deferred" || deferred["reason"] != "evaluation_cadence" || assessments != 0 {
		t.Fatalf("Goal quiet window was not preserved: result=%#v calls=%d err=%v", deferred, assessments, err)
	}
	// Simulate the existing quiet window elapsing only in this isolated DB.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET available_at=now() WHERE id=$1`, requestID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_agent_cadence SET last_activity_at=now()-interval '31 minutes',evaluation_next_at=now()-interval '1 minute' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID)
	if err != nil || result["status"] != "succeeded" || assessments != 1 {
		t.Fatalf("Goal assessment=%#v calls=%d err=%v", result, assessments, err)
	}
	check := func() {
		t.Helper()
		var messages, evaluations, evidence, resolutions int
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT g.status,
			(SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$2 AND kind='assistant'),
			(SELECT count(*) FROM public.goal_evaluations WHERE goal_id=$1 AND stage_id IS NULL AND commitment_id IS NULL),
			(SELECT count(*) FROM public.goal_evidence_links l JOIN public.goal_source_events e ON e.id=l.source_event_id WHERE l.goal_id=$1 AND e.source_id=$3 AND e.source_kind='message' AND l.status='confirmed'),
			(SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1)
			FROM public.fluctlight_goals g WHERE g.id=$1`, goalID, f.conversationID, messageID).Scan(&state, &messages, &evaluations, &evidence, &resolutions); err != nil || state != "completed" || messages != 1 || evaluations != 1 || evidence != 1 || resolutions != 1 {
			t.Fatalf("Goal settlement state=%s messages=%d evaluations=%d evidence=%d resolutions=%d err=%v", state, messages, evaluations, evidence, resolutions, err)
		}
	}
	check()
	if _, err := f.app.HandleTurn(f.ctx, f.ownerID, f.conversationID, payload); err != nil || mainCalls != 1 {
		t.Fatalf("direct reply replay repeated model call: calls=%d err=%v", mainCalls, err)
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID); err != nil || assessments != 1 {
		t.Fatalf("Goal evaluation replay repeated model call: calls=%d err=%v", assessments, err)
	}
	check()
}
