package core

import (
	"strings"
	"testing"
)

func TestPersonaTakeoverToolRejectsUndeclaredRuleTargetAndStaleSource(t *testing.T) {
	for _, testCase := range []struct {
		name string
		args map[string]any
		code string
	}{
		{name: "unknown rule", args: map[string]any{"decision": takeoverDecisionTakeoverB, "rule_id": "missing", "source_profile_id": "spark", "target_profile_id": "twilight"}, code: "persona_takeover_rule_not_found"},
		{name: "target mismatch", args: map[string]any{"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "undeclared"}, code: "persona_takeover_target_mismatch"},
		{name: "stale source", args: map[string]any{"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "twilight", "target_profile_id": "twilight"}, code: "persona_takeover_source_profile_stale"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			ownerID, fluctlightID, conversationID := "reject-owner-"+testCase.name, "reject-fluctlight-"+testCase.name, "reject-conversation-"+testCase.name
			takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
			app := newTestApp(t, repository, newFakeProviderRouter())
			receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{
				WorkingProfileID: "spark", CapabilityName: personaTakeoverCapabilityName,
				OperationID: "reject-" + testCase.name, EvidenceID: "reject-evidence-" + testCase.name,
				AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
				Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(testCase.args),
			})
			if err != nil || receipt.Result.Status != "rejected" || receipt.Result.ErrorCode != testCase.code {
				t.Fatalf("receipt=%#v err=%v", receipt, err)
			}
			if _, exists := mapValue(receipt.Result.Output)["working_persona"]; exists {
				t.Fatalf("rejected takeover returned working persona: %#v", receipt.Result.Output)
			}
			if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
				t.Fatalf("rejected takeover changed persistent profile to %q", active)
			}
		})
	}
}

func TestExecuteToolRejectsUndeclaredWorkingProfileBeforeCapability(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "working-reject-owner", "working-reject-fluctlight", "working-reject-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	app := newTestApp(t, repository, newFakeProviderRouter())
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		WorkingProfileID: "undeclared", CapabilityName: "relationship.lookup", OperationID: "undeclared-working-profile",
		EvidenceID: "undeclared-working-profile-evidence", AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		ConversationID: conversationID, Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"target_actor_id": ownerID}),
	})
	if err == nil || !strings.Contains(err.Error(), "working_profile_not_found") || receipt.Result.Status != "" {
		t.Fatalf("receipt=%#v err=%v", receipt, err)
	}
}
