package core

import (
	"context"
	"os"
	"testing"

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

	fluctlightID := "test-wakeup-intent-" + stableDigest(t.Name())
	intentID := "wake_up_intent:" + fluctlightID
	_, err = pool.Exec(ctx, `
		INSERT INTO public.fluctlights(
			id,created_by_actor_id,initialization_mode,status,identity,personality,
			behavioral_policy,life_profile,provenance
		) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}')
		ON CONFLICT (id) DO UPDATE SET status='active'`, fluctlightID, "test-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID)
		_, _ = pool.Exec(ctx, `DELETE FROM public.fluctlights WHERE id=$1`, fluctlightID)
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
