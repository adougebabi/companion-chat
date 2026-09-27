# 模型能力参数与结果契约

## Goal

让模型可用 Tool 只要求其必须提供的语义与目标选择，新增结果能用于后续更新，失败反馈可恢复。

## Confirmed facts

- 用户无法确定偶发失败来自哪个 Tool，没有原始日志；本任务按可复现契约问题修复，不声称定位原事件。
- Core 已注入 actor、fluctlight、conversation、operation、source fact 与多数 revision，模型不应重复传这些值（`apps/core-go/internal/core/adk_conversation_runtime.go:137-143`）。
- `intention.decide` 等 operation 在 schema 与运行时条件必填上有错位；`memory_event` create 输出 `memory_id`，但 revise 要 opaque `target_ref`（`apps/core-go/internal/core/intention_capabilities.go:130-184`、`apps/core-go/internal/core/memory_intelligence.go:20-95`）。
- Visual Identity 现场报错显示：普通 `ErrInvalidArguments` 经 Prepare 默认变成 `capability_prepare_failed` 且 retryable，ADK ToolNode 随之终止（`apps/core-go/internal/core/tool_execution.go:210-216`、`apps/core-go/internal/core/adk_conversation_runtime.go:216-223`）。
- native ToolCall ID 是 transport 字段，不属于业务 Tool 参数；缺失会报 `adk_tool_call_id_required`（`apps/core-go/internal/core/adk_conversation_runtime.go:94-109`）。

## Requirements

- R5.1：盘点 conversation surface 的查询/新增/更新 Tool。唯一当前目标由 Core 绑定；确需显式目标或语义字段时，在紧凑 Tool 描述与安全失败结果中说明。优先修 `intention.decide/inspect`、`memory_event`、`active_memory_event`、`life.activity.start/advance`、`wardrobe.inspect` 和适用的 persona Tool，不得让展开的 schema 超过 Prompt 预算。
- R5.2：由 Core 已知且唯一的身份、当前 profile、due schedule/单一当前活动等自动绑定；多个候选时返回安全且可选择的目标，不猜测。
- R5.3：新增记忆结果与后续更新目标字段闭环，使用安全 opaque ref；模型不依赖记忆数据库 ID 或 revision。其他 Tool 需要用户选择的对象标识保留现有兼容输出。
- R5.4：变更结果尽可能短，但保留 status、失败原因、重试/后续目标；查询结果保留所查内容。内部审计 receipt 仍完整。
- R5.5：区分业务目标缺失与 native ToolCall transport ID 缺失，使诊断能判别两者。
- R5.6：模型可纠正的 schema/缺参错误应返回非 retryable Tool 结果与安全字段提示；依赖、授权及数据库故障仍可终止运行。此通用分类是视觉身份有界修正调用的前置条件。

## Acceptance criteria

- [ ] 所改 Tool 的紧凑模型契约说明目标选择；唯一目标由 Core 自动绑定，0/多目标给出明确且可恢复的结果；Tool schema 与最终结构合计仍在默认 Prompt 预算内。
- [ ] 新增一条记忆后可在同一轮或后续轮安全更新，不需猜数据库 ID；revision/owner 校验仍生效。
- [ ] 确定的 `intention_id_required` 等路径有回归测试；native ToolCall ID 异常不会误导为业务 `id` 参数缺失。
- [ ] `commit_review.observations` 类型错误等 schema 失败不会被当成可重试系统故障终止 ADK，后续模型调用能收到安全错误结果。
- [ ] 变更结果简短、查询结果完整，ADK 后续模型调用可利用结果恢复或继续。

## Out of scope

- 不提供运行时安装或热更新 Agent/Tool 定义的管理 API。
- 不保证在没有现场 trace 时复现用户当时那一次失败。
