package core

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

func TestR9HeartbeatSerializationKeepsSQLCauseAndRetriesOnlyRolledBackMutation(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	ctx, release, err := f.app.enterLogicalRun(f.ctx, f.fluctlightID, "r9-fence")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	lease := ctx.Value(logicalAgentLeaseKey{}).(logicalAgentLease)
	renew := func() {
		t.Helper()
		ok, err := lease.store.Renew(ctx, lease.owner, lease.token)
		if err != nil || !ok {
			t.Fatal(ok, err)
		}
	}
	err = withTransaction(ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT 1`); err != nil {
			return err
		}
		renew()
		return nil
	})
	var pgerr *pgconn.PgError
	if errors.Is(err, errLogicalAgentLeaseLost) || !errors.As(err, &pgerr) || pgerr.Code != "40001" {
		t.Fatal("database cause masked as expired lease", err)
	}
	calls := 0
	err = withToolMutationTransaction(ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		calls++
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('r9-one-commit','{"v":1}')`); err != nil {
			return err
		}
		if calls == 1 {
			renew()
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatal("heartbeat conflict did not retry whole transaction", calls, err)
	}
	var rows int
	if err := f.repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.runtime_settings WHERE key='r9-one-commit'`).Scan(&rows); err != nil || rows != 1 {
		t.Fatal(rows, err)
	}
	attempts := 0
	err = withToolMutationTransaction(ctx, f.repository.Pool(), func(pgx.Tx) error { attempts++; return errLogicalAgentLeaseLost })
	if attempts != 1 || !errors.Is(err, errLogicalAgentLeaseLost) {
		t.Fatal("retried actual lease loss", attempts, err)
	}
}
