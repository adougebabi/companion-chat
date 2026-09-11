# Implementation Plan

状态：in_progress；所有步骤执行于 `/private/tmp/local-ai-companion-llm-capability-runtime` 的 `codex/llm-capability-runtime` 分支。不得在 `master` 工作区运行写入性实现命令。`task.py start` 已完成。

## Ordered checklist

### S00. Worktree and baseline gate

- [x] Assert `git branch --show-current` is `codex/llm-capability-runtime` and `git worktree list` points the active path at `/private/tmp/local-ai-companion-llm-capability-runtime`.
- [x] Snapshot dirty paths; preserve unrelated task files and do not stage them accidentally.
- [x] Re-run baseline Core/BFF `test`, `test -race`, `vet`, `build`, and `gofmt`; socket-listener failures remain the known restricted-sandbox limitation, while DB/live-provider checks remain opt-in.
- [x] Inspect and migrate active `cognition_frozen_actions`/autonomy payloads with strict postflight; verify explicit Temporal legacy fence/Worker Deployment cutover in a disposable stack. No existing production history was supplied for replay.

### S01. Capability core foundation

Likely files: `apps/core-go/internal/core/tool_contract.go`, `capability_runtime.go`, `app.go`, new `capability_core.go` / `capability_context.go` and focused tests.

- [x] Introduce canonical `CapabilityType`, `CapabilitySurface`, `CapabilityDefinition`, `CapabilityInvocation`, `CapabilityResult`, `CapabilityRegistry`, `ContextSlot`, `ContextRequest`, `CapabilityContext`, and `ContextResolver` in the existing `core` package.
- [x] Convert all active capability usages to the canonical objects; keep persisted JSON field names and schema versions stable.
- [x] Make Registry construction fail deterministically on nil, invalid, duplicate, or incompatible registrations; preserve stable catalog ordering and add surface filtering without concrete-name exclusions.
- [x] Move the provider renderer to `RenderCapabilityTools` over definitions. Keep one OpenAI-compatible boundary and no user-context schema copy.
- [x] Add sentinel/wrapped errors for not-found, invalid arguments, context resolution and execution failures.
- [x] Add registry/renderer/error tests before migrating domain implementations.

### S02. Context resolver

Likely files: new `capability_context.go`, existing `intelligence.go` read helpers, `media_context.go` tests, new resolver tests.

- [x] Implement slot-specific PostgreSQL loaders for only confirmed current dependencies: persona, current state, current life, schedule, visual identity, appearance, relationship scope, memory scope and agency.
- [x] Deduplicate duplicate slot requests inside one resolution request and preserve the request context.
- [x] Expose typed accessors/value objects rather than a public `map[string]any` slot bag.
- [x] Persist a bounded frozen context snapshot alongside the frozen invocation for replay; keep live authorization/revision/CAS checks at execution.
- [x] Cover successful resolution, missing slot, loader error, cancellation, snapshot round-trip and no unrelated slot load.

### S03. Definition and implementation migration

Migrate all 11 capabilities in one branch-wide pass. Temporary local adapters are allowed only while compilation is being moved and must be removed before S06.

- [x] `conversation.reply` and `moment.publish`: retain explicit text and generic deferred target settlement; visible-action selection uses target metadata.
- [x] `media.image.generate`: expose only `{intent}` in the Provider schema, compile the intent into the internal concept/context binding, and move preflight behind the capability seam.
- [x] `visual_identity.initialize`: expose only on WakeUp surface with empty input; keep initialization restrictions inside the capability/policy.
- [x] `scene_event` / `presence_event`: Provider schemas retain semantic state fields while implementation provenance is resolver/runtime-owned.
- [x] `schedule.replan`: Provider boundary is thin and intent-only; a domain-specific capability-local planner validates and freezes the replacement outside the mutation transaction. Missing planner input fails closed and never falls back to a thick schema.
- [x] `memory_event`: Provider schema keeps content/type/importance and removes storage/provenance fields; authoritative validation remains in Core.
- [x] `affect_event`: Provider schema keeps semantic event/confidence and no numeric reducer fields.
- [x] `relationship.lookup`: target actor remains explicit; resolver declares relationship scope while live authorization stays in the executor.
- [x] `capability.request`: Provider schema keeps the requested contract and removes evidence/storage/idempotency details.
- [x] Every implementation accepts `context.Context`, consumes `CapabilityContext`, and avoids undeclared ambient context reads.

### S04. Main flow and Prompt cutover

Likely files: `mutations.go`, `wakeup.go`, `autonomy.go`, `cognition_growth.go`, `workflow_ops.go`, `provider.go`, `provider_prompts.go`, `provider_prompt_composer.go`, `provider_schemas.go`, `media_context.go`, `composite_actions.go`.

- [x] Replace scenario-specific catalog calls with `Registry.Catalog(surface)`.
- [x] Route immediate/deferred execution through the canonical Runtime and generic target settlement.
- [x] Remove production use of name-specific media preflight, media context, reply-action, legacy media, and reply-rewrite branches.
- [x] Preserve native-vs-sidecar normalization, one cognition call, tool-only no-op, frozen action replay, deferred output target ordering, and current browser NDJSON semantics.
- [x] Add generic capability failure-policy metadata and fail closed before visible settlement when a required state-changing invocation fails; optional internal failures remain bounded ToolResults.
- [x] Add a compact generic capability policy to the active cognition/WakeUp/daily-review prompt paths; remove the former per-scenario prompt constants.
- [x] Ensure no new planner schema appears in provider `tools` or ordinary compact context.
- [x] Keep one root structured sidecar compatibility channel, remove duplicated nested `response_plan.tool_calls`, and preserve native-call precedence.

