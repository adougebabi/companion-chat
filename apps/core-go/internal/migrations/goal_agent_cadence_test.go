package migrations

import "testing"

func TestGoalAgentCadenceHead(t *testing.T) {
	if Head != "0063_goal_agent_cadence" {
		t.Fatalf("Goal cadence has no additive migration: head=%s", Head)
	}
}

func TestPostgresGoalAgentCadenceFreshUpgradeAndRerun(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('cadence-owner','human','active'),('cadence-fl','fluctlight','active');
INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('cadence-fl','cadence-owner','blank_slate','active','{}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}');
INSERT INTO public.goal_agent_cadence(fluctlight_id,evaluation_next_at,evaluation_request_id) VALUES('cadence-fl','2100-01-01T00:00:00Z','preserved-cycle');
INSERT INTO public.goal_planner_admissions(fluctlight_id,episode_key,run_ref) VALUES('cadence-fl','daily:2100-01-01','preserved-run');
INSERT INTO public.goal_planning_events(fluctlight_id,source_key,reason,payload,processed_run_id) VALUES('cadence-fl','preserved-private','actor_context_changed','{"profile_id":"private","fact":"keep"}',NULL);
INSERT INTO public.goal_planning_runs(id,fluctlight_id,watermark,status,result,error_code) VALUES('preserved-failed','cadence-fl',0,'failed','{"original":true}','original-error');`); err != nil {
		t.Fatal(err)
	}
	for _, head := range []string{GoalAgentCadencePreviousHead, Head} {
		// Runner must replace the old full/review bypass even if an older installer
		// restored it after the ledger already reached head.
		if _, err := pool.Exec(ctx, goalPlannerSchemaSQL+goalPlannerCadenceSchemaSQL); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, head); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
		var valid bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.goal_agent_cadence') IS NOT NULL AND to_regclass('public.goal_planner_admissions') IS NOT NULL AND pg_get_functiondef('public.request_goal_planning(text,text,text,jsonb)'::regprocedure) LIKE '%goal_planner_episode_keys%'`).Scan(&valid); err != nil || !valid {
			t.Fatal(head, valid, err)
		}
		if err := pool.QueryRow(ctx, `SELECT
 (SELECT evaluation_next_at='2100-01-01T00:00:00Z' AND evaluation_request_id='preserved-cycle' FROM public.goal_agent_cadence WHERE fluctlight_id='cadence-fl')
 AND EXISTS(SELECT 1 FROM public.goal_planner_admissions WHERE fluctlight_id='cadence-fl' AND episode_key='daily:2100-01-01' AND run_ref='preserved-run')
 AND EXISTS(SELECT 1 FROM public.goal_planning_events WHERE fluctlight_id='cadence-fl' AND source_key='preserved-private' AND payload='{"profile_id":"private","fact":"keep"}'::jsonb AND processed_run_id IS NULL)
 AND EXISTS(SELECT 1 FROM public.goal_planning_runs WHERE id='preserved-failed' AND status='failed' AND result='{"original":true}'::jsonb AND error_code='original-error')
 AND pg_get_functiondef('public.goal_planner_episode_keys(text,text,bigint)'::regprocedure) LIKE '%schedule_accepted_daily%'`).Scan(&valid); err != nil || !valid {
			t.Fatal("rerun reset cadence/history or lost daily predicate", head, valid, err)
		}
	}
}
