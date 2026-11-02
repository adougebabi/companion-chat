package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestWardrobeInitializationRecoveryKeepsDomainGuardsAndStableCheckpoint(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	var preparation string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		preparation, err = ensureWardrobePreparationTx(f.ctx, tx, f.fluctlightID, "recovery")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	path := "/internal/fluctlights/" + f.fluctlightID + "/wardrobe/recover"
	command := func() map[string]any {
		return map[string]any{"reason": "retry", "idempotency_key": randomID("recover_")}
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.repository.Pool().Exec(f.ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range []string{"pending", "retry", "started", "completed"} {
		exec(`UPDATE public.platform_workflow_intents SET status=$2 WHERE intent_id=$1`, "wardrobe_initialize:"+f.fluctlightID, status)
		out, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, command())
		if err != nil || out["restarted"] != false || out["status"] != status {
			t.Fatal("existing execution reopened", status, out, err)
		}
	}
	exec(`UPDATE public.platform_workflow_intents SET status='cancelled' WHERE intent_id=$1`, "wardrobe_initialize:"+f.fluctlightID)
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, command()); err == nil {
		t.Fatal("cancelled task reopened")
	}
	exec(`UPDATE public.platform_workflow_intents SET status='failed',attempt_count=3,last_error='original failure' WHERE intent_id=$1`, "wardrobe_initialize:"+f.fluctlightID)
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.foreignOwnerID, command()); err == nil {
		t.Fatal("foreign owner recovered task")
	}
	exec(`UPDATE public.fluctlights SET status='paused' WHERE id=$1`, f.fluctlightID)
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, command()); err == nil {
		t.Fatal("paused character recovered task")
	}
	exec(`UPDATE public.fluctlights SET status='active' WHERE id=$1`, f.fluctlightID)
	for _, body := range []map[string]any{
		{"reason": " ", "idempotency_key": "blank"},
		{"reason": strings.Repeat("r", 501), "idempotency_key": "long-reason"},
		{"reason": "retry", "idempotency_key": strings.Repeat("k", 257)},
		{"reason": "retry", "idempotency_key": "injected", "actor_id": f.ownerID},
	} {
		if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body); err == nil {
			t.Fatal("invalid recovery accepted", body)
		}
	}
	exec(`UPDATE public.fluctlight_wardrobe_preparations SET status='completed' WHERE id=$1`, preparation)
	out, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, command())
	if err != nil || out["restarted"] != false {
		t.Fatal("completed preparation reopened", out, err)
	}
	exec(`DELETE FROM public.platform_workflow_intents WHERE intent_id=$1`, "wardrobe_initialize:"+f.fluctlightID)
	var missing *SurfaceError
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, command()); !errors.As(err, &missing) || missing.Status != 404 || missing.Code != "owner_object_not_found" {
		t.Fatal("missing initializer fabricated", err)
	}
	f.requireNoModelRuns(t)
}
