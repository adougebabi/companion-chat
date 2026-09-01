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
	if Head != "0023_composite_action_targets" {
		t.Fatalf("Head = %q", Head)
	}
}

func TestCompositeActionSchemaIncludesMessageMediaTarget(t *testing.T) {
	if !strings.Contains(schemaSQL, "message_id varchar(128)") {
		t.Fatal("media_intents must persist a concrete conversation message target")
	}
	if !strings.Contains(compatibilitySQL, "ADD COLUMN IF NOT EXISTS message_id") {
		t.Fatal("compatibility SQL must add message_id for existing databases")
	}
	if !strings.Contains(compatibilitySQL, "ix_media_intents_message") {
		t.Fatal("compatibility SQL must index message targets")
	}
}
