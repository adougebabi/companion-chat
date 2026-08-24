# Temporal Runtime Gate Report

## Status

`FAIL`

The grouped Temporal/PostgreSQL gate is functional on the local OrbStack
environment and the bounded functional checks pass. T01B is not a final PASS:
the required 12-hour workload on the actual 16 GiB target NAS was not run, the
full 15-minute fake-h3 activity was not run, and the five-minute CPU sample
included startup spikes above the threshold. T02 and later children remain
blocked until the target-NAS soak and any remaining core gates pass.

## Environment

- Date: 2026-08-24 (Asia/Shanghai)
- Host: MacBook-Pro-16.local, arm64, OrbStack Linux VM
- Docker/OrbStack server: 29.4.0, 16 host CPUs, 9.77 GiB Docker memory limit
- Target NAS requirement: 16 GiB; this host is not an approved equivalent for final soak.
- Python host: 3.13.7; Compose image: 3.13.6
- uv: host 0.10.12; Compose image 0.8.9
- Temporal Python SDK: 1.20.0
- Temporal Server image: `temporalio/auto-setup:1.29.7`
- PostgreSQL image: 16.4-alpine
- Base implementation commit: current T01B working tree after T01B start
- Child: `08-24-temporal-runtime-gate`
- Default database size during sample: about `9372 kB`
- Visibility database size during sample: about `9172 kB`

The default gate disables Elasticsearch/OpenSearch, Temporal UI, Prometheus,
Grafana and OTLP. It uses grouped Frontend/History/Matching/Worker roles,
PostgreSQL default and visibility databases, and three application task queues.
Temporal SQL pools are explicitly bounded with `SQL_MAX_CONNS=4` and
`SQL_VIS_MAX_CONNS=2`.

## Reproduction Commands

```bash
uv sync --locked
uv run ruff format --check apps/core
uv run ruff check apps/core
uv run mypy apps/core/src apps/core/tests/temporal_gate
uv run pytest apps/core/tests -q
docker compose -f infra/compose/temporal-gate.compose.yml config
./infra/compose/run-temporal-gate.sh --clean --functional
./infra/compose/run-temporal-gate.sh --clean --soak-hours 12 --require-target-nas
```

The clean functional runner starts the stack, waits for `/readyz`, sends one
API intent, runs the deterministic gate tests and removes volumes through its
EXIT trap. The 12-hour command intentionally refuses to claim PASS unless
`TEMPORAL_GATE_TARGET_NAS=1` is set on the actual approved NAS.

## Topology

Live `docker compose ps` showed healthy PostgreSQL, grouped Temporal Server,
API and Worker. A one-shot `postgres-visibility` helper creates the
`temporal_visibility` database on that PostgreSQL service before Temporal
starts; it has no persistent data or runtime role. The API never polls
application queues. The Worker creates one bounded Temporal Worker per
`interaction`, `lifecycle` and `media` queue, with official Worker Deployment
Versioning enabled.

Temporal Server 1.27.2 was rejected during the gate because deployments were
disabled. The clean gate now pins 1.29.7, where deployment routing is enabled.
The Worker activates `fluctlight-gate/gate-v1` through the official
`set_worker_deployment_current_version` RPC.

## Gate Matrix

