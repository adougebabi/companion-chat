package migrations

import "testing"

func TestWardrobeWorld0095UpgradePreservesOldDataAndReruns(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	// Reconstruct the exact predecessor cardinality and columns in this disposable
	// database, with old IDs/source/history present before the upgrade.
	if _, err := pool.Exec(ctx, `
 DROP TABLE public.fluctlight_wardrobe_preparations;
 DROP INDEX public.wardrobe_single_body_slot;
 DROP INDEX public.wardrobe_outfit_single_body_slot;
 ALTER TABLE public.fluctlight_worn_items DROP CONSTRAINT wardrobe_worn_item_identity, ADD PRIMARY KEY(fluctlight_id,slot);
 ALTER TABLE public.fluctlight_wardrobe_outfit_items DROP CONSTRAINT wardrobe_outfit_item_identity, ADD PRIMARY KEY(fluctlight_id,outfit_id,slot);
 ALTER TABLE public.fluctlight_wardrobe_items DROP COLUMN clothing_type,DROP COLUMN seasons;
 ALTER TABLE public.fluctlight_wardrobe_outfits DROP COLUMN tags;
 ALTER TABLE public.fluctlight_wardrobe_states DROP COLUMN outfit_tags;
 ALTER TABLE public.life_worlds DROP COLUMN venue_categories;
 ALTER TABLE public.world_places DROP COLUMN framework_key;
 INSERT INTO public.actors(id,actor_type,status) VALUES('wardrobe-owner','human','active'),('wardrobe-fl','fluctlight','active');
 INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('wardrobe-fl','wardrobe-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');
 INSERT INTO public.fluctlight_wardrobe_states(fluctlight_id) VALUES('wardrobe-fl');
 INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key) VALUES('old-shirt','wardrobe-fl','shirt','top','旧衬衫','owned','available','initialization','old-source','old-member'),('old-accessory','wardrobe-fl','accessory','accessory','旧项链','owned','available','initialization','old-source','old-accessory');
 INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id) VALUES('wardrobe-fl','top','old-shirt'),('wardrobe-fl','accessory','old-accessory');
 INSERT INTO public.fluctlight_wardrobe_outfits(id,fluctlight_id,profile_id,name) VALUES('old-plan','wardrobe-fl','default','旧方案');
 INSERT INTO public.fluctlight_wardrobe_outfit_items(fluctlight_id,outfit_id,slot,item_id) VALUES('wardrobe-fl','old-plan','accessory','old-accessory');
 INSERT INTO public.life_worlds(id,fluctlight_id,name) VALUES('old-world','wardrobe-fl','旧世界');
 INSERT INTO public.world_places(id,world_id,name) VALUES('old-place','old-world','旧地点');
 UPDATE public.alembic_version SET version_num='0095_place_defaults';`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var good bool
	if err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.fluctlight_worn_items WHERE fluctlight_id='wardrobe-fl' AND item_id='old-shirt' AND slot='top') AND EXISTS(SELECT 1 FROM public.fluctlight_wardrobe_items WHERE id='old-shirt' AND source_ref='old-source' AND cardinality(seasons)=0 AND clothing_type IS NULL) AND EXISTS(SELECT 1 FROM public.fluctlight_wardrobe_outfits WHERE id='old-plan' AND name='旧方案' AND cardinality(tags)=0) AND EXISTS(SELECT 1 FROM public.world_places WHERE id='old-place' AND name='旧地点' AND framework_key IS NULL)`, Head).Scan(&good); err != nil || !good {
		t.Fatal(good, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key) VALUES('new-accessory','wardrobe-fl','accessory','accessory','新增手链','owned','available','initialization','new-source','new-member'); INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id) VALUES('wardrobe-fl','accessory','new-accessory'); INSERT INTO public.fluctlight_wardrobe_outfit_items(fluctlight_id,outfit_id,slot,item_id) VALUES('wardrobe-fl','old-plan','accessory','new-accessory');`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id) VALUES('wardrobe-fl','top','new-accessory')`); err == nil {
		t.Fatal("ordinary slot uniqueness lost")
	}
}
