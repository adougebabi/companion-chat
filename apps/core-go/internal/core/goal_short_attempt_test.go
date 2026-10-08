package core

import (
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestCompletedShortExpressionDoesNotAdvanceMutualGoalWithoutReply(t *testing.T) {
	runShortExpressionScopeScenario(t, false)
}

func TestQueuedDueRebindsWhenOwnedContactAppearsBeforeExecution(t *testing.T) {
	runShortExpressionScopeScenario(t, true)
}

func runShortExpressionScopeScenario(t *testing.T, contactAfterDue bool) {
	f := seedWardrobeToolFixture(t)
	if !contactAfterDue {
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, f.ownerID, f.fluctlightID, f.conversationID); err != nil {
			t.Fatal(err)
		}
	}

	goalID := createDialogueGoalForClosure(t, f, []string{"both participants explicitly confirm the relationship"})
	intentionID := "short-expression-" + f.suffix
	at := f.app.now()
	dueAt := at.Add(-time.Minute)
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		intention, record, err := CreateIntentionAuthority(IntentionAuthority{EntityID: intentionID, GoalEntityID: goalID, SchemaVersion: intentionAuthoritySchemaVersion, Ref: "intention:ctx_" + stableDigest(intentionID), FluctlightID: f.fluctlightID, GoalRef: "goal:ctx_" + stableDigest(goalID), ActionIntent: "express once", ExpectedOutcome: "actually sent expression", Trigger: TypedIntentionTrigger{Type: IntentionTriggerTime, DueAt: &dueAt}, Expiration: at.Add(time.Hour), Confidence: 1, Status: IntentionQualified, Revision: 1}, []string{"owner:plan"}, at)
		if err != nil {
			return err
		}
		_, err = persistIntentionAuthorityTx(f.ctx, tx, nil, intention, record, "short-expression-plan")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	due, err := f.app.ProcessIntentionTrigger(f.ctx, intentionID)
	if err != nil || stringValue(due["status"]) != "due" {
		t.Fatalf("due short expression: %#v %v", due, err)
	}
	originalGoalRef := stringValue(due["goal_ref"])
	if contactAfterDue {
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, f.ownerID, f.fluctlightID, f.conversationID); err != nil {
			t.Fatal(err)
		}
	}
	projection, err := f.app.BuildContextProjection(f.ctx, f.ownerID, f.fluctlightID, f.conversationID, stringValue(due["inbox_id"]), "")
	if err != nil {
		t.Fatal(err)
	}
	for ref, entry := range projection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceGoal && entry.EntityID == goalID {
			due["goal_ref"] = ref
		}
		if entry.Kind == ContextReferenceIntention && entry.EntityID == intentionID {
			due["intention_ref"] = ref
		}
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "short-expression-provider-"+f.suffix)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("native_cognition_response", func(_ map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("short-expression", conversationReplyCapabilityName, map[string]any{"text": "我想认真告诉你自己的心意，你可以自由决定是否回应。", "topic_key": "short-expression", "purpose": "express once without pressure"})}}
		}
		final := nativePersonaFinal()
		delete(final.Structured, "action_type")
		delete(final.Structured, "response_intent")
		for _, key := range []string{"attention", "thought", "desire", "agency"} {
			final.Structured[key] = "expression sent; mutual outcome remains unknown"
		}
		final.Structured["influences"] = []any{map[string]any{"ref": due["goal_ref"], "role": "motivates", "confidence": 1.0, "note": "long mutual goal"}, map[string]any{"ref": due["intention_ref"], "role": "grounds", "confidence": 1.0, "note": "one real expression"}}
		return final
	}).on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		snapshot := readProcessingGoalSnapshot(t, f)
		entry := snapshot.Goals[0]
		selfMessages := 0
		for _, source := range snapshot.Sources {
			if source.Kind == "message" {
				if source.SubjectActorID == f.ownerID {
					t.Fatal("test unexpectedly contains a counterpart reply")
				}
				selfMessages++
			}
		}
		if selfMessages != 1 {
			t.Fatalf("actual short expression missing: %d", selfMessages)
		}
		candidate := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "needs_evidence", WaitCondition: "await voluntary counterpart confirmation", Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "unknown", Kind: "relationship", Subject: "both", Discourse: "uncertain", EvidenceRefs: []string{}, Reason: "one expression is not mutual acceptance"}}}
		return fakeProviderResult{Structured: decodeObject(jsonBytes(GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}}))}
	})}
	if _, err := f.app.ProcessCognitionInbox(f.ctx, stringValue(due["inbox_id"])); err != nil {
		t.Fatal(err)
	}
	var reboundGoalRef, reboundIntentionRef string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload->'due_context'->>'goal_ref',payload->'due_context'->>'intention_ref' FROM public.cognition_inbox WHERE id=$1`, due["inbox_id"]).Scan(&reboundGoalRef, &reboundIntentionRef); err != nil {
		t.Fatal(err)
	}
	if reboundGoalRef != stringValue(due["goal_ref"]) || reboundIntentionRef != stringValue(due["intention_ref"]) || (contactAfterDue && reboundGoalRef == originalGoalRef) {
		t.Fatalf("queued references not rebound to actual scope: %s %s", reboundGoalRef, reboundIntentionRef)
	}
	var actualMessages int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.conversation_messages WHERE author_actor_id=$1 AND kind='assistant'`, f.fluctlightID).Scan(&actualMessages); err != nil {
		t.Fatal(err)
	}
	if actualMessages != 1 {
		var results []byte
		_ = f.repository.Pool().QueryRow(f.ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('capability',capability_name,'status',status,'error_code',error_code,'observed',observed)),'[]') FROM public.cognition_action_outcomes WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&results)
		t.Fatalf("short expression not actually delivered: messages=%d outcomes=%s", actualMessages, results)
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, latestPendingGoalRequest(t, f)); err != nil {
		t.Fatal(err)
	}
	var goalState, intentionState, attemptState string
	var progress float64
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT g.status,g.progress,i.status,a.status FROM public.fluctlight_goals g JOIN public.fluctlight_intentions i ON i.goal_id=g.id JOIN public.fluctlight_intention_attempts a ON a.attempt_id=i.current_attempt_id WHERE i.id=$1`, intentionID).Scan(&goalState, &progress, &intentionState, &attemptState); err != nil {
		t.Fatal(err)
	}
	if goalState != "active" || progress != 0 || intentionState != "completed" || attemptState != "succeeded" {
		t.Fatalf("short action fabricated long progress: %s %f %s %s", goalState, progress, intentionState, attemptState)
	}
}
