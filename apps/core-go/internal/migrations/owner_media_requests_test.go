package migrations

import "testing"

func TestOwnerMediaRequestUpgradePreservesExistingMediaTargets(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES('owner-media-legacy','legacy-role','image','image/png','{}','actual-request','actual-workflow','pending');ALTER TABLE public.media_intents DROP COLUMN owner_request_id CASCADE,DROP COLUMN owner_item_id;UPDATE public.alembic_version SET version_num='0085_media_reference_grants'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var valid bool
	if err := pool.QueryRow(ctx, `SELECT owner_request_id IS NULL AND owner_item_id IS NULL AND provider_request_id='actual-request' AND workflow_id='actual-workflow' FROM public.media_intents WHERE id='owner-media-legacy'`).Scan(&valid); err != nil || !valid {
		t.Fatal("upgrade reassigned an old media target", valid, err)
	}
}
