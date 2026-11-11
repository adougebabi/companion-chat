package migrations

import "testing"

func TestRuntimeSummarySnapshotUpgradePreservesLegacyIntent(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES('legacy-runtime-summary','legacy-runtime-summary-workflow','lifecycle','conversation.summary','{"source":"preserved"}','retry'); DROP TABLE public.runtime_summary_model_snapshots; UPDATE public.alembic_version SET version_num='0115_embedding_model_snapshot'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var valid bool
	err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_id='legacy-runtime-summary' AND status='retry' AND payload->>'source'='preserved') AND NOT EXISTS(SELECT 1 FROM public.runtime_summary_model_snapshots WHERE intent_id='legacy-runtime-summary')`, Head).Scan(&valid)
	if err != nil || !valid {
		t.Fatalf("runtime summary migration changed legacy data: valid=%t err=%v", valid, err)
	}
}
