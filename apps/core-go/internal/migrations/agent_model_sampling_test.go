package migrations

import "testing"

func TestAgentSamplingUpgradePreservesBindingsAndLaterSettings(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose) VALUES('sampling-migration-endpoint','openai_compatible','http://fixture.invalid','sampling-migration-secret');
INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES('agent:reflection','sampling-migration-endpoint','old-model',37);
ALTER TABLE public.model_roles DROP COLUMN temperature, DROP COLUMN top_p;
UPDATE public.alembic_version SET version_num='0104_kev_one_minute'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var preserved bool
	if err := pool.QueryRow(ctx, `SELECT model_id='old-model' AND timeout_seconds=37 AND temperature IS NULL AND top_p IS NULL FROM public.model_roles WHERE role='agent:reflection'`).Scan(&preserved); err != nil || !preserved {
		t.Fatalf("old binding changed: %v %v", preserved, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.model_roles SET temperature=0,top_p=0.7 WHERE role='agent:reflection'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT temperature=0 AND top_p=0.7 FROM public.model_roles WHERE role='agent:reflection'`).Scan(&preserved); err != nil || !preserved {
		t.Fatalf("rerun changed sampling: %v %v", preserved, err)
	}
}
