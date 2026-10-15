# F19 可审阅提交清单

基线42a2d7c；分支codex/fact-only-reply-vision。本轮实施已完成，真实环境验收待用户；按workflow3.4仅在本轮最终清单确认后提交，不push、不归档。

## 一笔工作提交

`fix(core): recover goal planner review commits`

源码（8项）：

- apps/core-go/internal/core/adk_conversation_runtime.go
- apps/core-go/internal/core/goal_planner.go
- apps/core-go/internal/core/goal_planner_request_policy.go
- apps/core-go/internal/core/goal_planner_commit_recovery.go
- apps/core-go/internal/core/goal_planner_commit_recovery_test.go
- apps/core-go/internal/core/goal_planner_test.go
- apps/core-go/internal/core/goal_set.go
- apps/core-go/internal/core/tool_business_recovery.go

可执行规格与本轮任务上下文（9项）：

- .trellis/spec/backend/goal-planner-contract.md
- .trellis/spec/backend/tool-availability-contract.md
- .trellis/tasks/10-14-fact-only-context-settlement/prd.md
- .trellis/tasks/10-14-fact-only-context-settlement/implement.md
- .trellis/tasks/10-14-fact-only-context-settlement/implementation-state.md
- .trellis/tasks/10-14-fact-only-context-settlement/implement.jsonl
- .trellis/tasks/10-14-fact-only-context-settlement/check.jsonl
- .trellis/tasks/10-14-fact-only-context-settlement/goal-planner-commit-followup.md
- .trellis/tasks/10-14-fact-only-context-settlement/research/goal-planner-commit-diagnosis.md

## 不纳入该提交

- 开始时已修改的apps/core-go/internal/core/testdata/turn_path_cost_report.json。
- 开始时未跟踪的F18研究/commit-plan/verification产物（tool-availability-independent-review.md、tool-common-implementation.md、tool-availability-commit-plan.md及旧verification产物）。
- .trellis/tasks/10-14-private-chat-persona-fidelity/下开始时未跟踪的context manifests、commit-plan、implementation-state、research、source和verification文件。
- 本轮一次性goal-planner-commit-plan.md，生成的verification/goal-planner-commit-results.json及goal-planner-commit-dirty-state.txt，/tmp原始测试日志/源码快照。

## 已执行验证

临时PG/race定向39 PASS事件、0 SKIP/FAIL；Go全量1898 PASS、574 SKIP、0 FAIL；Core/Workflow race1558 PASS、509 SKIP、0 FAIL；vet/build、工作区typecheck/128tests/build、gofmt和diff-check通过。PG定向的SQL真实执行与脚本Provider分开标记；未运行完整PG套件/真实外部服务。计数不等同于唯一业务效果数。

历史基线验证：42a2d7c的StandaloneReplayConflictAndProtection也失败，本轮校正既有no-effect断言并验证原Goal保持active/protected。独立审阅的P2已修正且有回归。
