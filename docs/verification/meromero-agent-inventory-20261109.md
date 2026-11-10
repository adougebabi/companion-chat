# Agent 入口盘点（起始源码）

基线 `ffa808bc81cc4d800376432216f3ea679da8de30`。这是 T01 的工作底稿，不表示各任务已通过 MeroMero 验收。具体 Prompt 和输入投影另见 [提示词组成清单](../llm-prompt-composition.md)；本表中的行号是基线定位，变更后应以符号重新核对。

后续实施已在 `agent_model_routing.go` 增加 Agent 与 variant 覆盖。以下表格保留起始基线；当前主认知、WakeUp、NPC 默认继承 `conversation_reply`，其他文本内部任务保留旧默认，具体覆盖以运行时配置为准。视觉和 embedding 独立。隔离 PostgreSQL 与受控 Provider 已验证部分配置；全部变体的真实 Provider 生效尚未验收。

起始 `formal_agents.go` 注册 30 个 ID。`G` = `providerBindingRole` 将逻辑角色归入 `generic_llm`；`R` = `conversation_reply`；`V` = `vision_understanding`。绑定决定实际 Provider/模型，不能仅凭角色名断言它是 Qwen 或 MeroMero。真实快照优先于当前绑定；历史通用绑定缺失时还有旧角色查询。下表的“生产上游”表示静态找到非测试调用，不等于功能已启用或真实模型已验证。

所有 FormalAgent 进入共同 Eino Runner；有工具的任务使用原生 ADK loop，工具集合受 surface、运行作用域与权限约束，注册表中的 mandatory tools 不是完整可调用集合。结构化任务使用各自 schema 和业务验证；文本任务仍须经过其领域解析与发布校验。`—` 表示本任务的模型调用无工具，不表示外层没有后续副作用。

