# Retired path cleanup

Date: 2026-09-22

## Result

The caller-owned model continuation and capability settlement paths have been
physically removed. Production conversation, Wake-up, native cognition, and
daily-review callers now have one implementation in
`agent_result_adapter.go`; native ToolCalls execute and commit inside the
formal Agent loop through `ExecuteTool`.

No compatibility switch, alternate query-result synthesis call, or
Prepare/plan/settle dispatcher remains in production Go code. Existing
Temporal action activities remain registered as durable recovery consumers,
but they now invoke the same standalone Tool boundary rather than re-running
the retired capability runtime.

No schema migration was added, no persisted business row was deleted, and no
historical data was rewritten.

## Mechanisms physically deleted

### Duplicate production callers

The following unreachable implementations were removed in full:

- `mutations.go: handleTurnLegacy`
- `wakeup.go: processWakeUpLegacy`
- `cognition_growth.go: processNativeCognitionFactLegacy`
- `autonomy.go: processDailyReviewLegacy`

Helpers used only by those implementations were also removed, including the
old frozen-turn assistant recovery settlement, Wake-up action/deferred
dispatcher persistence, Wake-up capability-result redispatch, and autonomy
assistant publication helpers.

### Query continuation protocol

`query_continuation.go` and its implementation-detail test file were deleted.
The following dedicated entry points and protocol branches were removed:

- `ConversationRuntime.RunQueryContinuation` and
  `QueryContinuationInput`
- `queryContinuationResponseSchema`
- `ProviderClient.StructuredQueryContinuation`
- the Provider `continuation` mode parameter, message validator, and
  continuation diagnostics branch
- `query_continuation_response` formal-schema routing
- turn-decision continuation state, continuation base messages, takeover
  continuation exclusion/budget branches, and the query-specific persona
  switch grant
- Core and browser HTTP allowlists for retired continuation error codes

The production code search now has zero matches for
`query_continuation`, `QueryContinuation`, or
`StructuredQueryContinuation`.

Read Tools such as `memory.recall` and `relationship.lookup` remain normal
formal Tools. Their actual result is returned to the Eino Runner as a native
ToolResult, and the same Agent continues according to the framework loop. No
caller builds a second `role=tool` request.

### Caller-owned capability execution

`capability_runtime.go` no longer contains:

- `ExecuteCapabilities`
- `prepareCapabilityInvocations`
- `planCapabilitiesForTransaction`
- `executeCapabilities`
- `settleDeferredCapabilitiesTx`
- settlement rollback conversion and required-output gates used only by that
  dispatcher

The remaining file contains registry/catalog lookup, deterministic candidate
validation, invocation metadata normalization, result decoding, and helpers
still consumed by formal Tool/Agent contracts. It no longer executes a batch
on behalf of an outer caller.

### Durable action workflow dispatcher

`ProcessAutonomyAction` and `ProcessCapabilityAction` remain because the
Temporal worker, cognition service, reconciliation, cancellation, and terminal
failure paths still call them for persisted `autonomy.action` and
`capability.action` intents.

Their implementation is now a thin recovery consumer:

1. Read and validate the frozen action, terminal state, policy snapshot, and
   expected authority revisions.
2. Materialize a missing historical publication command as the corresponding
   formal `conversation.reply` or `moment.publish` Tool invocation.
3. Invoke each command through `App.ExecuteTool` with a stable business
   operation ID and the authorized Owner/Fluctlight/resource scope.
4. Reuse terminal results already persisted on the action and rely on the
   `tool_executions` receipt ledger if a Tool committed before the action
   trace was updated.
5. Persist only the action trace, ActionOutcome, reflection intent, and
   completion/failure outbox after Tool execution.

The consumer never calls the removed Prepare/plan/settle functions, never
opens a caller transaction around multiple Tool effects, and never replays a
completed Agent trace.

## Product contracts retained

- **Actual Tool effects:** publications use `ToolPublicationService`;
  transactional mutations and async intents use the same implementations as
  direct and Eino-adapted Tool calls.
- **Short transactions and outbox:** each Tool owns its local mutation,
  receipt, and outbox transaction. Action audit settlement is a separate short
  transaction after the Tool result exists.
