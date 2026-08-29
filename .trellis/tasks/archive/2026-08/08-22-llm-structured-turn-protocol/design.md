# Technical Design: Structured Turn Control, Affect, Drives, and Memory

## Scope

本设计把 LLM 与后台之间的业务控制结果定义成可版本化、可校验的内部协议，同时保留现有浏览器 SSE 体验。第一阶段不要求模型返回单一 JSON-only 回复：用户可见文本仍走当前 `content`/token 流，状态、记忆和副作用只走结构化控制通道。

## Boundary Model

```text
provider content/token stream
  + native structured tool calls or structured control sidecar
        |
        v
provider adapter normalization
        |
        v
CompanionTurnResultV1
  - visible text/messages
  - validated control candidates
  - parse diagnostics
        |
        v
application policy and capability plans
        |
        v
SQLite transaction: assistant messages + memory + affect/drives events + effects
        |
        v
existing SSE token/done/error
```

The provider adapter is responsible for decoding provider-specific JSON/SSE and native tool calls. The application owns the canonical schema, policy, persona scope, idempotency, and effect application. The browser continues to consume the existing transport contract.

## Canonical Internal Contracts

The canonical normalized result is a backend-owned object, not an instruction to trust model output:

```text
CompanionTurnResultV1 {
  schemaVersion: "companion.turn.v1",
  text: string,
  tokens: string[],
  control: {
    affectEvents: AffectEventCandidate[],
    driveSignals: DriveSignalCandidate[],
    capabilityCalls: CapabilityCallCandidate[]
  },
  messages: MessageIntent[],
  parseDiagnostics: BoundedDiagnostic[],
  sourceMode: "text" | "native_tools" | "structured_sidecar" | "legacy_marker"
}
```

The model-facing control schema is intentionally smaller than the internal result:

```text
CapabilityCallCandidate {
  name: "memory_event" | "affect_event" | "drive_signal" | existing capability,
  arguments: bounded JSON object,
  source: "native" | "structured"
}

AffectEventCandidate {
  type: fixed allowlisted event type,
  confidence: 0..1,
  sourceMessageId: persona-scoped message ID or null,
  idempotencyKey: bounded string
}

DriveSignalCandidate {
  drive: "social" | "exploration" | "rest" | future extension,
  direction: "increase_pressure" | "decrease_pressure" | "neutral",
  confidence: 0..1,
  idempotencyKey: bounded string
}
```

`memory_event` is the only ordinary-chat route to durable memory. Its arguments are normalized into an application memory plan; model-supplied `memoryWrites` are not applied as raw repository calls. If a future provider exposes a parsed `memoryWrites` field, the adapter maps it to the same `memory_event` validator.

`affect_event` and `drive_signal` are structured state reports rather than executable capability effects. The normalizer converts them into bounded affect/drives candidates, removes them from the dispatcher call list, and leaves numeric deltas to the server-owned reducer.

Visible message text remains separate from control JSON. This allows the current text streaming path to continue while control data is accumulated and validated before any side effect runs.

## Provider Compatibility

Normalization order:

1. Native `tool_calls` with allowlisted function names.
2. Provider-specific parsed/structured control field, when available and validated.
3. Legacy text markers only for existing media/pending compatibility during migration.
4. Plain text with an empty control set.

New affect and memory behavior must not introduce additional natural-language markers. A malformed optional control result fails closed for control effects while preserving a valid visible reply; the bounded diagnostic is retained for development inspection. A malformed required capability call produces a capability error and no partial application.

The application must normalize all accepted provider forms into one `CompanionTurnResultV1`. The provider adapter must not decide whether a memory or affect event is semantically allowed.

## Affect and Drives Domain

### State shape

Add a dedicated materialized snapshot and append-only event table rather than extending `companion_persona_states`:

