# Fluctlight Runtime phase 1 design

## 1. Design stance

This phase deepens the Go runtime already merged into `master`; it does not
replace Core, Temporal, Redis, or the browser boundary. The runtime is the
authoritative owner of Fluctlight semantics. Plugins provide replaceable
external capabilities and never become a second domain state machine.

The target is a durable loop, not a process-local `for` loop:

```text
observation
  -> ordered fact
  -> context projection
  -> semantic assessment
  -> typed decision proposal
  -> deterministic validation/policy/revision gate
  -> frozen action
  -> effect/plugin execution
  -> settlement/reconciliation
  -> visible projection
  -> reflection evidence window
```

The first implementation slice concentrates on the interaction turn and its
provider/capability seam. Memory, Reflection, Autonomy, and Life World will
consume this seam later instead of introducing their own tool or transport
protocols.

## 2. Ownership and dependency direction

```text
HTTP/BFF transport
  -> Runtime application service
      -> domain authority ports
          -> PostgreSQL transaction/revision state
          -> capability registry (external provider adapters)
          -> durable workflow/outbox intent
      -> diagnostics/replay projection
```

### Native Fluctlight capabilities

The following remain Runtime-owned and typed: cognition/context, affect and
drives, Memory record/retrieve/forget, Reflection/evolution, Relationship,
Goal/Intention, Autonomy policy, Life World, self-model, and action lifecycle.
They may call provider ports but cannot be replaced by arbitrary plugin code.

### External capability plugins

Plugins implement contracts such as `llm.structured`, `llm.realization`,
`embedding`, `media.image`, `media.video`, `audio.tts`, `audio.stt`, search,
or notification. A plugin manifest declares:

```text
capability_id, version, input/output schema,
side_effect_class, concurrency_class, retry/cancel support,
idempotency strategy, data scope, preflight requirements,
cost/time/token limits, health state
```

The registry performs discovery and capability preflight. It does not grant a
plugin access to arbitrary tables. All writes go through authority ports and a
caller-owned transaction or a durable effect workflow.

There is intentionally no human approval state in this product model. A
capability that is installed and passes preflight is callable, subject to
deterministic resource ownership, schema, revision, budget, timeout, and
idempotency checks. A failed check returns an explicit rejected/deferred/error
outcome.

## 3. Canonical cognition data flow

### 3.1 Fact boundary

The caller-owned transaction must commit the user message, cognition fact,
idempotency record, workflow intent, and outbox event together. The fact
contains attachment references and stable `turn_id`, `causation_id`, and
`correlation_id`; a Worker replay uses the persisted payload rather than
reconstructing it from a message row.

### 3.2 Context snapshot

The Runtime builds a bounded, typed `ContextSnapshot` from authoritative
projections. Its sections are independently versioned and evidence-linked:

```text
identity/personality/policy
inner_state and drives
recent conversation
authorized memory context
relationship context
life-world context
available capabilities
```

The snapshot is persisted or content-addressed in the assessment/frozen-action
record. Realization consumes that same snapshot; it does not re-read mutable
personality, policy, or memory state after freeze.

### 3.3 Assessment and proposal

`Assessment` is provider output normalized by one schema registry. It must
declare schema version, action/effect list, evidence references, bounded
confidence, and provider provenance. Unknown fields, missing required values,
foreign evidence, and invalid numeric semantics are rejected; application code
does not infer a replacement action from prose.

`DecisionProposal` is an immutable candidate. The deterministic gate checks
capability visibility, resource authorization, current state revision, action
constraints, concurrency class, budget, and idempotency before creating a
`FrozenAction`.

### 3.4 Effect execution and settlement

Each frozen action has a stable action id and provider request id. Effects are
classified independently:

```text
pending -> running -> completed
                  -> failed_retryable -> failed_terminal
                  -> result_unknown
                  -> cancelled
```

An external submission whose response is lost enters `result_unknown` and is
reconciled by stable request identity; it is never blindly submitted again.
The visible assistant message is a projection of a settled action, not proof
that every external media/audio effect is complete.

