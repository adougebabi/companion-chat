# Structured Turn Contract

## Scenario: Structured control alongside streamed visible chat

### 1. Scope / Trigger

- Trigger: a chat completion must carry durable memory, affect/drives signals, or capability intent without asking application code to infer meaning from user-visible prose.
- The browser transport remains the existing `token`/`done`/`error` SSE contract; this spec governs the provider-to-application boundary and commit behavior.

### 2. Signatures

- Provider normalized completion: `{text, tokens, toolCalls, structuredTurn?, control?, parseErrors?, doneSeen}`.
- Canonical turn schema: `schemaVersion: 'companion.turn.v1'` with `control.affectEvents[]`, `control.driveSignals[]`, `control.memoryWrites[]`, and `control.capabilityCalls[]`.
- Supported first-release drives: `social`, `exploration`, `rest`; pressure is `0..1`, where higher means more unmet need.
- Memory capability: `memory_event({memory: {operation, key, value, confidence, sourceMessageId?, idempotencyKey}})`.
- Appearance capability: `appearance_event({operation: 'set'|'clear', outfit?, reason?})`; it is persona-scoped, source-message-bound, idempotent, and persists the current outfit in the normalized state projection while retaining an auditable `appearance_change` life event.
- State tools: `affect_event({event: {type, confidence, idempotencyKey}})` and `drive_signal({signal: {drive, direction, confidence, idempotencyKey}})`; the server owns numeric deltas.
- Native transport tools are defined once by `server/application/capability-catalog.js`. The current catalog exposes all seven universal tools on every chat request in stable order; capability filtering is intentionally deferred.
- Affect persistence: `companion_persona_affect_states` materialized snapshot plus `companion_persona_affect_events` append-only events, unique on `(persona_id, idempotency_key)`.

### 3. Contracts

- Visible text may stream from provider `content`; structured controls are accumulated and validated before any side effect is applied.
- Native tool calls, parsed provider sidecars, and legacy media/pending markers are normalized at one application boundary. New affect/memory behavior must not add text markers.
- Machine-readable argument shape belongs to the canonical capability catalog and provider `tools` payload. The model-facing system prompt contains only short behavioral guidance; it must not duplicate JSON schema bounds, dispatcher internals, or legacy marker syntax. Flow validators remain authoritative for ownership, time windows, policy, idempotency, and transactions.
- Native-capable providers receive the catalog directly. Legacy marker adapters remain compatibility fallbacks and must not be advertised in the normal prompt. A future provider-specific capability profile may filter tools, but the current base implementation sends the universal catalog unchanged.
- Scene and appearance are separate facts. When a scene transition also changes clothing, the model may issue one `scene_event` and one `appearance_event` in the same turn; an explicit clothing change in an unchanged scene may issue only `appearance_event`. Ordinary prose or transient gestures never update clothing state.
- `memory_event` is the only ordinary-chat path to long-term memory. It is persona-private, source-message-bound, idempotent, and committed with the assistant facts when the turn succeeds.
- Affect state uses persona baselines and lazy exponential decay; normal decay does not create timer events. Unknown future drive keys may be retained but are inactive until a server policy exists.
- Raw PAD values, hidden reasoning, prompts, credentials, and unbounded provider diagnostics never enter user-visible chat or ordinary API DTOs.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unknown schema version or oversized/unknown control field | Drop optional control effects, retain valid visible text, store a bounded diagnostic |
| Invalid memory source/persona/confidence/idempotency | Reject memory plan; do not write memory or claim `learned` success |
| Invalid affect event or arbitrary model delta | Reject the event; server reducer remains the only delta owner |
| Duplicate `(persona_id, idempotency_key)` | Replay existing result; do not duplicate rows or effects |
| Snapshot revision/CAS conflict | Refuse stale update; do not overwrite newer state |
| Provider text-only completion | Normalize with empty control channels and preserve existing chat behavior |
| Legacy marker plus supported native/structured call | Native/structured path owns the capability; matching marker cannot execute a second effect |
| Assistant/message or memory/effect transaction failure | Roll back the complete caller-owned transaction |

### 5. Good/Base/Bad Cases

- Good: a native `memory_event` call and a visible assistant sentence produce one assistant message and one persona-scoped memory row in one commit.
- Base: a plain text completion produces the same SSE events and no control rows.
- Bad: parsing “我有点生气” with a regex and writing PAD, trusting a model-supplied numeric delta, or exposing raw structured arguments in `token`/`done`.

### 6. Tests Required

- Contract tests for text, native tool, structured sidecar, strict JSON content, malformed sidecar, unknown fields, source scope, and duplicate idempotency.
- Provider tests for streamed/native control accumulation and JSON completion sidecar extraction without a second stream accumulator.
- Affect repository tests for lazy decay, bounded reducer deltas, event/snapshot atomicity, CAS, replay, future drive preservation, and persona isolation.
- Memory flow tests for explicit invocation, source ownership, upsert/replay, rollback, deletion compatibility, and no implicit extraction.
- Chat integration test asserting assistant message + memory + affect/drives commit and existing `token`/`done` output.

### 7. Wrong vs Correct

#### Wrong

```js
const angry = /生气|讨厌/.test(userText);
database.prepare('UPDATE companion_persona_states SET mood = ?').run(angry ? '生气' : '平静');
```

#### Correct

```js
const turn = normalizeStructuredTurn(completion, {personaId, sourceMessageId});
const plan = affectFlow.plan(turn.control);
// Commit the validated plan with the assistant facts in the caller transaction.
```
