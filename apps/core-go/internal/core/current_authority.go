package core

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// A domain write advances one Fluctlight-scoped generation in the same
// transaction. This version covers current state, derived memory and source
// corrections without scanning or locking all historical authority rows.
var ErrCurrentFactsStale = errors.New("current_facts_stale")

type currentAuthorityReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readCurrentFactsRevisionWith(ctx context.Context, query currentAuthorityReader, fluctlightID string) (string, error) {
	var generation int64
	if err := query.QueryRow(ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id=$1`, fluctlightID).Scan(&generation); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return "facts_gen_" + strconv.FormatInt(generation, 10), nil
}

func (a *App) readCurrentFactsRevision(ctx context.Context, fluctlightID string) (string, error) {
	tx, err := a.DB.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	revision, err := readCurrentFactsRevisionWith(ctx, tx, fluctlightID)
	if err != nil {
		return "", err
	}
	return revision, tx.Commit(ctx)
}

func (a *App) requireCurrentFactsRevisionTx(ctx context.Context, tx pgx.Tx, fluctlightID, expected string) error {
	if strings.TrimSpace(expected) == "" {
		return errors.New("current_facts_revision_required")
	}
	// Every relevant writer updates this row before its own commit. Holding
	// the row through settlement closes the read/commit race without locking
	// Memory, wardrobe, schedule or raw history rows individually.
	var generation int64
	if err := tx.QueryRow(ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&generation); err != nil {
		return err
	}
	if "facts_gen_"+strconv.FormatInt(generation, 10) != expected {
		return ErrCurrentFactsStale
	}
	return nil
}
