package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func requireOwnerMutableTx(ctx context.Context, tx pgx.Tx, owner, fid string) error {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR SHARE`, fid, owner).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == "retired" {
		return retiredOwnerWriteError()
	}
	return nil
}

func retiredOwnerWriteError() *SurfaceError {
	return &SurfaceError{Status: 409, Code: "fluctlight_retired", Message: "已退休角色只读，未保存修改"}
}
