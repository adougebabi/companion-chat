# Runtime usability recovery — final acceptance report

## Decision

S01-S12 are complete. The runtime is accepted for the requested recovery
scope: recurring WakeUp is PostgreSQL-authoritative and observable, Reflection
is a separate quiet-period process that completed against the configured local
LLM, initialization preserved dense single/multi/complex-card semantics in
live tests, and silent catch-all failures now retain correlated, bounded
operator evidence.

This report covers the uncommitted working tree on branch `master` with baseline
HEAD and `origin/master` at
`627cd7db0d1b18e0da14aacea3292b1dcaacbacc`. It does not claim a commit,
push, or deployment to the user's normal environment.

## Accepted architecture

```text
PostgreSQL stable WakeUp intent / next_attempt_at / cycle
  -> Redis hint or PostgreSQL due sweep
  -> fair dispatcher
  -> Temporal lifecycle-critical Worker lane
  -> Provider queue + correlated model-run attempt
  -> completed_noop | completed_actionable | blocked | retry
  -> cognition_wakeups fact + next due (same transaction)
  -> quiet-period Reflection intent
  -> Temporal lifecycle-critical lane
  -> strict Reflection candidates with Core-owned protocol header
  -> proposal/apply/no-change + watermark
```

`lifecycle-critical` is a capacity lane inside the existing Temporal runtime,
not a second scheduler or queue technology. New WakeUp and Reflection Runs use
it. Their registrations remain on `lifecycle` until pre-isolation histories are
drained, so recorded Task Queue commands continue to replay.

## PRD acceptance mapping

### R1 — WakeUp and Reflection liveness: PASS

Evidence:

- Activation/repair establish stable Schedule and WakeUp intents independently;
  missing Schedule no longer owns WakeUp liveness.
- PostgreSQL owns `next_attempt_at`, status, cycle and release CAS. Redis is a
  hint; exact lost-Redis/restart/dedup PostgreSQL tests pass.
- User activity does not postpone WakeUp and continues to rearm only Reflection.
- New WakeUp/Reflection Runs use two reserved `lifecycle-critical` Activity
  slots. Real contention showed the old shared lane could starve WakeUp; the
  fixed Run received Activity capacity immediately while legacy registration
  remained available.
- The stable test WakeUp intent completed cycle 1 and cycle 2 through real local
  LLM calls, retained the same intent/workflow identity, wrote one fact per
  cycle, emitted next due, and advanced cycle 2 through the PostgreSQL sweep.
- A missing influence sidecar no longer kills recurrence. Unsafe ungrounded
  state capabilities are dropped and the internal cycle persists as
  `completed_noop/capability_influences_missing`; no visible message is
  manufactured.
- WakeUp generated explicit quiet-period Reflection intents. A final real
  Reflection completed as `applied`, advanced watermark 0 to 4, wrote an
  accepted affect-profile revision, and returned its window to idle.
- Retry, lease recovery, no-evidence/no-change and stale revision tests pass.

### R2 — End-to-end diagnostics and no silent catch-all: PASS

Evidence:

- Lifecycle events use stable surfaces/transitions and correlate intent,
  workflow, Run, Activity, Provider attempt, model run, result and next due.
- Cycle 4 produced the ordered trace:
  `trigger_released`, `queued`, `dispatched`, `activity_started`,
  `provider_queued`, `next_cycle_scheduled`, `completed_noop`.
- The WakeUp model run persisted with the cycle correlation, Provider endpoint,
  configured model, attempt identity and latency. The successful Reflection did
  the same and ended in `completed_actionable/applied`.
- A previously invisible diagnostic persistence failure now logged bounded
  `safe_cause` and revealed PostgreSQL SQLSTATE `42P08`. The role parameter is
  explicitly typed, and live WakeUp/Reflection model-run rows now persist.
- Reconciliation skips future pending and retry intents, so not-yet-due
  Reflection is not mislabeled as a failed workflow describe.
- Redaction/collection bounds, event dedupe, workflow links, filter/export,
  overdue audit, model-run attempt separation and sink failure tests pass.
- Initialization diagnostics remain metadata-only; Owner source text is absent
  from ordinary Diagnostics and prompts.

### R3 — Initialization semantic fidelity: PASS

Evidence:

- `response_format` remains `json_object`; no strict full-schema constrained
  decoder was restored.
- Prompt ownership covers base identity, appearance/outfits, background,
  actor_user relationship, single/multi classification, profiles, switching,
  forced activation, voice/habits, cross-profile influence/integration,
  conflicts, special constraints, media preferences/frequencies, daily life,
  goals and intentions.
- Core normalization fills only documented absent values and containers;
  semantic-empty/truncated output remains a typed retryable failure.
