package core

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestGoalProjectionAndToolsPreservePrivateScopeAcrossDurablePersonaSwitch(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	owner, fluctlight, conversation := "goal-scope-owner", "goal-scope-fluctlight", "goal-scope-conversation"
	takeoverScopeMatrixSeed(t, ctx, repository, owner, fluctlight, conversation)
	app := newTestApp(t, repository, newFakeProviderRouter())
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlight_goals SET evidence_refs=$2 WHERE fluctlight_id=$1`, fluctlight, jsonBytes([]string{"owner:" + owner})); err != nil {
		t.Fatal(err)
	}
	project := func() ContextProjection {
		t.Helper()
		p, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, FluctlightID: fluctlight, ConversationID: conversation, SourceFactID: "goal-scope-check", CurrentUserText: "检查当前目标", MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	assertGoals := func(p ContextProjection, visible, hidden string) {
		t.Helper()
		wire := jsonString(compactCognitionContextForSurface(p, ProviderContextSurfaceConversationMain)["goals"])
		if !strings.Contains(wire, scopeMatrixSharedGoalOutcome) || !strings.Contains(wire, visible) || strings.Contains(wire, hidden) {
			t.Fatalf("private Goal visibility leaked: %s", wire)
		}
		for _, entry := range p.ReferenceIndex.ByRef {
			if entry.Kind == ContextReferenceGoal && ((hidden == scopeMatrixTwilightGoalOutcome && entry.EntityID == "matrix-goal-twilight") || (hidden == scopeMatrixSparkGoalOutcome && entry.EntityID == "matrix-goal-spark")) {
				t.Fatalf("hidden Goal received served ref: %#v", entry)
			}
		}
	}
	spark := project()
	assertGoals(spark, scopeMatrixSparkGoalOutcome, scopeMatrixTwilightGoalOutcome)
	for _, tc := range []struct{ capability, operation, code string }{{"goal.inspect", "detail", "goal_not_found"}, {"goal.decide", "pause", "goal_scope_invalid"}} {
		args := map[string]any{"operation": tc.operation, "goal_id": "matrix-goal-twilight"}
		if tc.operation == "pause" {
			args["expected_revision"], args["reason"] = 1, "cross-profile attempt"
		}
		receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{WorkingProfileID: "spark", CapabilityName: tc.capability, OperationID: "scope-" + tc.operation, AuthorizationActorID: owner, FluctlightID: fluctlight, ConversationID: conversation, EvidenceID: "scope-check", Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(args)})
		if err == nil || receipt.Result.ErrorCode != tc.code {
			t.Fatalf("cross-profile %s: %#v %v", tc.operation, receipt, err)
		}
	}
	var source GoalSource
	var sparkGoal GoalAuthority
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlight, "life.event.created", map[string]any{"summary": "profile-scoped actual expression"}, "goal-scope-message-fact")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,source_fact_id) VALUES('goal-scope-message',$1,50,$2,'assistant','actual twilight expression','[]','goal-scope-message',$3)`, conversation, fluctlight, factID); err != nil {
			return err
		}
		if err := app.recordGoalMessageTx(ctx, tx, fluctlight, "goal-scope-message", "twilight"); err != nil {
			return err
		}
		var eventID int64
		if err := tx.QueryRow(ctx, `SELECT id FROM public.goal_source_events WHERE source_id='goal-scope-message' AND fluctlight_id=$1`, fluctlight).Scan(&eventID); err != nil {
			return err
		}
		source, err = readGoalSourceWith(ctx, tx, eventID, false)
		if err != nil {
			return err
		}
		sparkGoal, err = loadGoalAuthorityTx(ctx, tx, fluctlight, "goal:ctx_"+stableDigest("matrix-goal-spark"), ContextReference{EntityID: "matrix-goal-spark", Revision: 1})
		if err != nil {
			return err
		}
		ref := ""
		for key, entry := range spark.ReferenceIndex.ByRef {
			if entry.Kind == ContextReferenceGoal && entry.EntityID == sparkGoal.EntityID {
				ref = key
			}
		}
		if ref == "" {
			t.Fatal("visible private Goal ref missing")
		}
		return app.enqueueTurnGoalCandidatesTx(ctx, tx, fluctlight, factID, spark, map[string]any{"goal_event_candidates": []any{map[string]any{"goal_ref": ref, "reason": "attempted cross-profile association"}}})
	}); err != nil {
		t.Fatal(err)
	}
	for _, verdict := range []string{"satisfied", "unknown", "not_satisfied"} {
		candidate := GoalEvaluationCandidate{GoalID: sparkGoal.EntityID, ExpectedRevision: sparkGoal.Revision, CriteriaVersion: effectiveGoalCriteriaVersion(sparkGoal), Impact: "needs_evidence", Judgments: []GoalCriterionJudgment{{CriterionID: goalCriterionIDs(sparkGoal)[0], Verdict: verdict, Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{source.Ref}, Reason: "cross-profile proof"}}}
		if _, _, _, err := ApplyGoalEvaluation(sparkGoal, candidate, map[string]GoalSource{source.Ref: source}, app.now()); err == nil || !strings.Contains(err.Error(), "goal_judgment_profile_invalid") {
			t.Fatalf("cross-profile %s accepted: %v", verdict, err)
		}
	}
	var linked int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.goal_evidence_links WHERE goal_id='matrix-goal-spark' AND source_event_id=$1`, source.EventID).Scan(&linked); err != nil || linked != 0 {
		t.Fatalf("private source linked across profiles: %d %v", linked, err)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: personaSwitchCapabilityName, OperationID: "goal-scope-switch", AuthorizationActorID: owner, FluctlightID: fluctlight, EvidenceID: "owner-command-scope-switch", Arguments: jsonBytes(map[string]any{"decision": "switch", "source_profile_id": "spark", "target_profile_id": "twilight", "trigger_id": "switch:safety", "reason": "scope acceptance"})})
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("durable switch: %#v %v", receipt, err)
	}
	assertGoals(project(), scopeMatrixTwilightGoalOutcome, scopeMatrixSparkGoalOutcome)
	var unchanged int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active' AND revision=1 AND ((id='matrix-goal-shared' AND profile_id IS NULL) OR (id='matrix-goal-spark' AND profile_id='spark') OR (id='matrix-goal-twilight' AND profile_id='twilight'))`, fluctlight).Scan(&unchanged); err != nil || unchanged != 3 {
		t.Fatalf("persona switch rewrote Goal ownership: %d %v", unchanged, err)
	}
}
