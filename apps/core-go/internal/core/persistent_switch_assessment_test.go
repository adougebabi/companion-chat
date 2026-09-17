package core

import (
	"strings"
	"testing"
)

// TestPersistentSwitchAssessmentRunsAfterTheCandidateKeepsTheTurn Simple:
// the normal Main response is produced first, then a tool-free switch
// assessment decides the persistent profile for the next turn. The assessment
// itself never becomes a user-visible candidate.
func TestPersistentSwitchAssessmentRunsAfterTheCandidateKeepsTheTurn(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "post-assessment-owner", "post-assessment-fluctlight", "post-assessment-conversation"
	takeoverChainSeedWithTakeoverRules(t, ctx, repository, ownerID, fluctlightID, conversationID, nil)
	router := newFakeProviderRouter().
		on(workingPersonaMainTurnSchema, func(map[string]any) fakeProviderResult {
			return takeoverChainMainResult("星火先完成本轮回复", nil)
		}).
		on(persistentSwitchAssessmentSchemaName, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"personality_decision": map[string]any{
				"decision": "switch", "from_profile_id": "spark", "target_profile_id": "twilight",
				"trigger_id": "safety", "reason": "回复后确认安全切换", "confidence": 0.95,
				"evidence_refs": []any{},
			}}}
		})
	app := newTestApp(t, repository, router)
	if _, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(
		fluctlightID, "安全确认已收到。", "post-assessment-turn", "post-assessment-turn",
	)); err != nil {
		t.Fatal(err)
	}
	if got := router.requestCount(workingPersonaMainTurnSchema); got != 1 {
		t.Fatalf("Main response calls = %d, want 1", got)
	}
	if got := router.requestCount(persistentSwitchAssessmentSchemaName); got != 1 {
		t.Fatalf("post-cognition switch assessments = %d, want 1", got)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "twilight" {
		t.Fatalf("post-cognition assessment did not settle active profile: %q", active)
	}
	texts := takeoverChainAssistantTexts(t, ctx, repository, conversationID, "post-assessment-turn")
	if len(texts) != 1 || texts[0] != "星火先完成本轮回复" {
		t.Fatalf("the switch assessment must not replace the current reply: %#v", texts)
	}
	assessmentPayloads := router.payloads(persistentSwitchAssessmentSchemaName)
	if len(assessmentPayloads) != 1 {
		t.Fatalf("assessment wire payload count = %d", len(assessmentPayloads))
	}
	assessmentWire := takeoverChainSystemContent(t, assessmentPayloads[0])
	if !strings.Contains(assessmentWire, "persistent_switch") || !strings.Contains(assessmentWire, "switch:safety") {
		t.Fatalf("post-cognition assessment did not receive the declared switch rule: %s", assessmentWire)
	}
}
