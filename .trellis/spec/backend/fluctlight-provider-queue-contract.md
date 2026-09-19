# Fluctlight Provider Queue Contract

## 1. Scope / Trigger

Every Core Provider HTTP request creates one durable diagnostic model-run row
before execution and enters the configured generated/embedding queue. The
queue has an in-process priority/FIFO executor plus an optional Redis
cross-process coordinator. The coordinator is an acceleration/lease layer;
domain workflows, PostgreSQL diagnostics, and Temporal still own retries,
idempotency, and side effects.

## 2. Signatures

- `ProviderClient.Structured*`, `Text`, `StreamText`, and `Embed` submit a
  `ProviderQueue` task and return the same normalized result/error as before.
- `runtime_settings["llm.queue"]` accepts
  `{generated_concurrency: integer, embedding_concurrency: integer}`.
- `GET /api/diagnostics/model-runs` returns `bindingRole`, `scenario`,
  `priority`, `queuedAt`, `startedAt`, and `completedAt` in addition to the
  existing model-run fields.
- Redis keys, when configured: `fluctlight:llm:<binding>:pending`,
  `fluctlight:llm:<binding>:processing`, `fluctlight:llm:<binding>:sequence`,
  and short-lived `fluctlight:llm:job:<id>` hashes.

## 3. Contracts: binding and scenario boundary

- Browser settings expose only `generic_llm` and `embedding` model bindings.
- Domain call sites retain explicit scenarios (`reply`,
  `cognitive_assessment`, `native_cognition`, `media_prompt`, `reflection`,
  `wake_up`, `initialization`, `daily_review`, `schedule_generation`, and
  `embedding`). The scenario is diagnostic metadata, not a model binding.
- Existing role-named rows are migrated to `generic_llm` with deterministic
  precedence and remain readable as a compatibility fallback; embedding is
  never used as a generative fallback.
- `diagnostic_model_runs.role` stores the semantic caller role while
  `binding_role` stores `generic_llm` or `embedding`; a shared binding must not
  erase the scenario's semantic role.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Binding role is not `generic_llm` or `embedding` | Reject configuration; legacy role-shaped clients are normalized to `generic_llm` only for compatibility. |
| Queue concurrency is missing | Use generated=1 and embedding=1. |
| Queue concurrency is non-integer or outside 1–8 | Reject settings; keep previous values. |
| Same-priority requests | FIFO by enqueue sequence. |
| Queue/HTTP context is cancelled | Mark cancelled, release the slot, and never block later requests. |
| Process restarts with stale queued/running row | Mark failed with `provider_process_restarted`; owning workflow may retry. |

## 5. Contracts: ordering and lifecycle

- Generated requests and embedding requests have separate queues and limits.
  Defaults are generated=1 and embedding=1; each setting is clamped to 1–8.
- Generated priority is interactive `reply` and conversation
  `cognitive_assessment` (100), native/daily-review/plan (90), media prompt
  (80), reflection/wake-up (70), initialization (60). Interactive cognition
  must outrank background schedule generation so a private chat cannot be
  starved by lifecycle work. A priority heap uses enqueue sequence as the tie
  breaker, so equal priorities are FIFO.
- A run transitions `queued → running → completed|failed|cancelled|timeout`.
  `queued_at`, `started_at`, `completed_at`, scenario, priority, binding role,
  model, and correlation ID stay on one row. Prompt/response remain redacted.
- Cancelling while queued removes the task; cancelling while running reaches
  the HTTP request context and releases the slot. A crashed process's stale
  queued/running rows are marked failed with `provider_process_restarted`.
- Redis claim moves a short-lived job from pending to processing atomically,
  renews its lease while the local provider closure runs, and removes it only
  when the same owner releases it. Redis errors fall back to the local queue.
- Pending jobs also carry a short lease and caller heartbeat. Claim and
  reconciliation remove legacy or expired pending members (including hashes
  left by a crashed process) so one orphan cannot block every later request
  until the long job-hash TTL expires. A successful Redis `Expire` command must
  be checked through its `.Err()` result; a command object is not an error.

