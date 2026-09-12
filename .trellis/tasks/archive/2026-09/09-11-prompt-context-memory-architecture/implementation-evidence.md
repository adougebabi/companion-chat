# Implementation Evidence

## Execution Boundary

- Task: `prompt-context-memory-architecture`
- Branch: `codex/prompt-context-memory`
- Base commit: `6cbc18b1329b7cba1c4b4a3af7091baf00811481`
- Strict policy: S01-S11 targeted tests only; all full/race/vet/build/PostgreSQL/Temporal/Live Provider/Compose gates deferred to S12.
- Protected unrelated path: `.trellis/tasks/09-08-project-health-evolution/` — not modified, formatted, removed, or staged.

## S01 — Baseline Gate and Exact-Code Read

Status: complete

### Write boundary

Only this task's planning and evidence files were changed while establishing S01. No production file was modified.

### Repository baseline

```text
branch: codex/prompt-context-memory
HEAD:   6cbc18b1329b7cba1c4b4a3af7091baf00811481
dirty:  ?? .trellis/tasks/09-08-project-health-evolution/
        ?? .trellis/tasks/09-11-prompt-context-memory-architecture/
```

### Production source hashes before S02

```text
42d3e65c4174a244aecbfc758b28171c1fef26231b4bfc4b5b564bb6590eb6ef  apps/core-go/internal/migrations/runner.go
096d8f031ddbe0db4ce7889ee08e4a85864c1dfce9968020fe3add3e87715691  apps/core-go/internal/migrations/runner_test.go
bd231bd41978e5924ca25129f446680c7ae7f0710a1742f08bbf7f9c2071c735  apps/core-go/internal/core/repository.go
ace36be80cb77bceb7048598b779583f0c9722e17a29f4af5dd17e9752c08d33  apps/core-go/internal/core/cognition.go
019e10d0c4ae351167acbbeb30f828d37c50bb40a80d69765762ac9eb4f715da  apps/core-go/internal/core/mutations.go
5feb62435b972feda342724d2c6983ad2638f696bcd37ce5d81012b5b6c0a6c2  apps/core-go/internal/core/conversation_delivery_regression_test.go
ca6373c28e95e5e98611bfa162d3673362b033f9fa95c49333aadd2c6d9835d4  apps/core-go/internal/core/provider_schemas.go
a6e925b16d45b306953f166cfe6c80f87cd5091b1b678ce27c179e39026179a1  docs/capability-schema-report.md
```

### Artifacts and specifications read

- Current task `prd.md`, `design.md`, `implement.md` and all task research Markdown.
- `.trellis/spec/backend/index.md` and shared guide index/thinking guides.
- Backend directory, error, debug-observability and quality guidelines.
- Fluctlight Memory, Provider, Cognitive Runtime, Structured Turn, Persistence, Workflow, Configuration, Diagnostics, Event and Temporal Gate contracts.
- `09-09-llm-capability-runtime/design.md` and its latest remediation evidence.
- Complete S02 production sources: migration runner/tests, repository, cognition, mutations, and conversation delivery regression tests.

### Producer/consumer search

Before changing persisted keys, repository-wide read-only searches mapped every production/test reference to `conversation_messages`, every `EnqueueTurnFact`/`enqueueTurnFact*` call and cognition inbox/head insert, and every migration Head/PreviousHead consumer.

Important outcomes:

1. Interactive user message persistence in `mutations.go:519-566` commits before `enqueueTurnFact*` opens its own transaction in `cognition.go:208-309`.
2. `inboxID` is deterministic (`inbox_ + stableDigest("turn:" + idempotency)`), so the message row can bind a stable source fact in the same transaction.
3. Existing enqueue logic owns supersession, per-Fluctlight sequence allocation, claim state, workflow intent and outbox. S02 must extract a caller-owned transaction seam without duplicating those rules.
4. Assistant settlement already shares one transaction with cognitive stages, capabilities, claims, action outcomes, result fact and inbox completion.
5. Other assistant/media producers may leave new source-link columns NULL in S02; the migration is additive and interactive turn linkage is the required first closure.
6. Existing PostgreSQL migration fixtures call `New(pool).Apply` and compare to dynamic `Head`; an additive `0032` should not require rewriting released tests. A dedicated new `prompt_context_memory_postgres_test.go` will cover the new migration and is explicitly added to S02's allowlist, but it will run only in S12.

### Schema and prompt baseline

From the committed deterministic report:

```text
all capability definitions:      4,864 bytes/chars
conversation surface (9 tools): 4,326 bytes/chars
native surface (8 tools):       3,734 bytes/chars
```

Current request facts:

- one leading system + one giant user document;
- current input appears twice;
- recent history is fixed at 12 rows and is not real transport roles;
- Memory is capped at 12 results / 2400 serialized runes;
- output `token_budget` defaults to 4096 and maps only to `max_tokens`;
- no total input budget or per-section trace exists.

The full production response-schema serialized byte/token count is not exposed by current diagnostics or a committed report. S01 did not run a test or ad-hoc package build to manufacture it because early verification is forbidden. Its exact source hash is frozen above; S05 will add a targeted unit metric before changing the assembler, and S12 will capture actual Provider usage.

Planning Role experiment baseline remains recorded separately: 36 successful local Provider calls, selected B, with per-layout request bytes/prompt/completion tokens/latency. It is research evidence, not an S01 verification run.

### Tests and commands

- Tests run in S01: none, by explicit strict boundary.
- Full test/race/vet/build: not run.
- PostgreSQL/Temporal/Compose/Live Provider: not run.

### S02 entry condition

- Exact S02 code and migration ownership have been read.
- S02 may write only its explicit allowlist and task evidence.
- S02 will run only exact pure/unit tests for the new Raw History/migration helpers. PostgreSQL integration tests may be authored but are not executed until S12.

## S02 — Raw History Source Contract and Turn Atomicity

Status: complete; PostgreSQL behavior tests authored and deferred to S12 by policy.

### Modified files

```text
apps/core-go/internal/migrations/runner.go
apps/core-go/internal/migrations/runner_test.go
apps/core-go/internal/migrations/prompt_context_memory_postgres_test.go (new)
apps/core-go/internal/core/cognition.go
apps/core-go/internal/core/mutations.go
apps/core-go/internal/core/raw_history.go (new)
apps/core-go/internal/core/raw_history_test.go (new)
apps/core-go/internal/core/conversation_delivery_regression_test.go
```

No file outside the S02 allowlist was modified. The protected `09-08` task directory remains untouched.

### Implemented

1. Added unreleased additive head `0032_prompt_context_memory`, preserving `0031_evolution_authority` as an explicit prior-head constant.
2. Added duplicate-sequence preflight for Conversation and Cognition streams, then unique sequence indexes.
3. Added nullable `turn_id`, `source_fact_id`, `correlation_id`, a generated FTS document, indexes, and a deferred source-fact FK to `conversation_messages`.
4. Extracted `enqueueTurnFactTx` as the single caller-owned transaction seam; public/background wrappers still own their transaction.
5. Interactive user message, claimed cognition fact, workflow intent and outbox now commit atomically before Provider I/O. Superseded Redis cancellation remains after commit.
6. New and verifiably replayed user messages bind the canonical fact; assistant messages bind the same turn/fact/correlation. Core/browser message maps do not expose the new internal columns.
7. Added logical `PostgresRawHistoryReader` over authoritative Conversation message, Cognition fact and Capability outcome rows. It supports bounded recent reads, FTS message search and explicit source reads with owner/participant checks.
8. Raw History is not duplicated into a `raw_history` table; each envelope names its owning authority and stable source ref.

### Targeted tests run

```text
go test ./internal/migrations \
  -run '^(TestPromptContextMemoryMigrationAddsRawHistorySourceContract|TestPersonalityGrowthSchemaIncludesTypedSlotsAndCapabilityRequests|TestMigrationBridgeAcceptsOnlyReleasedHead)$' \
  -count=1
PASS

go test ./internal/core \
  -run '^TestRawHistoryNormalization(IsBoundedAndSourceStrict|KeepsUniqueChronologicalEvents|BoundsThousandEvents)$' \
  -count=1
PASS
```

The Core package compiled all S02 source/test files while executing only the three pure Raw History unit cases. The migration package likewise compiled the new PostgreSQL test file while executing only exact pure migration-contract cases.

One formatting command initially used a workspace-relative path while already inside `apps/core-go` and failed with `lstat ... no such file or directory`; it changed nothing. The command was rerun with the correct package-relative path.

The recurring local warning `pyenv: cannot rehash: /Users/vinson/.pyenv/shims isn't writable` did not affect test status.

### Deferred to S12; not run in S02

- `TestPostgresPromptContextMemoryMigrationUpgradesEvolutionHeadAndIsIdempotent`
- `TestPostgresPromptContextMemoryMigrationRejectsDuplicateSequencesAndRollsBack`
- `TestPostgresRawHistoryReaderCombinesAuthoritiesAndSearchesMessages`
- `TestDirectConversationMessageIsDurableDuringCognitionAndNoReplyCannotComplete` with new atomic source assertions
- `TestDirectConversationStreamsCommittedUserBeforeProviderAndAssistantAfterCommit` with shared source-link assertions
- `TestDirectConversationSourceFactConflictRollsBackNewUserMessage`

No PostgreSQL URL was used. No full/race/vet/build, Temporal, Compose, or Live Provider command ran.

### S02 final hashes

```text
c13d8763b6ea3f9bcb354eec7814ab1ab4a2627cbad2b8f720183ccb29d701b3  apps/core-go/internal/migrations/runner.go
4def113d4163a1c9ffe16293c18781e612dc96e0c28b53970d0a5623a6d669ec  apps/core-go/internal/migrations/runner_test.go
9b9d61939a67b0cf596ebc727a3073a2d486f258fb3d97b8236483b6c4089db6  apps/core-go/internal/migrations/prompt_context_memory_postgres_test.go
aca48891bd026c7a6067a9c1b7ab7ab474bc31e4735a98976f52d3a717104ec1  apps/core-go/internal/core/cognition.go
7d606ad30ca203deb71f6cf3e910cb9cd8c6e26a6249a9286706556908860f81  apps/core-go/internal/core/mutations.go
e4d4819df6b4093d12e358fd4893c01d2dbbab33b31eed8e98dd5d64035eb41b  apps/core-go/internal/core/raw_history.go
f6459e001b32f5c9386c24e2e1e4774384e91201bcf9089844b2df43bd69ea64  apps/core-go/internal/core/raw_history_test.go
ad15acf7d578345c13e0f8e39f78e6588998de9403fbd166a60cb14d100d5957  apps/core-go/internal/core/conversation_delivery_regression_test.go
```

Targeted `gofmt -l` and targeted `git diff --check` produced no output.

## S03 — Boundary Audit Before Write

Status: paused before production write, pending user confirmation of an exact allowlist correction.

