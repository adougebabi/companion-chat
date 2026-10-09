package core

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"testing"
)

type candidateQueueRow struct{ value any }

func (r candidateQueueRow) Scan(dest ...any) error {
	switch v := r.value.(type) {
	case bool:
		*dest[0].(*bool) = v
	case string:
		*dest[0].(*string) = v
	}
	return nil
}

type candidateQueueTx struct {
	pgx.Tx
	inserted bool
	queued   int
}

func (t *candidateQueueTx) Exec(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
	if strings.Contains(query, "INSERT INTO public.goal_evidence_links") {
		if t.inserted {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		}
		return pgconn.NewCommandTag("INSERT 0 0"), nil
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (t *candidateQueueTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	if strings.Contains(query, "INSERT INTO public.goal_evaluation_requests") {
		t.queued++
		return candidateQueueRow{"evaluation"}
	}
	return candidateQueueRow{true}
}
func TestGoalCandidateRequiresNewCommittedEvidenceBeforeQueue(t *testing.T) {
	for _, inserted := range []bool{false, true} {
		tx := &candidateQueueTx{inserted: inserted}
		projection := ContextProjection{ReferenceIndex: ContextReferenceIndex{ActiveProfileID: "profile", ByRef: map[string]ContextReference{"goal-ref": {Kind: ContextReferenceGoal, EntityID: "goal"}}}}
		err := (&App{}).enqueueTurnGoalCandidatesTx(context.Background(), tx, "owner", "inbox", projection, map[string]any{"goal_event_candidates": []any{map[string]any{"goal_ref": "goal-ref"}}})
		if err != nil {
			t.Fatal(err)
		}
		expected := 0
		if inserted {
			expected = 1
		}
		if tx.queued != expected {
			t.Fatalf("new link=%t queued=%d want=%d", inserted, tx.queued, expected)
		}
	}
}
