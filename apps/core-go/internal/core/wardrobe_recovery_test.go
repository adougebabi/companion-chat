package core

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestAutomaticWardrobeRecoverySkipsCancelledAndPausedWithoutStarvingBatch(t *testing.T) {
	cancelled := seedWardrobeToolFixture(t)
	paused := seedWardrobeToolFixture(t)
	eligible := seedWardrobeToolFixture(t)
	prepare := func(f independentToolE2EFixture, intentStatus string) string {
		t.Helper()
		var id string
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			var err error
			id, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "recovery-regression")
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_preparations SET status='failed',attempt=2,result='{"inventory_committed":true}',updated_at=now()-interval '2 hours' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status=$2,last_error='original-failure' WHERE intent_id=$1`, "wardrobe_initialize:"+f.fluctlightID, intentStatus); err != nil {
			t.Fatal(err)
		}
		return id
	}
	cancelledID := prepare(cancelled, "cancelled")
	pausedID := prepare(paused, "failed")
	eligibleID := prepare(eligible, "failed")
	if _, err := paused.repository.Pool().Exec(paused.ctx, `UPDATE public.fluctlights SET status='paused' WHERE id=$1`, paused.fluctlightID); err != nil {
		t.Fatal(err)
	}
	// The cancelled row is older than the eligible row and must not occupy LIMIT 1.
	if _, err := eligible.repository.Pool().Exec(eligible.ctx, `UPDATE public.fluctlight_wardrobe_preparations SET updated_at=now()-interval '1 hour' WHERE id=$1`, eligibleID); err != nil {
		t.Fatal(err)
	}
	count, err := eligible.app.RepairWardrobePreparations(eligible.ctx, 1)
	if err != nil || count != 1 {
		t.Fatal("eligible preparation starved", count, err)
	}
	for _, row := range []struct {
		id, status string
		attempt    int
	}{{cancelledID, "failed", 2}, {pausedID, "failed", 2}, {eligibleID, "awaiting_wear", 3}} {
		var status string
		var attempt int
		if err := eligible.repository.Pool().QueryRow(eligible.ctx, `SELECT status,attempt FROM public.fluctlight_wardrobe_preparations WHERE id=$1`, row.id).Scan(&status, &attempt); err != nil || status != row.status || attempt != row.attempt {
			t.Fatal(row, status, attempt, err)
		}
	}
	var status, lastError string
	if err := eligible.repository.Pool().QueryRow(eligible.ctx, `SELECT status,last_error FROM public.platform_workflow_intents WHERE intent_id=$1`, "wardrobe_initialize:"+eligible.fluctlightID).Scan(&status, &lastError); err != nil || status != "failed" || lastError != "original-failure" {
		t.Fatal("original failure history changed", status, lastError, err)
	}
	if _, err := eligible.repository.Pool().Exec(eligible.ctx, `UPDATE public.fluctlight_wardrobe_preparations SET status='failed',updated_at=now()-interval '1 hour' WHERE id=$1`, eligibleID); err != nil {
		t.Fatal(err)
	}
	if count, err := eligible.app.RepairWardrobePreparations(eligible.ctx, 1); err != nil || count != 0 {
		t.Fatal("automatic retry exceeded bound or reversed cancellation", count, err)
	}
}
