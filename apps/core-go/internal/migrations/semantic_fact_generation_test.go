package migrations

import "testing"

func TestSemanticFactGenerationUpgradeAndRerun(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	// Recreate the released head's trigger to exercise an actual 0057 upgrade.
	if _, err := pool.Exec(ctx, `CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.memory_embeddings FOR EACH ROW EXECUTE FUNCTION public.bump_context_memory_child_trigger()`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, SemanticFactGenerationPreviousHead); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"upgrade", "head rerun"} {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatalf("%s: %v", phase, err)
		}
		var head string
		if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != Head {
			t.Fatal(phase, head, err)
		}
		var embeddings, sources bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.memory_embeddings'::regclass AND tgname='context_generation_bump'),EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.memory_source_links'::regclass AND tgname='context_generation_bump')`).Scan(&embeddings, &sources); err != nil || embeddings || !sources {
			t.Fatal("cache/provenance trigger boundary invalid", phase, embeddings, sources, err)
		}
	}
}
