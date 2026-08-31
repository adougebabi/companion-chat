package core

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func appendOutboxTx(ctx context.Context, tx pgx.Tx, kind, aggregateType, aggregateID, fluctlightID, causationID, correlationID, idempotency string, payload any) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.platform_outbox_events(id,kind,aggregate_type,aggregate_id,fluctlight_id,causation_id,correlation_id,idempotency_key,payload,attempt_policy) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'{}') ON CONFLICT (idempotency_key) DO NOTHING`, "outbox_"+stableDigest(idempotency), kind, aggregateType, aggregateID, nullableString(fluctlightID), causationID, correlationID, idempotency, jsonBytes(payload))
	return err
}
