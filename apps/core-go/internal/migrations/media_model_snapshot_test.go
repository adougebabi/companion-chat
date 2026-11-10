package migrations

import "testing"

func TestMediaModelSnapshotUpgradeKeepsLegacyAndMarksNewIntents(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,provider_job_id,workflow_id,status,quality_retry_count)
		VALUES('legacy-media-snapshot','old-owner','image','image/png','{"scene":"old"}','old-request','old-job','old-workflow','running',1);
		ALTER TABLE public.media_intents DROP COLUMN model_provenance;
		UPDATE public.alembic_version SET version_num='0108_wardrobe_model_snapshot'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES('new-media-snapshot','new-owner','image','image/png','{}','new-request','new-workflow','pending')`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var valid bool
	err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.media_intents WHERE id='legacy-media-snapshot' AND status='running' AND provider_job_id='old-job' AND quality_retry_count=1 AND prompt='{"scene":"old"}' AND model_provenance='{}'::jsonb) AND EXISTS(SELECT 1 FROM public.media_intents WHERE id='new-media-snapshot' AND model_provenance->>'_media_model_snapshot_contract'='v1')`, Head).Scan(&valid)
	if err != nil || !valid {
		t.Fatalf("media snapshot migration changed old or new intent semantics: valid=%t err=%v", valid, err)
	}
}
