package core

import (
	"context"
	"strings"
	"testing"
)

// This file is the multi-profile read/write matrix that design.md 10 (F04)
// requires and phases 7/8 only partially exercised: prompt-side scoping was
// verified for the persona, but the same proof is needed for every
// profile-scoped read domain (relationships, goals, intentions, memory
// perspectives) and — separately — for the settlement write path, where the
// interaction counter must follow the frozen reply owner rather than the
// persistent dominant profile.
//
// The seeded matrix is one fluctlight with three variants of every domain:
//   * a shared row (profile_id NULL) visible to every profile;
//   * a spark-owned row (the persistent dominant profile, A);
//   * a twilight-owned row (the takeover reply owner, B);
// plus one shared memory carrying a per-profile perspective for both.
//
// Every marker is ASCII so a plain substring check on the wire payload is
// unambiguous regardless of JSON escaping.

const (
	scopeMatrixSharedRelationshipSummary    = "matrix-shared-relationship-summary"
	scopeMatrixSparkRelationshipSummary     = "matrix-spark-relationship-summary"
	scopeMatrixTwilightRelationshipSummary  = "matrix-twilight-relationship-summary"
	scopeMatrixSharedGoalOutcome            = "matrix-shared-goal-outcome"
	scopeMatrixSparkGoalOutcome             = "matrix-spark-goal-outcome"
	scopeMatrixTwilightGoalOutcome          = "matrix-twilight-goal-outcome"
	scopeMatrixSharedIntentionAction        = "matrix-shared-intention-action"
	scopeMatrixSparkIntentionAction         = "matrix-spark-intention-action"
	scopeMatrixTwilightIntentionAction      = "matrix-twilight-intention-action"
	scopeMatrixMemoryContent                = "matrix-shared-memory-content"
	scopeMatrixSparkMemoryInterpretation    = "matrix-spark-memory-interpretation"
	scopeMatrixTwilightMemoryInterpretation = "matrix-twilight-memory-interpretation"

	scopeMatrixSharedRelationshipID   = "matrix-relationship-shared"
	scopeMatrixSparkRelationshipID    = "matrix-relationship-spark"
	scopeMatrixTwilightRelationshipID = "matrix-relationship-twilight"
	scopeMatrixRequestDigest          = "0123456789abcdef0123456789abcdef"
)

