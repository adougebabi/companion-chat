# Technical Design: Native Tool Calls With Marker Compatibility

## Scope And Non-Goals

This task moves media delivery and pending-event authorization from model-authored marker transport to structured native tool calls while preserving the existing application contract. The existing `scene_event` implementation, `media_event` implementation, and marker parsers are consolidated behind one capability dispatcher.

In scope:

- native `scene_event`, `media_event`, and `pending_event` registry entries;
- streamed tool-call accumulation and completion diagnostics;
- native/marker normalization, validation, ordering, idempotency, continuation, and failure behavior;
- durable provenance in existing SQLite rows and payload JSON;
- `/api/companion/chat` SSE integration and regression fixtures.

Out of scope:

- renaming `/api/companion`, existing `companion_*` tables, `companion.sqlite`, environment variables, or static files;
- replacing SQLite, adding a second queue, or introducing a new service process;
- changing the browser SSE event names or removing the marker fallback;
- general backend modularization beyond the smallest dispatcher/port boundary required by this task.

## Canonical Capability Contract

All providers enter the application through one normalized envelope:

```ts
type CapabilityCall = {
  id: string | null;
  index: number;
  name: 'scene_event' | 'media_event' | 'pending_event';
  argumentsText: string;
  arguments: unknown;
  source: 'native' | 'marker';
  personaId: string;
  causationUserMessageId: string;
  idempotencyKey: string;
};
```

`id` is the provider call id when present. `index` is a non-negative provider index; marker calls receive the next deterministic turn index. `argumentsText` is retained for bounded diagnostics, while `arguments` is created only after the complete call is assembled and passes JSON parsing. `idempotencyKey` is derived from persona id, causation message id, capability name, provider call id when present, and a canonicalized argument digest. It is not exposed to the browser.

The registry has one entry per capability:

| Capability | Native schema | Marker adapter | Cardinality | Effect/result handler |
| --- | --- | --- | --- | --- |
| `scene_event` | Existing `sceneEventTool` schema | None | One per turn | `SceneCapabilityResult` + event/projection intent |
| `media_event` | Existing `mediaEventTool` schema, including `MediaCapabilityCallV2` | `<media-intent>` | One call per turn; `count` 1-3 | `MediaCapabilityResult` + atomic placeholder/job intent |
| `pending_event` | `schemaVersion`, `summary`, `notBefore`, `expiresAt`, `dedupeKey` | `<pending-event>` | One per turn | `PendingCapabilityResult` + pending-job intent |

Registry entries own transport schema, strict validator, marker adapter, per-turn cardinality, effect handler, and bounded result serializer. No route or worker may dispatch a capability by importing its private handler directly.

## Stream Accumulator

`consumeStreamedCompletion()` returns visible text plus a completion record:

```ts
type CompletionRecord = {
  text: string;
  toolCalls: CapabilityCall[];
  finishReason: string | null;
  doneSeen: boolean;
  parseErrors: string[];
  incompleteToolIndexes: number[];
};
```

The upstream decoder parses each SSE payload independently. A malformed payload is appended to bounded `parseErrors` and does not discard already collected visible text. `reasoning_content` is ignored for visible output. `delta.content` is the only source for `token` events.

Tool fragments are keyed by a non-negative integer `index`. For providers that omit `index`, an existing stable `id` may identify the call; an unindexed fragment without an id is appendable only when exactly one unindexed call is active. Otherwise it is recorded as malformed and cannot create a side effect. `id` and function name use first-nonempty assignment and must not be concatenated when repeated metadata arrives. Arguments are concatenated in arrival order and parsed only after the provider completion boundary.

`[DONE]` sets `doneSeen`. EOF without `[DONE]` is an incomplete provider completion: complete visible text may be returned with a bounded diagnostic, but incomplete or malformed native calls are rejected and their matching marker fallback is blocked. A reader error emits the existing outer SSE `error` event; already emitted tokens are never rewritten and no partial assistant reply is persisted.

## Dispatch And Ordering

1. The chat flow persists the user message before the provider call and records the causation id.
2. The dispatcher sorts complete native calls by provider index and first-seen order.
3. It rejects duplicate calls beyond each registry cardinality before executing that capability. A rejected capability does not execute its marker fallback.
4. Each capability adapter validates ownership and schema, computes its idempotency key, and returns a bounded `CapabilityResult` plus facts/projections/effect intents. During this transitional task an adapter may wrap the existing persistence helper through a port, but the final backend flow runner owns transaction boundaries and effect settlement. Different capabilities execute in provider order; there is no fixed scene-before-media ordering.
5. A media `count` batch creates all placeholders and jobs in one SQLite transaction. A failed validation creates none.
6. A valid call replay with the same idempotency key returns the prior bounded result and creates no new event, message, or job. The key is stored inside existing event/job/pending payload JSON; no second receipt store is introduced.

