package core

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// A completed status is seeded as a fixture here; this checks subsequent
// import/planner behavior, not whether the completion itself was justified.
func TestCompletedGoalIsNotRevivedByOldInitialWish(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := plannerOwnerGoal(t, f, "已有愿望", false)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET status='completed',progress='1'::jsonb WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	goals := []any{map[string]any{"source_id": "old-wish-again", "description": "已有愿望", "success_criteria": []string{"真实结果"}, "motivation": "旧人格愿望"}}
	for i := 0; i < 2; i++ {
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			return f.app.insertAgency(f.ctx, tx, f.fluctlightID, f.ownerID, goals, []any{}, map[string]struct{}{"default": {}})
		}); err != nil {
			t.Fatal(err)
		}
	}
	var total int
	var linked, status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1),
		(SELECT goal_id FROM public.goal_initial_imports WHERE fluctlight_id=$1 AND source_id='old-wish-again'),
		(SELECT status FROM public.fluctlight_goals WHERE id=$2)`, f.fluctlightID, id).Scan(&total, &linked, &status); err != nil || total != 1 || linked != id || status != "completed" {
		t.Fatalf("old wish revived completed goal: total=%d linked=%s status=%s err=%v", total, linked, status, err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return requestGoalPlanningTx(f.ctx, tx, f.fluctlightID, "owner:completed-check", "owner_request", map[string]any{"profile_id": "default"})
	}); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 ORDER BY created_at DESC LIMIT 1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=1,claimed_at=now(),mode='apply',profile_id='default' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	command := plannerCommand(t, f, "completed-duplicate-protection")
	command.Changes = []GoalSetChange{{ID: "duplicate", Operation: "create", Outcome: "已有愿望", Criteria: []string{"真实结果"}, Motivation: "旧人格愿望", Activate: true}}
	err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.applyGoalSetTx(f.ctx, tx, f.ownerID, f.fluctlightID, "default", run+"@1", command)
		return err
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("planner accepted completed duplicate: %v", err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*),min(status) FROM public.fluctlight_goals WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&total, &status); err != nil || total != 1 || status != "completed" {
		t.Fatalf("planner changed completed history: total=%d status=%s err=%v", total, status, err)
	}
}

func TestGoalPlannerCannotResumeOwnerPausedGoal(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := plannerOwnerGoal(t, f, "owner-paused", false)
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, GoalOwnerCommand{Operation: "pause", ExpectedRevision: 1, IdempotencyKey: "owner-pause-t10", Reason: "Owner 暂停"}); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return requestGoalPlanningTx(f.ctx, tx, f.fluctlightID, "owner:paused-check", "owner_request", map[string]any{"profile_id": "default"})
	}); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 LIMIT 1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=1,claimed_at=now(),mode='apply',profile_id='default' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	command := plannerCommand(t, f, "owner-pause-protection")
	command.Changes = []GoalSetChange{{ID: id, Operation: "resume", ExpectedRevision: 2}}
	err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.applyGoalSetTx(f.ctx, tx, f.ownerID, f.fluctlightID, "default", run+"@1", command)
		return err
	})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("planner resumed Owner-paused goal: %v", err)
	}
	var status string
	var revision int
	var protected bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,revision,owner_protected FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&status, &revision, &protected); err != nil || status != "paused" || revision != 2 || !protected {
		t.Fatalf("Owner pause changed: status=%s revision=%d protected=%t err=%v", status, revision, protected, err)
	}
}
