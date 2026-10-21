package migrations

import "testing"

// New additive heads must retain every released predecessor and its immutable SQL.
func assertCurrentAdditiveChain(t *testing.T) {
	t.Helper()
	got := []string{Head, WardrobeWorldStructurePreviousHead, PlaceDefaultsPreviousHead, BusinessBackupsPreviousHead, WorkflowControlsPreviousHead, DiagnosticOperationsPreviousHead, ConfigurationCommandsPreviousHead, MomentNPCInteractionsPreviousHead, MomentFeedPreviousHead, MediaLifecyclePreviousHead, RoleMediaReferencesPreviousHead, OwnerMediaRequestsPreviousHead, MediaReferenceGrantsPreviousHead, OwnerMediaPreviousHead, StudioOutfitsPreviousHead, WorldItemsPreviousHead, StudioFoundationPreviousHead, StudioVisualStylePreviousHead, StudioMediaPreviousHead, StudioJobsPreviousHead, WorldEventsPreviousHead, WorldEncounterRuntimePreviousHead, PromiseLifecyclePreviousHead, OutfitHistoryPreviousHead, InventoryManagementPreviousHead, VisualBriefPreviousHead, PersonaGovernancePreviousHead, SelfGovernancePreviousHead, DiaryPreviousHead, RoleAutonomyPreviousHead, NotificationCenterPreviousHead}
	want := []string{"0096_wardrobe_world_structure", "0095_place_defaults", "0094_business_backups", "0093_workflow_controls", "0092_diagnostic_operations", "0091_configuration_commands", "0090_moment_npc_interactions", "0089_moment_feed", "0088_media_lifecycle", "0087_role_media_references", "0086_owner_media_requests", "0085_media_reference_grants", "0084_owner_media", "0083_studio_outfits", "0082_world_items", "0081_studio_foundation", "0080_studio_visual_style", "0079_studio_media", "0078_studio_jobs", "0077_world_events", "0076_world_encounter_runtime", "0075_promise_lifecycle", "0074_outfit_history", "0073_inventory_management", "0072_visual_brief", "0071_persona_governance", "0070_self_governance", "0069_diary", "0068_role_autonomy", "0067_notification_center", "0066_life_place"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("additive chain[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}
