# 正式链路与基线盘点（2026-09-24）

状态含义：已生效表示入口和权威读写均已查到；部分表示已有领域能力但需求链路断开；缺失表示尚无权威能力。文件锚点是规划期只读调查结果，实施时以代码和测试再次核验。

## 能力接线表

| 能力 | 正式入口 | 文件/符号 | 读取来源 | 写入来源 | 实际生效 | 本次修改 |
| --- | --- | --- | --- | --- | --- | --- |
| 原文分析/激活 | HTTP analysis/activation | `app.go:344` `AnalyzeDescription`; `app.go:1592` `CreateFluctlight` | 用户原文、分析 JSON | analysis source、Foundation revision、Fluctlight | 已生效 | 语义归类与幂等物品初值 |
| Working Persona | Main/WakeUp context | `provider_context.go:111`; `working_persona_persistence.go:288` | Foundation、accepted overlay、compiled row | 初始化/正式修改时编译 | 部分：动态事实可混入且 habits 覆盖键错误 | 限定稳定源、更新规则、迁移 |
| Main | `handleTurn` | `agent_result_adapter.go:278`; `cognition_agent.go:67-121` | ContextProjection、近期消息 | Native Tool、assistant publication、cognition | 已生效 | 投影和新 Tool catalog |
| WakeUp | `wake_up.current` workflow | `agent_result_adapter.go:571-705`; `workflow.go:231-253` | ContextProjection、触发事实 | Native Tool、wakeups/outcome | 已生效 | 持续事项与真实触发/无动作 |
| Tool 原生续接 | Eino ADK | `eino_model_runtime.go:467-557`; `adk_conversation_runtime.go:79-196` | Catalog、Tool result | `ExecuteTool` receipt/领域效果 | 已生效，有正式测试 | 加新能力与独立 E2E；补执行时 surface gate |
| 当前生活 | ContextProjection | `life_context.go:74-87,161-205` | Event > Schedule > pending，Presence 独立 | Event/Plan/Presence commands | 已生效 | 计划与实际活动/结果分离 |
| Goal/Intention | 初始化、Reflection、due workflow | `app.go:1992-2082`; `intention_runtime.go:160-176` | 当前表+revision | 初始化、Reflection | 部分：原生 due outcome refs 未贯通 | 正式读写 Tool、结算关联 |
| Schedule | `schedule.current_day` workflow；`schedule.replan` | `model_tasks.go:173-193`; `schedule_capability.go:13-39` | 初始仅 identity/life_profile；重排读取更多 | accepted version/CAS | 部分：初始缺意愿等 | 输入扩展、保留约束/空闲 |
| Affect | Cognition reducer；`affect_event` | `cognition_growth.go:135-222`; `affect_capability.go:224-327` | inner state/revisions | reducer/事件 Tool | 已生效，需实测全链 | 回归事件→下一轮选择 |
| 外观/当前穿着 | media appearance slot | `app_capability_context.go:96-111`; `capability_core.go:313-329` | 错读 identity.appearance；当前衣着无权威 | Foundation 初值/Visual Identity | 部分/缺失 | 共享身体与穿着权威、正确 snapshot |
| 衣柜与搭配 | 无 | `phase8_contract_matrix_test.go:16-39` | 无 | 无 | 缺失 | 新领域表/Tool/合法获得结果 |
| 人格详情 | `persona.detail` Tool | `persona_detail_capability.go:25-55,87-152` | Foundation+accepted overlay | 只读 | 部分：原始可变文本仍可当现在 | 默认有效值、历史时点标记 |
| 会话摘要 | assistant publication 后 durable intent | `conversation_summary.go:21,71-88,232-410` | 老消息、上版摘要 | revision/covered range | 已生效；语义回归待测 | 状态时间语义与失败覆盖测试 |
| 视觉与媒体 | `media.image.generate`；Visual Identity Agent | `builtin_capabilities.go:245-297`; `visual_identity_agent.go:94-166` | frozen life/appearance/identity | media intent、asset | 部分：外观 slot 错位、无当前穿着 | 当前版本绑定与结果新鲜度 |
| Prompt 预算 | Main/WakeUp assembler | `prompt_context_assembler.go:250-285` | messages/tools/schema | diagnostics | 部分：只估算，未执行总预算 | 完整请求门禁及分项 Trace |

## 初始化语义去向表