## 6. Good / Base / Bad Cases

- Good: a reply or interactive cognition (priority 100) starts ahead of an
  older schedule-generation job (90) or reflection (70), while two embedding
  jobs use their own single slot.
- Base: a legacy `action_realization` row is copied to `generic_llm`; the
  diagnostic still says `scenario=reply` and the actual model ID.
- Bad: an embedding row is used as the generative fallback, or a queued row is
  inserted only after the provider has returned.

## 7. Tests Required

`runtime_settings["llm.queue"]` stores:

```json
{"generated_concurrency": 1, "embedding_concurrency": 1}
```

Unknown keys, non-integers, and values outside 1–8 are rejected. Missing values
use the corresponding default. Changes apply to subsequent requests without a
process restart.

- Unit tests cover priority/FIFO, both queue limits, cancellation, timeout and
  slot release.
- Provider/diagnostic tests cover scenario persistence, lifecycle updates,
  generic-role compatibility, and redaction.
- Redis coordinator tests cover score ordering, atomic claim/release, lease
  renewal/requeue, orphan-job cleanup, cancellation, and unavailable-Redis
  fallback. Integration tests use a disposable Redis when local listeners are
  permitted.
- Core, browser boundary, browser-client, and Web contracts must be updated together when a
  model-run field changes.

## Wrong vs Correct

### Wrong

```go
modelRoles.require("action_realization")
```

This makes every business scenario a separate user-facing binding and cannot
explain which scene triggered a shared model.

### Correct

```go
assignment := modelRoles.require("generic_llm")
diagnostic.scenario = "reply"
```

The binding is stable while the diagnostic preserves the actual trigger.

## Scenario: Redis-backed cross-process coordination

### 1. Scope / Trigger

- Trigger: API and Worker processes must share generated-model priority and
  concurrency without moving provider closures or domain facts into Redis.

### 2. Signatures

- `ProviderClient.SetRedisClient(client, processID)` enables optional
  coordination.
- `ProviderClient.ReconcileRedisQueue(ctx, role)` requeues expired leases and
  deletes orphaned job references.
- `acquireProviderRedisSlot(ctx, role, priority, limit, diagnosticID)` returns
  an owner-checked release function, an enabled flag, and a bounded error.

### 3. Contracts

- Lower Redis scores run first: `(100-priority)*1_000_000_000_000 + global_sequence`.
- Pending members are short-lived job IDs; job hashes contain references and
  metadata only, never provider secrets or business result payloads.
- Lua claim/release/requeue operations are atomic. A processing lease is
  renewed periodically and the hash has a TTL; a crashed process cannot leave
  an immortal slot.
- Redis connection/command failure does not fail a synchronous provider call;
  it falls back to the existing local queue and PG lifecycle diagnostics.
- Redis does not replace `platform_workflow_intents`, Temporal queues, or
  diagnostic persistence.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Redis unavailable or script unsupported | Use local queue; do not block the provider call on Redis. |
| Equal priority | Global sequence preserves FIFO. |
| Lease expires | Requeue only when the job hash still exists; orphan members are removed. |
| Owner mismatch on release | No other owner's processing slot is deleted. |
| Context cancelled while pending | Remove pending/processing member and mark the model run cancelled. |

### 5. Good / Base / Bad Cases

- Good: two Worker processes claim disjoint slots, and a reply priority 100
  starts before an older reflection priority 70.
- Base: Redis restarts; the next claim/reconciliation cleans stale members and
  PG stale-run recovery remains authoritative.
- Bad: use `ZPOPMIN` without a processing lease, store a non-recoverable closure
  in Redis, or fail all synchronous requests because Redis is briefly down.

### 6. Tests Required

- Assert score priority/FIFO, separate embedding keys, owner-checked release,
  lease renewal, expired requeue, orphan cleanup, cancellation and fallback.
