# Fluctlight Intelligence P1 implementation plan

This task is deliberately one complete P1 slice. Do not archive or report it
as complete until the end-to-end behavior chain and failure cases below pass.
The visual-identity/avatar feature, HITL, P2 Harness, and independent SSE
subscription remain out of scope.

## Ordered implementation

### 1. Contracts and persistence helpers

- [x] Define strict Go contracts for `ContextProjectionV1`, `ClaimV1`,
  `SelfEvaluationV1`, `ResponsePlanV1`, `SceneEventCandidateV1`,
  `PresenceEventCandidateV1`, `MemoryQueryV1`, `MemoryContextV1`, and
  `ReflectionCandidateV1`.
- [x] Add explicit lifecycle/error enums and bounded reason codes. Unknown
  fields, malformed numeric semantics, foreign evidence, and oversized payloads
  fail closed; no semantic defaults.
- [x] Add revision/provenance/idempotency helpers shared by native capability
  authorities. Keep one canonical source for Context Projection.
- [x] Preserve the existing `ToolCallV1`/`ToolResultV1` registry and extend it
  for native slots without exposing SQL or plugin-owned tables.

### 2. Context Projection and claim authority

- [x] Implement a single Context Projection service that reads current fact,
  identity/personality/policy, inner state, resolved Event/Schedule/pending,
  Presence, authorized Memory, Relationship, and active hypotheses.
- [x] Enforce source/revision/confidence/expiry on every projected item and use
  owner/visibility/Actor/Conversation hard filters before ranking or prompt
  injection.
- [x] Replace ordinary cognition's direct history/profile assembly with this
  bounded projection. Keep the full transcript as record, not as truth.
- [x] Add correction/supersede/expiry handling so rejected claims cannot return
  to normal context.

### 3. Adaptive cognition and self-evaluation

- [x] Implement the ordinary-chat fast path: one cognition call returns visible
  draft + ResponsePlan + SelfEvaluation + optional tool/native candidates.
- [x] Implement the deliberate path only for effects, scene/memory/relationship
  candidates, or high uncertainty. Freeze the plan before any side effect.
- [x] Make realization a presentation compiler for the frozen plan; it receives
  approved claims and tool results, not a second copy of the full semantic
  prompt. It cannot add facts/effects.
- [x] Enforce at most one bounded rewrite. On failure, omit unsupported claims,
  use an uncertainty result, or defer; never loop indefinitely.
- [x] Record normalized claim/topic signatures and suppress repetition without
  new evidence.

### 4. Native scene/presence slots

- [x] Register `scene_event` and `presence_event` as native capability slots with
  manifests, strict candidate schemas, evidence/time bounds, and idempotency.
- [x] Implement Life World authority application: confirmed Event versus
  decaying Hypothesis versus Presence overlay versus no-op.
- [x] Ensure accepted scene changes enter the Fluctlight ordered cognition
  stream; Presence can only overlay `user_presence/current_task`.
- [x] Reuse the same resolved context in chat, media prompt, reflection and
  self-evaluation. A repeated candidate with no new evidence is replay/no-op.

### 5. Memory authority and retrieval

- [x] Create a Memory authority boundary for record/revise/forget/retrieve and
  initial revision + embedding intent in one transaction.
- [x] Implement authorization-first retrieval with FTS/vector/hybrid ranking,
  bounded rerank, recency/importance/confidence weighting and token budget.
- [x] Inject only `MemoryContextV1` items carrying memory ID/type/confidence,
  source/evidence refs and revision into Context Projection.
- [x] Ensure reflection candidates use the authority port rather than direct
  `INSERT memories`; embedding failures preserve the Memory row and are
  retryable/rebuildable.

### 6. Reflection and evolution

- [x] Produce a stable `reflection.run` intent after completed cognition facts
  and/or a durable periodic trigger; claim one Fluctlight evidence window with
  lease and CAS.
- [x] Validate candidate evidence against the claimed window and owner; reject
  candidates with missing/foreign evidence or semantic defaults.
