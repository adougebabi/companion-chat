# Main integration state (2026-09-22)

Task in_progress, user approved design and implementation. Latest user steering: GPU resources first; all LLM validation must be sequential; after implementation user may run final real LLM acceptance. Stop bulk live requests, finish code + non-LLM verification; pending real acceptance is NOT pass. Do not mark overall complete/archive with pending matrix.

Branch codex/agent-tool-plugin-e2e. No commits made by this task. Original HEAD 6161431 advanced externally to 32276a1 (Trellis-only changes); baseline files record this. User's unrelated phase8-eino-audit* files must remain untouched.

## Current state

- Shared native Eino loop no longer <=2 / deferred-stop / write-only max success; partial messages/tools retained.
- 16 formal Agent definitions + migrated model_tasks/provider/ADK bridge. Formal Agent implement worker was interrupted before report; main has reviewed key files but full verification still required.
- New typed standalone RunConversationCognitionAgent in cognition_agent.go. All 16 real E2E tests in formal_agent_e2e*.go; last complete live attempt media_quality pass, 15 fail mostly GPU OOM. Real secret attempt had two recalls/three decisions but wrong final answer (fail). All evidence preserved. Test layout later fixed simple alphanumeric category, not live rerun per user steering.
- New ExecuteTool boundary; native correlation separate from OperationID; authorization validated. Generic DirectToolCapability for publication/targets. Transactional/pure query existing implementation reused.
- New tool_executions receipt ledger in same local transaction as mutation: prevents redoing committed effects after model failure, fast replay before Prepare. Queries never cached. Main wrote tool_execution_ledger.go and integrated with ExecuteTool.
- Schema head 0034_tool_execution_source, previous0033: introduces receipt table, removes active-memory source_fact FK to cognition inbox (both active_memories and active_memory_commands), preserves evidence IDs/ownership/actor/conversation constraints. New schema not run on existing shared regression stack, only disposable tests. Migration tests still need specific upgrade/idempotency test.
- Existing 15 Tool capabilities all have actual independent implementation; direct tests cover many. Last runtime fixes: scene/presence no frozen ActionID requirement, stable operation IDs; presence AuthorizationActorID metadata. relationship summary string (was []byte). visual initialize async class dispatch to transaction + accepted not completed. accepted allowed shared validators. Some old tests now need updates for new legitimate contracts.
- Persona service wired builtin; main just changed persona tools from internal-only to visible Action on conversation/wake/autonomy/native surfaces. Runtime prompt now demands native persona.switch/takeover, and Tool result includes working_persona on applied transition. Need controlled regression and update old internal-only assertions; ensure takeover product actually preserved.
- Main fixed provider.go structured candidate condition to require jsonMode for text Agent; pending targeted regression.

## Production migration

production_callers worker completed: new agent_result_adapter.go (~881 lines) owns handleTurn/ProcessWakeUp/ProcessNativeCognitionFact/ProcessDailyReview and consumes committed trace, no replay tool calls. Stream uses RunMainStream; natural final via ToolPublicationService, explicit reply Tool loads committed message. Worker controlled PG and go test/vet ./... passed before later main changes.

