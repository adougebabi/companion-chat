# Fluctlight Intelligence P1 design

## 1. Design stance

P1 deepens the Go runtime already merged into `master`; it does not replace
Core, Temporal, Redis, or the browser boundary. The Runtime is the sole owner
of Fluctlight semantic authority. Capability plugins provide replaceable
implementations, but they never become a second domain state machine or agent
loop.

The target is a durable semantic loop:

```text
observation
  -> ordered CognitionFact
  -> ContextProjection
  -> expression/semantic assessment
  -> SelfEvaluation gate
  -> ResponsePlan / native capability candidates
  -> deterministic validation + revision gate
  -> FrozenAction
  -> optional capability effect
  -> realization/projection
  -> reflection evidence window
  -> governed domain revision
  -> future ContextProjection
```

This is an adaptive-depth loop. Ordinary chat uses one cognition call that
returns visible text plus typed control sidecars. Effects, scene/memory/relationship
candidates, or high uncertainty enter a bounded assessment/freeze/effect/
realization path. A second model call is never allowed to reinterpret the same
raw conversation as a fresh decision.

## 2. Ownership and dependency direction

```text
HTTP/BFF transport
  -> Runtime application service
      -> ContextProjection + claim/evaluation policies
      -> native capability authority ports
      -> capability registry (built-in or external plugins)
      -> PostgreSQL transaction/revision state
      -> durable workflow/outbox intent
      -> diagnostics/replay projection
```

### Fluctlight-native capability contracts

The contracts remain Runtime-owned and typed: cognition/context, self-evaluation,
affect/drives, Memory, Reflection/evolution, Relationship, Goal/Intention,
Autonomy, Life World, self-model, and action lifecycle. Their implementations
may be built-in or injected as plugins. The canonical authority, revision
ledger, evidence rules, and reader-facing projection remain in Core.

### External capability plugins

External slots include structured/streaming LLM, embeddings, image/video/audio,
search, notification, and other provider effects. Native slots such as
`scene_event`, `presence_event`, `memory_event`, `relationship_signal`, and
`self_evaluation` use the same manifest/registry shape, but route accepted
proposals back through a Core authority service instead of a plugin-owned
database.

Every manifest declares:

```text
capability_id, version, input/output schema,
side_effect_class, concurrency_class, retry/cancel support,
idempotency strategy, data scope, authority port,
preflight requirements, cost/time/token limits, health state
```

Installed and preflighted capabilities are callable without human approval.
The Runtime still enforces ownership, schema, evidence, revision, budget,
timeout, cancellation, idempotency, and result reconciliation.

## 3. Domain facts and ContextProjection

### 3.1 Fact levels

The Runtime never treats all model-visible text as the same kind of truth:

```text
Transcript       = what an actor said
Observation      = source-bound fact the system recorded
Hypothesis       = temporary Fluctlight interpretation
Memory/Belief     = governed long-lived fact
Personality/Self  = slow revision of durable self-model
```

An unconfirmed Fluctlight self-claim is a decaying hypothesis. It cannot be
promoted to Memory, Personality, Identity, or Life World solely because it
appeared in a previous assistant message.

### 3.2 Projection sections

`ContextProjection` is the only context assembler for Chat, realization,
Memory retrieval, Reflection, scene capabilities, Media, and future Autonomy.
Its sections are independently bounded and carry source/revision/confidence:

```text
current source fact and user intent
identity/personality/behavioral policy projection
inner-state, affect, and drives
resolved Event > Schedule > pending context
Presence overlay
authorized relevant MemoryContext
Relationship projection
active hypotheses with expiry/repetition metadata
available capability manifests
```

The projection selects recent relevant conversation rather than blindly passing
the complete transcript. Every item inserted into a provider request is
reconstructable from a persisted fact, revision, or projection version.

## 4. Adaptive cognition and ResponsePlan

### 4.1 Fast path

For ordinary chat with no external effect or native candidate, one cognition
provider call returns:

```text
visible draft
ResponsePlanV1
SelfEvaluationV1
optional ToolCallV1/native candidates
```

The Runtime validates the plan and freezes the approved expression before
streaming it. It does not send the same user text and history to a second model
for another semantic decision.

### 4.2 Deliberate path

When an effect/candidate/high-uncertainty flag is present:

```text
assessment + self-evaluation
  -> deterministic gate
  -> freeze ResponsePlan/FrozenAction
  -> execute native or external capability
  -> optional realization from the frozen plan and ToolResult
```

Realization receives only the bounded plan, approved claims, style surface,
and tool results. It cannot add a new fact, scene, memory, relationship,
personality change, or capability effect. One bounded rewrite is allowed when
the output violates the plan; repeated failure becomes `omitted`, `uncertain`,
or `deferred`.

