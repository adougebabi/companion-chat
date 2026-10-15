# F16 implementation evidence

Implemented directly in the dispatched implementer. Existing F15 cadence, worker,
Planner and migration changes were preserved. No git add/commit/push, production
workflow, real Provider, or service installation/start was performed.

## Behavior and ownership

- `goal_evaluation_tools.go`: same accepted capability/goal/object target returns
  the original accepted output, `replayed`, `ignored_payload_change`, no new
  mutation, and current unresolved short Goal refs. Native identity/claim guards
  remain upstream. Replay also rechecks projection, live Goal revision and source
  fingerprints; actual authority conflicts remain failures, not replay success.
  The original digest/journal is never overwritten. A common authority reader is
  shared with the normal submission path; only a new mutation rejects terminal
  parents, so a completed root can replay its accepted receipt.
- `goal_submission_recovery.go`: task-only progress accounting; trusted adapter
  converts all-unknown (including missing judgments) `no_change` to
  `needs_evidence`. Empty next-state fields receive only a technical evidence
  wait. Existing strict hydration and semantic/source validation still run.
  Ref normalization is deliberately limited to whitespace around an exact
  frozen ref. Same-target identical judgments merge evidence; conflicting
  duplicates/unknown/cross-Goal/cross-object refs produce a handled receipt with
  `assessment_accepted=false`, `mutation_committed=false`, legal scoped refs and
  a reason. They do not create an accepted journal, revision, memo or judgment.
  Accepted receipts record `adapted_fields` for impact/judgments/wait_condition.
- `goal_evaluation_submission_runtime.go`: stable accepted projection explicitly
  identifies capability and root/plan/object kind and short targets. Finalization
  recognizes the durable no-progress disposition: request cycle succeeds with
  `disposition=awaiting_evidence`, `assessment_complete=false`, real accepted
  siblings and unresolved IDs. It exits before consuming sources or inventing
  unresolved memos/root acceptance. Fully covered cycles separately report
  `assessment_complete=true`.
- `goal_evaluation_runtime.go`: the only F16 change to this F15-owned file is
  skipping model work when a previously exhausted cycle already has the durable
  disposition. Original claim/succeeded replay and F15 cadence remain in force.
- `eino_model_request_policy.go`: empty narrowed catalogs are legal only with
  explicit forbidden Tool choice. That physical summary request removes tools,
  tool_choice and parallel_tool_calls and restores the existing summary grammar.
  Normal nil/optional/forced catalogs and isolated repair behavior are retained.

The dedicated budget closes Tool selection after three completed native attempts
without a new durable accepted target. Successful new targets reset it, including
multiple Goals. Persistent marker is written under the current claim before the
final summary, preventing later workflow/claim replay from restarting the loop.
Real stale-authority feedback may summarize immediately and remains eligible for
the existing fresh-snapshot replacement; it cannot bypass the finite budget by
repeating optional calls. Provider/database/cancellation failures remain errors.

## Actual RED and GREEN

1. Before implementation, `TestGoalSubmissionRecoveryAcceptedKinds` failed
   against the existing production projection: accepted results lacked both
   capability_name and submission_kind. It passes after implementation.
2. The unknown test directly reproduces the existing strict validator's
   `goal_evaluation_unknown_requires_evidence` for both omitted and explicit
   unknown judgments, then proves the trusted adapter accepts the conservative
   form without success/progress. The strict validator was not relaxed.
3. Additional sensitivity RED was actually executed by temporarily disabling
   only the new adaptation helper / restoring the old changed-digest conflict
   branch, under Python `finally` restoration. Unknown normalization, scoped
   ref/dedup and changed-payload replay all failed. This is an equivalent old
   helper behavior experiment, not a PostgreSQL transaction result. Log:
   `/tmp/f16-helper-red.log`.
4. Native HTTP sensitivity RED was actually executed with the new no-progress
   gate disabled (the previous forced-loop behavior), then restored in `finally`.
   Continuous plan replay and bad criterion each exceeded the hard ninth-request
   test budget, triggering `t.Fatal`. Log: `/tmp/f16-loop-red.log`.
5. GREEN native HTTP with the actual shared transport and Eino/native loop:
   continuous changed-payload plan replay ends in 5 physical requests; continuous
   bad criterion ends in 4; four distinct accepted targets across two Goals
   continue to the fifth summary request. Tool-free request has original summary
   response_format and none of the three Tool wire fields. The scripted invoker
   uses the real new normalization/replay/progress helpers but represents accepted
   storage in memory: this does not claim real database effects.

## Checks actually executed

All commands use `FLUCTLIGHT_LIVE_PROVIDER_TEST=0`, empty
`GO_CORE_TEST_DATABASE_URL`, and `GOCACHE=/tmp/fluctlight-go-cache` as relevant.

- `go -C apps/core-go test -race ./internal/core -run
  'Test(GoalSubmissionRecovery|GoalEvaluation|PhysicalModelRequestPolicy|ToolFinalSchemaSeparation|ConversationPhysical|GoalJudgment|GoalClosure|GoalPlannerPhysical)'
  -count=1 -json`: PASS. `/tmp/f16-green-final.json` records 55 top-level tests:
  47 PASS, 8 SKIP; including subtests: 108 PASS, 14 SKIP, zero failures.
- After the last unresolved-receipt hint change, `go -C apps/core-go test -race
  ./internal/core -run '^TestGoalSubmissionRecovery' -count=1 -json`: PASS,
  `/tmp/f16-green-receipts.json`.
- `go -C apps/core-go vet ./internal/core`: PASS.
- gofmt of owned changed Go files and `git diff --check`: PASS.

Sandbox initially rejected local httptest listening. The same HTTP checks were
rerun outside the default sandbox and passed; that initial environmental failure
is not counted as a business RED.

## Persistence boundary and remaining verification

New tests call `seedWardrobeToolFixture` -> `newIndependentToolE2EFixture` ->
`isolatedCoreTestRepository`, which creates and cleans a unique database:

- `TestGoalSubmissionRecoveryPersistenceHandledCycleAndReplay/replay_plan`:
  SKIP (database URL absent).
- `.../bad_criterion`: SKIP (database URL absent).
- `TestGoalSubmissionRecoveryPersistenceUnknownNoChange`: SKIP (database URL absent).

These cover the real production task, accepted sibling preservation, unchanged
accepted plan digest, no unresolved judgment/source consumption, no model calls
on handled workflow replay, and persisted needs_evidence with active Goal status.
They are implemented and compile, but their database assertions were not executed.
Full repository gates, independent review and spec/report synchronization belong
to the parent session. Real Provider/infra behavior remains NOT_RUN.
