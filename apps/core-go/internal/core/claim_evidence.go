package core

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type cognitionClaimAssessment struct {
	Kind, Content, RepetitionKey, Source, Status, Reason string
	Evidence                                             []any
	Confidence                                           float64
	Expires                                              any
	BusinessAt                                           time.Time
}

// Classification and expiry retain the evaluated group's existing semantics.
// A new source is not permission to promote uncertainty without new evidence.
func claimAssessmentsForPlan(plan map[string]any, source string, at time.Time) []cognitionClaimAssessment {
	result := []cognitionClaimAssessment{}
	for _, group := range []struct{ key, status string }{{"approved_claims", "active"}, {"uncertain_claims", "uncertain"}, {"omitted_claims", "rejected"}} {
		for _, raw := range arrayValue(plan[group.key]) {
			claim := mapValue(raw)
			a := cognitionClaimAssessment{Kind: stringValue(claim["kind"]), Content: stringValue(claim["content"]), Source: source, Status: group.status, Reason: stringValue(claim["reason_code"]), Evidence: arrayValue(claim["evidence_refs"]), Confidence: boundedNumber(claim["confidence"], 0), BusinessAt: at.UTC()}
			a.RepetitionKey = firstString(claim["repetition_key"], repetitionKeyFor(a.Content))
			if a.Kind == ClaimUnsupportedSelf || a.Reason != "" {
				a.Status = "rejected"
			} else if a.Status != "rejected" && a.Kind == ClaimUncertainHypothesis {
				a.Status = "uncertain"
			}
			if a.Status == "uncertain" || a.Kind == ClaimSupportedHypothesis {
				a.Expires = at.UTC().Add(7 * 24 * time.Hour)
			}
			result = append(result, a)
		}
	}
	return result
}

func (a cognitionClaimAssessment) snapshot() map[string]any {
	return map[string]any{"source_fact_id": a.Source, "claim_type": a.Kind, "content": a.Content, "evidence_refs": a.Evidence, "confidence": a.Confidence, "repetition_key": a.RepetitionKey, "status": a.Status, "reason_code": a.Reason, "expires_at": a.Expires}
}

func (a cognitionClaimAssessment) key() string {
	payload := a.snapshot()
	// Retry time cannot turn the same evaluation into new support or a new TTL.
	delete(payload, "expires_at")
	refs := make([]string, 0, len(a.Evidence))
	for _, ref := range a.Evidence {
		refs = append(refs, stringValue(ref))
	}
	sort.Strings(refs)
	payload["evidence_refs"] = refs
	return stableDigest(jsonString(payload))
}

func persistClaimAssessmentsTx(ctx context.Context, tx pgx.Tx, fid, source string, plan map[string]any, at time.Time) error {
	assessments := claimAssessmentsForPlan(plan, source, at)
	keys := map[string]struct{}{}
	for _, a := range assessments {
		keys[a.RepetitionKey] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	for _, key := range ordered {
		lock := "claim-evidence:" + stableDigest(jsonString([]string{fid, key}))
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lock); err != nil {
			return err
		}
	}
	for _, a := range assessments {
		if err := persistClaimAssessmentTx(ctx, tx, fid, a); err != nil {
			return err
		}
	}
	return nil
}

// A model repetition label is not a complete fact identity. Preserve legacy
// keys/IDs; only a new conflicting fact receives a Core-owned storage key.
func claimIdentityTx(ctx context.Context, tx pgx.Tx, fid string, a cognitionClaimAssessment) (map[string]any, string, error) {
	alternative := "claim-fact:" + stableDigest(jsonString([]string{a.RepetitionKey, a.Content}))
	candidates := []string{alternative}
	if len([]rune(a.RepetitionKey)) <= 256 {
		candidates = append([]string{a.RepetitionKey}, candidates...)
	}
	free, haveFree := "", false
	for _, candidate := range candidates {
		var raw []byte
		var original string
		err := tx.QueryRow(ctx, `SELECT to_jsonb(c),COALESCE((SELECT original_repetition_key FROM public.cognition_claim_evaluations e WHERE e.claim_id=c.id AND e.applied ORDER BY e.version DESC LIMIT 1),c.repetition_key) FROM public.cognition_claims c WHERE c.fluctlight_id=$1 AND c.repetition_key=$2 FOR UPDATE OF c`, fid, candidate).Scan(&raw, &original)
		if errors.Is(err, pgx.ErrNoRows) {
			if !haveFree {
				free, haveFree = candidate, true
			}
			continue
		}
		if err != nil {
			return nil, "", err
		}
		row := decodeObject(raw)
		if original == a.RepetitionKey && stringValue(row["content"]) == a.Content {
			return row, candidate, nil
		}
	}
	if !haveFree {
		return nil, "", ErrConflict
	}
	return nil, free, nil
}

