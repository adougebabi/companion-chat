package migrations

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func applyLifeContextHeadEvolutionFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
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
		{name: "life_context", sql: lifeContextRevisionMigrationSQL},
	} {
		if _, err := pool.Exec(ctx, step.sql); err != nil {
			t.Fatalf("apply %s fixture: %v", step.name, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.alembic_version(version_num) VALUES($1)`, LifeContextRevisionHead); err != nil {
		t.Fatal(err)
	}
}

func assertEvolutionAuthorityRollback(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != LifeContextRevisionHead {
		t.Fatalf("failed 0031 changed ledger: head=%q err=%v", head, err)
	}
	for name, query := range map[string]string{
		"attempt table":     `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='fluctlight_intention_attempts'`,
		"disposition table": `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='cognition_reflection_candidate_dispositions'`,
		"overlay table":     `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='fluctlight_evolution_overlays'`,
		"goal columns":      `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='fluctlight_goals' AND column_name IN ('desired_outcome','success_criteria','motivation','needs_reflection','idempotency_key','request_digest')`,
	} {
		var count int
		if err := pool.QueryRow(ctx, query).Scan(&count); err != nil || count != 0 {
			t.Fatalf("failed 0031 left %s=%d err=%v", name, count, err)
		}
	}
}

func TestEvolutionAuthorityMigrationUpgradesLifeContextHeadAndIsIdempotent(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyLifeContextHeadEvolutionFixture(t, ctx, pool)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != Head {
		t.Fatalf("head=%q err=%v", head, err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatalf("0031 rerun failed: %v", err)
	}
	for name, query := range map[string]string{
		"goal fields":       `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='fluctlight_goals' AND column_name IN ('desired_outcome','success_criteria','motivation','needs_reflection','idempotency_key','request_digest') AND is_nullable='NO'`,
		"intention fields":  `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='fluctlight_intentions' AND column_name IN ('action_intent','expected_outcome','capability_constraints','idempotency_key','request_digest') AND is_nullable='NO'`,
		"reflection fields": `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='cognition_reflection_proposals' AND column_name IN ('schema_version','context_snapshot','expected_revisions','result','model_version','prompt_version','policy_version','request_digest','idempotency_key') AND is_nullable='NO'`,
		"authority tables":  `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('fluctlight_intention_attempts','cognition_reflection_candidate_dispositions','fluctlight_evolution_states','fluctlight_evolution_overlays')`,
	} {
		var count int
		if err := pool.QueryRow(ctx, query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		want := map[string]int{"goal fields": 6, "intention fields": 5, "reflection fields": 9, "authority tables": 4}[name]
		if count != want {
			t.Fatalf("%s=%d want=%d", name, count, want)
		}
	}
}

func TestEvolutionAuthorityMigrationRejectsNonemptyPreviousHeadAndRollsBack(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyLifeContextHeadEvolutionFixture(t, ctx, pool)
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('legacy-evolution-owner','human','active')`); err != nil {
		t.Fatal(err)
	}
	err := New(pool).Apply(ctx)
	if err == nil || !strings.Contains(err.Error(), "0031_evolution_authority is a clean-start cutover") {
		t.Fatalf("nonempty 0030 crossed 0031: %v", err)
	}
	assertEvolutionAuthorityRollback(t, ctx, pool)
}

func TestEvolutionAuthorityMigrationEnforcesClosedRows(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyLifeContextHeadEvolutionFixture(t, ctx, pool)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 32)
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_goals(id,fluctlight_id,profile_id,source,scope,description,desired_outcome,success_criteria,motivation,needs_reflection,importance,urgency,progress,status,evidence_refs,revision,idempotency_key,request_digest) VALUES('goal-v2','fl','default','reflection','general','finish','finish','["done"]','motivation',false,'0.8','0.6','0','active','["fact"]',1,'goal-v2',$1)`, digest); err != nil {
		t.Fatalf("valid Goal rejected: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_goals(id,fluctlight_id,profile_id,source,scope,description,desired_outcome,success_criteria,motivation,needs_reflection,importance,urgency,progress,status,evidence_refs,revision,idempotency_key,request_digest) VALUES('goal-bad','fl','default','reflection','general','bad','','[]','',true,'0.5','0.5','0','active','[]',0,'goal-bad','bad')`); err == nil {
		t.Fatal("invalid Goal authority was accepted")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_intentions(id,fluctlight_id,profile_id,goal_id,action,action_intent,expected_outcome,capability_constraints,trigger,confidence,expiration,evidence_refs,permission_snapshot,budget_snapshot,status,revision,idempotency_key,request_digest) VALUES('intention-v2','fl','default','goal-v2','act','act','result','["schedule.replan"]','{"type":"event","event_type":"life.event.created"}','0.9',now()+interval '1 day','["fact"]','{}','{}','qualified',1,'intention-v2',$1)`, digest); err != nil {
		t.Fatalf("valid Intention rejected: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_intention_attempts(attempt_id,fluctlight_id,intention_ref,goal_ref,action_id,outcome_id,outcome_digest,status,result,occurred_at) VALUES('attempt-v2','fl','intention:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','goal:ctx_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','action-v2','outcome-v2',$1,'suppressed','{}',now())`, digest); err != nil {
		t.Fatalf("valid Intention attempt rejected: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_evolution_states(fluctlight_id,profile_id,profile_ref,revision,domain_revisions) VALUES('fl','default','personality:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',0,'{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_evolution_overlays(id,ref,fluctlight_id,profile_id,kind,field_path,value_kind,semantic_direction,requested_delta,applied_delta,before_value,after_value,confidence,evidence_refs,evidence_windows,policy_version,base_revision,revision,status,cooldown_until,created_at) VALUES('overlay-v2','evolution_overlay:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','fl','default','personality','traits.openness','numeric','increase',0.2,0.1,'0.5','0.6',0.9,'["fact"]','["window-a","window-b"]','persona-evolution.policy.v1',0,1,'active',now()+interval '1 day',now())`); err != nil {
		t.Fatalf("valid overlay rejected: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_evolution_overlays(id,ref,fluctlight_id,profile_id,kind,field_path,value_kind,semantic_direction,before_value,after_value,confidence,evidence_refs,evidence_windows,policy_version,base_revision,revision,status,cooldown_until,created_at) VALUES('overlay-bad','bad-ref','fl','default','identity','identity.name','categorical','replace','"a"','"b"',2,'[]','[]','bad',1,1,'active',now(),now())`); err == nil {
		t.Fatal("invalid overlay authority was accepted")
	}
}
