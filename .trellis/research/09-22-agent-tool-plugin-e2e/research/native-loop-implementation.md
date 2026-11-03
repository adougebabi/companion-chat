# Native Eino loop implementation

## Scope and result

Implemented the native-loop slice only:

- `apps/core-go/internal/ai/agent/loop.go`
- `apps/core-go/internal/core/eino_model_runtime.go`
- `apps/core-go/internal/core/eino_adk_runtime_test.go`

The shared runner now passes `MaxIterations` directly to Eino v0.7.37. A
non-positive value uses Eino's native default of 20; positive task-specific
bounds above two are accepted. Max-iteration, cancellation, model and tool
errors remain errors. `ADKLoopResult` retains the last assistant event plus all
unique native ToolCall identities and tool-result events accumulated before an
error.

Removed the Core wrapper's deferred/rejected-result early stop and the
`ToolOnlyTermination` max-limit success path. `queuedToolCallingChatModel`
still acquires a separate queue lease, Provider request identity and diagnostic
sequence for every physical model call, with no model retry configured.

Successful runs retain every actual intermediate ToolCall on the returned
typed Eino message and keep the request-scoped invocation/result trace intact.
The last assistant message remains the final-content authority. Eino callback
and graph-output duplicates are collapsed only by the same formal ToolCall ID;
distinct call IDs from every round remain ordered.

## Controlled regressions

`eino_adk_runtime_test.go` now proves:

- two tool rounds followed by a third model decision;
- a business rejection returned as a tool result is fed back and the model can
  choose a recovery tool;
- same-round multiple ToolCalls keep their IDs paired with their own results;
- streaming uses the same Eino Runner/tool loop;
- max-iteration failure returns an error with partial assistant calls/results;
- existing model-error, tool-error, cancellation and invalid-final tests still
  fail closed;
- the Core Eino HTTP boundary performs three physical requests after two
  deferred results and retains both intermediate call identities.

## Verification

All commands ran from repository HEAD plus the current shared working tree.

| Command | Exit | Result |
| --- | ---: | --- |
| `GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test ./internal/ai/agent ./internal/core` | 0 | Both owning packages passed, including the new native-loop regressions. |
| `GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go vet ./internal/ai/agent ./internal/core` | 0 | No vet findings. |
| `git diff --check -- apps/core-go/internal/ai/agent/loop.go apps/core-go/internal/core/eino_model_runtime.go apps/core-go/internal/core/eino_adk_runtime_test.go` | 0 | No whitespace errors. |
| `rg -n "ToolOnlyTermination|stopOnDeferredToolRound|requireVisibleTextForDeferredStop|deferredToolOnlyRoundMessage|adk_iteration_limit_invalid" <owned files>` | 0, zero matches | Removed legacy termination mechanisms from the owned production and test slice. |

## Remaining integration considerations

- This slice deliberately does not change the Core capability executor. Some
  current business capabilities still return `deferred` until their owning
  tool-runtime slice makes them independently executable; the native Agent
  loop now feeds those results back instead of ending locally.
- This slice does not change schema allowlists, model-task callers or outer
  settlement. Those are owned by other implementation slices.
- No live Provider or database E2E was run for this isolated runtime change.
  The controlled HTTP Provider regression exercises the real locked Eino
  adapter and native Runner without external credentials.
