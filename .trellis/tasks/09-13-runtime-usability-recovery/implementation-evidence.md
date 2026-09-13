# Runtime usability recovery — implementation evidence

Status: in progress. S01-S10 are complete; S11 implementation and exact
verification are in progress.

## Planning baseline

- Task created with user consent on 2026-09-13.
- Current planning HEAD at last inspection: 627cd7d on master, matching
  origin/master.
- Protected unrelated path:
  .trellis/tasks/09-08-project-health-evolution/.
- Historical session evidence confirms recurring WakeUp liveness, silent
  diagnostics and initialization-fidelity failures across multiple sessions.
- Product decisions:
  - natural-language source may produce directly supported structured/numeric
    derivations while preserving original meaning;
  - repository uses anonymized fixtures and local Live Provider accepts the
    Owner card only by external path;
  - key lifecycle success/no-op/overdue transitions are persisted and shown;
  - WakeUp is fixed periodic; Reflection is interaction quiet-period;
  - visible private message is an optional WakeUp capability;
  - Schedule enriches WakeUp but does not gate liveness;
  - multi-personality requires explicit distinct-profile/switch evidence;
  - original character card is immutable Owner-only Foundation source and is
    excluded from ordinary prompts/logs/diagnostics.

## Stage evidence

### S01 — Freeze evidence and red-capable contracts

Status: complete; expected red baseline captured. Production files were read
only.

Baseline:

- branch: master
- HEAD/origin-master:
  627cd7db0d1b18e0da14aacea3292b1dcaacbacc
- unrelated protected path remained untracked and untouched:
  .trellis/tasks/09-08-project-health-evolution/

Production SHA-256:

- wakeup.go:
  af64ab8ef05ba7809974a36e919dacbcc219d711c41d2b93a743f452b02e55b8
- redis_triggers.go:
  8a56af0e06cd8496fd8bac48b892e4a837d41ba5e9eb3d9b41b1cee8a8f70552
- diagnostics.go:
  cc613b2388ac467bcde6770320b80a6a89ebe6b5a944c4968272ca167cf87b0e
- app.go:
  28661683b8b9294b1b9b00c20cf8aef85cd804995c16a25afe72eb13eb8ee250
- provider.go:
  5af02785aae29791d2b380864556741921ff30960826aded62464a2df70a1d49
- provider_schemas.go:
  2d94606ae95f7bd0ebefaee991ed5b22cf6e54b47e89d3d39f0e83c17308e188
- cognition.go:
  776a1536343cb589a84a5657a3bb7a0978cbd254803ce6eca54500579efe4531
- mutations.go:
  37730fb4f013144bf5d204f652b0cc7e284b2524a2fa260a70e48578f5e2fd04
- workflow_ops.go:
  ee22c1ea334454256af3b0e6d094bc275c0b4cd7444c511e260f9a41c648172c
- workflow/workflow.go:
  cec0f913b6bec3194bf481e59dbb468fe8e460aa1d2be6e946d29dc1056ab29b

Exact Core command:

    go test ./internal/core -run '^(TestEnsureWakeUpIntentsDoesNotRequireAcceptedSchedule|TestConversationDoesNotPostponeWakeUp|TestWakeUpHasPostgresDueSweepAndOverdueDiagnostic|TestInitializationStructuredFallbackCannotReplaceNonEmptyCharacterCard|TestInitializationModeMatchesDeclaredProfileCardinality|TestInitializationPreservesDenseCharacterCardSemanticOwners)$' -count=1 -v

Expected red results:

- TestEnsureWakeUpIntentsDoesNotRequireAcceptedSchedule:
  WakeUp still depends on accepted life_schedules.
- TestConversationDoesNotPostponeWakeUp:
  cognition.go/mutations.go still reset the fixed WakeUp cadence.
- TestWakeUpHasPostgresDueSweepAndOverdueDiagnostic:
  completed WakeUp has no PostgreSQL due release/overdue producer.
- TestInitializationStructuredFallbackCannotReplaceNonEmptyCharacterCard:
  empty StructuredFallback is accepted as a neutral default Persona.
- TestInitializationModeMatchesDeclaredProfileCardinality:
  multiple with one profile is accepted.
- TestInitializationPreservesDenseCharacterCardSemanticOwners:
  nickname/height/blood type/birthplace/shared media/core relationship/core
  conflict are moved to root initialization extensions rather than retained by
  runtime Persona owners.

The first Core command attempt used an incorrect repeated
apps/core-go/internal/core path while already running under apps/core-go; it
stopped at rg and produced no test verdict. The corrected command above ran all
six exact tests and all six failed for the intended reasons.

Exact Workflow command:

    go test ./internal/workflow -run '^TestWakeUpTerminalFailureRemainsContinuouslyRecoverable$' -count=1 -v

Expected red result:

- failed wake_up.current intent is absent from continuous reconciliation and
  can remain terminal until an unrelated Worker restart.

Formatting/evidence:

- gofmt ran only on the three modified S01 test files.
- git diff --check passed.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

### S08 — Lifecycle Diagnostics API, BFF and browser

Status: complete.

Implemented:

- Core exposes an Owner-only Lifecycle Diagnostics page filtered by
  Fluctlight/correlation/intent/workflow/run/surface/status and selects the
  newest bounded event window in chronological timeline order.
- the same response includes a bounded PostgreSQL
  `platform_workflow_intents` snapshot with status, attempts, next due,
  correlation/causation and safe last error. It never calls Temporal.
- filtered export reuses the same Core filter and includes lifecycle events and
  workflow-intent snapshots alongside the existing events/model/media modules.
- Core query failures return bounded stage/correlation details; BFF preserves
  safe Core code/details even for server failures instead of replacing them
  with a fixed runtime code.
- BFF performs explicit snake_case↔camelCase transport mapping and forwards
  every supported filter to both lifecycle query and export.
- browser OpenAPI and typed client are generated reproducibly from the approved
  generator sources; generation before/after hashes were identical.
- Pinia keeps independent source epochs for lifecycle/events/model runs/media/
  workflows and clears rows when a source changes filter identity, preventing a
  failed partial refresh from mixing correlations.
- the Diagnostics Center adds a Lifecycle section with typed filters, explicit
  empty/overdue/no-op/retry/failure presentation, stage/reason/attempt/next due,
  PostgreSQL intent snapshots, Workflow/Model Run links and narrow-screen
  wrapping. Existing sections remain present.

Final exact Core command:

    GOCACHE=/tmp/lac-runtime-recovery-s08-core-final go -C apps/core-go test ./internal/core -run '^(TestLifecycleDiagnosticsFilterBuildsAllPostgresPredicates|TestLifecycleDiagnosticsRejectsUnsafeFilterValues|TestLifecycleDiagnosticsExportUsesSameFilterAndPostgresSnapshot)$' -count=1 -v

Result: PASS (three exact tests).

Final exact Core HTTP command:

    GOCACHE=/tmp/lac-runtime-recovery-s08-http-final go -C apps/core-go test ./internal/httpapi -run '^(TestLifecycleDiagnosticsFilterMapsAllQueryFields|TestLifecycleDiagnosticsExportKeepsItsLargerBoundedLimit|TestDiagnosticsQueryFailureDetailsKeepOnlyBoundedCorrelation|TestLifecycleDiagnosticsRouteAndExportUseOneFilterContract)$' -count=1 -v

