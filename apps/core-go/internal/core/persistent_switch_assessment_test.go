package core

import "testing"

func TestNativePersistentSwitchCommitsOnceAndUpdatesSubsequentToolProfile(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "native-switch-owner", "native-switch-fluctlight", "native-switch-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("native-switch", personaSwitchCapabilityName, map[string]any{
				"decision": "switch", "source_profile_id": "spark", "target_profile_id": "twilight",
				"trigger_id": "switch:safety", "reason": "declared safety rule is satisfied",
			})}}
		case 2:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("native-switch-reply", "conversation.reply", map[string]any{"text": "暮光已接手持久主导"})}}
		default:
			return nativePersonaFinal()
		}
	})
	app := newTestApp(t, repository, router)
	result, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "安全确认已收到。", "native-switch-turn", "native-switch-turn-1"))
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(result.Assistant["text"]) != "暮光已接手持久主导" || readActiveProfileForGate(t, ctx, repository, fluctlightID) != "twilight" {
		t.Fatalf("persistent switch result=%#v active=%q", result, readActiveProfileForGate(t, ctx, repository, fluctlightID))
	}
	results := nativePersonaTrace(t, ctx, repository, "native-switch-turn")
	if acting := stringValue(nativePersonaResultByName(t, results, "conversation.reply")["acting_profile_id"]); acting != "twilight" {
		t.Fatalf("post-switch reply acting profile=%q", acting)
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_type='persona_action' AND fluctlight_id=$1 AND kind='persona.switch.committed'`, fluctlightID); count != 1 {
		t.Fatalf("persona.switch committed %d times", count)
	}
}