| Agent / variant | 生产调用或状态（`apps/core-go/internal/`） | 绑定 | 工具；输出和下游 |
| --- | --- | --- | --- |
| `conversation_cognition` / `conversation_turn_response` | `core/agent_result_adapter.go:296` → `core/cognition_agent.go:146`；恢复见 `core/reply_continuation.go:145` | G | Conversation surface；内部认知 DTO，实际回复和动作经工具提交。迁移时必须保留 claims、influences、偏好和 Goal 候选结算。 |
| `conversation_reply` / `conversation_reply_text` | `core/reply_failure_policy.go:139` → `core/reply_provider.go:199` | R，草稿冻结绑定 | 私有图片与人格详情工具；用户正文先成草稿，外层唯一发布。当前存在独立 Writer。 |
| `reply_action_check` / `reply_action_consistency` | `core/reply_action_consistency.go:33`、`:49` | G | —；内部核验未发布候选与动作回执。 |
| `wake_up` / `wake_up_response` | `core/agent_result_adapter.go:927`；调度 `workflow/workflow.go:941` | G | WakeUp surface；可静默或经授权工具行动，真实发布和副作用在工具侧。 |
| `takeover_judge` | `core/conversation_runtime.go:116`、`:121`；未找到非测试生产上游 | G | —；内部判断。保持注册，不据此激活。 |
| `takeover_reply` | `core/conversation_runtime.go:130`、`:138`；未找到非测试生产上游 | G | 由调用方提供的 Conversation 工具；内部 turn contract。保持注册，不据此激活。 |
| `wardrobe_preparation` / `wardrobe_preparation_v1` | `workflow/wardrobe.go:50` → `core/wardrobe_preparation.go:315` | G | —；库存/方案候选，后续独立事务提交并经真实 wear 执行。 |
| `initialization` / `initialization_response` | `core/app.go:380`、`core/studio_foundation.go:283` → `core/model_tasks.go:62` | G | —；身份和人格初稿，外层控制激活与落库。 |
| `persona_compilation` / `persona_compilation_response` | `core/working_persona_persistence.go:303`、`core/reflection_runtime_v2.go:309` → `core/persona_compilation.go:251` | G | —；内部 Working Persona，后续校验与持久化。 |
| `media_prompt` / `media_prompt_text`、`media_subject_parse`、`media_prompt_normalized` | `core/media.go:172` → `core/model_tasks.go:124`；`core/media_capture_plan.go:451`、`core/media_prompt_branches.go:229` | G | —；渲染提示词或结构化主体/规范化结果，冻结意图和实际渲染由媒体链负责。 |
| `media_quality` / `media_quality_acceptance_response` | `core/media_quality.go:211` → `core/model_tasks.go:267` | V | —；真实图片输入，内部质量结论，外层决定交付或待审核。 |
| `visual_identity_vision` / `visual_identity_vision_response` | `core/model_tasks.go:314`；未找到生产上游 | V | —；图片观察。不得视为当前生产视觉链已运行。 |
| `visual_identity_patch` / `visual_identity_patch_response` | `core/model_tasks.go:336`；未找到生产上游 | G | —；内部修正判断。不得擅自激活。 |
| `visual_identity` / `visual_identity_agent_response` | `core/visual_identity.go:1152` → `core/visual_identity_agent.go:187` | G；实际视觉阶段可冻结 V | 分阶段生成、审核、finalize 工具；真实工具和事务负责视觉身份生效。 |
| `conversation_summary` / `conversation_summary_v1`、`conversation_segment_v1`、`conversation_daily_memory_v1` | `core/runtime_summary.go:161`、`core/conversation_segment.go:298`、`core/conversation_daily_memory.go:191` → `core/model_tasks.go:367`、`:417`、`:437` | G | —；来源限定的累计摘要、片段或每日记忆材料，由各自领域链保存。 |
| `schedule_generation` / `schedule_response` | `core/schedule_generation.go:66` → `core/model_tasks.go:527` | G | —；日程候选，外层验证并保存，不等于实际到场。 |
| `native_cognition` / `native_cognition_response`、`owner_life_request_decision`、`npc_opportunity_decision` | `core/agent_result_adapter.go:1258`、`core/owner_life_requests.go:426`、`core/world_opportunity_cognition.go:32` | G | 按变体授予 Native surface 或指定工具；内部 DTO，真实动作由工具提交。 |
| `studio_assistant` / `studio_assistant_v1` | `workflow/studio.go:42` → `core/studio_runtime.go:201` | G | —；Owner 可见的隔离定义建议，不能直接改变角色生活。 |
| `studio_text_preview` / `studio_preview_text` | `workflow/studio.go:42` → `core/studio_runtime.go:240` | R | —；Owner 可见虚构预览，由 Studio 外层管理。 |
| `npc_response` / `npc_response_v1`、`npc_moment_response_v1` | `core/world_encounter_service.go:42`、`core/moment_npc_interactions.go:125` | G | —；NPC 自身定义与知识下的结构化回应，外层控制互动生效。 |
| `daily_review` / `daily_review_response` | `core/agent_result_adapter.go:1438` → `core/model_tasks.go:670` | G | Autonomy surface；内部日审，可经工具产生经授权效果。 |
| `diary_material` / `diary_material_v1` | `workflow/diary.go:42` → `core/diary_runtime.go:205` | G | —；选择真实素材，不是最终日记。 |
| `diary_expression` / `diary_expression_text` | `workflow/diary.go:42` → `core/diary_runtime.go:270` | R | —；日记正文，外层保存。 |
| `persistent_switch` / `persistent_switch_assessment` | `workflow/owner_persona.go:43` → `core/owner_persona_execution.go:89` → `core/model_tasks.go:694` | G | —；内部 keep/switch 判断，Owner 工作流重检版本后提交。旧 post-candidate 包装未找到生产上游；不能据此删除 Owner 路径。 |
| `reflection` / `reflection_proposal_v2` | `core/reflection_runtime_v2.go:167` → `core/model_tasks.go:743` | G | —；有范围的内部反思候选，外层校验后可更新持久状态。 |
| `schedule_replan` / `schedule_replan_plan` | `core/schedule_capability.go:74` → `core/model_tasks.go:766` | G | —；替换日程候选，外层 Capability 执行。 |
| `goal_planner` / `goal_planner_v1` | `workflow/workflow.go:852` → `core/goal_planner.go:301` | G | `goal_planner.query/commit`；真实目标提交受事务和原成功条件控制。 |
| `goal_evaluation` / `goal_evaluation_v1` | `core/goal_evaluation_runtime.go:478`、`:496` → `core/goal_evaluation_task.go:262` | G | GoalEvaluation 专用提交工具；证据及成功条件由 Core 校验。 |
| `virtual_activity_result` / `virtual_activity_result` | `core/life_activity_capabilities.go:493` → `core/model_tasks.go:492` | G | —；购物/理发内部结果，外层活动结算产生真实状态。 |
| `world_event_generation` / `world_event_generation_v1` | `workflow/world_events.go:74` → `core/world_event_generation.go:303` | G | —；世界事件提议，准入、事务和通知由外层负责。 |

后续新增 `expression_learning` / `expression_learning_v1`，使用现有正式结构化运行入口，默认继承 `conversation_reply` 文本模型，可按 Agent/变体覆盖。Owner 可从已发送人类消息发起提取；Core 核对来源身份、版本和连续原文，候选须经审核才可被 fact-only Writer 或实验 `direct` 主 Agent 按会话及触发词选用，发布前再次校验。隔离数据库和受控请求已覆盖部分链路；真实模型和 UI 到运行的完整链路未验收。

## 非注册入口及尚需核实

- Provider 修复和传输重试复用同一物理模型；Writer 格式修复沿冻结 assignment；显式旧 fallback 可重新读取 `generic_llm`。视觉探针、embedding 和独立 Kev 不属于上述 30 个 Agent。
- Summary 与 segment 在模型调用前后分别读取 assignment；Writer、日记、Studio 在 Runner 前冻结配置；真实视觉身份阶段可将 V 快照绑定到认知逻辑角色。只在 Runner 增加覆盖会造成错误归属或覆盖视觉配置。
- `agent_runs` 不是所有后台 typed task 的持久入口，也未完整保存模型配置快照。具体版本、各变体的 prompt hash、全部非注册物理请求及每条副作用的验收证据仍需逐项补齐；因此 T01 尚未标记 VERIFIED。
