package migrations

import "testing"

func TestPersonaModelSnapshotUpgradeDoesNotInventLegacyBinding(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status)
		VALUES('legacy-persona-intent','legacy-persona-workflow','lifecycle','persona.compile','{"fluctlight_id":"old-character"}','retry');
		DROP TABLE public.persona_preparation_model_snapshots;
		UPDATE public.alembic_version SET version_num='0110_visual_model_snapshot'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status)
		VALUES('new-persona-intent','new-persona-workflow','lifecycle','persona.compile','{"fluctlight_id":"new-character"}','pending');
		INSERT INTO public.persona_preparation_model_snapshots(intent_id,snapshot) VALUES('new-persona-intent','{"test_marker":"preserved"}')`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var valid bool
	err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_id='legacy-persona-intent' AND status='retry' AND payload->>'fluctlight_id'='old-character') AND NOT EXISTS(SELECT 1 FROM public.persona_preparation_model_snapshots WHERE intent_id='legacy-persona-intent') AND EXISTS(SELECT 1 FROM public.persona_preparation_model_snapshots WHERE intent_id='new-persona-intent' AND snapshot->>'test_marker'='preserved')`, Head).Scan(&valid)
	if err != nil || !valid {
		t.Fatalf("persona snapshot migration changed old intent or new snapshot: valid=%t err=%v", valid, err)
	}
}
