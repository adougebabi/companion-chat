package core

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEnsureWakeUpIntentsRepairsExistingLiveFluctlight(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	fixtureKey := stableDigest(t.Name() + time.Now().UTC().Format(time.RFC3339Nano))
	fluctlightID := "test-wakeup-intent-" + fixtureKey
	intentID := "wake_up_intent:" + fluctlightID
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	localDate := time.Now().In(location).Format("2006-01-02")
	_, err = pool.Exec(ctx, `
		INSERT INTO public.fluctlights(
			id,created_by_actor_id,initialization_mode,status,identity,personality,
			behavioral_policy,life_profile,provenance
		) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}')
		ON CONFLICT (id) DO UPDATE SET status='active'`, fluctlightID, "test-owner")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := "test-schedule-" + fixtureKey
	scheduleResult := jsonBytes(map[string]any{"id": scheduleID, "status": "accepted", "revision": 1, "expected_context_revision": "life_ctx_before", "resulting_context_revision": "life_ctx_after", "replayed": false})
	_, err = pool.Exec(ctx, `
		INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,idempotency_key,request_digest,result)
		VALUES($1,$2,$3,'Asia/Shanghai','accepted','test','[]',1,$1,$4,$5)
		ON CONFLICT (id) DO NOTHING`, scheduleID, fluctlightID, localDate, stableDigest(scheduleID), scheduleResult)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.life_schedules WHERE fluctlight_id=$1`, fluctlightID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.fluctlights WHERE id=$1`, fluctlightID)
	})

	app := &App{DB: &PostgresRepository{pool: pool}}
	count, err := app.EnsureWakeUpIntents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Fatalf("EnsureWakeUpIntents() count = %d, want at least one", count)
	}

	var intentType, status string
	var payload []byte
	if err := pool.QueryRow(ctx, `SELECT intent_type,status,payload FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&intentType, &status, &payload); err != nil {
		t.Fatal(err)
	}
	if intentType != "wake_up.current" || status != "pending" || string(payload) == "" {
		t.Fatalf("wake-up intent = type %q, status %q, payload %s", intentType, status, payload)
	}

	if _, err := pool.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',last_error='test-failure',completed_at=now() WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "retry" {
		t.Fatalf("repaired wake-up intent status = %q, want retry", status)
	}

}
