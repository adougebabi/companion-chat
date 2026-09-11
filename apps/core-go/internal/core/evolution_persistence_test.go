package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestEvolutionPersistenceS10(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	app := &App{DB: repository}
	now := time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC)
	ownerID, fluctlightID := "s10-owner", "s10-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return app.insertAgency(ctx, tx, fluctlightID, ownerID,
			[]any{map[string]any{"description": "初始化目标", "importance": 0.6, "urgency": 0.4}},
			[]any{map[string]any{"action": "等待新事实后重新评估", "goal_index": 0, "confidence": 0.7}},
			"default", map[string]struct{}{"default": {}})
	}); err != nil {
		t.Fatalf("0031 rejected initial Goal/Intention authority: %v", err)
	}
	var initialGoalNeedsReflection bool
	var initialIntentionStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT needs_reflection FROM public.fluctlight_goals WHERE id=$1`, "goal_initial_"+fluctlightID+"_0").Scan(&initialGoalNeedsReflection); err != nil || !initialGoalNeedsReflection {
		t.Fatalf("initial Goal did not preserve missing criteria as needs_reflection: value=%v err=%v", initialGoalNeedsReflection, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, "intention_initial_"+fluctlightID+"_0").Scan(&initialIntentionStatus); err != nil || initialIntentionStatus != "candidate" {
		t.Fatalf("initial Intention status=%q err=%v", initialIntentionStatus, err)
	}
	goalRef := "goal:ctx_" + strings.Repeat("a", 32)
	intentionRef := "intention:ctx_" + strings.Repeat("b", 32)
	eventRef := "scene:ctx_" + strings.Repeat("c", 32)
	profileRef := "personality:ctx_" + strings.Repeat("d", 32)

	goal, goalRecord, err := CreateGoalAuthority(GoalAuthority{
		EntityID: "goal-s10", SchemaVersion: goalAuthoritySchemaVersion, Ref: goalRef, FluctlightID: fluctlightID, ProfileID: "default",
		DesiredOutcome: "完成 S10 authority", SuccessCriteria: []string{"事务提交", "replay稳定"}, Motivation: "完成闭环",
		Scope: "project", Importance: 0.8, Urgency: 0.7, Progress: 0, Status: GoalActive, Revision: 1, EvidenceRefs: []string{"fact:goal"},
	}, []string{"fact:goal"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		replayed, err := persistGoalAuthorityTx(ctx, tx, nil, goal, goalRecord, "goal:create:s10")
		if replayed {
			return errors.New("first Goal persistence replayed")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		replayed, err := persistGoalAuthorityTx(ctx, tx, nil, goal, goalRecord, "goal:create:s10")
		if err == nil && !replayed {
			return errors.New("Goal replay did not reuse command")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}

	intention, intentionRecord, err := CreateIntentionAuthority(IntentionAuthority{
		EntityID: "intention-s10", GoalEntityID: goal.EntityID, SchemaVersion: intentionAuthoritySchemaVersion, Ref: intentionRef,
		FluctlightID: fluctlightID, ProfileID: "default", GoalRef: goalRef, ActionIntent: "重排日程", ExpectedOutcome: "提交新日程版本",
		CapabilityConstraints: []string{"schedule.replan"}, Trigger: TypedIntentionTrigger{Type: IntentionTriggerEvent, EventType: "life.event.created", EventRef: eventRef},
		Expiration: now.Add(24 * time.Hour), Confidence: 0.9, Status: IntentionQualified, Revision: 1, EvidenceRefs: []string{"fact:intention"},
	}, []string{"fact:intention"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistIntentionAuthorityTx(ctx, tx, nil, intention, intentionRecord, "intention:create:s10")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	due, dueNow, err := EvaluateIntentionDue(intention, IntentionTriggerObservation{At: now, EventType: "life.event.created", EventRef: eventRef})
	if err != nil || !dueNow {
		t.Fatalf("due=%#v now=%v err=%v", due, dueNow, err)
	}
	var dueIntention IntentionAuthority
	var persistedDue IntentionDueFact
	var dueInbox string
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		var replayed bool
		var err error
		dueIntention, persistedDue, dueInbox, replayed, err = persistIntentionDueFactTx(ctx, tx, app, intention, due)
		if replayed {
			return errors.New("first due fact replayed")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if dueInbox == "" || dueIntention.Status != IntentionDue || dueIntention.Revision != 2 || persistedDue.IntentionRevision != 2 {
		t.Fatalf("due intention=%#v fact=%#v inbox=%q", dueIntention, persistedDue, dueInbox)
	}
	restartedApp := &App{DB: repository}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		replayedIntention, replayedDue, replayedInbox, replayed, err := persistIntentionDueFactTx(ctx, tx, restartedApp, intention, due)
		if err == nil && (!replayed || replayedInbox != dueInbox || replayedIntention.Revision != dueIntention.Revision || replayedDue.IntentionRevision != persistedDue.IntentionRevision) {
			return errors.New("process restart duplicated or changed Intention due fact")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	influences := []DecisionInfluence{{Ref: goalRef, Role: "motivates", Confidence: 0.9}, {Ref: intentionRef, Role: "grounds", Confidence: 0.9}}
	frozen, err := FreezeIntentionAction(goal, dueIntention, persistedDue, "schedule.replan", influences, AgencyExecutionGate{Now: now, PermissionAllowed: true, BudgetAvailable: true, FoundationRevision: 0, CurrentStateRevision: 0, LifeContextRevision: "life_ctx_s10"})
	if err != nil {
		t.Fatal(err)
	}
	outcome := ActionOutcome{SchemaVersion: actionOutcomeSchemaVersion, ID: frozen.OutcomeID, FluctlightID: fluctlightID, ActionID: frozen.ActionID, CallID: actionPrimaryCallID, CapabilityName: frozen.CapabilityName, Status: ActionOutcomeCompleted, SuccessBoundary: "schedule_version_committed", Expected: map[string]any{}, Observed: map[string]any{"status": "accepted"}, GoalRefs: []string{goalRef}, IntentionRefs: []string{intentionRef}, EvidenceRefs: []string{"fact:outcome"}, ContextReferences: map[string]ContextReference{}, Revision: 1, OccurredAt: now}
	settlement, err := SettleIntentionAttempt(dueIntention, frozen, outcome, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistIntentionAttemptSettlementTx(ctx, tx, dueIntention, settlement)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		replayed, err := persistIntentionAttemptSettlementTx(ctx, tx, dueIntention, settlement)
		if err == nil && !replayed {
			return errors.New("Intention attempt replay did not reuse result")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}

	projection := ContextProjection{FluctlightID: fluctlightID, OwnerActorID: ownerID, ConversationID: "conversation-s10", CurrentSpeaker: map[string]any{"actor_id": ownerID}, PersonalityRuntime: map[string]any{"active_profile_id": "default"}}
	index := newContextReferenceIndex(projection)
	goalContextRef, _ := index.add(ContextReferenceGoal, goal.EntityID, goal.Revision, map[string]any{"desired_outcome": goal.DesiredOutcome})
	memoryContextRef, _ := index.add(ContextReferenceMemory, "memory-s10", 1, map[string]any{"content": "memory"})
	evolution, err := BuildEvolutionContext(EvolutionContext{
		ProviderRole: "reflection", FluctlightID: fluctlightID, SourceWindow: "window-s10", FromSequence: 1, ToSequence: 2, Watermark: 0,
		Evidence:       []EvolutionEvidence{{Ref: "fact:reflection:1", Kind: "conversation", Summary: "first fact", Sequence: 1, OccurredAt: now}, {Ref: "fact:reflection:2", Kind: "outcome", Summary: "second fact", Sequence: 2, OccurredAt: now}},
		ReferenceIndex: index, BaseRevisions: map[string]int{string(EvolutionMemory): 1, string(EvolutionGoal): 1, string(EvolutionAffectProfile): 0, string(EvolutionPersonality): 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal := ReflectionProposalV2{SchemaVersion: reflectionProposalV2SchemaVersion, Summary: "apply memory and goal", MemoryCandidates: []ReflectionMemoryCandidateV2{{Operation: "revise", TargetRef: memoryContextRef, Confidence: 0.9, Importance: 0.8, EvidenceRefs: []string{"fact:reflection:1"}, SemanticReason: "new evidence"}}, GoalCandidates: []ReflectionGoalCandidateV2{{Operation: "update", TargetRef: goalContextRef, Direction: "increase", Strength: 0.8, Confidence: 0.9, EvidenceRefs: []string{"fact:reflection:2"}, SemanticReason: "outcome"}}}
	plan, err := CompileReflectionPlan(proposal, evolution, ReflectionPolicyV2{}, now)
	if err != nil {
		t.Fatal(err)
	}
	evolutionState := EvolutionState{FluctlightID: fluctlightID, Watermark: 0, Revisions: cloneRevisionMap(evolution.BaseRevisions)}
	nextEvolution, reflectionResult, err := ApplyReflectionPlan(evolutionState, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistReflectionApplyTx(ctx, tx, evolution, proposal, plan, nextEvolution, reflectionResult, profileRef, "model-s10", "prompt-s10")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		replayed, err := persistReflectionApplyTx(ctx, tx, evolution, proposal, plan, nextEvolution, reflectionResult, profileRef, "model-s10", "prompt-s10")
		if err == nil && !replayed {
			return errors.New("Reflection replay did not reuse proposal")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}

	baseline := PersonaEvolutionState{FluctlightID: fluctlightID, ProfileID: "default", ProfileRef: profileRef, Personality: map[string]any{"traits": map[string]any{"openness": 0.5}}, BehaviorPolicy: map[string]any{"communication": map[string]any{"tone": "克制"}}}
	loaded, err := loadPersonaEvolutionState(ctx, repository.Pool(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	candidate := ReflectionOverlayCandidateV2{FieldPath: "traits.openness", Direction: "increase", Strength: 1, Confidence: 0.9, EvidenceRefs: []string{"fact:reflection:1", "fact:reflection:2"}}
	decision, err := CompileEvolutionOverlay(loaded, EvolutionOverlayRequest{ExpectedRevision: loaded.Revision, Candidate: candidate, EvidenceWindows: []string{"window-a", "window-b"}, OccurredAt: now}, EvolutionOverlayPolicy{})
	if err != nil || decision.Overlay == nil {
		t.Fatalf("overlay decision=%#v err=%v", decision, err)
	}
	afterOverlay, err := ApplyEvolutionOverlay(loaded, decision)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistEvolutionOverlayTx(ctx, tx, loaded, afterOverlay, *decision.Overlay)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	loadedAfter, err := loadPersonaEvolutionState(ctx, repository.Pool(), baseline)
	if err != nil || len(loadedAfter.Overlays) != 1 {
		t.Fatalf("loaded overlay=%#v err=%v", loadedAfter, err)
	}
	rolledBack, rollbackOverlay, err := RollbackEvolutionOverlay(loadedAfter, decision.Overlay.ID, loadedAfter.Revision, []string{"owner:rollback"}, now.Add(25*time.Hour), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistEvolutionOverlayTx(ctx, tx, loadedAfter, rolledBack, rollbackOverlay)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	rollbackBaseline := rolledBack
	rollbackBaseline.Overlays = append([]EvolutionOverlay(nil), rolledBack.Overlays...)
	rollbackCandidate := ReflectionOverlayCandidateV2{FieldPath: "expression.warmth", Direction: "increase", Strength: 1, Confidence: 0.9, EvidenceRefs: []string{"fact:reflection:1", "fact:reflection:2"}}
	rollbackBaseline.Personality = map[string]any{"traits": map[string]any{"openness": 0.5}, "expression": map[string]any{"warmth": 0.5}}
	atomicDecision, err := CompileEvolutionOverlay(rollbackBaseline, EvolutionOverlayRequest{ExpectedRevision: rollbackBaseline.Revision, Candidate: rollbackCandidate, EvidenceWindows: []string{"window-c", "window-d"}, OccurredAt: now.Add(50 * time.Hour)}, EvolutionOverlayPolicy{})
	if err != nil || atomicDecision.Overlay == nil {
		t.Fatalf("atomic overlay decision=%#v err=%v", atomicDecision, err)
	}
	atomicAfter, _ := ApplyEvolutionOverlay(rollbackBaseline, atomicDecision)
	tx, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persistEvolutionOverlayTx(ctx, tx, rollbackBaseline, atomicAfter, *atomicDecision.Overlay); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var transientRows int
	if err := repository.Pool().QueryRow(context.Background(), `SELECT count(*) FROM public.fluctlight_evolution_overlays WHERE field_path='expression.warmth'`).Scan(&transientRows); err != nil || transientRows != 0 {
		t.Fatalf("rolled-back overlay survived: rows=%d err=%v", transientRows, err)
	}

	var goalRows, attemptRows, proposalRows, dispositionRows, overlayRows, watermark int
	checks := []struct {
		query string
		dest  *int
	}{
		{`SELECT count(*) FROM public.fluctlight_goals WHERE id='goal-s10'`, &goalRows},
		{`SELECT count(*) FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, &attemptRows},
		{`SELECT count(*) FROM public.cognition_reflection_proposals WHERE id=$1`, &proposalRows},
		{`SELECT count(*) FROM public.cognition_reflection_candidate_dispositions WHERE proposal_id=$1`, &dispositionRows},
		{`SELECT count(*) FROM public.fluctlight_evolution_overlays WHERE fluctlight_id=$1`, &overlayRows},
		{`SELECT watermark FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, &watermark},
	}
	args := [][]any{{}, {settlement.Attempt.AttemptID}, {plan.ProposalID}, {plan.ProposalID}, {fluctlightID}, {fluctlightID}}
	for index, check := range checks {
		if err := repository.Pool().QueryRow(ctx, check.query, args[index]...).Scan(check.dest); err != nil {
			t.Fatal(err)
		}
	}
	if goalRows != 1 || attemptRows != 1 || proposalRows != 1 || dispositionRows != 2 || overlayRows != 2 || watermark != 2 {
		t.Fatalf("authority counts goal=%d attempt=%d proposal=%d disposition=%d overlay=%d watermark=%d", goalRows, attemptRows, proposalRows, dispositionRows, overlayRows, watermark)
	}
}