- **Committed facts survive later failure:** a successful sibling Tool is not
  rolled back when a later required Tool fails. The action becomes failed and
  records all per-call results while the committed sibling remains visible.
- **Idempotency:** action recovery derives stable operation IDs from the
  frozen action/call identity. Re-entry first observes the terminal action
  state; a crash after Tool commit is recovered through the receipt ledger.
- **Publication and async status:** completed replies/Moments return their
  committed target; accepted asynchronous Tools remain accepted/pending in
  ActionOutcome rather than being relabeled as synchronously completed.
- **Authorization and policy:** the Owner/Fluctlight relationship, action
  policy, expected authority revisions, Tool argument schema, target
  ownership, and Tool-specific CAS checks still fail closed.
- **Cancellation/recovery/audit:** terminal and
  `cancel_requested` action states are not executed. Temporal activity
  registration, retry reconciliation, `FailAutonomyAction`, Wake-up result
  linkage, ActionOutcome, reflection intent, lifecycle diagnostics, and
  outbox records remain.
- **Historical data:** active historical action rows can recover through the
  new Tool boundary. Malformed pre-existing envelopes fail closed and remain
  auditable. Completed audit rows are read, not re-executed.

## Regression changes

Tests that asserted the retired ordering
`Prepare -> freeze -> plan -> caller transaction settle`, the dedicated
query-continuation message layout, or continuation/takeover mutual exclusion
were removed or rewritten.

Replacement evidence checks product behavior:

- a Tool mutation commits once even when a later required sibling fails;
- the failed action and all three ActionOutcome rows remain auditable;
- replaying the terminal action does not duplicate the committed mutation;
- a recovered `moment.publish` action creates the real Moment and a completed
  per-call ActionOutcome;
- an architecture guard requires physical absence of every retired symbol/file
  and requires the workflow recovery consumer to call `ExecuteTool`;
- accepted asynchronous Tool results are valid persisted results and map to a
  pending ActionOutcome.

## Verification

All commands ran without `FLUCTLIGHT_LIVE_PROVIDER_TEST`; no live LLM request
was made by this cleanup slice.

| Check | Result |
| --- | --- |
| `gofmt -w internal/core internal/httpapi internal/personality` | PASS |
| `git diff --check` | PASS |
| `GOCACHE=/tmp/lac-retired-go-cache go test ./...` from `apps/core-go` | PASS |
| `GOCACHE=/tmp/lac-retired-go-cache go vet ./...` from `apps/core-go` | PASS |
| isolated PostgreSQL: `TestAutonomyCapabilityActionKeepsCommittedSiblingAndFailsRequiredCommand` | PASS |
| isolated PostgreSQL: `TestCapabilityActionBindsMomentPublishToDurableMomentTarget` | PASS |

Database evidence:

- `research/runs/retired-workflow-db-01.jsonl`
- `research/runs/retired-workflow-db-01.meta.json`

The database cases used the task's disposable PostgreSQL configuration. The
live-provider flag was explicitly removed, and no credential value was
printed.

## Remaining real call points

| Product boundary | Current implementation |
| --- | --- |
| Conversation / actor turn / streaming turn | `mutations.go` public wrappers -> `agent_result_adapter.go:handleTurn` -> `ConversationRuntime.RunMain` or `RunMainStream` |
| Wake-up | `agent_result_adapter.go:ProcessWakeUp` -> registered Wake-up formal Agent |
| Native cognition | `agent_result_adapter.go:ProcessNativeCognitionFact` -> `RunNativeCognitionTask` formal Agent |
| Daily review | `agent_result_adapter.go:ProcessDailyReview` -> `RunDailyReviewTask` formal Agent |
| Persisted autonomy/capability action recovery | Temporal activity -> cognition service -> `workflow_ops.go:ProcessAutonomyAction` / `ProcessCapabilityAction` -> `ExecuteTool` |
| Tool mutation/publication | `tool_execution.go:ExecuteTool` -> one capability implementation + receipt ledger |

The formal takeover Judge/reply Agents remain registered for their product
task definitions, but they no longer contain or call a query-continuation
protocol.

## Remaining verification outside this slice

The final real-Provider acceptance matrix is intentionally still pending and
must not be reported as passed. This cleanup introduced no live LLM call. The
user-authorized final real LLM acceptance can run after the complete integrated
implementation is ready.
