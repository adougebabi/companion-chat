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

- API and Worker share one Go module/image with separate commands. Only Worker processes poll application task queues.
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

## Scenario: Go Worker Queue Ownership And Cognition Intent

### 1. Scope / Trigger

- Trigger: a committed cognition fact is dispatched after API acceptance or a
  Go Worker starts its canonical Temporal queues.

### 2. Signatures

- `cognition.processing` maps to `CognitionProcessingWorkflow` on
  `interaction`; `platform.control` maps to `PlatformControlWorkflow` on
  `lifecycle`.
- `ProcessCognitionInbox` reuses the immutable inbox/message idempotency keys
  and returns the prior result for a processed fact.

### 3. Contracts

- Exactly one Worker poller owns each of `interaction`, `lifecycle`, and
  `media`; workflow/activity registration is queue-specific.
- Conversation fact commit writes a stable cognition workflow intent before
  external processing. API may still return its existing product response;
  Worker replay must not duplicate the side effect.
- Reconciliation scans pending/retry/started intents and maps Temporal terminal
  states to explicit durable status.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| duplicate cognition workflow start | Temporal stable ID/reuse guard; no second assistant effect |
| processed inbox replay | return `processed` without Provider call |
| Worker queue unavailable | intent remains retryable; no false completion |
| Temporal terminal failure | durable intent becomes failed and remains auditable |

### 5. Good/Base/Bad Cases

- Good: API commits fact, Worker processes `cognition.processing`, and a
  restart reconciles the completed intent with no duplicate message.
- Base: a pending media/schedule intent remains visible while its queue is
  unavailable and resumes later.
- Bad: register every workflow on every queue, process a fact from a different
  Fluctlight, or mark an intent complete before Temporal accepts it.

### 6. Tests Required

- Workflow registry/replay tests for cognition and platform control.
- Queue ownership assertions, stable-ID duplicate starts, restart/reconcile
  tests and a real Docker cognition intent sweep.

### 7. Wrong vs Correct

#### Wrong

```go
for _, queue := range queues {
    registerEveryWorkflow(queue)
}
```

#### Correct

```go
registerLifecycleWorkflows(lifecycleWorker)
registerInteractionWorkflows(interactionWorker)
registerMediaWorkflows(mediaWorker)
```

## Scenario: Cognition Claim Cleanup And Stream Retry

### 1. Scope / Trigger

- Trigger: a synchronous NDJSON conversation or the Worker-owned
  `cognition.processing` activity claims a `cognition_inbox` row and any later
  provider, validation, realization, or persistence step fails.

### 2. Signatures

- `enqueueTurnFactClaimed(...) -> (inboxID, claimOwner, error)` keeps the
  stream claim owner available to the caller.
- `releaseCognitionClaim(ctx, inboxID, claimOwner) -> error` conditionally
  releases only the matching active claim.
- `ProcessCognitionInbox(ctx, inboxID)` releases its claim before returning an
  error so Temporal activity retry can reclaim the fact.

### 3. Contracts

- A failed claim becomes `pending` with `claimed_by` and `claimed_at` cleared;
  it remains eligible for the durable retry path.
- If the assistant message for the same `turn_id` is already persisted, claim
  cleanup settles the inbox as `processed` and clears the lease instead of
  leaving a half-completed turn permanently claimed.
- Claim cleanup is conditional on the lease owner. A late cleanup from an old
  request must never clear a newer request's claim.
- The Core error log includes `conversation_id`, `fluctlight_id`, `turn_id`,
  and `idempotency_key` so a failed turn can be correlated with its inbox and
  workflow intent without exposing credentials.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Provider/validation failure after a stream claim | Release the matching claim to `pending`; preserve the original error for the caller |
| Worker activity failure after its claim | Release the matching claim before Temporal retry; do not convert the retry into `resource revision conflict` |
| Assistant message already exists during cleanup | Set inbox to `processed`, clear lease, preserve idempotent replay |
| Cleanup runs with a different/late owner | Conditional update affects no newer claim |
| Active claim has no committed assistant result | A concurrent duplicate still receives the bounded conflict until the lease is released or expires |

### 5. Good/Base/Bad Cases

- Good: a provider failure releases the inbox, the next activity attempt
  claims it, and a browser retry reuses the same fact without duplication.
- Base: a disconnected stream leaves a durable pending fact that the Worker
  can recover after the claim cleanup or lease expiry.
- Bad: reset only `cognition_inbox.status` while leaving a started workflow or
  frozen action unexplained, or let a failed activity retain its own claim
  until the ten-minute lease expires.

### 6. Tests Required

- PostgreSQL integration test: an owned claimed inbox with no assistant is
  released to `pending` with an empty lease.
