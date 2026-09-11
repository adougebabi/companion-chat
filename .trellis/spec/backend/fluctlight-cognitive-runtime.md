# Fluctlight Cognitive Runtime Contract

## Scenario: LLM-Owned Semantics With Server-Owned Correctness

### 1. Scope / Trigger

- Trigger: new-system code interprets an observation, user message, social signal, event, relationship meaning, memory significance, goal conflict, candidate action, or reflection evidence.
- This contract applies to the complete cognitive loop: `perception -> appraisal -> state update -> decision -> action -> reflection`.
- It exists to prevent a repeated implementation drift: replacing LLM semantic judgment with keywords, regex, substring checks, hardcoded phrase tables, fixed semantic thresholds, or default personality behavior.
- Historical code-specs remain evidence for the retired runtime. This contract is authoritative for the clean-start Go Core and its Go BFF boundary.

### 2. Signatures

Canonical application interfaces:

```go
ProcessCognitionInbox(ctx, inboxID) -> result
ProcessNativeCognitionFact(ctx, inboxID) -> error
ProcessReflection(ctx, fluctlightID, correlationID) -> ReflectionApplyResult
CompileReflectionPlan(proposal, evolutionContext, policy, now) -> ReflectionEvolutionPlan
ApplyReflectionPlan(state, plan, appliers) -> (nextState, result)
```

Required structured results:

```text
SemanticAssessmentV1
  schema_version
  perception
    event_kind
    observed_intent
    sentiment
    social_signals[]
    environment_meaning
  appraisal
    relevance
    goal_congruence
    reward
    loss
    social_threat
    controllability
    responsibility
    relationship_significance
    expected_effect
  direction / bounded strength / confidence
  evidence_refs[]
  model / model_version / prompt_version

DecisionProposalV1
  schema_version
  candidate_actions[]
  preferred_action
  bounded_explanation
  confidence
  evidence_refs[]
  model / model_version / prompt_version

ReflectionProposalV2
  schema_version
  summary
  memory_candidates[]
  relationship_observations[]
  goal_candidates[] / intention_candidates[]
  emotional_summary / affect_recalibration_candidates[]
  drive_candidates[] / preference_candidates[] / trigger_candidates[]
  developing_self_candidates[]
  personality_evolution_candidates[]
  behavior_policy_evolution_candidates[]
```

The Go Core policy result records `accepted`, `rejected`, or `deferred`, policy reason codes, current revision, requested/applied numeric changes, idempotency key, and the frozen action when one exists.

Direct conversation uses exactly one Main cognition. That response owns both
the visible assistant text and the structured/native Capability calls; Core
validates, freezes and settles it without a same-turn `role=tool` continuation
or a second realization call. Background surfaces may complete as explicit
`no_op`; direct conversation without visible text fails with
`cognition_visible_text_missing` and retains the same durable retry identity.

#### Foundation Expression Context

- For a direct conversation, the responder reads the current Fluctlight
  Foundation and attaches a bounded `persona_profile` to the CognitionFact.
  It contains stable identity context, `personality`, and `behavioral_policy`.
- The assessment receives this profile as authoritative factual context. When a
  reply action is frozen, Go Core copies that same profile to the immutable
  FrozenAction payload; realization receives only this frozen copy, never a
  later re-read of mutable personality state.
- `behavioral_policy` controls visible voice: response style, length, emoji and
  punctuation habits, humor, directness, initiative, emotional expression,
  conflict/refusal style, and intimacy expression. `personality` supplies
  durable inclination but is not a replacement for those expression fields.
- Initialization must route natural-language tone/voice descriptions into
  `behavioral_policy`; `identity.notes` is residual identity information, not a
  compatibility bucket for personality or expression. The initialization model
  must return every defined personality and behavioral-policy property. Missing
  properties are rejected instead of being silently normalized to neutral
  dataclass defaults.

#### Background Autonomy Facts

- `life_world.daily_review` is a deterministic Schedule-ready fact, not an
  inference about user inactivity. Its idempotency key is stable for one
  Fluctlight/local date/lifecycle trigger and it re-enters the same ordered
  cognition inbox as a conversation fact.
