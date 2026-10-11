package core

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestGoalPlannerVersionedContextReviewRecovery(t *testing.T) {
	for _, blocked := range []string{"legacy_query_failure", "committed_failure", "different_failure", "live_claim"} {
		t.Run(blocked, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			goalID := plannerOwnerGoal(t, f, "review-held", false)
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET context_review_required=true WHERE id=$1`, goalID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.platform_workflow_intents WHERE intent_type='goal.plan' AND payload->>'fluctlight_id'=$1`, f.fluctlightID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.goal_planning_runs WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
				t.Fatal(err)
			}
			status, code, result := "failed", "goal_planner_query_results_missing", `{"status":"failed","reason":"goal_planner_query_results_missing"}`
			if blocked == "committed_failure" {
				result = `{"status":"failed","commit_succeeded":true}`
			}
			if blocked == "different_failure" {
				code = "retry_exhausted"
			}
			if blocked == "live_claim" {
				status = "processing"
			}
			oldID := "old-planner-" + f.suffix
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_planning_runs(id,fluctlight_id,status,error_code,result,attempt_count,available_at,claimed_at) VALUES($1,$2,$3,$4,$5,5,now()+interval '60 seconds',now())`, oldID, f.fluctlightID, status, code, []byte(result)); err != nil {
				t.Fatal(err)
			}
			repair := func() int {
				t.Helper()
				count := 0
				if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
					var err error
					count, err = f.app.repairGoalPlannerContextReviewsTx(f.ctx, tx, 10)
					return err
				}); err != nil {
					t.Fatal(err)
				}
				return count
			}
			want := 0
			if blocked == "legacy_query_failure" {
				want = 1
			}
			if count := repair(); count != want {
				t.Fatal("wrong recovery admission", count, want)
			}
			if count := repair(); count != 0 {
				t.Fatal("unchanged recovery repeated", count)
			}
			var oldStatus string
			var attempts int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,attempt_count FROM public.goal_planning_runs WHERE id=$1`, oldID).Scan(&oldStatus, &attempts); err != nil || oldStatus != status || attempts != 5 {
				t.Fatal("old run changed", oldStatus, attempts, err)
			}
			if want == 1 {
				var pending int
				var backoff bool
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*),bool_and(available_at>now()+interval '50 seconds') FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID).Scan(&pending, &backoff); err != nil || pending != 1 || !backoff {
					t.Fatal("recovery backoff/run missing", pending, backoff, err)
				}
				var review bool
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT context_review_required FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&review); err != nil || !review {
					t.Fatal("recovery bypassed domain review", review, err)
				}
				// A later successful assessment may revise the Goal without resolving
				// its Planner hold. Exhausting this recovery must remain bounded.
				if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='failed',error_code='goal_planner_query_results_missing',result='{"reason":"goal_planner_query_results_missing"}' WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET revision=revision+1 WHERE id=$1`, goalID); err != nil {
					t.Fatal(err)
				}
				if count := repair(); count != 0 {
					t.Fatal("assessment revision reset recovery budget", count)
				}
			}
		})
	}
}
