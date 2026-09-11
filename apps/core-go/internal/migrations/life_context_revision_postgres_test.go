package migrations

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func applyMemoryHeadLifeContextFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, step := range []struct {
		name string
		sql  string
	}{
		{name: "schema", sql: schemaSQL},
		{name: "compatibility", sql: compatibilitySQL},
		{name: "capability_runtime", sql: capabilityRuntimeMigrationSQL},
		{name: "project_health", sql: projectHealthEvolutionMigrationSQL},
		{name: "affect", sql: affectCanonicalMigrationSQL},
		{name: "memory", sql: memoryLifecycleMigrationSQL},
	} {
		if _, err := pool.Exec(ctx, step.sql); err != nil {
			t.Fatalf("apply %s fixture: %v", step.name, err)
		}
	}
	if _, err := pool.Exec(ctx, `
ALTER TABLE public.life_events DROP COLUMN IF EXISTS revision;
ALTER TABLE public.life_events DROP COLUMN IF EXISTS request_digest;
ALTER TABLE public.life_events DROP COLUMN IF EXISTS result;
ALTER TABLE public.life_events DROP COLUMN IF EXISTS updated_at;
ALTER TABLE public.life_events ALTER COLUMN idempotency_key DROP NOT NULL;
ALTER TABLE public.life_presence_overlays DROP COLUMN IF EXISTS status;
ALTER TABLE public.life_presence_overlays DROP COLUMN IF EXISTS revision;
ALTER TABLE public.life_presence_overlays DROP COLUMN IF EXISTS superseded_by_overlay_id;
ALTER TABLE public.life_presence_overlays DROP COLUMN IF EXISTS idempotency_key;
ALTER TABLE public.life_presence_overlays DROP COLUMN IF EXISTS request_digest;
ALTER TABLE public.life_presence_overlays DROP COLUMN IF EXISTS result;
ALTER TABLE public.life_presence_overlays DROP COLUMN IF EXISTS updated_at;
ALTER TABLE public.life_schedules DROP COLUMN IF EXISTS result;
ALTER TABLE public.life_schedules DROP COLUMN IF EXISTS updated_at;
ALTER TABLE public.life_schedules ALTER COLUMN idempotency_key DROP NOT NULL;
ALTER TABLE public.life_schedules ALTER COLUMN request_digest DROP NOT NULL;
ALTER TABLE public.life_schedule_items DROP COLUMN IF EXISTS location;
DROP TABLE IF EXISTS public.life_context_commands;
`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.alembic_version(version_num) VALUES($1)`, MemoryLifecycleHead); err != nil {
		t.Fatal(err)
	}
}

func assertLifeContextMigrationRollback(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	assertLifeContextMigrationRollbackWithPreexistingColumns(t, ctx, pool, 0)
}

func assertLifeContextMigrationRollbackWithPreexistingColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool, preexistingColumns int) {
	t.Helper()
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != MemoryLifecycleHead {
		t.Fatalf("failed 0030 advanced ledger: head=%q err=%v", head, err)
	}
	checks := map[string]string{
		"constraints":   `SELECT count(*) FROM pg_constraint WHERE conname IN ('ck_life_events_revision_v1','ck_life_presence_revision_v1','ck_life_schedules_active_authority_v1','ck_life_context_commands_v1')`,
		"triggers":      `SELECT count(*) FROM pg_trigger WHERE tgname IN ('ct_life_events_replay_ready_v1','ct_life_presence_replay_ready_v1','ct_life_schedules_replay_ready_v1')`,
		"function":      `SELECT count(*) FROM pg_proc WHERE proname='fluctlight_life_authority_replay_ready_v1'`,
		"command table": `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='life_context_commands'`,
		"columns":       `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND ((table_name='life_events' AND column_name IN ('revision','request_digest','result','updated_at')) OR (table_name='life_presence_overlays' AND column_name IN ('status','revision','superseded_by_overlay_id','idempotency_key','request_digest','result','updated_at')) OR (table_name='life_schedules' AND column_name IN ('result','updated_at')) OR (table_name='life_schedule_items' AND column_name='location'))`,
	}
	for name, query := range checks {
		var artifactCount int
		expected := 0
		if name == "columns" {
			expected = preexistingColumns
		}
		if err := pool.QueryRow(ctx, query).Scan(&artifactCount); err != nil || artifactCount != expected {
			t.Fatalf("failed 0030 left %s=%d want=%d err=%v", name, artifactCount, expected, err)
		}
	}
}

func TestLifeContextRevisionMigrationUpgradesMemoryHeadAndIsIdempotent(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyMemoryHeadLifeContextFixture(t, ctx, pool)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != Head {
		t.Fatalf("head=%q err=%v", head, err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatalf("0030 rerun: %v", err)
	}
	var commandTable, scheduleResultColumn int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='life_context_commands'`).Scan(&commandTable); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='life_schedules' AND column_name='result'`).Scan(&scheduleResultColumn); err != nil {
		t.Fatal(err)
	}
	if commandTable != 1 || scheduleResultColumn != 1 {
		t.Fatalf("0030 command/schedule result schema missing: commands=%d schedule_result=%d", commandTable, scheduleResultColumn)
	}
	var requiredIdentityColumns, replayTriggers int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND ((table_name='life_events' AND column_name IN ('idempotency_key','request_digest')) OR (table_name='life_presence_overlays' AND column_name IN ('idempotency_key','request_digest')) OR (table_name='life_schedules' AND column_name IN ('idempotency_key','request_digest'))) AND is_nullable='NO'`).Scan(&requiredIdentityColumns); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid WHERE (t.tgname,t.tgrelid) IN (('ct_life_events_replay_ready_v1','public.life_events'::regclass),('ct_life_presence_replay_ready_v1','public.life_presence_overlays'::regclass),('ct_life_schedules_replay_ready_v1','public.life_schedules'::regclass)) AND t.tgconstraint<>0 AND t.tgdeferrable AND t.tginitdeferred AND t.tgenabled='O' AND p.proname='fluctlight_life_authority_replay_ready_v1'`).Scan(&replayTriggers); err != nil {
		t.Fatal(err)
	}
	if requiredIdentityColumns != 6 || replayTriggers != 3 {
		t.Fatalf("0030 strict replay schema columns=%d triggers=%d", requiredIdentityColumns, replayTriggers)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,status,revision,evidence_refs,idempotency_key,request_digest,result) VALUES('bad-life-event','fl','test',now(),now()+interval '1 hour','confirmed',0,'[]','bad-life-event',$1,'{}')`, strings.Repeat("a", 32)); err == nil {
		t.Fatal("post-cutover Event constraint accepted revision zero")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,result) VALUES('bad-active-schedule','fl',CURRENT_DATE,'UTC','accepted','bad','[]',1,'{}')`); err == nil {
		t.Fatal("post-cutover Schedule constraint accepted active row without replay authority")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,idempotency_key,request_digest,result) VALUES('bad-zero-schedule','fl',CURRENT_DATE,'UTC','accepted','bad','[]',0,'bad-zero-schedule',$1,'{"id":"bad-zero-schedule","status":"accepted","revision":0,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_after","replayed":false}')`, strings.Repeat("a", 32)); err == nil {
		t.Fatal("post-cutover Schedule constraint accepted revision zero")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.life_context_commands(id,fluctlight_id,command_type,idempotency_key,request_digest,result) VALUES('bad-command','fl','event.cancel','bad-command','not-a-digest','{}')`); err == nil {
		t.Fatal("post-cutover Life Context command constraint accepted invalid digest")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.life_context_commands(id,fluctlight_id,command_type,target_id,idempotency_key,request_digest,result) VALUES('bad-command-shape','fl','event.cancel','event-1','bad-command-shape',$1,'{"foo":"bar"}')`, strings.Repeat("a", 32)); err == nil {
		t.Fatal("post-cutover Life Context command constraint accepted a non-replayable result")
	}
}

