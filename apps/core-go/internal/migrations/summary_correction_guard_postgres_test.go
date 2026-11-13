package migrations

import (
	"strings"
	"testing"
)

func TestSummaryCorrectionGuardMigrationFromPreviousHeadAndRerun(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatalf("empty database to head: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('migration-0106-owner','human','active'),('migration-0106-character','fluctlight','active');
		INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance)
		VALUES('migration-0106-character','migration-0106-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');
		INSERT INTO public.runtime_settings(key,value_json) VALUES('test.summary_correction_0106','{"preserved":true}')`); err != nil {
		t.Fatal(err)
	}
	assertHead := func() {
		t.Helper()
		var head, functionBody, refreshBody, correctionBody string
		var preserved, actorPreserved bool
		err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head)
		if err == nil {
			err = pool.QueryRow(ctx, `SELECT pg_get_functiondef('public.memory_source_is_live(text,text,integer,text)'::regprocedure),pg_get_functiondef('public.refresh_resident_memory_snapshot(text)'::regprocedure)`).Scan(&functionBody, &refreshBody)
		}
		if err == nil {
			err = pool.QueryRow(ctx, `SELECT pg_get_functiondef(to_regprocedure($1))`, "public.summary_contains_corrected_memory_source(text,text,integer,integer)").Scan(&correctionBody)
		}
		if err == nil {
			err = pool.QueryRow(ctx, `SELECT (value_json::jsonb->>'preserved')::boolean FROM public.runtime_settings WHERE key='test.summary_correction_0106'`).Scan(&preserved)
		}
		if err == nil {
			err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlights WHERE id='migration-0106-character' AND created_by_actor_id='migration-0106-owner')`).Scan(&actorPreserved)
		}
		if err != nil || head != Head || !preserved || !actorPreserved || !strings.Contains(functionBody, "summary_contains_corrected_memory_source") || !strings.Contains(refreshBody, "memory_has_corrected_summary_source") {
			t.Fatalf("head=%q preserved=%v owner=%v liveness_guard=%v resident_guard=%v err=%v", head, preserved, actorPreserved, strings.Contains(functionBody, "summary_contains_corrected_memory_source"), strings.Contains(refreshBody, "memory_has_corrected_summary_source"), err)
		}
		if !strings.Contains(correctionBody, "source.source_kind='legacy'") || !strings.Contains(correctionBody, "source.status='unknown'") {
			t.Fatal("migration lost the resolvable legacy summary correction fence")
		}
		if strings.Contains(refreshBody, "support.source_kind IS NULL") || !strings.Contains(refreshBody, "AND public.active_memory_source_is_live(") {
			t.Fatal("resident publisher retained the unknown-source injection bypass")
		}
		var blocked bool
		if err := pool.QueryRow(ctx, `SELECT public.summary_contains_corrected_memory_source('migration-0106-character','missing',1,2)`).Scan(&blocked); err != nil || blocked {
			t.Fatalf("empty correction source blocked=%v err=%v", blocked, err)
		}
	}
	assertHead()

	// Recreate the immediately preceding head's liveness/refresh definitions
	// and remove the new functions. All other data remains intact.
	_, originalRefresh, ok := strings.Cut(residentMemorySchemaSQL, "CREATE OR REPLACE FUNCTION public.refresh_resident_memory_snapshot")
	if !ok {
		t.Fatal("old resident function missing")
	}
	originalRefresh, _, ok = strings.Cut(originalRefresh, "\nDO $resident_backfill$")
	if !ok {
		t.Fatal("old resident function terminator missing")
	}
	oldFunctions := `CREATE OR REPLACE FUNCTION public.memory_source_is_live(source_kind text,source_id text,source_revision integer,source_fingerprint text)
		RETURNS boolean LANGUAGE sql STABLE AS $old$ SELECT public.memory_source_is_live_deep($1,$2,$3,$4,0,ARRAY[]::text[]); $old$;` +
		"\nCREATE OR REPLACE FUNCTION public.refresh_resident_memory_snapshot" + originalRefresh + `
		DROP FUNCTION public.memory_has_corrected_summary_source(text,text,integer);
		DROP FUNCTION public.memory_retained_typed_sources(text,integer);
		DROP FUNCTION public.summary_contains_corrected_memory_source(text,text,integer,integer);
		UPDATE public.alembic_version SET version_num='0105_agent_model_sampling';`
	if _, err := pool.Exec(ctx, oldFunctions); err != nil {
		t.Fatalf("prepare 0105 fixture: %v", err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatalf("upgrade 0105 to 0106: %v", err)
	}
	assertHead()
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatalf("repeat 0106: %v", err)
	}
	assertHead()
	// Reinstall the historical publisher at the already-current head. Apply
	// must update its idempotent function without a data backfill or new head.
	legacyRefresh := strings.Replace(originalRefresh,
		"AND public.active_memory_source_is_live(support.source_kind,support.source_fact_id,support.source_fingerprint,support.command_id)",
		"AND (support.source_kind IS NULL OR support.source_kind='' OR public.active_memory_source_is_live(support.source_kind,support.source_fact_id,support.source_fingerprint,support.command_id))", 1)
	if legacyRefresh == originalRefresh {
		t.Fatal("legacy publisher fixture was not constructed")
	}
	if _, err := pool.Exec(ctx, "CREATE OR REPLACE FUNCTION public.refresh_resident_memory_snapshot"+legacyRefresh); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	assertHead()
}
