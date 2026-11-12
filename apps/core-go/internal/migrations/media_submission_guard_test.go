package migrations

import "testing"

func TestMediaSubmissionGuardUpgradePreservesLegacyAndMarksNew(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,provider_job_id,workflow_id,status)
		VALUES('legacy-ambiguous','old-owner','image','image/png','{}','old-request',NULL,'old-workflow','pending'),
		('legacy-known-job','old-owner','image','image/png','{}','known-request','known-job','known-workflow','running');
		ALTER TABLE public.media_intents DROP COLUMN renderer_submission_started;
		UPDATE public.alembic_version SET version_num='0116_runtime_summary_snapshot'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES('new-submit','new-owner','image','image/png','{}','new-request','new-workflow','pending')`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var valid bool
	err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.media_intents WHERE id='legacy-ambiguous' AND status='pending' AND provider_job_id IS NULL AND renderer_submission_started) AND EXISTS(SELECT 1 FROM public.media_intents WHERE id='legacy-known-job' AND status='running' AND provider_job_id='known-job' AND renderer_submission_started) AND EXISTS(SELECT 1 FROM public.media_intents WHERE id='new-submit' AND status='pending' AND NOT renderer_submission_started)`, Head).Scan(&valid)
	if err != nil || !valid {
		t.Fatalf("migration changed legacy state or misidentified new submissions: valid=%t err=%v", valid, err)
	}
}
