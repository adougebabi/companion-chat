package migrations

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func isolatedMigrationPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	adminConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	databaseName := fmt.Sprintf("lac_affect_migration_%d", time.Now().UnixNano())
	if !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(databaseName) {
		adminPool.Close()
		t.Fatalf("unsafe temporary database name %q", databaseName)
	}
	identifier := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}
	testConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP DATABASE "+identifier)
		adminPool.Close()
		t.Fatal(err)
	}
	testConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP DATABASE "+identifier)
		adminPool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if _, dropErr := adminPool.Exec(cleanupCtx, "DROP DATABASE "+identifier); dropErr != nil {
			t.Errorf("drop isolated PostgreSQL database %s: %v", databaseName, dropErr)
		}
		adminPool.Close()
	})
	return ctx, pool
}

func applyProjectHealthHeadFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, step := range []struct {
		name string
		sql  string
	}{
		{name: "schema", sql: schemaSQL},
		{name: "compatibility", sql: compatibilitySQL},
		{name: "capability_runtime", sql: capabilityRuntimeMigrationSQL},
		{name: "project_health", sql: projectHealthEvolutionMigrationSQL},
	} {
		if _, err := pool.Exec(ctx, step.sql); err != nil {
			t.Fatalf("apply %s fixture: %v", step.name, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.alembic_version(version_num) VALUES($1)`, ProjectHealthHead); err != nil {
		t.Fatal(err)
	}
}

func applyCapabilityRuntimeHeadFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, step := range []struct {
		name string
		sql  string
	}{
		{name: "schema", sql: schemaSQL},
		{name: "compatibility", sql: compatibilitySQL},
		{name: "capability_runtime", sql: capabilityRuntimeMigrationSQL},
	} {
		if _, err := pool.Exec(ctx, step.sql); err != nil {
			t.Fatalf("apply %s fixture: %v", step.name, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.alembic_version(version_num) VALUES($1)`, CapabilityRuntimeHead); err != nil {
		t.Fatal(err)
	}
}

func seedAffectMigrationState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, suffix string, momentum string) string {
	t.Helper()
	ownerID := "affect-migration-owner-" + suffix
	fluctlightID := "affect-migration-fluctlight-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,4,'{"pleasure":0.2,"arousal":-0.4,"dominance":0.6}','{"label":"mixed","intensity":0.5}',$2::jsonb,'{"stress":0.2,"stability":0.8}','[]','[]',now())`, fluctlightID, momentum); err != nil {
		t.Fatal(err)
	}
	return fluctlightID
}

func TestAffectCanonicalMigrationUpgradesProjectHealthHeadAndPreservesState(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyProjectHealthHeadFixture(t, ctx, pool)
	fluctlightID := seedAffectMigrationState(t, ctx, pool, "valid", `{"value":-0.3,"trend":0.2,"pleasure_momentum":-0.1,"arousal_momentum":0.4,"dominance_momentum":-0.5}`)
	if _, err := pool.Exec(ctx, `DELETE FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, affectCanonicalMigrationSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, AffectCanonicalHead); err != nil {
		t.Fatal(err)
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != AffectCanonicalHead {
		t.Fatalf("head=%q err=%v", head, err)
	}
	var driveHalfLife float64
	if err := pool.QueryRow(ctx, `SELECT (decay_policy->>'drive_half_life_seconds')::double precision FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1`, fluctlightID).Scan(&driveHalfLife); err != nil || driveHalfLife != 14400 {
		t.Fatalf("drive half-life=%v err=%v", driveHalfLife, err)
	}
	var reconciliationCount int
	var preserved bool
	if err := pool.QueryRow(ctx, `SELECT count(*),COALESCE(bool_and(before_state=after_state),false) FROM public.fluctlight_affect_reconciliations WHERE fluctlight_id=$1 AND source_head=$2`, fluctlightID, ProjectHealthHead).Scan(&reconciliationCount, &preserved); err != nil {
		t.Fatal(err)
	}
	if reconciliationCount != 1 || !preserved {
		t.Fatalf("reconciliation count=%d preserved=%v", reconciliationCount, preserved)
	}
	var constraintCount, sourceIndexCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conname IN ('ck_fluctlight_inner_state_affect_ranges_v2','ck_fluctlight_affect_profile_policy_v2','ck_fluctlight_drive_slot_pressure_v2')`).Scan(&constraintCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname='uq_fluctlight_state_revisions_source'`).Scan(&sourceIndexCount); err != nil {
		t.Fatal(err)
	}
	if constraintCount != 3 || sourceIndexCount != 1 {
		t.Fatalf("constraints=%d source_index=%d", constraintCount, sourceIndexCount)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.fluctlight_inner_states SET pad='{}' WHERE fluctlight_id=$1`, fluctlightID); err == nil {
		t.Fatal("post-cutover constraint accepted missing required PAD fields")
	}
	if _, err := pool.Exec(ctx, `UPDATE public.fluctlight_inner_states SET drives='[{"key":"social","pressure":1.2,"salience":0.5}]' WHERE fluctlight_id=$1`, fluctlightID); err == nil {
		t.Fatal("post-cutover constraint accepted out-of-range Drive pressure")
	}
	if _, err := pool.Exec(ctx, `UPDATE public.fluctlight_affect_profiles SET decay_policy='{}' WHERE fluctlight_id=$1`, fluctlightID); err == nil {
		t.Fatal("post-cutover constraint accepted missing required decay fields")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_drive_slots(id,fluctlight_id,key,label,description,value_schema,value,confidence,evidence_refs) VALUES('invalid-drive',$1,'invalid','invalid','invalid','pressure','{"pressure":1.2,"salience":0.5}','0.8','[]')`, fluctlightID); err == nil {
		t.Fatal("post-cutover constraint accepted invalid typed Drive pressure")
	}
	if _, err := pool.Exec(ctx, affectCanonicalMigrationSQL); err != nil {
		t.Fatalf("canonical migration rerun: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_affect_reconciliations WHERE fluctlight_id=$1 AND source_head=$2`, fluctlightID, ProjectHealthHead).Scan(&reconciliationCount); err != nil || reconciliationCount != 1 {
		t.Fatalf("rerun reconciliation count=%d err=%v", reconciliationCount, err)
	}
	if err := New(pool).Apply(ctx); err == nil || !strings.Contains(err.Error(), "clean-start cutover") {
		t.Fatalf("later clean-start revisions accepted Affect business data: %v", err)
	}
}

