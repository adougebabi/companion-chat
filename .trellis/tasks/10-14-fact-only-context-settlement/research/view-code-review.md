# Independent fact-only view code review

Read-only review of the three new view helpers and their context assembly integrations, plus Goal dependency and active-life metadata reads. No tests, DB, network, or product writes performed.

## P1 — Production Actor key mismatch removes the current relationship

`apps/core-go/internal/core/fact_only_context_views.go:64` compares each relationship's nonempty `target_actor_id` against `projection.CurrentSpeaker["id"]`. Production speaker maps use **`actor_id`**, not `id`: `intelligence.go:530` constructs self that way, `intelligence.go:549` constructs additional Actors that way, and `provider_context.go:1597`–1607 resolves them using `actor_id`. A normal human conversation therefore has an empty right-hand comparison; every real relationship is skipped. Both cognition and Writer lose relationship summary, role/addressing, trend, and emotional associations. This directly violates the requirement to preserve effective visible relationships.

The same incorrect key at `fact_only_context_views.go:42` also skips the helper's recipient Goal filter. Normal conversation projection construction already applies an upstream filter (`intelligence.go:263`–264), so broad Goal leakage is mitigated in that path; do not overstate this secondary consequence. The helper should nevertheless use the production key consistently.

The new synthetic fixture masks the issue: `testdata/fact_only_context/reported_morning.synthetic.json:53` supplies `current_speaker.id`, and its Actor rows also use `id`. Make its Actor shape match `buildActorProjection`, and cover retention of the current recipient's relationship with `CurrentSpeaker: {actor_id: ...}`. Existing synthetic tests currently establish behavior against a shape the production builder does not emit.

## Other checked areas

History serialization separates metadata after final selection and preserves actual role/content; recent groups carry required protection. Backfill SQL is constrained to the same conversation and exact source fact rather than an arbitrary preceding user. Fact-only Runtime fragments are required and use unbounded copies so wire-budget rejection can enforce the limit. Goals preserve original criteria while excluding strategy strings, and completed rows are reduced. Moments keeps its separate working-input branch. No additional actionable P1/P2 defect was established within this bounded review.
