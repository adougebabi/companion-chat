# Final architecture check

## Findings (fixed)

None. This pass was intentionally read-only for production code; the main session owns fixes and final reruns.

## Findings (not fixed)

### P0 — Production `StreamTurn` does not stream Provider tokens

- Files: `apps/core-go/internal/ai/agent/loop.go:299-303`, `apps/core-go/internal/core/agent_result_adapter.go:302-321`, `apps/core-go/internal/core/mutations.go:600-607`
- Trigger: any production streamed conversation turn.
- Evidence: the ADK iterator calls `MessageOutput.GetMessage()`, which aggregates a streaming message before returning it. No token callback is connected to the native model stream. `handleTurn` calls `onChunk` only from `emitCommittedAssistantMessages`, after the Agent has completed and the reply has been committed; `StreamTurn` therefore emits one whole reply as a `token` frame near the end.
- Impact: clients receive no incremental model output even though the formal Agent has `EnableStreaming=true`. This violates the structured-turn contract that aggregated final text is not streaming evidence, and a long model run can remain silent until completion.
- Minimal fix/test: expose native assistant text deltas from the Eino Runner event stream through a request-scoped callback while keeping ToolCall chunks private and final publication authoritative. Add a production `StreamTurn`/NDJSON test with a deliberately paused multi-chunk Provider stream which proves at least one token frame is flushed before the Provider releases the final chunk, and still proves exactly one terminal frame and no control JSON leakage.

### P1 — The strict Agent suite can report PASS without several required cross-Agent cases

- Files: `infra/acceptance/run-go-live-provider-smoke.sh:459-465`, `infra/acceptance/run-go-live-provider-smoke.sh:621-625`, `apps/core-go/internal/core/formal_agent_e2e_test.go:77-85`
- Trigger: run `run-go-live-provider-smoke.sh --suite agents` or `--suite all` after all 17 per-Agent rows pass.
- Evidence: `run_agent_matrix_gates` requires and executes only `TestFormalAgentE2ERejectsBrokenToolResultFeedback`. The per-Agent conversation row invokes `RunConversationCognitionAgent` directly and merely sets `EnableStreaming`; it does not traverse `StreamTurn`/HTTP NDJSON. The runner has no fixed required gate for production streaming, same-round multi-call pairing, committed-effect-then-model-failure recovery, cancellation/timeout/iteration exhaustion, or run/user isolation, despite those remaining mandatory cross-object rows in `acceptance-matrix.md`.
- Impact: the strict runner can write `overall_status=PASS` while mandatory R8/R9 behavior was never selected in that run. This is a concrete false-positive path independent of the intentionally deferred live execution.
- Minimal fix/test: define an explicit fixed list of cross-Agent gate test names, validate that every function exists, execute all of them in `run_agent_matrix_gates`, and pass each expected test name to `verify-go-e2e-events.mjs`. Include the production streaming test above. Keep controlled fault/guard cases labelled controlled, while the real Provider rows remain serial.

### P1 — A selected Tool run omits the Tool's formal Eino adapter case

- Files: `infra/acceptance/run-go-live-provider-smoke.sh:610-618`, `infra/acceptance/run-go-live-provider-smoke.sh:635-645`, `infra/acceptance/run-go-live-provider-smoke.sh:665-669`
- Trigger: run `run-go-live-provider-smoke.sh --suite tools --tool <name>`.
- Evidence: the full Tool suite runs `TestFormalToolEinoAdapterE2E` before the direct Tool rows, but the selected-Tool branch runs only the fixed-inventory guard and `run_tool`. `run_tool` selects the direct tests from `tool_test_regex`; it never selects `TestFormalToolEinoAdapterE2E/<name>`.
- Impact: the documented single-Tool acceptance command can exit zero without proving native ToolCall identity/result feedback or direct-vs-adapter equivalence for that Tool. A broken adapter for the selected Tool is therefore a false PASS.
- Minimal fix/test: in selected mode, run `^TestFormalToolEinoAdapterE2E/<escaped-name>$` and require both the parent and named subtest events before the direct Tool cases. Extend the runner test to assert a selected Tool whose adapter subtest fails makes the command fail.

## Reviewed invariants with no concrete defect found

- Mutation Tool effects and `tool_executions` receipts share one short repeatable-read transaction; the operation advisory lock and in-transaction replay check close the initial read race.
- Authority receipts record before/after revisions in the Tool transaction, and final settlement chains each successful mutation from the initial projection rather than accepting whatever revision is current.
- `agent_runs` is admitted before model work for Tool-capable business runs; completed results replay, while failed or still-running records fence whole-loop replay and retain committed Tool evidence.
- Native ToolCall ID and physical Provider request identity are required; repeated IDs with conflicting name/arguments are rejected, while stable business operation IDs are derived independently.
- Structured final validation is applied after the native loop and does not fall back to an earlier assistant message or reasoning content.
- Working-persona propagation composes the accepted evolution overlays and is consumed by subsequent Tool execution.
- No business capability-name dispatch switch remains in the generic `ExecuteTool` executor.

## Verification

- Lint: pass per main-session final evidence (`final-go-vet`, exit 0); not rerun in this read-only review.
- TypeCheck/build: pass per main-session final evidence (`final-go-build`, exit 0); not rerun in this read-only review.
- Tests: pass for the final isolated PostgreSQL full Go run (`integration-go-db-final`, exit 0, 1216 passing events, 24 skips limited to packages without tests and live-only cases) and race run (exit 0, 91 passing events, 0 skips), as reported by the main session with matching current source hashes. Real Provider/ComfyUI acceptance remains intentionally not run and must not be reported as PASS.


## Main-session disposition

- P0 token timing: **not accepted as a product regression**. At the task's HEAD,
  `mutations.go` lines 1327–1346 already emits `callbacks.onChunk(visible)` only
  after settlement and committed user/assistant publication. The requested
  contract forbids publishing intermediate candidates/control envelopes. Feeding
  unvalidated partial final JSON or ToolCall arguments to browser tokens would
  violate that boundary. The native model call remains truly streamed; Eino
  aggregates structured messages for validation. The spec now explicitly
  distinguishes Provider streaming from committed browser output instead of
  implying that a late complete token frame proves incremental Provider input.
  Added `TestLiveStreamTurnFormalAgentNDJSON`: actual production StreamTurn,
  `stream=true` on every physical model request, pass-through observation of
  consumed SSE deltas and DONE, committed SQL/source match, ordered NDJSON and
  exactly one terminal frame. It is live-only and remains NOT_RUN. The existing
  controlled production stream test now requires streaming and splits its final
  DTO across two real SSE data frames; no non-stream fallback can satisfy it.
- P1 strict cross-case gate and selected adapter omissions: accepted; the
  `strict_runner_final_gates` slice is implementing fixed required gates and
  corresponding negative runner tests. Completion/evidence will be recorded
  after its final checks, not assumed from dispatch.


## Runner findings closed

Both P1 findings are fixed in the existing runner. Full agents/all require the
fixed controlled cross-case list, real production StreamTurn gate and all 17
Agent rows. Selected Tool execution requires inventory, selected native adapter
and direct business cases; stateful visual selection runs the full controlled
adapter fixture while requiring the chosen child event. Fake-shell negative
cases and verifier tests passed (17 tests). Actual selected memory_event command
passed all three gates with exit0. Scope labels now distinguish selected Tool,
selected/all Agents, all Tools and complete dual-e2e; an individual PASS does not
claim full dual-suite acceptance. No real LLM request was made for these checks.
