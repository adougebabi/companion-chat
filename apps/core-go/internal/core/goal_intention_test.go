package core

import (
	"strings"
	"testing"
	"time"
)

func TestGoalIntentionStageS07(t *testing.T) {
	goalRef := "goal:ctx_" + strings.Repeat("a", 32)
	intentionRef := "intention:ctx_" + strings.Repeat("b", 32)
	eventRef := "scene:ctx_" + strings.Repeat("c", 32)
	outcomeRef := "outcome:ctx_" + strings.Repeat("d", 32)
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	goal := GoalAuthority{
		SchemaVersion: goalAuthoritySchemaVersion, Ref: goalRef, FluctlightID: "fl-s07", ProfileID: "default",
		DesiredOutcome: "完成可靠的目标闭环", SuccessCriteria: []string{"真实动作成功", "结果得到验证"}, Motivation: "持续完成承诺",
		Scope: "project", Importance: 0.8, Urgency: 0.6, Progress: 0, Status: GoalActive, Revision: 1, EvidenceRefs: []string{"fact:goal"},
	}
	intention := IntentionAuthority{
		SchemaVersion: intentionAuthoritySchemaVersion, Ref: intentionRef, FluctlightID: "fl-s07", ProfileID: "default", GoalRef: goalRef,
		ActionIntent: "执行一次受控能力调用", ExpectedOutcome: "能力产生可验证结果", CapabilityConstraints: []string{"schedule.replan"},
		Trigger:    TypedIntentionTrigger{Type: IntentionTriggerEvent, EventType: "life.event.created", EventRef: eventRef},
		Expiration: now.Add(24 * time.Hour), Confidence: 0.9, Status: IntentionQualified, Revision: 1, EvidenceRefs: []string{"fact:intention"},
	}

	t.Run("goal full-field update and lifecycle CAS", func(t *testing.T) {
		importance, urgency := 0.9, 0.7
		outcome, motivation, scope := "完成可审计闭环", "履行长期计划", "engineering"
		criteria := []string{"能力成功", "结果被下一轮消费"}
		deadline := now.Add(48 * time.Hour)
		updated, record, err := ApplyGoalCommand(&goal, GoalCommand{
			Operation: GoalUpdate, ExpectedRevision: 1, OccurredAt: now, EvidenceRefs: []string{"fact:update"}, Reason: "refine goal",
			Patch: GoalPatch{DesiredOutcome: &outcome, SuccessCriteria: criteria, Motivation: &motivation, Scope: &scope, Importance: &importance, Urgency: &urgency, Deadline: &deadline},
		})
		if err != nil || updated.Revision != 2 || updated.DesiredOutcome != outcome || updated.Motivation != motivation || updated.Scope != scope || updated.Importance != importance || updated.Urgency != urgency || updated.Deadline == nil || len(updated.SuccessCriteria) != 2 {
			t.Fatalf("goal update=%#v record=%#v err=%v", updated, record, err)
		}
		if record.BaseRevision != 1 || record.Revision != 2 || record.FromStatus != GoalActive || record.ToStatus != GoalActive {
			t.Fatalf("goal governance=%#v", record)
		}
		if _, _, err := ApplyGoalCommand(&updated, GoalCommand{Operation: GoalPause, ExpectedRevision: 1, OccurredAt: now, EvidenceRefs: []string{"fact:stale"}}); err == nil {
			t.Fatal("stale Goal command crossed CAS")
		}
		paused, _, err := ApplyGoalCommand(&updated, GoalCommand{Operation: GoalPause, ExpectedRevision: 2, OccurredAt: now, EvidenceRefs: []string{"fact:pause"}})
		if err != nil || paused.Status != GoalPaused {
			t.Fatalf("pause=%#v err=%v", paused, err)
		}
		resumed, _, err := ApplyGoalCommand(&paused, GoalCommand{Operation: GoalResume, ExpectedRevision: paused.Revision, OccurredAt: now, EvidenceRefs: []string{"fact:resume"}})
		if err != nil || resumed.Status != GoalActive {
			t.Fatalf("resume=%#v err=%v", resumed, err)
		}
	})

	t.Run("typed triggers create stable due facts without semantic matching", func(t *testing.T) {
		fact, due, err := EvaluateIntentionDue(intention, IntentionTriggerObservation{At: now, EventType: "life.event.created", EventRef: eventRef})
		if err != nil || !due || fact.EventType != intentionDueFactType || fact.AttemptID == "" || fact.IntentionRef != intentionRef || fact.GoalRef != goalRef {
			t.Fatalf("event due=%#v due=%v err=%v", fact, due, err)
		}
		replay, replayDue, err := EvaluateIntentionDue(intention, IntentionTriggerObservation{At: now.Add(time.Minute), EventType: "life.event.created", EventRef: eventRef})
		if err != nil || !replayDue || replay.ID != fact.ID || replay.AttemptID != fact.AttemptID {
			t.Fatalf("due identity drifted: first=%#v replay=%#v err=%v", fact, replay, err)
		}
		_, due, err = EvaluateIntentionDue(intention, IntentionTriggerObservation{At: now, EventType: "life.event.created", EventRef: "scene:ctx_" + strings.Repeat("e", 32)})
		if err != nil || due {
			t.Fatalf("wrong exact Event triggered intention: due=%v err=%v", due, err)
		}
		semantic := intention
		semantic.Trigger = TypedIntentionTrigger{Type: IntentionTriggerSemantic}
		if _, due, err = EvaluateIntentionDue(semantic, IntentionTriggerObservation{At: now, EventType: "message", NewFactRef: ""}); err != nil || due {
			t.Fatalf("semantic trigger inferred from event text: due=%v err=%v", due, err)
		}
		if _, due, err = EvaluateIntentionDue(semantic, IntentionTriggerObservation{At: now, NewFactRef: "fact:new"}); err != nil || !due {
			t.Fatalf("new fact did not re-enter semantic trigger: due=%v err=%v", due, err)
		}
	})

	t.Run("freeze requires served refs and all deterministic gates", func(t *testing.T) {
		due, ok, err := EvaluateIntentionDue(intention, IntentionTriggerObservation{At: now, EventType: "life.event.created", EventRef: eventRef})
		if err != nil || !ok {
			t.Fatalf("due=%#v ok=%v err=%v", due, ok, err)
		}
		influences := []DecisionInfluence{{Ref: goalRef, Role: "motivates", Confidence: 0.9, Note: "goal"}, {Ref: intentionRef, Role: "grounds", Confidence: 0.9, Note: "intention"}}
		gate := AgencyExecutionGate{Now: now, PermissionAllowed: true, BudgetAvailable: true, FoundationRevision: 3, CurrentStateRevision: 4, LifeContextRevision: "life_ctx_s07"}
		frozen, err := FreezeIntentionAction(goal, intention, due, "schedule.replan", influences, gate)
		if err != nil || frozen.ActionID == "" || frozen.OutcomeID == "" || frozen.AttemptID != due.AttemptID || frozen.ExpectedRevisions["life_context_revision"] != "life_ctx_s07" {
			t.Fatalf("frozen=%#v err=%v", frozen, err)
		}
		replay, err := FreezeIntentionAction(goal, intention, due, "schedule.replan", influences, gate)
		if err != nil || replay.ActionID != frozen.ActionID || replay.OutcomeID != frozen.OutcomeID {
			t.Fatalf("frozen replay drifted: first=%#v replay=%#v err=%v", frozen, replay, err)
		}
		if _, err := FreezeIntentionAction(goal, intention, due, "schedule.replan", influences[:1], gate); err == nil {
			t.Fatal("action without Intention influence was frozen")
		}
		blocked := gate
		blocked.QuietHoursBlocked = true
		if _, err := FreezeIntentionAction(goal, intention, due, "schedule.replan", influences, blocked); err == nil {
			t.Fatal("quiet-hours action was frozen")
		}
	})

	t.Run("outcome settles one attempt and only success advances Goal", func(t *testing.T) {
		due, _, _ := EvaluateIntentionDue(intention, IntentionTriggerObservation{At: now, EventType: "life.event.created", EventRef: eventRef})
		influences := []DecisionInfluence{{Ref: goalRef, Role: "motivates", Confidence: 0.9}, {Ref: intentionRef, Role: "grounds", Confidence: 0.9}}
		frozen, err := FreezeIntentionAction(goal, intention, due, "schedule.replan", influences, AgencyExecutionGate{Now: now, PermissionAllowed: true, BudgetAvailable: true, FoundationRevision: 1, CurrentStateRevision: 1, LifeContextRevision: "life_ctx_s07"})
		if err != nil {
			t.Fatal(err)
		}
		makeOutcome := func(status ActionOutcomeStatus) ActionOutcome {
			return ActionOutcome{SchemaVersion: actionOutcomeSchemaVersion, ID: frozen.OutcomeID, FluctlightID: "fl-s07", ActionID: frozen.ActionID, CallID: actionPrimaryCallID, CapabilityName: frozen.CapabilityName, Status: status, SuccessBoundary: "schedule_version_committed", Expected: map[string]any{}, Observed: map[string]any{}, GoalRefs: []string{goalRef}, IntentionRefs: []string{intentionRef}, EvidenceRefs: []string{"fact:outcome"}, ContextReferences: map[string]ContextReference{}, Revision: 1, OccurredAt: now}
		}
		failedSettlement, err := SettleIntentionAttempt(intention, frozen, makeOutcome(ActionOutcomeFailed), nil)
		if err != nil || failedSettlement.Intention.Status != IntentionQualified || failedSettlement.Attempt.Status != IntentionAttemptFailed {
			t.Fatalf("failed settlement=%#v err=%v", failedSettlement, err)
		}
		if _, _, err := ApplyGoalProgress(goal, GoalProgressProposal{GoalRef: goalRef, OutcomeRefs: []string{outcomeRef}, CriterionIndexes: []int{0}, Strength: 1, Confidence: 1, OccurredAt: now}, map[string]ActionOutcome{outcomeRef: makeOutcome(ActionOutcomeFailed)}); err == nil {
			t.Fatal("failed outcome advanced Goal")
		}
		success := makeOutcome(ActionOutcomeCompleted)
		succeeded, err := SettleIntentionAttempt(intention, frozen, success, nil)
		if err != nil || succeeded.Intention.Status != IntentionCompleted || succeeded.Attempt.Status != IntentionAttemptSucceeded {
			t.Fatalf("success settlement=%#v err=%v", succeeded, err)
		}
		replayed, err := SettleIntentionAttempt(intention, frozen, success, &succeeded.Attempt)
		if err != nil || !replayed.Replayed || replayed.Attempt.AttemptID != succeeded.Attempt.AttemptID {
			t.Fatalf("attempt replay=%#v err=%v", replayed, err)
		}
		progressed, record, err := ApplyGoalProgress(goal, GoalProgressProposal{GoalRef: goalRef, OutcomeRefs: []string{outcomeRef}, CriterionIndexes: []int{0, 1}, Strength: 1, Confidence: 1, Complete: true, EvidenceRefs: []string{"fact:assessment"}, OccurredAt: now}, map[string]ActionOutcome{outcomeRef: success})
		if err != nil || progressed.Progress != 1 || progressed.Status != GoalCompleted || record.Revision != 2 {
			t.Fatalf("progressed=%#v record=%#v err=%v", progressed, record, err)
		}
	})
}
