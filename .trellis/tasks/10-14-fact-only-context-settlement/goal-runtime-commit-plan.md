# F15/F16 提交计划

基线ec1ed82，分支codex/fact-only-reply-vision。拟一笔本地工作提交：`fix(goal): enforce cadence and recover stalled evaluations`。

包含Evaluation实例级30min周期、Planner真实空位与捕获事件准入、0063 additive迁移、native提交程序兜底/有限收束、原始响应诊断保留、对应回归和契约。所有文件均为本轮已识别改动；无未识别文件。

## 产品代码与回归

- `apps/core-go/cmd/worker/main.go`
- `apps/core-go/internal/core/eino_model_request_policy.go`
- `apps/core-go/internal/core/eino_model_runtime.go`
- `apps/core-go/internal/core/goal_agent_cadence.go`
- `apps/core-go/internal/core/goal_agent_cadence_test.go`
- `apps/core-go/internal/core/goal_assessment_memo_integration_test.go`
- `apps/core-go/internal/core/goal_boundary_regression_test.go`
- `apps/core-go/internal/core/goal_closure_recovery_test.go`
- `apps/core-go/internal/core/goal_closure_runtime_test.go`
- `apps/core-go/internal/core/goal_evaluation_runtime.go`
- `apps/core-go/internal/core/goal_evaluation_submission_runtime.go`
- `apps/core-go/internal/core/goal_evaluation_tools.go`
- `apps/core-go/internal/core/goal_evaluation_tools_integration_test.go`
- `apps/core-go/internal/core/goal_evidence_retention_test.go`
- `apps/core-go/internal/core/goal_planner.go`
- `apps/core-go/internal/core/goal_planner_test.go`
- `apps/core-go/internal/core/goal_recommendation_input_integration_test.go`
- `apps/core-go/internal/core/goal_review_test.go`
- `apps/core-go/internal/core/goal_sources.go`
- `apps/core-go/internal/core/goal_stage_replay_test.go`
- `apps/core-go/internal/core/goal_submission_recovery.go`
- `apps/core-go/internal/core/goal_submission_recovery_integration_test.go`
- `apps/core-go/internal/core/goal_submission_recovery_test.go`
- `apps/core-go/internal/migrations/goal_agent_cadence.go`
- `apps/core-go/internal/migrations/goal_agent_cadence_test.go`
- `apps/core-go/internal/migrations/media_kev_test.go`
- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`

## 可执行契约

- `.trellis/spec/backend/goal-life-consistency-contract.md`
- `.trellis/spec/backend/goal-planner-contract.md`
- `.trellis/spec/backend/quality-guidelines.md`
- `.trellis/spec/backend/structured-turn-contract.md`

## 交付报告

- `docs/goal-agent-runtime-availability.md`

## 任务上下文、研究、检查和计划

- `.trellis/tasks/10-14-fact-only-context-settlement/check.jsonl`
- `.trellis/tasks/10-14-fact-only-context-settlement/design.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/goal-cadence-followup.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/goal-runtime-commit-plan.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/goal-submission-recovery-followup.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/implement.jsonl`
- `.trellis/tasks/10-14-fact-only-context-settlement/implement.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/implementation-state.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/local-check.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/prd.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/context-capacity-contract.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/context-native-contract.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/context-repair-contract.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/goal-cadence-audit.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/goal-cadence-implementation.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/goal-evaluation-authority-contract.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/goal-runtime-check.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/goal-submission-audit.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/goal-submission-recovery-implementation.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/task.json`
- `.trellis/tasks/10-14-fact-only-context-settlement/verification/goal-runtime-check-summary.json`

共54文件：Go 28、spec 4、报告1、任务/证据21。

本地完整Go test/race/vet/build和workspace typecheck/test/build已通过；实际native HTTP/SSE与旧行为敏感性RED/GREEN完成。PostgreSQL时序/并发/升级/持久化用例实际SKIP，部分父容器PASS不算数据库通过；真实Provider/Temporal/Redis/部署NOT_RUN。

待一次确认后，只按该清单git add与git commit，不amend/push/部署，不取消或重试真实workflow。任务保持in_progress等待用户环境验收。