- `DailyReviewWorkflow` remains durable across local-day boundaries. A
  Schedule-pending result continues after a bounded delay; a completed or
  `no_op` day sleeps until the next local midnight and continues with a fresh
  local date, so proactive contact/moment decisions are not one-shot startup
  behavior.
- The first accepted daily-review fact owns an immutable snapshot of its
  schedule, persona, goals, intentions, and conversation target. Retries and
  duplicate lifecycle triggers check the existing fact status before reading
  mutable context; they replay the persisted fact and never enqueue the same
  ID with a newly assembled payload.
- Assessment for that fact may choose only `no_op`, `proactive_message`, or
  `moment`. The decision contains no visible text. A Moment that needs an
  image includes a frozen `media.image.generate` CapabilityInvocation with a
  thin `intent`; Core prepares the visual concept and target binding, not the
  Main LLM.
- Realization writes the visible direct-message or Moment text from the frozen
  action. It cannot decide whether to request an image. The frozen image
  concept, when present, remains unchanged for the media-prompt role.

#### Compound Decision Effects

- A cognitive decision returns one ordered root CapabilityInvocation sidecar,
  alongside the visible action projection. Each invocation has a stable call
  ID, source fact, provider request ID, sequence and Definition-owned target
  metadata; it is frozen before execution and never copied into a nested
  response-plan field.
- For a conversation fact, `conversation.reply` is the optional visible output
  capability; other invocations may independently record image, scene,
  presence, schedule, memory or affect changes. Daily review uses the same
  invocation model for `proactive_message`, `moment`, or capability-only
  actions. Retrying one invocation never creates a sibling invocation.
- Image intent is kept in the `media.image.generate` invocation. Capability
  Prepare resolves the declared context slots and freezes the internal visual
  concept before durable media-intent settlement. Realization creates visible
  text only and cannot add, remove, or reinterpret invocations.

### 3. Contracts

#### Semantic Ownership Matrix

| Concern | Owner | Contract |
| --- | --- | --- |
| Actor/message/event/time facts | Go Core | Read authoritative facts; never ask the model to invent IDs, ownership, or timestamps. |
| Intent, sentiment, social meaning, appraisal | LLM | Structured semantic result with bounded confidence and evidence references. |
| Candidate behavior and reflection meaning | LLM | Structured proposal only; it cannot execute side effects. |
| PAD/momentum/drive/relationship numeric delta | Go Core policy | Calculate from validated semantic signals, elapsed wall time, and policy version. |
| Schema, scope, authorization, safety, idempotency, CAS, transaction | Go Core | Reject invalid or stale proposals; never delegate these invariants to the model. |
| Workflow, retry, timeout, cancellation, compensation | Go runtime | Execute only a validated frozen decision. |
| Browser framing and redaction | Go BFF | Translate normalized application output; never reinterpret semantic content. |

#### Forbidden Semantic Implementations

The following are prohibited for semantic inference in production code and fallbacks:

- regex, substring, prefix/suffix, token-count, or keyword-list classification;
- language-specific phrase dictionaries, sentiment word lists, emoji tables, or punctuation heuristics;
- fixed scores such as “contains apology => trust +0.1” or “message delay > N => declining relationship”;
- inferring memory, identity, personality, goals, intentions, relationship meaning, or candidate actions from visible reply text;
- defaulting to a synthetic appraisal, personality, relationship update, or action when the model is unavailable or invalid;
- hiding a heuristic path behind names such as `fast_path`, `fallback`, `safety_default`, `simple_classifier`, or `temporary_parser`.

Deterministic code may parse and validate protocol facts: JSON/schema, IDs, actor ownership, exact enum values, timestamps, durations, numeric bounds, provider envelopes, stream frames, idempotency keys, and hard safety/permission rules. It may not assign semantic meaning to natural language or social behavior.

#### Failure Boundary

