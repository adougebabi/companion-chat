package migrations

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestReleasedProjectHealthMigrationIsImmutable(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(projectHealthEvolutionMigrationSQL)))
	const expected = "09bfe3f6d1f7113fd8f298bef421c86ba54afd56ed533dff2929e880a23703e5"
	if digest != expected {
		t.Fatalf("0027_project_health_evolution was rewritten: digest=%s", digest)
	}
}

func TestReleasedAffectCanonicalMigrationIsImmutable(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(affectCanonicalMigrationSQL)))
	const expected = "df5c7c8d6be6739920e535018efdf00e4182ce77a9771932884a58356b523b83"
	if digest != expected {
		t.Fatalf("0028_affect_canonical was rewritten: digest=%s", digest)
	}
}

func TestReleasedMemoryLifecycleMigrationIsImmutable(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(memoryLifecycleMigrationSQL)))
	const expected = "b5f66b3c0e4b331b7c3ffe2f91f050237aa4b8ecc9243fc309fca548ea8e8219"
	if digest != expected {
		t.Fatalf("0029_memory_lifecycle was rewritten: digest=%s", digest)
	}
}

func TestReleasedLifeContextRevisionMigrationIsImmutable(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(lifeContextRevisionMigrationSQL)))
	const expected = "168cbb86b84bfaf76dc7a13d654d7e964d80214a3984571be767e5c5fc27270b"
	if digest != expected {
		t.Fatalf("0030_life_context_revision was rewritten: digest=%s", digest)
	}
}

func TestReleasedEvolutionAuthorityMigrationIsImmutable(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(evolutionAuthorityMigrationSQL)))
	const expected = "0ded302af878cd0c5b5c4d544b61b9c1f976b1004061ac9ecd55655e900bb4b5"
	if digest != expected {
		t.Fatalf("0031_evolution_authority was rewritten: digest=%s", digest)
	}
}

func TestCompatibilitySQLDoesNotWriteGeneratedSearchDocument(t *testing.T) {
	if strings.Contains(compatibilitySQL, "UPDATE public.memories SET search_document") {
		t.Fatal("compatibility SQL must not update the generated memories.search_document column")
	}
}

func TestPersonalityGrowthSchemaIncludesTypedSlotsAndCapabilityRequests(t *testing.T) {
	for _, table := range []string{"cognition_appraisals", "cognition_internal_dynamics", "cognition_action_outcomes", "fluctlight_drive_slots", "fluctlight_preference_slots", "fluctlight_trigger_preferences", "fluctlight_visual_identities", "fluctlight_visual_identity_revisions", "fluctlight_visual_identity_sessions", "fluctlight_visual_identity_attempts", "fluctlight_visual_identity_timeline", "fluctlight_developing_self_claims", "fluctlight_developing_self_revisions", "capability_requests"} {
		if !strings.Contains(schemaSQL, "public."+table) {
			t.Fatalf("schemaSQL is missing %s", table)
		}
	}
	if Head != "0031_evolution_authority" || PreviousHead != "0030_life_context_revision" {
		t.Fatalf("Head = %q", Head)
	}
}

func TestEvolutionAuthorityMigrationHasIndependentSchema(t *testing.T) {
	for _, fragment := range []string{
		"0031_evolution_authority is a clean-start cutover",
		"desired_outcome text",
		"success_criteria jsonb",
		"fluctlight_intention_attempts",
		"cognition_reflection_candidate_dispositions",
		"fluctlight_evolution_states",
		"fluctlight_evolution_overlays",
		"ck_reflection_proposal_authority_v2",
		"ck_evolution_overlay_authority_v1",
	} {
		if !strings.Contains(evolutionAuthorityMigrationSQL, fragment) {
			t.Fatalf("0031 evolution authority migration missing %q", fragment)
		}
	}
}

func TestLifeContextRevisionSchemaIncludesOwnerCommandReplayAuthority(t *testing.T) {
	for _, fragment := range []string{
		"public.life_context_commands",
		"command_type varchar(64) NOT NULL",
		"UNIQUE(fluctlight_id,idempotency_key)",
		"public.life_schedules ADD COLUMN IF NOT EXISTS result",
		"clean-start cutover",
		"ALTER COLUMN idempotency_key SET NOT NULL",
		"ALTER COLUMN request_digest SET NOT NULL",
		"ck_life_schedules_active_authority_v1",
		"ck_life_context_commands_v1",
		"fluctlight_life_authority_replay_ready_v1",
		"DEFERRABLE INITIALLY DEFERRED",
	} {
		if !strings.Contains(schemaSQL+lifeContextRevisionMigrationSQL, fragment) {
			t.Fatalf("Life Context schema is missing %q", fragment)
		}
	}
	if strings.Contains(lifeContextRevisionMigrationSQL, "life_context_repairs") {
		t.Fatal("clean-start Life Context migration must not retain a repair compatibility authority")
	}
}

