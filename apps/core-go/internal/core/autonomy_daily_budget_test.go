package core

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type autonomyConfigRow struct{ raw []byte }

func (r autonomyConfigRow) Scan(dest ...any) error {
	if r.raw == nil {
		return pgx.ErrNoRows
	}
	*dest[0].(*[]byte) = r.raw
	return nil
}

type autonomyConfigReader struct{ raw []byte }

func (r autonomyConfigReader) QueryRow(context.Context, string, ...any) pgx.Row {
	return autonomyConfigRow{raw: r.raw}
}

func TestAutonomyBudgetRefillsOnANewDay(t *testing.T) {
	now := time.Date(2026, 11, 26, 1, 0, 0, 0, time.UTC)
	if !autonomyBudgetNewDay(now.Add(-2*time.Hour), now) {
		t.Fatal("budget spent yesterday was not refilled")
	}
	if autonomyBudgetNewDay(now.Add(-30*time.Minute), now) || autonomyBudgetNewDay(time.Time{}, now) {
		t.Fatal("budget refilled within the same day")
	}
}

func TestAutonomyDailyAllowanceFollowsOwnerSetting(t *testing.T) {
	ctx := context.Background()
	if got := autonomyDailyAllowance(ctx, autonomyConfigReader{raw: []byte(`{"budget_remaining":"0"}`)}, "inherit"); got != 0 {
		t.Fatalf("owner-disabled budget was refilled to %v", got)
	}
	if got := autonomyDailyAllowance(ctx, autonomyConfigReader{raw: []byte(`{"budget_remaining":"20"}`)}, "inherit"); got != 20 {
		t.Fatalf("inherited allowance = %v, want 20", got)
	}
	if got := autonomyDailyAllowance(ctx, autonomyConfigReader{}, "override"); got != autonomyDefaultDailyBudget {
		t.Fatalf("override allowance = %v", got)
	}
}
