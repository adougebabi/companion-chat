# Capability Runtime

The Core exposes one Capability Runtime to cognition, WakeUp, autonomy, and
replay. A Provider sees only the definitions selected for its surface; it does
not see executors, database fields, workflow settings, or planner schemas.

```text
native/sidecar provider shape
  -> one codec normalization boundary
  -> CapabilityInvocation
  -> CapabilityRegistry.Catalog/Lookup
  -> ContextResolver (declared ContextSlot values)
  -> Capability preflight/Prepare
  -> frozen PreparedPayload
  -> Execute / caller-owned transactional apply
  -> CapabilityResult
  -> frozen action, target settlement, and later cognition
```

## Core objects

- `CapabilityDefinition` is the stable Provider contract. It contains a short
  description, thin `InputSchema`, output metadata, surfaces, type, target
  kinds, failure policy, and declared context slots.
- `CapabilityInvocation` is the only runtime and persistence call envelope.
  Provider-native and root sidecar shapes are normalized into it exactly once;
  frozen IDs, provider request IDs, and replay keys remain stable. Immutable
  Provider `Arguments` and versioned Runtime-owned `PreparedPayload` are
  physically separate; a Provider cannot inject a planner result through the
  thin input object.
- `CapabilityContext` contains typed persona, state, life, schedule, visual,
  appearance, relationship, memory, and agency values. `ContextResolver`
  resolves only the slots declared by the selected definition and preserves a
  bounded snapshot for replay.
- `CapabilityResult` is provider-safe and business-agnostic. The Runtime
  records status, bounded output, error code, retryability, duration, and
  correlation identity; it never interprets a domain payload.
- `CapabilityRegistry` owns registration, duplicate detection, stable ordering,
  and generic `Catalog(surface)` filtering. A new capability is implemented and
  registered without editing MainAgent or adding a name switch.

## Thin input and implementation ownership

Thin input contains only decisions the model must make. Current boundaries are:

| Capability | Public decision fields | Core-owned implementation fields |
| --- | --- | --- |
| `media.image.generate` | `intent` | visual concept, renderer/workflow settings, context binding, provider IDs |
| `schedule.replan` | `intent` | local date, timezone, revision/CAS, completed boundary, full replacement |
| `memory_event` | `content`, `type`, `confidence`, `importance` | owner, visibility, evidence, actor/event refs, idempotency, embedding |
| `scene_event` | `operation`, `confidence`; start/switch require `scene` and `activity`; optional `location` | source/evidence/time/idempotency and transaction rules |
| `presence_event` | at least one bounded presence/task field plus `confidence`; optional expiry | actor scope, source, evidence, persistence and expiry validation |
| `affect_event` | semantic event type and `confidence` | reducer, numeric state, evidence and revision/CAS |
| `relationship.lookup` | `target_actor_id` | authorization scope, profile selection, live relationship row |

Runtime may attach only mechanically known provenance such as the current
source fact, evidence reference, and stable call-derived idempotency key. It
never invents semantic
confidence, scene operation, memory type, schedule contents, or a fallback
meaning. Missing semantic input fails closed.

Image and schedule may use capability-local structured planners. Planner
schemas are internal and are never placed in the Main LLM `tools` catalog or
used to start a second MainAgent reply turn. Planner failure is an execution
failure; there is no heuristic or thick-schema fallback.

The current cutover exposes `schedule.replan` as `{intent}`. A capability-local
planner uses the structured `cognitive_assessment` provider role outside the
mutation transaction and must produce the complete validated Core schedule
payload before `ReplanSchedule`; its schema is never exposed to Main LLM.

## Execution and failure policy

The Runtime executes calls in Provider order. Deferred output capabilities first
record a bounded `deferred` result, then bind to a durable conversation message,
Moment, or WakeUp target in the caller-owned transaction. External Provider,
storage, Redis, and workflow calls remain outside PostgreSQL transactions.
Native Memory/Affect mutations implement the transaction-aware Capability seam:
interactive settlement applies each in a pgx savepoint owned by the assistant
transaction. Optional failure rolls back only that mutation and records a failed
result; required failure rolls back the complete visible settlement.

