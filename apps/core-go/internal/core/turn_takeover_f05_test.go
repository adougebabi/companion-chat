package core

import (
	"strings"
	"testing"
)

func seedCorruptTargetPersonaOverlay(t *testing.T, repository *PostgresRepository, fluctlightID string) {
	t.Helper()
	if _, err := repository.Pool().Exec(t.Context(), `INSERT INTO public.fluctlight_evolution_states(fluctlight_id,profile_id,profile_ref,revision,domain_revisions) VALUES($1,'twilight','corrupt-profile-ref',0,'{}')`, fluctlightID); err != nil {
		t.Fatal(err)
	}
}

// A corrupt overlay belonging to an unrelated declared profile must not make
// an ordinary turn unavailable. The Agent never requests that persona, so the
// native reply Tool commits under the active profile without loading twilight.
func TestOrdinaryNativeReplyDoesNotLoadUnusedTargetPersonaOverlay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "f05-unused-owner", "f05-unused-fluctlight", "f05-unused-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	seedCorruptTargetPersonaOverlay(t, repository, fluctlightID)

	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(payload map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{
				nativePersonaToolCall("f05-source-reply", "conversation.reply", map[string]any{"text": "星火正常回复"}),
			}}
		case 2:
			if tools := nativePersonaToolMessages(payload); !strings.Contains(tools, "conversation_message") {
				t.Fatalf("committed reply receipt missing from final model decision: %s", tools)
			}
			return nativePersonaFinal()
		default:
			t.Fatalf("unexpected extra model decision %d", step)
			return fakeProviderResult{Status: 500}
		}
	})
	app := newTestApp(t, repository, router)
	result, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "继续普通对话。", "f05-unused-turn", "f05-unused-turn-1"))
	if err != nil {
		t.Fatal(err)
	}
	if got := stringValue(result.Assistant["text"]); got != "星火正常回复" {
		t.Fatalf("assistant text=%q", got)
	}
	if step != 2 || router.requestCount(workingPersonaMainTurnSchema) != 2 {
		t.Fatalf("ordinary reply used %d decisions / %d requests", step, router.requestCount(workingPersonaMainTurnSchema))
	}
	results := nativePersonaTrace(t, ctx, repository, "f05-unused-turn")
	reply := nativePersonaResultByName(t, results, "conversation.reply")
	if firstString(reply["status"], stringValue(reply["Status"])) != "completed" {
		t.Fatalf("reply receipt was not committed: %#v", reply)
	}
	if acting := firstString(reply["acting_profile_id"], stringValue(reply["ActingProfileID"])); acting != "spark" {
		t.Fatalf("ordinary reply acting profile=%q", acting)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("ordinary reply changed persistent profile to %q", active)
	}
}

// Once the Agent actually asks to use twilight, the target persona becomes an
// execution dependency. Its corrupt persisted overlay must fail the Tool/run;
// Core must not return the declared baseline as a successful working_persona.
func TestNativePersonaTakeoverFailsClosedOnCorruptTargetOverlay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "f05-target-owner", "f05-target-fluctlight", "f05-target-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	seedCorruptTargetPersonaOverlay(t, repository, fluctlightID)

	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
		return fakeProviderResult{ToolCalls: []map[string]any{
			nativePersonaToolCall("f05-corrupt-takeover", personaTakeoverCapabilityName, map[string]any{
				"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt",
				"source_profile_id": "spark", "target_profile_id": "twilight", "reason": "use declared target",
			}),
		}}
	})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "请由暮光接管。", "f05-target-turn", "f05-target-turn-1")); err == nil {
		t.Fatal("a corrupt target overlay was exposed as a successful working persona")
	}
	if requests := router.requestCount(workingPersonaMainTurnSchema); requests != 1 {
		t.Fatalf("target overlay failure must stop before another model decision, got %d requests", requests)
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID); count != 0 {
		t.Fatalf("target overlay failure published %d assistant messages", count)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("failed takeover changed persistent profile to %q", active)
	}
}
