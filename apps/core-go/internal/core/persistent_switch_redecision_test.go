package core

import (
	"strings"
	"testing"
)

// TestPersistentSwitchReDecidesWithinTheSameTurn pins the user-visible
// contract that was missing from the first implementation: the first cognition
// only decides whether the persistent profile changes, and the final cognition
// runs under the selected Working Persona before any assistant message is
// committed.
func TestPersistentSwitchReDecidesWithinTheSameTurn(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "redecision-owner", "redecision-fluctlight", "redecision-conversation"
	takeoverChainSeedWithTakeoverRules(t, ctx, repository, ownerID, fluctlightID, conversationID, nil)
	router := newFakeProviderRouter().
		on(persistentSwitchAssessmentSchemaName, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"personality_decision": map[string]any{
				"decision": "switch", "from_profile_id": "spark", "target_profile_id": "twilight",
				"trigger_id": "safety", "reason": "收到明确的安全确认", "confidence": 0.95,
				"evidence_refs": []any{},
			}}}
		}).
		on(persistentSwitchReplySchemaName, func(map[string]any) fakeProviderResult {
			return takeoverChainMainResult("暮光在同一轮生成的最终回复", nil)
		})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(
		fluctlightID, "安全确认已收到，请切换到暮光。", "redecision-turn", "redecision-turn",
	)); err != nil {
		t.Fatal(err)
	}
	if got := router.requestCount(persistentSwitchAssessmentSchemaName); got != 1 {
		t.Fatalf("persistent switch assessment calls = %d, want 1", got)
	}
	if got := router.requestCount(persistentSwitchReplySchemaName); got != 1 {
		t.Fatalf("post-switch cognition calls = %d, want 1", got)
	}
	assessment := router.payloads(persistentSwitchAssessmentSchemaName)
	final := router.payloads(persistentSwitchReplySchemaName)
	if len(assessment) != 1 || len(final) != 1 {
		t.Fatalf("captured live-shaped calls assessment=%d final=%d", len(assessment), len(final))
	}
	assessmentSystem := takeoverChainSystemContent(t, assessment[0])
	finalSystem := takeoverChainSystemContent(t, final[0])
	if !strings.Contains(assessmentSystem, takeoverChainSparkMarker) || strings.Contains(assessmentSystem, takeoverChainTwilightMarker) {
		t.Fatalf("switch assessment did not run in the original profile scope: %s", assessmentSystem)
	}
	if !strings.Contains(finalSystem, takeoverChainTwilightMarker) || strings.Contains(finalSystem, takeoverChainSparkMarker) {
		t.Fatalf("post-switch cognition did not run in the target profile scope: %s", finalSystem)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "twilight" {
		t.Fatalf("same-turn persistent switch did not settle active profile: %q", active)
	}
	texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "redecision-turn")
	if len(texts) != 1 || texts[0] != "暮光在同一轮生成的最终回复" {
		t.Fatalf("the assessment candidate must never be sent: %#v", texts)
	}
}