func TestMigrationRoutesCapabilityRuntimeThroughProjectHealthAndAffectAtomically(t *testing.T) {
	t.Run("valid 0026 runs both later revisions", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyCapabilityRuntimeHeadFixture(t, ctx, pool)
		fluctlightID := seedAffectMigrationState(t, ctx, pool, "route-valid", `{"value":-0.2,"trend":0.1}`)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, projectHealthEvolutionMigrationSQL); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, affectCanonicalMigrationSQL); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, AffectCanonicalHead); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var head string
		if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != AffectCanonicalHead {
			t.Fatalf("head=%q err=%v", head, err)
		}
		var projectHealthReconciliation, affectReconciliation int
		if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE source_head=$2),count(*) FILTER (WHERE source_head=$3) FROM public.fluctlight_affect_reconciliations WHERE fluctlight_id=$1`, fluctlightID, CapabilityRuntimeHead, ProjectHealthHead).Scan(&projectHealthReconciliation, &affectReconciliation); err != nil {
			t.Fatal(err)
		}
		if projectHealthReconciliation != 1 || affectReconciliation != 1 {
			t.Fatalf("0026->0027->0028 effects missing: project_health=%d affect=%d", projectHealthReconciliation, affectReconciliation)
		}
	})

	t.Run("0028 failure rolls 0027 effects back to 0026", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyCapabilityRuntimeHeadFixture(t, ctx, pool)
		fluctlightID := seedAffectMigrationState(t, ctx, pool, "route-bad", `{"value":0.1,"pleasure_momentum":1.2}`)
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("malformed 0028 state unexpectedly migrated")
		}
		var head string
		if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != CapabilityRuntimeHead {
			t.Fatalf("failed later revision advanced ledger: head=%q err=%v", head, err)
		}
		var reconciliationCount, profileCount int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_affect_reconciliations WHERE fluctlight_id=$1`, fluctlightID).Scan(&reconciliationCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1`, fluctlightID).Scan(&profileCount); err != nil {
			t.Fatal(err)
		}
		if reconciliationCount != 0 || profileCount != 0 {
			t.Fatalf("failed 0028 left 0027 effects: reconciliations=%d profiles=%d", reconciliationCount, profileCount)
		}
	})
}

func TestMigrationRejectsWhitespacePaddedLedgerHead(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		t.Fatal(err)
	}
	padded := " " + Head + " "
	if _, err := pool.Exec(ctx, `INSERT INTO public.alembic_version(version_num) VALUES($1)`, padded); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err == nil {
		t.Fatal("noncanonical migration ledger head was accepted")
	}
	var count int
	var stored string
	if err := pool.QueryRow(ctx, `SELECT count(*),max(version_num) FROM public.alembic_version`).Scan(&count, &stored); err != nil {
		t.Fatal(err)
	}
	if count != 1 || stored != padded {
		t.Fatalf("noncanonical ledger was mutated: count=%d stored=%q", count, stored)
	}
}

func TestAffectCanonicalMigrationRejectsMalformedMomentumWithoutAdvancingLedger(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyProjectHealthHeadFixture(t, ctx, pool)
	fluctlightID := seedAffectMigrationState(t, ctx, pool, "malformed", `{"value":0.1,"pleasure_momentum":1.2}`)
	if err := New(pool).Apply(ctx); err == nil {
		t.Fatal("malformed bipolar momentum unexpectedly migrated")
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != ProjectHealthHead {
		t.Fatalf("failed migration advanced ledger: head=%q err=%v", head, err)
	}
	var reconciliationCount, v2ConstraintCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_affect_reconciliations WHERE fluctlight_id=$1 AND source_head=$2`, fluctlightID, ProjectHealthHead).Scan(&reconciliationCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conname='ck_fluctlight_inner_state_affect_ranges_v2'`).Scan(&v2ConstraintCount); err != nil {
		t.Fatal(err)
	}
	if reconciliationCount != 0 || v2ConstraintCount != 0 {
		t.Fatalf("failed migration left side effects: reconciliation=%d constraint=%d", reconciliationCount, v2ConstraintCount)
	}
}
