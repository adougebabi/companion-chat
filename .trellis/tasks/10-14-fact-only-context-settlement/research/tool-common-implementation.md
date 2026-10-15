# F18 shared Tool availability implementation — stage 1

Implemented directly by the dispatched implementer; no additional agents, commits, push, deployment, real workflow or unknown environment/DB reads.

## Stable API for domain stage

- `handledCapabilityResult(inv CapabilityInvocation, code, disposition string, output map[string]any) CapabilityResult` returns `Status:"handled"`, empty `ErrorCode`, nonretryable, and a copied output with `handled=true`, `effect_applied=false`, `mutation_committed=false`, `warning_code=code`, `disposition=disposition`. Callers return `(result,nil)` only for a genuine no-effect branch. Code and disposition must be nonempty.
- A preparer uses `withHandledCapabilityPreparation(inv, code, disposition, output) (CapabilityInvocation,error)`. It stores a private `core_handled_no_effect` entry in the Core-owned frozen preparation envelope. Core pure/transactional runtime and ExecuteTool Direct path recognize it and skip the capability effect. Arguments cannot set prepared_payload/context_snapshot/authorization envelope fields.
- `capability.NormalizeModelArguments(raw,schema)` is a shared exported representation-only normalizer. The actual native invoker runs it before native schema validation/ref decode/binding. It applies schema defaults, omits optional nonnullable null, preserves nullable null, drops closed-schema extras, normalizes unique string enum case/whitespace, numeric/boolean string representations, and scalar singleton arrays. It does not invent semantic required inputs, IDs, facts, owner or scope.

Domain implementations must keep real successful/accepted/failed effects and all real DB/network/provider/cancellation/Owner/private-session failures. Never relabel an actual committed failed business effect as handled.

## Changes and boundaries

- canonical `capability.go` and Core `capability_core.go`: required means property presence; explicit null is validated by the property schema. Union types dispatch correctly and reject object/array/number values where Actor only permits string/boolean/null. Canonical result validation accepts handled only with explicit no-effect/warning shape; Core runtime/output paths accept the status without applying successful-effect output schemas.
- `capability/support.go`, Core `tool_contract.go`: valid complete argument JSON above 64 KiB continues; `tool.arguments.capacity_warning` logs byte count and advisory threshold only.
- New `capability/argument_normalization.go`, Core `tool_recovery.go`: normalizer and stable no-effect helpers.
- `adk_conversation_runtime.go`: deterministic normalization before native validation/binder; unresolved representations and ref binding failures return traced handled needs_input with bounded schema hints. Original repeated call ID identity remains strict. Malformed JSON receives a valid trace envelope and cannot poison later JSON encoding. Handled output bypasses native business-output rewriting. Explicit surface mismatch and cancellation remain errors. Undisclosed installed execution emits recommendation warning.
- `tool_execution.go`, `tool_execution_ledger.go`: handled is terminal and can be replayed. Domain execution plus linked attempt admission run in a nested pgx transaction/savepoint. Handled rolls back that business savepoint before the technical receipt is committed, and does not publish a Direct linked outcome. Ordinary errors still roll back the transaction; no generic catch-all error conversion.
- `action_outcome.go`, `goal_sources.go`: handled becomes suppressed, never pending, requires no async external ref, and is not Goal evidence. Aggregate handled-only outcomes preserve the no-effect marker. Intention settlement does not complete an attempt for the no-effect marker. A successful sibling or actual failed sibling retains the existing aggregate behavior.
- `kev_tools.go`, `kev_persona.go`: execution checks original authorized installation rather than physical disclosure window. No, config changes and replaced disclosure windows do not remove execution permission. Raw Kev decisions are preserved; No receives warning metadata. Named discovery/catalog deduplicate, retain the installed subset and report unavailable names separately. Requests above 8 are accepted; existing decision service batches by its configured BatchSize. Physical disclosure window behavior remains separate. Missing selection infrastructure is not given fake domain success.
- `apps/web/src/lib/fluctlight-display.ts`: handled label `已处理（告警/待处理）`. No layout change. OpenAPI inspection found no CapabilityResult/ToolExecution closed status enum to regenerate; existing ActionStatus/Goal enums have different meanings.
- Tests: new `tool_availability_common_test.go`, adjusted `native_tool_contract_test.go`, `kev_test.go`, `kev_three_tool_test.go` assertions to distinguish advisory disclosure from actual installation.

