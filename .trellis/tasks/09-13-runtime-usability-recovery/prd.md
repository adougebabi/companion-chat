# Runtime usability recovery

## Goal

Restore the Fluctlight runtime to a usable, diagnosable state by closing three
P0 failures together: recurring WakeUp/Reflection must run or expose a precise
reason why they did not; every major entry point and asynchronous boundary must
produce correlated, bounded diagnostics instead of silently falling through a
catch-all; and initialization must preserve the semantic information in both
single- and multi-personality character cards without returning to Provider
formats that make the configured local LLM time out.

## Background

- This is a repeated failure class rather than three isolated regressions.
  Local session history records “WakeUp fails once and never wakes again”,
  structured wake output that never reaches chat, missing failure logs, and
  initialization errors repeatedly collapsing into one generic code from
  2026-09-04 through 2026-09-13. Relevant Codex sessions include
  `01a06ce5-ef6f-7ce3-8623-37f6cc97d6cf`,
  `01a07226-04dd-7323-baeb-2231d71912d7`, and the 2026-09-12
  `initialization_persona_invalid` diagnosis sessions.
- Prior fixes improved individual error mappings and Visual Identity retry
  behavior, but they did not prove the full expected-trigger-to-outcome chain
  or detect the absence of an expected trigger.
- Initialization changed from strict `json_schema` constrained decoding and a
  full field-by-field extraction prompt to `json_object`, a compact skeleton,
  sparse required fields, and server that omitted fields are acceptable. This
  removed timeouts/invalid JSON on the configured local Provider, but the same
  source character card now produces substantially fewer semantic values.
- Core normalization can add missing keys as `null`, `{}`, `[]`, or neutral
  defaults. Structural completion does not recover source meaning that the LLM
  failed to extract.

## Requirements

### R1 — WakeUp and Reflection liveness

- Every active Fluctlight has one durable, idempotent recurring WakeUp cycle
  governed by the configured interval. Paused/retired states follow their
  explicit lifecycle policy and cannot accidentally keep invoking the LLM.
- WakeUp uses a fixed recurring cadence calculated from the prior WakeUp cycle;
  a user conversation never resets or postpones the full WakeUp interval. If a
  visible turn or Provider call is active at the due time, WakeUp may receive a
  short bounded jitter but retains the original cycle identity and due basis.
- PostgreSQL owns `next_due_at`, cycle, suppression, and release CAS. Redis TTL
  expiry is an optional low-latency hint only. Redis SET failure, lost Pub/Sub,
  disabled keyspace notifications, listener restart, or a crash between domain
  commit and Redis scheduling cannot permanently stop the next cycle.
- A Worker periodically sweeps due PostgreSQL WakeUp cycles and releases each
  cycle at most once. Duplicate Redis expiry, multiple Workers, and restart
  replay preserve one fact/Run per stable cycle identity.
- Reflection, unlike WakeUp, is an inactivity quiet-period one-shot. A new user
  turn resets the pending Reflection due time; it does not affect WakeUp.
- Paused/disabled WakeUp does not call the LLM but retains an explicit durable
  suppression/next-policy state. On resume/re-enable, an overdue cycle runs
  after short jitter; a future cycle keeps its original due time.
- The complete path is observable and testable:
  trigger scheduled → Redis release/fallback → PostgreSQL workflow intent →
  dispatcher selection → Temporal Run/Activity → Provider queue/model call →
  accepted/no-op/failed domain outcome → next trigger scheduled.
- Every branch that intentionally does no work records a stable reason such as
  not due, paused, inactive, superseded, missing prerequisite, already running,
  retry backoff, or no actionable decision. An expected WakeUp that is absent
  must be distinguishable from one that ran and chose no action.
- Every due WakeUp performs one internal autonomous cognition. A user-visible
  direct message is only one optional capability decision alongside moments,
  media, Schedule, Goal, Intention, or other bounded actions. WakeUp does not
  have to manufacture a visible message in order to count as successful.
- Internal assessment text, narrative state description, hidden reasoning, and
  raw Provider `text` never become a chat message. A visible private message
  exists only when the accepted WakeUp decision invokes the canonical
  communication capability and that capability settles successfully.
- A WakeUp with no selected capability is persisted as
  `completed_noop` with a stable reason. Actionable, blocked, suppressed, and
  failed outcomes are distinct diagnostic states, so an empty chat surface
  cannot be mistaken for a missing WakeUp.
- A WakeUp failure remains recoverable with stable IDs and bounded backoff; one
  failure cannot permanently stop future cycles.