## 4. Tool-call contract

### 4.1 Provider normalization

Providers may return native tool-call deltas or a structured JSON sidecar. The
provider adapter normalizes both into:

```text
ToolCallV1 {
  id, name, arguments,
  source_fact_id, action_id,
  schema_version, provider_request_id,
  sequence/index
}
```

The provider adapter must not parse natural-language markers. A provider that
cannot emit native calls uses the same canonical JSON schema as a sidecar;
there is one application normalization boundary and one schema owner.

### 4.2 Execution pipeline

```text
tool_call received
  -> schema/size validation
  -> capability lookup and preflight
  -> resource/revision/idempotency gate
  -> plugin execute (timeout/cancel/retry)
  -> result normalization and post-effect invariant
  -> persist ToolResultV1
  -> optionally append result to next model input
```

Tools are not direct database APIs. Memory/Reflection/affect calls, if exposed
to the model at all, are typed intents routed through their authority service;
the model cannot choose SQL or bypass revision governance. Image/video/audio
and search are normal external capability calls.

Tool execution defaults to `exclusive` for mutable Fluctlight state. Only an
explicitly declared, side-effect-free capability may run in parallel. The
runtime bounds loop steps, calls per step, elapsed time, token/cost budget, and
repeat signatures; a no-progress loop ends as `stuck` or `deferred`.

### 4.3 Browser projection

Raw tool arguments, provider chunks, hidden assessment, credentials, and
internal diagnostics stay server-side. The browser may receive bounded
`action_result`, `media`, `heartbeat`, `token`, and terminal frames according to
the existing BrowserTurnEventV1 contract.

## 5. Transport decision: NDJSON now, SSE as a separate subscription

The turn remains `POST /api/conversations/{id}/turn` with an
`application/x-ndjson` response. Reasons:

1. The command carries a JSON request body, attachments, idempotency key, and
   CSRF context; native `EventSource` is GET-only and would require a second
   command channel or a non-native fetch-SSE parser.
2. The Go BFF already incrementally parses Core NDJSON across arbitrary byte
   and UTF-8 boundaries, enforces monotonic sequence and one terminal frame,
   and propagates `AbortSignal` to Core.
3. NDJSON frames can carry typed payloads without inventing an SSE `event/data`
   serialization layer. It is already the browser contract in
   `fluctlight-bff-contract.md` and `packages/browser-client`.
4. Changing to SSE would not repair the current pseudo-streaming issue. The
   real fix is to forward Provider chunks from `StreamText` through Core and
   BFF while retaining the same durable settlement behavior.

If server-push is later needed, add a separate authenticated subscription,
for example `/api/events` or `/api/progress`, using SSE with `Last-Event-ID`
and a bounded replay window. It must be a projection of committed outbox/event
state, never a second cognition or action state source, and it must not replace
the POST turn command.

The stale phrase “browser transport ... SSE” in
`structured-turn-contract.md` should be changed to NDJSON in the same contract
update; provider-to-application structured controls remain independent of the
browser transport.

## 6. Compatibility and rollout

- Existing Core/BFF BrowserTurnEventV1 frame names remain stable during this
  phase.
- `action_type`/`visual_concept` legacy response shapes may be accepted only by
  an explicitly marked compatibility adapter while canonical ToolCallV1 and
  typed action schemas are introduced. The adapter may not parse prose or add
  new semantic behavior.
- Existing Temporal, outbox, inbox, and provider request IDs remain the durable
  recovery boundaries. No second workflow engine or provider-managed
  conversation state is introduced.
- New plugin manifests and action states are additive and versioned. A failed
  migration or preflight leaves the prior capability disabled rather than
  silently selecting a fallback model or action.

## 7. Non-goals

- Claude Agent SDK permission modes, human approval, or HITL.
- Replacing Temporal with LangGraph/AutoGen or using a Letta memory filesystem
  as the domain source of truth.
- Completing every Memory/Reflection/Autonomy/Life World feature in this phase.
- Exposing internal state tables as arbitrary model tools.
