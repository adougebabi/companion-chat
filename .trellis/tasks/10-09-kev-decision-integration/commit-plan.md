# Proposed feedback-fix commit

`fix(runtime): serialize cognitive runs and gate goal evaluation on new evidence`

Only this feedback round: navigation guards/regressions, 0057 logical leases and entry/commit fences, Goal source queue conditions/backoff, corresponding tests/spec/report/task updates. Initial checkout was clean at c4cd44e; all dirty paths below were edited this round. No push is proposed.

## Files

- `.trellis/spec/backend/fluctlight-provider-queue-contract.md`
- `.trellis/spec/backend/goal-planner-contract.md`
- `.trellis/spec/backend/kev-decision-contract.md`
- `.trellis/spec/frontend/state-management.md`
- `.trellis/tasks/10-09-kev-decision-integration/design.md`
- `.trellis/tasks/10-09-kev-decision-integration/implement.md`
- `.trellis/tasks/10-09-kev-decision-integration/prd.md`
- `.trellis/tasks/10-09-kev-decision-integration/task.json`
- `apps/core-go/internal/core/agent_result_adapter.go`
- `apps/core-go/internal/core/app.go`
- `apps/core-go/internal/core/cognition.go`
- `apps/core-go/internal/core/goal_assessment_memo.go`
- `apps/core-go/internal/core/goal_capabilities.go`
- `apps/core-go/internal/core/goal_evaluation_runtime.go`
- `apps/core-go/internal/core/goal_planner.go`
- `apps/core-go/internal/core/goal_sources.go`
- `apps/core-go/internal/core/workflow_ops.go`
- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`
- `apps/web/src/App.vue`
- `apps/web/src/app/navigation.ts`
- `docs/kev-integration-report.md`
- `apps/core-go/internal/core/goal_candidate_queue_test.go`
- `apps/core-go/internal/core/logical_agent_run.go`
- `apps/core-go/internal/core/logical_agent_run_test.go`
- `apps/core-go/internal/migrations/logical_agent_leases.go`
- `apps/web/test/kev-routing.test.mjs`
