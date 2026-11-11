package migrations

import "testing"

func TestGoalModelSnapshotUpgradePreservesLegacyRequest(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,reason,status,claim_revision,snapshot,result)
		VALUES('legacy-goal-snapshot','old-character','default','test','retry',3,'{"source":"preserved"}','{"submitted":true}');
		ALTER TABLE public.goal_evaluation_requests DROP COLUMN model_provenance, DROP COLUMN model_snapshot_required;
		UPDATE public.alembic_version SET version_num='0111_persona_model_snapshot'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,reason,status) VALUES('new-goal-snapshot','new-character','default','test','pending')`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var valid bool
	err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version) AND EXISTS(SELECT 1 FROM public.goal_evaluation_requests WHERE id='legacy-goal-snapshot' AND status='retry' AND claim_revision=3 AND snapshot->>'source'='preserved' AND result->>'submitted'='true' AND model_provenance='{}'::jsonb AND NOT model_snapshot_required) AND EXISTS(SELECT 1 FROM public.goal_evaluation_requests WHERE id='new-goal-snapshot' AND model_provenance->>'_goal_evaluation_model_snapshot_contract'='v1' AND model_snapshot_required)`, Head).Scan(&valid)
	if err != nil || !valid {
		t.Fatalf("goal model migration changed legacy or new requests: valid=%t err=%v", valid, err)
	}
}
