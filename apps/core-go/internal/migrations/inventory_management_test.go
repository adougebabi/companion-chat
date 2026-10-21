package migrations

import "testing"

func TestInventoryUpgradeRepairsLegacyEligibilityOnceAndRetainsUnknownWornOwnership(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
 -- Reconstruct the old worn relation before seeding the legacy double-slot
 -- corruption. The new 0096 identity correctly forbids it on a current schema.
 DROP INDEX public.wardrobe_single_body_slot;
 ALTER TABLE public.fluctlight_worn_items DROP CONSTRAINT wardrobe_worn_item_identity;
 ALTER TABLE public.fluctlight_worn_items ADD CONSTRAINT fluctlight_worn_items_pkey PRIMARY KEY(fluctlight_id,slot);
 INSERT INTO public.actors(id,actor_type,status)VALUES('inventory-migration-owner','human','active'),('inventory-migration-role','fluctlight','active');
 INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance)VALUES('inventory-migration-role','inventory-migration-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');
 INSERT INTO public.fluctlight_wardrobe_states(fluctlight_id,revision)VALUES('inventory-migration-role',10);
 INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key,item_kind)VALUES
 ('bad-use','inventory-migration-role','book','','legacy unavailable object','owned','unavailable','accepted_event','owner:inventory-migration-owner','bad-use','object'),
 ('good-use','inventory-migration-role','book','','known valid object','owned','available','accepted_event','owner:inventory-migration-owner','good-use','object'),
 ('unknown-worn','inventory-migration-role','coat','outer','known worn, ownership unknown','unknown','available','initialization','initialization:fixture','unknown-worn','wearable');
 INSERT INTO public.fluctlight_item_uses(fluctlight_id,item_id,activity,started_at,source_event_id)VALUES('inventory-migration-role','bad-use','reading',now(),'fixture'),('inventory-migration-role','good-use','reading',now(),'fixture');
 INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id)VALUES('inventory-migration-role','top','unknown-worn'),('inventory-migration-role','outer','unknown-worn');
 DELETE FROM public.fluctlight_inventory_repairs WHERE repair_key='0073_eligibility';
 UPDATE public.alembic_version SET version_num='0072_visual_brief';`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var repaired bool
	if err := pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.fluctlight_item_uses WHERE item_id='bad-use') AND EXISTS(SELECT 1 FROM public.fluctlight_item_uses WHERE item_id='good-use') AND (SELECT count(*)=1 FROM public.fluctlight_worn_items WHERE item_id='unknown-worn') AND EXISTS(SELECT 1 FROM public.fluctlight_worn_items WHERE item_id='unknown-worn' AND slot='outer') AND EXISTS(SELECT 1 FROM public.fluctlight_item_use_events WHERE item_id='bad-use' AND operation='stop' AND source_ref='migration:0073_inventory_management')`).Scan(&repaired); err != nil || !repaired {
		t.Fatal("upgrade repair lost valid state/history", repaired, err)
	}
	// This is a deliberately invalid post-upgrade fixture. Restart is not allowed
	// to silently reclassify it as an old migration correction.
	if _, err := pool.Exec(ctx, `INSERT INTO public.fluctlight_item_uses(fluctlight_id,item_id,activity,started_at,source_event_id)VALUES('inventory-migration-role','bad-use','new invalid fixture',now(),'post-migration')`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var retained bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_item_uses WHERE item_id='bad-use' AND source_event_id='post-migration') AND (SELECT count(*)=1 FROM public.fluctlight_inventory_repairs WHERE repair_key='0073_eligibility')`).Scan(&retained); err != nil || !retained {
		t.Fatal("restart reapplied data repair", retained, err)
	}
}