### 4.3 ResponsePlanV1

```text
schema_version
source_fact_id
context_revision
answer_mode
approved_claims[]
uncertain_claims[]
omitted_claims[]
response_outline
tone
tool_calls[]
native_candidates[]
self_evaluation
```

Claims have a kind (`confirmed_fact`, `observed_fact`,
`supported_hypothesis`, `uncertain_hypothesis`, `unsupported_self_claim`),
evidence refs, confidence, and optional expiry/repetition key. Runtime stores
bounded reason codes, not hidden chain-of-thought.

## 5. SelfEvaluation and anti-loop

SelfEvaluation checks:

- relevance to current user intent and resolved Life World context;
- evidence ownership and claim type;
- compatibility with Identity/Personality/affect/Memory;
- novelty versus recent accepted claims;
- whether a detail is necessary to answer;
- whether the wording overstates certainty.

The deterministic gate maps a candidate to `accepted`, `uncertain`, `omitted`,
or `deferred`. No-evidence self-claims are not silently turned into facts.

The Runtime computes a normalized semantic repetition signature from claim/topic,
action, source fact and relevant state revision. Repetition without new evidence
does not raise confidence, create another Memory/Scene row, or re-enter the
same context section. A rejected or expired claim is retained as provenance
(`superseded`, `rejected`, or `expired`) so projection does not resurrect it.

## 6. Native scene/presence capability slots

`scene_event` and `presence_event` are native Fluctlight slots with replaceable
implementations. Their candidates include:

```text
scene_event: scene, activity, location, start_at?, end_at?,
             kind, confidence, evidence_refs, source_fact_id, idempotency_key
presence_event: user_presence?, current_task?, expires_at?,
                confidence, evidence_refs, source_fact_id, idempotency_key
```

Life World authority decides whether a candidate is a confirmed event, a
decaying hypothesis, an overlay, or a no-op. Accepted changes enter the ordered
Fluctlight cognition stream. Presence may overlay only `user_presence` and
`current_task`; it cannot replace authoritative scene/activity/location.

No new scene fact is created when the semantic key and time window are already
represented without new evidence. The chosen plugin implementation never owns
the canonical Event table or Context resolver.

## 7. Memory authority and retrieval

Memory owns typed record/revise/forget/retrieve/embed. All new records create
their initial revision and embedding workflow request in one authority
transaction. The embedding worker may fail or retry without deleting the
authoritative Memory row.

Retrieval is ordered:

```text
owner/visibility/Actor/Conversation hard filter
  -> FTS/vector/hybrid rank
  -> recency/importance/confidence weighting
  -> bounded rerank
  -> token budget
  -> MemoryContext with evidence/source/revision
```

Reflection and ordinary chat use the Memory authority port; they cannot insert
raw `memories` rows. Memory correction and forget create auditable revisions,
and rejected/superseded claims are excluded from normal context.

## 8. Reflection and evolution

Reflection is triggered by a committed cognition completion or durable periodic
fact. It claims one Fluctlight evidence window, validates candidate evidence
against that window, and applies candidates through authority ports:

```text
evidence window
  -> ReflectionCandidate
  -> evidence/consistency/revision gate
  -> Memory / Relationship / Self-model / Personality revision
  -> embedding/reindex/projection
```

The window has a watermark, base state revision, owner/lease, and CAS. A
candidate cannot directly mutate a canonical projection. Every revision stores
the previous value, evidence refs, source window, provider/prompt/schema
versions, and a rollback identity.

Evolution speeds are explicit:

```text
fast: affect, attention, temporary intention, hypothesis
medium: Memory, Relationship trend, recurring preference, goal priority
slow: Personality, Identity biography, Self-model, behavioral policy
```

One response cannot alter a slow field. A recurring pattern needs multiple
evidence-bearing windows and a valid revision transition.

## 9. Durable outcomes and transport

Fact/action/capability/reflection outcomes use explicit states:

```text
proposed, validated, rejected, frozen, realizing,
completed, failed_retryable, failed_terminal,
result_unknown, cancelled, deferred, stuck
```

The turn remains `POST` + `application/x-ndjson`. Provider SSE is an adapter
detail only. A future SSE subscription for committed server-push projections
is separate and out of P1. Core/BFF preserve monotonic sequence, one terminal
frame, hidden-payload redaction, and abort propagation.

## 10. Non-goals

- visual identity/avatar/image-to-image/AppearanceProfile;
- human approval/HITL or Claude Agent SDK;
- P2 Harness, multi-agent handoff, or a second workflow/checkpoint runtime;
- provider-managed conversation state as Fluctlight truth;
- unlimited self-critique or keyword-based scene inference.
