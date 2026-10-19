package migrations

import "testing"

func TestWorldCatalog0064UpgradePreservesIdentityAndBinding(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('catalog-owner','human','active'),('catalog-fl','fluctlight','active');
INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('catalog-fl','catalog-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');
DROP TABLE public.fluctlight_world_bindings;
ALTER TABLE public.life_worlds ADD CONSTRAINT life_worlds_fluctlight_id_key UNIQUE(fluctlight_id);
INSERT INTO public.life_worlds(id,fluctlight_id,name,background) VALUES('catalog-old-world','catalog-fl','Same name','Preserved source');
INSERT INTO public.world_places(id,world_id,name) VALUES('catalog-old-place','catalog-old-world','Same name');
UPDATE public.alembic_version SET version_num='0064_world_runtime';`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.life_worlds(id,fluctlight_id,name) VALUES('catalog-new-world','catalog-fl','Same name')`); err != nil {
		t.Fatal("old per-character UNIQUE survived", err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var good bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_world_bindings WHERE fluctlight_id='catalog-fl' AND world_id='catalog-old-world' AND revision=1) AND EXISTS(SELECT 1 FROM public.life_worlds WHERE id='catalog-old-world' AND background='Preserved source') AND EXISTS(SELECT 1 FROM public.world_places WHERE id='catalog-old-place' AND world_id='catalog-old-world') AND (SELECT count(*)=2 FROM public.life_worlds WHERE fluctlight_id='catalog-fl')`).Scan(&good); err != nil || !good {
		t.Fatal(good, err)
	}
}
