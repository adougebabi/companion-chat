# Persona / working-profile DB regression migration

Status: partial implementation, controlled HTTP Provider + isolated PostgreSQL only. All commands explicitly unset `FLUCTLIGHT_LIVE_PROVIDER_*`; no live model request was made.

## Contract migration

| Retired assertion | Current evidence |
|---|---|
| Candidate A, Judge verdict and candidate B are separate caller-owned stages | `TestNativePersonaTakeoverFeedsScopedToolsReplyAndFinal` drives one formal conversation Agent through native `persona.takeover` → real `relationship.lookup` + `memory.recall` → committed `conversation.reply` → legal final DTO. The next physical model request contains the committed Tool receipt. |
| A takeover reply requires a second special Main generation | The generic Eino loop changes `WorkingProfileID` from the committed takeover receipt. Subsequent query and reply results have `ActingProfileID=twilight`; no Tool-name branch or takeover-reply generation is used. |
| Main generation is capped at two and Judge has a separate fixed budget | `TestNativePersonaLoopUsesGenericIterationGuardInsteadOfLegacyStageBudget` completes four Tool rounds and a fifth final decision. Only the generic Runner iteration guard remains relevant. |
| Frozen A/Judge/B stages are the recovery unit | `TestInvalidFinalKeepsCommittedPersonaReceiptAndFailedRunDoesNotReplay`, `TestCompletedNativePersonaTurnReplaysCommittedAssistantWithoutProvider`, and `TestCommittedReplySurvivesInvalidFinalAndFailedRunDoesNotReplay` use committed Tool receipts / `agent_runs`: committed effects survive later invalid final output, completed runs replay without Provider I/O, and failed runs do not restart the model loop. |
| Takeover changes the durable dominant profile | `TestNativePersonaTakeoverFeedsScopedToolsReplyAndFinal` and `TestNextTurnAfterNativeTakeoverRestoresPersistentProfile` prove the durable runtime remains `spark`, the takeover reply settles Relationship as `twilight`, and the next turn reads/replies as `spark`. |
| Persistent switch is applied later by caller settlement | `TestNativePersistentSwitchCommitsOnceAndUpdatesSubsequentToolProfile` and `TestAuthorizedPersistentSwitchLoadsTheNewPersonaNextTurn` execute native `persona.switch`, assert exactly one `persona.switch.committed` audit, use `twilight` for later Tools, and expose `twilight` in the next turn prompt. |
| Rejection may degrade to an older candidate or widen B scope | `TestRejectedNativePersonaTakeoverCannotEscalateWorkingProfile`, `TestPersonaTakeoverToolRejectsUndeclaredRuleTargetAndStaleSource`, and `TestExecuteToolRejectsUndeclaredWorkingProfileBeforeCapability` retain the rejection receipt, keep the persistent/source profile, and reject undeclared rule, target, source, and working profile identities. |
| Profile scope is inferred only from persistent runtime | `TestToolProfileContextResolverScopesRealRelationshipAndMemoryReads` executes the formal Tool boundary against real DB rows for both profiles. Relationship returns the profile-owned row and shadows shared/foreign rows; Memory reads carry the declared acting profile and return real memory data. |
| A visible response requires a synthetic reply invocation | `TestNaturalFinalReplyNeedsNoSyntheticConversationReplyTool` proves a legal natural final publishes once with no fabricated Tool result. |

## Files changed

- `apps/core-go/internal/core/turn_takeover_chain_test.go`
- `apps/core-go/internal/core/turn_chain_budget_test.go`
- `apps/core-go/internal/core/takeover_scope_matrix_test.go`
- `apps/core-go/internal/core/persistent_switch_assessment_test.go`
- `apps/core-go/internal/core/phase11_verification_test.go`
- `apps/core-go/internal/core/turn_takeover_f01_test.go`
- `apps/core-go/internal/core/turn_takeover_f02_test.go`
- `apps/core-go/internal/core/turn_takeover_f04_test.go`
- `apps/core-go/internal/core/turn_takeover_recovery_test.go`

No production compatibility branch was added. The shared `takeoverChain*` helpers retained for still-unmigrated external tests emit the current strict final DTO and never place `response_mode` or sidecar `tool_calls` in structured output.

## Verification

Environment source: `/tmp/lac-agent-tool-current.json` → isolated `test-env.json`; live Provider variables removed before `go test`.

Passed in the combined isolated-PostgreSQL run before two string-escaping assertions were corrected:

- `TestNativePersistentSwitchCommitsOnceAndUpdatesSubsequentToolProfile`
- `TestPersonaToolsAreAvailableToAgentsAndCommitThroughDomainService`
- `TestPersonaPolicyInvocationUsesStablePolicyIdentity`
- `TestPostgresDirectPersonaToolsCommitRejectReplayConflictAndAudit`
- `TestAuthorizedPersistentSwitchLoadsTheNewPersonaNextTurn`
- `TestToolProfileContextResolverScopesRealRelationshipAndMemoryReads` (`spark`, `twilight`)
- `TestRelationshipInteractionSettlementFollowsNativeReplyActingProfile`
- `TestNextTurnAfterNativeTakeoverRestoresPersistentProfile`
- `TestNativePersonaLoopUsesGenericIterationGuardInsteadOfLegacyStageBudget`
- `TestInvalidFinalKeepsCommittedPersonaReceiptAndFailedRunDoesNotReplay`
- `TestNaturalFinalReplyNeedsNoSyntheticConversationReplyTool`
- `TestPersonaTakeoverToolRejectsUndeclaredRuleTargetAndStaleSource` (all three rejection cases)
- `TestExecuteToolRejectsUndeclaredWorkingProfileBeforeCapability`
- `TestDeclaredPersonaTakeoverReturnsAuthoritativeWorkingPersonaWithoutPersistentWrite`

After correcting the assertions, a focused rerun passed:

- `TestNativePersonaTakeoverFeedsScopedToolsReplyAndFinal`
- `TestRejectedNativePersonaTakeoverCannotEscalateWorkingProfile`

Compilation gate `go test ./internal/core -run '^$'` passed after restoring the shared helper symbols. A later concurrent edit outside this slice introduced `capability_core_test.go:886:13: undefined: productionSourceText`; final package-wide compilation/testing must be repeated after that owner finishes.

## Remaining follow-ups

- The newly rewritten `turn_takeover_recovery_test.go` tests were added after the last successful package compile and have not yet been run because the concurrent `productionSourceText` compile error blocks all Core tests.
- `turn_takeover_f05_test.go` still contains one production-turn A/Judge overlay fallback fixture. Its pure control-view tests remain useful, but `TestOverlayFailureKeepsAByExplicitContractInProductionTurn` still needs migration/removal.
- `turn_takeover_test.go` still contains unit coverage for canonical visible-text/legacy candidate normalization. This slice did not delete the production helpers they exercise because the shared tree still has consumers; they need a coordinated retirement pass.
- `turn_path_cost_report_test.go` still reports the retired A/Judge/B stage costs. It was outside the explicitly assigned file list and remains for the main integration pass to rewrite around native model decisions and committed receipts.
- Package vet and the complete isolated DB group remain for the main final pass after concurrent compile errors settle.
