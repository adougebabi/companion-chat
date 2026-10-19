package migrations

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Build the actual pre-upgrade schema from its released SQL. Downgrading a head
// database by dropping selected tables is invalid once newer tables reference
// them, and cannot prove a previous-release upgrade preserves business records.
func applyReleasedMigrationFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, target string) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, schemaSQL+compatibilitySQL); err != nil {
		t.Fatal("released base", err)
	}
	steps := []struct{ head, sql string }{
		{"0026_capability_runtime", capabilityRuntimeMigrationSQL},
		{"0027_project_health_evolution", projectHealthEvolutionMigrationSQL},
		{"0028_affect_canonical", affectCanonicalMigrationSQL},
		{"0029_memory_lifecycle", memoryLifecycleMigrationSQL},
		{"0030_life_context_revision", lifeContextRevisionMigrationSQL},
		{"0031_evolution_authority", evolutionAuthorityMigrationSQL},
		{"0032_prompt_context_memory", promptContextMemoryMigrationSQL},
		{"0033_initialization_source", initializationSourceSchemaSQL},
		{"0034_tool_execution_source", toolExecutionSourceSchemaSQL},
		{"0035_working_persona", workingPersonaSchemaSQL},
		{"0036_effective_life", effectiveLifeSchemaSQL},
		{"0037_memory_provenance", memoryProvenanceSchemaSQL + residentMemorySchemaSQL + contextAuthorityGenerationSQL},
		{"0038_scheduled_actions", scheduledLifeActionSchemaSQL},
		{"0039_message_time", messageTimeSchemaSQL},
		{"0040_activity_authority", activityAuthoritySchemaSQL},
		{"0041_agent_run_diagnostics", agentRunDiagnosticsSchemaSQL},
		{"0042_conversation_daily_memory", conversationDailyMemorySchemaSQL},
		{"0043_actor_facts", actorFactsSchemaSQL},
		{"0044_inventory_usage", inventoryUsageSchemaSQL},
		{"0045_runtime_summary", runtimeSummarySchemaSQL},
		{"0046_diagnostic_pagination", diagnosticPaginationSchemaSQL},
		{"0047_history_repair", historyRepairSchemaSQL + withdrawnSourceSchemaSQL},
		{"0048_proactive_topics", proactiveTopicsSchemaSQL},
		{"0049_actor_user_background", actorUserBackgroundSchemaSQL},
		{"0050_goal_execution", goalExecutionSchemaSQL},
		{"0051_goal_closure", goalClosureSchemaSQL + goalObjectHistorySchemaSQL + goalSourceTrackingSchemaSQL},
		{"0052_goal_governance", goalGovernanceSchemaSQL},
		{"0053_goal_reconciliation", goalReconciliationSchemaSQL},
		{"0054_goal_planner", goalPlannerSchemaSQL},
		{"0055_goal_planner_cadence", goalPlannerCadenceSchemaSQL},
		{"0056_kev_decisions", kevDecisionsSchemaSQL},
		{"0057_logical_agent_leases", logicalAgentLeasesSchemaSQL},
		{"0058_semantic_fact_generation", semanticFactGenerationSchemaSQL},
		{"0059_fact_generation_provenance", factGenerationProvenanceSchemaSQL},
		{"0060_media_kev", mediaKevSchemaSQL},
		{"0061_fact_only_reply", factOnlyReplySchemaSQL},
		{"0062_reply_media", replyMediaSchemaSQL},
		{"0063_goal_agent_cadence", goalAgentCadenceSchemaSQL},
		{"0064_world_runtime", ownerSurfaceSchemaSQL + worldRuntimeSchemaSQL},
		{"0065_world_catalog", worldCatalogSchemaSQL},
		{"0066_life_place", lifePlaceSchemaSQL},
		{"0067_notification_center", notificationCenterSchemaSQL},
		{"0068_role_autonomy", roleAutonomySchemaSQL},
		{"0069_diary", diarySchemaSQL},
		{"0070_self_governance", selfGovernanceSchemaSQL},
		{"0071_persona_governance", personaGovernanceSchemaSQL},
		{"0072_visual_brief", visualBriefSchemaSQL},
		{"0073_inventory_management", inventoryManagementSchemaSQL},
		{"0074_outfit_history", outfitHistorySchemaSQL},
		{"0075_promise_lifecycle", promiseLifecycleSchemaSQL},
		{"0076_world_encounter_runtime", worldEncounterRuntimeSchemaSQL},
		{"0077_world_events", worldEventsSchemaSQL},
	}
	found := false
	for _, step := range steps {
		if _, err = tx.Exec(ctx, step.sql); err != nil {
			t.Fatalf("released %s fixture: %v", step.head, err)
		}
		if step.head == target {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unknown released fixture head %q", target)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.alembic_version`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.alembic_version(version_num) VALUES($1)`, target); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
