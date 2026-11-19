package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Only a transaction that actually adds an attachment can certify its time.
// An existing reference, even with unknown time, remains immutable on replay.
func recordMediaPublicationTx(ctx context.Context, tx pgx.Tx, id, asset, fid, targetType, target string, newlyAttached bool) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id,published_at) VALUES($1,$2,$3,$4,$5,CASE WHEN $6 THEN clock_timestamp() ELSE NULL END) ON CONFLICT DO NOTHING`, id, asset, fid, targetType, target, newlyAttached)
	return err
}

func mediaPublicationTimeIdentity(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}
