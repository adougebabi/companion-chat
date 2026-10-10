package core

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errLogicalAgentLeaseLost = errors.New("logical_agent_lease_lost")

type logicalAgentLeaseStore interface {
	TryAcquire(context.Context, string, string, string) (bool, error)
	Renew(context.Context, string, string) (bool, error)
	Release(context.Context, string, string) error
	Fence(context.Context, pgx.Tx, string, string) error
}
type postgresLogicalAgentLeases struct{ pool *pgxpool.Pool }

func (s postgresLogicalAgentLeases) TryAcquire(ctx context.Context, owner, token, kind string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `INSERT INTO public.logical_agent_leases(fluctlight_id,owner_token,run_kind,expires_at) VALUES($1,$2,$3,now()+interval '3 minutes') ON CONFLICT(fluctlight_id) DO UPDATE SET owner_token=excluded.owner_token,run_kind=excluded.run_kind,expires_at=excluded.expires_at WHERE logical_agent_leases.expires_at<=now()`, owner, token, kind)
	return err == nil && tag.RowsAffected() == 1, err
}
func (s postgresLogicalAgentLeases) Renew(ctx context.Context, owner, token string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE public.logical_agent_leases SET expires_at=now()+interval '3 minutes' WHERE fluctlight_id=$1 AND owner_token=$2 AND expires_at>now()`, owner, token)
	return err == nil && tag.RowsAffected() == 1, err
}
func (s postgresLogicalAgentLeases) Release(ctx context.Context, owner, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM public.logical_agent_leases WHERE fluctlight_id=$1 AND owner_token=$2`, owner, token)
	return err
}
func (s postgresLogicalAgentLeases) Fence(ctx context.Context, tx pgx.Tx, owner, token string) error {
	var current string
	// Keep only this short commit transaction fenced. No SQL/Actor lock spans
	// a model call; expiry takeover waits for any current local commit to finish.
	if err := tx.QueryRow(ctx, `SELECT owner_token FROM public.logical_agent_leases WHERE fluctlight_id=$1 AND expires_at>now() FOR SHARE`, owner).Scan(&current); err != nil {
		return errLogicalAgentLeaseLost
	}
	if current != token {
		return errLogicalAgentLeaseLost
	}
	return nil
}

type logicalAgentLeaseKey struct{}
type logicalAgentLease struct {
	owner, token string
	store        logicalAgentLeaseStore
}

// enterLogicalAgentRun serializes snapshots→all native model/tool rounds→final
// settlement across processes. The independent physical Provider queue remains
// released between calls, so nested Agents can use it without self-deadlock.
func enterLogicalAgentRun(ctx context.Context, store logicalAgentLeaseStore, owner, kind string) (context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return ctx, nil, context.Cause(ctx)
	}
	if current, ok := ctx.Value(logicalAgentLeaseKey{}).(logicalAgentLease); ok && current.owner == owner {
		return ctx, func() {}, nil
	}
	token := randomID("logical_run_")
	for {
		acquired, err := store.TryAcquire(ctx, owner, token, kind)
		if err != nil {
			return ctx, nil, err
		}
		if acquired {
			break
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx, nil, context.Cause(ctx)
		case <-timer.C:
		}
	}
	child, cancel := context.WithCancelCause(ctx)
	child = context.WithValue(child, logicalAgentLeaseKey{}, logicalAgentLease{owner, token, store})
	done := make(chan struct{})
	stopped := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-child.Done():
				return
			case <-ticker.C:
				renewCtx, stop := context.WithTimeout(child, 5*time.Second)
				valid, err := store.Renew(renewCtx, owner, token)
				stop()
				if err != nil || !valid {
					cancel(errLogicalAgentLeaseLost)
					return
				}
			}
		}
	}()
	release := func() {
		once.Do(func() {
			close(done)
			cancel(nil)
			<-stopped
			releaseCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer stop()
			if err := store.Release(releaseCtx, owner, token); err != nil {
				slog.Warn("logical_agent_release_failed", "fluctlight_id", owner)
			}
		})
	}
	return child, release, nil
}
func (a *App) enterLogicalRun(ctx context.Context, owner, kind string) (context.Context, func(), error) {
	if a != nil && a.logicalAgentLeases != nil {
		return enterLogicalAgentRun(ctx, a.logicalAgentLeases, owner, kind)
	}
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return ctx, func() {}, ctx.Err()
	}
	return enterLogicalAgentRun(ctx, postgresLogicalAgentLeases{a.DB.Pool()}, owner, kind)
}

func (a *App) runBackgroundLogicalWork(ctx context.Context, owner, kind string, work func(context.Context) (map[string]any, error)) (map[string]any, error) {
	runCtx, release, err := a.enterLogicalRun(ctx, owner, kind)
	if err != nil {
		return nil, err
	}
	defer release()
	result, err := work(runCtx)
	if err != nil {
		if cause := context.Cause(runCtx); cause != nil {
			return result, cause
		}
	}
	return result, err
}
func (a *App) logicalRunOwner(ctx context.Context, inbox string) (string, error) {
	var owner string
	err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id FROM public.cognition_inbox WHERE id=$1`, inbox).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return owner, err
}
func fenceLogicalAgentRun(ctx context.Context, tx pgx.Tx) error {
	if lease, ok := ctx.Value(logicalAgentLeaseKey{}).(logicalAgentLease); ok {
		return lease.store.Fence(ctx, tx, lease.owner, lease.token)
	}
	return nil
}
