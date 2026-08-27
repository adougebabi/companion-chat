# Fluctlight Cognitive Runtime Contract

## Scenario: LLM-Owned Semantics With Server-Owned Correctness

### 1. Scope / Trigger

- Trigger: new-system code interprets an observation, user message, social signal, event, relationship meaning, memory significance, goal conflict, candidate action, or reflection evidence.
- This contract applies to the complete cognitive loop: `perception -> appraisal -> state update -> decision -> action -> reflection`.
- It exists to prevent a repeated implementation drift: replacing LLM semantic judgment with keywords, regex, substring checks, hardcoded phrase tables, fixed semantic thresholds, or default personality behavior.
- Historical code-specs remain evidence for the frozen old system. This contract is authoritative for the clean-start Python Core and its Node BFF boundary.

### 2. Signatures

Canonical application interfaces:

```python
assess(command: AssessObservation) -> SemanticAssessmentV1
propose_decision(command: ProposeDecision) -> DecisionProposalV1
reflect(command: ReflectEvidenceWindow) -> ReflectionProposalV1
apply_state(command: ApplySemanticAssessment) -> StateTransition
execute(command: ExecuteFrozenDecision) -> ActionResult
realize(command: RealizeFrozenAction) -> AsyncIterator[VisibleChunk]
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

ReflectionProposalV1
  schema_version
  memory_candidates[]
  relationship_candidates[]
  drive_recalibration_candidates[]
  personality_revision_candidates[]
  autobiographical_candidates[]
  evidence_refs[]
  model / model_version / prompt_version
```

The Python policy result records `accepted`, `rejected`, or `deferred`, policy reason codes, current revision, requested/applied numeric changes, idempotency key, and the frozen action when one exists.

Interactive work uses two model stages. `assess` / `propose_decision` return no user-visible content. Python validates and freezes the action before `realize` is called. `realize` may produce language or media content for that frozen action but cannot return semantic state candidates.

#### Foundation Expression Context

- For a direct conversation, the responder reads the current Fluctlight
  Foundation and attaches a bounded `persona_profile` to the CognitionFact.
  It contains stable identity context, `personality`, and `behavioral_policy`.
- The assessment receives this profile as authoritative factual context. When a
  reply action is frozen, Python copies that same profile to the immutable
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

### 3. Contracts

#### Semantic Ownership Matrix

| Concern | Owner | Contract |
| --- | --- | --- |
| Actor/message/event/time facts | Python | Read authoritative facts; never ask the model to invent IDs, ownership, or timestamps. |
| Intent, sentiment, social meaning, appraisal | LLM | Structured semantic result with bounded confidence and evidence references. |
| Candidate behavior and reflection meaning | LLM | Structured proposal only; it cannot execute side effects. |
| PAD/momentum/drive/relationship numeric delta | Python policy | Calculate from validated semantic signals, elapsed wall time, and policy version. |
| Schema, scope, authorization, safety, idempotency, CAS, transaction | Python | Reject invalid or stale proposals; never delegate these invariants to the model. |
| Workflow, retry, timeout, cancellation, compensation | Python runtime | Execute only a validated frozen decision. |
| Browser framing and redaction | Node BFF | Translate normalized application output; never reinterpret semantic content. |

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
- Python may reject a model proposal but cannot synthesize a semantic replacement. A hard policy may force a safe `no_op` or explicit refusal without claiming that the model made that judgment.
- Deterministic time decay, schedule boundaries, authorization, and safety continue to operate when the LLM is unavailable because they do not infer meaning.
- Persist structured results, evidence references, model/prompt/policy versions, and bounded explanations. Do not persist hidden reasoning or credentials.
- Action realization receives a frozen action and the post-transition read model. It cannot mutate or propose affect, drives, relationships, memory, goals, intentions, identity, or personality.
- Actions that produce no visible content, including `ignore` and `delay`, do not call realization.
- Reflection always runs in an owning background workflow over an explicit evidence window; it never shares the interactive realization response.
- Every source observation is persisted as an idempotent, monotonically sequenced inbox fact for its Fluctlight instance.
- One cognitive writer owns state transitions and action-delivery order for one Fluctlight instance. Different instances may execute concurrently.
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
| Model supplies raw PAD/trait/relationship delta | Reject the raw delta; Python policy remains the only numeric owner. |
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

- Good: the model classifies an ambiguous message as mixed concern and social distance with evidence references; Python validates ownership, computes bounded affect/relationship changes, and commits one revision.
- Good: multiple drives are high; the model proposes `delay_reply` with semantic reasons, Python checks current schedule and policy, freezes the decision, and the Worker executes it once.
- Base: the model returns a valid neutral assessment and no state-changing candidate; Python records `no_op` without manufacturing change.
- Base: an explicit timestamp expires an intention; Python closes it deterministically without an LLM call because no semantic interpretation is required.
- Bad: `/sorry|对不起|抱歉/` increases trust, an emoji table changes affect, message length chooses response style, or a fixed inactivity threshold marks a relationship as declining.
- Bad: a provider failure creates a default friendly reply, default appraisal, default personality, or keyword-derived memory.

### 6. Tests Required

- Contract tests reject malformed/unknown semantic schemas, raw numeric deltas, foreign evidence, stale revisions, and duplicate idempotency keys.
- Failure tests prove provider timeout, invalid JSON, and exhausted retries never call a heuristic classifier and never persist fabricated semantic state.
- Paraphrase and multilingual fixtures assert that application outcomes come from injected model results rather than exact wording in source text.
- Negative architecture tests scan Python Core and Node BFF production paths for newly introduced semantic regex/keyword dictionaries and require explicit review for any natural-language matching.
- State-transition tests assert numeric policy owns requested/applied deltas, clamps canonical ranges, records policy/model versions, and is independent of Worker tick frequency.
- Decision tests assert policy rejection produces no effect and no code-selected semantic alternative.
- Two-stage tests assert assessment emits no visible content, final action is frozen before realization, no-content actions skip realization, and realization cannot return semantic side effects.
- Realization failure/retry tests assert the same frozen decision is reused without another implicit assessment.
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