An invalid native call is non-fatal to an already valid visible reply, but its capability result is marked failed and its same-capability marker fallback is blocked. An effect that commits successfully is not rolled back by a later capability failure.

## Native-First Marker Fallback

Fallback is decided independently for each registered capability:

| Same-turn state | Native effect | Matching marker |
| --- | --- | --- |
| No native call for capability; valid marker | Not attempted | Execute marker adapter once |
| Native call valid | Execute once | Block |
| Native call malformed, incomplete, duplicated, or schema-invalid | No effect | Block |
| Native call valid but idempotently replayed | Return prior result | Block |
| Unknown native tool anywhere in turn | No unknown effect; record bounded diagnostic | Block all marker side effects for fail-closed behavior |

Native `scene_event` plus a marker `media-intent` is allowed because they are different capabilities and there is no native media call. Native `media_event` plus a marker media intent never creates a second media job. Marker text is redacted exactly as today and never enters `token` events.

## Pending-Event Native Contract

The native `pending_event` arguments mirror the existing marker contract: `schemaVersion: 1`, a bounded `summary`, absolute timezone-bearing `notBefore` and `expiresAt`, and a stable short `dedupeKey`. `expiresAt` must be later than `notBefore`, no more than 30 days in the future, and the source user message must belong to the selected persona conversation.

The native handler supplies normalized `CapabilityCall` provenance to the pending repository/effect port. A transitional adapter may call `createPendingEvent()` behind that port. The existing unique `(persona_id, dedupe_key, not_before)` constraint remains authoritative. A missing job for an existing pending row is repaired idempotently rather than creating a second pending row.

## Continuation And SSE

When a tool-only completion has no visible text, the chat flow builds the standard assistant tool-call message and one `tool` result message in memory, then performs at most one follow-up with `tool_choice: 'none'`. A follow-up that attempts another tool call is rejected as bounded continuation failure; it is not executed a second time.

The browser still receives only `token`, `done`, and `error` event types. `done.messages` remains authoritative and `done.message` remains the first-message compatibility alias. Existing per-capability result fields (`sceneEvent`, `mediaEvent`, and `pendingEvent`) carry bounded success/error summaries; raw arguments and provider diagnostics are never sent as user-visible text.

Continuation failure keeps committed durable effects, persists a capability-specific fallback sentence, and emits normal `done` data. Provider failure before a complete visible reply emits the existing SSE `error` and persists no partial assistant message.

## Persistence And Provenance

- Scene event payloads include `capabilityCallId`, `idempotencyKey`, `source`, and `causationId` alongside the existing scene and previous-scene facts; the runner persists the event.
- Media effect intents include the same bounded provenance and call key; the media repository/effect handler passes causation into `createChatMediaRequest()` instead of discarding it.
- Pending-event effect intents include native/marker source and call provenance while retaining the existing dedupe fields and job linkage; the pending repository owns the durable row.
- Tool-call assistant/result messages remain in-memory continuation context only. Prompt-run/debug records may retain bounded call metadata, but raw arguments and credentials are redacted.

No existing table is renamed or replaced. Any additive migration needed for indexes is optional and must not become a second source of truth; the first implementation uses existing JSON payloads and constraints.

## Compatibility And Rollout

1. Freeze fixtures for current scene/media/marker behavior and add normalized replay helpers.
2. Implement registry and accumulator behind the existing chat route while retaining the old marker functions as adapters.
3. Add native pending-event handling and update the system capability prompt to advertise all three tools with marker fallback language.
4. Switch the chat flow to one dispatcher, then remove direct `executeSceneToolCall()` / `executeMediaToolCall()` branching from the route.
5. Run mock MTPLX fixtures, real fixture replay when credentials are available, temporary SQLite API/SSE smoke tests, and browser disconnect tests.

Rollback is the previous commit/build. Do not retain a production duplicate dispatcher or silently restore marker-only behavior after cutover.

## Error Matrix And Test Obligations

| Condition | Required result |
| --- | --- |
| Malformed SSE payload | Collect bounded `parseErrors`; no side effect from incomplete call |
| Missing `[DONE]` | Mark completion incomplete; block affected native and matching marker fallback |
| Unknown tool | No side effect; block marker side effects; keep visible reply if otherwise valid |
| Invalid JSON/schema | No durable effect; no outer SSE error after a valid visible reply |
| Duplicate/replayed call | Return prior result by idempotency key; no duplicate row/job/message |
| Provider error before completion | SSE `error`; no partial assistant reply |
| Provider error during continuation | Keep committed effects; bounded capability fallback in `done` |
| Browser disconnect | Abort upstream reader/provider where supported; release request resources; no new effect after disconnect |
| Marker plus native same capability | Native state wins; marker is redacted and not executed |

Tests must assert each row above, plus deterministic multi-tool order, pending native persistence/worker behavior, media count atomicity, scene provenance, reasoning redaction, `done.messages` compatibility, and no keyword-triggered side effects.
