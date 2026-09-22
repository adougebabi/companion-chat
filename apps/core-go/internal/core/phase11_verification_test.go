package core

import (
	"strings"
	"testing"
)

// Phase 11 (implement.md 11.1/11.2) — the deterministic runtime scenarios and
// the persona/prompt contract checks that earlier phases left implicit.
//
// Coverage map against design.md 17.1/17.2:
//   * pure QUERY skips arbitration            → TestPureQueryTurnNeverInvokesTheJudge
//   * mixed QUERY+ACTION is arbitrated        → TestMixedQueryActionBatchMayBeArbitratedButNotContinued
//   * B's persistent-switch proposal is dead  → TestTakeoverReplyCannotWritePersistentActive
//   * rejected candidate never becomes fact   → TestRejectedCandidateLeavesNoFactTrail
//   * A/B prompt isolation, shared scope      → TestMultiProfileScopeMatrixReadFollowsTheSpeaker
//   * declared rule id end to end             → TestDeclaredRulesReachTheProductionTurn
//   * traits verbatim, no invented numbers    → TestWorkingPersonaKeepsDescriptiveTraitsVerbatim et al.
//   * Main system carries no takeover_rules   → TestProviderSystemPersonaHasNoTakeoverRules
//
// This file adds the three gaps: the authorized persistent switch must load
// the new persona on the NEXT turn (end to end), the profile roster on the
// system persona must be identifier-only as a precise field assertion, and the
// request budget must account for tools and the response schema.

