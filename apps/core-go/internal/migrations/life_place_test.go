package migrations

import "testing"

func TestLifePlace0065UpgradePreservesTextAndExplicitIdentity(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	// Recreate the preceding schema shape. The fixture is a disposable database;
	// production migration only adds the nullable column and preserves history.
	if _, err := pool.Exec(ctx, `
INSERT INTO public.actors(id,actor_type,status) VALUES('place-owner','human','active'),('place-fl','fluctlight','active');
INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('place-fl','place-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');
INSERT INTO public.life_worlds(id,fluctlight_id,name) VALUES('place-world','place-fl','Test world');
INSERT INTO public.world_places(id,world_id,name) VALUES('stable-place','place-world','书屋');
ALTER TABLE public.life_schedule_items DROP COLUMN place_id;
INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,idempotency_key,request_digest,result) VALUES('old-schedule','place-fl','2026-10-16','Asia/Shanghai','proposed','owner','[]','old-schedule','0123456789abcdef0123456789abcdef','{"id":"old-schedule","revision":0,"status":"proposed","expected_context_revision":"fixture-before","resulting_context_revision":"fixture-after","replayed":false}');
INSERT INTO public.life_schedule_items(id,schedule_id,start_at,end_at,activity,scene,location,item_type,status,priority,flexibility,interruption_cost) VALUES('old-item','old-schedule','2026-10-16T01:00:00Z','2026-10-16T02:00:00Z','阅读','室内','书屋','activity','planned','normal','flexible','low');
UPDATE public.alembic_version SET version_num='0065_world_catalog';`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var good bool
	if err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version)
AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='life_schedule_items' AND column_name='place_id' AND is_nullable='YES')
AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.life_schedule_items'::regclass AND confrelid='public.world_places'::regclass AND contype='f')
AND EXISTS(SELECT 1 FROM public.life_schedule_items WHERE id='old-item' AND location='书屋' AND place_id IS NULL AND activity='阅读')
AND EXISTS(SELECT 1 FROM public.world_places WHERE id='stable-place' AND name='书屋')`, Head).Scan(&good); err != nil || !good {
		t.Fatal(good, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.life_schedule_items SET place_id='stable-place' WHERE id='old-item'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var place string
	if err := pool.QueryRow(ctx, `SELECT place_id FROM public.life_schedule_items WHERE id='old-item'`).Scan(&place); err != nil || place != "stable-place" {
		t.Fatal("explicit identity was lost on rerun", place, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.life_schedule_items SET place_id='missing-place' WHERE id='old-item'`); err == nil {
		t.Fatal("unknown place bypassed FK")
	}
}
