# Reflection / diagnostics / migration DB regressions

Date: 2026-09-22

Scope: broad isolated PostgreSQL regression follow-up for Reflection memory and
Goal/Outcome authority, Provider terminal diagnostics, the current migration
head, and the scene/presence/schedule independent Tool expectations. All runs
removed `FLUCTLIGHT_LIVE_PROVIDER_*`. Controlled HTTP and isolated PostgreSQL
were allowed; no real LLM or GPU request was sent.

## Implemented corrections

- Reflection's Provider schema now matches the existing operation-specific
  domain contract while remaining closed:
  - durable Memory `create`, `revise`, `merge`, and `supersede` require the
    complete semantic shape;
  - durable Memory `confirm` and `deprecate` require target/evidence/reason and
    reject semantic fields;
  - Active Memory `create`, `revise`, and `supersede` require the complete
    semantic shape;
  - Active Memory `confirm`, `complete`, and `expire` require
    target/evidence/reason and reject semantic fields.
  The shared top-level candidate schema remains `additionalProperties:false`,
  and each disjoint operation branch is also closed. No default repair or
  relaxed final validation was introduced.
- Reflection's task-owned evidence projection now adds an opaque
  `outcome:ctx_*` reference only when the current `ContextReferenceIndex`
  contains the authoritative Outcome entity. Raw Outcome IDs and storage
  metadata remain absent. This lets Goal candidates bind `target_ref` and
  `outcome_refs` to the same frozen projection that Core later validates.
- The malformed Reflection rollback fixture now expects the strict final schema
  error before domain compilation. It still proves that proposal rows, Memory,
  revisions, governance, embedding intents, outbox rows, and the reflection
  watermark all remain unchanged.
- Provider failure diagnostics use a two-second
  `context.WithTimeout(context.WithoutCancel(ctx), ...)` write context. This
  preserves scenario, physical attempt identity and prompt diagnostics after a
  caller cancellation while bounding terminal persistence. Model-run state
  updates use the same bounded value-preserving context. Existing first-terminal
  wins behavior remains unchanged.
- The initialization-source migration regression now expects the actual
  `0034_tool_execution_source` head and proves current-head reruns are
  idempotent. A new real PostgreSQL `0033_initialization_source` → `0034`
  upgrade test seeds an immutable initialization source, runs the upgrade twice,
  verifies its text/digests/projection are unchanged, and verifies the three
  Tool/Agent execution tables exist.
- Independent scene and presence business-failure assertions now require the
  actual typed codes `scene_start_requires_no_active_event` and
  `presence_expiration_invalid`. Dependency-failure expectations remain
  separate. The real `schedule.replan` Tool test skips ordinary development
  runs unless `FLUCTLIGHT_LIVE_PROVIDER_TEST=1`; it remains reserved for the
  final serial live suite.

## Verification evidence

- Targeted PostgreSQL regression:
  `reflection-db-final-targeted2-170523`
  - command exit: `0`
  - passing test events: `23`
  - failures: `0`
  - skips: `1` (`TestIndependentToolE2EScheduleReplan`, live flag absent)
- Broader owning Reflection/diagnostics regression:
  `reflection-owning-broad-170607`
  - command exit: `0`
  - passing test events: `57`
  - failures: `0`
  - skips: `2`
  - expected live-only skips: `TestIndependentToolE2EScheduleReplan` and
    `TestLiveWakeUpRealToolCallsReachDurableActionAndReflection`
- Full migrations package:
  `reflection-migrations-full-165535`
  - command exit: `0`
  - passing test events: `76`
  - failures/skips: `0/0`
- Goal/Outcome focused rerun:
  `reflection-goal-fix-170357`
  - command exit: `0`
  - passing test events: `1`
  - failures/skips: `0/0`
- Owning vet:
  `reflection-owning-vet-170642`
  - `go -C apps/core-go vet ./internal/core ./internal/migrations`
  - command exit: `0`
- Production package build:
  `go -C apps/core-go build ./internal/core ./internal/migrations`
  - command exit: `0`

Every named run above is under
`.trellis/tasks/09-22-agent-tool-plugin-e2e/research/runs/<run>/` and contains a
redacted `output.log` plus `meta.json` with the command, exit code, HEAD, Go
source hashes, duration, and `live_provider_enabled:false`.

## Remaining verification

- `schedule.replan` still requires the user-deferred final serial live Provider
  run. Its ordinary isolated DB regression is intentionally a Go `SKIP`; the
  strict live acceptance runner must continue treating a required skipped row
  as non-success.
- The live WakeUp/Reflection chain was not run in this slice and is not counted
  as covered by controlled tests.
- No production migration SQL, result adapter, persona/visual code, or
  acceptance runner was changed by this slice.
