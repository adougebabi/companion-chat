package migrations

import "testing"

func TestPostgresGoalPlannerCadenceUpgradeFrom0054AndRerun(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	// Restore the released SQL function as well as its head marker. Changing only
	// the version row would test a rerun of 0055, not a real 0054 -> 0055 upgrade.
	if _, err := pool.Exec(ctx, goalPlannerSchemaSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, GoalPlannerCadencePreviousHead); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil {
		t.Fatal(err)
	}
	if head != Head {
		t.Fatalf("cadence migration head=%q want=%q", head, Head)
	}
	var functionPresent bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_proc WHERE pronamespace='public'::regnamespace AND proname='request_goal_planning' AND pg_get_functiondef(oid) LIKE '%schedule_accepted_daily%')`).Scan(&functionPresent); err != nil || !functionPresent {
		t.Fatalf("cadence function present=%v err=%v", functionPresent, err)
	}
}
