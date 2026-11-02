package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type ownerRequestClaimKey struct{}
type ownerRequestClaim struct {
	InboxID, Owner, Token string
}

var errOwnerRequestClaimLost = errors.New("owner_request_claim_lost")

// The independent Owner-request lane uses its existing inbox claim as the
// execution fence. Lock Life first, matching admission and Tool settlement,
// then hold the claim only for this short transaction. Provider work stays
// outside SQL transactions and never holds the chat logical lease.
func fenceOwnerRequestClaimTx(ctx context.Context, tx pgx.Tx) error {
	claim, ok := ctx.Value(ownerRequestClaimKey{}).(ownerRequestClaim)
	if !ok {
		return nil
	}
	if err := lockLifeContextTx(ctx, tx, claim.Owner); err != nil {
		return err
	}
	var valid bool
	err := tx.QueryRow(ctx, `SELECT status='claimed' AND claimed_by=$2 AND claimed_at>now()-interval '10 minutes' FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$3 FOR SHARE`, claim.InboxID, claim.Token, claim.Owner).Scan(&valid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !valid) {
		return errOwnerRequestClaimLost
	}
	return err
}