Known remaining main work:
1. physically DELETE *Legacy functions and old query continuation/deferred duplicate workflow code; worker only renamed old entry functions to Legacy. Do not leave unreachable old loops. Use exact symbol/consumer tracing and preserve useful domain helper/tests.
2. Review new production adapter for side effects/error consistency/product gaps. `failAgentTurnAfterRun` currently uses caller ctx (cancellation may prevent durable failure marker), errors often ignored; must use bounded cancellation-independent persistence and test.
3. `finalAgentVisibleText` currently falls back to completion.Text even when Structured exists but visible field empty => can publish JSON; fix.
4. Production handleTurn still duplicates context assembly instead of calling new standalone cognition_agent entry. Need unify typed service/input (preserve SpeakerActorID vs Owner, actual SourceFactID, grant) and output contract while no fake Main requirement.
5. Formal schema mapping remains explicit registration routing, old QueryContinuation method still present; delete old bypass references.
6. Every business Tool needs formal actual Eino adapter execution evidence, not just fake NativeToolCallID in direct requests.
7. Finish all independent Tool six-case matrix + agents controlled edge cases, run isolation and commit-then-model-failure, no intermediate fallback.
8. Two isolated destructive validation experiments: remove tool result refill; fake successful memory write without SQL; both must make target tests fail. Use temp copy, no production flags. Restore final-code regression.
9. Extend existing infra/acceptance/run-go-live-provider-smoke.sh with strict serial suite/tool/agent selectors and fail closed on missing/zero/skip/blocked; avoid new testing platform. Full final real LLM acceptance deferred to user per steering; no broad concurrent live runs.
10. Update existing conflicting Trellis specs, full quality check, final code/version/log matrices; do not overall-complete until required real acceptance.

## Active agents

visual_identity_agent is STILL RUNNING. Owns visual_identity.go/new visual agent/tools/tests, may add formal_agents.go definition and builtin assembly entries only. Latest instruction: NO new live LLM calls; finish true complete visual Agent loop, real picture content, independent tools, controlled + real DB, defer final real LLM/media acceptance. Do not take over its files while in flight. Other agents completed or interrupted, not reusable per user instruction.

## Isolated environments (NO secrets in repo)

/tmp/lac-agent-tool-current.json contains session folder/project/compose command. folder/test-env.json mode0600 contains isolated PG URL and current real test Provider URL/model/key (endpoint no secret row). Read into process env, never print values.
- Compose project lac-agent-tool-5c2f9f296b, postgres mapped localhost random port. Parent created from infra/compose/fluctlight.compose.yml + private local env + temp ports override. Each Go isolatedCoreTestRepository creates/drops its own DB.
- Root .env model endpoint was connection refused. Read-only query of existing dedicated fluctlight-phase8-regression DB found different working endpoint; /models200 and TestLiveProviderRecognizesImageGenerationIntent PASS25s. Loaded into private test env. Existing stack NEVER written/migrated/stopped.
- ComfyUI root .env /system_stats200. Visual worker authorized start this task's disposable MinIO/other deps if needed, not existing stack.
- /tmp/lac-run-tool-test.py <unique-log-name> <regex> runs core test -json with private env, timeout180, writes research/runs logs/meta. Must remove FLUCTLIGHT_LIVE_PROVIDER_TEST for broad non-LLM tests. Current helper prints last9000chars (may be noisy); API keys must be redacted for future live logs, helper only PG password currently redacts. Endpoint current key empty.
- Shutdown only this task compose project via stored command down -v when done; retain reproduction instructions.

## Evidence

research/runs/memory-direct-01 failed test SQL (source_fact_id missing); fixed to evidence_refs->>0 and removed fake persona fixture. memory-direct-02/03 pass.
independent-tools-ledger-01 failure old expected conflict string; -02 pass9root tests (memory,active,persona,reply,moment,media).
remaining-tools-e2e-db-final: fixed15 baseline + 6 tools36 subcases pass. schedule real success in schedule-retry log but overall command failed overly strict diagnostic count. Assertion fixed; subsequent actual final attempts failGPU OOM, never count pass. User will rerun serial.
production-callers-controlled/stream/daily pass; actual full live production-callers-formal-live failedGPU.
formal-agents-06-full-suite.log media_quality PASS,15FAIL,0SKIP; formal-agents-controlled-01-memory-loop PASS.

Last main test /tmp/lac-agent-tests-after-persona.log failed old internal-only persona/catalog expectations (five tests), not compile. Update tests to explicit new product catalog/authorization requirements; don't weaken behavioral tests. Generic runtime static guard previously updated to only forbid business-name dispatch in generic boundaries (persona domainservice switch is legitimate). Migration head assertions updated to0034 previous0033.