### S05. Persistence/replay and migration cleanup

- [x] Add a bounded migration/reconciliation SQL path plus a pure payload migration helper for still-executable v1 calls; completed audit history remains untouched and incomplete active provenance fails closed.
- [x] Add an explicit migration/reconciliation preflight for pending/claimed frozen actions and active autonomy payloads; malformed or dual-authority rows abort before cutover.
- [x] Remove the old executor registry, schema helper names, temporary adapters, per-tool prompt fragments, v1 runtime codec, and dead compatibility branches from active production code.
- [x] Keep stable names, IDs, provider request IDs, workflow IDs, target bindings, and idempotency keys unchanged.

### S06. Tests and architecture guards

- [x] Registry: register, lookup, duplicate, nil/invalid, stable ordering, surface catalog.
- [x] Context: required slots, correct loader, missing slot, error, cancellation, snapshot replay, no unrelated eager load.
- [x] Schema: every migrated behavior capability's public schema, forbidden implementation fields, serialized bytes/chars before/after.
- [x] Runtime: invocation → registry → context → preflight → execute → result; taxonomy and per-call continuation behavior.
- [x] Deferred: reply/Moment/image target binding, same transaction, replay/idempotency, and stable no-duplicate external identities.
- [x] Dummy capability: implement and register only; catalog/render/runtime invocation works without MainAgent or schema switch edits.
- [x] Behavior fixtures: ordinary chat no forced call, image intent, scene capability, Moment output, tool-only no-op, and native/sidecar precedence.
- [x] One-cognition fixtures: one Main LLM call per conversation turn, no `role=tool` continuation, planner isolation, and required state-change failure before assistant settlement.
- [x] Architecture guard: production Go code has no concrete capability-name dispatch in MainAgent/Runtime; migration/provider sidecars are the only allow-listed legacy shapes.
- [x] Cancellation/ordering/logging tests cover sequential calls, context propagation, duration/status fields, and bounded redaction.

### S07. Documentation and schema/token report

- [x] Add `docs/capability-architecture.md` with goals, objects, execution flow, Thin rules, description rules, ContextSlot rules, adding a capability, and all ten forbidden practices from the request.
- [x] Record the pre-refactor baseline and add `CapabilityToolSchemaStats` for exact post-change Tool schema bytes/chars. Provider token usage remains unavailable unless the envelope exposes `usage`; no tokenizer was added.
- [x] Update relevant Go Core specs to document the canonical Capability Runtime, strict context boundary, and post-settlement visible delivery.

### S08. Full verification and cutover gate

- [x] Run Core/BFF `go test`, Core `go test -race`, `go vet`, `go build`, and `gofmt -l` with disposable caches. The suites pass outside the restricted sandbox; the sandbox-only listener failures are documented.
- [x] Run disposable PostgreSQL/Redis/Temporal integration: empty and `0025 -> 0026` migration, malformed rollback, bounded snapshot, transaction/idempotency DB tests, platform consumer tests, full cutover/Worker smoke. Live Provider remains unconfigured and is reported separately.
- [x] Run the existing provider/tool focused tests plus new registry/context/runtime/migration tests.
- [x] Re-run `rg` guards for concrete capability-name dispatch and old chain symbols; remaining references are documented compatibility fixtures/codecs.
- [x] Inspect `git diff --stat`, generated artifacts, task files and `master` status. No commit or push was performed.

## Risk and rollback points

| Point | Main risk | Rollback boundary |
|---|---|---|
| S01 | Renaming current contract breaks many tests/callers | Revert foundation-only edits before domain migration; persisted JSON unchanged |
| S02 | Resolver snapshot differs from live authority | Keep old read path behind unstarted branch until snapshot and authorization tests pass |
| S03 | Thin schemas remove semantic decisions or add planner latency | Revisit mapping table; preserve only genuinely model-owned fields, do not add prompt hacks |
| S04 | Frozen replay/active workflow incompatibility | Stop cutover; retain compatible codec/worker and drain or migrate before deleting it |
| S05 | Duplicate media/provider side effect | Verify stable provider/workflow/idempotency IDs and replay before cleanup |
| S06 | Existing behavior regression hidden by unit tests | Re-run baseline fixtures and opt-in integration tests; no success claim from skipped live tests |

## Required validation commands

```bash
GOCACHE=/tmp/lac-cap-runtime-final-cache go -C apps/core-go test ./... -count=1
GOCACHE=/tmp/lac-cap-runtime-race-final-cache go -C apps/core-go test -race ./... -count=1
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
GOCACHE=/tmp/lac-gateway-final-cache go -C apps/gateway-go test ./... -count=1
go -C apps/gateway-go vet ./...
go -C apps/gateway-go build ./...
test -z "$(find apps/core-go apps/gateway-go -name '*.go' -print0 | xargs -0 gofmt -l)"
```