- Reflection remains a separate, evidence-driven one-shot process released
  after its configured quiet period. WakeUp may produce a Reflection intent,
  but the two lifecycles and their diagnostics are not conflated.
- Visual Identity, media, summary, or other long work cannot starve due WakeUp,
  Daily Review, or Reflection work at dispatcher, Temporal Activity, or Provider
  queue boundaries.
- An accepted current-day Schedule is an optional high-value WakeUp context
  source, not a liveness prerequisite. Activation establishes both the stable
  Schedule lifecycle and stable WakeUp cadence independently.
- When Schedule is ready, WakeUp uses its current activity, place and temporal
  context. When Schedule is pending, failed or missing, WakeUp still performs
  cognition with an explicit `schedule_status`/missing-context marker and must
  not invent a current activity, location or plan.
- Schedule owns independent retry, failure diagnostics and recovery SLA.
  Diagnostics distinguish `running_without_schedule` or
  `waiting_on_schedule_context` from an absent WakeUp cycle.

### R2 — End-to-end diagnostics and no silent catch-all

- Audit all active Go Core/BFF/Worker entry points and asynchronous boundaries,
  prioritizing HTTP commands, dispatcher/reconciliation, Workflow/Activity,
  Redis triggers, Provider queue/model calls, capability execution, Schedule,
  WakeUp, Reflection, Visual Identity, and media settlement.
- A catch-all may protect the public API, but it must retain an internal typed
  category, stage, bounded cause/code, retryability, correlation ID, and owning
  Fluctlight/intent/workflow/run identifier where available.
- No branch may silently swallow an unexpected error, ignore a failed status
  update, or present a no-op as success without a stable diagnostic reason.
- Correlation and causation identities propagate across HTTP → domain command →
  intent/outbox → dispatcher → Temporal → Activity → Provider → persistence.
  Operators can query one identity and reconstruct the ordered lifecycle.
- Diagnostics distinguish at least: not triggered, scheduled/not due, queued,
  dispatched, already running, executing, waiting on dependency, retrying,
  completed/no-op, completed/actionable, cancelled, and failed.
- Key lifecycle transitions are persisted and shown as one ordered Diagnostics
  Center timeline filterable by Fluctlight, correlation ID, intent ID, workflow
  ID, and Run ID. The canonical transition vocabulary includes `scheduled`,
  `trigger_released`, `intent_created`, `queued`, `dispatched`,
  `already_running`, `activity_started`, `provider_queued`,
  `completed_actionable`, `completed_noop`, `retry_scheduled`, `failed`,
  `next_cycle_scheduled`, and `overdue`.
- A health audit emits `overdue` when an active Fluctlight has passed its
  configured WakeUp due time plus a bounded grace period without the expected
  durable intent/Run/outcome. Absence is therefore observable even though no
  error object exists.
- Do not persist every dispatcher/reconciliation poll. Emit a durable event only
  when state or reason changes, when an overdue threshold is crossed, or when a
  bounded sampling window permits a repeat. This prevents diagnostic storms
  while retaining first/latest cause and attempt counters.
- Diagnostic persistence failure is itself surfaced through a rate-bounded
  operational warning/health counter; the diagnostic writer must not silently
  discard its own failure.
- Logs and Diagnostics never expose credentials, authorization headers, hidden
  reasoning, raw database rows, unrestricted Provider payloads, or unbounded
  character-card/conversation content. Public errors remain smaller than
  operator diagnostics.
- Repeated identical failures are rate/bound controlled without erasing the
  first cause, latest cause, attempt count, or state transitions.

### R3 — Initialization semantic fidelity

- Support both single-personality and multi-personality cards. The analyzer
  determines the mode from source content and never defaults every card to
  `multiple` or fabricates extra profiles.
- Classify as `multiple` only when the source explicitly establishes distinct
  personalities through separate names/identities, stable differentiating
  traits, takeover/switch mechanisms or triggers, or cross-profile
  relationship/conflict/influence/integration. Ordinary situational contrast,
  mood variation, hidden facets, work/private behavior, or contradictory
  single-person traits remain `single` and map to scenario behavior, emotional
  state, behavior loops, secrets, or context-specific policy.
- Ambiguous evidence defaults to a complex single personality while preserving
  the source language; recall for implicit multi-personality is intentionally
  lower than the risk of fabricating extra profiles.
- Preserve all explicit source information. Canonical structured fields receive
  known values; material that has no canonical owner is preserved under a
  bounded, namespaced `extensions` field rather than discarded.
