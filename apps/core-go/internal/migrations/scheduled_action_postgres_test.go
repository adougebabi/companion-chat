package migrations

import "testing"

func TestPostgresScheduledActionSchemaPreservesLegacyItemsAndReruns(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`ALTER TABLE public.life_schedule_items DROP CONSTRAINT ck_schedule_item_action_link`,
		`DROP INDEX public.ix_life_schedule_items_intention`,
		`ALTER TABLE public.life_schedule_items DROP COLUMN action_plan`,
		`ALTER TABLE public.life_schedule_items DROP COLUMN intention_id`,
		`ALTER TABLE public.fluctlight_life_activity_runs DROP CONSTRAINT fluctlight_life_activity_runs_kind_check`,
		`ALTER TABLE public.fluctlight_life_activity_runs ADD CONSTRAINT fluctlight_life_activity_runs_kind_check CHECK (kind IN ('virtual_shopping','haircut'))`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{
		`INSERT INTO public.life_schedule_items(id,schedule_id,start_at,end_at,activity,scene,item_type,status,priority,flexibility,interruption_cost) VALUES('scheduled-migration-item','scheduled-migration-schedule',now(),now()+interval '1 hour','阅读','书房','planned','planned','0.5','0.5','0.5')`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, ScheduledActionPreviousHead); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var head string
	var legacyCount, columns int
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.life_schedule_items WHERE id='scheduled-migration-item' AND intention_id IS NULL AND action_plan IS NULL`).Scan(&legacyCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='life_schedule_items' AND column_name IN ('intention_id','action_plan')`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if head != Head || legacyCount != 1 || columns != 2 {
		t.Fatalf("scheduled migration head=%q legacy=%d columns=%d", head, legacyCount, columns)
	}
}
