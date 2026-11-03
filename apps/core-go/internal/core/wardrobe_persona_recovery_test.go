package core

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPostgresWardrobeRecoveryRestartsFailedPersonaPreparation(t *testing.T) {
	for _, endpoint := range []string{"wardrobe/recover", "inventory/preparation"} {
		for _, status := range []string{"failed", "dead_letter", "cancelled", "terminated"} {
			t.Run(endpoint+"/"+status, func(t *testing.T) {
				router := newFakeProviderRouter()
				ctx, repo, app := personaPreparationFixture(t, router)
				original := latestPreparationID(t, ctx, repo)
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status=$2,last_error='provider failed: http://internal/private' WHERE intent_id=$1`, original, status); err != nil {
					t.Fatal(err)
				}
				var checkpoint []byte
				if err := repo.Pool().QueryRow(ctx, `SELECT to_jsonb(p) FROM public.fluctlight_wardrobe_preparations p WHERE fluctlight_id='prep-role'`).Scan(&checkpoint); err != nil {
					t.Fatal(err)
				}
				preparation, err := app.OwnerWardrobePreparation(ctx, "prep-owner", "prep-role")
				if err != nil || preparation["status"] != "queued" || preparation["can_recover"] != true || preparation["error_code"] != "persona_preparation_failed" || !strings.Contains(stringValue(mapValue(preparation["result"])["failure_reason"]), "将先恢复人格准备") || strings.Contains(jsonString(preparation), "http://internal") {
					t.Fatal("upstream failure not actionable through wardrobe", preparation, err)
				}
				body := map[string]any{"reason": "恢复初始穿搭", "idempotency_key": "wardrobe-recovery"}
				if endpoint == "inventory/preparation" {
					var revision int
					if err := repo.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id='prep-role'`).Scan(&revision); err != nil {
						t.Fatal(err)
					}
					body["expected_revision"] = revision
				}
				path := "/internal/fluctlights/prep-role/" + endpoint
				out, err := app.DispatchOwnerSurface(ctx, "POST", path, nil, "prep-owner", body)
				if err != nil || out["message"] != wardrobePersonaRecoveryMessage {
					t.Fatal("wardrobe recovery did not explain persona recovery", out, err)
				}
				if endpoint == "wardrobe/recover" && (out["restarted"] != true || out["intent_id"] != preparation["task_id"] || out["status"] != "pending") {
					t.Fatal("wardrobe recovery lost original initializer", out)
				}
				recovered := latestPreparationID(t, ctx, repo)
				if recovered == original {
					t.Fatal("terminal persona execution reopened")
				}
				if _, err := app.DispatchOwnerSurface(ctx, "POST", path, nil, "prep-owner", body); err != nil {
					t.Fatal("recovery replay failed", err)
				}
				body["idempotency_key"] = "another-recovery"
				if _, err := app.DispatchOwnerSurface(ctx, "POST", path, nil, "prep-owner", body); err != nil {
					t.Fatal("pending recovery failed", err)
				}
				var count, outboxCount int
				var originalStatus, recoveredStatus string
				if err := repo.Pool().QueryRow(ctx, `SELECT count(*),max(status) FILTER(WHERE intent_id=$1),max(status) FILTER(WHERE intent_id=$2) FROM public.platform_workflow_intents WHERE intent_type='persona.compile'`, original, recovered).Scan(&count, &originalStatus, &recoveredStatus); err != nil || count != 2 || originalStatus != status || recoveredStatus != "pending" {
					t.Fatal("recovery duplicated or reopened persona execution", count, originalStatus, recoveredStatus, err)
				}
				if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='persona.compilation.requested'`).Scan(&outboxCount); err != nil || outboxCount != 2 {
					t.Fatal("persona recovery missing durable dispatch or duplicated outbox", outboxCount, err)
				}
				var after []byte
				if err := repo.Pool().QueryRow(ctx, `SELECT to_jsonb(p) FROM public.fluctlight_wardrobe_preparations p WHERE fluctlight_id='prep-role'`).Scan(&after); err != nil || string(after) != string(checkpoint) {
					t.Fatal("persona recovery or status read changed wardrobe checkpoint", string(after), string(checkpoint), err)
				}
				if _, err := app.ProcessPersonaPreparationIntent(ctx, recovered); err != nil {
					t.Fatal("recovered persona cannot complete", err)
				}
				preparation, err = app.OwnerWardrobePreparation(ctx, "prep-owner", "prep-role")
				if err != nil || preparation["status"] != "queued" || preparation["can_recover"] != false || preparation["error_code"] == "persona_preparation_failed" || len(router.requests) != 0 {
					t.Fatal("completed upstream preparation still blocks wardrobe", preparation, len(router.requests), err)
				}
			})
		}
	}
}

func TestPostgresWardrobePersonaRecoveryKeepsLifecycleAndRevisionGuards(t *testing.T) {
	ctx, repo, app := personaPreparationFixture(t, newFakeProviderRouter())
	original := latestPreparationID(t, ctx, repo)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := repo.Pool().Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	path := "/internal/fluctlights/prep-role/wardrobe/recover"
	recover := func(owner string) (map[string]any, error) {
		return app.DispatchOwnerSurface(ctx, "POST", path, nil, owner, map[string]any{"reason": "恢复准备", "idempotency_key": randomID("recovery_")})
	}
	checkCount := func(want int) {
		t.Helper()
		var count int
		if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='persona.compile'`).Scan(&count); err != nil || count != want {
			t.Fatal("unexpected persona attempts", count, want, err)
		}
	}
	checkNotRecoverable := func() {
		t.Helper()
		out, err := app.OwnerWardrobePreparation(ctx, "prep-owner", "prep-role")
		if err != nil || out["can_recover"] != false {
			t.Fatal("unrelated persona failure exposed wardrobe recovery", out, err)
		}
		out, err = recover("prep-owner")
		if err != nil || out["restarted"] != false {
			t.Fatal("unrelated persona execution restarted", out, err)
		}
	}
	exec(`UPDATE public.platform_workflow_intents SET status='failed',created_at=now()-interval '1 hour' WHERE intent_id=$1`, original)
	if _, err := recover("another-owner"); err == nil {
		t.Fatal("foreign owner recovered upstream preparation")
	}
	exec(`UPDATE public.fluctlights SET status='paused' WHERE id='prep-role'`)
	if _, err := recover("prep-owner"); err == nil {
		t.Fatal("paused character recovered upstream preparation")
	}
	checkCount(1)
	exec(`UPDATE public.fluctlights SET status='active' WHERE id='prep-role'`)
	exec(`UPDATE public.fluctlight_wardrobe_preparations SET status='completed' WHERE fluctlight_id='prep-role'`)
	checkNotRecoverable()
	checkCount(1)
	exec(`UPDATE public.fluctlight_wardrobe_preparations SET status='queued' WHERE fluctlight_id='prep-role'`)
	exec(`UPDATE public.platform_workflow_intents SET payload=jsonb_set(payload,'{expected_revision}','99') WHERE intent_id=$1`, original)
	checkNotRecoverable()
	checkCount(1)
	exec(`UPDATE public.platform_workflow_intents SET payload=jsonb_set(payload,'{expected_revision}','0') WHERE intent_id=$1`, original)
	var latest string
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		var err error
		latest, err = enqueuePersonaPreparationTx(ctx, tx, "prep-owner", "prep-role", 0, "owner_recovery", "newer")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"pending", "retry", "started", "completed"} {
		exec(`UPDATE public.platform_workflow_intents SET status=$2 WHERE intent_id=$1`, latest, status)
		checkNotRecoverable()
		checkCount(2)
	}
	exec(`DELETE FROM public.platform_workflow_intents WHERE intent_type='persona.compile'`)
	checkNotRecoverable()
	checkCount(0)
}
