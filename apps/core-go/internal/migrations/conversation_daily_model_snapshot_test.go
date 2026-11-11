package migrations

import "testing"

func TestDailyModelSnapshotUpgradePreservesLegacyIntent(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status)
		VALUES('legacy-daily-snapshot','legacy-daily-workflow','lifecycle','conversation.daily_memory','{"source_generation":2,"source":"preserved"}','retry');
		DROP TABLE public.conversation_daily_model_snapshots;
		UPDATE public.alembic_version SET version_num='0112_goal_model_snapshot'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.conversation_daily_model_snapshots(intent_id,source_generation,snapshot,selected) VALUES('legacy-daily-snapshot',3,'{"version":"v1"}',true)`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var valid bool
	err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_id='legacy-daily-snapshot' AND status='retry' AND payload->>'source_generation'='2' AND payload->>'source'='preserved' AND NOT (payload ? '_daily_memory_model_snapshot_contract')) AND NOT EXISTS(SELECT 1 FROM public.conversation_daily_model_snapshots WHERE intent_id='legacy-daily-snapshot' AND source_generation=2) AND EXISTS(SELECT 1 FROM public.conversation_daily_model_snapshots WHERE intent_id='legacy-daily-snapshot' AND source_generation=3 AND snapshot->>'version'='v1' AND selected)`, Head).Scan(&valid)
	if err != nil || !valid {
		t.Fatalf("daily model migration changed legacy or stored data: valid=%t err=%v", valid, err)
	}
}