- Run a disposable multi-client Redis test where available; otherwise retain
  deterministic unit coverage and record the environment limitation.

### 7. Wrong vs Correct

#### Wrong

```go
id := redis.ZPopMin(ctx, pendingKey).Val()
callProvider(loadClosure(id)) // crash here permanently loses the task
```

#### Correct

```go
claimWithLease(ctx, pendingKey, processingKey, jobID)
defer releaseOwnedLease(ctx, processingKey, jobID, owner)
callLocalProviderClosure()
```

## Scenario: Per-call queue leases inside an ADK loop

### 1. Scope / Trigger

- Trigger: one direct conversation uses ADK to perform more than one ChatModel
  generation after a tool call.

### 2. Signatures

```go
runProviderQueued(ctx, role, scenario, priority, diagnosticID, func(ctx) (T, error))
queuedToolCallingChatModel.Generate(ctx, messages, opts...) (*schema.Message, error)
```

### 3. Contracts

- The outer conversation orchestration must not hold a generated-model queue
  lease across the entire Agent loop.
- Each ADK `Generate`/`Stream` call enters `runProviderQueued` independently;
  the next model call waits for a fresh local/Redis lease.
- The request keeps one correlation/turn identity, but each underlying call
  remains cancellation-, timeout- and lifecycle-diagnosable.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| ADK makes two model generations | Two queue acquire/release cycles; no long-lived lease |
| Tool callback is slow | The current model-call lease is released only after that call; no second model call is admitted on the same lease |
| Redis unavailable | Each call falls back to the local queue independently |
| Context cancelled between calls | No new queue claim; the Agent returns a cancellation error |

### 5. Good/Base/Bad Cases

- Good: model A → query tool → model B produces two bounded model-run states
  and two queue leases.
- Base: a tool-only turn makes one model claim and settles without a second
  claim.
- Bad: wrap `Runner.Run` in one `runProviderQueued` callback and let all ADK
  generations execute under that single permit.

### 6. Tests Required

- Fake ChatModel test counts queue claims/releases for a tool loop and asserts
  cancellation before the second claim.
- Redis coordinator test asserts each ADK call has an owner-checked lease and
  no orphan processing member remains after a model failure.

### 7. Wrong vs Correct

#### Wrong

```go
runProviderQueued(ctx, "generic_llm", "conversation", 100, id, func(ctx context.Context) error {
	return runner.Run(ctx, messages) // includes all model/tool/model iterations
})
```

#### Correct

```go
// Runner has no queue lease; the Eino ChatModel adapter claims one per call.
queuedModel := &queuedToolCallingChatModel{inner: chatModel, provider: provider}
runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
```

## Scenario: Physical Provider Attempts Inside an ADK Loop

### 1. Scope / Trigger

- Trigger: one logical structured task performs more than one bounded ADK
  model generation.

### 2. Contracts

- The outer ADK runner does not hold one queue lease for the whole loop. Each
  physical `Generate`/`Stream` call acquires and releases the normal generated
  queue independently.
- Each physical call receives a fresh Provider attempt identity, request ID
  and correlation suffix while retaining the logical scenario/correlation.
  Cancellation, timeout and Redis lifecycle markers reach the current call and
  release its queue slot.
- Each physical call creates its own `diagnostic_model_runs` lifecycle row
  when PostgreSQL diagnostics are available. A later model generation must not
  overwrite the first attempt's identity or usage.
- Provider queue bypass is permitted only around the ADK runner's outer
  wrapper; the `queuedToolCallingChatModel` is the re-entry point that restores
  the ordinary queue and diagnostics boundary.

### 3. Tests Required

- Assert a two-generation WakeUp/Conversation loop produces two request IDs,
  independent queue lifecycles and no lease held across tool execution.
- Assert cancellation/failure of one physical call releases the slot and does
  not become a successful no-op or block a later request.
