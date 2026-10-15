# 本轮一次提交计划

拟提交：`fix(core): recover tool calls with truthful no-effect receipts`。

范围：F17/F18的共有handled、参数修复/Kev告警、Goal Review/可选引用、各域确定性恢复、回滚/刷新/结算与对应测试和spec。仅以下本轮hunk，按verification/tool-owned-changes.patch暂存；不整文件暂存共享reply文件。补丁来源及SHA见verification/tool-snapshot-manifest.json。尚未暂存/提交，不push。

验收：独立可提交快照完整Go/类型检查/build通过，92个重点PG/race和10个完整性回归通过；完整PG基线/中间快照均未全绿，限制见verification/tool-availability-results.json。整工作区另一项并行任务未计入本轮验收。

## 文件（102项）

- `.trellis/spec/backend/fact-only-reply-contract.md`
- `.trellis/spec/backend/index.md`
- `.trellis/spec/backend/kev-decision-contract.md`
- `.trellis/spec/backend/quality-guidelines.md`
- `.trellis/spec/backend/tool-availability-contract.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/check.jsonl`
- `.trellis/tasks/10-14-fact-only-context-settlement/design.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/implement.jsonl`
- `.trellis/tasks/10-14-fact-only-context-settlement/implement.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/implementation-state.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/prd.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/reply-evidence-followup.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/reply-evidence-binding-audit.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/tool-availability-independent-review.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/tool-common-implementation.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/task.json`
- `.trellis/tasks/10-14-fact-only-context-settlement/tool-availability-audit.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/tool-availability-followup.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/verification/tool-availability-results.json`
- `.trellis/tasks/10-14-fact-only-context-settlement/verification/tool-inventory-before.json`
- `.trellis/tasks/10-14-fact-only-context-settlement/verification/tool-snapshot-manifest.json`
- `apps/core-go/internal/capability/argument_normalization.go`
- `apps/core-go/internal/capability/capability.go`
- `apps/core-go/internal/capability/handled.go`
- `apps/core-go/internal/capability/handled_test.go`
- `apps/core-go/internal/capability/support.go`
- `apps/core-go/internal/core/action_outcome.go`
- `apps/core-go/internal/core/active_memory_tool_test.go`
- `apps/core-go/internal/core/actor_facts.go`
- `apps/core-go/internal/core/actor_facts_test.go`
- `apps/core-go/internal/core/adk_conversation_runtime.go`
- `apps/core-go/internal/core/affect_capability.go`
- `apps/core-go/internal/core/agent_result_adapter.go`
- `apps/core-go/internal/core/appearance_style_capability_test.go`
- `apps/core-go/internal/core/builtin_capabilities.go`
- `apps/core-go/internal/core/capability_core.go`
- `apps/core-go/internal/core/capability_requests.go`
- `apps/core-go/internal/core/fact_only_moment_capability.go`
- `apps/core-go/internal/core/fact_only_moment_test.go`
- `apps/core-go/internal/core/fact_only_reply_capability.go`
- `apps/core-go/internal/core/goal_agent_cadence_test.go`
- `apps/core-go/internal/core/goal_capabilities.go`
- `apps/core-go/internal/core/goal_evaluation_submission_runtime.go`
- `apps/core-go/internal/core/goal_evaluation_task.go`
- `apps/core-go/internal/core/goal_evaluation_tools.go`
- `apps/core-go/internal/core/goal_evaluation_tools_integration_test.go`
- `apps/core-go/internal/core/goal_evaluation_validation_test.go`
- `apps/core-go/internal/core/goal_persona_scope_test.go`
- `apps/core-go/internal/core/goal_planner.go`
- `apps/core-go/internal/core/goal_planner_request_policy.go`
- `apps/core-go/internal/core/goal_planner_request_policy_test.go`
- `apps/core-go/internal/core/goal_planner_test.go`
- `apps/core-go/internal/core/goal_review.go`
- `apps/core-go/internal/core/goal_review_test.go`
- `apps/core-go/internal/core/goal_sources.go`
- `apps/core-go/internal/core/habit_capabilities.go`
- `apps/core-go/internal/core/independent_tools_e2e_test.go`
- `apps/core-go/internal/core/intention_capabilities.go`
- `apps/core-go/internal/core/item_use_capability.go`
- `apps/core-go/internal/core/item_use_capability_test.go`
- `apps/core-go/internal/core/kev_persona.go`
- `apps/core-go/internal/core/kev_test.go`
- `apps/core-go/internal/core/kev_three_tool_test.go`
- `apps/core-go/internal/core/kev_tools.go`
- `apps/core-go/internal/core/life_activity_capabilities.go`
- `apps/core-go/internal/core/life_activity_resolution.go`
- `apps/core-go/internal/core/life_capability_plans.go`
- `apps/core-go/internal/core/native_tool_contract_test.go`
- `apps/core-go/internal/core/persona_action_capabilities_test.go`
- `apps/core-go/internal/core/persona_action_service.go`
- `apps/core-go/internal/core/persona_detail_capability.go`
- `apps/core-go/internal/core/persona_detail_capability_test.go`
- `apps/core-go/internal/core/provider_ref_codec.go`
- `apps/core-go/internal/core/relationship_capability.go`
- `apps/core-go/internal/core/reply_evidence_recovery_test.go`
- `apps/core-go/internal/core/reply_image_request.go`
- `apps/core-go/internal/core/reply_media.go`
- `apps/core-go/internal/core/schedule_capability.go`
- `apps/core-go/internal/core/schedule_edit_capability.go`
- `apps/core-go/internal/core/schedule_inspect_capability.go`
- `apps/core-go/internal/core/scheduled_activity_capability.go`
- `apps/core-go/internal/core/sleep_cycle_test.go`
- `apps/core-go/internal/core/tool_availability_common_test.go`
- `apps/core-go/internal/core/tool_availability_domains_test.go`
- `apps/core-go/internal/core/tool_availability_integration_test.go`
- `apps/core-go/internal/core/tool_business_recovery.go`
- `apps/core-go/internal/core/tool_contract.go`
- `apps/core-go/internal/core/tool_contract_test.go`
- `apps/core-go/internal/core/tool_execution.go`
- `apps/core-go/internal/core/tool_execution_ledger.go`
- `apps/core-go/internal/core/tool_execution_test.go`
- `apps/core-go/internal/core/tool_publication_test.go`
- `apps/core-go/internal/core/tool_recovery.go`
- `apps/core-go/internal/core/tool_semantic_recovery.go`
- `apps/core-go/internal/core/turn_takeover_f02_test.go`
- `apps/core-go/internal/core/visual_identity_tools_test.go`
- `apps/core-go/internal/core/wardrobe_borrow_capability.go`
- `apps/core-go/internal/core/wardrobe_borrow_capability_test.go`
- `apps/core-go/internal/core/wardrobe_capabilities.go`
- `apps/core-go/internal/core/wardrobe_capabilities_test.go`
- `apps/core-go/internal/core/wardrobe_event_test.go`
- `apps/web/src/lib/fluctlight-display.ts`

## 明确排除

- 另一项 `.trellis/tasks/10-14-private-chat-persona-fidelity/` 及其新增Expression/Projection源码、测试数据。
- `cognition_agent.go`、`fact_only_context_views.go`、`private_reply_run.go`、`reply_context.go`、`reply_vision_settings.go` 等另一项任务hunk。
- 三个共享reply文件中所有Expression/Projection/checkpoint hunk；本补丁只含本轮可用性hunk。
- 测试生成的 `apps/core-go/internal/core/testdata/turn_path_cost_report.json`。
- 本commit-plan和派生owned patch本身仅供审阅，不递归写入本补丁。

按仓库workflow.md阶段3.4，一次确认此计划后才执行。若HEAD/index已被另一任务改变，先检查补丁适用性和实际暂存diff，不能覆盖他人index或工作区。
