package migrations

import "testing"

func TestStudioMediaUpgradePreservesFormalOwnershipAndAllowsIsolatedScope(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	// This is a schema fixture, not evidence of object or model generation.
	if _, err := pool.Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES('studio-upgrade-legacy','legacy-fl','v1','image','image/png',1,'fixture-digest','private','fixture-key','fixture-provider','fixture-workflow','ready');UPDATE public.alembic_version SET version_num='0078_studio_jobs'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
		var valid bool
		if err := pool.QueryRow(ctx, `SELECT owner_fluctlight_id='legacy-fl' AND media_scope='fluctlight' AND studio_owner_actor_id IS NULL AND studio_job_id IS NULL AND object_key='fixture-key' AND (SELECT version_num=$1 FROM public.alembic_version) FROM public.media_assets WHERE id='studio-upgrade-legacy'`, Head).Scan(&valid); err != nil || !valid {
			t.Fatal(valid, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,media_scope,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES('studio-invalid-mixed','legacy-fl','studio','v1','image','image/png',1,'fixture-digest','private','fixture-key','fixture-provider','fixture-workflow','ready')`); err == nil {
		t.Fatal("mixed or incomplete Studio ownership accepted")
	}
}