- Invalid or unavailable semantic assessment creates no inferred semantic state.
- Interactive work returns an explicit bounded failure when a required model result cannot be obtained; it does not persist a fabricated assistant reply.
- Background work retries according to workflow policy, then settles as `deferred`, `no_op`, or terminal failure with bounded diagnostics.
- Go Core may reject a model proposal but cannot synthesize a semantic replacement. A hard policy may force a safe `no_op` or explicit refusal without claiming that the model made that judgment.
- Deterministic time decay, schedule boundaries, authorization, and safety continue to operate when the LLM is unavailable because they do not infer meaning.
- Persist structured results, evidence references, model/prompt/policy versions, and bounded explanations. Do not persist hidden reasoning or credentials.
- Action realization receives a frozen action and the post-transition read model. It cannot mutate or propose affect, drives, relationships, memory, goals, intentions, identity, or personality.
- Actions that produce no visible content, including `ignore` and `delay`, do not call realization.
- Reflection always runs in an owning background workflow over an explicit evidence window; it never shares the interactive realization response.
- Every source observation is persisted as an idempotent, monotonically sequenced inbox fact for its Fluctlight instance.
- One cognitive writer owns state transitions and action-delivery order for one Fluctlight instance. Different instances may execute concurrently.
- An interactive responder may process only the inbox fact for its current turn ID. It must claim with an expected fact ID; consuming an older pending/background fact and attributing its realization to the current HTTP turn is a contract failure.
- The interactive NDJSON responder claims a newly enqueued inbox fact atomically with its creation. The background cognition Worker must observe that active claim and defer until the responder settles or the claim lease expires; both paths must never realize the same `assistant:<turn_id>` message concurrently.
- A failed interactive fact may be retried in place with the same turn/fact and idempotency IDs only while no later fact has been processed. A completed fact is replayed from its persisted realization; retries never create a second user message or reassess a completed turn.
- Reflection commits with evidence watermark and state-revision CAS; stale work cannot overwrite a newer interactive transition.
- Media execution may run concurrently, but only a sequenced completion/failure fact may re-enter cognition.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Provider timeout/unavailable for required interactive assessment | Return bounded interaction failure; persist no assistant reply or semantic state transition. |
| Background assessment timeout with attempts remaining | Retry through the owning workflow; do not run a code heuristic. |
| Background attempts exhausted | Settle `deferred`, `no_op`, or terminal failure according to the owning contract; preserve diagnostic and source facts. |
| Unknown schema/version/field or malformed structured result | Reject the semantic result; preserve source facts; do not synthesize defaults. |
| Evidence reference is missing, foreign, or outside the authorized window | Reject the candidate before numeric state update. |
| Model supplies raw PAD/trait/relationship delta | Reject the raw delta; Go Core policy remains the only numeric owner. |
| Policy rejects an unsafe or unauthorized action | Record policy rejection and execute no effect; do not choose a heuristic alternative. |
| Duplicate idempotency key | Replay the persisted assessment/decision outcome without another model call or side effect. |
| Stale state revision | Reject or re-assess through an explicit workflow transition; never overwrite newer state. |
| Deterministic timestamp/schema/ownership validation fails | Return the typed validation failure without calling the model. |
| Assessment succeeds but policy chooses `ignore` / `delay` | Freeze the action and skip realization; execute only the owning workflow transition. |
| Realization output contains semantic state candidates | Reject the forbidden fields; do not apply them or open a second state-update boundary. |
| Realization fails after a frozen state transition | Preserve the frozen semantic transition and settle/retry the action according to its contract; do not re-assess implicitly. |
| Two facts arrive concurrently for one Fluctlight | Persist distinct sequence numbers and process in order through the single cognitive writer. |
| Reflection state revision/watermark is stale | Reject the candidate and explicitly discard or reschedule; never overwrite current state. |
| Media completes outside the cognitive writer | Persist a media-result inbox fact; do not mutate cognitive state from the media Worker. |

### 5. Good / Base / Bad Cases

- Good: the model classifies an ambiguous message as mixed concern and social distance with evidence references; Go Core validates ownership, computes bounded affect/relationship changes, and commits one revision.
- Good: multiple drives are high; the model proposes `delay_reply` with semantic reasons, Go Core checks current schedule and policy, freezes the decision, and the Worker executes it once.
- Base: the model returns a valid neutral assessment and no state-changing candidate; Go Core records `no_op` without manufacturing change.
- Base: an explicit timestamp expires an intention; Go Core closes it deterministically without an LLM call because no semantic interpretation is required.
- Bad: `/sorry|对不起|抱歉/` increases trust, an emoji table changes affect, message length chooses response style, or a fixed inactivity threshold marks a relationship as declining.
- Bad: a provider failure creates a default friendly reply, default appraisal, default personality, or keyword-derived memory.

