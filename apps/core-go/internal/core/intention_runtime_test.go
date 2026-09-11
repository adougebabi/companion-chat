package core

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestIntentionTriggerProductionFlowCreatesDueFactAndSettlesFromOutcome(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "intention-runtime-owner", "intention-runtime-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{"personality_system":{"active_profile_id":"default"}}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id) VALUES($1,'default')`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,1,0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	goal, goalRecord, err := CreateGoalAuthority(GoalAuthority{
		EntityID: "intention-runtime-goal", SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_0123456789abcdef0123456789abcdef",
		FluctlightID: fluctlightID, ProfileID: "default", DesiredOutcome: "完成一次受控动作", SuccessCriteria: []string{"真实 Outcome 成功"},
		Motivation: "验证执行闭环", Scope: "general", Importance: 0.8, Urgency: 0.7, Status: GoalActive, Revision: 1, EvidenceRefs: []string{"owner:goal"},
	}, []string{"owner:goal"}, now)
	if err != nil {
		t.Fatal(err)
	}
	dueAt := now.Add(-time.Minute)
	intention, intentionRecord, err := CreateIntentionAuthority(IntentionAuthority{
		EntityID: "intention-runtime-intention", GoalEntityID: goal.EntityID, SchemaVersion: intentionAuthoritySchemaVersion,
		Ref: "intention:ctx_0123456789abcdef0123456789abcdef", FluctlightID: fluctlightID, ProfileID: "default", GoalRef: goal.Ref,
		ActionIntent: "执行已授权能力", ExpectedOutcome: "得到成功结果", Trigger: TypedIntentionTrigger{Type: IntentionTriggerTime, DueAt: &dueAt},
		Expiration: now.Add(time.Hour), Confidence: 0.9, Status: IntentionQualified, Revision: 1, EvidenceRefs: []string{"owner:intention"},
	}, []string{"owner:intention"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		if _, err := persistGoalAuthorityTx(ctx, tx, nil, goal, goalRecord, "intention-runtime-goal-create"); err != nil {
			return err
		}
		_, err := persistIntentionAuthorityTx(ctx, tx, nil, intention, intentionRecord, "intention-runtime-intention-create")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var triggerIntentCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='intention.trigger' AND payload->>'intention_id'=$1`, intention.EntityID).Scan(&triggerIntentCount); err != nil || triggerIntentCount != 1 {
		t.Fatalf("trigger workflow count=%d err=%v", triggerIntentCount, err)
	}
	app := &App{DB: repository}
	first, err := app.ProcessIntentionTrigger(ctx, intention.EntityID)
	if err != nil || stringValue(first["status"]) != "due" || stringValue(first["inbox_id"]) == "" {
		t.Fatalf("first trigger=%#v err=%v", first, err)
	}
	second, err := app.ProcessIntentionTrigger(ctx, intention.EntityID)
	if err != nil || !boolValueForTest(second["replayed"]) || stringValue(second["inbox_id"]) != stringValue(first["inbox_id"]) {
		t.Fatalf("replayed trigger=%#v err=%v", second, err)
	}
	projection, err := app.BuildContextProjection(ctx, ownerID, fluctlightID, "", stringValue(first["inbox_id"]), "")
	if err != nil {
		t.Fatal(err)
	}
	var goalEntry, intentionEntry ContextReference
	for _, entry := range projection.ReferenceIndex.ByRef {
		switch {
		case entry.Kind == ContextReferenceGoal && entry.EntityID == goal.EntityID:
			goalEntry = entry
		case entry.Kind == ContextReferenceIntention && entry.EntityID == intention.EntityID:
			intentionEntry = entry
		}
	}
	if goalEntry.Ref == "" || intentionEntry.Ref == "" {
		t.Fatalf("projection refs missing: goal=%#v intention=%#v", goalEntry, intentionEntry)
	}
	outcome := ActionOutcome{
		SchemaVersion: actionOutcomeSchemaVersion, ID: "outcome_intention_runtime", FluctlightID: fluctlightID,
		ActionID: "action_intention_runtime", CallID: actionPrimaryCallID, Status: ActionOutcomeCompleted, SuccessBoundary: "action_settled",
		Expected: map[string]any{"action_type": "capability"}, Observed: map[string]any{"status": "completed"},
		GoalRefs: []string{goalEntry.Ref}, IntentionRefs: []string{intentionEntry.Ref}, EvidenceRefs: []string{stringValue(first["due_fact_id"])},
		ContextReferences: map[string]ContextReference{goalEntry.Ref: goalEntry, intentionEntry.Ref: intentionEntry}, Revision: 1, OccurredAt: now.Add(time.Minute),
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error { return persistActionOutcomesTx(ctx, tx, []ActionOutcome{outcome}) }); err != nil {
		t.Fatal(err)
	}
	var status string
	var revision, attempts int
	if err := repository.Pool().QueryRow(ctx, `SELECT status,revision FROM public.fluctlight_intentions WHERE id=$1`, intention.EntityID).Scan(&status, &revision); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE intention_ref=$1`, intentionEntry.Ref).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || revision != 3 || attempts != 1 {
		t.Fatalf("settlement status=%s revision=%d attempts=%d", status, revision, attempts)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error { return persistActionOutcomesTx(ctx, tx, []ActionOutcome{outcome}) }); err != nil {
		t.Fatalf("outcome replay failed: %v", err)
	}

	eventIntention, eventRecord, err := CreateIntentionAuthority(IntentionAuthority{
		EntityID: "intention-runtime-event", GoalEntityID: goal.EntityID, SchemaVersion: intentionAuthoritySchemaVersion,
		Ref: "intention:ctx_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", FluctlightID: fluctlightID, ProfileID: "default", GoalRef: goal.Ref,
		ActionIntent: "处理匹配事件", ExpectedOutcome: "事件得到处理", Trigger: TypedIntentionTrigger{Type: IntentionTriggerEvent, EventType: "event.match"},
		Expiration: now.Add(2 * time.Hour), Confidence: 0.9, Status: IntentionQualified, Revision: 1, EvidenceRefs: []string{"owner:event-intention"},
	}, []string{"owner:event-intention"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistIntentionAuthorityTx(ctx, tx, nil, eventIntention, eventRecord, "intention-runtime-event-create")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		if _, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "event.other", map[string]any{"summary": "不匹配"}, "intention-runtime-event-other"); err != nil {
			return err
		}
		_, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "event.match", map[string]any{"summary": "匹配"}, "intention-runtime-event-match")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	firstEventPoll, err := app.ProcessIntentionTrigger(ctx, eventIntention.EntityID)
	if err != nil || stringValue(firstEventPoll["status"]) != "pending" {
		t.Fatalf("unmatched event poll=%#v err=%v", firstEventPoll, err)
	}
	secondEventPoll, err := app.ProcessIntentionTrigger(ctx, eventIntention.EntityID)
	if err != nil || stringValue(secondEventPoll["status"]) != "due" {
		t.Fatalf("matching event was starved: %#v err=%v", secondEventPoll, err)
	}
	eventProjection, err := app.BuildContextProjection(ctx, ownerID, fluctlightID, "", stringValue(secondEventPoll["inbox_id"]), "")
	if err != nil {
		t.Fatal(err)
	}
	var eventIntentionEntry ContextReference
	for _, entry := range eventProjection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceIntention && entry.EntityID == eventIntention.EntityID {
			eventIntentionEntry = entry
		}
		if entry.Kind == ContextReferenceGoal && entry.EntityID == goal.EntityID {
			goalEntry = entry
		}
	}
	failedOutcome := ActionOutcome{
		SchemaVersion: actionOutcomeSchemaVersion, ID: "outcome_intention_runtime_failed", FluctlightID: fluctlightID,
		ActionID: "action_intention_runtime_failed", CallID: actionPrimaryCallID, Status: ActionOutcomeFailed, SuccessBoundary: "action_settled",
		Expected: map[string]any{"action_type": "capability"}, Observed: map[string]any{"status": "failed"}, ErrorCode: "test_failed",
		GoalRefs: []string{goalEntry.Ref}, IntentionRefs: []string{eventIntentionEntry.Ref}, EvidenceRefs: []string{stringValue(secondEventPoll["due_fact_id"])},
		ContextReferences: map[string]ContextReference{goalEntry.Ref: goalEntry, eventIntentionEntry.Ref: eventIntentionEntry}, Revision: 1, OccurredAt: now.Add(2 * time.Minute),
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error { return persistActionOutcomesTx(ctx, tx, []ActionOutcome{failedOutcome}) }); err != nil {
		t.Fatal(err)
	}
	var rearmed int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='intention.trigger' AND payload->>'intention_id'=$1`, eventIntention.EntityID).Scan(&rearmed); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, eventIntention.EntityID).Scan(&status); err != nil || status != "qualified" || rearmed != 2 {
		t.Fatalf("failed attempt did not re-arm: status=%s workflows=%d err=%v", status, rearmed, err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "event.match", map[string]any{"summary": "再次匹配"}, "intention-runtime-event-match-again")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	asyncDue, err := app.ProcessIntentionTrigger(ctx, eventIntention.EntityID)
	if err != nil || stringValue(asyncDue["status"]) != "due" {
		t.Fatalf("re-armed intention did not become due: %#v err=%v", asyncDue, err)
	}
	asyncProjection, err := app.BuildContextProjection(ctx, ownerID, fluctlightID, "", stringValue(asyncDue["inbox_id"]), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range asyncProjection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceIntention && entry.EntityID == eventIntention.EntityID {
			eventIntentionEntry = entry
		}
		if entry.Kind == ContextReferenceGoal && entry.EntityID == goal.EntityID {
			goalEntry = entry
		}
	}
	asyncActionID, externalRef := "action_intention_runtime_async", "media-intent-runtime-async"
	primaryPending := ActionOutcome{
		SchemaVersion: actionOutcomeSchemaVersion, ID: "outcome_intention_runtime_async_primary", FluctlightID: fluctlightID,
		ActionID: asyncActionID, CallID: actionPrimaryCallID, Status: ActionOutcomePending, SuccessBoundary: "action_in_progress",
		Expected: map[string]any{"action_type": "capability"}, Observed: map[string]any{"status": "in_progress"},
		GoalRefs: []string{goalEntry.Ref}, IntentionRefs: []string{eventIntentionEntry.Ref}, EvidenceRefs: []string{stringValue(asyncDue["due_fact_id"])},
		ContextReferences: map[string]ContextReference{goalEntry.Ref: goalEntry, eventIntentionEntry.Ref: eventIntentionEntry}, Revision: 1, OccurredAt: now.Add(3 * time.Minute),
	}
	asyncPending := ActionOutcome{
		SchemaVersion: actionOutcomeSchemaVersion, ID: "outcome_intention_runtime_async_call", FluctlightID: fluctlightID,
		ActionID: asyncActionID, CallID: "async-call", CapabilityName: "media.image.generate", Status: ActionOutcomePending,
		SuccessBoundary: "durable_media_intent_created", CompletionBoundary: "final_media_asset_ready", ExternalRef: externalRef,
		Expected: map[string]any{"action_type": "capability"}, Observed: map[string]any{"status": "pending"},
		GoalRefs: []string{goalEntry.Ref}, IntentionRefs: []string{eventIntentionEntry.Ref}, EvidenceRefs: []string{stringValue(asyncDue["due_fact_id"])},
		ContextReferences: map[string]ContextReference{goalEntry.Ref: goalEntry, eventIntentionEntry.Ref: eventIntentionEntry}, Revision: 1, OccurredAt: now.Add(3 * time.Minute),
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return persistActionOutcomesTx(ctx, tx, []ActionOutcome{primaryPending, asyncPending})
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, eventIntention.EntityID).Scan(&status); err != nil || status != "due" {
		t.Fatalf("async durable intent completed intention early: status=%s err=%v", status, err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.settleActionOutcomeByExternalRefTx(ctx, tx, externalRef, ActionOutcomeCompleted, map[string]any{"delivery_status": "asset_ready"}, "")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, eventIntention.EntityID).Scan(&status); err != nil || status != "completed" {
		t.Fatalf("async final boundary did not complete intention: status=%s err=%v", status, err)
	}

	expiredAt := now.Add(-time.Minute)
	expiredIntention, expiredRecord, err := CreateIntentionAuthority(IntentionAuthority{
		EntityID: "intention-runtime-expired", GoalEntityID: goal.EntityID, SchemaVersion: intentionAuthoritySchemaVersion,
		Ref: "intention:ctx_cccccccccccccccccccccccccccccccc", FluctlightID: fluctlightID, ProfileID: "default", GoalRef: goal.Ref,
		ActionIntent: "不会执行", ExpectedOutcome: "不会发生", Trigger: TypedIntentionTrigger{Type: IntentionTriggerSemantic},
		Expiration: expiredAt, Confidence: 0.9, Status: IntentionQualified, Revision: 1, EvidenceRefs: []string{"owner:expired-intention"},
	}, []string{"owner:expired-intention"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistIntentionAuthorityTx(ctx, tx, nil, expiredIntention, expiredRecord, "intention-runtime-expired-create")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	expiredResult, err := app.ProcessIntentionTrigger(ctx, expiredIntention.EntityID)
	if err != nil || stringValue(expiredResult["status"]) != "expired" {
		t.Fatalf("expired intention result=%#v err=%v", expiredResult, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, expiredIntention.EntityID).Scan(&status); err != nil || status != "expired" {
		t.Fatalf("expired authority not persisted: status=%s err=%v", status, err)
	}
}