The complete S03 reference/provider/reflection/built-in/test files were read before editing. The strict Reflection request is built by `reflectionProposalV2ProviderSchema()` in `apps/core-go/internal/core/provider_schemas.go`. Adding `ReflectionProposalV2.ActiveMemoryCandidates` without updating that strict schema would make every real Provider response either omit the candidates or fail closed as an unknown field.

Required exact S03 allowlist addition:

```text
apps/core-go/internal/core/provider_schemas.go
apps/core-go/internal/core/provider_schemas_test.go (new)
```

The user approved this exact S03 extension on 2026-09-11. `intelligence.go` remains reserved for S07; Active Memory prompt/retrieval injection will not be pulled forward.

## S03 — Active Memory Authority

Status: complete; pure/unit contracts passed. PostgreSQL lifecycle/runtime cases were authored and compiled but are deferred to S12 by policy.

### Modified files

```text
apps/core-go/internal/migrations/runner.go
apps/core-go/internal/migrations/runner_test.go
apps/core-go/internal/core/active_memory.go (new)
apps/core-go/internal/core/active_memory_lifecycle.go (new)
apps/core-go/internal/core/active_memory_test.go (new)
apps/core-go/internal/core/context_reference.go
apps/core-go/internal/core/provider_context.go
apps/core-go/internal/core/provider_context_test.go
apps/core-go/internal/core/reflection_v2.go
apps/core-go/internal/core/reflection_memory.go
apps/core-go/internal/core/reflection_runtime_v2.go
apps/core-go/internal/core/reflection_memory_test.go
apps/core-go/internal/core/provider_schemas.go
apps/core-go/internal/core/provider_schemas_test.go (new)
apps/core-go/internal/core/builtin_capabilities.go
apps/core-go/internal/core/capability_core_test.go
apps/core-go/internal/core/tool_contract_test.go
```

No file outside the approved S03 allowlist was modified during S03. Existing S02 changes remain present. The protected `.trellis/tasks/09-08-project-health-evolution/` directory was not read for implementation, modified, formatted, removed, or staged.

### Implemented

1. Extended unreleased `0032_prompt_context_memory` with independent `active_memories`, `active_memory_revisions`, and `active_memory_commands` tables. Durable `memories` remains unchanged and authoritative only for the existing four long-term types.
2. Added closed database constraints for `future_event|commitment|temporary_context`, `active|completed|expired|superseded`, six lifecycle operations, confidence/importance, JSON shapes, time precision/range, closed-state semantics, full FK provenance, partial active canonical identity, owner/idempotency replay, and owner/conversation/source/revision indexes.
3. Implemented the single production lifecycle writer `applyActiveMemoryCommandTx` with create/confirm/revise/complete/expire/supersede, owner lock, expected-revision CAS, deterministic IDs, exact-duplicate no-change, request-digest conflict detection, command replay, full revision snapshots, frozen command audit, and transactional outbox.
4. Kept mutation actor and semantic actor refs distinct: the direct writer is the Fluctlight; the current frozen speaker is Core-owned `actor_refs`. Neither field is Provider-owned.
5. Added Core-only bounded retrieval with owner/conversation/status/time hard filters. `valid_from` not reached and `valid_until` elapsed rows are removed before ranking. Ranking emits importance, confidence, lexical relevance, temporal urgency, critical-window, and last-relevant recency components plus selected/dropped reasons.
6. Added `active_memory` opaque ContextReference kind and target validation. Provider compaction exposes only ref, kind/content, active status semantics, confidence/importance, original/absolute time semantics, precision, and timezone; raw IDs, revisions, owner/scope, evidence, source facts, request digests, and traces stay Core-only.
7. Registered `active_memory_event` as the twelfth built-in Thin ACTION with `native_projection`, `transactional`, and `optional_internal`. Provider input contains only semantic operation/target/time fields. Prepare resolves the frozen owner, conversation, speaker, source fact occurrence time, timezone, evidence, action/call identity, expected revision, idempotency, and digest. Non-transactional execution is rejected.
8. Added strict offset-vs-IANA-timezone, order, precision, and 366-day horizon checks. Core never parses relative-language keywords; `unknown` requires both absolute anchors to remain absent.
9. Added independent `EvolutionActiveMemory` and closed `active_memory_candidates` to Reflection V2. Reflection sees bounded active semantics and source occurrence timestamps; accepted Active and durable Memory commands apply in the same caller-owned PostgreSQL transaction before proposal/watermark persistence. Candidate results reconcile back into dispositions without converting Active Memory into a durable Memory type.
10. Reflection reference capacity is bounded before adding Active refs. S03 does not add an Active field to `ContextProjection` or inject Active Memory into ordinary Main prompts; that production assembly cutover remains owned by S07 as planned.
11. Authored PostgreSQL cases for all six operations, replay/digest conflict, exact revision CAS, full revision/command audit, read-time expiry, supersede lineage, and the direct frozen Thin capability path. The direct path test constructs a single Main-produced invocation and proves preparation/application requires no second Memory Provider.

### Targeted tests run

```text
GOCACHE=/tmp/local-ai-companion-s03-gocache go test ./internal/core \
  -run '^(TestActiveMemoryCapabilityDefinitionIsThinOptionalTransactionalAction|TestActiveMemorySemanticValidationPreservesUnknownTimeAndChecksOffset|TestActiveMemoryPreparedCommandDigestFreezesSourceTime|TestActiveMemoryOpaqueReferenceAndProviderCompaction|TestActiveMemoryRankingPrioritizesCriticalRelevantFact|TestActiveMemoryLifecycleStaticGuardKeepsOneProductionSQLAuthority|TestCompileReflectionActiveMemoryCommandsKeepsSeparateDomainAndFrozenIndex|TestCompileReflectionPlanTreatsActiveMemoryAsIndependentDomain|TestReflectionProviderSchemaClosesActiveMemoryCandidates|TestDecodeReflectionProposalV2AcceptsActiveMemoryAndRejectsUnknownFields|TestProviderMetadataStripsRawActiveMemoryIdentifiers|TestBuiltinRegistryContainsExactlyTwelveDirectCapabilities|TestBuiltInCapabilityExecutionClassesUseGenericMetadataAndInterfaces|TestAllBuiltinDefinitionsExposeExpectedThinRequiredInputs|TestAllBuiltinDefinitionsAcceptMinimalProviderInput|TestOptionalToolFailuresDoNotAbortConversation|TestOperationSpecificResponseSchemasRequireTheirDomainShape)$' \
  -count=1
PASS

GOCACHE=/tmp/local-ai-companion-s03-gocache go test ./internal/migrations \
  -run '^(TestPromptContextMemoryMigrationAddsRawHistorySourceContract|TestPromptContextMemoryMigrationAddsIndependentActiveMemoryAuthority)$' \
  -count=1
PASS
```

The Core command compiled all S03 production and test files while executing only the exact pure/unit cases named above. The migration command executed only two exact string-contract tests. The recurring local warning `pyenv: cannot rehash: /Users/vinson/.pyenv/shims isn't writable` did not affect either result.

### Deferred to S12; not run in S03

- `TestPostgresActiveMemoryLifecycleReplayCASExpiryAndSupersede`
- `TestPostgresActiveMemoryEventFreezesRuntimeAuthorityWithoutMemoryProvider`
- All previously deferred S02 PostgreSQL tests
- Full package/repository tests, race, vet/build, Temporal, Docker Compose, and Live Provider gates

No PostgreSQL URL was used in S03. No Provider endpoint was called.

### Final targeted checks

- Targeted `gofmt -l` over only S03 Go files: no output.
- Targeted `git diff --check` over tracked S03 files: no output.
- Trailing-whitespace scan over the four new S03 files: no output.
- Static unit guard confirms no production Active Memory lifecycle `INSERT`/`UPDATE` exists outside `active_memory_lifecycle.go`.

### S03 final hashes

```text
518a476945ed27bf553cbf344daa7883425fe9cf2c62d014fd29564183a33d54  apps/core-go/internal/migrations/runner.go
bce611b9834ae27e002364054404b14e60633a04db180c193969a93811847ac0  apps/core-go/internal/migrations/runner_test.go
b9194fadf7f367fa42f5929f2e73226f95ad6731077a8914e8eb84542ff2836a  apps/core-go/internal/core/active_memory.go
df20c2417217ff338970cdc2eb67d187ee761aeba8caad64c1bff61d1cd8a3ad  apps/core-go/internal/core/active_memory_lifecycle.go
23661f5c55c6fce388e837e33c1761c3381e906886917fc742b46b9aee5c0786  apps/core-go/internal/core/active_memory_test.go
2033fe3f5bd2484954716d1ae1eae458d04d139ff0cf26475cd019dfe2dd5df0  apps/core-go/internal/core/context_reference.go
51ea842ba309b9bd303bfcf1afe6899fc5b4f80e13c0790a23f2cbffcdb676a5  apps/core-go/internal/core/provider_context.go
7220e4fad5c6686e472d97a34a3cee2d11e9042116b8c03bfbd34e9c90348522  apps/core-go/internal/core/provider_context_test.go
27f0d35c278077ef851576cc4fe3c5dbd4da846712661bc1154eff524fab3be3  apps/core-go/internal/core/reflection_v2.go
45cf26be03652e1756819d3661472ccd0a1ae226277cc87de9ce04aacfe15446  apps/core-go/internal/core/reflection_memory.go
70cd92479fb28ae08541ddf6b642857905e79f50f2f6ff2be8bf5262f61ea2aa  apps/core-go/internal/core/reflection_runtime_v2.go
930187fd4507198e0b8e5c3a8e2a96bd48326cc717a1ca1c59600657929eca90  apps/core-go/internal/core/reflection_memory_test.go
7ca2402dcc16261706124383992df780c683820c534f2094bedacfb128d981cb  apps/core-go/internal/core/provider_schemas.go
e65d7b007172b2e9fe0183e8eefebe04410daee063292960c7bbb7c49d855133  apps/core-go/internal/core/provider_schemas_test.go
3f977c83bacee78057b483e46613a48275b13266b6b8cd55db1c957bd53b6d49  apps/core-go/internal/core/builtin_capabilities.go
27e7c8896d8cc0ed9054f09cc3dc5493310c5fadfd10f5f13d079b31efa7b753  apps/core-go/internal/core/capability_core_test.go
4c00a298b0fdf998534a0d4afdec90aefec9a36a967f63858252ae8aef558d20  apps/core-go/internal/core/tool_contract_test.go
```

### S04 entry condition

- Active Memory authority is complete but intentionally not injected into the ordinary production prompt yet.
- S04 may modify only its Conversation Summary allowlist plus this task's checklist/evidence.
- S04 must continue exact-test-only verification; PostgreSQL/Temporal/full gates remain deferred to S12.

## S04 — Conversation Summary Projection

