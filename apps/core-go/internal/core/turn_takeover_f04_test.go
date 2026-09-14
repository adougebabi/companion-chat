package core

import (
	"strings"
	"testing"
)

// TestRecoveryFromPendingTakeoverResumesFromFrozenRuleNotCurrentRules is the
// F-04 guard. The rule that won the arbitration is frozen alongside the
// verdict (rule_id, content digest, target, condition, version). On resume the
// normalizer must rebuild the rule from that frozen record and validate the
// frozen target is still a declared profile; it must never re-select from the
// live rule set.
//
// The test corrupts the frozen target to a profile the persona no longer
// declares. A resume that rebuilds from the frozen record fails closed
// (takeover_resume_rule_missing) because the frozen target is gone; a resume
// that re-selects from the live rule set would still find the original rule
// (whose target is still declared) and succeed. Only the frozen-rule resume
// fails, which is exactly the F-04 contract.
func TestRecoveryFromPendingTakeoverResumesFromFrozenRuleNotCurrentRules(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "f04-owner", "f04-fluctlight", "f04-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "f04-turn", "f04-turn-1"

	// First attempt: the Judge approves, the takeover generation dies. The
	// frozen record carries the winning rule identity, digest, target,
	// condition and version.
	first := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, nil))).
		on(takeoverReplySchemaName, takeoverChainFailGeneration()).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, first)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, takeoverChainPendingUserText, inboxKey, turnID); err == nil {
		t.Fatal("a failing takeover generation must fail the turn")
	}
	frozen := takeoverChainFrozenPayload(t, ctx, repository, inboxKey)
	if stage := turnStageOf(frozen); stage != turnStageArbitrationDecided {
		t.Fatalf("the persisted verdict must leave the turn at %q, got %q", turnStageArbitrationDecided, stage)
	}
	persisted := mapValue(frozen[turnTakeoverPayloadKey])
	if persisted["rule_id"] == "" || persisted["target_profile_id"] == "" || persisted["rule_condition"] == "" {
		t.Fatalf("the frozen verdict must carry the winning rule identity, target and condition: %#v", persisted)
	}
	if persisted["rule_version"] != personaSwitchRuleSetVersion {
		t.Fatalf("the frozen verdict must carry the rule-set version, got %#v", persisted["rule_version"])
	}

	// Corrupt the frozen target to a profile the persona does not declare. A
	// frozen-rule resume validates the target against DeclaredProfileIDs and
	// fails closed; a re-selecting resume would ignore this and pick the live
	// rule (whose target is still declared) and succeed.
	takeoverChainRewind(t, ctx, repository, conversationID, turnID, inboxKey, turnStageArbitrationDecided, func(payload map[string]any) {
		if block := mapValue(payload[turnTakeoverPayloadKey]); len(block) > 0 {
			block["target_profile_id"] = "f04-ghost-profile"
		}
	})

	recovery := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainFailGeneration()).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult(takeoverChainTakeoverText, nil))).
		on(takeoverJudgeSchemaName, takeoverChainFailGeneration())
	restarted := newTestApp(t, repository, recovery)
	err := takeoverChainRunTurn(t, restarted, ctx, ownerID, conversationID, fluctlightID, takeoverChainPendingUserText, inboxKey, turnID)
	if err == nil || !strings.Contains(err.Error(), "takeover_resume_rule_missing") {
		t.Fatalf("a resume whose frozen target is no longer declared must fail closed with takeover_resume_rule_missing, got %v", err)
	}
	if count := recovery.requestCount(takeoverJudgeSchemaName); count != 0 {
		t.Fatalf("the failed resume must never consult the Judge, got %d", count)
	}
	if count := recovery.requestCount(takeoverReplySchemaName); count != 0 {
		t.Fatalf("the failed resume must not generate a takeover reply, got %d", count)
	}
}

