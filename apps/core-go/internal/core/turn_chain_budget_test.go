package core

import (
	"strings"
	"testing"
)

// The native loop may take as many model decisions as its Tool work requires.
// The generic Runner guard is the only iteration ceiling; there is no A/Judge/B
// stage budget and no query-only continuation special case.
func TestNativePersonaLoopUsesGenericIterationGuardInsteadOfLegacyStageBudget(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "native-budget-owner", "native-budget-fluctlight", "native-budget-conversation"
	takeoverScopeMatrixSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(payload map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("budget-takeover", personaTakeoverCapabilityName, map[string]any{
				"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "twilight",
			})}}
		case 2:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("budget-memory", "memory.recall", map[string]any{"intent": scopeMatrixMemoryContent})}}
		case 3:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("budget-relationship", "relationship.lookup", map[string]any{"target_actor_id": ownerID})}}
		case 4:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("budget-reply", "conversation.reply", map[string]any{"text": "四轮工具后完成"})}}
		case 5:
			return nativePersonaFinal()
		default:
			t.Fatalf("unexpected model request %d", step)
			return fakeProviderResult{Status: 500}
		}
	})
	app := newTestApp(t, repository, router)
	result, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "请先核对再回答。", "native-budget-turn", "native-budget-turn-1"))
	if err != nil {
		t.Fatal(err)
	}
	if step != 5 || router.requestCount(takeoverJudgeSchemaName) != 0 || router.requestCount(takeoverReplySchemaName) != 0 {
		t.Fatalf("native loop requests=%d judge=%d reply-agent=%d", step, router.requestCount(takeoverJudgeSchemaName), router.requestCount(takeoverReplySchemaName))
	}
	if stringValue(result.Assistant["text"]) != "四轮工具后完成" {
		t.Fatalf("assistant=%#v", result.Assistant)
	}
}

// A committed Tool receipt is the recovery unit. If the later final DTO is
// invalid, the run fails, the persistent switch remains committed, and retrying
// the same business run performs no Provider call and no second switch.
func TestInvalidFinalKeepsCommittedPersonaReceiptAndFailedRunDoesNotReplay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "receipt-recovery-owner", "receipt-recovery-fluctlight", "receipt-recovery-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	payload := takeoverChainTurnPayload(fluctlightID, "安全确认已收到。", "receipt-recovery-turn", "receipt-recovery-turn-1")
	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
		step++
		if step == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("receipt-switch", personaSwitchCapabilityName, map[string]any{
				"decision": "switch", "source_profile_id": "spark", "target_profile_id": "twilight", "trigger_id": "switch:safety", "reason": "explicit controlled evidence",
			})}}
		}
		return fakeProviderResult{Structured: map[string]any{
			"action_type": "reply", "response_intent": "invalid final", "influences": []any{}, "unexpected_legacy_fallback": true,
		}}
	})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, payload); err == nil || !strings.Contains(err.Error(), "adk_final_output_invalid") {
		t.Fatalf("invalid final must fail after the committed switch: %v", err)
	}
	if step != 2 || readActiveProfileForGate(t, ctx, repository, fluctlightID) != "twilight" {
		t.Fatalf("switch was not committed before final failure: steps=%d active=%q", step, readActiveProfileForGate(t, ctx, repository, fluctlightID))
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_type='persona_action' AND fluctlight_id=$1 AND kind='persona.switch.committed'`, fluctlightID); count != 1 {
		t.Fatalf("persona switch audit count=%d, want 1", count)
	}

	replayRouter := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		t.Fatal("failed Agent run was replayed through the Provider")
		return fakeProviderResult{Status: 500}
	})
	restarted := newTestApp(t, repository, replayRouter)
	if _, err := restarted.HandleTurn(ctx, ownerID, conversationID, payload); err == nil {
		t.Fatal("failed run retry was reported as success")
	}
	if replayRouter.totalRequests() != 0 {
		t.Fatalf("failed run retry made %d Provider calls", replayRouter.totalRequests())
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "twilight" {
		t.Fatalf("failed final falsely rolled back committed switch to %q", active)
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_type='persona_action' AND fluctlight_id=$1 AND kind='persona.switch.committed'`, fluctlightID); count != 1 {
		t.Fatalf("retry duplicated committed switch audit: %d", count)
	}
}