Status: complete; pure Core/migration/Temporal workflow contracts passed. PostgreSQL summary lifecycle and fake-Provider cases were authored and compiled but are deferred to S12.

### Modified files

```text
apps/core-go/internal/migrations/runner.go
apps/core-go/internal/migrations/runner_test.go
apps/core-go/internal/core/conversation_summary.go (new)
apps/core-go/internal/core/conversation_summary_test.go (new)
apps/core-go/internal/core/mutations.go
apps/core-go/internal/core/cognition.go
apps/core-go/internal/core/workflow_ops.go
apps/core-go/internal/core/provider_schemas.go
apps/core-go/internal/core/provider_schemas_test.go
apps/core-go/internal/workflow/runtime.go
apps/core-go/internal/workflow/workflow.go
apps/core-go/internal/workflow/workflow_test.go
```

`provider.go` was inspected but did not require modification because its existing `StructuredWithSchema`, stable Provider correlation/idempotency headers, explicit operation schema, and reflection-role binding already satisfy S04. No file outside the S04 allowlist was modified during S04. Earlier S02/S03 changes remain present. The protected `09-08` task directory remains untouched.

### Implemented

1. Extended unreleased `0032_prompt_context_memory` with `conversation_summaries`, immutable source windows/refs/digest, `active|superseded` projection state, per-window revision lineage, Provider/model/prompt/schema/policy provenance, request digest/idempotency, deferred FKs, an active-window uniqueness constraint, and current/source indexes.
2. Added deterministic old-chunk selection after assistant settlement. The current/latest 24 messages remain Recent; a summary intent appears only when the older stable range reaches 20 completed assistant turns, 40 whole messages, or about 6000 conservatively estimated tokens. Selection always ends at an assistant boundary.
3. Enqueued `conversation.summary` in the same caller-owned transaction as direct assistant/frozen-action/inbox completion. Normal and assistant-row crash-recovery paths share `completeTurnCognitionTx`; proactive assistant settlement calls the same helper. Duplicate-suppressed proactive messages do not enqueue a new job.
4. Window identity is derived from owner Fluctlight, conversation, exact from/to sequence, and source digest. Repeated assistant settlement while a chunk is pending writes the same intent; once an active chunk advances coverage, the next stable chunk receives a new non-overlapping identity.
5. The worker revalidates the triggering assistant identity, reads only the frozen `[from_sequence,to_sequence]` range, and recomputes ordered source refs/digest. Later messages cannot leak into a delayed/retried request. Raw rows are never deleted or rewritten by production summary code.
6. Added a closed `conversation-summary.v1` Provider response schema containing only `schema_version` and `summary`. The existing reflection model role is reused with an explicit operation schema and stable scenario/correlation. Provider input contains raw bounded message semantics only—never an old summary, database/message IDs, author IDs, source digest, or request metadata.
7. Provider I/O runs outside PostgreSQL transactions. The request/result path rechecks model assignment, then opens a short settlement transaction, locks the conversation, revalidates source digest, replays an already committed identical projection, supersedes an exact-range rebuild, or rejects a gap/overlap as stale.
8. Added bounded matching-conversation summary retrieval. Whole summary items are selected under a hard rune budget, newest source windows rank first, output uses `summary:ctx_<digest>` and `summary_projection`, and Core-only trace records raw summary identity plus selected/dropped budget reason.
9. Added `ConversationSummaryWorkflow` and activity on the existing lifecycle queue, reused `registerWorkflowControl`, and wired all three required registries: restart lookup, worker registration, and dispatcher switch. No second scheduler, queue runtime, or direct Temporal call was introduced.

### Targeted tests run

```text
GOCACHE=/tmp/local-ai-companion-s04-gocache go test ./internal/core \
  -run '^(TestConversationSummaryChunkRequiresThresholdAndEndsAtCompletedTurn|TestConversationSummaryProviderInputUsesOnlyRawBoundedMessages|TestConversationSummaryResponseIsStrictAndBounded|TestConversationSummaryStaticGuardKeepsProjectionWritesInOneModule|TestConversationSummaryProviderSchemaIsClosedAndSemanticOnly)$' \
  -count=1
PASS

GOCACHE=/tmp/local-ai-companion-s04-gocache go test ./internal/migrations \
  -run '^TestPromptContextMemoryMigrationAddsSourceBoundedConversationSummaryProjection$' \
  -count=1
PASS

GOCACHE=/tmp/local-ai-companion-s04-gocache go test ./internal/workflow \
  -run '^(TestWorkflowFunctionRegistryIncludesPlatformBoundaries|TestConversationSummaryWorkflowExecutesOneSourceBoundedActivity|TestConversationSummaryWorkflowRejectsInvalidWindowBeforeActivity)$' \
  -count=1
PASS
```

The first workflow assertion initially compared Temporal-decoded numeric values using concrete Go `int` identity and failed even though the values were `1` and `40`. The assertion was corrected to compare their deterministic textual numeric representation; no production code changed for that test-only mismatch. All three exact commands then passed.

The recurring `pyenv: cannot rehash: /Users/vinson/.pyenv/shims isn't writable` warning did not affect results.

### Deferred to S12; not run in S04

- `TestPostgresConversationSummaryIntentIsStableChunkedAndRawPreserving`
- `TestPostgresConversationSummaryProviderFailureRetryAndCommittedReplay`
- `TestPostgresConversationSummaryRebuildSupersedesAndOverlapCASFailsClosed`
- All previously deferred S02/S03 PostgreSQL tests
- Full package/repository tests, race, vet/build, Docker Compose, Live Provider, and full Temporal integration/replay gates

The deferred cases cover stable repeated enqueue, multi-chunk cursor progression, Recent reserve, whole-item summary budget, opaque refs, Raw row preservation, bounded Provider input, Provider failure→retry, committed replay without another Provider call, source drift before Provider, rebuild superseding lineage, and overlapping-window CAS rejection.

No PostgreSQL URL was used and no Provider endpoint was called in S04.

### Final targeted checks

- Targeted `gofmt -l` over only S04 Go files: no output.
- Targeted `git diff --check` over tracked S04 files: no output.
- Trailing-whitespace scan over the two new S04 files: no output.
- Static pure test confirms production `conversation_summaries` INSERT/UPDATE statements remain in `conversation_summary.go` only.

### S04 final hashes

```text
0a3e84ea46cc5dfd2176b019a87aff1b565dcfb91f52477e601e5843047a70d8  apps/core-go/internal/core/conversation_summary.go
44b9e22f53d4ebb7a4fc7ebb04f3a196df8032a920ef1bb02bf1421b4bcbb025  apps/core-go/internal/core/conversation_summary_test.go
776a1536343cb589a84a5657a3bb7a0978cbd254803ce6eca54500579efe4531  apps/core-go/internal/core/cognition.go
858f54d3c929584c5c02fba3c4afe1df20792f973ddc00cbdd0c543e4b0c43cc  apps/core-go/internal/core/mutations.go
02d63fbf4717a78abb917b796315a36301668e60ad081cbb93e743bb2f1c3551  apps/core-go/internal/core/workflow_ops.go
1ac057d57e37212a107a59d52d9882f5a9f627c32f76c807794d6df36ad2990a  apps/core-go/internal/core/provider_schemas.go
535024dbbe93f45e12d546929c9d9928c60b749239a2b0786a1d12f851c3d5ce  apps/core-go/internal/core/provider_schemas_test.go
de2b39c9126ed75ac4377e7fb938bb52126742b5acb960520aabbd361b12b2cc  apps/core-go/internal/migrations/runner.go
b5ba38797f429c052cf9f2bf6883fef056af31e923ec393eefb3cd3c617bdf6b  apps/core-go/internal/migrations/runner_test.go
1e8a8f60b353066917efac34c8c353fd57e00afff4d4059bae89d66117ba0af3  apps/core-go/internal/workflow/runtime.go
4e49ccf371ee0d79e14b44a45a20362543e4ed59b171507b0a15f3c8c1343655  apps/core-go/internal/workflow/workflow.go
378ed5a798c2896920e9f7bc4a615e28d45d7e1d034ec5f2dfa802a93f3bed96  apps/core-go/internal/workflow/workflow_test.go
```

### S05 entry condition

- Summary authority and async generation are complete, but ordinary production prompt assembly still uses the pre-cutover path.
- S05 may modify only its Working Memory/total-budget allowlist plus this task's checklist/evidence.
- PostgreSQL/Provider/Temporal/full gates remain deferred to S12.

## S05 — Working Memory and Total Prompt Budget

Status: complete; pure resolver/assembler/provider/migration contracts passed. The PostgreSQL role-configuration round-trip was authored and compiled but deferred to S12.

### Modified files

```text
apps/core-go/internal/core/working_memory.go (new)
apps/core-go/internal/core/working_memory_test.go (new)
apps/core-go/internal/core/prompt_context_assembler.go (new)
apps/core-go/internal/core/prompt_context_assembler_test.go (new)
apps/core-go/internal/core/provider.go
apps/core-go/internal/core/provider_test.go (new)
apps/core-go/internal/core/settings.go
apps/core-go/internal/core/operations.go
apps/core-go/internal/migrations/runner.go
apps/core-go/internal/migrations/runner_test.go
```

`provider_context.go`, `provider_context_test.go`, `provider_prompt_composer.go`, and `provider_prompt_composer_test.go` were inspected/formatted within the S05 allowlist but required no semantic edit. No file outside the S05 allowlist was modified. In particular, Gateway/Browser/UI DTOs discovered by read-only inventory were not touched; Core missing-field behavior preserves an existing role budget or uses canonical defaults.

### Implemented

1. Added the conservative estimator from design: `text_units=max(ceil(utf8_bytes/3),unicode_runes)`, then `ceil(text_units*1.25)`, with message/final-wire envelope overhead.
2. Added `PromptFragment`, `WorkingMemoryInput`, section policy, selected/dropped trace, and a pure `ResolveWorkingMemory` that never performs SQL, embedding, LLM calls, or source mutation.
3. Active → runtime facts → Recent → retrieved durable Memory → Summary authority ordering is deterministic. Source refs deduplicate across layers; every drop records `section_cap` or `deduplicated` without deleting the source.
4. Recent is selected newest-first by complete `GroupKey` turn, then restored to chronological transport order. Both section selection and assembler total-cap fallback treat a Recent turn as one atomic unit, so neither path can retain only one side of a turn.
5. Added pure B-layout assembly: one stable system with filtered Core Persona; one bounded `[RUNTIME CONTEXT]` user message for dynamic facts/Active/Retrieved/Summary; real selected `user`/`assistant` Recent messages; and final current `user` input exactly once.
6. Required System, rendered Tools+response schema, and Current Input have independent hard caps. Required overflow returns typed `prompt_required_budget_exceeded`; optional units are dropped whole. A final estimator pass over actual messages/tools/schema prevents upstream fragment estimates from crossing `max_input_tokens`.
7. Added the canonical role policy defaults `65536 context / 49152 max input / 4096 output reserve / 4096 safety`, preserving 8192 unused headroom. Unknown policy versions and invalid capacity formulas fail closed.
8. Extended fresh and additive `0032` `model_roles` with `context_window_tokens`, `max_input_tokens`, and `prompt_budget_policy_version`; added migration preflight plus database CHECK. Existing `token_budget` remains the single writable output-reserve authority.
9. Extended Provider assignment and Core settings/write paths with the new fields. `ProviderBindings` reports both legacy `token_budget` and derived `output_reserve_tokens`, plus safety/policy values. Role updates preserve existing new fields when legacy callers omit them.
10. Non-streaming and streaming chat payloads now both emit `max_tokens` from `token_budget`. Input estimation remains separate and never counts output reserve as prompt input.
11. Added 1000 Raw/Recent + 100 durable + 30 Active pressure fixtures proving selected Working Memory and final wire remain bounded while all source collections retain their original lengths.

