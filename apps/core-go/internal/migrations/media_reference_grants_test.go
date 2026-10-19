package migrations

import "testing"

func TestMediaReferenceUpgradeDoesNotInventLegacyModelPermission(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES('reference-legacy','legacy-role','v1','image','image/png',68,'exact-old-digest','private','legacy-key','actual-provider','actual-workflow','ready');DROP TABLE public.media_reference_grant_history;DROP TABLE public.media_reference_grants;UPDATE public.alembic_version SET version_num='0084_owner_media'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var valid bool
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*)=0 FROM public.media_reference_grants) AND (SELECT sha256='exact-old-digest' AND version='v1' AND provider_request_id='actual-provider' AND workflow_id='actual-workflow' FROM public.media_assets WHERE id='reference-legacy')`).Scan(&valid); err != nil || !valid {
		t.Fatal("upgrade invented permission or changed content provenance", valid, err)
	}
}
