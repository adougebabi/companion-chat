package core

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestWardrobeInactivePreparationPersistsValidNoopDiagnostic(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var id string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		id, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "noop-diagnostic")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET status='paused' WHERE id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.ProcessWardrobePreparation(f.ctx, id)
	if err != nil || result["status"] != "awaiting_wear" || result["reason_code"] != "character_inactive" || result["reason"] != "角色当前未处于活动状态" {
		t.Fatal(result, err)
	}
	eventID, err := f.app.RecordLifecycleDiagnostic(f.ctx, LifecycleDiagnostic{
		Surface: "wardrobe", Transition: LifecycleTransitionCompletedNoop,
		FluctlightID: f.fluctlightID, CorrelationID: "wardrobe-preparation:" + id,
		Stage: "activity", Status: stringValue(result["status"]), ReasonCode: stringValue(result["reason_code"]),
	})
	if err != nil || eventID == "" {
		t.Fatal("actual wardrobe noop diagnostic was rejected", result, eventID, err)
	}
	f.requireNoModelRuns(t)
}

func TestAutomaticWardrobeRecoverySkipsCancelledAndPausedWithoutStarvingBatch(t *testing.T) {
	eligible := seedWardrobeToolFixture(t)
	// All three rows must compete in the same recovery query and LIMIT. Each
	// seedWardrobeToolFixture call creates its own isolated database.
	sharedFixture := func(label string) independentToolE2EFixture {
		t.Helper()
		f := eligible
		f.ownerID += "-" + label
		f.fluctlightID += "-" + label
		seedLifeContextFluctlight(t, f.ctx, f.repository, f.ownerID, f.fluctlightID)
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(f.ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'default',0)`, f.fluctlightID); err != nil {
				return err
			}
			return initializeEffectiveLifeTx(f.ctx, tx, f.fluctlightID, map[string]any{})
		}); err != nil {
			t.Fatal(err)
		}
		return f
	}
	cancelled := sharedFixture("cancelled")
	paused := sharedFixture("paused")
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
	var competing int
	if err := eligible.repository.Pool().QueryRow(eligible.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_preparations WHERE id IN ($1,$2,$3)`, cancelledID, pausedID, eligibleID).Scan(&competing); err != nil || competing != 3 {
		t.Fatal("recovery fixtures do not share a candidate database", competing, err)
	}
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