### Targeted tests run

```text
GOCACHE=/tmp/local-ai-companion-s05-gocache go test ./internal/core \
  -run '^(TestWorkingMemorySelectsNewestWholeRecentTurnsChronologically|TestWorkingMemoryDeduplicatesAcrossAuthorityLayers|TestWorkingMemoryPressureIsBoundedWithoutDeletingSources|TestPromptEstimatorUsesConservativeUTF8Formula|TestPromptAssemblerBuildsBLayoutAndCurrentInputExactlyOnce|TestPromptAssemblerFailsWhenRequiredWireSectionsExceedCaps|TestPromptAssemblerTotalCapNeverSplitsRecentTurn|TestPromptAssemblerPressureDoesNotScaleWithStores|TestPromptBudgetConfigurationUsesOutputReserveAndSafetyMargin|TestProviderWirePayloadKeepsOutputReserveSeparateFromEstimatedInput|TestProviderStreamingPayloadHonorsOutputReserve)$' \
  -count=1
PASS

GOCACHE=/tmp/local-ai-companion-s05-gocache go test ./internal/migrations \
  -run '^TestPromptContextMemoryMigrationAddsValidatedModelPromptBudgets$' \
  -count=1
PASS
```

During the first Core compile, a missing closing brace at the end of the new assembler implementation was reported and fixed before the Core cases ran. The first atomic-turn cap fixture then used a deliberately tiny max-input value that also excluded the required system section; the fixture was corrected to leave required headroom while still forcing the optional turn to drop. Production behavior did not change for that fixture correction.

The recurring `pyenv: cannot rehash: /Users/vinson/.pyenv/shims isn't writable` warning did not affect results.

### Deferred to S12; not run in S05

- `TestPostgresProviderRolePromptBudgetRoundTripAndValidation`
- All S02–S04 deferred PostgreSQL/fake-Provider cases
- Full package/repository tests, race, vet/build, Docker Compose, Live Provider, and full Temporal gates

No PostgreSQL URL was used and no Provider endpoint was called in S05.

### Final targeted checks

- Targeted `gofmt -l` over only S05 Go files: no output.
- Targeted `git diff --check` over tracked S05 files: no output.
- Trailing-whitespace scan over five new S05 files: no output.

### S05 final hashes

```text
fa7d41b232d7acf259f140082128df62b83c1b7ac529abb91bd9702dd5a1ccd2  apps/core-go/internal/core/working_memory.go
102c9ba203d37f01c43b8fb871b4cdefc64e03b0bc5c96e623c4ce9688987441  apps/core-go/internal/core/working_memory_test.go
eb7231679d995cec5e6b0111e5437e3984c3cbeac122fa77c0e9d2a8dc66f639  apps/core-go/internal/core/prompt_context_assembler.go
4568fa52a9723be989554e6bd0b04623dd74cbbac59cf2448eb4f1b5fb0c8dac  apps/core-go/internal/core/prompt_context_assembler_test.go
38d10f106d2a785b2ddd90b5418b50c469dea14d6ab88ff2ec2c0c368877c564  apps/core-go/internal/core/provider.go
8a9729324d876dc904d01576aaad093b7a8917fd17c6a7e2bda67335a8bcb669  apps/core-go/internal/core/provider_test.go
e2bcd9f2f380f997537d0285809cbeedb1391d021544a8267ebf7328b664fdcd  apps/core-go/internal/core/settings.go
64b87576dcf91ec80b8a022dddc017098befa3c3f531cce88c16bb78a2280cd9  apps/core-go/internal/core/operations.go
8692caa1d31455d09a2a645529618866570b394a0354d77a115847fef74254ad  apps/core-go/internal/migrations/runner.go
4d371a3b12780377d3b451506ab8be8dda3562eade2c344737080eb558993264  apps/core-go/internal/migrations/runner_test.go
```

### S06 entry condition

- Working Memory and B-layout assembler are pure and tested but not yet called by production Provider paths.
- S06 owns the four-surface production cutover and removal of the old giant-user assembly path; only its explicit allowlist may be modified.
- All full/PostgreSQL/Temporal/Live Provider gates remain deferred to S12.

## S06 — B-layout Production Context Assembly Cutover

Status: complete; exact role/boundary/static tests passed. The opt-in live full-schema fixture was updated but not executed.

### Modified files

```text
apps/core-go/internal/core/mutations.go
apps/core-go/internal/core/cognition_growth.go
apps/core-go/internal/core/autonomy.go
apps/core-go/internal/core/wakeup.go
apps/core-go/internal/core/reflection_runtime_v2.go
apps/core-go/internal/core/provider.go
apps/core-go/internal/core/provider_context.go
apps/core-go/internal/core/provider_prompt_composer.go
apps/core-go/internal/core/provider_context_test.go
apps/core-go/internal/core/provider_prompt_composer_test.go
apps/core-go/internal/core/provider_live_tool_test.go
```

Other S06-allowlisted prompt/life test files were inspected/formatted but required no semantic change. No out-of-allowlist file was modified.

### Implemented

1. Conversation, native cognition, daily review, wake-up, and Reflection now call one `assembleProjectionPrompt` adapter and then `StructuredAssembledWithToolsSchema`. The adapter feeds the S05 resolver/assembler; `media_prompt` remains an explicit legacy-format bypass.
2. Production system messages now receive only runtime protocol, stable operation rules, and filtered Core Persona. Actor/relationship/current state/schedule/memory facts are emitted inside the delimited Runtime Context user message.
3. Recent Conversation is converted from authoritative message rows to true `user`/`assistant` transport messages. Sender and time remain bounded content headers; unknown/domain rows never become synthetic `tool` roles.
4. Current conversation text or operation input is the final user message exactly once. The old `{current_message,text,context}` envelope, dynamic relationship system wrapper, and Recent TOON document path were removed from all five Main callers.
5. The adapter obtains Active and source-bounded Summary projections, produces opaque refs, preserves the updated ContextReferenceIndex for frozen decisions, builds runtime/Active/Recent/Retrieved/Summary fragments, and uses the role's persisted total budget.
6. Tools remain rendered only by canonical `RenderCapabilityTools`; the assembler estimates that exact rendering and the exact strict response format. Provider renders the same definitions for wire transmission.
7. Added a preassembled Provider entrypoint which rejects media use, multiple/non-leading system messages, blank content, non-final current user messages, and any role other than system/user/assistant. It bypasses the old composer so B-layout cannot be flattened back into a giant document.
8. All non-media Provider requests receive a final wire estimate gate against `assignment.MaxInputTokens`, including the S04 Summary path. Thus simple role-specific operations share the same capacity policy even without a full ContextProjection.
9. Updated the opt-in live Provider tool test to build the selected B layout with the full cognitive response schema and canonical tool rendering. It remains gated by `FLUCTLIGHT_LIVE_PROVIDER_TEST=1` and was not run before S12.

### Targeted tests run

```text
GOCACHE=/tmp/local-ai-companion-s06-gocache go test ./internal/core \
  -run '^(TestProductionMainCallersUseOnlyPromptContextAssembler|TestAssembledProviderMessagesRequireOneSystemRealRolesAndFinalUser|TestRecentPromptFragmentsUseRealRolesAndSkipCurrentInput|TestWorkingMemoryProjectionKeepsRelationshipFactsOutOfPersona|TestQuotedHistoricalInstructionCannotBecomeSystemRule|TestPromptAssemblerBuildsBLayoutAndCurrentInputExactlyOnce|TestComposeProviderMessagesAlwaysEmitsOneLeadingSystem)$' \
  -count=1
PASS
```

The first relationship-runtime test fixture omitted the active profile/actor scope required by the existing relationship compactor, so it produced no relationship row. The fixture was corrected to a valid scoped projection; production code was unchanged. The recurring pyenv rehash warning did not affect the pass.

### Deferred to S12; not run in S06

- `TestLiveProviderRecognizesImageGenerationIntent` (now B-layout + full schema)
- All PostgreSQL/fake-Provider cases from S02–S05
- Full tests, race, vet/build, Docker Compose, full Temporal, and other Live Provider gates

No Provider endpoint, PostgreSQL integration database, Compose service, or full test command was used in S06.

### Final targeted checks

- Targeted `gofmt -l`: no output.
- Targeted `git diff --check`: no output.
- Static caller guard found the assembler/preassembled entrypoint in all five files and found none of the legacy Main fragments.

### S06 final hashes

```text
6d29b49efcde0de49f68a0b5dc61b8eb46483619d0cd9a92eb6387df5475f0f3  apps/core-go/internal/core/mutations.go
0264b2daa1ed5041c0b8b528385693f5a6c484bc9d81466025e18fdd929ea163  apps/core-go/internal/core/cognition_growth.go
ee2376ad426e1346ce259a83eb94f66761db8580a409ea5368c2ca945266422b  apps/core-go/internal/core/autonomy.go
72600e4e248b1fcb61b405268b4b4eed8042625d71586c57e75fabc3eee70da3  apps/core-go/internal/core/wakeup.go
84b2282bfdc7edf00799cfe949619b7f7cd65cded21c6db7f1d0d9b17d2ba156  apps/core-go/internal/core/reflection_runtime_v2.go
beceea7ae3c8fc6ca767b20973de839d5b4a7df1749f4570e40bd513deaff11f  apps/core-go/internal/core/provider.go
97a1c76544d99e7f412e23cd03df8b63f93edf4762dbe8ebe45a802dfc948a64  apps/core-go/internal/core/provider_context.go
07ab1d12ff0cdda18d1de423cae700cbfcef4e65feb3dc5b7fb6d03f18dbb657  apps/core-go/internal/core/provider_prompt_composer.go
620c87d551ffc137a6992a6f4418533e9bd84fbe0bcf291347f08b5e5673832e  apps/core-go/internal/core/provider_context_test.go
b2bb0f7e31eab50979e1010a461c6a58d5075e3a3fd222a8c731f0eb3e9f7962  apps/core-go/internal/core/provider_prompt_composer_test.go
670fae2fb8622844a202e127f4d666ef4c92dd7a2777c67b563bfd18b8092293  apps/core-go/internal/core/provider_live_tool_test.go
```

