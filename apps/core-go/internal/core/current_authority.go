package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// A domain write advances one Fluctlight-scoped generation in the same
// transaction. This version covers current state, derived memory and source
// corrections without scanning or locking all historical authority rows.
var ErrCurrentFactsStale = errors.New("current_facts_stale")

type currentFactsMismatch struct {
	Expected, Actual, Boundary string
}

func (e *currentFactsMismatch) Error() string { return ErrCurrentFactsStale.Error() }
func (e *currentFactsMismatch) Unwrap() error { return ErrCurrentFactsStale }

const currentFactsGenerationSourceLimit = int64(512)

type currentFactsGenerationSource struct {
	Table     string
	Operation string
	Count     int64
}

func currentFactsGenerationNumber(revision string) (int64, error) {
	const prefix = "facts_gen_"
	revision = strings.TrimSpace(revision)
	if !strings.HasPrefix(revision, prefix) {
		return 0, fmt.Errorf("current facts revision %q has invalid prefix", revision)
	}
	generation, err := strconv.ParseInt(strings.TrimPrefix(revision, prefix), 10, 64)
	if err != nil || generation < 0 {
		return 0, fmt.Errorf("current facts revision %q has invalid generation", revision)
	}
	return generation, nil
}

func currentFactsGenerationIntervalBounds(expected, actual string) (int64, int64, error) {
	expectedGeneration, err := currentFactsGenerationNumber(expected)
	if err != nil {
		return 0, 0, err
	}
	actualGeneration, err := currentFactsGenerationNumber(actual)
	if err != nil {
		return 0, 0, err
	}
	if expectedGeneration > actualGeneration {
		expectedGeneration, actualGeneration = actualGeneration, expectedGeneration
	}
	return expectedGeneration, actualGeneration, nil
}

func currentFactsGenerationCoverageComplete(lower, upper, observed, minimum, maximum int64) bool {
	if lower == upper {
		return observed == 0
	}
	return observed == upper-lower && minimum == lower+1 && maximum == upper
}

// currentFactsGenerationProvenance reads only the bounded journal interval
// that existed at the mismatch boundary. A later failure-recording mutation
// may advance the generation again, but can never enter this closed upper
// bound. Query failure is diagnostic-only and must not replace the stale CAS.
func (a *App) currentFactsGenerationProvenance(ctx context.Context, fluctlightID, expected, actual string) (map[string]any, error) {
	lower, upper, err := currentFactsGenerationIntervalBounds(expected, actual)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"available": false, "complete": false,
		"from_generation": lower, "to_generation": upper,
		"observed_count": int64(0), "sources": []any{},
	}
	if upper == lower {
		result["available"] = true
		result["complete"] = true
		return result, nil
	}
	rows, err := a.DB.Pool().Query(ctx, `
		WITH bounded AS (
		 SELECT generation,source_table,source_operation
		 FROM public.fluctlight_context_generation_journal
		 WHERE fluctlight_id=$1 AND generation>$2 AND generation<=$3
		 ORDER BY generation DESC LIMIT $4
		), grouped AS (
		 SELECT source_table,source_operation,count(*)::bigint AS source_count,
		        min(generation) AS min_generation,max(generation) AS max_generation
		 FROM bounded GROUP BY source_table,source_operation
		)
		SELECT source_table,source_operation,source_count,
		       (sum(source_count) OVER ())::bigint,min(min_generation) OVER (),max(max_generation) OVER ()
		FROM grouped ORDER BY source_table,source_operation`, fluctlightID, lower, upper, currentFactsGenerationSourceLimit)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	result["available"] = true
	sources := make([]any, 0)
	var observed, minimum, maximum int64
	for rows.Next() {
		var source currentFactsGenerationSource
		if err := rows.Scan(&source.Table, &source.Operation, &source.Count, &observed, &minimum, &maximum); err != nil {
			return result, err
		}
		sources = append(sources, map[string]any{
			"table": source.Table, "operation": source.Operation, "count": source.Count,
		})
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result["sources"] = sources
	result["observed_count"] = observed
	result["complete"] = currentFactsGenerationCoverageComplete(lower, upper, observed, minimum, maximum)
	return result, nil
}

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
		return &currentFactsMismatch{Expected: expected, Actual: "facts_gen_" + strconv.FormatInt(generation, 10), Boundary: "settlement"}
	}
	return nil
}
