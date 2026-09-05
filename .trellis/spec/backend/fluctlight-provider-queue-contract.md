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
| Queue concurrency is missing | Use generated=2 and embedding=1. |
| Queue concurrency is non-integer or outside 1–8 | Reject settings; keep previous values. |
| Same-priority requests | FIFO by enqueue sequence. |
| Queue/HTTP context is cancelled | Mark cancelled, release the slot, and never block later requests. |
| Process restarts with stale queued/running row | Mark failed with `provider_process_restarted`; owning workflow may retry. |

## 5. Contracts: ordering and lifecycle

- Generated requests and embedding requests have separate queues and limits.
  Defaults are generated=2 and embedding=1; each setting is clamped to 1–8.
- Generated priority is `reply` (100), cognitive/native/daily-review/plan (90),
  media prompt (80), reflection/wake-up (70), initialization (60). A priority
  heap uses enqueue sequence as the tie breaker, so equal priorities are FIFO.
- A run transitions `queued → running → completed|failed|cancelled|timeout`.
  `queued_at`, `started_at`, `completed_at`, scenario, priority, binding role,
  model, and correlation ID stay on one row. Prompt/response remain redacted.
- Cancelling while queued removes the task; cancelling while running reaches
  the HTTP request context and releases the slot. A crashed process's stale
  queued/running rows are marked failed with `provider_process_restarted`.
- Redis claim moves a short-lived job from pending to processing atomically,
  renews its lease while the local provider closure runs, and removes it only
  when the same owner releases it. Redis errors fall back to the local queue.

## 6. Good / Base / Bad Cases

- Good: a reply (priority 100) starts ahead of an older reflection (70), while
  two embedding jobs use their own single slot.
- Base: a legacy `action_realization` row is copied to `generic_llm`; the
  diagnostic still says `scenario=reply` and the actual model ID.
- Bad: an embedding row is used as the generative fallback, or a queued row is
  inserted only after the provider has returned.

## 7. Tests Required

`runtime_settings["llm.queue"]` stores:

```json
{"generated_concurrency": 2, "embedding_concurrency": 1}
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
- Core, BFF, browser-client, and Web contracts must be updated together when a
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
