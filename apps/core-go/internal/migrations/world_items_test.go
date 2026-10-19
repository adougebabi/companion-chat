package migrations

import "testing"

func TestWorldItemsUpgradePreservesLegacyHeldItemsWithoutInventingDefinitionSource(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('catalog-owner','human','active'),('catalog-fl','fluctlight','active');INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('catalog-fl','catalog-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');INSERT INTO public.fluctlight_wardrobe_states(fluctlight_id,revision) VALUES('catalog-fl',7);INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,item_kind,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key) VALUES('legacy-book','catalog-fl','object','book','','Original recorded book','owned','available','initialization','original-owner-record','primary');ALTER TABLE public.fluctlight_wardrobe_items DROP COLUMN source_resource_id CASCADE,DROP COLUMN source_version CASCADE,DROP COLUMN source_world_item_id CASCADE,DROP COLUMN source_world_item_revision;DROP TABLE public.world_item_revisions,public.world_items;UPDATE public.alembic_version SET version_num='0081_studio_foundation'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var intact bool
	if err := pool.QueryRow(ctx, `SELECT description='Original recorded book' AND ownership='owned' AND source_kind='initialization' AND source_resource_id IS NULL AND source_version IS NULL AND source_world_item_id IS NULL AND source_world_item_revision IS NULL AND (SELECT revision=7 FROM public.fluctlight_wardrobe_states WHERE fluctlight_id='catalog-fl') FROM public.fluctlight_wardrobe_items WHERE id='legacy-book'`).Scan(&intact); err != nil || !intact {
		t.Fatal("legacy item/source or wardrobe revision changed", intact, err)
	}
}
