# T01B Temporal Runtime Gate Brief

## Purpose

Historical child brief for the completed T01B evaluation. Parent D020/D036 now accept Temporal core and supersede the resource-duration/12-hour soak blocker. This file preserves gate scope; it is not a future implementation blocker.

## Required Decisions

D001, D003, D005-D007, D016, D020-D022, D028-D029, D031-D036.

Decision meanings are part of this brief, not assumed from chat history:

- D001/D034: personal 16 GiB NAS; external MTPLX/ComfyUI/h3; low idle footprint and long-term stability.
- D003/D031/D033: one complete product cutover, strict single writer, child brief/dry-run required before start.
- D005-D007: Node owns browser transport only; Python owns workflows/domain; API and Worker share package but are separate processes.
- D016: stable committed intent/outbox precedes external work; external effects are idempotent.
- D020: DBOS is rejected; Temporal is the only T01B candidate; no Celery/custom fallback.
- D021-D022: Python 3.13/uv/FastAPI stack and Node 24/Fastify/Vue boundaries remain fixed.
- D028-D029: built-in diagnostics/no external telemetry; PostgreSQL/SQLAlchemy/Psycopg/Alembic ownership remains fixed.
- D032: pinned uv, committed lockfile and locked commands.
- D035/D036: grouped non-HA Temporal, PostgreSQL visibility, no ES/default UI/metrics; measured resource viability is accepted and fixed-duration/resource thresholds are not future blockers.

Before editing, a new session must manually read parent `READ_FIRST.md`, `decisions.md`, `design.md` Workflow Direction, and parent `implement.md` T01/T01B/T02 sections. Child injected context does not replace this step.

Use `/usr/bin/python3 ./.trellis/scripts/task.py ...` for Trellis lifecycle commands if pyenv cannot resolve the application `.python-version`; use `uv run` for gate application/tests.

Hard prohibitions: no old-product compatibility work; no Node database/runtime access; no DBOS/Celery/custom concurrent runtime; no `start-dev` sustained topology; no Elasticsearch/OpenSearch/default UI/Prometheus/OTLP; no deprecated Build-ID redirect strategy; no weakening gates inside the child.

## Exact Child Manifest

T01B implement/check manifests contain exactly:

1. `.trellis/spec/backend/fluctlight-workflow-contract.md`
2. `.trellis/spec/backend/fluctlight-temporal-gate-contract.md`
3. `.trellis/tasks/08-24-python-core-architecture-refactor/research/t01b-temporal-runtime-gate-brief.md`
4. `.trellis/tasks/08-24-python-core-architecture-refactor/research/temporal-runtime-candidate.md`
5. `.trellis/tasks/08-24-python-core-architecture-refactor/research/dbos-runtime-gate-report.md`
6. `.trellis/tasks/08-24-python-core-architecture-refactor/research/temporal-runtime-gate-report-template.md`

## Owned Paths

- `.python-version`, root `pyproject.toml`, root `uv.lock`.
- `apps/core/pyproject.toml`.
- `apps/core/src/fluctlight_core/temporal_gate/__init__.py`.
- `apps/core/src/fluctlight_core/temporal_gate/api_entrypoint.py`.
- `apps/core/src/fluctlight_core/temporal_gate/worker_entrypoint.py`.
- all other files under `apps/core/src/fluctlight_core/temporal_gate/`.
- all files under `apps/core/tests/temporal_gate/`.
- `infra/compose/temporal-gate.compose.yml` and `infra/compose/temporal-gate.env.example`.
- `infra/compose/run-temporal-gate.sh`.
- existing `apps/core/src/fluctlight_core/workflow_gate/` and `apps/core/tests/workflow_gate/` for removal from the current tree.
- existing `infra/compose/dbos-gate.compose.yml` and `infra/compose/dbos-gate.env.example` for removal from the current tree.
- Parent `research/temporal-runtime-gate-report.md` created from the approved template.

T01B removes DBOS from current production/default dependencies, scripts, source/tests and Compose paths. Archived T01 task, FAIL report and git commits remain the historical evidence. Normal wheel/install/test/Compose paths must contain no DBOS dependency or runnable fixture.

No other root/shared file is allowed without returning parent planning.

## Topology Slice

- One grouped non-HA Temporal Server container, not `start-dev`.
- PostgreSQL `temporal` and `temporal_visibility` databases on the gate PostgreSQL service.
- No Elasticsearch/OpenSearch, Temporal UI, Prometheus, Grafana or OTLP collector in the default gate run.
- Minimal gate API process starts/controls workflows but polls no task queue.
- Minimal gate Worker process polls `interaction`, `lifecycle` and `media` task queues with bounded pollers/concurrency.

## Canonical Operations

- list/search workflows through visibility
- get Workflow/Run and Activity status
- Query current durable state
- Signal pause and resume
- validated Update/repair with acknowledgement
- cancel and explicit emergency terminate
- retry/restart a failed business workflow with stable domain identity
- official reset/replay from Event History point
- saved-history Replayer
- current Worker Deployment Versioning coexist/drain/rollback
- continue-as-new with state and pending-signal continuity

T01B proved the core management model. Remaining full live reset/restart/history-point and operation-matrix integration is assigned to T02; it is not a reason to repeat T01B.

Every operation must have its own report row and authorization/audit evidence; one aggregate “management operations” PASS is insufficient.

## Failure And Recovery Scenarios

- Durable timer across Worker, Temporal Server and PostgreSQL restarts.
- Bounded Activity heartbeat/timeout/cancellation was sufficient for runtime acceptance. Media-duration and live Provider checkpoint behavior is assigned to T09.
- Worker/Temporal/PostgreSQL restart was proven at bounded scope.
- v1 saved histories replay under v2 or route to compatible Worker through current Worker Deployment Versioning.
- Rollback/drain, continue-as-new and reset preserve business/domain references.
- PostgreSQL rows backup/restore was proven; full active-workflow restore/resume is assigned to T11.

## Resource Observation (Handled)

T01B measured about 139 MiB Temporal Server RSS, 425 MiB complete gate-stack RSS, 20 PostgreSQL connections and small default/visibility databases. D036 accepts this as sufficient. No 12-hour soak, fixed run duration, strict RSS/CPU threshold or 30-day disk projection remains as a T02/release gate.

## Diagnostics Slice

Structured stdout/report only: correlate gate intent → Workflow/Run → Workflow Task/Activity attempts → Provider request → Signal/Update/cancel/recovery → final result. Do not implement final diagnostic tables/UI or external telemetry.

## Exact Final Commands

```bash
uv sync --locked
uv run ruff format --check apps/core
uv run ruff check apps/core
uv run mypy apps/core/src apps/core/tests/temporal_gate
uv run pytest apps/core/tests -q
docker compose -f infra/compose/temporal-gate.compose.yml config
./infra/compose/run-temporal-gate.sh --clean --functional
```

The runner must install a shell trap/finally cleanup and execute `docker compose down -v --remove-orphans` before and after every clean trial, including failed tests. Each of the three resource trials starts from clean volumes unless the scenario explicitly tests backup/restore or restart continuity.

## Task Lifecycle

- Completed: report/check/commits/archive exist. Parent D020/D036 accept Temporal core and assign carry-forward functional checks to T02/T09/T11.

## Failure Escalation

No DBOS/Celery/custom fallback is allowed. Future failures in assigned T02/T09/T11 functionality are handled by those child contracts, not by reopening resource-duration research.
