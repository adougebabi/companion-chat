package migrations

import "testing"

func TestStudioJobsPreviousHeadUpgradeAndRerun(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyReleasedMigrationFixture(t, ctx, pool, "0077_world_events")
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.studio_jobs') IS NOT NULL AND to_regclass('public.studio_job_attempts') IS NOT NULL AND to_regclass('public.studio_resource_materials') IS NOT NULL AND (SELECT version_num=$1 FROM public.alembic_version)`, Head).Scan(&exists); err != nil || !exists {
			t.Fatal(exists, err)
		}
	}
}