func TestFreshSchemaDoesNotCreateLegacyActionResultAuthority(t *testing.T) {
	if strings.Contains(schemaSQL, "CREATE TABLE IF NOT EXISTS public.cognition_action_results") {
		t.Fatal("fresh schema must not create the legacy action-result authority")
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

func TestRelationshipGovernanceSchemaIncludesActorRoleAndGoalScope(t *testing.T) {
	for _, fragment := range []string{
		"role jsonb NOT NULL DEFAULT '{}'",
		"provenance jsonb NOT NULL DEFAULT '{}'",
		"scope varchar(32) NOT NULL DEFAULT 'general'",
		"target_actor_id varchar(128)",
	} {
		if !strings.Contains(schemaSQL+compatibilitySQL, fragment) {
			t.Fatalf("relationship schema missing %q", fragment)
		}
	}
}

func TestPersonalityRuntimeSchemaExists(t *testing.T) {
	if !strings.Contains(schemaSQL, "fluctlight_personality_runtime") {
		t.Fatal("personality runtime table is missing")
	}
}

func TestPersonalityScopedAgencyRelationshipAndMemorySchema(t *testing.T) {
	for _, fragment := range []string{
		"fluctlight_goals (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, profile_id varchar(128)",
		"fluctlight_intentions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, profile_id varchar(128)",
		"relationships (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, profile_id varchar(128)",
		"personality_perspectives jsonb NOT NULL DEFAULT '[]'",
		"ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS personality_perspectives",
	} {
		if !strings.Contains(schemaSQL+compatibilitySQL, fragment) {
			t.Fatalf("personality-scoped schema is missing %q", fragment)
		}
	}
}

func TestLLMQueueSchemaIncludesGenericBindingAndLifecycleFields(t *testing.T) {
	if CapabilityRuntimePreviousHead != "0025_llm_queue" {
		t.Fatalf("CapabilityRuntimePreviousHead = %q, want 0025_llm_queue", CapabilityRuntimePreviousHead)
	}
	for _, fragment := range []string{
		"binding_role varchar(64)",
		"scenario varchar(128)",
		"queued_at timestamptz",
		"started_at timestamptz",
		"completed_at timestamptz",
		"llm.queue",
		`"generated_concurrency":1`,
		"'generic_llm'",
	} {
		if !strings.Contains(schemaSQL+compatibilitySQL, fragment) {
			t.Fatalf("LLM queue migration is missing %q", fragment)
		}
	}
}

func TestCapabilityRuntimeMigrationEnrichesOnlyExecutablePayloads(t *testing.T) {
	for _, fragment := range []string{"capability_runtime_version", "capability_invocations", "capability_context_snapshot", "cognition_frozen_actions", "autonomy_actions"} {
		if !strings.Contains(capabilityRuntimeMigrationSQL, fragment) {
			t.Fatalf("capability runtime migration is missing %q", fragment)
		}
	}
	if strings.Contains(compatibilitySQL, "Capability Runtime cutover") {
		t.Fatal("capability cutover must not run as compatibility SQL on every startup")
	}
	if !strings.Contains(capabilityRuntimeMigrationSQL, "status IN ('frozen','pending','claimed','started','running')") {
		t.Fatal("capability runtime migration must be active-row scoped")
	}
	for _, fragment := range []string{"RAISE EXCEPTION", "IS DISTINCT FROM 'object'", "jsonb_agg", "call_id", "capability_name", "- 'tool_calls'", "capability_results", "pg_temp.fluctlight_capability_snapshot", "pg_temp.fluctlight_capability_thin_arguments", "pg_temp.fluctlight_capability_required_arguments_present", "pg_temp.fluctlight_capability_context_complete", "pg_temp.fluctlight_capability_prepared_complete", "active workflow intent(s) without v2 action authority", "prepared_payload", "tool_results"} {
		if !strings.Contains(capabilityRuntimeMigrationSQL, fragment) {
			t.Fatalf("capability runtime migration missing strict fragment %q", fragment)
		}
	}
	for _, fragment := range []string{"f.payload ? 'capability_invocations' AND f.payload ? 'tool_calls'", "a.payload ? 'capability_invocations' AND a.payload ? 'tool_calls'", "f.provider_request_id", "a.provider_request_id", "(f.payload->'decision') - 'tool_calls'", "(a.payload->'decision') - 'tool_calls'"} {
		if !strings.Contains(capabilityRuntimeMigrationSQL, fragment) {
			t.Fatalf("capability runtime migration missing authority/provenance guard %q", fragment)
		}
	}
}

func TestScheduleCapabilitySchemaHasDatabaseIdempotencyBoundary(t *testing.T) {
	for _, fragment := range []string{"idempotency_key varchar(256)", "request_digest varchar(128)", "uq_life_schedules_idempotency"} {
		if !strings.Contains(schemaSQL+compatibilitySQL, fragment) {
			t.Fatalf("schedule capability idempotency schema missing %q", fragment)
		}
	}
}

func TestProjectHealthMigrationHasIndependentOutcomeAndDrainBoundary(t *testing.T) {
	for _, fragment := range []string{
		"cognition_action_outcomes",
		"UNIQUE(action_id,call_id)",
		"context_reference_version",
		"context_reference_index",
		"influences",
		"goal_refs",
		"intention_refs",
		"requires draining",
		"status IN ('frozen','pending','claimed','started','running')",
	} {
		if !strings.Contains(schemaSQL+projectHealthEvolutionMigrationSQL, fragment) {
			t.Fatalf("project health migration missing %q", fragment)
		}
	}
	if strings.Contains(compatibilitySQL, "project health evolution cutover") {
		t.Fatal("project health cutover must not run as recurring compatibility SQL")
	}
}

func TestAffectReconciliationUsesIndependentCanonicalRevision(t *testing.T) {
	for _, fragment := range []string{
		"fluctlight_affect_profiles",
		"fluctlight_affect_reconciliations",
		"preserve_numeric_subset_no_semantic_inference",
		"ck_fluctlight_inner_state_affect_ranges_v2",
		"uq_fluctlight_state_revisions_source",
		"drive_half_life_seconds",
		"canonical affect migration",
	} {
		if !strings.Contains(schemaSQL+projectHealthEvolutionMigrationSQL+affectCanonicalMigrationSQL, fragment) {
			t.Fatalf("canonical Affect migration missing %q", fragment)
		}
	}
	if strings.Contains(capabilityRuntimeMigrationSQL, "fluctlight_affect_profiles") || strings.Contains(capabilityRuntimeMigrationSQL, "project health affect reconciliation") {
		t.Fatal("Capability Runtime migration must not own Project Health affect state")
	}
}

func TestMigrationBridgeAcceptsOnlyReleasedHead(t *testing.T) {
	if ReleasedHead != "0020_media_provider_job" {
		t.Fatalf("ReleasedHead = %q", ReleasedHead)
	}
	if Head == ReleasedHead {
		t.Fatal("bridge head must differ from current Go head")
	}
	if PreviousHead != LifeContextRevisionHead || PreviousHead == Head || LifeContextRevisionHead == MemoryLifecycleHead || MemoryLifecycleHead == AffectCanonicalHead || AffectCanonicalHead == ProjectHealthHead || ProjectHealthHead == CapabilityRuntimeHead {
		t.Fatalf("Evolution/Life/Memory/Affect/Project Health migration chain is invalid: previous=%q life=%q memory=%q affect=%q project_health=%q head=%q", PreviousHead, LifeContextRevisionHead, MemoryLifecycleHead, AffectCanonicalHead, ProjectHealthHead, Head)
	}
	if CapabilityRuntimePreviousHead != "0025_llm_queue" || CapabilityRuntimeHead != "0026_capability_runtime" {
		t.Fatalf("capability migration chain is invalid: previous=%q head=%q", CapabilityRuntimePreviousHead, CapabilityRuntimeHead)
	}
}

func TestLifeContextRevisionMigrationHasIndependentAuthority(t *testing.T) {
	for _, fragment := range []string{
		"life_events ADD COLUMN IF NOT EXISTS revision", "life_presence_overlays ADD COLUMN IF NOT EXISTS revision",
		"uq_life_presence_active", "uq_life_presence_idempotency", "ck_life_events_revision_v1",
		"ck_life_presence_revision_v1", "request_digest", "result jsonb",
	} {
		if !strings.Contains(schemaSQL+lifeContextRevisionMigrationSQL, fragment) {
			t.Fatalf("Life Context migration missing %q", fragment)
		}
	}
	if strings.Contains(memoryLifecycleMigrationSQL, "ck_life_events_revision_v1") || strings.Contains(affectCanonicalMigrationSQL, "ck_life_events_revision_v1") {
		t.Fatal("released 0028/0029 migrations must not own Life Context revision")
	}
}

func TestMemoryLifecycleMigrationHasGovernanceAndFailClosedCutover(t *testing.T) {
	for _, fragment := range []string{
		"memory_governance", "memory_lifecycle_repairs", "memory.lifecycle.repair.v1", "verifiable audited repair", "canonical_key", "request_digest", "superseded_by_memory_id",
		"uq_memories_active_canonical", "uq_memory_revisions_identity", "uq_memory_embeddings_current_tuple",
		"Memory lifecycle cutover requires audited repair", "memory.lifecycle.v2",
	} {
		if !strings.Contains(schemaSQL+memoryLifecycleMigrationSQL, fragment) {
			t.Fatalf("Memory lifecycle migration missing %q", fragment)
		}
	}
	if strings.Contains(affectCanonicalMigrationSQL, "memory_governance") || strings.Contains(projectHealthEvolutionMigrationSQL, "memory_governance") {
		t.Fatal("released 0027/0028 migrations must not own Memory lifecycle")
	}
}