Result: PASS (four exact tests).

Final exact BFF command:

    GOCACHE=/tmp/lac-runtime-recovery-s08-bff-final go -C apps/gateway-go test ./internal/bff -run '^(TestBFFLifecycleDiagnosticsMapsFiltersAndPreservesPostgresSnapshot|TestBFFDiagnosticsPreservesSafeCoreServerError|TestBFFDiagnosticsExportForwardsLifecycleFilters)$' -count=1 -v

Result: PASS (three exact tests).

Final browser-client command:

    pnpm exec tsx --test --test-name-pattern='BrowserClient (preserves structured diagnostics details|serializes every lifecycle diagnostics filter for query and export)' test/client.test.ts

Result: PASS (two exact tests).

Final Web commands:

    node --test --test-name-pattern='Lifecycle Diagnostics|diagnostics sources|filtered export' test/diagnostics.test.mjs
    node --test --test-name-pattern='diagnostics keeps existing modules' test/layout.test.mjs

Result: PASS (five exact tests).

Checks:

- approved allowlist corrections covered the real HTTP handler, generated
  browser sources and conflicting layout test; no unapproved file was edited.
- targeted browser generation is reproducible.
- S08 Go files are gofmt-clean and `git diff --check` passes.
- HEAD and origin/master both remain
  `627cd7db0d1b18e0da14aacea3292b1dcaacbacc`.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

### S09 — Immutable initialization source and schema ownership

Status: complete.

Implemented:

- migration head `0033_initialization_source` upgrades from
  `0032_prompt_context_memory` without rewriting any released migration SQL.
- immutable `fluctlight_initialization_sources` stores Owner, exact source and
  digest, correlation, Provider/model, prompt/schema versions, classification
  evidence, derivation/coverage objects, structured projection and projection
  digest. A database trigger rejects UPDATE/DELETE.
- immutable `fluctlight_initialization_source_links` separately binds one source
  to one Fluctlight/Foundation revision; separating the link avoids mutating the
  original source row. Its own trigger rejects UPDATE/DELETE.
- successful analysis writes the source without logging it. Correlation replay
  verifies source/projection digests instead of updating the immutable row.
- activation freezes the projection digest before assigning the real
  `identity.id`, then links the latest matching unlinked Owner source inside the
  same transaction as Foundation revision creation.
- Owner-only Fluctlight detail exposes the dedicated source view. Existing
  Fluctlights without a link return a missing/null source rather than failing.
- canonical owners now include nickname, height/height_cm, blood type,
  birthplace, long background story, appearance description/features/daily
  outfits/style, media preferences, personality core relationship/conflict and
  forced activation metadata.
- initial social Relationships accept only `actor_user`; profile-to-profile
  relations remain inside personality_system.
- root semantic extensions are merged into Core Persona and survive ordinary
  Persona projection, while source/projection/provider/correlation bookkeeping
  is excluded. The source table is never read by ContextProjection or ordinary
  Prompt composition.

Exact Core command:

    GOCACHE=/tmp/lac-runtime-recovery-s09-core-final2 go -C apps/core-go test ./internal/core -run '^(TestInitializationCanonicalOwnersIncludeDenseCharacterFields|TestInitializationSourceIsDedicatedAndExcludedFromOrdinaryPrompt|TestRuntimePersonaKeepsSemanticExtensionsWithoutSourceBookkeeping|TestInitializationRelationshipsAcceptOnlyActorUser|TestInitializationPreservesDenseCharacterCardSemanticOwners|TestNormalizeInitializationResponseMovesUnknownFieldsToExtensions|TestNormalizeInitializationProfilesKeepsKnownFieldsAndMovesUnknowns|TestPrepareInitializationResponseNormalizesSafeContainersBeforeValidation)$' -count=1 -v

Result: PASS (eight exact tests).

Exact migration command:

    GOCACHE=/tmp/lac-runtime-recovery-s09-migrations-final2 go -C apps/core-go test ./internal/migrations -run '^(TestPersonalityGrowthSchemaIncludesTypedSlotsAndCapabilityRequests|TestInitializationSourceMigrationIsAppendOnlyAndOwnerScoped|TestPromptContextMemoryMigrationAddsRawHistorySourceContract|TestMigrationBridgeAcceptsOnlyReleasedHead)$' -count=1 -v

Result: PASS (four exact tests).

Deferred S12 PostgreSQL test authored but not run:

- `TestPostgresInitializationSourceMigrationFromPreviousHead` applies the
  previous head to an explicitly supplied disposable database and verifies the
  new head/source/link tables.

Expected red contract retained for S10:

- `TestInitializationModeMatchesDeclaredProfileCardinality` still fails until
  S10 installs the complete single/multiple classification contract. It was not
  changed opportunistically during S09.

Checks:

- gofmt ran only on S09 Go files and `git diff --check` passed.
- no source text appears in the new operational logs, ordinary ContextProjection
  or Prompt composer. S10 separately owns metadata-only initialization model-run
  diagnostics at the Provider boundary.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

### S10 — Rich initialization prompt and semantic coverage

Status: complete.

Implemented:

- initialization remains `response_format.type=json_object`; no full strict
  constrained decoder was restored.
- the Prompt now provides exhaustive semantic vocabulary, source-to-owner and
  no-invention rules, complex-single versus explicit-multiple classification,
  supported prose-derived values and a complete canonical JSON skeleton.
- semantic-empty/truncated/unparseable structured fallback cannot normalize to
  a neutral Persona. It returns correlated, retryable
  `initialization_response_semantic_empty` with
  `validation_type=semantic_empty,path=provider_response`.
- `single` accepts at most one declared profile; `multiple` requires at least
  two. Situational contrast remains a complex single personality.
- initialization Provider model-run Prompt/Response are metadata-only digests,
  sizes and token estimates; source card, structured response and reasoning are
  excluded.
- Core owns one explicit `InitializationDescriptionMaxBytes=60000` byte limit;
  S11 aligns BFF/Web consumers to that constant contract.
- anonymized dense single/multi cards and semantic manifests cover identity,
  appearance/outfit, background, actor_user relationship, daily habits,
  per-profile speech/behavior, forced activation, influence/integration,
  media frequency and core conflict.
- semantic evaluator reports `preserved`, `default_only`, `missing`,
  `moved_to_extension`, `contradicted` and `invented` per semantic assertion.
- opt-in real Provider tests exist for dense single, dense multi and an external
  Owner card. External files must be outside the repository; private HTTP/
  parsing failures and coverage summaries never print source, expectations or
  full Provider response.

