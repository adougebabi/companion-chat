package migrations

import "testing"

func TestClaimEvidenceUpgradePreservesLegacyAndRerun(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
 DROP TABLE public.cognition_claim_evaluations;
 INSERT INTO public.cognition_claims(id,fluctlight_id,source_fact_id,claim_type,content,evidence_refs,confidence,repetition_key,status,expires_at)
 VALUES('legacy-claim','legacy-role','legacy-source','uncertain_hypothesis','旧判断原文','["legacy-source"]',0.2,'legacy-alias','uncertain','2026-11-26T00:00:00Z');
 UPDATE public.alembic_version SET version_num='0117_media_submission_guard'`); err != nil {
		t.Fatal(err)
	}
	var before []byte
	if err := pool.QueryRow(ctx, `SELECT to_jsonb(c) FROM public.cognition_claims c WHERE id='legacy-claim'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var after []byte
	if err := pool.QueryRow(ctx, `SELECT to_jsonb(c) FROM public.cognition_claims c WHERE id='legacy-claim'`).Scan(&after); err != nil || string(before) != string(after) {
		t.Fatalf("upgrade rewrote legacy claim: err=%v before=%s after=%s", err, before, after)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.cognition_claim_evaluations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("upgrade invented evaluation history: count=%d err=%v", count, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.cognition_claim_evaluations(claim_id,assessment_key,version,original_repetition_key,source_fact_id,snapshot,applied,origin) SELECT id,'legacy-snapshot',0,repetition_key,source_fact_id,to_jsonb(c),true,'legacy_current' FROM public.cognition_claims c WHERE id='legacy-claim'`); err != nil {
		t.Fatal(err)
	}
	var evaluationBefore []byte
	if err := pool.QueryRow(ctx, `SELECT to_jsonb(e) FROM public.cognition_claim_evaluations e WHERE claim_id='legacy-claim'`).Scan(&evaluationBefore); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var evaluationAfter []byte
	if err := pool.QueryRow(ctx, `SELECT to_jsonb(e) FROM public.cognition_claim_evaluations e WHERE claim_id='legacy-claim'`).Scan(&evaluationAfter); err != nil || string(evaluationBefore) != string(evaluationAfter) {
		t.Fatalf("head rerun changed audit: err=%v", err)
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != Head {
		t.Fatalf("head=%q err=%v", head, err)
	}
}

func TestClaimEvidenceStorageRejectsInvalidAndDuplicateVersions(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.cognition_claims(id,fluctlight_id,source_fact_id,claim_type,content,evidence_refs,confidence,repetition_key,status) VALUES('claim','role','source','observed_fact','fact','["source"]',0.8,'alias','active')`); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO public.cognition_claim_evaluations(claim_id,assessment_key,version,original_repetition_key,source_fact_id,snapshot,applied,origin) VALUES($1,$2,$3,'alias','source',$4,true,$5)`
	if _, err := pool.Exec(ctx, insert, "claim", "first", 1, `{"content":"fact"}`, "evaluated"); err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		id, key          string
		version          int
		snapshot, origin string
	}{{"missing", "missing", 1, `{}`, "evaluated"}, {"claim", "first", 2, `{}`, "evaluated"}, {"claim", "other", 1, `{}`, "evaluated"}, {"claim", "negative", -1, `{}`, "evaluated"}, {"claim", "scalar", 2, `[]`, "evaluated"}, {"claim", "origin", 2, `{}`, "invented"}} {
		if _, err := pool.Exec(ctx, insert, sample.id, sample.key, sample.version, sample.snapshot, sample.origin); err == nil {
			t.Fatalf("invalid evaluation accepted: %#v", sample)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.cognition_claim_evaluations`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("invalid writes affected evaluations: count=%d err=%v", count, err)
	}
}