- PostgreSQL integration test: an owned claimed inbox with a committed
  assistant is settled to `processed` with `processed_at` and an empty lease.
- Cognition activity failure/retry test: the retry does not return
  `resource revision conflict` solely because the previous attempt failed.
- Real conversation regression: force a provider/realization failure, retry
  the same `turn_id` and `idempotency_key`, and assert one assistant message
  and one processed inbox.

### 7. Wrong vs Correct

#### Wrong

```go
payload, _, err := claimCognitionInbox(ctx, inboxID)
if err != nil {
    return nil, err
}
return a.HandleTurn(ctx, actorID, conversationID, data)
// A later failure leaves the inbox claimed forever until lease expiry.
```

#### Correct

```go
claimOwner := "go-cognition:" + randomID("worker_")
payload, _, err := claimCognitionInbox(ctx, inboxID, claimOwner)
if err != nil {
    return nil, err
}
defer releaseCognitionClaim(ctx, inboxID, claimOwner)
return a.HandleTurn(ctx, actorID, conversationID, data)
```

## Scenario: Worker Deployment Routing Bootstrap

### 1. Scope / Trigger

- Trigger: a Go Worker starts or restarts against a new or existing Temporal
  namespace with Worker Deployment Versioning enabled.

### 2. Signatures

- `EnsureWorkerDeploymentCurrentVersion(ctx, deploymentHandle, buildID)` sets
  the `fluctlight` Deployment current version to the Worker Build ID.

### 3. Contracts

- Worker starts all canonical queue pollers (`interaction`, `lifecycle`,
  `media`) before attempting the Deployment update.
- Startup retries the idempotent `SetCurrentVersion(BuildID)` operation while
  Temporal discovers the pollers; it does not bypass the no-poller or missing
  task-queue protections.
- The Worker writes its readiness signal only after the current version is
  accepted. A fresh namespace therefore routes new and AutoUpgrade workflows
  to `fluctlight:<build_id>` instead of accumulating `UNVERSIONED` workflow
  tasks.
- Re-running bootstrap when the requested version is already current is a
  successful no-op. A failed bootstrap is a startup/readiness failure, not a
  reason to mark domain workflow intents completed.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Poller discovery is still in progress | Retry with bounded backoff; keep Worker unready |
| Deployment already targets the requested Build ID | Return success; do not create another Deployment |
| Target Build ID has no observed poller | Fail/retry without `AllowNoPollers` |
| Expected queue is missing | Fail/retry without `IgnoreMissingTaskQueues` |
| Temporal control API is unavailable | Worker exits or remains unready; domain intents remain retryable |

### 5. Good / Base / Bad Cases

- Good: a clean Temporal namespace starts a `platform-v1` Worker, bootstrap
  sets `fluctlight/platform-v1`, and the first schedule workflow advances past
  its initial workflow task.
- Base: a restart observes `platform-v1` already current and immediately
  becomes ready.
- Bad: expose a healthy Worker before routing is set, leave new tasks in the
  unversioned queue, or require an operator to run a CLI command after every
  rebuild.

### 6. Tests Required

- Unit-test retry, idempotency, empty Build ID, cancellation and no-poller
  errors through the narrow Temporal control seam.
- Fresh-namespace integration test asserts Deployment current version,
  versioned pollers on all three queues, and a schedule workflow that reaches
  `EnsureCurrentDayScheduleActivity`.
- Restart test asserts bootstrap is safe when the Deployment is already
  current and does not duplicate workflow intents.

### 7. Wrong vs Correct

#### Wrong

```text
Start versioned Worker → accept API traffic → ask operator to set current version
```

#### Correct

```text
Start queue pollers → idempotently set current version → write readiness → accept traffic
```

## Scenario: Bounded, Restart-Safe Intent Dispatch

### 1. Scope / Trigger

- Trigger: a Worker restarts while PostgreSQL contains historical and newly
  committed workflow intents.
- This rule applies to `CommittedIntentDispatcher.dispatch_once()` and all
  Temporal starts during the legacy→Go Core migration.

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

## Scenario: Long-Lived Wake-Up Workflow

### 1. Scope / Trigger

- Trigger: an active Fluctlight is activated or a Worker resumes its stable
  `wake_up:<fluctlight_id>` execution.
- Purpose: periodically create one internal cognition fact without adding a
  second scheduler, delayed queue, or process-local timer.

### 2. Signatures

```text
intent_type: wake_up.current
task_queue: lifecycle
payload: {fluctlight_id: string, cycle: integer}
workflow: WakeUpWorkflow -> ProcessWakeUpActivity -> ContinueAsNew
```

### 3. Contracts

