# T08 Design Baseline

## Authority

Parent task `08-24-python-core-architecture-refactor` owns architecture and cross-task contracts. This child may refine only implementation detail inside its approved scope after its child-specific brief is reviewed.

## Scope

life_world module、Context/Event/full-day Schedule/timezone/replan、Goals/Intentions workflows、pending/deferred/proactive actions、budget/quiet-hours/pause/cancel/reconciliation.

## Dependencies

T07 completed and merged.

## Boundary Rules

- Use only public module/application interfaces and assigned shared-file ownership.
- No old-code maintenance, compatibility, alternative infrastructure/runtime, or silent decision changes.
- External I/O, persistence, semantic and security behavior follow assigned parent code-specs.
- Product remains uncut-over until T12.
- This child owns implementation evidence only. T12 owns all functional acceptance and re-runs the required Life-world/Schedule/Autonomy scenarios; no child PASS is established here.
- `pending/deferred` are real Required lifecycle states only when connected to an authoritative producer/consumer; placeholder-only routines are excluded from positive acceptance.

## Local-Day Recovery Decision

Owner decision, 2026-08-27: a Fluctlight's Schedule is authoritative only for
its current local date. Activation creates the current local-day Schedule. At
each local-day boundary, the durable Life World workflow creates the next
local-day Schedule. If the NAS, Worker, or workflow runtime was offline across
that boundary, recovery checks whether the current local date has an accepted
Schedule and generates only that missing current-day version. It must never
backfill missed past dates, because doing so would fabricate unobserved life
history.

The same current-day-only compensation may be enqueued by an eligible
user-facing interaction when a current-day Schedule is absent. The browser
request does not synchronously wait for an LLM; it observes a pending or
completed projection. Timezone calculations use the Fluctlight's canonical
IANA timezone.

## Initialization Agency Decision

Owner decision, 2026-08-27: description-based creation uses the
`initialization` model to propose the initial Goal and Intention set together
with the Foundation. Python validates and persists the model-owned semantic
content through the existing Goal/Intention domain commands; it never invents
an objective, action, trigger, or schedule meaning. White-paper creation uses
a neutral Foundation and intentionally leaves Goals/Intentions empty until
lived facts, interaction, cognition, or reflection form them.

## Readiness Gate

Before `task.py start`, parent must add exact decisions/manifests/paths/implementation-check commands/T12 coverage IDs/rollback and pass a no-history handoff dry run. Until then this document is program-level planning only.
