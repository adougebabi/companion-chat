# DBOS Runtime Gate Report

## Status

`NOT RUN | PASS | FAIL`

## Environment

- Date/host/architecture
- Python/uv/DBOS/PostgreSQL/Docker versions
- Exact commit and child task
- CPU/memory/storage constraints

## Reproduction Commands

List every command needed from empty checkout through cleanup. Commands must use committed lock/config files.

## Topology

Document API and Worker processes, DBOS system/application database arrangement, queue registration, Compose services, ports/volumes and health checks.

## Gate Matrix

| Gate | Result | Automated test/command | Evidence/artifact | Notes |
| --- | --- | --- | --- | --- |
| Three queue isolation and limits | NOT RUN | | | |
| API/Worker separate entrypoints | NOT RUN | | | |
| Durable sleep across restart | NOT RUN | | | |
| 15-minute fake h3 heartbeat | NOT RUN | | | |
| Timeout and cooperative cancellation | NOT RUN | | | |
| Crash/restart recovery | NOT RUN | | | |
| Provider success before checkpoint | NOT RUN | | | |
| Stable workflow/Provider idempotency | NOT RUN | | | |
| List/get/pause/resume/cancel/restart/fork-from-step | NOT RUN | | | |
| Active-history code/schema upgrade | NOT RUN | | | |
| Backup/restore system state | NOT RUN | | | |
| Diagnostics/correlation chain | NOT RUN | | | |
| All quantified NAS resource thresholds in T01 brief | NOT RUN | | | |

## Failure Injection

For each killed process/network/database/provider boundary, record when failure occurred, persisted state, restart action, number of external effects, and final domain/workflow result.

## Resource Measurements

Record three runs, median and maximum for every quantified threshold in `t01-dbos-runtime-gate-brief.md`, plus PostgreSQL storage growth and queue throughput for the gate workload.

## Upgrade And Recovery

Document active-workflow upgrade compatibility, backup inputs, restore commands, and any history/version restrictions.

## Decision

- `PASS`: authorize T02 after parent/check review.
- `FAIL`: identify failed core gate, stop T02+, return parent to planning and evaluate Temporal. Celery/custom queue is not an allowed workaround.
