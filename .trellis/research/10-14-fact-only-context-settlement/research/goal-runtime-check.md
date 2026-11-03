# F15/F16 independent runtime check

Reviewer: dispatched trellis-check; baseline ec1ed82. Existing implementer changes retained. No commits, staging, push, production workflows, live Provider calls or service installation/start.

## Findings fixed

1. **F16 terminal Tool selection was advisory to the Provider.** `goal_submission_recovery.go:37` previously supplied an empty Tool catalog after three no-progress attempts, but the native runner could still execute unsolicited Provider ToolCalls forever. Added task-owned `physicalModelRequestDecision.ToolCallFallback`, accepted only with forbidden choice plus an explicitly empty catalog. `eino_model_runtime.go:186` and `:272` adapt successful Generate/Stream responses that still contain ToolCalls to the Goal-owned operational summary. Streaming buffers only this terminal response, preserving stream errors/cancellation before adaptation. Raw physical response/usage diagnostics remain recorded before adaptation. No ToolCall, receipt, root acceptance or business effect is invented; ordinary Agents never install this fallback.

2. **F15 unspent daily budget was sufficient without a captured daily trigger.** `migrations/goal_agent_cadence.go:38` now requires a visible, unprocessed, watermark-captured daily/boot event for the current local day. Allowed reasons are schedule_accepted_daily, daily_cadence_due, initialization_completed, auto_planning_enabled and startup_recovery. Explicit local_date wins; boot events use their captured timestamp in the instance timezone. Actor/profile/wish/pause/cancel hints alone cannot mint the daily episode. The same SQL function governs SQL producer, old queued claim and final native admission. Completed and explicit Owner event keys remain exact per-event keys.

3. **Daily repair must produce the trigger after predicate tightening.** `goal_planner.go:622` separately admits repair candidates with an unspent current daily key, allowing repair to insert the stable daily event before episode eligibility is checked. The event includes local_date. Existing unspent completed/private events remain repair candidates; failed runs/admissions are not reset.

## Added/updated regression coverage

- Actual local native HTTP: repeated plan replay and bad criterion Providers ignore the terminal no-tools request. Both terminate after the single final response without executing its unsolicited ToolCall. The plan mode also runs through actual SSE/native streaming.
- Final HTTP 503 and actual context cancellation remain errors; no fallback success masks either.
- Existing distinct accepted targets still progress to full coverage without budget truncation.
- Actual sensitivity RED: temporarily disabled only ToolCallFallback under Python finally restoration. Both uncooperative HTTP modes failed at the bounded tenth request, rather than relying on service timeout. `/tmp/goal-runtime-check-fallback-red.json` has one failed parent plus two failed subtests. Source was restored immediately.
- Disposable PG tests now cover actor/profile/wish/pause/cancel hints with a free slot and unspent day, old queued runs, direct native admission, yesterday's schedule, a valid captured day awakened by a new hint, and pending hints left unconsumed. The original local-day test now requires captured events/repair rather than treating the date itself as a trigger.
- Disposable PG migration test verifies fresh setup and released installer restoration followed by 0062 upgrade/head rerun, preserving exact stored clock/request, admission, failed run/error/result and private source event; verifies the new episode function survives restoration.
- Disposable PG F16 handled-cycle tests include Providers ignoring the final empty catalog, alongside original cooperative modes.

## Other source paths checked

- Evaluation claim takes lifecycle/Life then request locks and DB time; pending, retry and frozen snapshots share per-instance cadence. Deferred admission changes available_at without attempt/source consumption. Failure backoff cannot precede evaluation_next_at. Queue and old SQL producer intents converge at claim. Existing logical-run release returns before workflow deferred waiting; no added sleep.
- Periodic repair scans due active instances with visible active/paused goals, rechecks under Life lock and uses stable clock identity plus existing request uniqueness/intents.
- Planner checks full capacity before claim/model and at final native admission. The episode ledger is written under Life lock before the native Agent; failure keeps the budget spent. Durable commit recovery precedes capacity/budget checks. Late-event scheduling calls the same predicate, and visible-profile exact events are used rather than a global consumed watermark.
- Accepted target replay still passes current claim and projection/Goal revision/source fingerprint authority before returning the original stored output/digest. Normalization runs only in the dedicated native adapter, after authority checks; ambiguous criterion refs return a handled receipt before revision/judgment/memo writes. Strict general hydration/semantic evidence validation remains.
- Accepted projection is sorted and identifies root/plan/object. No-progress finalization preserves accepted siblings and unresolved roots and exits before shared source consumption. A handled request replay cannot restart the model loop.
- 0063 installs after released SQL bundles; no released SQL constants were edited.

## Verification actually executed

All tests use FLUCTLIGHT_LIVE_PROVIDER_TEST=0, empty GO_CORE_TEST_DATABASE_URL and GOCACHE=/tmp/fluctlight-go-cache.

- `go -C apps/core-go test -race ./internal/core ./internal/migrations -run 'Test(Goal|PostgresGoal|PhysicalModelRequestPolicy|ToolFinalSchemaSeparation|ConversationPhysical)' -count=1 -json`: PASS; including subtests **177 PASS, 93 SKIP, 0 FAIL**, top-level **87 PASS, 59 SKIP**. Log `/tmp/goal-runtime-check-race.json`.
- Final persistence/test compile after adding the noncompliant Provider PG variants: `go -C apps/core-go test ./internal/core ./internal/migrations -run 'Test(GoalSubmissionRecoveryPersistence|GoalAgentCadence|PostgresGoalAgentCadence)' -count=1 -json`: PASS with DB cases skipped. Log `/tmp/goal-runtime-check-persistence-compile.json`.
- `go -C apps/core-go vet ./internal/core ./internal/migrations ./cmd/worker`: PASS.
- gofmt check for all changed/untracked task Go files: PASS.
- `git diff --check`: PASS.
- Native HTTP tests initially failed to create a listener inside sandbox; rerun with approved local-listener escalation passed. This environmental error is not recorded as a business RED.

## Findings not fixed / verification limits

No additional in-scope concrete product defect found after the fixes above. PostgreSQL/pgvector is unavailable and GO_CORE_TEST_DATABASE_URL is unset: transaction-level concurrency, SQL function execution, migration upgrade/rerun and durable replay/source assertions are **SKIP, not accepted**. Their Go tests compile, but source review cannot replace executing them. Real Provider, Temporal, Redis and production behavior are NOT_RUN.

Per dispatch, main session owns final full Go/workspace typecheck/test/build plus spec/report synchronization. This reviewer did not modify specs or main task documents. Existing backend Goal contracts still need F15/F16 updates describing the new cadence, authorized empty-slot rule and deterministic adapter/finite termination behavior.