### S07 entry condition

- Production request assembly now uses B-layout and bounded Active/Summary/Recent/Durable projections.
- S07 may modify only Memory retrieval, ContextProjection, and provider-context files in its allowlist.
- Full/PostgreSQL/Live Provider gates remain deferred to S12.

## S07 — Automatic Retrieval and Memory Selection

Status: complete; exact cue/SQL/provider-boundary tests passed. The old-relevant PostgreSQL regression was authored and compiled but deferred to S12.

### Modified files

```text
apps/core-go/internal/core/memory_retrieval.go
apps/core-go/internal/core/memory_retrieval_test.go
apps/core-go/internal/core/intelligence.go
apps/core-go/internal/core/provider_context.go
```

No out-of-allowlist file was modified.

### Implemented

- `ContextProjection` now carries bounded Active Memory plus its Core-only rank trace. Active rows are loaded at the frozen projection time and receive opaque refs after the existing reference index is built.
- Recent candidate loading increased from the hardcoded 12-row effective boundary to repository-bounded 200 rows; Working Memory remains the actual token/whole-turn selector.
- Automatic cues now include current input, life context, current mood/PAD/drives/conflicts, last six Recent messages, selected Active content, and existing goals/intentions/outcomes/hypotheses.
- The query is capped at 1024 estimated tokens. Projection-derived cues forcibly disable embedding egress; explicit non-projection plans retain the authorized embedding seam.
- Authorized rows are ordered by OR-FTS match/rank, importance, creation time, and ID before `LIMIT 200`; owner/status/type/visibility/viewer/conversation predicates remain before candidate/vector ranking.
- Per-candidate trace records score components and selected/dropped reasons. Provider compaction continues to exclude trace and raw identities.

### Targeted tests run

```text
GOCACHE=/tmp/local-ai-companion-s07-gocache go test ./internal/core \
  -run '^(TestBuildMemoryQueryPlanSeparatesLocalCuesFromEmbeddingEgress|TestAutomaticMemoryCuesAreBoundedAndNeverEnableEmbedding|TestMemoryRetrievalSQLRanksAuthorizedRelevanceBeforeCandidateLimit|TestRecentPromptFragmentsUseRealRolesAndSkipCurrentInput|TestWorkingMemoryProjectionKeepsRelationshipFactsOutOfPersona)$' \
  -count=1
PASS
```

Deferred to S12: `TestPostgresMemoryRetrievalFindsOldRelevantBeyondRecentCandidateLimit`, existing scope/vector/embedding/ABA integrations, and all full gates. No PostgreSQL, Provider, full, race, vet, build, Compose, or Temporal command ran in S07.

Targeted `gofmt -l` and `git diff --check` produced no output. Final hashes:

```text
45ea13df7c3ea902fbe1459a8e0be1bfef439ed5343a25bda911108afaf82fee  apps/core-go/internal/core/memory_retrieval.go
b646830749e05d2adc41bdc32596a791db784db4dfe775322421873b40e0edb0  apps/core-go/internal/core/memory_retrieval_test.go
04807746f698abe1686434acdd2cdd6d0df0463f7862bc935f1585fa53d75d49  apps/core-go/internal/core/intelligence.go
90d8446235d7d3ba13501b840f3d800cfd82c77b536fe7b38c0261fd318eba80  apps/core-go/internal/core/provider_context.go
```

### S08 entry condition

- Automatic Retrieval is bounded and production assembly consumes its selected projection.
- S08 may modify only the explicit recall/built-in test allowlist plus task evidence; continuation remains S09.

## S08 — `memory.recall` Thin QUERY

Status: complete; exact definition/scope/service/catalog tests passed.

Implemented a conversation-only `memory.recall` with intent-only input, `SlotMemoryScope`, `read_only`, `parallel`, `optional_internal`, and generic pure-query classification. Its injected `MemoryRecallService` contains only narrow retrieval function fields/readers—not `*App`. Frozen MemoryScope supplies owner/viewer/conversation/profile authorization; the intent drives a new deep 50-candidate Long-term plan rather than returning the already-selected automatic Top-K.

The service combines Active, deep Long-term, authorized older message FTS, and source-bounded Summary results, ranks them, deduplicates opaque refs, and returns at most 12 whole items / 3072 estimated tokens. Provider output contains only opaque ref, source kind, semantic kind/content, bounded confidence/importance/time. Raw IDs, visibility, revisions, evidence, plan IDs and scores remain internal.

Built-ins now total 13. Exact schema stats recorded by `CapabilityToolSchemaStats`: all 6924 bytes/chars, conversation 6386 (11 definitions), native cognition 5414 (9 definitions). `docs/capability-schema-report.md` was updated accordingly.

```text
GOCACHE=/tmp/local-ai-companion-s08-gocache go test ./internal/core \
  -run '^(TestMemoryRecallDefinitionIsConversationOnlyPureQuery|TestMemoryRecallCapabilityUsesFrozenAuthorizationScope|TestMemoryRecallServiceCombinesDeepAuthoritiesWithOpaqueBoundedOutput|TestBuiltinRegistryContainsExactlyThirteenDirectCapabilities|TestBuiltInCapabilityExecutionClassesUseGenericMetadataAndInterfaces|TestOptionalToolFailuresDoNotAbortConversation|TestBuiltinCapabilitiesDoNotUseAppAsServiceLocator)$' -count=1
PASS

GOCACHE=/tmp/local-ai-companion-s08-gocache go test ./internal/core -run '^TestMemoryRecallCatalogSchemaStats$' -count=1 -v
PASS: all=6924/6924 conversation=6386/6386 native=5414/5414
```

No PostgreSQL, Provider, full, race, vet, build, Compose, or Temporal command ran. Targeted format/diff/whitespace checks produced no output.

Final new-file hashes:

```text
1a13b377e0f52d7a681edfbc2e588a458ac7b8cf7e85fd49bf56ac0ebe8bbbb2  apps/core-go/internal/core/memory_recall_capability.go
cba98b31092f3e4e246e48de9e27abb06b7707ac3a0b2e64a0838d30feb85a9b  apps/core-go/internal/core/memory_recall_capability_test.go
b3864a6bc7e6442d3bb36b4ba8e64c9ee864367d6359dba742cdba22661221ad  docs/capability-schema-report.md
```

### S09 entry condition

- Recall is executable as an ordinary pure query and remains useful to later cognition even without continuation.
- S09 alone owns the generic same-turn pure-query continuation exception and its guard changes.

## S09 — Generic Pure-QUERY Continuation Coordinator

Status: complete; exact coordinator/schema/guard tests passed. Full crash/concurrency/turn-supersession integration remains part of S12.

Implemented required Main `response_mode=final|query_continuation`. `final` preserves one-call behavior. Continuation is accepted only with no visible text, 1–2 invocations, and every capability classified through registry metadata/interfaces as `pure_query`; ACTION and mixed batches fail closed without a name special case.

The frozen action now carries a digest-bound `query-continuation.v1` state with `requested → queries_completed → provider_completed`, frozen B messages, canonical query results, and visible output. Prepare/query execution stays outside the assistant business transaction. Existing prepared invocation and capability-result persistence provide crash replay; completed query results are reused, and provider-completed text is reused without another call.

Canonical Invocation/Result pairs serialize to one assistant `tool_calls` message plus 1–2 `role=tool` results. A dedicated Provider entrypoint accepts only that shape, sends no definitions/tools, uses stable frozen correlation/idempotency, and accepts only closed `{visible_text}` output. Tool calls, claims, appraisal or other mutation fields in the second response are rejected. Superseded-fact checks fence the second Provider both before and after I/O.

Blanket no-tool-role guards were replaced with an allowlist for `query_continuation.go`; concrete capability-name dispatch remains forbidden.

```text
GOCACHE=/tmp/local-ai-companion-s09-gocache go test ./internal/core \
  -run '^(TestQueryContinuationAcceptsOnlyOneOrTwoGenericPureQueries|TestQueryContinuationStateAndToolMessagesAreReplayStable|TestQueryContinuationResponseSchemaIsVisibleTextOnlyAndResponseModeRequired|TestContinuationVisibleTextRejectsMutationFields|TestCapabilityRuntimeStaticGuardsPreserveActionSingleCognitionAndGenericQueryContinuation|TestProjectHealthArchitectureGuardAllowsToolRoleOnlyInQueryContinuation|TestOperationSpecificResponseSchemasRequireTheirDomainShape)$' -count=1
PASS
```

Initial test compilation exposed one malformed nested composite literal and one incorrect three-argument use of the existing two-argument `firstString`; both were corrected before the exact suite passed. No PostgreSQL, Provider, full, race, vet, build, Compose, or Temporal command ran. Targeted formatting/diff/whitespace checks produced no output.

New coordinator hashes:

```text
81d9c9a07efd68fc2ba93c329ecc7bb8e0e8fe6e1c3e29b4ece66571ea92cbbd  apps/core-go/internal/core/query_continuation.go
3921853d28f03b317d39df0b48a6a1861b129d1671c65bb02272c73f78678165  apps/core-go/internal/core/query_continuation_test.go
```

## S10 — Boundary Audit Before Write

Status: paused before S10 production write, pending exact allowlist correction.

Read-only inspection found that the current S10 allowlist (`diagnostics.go`, `provider.go`, migration/tests) can derive final message/tools/schema token counts and Provider usage, but cannot access the already-computed Working Memory/assembler dropped refs, Active/Long-term score components, Summary selection reasons, or the full prompt selection trace. Those values exist before the Provider call and are intentionally absent from final Provider messages; reconstructing them from prompt text would be lossy and would risk exposing raw IDs.

Required exact S10 allowlist addition:

```text
apps/core-go/internal/core/prompt_context_assembler.go
apps/core-go/internal/core/provider_context.go
apps/core-go/internal/core/mutations.go
apps/core-go/internal/core/cognition_growth.go
apps/core-go/internal/core/autonomy.go
apps/core-go/internal/core/wakeup.go
apps/core-go/internal/core/reflection_runtime_v2.go
```

Intended bounded change: add a Core-only diagnostic trace to `PromptAssemblyResult`; have `assembleProjectionPrompt` merge assembler/Working Memory/Active/Long-term/Summary selection traces; pass it through a diagnostic context value at the five S06 Provider call sites. Provider/diagnostics will persist redacted bounded metrics and normalized actual usage. No prompt content, capability behavior, retrieval decision, or external API shape changes are intended.

The user approved this exact seven-file S10 extension on 2026-09-12.

