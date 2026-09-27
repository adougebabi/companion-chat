# Fluctlight Cognitive Runtime Contract

## Scenario: LLM semantics, native Agent loops, server-owned correctness

### 1. Scope / Trigger

Applies to interpretation of observations, messages, relationships, memories,
goals, visual identity and reflection. The former one-Main/two-generation,
pure-query continuation, winner-only global commit and caller Tool-settlement
rules are replaced by the independent Agent/Tool contract in
[Structured Turn](./structured-turn-contract.md).

### 2. Signatures

```go
ProcessCognitionInbox(ctx, inboxID)
ProcessNativeCognitionFact(ctx, inboxID)
RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput)
RunFormalAgent(ctx, FormalAgentID, FormalAgentRunInput)
ExecuteTool(ctx, ToolExecutionRequest)
ProcessReflection(ctx, fluctlightID, correlationID)
CompileReflectionPlan(proposal, evolutionContext, policy, now)
ApplyReflectionPlan(state, plan, appliers)
```

Typed model tasks keep their own prompts, input/context, model role and final
contract. Shared `RunADKLoop` is the only model/Tool loop. Default no-tool tasks
still use it. Embedding remains model infrastructure rather than an Agent.

### 3. Contracts

| Concern | Owner | Contract |
| --- | --- | --- |
| Actor, resource, event and time facts | Go Core | Read owned authoritative data; never ask the model to invent identifiers |
| Intent, social meaning, appraisal and task decisions | LLM | Bounded semantic output with valid evidence |
| Tool execution requests | Native model ToolCalls | No prose, reasoning or final sidecar execution protocol |
| PAD, momentum, Drive and relationship numeric deltas | Core policy | Validate semantics, compute bounded numbers, record revision and provenance |
| Authorization, schema, CAS and local transactions | Core services | Never delegate these invariants to the model |
| Model/Tool continuation | Eino Runner | Consume actual results, including business rejection; no action-type stop rule |
| Durable scheduling and external media work | Existing workflows | Stable task IDs, bounded retry/cancel, actual task and asset states |
| Browser events | Go boundary | Expose committed safe resources and one terminal result |

- Complete tasks are explicitly registered: cognition, wake-up, takeover judge
  and reply, initialization, media prompt/quality, visual vision/patch and full
  visual identity, summary, schedule generation/replan, native cognition, daily
  review, persistent-switch assessment and reflection.
- Different runs never share mutable prompt, trace, result or task state.
  Persisted context is read through owning business interfaces.
- Initial projection is a bounded snapshot, not a promise that later Tool writes
  never happen. Committed results and later queries update the facts used by
  subsequent decisions. Keep real query/result/model-request correlation.
- Provider-facing opaque refs are filtered by both surface and frozen
  `ContextReferenceKind`. In ordinary Conversation/Takeover context, retain
  Memory/Active Memory update targets and one Life Context causal anchor;
  nested Scene/Schedule/Schedule Item/Presence refs remain Core-side unless the
  surface explicitly admits those kinds. A valid token of the wrong kind is
  not a valid ref for another field.
- No-tool completion is valid. Read, write, mixed and multiple Tool batches may
  continue normally. Model errors and cancellation are errors, never inferred
  completion. Only the actual final assistant result is decoded against the
  task contract; no previous text fallback.
- Tool writes are independent local commits. A later invalid final result does
  not roll them back. `tool_executions` preserves receipts; `agent_runs` fences
  failed/interrupted decision-loop replay. Final cognition projection failures
  preserve these effects and remain errors.
- Conversation and background callers consume completed Agent contracts and
  actual Tool outcomes. They never parse a second action list, freeze tools for
  delayed execution or manually assemble continuation messages.
- `persona.switch` applies an authorized declared persistent transition once.
  `persona.takeover` commits/audits a per-run decision and returns its working
  persona without overwriting persistent dominance. Subsequent expression and
  profile-scoped effects use their actual acting identity. No outer second CAS
  or hidden one-Judge/two-generation ceiling is allowed.
- Background daily-review input owns date/timezone and its durable evidence.
  Wake-up/Reflection scheduling, inbox order, supersession, bounded depth and
  cancellation remain product protections, not Tool source-stage requirements.
- A direct Tool command supplies authorized resources and business evidence; it
  need not manufacture a chat, cognition fact or frozen action.
- No appraisal means no fabricated state revision. A malformed present appraisal
  is an error. Semantic summaries stay bounded; hidden reasoning, credentials
  and unbounded raw Provider details do not enter public DTOs.

Forbidden semantic implementations remain regex/keyword/sentiment dictionaries,
emoji tables, prose-derived memories/personality, synthetic neutral appraisals,
or fixed phrase thresholds that replace model decisions. Deterministic schema,
time expiry/decay, resource ownership and CAS are still server responsibilities.

