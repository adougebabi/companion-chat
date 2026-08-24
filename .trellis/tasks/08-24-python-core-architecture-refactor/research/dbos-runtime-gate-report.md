# DBOS Runtime Gate Report

## Status

`FAIL`

The PostgreSQL/Compose gate is now runnable and most bounded runtime checks
pass. T01 still fails the mandatory core gate because DBOS 2.30.0 has no
official workflow pause operation, active history does not replay when the
Worker application version changes, and the full 15-minute fake-h3 run plus
provider-success-before-checkpoint fault boundary were not completed in the
bounded local run. T02 and later children remain blocked.

## Environment

- Date: 2026-08-24 (Asia/Shanghai)
- Host: Darwin MacBook-Pro-16.local, arm64
- Docker: 29.4.0, OrbStack server 29.4.0
- Python host: 3.13.7 (`.venv/bin/python`)
- Python Compose image: 3.13.6
- uv host: 0.10.12; Compose image uv: 0.8.9
- DBOS dependency: 2.30.0 (package does not expose `__version__`)
- PostgreSQL image: 16.4-alpine
- Base implementation commit: `d70879b`
- Child: `08-24-dbos-runtime-gate`
- PostgreSQL database size during gate: `8484 kB`

The Compose entrypoints use `--python 3.13` and
`UV_PROJECT_ENVIRONMENT=/tmp/fluctlight-venv`; this is required because the
repository pins `.python-version=3.13.7`, while the uv image ships 3.13.6 and
the mounted workspace is read-only.

## Reproduction Commands

```bash
uv sync --locked
uv run pytest apps/core/tests/workflow_gate -q
docker compose -f infra/compose/dbos-gate.compose.yml up -d --build
docker compose -f infra/compose/dbos-gate.compose.yml ps
uv run pytest apps/core/tests/workflow_gate -m compose -q
docker compose -f infra/compose/dbos-gate.compose.yml down
```

Live evidence was collected while the stack was running:

```bash
PYTHONPATH=apps/core/src ./.venv/bin/python -m pytest apps/core/tests/workflow_gate -q
```

Result: `18 passed` on the host fixture. The explicitly escalated live Compose
test passed (`1 passed`); a restricted host-side run may skip the Docker check
because the socket is inaccessible from that shell.

## Topology

The committed Compose fixture defines one PostgreSQL 16.4 service, one API
process and one Worker process from the same `fluctlight-core` package. The
Worker registers independent `interaction`, `lifecycle`, and `media` DBOS
queues after `dbos.launch()`. The API uses `DBOSClient` only to register queues
and enqueue a stable workflow ID; it never starts queue listeners. Both DBOS
system and application URLs point at PostgreSQL in the fixture. OTLP is
disabled and no external telemetry stack is started.

The entrypoints use a writable `/tmp/fluctlight-venv` and select the image's
Python 3.13.6 explicitly so the read-only source bind does not cause uv to
mutate the host `.venv`.

The topology is statically valid:

```bash
docker compose -f infra/compose/dbos-gate.compose.yml config
```

Result: configuration rendered successfully. The live stack reached healthy
PostgreSQL, API readiness, and Worker queue listening.

## Gate Matrix

Management API inspection used the installed package:

```bash
uv run python -c 'from dbos import DBOSClient; print(hasattr(DBOSClient, "pause_workflow"), hasattr(DBOSClient, "resume_workflow"), hasattr(DBOSClient, "cancel_workflow"), hasattr(DBOSClient, "restart_workflow"), hasattr(DBOSClient, "fork_workflow"))'
```

Result: `False True True False True` for DBOS 2.30.0. The wrapper therefore
cannot satisfy the canonical pause or restart operation without inventing a
non-official operation; the report records this as a core failure.

