package migrations

import "testing"

func TestOwnerMediaUpgradePreservesRealGeneratedProducer(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('media-upgrade-owner','human','active'),('media-upgrade-fl','fluctlight','active');INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('media-upgrade-fl','media-upgrade-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES('media-before-upload','media-upgrade-fl','1','image','image/png',68,'checksum','private','original-key','actual-provider','actual-workflow','ready');DROP TABLE public.media_asset_annotations;ALTER TABLE public.media_assets DROP CONSTRAINT media_assets_scope_check,DROP COLUMN upload_owner_actor_id,DROP COLUMN owner_upload_id,DROP COLUMN revision;DROP TABLE public.owner_media_uploads;ALTER TABLE public.media_assets ALTER COLUMN provider_request_id SET NOT NULL;ALTER TABLE public.media_assets ALTER COLUMN workflow_id SET NOT NULL;UPDATE public.alembic_version SET version_num='0083_studio_outfits'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var valid bool
	if err := pool.QueryRow(ctx, `SELECT media_scope='fluctlight' AND provider_request_id='actual-provider' AND workflow_id='actual-workflow' AND object_key='original-key' AND upload_owner_actor_id IS NULL AND owner_upload_id IS NULL AND revision=1 FROM public.media_assets WHERE id='media-before-upload'`).Scan(&valid); err != nil || !valid {
		t.Fatal("upgrade rewrote old producer", valid, err)
	}
}
