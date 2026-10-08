package core

import (
	"errors"
	"testing"
)

func TestGoalStockReconciliationPreviewsReplaysAndPreservesEndedHistory(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	legacy := createDialogueGoalForClosure(t, f, []string{"legacy criterion"})
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET status='completed',progress='1' WHERE id=$1`, legacy); err != nil {
		t.Fatal(err)
	}
	desired, motivation := "旧长期原意", "保留原动机"
	active, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", IdempotencyKey: "stock-active", Reason: "存量夹具", DesiredOutcome: &desired, Motivation: &motivation, SuccessCriteria: []string{"真实长期结果"}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := f.app.ReconcileGoalStock(f.ctx, f.ownerID, f.fluctlightID, "", 10, false, "")
	if err != nil || len(report.Items) != 2 || report.Digest == "" || report.Applied {
		t.Fatalf("preview %#v %v", report, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_migration_review_flags WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("preview wrote flags %d %v", count, err)
	}
	if _, err := f.app.ReconcileGoalStock(f.ctx, "foreign-owner", f.fluctlightID, "", 10, true, report.Digest); err == nil {
		t.Fatal("foreign owner repaired stock")
	}
	applied, err := f.app.ReconcileGoalStock(f.ctx, f.ownerID, f.fluctlightID, "", 10, true, report.Digest)
	if err != nil || !applied.Applied {
		t.Fatalf("apply %#v %v", applied, err)
	}
	replay, err := f.app.ReconcileGoalStock(f.ctx, f.ownerID, f.fluctlightID, "", 10, true, report.Digest)
	if err != nil || jsonString(replay) != jsonString(applied) {
		t.Fatalf("batch replay %#v %v", replay, err)
	}
	var state, description string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,desired_outcome FROM public.fluctlight_goals WHERE id=$1`, legacy).Scan(&state, &description); err != nil || state != "completed" || description != "向对方清楚表达自己的心意" {
		t.Fatalf("legacy rewritten %s %s %v", state, description, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_migration_review_flags WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("missing review flag %d %v", count, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_stages WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("SQL fabricated stage %d %v", count, err)
	}
	preview, err := f.app.ReconcileGoalStock(f.ctx, f.ownerID, f.fluctlightID, "", 1, false, "")
	if err != nil || preview.NextCursor == "" {
		t.Fatalf("bounded batch %#v %v", preview, err)
	}
	stalePreview, err := f.app.ReconcileGoalStock(f.ctx, f.ownerID, f.fluctlightID, "", 10, false, "")
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(active["goal_id"])
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, GoalOwnerCommand{Operation: "pause", ExpectedRevision: 1, IdempotencyKey: "stock-mutation", Reason: "并发治理"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ReconcileGoalStock(f.ctx, f.ownerID, f.fluctlightID, "", 10, true, stalePreview.Digest); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale preview accepted %v", err)
	}
}

func TestGoalStockReconciliationResumesBatchesWithoutRevivingEndedGoals(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	states := map[string]string{}
	for _, status := range []string{"active", "paused", "cancelled", "abandoned"} {
		desired, motivation := "preserve "+status, "original motivation"
		created, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", IdempotencyKey: "resume-create-" + status, Reason: "stock fixture", DesiredOutcome: &desired, Motivation: &motivation, SuccessCriteria: []string{"actual result"}})
		if err != nil {
			t.Fatal(err)
		}
		id := stringValue(created["goal_id"])
		if status != "active" {
			operation := map[string]string{"paused": "pause", "cancelled": "cancel", "abandoned": "abandon"}[status]
			if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, GoalOwnerCommand{Operation: operation, ExpectedRevision: 1, IdempotencyKey: "resume-end-" + status, Reason: "explicit owner lifecycle"}); err != nil {
				t.Fatal(err)
			}
		}
		states[id] = status
	}
	cursor := ""
	seen := map[string]bool{}
	app := f.app
	for {
		preview, err := app.ReconcileGoalStock(f.ctx, f.ownerID, f.fluctlightID, cursor, 1, false, "")
		if err != nil || len(preview.Items) != 1 {
			t.Fatalf("preview=%#v err=%v", preview, err)
		}
		applied, err := app.ReconcileGoalStock(f.ctx, f.ownerID, f.fluctlightID, cursor, 1, true, preview.Digest)
		if err != nil {
			t.Fatal(err)
		}
		// A new App loses all process-local state between committed batches.
		restarted := *app
		app = &restarted
		replay, err := app.ReconcileGoalStock(f.ctx, f.ownerID, f.fluctlightID, cursor, 1, true, preview.Digest)
		if err != nil || jsonString(replay) != jsonString(applied) {
			t.Fatalf("restarted replay differs: %#v %v", replay, err)
		}
		item := applied.Items[0]
		if seen[item.GoalID] {
			t.Fatal("cursor repeated a Goal")
		}
		seen[item.GoalID] = true
		if (item.Status == "cancelled" || item.Status == "abandoned") && item.EvaluationRequestID != "" {
			t.Fatalf("ended goal queued for assessment: %#v", item)
		}
		if applied.NextCursor == "" {
			break
		}
		cursor = applied.NextCursor
	}
	if len(seen) != len(states) {
		t.Fatalf("restart lost batch: %d/%d", len(seen), len(states))
	}
	for id, want := range states {
		var state, description string
		var revision int
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,desired_outcome,revision FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&state, &description, &revision); err != nil {
			t.Fatal(err)
		}
		wantRevision := 2
		if want == "active" {
			wantRevision = 1
		}
		if state != want || description != "preserve "+want || revision != wantRevision {
			t.Fatalf("stock rewrote authority: %s %s rev=%d", state, description, revision)
		}
	}
	var batches, attempts int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_reconciliation_batches WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&batches); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if batches != 4 || attempts != 0 {
		t.Fatalf("duplicate batches or fabricated operations: batches=%d attempts=%d", batches, attempts)
	}
}
