# Fluctlight Durable Workflow Contract

## Scenario: One Durable Runtime With Domain-Owned State

### 1. Scope / Trigger

- Trigger: the clean-start system delays, schedules, retries, cancels, resumes, compensates, upgrades, or administratively repairs background work.
- This contract is runtime-neutral. T01 rejected DBOS; Temporal is the current candidate and must pass `fluctlight-temporal-gate-contract.md` before T02.
- Workflow history executes application processes; it never replaces PostgreSQL domain facts, outbox/inbox, Redis event transport, or media metadata.

### 2. Signatures

```python
start_workflow(intent: CommittedWorkflowIntent) -> WorkflowHandle
signal_workflow(command: WorkflowSignalCommand) -> None
query_workflow(query: WorkflowQuery) -> WorkflowView
update_workflow(command: WorkflowUpdateCommand) -> UpdateResult
cancel_workflow(command: CancelWorkflow) -> CancelResult
restart_or_reset(command: RepairWorkflow) -> WorkflowHandle
```

Application task queues are `interaction`, `lifecycle`, and `media`. Every start uses a stable workflow ID derived from a committed domain intent/idempotency key. Node never accesses the workflow runtime directly.

### 3. Contracts

- API and Worker share one Python package/image with separate commands. Only Worker processes poll application task queues.
- A domain transaction writes business state plus stable workflow intent/outbox before runtime start.
- Domain status remains queryable without interpreting runtime history. Runtime stores execution history, timers, retries and management state only.
- Task queues have independent concurrency/rate policies; Provider-specific limits are explicit.
- External Provider/object actions use stable request IDs and idempotent lookup/result recovery. Runtime replay alone does not make arbitrary external effects exactly once.
- Long activities configure heartbeat, timeout, cooperative cancellation and bounded shutdown. Stale/cancelled executions cannot commit domain results.
- Durable timers express pending events, delayed replies and lifecycle schedules without Redis delayed queues.
- Runtime must support list/get, durable pause/resume semantics, cancel, retry/restart/reset or fork-from-checkpoint, and authorized/audited repair through application commands.
- Workflow code/history upgrades require deterministic replay tests, explicit Worker deployment/version routing and rollback/drain procedures; old history cannot be abandoned silently.
- Long-lived histories use runtime-supported history rollover/continue-as-new policy before limits are approached.
- Workflow/Activity/Provider/outbox/inbox/Fluctlight/correlation IDs appear in built-in diagnostics and structured logs.
- Exactly one workflow runtime is allowed. Celery, custom queues, Redis delayed work and concurrent old/new workflow engines are prohibited.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Start lacks committed stable intent/ID | Reject; create domain intent first. |
| Duplicate start uses same business/workflow ID | Return/reuse existing execution; no duplicate external effect. |
| Long Activity misses heartbeat/timeout | Retry/cancel by policy; stale execution cannot commit result. |
| Cancellation requested | Audit, propagate cooperatively, and settle domain state explicitly. |
| Worker/runtime/DB restarts during timer/activity | Resume from durable history and stable IDs. |
| Provider succeeds before local result commit | Recover by stable Provider request ID and persist existing result. |
| Current code cannot replay active history | Release/gate fails; keep compatible Worker or rollback, never strand history. |
| History nears runtime limits | Continue/roll over with explicit state and stable business identity. |
| Unauthorized management command | Reject without runtime mutation. |
| Second workflow runtime dependency appears | Architecture failure; remove it or return parent planning. |

### 5. Good / Base / Bad Cases

- Good: a long fake h3 Activity heartbeats, is cancelled/restarted, reuses its Provider result, and commits one media outcome.
- Good: a delayed reply survives service/PostgreSQL/Worker restart and delivers one frozen Action.
- Good: old Event History replays under new code or remains routed to a compatible Worker until continue-as-new/drain completes.
- Base: a due workflow finds its domain Intention closed and completes `no_op` without effect.
- Bad: put workflow truth in Redis PEL, start before intent commit, generate new Provider IDs on retry, deploy incompatible Workflow code without replay tests, or run two engines.

