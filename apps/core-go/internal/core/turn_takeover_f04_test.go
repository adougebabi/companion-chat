package core

import (
	"strings"
	"testing"
)

func TestDeclaredPersonaTakeoverReturnsAuthoritativeWorkingPersonaWithoutPersistentWrite(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "declared-owner", "declared-fluctlight", "declared-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	app := newTestApp(t, repository, newFakeProviderRouter())
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		WorkingProfileID: "spark", CapabilityName: personaTakeoverCapabilityName,
		OperationID: "declared-takeover", EvidenceID: "declared-takeover-evidence",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{
			"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "twilight",
		}),
	})
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("receipt=%#v err=%v", receipt, err)
	}
	working := mapValue(mapValue(receipt.Result.Output)["working_persona"])
	if stringValue(working["id"]) != "twilight" || !strings.Contains(jsonString(mapValue(working["working_persona"])), takeoverChainTwilightMarker) || len(mapValue(working["personality"])) != 0 {
		t.Fatalf("working persona=%#v", working)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("takeover changed persistent active profile to %q", active)
	}

	replayed, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		WorkingProfileID: "spark", CapabilityName: personaTakeoverCapabilityName,
		OperationID: "declared-takeover", EvidenceID: "declared-takeover-evidence",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{
			"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "twilight",
		}),
	})
	if err != nil || !replayed.Replayed || !boolValueForTest(mapValue(replayed.Result.Output)["replayed"]) {
		t.Fatalf("replay=%#v err=%v", replayed, err)
	}
	if count := takeoverChainCount(t, ctx, repository, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_type='persona_action' AND fluctlight_id=$1 AND kind='persona.takeover.committed'`, fluctlightID); count != 1 {
		t.Fatalf("takeover audit count=%d", count)
	}
}