## S10 — Observability and Diagnostics

Status: complete; exact redaction/usage/API-boundary/migration tests passed.

The approved trace plumbing adds a Core-only `Diagnostics` payload to `PromptAssemblyResult`. `assembleProjectionPrompt` merges prompt-budget sections, Working Memory selected/dropped decisions, Active/Long-term rank components, Summary selection, and persona/conversation scope. Five assembled callers pass it via a private context value; Summary, Continuation, legacy structured operations, and streaming receive final-wire metrics synthesized inside Provider.

`diagnostic_model_runs` now stores nullable `fluctlight_id`, bounded redacted `metrics`, estimated input, actual prompt/completion tokens, and Provider latency. Provider `usage.prompt_tokens/completion_tokens/total_tokens` is normalized; actual-vs-estimated delta is computed when both exist. Tools and strict response schema have independent section counts/tokens. Continuation calls record `queries_completed` phase and are scoped back to their frozen action; Summary is scoped through its durable intent.

Detailed collections are capped at 64 entries and pass existing credential/reasoning redaction. Existing ordinary ModelRuns API does not select the new metrics or raw scope columns, while diagnostic clear/prune still includes model runs. All diagnostic writes remain best-effort/non-fatal.

```text
GOCACHE=/tmp/local-ai-companion-s10-gocache go test ./internal/core \
  -run '^(TestPromptDiagnosticsAreRedactedAndCollectionBounded|TestProviderUsageAndWireBudgetDiagnosticsNormalizeActuals|TestDetailedPromptMetricsStayOutOfOrdinaryModelRunsAPIAndRemainPrunable|TestProviderWirePayloadKeepsOutputReserveSeparateFromEstimatedInput|TestProviderStreamingPayloadHonorsOutputReserve)$' -count=1
PASS

GOCACHE=/tmp/local-ai-companion-s10-gocache go test ./internal/migrations \
  -run '^TestPromptContextMemoryMigrationAddsPromptDiagnosticsMetrics$' -count=1
PASS
```

The first API-boundary fixture searched for a non-existent `MediaDiagnostics` function name; it was corrected to the actual following function `MediaPromptsFiltered`. Production code was unchanged. No PostgreSQL, external Provider, full, race, vet, build, Compose, or Temporal command ran. Targeted formatting/diff/whitespace checks produced no output.

Primary hashes:

```text
455da92feee5dc2c0b9f608d1718c8bb40b8f2539a177924b8646a113ef1b90b  apps/core-go/internal/core/diagnostics.go
b81993f92d1396647bd071c1ddb0d1cbf26dd42876f6c570d1d7fc858d41cbba  apps/core-go/internal/core/diagnostics_test.go
f01bee7db39f86c993eaa795b74519b345e1894fc169f09a7d84b6f1cb66fb87  apps/core-go/internal/core/provider.go
65d0eb2979089e12f0e64f2e1d0d0fbd58897e95a0157bd73a93e87bd6a06fd9  apps/core-go/internal/migrations/runner.go
```

## S11 — Boundary Audit Before Cleanup

Status: paused before S11 production cleanup, pending one exact allowlist addition.

Repository-wide read-only contract search found one live runtime instruction outside the current S11 boundary:

```text
apps/core-go/internal/core/capability_prompt_policy.go
```

`capabilityConversationPolicyInstruction` still says every direct conversation “必须在同一次 Main cognition 中返回可见回复”. That contradicts the newly approved and implemented pure-query-only continuation and would discourage the model from selecting `response_mode=query_continuation`. Documentation-only updates cannot fix this runtime conflict.

Required S11 allowlist addition: the single production file above. Intended change: state that direct conversation defaults to final visible output in the Main response; only a result-dependent batch of one or two pure QUERY capabilities may omit visible text and request the one bounded continuation; ACTION or mixed batches must still provide visible output in the Main response. No schema, dispatch, capability, or persistence change is required.

The user approved this exact one-file S11 allowlist extension on 2026-09-12.

Further read-only S11 audit found that `validQueryContinuationMessages` rejected
ordinary content-bearing assistant messages in the frozen B-layout history,
because it required every assistant message to carry one or two `tool_calls`.
The existing unit fixture contained only system plus current user and did not
cover real recent assistant history. The same audit found stale broad
tool-result continuation statements in `shared-scene-contract.md` and
`error-handling.md`. The required bounded extension is exactly:

```text
apps/core-go/internal/core/query_continuation_test.go
.trellis/spec/backend/shared-scene-contract.md
.trellis/spec/backend/error-handling.md
```

The user approved this exact three-file S11 extension on 2026-09-12. The
production validator change remains in already-allowed `provider.go`: accept
ordinary nonempty assistant history before exactly one terminal assistant
tool-call envelope, while preserving the 1–2 pure-query result boundary. The two
spec files will be narrowed from broad/mutation continuation to the same
generic pure-QUERY-only exception.

## S11 — One-Authority Cleanup and Contract Updates

Status: complete. S01–S11 implementation and exact-stage tests are complete;
full acceptance remains unverified until S12.

The conversation prompt policy now states the real boundary: a direct
conversation defaults to a final visible response in the Main cognition; only
one or two result-dependent pure QUERY calls may request the single no-tools,
visible-text-only continuation. ACTION and QUERY+ACTION mixed batches remain
single-Main, and direct conversation cannot end with `no_op`.

The read-only audit also found and fixed a B-layout continuation bug. The
Provider validator previously rejected any ordinary recent assistant message
because it required every assistant role to contain `tool_calls`. It now accepts
nonempty ordinary assistant history before exactly one terminal assistant
tool-call envelope, rejects user/history messages after that envelope, requires
1–2 calls and the same number of nonempty tool results, and still rejects empty
assistant history. `TestQueryContinuationMessagesAllowBLayoutAssistantHistory`
is the regression fixture.

Contract sync completed in README, Capability architecture, and the Memory,
Provider, Cognitive, Structured Turn, Persistence, Workflow, Diagnostics,
Shared Scene, and Error Handling code-specs. Cross-layer specs now contain
executable scope/signatures, request/storage contracts, validation matrices,
good/base/bad cases, required tests, and wrong/correct examples for:

- Raw History logical sources and atomic turn linkage;
- independent Active Memory lifecycle and time semantics;
- source-bound Conversation Summary and lifecycle-only Temporal workflow;
- Working Memory whole-item/turn selection and B-layout Prompt Assembly;
- fixed model-role/final-wire budgets and conservative estimation;
- Automatic Retrieval and provider-safe `memory.recall`;
- generic pure-query-only continuation phases/replay/supersession;
- `0032_prompt_context_memory` migration and diagnostic prompt metrics.

One-authority audit results:

```text
durable Memory lifecycle DML       apps/core-go/internal/core/memory_lifecycle.go
Active Memory lifecycle DML        apps/core-go/internal/core/active_memory_lifecycle.go
Conversation Summary projection    apps/core-go/internal/core/conversation_summary.go
Capability Runtime                 App-owned registry/resolver/runtime; lazy helper caches the same App runtime
Temporal workers                   one worker.New loop for lifecycle/media/interaction queues
production Main prompt callers     assembleProjectionPrompt -> StructuredAssembledWithToolsSchema
role A/C production flag           none
```

Migration DDL in `runner.go` and the narrowly scoped embedding materialization
updates in `memory_embedding.go` are intentional, not competing lifecycle
authorities. The legacy `withActorRelationshipSystemContext` helper and generic
non-preassembled composer remain source-compatible for non-Main/legacy tests,
but all five production Main callers are statically guarded against them. The
removed/replaced Main paths are the giant `{current_message,text,context}` user
document, duplicated current input, Recent-as-one-user-table, dynamic
relationship/state facts in system, fixed-12 as the prompt boundary, and
caller-local Main assembly.

Recorded architecture values:

```text
migration head                  0032_prompt_context_memory
context window                 65536
max input                      49152
output reserve                 4096
safety margin                  4096
unused headroom                8192
budget policy                  prompt-budget.v1
catalog all                    13 definitions, 6924 bytes/chars
catalog conversation           11 definitions, 6386 bytes/chars
catalog native cognition       9 definitions, 5414 bytes/chars
role experiment                B selected after two runs / 36 real local calls
```

Exact S11 validation:

```text
GOCACHE=/tmp/local-ai-companion-s11-gocache go -C apps/core-go test ./internal/core \
  -run '^(TestProviderPromptInstructionsStayCompactAndPreserveContracts|TestQueryContinuationStateAndToolMessagesAreReplayStable|TestQueryContinuationMessagesAllowBLayoutAssistantHistory|TestProductionMainCallersUseOnlyPromptContextAssembler|TestActiveMemoryLifecycleStaticGuardKeepsOneProductionSQLAuthority|TestMemoryLifecycleStaticGuardKeepsOneProductionSQLAuthority|TestConversationSummaryStaticGuardKeepsProjectionWritesInOneModule|TestCapabilityRuntimeStaticGuardsPreserveActionSingleCognitionAndGenericQueryContinuation|TestProjectHealthArchitectureGuardAllowsToolRoleOnlyInQueryContinuation)$' -count=1
PASS

GOCACHE=/tmp/local-ai-companion-s11-gocache go -C apps/core-go test ./internal/workflow \
  -run '^(TestWorkflowFunctionRegistryIncludesPlatformBoundaries|TestConversationSummaryWorkflowExecutesOneSourceBoundedActivity|TestConversationSummaryWorkflowRejectsInvalidWindowBeforeActivity)$' -count=1
PASS

gofmt -l <three S11 Go files>
NO OUTPUT

git diff --check -- <S11 allowlist files>
NO OUTPUT
```

S11 acceptance-to-evidence map:

| PRD acceptance | Implemented evidence | Final acceptance state |
| --- | --- | --- |
| AC1 Raw growth / bounded prompt | S02 Raw pagination + S05 store-pressure budget tests | S12 PostgreSQL/full gate pending |
| AC2 flight survives Recent | S03 Active selection + S05 Working Memory tests | S12 Provider/PostgreSQL gate pending |
| AC3 expiry/provenance | S03 read-time expiry/lifecycle tests | S12 PostgreSQL gate pending |
| AC4 relevant Long-term only | S07 authorization/older-result unit tests | S12 PostgreSQL gate pending |
| AC5 ABA current lineage | S03/S07 deterministic lineage tests | S12 integration gate pending |
| AC6 Summary rebuild/no Raw delete | S04 source/digest/rebuild tests | S12 PostgreSQL/Temporal gate pending |
| AC7 fixed total budget | S05 assembler stress + S10 diagnostics tests | S12 full Provider gate pending |
| AC8 rule/fact boundary | S06 B-layout/static prompt tests | S12 live Provider gate pending |
| AC9 real roles/current once | S06 role/current-input tests | S12 live Provider gate pending |
| AC10 real A/B/C experiment | S00 two-run 36-call evidence; B selected | S12 full-schema live recheck pending |
| AC11 Automatic + recall + bounded continuation | S07/S08/S09 exact tests + S11 assistant-history regression | S12 integration/live gate pending |
| AC12 timely bounded observation | S03 same-Main candidate + S04 chunked async Summary tests | S12 Provider/Temporal gate pending |
| AC13 one assembly authority | S06 cutover + S11 static/`rg` guards | S12 full architecture gate pending |
| AC14 full quality gate | Not yet run by strict stage rule | S12 pending |
| AC15 final 13-part report | Task evidence is accumulating | S13 pending |

