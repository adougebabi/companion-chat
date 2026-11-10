package migrations

import "testing"

func TestExpressionLearningMigrationPreservesPreviousHead(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('expression-migration-owner','human','active'),('expression-migration-character','fluctlight','active');
		INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance)
		VALUES('expression-migration-character','expression-migration-owner','blank_slate','active','{"identity":{"name":"保留"}}','{}','{}','{}','{}','{}');
		DROP TRIGGER expression_learning_source_version ON public.conversation_messages;
		DROP FUNCTION public.bump_expression_learning_source_version();
		DROP TABLE public.expression_learning_reviews,public.expression_learning_candidates,public.expression_learning_extractions,public.expression_learning_settings,public.expression_learning_source_versions;
		UPDATE public.alembic_version SET version_num='0106_summary_correction_guard'`); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatalf("upgrade/rerun %d: %v", attempt, err)
		}
		var head, name string
		var settings, candidates int
		if err := pool.QueryRow(ctx, `SELECT (SELECT version_num FROM public.alembic_version),
			(SELECT core_persona->'identity'->>'name' FROM public.fluctlights WHERE id='expression-migration-character'),
			(SELECT count(*) FROM public.expression_learning_settings),
			(SELECT count(*) FROM public.expression_learning_candidates)`).Scan(&head, &name, &settings, &candidates); err != nil || head != Head || name != "保留" || settings != 0 || candidates != 0 {
			t.Fatalf("head=%s name=%s settings=%d candidates=%d err=%v", head, name, settings, candidates, err)
		}
	}
}