```text
companion_persona_affect_states
  persona_id PRIMARY KEY
  pleasure REAL
  arousal REAL
  dominance REAL
  drives_json TEXT
  revision INTEGER
  effective_at TEXT
  updated_at TEXT
  source_event_id TEXT
  model_version TEXT

companion_persona_affect_events
  id PRIMARY KEY
  persona_id
  event_type
  effective_at
  causation_id
  source_message_id
  idempotency_key
  pleasure_delta REAL
  arousal_delta REAL
  dominance_delta REAL
  drives_delta_json TEXT
  payload_json TEXT
  model_version TEXT
  created_at TEXT
  UNIQUE(persona_id, idempotency_key)
```

The migration must add persona/time and causation indexes, use the existing migration ledger, and preserve old data. Snapshot updates use revision/CAS or an equivalent transaction-local expected revision to prevent lost updates.

### Policy and decay

The first release recognizes `social`, `exploration`, and `rest`. Their pressure values are `0..1`; higher means more unmet need. The storage shape preserves unknown future drive keys without activating them until a registered policy exists.

Per-persona policy is read from the versioned life blueprint with safe defaults:

```text
affectPolicy.baseline = {pleasure, arousal, dominance}
affectPolicy.halfLife = {pleasure, arousal, dominance}
drives.social = {baseline, weight, halfLife}
drives.exploration = {baseline, weight, halfLife}
drives.rest = {baseline, weight, halfLife}
```

Decay is lazy. A read or new event first evaluates the snapshot at the requested time using a fixed model version, clamps values to their domain ranges, then applies the new event delta. No timer writes a decay row every minute. Hard reset is a development/recovery operation, not normal behavior.

The server maps allowlisted affect event types to bounded deltas. The model cannot submit arbitrary PAD numbers. `replyPosture` is derived from the current state and governance inputs, then passed to the prompt as a coarse behavior constraint; raw values remain server/debug data.

## Memory Capability and Commit

`memory_event` is added to the capability contract and registry as a persona-scoped application flow. Its plan stage validates:

- persona ownership and source message ownership;
- bounded key/value/confidence and supported operation;
- maximum writes per turn;
- idempotency and duplicate/upsert policy;
- source type and source ID;
- no cross-persona references.

The apply stage calls a memory application service/repository port. The existing conversation commit boundary writes assistant facts and applies capability plans in one synchronous SQLite transaction. The response may expose a bounded `learned`/`memoryChanges` summary only after a successful commit; a candidate or rejected write must not be presented as persisted memory.

## Chat Flow Integration

The existing flow remains the orchestration owner:

1. Persist the user message as today.
2. Read life state, decayed affect snapshot, drives, relationship layer, memories, and history.
3. Derive a bounded `replyPosture` and include it in the server-owned prompt layer.
4. Stream visible text and collect structured tool/control data.
5. Normalize and validate the control result.
6. Build assistant message facts plus memory/affect/effect plans.
7. Commit all accepted facts and plans transactionally.
8. Emit existing `token` and `done` events; never expose raw control diagnostics in normal chat.

Existing `timeline-flow`, `deferred-chat-policy`, proactive worker, quiet-hours, screening, budget, lease, and safety checks remain final gates. Affect/drives can bias a candidate or posture but cannot bypass those gates.

## Observability and Privacy

Development-only inspector data may contain bounded PAD/drives snapshots, decay metadata, event IDs, source IDs, policy version, posture, and governance reason codes. It must not expose full prompts, provider credentials, authorization headers, hidden reasoning, or another persona's data. Ordinary persona and chat APIs remain user-safe projections.

## Migration and Rollback

- Add one ordered SQLite migration after the current ledger version for affect snapshot/event tables and indexes.
- Extend existing contract/tool registries additively; keep old text and marker parsing for compatibility.
- Gate structured control execution behind a runtime setting or feature flag during rollout.
- The `structuredTurnControl` runtime option disables control-side effects while preserving text-only chat, allowing a rollback without changing provider or browser contracts.
- On malformed control data, preserve a valid visible reply and drop only the invalid control side effect; on a required capability failure, fail the capability without partial writes.
- Rollback disables control generation and new writes while retaining new tables/rows; old clients and old text-only turns continue to operate against unchanged conversation data.