No full, race, vet, build, PostgreSQL, Live Provider, Compose, or cross-stage
acceptance command ran during S11. The unrelated untracked
`.trellis/tasks/09-08-project-health-evolution/` directory remains untouched.

Final S11 hashes:

```text
410b299f71ec6ba144f567c49706a3e82cf0c35cb70e24291a856d1164d3d1f8  capability_prompt_policy.go
cc7e8988729b62d0c49fcf9e1a1eccabbf85cdb60cef29aafb576ec1d4c68ff3  provider.go
2f2637eedfcb24298661818cfa5b12f0a28106be8c522ecbf914bd10f94a3225  query_continuation_test.go
```

## S12 — Full Verification, Pass 1

Core and Gateway non-database gates passed:

```text
go -C apps/core-go test ./... -count=1                         PASS
go -C apps/core-go test -race ./... -count=1                   PASS
go -C apps/core-go vet ./...                                   PASS
go -C apps/core-go build ./...                                 PASS
go -C apps/gateway-go test ./... -count=1                      PASS
go -C apps/gateway-go test -race ./... -count=1                PASS
go -C apps/gateway-go vet ./...                                PASS
go -C apps/gateway-go build ./...                              PASS
```

The first sandboxed Core/Gateway test attempts failed only because the sandbox
forbids local `httptest`/Redis test listeners; the same exact commands passed
outside the sandbox. The first sandboxed vet attempt could not write the user
Go build cache; the exact command passed outside the sandbox.

A dedicated disposable `pgvector/pgvector:pg16` container named
`lac-prompt-memory-pg-0912` was started on random host port 32768. An explicit
empty database migrated to `0032_prompt_context_memory` twice successfully and
was dropped. Isolated `0031 -> 0032` plus duplicate conversation/cognition
sequence rollback tests passed.

The first database-enabled Core package run found the following before later
tests were stopped by a panic:

| Owning stage | Failure | Status before fix |
| --- | --- | --- |
| Environment gate | Direct-URL legacy PostgreSQL tests saw an unmigrated dedicated `postgres` base database | Migrate only the disposable base DB, then re-run exact tests |
| S02 | `TestDirectConversationSourceFactConflictRollsBackNewUserMessage` and `TestPostgresRawHistoryReaderCombinesAuthoritiesAndSearchesMessages`: pgx rejected multi-command prepared statements | Return to S02 test allowlist |
| S04 | `TestPostgresConversationSummaryProviderFailureRetryAndCommittedReplay`: Provider calls remained 0; rebuild test failed `conversation_summary_source_drift` | Return to S04 code/test allowlist after inspection |
| S07 | `TestPostgresMemoryRetrievalFindsOldRelevantBeyondRecentCandidateLimit`: PostgreSQL could not infer parameter `$2` | Return to S07 retrieval allowlist |
| S10 | `TestSchedulePlannerProviderRequestUsesOpaqueAllowlist`: panic assigning final wire diagnostics into a nil map at `provider.go` | Return to S10 Provider allowlist |

No failure was repaired opportunistically in S12. The next steps are exact
environment recheck, then owning-stage fixes and exact tests before re-entering
the full S12 gate.

### S12 remediation: return to S02

The two S02 PostgreSQL fixtures sent several semicolon-separated commands with
parameters through one pgx extended-protocol `Exec`, which PostgreSQL rejects as
`cannot insert multiple commands into a prepared statement`. The fixture setup
was split into explicit single-statement executions; production code and schema
were unchanged.

```text
GO_CORE_TEST_DATABASE_URL=<dedicated disposable PostgreSQL> \
GOCACHE=/tmp/lac-prompt-memory-s02-pg-fix \
go -C apps/core-go test ./internal/core \
  -run '^(TestDirectConversationSourceFactConflictRollsBackNewUserMessage|TestPostgresRawHistoryReaderCombinesAuthoritiesAndSearchesMessages)$' -count=1 -v
PASS
```

The dedicated container's `postgres` base database was then migrated to
`0032_prompt_context_memory` so older direct-URL integration tests see the
required schema. This changes only the disposable validation container.

### S12 remediation: return to S04 (first pass)

`conversation.summary` stores ordered source refs, but
`validateConversationSummaryWorkIdentity` and its PostgreSQL fixtures used
`decisionServiceRefValues`, a generic set helper that sorts strings. For ranges
containing `message-10`, lexical sorting differs from source sequence order:
production workflow input failed `conversation_summary_intent_conflict`, while
the sorted test input passed identity validation and then failed the strict
source-order check as `conversation_summary_source_drift`.

S04 now has a dedicated order-preserving `conversationSummarySourceRefValues`;
identity compares ordered slices, and fixtures no longer sort source evidence.
`TestConversationSummarySourceRefsPreserveSequenceOrder` passed. The two
PostgreSQL Summary tests then progressed to Provider invocation and exposed the
separate S10 nil prompt-diagnostics panic. S04 remains pending until that
dependency is fixed and both exact PostgreSQL tests pass.

### S12 remediation: return to S10

`providerPromptDiagnostics` cloned a missing context value through JSON. JSON
`null` resets the preallocated destination map to nil, and
`completeWithToolsSchemaMode` then assigned `prompt_budget` into that nil map.
The helper now guarantees a fresh mutable empty map for nil/missing context.
This keeps the Provider path best-effort and avoids requiring every caller to
seed diagnostic state.

```text
GO_CORE_TEST_DATABASE_URL=<dedicated disposable PostgreSQL> \
GOCACHE=/tmp/lac-prompt-memory-s10-nil-fix \
go -C apps/core-go test ./internal/core \
  -run '^(TestPromptDiagnosticsAlwaysReturnsMutableMap|TestSchedulePlannerProviderRequestUsesOpaqueAllowlist)$' -count=1 -v
PASS
```

### S12 remediation: complete S04

After the S10 dependency fix, Summary retry/replay reached its final assertion,
which omitted the `$1` `conversationID` argument from a test-only `QueryRow`.
The fixture now supplies the argument. Ordered identity, first Provider failure,
stable retry, committed replay without a third Provider call, source drift
before Provider, rebuild revision/supersession, and overlapping projection CAS
all pass.

```text
GO_CORE_TEST_DATABASE_URL=<dedicated disposable PostgreSQL> \
GOCACHE=/tmp/lac-prompt-memory-s04-pg-fix \
go -C apps/core-go test ./internal/core \
  -run '^(TestConversationSummarySourceRefsPreserveSequenceOrder|TestPostgresConversationSummaryProviderFailureRetryAndCommittedReplay|TestPostgresConversationSummaryRebuildSupersedesAndOverlapCASFailsClosed)$' -count=1 -v
PASS
```

### S12 remediation: return to S07

The old-relevant PostgreSQL regression first failed inside its seed SQL because
`jsonb_build_array($2)` gives PostgreSQL no concrete type for a polymorphic
parameter. Both fixture occurrences now use `$2::text`. The production FTS
LATERAL query also explicitly casts its repeated `$2` to text before
`to_tsvector`, preventing the same extended-protocol ambiguity when the actual
retrieval path runs.

```text
GO_CORE_TEST_DATABASE_URL=<dedicated disposable PostgreSQL> \
GOCACHE=/tmp/lac-prompt-memory-s07-pg-fix \
go -C apps/core-go test ./internal/core \
  -run '^(TestMemoryRetrievalSQLRanksAuthorizedRelevanceBeforeCandidateLimit|TestPostgresMemoryRetrievalFindsOldRelevantBeyondRecentCandidateLimit)$' -count=1 -v
PASS
```

All Pass-1 failures have now returned to and passed their owning-stage exact
tests. S12 restarts from the affected full/database gates; earlier successful
Core/Gateway non-database commands remain recorded but will be rerun after all
database fixes are green as part of the final clean pass.

### S12 PostgreSQL pass 2

After the four owning-stage remediations, the database-enabled full Core package
and complete migrations package passed against the dedicated disposable
PostgreSQL container:

```text
GO_CORE_TEST_DATABASE_URL=<dedicated disposable PostgreSQL> \
go -C apps/core-go test ./internal/core -count=1
PASS (60.255s)

GO_CORE_TEST_DATABASE_URL=<dedicated disposable PostgreSQL> \
go -C apps/core-go test ./internal/migrations -count=1
PASS (26.867s)
```

This executed the previously deferred Raw/Active/Summary/retrieval/provider-role
cases plus existing PostgreSQL integration tests; Live Provider remained
explicitly skipped because its separate flag was not set.

### S12 remediation: return to S06 for missing live gates

The plan's required live command targets
`TestLiveProvider(RoleOrganization|ActiveMemory|RecallContinuation)`, but the
allowed live fixture file contained only the older image-generation test. A
matching regex would therefore pass with no tests run. S06 is reopened only for
`provider_live_tool_test.go` to add the three named, opt-in real local Provider
tests using the production B-layout/full schema. No production code is changed
for this remediation.

The first real-model run produced:

```text
TestLiveProviderRoleOrganization  PASS (96.37s)
TestLiveProviderActiveMemory      model selected active_memory_event/create,
                                  future_event, original expression and bounded
                                  absolute times; no structured sidecar
TestLiveProviderRecallContinuation model selected memory.recall; no structured
                                   sidecar
```

mlx-serve returned native `tool_calls` with `content=null` and natural-language
`reasoning_content`, despite the strict response format. The production adapter
correctly treats reasoning as non-visible and creates `StructuredFallback`, but
the conversation path defaulted its empty `response_mode` to `final`. That makes
a real provider-selected pure QUERY fail `cognition_visible_text_missing`
instead of entering the approved continuation. S09 is reopened for a generic
transport normalization: only structured fallback + no visible text + 1–2
registry-classified pure QUERY calls may normalize the missing mode to
`query_continuation`. Explicit modes, ACTION, mixed batches, and non-fallback
schema omissions do not use this compatibility path.

## Historical S11 — Second Boundary Audit Before Contract Write (resolved)

Status: resolved; the user approved the exact three-file extension and the validator/test/contracts were updated in S11.

