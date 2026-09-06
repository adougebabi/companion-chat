# Implementation Plan

## Phase A — Contracts and persistence

1. Add the Actor-aware provider context contract and stable alias builder.
   - Define self Actor, participant Actor, sender Actor, and relationship
     snapshot shapes.
   - Preserve raw `author_actor_id` in internal projections while redacting it
     from natural-language rendering when an alias is available.
2. Extend Relationship validation and persistence with a typed role envelope.
   - Normalize legacy rows to `unknown`.
   - Keep Owner/Creator authorization metadata separate from social
     Relationship rows.
   - Materialize optional initialization relationship seeds during Fluctlight
     creation/activation.
   - Persist relationship provenance (`initialization` or `reflection`) and
     evidence refs.
   - Preserve relationship revision/evidence/idempotency behavior.
   - Add role fields to reflection candidates and their apply path.
3. Extend goals and intentions with relationship scope and optional
   `target_actor_id`.
   - Keep generic goals backward-compatible.
   - Accept optional relationship-goal seeds at initialization.
   - Add candidate operations and revision/audit writes for Reflection-driven
     goal/intention evolution.

## Phase B — Provider pipeline

4. Build a speaker-scoped relationship snapshot in ContextProjection.
   - Resolve the current speaker from the authorized message/conversation
     participant, never from an untrusted browser Actor ID.
   - Select only the self-to-speaker relationship and matching relationship
     goals/intents.
5. Update provider composition.
   - Render an explicit `# Actor 与关系上下文` system section.
   - Keep `system/user/assistant` as transport roles.
   - Render recent/current messages with sender Actor aliases.
   - Ensure the same frozen snapshot is used by assessment, tool decision and
     action realization.
6. Add the read-only `relationship.lookup` capability.
   - Validate target Actor scope and return a bounded direct relationship view.
   - Add no write or external side effect.

## Phase C — Reflection and action integration

7. Separate cognition and Reflection contracts in prompts, schemas, and tests.
   - Cognition may read relationship context but cannot emit/apply long-lived
     Relationship/Goal/Intention mutations.
   - Reflection can emit relationship, goal, and intention candidates.
   - Reflection can create a relationship from `unknown`, revise an
     initialization seed, and create/adjust a relationship goal from evidence.
8. Make relationship-scoped goals participate in decision input and action
   selection.
   - Match current speaker/target Actor before applying a relationship goal.
   - Carry goal/intent evidence and revisions into the frozen decision.
   - Keep external actions behind existing capability, permission, budget and
     idempotency gates.
9. Ensure Reflection can autonomously apply semantic evolution without Owner
   approval while retaining evidence/CAS/audit guarantees.

## Phase D — API/UI projection and verification

10. Update Core/BFF/browser DTOs and detail projections to expose Actor-aware
    messages, relationship role/state and relationship-scoped goals without
    breaking existing field aliases.
   - Enrich relationship targets with `target_actor_type` and server-derived
     `is_current_user`.
   - Add the authenticated relationship edit command and browser client method;
     keep target Actor immutable and use expected-revision CAS.
11. Add the relationship editor to the Governance UI.
   - Show Human/Fluctlight type and a clear “当前用户” marker.
   - Edit role/labels, metrics, trend, summary and emotional association.
   - Collect evidence refs and governance reason; show revision conflicts.
12. Add regression tests for:
    - mixed Human/Fluctlight sender rendering;
    - system relationship snapshot and revision refresh;
    - direct relationship role and target matching;
    - relationship goal filtering and action selection;
    - cognition no-write boundary;
    - Reflection autonomous relationship/goal/intention revisions;
    - lookup authorization and no global relationship leakage;
    - Actor type/current-user projection;
    - relationship edit CAS, audit row and immutable target Actor;
    - existing single-owner direct conversation compatibility.

## Validation commands

- `gofmt -w` on changed Go files.
- `GOCACHE="$PWD/.gocache" go test ./apps/core-go/...` (or the repository's
  focused Core package command if the full suite is unavailable).
- Existing browser/client tests for conversation and detail DTO compatibility.
- Focused integration tests against the migration-backed Core persistence for
  relationship revision, goal target matching, and Reflection CAS/idempotency.

## Risky files / rollback points

- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/provider_prompt_composer.go`
- `apps/core-go/internal/core/provider_prompts.go`
- `apps/core-go/internal/core/workflow_ops.go`
- `apps/core-go/internal/core/provider_schemas.go`
- `apps/core-go/internal/core/tool_contract.go`
- `apps/core-go/internal/core/capability_runtime.go`
- `apps/core-go/internal/migrations/runner.go`
- Core/BFF/browser conversation and detail DTOs

Rollback points:

1. Provider projection can fall back to the legacy `user/assistant` rendering
   while retaining persisted Actor IDs.
2. New optional relationship/goal fields can be ignored for legacy rows.
3. Reflection candidate application remains idempotent by source-window keys;
   failed migration or contract rollout must not rewrite prior revisions.

## Gate before activation

- Re-read the final PRD top to bottom and remove resolved open questions.
- Review `design.md` and `implement.md` with the user.
- Load `trellis-before-dev` and relevant backend/frontend spec indexes only after
  the user approves the planning artifacts.
- Run `task.py start` only after planning review; do not implement while status
  remains `planning`.