- Natural-language character design is the primary authoring form. The LLM may
  derive system-owned numeric traits, tendencies, frequencies, trigger rules,
  and behavioral policies when the source prose directly supports the
  interpretation; users are not expected to author those numbers manually.
- Every derived value remains traceable to preserved source semantics. The
  analyzer keeps the relevant original description alongside its structured
  projection so later review can distinguish an explicit fact, a directly
  supported interpretation, and a server default/empty default. Explicit
  source content must never be replaced by only a numeric projection.
- Activation persists the complete original character card as an immutable,
  Owner-only Foundation initialization source linked to the accepted Foundation
  revision, initialization correlation, Provider/model identity, prompt/schema
  versions, classification result, and structured projection.
- The original source is excluded from normal operational logs, default
  Diagnostics listings, and ordinary cognition prompts. It is read only for an
  Owner-authorized initialization-source view, semantic coverage audit,
  re-analysis, migration, or explicit repair.
- The Fluctlight detail surface provides a collapsed Owner-only initialization
  source view with source text, model/version, single/multi classification,
  extraction coverage summary, canonical/extension placement, supported
  derivations, and later Foundation revision relationship.
- Unsupported factual details are never inferred. For example, prose may
  support a lower baseline extraversion or a familiar-person behavior override,
  but it cannot establish an unstated blood type, birthday, birthplace, trauma,
  relationship, or life event.
- The supported base-information vocabulary includes name, nickname, gender,
  age, occupation, height, blood type, and birthplace. Existing identity
  fields such as residence, timezone, birthday, biography, values, worldview,
  and notes remain supported.
- Appearance supports a long descriptive source plus existing structured
  renderer fields and ordinary/daily outfit preferences. Renderer normalization
  must not replace or discard the richer human-readable description.
- Background story supports long-form narrative without forcing it into a
  short label or silently truncating it.
- Initial social relationships are currently limited to the Fluctlight’s
  directed relationship with `actor_user`. The contract reserves future Actor
  relationships but does not invent Fluctlight-to-Fluctlight relationships.
- Multi-personality extraction preserves the core relationship between
  profiles, each profile’s identity and traits, mandatory/forced activation
  mechanisms, trigger conditions, detailed temperament, speech style,
  behavioral habits, cross-profile influence, conflict, integration/fusion,
  and differentiation. Existing profile/switching/influence/conflict/
  integration/state-machine fields are reused and extended only where they
  cannot preserve the source.
- Special settings preserve profile-specific or shared habits, rituals,
  sensitivities, boundaries, and fixed behavioral patterns.
- Media behavior preserves desired frequencies and triggers for moments,
  selfies, artwork, and other configured media. Frequency is represented as a
  policy/preference, not immediately executed during initialization.
- Core conflict supports long-form description and structured profile conflict
  policy without reducing one to the other.
- Daily settings preserve likes, dislikes, preferred activities, fixed routines,
  habits, recurring commitments, and explicit “never/always” constraints using
  the current Life Profile and behavioral-policy owners where possible.
- Keep Provider execution practical for the configured local model. Do not
  restore the full strict initialization Schema decoder unless live evidence
  proves it meets the timeout budget. Prefer `json_object` plus a complete,
  explicit extraction contract, canonical skeleton, and Core normalization.
- “Omitted because absent in the source” is valid; “omitted despite explicit
  source content” is an acceptance failure. Empty/default values must not be
  counted as semantic preservation.
- A non-empty character card cannot succeed as an empty StructuredFallback or
  semantic-empty default Persona. If Provider JSON is truncated/unparseable or
  no source-supported semantic is extracted, return a typed, correlated,
  retryable analysis failure while retaining the source for review/retry.
- Browser, BFF and Core use one documented character-card input byte limit and
  reject oversize input explicitly; no layer silently truncates or imposes a
  smaller undocumented limit.
- Bounded shared semantic extensions required by the character card remain
  available to runtime Persona projection. They do not become disconnected
  provenance-only data, and infrastructure/debug metadata remains excluded.
- Initial social Relationship validation accepts only actor_user in this
  release. Profile-to-profile relations belong to personality_system and no
  arbitrary Actor relationship is invented.
- Preview keeps one authoritative typed Foundation value. Editing its
  goals/intentions or other fields changes the activation payload; a failed or
  superseded analysis marks the prior preview stale and disables activation.