### 6. Tests Required

- Contract tests reject malformed/unknown semantic schemas, raw numeric deltas, foreign evidence, stale revisions, and duplicate idempotency keys.
- Failure tests prove provider timeout, invalid JSON, and exhausted retries never call a heuristic classifier and never persist fabricated semantic state.
- Paraphrase and multilingual fixtures assert that application outcomes come from injected model results rather than exact wording in source text.
- Negative architecture tests scan Go Core and Go BFF production paths for newly introduced semantic regex/keyword dictionaries and require explicit review for any natural-language matching.
- State-transition tests assert numeric policy owns requested/applied deltas, clamps canonical ranges, records policy/model versions, and is independent of Worker tick frequency.
- Decision tests assert policy rejection produces no effect and no code-selected semantic alternative.
- Single-Main tests assert one `conversation_turn_response`, zero `role=tool`
  messages, zero `action_realization` calls, and frozen retry without another
  Main request. Capability-local HOW planners are counted by their own schema,
  not as a second Main cognition.
- Delivery failure/retry tests assert the same frozen decision and message
  identities are reused without another implicit assessment.
- Concurrency tests assert per-Fluctlight ordering, cross-Fluctlight parallelism, stable action delivery, stale-reflection rejection, and media-result inbox re-entry.
- Reflection tests assert identity/personality/memory/relationship candidates require evidence windows and cannot be created from one visible message by application code.
- End-to-end tests inject a fake semantic provider and verify `facts -> structured result -> policy -> transaction/workflow -> observable outcome` without testing past the cognitive module interface.

### T04 Numeric Transition Record

The T04 numeric policy treats lazy wall-time decay and one validated semantic
assessment as one auditable transition when they are applied in the same
command. The transition increments the inner-state revision exactly once and
records requested/applied deltas for PAD, momentum, mood intensity, and every
drive touched by the typed assessment. A model-provided `raw_numeric_delta`,
including an empty object, is invalid input rather than a no-op fallback.

PAD plus `momentum.value`, numeric `momentum.trend`, and `*_momentum` fields are
bipolar `-1..1`; mood/regulation/Drive/conflict pressure are unit `0..1`.
Elapsed-time decay uses the frozen `affect.reducer.v2` profile and is partition
invariant. An unsupported policy version, stale State/AffectProfile revision,
foreign appraisal evidence, or non-finite value fails closed. Built-in and
active typed Drives receive opaque refs; the model can propose semantic
increase/decrease signals, while Core computes pressure and opposed-signal
conflict. A capability-only result with no appraisal performs no state change.

### 7. Wrong vs Correct

#### Wrong

```python
def infer_social_signal(text: str) -> str:
    if "leave me alone" in text.lower() or re.search(r"别烦我|走开", text):
        return "rejection"
    return "neutral"

def fallback_appraisal() -> Appraisal:
    return Appraisal(relevance=0.5, reward=0.5, confidence=1.0)
```

#### Correct

```python
assessment = semantic_runtime.assess(
    AssessObservation(
        actor_id=actor_id,
        source_fact_ids=source_fact_ids,
        context_revision=context_revision,
    )
)

validated = semantic_policy.validate(assessment, authorized_facts)
transition = state_policy.apply(validated, current_state, elapsed_time)
unit_of_work.commit(transition)
```

## Scenario: Compound Effects And Reflection Commit Boundary

### 1. Scope / Trigger

- Trigger: a provider returns ordered cognitive effects or a reflection proposal
  containing memory/relationship candidates.
- The validator runs before any freeze, autonomy enqueue, candidate apply or
  reflection watermark advance.

### 2. Signatures

```python
validate_envelope(claim, envelope) -> None
validate_reflection_payload(payload) -> None
commit_reflection(proposal, *, expected_watermark, applier) -> None
```

### 3. Contracts

