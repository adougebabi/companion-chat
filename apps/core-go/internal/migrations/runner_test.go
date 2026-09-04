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
	for _, table := range []string{"cognition_appraisals", "cognition_internal_dynamics", "cognition_action_results", "fluctlight_drive_slots", "fluctlight_preference_slots", "fluctlight_trigger_preferences", "fluctlight_visual_identities", "fluctlight_visual_identity_revisions", "fluctlight_visual_identity_sessions", "fluctlight_visual_identity_attempts", "fluctlight_visual_identity_timeline", "fluctlight_developing_self_claims", "fluctlight_developing_self_revisions", "capability_requests"} {
		if !strings.Contains(schemaSQL, "public."+table) {
			t.Fatalf("schemaSQL is missing %s", table)
		}
	}
	if Head != "0025_llm_queue" {
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

func TestMediaIntentSchemaIncludesQualityGateState(t *testing.T) {
	for _, fragment := range []string{
		"provider_prompt text",
		"quality_retry_count integer",
		"quality_retry_guidance text",
		"quality_verdict varchar(16)",
		"quality_candidate_sha256 varchar(128)",
		"quality_checked_at timestamptz",
	} {
		if !strings.Contains(schemaSQL+compatibilitySQL, fragment) {
			t.Fatalf("media intent schema missing %q", fragment)
		}
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

func TestLLMQueueSchemaIncludesGenericBindingAndLifecycleFields(t *testing.T) {
	if Head != "0025_llm_queue" {
		t.Fatalf("Head = %q, want 0025_llm_queue", Head)
	}
	for _, fragment := range []string{
		"binding_role varchar(64)",
		"scenario varchar(128)",
		"queued_at timestamptz",
		"started_at timestamptz",
		"completed_at timestamptz",
		"llm.queue",
		"'generic_llm'",
	} {
		if !strings.Contains(schemaSQL+compatibilitySQL, fragment) {
			t.Fatalf("LLM queue migration is missing %q", fragment)
		}
	}
}