| 原始语义 | 结构化字段 | 权威存储 | 主体/作用域 | 运行时投影 | 查询入口 | 最终请求证据/缺口 |
| --- | --- | --- | --- | --- | --- | --- |
| 原始人格文本 | `source_text`, analysis projection | immutable analysis + Foundation source link (`app.go:377-419`) | Owner/Fluctlight | 不逐轮注入 | 原始/历史明确查询 | `initialization_contract_test.go:148` 证明隔离 |
| 身份与机制 | `core_persona.identity/personality/behavioral_policy` | `fluctlights.core_persona` + revision | shared 与 speaking profile | compiled portrait System | `persona.detail` | `working_persona_chain_test.go:481-536` |
| 稳定偏好/习惯 | `life_profile.preferences/life_habits` 及 profile 来源 | Foundation/overlay；未来有效习惯 revision | shared 输入/特定 profile 偏好 | 应在 portrait 常驻 | `persona.detail`/习惯 Tool | `persona_compilation.go:287-295` 错读 `habits` |
| 发长/发色/临时发型/伤势 | `life_profile.appearance` 自由对象 | 仅 Foundation 初值；无当前身体 authority | 应为 shared | 当前被 portrait 裁或误编，无一致 Runtime | 原始详情/Visual Identity | `working_persona.go:46-53` 与 `provider_context.go:439` 断面 |
| 当前衣着/已拥有衣物 | 初始化无独立字段 | 无 | 应为 shared；未知所有权保留 | 无 | 无 | `app.go:330-334` 仅偏好，未闭环 |
| 生活背景/课程约束 | `social_background/recurring_commitments` | Foundation | shared | 初始日程输入；普通 Runtime 缺完整约束 | `persona.detail` | `model_tasks.go:173-193`，需 Provider 请求验证 |
| Goal/Intention | structured initial goals/intentions | 当前表+revision | speaking profile | `ContextProjection` 按 profile | 现有领域 API，缺独立 Tool | `intelligence.go:215-225`；due 结果 refs 断线 |
| 情绪 | initial inner state + later event | inner state + revisions | Fluctlight 当前状态 | `current_state` | 当前状态 API | `cognition_growth.go:135-222`，需实测下一轮 |
| 日程/实际活动 | accepted schedule；Event | versioned schedule；life_events | shared life | Life Context/Runtime | Schedule/Event API | `life_context.go:161-205`；计划非完成 |
| 关系/历史消息 | relationship seeds；messages/summary | profile-aware relationship；conversation records | Owner/Fluctlight/profile | scope-filtered Runtime 与摘要/近期原文 | lookup/recall | `provider_context.go:85-171`; 需最终请求检查 |

## 已核验的关键断点

1. 编译器将整个 `life_profile` 放入可引用源（`persona_compilation.go:124`），但仅用关键词形式移除少量 `current_*` 字段（`:158-163`）；production compiled row 没有执行 `working_persona.go:232` 的确定性 F-06 裁剪。
2. canonical `life_habits` 与覆盖检查/基线 fallback 的 `habits` 不一致（`persona_compilation.go:287-295`；`working_persona_persistence.go:194-200`）。初始化编译失败可回落不完整 baseline（`:232-245`）。
3. `SlotAppearance` 读 `Identity["appearance"]`，初始化保存 `LifeProfile["appearance"]`（`app_capability_context.go:106`；`app.go:330-334`）。
4. `intention.due` 的阶段 refs 被验证，但 settlement 未复制；`buildActionOutcomes` 从 settlement 取 refs，attempt 因空 refs 不结算（`agent_result_adapter.go:843-853`；`action_outcome.go:83-103,277-280`）。
5. 初始日程只传 identity/life_profile，8–16 连续时段全日覆盖指令可能鼓励填满模式；没有 Goals/Intentions/current facts（`model_tasks.go:173-193`）。
6. Prompt assembler 无条件接受所有 optional candidates，统计 total 后返回；`MaxInputTokens` 等未在这条生产路径上执行（`prompt_context_assembler.go:250-285`）。
7. `ExecuteTool` 缺少执行时 `SupportsSurface` 再校验；正式 catalog 目前有过滤（`tool_execution.go:139-171`；`tool_contract.go:147`）。
8. 受控 Provider/PostgreSQL 测试证明 Eino 真实 ToolCall→Tool result→再次模型调用（`eino_adk_runtime_test.go:707-751,987-1045`）；这不等于真实外部模型行为评估。

## 验收环境基线

- 当前 migration head 是 `0035_working_persona`（`migrations/runner.go:12-15`）。新增 durable schema 进入后续版本，同时更新 clean start 与 previous-release upgrade 测试。
- 项目 README 的标准检查为 `go -C apps/core-go test -race ./...`、`vet`、`build`、`pnpm generate/typecheck/test/build`。真实模型 smoke 需要显式 Provider URL/model；数据库 E2E 需要 `GO_CORE_TEST_DATABASE_URL`。缺失时记录 BLOCKED。
- 09-22/09-23 任务里受控 Tool/人格链已验，但真实 Provider + ComfyUI 完整闭环仍未验，不能沿用为本任务的 live acceptance。