- Activation commits only the stable `schedule.current_day` intent alongside
  the Fluctlight aggregate. Once that current-day Schedule is accepted, the
  acceptance transaction creates the stable `wake_up.current` and
  `daily_review.current_day` intents. This makes the first cognition causally
  downstream of an accepted Schedule rather than relying on Temporal's queue
  ordering.
- Worker startup idempotently backfills `wake_up.current` for existing
  `active`/`paused` Fluctlights only when the current local-day Schedule is
  already accepted. Existing `pending`, `retry`, `started`, and
  `cancel_requested` intents are preserved; failed/completed intents for
  still-live Fluctlights become retryable without creating a second workflow
  ID.
- `WakeUpWorkflow` sleeps for the interval returned by Core and increments the
  cycle only through `ContinueAsNew`; the workflow ID stays stable while each
  cycle's fact/action/reflection IDs are derived from `(fluctlight_id, cycle)`.
- The Worker registers the workflow and activity only on `lifecycle`, and the
  dispatcher treats `wake_up.current` as a lifecycle intent with the same
  retry/reconcile semantics as other Go workflows.
- A terminal failed/completed wake-up workflow for an `active`/`paused`
  Fluctlight is requeued after reconciliation with a bounded delay. A
  deliberate cancellation or a `retired` Fluctlight is not automatically
  restarted.
- Reconciliation does not inspect a `retry` intent before its
  `next_attempt_at`; otherwise each polling pass can push the bounded retry
  window forward forever. Wake-up recovery uses Temporal's
  `ALLOW_DUPLICATE` policy for the stable workflow ID because both failed and
  completed terminal executions may be requeued; the per-Fluctlight cycle key
  remains the idempotency boundary.
- A wake-up that proposes a Capability tool call freezes a generic
  `capability.action` on `interaction`; the CapabilityActionWorkflow reuses the
  same stable action/lease/result/reflection boundary as legacy autonomy
  actions, so the wake-up activity never executes an external effect directly.
- Disabled or inactive results still use the durable timer; inactive results
  terminate, while disabled results sleep and re-read settings on the next
  cycle.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unsupported intent type or queue | Leave the intent pending and emit a bounded warning; never start another runtime |
| Empty/negative cycle | Activity rejects the input without writing a cognition fact |
| Temporal start failure | Keep the intent retryable; do not mark it complete |
| Worker restart after a start but before status update | Reuse the stable workflow ID and let reconciliation repair the ledger |
| Existing live Fluctlight has no wake-up intent | Worker startup inserts the stable `wake_up.current` intent idempotently |
| Wake-up activity/provider failure | Reconcile requeues the live Fluctlight's intent after a bounded delay; preserve the failure in diagnostics |
| Assessment returns a chat-only action without a capability call | Preserve the internal stages and persist the external choice as a bounded `no_op`; do not terminate the long-lived timer |
| Wake-up is cancelled or Fluctlight is retired | Do not auto-restart the workflow |
| History grows across cycles | Continue-As-New preserves the Fluctlight/cycle identity and bounds history |

### 5. Good/Base/Bad Cases

- Good: a Worker restart resumes one stable wake-up timer and the next cycle
  allocates one cognition sequence.
- Base: settings disable internal life; the workflow remains inspectable and
  wakes again only after the bounded interval.
- Bad: use a Go `time.Ticker`, Redis delayed stream, or a new Temporal Schedule
  client that can outlive the domain intent ledger.

### 6. Tests Required

- Assert registry/dispatcher queue mapping, stable IDs, retry behavior, and
  lifecycle-only registration.
- Assert activation creates only the schedule intent, Schedule acceptance
  creates wake-up/daily-review intents atomically, and the first wake-up
  timestamp follows Schedule acceptance.
- Assert Worker startup backfills an existing live Fluctlight and requeues a
  terminal wake-up intent without duplicating the stable workflow ID.
- Assert a retry with a future `next_attempt_at` is not requeued on every
  reconciliation poll, and a due retry starts a new Temporal run with the
  stable wake-up ID.
- Assert a shared cognitive-assessment response such as `reply` cannot kill a
  Wake-up cycle when no capability tool call is present.
- Assert the interval clamp and Continue-As-New cycle increment with Temporal's
  workflow test environment.
- Assert inactive termination and disabled sleep behavior without provider
  calls.

### 7. Wrong vs Correct

#### Wrong

```go
go func() {
	for range time.NewTicker(interval).C {
		app.ProcessWakeUp(ctx, fluctlightID, 0)
	}
}()
```

#### Correct

```go
workflow.ExecuteActivity(ctx, ProcessWakeUpActivity, input).Get(ctx, &result)
workflow.Sleep(ctx, wakeUpInterval(result))
return nil, workflow.NewContinueAsNewError(ctx, WakeUpWorkflow, nextInput)
```