### 6. Tests Required

- Runtime gate report covering topology, three queues, timers, long activity, heartbeat, timeout, cancel, restarts, stable IDs, management operations, history replay/versioning, backup/restore, resource/disk growth and correlation.
- Contract tests for committed intent, stable IDs, duplicate start, frozen decision, idempotent Activity replay and domain-status separation.
- Queue tests for independent concurrency/rate limits and default one-Worker application topology.
- Failure injection at every external/checkpoint/result boundary plus runtime/PostgreSQL/Worker restarts.
- Authorization/audit tests for query, pause/resume, cancel, restart/reset/repair.
- Saved-history replay tests and old/new Worker deployment/version compatibility tests before every Workflow code change.
- Continue-as-new/history rollover tests preserving business identity, domain links and pending signals.
- Architecture test fails if another workflow/task/delayed queue runtime enters production dependencies.

### 7. Wrong vs Correct

#### Wrong

```python
await redis.xadd("delayed-jobs", payload)
await celery.send_task("generate_media", args=[payload])
```

#### Correct

```python
async with unit_of_work.begin(command_id=intent_id) as tx:
    media.create_intent(command, tx=tx)
    tx.outbox.add(MediaWorkflowRequested(workflow_id=intent_id))
    await tx.commit()

handle = await workflow_runtime.start_workflow(committed_intent)
```

## Scenario: Bounded, Restart-Safe Intent Dispatch

### 1. Scope / Trigger

- Trigger: a Worker restarts while PostgreSQL contains historical and newly
  committed workflow intents.
- This rule applies to `CommittedIntentDispatcher.dispatch_once()` and all
  Temporal starts during the Python→Go Core migration.

### 2. Signatures

```python
dispatch_once(*, limit: int = 20) -> int
commit_workflow_intent(session, intent: CommittedWorkflowIntent) -> intent
```

### 3. Contracts

- One dispatch pass selects at most `limit` intents and excludes intent IDs
  already attempted by the current process.
- Ordering prioritizes `daily_review.*`, `schedule.*`, `autonomy.*`, media and
  reflection so historical reflection work cannot starve fresh lifecycle work.
- A process restart may attempt an old intent again; its stable `workflow_id`
  makes Temporal return `WorkflowAlreadyStartedError`, which is treated as a
  successful replay guard.
- Unsupported task queues remain unstarted and produce a bounded operational
  error. No second Worker/runtime is created as a fallback.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| `limit <= 0` | Clamp to one intent; never scan an unbounded page. |
| Already attempted intent in this process | Exclude from the next query. |
| Worker restarted before durable dispatch state exists | Retry stable workflow ID; reuse/AlreadyStarted is success. |
| Temporal start fails transiently | Leave intent eligible for a later pass; do not mark it complete. |
| Fresh daily-review intent behind old reflection rows | Priority order selects lifecycle work first. |

### 5. Good / Base / Bad Cases

- Good: a newly activated Fluctlight's daily review and autonomy intents are
  dispatched while old reflection rows remain pending.
- Base: a restart retries an already running workflow and Temporal deduplicates
  it by stable ID.
- Bad: select the first 20 rows forever after process-local skipping, or scan
  the entire table on every one-second tick.

### 6. Tests Required

- Assert one dispatch pass never calls `start_workflow` more than `limit` times.
- Assert process-local attempted IDs are excluded while a restart still handles
  `WorkflowAlreadyStartedError`.
- Assert priority ordering puts daily-review/autonomy before reflection.
- Assert transient start failure remains retryable and does not create a second
  workflow ID.

### 7. Wrong vs Correct

#### Wrong

```python
select(workflow_intents).limit(20)  # always the same historical rows
if intent_id in self._started:
    continue
```

#### Correct

```python
statement = select(workflow_intents)
if self._started:
    statement = statement.where(workflow_intents.c.intent_id.not_in(self._started))
rows = await session.execute(priority_order(statement).limit(limit))
```
