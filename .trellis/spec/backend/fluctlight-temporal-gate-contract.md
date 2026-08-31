# Fluctlight Temporal Runtime Gate Contract (Completed)

## Scenario: Low-Footprint Temporal On A 16 GiB NAS

### 1. Scope / Trigger

- Trigger: T01B evaluated Temporal after T01 rejected DBOS. Parent D020/D036 accept the core result and supersede the old resource-duration PASS requirement.
- Target host is a 16 GiB NAS; MTPLX, ComfyUI and h3 are remote. The gate measures only local Fluctlight infrastructure.
- T01B validates Temporal suitability only. It does not implement Fluctlight domain modules, final BFF/browser, production schemas or Control Center.

### 2. Signatures

Go gate uses current Temporal SDK concepts:

```python
@workflow.defn
class GateWorkflow:
    @workflow.run
    async def run(self, input: GateInput) -> GateResult: ...

    @workflow.signal
    async def pause(self) -> None: ...

    @workflow.signal
    async def resume(self) -> None: ...

    @workflow.query
    def status(self) -> GateStatus: ...

    @workflow.update
    async def repair(self, command: RepairCommand) -> RepairResult: ...

@activity.defn
async def fake_h3(input: FakeH3Input) -> ProviderResult:
    activity.heartbeat(progress)
```

Task queues: `interaction`, `lifecycle`, `media`. Stable Workflow IDs derive from committed gate intents. Saved histories are replayed through the Go SDK replay test harness before upgrade acceptance.

### 3. Contracts

- One grouped non-HA Temporal Server process/container runs Frontend, History, Matching and internal Worker roles.
- PostgreSQL provides separate `temporal` default and `temporal_visibility` databases on the existing server. Elasticsearch/OpenSearch is disabled.
- Temporal UI, Prometheus and external telemetry are disabled by default; CLI/UI may run temporarily for gate/admin evidence.
- The final deployment cannot use `temporal server start-dev`; the gate uses self-hosted server configuration intended for sustained operation.
- Go Core API does not poll application task queues. Go Worker(s) poll three queues with bounded pollers/concurrency.
- Pause/resume is durable workflow state changed through Signals and observable through Query. Update is used where the caller needs validated acknowledgement.
- Cancellation propagates to Activities; long Activities heartbeat and recover progress after Worker/runtime/PostgreSQL restart.
- Stable Workflow/Provider IDs prevent duplicate external effects across Activity retry and result-commit crash windows.
- Event History from v1 code must replay under compatible v2 code or remain routed to an old Worker through current Worker Deployment Versioning until drain/continue-as-new. Deprecated Build ID APIs are not accepted as the long-term strategy.
- `continue_as_new` is triggered by SDK history-size suggestion or an earlier bounded policy, preserving domain/business identity and required pending state.
- Canonical management operations are list/search/get/query, pause/resume, cancel, terminate by explicit emergency policy, retry/restart, reset/replay from an official history point, and audited repair/update.
- Temporal execution history does not replace Fluctlight domain state or PostgreSQL outbox/inbox.
- No DBOS/Celery/custom delayed queue remains active after a Temporal PASS/cutover plan.
- Current production/default dependencies, scripts, source/tests and Compose paths contain no runnable DBOS fixture after T01B; archived task/report/git commits preserve the failure evidence.

Resource observation is non-blocking. T01B measured approximately 139 MiB Temporal Server RSS and 425 MiB complete gate-stack RSS with 20 PostgreSQL connections and small default/visibility databases. Future tasks use normal health/readiness, bounded connection pools and cleanup checks; they do not repeat fixed-duration soak or resource-threshold research.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Grouped server requires Elasticsearch/OpenSearch or default UI/metrics stack | Gate FAIL; simplify config or return parent planning. |
| Stable duplicate start | Reuse/reject according to explicit Workflow ID policy; one external effect. |
| Pause Signal arrives during Activity | Workflow records paused state; new work stops at safe point, query reports it, resume continues deterministically. |
| Cancel during heartbeating Activity | Activity observes cancellation and cannot commit stale result. |
| Runtime/PostgreSQL/Worker restarts during timer/Activity | Durable history resumes one workflow with stable IDs. |
| New code fails saved-history Replayer | Upgrade gate FAIL; do not deploy incompatible Worker. |
| New Worker cannot safely coexist/route old histories | Versioning gate FAIL until official Worker Deployment strategy passes. |
| History approaches limit | Continue-as-new with explicit state; no lost signal/business link. |
| Reset/restart/repair lacks official supported path | Gate FAIL; no custom second workflow runtime. |
| Production/default path still includes DBOS dependency/entrypoint/Compose | Gate FAIL; historical archive/report/commits are the only retained DBOS evidence. |