func TestLifeContextRevisionMigrationRejectsNonemptyPreviousHead(t *testing.T) {
	for _, testCase := range []struct {
		name string
		seed string
	}{
		{name: "Actor", seed: `INSERT INTO public.actors(id,actor_type,status) VALUES('legacy-owner','human','active')`},
		{name: "Goal", seed: `INSERT INTO public.fluctlight_goals(id,fluctlight_id,source,description,importance,urgency,progress,status,evidence_refs) VALUES('legacy-goal','fl','model','goal','0.5','0.5','0','active','[]')`},
		{name: "Provider configuration", seed: `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose) VALUES('legacy-provider','openai_compatible','http://legacy.invalid','legacy-secret')`},
		{name: "Workflow intent", seed: `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES('legacy-intent','legacy-workflow','lifecycle','legacy.test','{}')`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, pool := isolatedMigrationPool(t)
			applyMemoryHeadLifeContextFixture(t, ctx, pool)
			if _, err := pool.Exec(ctx, testCase.seed); err != nil {
				t.Fatal(err)
			}
			err := New(pool).Apply(ctx)
			if err == nil || !strings.Contains(err.Error(), "clean-start cutover") {
				t.Fatalf("nonempty previous head migration err=%v", err)
			}
			assertLifeContextMigrationRollback(t, ctx, pool)
		})
	}
}