// takeoverScopeMatrixSeed seeds the full A-only / B-only / shared read matrix
// on top of the standard two-profile chain seed.
func takeoverScopeMatrixSeed(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID string) {
	t.Helper()
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	if _, err := repository.Pool().Exec(ctx, `
		INSERT INTO public.relationships(id,owner_fluctlight_id,profile_id,target_actor_id,role,metrics,interaction_frequency,trend,summary,emotional_association,provenance,revision) VALUES
		  ($1,$2,NULL,$3,'{"label":"matrix"}','{}',0,'stable',$4,'{}','{}',0),
		  ($5,$2,'spark',$3,'{"label":"matrix"}','{}',0,'stable',$6,'{}','{}',0),
		  ($7,$2,'twilight',$3,'{"label":"matrix"}','{}',0,'stable',$8,'{}','{}',0)`,
		scopeMatrixSharedRelationshipID, fluctlightID, ownerID, scopeMatrixSharedRelationshipSummary,
		scopeMatrixSparkRelationshipID, scopeMatrixSparkRelationshipSummary,
		scopeMatrixTwilightRelationshipID, scopeMatrixTwilightRelationshipSummary); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `
		INSERT INTO public.fluctlight_goals(id,fluctlight_id,profile_id,source,scope,description,desired_outcome,success_criteria,motivation,needs_reflection,importance,urgency,progress,status,evidence_refs,revision,idempotency_key,request_digest) VALUES
		  ('matrix-goal-shared',$2,NULL,'reflection','general','shared goal',$3,'["done"]','motivation',false,'0.8','0.6','0','active','[]',1,'matrix-goal-shared',$1),
		  ('matrix-goal-spark',$2,'spark','reflection','general','spark goal',$4,'["done"]','motivation',false,'0.8','0.6','0','active','[]',1,'matrix-goal-spark',$1),
		  ('matrix-goal-twilight',$2,'twilight','reflection','general','twilight goal',$5,'["done"]','motivation',false,'0.8','0.6','0','active','[]',1,'matrix-goal-twilight',$1)`,
		scopeMatrixRequestDigest, fluctlightID, scopeMatrixSharedGoalOutcome, scopeMatrixSparkGoalOutcome, scopeMatrixTwilightGoalOutcome); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `
		INSERT INTO public.fluctlight_intentions(id,fluctlight_id,profile_id,goal_id,action,action_intent,expected_outcome,capability_constraints,trigger,confidence,expiration,evidence_refs,permission_snapshot,budget_snapshot,status,revision,idempotency_key,request_digest) VALUES
		  ('matrix-intention-shared',$2,NULL,NULL,'shared','`+scopeMatrixSharedIntentionAction+`','outcome','[]','{"type":"time"}','0.9',now()+interval '30 days','[]','{}','{}','qualified',1,'matrix-intention-shared',$1),
		  ('matrix-intention-spark',$2,'spark',NULL,'spark','`+scopeMatrixSparkIntentionAction+`','outcome','[]','{"type":"time"}','0.9',now()+interval '30 days','[]','{}','{}','qualified',1,'matrix-intention-spark',$1),
		  ('matrix-intention-twilight',$2,'twilight',NULL,'twilight','`+scopeMatrixTwilightIntentionAction+`','outcome','[]','{"type":"time"}','0.9',now()+interval '30 days','[]','{}','{}','qualified',1,'matrix-intention-twilight',$1)`,
		scopeMatrixRequestDigest, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `
		INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest,created_at)
		VALUES('matrix-memory',$1,'semantic',$2,jsonb_build_array($3::text),NULL,'[]','["fact"]',
		  $4::jsonb,0.9,0.9,0.3,'participants','active',0,'cccccccccccccccccccccccccccccccc','dddddddddddddddddddddddddddddddd',now())`,
		fluctlightID, scopeMatrixMemoryContent, ownerID,
		`[{"profile_id":"spark","interpretation":"`+scopeMatrixSparkMemoryInterpretation+`"},{"profile_id":"twilight","interpretation":"`+scopeMatrixTwilightMemoryInterpretation+`"}]`); err != nil {
		t.Fatal(err)
	}
}

// scopeMatrixWireJSON renders one captured request payload as the exact JSON
// the Provider received, so an assertion cannot miss a fragment that landed in
// the user message instead of the system message.
func scopeMatrixWireJSON(t *testing.T, payload map[string]any) string {
	t.Helper()
	return jsonString(payload)
}

// TestMultiProfileScopeMatrixReadFollowsTheSpeaker asserts the whole read side
// of the F04 matrix on the real wire: whichever profile is speaking sees its
// own rows plus shared rows, never the other profile's rows, and a shared
// relationship is shadowed by the speaker's own row for the same target.
func TestMultiProfileScopeMatrixReadFollowsTheSpeaker(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "scope-matrix-owner", "scope-matrix-fluctlight", "scope-matrix-conversation"
	takeoverScopeMatrixSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult("星火的候选回复", nil))).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult("暮光的接管回复", nil))).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "我们上次聊到哪儿了？", "scope-matrix-read-turn", "scope-matrix-read-turn-1")); err != nil {
		t.Fatal(err)
	}

	candidates := router.payloads(workingPersonaMainTurnSchema)
	replies := router.payloads(takeoverReplySchemaName)
	if len(candidates) != 1 || len(replies) != 1 {
		t.Fatalf("expected one candidate and one takeover generation, got %d and %d", len(candidates), len(replies))
	}
	aWire := scopeMatrixWireJSON(t, candidates[0])
	bWire := scopeMatrixWireJSON(t, replies[0])

	// A (the persistent dominant profile) reads its own rows plus shared rows.
	for _, marker := range []string{
		scopeMatrixSparkRelationshipSummary, scopeMatrixSparkGoalOutcome, scopeMatrixSparkIntentionAction,
		scopeMatrixSharedGoalOutcome, scopeMatrixSharedIntentionAction,
		scopeMatrixMemoryContent, scopeMatrixSparkMemoryInterpretation,
	} {
		if !strings.Contains(aWire, marker) {
			t.Fatalf("the A generation is missing its own or shared scope data: %s", marker)
		}
	}
	// A never reads B's rows, and A's own relationship shadows the shared one.
	for _, marker := range []string{
		scopeMatrixTwilightRelationshipSummary, scopeMatrixTwilightGoalOutcome, scopeMatrixTwilightIntentionAction,
		scopeMatrixTwilightMemoryInterpretation, scopeMatrixSharedRelationshipSummary,
	} {
		if strings.Contains(aWire, marker) {
			t.Fatalf("the A generation leaked out-of-scope data: %s", marker)
		}
	}

	// B (the takeover reply owner) reads its own rows plus the same shared rows.
	for _, marker := range []string{
		scopeMatrixTwilightRelationshipSummary, scopeMatrixTwilightGoalOutcome, scopeMatrixTwilightIntentionAction,
		scopeMatrixSharedGoalOutcome, scopeMatrixSharedIntentionAction,
		scopeMatrixMemoryContent, scopeMatrixTwilightMemoryInterpretation,
	} {
		if !strings.Contains(bWire, marker) {
			t.Fatalf("the B generation is missing its own or shared scope data: %s", marker)
		}
	}
	// B never reads A's rows, and B's own relationship shadows the shared one.
	for _, marker := range []string{
		scopeMatrixSparkRelationshipSummary, scopeMatrixSparkGoalOutcome, scopeMatrixSparkIntentionAction,
		scopeMatrixSparkMemoryInterpretation, scopeMatrixSharedRelationshipSummary,
	} {
		if strings.Contains(bWire, marker) {
			t.Fatalf("the B generation leaked out-of-scope data: %s", marker)
		}
	}
}

