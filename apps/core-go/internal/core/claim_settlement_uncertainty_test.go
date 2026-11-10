package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type claimCaptureTx struct {
	pgx.Tx
	rows [][]any
}

func (tx *claimCaptureTx) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if !strings.Contains(query, "INSERT INTO public.cognition_claims") {
		return pgconn.CommandTag{}, fmt.Errorf("unexpected statement")
	}
	tx.rows = append(tx.rows, append([]any(nil), args...))
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func claimUncertaintyFixture(t *testing.T) map[string]any {
	t.Helper()
	decision := map[string]any{"action_type": "reply", "claims": []any{
		map[string]any{"kind": ClaimObservedFact, "content": "低置信度陈述", "confidence": 0.2, "evidence_refs": []any{"current_message"}},
		map[string]any{"kind": ClaimObservedFact, "content": "有依据的陈述", "confidence": 0.9, "evidence_refs": []any{"current_message"}},
		map[string]any{"kind": ClaimSupportedHypothesis, "content": "有依据的假设", "confidence": 0.9, "evidence_refs": []any{"current_message"}},
		map[string]any{"kind": ClaimUnsupportedSelf, "content": "没有依据的自述", "confidence": 0.9, "evidence_refs": []any{}},
		map[string]any{"kind": ClaimUnsupportedSelf, "content": "不应提升的自述", "confidence": 0.9, "evidence_refs": []any{"current_message"}},
		map[string]any{"kind": ClaimUncertainHypothesis, "content": "待确认假设", "confidence": 0.9, "evidence_refs": []any{"current_message"}},
	}}
	plan, err := normalizeResponsePlan(decision, "claim-source", ContextProjection{})
	if err != nil || len(arrayValue(plan["approved_claims"])) != 3 || len(arrayValue(plan["uncertain_claims"])) != 2 || len(arrayValue(plan["omitted_claims"])) != 1 {
		t.Fatalf("claim evaluation failed: %#v %v", plan, err)
	}
	return plan
}

func TestClaimSettlementPreservesEvaluatedUncertainty(t *testing.T) {
	tx := &claimCaptureTx{}
	at := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	plan := claimUncertaintyFixture(t)
	plan["omitted_claims"] = append(arrayValue(plan["omitted_claims"]), map[string]any{"kind": ClaimUncertainHypothesis, "content": "省略的假设", "confidence": 0.2, "evidence_refs": []any{}})
	if err := persistClaimsTx(context.Background(), tx, "fixture-character", "claim-source", plan, at); err != nil {
		t.Fatal(err)
	}
	byContent := make(map[string][]any, len(tx.rows))
	for _, row := range tx.rows {
		byContent[row[4].(string)] = row
	}
	for content, status := range map[string]string{"低置信度陈述": "uncertain", "有依据的陈述": "active", "有依据的假设": "active", "没有依据的自述": "rejected", "不应提升的自述": "rejected", "待确认假设": "uncertain", "省略的假设": "rejected"} {
		row := byContent[content]
		if len(row) != 10 || row[8] != status {
			t.Fatalf("claim %q stored with wrong status: %#v", content, row)
		}
		if status == "uncertain" || content == "有依据的假设" {
			expires, ok := row[9].(time.Time)
			if !ok || !expires.Equal(at.Add(7*24*time.Hour)) {
				t.Fatalf("uncertain claim %q lost expiry: %#v", content, row[9])
			}
		} else if content == "有依据的陈述" && row[9] != nil {
			t.Fatalf("observed claim gained an expiry: %#v", row[9])
		}
	}
}

func TestPostgresClaimSettlementPreservesEvaluatedUncertainty(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	seedLifeContextFluctlight(t, ctx, repo, "claim-owner", "claim-character")
	at := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	for _, runAt := range []time.Time{at, at.Add(time.Hour)} {
		if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
			return persistClaimsTx(ctx, tx, "claim-character", "claim-source", claimUncertaintyFixture(t), runAt)
		}); err != nil {
			t.Fatal(err)
		}
	}
	var status string
	var expires *time.Time
	if err := repo.Pool().QueryRow(ctx, `SELECT status,expires_at FROM public.cognition_claims WHERE fluctlight_id='claim-character' AND content='低置信度陈述'`).Scan(&status, &expires); err != nil || status != "uncertain" || expires == nil || !expires.Equal(at.Add(7*24*time.Hour)) {
		t.Fatalf("persisted low-confidence claim: status=%q expires=%v err=%v", status, expires, err)
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_claims WHERE fluctlight_id='claim-character' AND content='低置信度陈述'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("claim replay inserted duplicates: count=%d err=%v", count, err)
	}
}