- Conversation effects have a first `reply`/`no_op`; later effects are only
  autonomous side effects. Background effects have a first
  `proactive_message`/`moment`/`no_op`.
- Duplicate effect IDs, invalid sibling types and inconsistent primary payloads
  fail before the primary action is frozen.
- Reflection validates every candidate's required fields, enum, numeric bounds,
  evidence and timestamp before writing. `applier.apply(..., tx=tx)` and the
  proposal/watermark update share one Unit of Work.
- The raw Reflection object is checked against the closed response schema
  before alias normalization. An empty object, scalar candidate, wrong
  container, unknown field, or foreign opaque ref invalidates the proposal;
  normalization must never erase it and advance the watermark as `no_change`.
- Reflection Memory evidence may cite the current sequence observation,
  its authoritative appraisal, an allowlisted ActionOutcome, or an authorized
  opaque Memory ref. `autonomy.result` is projected through a field allowlist;
  visible assistant realization and raw expected/observed/runtime IDs never
  become learning evidence.
- Memory target/merge refs resolve only through the frozen
  `ContextReferenceIndex`. Core derives internal IDs, expected revisions,
  owner/profile/Conversation scope, provenance/idempotency and embedding work.
  Conflicting/unknown Conversation evidence fails rather than widening a new
  Memory to global scope.
- A Reflection apply must update the already-claimed watermark row with
  exactly one CAS write. Zero affected rows are a conflict even at watermark
  zero; an `INSERT ... ON CONFLICT DO NOTHING` fallback cannot authorize
  committed domain mutations.
- A Reflection V2 relationship observation carries only an opaque
  `target_ref`, semantic observation/direction/strength/confidence and bounded
  evidence refs. Core resolves the frozen Relationship snapshot, owns numeric
  metric changes and enforces its revision CAS; Provider-supplied database IDs,
  metrics, revisions and provenance are rejected by the closed schema.
- Reflection prompts include the actual bounded evidence window, not only
  sequence numbers.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Secondary `reply`/`no_op` in a conversation decision | Typed failure; no action or media intent is frozen. |
| Missing/unsupported reflection `type`, `content`, `confidence`, trend or metrics | `ReflectionValidationError`; watermark unchanged. |
| Reflection applier fails | Entire candidate/proposal/watermark transaction rolls back. |
| Empty/scalar/unknown-field Reflection candidate | Reject before normalization; keep watermark unchanged and release the window lease. |
| Memory ref is wrong-kind/stale/overlapping or evidence scope conflicts | Reject before apply or roll back on live CAS; no partial governance. |
| Claimed watermark CAS updates zero rows | Roll back proposal and all domain mutations; do not insert/fall through. |
| Duplicate action/effect retry | Stable IDs replay existing rows; no duplicate user/assistant/media effect. |

### 5. Good / Base / Bad Cases

- Good: `[conversation.reply, media.image.generate]` validates, freezes the
  root invocations, then settles the output targets independently.
- Base: an empty candidate list is a valid reflection no-op and advances only a
  valid evidence watermark.
- Bad: process a visible reply and a failed required CapabilityInvocation,
  freeze the output, then discover the invalid sibling; or use
  `.get("type", "episodic")` to hide a malformed candidate.

### 6. Tests Required

- Valid/invalid compound effect tests for both `process_next` and `stream_next`.
- Assert invalid siblings cause zero `_freeze` calls and one failed settlement.
- Reflection tests for missing fields, unsupported enums, malformed timestamps,
  duplicate candidates, retry and watermark rollback after applier failure.
- Memory Reflection tests cover all six lifecycle candidates, opaque ref
  compilation, assistant-prose exclusion, cross-Conversation rejection,
  stale target CAS, exact duplicate disposition, rollback of Memory/revision/
  governance/embedding/outbox/proposal/watermark, and next projection refs.
- Prompt test asserts evidence payloads are present in the reflection request.

### 7. Wrong vs Correct

#### Wrong

```python
primary = await freeze(effects[0])
for effect in effects[1:]:
    await freeze(effect)  # validation discovers a bad sibling too late
```

#### Correct

```python
validate_effects(effects)
primary = await freeze(effects[0])
await settle_secondary_effects(effects[1:])
```
