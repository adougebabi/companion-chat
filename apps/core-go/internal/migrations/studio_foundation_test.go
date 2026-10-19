package migrations

import "testing"

func TestStudioFoundationUpgradeFromVisualStyleHeadInstallsExactSourceConstraints(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE public.fluctlight_foundation_revisions DROP COLUMN source_resource_id CASCADE,DROP COLUMN source_version CASCADE,DROP COLUMN source_choices;UPDATE public.alembic_version SET version_num='0080_studio_visual_style'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var constraints int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conname IN ('foundation_source_pair','foundation_source_version_fk')`).Scan(&constraints); err != nil || constraints != 2 {
		t.Fatal(constraints, err)
	}
}
