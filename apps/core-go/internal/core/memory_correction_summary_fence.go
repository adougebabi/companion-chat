package core

import "context"

// A Memory correction does not rewrite the original conversation. Until a
// summary can prove it incorporated that correction, do not use a summary
// whose covered messages include a prior revision's typed source. Earlier
// links also cover old confirm revisions that did not preserve typed identity.
// A rebuild from the same raw messages cannot silently make the summary
// trusted again. This is a conservative read fence, not a replacement for
// corrected-summary rebuilding.
func summaryContainsCorrectedMemorySource(ctx context.Context, q lifeContextQuerier, owner, conversation string, from, to int) (bool, error) {
	var blocked bool
	err := q.QueryRow(ctx, `SELECT public.summary_contains_corrected_memory_source($1,$2,$3,$4)`, owner, conversation, from, to).Scan(&blocked)
	return blocked, err
}