// TestAuthorizedPersistentSwitchLoadsTheNewPersonaNextTurn is scenario 3 of
// implement.md 11.1: an authorized main-turn persistent switch (cognitive
// assessment scenario, declared switching rule "safety") updates the durable
// active profile during settlement, and the next Main generation is composed
// inside the NEW persona's scope — with the persistent grant consumed, not
// inherited by anything else.
func TestAuthorizedPersistentSwitchLoadsTheNewPersonaNextTurn(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "switch-next-owner", "switch-next-fluctlight", "switch-next-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	step := 0
	var nextTurnWire string
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(payload map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("next-switch", personaSwitchCapabilityName, map[string]any{
				"decision": "switch", "source_profile_id": "spark", "target_profile_id": "twilight",
				"trigger_id": "switch:safety", "reason": "收到明确的安全确认",
			})}}
		case 2:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("switch-first-reply", "conversation.reply", map[string]any{"text": "暮光确认切换"})}}
		case 3:
			return nativePersonaFinal()
		case 4:
			nextTurnWire = jsonString(payload)
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("switch-next-reply", "conversation.reply", map[string]any{"text": "暮光的后续回复"})}}
		case 5:
			return nativePersonaFinal()
		default:
			return fakeProviderResult{Status: 500}
		}
	})
	app := newTestApp(t, repository, router)

	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "安全确认已收到", "switch-next-turn-1", "switch-next-turn-1")); err != nil {
		t.Fatal(err)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "twilight" {
		t.Fatalf("the authorized switch did not reach the durable runtime, active=%q", active)
	}

	if _, err := app.HandleTurn(ctx, ownerID, conversationID,
		takeoverChainTurnPayload(fluctlightID, "继续聊聊", "switch-next-turn-2", "switch-next-turn-2")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(nextTurnWire, takeoverChainTwilightMarker) {
		t.Fatal("the next turn after an authorized switch did not load the new persona's Working Persona")
	}
	if strings.Contains(nextTurnWire, takeoverChainSparkMarker) {
		t.Fatal("the next turn still carries the previous persona's personality content")
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "twilight" {
		t.Fatalf("a plain follow-up turn must not move the durable profile, active=%q", active)
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_type='persona_action' AND fluctlight_id=$1 AND kind='persona.switch.committed'`, fluctlightID); count != 1 {
		t.Fatalf("persistent switch committed %d times", count)
	}
}

// TestProviderRosterCarriesOnlyIdentifierFields is the 17.2 "profiles 只剩
// {id,name}" contract as a precise field assertion on the single System
// Persona exit, using the exact A/B card the chain tests seed. Every roster
// entry may keep only its identifiers; every content field (including the
// active profile's) must come from the Working Persona body instead, and the
// switching section keeps only its semantic protocol values.
func TestProviderRosterCarriesOnlyIdentifierFields(t *testing.T) {
	personalitySystem := map[string]any{
		"mode": "multiple", "active_profile_id": "spark",
		"profiles": []any{
			map[string]any{
				"id": "spark", "name": "星火",
				"personality":       map[string]any{"openness": 0.8, "signature_phrase": takeoverChainSparkMarker},
				"behavioral_policy": map[string]any{"directness": 0.9},
			},
			map[string]any{
				"id": "twilight", "name": "暮光",
				"personality":       map[string]any{"openness": 0.3, "signature_phrase": takeoverChainTwilightMarker},
				"behavioral_policy": map[string]any{"gentleness": 0.9},
			},
		},
		"switching": map[string]any{"rules": []any{
			map[string]any{"id": "safety", "condition": "收到明确的安全确认后由暮光主导", "target_profile_id": "twilight"},
		}},
		"takeover_rules": takeoverChainDefaultRules(),
	}
	filtered := filterCorePersona(map[string]any{"personality_system": personalitySystem})
	system := mapValue(filtered["personality_system"])

	roster, ok := system["profiles"].([]any)
	if !ok || len(roster) != 2 {
		t.Fatalf("the roster must survive as a two-entry list: %#v", system["profiles"])
	}
	for _, item := range roster {
		profile := mapValue(item)
		if len(profile) == 0 {
			t.Fatalf("a roster entry vanished: %#v", item)
		}
		for key := range profile {
			switch key {
			case "id", "name", "profile_id":
			default:
				t.Fatalf("roster entry %q leaked a content field %q: %#v", profile["id"], key, profile)
			}
		}
		if strings.TrimSpace(stringValue(profile["id"])) == "" {
			t.Fatalf("a roster entry lost its identifier: %#v", profile)
		}
	}

	// The switching section stays semantic (id/target/trigger), but the
	// declared condition text is also a decision input the Main turn needs to
	// evaluate the rule, so it must not be dropped wholesale.
	rules := mapValue(system["switching"])["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("the switching section lost its rules: %#v", system["switching"])
	}
	rule := mapValue(rules[0])
	if stringValue(rule["id"]) != "safety" || stringValue(rule["target_profile_id"]) != "twilight" {
		t.Fatalf("the switching rule lost its semantic values: %#v", rule)
	}

	// Content reaches the Provider only through the Working Persona body, so
	// the filtered persona as a whole must still carry the active profile's
	// personality while the roster itself stays clean (checked above).
	if !strings.Contains(jsonString(filtered), takeoverChainSparkMarker) {
		t.Fatal("the active profile's content never reached the System Persona body")
	}
}

// TestWireBudgetAccountsForToolsAndResponseFormat is the 17.2 budget clause:
// EstimatedInputTokens is the full wire estimate — messages plus tools plus
// the response schema — and the trace attributes each part, so a large tool
// schema or a large response schema can never silently exceed the cap that a
// messages-only estimate would allow.
func TestWireBudgetAccountsForToolsAndResponseFormat(t *testing.T) {
	base := PromptAssemblyInput{
		Role: "main", CurrentInput: "预算核对输入",
		Policy: DefaultPromptBudgetPolicy(0),
	}
	bare, err := AssemblePromptContext(base)
	if err != nil {
		t.Fatal(err)
	}
	// An empty schema still costs the JSON wrapper constant, so the exact
	// baseline is the empty-slice estimate, not zero.
	emptyCost := EstimatePromptTokens([]map[string]any{})
	if got := bare.Trace.SectionTokens["tools"]; got != emptyCost {
		t.Fatalf("the empty tools section is %d, want the %d constant", got, emptyCost)
	}
	if got := bare.Trace.SectionTokens["response_schema"]; got != emptyCost {
		t.Fatalf("the empty response_schema section is %d, want the %d constant", got, emptyCost)
	}

	tools := []map[string]any{{
		"type": "function", "function": map[string]any{
			"name": "budget_probe_capability", "description": strings.Repeat("预算探针描述。", 200),
			"parameters": map[string]any{"type": "object", "properties": map[string]any{
				"argument": map[string]any{"type": "string", "description": strings.Repeat("参数描述。", 100)},
			}},
		},
	}}
	responseFormat := map[string]any{"type": "json_schema", "json_schema": map[string]any{
		"name": "budget_probe", "schema": map[string]any{
			"type": "object", "properties": map[string]any{
				"visible_text": map[string]any{"type": "string", "description": strings.Repeat("输出描述。", 150)},
			},
		},
	}}
	full := base
	full.Tools, full.ResponseFormat = tools, responseFormat
	loaded, err := AssemblePromptContext(full)
	if err != nil {
		t.Fatal(err)
	}

	wantTools := EstimatePromptTokens(tools)
	wantSchema := EstimatePromptTokens(responseFormat)
	if got := loaded.Trace.SectionTokens["tools"]; got != wantTools {
		t.Fatalf("the trace tools section is %d, want %d", got, wantTools)
	}
	if got := loaded.Trace.SectionTokens["response_schema"]; got != wantSchema {
		t.Fatalf("the trace response_schema section is %d, want %d", got, wantSchema)
	}
	delta := loaded.Trace.EstimatedInputTokens - bare.Trace.EstimatedInputTokens
	if delta != wantTools+wantSchema-2*emptyCost {
		t.Fatalf("EstimatedInputTokens grew by %d, want tools %d + schema %d minus the two empty constants (%d each)", delta, wantTools, wantSchema, emptyCost)
	}
}
