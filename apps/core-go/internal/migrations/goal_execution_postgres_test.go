package migrations

import (
	"strings"
	"testing"
)

func TestPostgresGoalExecutionUpgradePreservesAttemptsAndReruns(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 32)
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_intention_attempts(attempt_id,fluctlight_id,intention_ref,goal_ref,action_id,outcome_id,outcome_digest,status,result,occurred_at) VALUES('historic-attempt','historic-fl','intention:historic','goal:historic','historic-action','historic-outcome',$1,'succeeded','{"historical":true}','2026-01-01T00:00:00Z')`, digest); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE public.fluctlight_intention_attempts DROP COLUMN settled_at,DROP COLUMN started_at,DROP COLUMN intention_id,DROP COLUMN wait_ref,DROP COLUMN deadline; UPDATE public.alembic_version SET version_num='0049_actor_user_background'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var head, status, storedDigest string
	var retained bool
	if err := pool.QueryRow(ctx, `SELECT status,outcome_digest,result->>'historical'='true' AND settled_at=occurred_at FROM public.fluctlight_intention_attempts WHERE attempt_id='historic-attempt'`).Scan(&status, &storedDigest, &retained); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil {
		t.Fatal(err)
	}
	if head != Head || status != "succeeded" || storedDigest != digest || !retained {
		t.Fatalf("history changed: %s %s %s %v", head, status, storedDigest, retained)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_intention_attempts(attempt_id,fluctlight_id,intention_ref,goal_ref,action_id,outcome_id,outcome_digest,status,result,occurred_at) VALUES('running-attempt','new-fl','intention:new','goal:new','new-action','new-outcome','','running','{}',now())`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var live string
	if err := pool.QueryRow(ctx, `SELECT status FROM public.fluctlight_intention_attempts WHERE attempt_id='running-attempt'`).Scan(&live); err != nil || live != "running" {
		t.Fatalf("rerun falsely settled attempt: %s %v", live, err)
	}
}

func TestPostgresGoalExecutionMalformedAttemptRollsBackLedger(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE public.fluctlight_intention_attempts DROP CONSTRAINT ck_intention_attempt_authority_v2; INSERT INTO public.fluctlight_intention_attempts(attempt_id,fluctlight_id,intention_ref,goal_ref,action_id,outcome_id,outcome_digest,status,result,occurred_at) VALUES('bad-attempt','bad-fl','intention:bad','goal:bad','bad-action','bad-outcome','','succeeded','{}',now()); UPDATE public.alembic_version SET version_num='0049_actor_user_background'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err == nil {
		t.Fatal("invalid terminal evidence accepted")
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != GoalExecutionPreviousHead {
		t.Fatalf("failed migration advanced ledger: %s %v", head, err)
	}
}