Numeric transition contract: PAD, numeric momentum/trend and momentum components
are in `-1..1`; mood/regulation/Drive/conflict pressure is in `0..1`. Lazy decay
and a validated assessment in one command form one audited revision, with
requested/applied deltas and policy identity. Non-finite values, stale
State/AffectProfile revision, foreign evidence and unsupported policy fail
closed. Model Drive signals use refs, direction, bounded strength/confidence;
Core computes numbers. A Tool-only run without appraisal performs no invented
semantic transition.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Required model unavailable / invalid final schema | Failed run; no synthesized reply or appraisal |
| Tool already committed before that failure | Preserve receipt/resource; no whole-run re-execution |
| Business constraint rejects a Tool | Return actual failure to the model for a further decision |
| Code/dependency error or cancellation | Keep exact cause and partial facts; never success |
| Raw numeric delta or foreign evidence | Reject semantic proposal before applying numbers |
| Duplicate run or operation | Replay completed facts or report failed/incomplete run; reject conflicting input |
| Newer conversation supersedes an old run | Fence later settlement; never deny already committed facts |
| Concurrent facts for one Fluctlight | Preserve ordered inbox ownership and expected claim |
| Media completes separately | Publish authoritative media outcome; cognition re-entry uses a real sequenced fact |

### 5. Good / Base / Bad Cases

Good: write Memory, read it through a Tool, then decide based on the returned
value. Good: a declared persona Tool commits and subsequent decisions consume
its outcome. Base: a summary finishes with one model call and no Tool. Bad:
turn a timeout into a friendly synthetic reply, execute final `tool_calls` JSON,
roll back the description of an already-delivered message, or silently retry the
whole run after a committed side effect.

### 6. Tests Required

Real isolated Tool success, rejection, durable result, repeat and dependency
fault tests; native adapter identity and feedback; controlled rare protocol,
multiple-call and cancellation tests; per-Agent real Provider evidence; bounded
streaming and run/user isolation; random-secret retrieval; two isolated fault
sensitivity experiments. Real LLM acceptance is strictly serial. A user-deferred
live suite remains pending, not covered by ordinary Go or controlled tests.

### 7. Wrong vs Correct

Wrong: `if strings.Contains(text, "抱歉") { trust += 0.1 }` or a default appraisal
on Provider error.

Correct: validate model semantics/evidence, compute the domain's bounded policy
transition, and commit through the owning service. Let Eino consume each actual
Tool result before another decision.

## Scenario: Actor-owned life context and bounded conversation outcomes

### 1. Scope / Trigger

- Trigger: a conversation or Wake-up Provider projection includes the current Life Context, recent Action Outcomes, current appearance/drives and an ADK Tool continuation.
- A Human speaker may report being at home while the Fluctlight remains at a different location.

### 2. Signatures

```text
current_state: {subject: "actor_self", data: {life_context, inner_state, appearance, active_activities}}
compactRecentActionOutcomesForSurface(values, surface) -> bounded current signals
scene_event -> mutates actor_self Life Context only
conversation.reply Tool result -> {status, output?: {target_kind, delivery_status?}}
```

### 3. Contracts

- `current_state.data.life_context.scene/activity/location` belongs only to `actor_self`. `current_speaker` and `[sender=actor_user]` label the Human's statements; they do not reassign the Fluctlight's location. A Human saying “我回家了” is not an authorization to call `scene_event`. Provider rules and Tool description must state this ownership without an impossible `context_override` Tool parameter. User location may be drawn only from attributed message/authorized memory evidence; Core does not guess it from keywords or write it into Presence.
- Conversation/Wake-up/Takeover outcome projection drops completed `affect_event` once current inner state represents it, completed `conversation.reply` once published history represents it, and empty completed aggregate rows. Newer success suppresses older failure for the same capability; repeated failed/rejected `(capability,error_code)` keeps the newest. Pending/accepted external work remains, and unresolved outcomes keep occurrence time. Other surfaces retain their own evidence policy.
- The model-facing `conversation.reply` receipt omits already-visible text, target storage ref and replay marker; the `affect_event` receipt omits event ID/revision and refreshed label/intensity, retaining business status/type. Full native ToolCall/ToolResult and Core audit receipts remain intact. During an ADK continuation, exclude a published reply from the **outbound Recent view only** when the current in-memory trace contains its completed `conversation.reply` result with that exact target message ID; the authoritative refreshed Projection still retains the message for later Tool resolution. A fresh retry has an empty trace and must still see the committed reply, even when it reuses the same user text and turn ID.
- Conversation and Wake-up Runtime Facts `current_state`, `self_actor`, and `current_speaker` are critical at the first Working Memory cap. After field compaction, if one cannot fit, return `working_memory_required_budget_exceeded` rather than silently dropping it. Their priority precedes optional schedule/relationship fragments.
- Provider appearance keeps its safe ref, current wearing state, known body values, explicit `cleared` state and bounded worn-item target/description. It omits `unknown` body fields, revision/captured metadata and worn-item transport metadata. Built-in zero-pressure/zero-salience drives are omitted; nonzero built-ins use short label/pressure signals, while custom typed drives may retain their distinct description.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| User reports their own location | Attribute to `actor_user`; no Fluctlight `scene_event` from that report alone. |
| User explicitly asks Fluctlight to move | Tool may be proposed; only committed Tool result changes `actor_self` Life Context. |
| Completed affect/reply outcome repeats current state/history | Omit model-facing outcome; keep Core audit row. |
| Latest failure still actionable | Keep one safe status/code/occurred time; do not erase pending work. |
| Critical Runtime Fact exceeds section cap | Explicit `working_memory_required_budget_exceeded`; no silent scene/actor loss. |
| Body field is `unknown` or `cleared` | Omit unknown; preserve explicit cleared marker. |

