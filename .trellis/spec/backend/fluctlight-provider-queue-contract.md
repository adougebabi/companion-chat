# Fluctlight Provider Queue Contract

## 1. Scope / Trigger

Every Core Provider HTTP request goes through an in-process priority queue and
creates one durable diagnostic model-run row before execution. The queue is a
transport concern: domain workflows still own retries, idempotency, and side
effects.

## 2. Signatures

- `ProviderClient.Structured*`, `Text`, `StreamText`, and `Embed` submit a
  `ProviderQueue` task and return the same normalized result/error as before.
- `runtime_settings["llm.queue"]` accepts
  `{generated_concurrency: integer, embedding_concurrency: integer}`.
- `GET /api/diagnostics/model-runs` returns `bindingRole`, `scenario`,
  `priority`, `queuedAt`, `startedAt`, and `completedAt` in addition to the
  existing model-run fields.

## 3. Contracts: binding and scenario boundary

- Browser settings expose only `generic_llm` and `embedding` model bindings.
- Domain call sites retain explicit scenarios (`reply`,
  `cognitive_assessment`, `native_cognition`, `media_prompt`, `reflection`,
  `wake_up`, `initialization`, `daily_review`, `schedule_generation`, and
  `embedding`). The scenario is diagnostic metadata, not a model binding.
- Existing role-named rows are migrated to `generic_llm` with deterministic
  precedence and remain readable as a compatibility fallback; embedding is
  never used as a generative fallback.

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
