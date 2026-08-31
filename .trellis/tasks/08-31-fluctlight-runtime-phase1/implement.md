# Fluctlight Runtime phase 1 implementation plan

This is a bounded first slice. It establishes the canonical provider/tool and
transport seams without claiming that Memory, Reflection, Autonomy, or Life
World are already complete.

## Ordered checklist

### 1. Contract and schema owner

- [x] Add a Go-owned canonical schema package (or equivalent single owner) for
  `ToolCallV1`, `ToolResultV1`, typed assessment/action envelopes, capability
  manifest, and explicit outcome/error codes.
- [x] Reject unknown fields, oversized arguments, invalid call IDs, unsupported
  capabilities, and missing source/correlation IDs. Do not fill malformed
  semantic values with defaults.
- [x] Keep Memory/Reflection/affect/evolution as Runtime authority ports; do
  not expose SQL/table access through the tool registry.
- [x] Update the stale structured-turn transport wording from SSE to NDJSON.

### 2. Provider normalization

- [x] Extend the Go provider adapter to normalize native tool-call deltas and a
  canonical JSON sidecar into the same `ToolCallV1` representation.
- [ ] Preserve provider request/model/prompt/schema versions and stable
  `provider_request_id` in diagnostics and frozen action payloads.
- [ ] Keep the old `action_type`/`visual_concept` shape behind an explicit
  compatibility adapter only; no new prose-marker parsing.

### 3. Durable action seam

- [ ] Introduce a single application boundary that validates a proposal,
  checks capability/resource ownership, state revision, idempotency,
  concurrency class, and bounded budgets, then creates a frozen action.
- [x] Persist tool call/result and action status transitions with stable IDs.
- [ ] Ensure retry reuses the frozen proposal/provider request and that an
  ambiguous external result is `result_unknown` plus reconciliation, not a
  second submission.
- [ ] Keep the no-human-approval product rule: failed deterministic checks
  produce rejected/deferred/error outcomes.

### 4. Real incremental turn stream

- [x] Thread Provider `StreamText` chunk callbacks through Core NDJSON frames
  and the existing Go BFF translator.
- [x] Keep one monotonic sequence and one terminal frame; suppress hidden
  provider/tool internals at the BFF boundary.
- [x] Verify browser abort cancels provider/Core reads while committed domain
  settlement remains durable.
- [x] Do not add SSE to the turn endpoint. If a separate server-push need is
  proven later, design it as a new projection-only subscription.

### 5. Tests and documentation

- [ ] Unit-test native tool-call and JSON-sidecar normalization, strict schema,
  unknown capability, oversized input, duplicate call id, tool result
  round-trip, and compatibility adapter boundaries.
- [ ] Add Core/BFF integration tests for split UTF-8/chunk streaming, early
  token delivery, abort, terminal uniqueness, and no hidden payload leakage.
- [ ] Add idempotency/replay tests proving a processed fact and frozen action do
  not call the provider twice.
- [ ] Record explicit follow-up tasks for Memory retrieval, Reflection
  producer/window, Autonomy freeze/policy, Life World facts, and media video/
  audio plugins.

## Validation commands

Run from the phase worktree:

```bash
go test ./apps/core-go/...
go test ./apps/gateway-go/...
go vet ./apps/core-go/...
go vet ./apps/gateway-go/...
pnpm --filter @fluctlight/browser-client test
pnpm typecheck
```

If dependencies are unavailable in the sandbox, run the focused pure Go tests
that do not require PostgreSQL/Temporal and record the blocked integration
checks rather than treating them as passing.

## Risk files and rollback points

- `apps/core-go/internal/core/mutations.go`: current synchronous turn path and
  pseudo-streaming behavior. Keep a compatibility path until chunk-to-frame
  tests pass.
- `apps/core-go/internal/core/cognition.go`: fact/frozen-action persistence;
  any transaction change must preserve existing idempotency keys and migrate
  old pending rows safely.
- `apps/core-go/internal/core/provider.go`: provider normalization and stream
  cancellation. Do not silently fall back from native tool calls to prose.
- `apps/core-go/internal/workflow/workflow.go` and
  `apps/core-go/internal/platform/redis_pipeline.go`: workflow/outbox recovery
  boundaries. Do not change queue ownership in this slice.
- `apps/gateway-go/internal/bff/ndjson.go` and `routes.go`: browser contract;
  preserve redaction, sequence and terminal guarantees.
- `packages/browser-client/src/index.ts` and generated OpenAPI artifacts:
  update together if the frame schema changes; no SSE/NDJSON dual contract for
  one endpoint.

Rollback is additive: disable the new canonical tool-call capability manifest
and retain the compatibility adapter while database rows and workflow IDs stay
unchanged. Never delete existing frozen actions or outbox events as part of a
transport experiment.

## Before implementation starts

- [x] User confirms the recommended scope: native tool calls are the control
  plane for external capabilities and narrowly typed native intents; Memory,
  Reflection, affect, and evolution remain Runtime-owned authority services;
  no human approval is introduced.
- [x] User accepts POST NDJSON for turns and a future, separate SSE subscription
  only if server-push is later required.

## Validation record

- [x] `go -C apps/core-go test ./...` passed outside the sandbox, including the
  Redis loopback integration test.
- [x] `go -C apps/gateway-go test ./...` passed outside the sandbox, including
  the public HTTP boundary test.
- [x] `go vet ./...`, both Go builds, focused race tests for Core/BFF packages,
  and `git diff --check` passed.
- [ ] Browser `pnpm` checks remain to be run after dependencies are available;
  the first attempt was blocked because the worktree had no installed
  packages and the sandbox could not resolve `registry.npmjs.org`. No lockfile
  or generated browser artifact was changed.

This phase deliberately leaves the full Memory retrieval, Reflection producer,
Autonomy policy/freeze, Life World fact ordering, and video/audio provider
acceptance for follow-up slices described in `prd.md`.
