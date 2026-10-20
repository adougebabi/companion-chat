package core

import (
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
	if step != 4 || router.requestCount(takeoverJudgeSchemaName) != 0 || router.requestCount(takeoverReplySchemaName) != 0 {
		var cause *string
		_ = repository.Pool().QueryRow(ctx, `SELECT payload->>'safe_cause' FROM public.diagnostic_events WHERE fluctlight_id=$1 AND event_type='agent.run.termination' ORDER BY created_at DESC LIMIT 1`, fluctlightID).Scan(&cause)
		reason := ""
		if cause != nil {
			reason = *cause
		}
		t.Fatalf("native loop requests=%d judge=%d reply-agent=%d cause=%s", step, router.requestCount(takeoverJudgeSchemaName), router.requestCount(takeoverReplySchemaName), reason)
	}
	if stringValue(result.Assistant["text"]) != "四轮工具后完成" {
		t.Fatalf("assistant=%#v", result.Assistant)
	}
	rows, err := repository.Pool().Query(ctx, `SELECT payload FROM public.diagnostic_events WHERE event_type='adk.model.input_budget' ORDER BY created_at,id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	budgets := []any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		budgets = append(budgets, decodeObject(raw))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(budgets) < step {
		t.Fatalf("physical budgets=%d calls=%d", len(budgets), step)
	}
	t.Logf("PHYSICAL_BUDGET_EVIDENCE=%s", jsonString(map[string]any{"conversation_id": conversationID, "physical_main_calls": step, "budgets": budgets, "measurement": "scripted Provider and actual PostgreSQL; estimates, not live usage"}))

}

// A committed reply ends the turn. The persistent switch stays committed, and
// replaying the same business run performs no Provider call and no second switch.
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
		if step == 2 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("receipt-reply", conversationReplyCapabilityName, map[string]any{"text": "确认已收到。"})}}
		}
		return fakeProviderResult{Text: `{"action_type":`}
	})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, payload); err != nil {
		t.Fatal(err)
	}
	var inboxStatus, inboxError string
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE idempotency_key=$1`, payload["idempotency_key"]).Scan(&inboxStatus, &inboxError); err != nil || inboxStatus != "processed" || inboxError != "" {
		t.Fatalf("committed reply did not end the turn: status=%s code=%s err=%v", inboxStatus, inboxError, err)
	}
	if step != 2 || readActiveProfileForGate(t, ctx, repository, fluctlightID) != "twilight" {
		t.Fatalf("switch was not committed before the reply ended the turn: steps=%d active=%q", step, readActiveProfileForGate(t, ctx, repository, fluctlightID))
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_type='persona_action' AND fluctlight_id=$1 AND kind='persona.switch.committed'`, fluctlightID); count != 1 {
		t.Fatalf("persona switch audit count=%d, want 1", count)
	}

	replayRouter := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		t.Fatal("failed Agent run was replayed through the Provider")
		return fakeProviderResult{Status: 500}
	})
	restarted := newTestApp(t, repository, replayRouter)
	if replay, err := restarted.HandleTurn(ctx, ownerID, conversationID, payload); err != nil || replay.Assistant["text"] != "确认已收到。" {
		t.Fatal("committed reply was not recovered after settlement failure", replay, err)
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