- [x] Apply Memory, Relationship and Self-model candidates through their
  authority ports; add slow Personality/Identity evolution only after the
  evidence threshold and revision policy pass.
- [x] Persist prior revision, source window, provider/prompt/schema versions,
  confidence, reason codes and rollback identity.
- [x] Rebuild Context Projection after accepted revision so future cognition
  actually consumes the new state.

### 7. End-to-end closure and regression tests

- [x] Test current-input priority, unsupported self-claim omission, uncertain
  wording, one bounded rewrite, repetition suppression, correction/supersede,
  and no semantic loop.
- [x] Test scene/presence candidate acceptance, duplicate no-op, temporal bounds,
  context resolution and cognition inbox ordering.
- [x] Test Memory authorization-before-ranking, FTS/vector/hybrid fallback,
  token budget, embedding pending/failed/retry, revise/forget and rollback.
- [x] Test Reflection trigger, window claim/CAS, evidence ownership, candidate
  apply, slow/medium/fast evolution gates and future prompt consumption.
- [x] Test native tool/JSON sidecar normalization, tool result persistence,
  frozen replay, external effect idempotency, cancellation, NDJSON chunking and
  one terminal frame.
- [x] Add a deterministic scenario that runs the full chain:

  ```text
  user fact
    → context projection
    → self-evaluated response/scene/memory candidate
    → freeze/effect
    → evidence window
    → reflection candidate
    → governed revision
    → next turn consumes revision
  ```

## Validation commands

Run from the phase worktree:

```bash
gofmt -w apps/core-go/internal/core apps/core-go/internal/httpapi apps/gateway-go/internal/bff
go -C apps/core-go test ./...
go -C apps/gateway-go test ./...
go -C apps/core-go vet ./...
go -C apps/gateway-go vet ./...
go -C apps/core-go build ./...
go -C apps/gateway-go build ./...
go -C apps/core-go test -race ./...
go -C apps/gateway-go test -race ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
```

PostgreSQL/Redis/HTTP loopback tests must run in an environment that permits
the required services. If browser dependencies cannot be installed, record
the exact network blocker and do not claim the browser gate passed.

## Risk files and rollback points

- `apps/core-go/internal/core/mutations.go`: adaptive fast/ deliberate turn
  paths, frozen ResponsePlan, realization boundary and stream callbacks.
- `apps/core-go/internal/core/cognition.go`: ordered fact claim, state/revision
  CAS, reflection trigger and replay semantics.
- `apps/core-go/internal/core/detail.go`: single Context Projection and
  Event/Schedule/Presence precedence.
- `apps/core-go/internal/core/operations.go`: Memory/Life World/Relationship
  authority writes and revision ledgers.
- `apps/core-go/internal/core/workflow_ops.go` and `workflow.go`: reflection
  window, candidates, embedding and durable intent dispatch.
- `apps/core-go/internal/core/provider.go` and capability contracts: strict
  structured output, tool/native slot normalization, provenance and limits.
- `apps/gateway-go/internal/bff/ndjson.go` and browser client: public frame
  allow-list, redaction, sequence and cancellation.

Every migration is additive and reversible. Existing domain rows and workflow
IDs remain valid; an unready capability is disabled/deferred rather than
silently falling back to a different semantic source.

## Completion gate

Do not mark this task complete merely because compilation or a happy-path chat
passes. Completion requires the end-to-end scenario and each P1 acceptance
criterion in `prd.md`, plus an explicit record of any environment-only checks
that could not run.

## Validation record

- [x] `go -C apps/core-go test ./...` passed.
- [x] `go -C apps/gateway-go test ./...` passed.
- [x] Both Go modules passed `vet` and `build`.
- [x] Full Go Core and Gateway `-race` suites passed outside the sandbox,
  including loopback Redis/HTTP tests.
- [x] `git diff --check` passed.
- [ ] Browser `pnpm generate/typecheck/test/build` could not be completed in
  this environment because the new worktree has no installed Node packages and
  `registry.npmjs.org` DNS resolution is unavailable. No lockfile or generated
  browser artifact was changed; this is an environment-only follow-up gate.