- Initialization model-run diagnostics are metadata-only by default. They do
  not expose the original card or full structured response through ordinary
  Diagnostics.

### R4 — Evidence and rollout discipline

- Use two initialization acceptance fixtures. A dense, structurally equivalent
  anonymized single/multi-character-card fixture is committed for deterministic
  CI/regression coverage. The Owner’s real card is supplied only to an opt-in
  local Live Provider test through an external file path or environment
  variable and is never committed, copied into snapshots, or written to normal
  logs/diagnostic payloads.
- Reports produced from the real-card run contain only semantic coverage counts,
  missing semantic categories, bounded redacted assertions, timing, Provider
  identity, and pass/fail status. They do not persist the source card or full
  model response.
- Development follows the user’s strict stage isolation rules. Each stage may
  modify only its listed files and may run only exact tests for those files.
- S01–S11 do not run `./...`, race, Docker Compose, live deployment, or the full
  Provider suite. S12 alone owns combined validation and acceptance gates.
- Initialization acceptance includes real configured-LLM tests, not only local
  mocks: at least one dense single-personality card and one dense
  multi-personality card must be analyzed.
- Initialization live evidence compares the current result with the source and
  the `ea2c84ccbd36a8e4b93810af44f6f0a768a1d870` behavior at semantic-field
  level. It records missing, preserved, default-only, moved-to-extensions, and
  invented information separately.
- WakeUp/Reflection acceptance includes a deterministic short-interval test and
  a real Worker/Temporal run showing trigger, decision/no-op, Reflection handoff
  when applicable, retry recovery, and next-cycle scheduling.
- Diagnostics acceptance injects failures at representative boundaries and
  proves the correlation query identifies the exact failed/no-op stage without
  exposing prohibited data.

## Acceptance Criteria

- [ ] An active Fluctlight produces repeated WakeUp cycles across at least two
      intervals, or each suppressed cycle has an explicit policy reason.
- [ ] WakeUp cadence remains fixed during frequent user turns, while the same
      turns reset only the Reflection quiet period; missing Schedule and Redis
      outage do not prevent a due WakeUp.
- [ ] Killing/restarting Worker and forcing one WakeUp failure does not prevent
      a later cycle; IDs remain stable and effects are not duplicated.
- [ ] A due WakeUp and Reflection proceed while Visual Identity/media work is
      pending or retrying.
- [ ] One correlation query reconstructs every stage of a WakeUp and a
      Reflection, including intentionally skipped/no-op branches.
- [ ] Representative HTTP, dispatcher, Workflow, Activity, Provider, Redis,
      capability, and persistence failures have typed, bounded diagnostics;
      no tested failure collapses to an untraceable catch-all.
- [ ] The Diagnostics Center shows a transition-only Lifecycle timeline and
      emits an overdue event for expected-but-absent WakeUp work without
      leaking hidden reasoning, credentials, or unrestricted source content.
- [ ] The dense single-personality fixture remains single and preserves every
      explicitly asserted source fact with no fabricated profile.
- [ ] The dense multi-personality fixture preserves all required source modules:
      base information, appearance/outfits, background story, actor_user
      relationship, profile/core relationship, forced activation and triggers,
      per-profile speech/behavior, influence/integration, special habits, media
      frequency/triggers, core conflict, and daily preferences/routines.
- [ ] Initialization completes within the agreed live Provider timeout and
      returns valid JSON without relying on full strict constrained decoding.
- [ ] Truncated/unparseable or semantic-empty Provider output fails with a
      typed correlated error and never activates a neutral default Persona.
- [ ] Normalization adds safe missing containers/defaults but never treats those
      defaults as evidence that source semantics were preserved.
- [ ] The immutable original character-card source is Owner-only, linked to its
      Foundation revision and analysis provenance, visible through its dedicated
      detail surface, and absent from ordinary prompts/logs/Diagnostics.
- [ ] Initialization preview uses one typed Foundation state, honors edited
      goals/intentions, invalidates stale analysis, and uses one documented
      input limit across Web/BFF/Core.
- [ ] S12 required validation commands pass only after all earlier stage code
      and exact tests are complete.

## Out of Scope

- Building Fluctlight-to-Fluctlight social relationship gameplay in this task;
  only schema reservation and non-invention are required.
- Replacing Temporal, PostgreSQL, Redis, the Provider runtime, or the capability
  registry with a second execution system.
- Automatically generating media during initialization solely because a card
  declares a frequency preference.
- Claiming metaphysical consciousness or changing the established self-awareness
  product contract.
