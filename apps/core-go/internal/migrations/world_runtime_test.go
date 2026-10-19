package migrations

import "testing"

func TestWorldRuntimeFreshPriorHeadUpgradeAndRerun(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyReleasedMigrationFixture(t, ctx, pool, "0063_goal_agent_cadence")
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('world-upgrade-owner','human','active'),('world-upgrade-fl','fluctlight','active');
INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('world-upgrade-fl','world-upgrade-owner','blank_slate','active','{}','{"name":"Preserved"}','{}','{}','{}','{}');
INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,status,evidence_refs,idempotency_key,request_digest,result) VALUES('world-upgrade-event','world-upgrade-fl','historical','2026-10-01T00:00:00Z','2026-10-01T01:00:00Z','confirmed','["preserved-source"]','world-upgrade-key','00000000000000000000000000000000','{"id":"world-upgrade-event","revision":1,"status":"confirmed","expected_context_revision":"prior-context","resulting_context_revision":"preserved-context","replayed":false}');
`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
		var good bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_events WHERE id='world-upgrade-event' AND request_digest='00000000000000000000000000000000') AND EXISTS(SELECT 1 FROM public.fluctlights WHERE id='world-upgrade-fl' AND identity->>'name'='Preserved') AND to_regclass('public.world_npcs') IS NOT NULL AND (SELECT version_num=$1 FROM public.alembic_version)`, Head).Scan(&good); err != nil || !good {
			t.Fatal("upgrade changed historical facts", good, err)
		}
	}
}