### 5. Good / Base / Bad Cases

- Good: a delayed Action survives Temporal/PostgreSQL/Worker restarts, pauses/resumes by Signal, and realizes once.
- Good: v1 saved Event History replays under v2, old and new Worker versions route safely, then workflow continues-as-new.
- Good: a 15-minute fake h3 Activity heartbeats, cancels cooperatively and recovers the stable Provider result.
- Base: a due workflow finds its domain intent closed and completes `no_op`.
- Bad: use start-dev in final NAS, require Elasticsearch for visibility, deploy incompatible Workflow code without replay, or emulate reset with another queue.

### 6. Tests Required

- Compose topology/config test for grouped server, PostgreSQL default+visibility, internal ports, UI/metrics/Elasticsearch disabled.
- Three queue/poller/concurrency/rate tests with API/Worker separation.
- Durable timer, Signal pause/resume, Query, Update, cancel/terminate, retry/reset/replay and audited repair tests.
- 15-minute fake h3 heartbeat/timeout/cancel plus live Worker/Temporal/PostgreSQL kill/restart and provider-success-before-completion failure injection.
- Stable Workflow/Provider ID duplicate tests and one final domain result.
- Export real v1 histories; Go replay harness against v2; current Worker Deployment Versioning coexist/drain/rollback; continue-as-new state/signal tests.
- PostgreSQL backup/restore of Temporal default+visibility and resumed active workflow.
- Basic health/readiness, bounded connection-pool and cleanup checks; no fixed-duration/resource-threshold release gate.
- Structured diagnostics correlation from gate intent through Workflow/Activity/Provider/recovery/result without external telemetry stack.
- Ruff format/lint, mypy, full Core suite, clean-volume trap-based functional runner and cleanup verification.

### 7. Wrong vs Correct

#### Wrong

```python
# Deploy changed Workflow code and hope active histories still replay.
worker = Worker(client, task_queue="interaction", workflows=[ChangedWorkflow])
```

#### Correct

```python
await Replayer(workflows=[CompatibleWorkflow]).replay_workflow(saved_history)
# Deploy through the current Worker Deployment Versioning strategy,
# retain rollback compatibility, then drain or continue-as-new old histories.
```

## Deployment Invariants

- Compose bind sources for `infra/postgres/10-temporal-databases.sql` and
  `infra/redis/redis.conf` must be regular files before startup. File mounts
  use `bind.create_host_path: false`; a missing source must fail before a
  container is created rather than being silently replaced by a directory.
- PostgreSQL readiness is not satisfied by `pg_isready` alone. The probe must
  also verify that both `temporal` and `temporal_visibility` exist, because
  `/docker-entrypoint-initdb.d` scripts run only for an empty data volume.
- The disposable Compose smoke must wait for every long-running service to be
  healthy and assert successful exit codes for `migrate`, `minio-init`, and
  `cutover`. Logging only application services is insufficient to diagnose a
  PostgreSQL or Temporal bootstrap failure.

### Common Mistake

```yaml
# Wrong: Compose creates a host directory when the SQL file is absent.
- ../postgres/10-temporal-databases.sql:/docker-entrypoint-initdb.d/10-temporal-databases.sql:ro
```

```yaml
# Correct: the deployment fails deterministically and the preflight reports
# the missing source before Docker creates any containers or volumes.
- type: bind
  source: ../postgres/10-temporal-databases.sql
  target: /docker-entrypoint-initdb.d/10-temporal-databases.sql
  read_only: true
  bind:
    create_host_path: false
```
