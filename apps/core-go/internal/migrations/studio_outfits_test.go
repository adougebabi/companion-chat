package migrations

import "testing"

func TestStudioOutfitUpgradePreservesExistingConcreteCombination(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('outfit-owner','human','active'),('outfit-fl','fluctlight','active');INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('outfit-fl','outfit-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');INSERT INTO public.fluctlight_wardrobe_states(fluctlight_id,revision) VALUES('outfit-fl',9);INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,item_kind,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key) VALUES('outfit-shirt','outfit-fl','wearable','top','top','Original held shirt','owned','available','initialization','original-source','primary');INSERT INTO public.fluctlight_wardrobe_outfits(id,fluctlight_id,profile_id,name,revision) VALUES('legacy-outfit','outfit-fl','default','Original combination',3);INSERT INTO public.fluctlight_wardrobe_outfit_items(fluctlight_id,outfit_id,slot,item_id) VALUES('outfit-fl','legacy-outfit','top','outfit-shirt');ALTER TABLE public.fluctlight_wardrobe_outfits DROP COLUMN source_resource_id CASCADE,DROP COLUMN source_version CASCADE,DROP COLUMN source_overrides;UPDATE public.alembic_version SET version_num='0082_world_items'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var intact bool
	if err := pool.QueryRow(ctx, `SELECT name='Original combination' AND revision=3 AND source_resource_id IS NULL AND source_version IS NULL AND source_overrides='{}'::jsonb AND (SELECT item_id='outfit-shirt' FROM public.fluctlight_wardrobe_outfit_items WHERE outfit_id='legacy-outfit') AND (SELECT revision=9 FROM public.fluctlight_wardrobe_states WHERE fluctlight_id='outfit-fl') FROM public.fluctlight_wardrobe_outfits WHERE id='legacy-outfit'`).Scan(&intact); err != nil || !intact {
		t.Fatal("upgrade rewrote concrete combination or invented template source", intact, err)
	}
}
