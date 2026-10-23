package migrations

import "testing"

func Test0066UpgradeAndRestartPreserveExhaustedBudgetAndChosenQueue(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
 INSERT INTO public.actors(id,actor_type,status) VALUES('delivery-policy-owner','human','active'),('delivery-policy-fl','fluctlight','active');
 INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('delivery-policy-fl','delivery-policy-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');
 INSERT INTO public.autonomy_policies(fluctlight_id,mode,allowed_actions,budget_remaining,quiet_hours,concurrency_limit,revision) VALUES('delivery-policy-fl','paused','["moment"]','0','{}',4,7);
 INSERT INTO public.runtime_settings(key,value_json) VALUES('product.autonomy','{"mode":"paused","budget_remaining":0,"allowed_actions":["moment"]}') ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json;
 INSERT INTO public.runtime_settings(key,value_json) VALUES('llm.queue','{"generated_concurrency":2,"embedding_concurrency":1}') ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json;
 ALTER TABLE public.autonomy_policies DROP COLUMN policy_source,DROP COLUMN outreach_paused,DROP COLUMN notifications_muted,DROP COLUMN confirm_action_types,DROP COLUMN cooldown_seconds;
 DROP TABLE public.autonomy_policy_history;
 DROP TABLE public.owner_notifications;
 DROP TABLE public.fluctlight_diary_annotations;
 DROP TABLE public.fluctlight_diary_attempts;
 DROP TABLE public.fluctlight_diary_days;
 UPDATE public.alembic_version SET version_num='0066_life_place';`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var preserved bool
	if err := pool.QueryRow(ctx, `SELECT (SELECT version_num=$1 FROM public.alembic_version)
 AND EXISTS(SELECT 1 FROM public.autonomy_policies WHERE fluctlight_id='delivery-policy-fl' AND budget_remaining='0' AND mode='paused' AND concurrency_limit=4 AND revision=7 AND policy_source='override' AND NOT notifications_muted)
 AND EXISTS(SELECT 1 FROM public.runtime_settings WHERE key='product.autonomy' AND (value_json::jsonb->>'budget_remaining')::int=0)
 AND EXISTS(SELECT 1 FROM public.runtime_settings WHERE key='llm.queue' AND (value_json::jsonb->>'generated_concurrency')::int=1)
 AND to_regclass('public.owner_notifications') IS NOT NULL
 AND to_regclass('public.fluctlight_diary_days') IS NOT NULL`, Head).Scan(&preserved); err != nil || !preserved {
		t.Fatalf("upgrade/restart rewrote live configuration %v %v", preserved, err)
	}
}