| Gate | Result | Evidence | Notes |
| --- | --- | --- | --- |
| Grouped non-HA topology, PG visibility, no ES/UI/metrics | PASS | Clean Compose config and live healthy services | `temporalio/auto-setup:1.29.7`; optional stacks absent. |
| API/Worker separation and three task queues | PASS | Live `/healthz`/`/readyz`, Worker startup, task queues | API starts workflows; Worker polls all three queues. |
| Stable Workflow/Provider IDs and one result | PASS | `wf_c7f9bf512abfe3b325523a4f`; PostgreSQL `gate_results` count 1 | Stable IDs survive normal execution. |
| Durable timer across Worker/Temporal/PostgreSQL restart | PASS | `wf_10625eba3b9751602ac3a624` completed after all three services restarted | Worker initially lost the connection; `restart: unless-stopped` recovered it. |
| Signal pause/resume + Query status | PASS | `wf_abe63d907c1123683a8fc92b` Query returned `paused=true`, then resume Signal | Query exposes status, signals are durable history events. |
| Update acknowledgement/validation | PASS | Live repair returned `accepted=true`, acknowledged `gate-v1`; unit conflict tests pass | Unauthorized operations return 403 and produce audit records. |
| Activity heartbeat/timeout/cancel | PASS (bounded) | Live heartbeat Activity, timeout tests, corrected cancel `CANCELED` workflow | Full 15-minute Activity remains outstanding. |
| 15-minute fake h3 | PARTIAL | Bounded live durations and heartbeat records pass | No full 900-second run was executed in this session. |
| Provider success before Activity completion | PASS (deterministic) / NOT RUN (live) | Stable Provider fixture tests | Live process-kill-after-provider injection remains unrun. |
| Temporal/PostgreSQL/Worker live recovery | PASS (bounded) | Timer completed after PostgreSQL/Temporal/Worker restart | No duplicate final result in bounded run. |
| List/search workflows | PASS (SDK surface/unit) | Management client tests and Temporal CLI visibility | Full filtered search matrix remains bounded. |
| Get Workflow/Run and Activity status | PASS (live) | Temporal CLI `workflow describe` showed Run, Activity and result | |
| Query durable status | PASS (live) | Pause Query response included status, pending signals and heartbeat state | |
| Signal pause/resume | PASS (live) | Management endpoints and Query verification | |
| Validated Update/repair acknowledgement | PASS (live) | Owner repair Update acknowledged version | |
| Cancel and emergency terminate | PASS (bounded) | Cancel endpoint produced Temporal `CANCELED`; terminate has unit coverage | Emergency terminate live call remains unrun. |
| Retry/restart with stable domain identity | PARTIAL | Deterministic runtime restart and stable IDs pass | Official live reset/restart after a failed history remains unrun. |
| Official reset/history-point replay | PARTIAL | Management adapter uses official `reset_workflow_execution`; unit validation passes | Live reset against a real history remains unrun. |
| Authorization/audit for every operation | PASS (bounded) | Management unit tests and live unauthorized pause `403` | Full live matrix remains bounded. |
| Saved history Replayer v1→v2 | PASS (live) | Exported Temporal history replayed with SDK `Replayer`, `replay_failure=None` | Uses real workflow history from the live server. |
| Worker Deployment Versioning coexist/drain/rollback | PASS (bounded live) | v1 current route, v2 coexist/current route, v2 workflow metadata, rollback to v1 | Current routing was switched through official RPC. |
| Continue-as-new state/signal continuity | PASS (live) | `wf_768a7d68885c34fef670880d` produced multiple runs and completed final run | Control fields are filtered during replay. |
| Backup/restore active workflow | PASS (fixture/DB rows) / PARTIAL (resume) | Default and visibility dumps restored into empty databases with 14 execution/visibility rows | Restored server was not booted against the restored databases in this session. |
| Built-in diagnostics correlation | PASS (bounded) | Structured result IDs/correlation IDs and diagnostics tests | No external telemetry stack. |
| Resource/disk thresholds | PARTIAL | Three RSS/CPU samples, connections 20, disk under 10 MB | CPU sample included startup max 7.91%; actual NAS soak absent. |
| 12-hour target-NAS soak/leak/backlog | NOT RUN | Runner requires explicit target-NAS flag | This alone prevents final PASS. |

## Failure Injection

Deterministic tests cover provider checkpoint windows, result idempotency,
timer recovery, signals, updates, cancellation, reset and continue-as-new.
Live bounded tests cover Activity heartbeat, timeout, cancellation, PostgreSQL
restart, Temporal restart, Worker restart, and final result persistence. A live
process kill while an external provider succeeds before Activity completion was
not run; the full 15-minute h3 and 12-hour soak remain required.

## Resource And Disk Measurements

Ten 30-second samples were collected after the clean stack was running. Values
are MiB unless noted:

| Metric | Median | Maximum | Threshold |
| --- | ---: | ---: | ---: |
| Grouped Temporal Server RSS | 138.75 | 152.5 | <= 768 MiB idle / <= 1.5 GiB peak |
| PostgreSQL RSS | 79.40 | 81.95 | included in full-stack gate |
| API RSS | 111.6 | 129.4 | included in full-stack gate |
| Worker RSS | 93.91 | 105.4 | included in full-stack gate |
| Complete stack RSS | 425.39 | 444.78 | <= 2.5 GiB idle / <= 4 GiB peak |
| PostgreSQL connections | 20 | 20 | <= 30 |
| Temporal default database | 9.4 MB | 9.4 MB | projected 30-day growth <= 5 GiB |
| Visibility database | 9.2 MB | 9.2 MB | projected 30-day growth <= 5 GiB |

Temporal CPU samples were `7.91, 1.26, 1.76, 2.36, 1.90, 1.75, 5.88,
1.80, 1.98, 5.59%`; the first and later peaks exceed the strict <=5% idle
criterion. A clean five-minute steady-state sample and 30-day disk projection
are still required for a final resource decision.

## Upgrade And Recovery

The live history from `wf_768a7d68885c34fef670880d` replayed successfully with
the SDK Replayer. Worker Deployment Versioning was enabled only after moving
from Server 1.27.2 to 1.29.7; v1/v2 coexist and current-route rollback were
verified through Temporal's official deployment RPC. PostgreSQL default and
visibility backups restored execution and visibility rows into empty databases,
but the restored server was not started against those databases, so active
workflow resume after restore remains partial.

## Decision

`FAIL` for final T01B completion. The local grouped Temporal runtime is a
credible replacement candidate and all bounded deterministic/live functional
checks above are positive, but the actual 16 GiB NAS 12-hour soak, full
15-minute Activity, live provider checkpoint fault, clean steady-state CPU
sample, and active-workflow resume from restored databases remain incomplete.
T02-T12 remain blocked until those gates pass on the approved target NAS.
No DBOS/Celery/custom runtime or deprecated Build-ID redirect was introduced.
