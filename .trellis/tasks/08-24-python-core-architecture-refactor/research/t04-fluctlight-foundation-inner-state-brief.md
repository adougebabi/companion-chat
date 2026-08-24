# T04 Fluctlight Foundation And Inner State Brief

## Authorization And Entry

This brief is the child-specific implementation record for T04. Parent decision D038 records the Owner instruction on 2026-08-24 to continue T04 implementation while T03 is still `in_progress` and to defer Docker, Compose, long-running process, and full-stack runtime acceptance. The exception is serialized: T04 is the only writing child in this session. It consumes only the current public T03 HumanActor reference and does not claim T03 completion, merge, or production readiness.

T03's skipped acceptance and unmerged worktree are carry-forward risks. Any conflict in the actor reference, migration graph, metadata ownership, or security boundary returns this task to parent planning rather than creating a compatibility path.

## Required Inputs

1. `.trellis/tasks/08-24-python-core-architecture-refactor/READ_FIRST.md`
2. `.trellis/tasks/08-24-python-core-architecture-refactor/decisions.md`
3. `.trellis/tasks/08-24-python-core-architecture-refactor/design.md`
4. `.trellis/spec/backend/fluctlight-cognitive-runtime.md`
5. `.trellis/spec/backend/fluctlight-persistence-contract.md`
6. `.trellis/spec/backend/fluctlight-autonomy-contract.md`
7. `.trellis/tasks/08-24-python-core-architecture-refactor/research/fluctlight-domain-model.md`
8. `.trellis/tasks/08-24-python-core-architecture-refactor/research/capability-inventory.md`
9. `.trellis/tasks/08-24-python-core-architecture-refactor/research/t03-actors-auth-settings-providers-report.md`

## Owned Paths

- `apps/core/src/fluctlight_core/fluctlights/`
- `apps/core/src/fluctlight_core/inner_state/`
- `apps/core/tests/fluctlights/`
- `apps/core/tests/inner_state/`
- `apps/core/tests/contract/test_t04_*.py`
- `apps/core/tests/architecture/test_t04_*.py`
- `apps/core/migrations/versions/0003_t04_fluctlight.py`
- `apps/core/migrations/env.py` only to register T04 schemas in the existing metadata graph.
- `apps/core/src/fluctlight_core/transport/api.py` only to advance the explicit readiness revision to the T04 head; no T04 HTTP route is added in this slice.
- This brief, the T04 dry run, and the T04 report/handoff under the parent `research/` directory.

## Forbidden Paths And Behaviors

- Do not modify the frozen `server/`, `web/`, or old `test/` system.
- Do not modify `0001_platform.py` or the unowned T03 tables; do not create a second metadata or migration graph.
- Do not add BFF/browser/generated-client routes for T04 in this domain-only slice.
- Do not import FastAPI, Temporal, Redis, Provider clients, object storage, or another domain's private repository from `fluctlights` or `inner_state`.
- Do not infer semantic meaning from text, keywords, regexes, fixed phrase tables, or default appraisals.
- Do not let a model-provided raw numeric delta update PAD, momentum, drives, goals, or personality.
- Do not execute Docker/Compose, start long-running services, or claim full-stack runtime acceptance in this session.

## Contracts To Deliver

- Fluctlight lifecycle with initialization mode, stable identity, personality, behavioral policy, immutable revision history, evidence-gated governance, accept/reject/rollback, retirement, and stale-revision CAS.
- Canonical numeric state: PAD and momentum in `-1..1`; normalized intensities, drives, progress, confidence, and policy values in `0..1`; deterministic wall-time decay and regulation.
- Structured semantic assessment validation requiring schema/version, evidence references, bounded appraisal, and no raw numeric deltas; Python policy computes requested/applied changes and records policy/model versions.
- Drive state and conflict records with bounded transitions.
- Goal and intention candidates/lifecycle with evidence, typed triggers, expiration, revisions, governance history, and no free-form keyword triggers.
- Append-only inner-state event history with idempotency and revision checks.

## Focused Validation

Run only the following commands in this session, subject to locally available dependencies:

```bash
uv sync --locked
uv run ruff format --check apps/core/src/fluctlight_core/fluctlights apps/core/src/fluctlight_core/inner_state apps/core/tests/fluctlights apps/core/tests/inner_state apps/core/tests/contract/test_t04_*.py apps/core/tests/architecture/test_t04_*.py
uv run ruff check apps/core/src/fluctlight_core/fluctlights apps/core/src/fluctlight_core/inner_state apps/core/tests/fluctlights apps/core/tests/inner_state apps/core/tests/contract/test_t04_*.py apps/core/tests/architecture/test_t04_*.py
uv run mypy apps/core/src/fluctlight_core/fluctlights apps/core/src/fluctlight_core/inner_state
uv run pytest apps/core/tests/fluctlights apps/core/tests/inner_state apps/core/tests/contract/test_t04_*.py apps/core/tests/architecture/test_t04_*.py -q
```

The focused tests must cover canonical ranges, wall-time decay, malformed semantic input, evidence authorization, raw-delta rejection, idempotency, stale revision/CAS, personality evidence windows/cooldown, goal/intention lifecycle and typed-trigger rejection, migration imports, and module boundary scans. Docker/Compose, real PostgreSQL, long-running process, full BFF/Core/browser, and full-product tests are deferred and must be listed as unverified.

## Rollback And Handoff

Rollback is limited to reverting the T04-owned files and migration/readiness integration while preserving the unmodified T03 worktree. The report must list exact commands/results, changed paths, migration revision, focused test coverage, T03 carry-forward risk, and every deferred runtime gate. No task archive or production-readiness claim is implied by this implementation exception.
