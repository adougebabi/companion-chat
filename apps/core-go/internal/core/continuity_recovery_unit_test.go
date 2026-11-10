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

type continuityRows struct {
	pgx.Rows
	values []string
	index  int
}

func (r *continuityRows) Close()     {}
func (r *continuityRows) Err() error { return nil }
func (r *continuityRows) Next() bool { r.index++; return r.index <= len(r.values) }
func (r *continuityRows) Scan(dest ...any) error {
	*dest[0].(*string) = r.values[r.index-1]
	return nil
}

type continuityRow func(...any) error

func (r continuityRow) Scan(dest ...any) error { return r(dest...) }

type turnRetryTx struct {
	pgx.Tx
	status, errorCode string
	assistant         bool
	committed         bool
	updates           []string
}

type committedLedgerRows struct {
	pgx.Rows
	read bool
}

func (r *committedLedgerRows) Close()     {}
func (r *committedLedgerRows) Err() error { return nil }
func (r *committedLedgerRows) Next() bool {
	if r.read {
		return false
	}
	r.read = true
	return true
}
func (r *committedLedgerRows) Scan(dest ...any) error {
	*dest[0].(*[]byte) = jsonBytes(map[string]any{"status": "completed", "capability_name": "test.committed.effect"})
	return nil
}

func (tx *turnRetryTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM public.tool_executions") {
		if tx.committed {
			return &committedLedgerRows{}, nil
		}
		return &continuityRows{}, nil
	}
	return nil, fmt.Errorf("unexpected query: %s", sql)
}

func (tx *turnRetryTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	return continuityRow(func(dest ...any) error {
		switch {
		case strings.Contains(sql, "FROM public.cognition_inbox WHERE fluctlight_id"):
			*dest[0].(*string) = "inbox-old"
			*dest[1].(*[]byte) = jsonBytes(map[string]any{"actor_id": "owner", "conversation_id": "conversation", "turn_id": "turn", "text": "old message"})
			*dest[2].(*string), *dest[3].(*string), *dest[4].(*string) = tx.status, tx.errorCode, ""
			*dest[5].(**time.Time) = nil
		case strings.Contains(sql, "FROM public.cognition_frozen_actions"):
			*dest[0].(*bool), *dest[1].(*string), *dest[2].(*string) = tx.assistant, "", ""
		default:
			return fmt.Errorf("unexpected query row: %s", sql)
		}
		return nil
	})
}

func (tx *turnRetryTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	tx.updates = append(tx.updates, sql)
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func TestSupersededTurnRetryDoesNotReclaimOldMessage(t *testing.T) {
	for _, claim := range []string{"", "stream-owner"} {
		tx := &turnRetryTx{status: "failed", errorCode: "superseded_by_newer_turn"}
		id, _, err := (&App{}).enqueueTurnFactTx(context.Background(), tx, "owner", "", "fluctlight", "conversation", "turn", "key", "old message", nil, messageTime{}, claim)
		if err != nil || id != "inbox-old" || len(tx.updates) != 0 {
			t.Fatalf("superseded retry claim=%q id=%q updates=%v err=%v", claim, id, tx.updates, err)
		}
	}
	committed := &turnRetryTx{status: "failed", errorCode: "superseded_by_newer_turn", committed: true}
	_, _, err := (&App{}).enqueueTurnFactTx(context.Background(), committed, "owner", "", "fluctlight", "conversation", "turn", "key", "old message", nil, messageTime{}, "stream-owner")
	if err != nil || len(committed.updates) != 1 || !strings.Contains(committed.updates[0], "SET error_code=$2") {
		t.Fatalf("committed effects were not retained as a terminal outcome: updates=%v err=%v", committed.updates, err)
	}
	for _, assistant := range []bool{false, true} {
		tx := &turnRetryTx{status: "failed", errorCode: "retryable_failure", assistant: assistant}
		_, _, err := (&App{}).enqueueTurnFactTx(context.Background(), tx, "owner", "", "fluctlight", "conversation", "turn", "key", "old message", nil, messageTime{}, "stream-owner")
		if err != nil || len(tx.updates) != 1 {
			t.Fatalf("ordinary recovery assistant=%t updates=%v err=%v", assistant, tx.updates, err)
		}
		want := "status='claimed'"
		if assistant {
			want = "status='processed'"
		}
		if !strings.Contains(tx.updates[0], want) {
			t.Fatalf("recovery chose wrong transition: %s", tx.updates[0])
		}
	}
}
