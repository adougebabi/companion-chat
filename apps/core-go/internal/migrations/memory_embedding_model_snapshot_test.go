package migrations

import "testing"

func TestEmbeddingModelSnapshotUpgradePreservesLegacyIntent(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status)
		VALUES('legacy-embedding-snapshot','legacy-embedding-workflow','lifecycle','memory.embedding','{"memory_id":"legacy-memory","revision":0,"provider_endpoint_id":"old-endpoint","model_id":"old-model"}','retry');
		DROP TABLE public.memory_embedding_model_snapshots;
		ALTER TABLE public.platform_workflow_intents DROP COLUMN embedding_snapshot_contract;
		UPDATE public.alembic_version SET version_num='0114_segment_model_snapshot'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,embedding_snapshot_contract) VALUES('new-embedding-snapshot','new-embedding-workflow','lifecycle','memory.embedding','{"memory_id":"new-memory","revision":0}','pending','v1'); INSERT INTO public.memory_embedding_model_snapshots(intent_id,snapshot,selected) VALUES('new-embedding-snapshot','{"version":"v1"}',true)`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var valid bool
	err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_id='legacy-embedding-snapshot' AND status='retry' AND payload->>'provider_endpoint_id'='old-endpoint' AND payload->>'model_id'='old-model' AND embedding_snapshot_contract='') AND NOT EXISTS(SELECT 1 FROM public.memory_embedding_model_snapshots WHERE intent_id='legacy-embedding-snapshot') AND EXISTS(SELECT 1 FROM public.platform_workflow_intents w JOIN public.memory_embedding_model_snapshots s USING(intent_id) WHERE w.intent_id='new-embedding-snapshot' AND w.embedding_snapshot_contract='v1' AND s.selected AND s.snapshot->>'version'='v1')`, Head).Scan(&valid)
	if err != nil || !valid {
		t.Fatalf("embedding model migration changed legacy or stored data: valid=%t err=%v", valid, err)
	}
}