// TestRecoveryFromPendingTakeoverRebuildsRuleFromFrozenRecord is the positive
// F-04 case: a normal resume (no corruption) rebuilds the rule from the
// frozen record and the frozen rule identity, target and condition survive
// the resume. The rule is not re-selected, so it cannot drift to a different
// rule even if the live rule set changed.
func TestRecoveryFromPendingTakeoverRebuildsRuleFromFrozenRecord(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "f04p-owner", "f04p-fluctlight", "f04p-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	const inboxKey, turnID = "f04p-turn", "f04p-turn-1"

	first := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainSequence(takeoverChainMainResult(takeoverChainCandidateText, nil))).
		on(takeoverReplySchemaName, takeoverChainFailGeneration()).
		on(takeoverJudgeSchemaName, takeoverChainJudge(true))
	app := newTestApp(t, repository, first)
	if err := takeoverChainRunTurn(t, app, ctx, ownerID, conversationID, fluctlightID, takeoverChainPendingUserText, inboxKey, turnID); err == nil {
		t.Fatal("a failing takeover generation must fail the turn")
	}
	frozen := takeoverChainFrozenPayload(t, ctx, repository, inboxKey)
	persisted := mapValue(frozen[turnTakeoverPayloadKey])
	frozenRuleID := stringValue(persisted["rule_id"])
	frozenTarget := stringValue(persisted["target_profile_id"])
	frozenCondition := stringValue(persisted["rule_condition"])
	if frozenRuleID == "" || frozenTarget == "" || frozenCondition == "" {
		t.Fatalf("the frozen verdict must carry the winning rule identity, target and condition: %#v", persisted)
	}

	takeoverChainRewind(t, ctx, repository, conversationID, turnID, inboxKey, turnStageArbitrationDecided, nil)

	recovery := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, takeoverChainFailGeneration()).
		on(takeoverReplySchemaName, takeoverChainSequence(takeoverChainMainResult(takeoverChainTakeoverText, nil))).
		on(takeoverJudgeSchemaName, takeoverChainFailGeneration())
	restarted := newTestApp(t, repository, recovery)
	if err := takeoverChainRunTurn(t, restarted, ctx, ownerID, conversationID, fluctlightID, takeoverChainPendingUserText, inboxKey, turnID); err != nil {
		t.Fatalf("a normal resume must rebuild the rule from the frozen record and continue: %v", err)
	}
	if count := recovery.requestCount(takeoverJudgeSchemaName); count != 0 {
		t.Fatalf("the resumed verdict must never be re-judged, got %d", count)
	}
	if count := recovery.requestCount(takeoverReplySchemaName); count != 1 {
		t.Fatalf("exactly the takeover reply must be generated from the frozen rule, got %d", count)
	}
	takeoverChainAssertDeliveredOnce(t, ctx, repository, conversationID, turnID, takeoverChainTakeoverText)

	refrozen := takeoverChainFrozenPayload(t, ctx, repository, inboxKey)
	if repersisted := mapValue(refrozen[turnTakeoverPayloadKey]); stringValue(repersisted["rule_id"]) != frozenRuleID || stringValue(repersisted["target_profile_id"]) != frozenTarget || stringValue(repersisted["rule_condition"]) != frozenCondition {
		t.Fatalf("the resumed record must keep the frozen rule identity, target and condition: %#v", repersisted)
	}
}

// TestRecoveryFromPendingTakeoverRequiresFrozenRuleProof keeps the recovery
// contract strict: a pending verdict without the Runtime version or content
// digest is not safe to resume. In that case Core must fail closed instead of
// guessing which rule the old payload meant.
func TestRecoveryFromPendingTakeoverRequiresFrozenRuleProof(t *testing.T) {
	base := map[string]any{
		"rule_id":             "takeover:public-doubt",
		"rule_content_digest": "0123456789abcdef0123456789abcdef",
		"rule_version":        personaSwitchRuleSetVersion,
		"rule_condition":      "公开质疑时接管",
		"target_profile_id":   "twilight",
	}
	input := turnTakeoverInput{
		Switch: personaSwitchNormalization{DeclaredProfileIDs: map[string]struct{}{"twilight": {}}},
		Frozen: frozenTurn{Payload: map[string]any{turnTakeoverPayloadKey: base}},
	}
	if rule, ok := resumeTakeoverRuleFromPayload(base, input); !ok || rule.RuleContentDigest == "" {
		t.Fatalf("a complete frozen rule proof must resume: rule=%#v ok=%v", rule, ok)
	}
	for name, mutate := range map[string]func(map[string]any){
		"missing version":     func(payload map[string]any) { delete(payload, "rule_version") },
		"unsupported version": func(payload map[string]any) { payload["rule_version"] = "persona-switch-rules.v0" },
		"missing digest":      func(payload map[string]any) { delete(payload, "rule_content_digest") },
	} {
		payload := cloneMap(base)
		mutate(payload)
		if rule, ok := resumeTakeoverRuleFromPayload(payload, input); ok {
			t.Fatalf("%s must fail closed, got rule=%#v", name, rule)
		}
	}
}