Exact deterministic command:

    GOCACHE=/tmp/lac-runtime-recovery-s10-final2 go -C apps/core-go test ./internal/core -run '^(TestInitializationRejectsSemanticEmptyProviderFallback|TestInitializationPromptRestoresCompleteSemanticVocabulary|TestInitializationModelRunDiagnosticsAreMetadataOnly|TestInitializationDescriptionLimitIsByteBased|TestInitializationModeMatchesDeclaredProfileCardinality|TestInitializationCoverageEvaluatorUsesSemanticCategories|TestDenseInitializationFixturesCoverSingleAndMultipleModules|TestInitializationCoverageSummaryDoesNotLeakSourceOrExpectations|TestPrivateInitializationLiveHarnessNeverPrintsSourceOrResponse|TestInitializationPreservesDenseCharacterCardSemanticOwners|TestRuntimePersonaKeepsSemanticExtensionsWithoutSourceBookkeeping)$' -count=1 -v

Result: PASS (eleven exact tests).

Live Provider tests authored but not run before S12:

- `TestLiveProviderDenseSingleInitialization`
- `TestLiveProviderDenseMultiInitialization`
- `TestLiveProviderDenseExternalInitialization`

Checks:

- S10 files are gofmt-clean and `git diff --check` passed.
- Live Provider, full/race/vet/build, PostgreSQL, Temporal and Docker validation
  did not run.

### S07 — Instrument active lifecycle boundaries

Status: complete.

Implemented within the previously approved S07 files:

- Dispatcher hydrates stable correlation/causation before Temporal start;
  WakeUp uses `wake_up:<fluctlight_id>:cycle:<cycle>` and legacy Reflection
  payloads fall back to their durable intent identity.
- queued, dispatched, already-running, retry-scheduled, Activity-started,
  Provider-queued, completed-noop, completed-actionable and failed lifecycle
  boundaries now carry the intent/workflow/run/Activity identity available at
  that stage. AlreadyStarted errors retain their Temporal Run ID.
- WakeUp and Reflection propagate their root correlation into Provider calls;
  Activity attempt identity is propagated into Provider diagnostics.
- every top-level Provider invocation owns one explicit attempt identity. The
  model-run digest includes that attempt, so different retries no longer
  overwrite one row, while queued/running/terminal writes within one attempt
  stay on one identity.
- model-run status writes are monotonic: terminal rows cannot regress to queued
  and running cannot regress to queued on replay.
- Provider assignment/message/wire-budget/payload preflight failures now emit
  bounded metadata-only diagnostics before the queue boundary.
- Dispatcher/Reconcile authoritative writes no longer use ignored Exec errors;
  CAS misses do not report progress. Dependency lookup failures for cognition,
  WakeUp, actions, Reflection and Visual Identity remain eligible and emit a
  sampled lifecycle failure instead of falling through to terminal settlement.
- Temporal Describe/history empty/failure paths, post-start ledger settlement,
  media terminal settlement, WakeUp suppression/no-op and Reflection no-evidence
  paths now retain explicit stage/reason/correlation.
- User approved adding `visual_identity.go` and `visual_identity_test.go` to
  S07. Renderer-pending, max-attempts, candidate-asset and timeline settlement
  failures now return Activity errors instead of a false waiting success.
- WakeUp action/Reflection derivatives retain the original cycle correlation;
  source fact/action IDs remain causation rather than replacing the trace root.
- queued actions remain queued in lifecycle diagnostics until the action
  Activity commits the domain result; Reconcile no longer guesses that every
  completed Workflow was a no-op, and cancellation is explicit.
- Provider preflight model-run prompts are metadata-only. Lifecycle metadata
  handles typed maps/structs/binary/cycles safely, Provider attempts are
  first-class event identity, and aggregation preserves first/latest cause.
- multi-dispatcher CAS loss re-reads the authoritative intent status and treats
  an already-settled winner as convergence rather than a batch failure.

Initial narrow compile command exposed and then resolved the missing standard
library `errors` import:

    GOCACHE=/tmp/lac-runtime-recovery-s07-workflow go -C apps/core-go test ./internal/workflow -run '^TestWakeUpTerminalFailureRemainsContinuouslyRecoverable$' -count=1

Final exact new/changed Workflow contracts:

    GOCACHE=/tmp/lac-runtime-recovery-s07-workflow go -C apps/core-go test ./internal/workflow -run '^(TestWakeUpTerminalFailureRemainsContinuouslyRecoverable|TestLifecycleCorrelationForIntentUsesStableWakeUpCycleIdentity|TestDispatcherHydratesCorrelationAndCausationBeforeWorkflowStart|TestActivityLifecycleDiagnosticIncludesTemporalIdentityAndAttempt|TestActivityLifecycleDistinguishesNoopAndActionableOutcomes|TestAlreadyRunningTransitionRetainsTemporalRunIdentity|TestWorkflowCriticalPersistenceWritesAreNeverIgnored)$' -count=1

Result: PASS (seven exact tests).

Final exact new/changed Core diagnostic contracts:

    GOCACHE=/tmp/lac-runtime-recovery-s07-core go -C apps/core-go test ./internal/core -run '^(TestModelRunPersistenceDoesNotReturnGhostID|TestProviderAttemptIdentityIsStableWithinInvocationAndDistinctAcrossRetries|TestProviderModelRunIdentitySeparatesAttempts|TestProviderRequestIdentityStaysStableAcrossDiagnosticAttempts|TestProviderPreflightFailuresAreDiagnosedBeforeQueue|TestProviderModelRunLifecycleCannotRegressFromTerminalToQueued|TestWakeUpAndReflectionProviderCallsUseLifecycleCorrelation)$' -count=1

Result: PASS (seven exact tests).

Exact directly affected regression contracts:

    GOCACHE=/tmp/lac-runtime-recovery-s07-workflow go -C apps/core-go test ./internal/workflow -run '^(TestWorkflowIDReusePolicyAllowsWakeUpRecovery|TestVisualIdentityStartOptionsAllowFailedRecoveryAndExposeDuplicateStart|TestWakeUpRetryBackoffIsNotRequeuedBeforeDueTime|TestReflectionTerminalFailureRemainsContinuouslyRecoverable|TestReflectionRetryRequiresLiveFluctlightEvidenceAndBudget|TestActionIntentRetryOnlyWhenActionRemainsExecutable|TestDispatcherPrioritizesLifecycleRecoveryBeforeVisualIdentityRetries|TestDispatcherFairSelectionRanksEachIntentClassBeforeBacklog)$' -count=1

    GOCACHE=/tmp/lac-runtime-recovery-s07-core go -C apps/core-go test ./internal/core -run '^(TestPromptDiagnosticsAlwaysReturnsMutableMap|TestPromptDiagnosticsAreRedactedAndCollectionBounded|TestProviderUsageAndWireBudgetDiagnosticsNormalizeActuals|TestWakeUpScheduleStatusIsExplicitWhenContextIsMissing)$' -count=1

Result: PASS (twelve exact tests).

