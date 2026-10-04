package core

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

// A periodic check during sleep is operational bookkeeping. It produces no
// cognition fact, action outcome, reflection input or physiological event.
func (a *App) persistSleepingCycle(ctx context.Context, wakeID, fluctlightID string, cycle, interval int) (map[string]any, error) {
	result := map[string]any{"status": "no_op", "reason": "sleeping", "trigger_source": "periodic_check"}
	var nextDue time.Time
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fluctlightID); err != nil {
			return err
		}
		current, err := wakeUpExecutionCurrentTx(ctx, tx, cycle)
		if err != nil {
			return err
		}
		if !current {
			return errLifecycleSupersededByCognition
		}
		if err := lockLifeContextTx(ctx, tx, fluctlightID); err != nil {
			return err
		}
		_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, a.now().UTC())
		if err != nil {
			return err
		}
		if stringValue(life["behavior_state"]) != "sleep" {
			return ErrLifeContextStale
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_wakeups(id,fluctlight_id,cycle,internal_dynamics,attention,thought,desire,agency,action_type,result,status) VALUES($1,$2,$3,'{}','{}','{}','{}','{}','no_op',$4,'no_op') ON CONFLICT(id) DO NOTHING`, wakeID, fluctlightID, cycle, jsonBytes(result)); err != nil {
			return err
		}
		nextDue, err = updateWakeUpNextDueTx(ctx, tx, fluctlightID, cycle, interval, a.now().UTC())
		return err
	})
	if err != nil {
		return nil, err
	}
	a.scheduleWakeUpHint(ctx, fluctlightID, cycle, nextDue)
	return map[string]any{"wake_up_id": wakeID, "fluctlight_id": fluctlightID, "cycle": cycle, "status": "no_op", "reason": "sleeping", "action_type": "no_op", "result": result, "next_due_at": formatInstant(nextDue)}, nil
}
