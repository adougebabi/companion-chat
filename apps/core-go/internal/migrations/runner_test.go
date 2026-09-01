package migrations

import (
	"strings"
	"testing"
)

func TestCompatibilitySQLDoesNotWriteGeneratedSearchDocument(t *testing.T) {
	if strings.Contains(compatibilitySQL, "UPDATE public.memories SET search_document") {
		t.Fatal("compatibility SQL must not update the generated memories.search_document column")
	}
}

func TestPersonalityGrowthSchemaIncludesTypedSlotsAndCapabilityRequests(t *testing.T) {
	for _, table := range []string{"cognition_appraisals", "cognition_internal_dynamics", "cognition_action_results", "fluctlight_drive_slots", "fluctlight_preference_slots", "fluctlight_trigger_preferences", "capability_requests"} {
		if !strings.Contains(schemaSQL, "public."+table) {
			t.Fatalf("schemaSQL is missing %s", table)
		}
	}
	if Head != "0022_personality_growth_loop" {
		t.Fatalf("Head = %q", Head)
	}
}