Final S07 exact-name gate after the approved Visual Identity extension and
independent-review fixes:

    GOCACHE=/tmp/lac-runtime-recovery-s07-core-final2 go -C apps/core-go test ./internal/core -run '^(TestLifecycleDiagnosticPayloadIsBoundedAndRedacted|TestLifecycleDiagnosticTransitionIdentityDeduplicatesRepeats|TestLifecycleDiagnosticWriterRejectsUnavailableStore|TestLifecycleDiagnosticWriterOwnsWorkflowLinkPersistence|TestPromptDiagnosticsAlwaysReturnsMutableMap|TestPromptDiagnosticsAreRedactedAndCollectionBounded|TestProviderUsageAndWireBudgetDiagnosticsNormalizeActuals|TestModelRunPersistenceDoesNotReturnGhostID|TestProviderAttemptIdentityIsStableWithinInvocationAndDistinctAcrossRetries|TestProviderModelRunIdentitySeparatesAttempts|TestProviderRequestIdentityStaysStableAcrossDiagnosticAttempts|TestProviderPreflightFailuresAreDiagnosedBeforeQueue|TestProviderPreflightDiagnosticsAreMetadataOnly|TestProviderModelRunLifecycleCannotRegressFromTerminalToQueued|TestLifecycleProviderAttemptsRemainDistinct|TestLifecycleMetadataRedactionHandlesTypedContainersAndCycles|TestLifecycleDiagnosticsPreserveFirstAndLatestFailureCause|TestProviderPreflightErrorsHaveStableCategoryAndRetryability|TestLifecycleOwningFilesUseOnlyExplicitBestEffortAssignments|TestLifecycleTraceDistinguishesNoopActionableRetryAndFailure|TestWakeUpAndReflectionProviderCallsUseLifecycleCorrelation|TestWakeUpDerivedIntentsKeepCycleCorrelation|TestWakeUpReturnPreservesQueuedBlockedAndNoopStatus|TestWakeUpScheduleStatusIsExplicitWhenContextIsMissing|TestVisualIdentityWaitingOutcomesRequireAuthoritativeSettlement)$' -count=1 -v

    GOCACHE=/tmp/lac-runtime-recovery-s07-workflow-final2 go -C apps/core-go test ./internal/workflow -run '^(TestWakeUpTerminalFailureRemainsContinuouslyRecoverable|TestLifecycleCorrelationForIntentUsesStableWakeUpCycleIdentity|TestDispatcherHydratesCorrelationAndCausationBeforeWorkflowStart|TestActivityLifecycleDiagnosticIncludesTemporalIdentityAndAttempt|TestActivityLifecycleDistinguishesNoopAndActionableOutcomes|TestAlreadyRunningTransitionRetainsTemporalRunIdentity|TestDispatcherCASCompetitionDoesNotFailSettledIntent|TestWorkflowCriticalPersistenceWritesAreNeverIgnored|TestReconcileDoesNotGuessCompletedWorkflowOutcome|TestWorkflowIDReusePolicyAllowsWakeUpRecovery|TestVisualIdentityStartOptionsAllowFailedRecoveryAndExposeDuplicateStart|TestWakeUpRetryBackoffIsNotRequeuedBeforeDueTime|TestReflectionTerminalFailureRemainsContinuouslyRecoverable|TestReflectionRetryRequiresLiveFluctlightEvidenceAndBudget|TestActionIntentRetryOnlyWhenActionRemainsExecutable|TestDispatcherPrioritizesLifecycleRecoveryBeforeVisualIdentityRetries|TestDispatcherFairSelectionRanksEachIntentClassBeforeBacklog)$' -count=1 -v

Result: PASS (25 Core exact tests and 17 Workflow exact tests).

Checks:

- gofmt ran only on S07 Go files.
- git diff --check passed.
- HEAD and origin/master both remain
  `627cd7db0d1b18e0da14aacea3292b1dcaacbacc`.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

### S03 — PostgreSQL-authoritative WakeUp clock

Status: complete.

Implemented:

- platform_workflow_intents.next_attempt_at is the durable next due time.
- normal WakeUp outcome writes next due inside the same transaction as the
  cognition fact, wake row, Reflection/action intents and outbox events.
- cadence advances from the prior due slot to the next future slot; Provider
  latency cannot drift the entire schedule.
- duplicate cycle replay repairs/preserves a future next due before returning.
- disabled/paused/inactive outcomes preserve an explicit next due without
  invoking normal cognition.
- ReleaseDueWakeUpIntents atomically selects completed/due active rows with
  FOR UPDATE SKIP LOCKED and applies status/due CAS while incrementing cycle.
- Redis expiry calls the same single-row conditional release; Redis SET now
  returns transport failures, while PostgreSQL release remains authoritative.
- startup initializes legacy null due times and schedules Redis only for future
  due rows.
- Worker releases due rows at startup and each dispatcher tick, and performs a
  bounded missing-clock audit on the retention interval.
- overdue/missing and trigger-released transitions use lifecycle diagnostics.

Exact Core command:

    go test ./internal/core -run '^(TestRedisTriggerSchedulingUsesStableKeysAndTTL|TestWakeUpRedisHintReturnsTransportFailure|TestNextWakeUpDuePreservesFixedCadence|TestWorkerRunsPostgresWakeUpReleaseAndClockAudit|TestWakeUpHasPostgresDueSweepAndOverdueDiagnostic|TestWakeUpDueReleaseUsesLockedStatusAndDueCAS|TestWakeUpOutcomePersistsNextDueInsideOwningTransaction)$' -count=1 -v

Result: PASS (seven exact tests).

Targeted Worker compile:

    go test ./cmd/worker -run '^$' -count=1

Result: PASS; package has no test files.

Deferred S12 PostgreSQL test authored but not run:

- TestPostgresWakeUpClockRecoversLostRedisAndDeduplicatesRelease covers two
  cycles, lost Redis expiry, duplicate release CAS and overdue event
  persistence.

Checks:

- gofmt ran only on S03 Go files.
- git diff --check passed.
- The S01 Schedule-gate/user-turn reset red tests intentionally remain red
  until S04; terminal-failure reconciliation remains S05.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

### S02 — Typed lifecycle diagnostics foundation

Status: complete.

Implemented:

- lifecycle-diagnostic.v1 typed transition envelope with the approved
  transition vocabulary, stable correlation/identity validation, bounded
  strings/arrays/maps/depth, binary redaction and secret-key redaction.
- deterministic transition ID independent of incidental metadata; attempt
  changes remain distinct.
- transactionally coupled diagnostic_events and diagnostic_workflow_links
  persistence.
- conflict aggregation retaining first_seen_at, updating last_seen_at and
  incrementing occurrence_count.
- explicit ErrDiagnosticsUnavailable from the checked writer plus a
  component/stage/type keyed one-minute operational warning limiter for
  best-effort callers.
- checked model-run/event/state/metrics persistence; queued model-run creation
  returns an empty ID after persistence failure instead of a ghost ID.
- existing queued→terminal deterministic row identity retained until S07 can
  propagate a Provider-owned attempt identity across provider.go.
- additive diagnostic event/link lookup indexes in the authoritative schema.

Exact Core command:

    go test ./internal/core -run '^(TestModelRunPersistenceDoesNotReturnGhostID|TestLifecycleDiagnostic(PayloadIsBoundedAndRedacted|TransitionIdentityDeduplicatesRepeats|WriterRejectsUnavailableStore|WriterOwnsWorkflowLinkPersistence))$' -count=1 -v

Result: PASS (five exact tests).

Additional redaction command:

    go test ./internal/core -run '^TestLifecycleDiagnosticPayloadIsBoundedAndRedacted$' -count=1 -v