### 5. Good / Base / Bad Cases

- Good: user is at the user's home, Fluctlight is at the Fluctlight's home; the wire prompt labels self Life Context and user message separately, and no scene Tool is inferred from user self-location.
- Base: no user-location fact is available; do not invent a current Human position.
- Bad: treat the only structured `location: 家` as the current speaker's home, copy twelve identical affect/reply outcomes into every prompt, or remove current_state to satisfy a section cap.

### 6. Tests Required

- Dual-location wire fixture, actor-subject prompt/Tool contract, Event→Schedule authority and same-turn scene/reply continuity.
- Conversation/Wake-up outcome fixture with repeated affect/reply/failure and pending media; reflect/review surfaces must retain their separate policy.
- Tool continuation fixture proves native pair preservation and no triple reply text; a fresh retry of the same turn (as well as the next independent user turn) includes the committed historical reply.
- Appearance/drives fixture covers known/unknown/cleared, worn item target, zero-pressure built-in and custom typed drive. Critical-fact overflow must fail explicitly.

### 7. Wrong vs Correct

Wrong: interpret `actor_user`'s “我回家了” as a `scene_event` for `actor_self`, or rely on a YAML→TOON switch to remove duplicate facts.

Correct: mark current Life Context as `subject=actor_self`, keep the Human's statement attributed to `actor_user`, and remove already-absorbed outcomes before serialization.

## Scenario: Reflection and final semantic projection

### 1. Scope / Trigger

Applies to final cognition claims and reflection candidates over an authorized
bounded evidence window. This retains the original semantic/learning protections
while separating them from already-committed business Tools.

### 2. Signatures

`ReflectionProposalV2` includes summary, memory candidates, relationship
observations, goals/intentions, emotional summary/recalibration, drives,
preferences/triggers, Developing Self and personality/behavior-policy candidates.
Domain appliers and the claimed watermark share one local transaction.

### 3. Contracts

- Validate the entire closed raw proposal before alias normalization. Empty,
  scalar, wrong-container, unknown-field and foreign-ref candidates fail; they
  are not erased into a successful no-change result.
- Evidence may use original observations, authoritative appraisal, allowlisted
  ActionOutcome facts and authorized Memory refs. Assistant prose, rejected
  candidate text and raw runtime payloads are not new learning evidence.
- Resolve opaque target/merge/relationship refs through the authorized context
  index. Core owns identifiers, expected revisions, profile/conversation scope,
  numeric changes, provenance, idempotency and embedding intents.
- Unknown or conflicting conversation evidence must not become global Memory.
  Slow fields require corroborating evidence and remain auditable/rollbackable.
- Update the already-claimed watermark by CAS exactly once. Zero affected rows
  rolls back that reflection transaction, including candidate mutations; never
  insert a fallback watermark to justify writes.
- A rejected unexecuted model candidate creates no semantic fact. An actual
  committed Tool result is a fact even if the Agent later rejects a different
  proposal; never confuse the two boundaries.
- Final conversation projection preserves speaker/reply attribution, claims,
  relationship observations and follow-up scheduling. It cannot replay Tools or
  publish the final text again after an explicit reply Tool delivered it.

### 4. Validation & Error Matrix

Malformed proposal, stale/overlapping target, foreign evidence or failed watermark
CAS leaves the reflection watermark unchanged. A later reflection/projection
failure never rewrites an earlier independent Tool receipt as rolled back.

### 5. Good / Base / Bad Cases

Good: Reflection merges two owned memories and exposes the new revision in the
next projection. Base: an empty valid candidate list advances only a valid
watermark. Bad: learn a fact from assistant prose or commit candidates despite a
zero-row watermark update.

### 6. Tests Required

Closed-schema rejection, evidence windows, all Memory lifecycle operations,
relationship CAS, atomic watermark/proposal/embedding/outbox rollback,
non-promotion of unsupported/repeated claims, profile attribution and no duplicate
publication after Agent completion or failure.

### 7. Wrong vs Correct

Wrong: silently default a missing Memory type or insert a missing watermark after
applying candidates.
Correct: validate the closed proposal, resolve scope, apply through authority
ports and require one successful claimed-watermark CAS in the same transaction.
