# T05 Cognition / Inbox / Diagnostics Implementation Brief

## Status

Parent-authorized implementation brief for the first executable child in the
T05-T12 chain. This child produces implementation evidence only; T12 remains
the acceptance owner.

## Dependency

T04 public Fluctlight and inner-state contracts are available in the current
workspace. Existing T03/T04 worktree changes are inputs and are not rewritten
by this child; missing upstream integration is a T12 carry-forward risk.

## Owned Paths

- `apps/core/src/fluctlight_core/cognition/**`
- `apps/core/src/fluctlight_core/diagnostics/**`
- `apps/core/src/fluctlight_core/transport/diagnostics.py`
- `apps/core/src/fluctlight_core/entrypoints/cognition_worker.py`
- `apps/core/src/fluctlight_core/platform/cognition_events.py`
- `apps/core/migrations/versions/0004_t05_cognition_diagnostics.py`
- `apps/core/tests/cognition/**`
- `apps/core/tests/diagnostics/**`
- `apps/core/tests/contract/test_t05_*.py`
- `apps/core/tests/architecture/test_t05_*.py`

For this serialized child, the following shared files may be changed only for
T05 registration: `apps/core/migrations/env.py`,
`apps/core/src/fluctlight_core/transport/api.py`,
`apps/core/src/fluctlight_core/entrypoints/worker.py`, and
`apps/core/src/fluctlight_core/platform/schema.py`. T05 domain tables remain
owned by the new modules.

## Forbidden Paths

- Frozen legacy `server/**`, `web/**`, `test/**`, `compose.yaml`, and the root
  legacy Node entrypoint/dependencies.
- T06-T12 domain modules (`conversations`, `memory`, `relationships`,
  `life_world`, `moments`, `media`) except for framework-free Protocols at the
  T05 cognition boundary.
- Provider SDK calls from cognition domain code, direct SQL reads of another
  module's tables, and any new queue/workflow runtime.

## Decisions And Contracts

Implement without changing D002, D012-D016, D019, D020, D025, D028-D033, or
D039. The assigned contracts are:

- `fluctlight-cognitive-runtime.md`: LLM-owned semantics, two-stage turn,
  ordered cognitive writer, reflection watermark/CAS, and explicit failure.
- `fluctlight-diagnostics-contract.md`: isolated redacted diagnostics sink,
  bounded retention, Owner query/export, and stdout fallback.
- `fluctlight-provider-contract.md`: explicit role/provenance ports; no
  implicit semantic fallback.
- `fluctlight-workflow-contract.md`: committed intents, stable IDs, and
  Temporal-only execution.
- `fluctlight-persistence-contract.md` and `fluctlight-event-contract.md`:
  short UoW phases, outbox/inbox idempotency, and commit-before-ack ordering.

## Implementation Checklist

1. Add framework-free cognition contracts for assessment, decision proposal,
   frozen action, realization, reflection evidence and provider ports.
2. Add T05-owned PostgreSQL tables and services for per-Fluctlight sequenced
   inbox claims, assessment/decision/action records and reflection watermark.
3. Implement short transaction phases: claim -> external assessment port -> CAS
   apply/freeze -> external realization port -> action result/delivery.
4. Add diagnostics tables, typed redaction, query/export/clear services and
   bounded retention without coupling sink failure to business transactions.
5. Add a deterministic in-process cognitive worker adapter and a Temporal
   workflow registration seam; provider/network calls remain injected ports.
6. Add contract, architecture, idempotency, CAS, redaction and retention
   tests. These are implementation evidence, not acceptance.

## Implementation Checks

Run only these local checks while T05 is active:

```bash
uv run --offline ruff format --check apps/core/src/fluctlight_core/cognition apps/core/src/fluctlight_core/diagnostics apps/core/tests/cognition apps/core/tests/diagnostics
uv run --offline ruff check apps/core/src/fluctlight_core/cognition apps/core/src/fluctlight_core/diagnostics apps/core/tests/cognition apps/core/tests/diagnostics
uv run --offline pytest -q apps/core/tests/cognition apps/core/tests/diagnostics apps/core/tests/contract/test_t05_cognition.py apps/core/tests/contract/test_t05_diagnostics.py
```

These results are evidence only. Real PostgreSQL, provider failure, Redis
replay, Temporal restart, security/redaction and cross-module scenarios are
T12 coverage.

## T12 Coverage IDs

T05 hands off: `T05-CGN-01` ordered per-Fluctlight inbox and cross-Fluctlight
concurrency; `T05-CGN-02` LLM-first assessment/decision with explicit provider
failure; `T05-CGN-03` CAS/frozen action/realization idempotency;
`T05-CGN-04` reflection watermark and stale revision rejection;
`T05-DIA-01` diagnostics correlation and typed redaction; `T05-DIA-02`
Owner-only query/export/clear; `T05-DIA-03` retention limits and sink-failure
isolation; `T05-EVT-01` commit-before-ack inbox replay.

## Rollback Point

Before T06 starts, revert only T05-owned paths and migration `0004` if this
child cannot produce a coherent handoff. Do not reset the shared worktree or
remove pre-existing T03/T04 edits.

## Implementation Evidence Handoff

Record changed paths, implementation-check commands/results, contract/schema
artifacts, unresolved risks, excluded Future-only/reserved/placeholder-only
scope, the T12 coverage IDs above, and the rollback point. The handoff must
state `acceptance_owner=T12` and `acceptance=pending`; no child PASS,
production readiness or cutover is established here.
