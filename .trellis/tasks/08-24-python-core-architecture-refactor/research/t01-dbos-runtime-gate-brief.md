# T01 DBOS Runtime Gate Brief

## Purpose

This is the child-specific context for T01. T01 validates DBOS suitability only; it does not implement product/domain modules, BFF/browser, production persistence abstractions, or the final Diagnostics UI.

## Required Decisions

D001, D003, D005-D007, D016, D020-D021, D028-D029, D031-D033.

## Exact Child Manifest

The T01 implement/check manifests contain exactly:

1. `.trellis/spec/backend/fluctlight-workflow-contract.md`
2. `.trellis/tasks/08-24-python-core-architecture-refactor/research/t01-dbos-runtime-gate-brief.md`
3. `.trellis/tasks/08-24-python-core-architecture-refactor/research/workflow-engine-candidate.md`
4. `.trellis/tasks/08-24-python-core-architecture-refactor/research/evidence-baseline.md`
5. `.trellis/tasks/08-24-python-core-architecture-refactor/research/dbos-runtime-gate-report-template.md`

The persistence and diagnostics contracts are background parent contracts, not assigned T01 acceptance surfaces. T01 validates only the slices below; T02 owns the complete persistence contract and T05 owns the complete diagnostics contract.

## Persistence Slice

- Commit one minimal gate intent with stable workflow/provider IDs before workflow start.
- Keep fake external work outside the PostgreSQL transaction.
- Reuse stable IDs after crash/retry and persist one final result.
- Do not build multi-module Unit of Work, full outbox publisher, Alembic production schema, or cross-module architecture tests in T01.

## Diagnostics Slice

- Emit structured stdout records linking gate intent, workflow, step, provider request, attempt, correlation and result IDs.
- Prove the report can reconstruct one end-to-end failure/recovery chain.
- Do not implement PostgreSQL diagnostic tables, BFF ingestion, Owner UI, retention, prompt capture, export, or OTLP in T01.

## Owned Paths

- `.python-version`
- root `pyproject.toml`
- root `uv.lock`
- `apps/core/pyproject.toml`
- `apps/core/src/fluctlight_core/workflow_gate/__init__.py`
- `apps/core/src/fluctlight_core/workflow_gate/api_entrypoint.py`
- `apps/core/src/fluctlight_core/workflow_gate/worker_entrypoint.py`
- all other files below `apps/core/src/fluctlight_core/workflow_gate/`
- all files below `apps/core/tests/workflow_gate/`
- `infra/compose/dbos-gate.compose.yml`
- `infra/compose/dbos-gate.env.example`
- `.trellis/tasks/08-24-python-core-architecture-refactor/research/dbos-runtime-gate-report.md`

No other root/shared file is allowed without returning to parent planning.

## Canonical Management Operations

T01 must prove these operations through DBOS official APIs/client or a thin Python application wrapper:

- list workflows
- get workflow and step status
- pause
- resume
- cancel
- restart a failed/cancelled workflow
- fork/restart from a selected durable step

Automatic step retry is workflow policy, not a separate manual operation. The thin wrapper must accept a fake authorized administrative context and emit a structured audit record. If DBOS cannot reliably support any canonical operation, T01 is a core FAIL; do not invent a second runtime.

## Quantified NAS Resource Gates

Measure three runs and report median plus maximum. The fake external h3 process is excluded from application RSS but its orchestration overhead is included.

| Metric | PASS threshold |
| --- | --- |
| Core gate API + Worker combined idle RSS | <= 512 MiB |
| PostgreSQL + gate API + Worker combined idle RSS | <= 1 GiB |
| Gate API + Worker peak RSS during recovery suite | <= 1 GiB |
| Idle CPU over 5 minutes | <= 5% of one CPU core combined |
| PostgreSQL connections after steady state | <= 20 |
| API/Worker readiness after PostgreSQL healthy | <= 30 seconds |
| Due workflow resumes after Worker becomes ready | <= 30 seconds |

Exceeding a threshold is a core FAIL unless parent planning explicitly revises the threshold with measured evidence and user approval.

## Correlation Gate

T01 uses built-in structured diagnostics/stdout only. It does not start OpenTelemetry, a collector, Prometheus, Grafana, Loki or Tempo. One correlation query/report must connect intent → workflow → step attempts → fake Provider request → crash/recovery → final result.

## Commands And Report

Use the exact commands in `implement.md` T01. Complete `dbos-runtime-gate-report.md` from the approved template with exact versions, commands, PASS/FAIL evidence, failure injection and resource measurements.

## Failure Escalation

Any workflow contract gate, canonical management operation, quantified resource threshold, upgrade/replay, idempotency or recovery failure blocks T02+, returns the parent to planning and triggers Temporal evaluation. Celery/custom queues are not allowed workarounds.
