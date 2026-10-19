package core

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Called only after the existing executor has observed Temporal's terminal
// status. A crashed process cannot run its defer; close both product rows so
// the normal Owner retry command remains available.
func (a *App) ReconcileDiaryWorkflowTerminal(ctx context.Context, intentID, terminalStatus string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var id, diary, status string
		var attempt, latest int
		err := tx.QueryRow(ctx, `SELECT g.id,g.diary_id,g.status,g.attempt,d.latest_attempt FROM public.platform_workflow_intents i JOIN public.fluctlight_diary_attempts g ON g.id=i.payload->>'diary_attempt_id' JOIN public.fluctlight_diary_days d ON d.id=g.diary_id WHERE i.intent_id=$1 AND i.intent_type='diary.generate' FOR UPDATE OF g,d`, intentID).Scan(&id, &diary, &status, &attempt, &latest)
		if err != nil {
			return err
		}
		if status != "pending" && status != "running" {
			return nil
		}
		code := "diary_workflow_" + terminalStatus
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_diary_attempts SET status='failed',error_code=$2,finished_at=now() WHERE id=$1`, id, code); err != nil {
			return err
		}
		if latest == attempt {
			_, err = tx.Exec(ctx, `UPDATE public.fluctlight_diary_days SET status='failed',last_error_code=$2,revision=revision+1,updated_at=now() WHERE id=$1 AND status IN ('pending','running')`, diary, code)
		}
		return err
	})
}

// Reuse completed stages only after the source window is revalidated. A failed
// model run itself gets a fresh attempt/run identity; a completed body is never
// regenerated merely because the final transaction was interrupted.
func copyDiaryRetryStagesTx(ctx context.Context, tx pgx.Tx, fid, id string, previous, next int) error {
	var snapshot, plan, persona, models []byte
	var body *string
	if err := tx.QueryRow(ctx, `SELECT source_snapshot,plan,body,persona_snapshot,model_provenance FROM public.fluctlight_diary_attempts WHERE diary_id=$1 AND attempt=$2`, id, previous).Scan(&snapshot, &plan, &body, &persona, &models); err != nil {
		return err
	}
	window := decodeObject(snapshot)
	if len(window) == 0 || len(decodeObject(persona)) == 0 {
		return nil
	}
	start, e1 := time.Parse(time.RFC3339Nano, stringValue(window["window_start"]))
	end, e2 := time.Parse(time.RFC3339Nano, stringValue(window["window_end"]))
	asOf, e3 := time.Parse(time.RFC3339Nano, stringValue(window["as_of"]))
	if e1 != nil || e2 != nil || e3 != nil {
		return nil
	}
	current, err := readDiarySourcesWith(ctx, tx, fid, start, end, asOf)
	if err != nil {
		return err
	}
	if stableDigest(jsonString(current)) != stableDigest(jsonString(window)) {
		return nil
	}
	nextID := fmt.Sprintf("%s_attempt_%d", id, next)
	provenance := decodeObject(models)
	provenance["reused_from_attempt_id"] = fmt.Sprintf("%s_attempt_%d", id, previous)
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_diary_attempts(id,diary_id,attempt,status,source_snapshot,source_digest,plan,body,persona_snapshot,model_provenance) VALUES($1,$2,$3,'pending',$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`, nextID, id, next, snapshot, stableDigest(jsonString(window)), plan, body, persona, jsonBytes(provenance))
	return err
}