Result: PASS.

Exact migration command:

    go test ./internal/migrations -run '^TestLifecycleDiagnosticSchemaAddsCorrelationLookupIndexes$' -count=1 -v

Result: PASS.

Checks:

- gofmt ran only on S02 Go files.
- git diff --check passed.
- No production lifecycle producer is connected yet; that remains S03/S07.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

### S06 — Fair dispatcher, Temporal and Provider capacity

Status: complete.

Implemented:

- Dispatcher SQL ranks candidates by ROW_NUMBER partitioned by intent_type and
  orders class_rank before business priority. One backlog can no longer fill
  the entire LIMIT before another due intent class receives a slot.
- new Visual Identity executions dispatch to the isolated visual-identity
  Temporal queue with concurrency 1.
- lifecycle Worker retains VisualIdentity workflow/activity registration so
  pre-isolation histories continue on their original task queue.
- local Provider heap records enqueue time and selects the oldest task after a
  two-minute maximum wait before fresh strict-priority tasks; normal priority/
  FIFO, cancellation and configured concurrency remain unchanged.
- Redis Provider queue persists queued_at/sequence and atomically ages the
  first bounded pending window ahead of fresh priority requests after the same
  two-minute wait.
- dispatcher status settlement retains expected pending/retry CAS introduced
  by the prior Visual Identity fix and does not overwrite cancel/state drift.

Exact Core command:

    go test ./internal/core -run '^(TestProviderQueueHonorsPriorityAndFIFO|TestProviderQueueCancellationReleasesPendingTask|TestProviderQueueHonorsConfiguredConcurrency|TestProviderQueueAgesBoundedWaitTaskAheadOfFreshPriority|TestProviderRedisKeysKeepBindingsSeparate|TestProviderRedisScoreKeepsPriorityBeforeFIFO|TestProviderRedisQueueScriptAgesBoundedWaitJobs|TestProviderRedisSlotHonorsPriorityAndLease|TestProviderRedisSlotDropsLegacyPendingOrphan)$' -count=1 -v

Result: PASS (nine exact tests).

Exact Redis aging behavior:

    go test ./internal/core -run '^TestProviderRedisAgingSelectsOldLifecycleBeforeFreshPriority$' -count=1 -v

Result: PASS against Miniredis/Lua claim behavior.

Exact Workflow command:

    go test ./internal/workflow -run '^(TestDispatcherFairSelectionRanksEachIntentClassBeforeBacklog|TestVisualIdentityUsesDedicatedQueueWithLifecycleCompatibility|TestDispatcherPrioritizesLifecycleRecoveryBeforeVisualIdentityRetries)$' -count=1 -v

Result: PASS (three exact tests).

Checks:

- gofmt ran only on S06 files.
- git diff --check passed.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

### S05 — Reflection quiet-period and crash recovery

Status: complete.

Implemented:

- all production reflection.run writers in cognition.go, wakeup.go,
  workflow_ops.go, action_outcome.go, operations.go and autonomy.go use one
  insertReflectionIntentTx boundary with explicit next_attempt_at.
- the extra action/autonomy files were added to the S05 allowlist only after
  explicit user approval.
- user quiet-period creation supersedes older pending/retry user-activity
  intents in the same cognition transaction; started work is not overwritten.
- Reflection window claim stores an exact updated_at lease token in context;
  cleanup requires the same token/status and checks RowsAffected, so an old
  attempt cannot clear a newer claim.
- cleanup failure/conflict emits a bounded operational warning and lifecycle
  failure event even when callers perform best-effort cleanup.
- evidence and appraisal iterators check rows.Err before accepting partial
  data or advancing the watermark.
- no evidence returns status=no_op, reason=no_evidence.
- failed Reflection intents remain in reconciliation; active + unconsumed
  evidence + attempt budget is required for retry.
- retry uses failed-only workflow-ID reuse, five-minute backoff and five
  attempts, ensuring the retry horizon outlives the 15-minute window lease.
- exhausted/non-actionable failure settles to dead_letter with bounded cause
  and lifecycle diagnostic.

Exact Core command:

    go test ./internal/core -run '^(TestReflectionIntentWritersUseExplicitQuietPeriod|TestReflectionWindowCleanupUsesLeaseTokenAndCAS|TestReflectionEvidenceReadersCheckRowsErrors|TestUserActivitySupersedesEarlierPendingReflectionQuietPeriod)$' -count=1 -v

Result: PASS (four exact tests).

Exact Workflow command:

    go test ./internal/workflow -run '^(TestReflectionTerminalFailureRemainsContinuouslyRecoverable|TestReflectionRetryRequiresLiveFluctlightEvidenceAndBudget)$' -count=1 -v

Result: PASS (two exact tests).

Additional direct-writer count test:

    go test ./internal/core -run '^TestReflectionIntentWritersUseExplicitQuietPeriod$' -count=1 -v

Result: PASS.

Deferred S12 PostgreSQL test authored:

- TestReflectionWindowLeasePreventsOldAttemptCleanup reclaims an expired lease,
  proves old cleanup conflicts without clearing the new claim, and proves the
  current lease can release.

Checks:

- gofmt ran only on S05 files.
- git diff --check passed.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

### S04 — Activation, Schedule decoupling and lifecycle rearm

Status: complete.

Implemented:

- CreateFluctlight creates stable schedule.current_day and wake_up.current
  intents in the same activation transaction.
- idempotent activation replay calls the same lifecycle-intent repair helper.
- the stable WakeUp starts on the configured interval independently of Schedule.
- EnsureWakeUpIntents no longer queries life_schedules and initializes missing
  due times for active/paused Fluctlights.
- successful user-turn paths only reset Reflection quiet-period hints; all
  scheduleWakeUpTrigger calls were removed from cognition.go/mutations.go.
- WakeUp current input carries schedule_status=missing/pending/ready; missing
  Schedule remains explicit and no current activity/place is synthesized.
- paused/inactive/disabled results preserve due policy; active lifecycle
  transition ensures/releases an overdue stable WakeUp.
- UpdateSettings validates/normalizes partial product.wakeup updates, preserves
  omitted values and rearms only disabled→enabled.
- rearm failure after a committed lifecycle/settings transition is surfaced as
  an operational/lifecycle diagnostic rather than silently discarded.

Exact command:

    go test ./internal/core -run '^(TestEnsureWakeUpIntentsDoesNotRequireAcceptedSchedule|TestConversationDoesNotPostponeWakeUp|TestCreationOwnsIndependentScheduleAndWakeUpIntents|TestWakeUpScheduleStatusIsExplicitWhenContextIsMissing|TestFluctlightActivationRearmsWakeUpClock|TestWakeUpSettingsNeedsRearmOnlyWhenEnabledAgain|TestMergeWakeUpSettingsPreservesOmittedValuesAndRejectsWrongTypes|TestUpdateSettingsRearmsDurableWakeUpClockAfterEnable)$' -count=1 -v

Result: PASS (eight exact tests).

The new settings file was added to the S04 allowlist only after explicit user
approval. The implementation was kept in the application settings owner rather
than adding HTTP-layer business repair.

