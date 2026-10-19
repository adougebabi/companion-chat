package migrations

import "testing"

func TestWorldEventsPreviousHeadUpgradePreservesEventReplayAndAuthority(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyReleasedMigrationFixture(t, ctx, pool, "0076_world_encounter_runtime")
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('event-upgrade-owner','human','active'),('event-upgrade-fl','fluctlight','active');
INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('event-upgrade-fl','event-upgrade-owner','blank_slate','active','{}','{"name":"Preserved"}','{}','{}','{}','{}');
INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,status,evidence_refs,idempotency_key,request_digest,result) VALUES('event-upgrade-record','event-upgrade-fl','historical','2026-10-01T00:00:00Z','2026-10-01T01:00:00Z','confirmed','["preserved-source"]','event-upgrade-key','00000000000000000000000000000000','{"id":"event-upgrade-record","source":"original","revision":1,"status":"confirmed","expected_context_revision":"prior-context","resulting_context_revision":"preserved-context","replayed":false}');
`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
		var valid bool
		if err := pool.QueryRow(ctx, `SELECT e.context_scope='life' AND e.request_digest='00000000000000000000000000000000' AND e.result->>'source'='original' AND (SELECT version_num=$1 FROM public.alembic_version) FROM public.life_events e WHERE id='event-upgrade-record'`, Head).Scan(&valid); err != nil || !valid {
			t.Fatal("upgrade rewrote historical event", valid, err)
		}
	}
}
