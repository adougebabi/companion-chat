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
	for _, table := range []string{"cognition_appraisals", "cognition_internal_dynamics", "cognition_action_results", "fluctlight_drive_slots", "fluctlight_preference_slots", "fluctlight_trigger_preferences", "fluctlight_developing_self_claims", "fluctlight_developing_self_revisions", "capability_requests"} {
		if !strings.Contains(schemaSQL, "public."+table) {
			t.Fatalf("schemaSQL is missing %s", table)
		}
	}
	if Head != "0024_persona_layers" {
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

func TestPersonaLayerSchemaIncludesCanonicalStores(t *testing.T) {
	for _, fragment := range []string{
		"core_persona jsonb",
		"fluctlight_developing_self_claims",
		"fluctlight_developing_self_revisions",
		"ix_developing_self_claims_active",
	} {
		if !strings.Contains(schemaSQL+compatibilitySQL, fragment) {
			t.Fatalf("persona layer schema missing %q", fragment)
		}
	}
}