func persistClaimAssessmentTx(ctx context.Context, tx pgx.Tx, fid string, a cognitionClaimAssessment) error {
	row, storageKey, err := claimIdentityTx(ctx, tx, fid, a)
	if err != nil {
		return err
	}
	id := stringValue(row["id"])
	created := id == ""
	if created {
		id = "claim_" + stableDigest(jsonString([]string{fid, storageKey, a.Content}))
		_, err = tx.Exec(ctx, `INSERT INTO public.cognition_claims(id,fluctlight_id,source_fact_id,claim_type,content,evidence_refs,confidence,repetition_key,status,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, fid, a.Source, a.Kind, a.Content, jsonBytes(a.Evidence), a.Confidence, storageKey, a.Status, a.Expires)
		if err != nil {
			var conflict *pgconn.PgError
			if errors.As(err, &conflict) && conflict.Code == "23505" {
				return ErrConflict
			}
			return err
		}
	}
	var replay bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_claim_evaluations WHERE claim_id=$1 AND assessment_key=$2)`, id, a.key()).Scan(&replay); err != nil {
		return err
	}
	if replay {
		return nil
	}
	var version int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(version),-1) FROM public.cognition_claim_evaluations WHERE claim_id=$1`, id).Scan(&version); err != nil {
		return err
	}
	if !created && version == -1 {
		// This is the actual pre-upgrade current row, not an invented past run.
		_, err := tx.Exec(ctx, `INSERT INTO public.cognition_claim_evaluations(claim_id,assessment_key,version,original_repetition_key,source_fact_id,snapshot,business_at,applied,origin) VALUES($1,$2,0,$3,$4,$5,NULL,true,'legacy_current')`, id, "legacy:"+stableDigest(jsonString(row)), a.RepetitionKey, stringValue(row["source_fact_id"]), jsonBytes(row))
		if err != nil {
			return err
		}
		version = 0
	}
	apply, applicationReason := created, "applied"
	if !created {
		apply, applicationReason, err = applyClaimAssessmentTx(ctx, tx, id, a)
		if err != nil {
			return err
		}
	}

	if version < 0 {
		version = 0
	}
	snapshot := a.snapshot()
	snapshot["application_reason"] = applicationReason
	_, err = tx.Exec(ctx, `INSERT INTO public.cognition_claim_evaluations(claim_id,assessment_key,version,original_repetition_key,source_fact_id,snapshot,business_at,applied,origin) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'evaluated')`, id, a.key(), version+1, a.RepetitionKey, a.Source, jsonBytes(snapshot), a.BusinessAt, apply)
	return err
}

func applyClaimAssessmentTx(ctx context.Context, tx pgx.Tx, id string, a cognitionClaimAssessment) (bool, string, error) {
	if a.Status == "rejected" {
		return false, "rejected_evaluation", nil
	}
	fresh, err := claimHasNewSupportTx(ctx, tx, id, a.Evidence)
	if err != nil {
		return false, "", err
	}
	if !fresh {
		return false, "known_evidence", nil
	}
	var latest *time.Time
	err = tx.QueryRow(ctx, `SELECT business_at FROM public.cognition_claim_evaluations WHERE claim_id=$1 AND applied AND business_at IS NOT NULL ORDER BY version DESC LIMIT 1`, id).Scan(&latest)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, "", err
	}
	if latest != nil && a.BusinessAt.Before(*latest) {
		return false, "older_evaluation", nil
	}
	_, err = tx.Exec(ctx, `UPDATE public.cognition_claims SET source_fact_id=$2,claim_type=$3,content=$4,evidence_refs=$5,confidence=$6,status=$7,expires_at=$8,updated_at=now() WHERE id=$1`, id, a.Source, a.Kind, a.Content, jsonBytes(a.Evidence), a.Confidence, a.Status, a.Expires)
	return err == nil, "applied", err
}

// A different settlement source is not itself new claim support. History is
// consulted for novelty only; old support is never unioned into current facts.
func claimHasNewSupportTx(ctx context.Context, tx pgx.Tx, claimID string, refs []any) (bool, error) {
	if len(refs) == 0 {
		return false, nil
	}
	var fresh bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jsonb_array_elements_text($2::jsonb) ref WHERE NOT EXISTS(SELECT 1 FROM public.cognition_claim_evaluations e WHERE e.claim_id=$1 AND e.snapshot->'evidence_refs' @> jsonb_build_array(ref.value)))`, claimID, jsonBytes(refs)).Scan(&fresh)
	return fresh, err
}

func sameClaimContent(hypothesis map[string]any, content string) bool {
	return stringValue(hypothesis["content"]) == content
}