func TestLifeContextRevisionMigrationFailsClosed(t *testing.T) {
	t.Run("unprepared Event", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyMemoryHeadLifeContextFixture(t, ctx, pool)
		if _, err := pool.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,status,evidence_refs,idempotency_key) VALUES('legacy-event','fl','test',now(),now()+interval '1 hour','confirmed','["fact"]','legacy-event')`); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("unprepared Event crossed 0030")
		}
		assertLifeContextMigrationRollback(t, ctx, pool)
	})

	t.Run("unprepared Presence", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyMemoryHeadLifeContextFixture(t, ctx, pool)
		if _, err := pool.Exec(ctx, `INSERT INTO public.life_presence_overlays(id,fluctlight_id,actor_id,current_task) VALUES('legacy-presence','fl','owner','reading')`); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("unprepared Presence crossed 0030")
		}
		assertLifeContextMigrationRollback(t, ctx, pool)
	})

	t.Run("multiple active Presence rows", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyMemoryHeadLifeContextFixture(t, ctx, pool)
		if _, err := pool.Exec(ctx, `
ALTER TABLE public.life_presence_overlays ADD COLUMN status varchar(32) NOT NULL DEFAULT 'active';
ALTER TABLE public.life_presence_overlays ADD COLUMN revision integer NOT NULL DEFAULT 1;
ALTER TABLE public.life_presence_overlays ADD COLUMN superseded_by_overlay_id varchar(128);
ALTER TABLE public.life_presence_overlays ADD COLUMN idempotency_key varchar(256);
ALTER TABLE public.life_presence_overlays ADD COLUMN request_digest varchar(128);
ALTER TABLE public.life_presence_overlays ADD COLUMN result jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.life_presence_overlays ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO public.life_presence_overlays(id,fluctlight_id,actor_id,current_task,status,revision,idempotency_key,request_digest,result) VALUES('presence-a','fl','owner','a','active',1,'presence-a',$1,'{}'),('presence-b','fl','owner','b','active',1,'presence-b',$2,'{}')`, strings.Repeat("a", 32), strings.Repeat("b", 32)); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("multiple active Presence rows crossed 0030")
		}
		assertLifeContextMigrationRollbackWithPreexistingColumns(t, ctx, pool, 7)
	})

	t.Run("active Schedule without replay authority", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyMemoryHeadLifeContextFixture(t, ctx, pool)
		if _, err := pool.Exec(ctx, `INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision) VALUES('legacy-active-schedule','fl',CURRENT_DATE,'UTC','accepted','legacy','["fact"]',1)`); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("active Schedule without replay authority crossed 0030")
		}
		assertLifeContextMigrationRollback(t, ctx, pool)
	})
}

func TestLifeContextRevisionDeferredConstraintsValidateCommittedReplayState(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyMemoryHeadLifeContextFixture(t, ctx, pool)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 32)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,status,revision,evidence_refs,idempotency_key,request_digest,result) VALUES('staged-event','fl','test',now(),now()+interval '1 hour','confirmed',1,'["fact"]','staged-event',$1,'{}')`, digest); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	result := `{"id":"staged-event","status":"confirmed","revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_after","replayed":false}`
	if _, err := tx.Exec(ctx, `UPDATE public.life_events SET result=$2::jsonb WHERE id=$1`, "staged-event", result); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("deferred constraint rejected completed staged result: %v", err)
	}
	for _, valid := range []struct {
		name   string
		insert string
		update string
	}{
		{name: "Presence", insert: `INSERT INTO public.life_presence_overlays(id,fluctlight_id,actor_id,current_task,status,revision,idempotency_key,request_digest,result,expires_at) VALUES('staged-presence','fl-staged-presence','owner','reading','active',1,'staged-presence',$1,'{}',now()+interval '1 hour')`, update: `UPDATE public.life_presence_overlays SET result='{"id":"staged-presence","status":"active","revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_after","replayed":false}' WHERE id='staged-presence'`},
		{name: "Schedule", insert: `INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,idempotency_key,request_digest,result) VALUES('staged-schedule','fl-staged-schedule',CURRENT_DATE,'UTC','accepted','test','[]',1,'staged-schedule',$1,'{}')`, update: `UPDATE public.life_schedules SET result='{"id":"staged-schedule","status":"accepted","revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_after","replayed":false}' WHERE id='staged-schedule'`},
	} {
		t.Run("staged "+valid.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, valid.insert, digest); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, valid.update); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatalf("deferred constraint rejected staged %s: %v", valid.name, err)
			}
		})
	}

	for _, testCase := range []struct {
		name string
		sql  string
		args []any
	}{
		{name: "Event", sql: `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,status,revision,evidence_refs,idempotency_key,request_digest,result) VALUES('invalid-event','fl-invalid-event','test',now(),now()+interval '1 hour','confirmed',1,'[]','invalid-event',$1,'{}')`, args: []any{digest}},
		{name: "Presence", sql: `INSERT INTO public.life_presence_overlays(id,fluctlight_id,actor_id,current_task,status,revision,idempotency_key,request_digest,result,expires_at) VALUES('invalid-presence','fl-invalid-presence','owner','reading','active',1,'invalid-presence',$1,'{}',now()+interval '1 hour')`, args: []any{digest}},
		{name: "Schedule", sql: `INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,idempotency_key,request_digest,result) VALUES('invalid-schedule','fl-invalid-schedule',CURRENT_DATE,'UTC','accepted','test','[]',1,'invalid-schedule',$1,'{}')`, args: []any{digest}},
		{name: "historical Event insert", sql: `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,status,revision,evidence_refs,idempotency_key,request_digest,result) VALUES('invalid-historical-event','fl-invalid-historical','test',now()-interval '2 hours',now()-interval '1 hour','confirmed',1,'[]','invalid-historical-event',$1,'{}')`, args: []any{digest}},
		{name: "expired Presence insert", sql: `INSERT INTO public.life_presence_overlays(id,fluctlight_id,actor_id,current_task,status,revision,idempotency_key,request_digest,result,expires_at) VALUES('invalid-expired-presence','fl-invalid-expired','owner','reading','active',1,'invalid-expired-presence',$1,'{}',now()-interval '1 hour')`, args: []any{digest}},
		{name: "past Schedule insert", sql: `INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,idempotency_key,request_digest,result) VALUES('invalid-past-schedule','fl-invalid-past',CURRENT_DATE-1,'UTC','accepted','test','[]',1,'invalid-past-schedule',$1,'{}')`, args: []any{digest}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, testCase.sql, testCase.args...); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatalf("staging row failed before deferred check: %v", err)
			}
			if err := tx.Commit(ctx); err == nil {
				t.Fatal("deferred replay-ready constraint accepted incomplete active authority")
			}
		})
	}
}
