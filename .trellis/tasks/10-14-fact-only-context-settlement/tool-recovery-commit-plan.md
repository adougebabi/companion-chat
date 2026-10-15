# F19 + F20 当前工作提交清单

基线42a2d7c，分支codex/fact-only-reply-vision。F19已呈现的提交确认尚未收到；用户追加F20后，本清单替代goal-planner-commit-plan.md的当前范围。实现与验证均已完成，不push/deploy/归档。

拟一笔：`fix(core): recover planner and reply tool inputs`

源码（11项）：

- apps/core-go/internal/core/adk_conversation_runtime.go
- apps/core-go/internal/core/fact_only_reply_capability.go
- apps/core-go/internal/core/provider_ref_codec.go
- apps/core-go/internal/core/reply_draft_recovery_test.go
- apps/core-go/internal/core/goal_planner.go
- apps/core-go/internal/core/goal_planner_request_policy.go
- apps/core-go/internal/core/goal_planner_commit_recovery.go
- apps/core-go/internal/core/goal_planner_commit_recovery_test.go
- apps/core-go/internal/core/goal_planner_test.go
- apps/core-go/internal/core/goal_set.go
- apps/core-go/internal/core/tool_business_recovery.go

规格与任务上下文（13项）：

- .trellis/spec/backend/goal-planner-contract.md
- .trellis/spec/backend/tool-availability-contract.md
- .trellis/spec/backend/fact-only-reply-contract.md
- .trellis/tasks/10-14-fact-only-context-settlement/prd.md
- .trellis/tasks/10-14-fact-only-context-settlement/implement.md
- .trellis/tasks/10-14-fact-only-context-settlement/implementation-state.md
- .trellis/tasks/10-14-fact-only-context-settlement/implement.jsonl
- .trellis/tasks/10-14-fact-only-context-settlement/check.jsonl
- .trellis/tasks/10-14-fact-only-context-settlement/goal-planner-commit-followup.md
- .trellis/tasks/10-14-fact-only-context-settlement/reply-draft-recovery-followup.md
- .trellis/tasks/10-14-fact-only-context-settlement/research/goal-planner-commit-diagnosis.md
- .trellis/tasks/10-14-fact-only-context-settlement/research/reply-draft-implementation.md
- .trellis/tasks/10-14-fact-only-context-settlement/research/reply-draft-check.md

不包含：开始时既有testdata/turn_path_cost_report.json、F18未跟踪research/commit-plan/verification、private-chat任务未跟踪产物、当前与历史一次性提交清单、本轮生成verification JSON/dirty-state及/tmp日志/源码快照。保存这些排除项，不还原或删除。

实际验证：主线程F19/F20联合PG/race40 PASS事件0SKIP/FAIL；check F20及相关scope23顶层PASS0SKIP/FAIL；完整Go1907 PASS/580SKIP/0FAIL，Core/Workflow race1567PASS/515SKIP/0FAIL；vet/build、typecheck/128tests/build、格式/差异检查通过。完整PG与真实外部服务未运行。

扩大PG的既有人格修复用例失败，主线程在42a2d7c独立快照上复现同失败；保留证据和待处理状态，不算本次全范围PG通过。其他验证/限制详见verification/reply-draft-recovery-results.json。
