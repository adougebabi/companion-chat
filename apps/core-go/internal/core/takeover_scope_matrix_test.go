package core

import (
	"context"
	"strings"
	"testing"
)

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

func scopeMatrixInteractionFrequency(t *testing.T, ctx context.Context, repository *PostgresRepository, relationshipID string) int {
	t.Helper()
	var frequency int
	if err := repository.Pool().QueryRow(ctx, `SELECT interaction_frequency FROM public.relationships WHERE id=$1`, relationshipID).Scan(&frequency); err != nil {
		t.Fatal(err)
	}
	return frequency
}

func scopeMatrixWireJSON(_ *testing.T, payload map[string]any) string { return jsonString(payload) }

func TestToolProfileContextResolverScopesRealRelationshipAndMemoryReads(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "scope-tool-owner", "scope-tool-fluctlight", "scope-tool-conversation"
	takeoverScopeMatrixSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	app := newTestApp(t, repository, newFakeProviderRouter())
	for _, testCase := range []struct {
		profile, wantRelationship, rejectRelationship string
	}{
		{profile: "spark", wantRelationship: scopeMatrixSparkRelationshipSummary, rejectRelationship: scopeMatrixTwilightRelationshipSummary},
		{profile: "twilight", wantRelationship: scopeMatrixTwilightRelationshipSummary, rejectRelationship: scopeMatrixSparkRelationshipSummary},
	} {
		t.Run(testCase.profile, func(t *testing.T) {
			relationship, err := app.ExecuteTool(ctx, ToolExecutionRequest{
				WorkingProfileID: testCase.profile, CapabilityName: "relationship.lookup", OperationID: "scope-relationship-" + testCase.profile,
				AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
				EvidenceID: "scope-evidence-" + testCase.profile, Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"target_actor_id": ownerID}),
			})
			if err != nil || relationship.Result.Status != "completed" || relationship.Result.ActingProfileID != testCase.profile {
				t.Fatalf("relationship receipt=%#v err=%v", relationship, err)
			}
			encoded := jsonString(relationship.Result.Output)
			if !strings.Contains(encoded, testCase.wantRelationship) || strings.Contains(encoded, testCase.rejectRelationship) {
				t.Fatalf("relationship read escaped %s scope: %s", testCase.profile, encoded)
			}
			memory, err := app.ExecuteTool(ctx, ToolExecutionRequest{
				WorkingProfileID: testCase.profile, CapabilityName: "memory.recall", OperationID: "scope-memory-" + testCase.profile,
				AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
				EvidenceID: "scope-evidence-" + testCase.profile, Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"intent": scopeMatrixMemoryContent}),
			})
			if err != nil || memory.Result.Status != "completed" || memory.Result.ActingProfileID != testCase.profile || !strings.Contains(jsonString(memory.Result.Output), scopeMatrixMemoryContent) {
				t.Fatalf("memory receipt=%#v err=%v", memory, err)
			}
		})
	}
}

func TestRelationshipInteractionSettlementFollowsNativeReplyActingProfile(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "scope-write-owner", "scope-write-fluctlight", "scope-write-conversation"
	takeoverScopeMatrixSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("scope-takeover", personaTakeoverCapabilityName, map[string]any{
				"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "twilight",
			})}}
		case 2:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("scope-reply", "conversation.reply", map[string]any{"text": "暮光发言"})}}
		default:
			return nativePersonaFinal()
		}
	})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "由你回答。", "scope-write-turn", "scope-write-turn-1")); err != nil {
		t.Fatal(err)
	}
	if got := scopeMatrixInteractionFrequency(t, ctx, repository, scopeMatrixTwilightRelationshipID); got != 1 {
		t.Fatalf("twilight interaction_frequency=%d, want 1", got)
	}
	if got := scopeMatrixInteractionFrequency(t, ctx, repository, scopeMatrixSparkRelationshipID); got != 0 {
		t.Fatalf("spark interaction_frequency=%d, want 0", got)
	}
}

func TestNextTurnAfterNativeTakeoverRestoresPersistentProfile(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "scope-next-owner", "scope-next-fluctlight", "scope-next-conversation"
	takeoverScopeMatrixSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(payload map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("next-takeover", personaTakeoverCapabilityName, map[string]any{
				"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "twilight",
			})}}
		case 2:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("next-twilight-reply", "conversation.reply", map[string]any{"text": "暮光第一轮"})}}
		case 3:
			return nativePersonaFinal()
		case 4:
			wire := jsonString(payload)
			if !strings.Contains(wire, takeoverChainSparkMarker) || strings.Contains(wire, takeoverChainTwilightMarker) {
				t.Fatalf("next turn did not restore persistent persona")
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("next-spark-relationship", "relationship.lookup", map[string]any{"target_actor_id": ownerID})}}
		case 5:
			tools := nativePersonaToolMessages(payload)
			if !strings.Contains(tools, scopeMatrixSparkRelationshipSummary) || strings.Contains(tools, scopeMatrixTwilightRelationshipSummary) {
				t.Fatalf("next-turn relationship scope did not restore spark: %s", tools)
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("next-spark-reply", "conversation.reply", map[string]any{"text": "星火第二轮"})}}
		case 6:
			return nativePersonaFinal()
		default:
			t.Fatalf("unexpected model decision %d", step)
			return fakeProviderResult{Status: 500}
		}
	})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "第一轮", "scope-next-turn-1", "scope-next-turn-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "第二轮", "scope-next-turn-2", "scope-next-turn-2")); err != nil {
		t.Fatal(err)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("takeover changed persistent profile to %q", active)
	}
	secondTrace := nativePersonaTrace(t, ctx, repository, "scope-next-turn-2")
	if acting := stringValue(nativePersonaResultByName(t, secondTrace, "conversation.reply")["acting_profile_id"]); acting != "spark" {
		t.Fatalf("next-turn reply acting profile=%q", acting)
	}
	if got := scopeMatrixInteractionFrequency(t, ctx, repository, scopeMatrixSparkRelationshipID); got != 1 {
		t.Fatalf("spark interaction_frequency=%d, want 1", got)
	}
	if got := scopeMatrixInteractionFrequency(t, ctx, repository, scopeMatrixTwilightRelationshipID); got != 1 {
		t.Fatalf("twilight interaction_frequency=%d, want 1", got)
	}
}
