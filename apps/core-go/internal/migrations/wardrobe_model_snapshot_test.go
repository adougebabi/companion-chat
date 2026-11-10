package migrations

import "testing"

func TestWardrobeModelSnapshotUpgradePreservesLegacyPreparation(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.actors(id,actor_type,status) VALUES('snapshot-owner','human','active'),('snapshot-character','fluctlight','active');
		INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance)
		VALUES('snapshot-character','snapshot-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');
		INSERT INTO public.fluctlight_wardrobe_preparations(id,fluctlight_id,owner_actor_id,profile_id,intent_id,status,result,expected_revision,source_ref)
		VALUES('snapshot-preparation','snapshot-character','snapshot-owner','default','snapshot-intent','awaiting_wear','{"inventory_committed":true,"item_count":2}',4,'old-source');
		ALTER TABLE public.fluctlight_wardrobe_preparations DROP COLUMN model_provenance;
		UPDATE public.alembic_version SET version_num='0107_expression_learning'`); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatalf("upgrade/rerun %d: %v", attempt, err)
		}
		var valid bool
		expectedProvenance := `{}`
		if attempt > 0 {
			expectedProvenance = `{"test_marker":"preserved"}`
		}
		err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.fluctlight_wardrobe_preparations WHERE id='snapshot-preparation' AND status='awaiting_wear' AND expected_revision=4 AND source_ref='old-source' AND result->>'inventory_committed'='true' AND result->>'item_count'='2' AND model_provenance=$2::jsonb)`, Head, expectedProvenance).Scan(&valid)
		if err != nil || !valid {
			t.Fatalf("legacy preparation changed on upgrade/rerun %d: valid=%t err=%v", attempt, valid, err)
		}
		if attempt == 0 {
			if _, err := pool.Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations SET model_provenance='{"test_marker":"preserved"}' WHERE id='snapshot-preparation'`); err != nil {
				t.Fatal(err)
			}
		}
	}
}