Read-only inspection of the dedicated continuation Provider validator found a B-layout compatibility defect. `validQueryContinuationMessages` currently requires every historical `assistant` message to contain `tool_calls`. A frozen B-layout context may legitimately contain ordinary non-empty assistant history before the one appended canonical assistant tool-call envelope, so a result-dependent pure-query continuation with recent assistant history is rejected as `provider_assembled_messages_invalid`. `provider.go` is already inside the S11 production allowlist; the required regression fixture is not.

The same read-only contract search found two stale specifications outside the S11 documentation allowlist:

```text
.trellis/spec/backend/shared-scene-contract.md
.trellis/spec/backend/error-handling.md
```

Both still describe a broad standard ToolResult continuation in which mutation effects may commit before the continuation. The active Go contract permits only one generic, result-dependent batch of one or two pure QUERY capabilities before assistant settlement; ACTION and QUERY+ACTION batches must return visible text in the first Main cognition and never continue.

Required exact S11 allowlist extension:

```text
apps/core-go/internal/core/query_continuation_test.go
.trellis/spec/backend/shared-scene-contract.md
.trellis/spec/backend/error-handling.md
```

Intended bounded changes: update the already-allowed `provider.go` validator to accept ordinary non-empty historical assistant messages while requiring exactly one terminal assistant tool-call envelope followed by one or two matching ToolResults; add one focused regression case to `query_continuation_test.go`; and replace the two stale broad-continuation statements with the generic pure-QUERY-only boundary. No Capability schema, runtime dispatch, persistence, workflow, scene behavior, error transport, or browser contract change is intended.

### S12 remediation: S09 native pure-query transport normalization

`normalizeConversationResponseMode` now keeps every explicit mode. An omitted
mode becomes `query_continuation` only for an actual Provider
`StructuredFallback`, empty visible text, and a registry-validated 1–2 pure
QUERY batch. Missing mode without transport fallback, visible fallback, ACTION,
mixed, zero, and over-limit batches remain `final` and hit their existing
visible-output/contract guards.

```text
GOCACHE=/tmp/lac-prompt-memory-s09-native-fallback \
go -C apps/core-go test ./internal/core \
  -run '^(TestNativeToolOnlyFallbackNormalizesOnlyPureQueriesToContinuation|TestQueryContinuationAcceptsOnlyOneOrTwoGenericPureQueries|TestQueryContinuationStateAndToolMessagesAreReplayStable|TestQueryContinuationMessagesAllowBLayoutAssistantHistory|TestCapabilityRuntimeStaticGuardsPreserveActionSingleCognitionAndGenericQueryContinuation)$' -count=1 -v
PASS
```

## S12 — Final Verification Status

Status: all task-owned code, PostgreSQL, Provider, Temporal/Compose, formatting,
architecture, and Core OpenAPI gates passed. One authenticated pre-seeded
projection acceptance command remains blocked on external fixture inputs.

Final deterministic gates:

```text
GOCACHE=/tmp/lac-prompt-memory-core-test-final go -C apps/core-go test ./... -count=1
PASS

GOCACHE=/tmp/lac-prompt-memory-core-race-final go -C apps/core-go test -race ./... -count=1
PASS

go -C apps/core-go vet ./...                         PASS
go -C apps/core-go build ./...                       PASS

GOCACHE=/tmp/lac-prompt-memory-gateway-test go -C apps/gateway-go test ./... -count=1
PASS
GOCACHE=/tmp/lac-prompt-memory-gateway-race go -C apps/gateway-go test -race ./... -count=1
PASS
go -C apps/gateway-go vet ./...                      PASS
go -C apps/gateway-go build ./...                    PASS

test -z "$(find apps/core-go apps/gateway-go -name '*.go' -print0 | xargs -0 gofmt -l)"
PASS / no output
git diff --check                                    PASS / no output
```

Final PostgreSQL gates used disposable `pgvector/pgvector:pg16` containers and
random databases. They covered empty→0032, head rerun, 0031→0032, duplicate
sequence/malformed rollback, the complete migrations package, the complete
database-enabled Core package, and a final post-schema Active lifecycle/
capability rerun. Every temporary database/container was removed.

```text
GO_CORE_TEST_DATABASE_URL=<disposable> go -C apps/core-go test ./internal/core -count=1
PASS (60.255s)
GO_CORE_TEST_DATABASE_URL=<disposable> go -C apps/core-go test ./internal/migrations -count=1
PASS (26.867s)
GO_CORE_TEST_DATABASE_URL=<disposable> go -C apps/core-go test ./internal/core \
  -run '^(TestPostgresActiveMemoryLifecycleReplayCASExpiryAndSupersede|TestPostgresActiveMemoryEventFreezesRuntimeAuthorityWithoutMemoryProvider)$' -count=1 -v
PASS
```

Final real local Provider gate against
`huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX` at active context length 65536:

```text
FLUCTLIGHT_LIVE_PROVIDER_TEST=1 \
FLUCTLIGHT_LIVE_PROVIDER_URL=http://127.0.0.1:11234/v1 \
FLUCTLIGHT_LIVE_PROVIDER_MODEL=huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX \
go -C apps/core-go test ./internal/core \
  -run 'TestLiveProvider(RoleOrganization|ActiveMemory|RecallContinuation)' -count=1 -v

TestLiveProviderRoleOrganization   PASS (50.93s)
TestLiveProviderActiveMemory       PASS (11.87s)
TestLiveProviderRecallContinuation PASS (22.33s)
PASS (85.602s)
```

Observed Provider variability is retained: one combined attempt timed out the
Role case at 180 seconds, while identical isolated and final combined runs
passed; earlier runs exposed missing native structured sidecars and drove the
generic S09 fallback fix. Results were not hidden or prompt-tuned per group.

An isolated Compose project built the final Core/BFF/Web images, migrated to
0032, bootstrapped Worker Deployment, and reported PostgreSQL, Redis, MinIO,
Temporal, Core, Worker, BFF, and Web healthy; migrate/minio-init/cutover exited
0. The script ran `down -v`, and a later Docker inspection found no task
containers. Static acceptance results:

```text
./infra/acceptance/check-compose-bind-sources.sh     PASS
./infra/acceptance/check-core-openapi.sh             PASS (artifact 0.1.0)
./infra/acceptance/check-go-projections.sh           NOT RUN: required BFF_ORIGIN,
  authenticated COOKIE_JAR, MOMENT_FLUCTLIGHT_ID, and PROACTIVE_FLUCTLIGHT_ID
```

The projection script is read-only but requires two already-populated product
flows. The disposable smoke project intentionally creates no Owner session,
Moment, or completed proactive action, and this task does not authorize reading
the unrelated existing Compose stack's user data. This is the only remaining
S12 decision: supply an isolated authenticated fixture or explicitly accept an
environmental waiver because this task changes no BFF/API projection shape.

Final catalog stats are `8005` all / `7467` conversation / `6495` native
cognition bytes/chars across 13 / 11 / 9 definitions. The protected unrelated
`.trellis/tasks/09-08-project-health-evolution/` remains an untouched untracked
directory. No commit, push, merge, reset, checkout, or stage operation has run.

### S12 projection-check waiver and S13 report

The user explicitly approved the documented environmental waiver for
`check-go-projections.sh` on 2026-09-12 and authorized continuation into S13
final report, commit, and archive. This task changes no BFF/API projection shape;
the Core OpenAPI artifact check and disposable full-stack smoke passed.

`final-report.md` now contains the old architecture analysis, Prompt/Role
baseline, two-run/36-call A/B/C experiment, B recommendation, complete Memory
and Prompt architecture, budget, Active lifecycle, Automatic Retrieval,
`memory.recall`, continuation, Summary/workflow, diagnostics/migration,
deleted/replaced path list, verification, risks, and direct answers to all ten
original questions. PRD AC1–AC15 are checked with their evidence and the waiver
explicitly retained rather than reported as a passing authenticated fixture.

Phase 3.4 work commits:

```text
8d58c90 feat(core): rebuild prompt context and memory runtime
68597ad docs: document prompt context and memory contracts
```

Neither commit contains `.trellis/tasks/09-11-*` bookkeeping or the protected
untracked `.trellis/tasks/09-08-project-health-evolution/` directory. No push
was performed.

### S12 live pass 2 and return to S03

After adapting the live harness to the Provider's real native-tool transport,
`TestLiveProviderActiveMemory` and `TestLiveProviderRecallContinuation` passed
individually. Recall selected `memory.recall`, normalized the native
StructuredFallback through the S09 generic gate, accepted frozen B-layout with
ordinary assistant history, sent no tools on the second call, and returned the
provided 2026-08-18 fact.

The combined rerun showed the Role test's old `visible_text` substring assertion
was too broad: the full schema correctly returned `personality_decision=keep`
and `core_alignment.aligned=true` while mentioning the rejected quoted phrase.
The assertion now checks those structured boundary decisions.

The same combined run showed a real S03 contract gap. The model selected
`active_memory_event/create` with `kind=future_event` and the correct flight
semantics, but the optional schema allowed it to omit
`original_time_expression` and `valid_until`, and it set `valid_from` to the
event start. That projection would not be visible the preceding evening and
would have no deterministic expiry. S03 is reopened to require the original
time expression/time precision for creates and `valid_until` for future events,
plus clarify that `valid_from` is the relevance-start boundary rather than the
event timestamp.

S03 now requires `original_time_expression` and `time_precision` for every
create. The `future_event` create branch additionally requires nonempty
original expression, non-unknown precision, and `valid_until`; its Provider
description defines `valid_from` as an optional relevance-start boundary that
must be omitted or set to current/source time when the event should remain
visible beforehand. Commitment/temporary-context creates retain unknown-time
support with an explicit (possibly empty) original expression.

```text
GOCACHE=/tmp/lac-prompt-memory-s03-future-schema \
go -C apps/core-go test ./internal/core \
  -run '^(TestActiveMemoryCapabilityDefinitionIsThinOptionalTransactionalAction|TestAllBuiltinDefinitionsExposeExpectedThinRequiredInputs|TestAllBuiltinDefinitionsAcceptMinimalProviderInput|TestProductionCapabilityCatalogKeepsImplementationFieldsOutOfProviderSchema)$' -count=1 -v
PASS

new schema stats: all=8005, conversation=7467, native=6495 bytes/chars
```

S08 `docs/capability-schema-report.md` and S11 Provider/Memory/Cognitive/
Structured-Turn/Capability architecture contracts now carry the final schema
sizes and the future-event/native StructuredFallback rules. Targeted gofmt,
compile-only live-test selection, and `git diff --check` passed. S12 now resumes
at the required combined Live Provider command.

The next combined live attempt passed Active and Recall again. RoleOrganization
timed out once at 180 seconds, then returned a correct full-schema decision on
retry: final response, 7:00 fact, `personality_decision=keep`, and
`response_plan.core_alignment.aligned=true`. The fixture had required only the
optional root `core_alignment`; it now mirrors production normalization and
accepts the authoritative value from root or response plan. No Provider output
or product rule was weakened.