| Gate | Result | Evidence | Notes |
| --- | --- | --- | --- |
| Three queue isolation and limits | PASS | PostgreSQL `dbos.queues`; `test_contract.py` | interaction `2 / 4 per 1s`, lifecycle `1 / 1 per 1s`, media `1 / 1 per 2s`. |
| API/Worker separate entrypoints | PASS | Live `docker compose ps`, API readiness, Worker logs | API has no queue listener; Worker listens to 3 queues. |
| Durable sleep across Worker restart | PASS | `wf_e4f4565d4256fd636a04edc7`: `PENDING` during restart, then `SUCCESS` | Stable workflow ID preserved. |
| Durable sleep across PostgreSQL restart | PASS | `wf_58bc8ef5f8805bcebf552dbb`: `SUCCESS` after PostgreSQL restart | DBOS reconnected and resumed timer. |
| Fake-h3 heartbeat | PARTIAL | `wf_61e58bfeadeafbfe164f96f9`: 2/4/6/8/10 second structured heartbeat records | Bounded proof passed; full 15-minute run was not executed. |
| Timeout | PASS | `wf_ce0cc9f69232e1c3f710298e`: `workflow_timeout_ms=5000`, final `CANCELLED` | Timeout is propagated through official `workflow_timeout`. |
| Cooperative cancellation | PASS | `wf_d1a7351a2db2ab7af13e7dd8`: official `DBOSClient.cancel_workflow`, final `CANCELLED`, no completed step output | No duplicate external effect observed. |
| Crash/restart recovery | PASS (deterministic) / NOT RUN (live kill) | `test_recovery.py`, `test_diagnostics.py` | Deterministic checkpoint windows pass; live process-death injection remains unrun. |
| Provider success before checkpoint | PASS (deterministic) / NOT RUN (live) | `test_recovery.py`, stable provider lookup | Live DBOS provider checkpoint fault injection remains unrun. |
| Stable workflow/provider idempotency | PASS | Duplicate API POST returned `wf_b77d5a160880593f9a2f4ed4`; one intent and one result remained | Stable IDs and DBOS deduplication are live. |
| List/get/pause/resume/cancel/restart/fork-from-step | FAIL | DBOSClient surface: `False True True False True` for pause/resume/cancel/restart/fork | `pause_workflow` is absent; the complete canonical set is unavailable. |
| Active-history code/schema upgrade/replay | FAIL | v1 workflow `wf_6d40ec16100277ae4bb829a7` stayed `PENDING` under v2 Worker and later became `CANCELLED` | v2 logged no workflows to recover from v2 history. |
| Backup/restore system state | PASS (fixture) | `pg_dump`/`pg_restore` into `gate_restore_v2`: 11 workflows, 11 intents, 2 final results restored | Empty database restore verified. |
| Diagnostics/correlation chain | PASS (deterministic + live heartbeat) | `test_diagnostics.py`; structured Worker stdout | Full diagnostics tables/UI are out of scope. |
| NAS resource thresholds | PASS for measured values; full 15-minute run incomplete | Ten 30-second idle samples plus recovery samples | All measured RSS/CPU/connection/readiness thresholds pass; full 15-minute workload remains required. |

## Failure Injection

Deterministic tests cover provider success before checkpoint, crash after the
provider checkpoint, crash before result commit, timeout, cancellation and
Worker restart. Each successful recovery reuses the stable provider request ID
and asserts one provider effect plus one final result ID. Live tests covered
Worker restart, PostgreSQL restart, timeout, official cancellation, durable
sleep and structured heartbeat. Live provider-success-before-checkpoint and
process-kill-during-provider boundaries remain unrun.

## Resource Measurements

Ten idle samples, in MiB and Docker CPU percent:

| Metric | Samples | Median | Maximum | Threshold |
| --- | --- | ---: | ---: | ---: |
| API RSS | 71.86, 71.59, 73.2, 73.2, 71.76, 72.64, 71.97, 71.97, 71.73, 72.1 | 71.97 | 73.2 | n/a |
| Worker RSS | 61.99, 62.3, 60.48, 60.48, 62.79, 60.39, 62.8, 62.8, 60.86, 62.09 | 62.04 | 62.8 | n/a |
| PostgreSQL RSS | 63.43, 65.71, 62.23, 62.86, 65.25, 61.94, 65.89, 65.12, 61.9, 65.72 | 64.28 | 65.89 | n/a |
| API + Worker idle RSS | 133.85, 133.89, 133.68, 133.68, 134.55, 133.03, 134.77, 134.77, 132.59, 134.19 | 133.87 | 134.77 | <= 512 MiB |
| PostgreSQL + API + Worker idle RSS | 197.28, 199.6, 195.91, 196.54, 199.8, 194.97, 200.66, 199.89, 194.49, 199.91 | 198.44 | 200.66 | <= 1 GiB |
| API + Worker idle CPU | 4.15, 4.86, 4.19, 4.07, 4.79, 3.62, 4.16, 4.36, 2.53, 4.01 | 4.16 | 4.86 | <= 5% of one CPU |
| PostgreSQL connections | 8, 8, 8 | 8 | 8 | <= 20 |

During the bounded recovery workload, API + Worker peak RSS was `192.71 MiB`
(threshold `<= 1 GiB`). API readiness after restart was `2 s` (threshold
`<= 30 s`). A due workflow was already eligible when the Worker became ready
and settled within the first sample (`<= 1 s`, threshold `<= 30 s`). The ten
samples span five minutes and keep combined API + Worker idle CPU below 5%;
this resource threshold passes. The full 15-minute fake-h3 workload still
remains outstanding.

## Upgrade And Recovery

PostgreSQL backup/restore passed into an empty `gate_restore_v2` database. The
active-history upgrade gate failed: a workflow created under `t01-gate-v1` was
not recovered by a `t01-gate-v2` Worker. Returning to v1 did not rescue it
before its timeout. An explicit DBOS patch/continuation strategy or a Temporal
evaluation is required; no custom queue workaround is allowed.

## Decision

`FAIL`. T02 and all later children remain blocked. The parent task must return
to planning to resolve the missing pause management operation and active-history
upgrade strategy, then rerun the full 15-minute/5-minute and live provider-fault
gates. No Celery, custom queue, OTLP collector or second workflow runtime was
introduced.
