package migrations

import "testing"

func TestMediaPublicationTimeUpgradePreservesUnknownAndKnown(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE public.media_references DROP COLUMN published_at;
 INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id,created_at) VALUES('old','asset','role','moment','moment','2020-01-01');
 UPDATE public.alembic_version SET version_num='0118_claim_evidence_versions'`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var unknown bool
	var head string
	if err := pool.QueryRow(ctx, `SELECT published_at IS NULL AND created_at='2020-01-01'::timestamptz FROM public.media_references WHERE id='old'`).Scan(&unknown); err != nil || !unknown {
		t.Fatalf("invented legacy publication time: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id,published_at) VALUES('new','asset','role','moment','moment','2026-11-19T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	var before, after string
	if err := pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(r) ORDER BY id)::text FROM public.media_references r`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(r) ORDER BY id)::text FROM public.media_references r`).Scan(&after); err != nil || after != before {
		t.Fatal("rerun rewrote publication history", err)
	}
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != Head {
		t.Fatalf("head=%s err=%v", head, err)
	}
}