- One explicit `analysis_id` selects the latest Owner analysis. Preview edits
  are authoritative activation input, stale analysis is rejected, and the
  original source is immutable/Owner-only.
- Browser/BFF/Core share the 60,000-byte UTF-8 limit.
- Dense single-personality Live Provider test: PASS in 101.53s.
- Dense explicit multi-personality Live Provider test: PASS in 79.61s.
- User-authorized external constructed complex multi-personality card: PASS in
  157.93s with 30 semantic assertions and five invention guards. The 6,991-byte
  source and manifest remained outside the repository with mode 0600.

### R4 — Evidence and rollout discipline: PASS

Evidence:

- S01-S11 ran only stage-specific tests. Full/race/PostgreSQL/Live/Compose
  verification started only in S12.
- Every additional S12 file was added to the explicit allowlist before editing.
- Full Core/Gateway/workspace gates, disposable PostgreSQL, configured local
  Provider and isolated Compose/Temporal acceptance all passed.
- The private/constructed source never entered Git, snapshots, ordinary logs or
  this report. Only size, categories, counts, timings and pass/fail are recorded.
- The protected unrelated
  `.trellis/tasks/09-08-project-health-evolution/` directory was not modified.
- No commit or push occurred.

## Final validation matrix

| Gate | Final result | Notes |
| --- | --- | --- |
| Core `go test ./...` | PASS | Re-run after all S12 Go fixes |
| Core race | PASS | Re-run after all S12 Go fixes |
| Core vet/build | PASS | Re-run after all S12 Go fixes |
| Gateway full/race/vet/build | PASS | Final Gateway code unchanged after its green run |
| gofmt / `git diff --check` | PASS | Final code and specs |
| `pnpm -r test/typecheck/build` | PASS | Full workspace gate |
| `pnpm generate` + drift check | PASS | Final generation/typecheck re-run |
| Disposable PostgreSQL migration | PASS | Dedicated pgvector PostgreSQL 16 |
| PostgreSQL migrations/Core/HTTPAPI | PASS | Serial `-p 1` isolation |
| Focused WakeUp/Reflection/Diagnostics/Source | PASS | Final production code |
| Live initialization, three cards | PASS | 339.493s total |
| Isolated Compose build/up | PASS | Project `lac-runtime-recovery` |
| Platform smoke | PASS | Fresh isolated smoke project |
| Bind sources / Core OpenAPI | PASS | Final artifacts |
| Go projections | PASS | Real BFF/session |
| Active Temporal workflow | PASS | Build `s12-runtime-recovery-final6`, history 11 |
| Real WakeUp cycles | PASS | At least cycles 1 and 2, plus final model-run cycle 4 |
| Real Reflection | PASS | `applied`, watermark 4, correlated model run |

## S12 defects found and closed

1. Shared lifecycle capacity was expansion, not reservation. Fixed with the
   history-compatible `lifecycle-critical` lane.
2. WakeUp treated missing optional influences as a terminal cycle failure.
   Fixed with safe call filtering and an explicit no-op reason.
3. Diagnostic provenance SQL rolled back all model-run rows because of
   ambiguous PostgreSQL parameter inference. Fixed with an explicit role cast;
   bounded failure logs now include the discriminating safe cause.
4. Reflection let a model-owned version alias veto an otherwise valid closed
   shape. Fixed by making protocol version Core authority and summary omission
   a neutral no-change default.
5. Reconciliation inspected future pending Reflection and emitted false
   `workflow_describe_failed`. Fixed by applying the due predicate to pending
   rows as well as retries.

## Security and privacy

- No credentials, card text, unrestricted Provider payload, hidden reasoning,
  cookie, authorization header or database row dump was added to repository
  evidence.
- Diagnostic failure causes are whitespace-normalized, capped at 512 runes and
  replaced with `[REDACTED]` when secret markers are present.
- Missing influences never authorize a state-changing/internal capability.
- Initialization source remains Owner-only and `no-store` at its detail
  boundary.

## Remaining operational notes

- The working tree intentionally remains uncommitted pending explicit user
  authorization.
- The Live initialization suite was not repeated after the final changes
  because those changes were confined to WakeUp/Reflection scheduling,
  normalization and diagnostics; its already-passed Provider prompt/semantic
  code was unchanged. Core full/race/PostgreSQL and Compose were re-run after
  the final Go changes.
- Test-only retries and failure events from the deliberately reproduced broken
  cycles were confined to the isolated Compose database; the task project,
  network and volumes were removed after evidence capture.
- The dedicated PostgreSQL container and all
  `/private/tmp/lac-runtime-recovery*` files/caches were also removed after
  validation. The private constructed card/manifest are therefore no longer
  present on disk.
- `trellis-break-loop` normally requests global template synchronization and an
  immediate commit. Both were withheld because the strict boundary excludes
  template paths and the user did not authorize committing.
