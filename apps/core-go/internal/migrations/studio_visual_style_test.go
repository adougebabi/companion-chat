package migrations

import "testing"

func TestStudioVisualStyleMigrationPreservesManualBriefAndConstrainsSource(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('style-owner','human','active'),('style-fl','fluctlight','active');INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('style-fl','style-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');INSERT INTO public.fluctlight_visual_briefs(fluctlight_id,revision,appearance,clothing,constraints,"references",updated_by_actor_id) VALUES('style-fl',1,'manual appearance','manual clothing','constraints','references','style-owner');ALTER TABLE public.fluctlight_visual_briefs DROP COLUMN source_resource_id CASCADE,DROP COLUMN source_version CASCADE,DROP COLUMN source_overrides;UPDATE public.alembic_version SET version_num='0079_studio_media'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
		var valid bool
		if err := pool.QueryRow(ctx, `SELECT revision=1 AND appearance='manual appearance' AND clothing='manual clothing' AND source_resource_id IS NULL AND source_version IS NULL AND source_overrides='{}'::jsonb AND (SELECT version_num=$1 FROM public.alembic_version) FROM public.fluctlight_visual_briefs WHERE fluctlight_id='style-fl'`, Head).Scan(&valid); err != nil || !valid {
			t.Fatal("manual data changed or source invented", valid, err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE public.fluctlight_visual_briefs SET source_resource_id='unknown' WHERE fluctlight_id='style-fl'`); err == nil {
		t.Fatal("half source pair accepted")
	}
	if _, err := pool.Exec(ctx, `UPDATE public.fluctlight_visual_briefs SET source_resource_id='unknown',source_version='1' WHERE fluctlight_id='style-fl'`); err == nil {
		t.Fatal("nonexistent published source accepted")
	}
}