Deferred S12 PostgreSQL evidence:

- TestEnsureWakeUpIntentsRepairsExistingLiveFluctlight now intentionally creates
  no Schedule row before asserting the stable WakeUp intent.
- TestPostgresWakeUpClockRecoversLostRedisAndDeduplicatesRelease remains the
  cross-cycle database gate.

Checks:

- gofmt ran only on S04 files.
- git diff --check passed.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

### S11 — Initialization source UI, contract audit and S12 preparation

Status: complete; PostgreSQL concurrency behavior is authored for S12 and was
not executed during S11.

Implemented:

- successful analysis now returns a safe root `analysis_id` plus its diagnostic
  `correlation_id`; source persistence uses the authenticated Owner rather than
  selecting an arbitrary account.
- activation requires the explicit analysis identity for `llm_defined`, locks
  the Owner before checking/creating the deterministic Fluctlight, accepts only
  the latest successful unlinked Owner source, and binds that exact source to
  the accepted initialization Foundation revision.
- source freshness orders by the request-admission timestamp captured before
  Provider I/O. A slow older Provider response cannot supersede a newer
  analysis request merely because it completed later.
- same-request concurrency is serialized on the Owner row. Replays verify the
  already linked source and frozen activation digest; changed blank-slate or
  analyzed payloads conflict instead of silently returning another payload.
- Foundation edits remain intentional: source freshness/identity is checked,
  but the edited projection is not required to equal the original Provider
  projection digest. Edited Core Persona, Developing Self, extensions,
  actor_user relationships, goals and intentions remain activation authority.
- Provider and source-persistence failures now use runtime HTTP statuses while
  preserving bounded correlation/retryability. Provider-derived error codes
  are limited to bounded snake_case prefixes; raw Provider text cannot become
  an error code or log field.
- Browser/BFF/Core align on 60,000 UTF-8 bytes. Web uses `TextEncoder`; BFF uses
  Go string byte length and returns explicit 413
  `initialization_description_too_large`; Core retains the same byte contract.
- Browser OpenAPI now owns typed analysis/create/activation/source/detail
  contracts. The generated client carries analysis authority and the optional,
  nullable Owner-only source projection.
- the creation review owns one typed `BrowserFluctlightCreationAnalysis`; raw
  JSON is only its editor. Valid edits replace the typed value, invalid JSON
  disables activation, edits rotate the request identity, and request epochs
  prevent superseded analysis responses/failures from publishing.
- detail source rendering is collapsed, Owner-labelled, text-bound (no
  `v-html`), and shows source, classification, Provider/model/version,
  coverage, derivations, structured projection, Foundation relationship,
  timestamps and digests. Missing source hides the panel.
- public detail sets `Cache-Control: no-store, private`. Pinia and the detail
  component bind each detail response to a Fluctlight/request epoch, preventing
  one instance's original card from appearing beneath another instance's
  heading during late response/poll races.
- workflow, diagnostics, Persona and Visual Identity executable contracts were
  updated for PostgreSQL-authoritative WakeUp, Reflection separation,
  transition-only lifecycle diagnostics, diagnostic-writer warnings,
  metadata-only initialization diagnostics, source authority, and
  authoritative Visual Identity settlement.

Final exact Core HTTP command:

    GOCACHE=/tmp/lac-runtime-recovery-s11-core-final go -C apps/core-go test ./internal/httpapi -run '^(TestActivationFailureDetailsSeparatePersonaConflictAndPersistence|TestActivationFailureDetailsPreserveWrappedPersonaAndBoundLogCode|TestActivationAnalysisFailureDetailsAndStatusesRemainDistinct|TestInitializationAnalysisFailureStatusDistinguishesRuntimeAndSemanticErrors|TestInitializationDescriptionLimitUsesUTF8Bytes|TestInitializationCreationContractBindsExplicitLatestAnalysis)$' -count=1 -v

Result: PASS (six exact tests).

Final exact BFF command:

    GOCACHE=/tmp/lac-runtime-recovery-s11-gateway-final go -C apps/gateway-go test ./internal/bff -run '^TestBFFErrorMappingsPreserveRouteSpecificPolicy$' -count=1 -v

Result: PASS (one exact test).

Final exact browser-client command:

    pnpm exec tsx --test --test-name-pattern='^BrowserClient transports initialization analysis authority and typed detail source$' test/client.test.ts

Result: PASS (one exact test).

Final exact Web commands:

    node --test --test-name-pattern='^detail exposes the owner-only initialization source as a collapsed safe panel$' test/detail-display.test.mjs
    node --test --test-name-pattern='^creation preview has one typed authority, stale-analysis epochs, and a UTF-8 byte limit$' test/skeleton.test.mjs

Result: PASS (two exact tests).

Deferred S12 PostgreSQL test authored but not run:

- `TestPostgresInitializationAnalysisAuthoritySerializesLatestSourceAndReplay`
  creates its own disposable child database and verifies stale-source rejection,
  simultaneous same-request replay, one source link, and changed blank-slate
  payload conflict. The S12 disposable PostgreSQL command now includes
  `./internal/httpapi` so this test executes only at the final gate.

Generation and checks:

- two consecutive targeted OpenAPI/client generations were byte-identical:
  - `openapi.json` SHA-256:
    `2dcd4a11f80147b805b02c75a131846566443bfa6271227492910bc1457e7c26`
  - `src/index.ts` SHA-256:
    `4bb71476becfc22e044b6d8e1b54bf085437b9c6701704a757e3072044abf1b2`
- S01-S10 checklist audit found no unchecked item before S11.
- S11 Go files are gofmt-clean and the targeted `git diff --check` passed.
- an optional targeted Prettier check was unavailable because this workspace
  has no `prettier` command; an optional direct SFC parser probe was also
  unavailable because `@vue/compiler-sfc` is not installed at that package
  boundary. No dependency was installed or unrelated file changed; exact Web
  source-contract tests passed, and S12 retains the required workspace
  typecheck/build gates.
- the protected `.trellis/tasks/09-08-project-health-evolution/` path remains
  untracked and untouched.
- HEAD and origin/master remain
  `627cd7db0d1b18e0da14aacea3292b1dcaacbacc`.
- No full, race, vet, build, PostgreSQL, Temporal live, Docker Compose or Live
  Provider validation ran.

S12 environment prerequisites, not yet asserted available:

- `GO_CORE_TEST_DATABASE_URL` points only to a dedicated disposable PostgreSQL
  server/database with permission to create/drop an isolated child database;
  never a user/shared database because the migration fixture mutates its ledger.
- Docker is available with a task-specific env file and isolated Compose
  project name `lac-runtime-recovery`; no existing
  `infra/compose/fluctlight.env` is overwritten or reused destructively.
- Temporal, Redis, object storage and required application ports/volumes are
  available to that isolated Compose project.
- the configured local Provider is reachable with the intended
  `generic_llm`/initialization model and secret; Live tests use the explicit
  `FLUCTLIGHT_LIVE_PROVIDER_*` values from the S12 command.
