package core

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

// A domain mutation rechecks the same authoritative Event/Schedule snapshot,
// under the Life lock. No text, Actor occupation or Tool name is interpreted.
func requireAwakeLifeTx(ctx context.Context, tx pgx.Tx, fluctlightID string, at time.Time) error {
	if err := lockLifeContextTx(ctx, tx, fluctlightID); err != nil {
		return err
	}
	_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, at)
	if err != nil {
		return err
	}
	if stringValue(life["behavior_state"]) == "sleep" {
		return newCapabilityError("life_state_sleeping", false, ErrConflict)
	}
	return nil
}