// scopeMatrixInteractionFrequency reads one relationship's interaction counter.
func scopeMatrixInteractionFrequency(t *testing.T, ctx context.Context, repository *PostgresRepository, relationshipID string) int {
	t.Helper()
	var frequency int
	if err := repository.Pool().QueryRow(ctx, `SELECT interaction_frequency FROM public.relationships WHERE id=$1`, relationshipID).Scan(&frequency); err != nil {
		t.Fatal(err)
	}
	return frequency
}

// TestRelationshipInteractionSettlementFollowsTheReplyOwner asserts the write
// side of the F04 matrix: the durable interaction fact of a turn lands on the
// row of the profile that actually replied — A's turn updates A's row, a
// takeover B reply updates B's row, and a profile without its own row falls
// back to the shared row without inventing one.
func TestRelationshipInteractionSettlementFollowsTheReplyOwner(t *testing.T) {
	for _, scenario := range []struct {
		name           string
		judgeApproves  bool
		wantSpark      int
		wantTwilight   int
		wantShared     int
		seedSharedOnly bool
		wantSharedOnly int
	}{
		{name: "judge kept A", judgeApproves: false, wantSpark: 1, wantTwilight: 0, wantShared: 0},
		{name: "judge approved B", judgeApproves: true, wantSpark: 0, wantTwilight: 1, wantShared: 0},
		// Without a profile-owned row the shared row is the fallback for both
		// speakers; an interaction alone must never create a new row.
		{name: "shared fallback for A", judgeApproves: false, seedSharedOnly: true, wantSharedOnly: 1},
		{name: "shared fallback for B", judgeApproves: true, seedSharedOnly: true, wantSharedOnly: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			slug := strings.ReplaceAll(scenario.name, " ", "-")
			ownerID, fluctlightID, conversationID := "scope-write-owner-"+slug, "scope-write-fluctlight-"+slug, "scope-write-conversation-"+slug
			takeoverScopeMatrixSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
			if scenario.seedSharedOnly {
				if _, err := repository.Pool().Exec(ctx, `DELETE FROM public.relationships WHERE owner_fluctlight_id=$1 AND profile_id IS NOT NULL`, fluctlightID); err != nil {
					t.Fatal(err)
				}
			}

			router := newFakeProviderRouter().
				on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult("星火的候选回复", nil))).
				on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult("暮光的接管回复", nil))).
				on(takeoverJudgeSchemaName, takeoverChainJudge(scenario.judgeApproves))
			app := newTestApp(t, repository, router)
			if _, err := app.HandleTurn(ctx, ownerID, conversationID,
				takeoverChainTurnPayload(fluctlightID, "你在听吗？", "scope-write-turn-"+slug, "scope-write-turn-"+slug+"-1")); err != nil {
				t.Fatal(err)
			}

			if scenario.seedSharedOnly {
				if got := scopeMatrixInteractionFrequency(t, ctx, repository, scopeMatrixSharedRelationshipID); got != scenario.wantSharedOnly {
					t.Fatalf("shared-row fallback interaction_frequency=%d, expected %d", got, scenario.wantSharedOnly)
				}
				if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.relationships WHERE owner_fluctlight_id=$1`, fluctlightID); count != 1 {
					t.Fatalf("an interaction must not create relationship rows, got %d", count)
				}
				return
			}
			for _, check := range []struct {
				id   string
				want int
			}{
				{scopeMatrixSparkRelationshipID, scenario.wantSpark},
				{scopeMatrixTwilightRelationshipID, scenario.wantTwilight},
				{scopeMatrixSharedRelationshipID, scenario.wantShared},
			} {
				if got := scopeMatrixInteractionFrequency(t, ctx, repository, check.id); got != check.want {
					t.Fatalf("relationship %s interaction_frequency=%d, expected %d", check.id, got, check.want)
				}
			}
		})
	}
}

// TestNextTurnAfterTakeoverRestoresThePersistentPerspective asserts that a
// takeover is per-turn in the read direction too: after B's turn settles, the
// next Main generation runs as the persistent dominant profile again, with
// A's scoped rows and A's settlement writes.
func TestNextTurnAfterTakeoverRestoresThePersistentPerspective(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "scope-next-owner", "scope-next-fluctlight", "scope-next-conversation"
	takeoverScopeMatrixSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	// The Judge approves the first turn's handover and declines the second, so
	// turn 1 settles as B and turn 2 settles as A.
	judgeCall := 0
	judgeScript := func(map[string]any) fakeProviderResult {
		judgeCall++
		return takeoverChainJudge(judgeCall == 1)(nil)
	}
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(
			takeoverChainMainResult("星火的第一回合", nil),
			takeoverChainMainResult("星火的第二回合", nil),
		)).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult("暮光的接管回复", nil))).
		on(takeoverJudgeSchemaName, judgeScript)
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "你根本没在听我说话。", "scope-next-turn-1", "scope-next-turn-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "现在好一些了吗？", "scope-next-turn-2", "scope-next-turn-2")); err != nil {
		t.Fatal(err)
	}

	candidates := router.payloads(workingPersonaMainTurnSchema)
	if len(candidates) != 2 {
		t.Fatalf("expected two Main generations, got %d", len(candidates))
	}
	secondWire := scopeMatrixWireJSON(t, candidates[1])
	if !strings.Contains(secondWire, takeoverChainSparkMarker) || strings.Contains(secondWire, takeoverChainTwilightMarker) {
		t.Fatalf("the turn after a takeover did not run as the persistent dominant profile")
	}
	if !strings.Contains(secondWire, scopeMatrixSparkRelationshipSummary) || !strings.Contains(secondWire, scopeMatrixSparkMemoryInterpretation) {
		t.Fatalf("the turn after a takeover lost the persistent profile's scoped rows")
	}
	if strings.Contains(secondWire, scopeMatrixTwilightRelationshipSummary) || strings.Contains(secondWire, scopeMatrixTwilightMemoryInterpretation) {
		t.Fatalf("the turn after a takeover leaked the takeover owner's scoped rows")
	}

	// takeover_once in the durable direction: the persistent dominant profile
	// row is unchanged, and the second (A) settlement wrote into A's scope.
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("a settled takeover changed the persistent dominant profile to %q", active)
	}
	if got := scopeMatrixInteractionFrequency(t, ctx, repository, scopeMatrixSparkRelationshipID); got != 1 {
		t.Fatalf("the post-takeover A turn must write A's relationship row, got interaction_frequency=%d", got)
	}
	if got := scopeMatrixInteractionFrequency(t, ctx, repository, scopeMatrixTwilightRelationshipID); got != 1 {
		t.Fatalf("the takeover turn must have written B's relationship row exactly once, got interaction_frequency=%d", got)
	}

	texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "scope-next-turn-2")
	if len(texts) != 1 || texts[0] != "星火的第二回合" {
		t.Fatalf("the second turn did not deliver A's reply, got %#v", texts)
	}
}