- `FLUCTLIGHT_LIVE_INITIALIZATION_CARD_PATH` and
  `FLUCTLIGHT_LIVE_INITIALIZATION_EXPECTATIONS_PATH` point to readable Owner
  files outside the repository. Neither path/content may be copied into Git,
  snapshots, diagnostics or test output.

### S12 entry-gate preflight — blocked before execution

Read-only preflight on 2026-09-13 found:

- none of `GO_CORE_TEST_DATABASE_URL`, `FLUCTLIGHT_LIVE_PROVIDER_TEST`,
  `FLUCTLIGHT_LIVE_PROVIDER_URL`, `FLUCTLIGHT_LIVE_PROVIDER_MODEL`,
  `FLUCTLIGHT_LIVE_INITIALIZATION_CARD_PATH`,
  `FLUCTLIGHT_LIVE_INITIALIZATION_EXPECTATIONS_PATH`, `FLUCTLIGHT_ENV_FILE`,
  or `COMPOSE_PROJECT_NAME` is set in the current process environment;
- the external Owner card and expectation manifest are therefore both unset;
- Docker client `29.4.0` and Compose `5.1.2` are installed, but the Docker
  daemon is unavailable at the configured OrbStack socket;
- repository templates `infra/compose/fluctlight.env.example` and
  `infra/compose/temporal-gate.env.example` exist, but no template was copied,
  edited, or treated as an authorized isolated runtime environment.

S12 has not started. No full/race/vet/build, PostgreSQL, Temporal, Live
Provider, Compose, smoke, or acceptance command was run after this preflight.

Continuation preflight progress:

- OrbStack was started successfully; Docker client/server are both `29.4.0`.
- a dedicated disposable PostgreSQL container named
  `lac-runtime-recovery-pg-0913` now runs `pgvector/pgvector:pg16`, is labelled
  for this S12 validation, binds only `127.0.0.1:55433`, owns no reused volume,
  and passes `pg_isready` for its disposable test database;
- `/tmp/lac-runtime-recovery.env` was created as a non-repository,
  task-specific Compose environment with independent browser ports
  `23000/23001`, test-only credentials, a valid 32-byte settings key, and the
  `runtime-usability-recovery` Temporal namespace;
- `docker compose ... config --quiet` accepts that environment and both host
  ports are available. Repository `.env` and
  `infra/compose/fluctlight.local.env` were not modified or copied;
- the configured local Provider endpoint at `127.0.0.1:11234/v1` is listening,
  and its read-only model inventory includes
  `huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX`; no completion or character
  card was sent during preflight;
- local Trellis conversation history contains no trustworthy external Owner
  card/expectation file path. The shared attachment is the Prompt Context /
  Memory architecture requirement, not a private character card.

The sole remaining S12 entry-gate dependency is an explicit readable external
Owner card path plus its external expectation manifest path. They may not be
replaced with the committed anonymized fixtures because the PRD requires a real
configured-LLM regression against the Owner's actual card.

User-authorized external acceptance substitution:

- the Owner confirmed no pre-existing real card/manifest is available and
  explicitly requested that this task construct a complex card instead;
- `/tmp/lac-runtime-recovery-complex-card.txt` is a 6,991-byte natural-language
  dual-profile card covering full base identity, appearance/outfits, long
  background, actor_user relationship, ordered profiles, voice/habits,
  ordinary/forced switching, influence/fusion, special boundaries, media
  frequencies, daily settings, core conflict, goals, and intentions;
- `/tmp/lac-runtime-recovery-complex-expectations.json` contains 30 canonical
  semantic assertions and five non-invention guards, including no third
  profile, invented birthday/trauma/medical history, or extra Actor relation;
- both files are outside the repository, mode `0600`, JSON/path validated, and
  will be supplied only to the private Live Provider harness. This explicit
  user decision replaces the unavailable personal-card canary without
  replacing the repository's separate dense single/multi live cases.

### S12 validation progress

Entry gate: PASS after the Owner-authorized constructed external card and
manifest were prepared outside the repository.

Required-command results in order:

1. Core full test initially exposed three stale initialization test assumptions
   in the already-authorized `initialization_contract_test.go`: the former
   projection-digest variable name, a helper-only StructuredFallback assertion
   that skipped production normalization, and a one-profile fixture marked
   `multiple` after the cardinality contract changed. The tests were aligned
   with the accepted S10/S11 behavior; their three exact regressions passed.
2. Re-run Core full test: PASS.
3. Core race test: PASS.
4. Core vet: PASS.
5. Core build: PASS.
6. Gateway full test: FAIL at
   `TestGoRouteInventoryMatchesBrowserOpenAPI`: Go inventory 76 versus OpenAPI
   77, with only `GET /api/diagnostics/lifecycle` missing from the hand-written
   `browserRouteCases()` fixture. The production route exists and its S08
   behavior tests pass; deleting it or the OpenAPI operation would violate the
   accepted Lifecycle Diagnostics contract.

Validation is paused before Gateway race. The required fix belongs to
`apps/gateway-go/internal/bff/parity_routes_test.go`, which was not previously
modified in S01-S11. It has been proposed in the S12 allowlist and remains
untouched pending the Owner's explicit boundary approval.

Repeated S12 entry-gate audit:

- a subsequent continuation revalidated the dedicated PostgreSQL container,
  isolated Compose environment, Docker daemon, Provider endpoint, and exact
  target model as ready;
- neither external-card environment variable is set and the attachment
  directory still contains only `pasted-text.txt`, whose contents are the
  Prompt Context/Memory architecture requirement rather than a character card;
- the same external Owner-card dependency has therefore remained unresolved
  across three consecutive goal turns. S12 remains unstarted so the anonymous
  fixtures are not falsely reported as the required real-card acceptance.

PostgreSQL-gate prompt-budget diagnosis:

- after migration-test isolation fixes, the combined PostgreSQL gate reached
  real conversation/life-context integration cases but stopped before Provider
  I/O with `prompt_required_budget_exceeded`;
- a new deterministic full-catalog regression measured 11 conversation
  capabilities at 9,334 estimated tokens and the response schema at 7,752,
  totalling 17,086 against an obsolete 16,384 Tools/schema sub-cap. System was
  4,656 and complete required input 21,758, safely below both the current
  98,304 default and legacy 49,152 maximum input budgets;
- the failure therefore reflects a 702-token sub-cap mismatch, not an unsafe
  expansion of the total model context. The proposed correction raises only
  `defaultToolsSchemaTokensCap` to 24,576 and retains the existing system,
  current-input, total-input, output-reserve, and safety-margin gates;
- `prompt_context_assembler.go` remains untouched pending explicit S12
  allowlist approval. Validation is paused before re-running the PostgreSQL
  command or any later Live Provider/Compose command.

Latest S12 continuation superseding the historical pause above:

- the Owner explicitly approved the constructed external card/manifest and the
  `parity_routes_test.go` S12 allowlist addition;
- Lifecycle Diagnostics was added to the hand-written browser route inventory;
  its two exact route/parity tests passed, followed by Gateway full test, race,
  vet and build PASS;
- repository-wide Go formatting and `git diff --check` passed;
- workspace test, TypeScript/Vue typecheck, production build, root generation,
  and generation-following `git diff --check` passed;
