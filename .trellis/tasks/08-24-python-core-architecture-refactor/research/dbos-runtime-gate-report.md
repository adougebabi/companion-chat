# DBOS Runtime Gate Report

## Status

`FAIL`

The deterministic gate and a real DBOS SQLite smoke pass, but the required
PostgreSQL/Compose gate did not run because the local Docker daemon is down.
The installed DBOS 2.30.0 client also has no workflow pause operation. Either
condition blocks T02 and returns the parent task to planning for a Temporal
evaluation; this report does not authorize a workaround runtime.

## Environment

- Date: 2026-08-24T05:18:36Z
- Host: Darwin MacBook-Pro-16.local, arm64
- Python: 3.13.7 (`.venv/bin/python`)
- uv: 0.10.12
- DBOS dependency: 2.30.0 (package does not expose `__version__`)
- Docker CLI: 29.4.0
- Commit at measurement: `6e52536`
- Child: `08-24-dbos-runtime-gate`
- Docker socket: `unix:///Users/vinson/.orbstack/run/docker.sock` (not present)
- Resource constraints: NAS thresholds from `research/t01-dbos-runtime-gate-brief.md`; no NAS measurement was possible on this host.

## Reproduction Commands

```bash
uv sync --locked
uv run pytest apps/core/tests/workflow_gate -q
docker compose -f infra/compose/dbos-gate.compose.yml up -d --build
docker compose -f infra/compose/dbos-gate.compose.yml ps
uv run pytest apps/core/tests/workflow_gate -m compose -q
docker compose -f infra/compose/dbos-gate.compose.yml down
```

Additional local API smoke used while Docker was unavailable:

```bash
PYTHONPATH=apps/core/src ./.venv/bin/python -m pytest apps/core/tests/workflow_gate -q
```

Result: `17 passed, 1 skipped` after the review fixes for stable API
IDs, queue ownership, persisted durable-sleep deadlines, and explicit Compose
daemon detection. The one skipped test is the Compose service check because the
Docker daemon is unavailable. This remains a deterministic fixture result; it
is not evidence of a PostgreSQL/Compose run.

## Topology

The committed Compose fixture defines one PostgreSQL 16.4 service, one API
process and one Worker process from the same `fluctlight-core` package. The
Worker registers independent `interaction`, `lifecycle`, and `media` DBOS
queues after `dbos.launch()`. The API uses `DBOSClient` only to register queues
and enqueue a stable workflow ID; it never starts queue listeners. Both DBOS
system and application URLs point at PostgreSQL in the fixture. OTLP is
disabled and no external telemetry stack is started.

The topology is statically valid:

```bash
docker compose -f infra/compose/dbos-gate.compose.yml config
```

Result: configuration rendered successfully. Container startup was not
possible because the Docker daemon socket was unavailable. The static config
check does not prove that the read-only `/workspace` bind can support `uv run`
environment creation inside the image, so that startup path remains
unverified.

## Gate Matrix

Management API inspection used the installed package:

```bash
uv run python -c 'from dbos import DBOSClient; print(hasattr(DBOSClient, "pause_workflow"), hasattr(DBOSClient, "resume_workflow"), hasattr(DBOSClient, "cancel_workflow"), hasattr(DBOSClient, "restart_workflow"), hasattr(DBOSClient, "fork_workflow"))'
```

Result: `False True True False True` for DBOS 2.30.0. The wrapper therefore
cannot satisfy the canonical pause or restart operation without inventing a
non-official operation; the report records this as a core failure.

| Gate | Result | Automated test/command | Evidence/artifact | Notes |
| --- | --- | --- | --- | --- |
| Three queue isolation and limits | PASS (fixture) / NOT RUN (PostgreSQL) | `uv run pytest apps/core/tests/workflow_gate -q` | `test_contract.py`; `queues.py` | Independent policies are covered in deterministic tests; Compose execution not measured. |
| API/Worker separate entrypoints | PASS (fixture) / NOT RUN (Compose) | `test_compose.py`; SQLite DBOS smoke | `api_entrypoint.py`, `worker_entrypoint.py` | Compose check explicitly skipped because the Docker daemon was unavailable; real container readiness not measured. |
| Durable sleep across restart | PASS (deterministic fixture) / NOT RUN (PostgreSQL) | `test_recovery.py` | `runtime.py` | SQLite smoke used zero sleep only. |
| 15-minute fake h3 heartbeat | PASS (deterministic fixture) / NOT RUN (Compose) | `test_recovery.py` | `runtime.py`, `provider.py` | No 15-minute container run. |
| Timeout and cooperative cancellation | PASS (deterministic fixture) / NOT RUN (Compose) | `test_recovery.py` | `runtime.py` | No PostgreSQL/Worker failure run. |
| Crash/restart recovery | PASS (deterministic fixture) / NOT RUN (Compose) | `test_recovery.py` | `runtime.py` | Input rehydration after a new runtime instance is covered. |
| Provider success before checkpoint | PASS (deterministic fixture) / NOT RUN (Compose) | `test_recovery.py`, `test_diagnostics.py` | `provider.py`, `diagnostics.py` | Exactly one fake provider effect is asserted. |
| Stable workflow/Provider idempotency | PASS (deterministic fixture) / NOT RUN (DBOS crash/restart) | `test_contract.py`, `test_recovery.py`, SQLite DBOS smoke | `ids.py`, `dbos_runtime.py` | Stable API workflow/provider IDs and DBOS deduplication options are exercised; the real DBOS crash/restart provider window was not run. |
| List/get/pause/resume/cancel/restart/fork-from-step | FAIL | DBOSClient surface inspection; management tests | `management.py` | `pause_workflow` is absent in DBOS 2.30.0. Restart is represented by official `fork_workflow(..., 0)`; pause has no equivalent official API. |
| Active-history code/schema upgrade | NOT RUN | Compose gate required | report template | Docker unavailable. |
| Backup/restore system state | NOT RUN | Compose gate required | report template | Docker unavailable. |
| Diagnostics/correlation chain | PASS (deterministic fixture) / NOT RUN (Compose) | `test_diagnostics.py` | `diagnostics.py` | Structured stdout chain is reconstructed in memory. |
| All quantified NAS resource thresholds in T01 brief | NOT RUN | Three Compose measurements required | report template | No NAS/Docker runtime available. |

## Failure Injection

Deterministic tests cover provider success before checkpoint, crash after the
provider checkpoint, crash before result commit, timeout, cancellation and
worker restart. Each successful recovery reuses the stable provider request ID
and asserts one provider effect plus one final result ID. PostgreSQL connection
loss, database restart and container kill boundaries were not run.

## Resource Measurements

No measurements were recorded. The required three-run median/max values for
RSS, CPU, PostgreSQL connections, readiness and due-workflow recovery require
the Compose stack and therefore remain `NOT RUN`.

## Upgrade And Recovery

The deterministic fixture has no DBOS PostgreSQL history to upgrade. The
required active-history replay and backup/restore drills remain `NOT RUN`.

## Decision

`FAIL`. T02 and all later children remain blocked. The parent task must return
to planning, resolve the missing pause management capability and rerun the
PostgreSQL/Compose gate on a host with a running Docker daemon before choosing
DBOS or evaluating Temporal. No Celery, custom queue, OTLP collector or other
second runtime was introduced.
