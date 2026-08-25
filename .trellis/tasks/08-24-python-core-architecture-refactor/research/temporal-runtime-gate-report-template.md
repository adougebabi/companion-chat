# Historical Temporal Runtime Gate Report Template

## Status

`NOT RUN | PASS | FAIL`

Parent D036 supersedes the old resource-duration/12-hour soak PASS requirement. This template is retained as T01B history; future tasks do not rerun it as a release gate.

## Environment

- Date/host/architecture and 16 GiB NAS equivalence
- Python/uv/Temporal SDK/Temporal Server/PostgreSQL/Docker versions
- Exact commit and T01B child task
- CPU/memory/storage constraints

## Reproduction Commands

List every command from empty checkout through cleanup using committed locks/config.

## Topology

Document grouped server roles, PostgreSQL default/visibility databases, namespace/retention/history shards, task queues/pollers, internal ports/volumes and disabled optional components.

## Gate Matrix

| Gate | Result | Automated command/test | Evidence/artifact | Notes |
| --- | --- | --- | --- | --- |
| Grouped non-HA topology, PG visibility, no ES/UI/metrics | NOT RUN | | | |
| API/Worker separation and three task queues | NOT RUN | | | |
| Stable Workflow/Provider IDs | NOT RUN | | | |
| Durable timer across restarts | NOT RUN | | | |
| Signal pause/resume + Query status | NOT RUN | | | |
| Update acknowledgement/validation | NOT RUN | | | |
| Activity heartbeat/timeout/cancel | NOT RUN | | | |
| 15-minute fake h3 | NOT RUN | | | |
| Provider success before Activity completion | NOT RUN | | | |
| Temporal/PostgreSQL/Worker live kill/recovery | NOT RUN | | | |
| List/search workflows | NOT RUN | | | |
| Get Workflow/Run and Activity status | NOT RUN | | | |
| Query durable status | NOT RUN | | | |
| Signal pause/resume | NOT RUN | | | |
| Validated Update/repair acknowledgement | NOT RUN | | | |
| Cancel and emergency terminate | NOT RUN | | | |
| Retry/restart with stable domain identity | NOT RUN | | | |
| Official reset/history-point replay | NOT RUN | | | |
| Authorization/audit evidence for every operation | NOT RUN | | | |
| Saved history Replayer v1→v2 | NOT RUN | | | |
| Current Worker Deployment Versioning coexist/drain/rollback | NOT RUN | | | |
| Continue-as-new state/signal continuity | NOT RUN | | | |
| Backup/restore active workflow | NOT RUN | | | |
| Built-in diagnostics correlation | NOT RUN | | | |
| Resource/disk observation (informational) | NOT RUN | | | |

## Failure Injection

Record every process/database/provider boundary, persisted history/domain state, restart action, external effect count and final result.

## Resource And Disk Measurements

Record three runs with median/max for each Temporal gate threshold, per-workflow history/visibility growth, retention projection and complete local stack RSS.

## Upgrade And Recovery

Attach saved v1 histories, Replayer output, Worker Deployment routing evidence, rollback/drain/continue-as-new procedure and restored active workflow proof.

## Decision

- Historical outcome remains `FAIL` under the superseded old gate. Parent D020/D036 accept Temporal core and authorize T02 preparation without rerunning resource/soak research.

Record exact changed paths, lint/type/full-test/functional/soak commands and results, clean-volume cleanup status, remaining risks and archive location.
