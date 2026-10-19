package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Legacy callers keep their signature, sharing the same snapshot writer and
// durable replay rather than retaining a second, incomplete rollback path.
func (a *App) legacyOwnerSelfCommand(ctx context.Context, owner, id, operation string, expected int, reason string) (map[string]any, error) {
	if expected < 1 {
		return nil, ErrInvalidArguments
	}
	var fid string
	err := a.DB.Pool().QueryRow(ctx, `SELECT c.fluctlight_id FROM public.fluctlight_developing_self_claims c JOIN public.fluctlights f ON f.id=c.fluctlight_id WHERE c.id=$1 AND f.created_by_actor_id=$2`, id, owner).Scan(&fid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	body := map[string]any{"expected_revision": expected, "reason": reason}
	if operation == "rollback" {
		body["target_revision"] = expected - 1
	}
	digest := stableDigest(jsonString([]any{owner, fid, id, operation, body}))
	key := "self-legacy:" + digest
	var result map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
			return err
		}
		store := postgresSurfaceTx{tx}
		previous, cached, found, err := store.Idempotency(ctx, owner, key)
		if err != nil {
			return err
		}
		if found {
			if previous != digest {
				return ErrConflict
			}
			result = cached
			result["replayed"] = true
			return nil
		}
		result, err = a.OwnerSelfCommand(context.WithValue(ctx, surfaceDBTxKey{}, tx), owner, fid, id, operation, body)
		if err != nil {
			return err
		}
		if operation == "rollback" {
			result["status"] = "rolled_back"
		}
		result["replayed"] = false
		if err := store.SaveIdempotency(ctx, owner, key, digest, result); err != nil {
			return fmt.Errorf("self command result persistence: %w", err)
		}
		return nil
	})
	return result, err
}