Definitions declare either `required_for_visible_claim` or `optional_internal`.
The former fails closed: a failed state-changing capability rolls back a visible
assistant settlement that could claim the change happened. Optional failures
remain structured and auditable without fabricating success. Ordinary
conversation has exactly one Main LLM cognition call; results are persisted for
replay/later cognition and are not sent as a same-turn `role=tool` continuation.

## Adding a capability

1. Define a stable name/version, short description, thin schema, type, surfaces,
   target kinds, failure policy, and the smallest required context slots.
2. Implement `Capability` and only the required optional prepare,
   transactional, or deferred interface. Inject a narrow domain dependency;
   the Capability must not retain the whole `*App` service locator.
3. Register it in the application composition root.
4. Add focused definition/schema, registry, context, runtime, cancellation,
   failure, and replay tests.
5. Measure the serialized `RenderCapabilityTools` bytes/chars and update the
   inventory/report. Token usage is reported only when the Provider envelope
   supplies it; this runtime does not add a tokenizer.

## Forbidden practices

1. Do not add a concrete capability-name switch to MainAgent or the generic
   Runtime.
2. Do not expose camera, workflow, sampler, database IDs, evidence storage
   fields, or other implementation details in a thin schema.
3. Do not duplicate a capability schema in prompts or a nested response-plan
   `tool_calls`; the root codec sidecar is the only compatibility channel.
4. Do not infer semantic effects from keywords, regexes, prose, or visible
   reply text.
5. Do not claim an external effect in prose without a real Capability call.
6. Do not open a second Main LLM continuation for ToolResults or an internal
   planner.
7. Do not run Provider, Redis, object storage, HTTP callbacks, or workflow APIs
   inside a PostgreSQL business transaction.
8. Do not bypass ContextResolver with ambient global state or an untyped slot
   bag; declare and resolve typed context slots.
9. Do not submit a new external job on retry; preserve provider/workflow and
   idempotency identities and reconcile the existing request.
10. Do not log prompts, hidden reasoning, credentials, or unbounded context;
    runtime lifecycle records are bounded and redacted.

## Project Health and Reflection V2

Capability Runtime is the Action plane; Reflection V2 is the asynchronous
Evolution plane. It consumes only a bounded Provider-safe evidence window:
opaque `sequence:*`/context/outcome refs, bounded event semantics, user
observation, appraisal allowlist and strict ActionOutcome projection. Assistant
visible text, hidden reasoning, transcripts, raw expected/observed results,
database/provider/workflow/idempotency identities, provenance and the internal
ContextReferenceIndex never cross this Provider boundary.

`ReflectionProposalV2` is closed and semantic-only. Core resolves its refs,
owns numeric reducers and commits every accepted domain mutation together with
proposal, per-candidate disposition, revisions and watermark in one
caller-owned transaction. Goal/Intention typed triggers reuse durable
`intention.trigger` and `agency.intention_due`; Personality/Behavior evolution
uses a separate profile-scoped overlay and next-projection `effective_persona`
without modifying Identity/Core Persona baselines.

Active/frozen executable payloads require the canonical runtime version,
CapabilityInvocation/Result arrays, ContextReference authority and complete
frozen projection/composite/response snapshots. Missing or dual authority fails
closed; only terminal history may remain audit-readable in an older shape.

## Compatibility and rollout

Migration head `0026_capability_runtime` owns the cutover from the released
`0025_llm_queue` bundle. `capability_runtime_version=v2`, `capability_invocations`, and
`capability_results` are the sole active frozen/action authority. The bounded
Go migration converts still-executable released rows transactionally, moves
legacy prepared/provenance data out of Provider Arguments, converts legacy
results, builds bounded Slot snapshots, validates active workflow authority,
aborts on malformed active payloads, and leaves completed/failed rows untouched.
Replay skips already-completed calls; workflow settlement and recovery read
only canonical fields, with no runtime v1 fallback. The current migration chain
continues through `0027_project_health_evolution`, `0028_affect_canonical`,
`0029_memory_lifecycle`, `0030_life_context_revision`, and
`0031_evolution_authority`; `0030`/`0031` are clean-start-only and reject any
existing business authority instead of repairing or reinterpreting it.