## Verification evidence

1. Actual initial RED (`TestToolAvailabilityActorNullableUnion|LargeArguments`): native/Core/canonical rejected real Actor required value:null; canonical accepted an invalid object union member; both normalizers rejected complete 70 KiB JSON.
2. Corresponding GREEN passed after implementation. `TestToolAvailability` additionally verifies null/union negative cases, schema defaults and representation repairs, authority-envelope rejection, No raw audit preservation, revoked-window execution, cancellation/uninstalled negative cases, native handled replay, suppressed/no-Goal-evidence outcomes, and Core preparation marker skips actual Execute.
3. `go test ./internal/core -run 'Test(Kev|NativeArgumentRejection|ToolAvailability)' -count=1` passed with controlled local listeners after updating obsolete assertions. Initial sandbox listener failure was an environment restriction; rerun outside sandbox passed. No real model used.
4. Scoped race command (full expression in `/tmp/tool-common-verification.jsonl`): `go test -race ./internal/capability ./internal/core -run 'Test(ToolAvailability|Kev|NativeArgumentRejection|NativeGoalPhysicalSchemas|NativeActorCurrentInbound|Capability|DirectTool|ToolExecution|BuildActionOutcomes|GoalOutcome|DummyCapability)' -count=1 -json` passed: 89 test/subtest pass events, 3 PostgreSQL test skips. The canonical package currently has no own test files. This includes actual controlled native HTTP/SSE physical schemas/ref repair through the existing tests, rather than just inventory counts.
5. `go vet ./internal/capability ./internal/core` passed. Owned Go files gofmt applied. `git diff --check` passed. Final narrow `go test -race ./internal/core -run TestToolAvailability -count=1 -json` after the malformed-JSON trace regression also passed: 11 test/subtest pass events, no skips.

The three scoped PG skips are `TestNativeActorCurrentInboundSourcePostgres`, `TestCapabilityActionBindsMomentPublishToDurableMomentTarget`, and `TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay` because GO_CORE_TEST_DATABASE_URL was deliberately empty.

## Remaining work / limits for main session

- No full 49-name behavior/recovery manifest was added in stage 1. Inventory must remain distinct from actual domain invocation evidence.
- Native production entry uses pre-schema normalization. The exported canonical normalizer exists, but a caller that bypasses native and invokes canonical Runtime Prepare directly still gets strict input validation; normalization/no-effect recovery at that distinct entry is not yet integrated.
- Core preparer marker is implemented; canonical independent Runtime does not interpret Core's private preparation marker. Both runtimes accept the handled result status.
- No live PostgreSQL ran. The new savepoint/ledger/replay and linked-attempt rollback need isolated PG regression before claiming persistence acceptance. No fake nil-App execution counts as business success.
- The new initial RED is specifically nullable/union and argument capacity. Kev/handled regressions were added/updated after behavior changes and passed; no baseline sensitivity RED was claimed for those tests.
- No final full-repository/frontend typecheck/build/test was run by this stage; main session owns the combined gates and all domain behavior stages.
- Existing downstream ActionOutcome payload byte/count constraints were not changed. A very large result can still meet a separate outcome constraint; raw argument acceptance alone does not certify every business output capacity.
- All business family files, Goal review/schema policy, F17 reply reconciliation, and main specs/reports are untouched by this stage.

## Main-thread closure after the stage-1 handoff

Stage-1 gaps above are historical. Main added canonical fresh-Prepare marker handling and cancellation, shared pre-schema normalization, exact domain error recovery, stale refresh and verified recovery anchors, empirical ActionOutcome capacity advisories, and suppressed settlement for attempts claimed before Tool execution. New task-owned PG/race checks actually ran; 92 focused test/subtest events passed, plus the final integrity cohort. Complete PG runs are not all green: both b24fdd1 and intermediate current snapshots have recorded failures; current-only changed assertions were updated with no-effect/ownership/rollback checks preserved. Other private-chat work concurrently edits shared files; the final owned source snapshot and hunk manifest isolate this task without overwriting it.