- the first combined disposable-PostgreSQL command exposed package-level DB
  interference and three test-harness defects. Initialization Source migration
  and activation-authority tests now create/use true child databases and avoid
  pgx multi-command prepared statements; their exact PostgreSQL tests pass;
- the documented PostgreSQL gate now applies the migration head first and runs
  packages with `-p 1`, preventing migration-ledger fixtures from racing Core
  and HTTP integration tests;
- one stale previous-head assertion remains in
  `prompt_context_memory_postgres_test.go`: the test correctly constructs the
  `0031` fixture and runs the current migration runner, but still expects the
  resulting head to stop at `0032` although the released current head is
  `0033_initialization_source`. The file is untouched pending explicit S12
  allowlist approval.

### S12 final validation and live runtime recovery

The historical pauses above were resolved through explicit S12 allowlist
approvals. Final validation completed on `master` at baseline HEAD
`627cd7db0d1b18e0da14aacea3292b1dcaacbacc`; no commit or push was made.

Required validation results:

- Core: full test PASS, race PASS, vet PASS, build PASS.
- Gateway: full test PASS, race PASS, vet PASS, build PASS.
- repository Go formatting and `git diff --check`: PASS.
- workspace: `pnpm -r test`, typecheck, build, generation and post-generation
  drift check: PASS. Final lightweight generation/typecheck and Core OpenAPI
  checks also PASS.
- disposable PostgreSQL: migration PASS; serial migrations/Core/HTTPAPI PASS;
  focused WakeUp/Reflection/LifecycleDiagnostic/InitializationSource suite
  PASS.
- configured Live Provider initialization: dense single PASS in 101.53s,
  dense multi PASS in 79.61s, external constructed complex card PASS in
  157.93s; complete command PASS in 339.493s.
- isolated Compose: build/up PASS, platform smoke PASS, bind-source check PASS,
  Core OpenAPI check PASS, Go projections PASS, active Temporal workflow PASS.
  Final active workflow used Worker Build ID
  `s12-runtime-recovery-final6`, completed with history length 11, and retained
  AutoUpgrade deployment routing.

S12 live WakeUp initially exposed four acceptance-only defects inside the
accumulated allowlist:

1. `lifecycle` had two shared Activity slots. Provider-backed Daily Review work
   consumed both, so a due WakeUp had `ActivityTaskScheduled` but no
   `ActivityTaskStarted` for more than four minutes. New WakeUp/Reflection Runs
   now use the two-slot `lifecycle-critical` queue; both types remain registered
   on `lifecycle` for pre-isolation history compatibility. The exact queue,
   routing, legacy registration and fairness tests pass.
2. A normal WakeUp response with state-changing capability calls but no
   `influences` failed as `wake_up_influences_required`. Core now treats the
   field as empty, keeps only deferred output capabilities, discards
   ungrounded internal/state calls, and persists
   `completed_noop/capability_influences_missing`. A proposed external action
   with no capability call similarly persists
   `completed_noop/action_requires_capability_call`.
3. `diagnostic_model_runs` remained empty because the provenance INSERT reused
   parameter `$2` in contexts where PostgreSQL inferred inconsistent types.
   The binding-role parameter is now explicitly `varchar(64)`. Diagnostic
   persistence warnings also include a bounded secret-redacted `safe_cause`,
   which exposed SQLSTATE `42P08` without logging Prompt/card content.
4. Reflection returned a complete closed shape but used Provider alias
   `reflection.v2`; Core rejected it as `reflection_v2_header_invalid`.
   Reflection protocol version is now unconditionally Core-owned and a missing
   summary receives neutral `no_change`. Unknown candidates, evidence, and
   mutation authority remain strict. Reconciliation also skips future pending
   intents, preventing scheduled Reflection from being falsely diagnosed as
   `workflow_describe_failed`.

Authoritative real-runtime evidence for test Fluctlight
`fluctlight_81f7060fa3edd766a0306fb917591431`:

- stable WakeUp intent/workflow IDs were retained across retries and cycles;
- cycle 1 completed at `2026-09-13T14:24:34Z`, cycle 2 completed at
  `2026-09-13T14:25:37Z`; PostgreSQL due release emitted
  `trigger_released` and incremented the same payload to cycle 2;
- later post-diagnostic-fix cycle 4 completed with one model run
  `model_run_a657c25be03809cd982fd974e715fb59`, one provider attempt, endpoint
  `s12-local-provider`, the configured 27B local model, and 39.382s Provider
  latency;
- cycle 4 lifecycle order was
  `trigger_released -> queued -> dispatched -> activity_started ->
  provider_queued -> next_cycle_scheduled -> completed_noop`;
- four WakeUp facts were written by real LLM calls; all had explicit
  `capability_influences_missing` no-op results and next due times. No visible
  chat message was required or fabricated;
- each WakeUp produced an explicit quiet-period Reflection intent with the
  stable cycle correlation;
- final Reflection for cycle 2 completed on `lifecycle-critical` with Worker
  Build ID `s12-runtime-recovery-final6`, model run
  `model_run_e4e3c051c01dfacf5f1dcc54f19274f9`, 30.755s Provider latency,
  proposal `reflection_proposal_f60516a6903832ad18f3ce5622f1d578`, status
  `applied`, watermark `0 -> 4`, and one accepted affect-profile revision;
- Reflection lifecycle ended in `completed_actionable/applied`; the Reflection
  window returned to `idle`.

The repeated-bug retrospective classified the failures as a combination of:

- D / test coverage gap: isolated queue and parser tests did not exercise a
  contended real Worker plus local Provider;
- E / implicit assumption: increasing shared lifecycle concurrency was treated
  as reservation, Provider protocol strings were treated as model authority,
  and PostgreSQL parameter inference was assumed stable across SELECT target
  and subquery contexts;
- B / cross-boundary contract: optional model metadata was validated before
  safe Core normalization, and diagnostic persistence failure logs dropped the
  only discriminating cause.

Prevention is now present in architecture (critical lane and Core-owned
headers), runtime behavior (safe no-op and bounded cause), tests (queue/history,
missing influences, header alias, provenance cast, future pending reconcile),
live acceptance (two WakeUp cycles plus Reflection), and updated backend
workflow/diagnostics/persona specs. `trellis-break-loop` also asks to sync
global templates and commit immediately; those actions were deliberately not
performed because the user explicitly prohibited out-of-allowlist edits and
did not authorize a commit.

No unrelated failure was repaired. The untracked protected directory
`.trellis/tasks/09-08-project-health-evolution/` remained untouched.

Post-acceptance cleanup completed after evidence capture:

- removed isolated Compose project `lac-runtime-recovery`, its network and its
  three named PostgreSQL/Redis/MinIO volumes;
- removed dedicated disposable PostgreSQL container
  `lac-runtime-recovery-pg-0913`;
- the smoke and active-workflow scripts had already removed their own temporary
  projects;
- removed only `/private/tmp/lac-runtime-recovery*` task files/caches,
  including the constructed private card/manifest, cookie jar and env files;
- a final exact-name/project-label check found no matching task containers,
  volumes or temporary files.
