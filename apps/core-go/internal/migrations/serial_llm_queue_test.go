package migrations

import "testing"

func TestSerialLLMQueueUpgradeOncePreservesLaterSettings(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.runtime_settings SET value_json='{"generated_concurrency":4,"embedding_concurrency":3,"keep":"value"}',revision=7 WHERE key='llm.queue'; UPDATE public.alembic_version SET version_num='0100_kev_timeout'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var good bool
	if err := pool.QueryRow(ctx, `SELECT revision=8 AND value_json::jsonb->>'generated_concurrency'='1' AND value_json::jsonb->>'embedding_concurrency'='1' AND value_json::jsonb->>'keep'='value' FROM public.runtime_settings WHERE key='llm.queue'`).Scan(&good); err != nil || !good {
		t.Fatal(good, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.runtime_settings SET value_json='{"generated_concurrency":2,"embedding_concurrency":1}',revision=9 WHERE key='llm.queue'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT revision=9 AND value_json::jsonb->>'generated_concurrency'='2' FROM public.runtime_settings WHERE key='llm.queue'`).Scan(&good); err != nil || !good {
		t.Fatal("rerun overwrote later owner setting", good, err)
	}
}