## Later integration updates (all implementation agents now completed)

- Visual complete Agent production and 3 Tools done, formal total17/Tools18. New visual typed Agent emits true image_url bytes read from media_assets+MinIO; ProcessVisualIdentity uses durable state-derived run IDs. Controlled PG+MinIO + error-after-commit tests pass; real visual handler E2E written (formal_agent_e2e row17) but NOT RUN per user. It advances production ProcessVisualIdentity/ProcessMediaIntent with real Provider/Comfy/S3, does not claim Temporal transport coverage.
- retired_path_cleanup completed: four Legacy functions, query_continuation and runtime caller prepare/plan/settle removed physically. Temporal recovery activities preserved as thin ExecuteTool consumers. Report retired-path-cleanup.md. Broader go test/vet passed at that slice state.
- serial runner + live visual mapping done: fixed18 Tools/17 Agents, strict sequential/crossprocesslock, source hash, missing/skip/zero/fail nonzero. Visual JSON/S3 preflight before any model rows. Reports serial-runner.md and visual-live-harness.md. No new live calls.
- adapter_and_fault_evidence done: formal_tool_adapter_e2e_test.go 18 actual Eino adapters (controlled HTTP + real PG), two subprocess fault experiments (only *_test.go TEST_ONLY flags) + restored probes, all pass and race/vet pass. Report adapter-and-fault-evidence.md. Three required root symbols now exist, runner mapping needs final verification.
- Main added generic agent_runs admission/finish fences in agent_run_record.go and linked Tool ledger agent_id/run_id/invocation fields; RunADKStructuredTask wraps execution with durable admission/final record. Failed/running runs don't rerun; failed/interrupted Tool receipts reconstruct partial trace. TestAgentRunRecordRetainsCommittedToolAfterCancellationAndPreventsReplay realPG pass (agent-run-record-01), plus no raw JSON as visible text unit pass.
- Main fixed JSON-mode-only candidate validation for text Agents; exposed persona Tools as Action across usual default catalogs; returned actual working_persona and updated runtime prompt. Updated old internal-only tests/catalog arrays, core tests pass.
- Removed hidden ADK SupportsSurface execution gate; surface still only default catalog grouping. Explicit canonical public Tool can be installed by another Agent. Updated tests.
- Main added SubjectActorID to ToolExecutionRequest/InvocationMetadata, explicit authorization (own resource or authorized conversation actor), metadata to presence; ADK gets real projection.ReferenceIndex.SpeakerActorID. tool digest binds subject. Ensure ledger invocation also includes SubjectActorID (currently reconstructed metadata might omit it; main noted follow-up).
- Life scene conflicts now typed nonretryable ErrConflict, invalid business inputs typed ErrInvalidArguments, consumable by Agent. Prompt context authority changed to respect committed newer Tool facts instead of stale initial frozen snapshot.
- Main added DirectTool result/schema validation inside its owned transaction before ledger commit.
- Main RecordModelToolCalls now keeps first physical model identity for repeated native ID, does not overwrite original provenance.
- Main finalAgentVisibleText rejects raw completion.Text fallback when Structured exists; failAgentTurnAfterRun uses bounded context.WithoutCancel. Tool-only handling allows empty assistant map, but settleAgentConversationTurn and callbacks/replay need final no_op/empty-message review (main hasn't yet completed that follow-up).
- Last main /tmp/lac-core-current-05.log core/aiagent/migrations tests pass (no real DB in that invocation).
- Main rewrote existing structured-turn-contract.md and fluctlight-cognitive-runtime.md to authoritative new contracts. Memory/provider/autonomy/visual/persistence related docs still need targeted updates. trellis-update-spec skill read; no new spec platform.
- Remaining: finish docs; final full-scope Trellis check agent with task override and NO LIVE LLM; actual broad isolated DB regressions and Go race/vet/build, frontend contracts if HTTP diff needs checks, strict runner unit tests; additional migration upgrade test; persona acting-profile attribution review; finalize matrix/commands/current source version; clean only our PG/temp deps when safe; user's real LLM/Comfy acceptance pending. Do NOT archive/claim full acceptance, don't auto-commit unrelated changes.

## Main integration update after user progress request (2026-09-22)

User asks "还没好吗？进度是多少，先汇报一下". Replied honestly implementation/non-live validation ~80% work estimate, not completion: full DB regression still failing, real model acceptance deferred to user strictly serial. Continue implementation; no live LLM requests.

Previous 3 regression agents are now complete/interrupted. Reflection terminal report saved. Persona partial report saved `persona-db-regressions.md`; conversation partial did not save report before interrupt.

Latest full DB run `integration-go-db-02`: exit1,1157 passing events,26 skips. Fail clusters were source-fact global requirement, synthetic default working profile for no-projection adapter, missing reply source correlation/tokens, obsolete native/life/persona fixtures, stale authority receipt lookup. Logs all preserved.

Main fixes since:
- Removed blanket SourceFactID requirement from capability.CapabilityInvocation.Validate (domain-specific evidence validators retain requirements).
- workingProfileForToolExecution no longer invents default profile when projection absent.
- Added ToolExecutionRequest.CorrelationID; ADK passes request correlation; invocation uses it. Actual publication source/turn resolution uses that real correlation.
- Restored capability_core_test guard by reading its exact files directly instead of deleted productionSourceText helper.
- Typed entry guard now expects RunConversationCognitionAgent.
- Removed the NEW business-name-based refreshProjectionAfterCommittedTool entirely from generic ADK adapter: Tool Prepare already resolves actual dependencies, no projection prerequisite. 18/18 actual Eino adapters passed again in `integration-adapter-fixes-02` exit0 (19 pass events including parent).
- Added every-round input budget checking in queuedToolCallingChatModel Generate/Stream, counting full messages rather than diagnostic64 cap. Added agent_round_budget_test.go; focused unit pass. Need inspect JSON/text responseFormat estimate and ensure media-specific bypass matches existing contract.
- agent_run_record digest now binds Tools/role/schema/subject/conversation/target/authpolicy/source as well as prompt/actor; changed-input conflict checked before running replay too.
- persona_action_service now composes target accepted Evolution overlays into working_persona inside tx. Corrupt overlay errors, not baseline fallback. Added persona_tool_overlay_test.go realPG test; not yet run due concurrent seam compile gap. Do not claim pass.

Checks completed: node runner/verifier14/14, pnpm typecheck, pnpm test (12 browser-client +47 web), pnpm build all exit0. Tool group integration-tool-fixes-01 passed27 events, failed4 adapter refresh cases subsequently fixed in adapter-fixes-02.

Docs updated in-place: provider (deleted duplicated 2-round/allowlist/deferred sections and derived-ID/continuation contradictions), autonomy (explicit per-Tool policy/local transactions), memory (direct boundary/evidence), visual (formal Agent4Tools/multimodal/checkpoints), persistence (0034 receipts/runs/reservations), persona-layer bottom native scope, backend index. Structured/cognitive already updated. May still have stale old wording; final review needed. Matrix and PRD/implement now reflect in_progress, current failed full run and pending final live. All rows not blanket PASS.

### Currently active new agents

- /root/finish_conversation_regressions: owns agent_result_adapter.go, tool_publication.go and conversation/life/native fixtures. Fix stream token/source linkage, stale CAS, remaining native fixtures. Told that remove_deferred_seam deletes ExecuteDeferredTx; it must migrate three life_context_surface_test direct image calls. Main must not race its files.
- /root/finish_persona_fixtures: owns remaining turn_path_cost_report(+json), turn_takeover_f05_test, turn_takeover_test/helpers. Rewrote to native physical request costs and overlay corruption; told to return terminal now, main reruns after shared compilation restored.
- /root/remove_deferred_seam: owns core/capability_core.go, internal/capability/capability.go, builtin_capabilities.go and capability_core_test.go; removing unused DeferredCapability/Runtime.ExecuteDeferred/fake-completed ExecuteDeferredTx + deferred low-level outputs. Can move DirectToolTarget/DirectToolCapability into capability package with core aliases for generic classification. DO NOT race these files. Main already fixed global source validation, preserve it. Last compile failure expected during removal `undefined DeferredCapability`.

### Outstanding must-fix/review

- `agentSettlementAuthorityRevisionsTx` still has business-name checks and blind adopts post-Tool snapshots; rejected Tool/no ledger yields no rows. Agent owns currently. Need generic before/after authority deltas bound to each actual mutation, preserving initial and unrelated concurrent revisions. Generic Tool/Agent code must not need name branches for new Tools.
- Old turn_takeover.go still contains uncalled A/Judge/B orchestration/judge path, many old tests use helpers; user requires physical cleanup, not just no callers. Must trace/reduce while retaining formal standalone Judge/Reply Agents and useful profile/control-view helpers. Do not race fixture agent before terminal.
- Profile scopes: memory+relationship covered, working persona overlay added; Goal/Intention context in prompt after native takeover deserves review.
- Empty legal final no_op without tools currently handleTurn returns cognition_visible_text_missing; fix explicit no_op and browser completed with no empty message ID.
- Direct source identity now truly optional for queries/publications; no fabricated model facts. Ensure source-aware domains require real evidence as needed.
- Final model budget/partial receipt/real stream tests, full DB run, race/vet/build, fresh trellis-check (no live) still needed.
- Strict serial live commands and visual config docs exist. Do not mark whole task complete/archive or commit without final concrete review.


## Delivery checkpoint (latest)

- All regression/cleanup agents except strict_runner_final_gates are terminal.
- Retired A/Judge/B orchestration and turn_decision.go physically removed; formal Judge/Reply prompts/typed entries preserved. Static persistent-switch guard migrated; dead disabled nativeReplay fixture physically deleted.
- DeferredCapability/ExecuteDeferred/ExecuteDeferredTx/CapabilityExecutionDeferredOutput removed from core+capability packages and builtins. Direct low-level Execute returns execute_tool_required; real App.ExecuteTool remains authority. DirectToolTarget aliases capability target; DirectToolCapability remains core-specific because Core CapabilityContext is a distinct type (do not alias full interface without considering types).
- Authority receipts now contain Before pointer + after. executeToolMutation uses short REPEATABLE READ transaction to distinguish own writes; final adapter generically chains all successful mutation receipts against initial projection (queries skipped by generic readonly metadata), no business-name switch. Real DB full suite passed.
- Generic source/profile/correlation fixes and target persona accepted overlay composition verified realPG.
- Two stale DB tests migrated: TestIndependentToolReceiptFailureRollsBackMemoryAndAffect injects failure at tool_executions INSERT, proves all domain/outbox effects rollback, removes fault then proves writes; TestAcceptScheduleUsesDatabaseIdempotencyBoundary uses isolated current-head DB. Neither silently skips old head now.
- Added dependency_failure and foreign_owner_rejected subtests to every fixed18 formal Tool adapter row; adapters-full-failures-02 passed55 events0skip. Fixed typed-nil affect service in builtinCapabilities(nil).
- Removed attempted conversation no_op test/branch: actual conversation final schema requires reply; do not add unrequested no_op feature. Tool-only empty completion remains valid with committed effects and existing tests.
- Full current-source run integration-go-db-final PASS1216 events/24skip, Go vet/build0, race91pass0skip; SHA maps all matched that source.

### Review disposition and remaining active work

final_architecture_check completed read-only; report research/final-architecture-check.md. No defects in txn/CAS/run fencing/native identity/profile. P1 accepted: full agent runner missing fixed cross-case gates; selected Tool missing formal adapter selection. Active /root/strict_runner_final_gates (trellis-implement) owns ONLY runner script + node tests, adding fixed mandatory controlled gates + new TestLiveStreamTurnFormalAgentNDJSON and selected adapter, negative script tests. Await terminal; no live calls.

P0 assertion about frontend incremental raw tokens was rejected with actual HEAD evidence: old mutations.go1327-1346 already emitted whole callbacks.onChunk(visible) only after commit. Precommit Tool arguments/partial JSON would violate publication contract. Existing behavior preserved, spec clarified actual Provider SSE vs committed NDJSON delivery. Main added production_stream_live_e2e_test.go: TestLiveStreamTurnFormalAgentNDJSON checks actual stream=true on every request, passthrough consumed SSE deltas/DONE (no read-ahead), production StreamTurn NDJSON ordered/one terminal/actual SQL reply/source. Live row NOT_RUN per user. Existing TestDirectConversationStreamsCommittedUserBeforeProviderAndAssistantAfterCommit now fails nonstream request and delivers final JSON in2SSE deltas. production-stream-wire-controlled PASS1 + live SKIP1. These are tests-only changes since last full pass.

Current running final checks (no production edits since):
- integration-go-db-delivery, exec session8071 (full isolated PG, expected1216pass/25skip after added live row)
- delivery-stream-race, session22897 (3 production stream tests)
- delivery-go-vet finished PASS0.
Earlier final-go-build still corresponds identical production code; only two test files changed after.

### User live handoff ready

Copied media.comfyui read-only from existing fluctlight-phase8-regression-postgres-1 into owned session folder/visual-live.json0600, with own MinIO. Source contains baseUrl+workflow. Translated only container host.docker.internal to127.0.0.1 for host-run tests; first preflight failure retained, then visual-dependencies-preflight-host PASS (Comfy /system_stats + empty temporary S3 bucket). No LLM or image generation request.

/tmp/lac-run-final-live.py0700 loads /tmp/lac-agent-tool-current.json -> private test-env.json and invokes existing repository serial runner. Guards exact owned project lac-agent-tool-5c2f9f296b. private env includes FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE. User commands documented research/user-live-verification.md (single Tool/Agent/full suites). Keep owned PG/MinIO until user verifies; document cleanup, never touch existing phase8 stack.

Still needed: consume strict runner terminal, run node tests if not done on final state, consume final Go/race checks, update acceptance matrix and concise delivery report with actual counts/source hashes/commands, final git diff --check. Do NOT claim real dual E2E pass, archive task, or commit: real acceptance intentionally pending. Trellis finish-work cannot archive an unfinished user acceptance; remain in_progress. User asked status earlier, answered implementation/non-live ~80% then later precise1216pass results; do not repeat stale percentage as final.


## Final delivered state

All subagents are terminal. No ongoing tool sessions. Current Go code matches all four final-source snapshots: final-source-go-db exit0/1217pass/25skip (no-test packages + live-only), final-source-race exit0/93pass/0skip, final-source-vet/build exit0. Per-round multimodal budget now retains text and image allowance (not raw base64); new focused race passed. Production streaming live test added but NOT RUN, controlled two-SSE production test + stream race passed.

Runner P1s fixed and checked17Node tests; actual strict selected memory command passed inventory+adapter+direct. Scope label refined to tool-e2e:memory_event (only suiteall says dual-e2e), targeted Node regression and actual strict-selected-memory-delivery both pass on this last script change. git diff --check and bash syntax pass.

Delivery files now authoritative: research/delivery.md, acceptance-matrix.md (fixed18Tools17Agents explicit cases/statuses), research/user-live-verification.md. task.json notes updated, status remains in_progress because user-run real acceptance pending. No code commits, production migrations/writes, pushes, archives. Private test stack intentionally retained for user verification. User can run `python3 /tmp/lac-run-final-live.py --suite all`, or selected Tool/Agent flags; wrapper only loads private session env and invokes existing serial/locked runner. Real Provider old attempts OOM/wrong-secret failures still retained and not passes.
