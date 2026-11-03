# Production caller migration implementation

## Implemented boundary

Production callers now treat the Eino/ADK Tool trace as an audit channel for
already executed and committed Tools. The new adapter is
`apps/core-go/internal/core/agent_result_adapter.go`.

- Conversation turns call `RunMain` or `RunMainStream` according to the real
  transport path. They never execute `completion.ToolCalls`, prepare a second
  capability plan, run a deferred dispatcher, or enforce a visible-output
  failure after the Agent has made its final decision.
- A completed `conversation.reply` result is resolved to its authoritative
  committed `conversation_messages` row. The final `visible_text` is not
  published again. If the Agent naturally finishes without that Tool, the
  output adapter publishes exactly once through `ToolPublicationService`.
- Agent failure after Tool commit stores the trace under the cognition inbox
  partial result and marks the run failed. A retry with the same turn identity
  returns that failure instead of replaying the complete Agent run.
- Wake-up and daily review persist their lifecycle/autonomy audit directly from
  the committed trace. They no longer enqueue a second `capability.action` or
  `autonomy.action` for those ToolCalls.
- Native cognition applies only its final semantic cognition contract and
  records Tool outcomes from the trace. It no longer prepares, plans, or
  settles the calls after `RunNativeCognitionTask`.
- Conversation, Wake-up, and daily-review final schemas no longer duplicate
  native `tool_calls`. The old `response_mode=query_continuation` field was
  removed from the conversation final schema; query/result continuation is
  owned by the Eino loop.
- Conversation prompt policy now permits multiple native Tool rounds, business
  rejection recovery, and same-round multiple calls. It distinguishes
  `accepted` from `completed` and forbids a duplicated final Tool sidecar.

The previous large implementations remain compiled under private `*Legacy`
names during this integration slice so existing low-level helpers and tests can
be migrated independently. No production entry calls them. Final dead-code
deletion should remove `handleTurnLegacy`, `processWakeUpLegacy`,
`processNativeCognitionFactLegacy`, `processDailyReviewLegacy`, the standalone
query-continuation state machine, and caller settlement workers after the
remaining product callers no longer reference their helper symbols.

## Verification

All commands below ran against the working tree after migration. PostgreSQL
commands used the private isolated environment selected by
`/tmp/lac-agent-tool-current.json`; no configuration values were printed.

| Command / evidence | Result |
| --- | --- |
| `go -C apps/core-go test ./internal/core` | PASS |
| `go -C apps/core-go test ./...` | PASS |
| isolated PostgreSQL: `TestHandleTurnProductionADKToolLoopSettlesOneAssistant`, `TestFormalConversationAgentRecallsRandomDatabaseSecretThroughRealToolBridge`, `TestWakeUpConversationReplyCreatesAndDeliversPrivateMessage`, `TestWakeUpNoOpSidecarWithAffectAndReplyStillDeliversPrivateMessage` | PASS; `research/runs/production-callers-controlled.jsonl` |
| isolated PostgreSQL: `TestDirectConversationStreamsCommittedUserBeforeProviderAndAssistantAfterCommit` | PASS; production request uses the registered streaming Runner and emits committed user/message resources without replaying aggregated final text as token evidence; `research/runs/production-callers-stream.jsonl` |
| isolated PostgreSQL: `TestConcurrentDailyReviewUsesOneMainProviderCall` | PASS; `research/runs/production-callers-daily.jsonl` |
| real Provider: `TestFormalAgentE2E` | FAIL; Provider returned HTTP 400 GPU-memory exhaustion for multiple agents (examples: prompt required ~1240–1461 MB while ~404–920 MB was available). This is recorded as a failed real attempt, not a pass; `research/runs/production-callers-formal-live.jsonl` |

## Remaining integration risks

1. `persona.switch` and `persona.takeover` are still internal-only Autonomy
   capabilities in the shared registry. The new conversation caller correctly
   does not parse a final personality field and then execute/apply it again, but
   the formal conversation/takeover Agents need those typed Tools exposed under
   their authorized surfaces to preserve the product behavior without restoring
   caller-side execution.
2. The caller-owned legacy functions are unreachable from production but still
   present. Their focused deletion is required once dependent helper/test
   migrations are complete; leaving them callable under new public entry names
   would reintroduce duplicate execution.
3. The complete real-Provider matrix could not pass while the configured
   Provider lacked GPU memory. Controlled Agent/Tool/PostgreSQL regressions pass,
   but the failed live suite must be rerun after provider capacity is restored.
