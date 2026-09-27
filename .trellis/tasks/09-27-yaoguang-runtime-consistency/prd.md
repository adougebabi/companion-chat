# 摇光运行时上下文与生活状态一致性修复

## Goal

让摇光生成视觉身份、理解当前时间与生活状态、调用能力及展示对话诊断时，使用清晰且一致的事实，并让失败信息足以定位而不会把内部引用当作用户需要提供的参数。

## Background and confirmed facts

- 用户报告七类现象；视觉身份问题现已由现场错误明确定位为 `visual_identity.commit_review` 的 `observations` 参数形态失败，原先怀疑的随机种子不是本次故障。
- 当前媒体提交路径已在 ComfyUI workflow 中把 `{{seed}}` 替换成一次生成所共用的整数（`apps/core-go/internal/core/media.go:499-607`）。本任务不再追查 seed 重试语义。
- `visual_identity.commit_review` 要求 `observations` 为字符串数组（`apps/core-go/internal/core/visual_identity_tools.go:121-140`），而旧 Vision response schema 使用对象（`apps/core-go/internal/core/provider_schemas.go:662-669`）；现场 ToolCall 提交了非数组值。参数校验失败进入 `capability_prepare_failed`，其默认 retryable 分类使 ADK ToolNode 终止（`apps/core-go/internal/core/tool_execution.go:210-216`、`apps/core-go/internal/core/adk_conversation_runtime.go:216-223`）。
- `[RUNTIME CONTEXT]` 是经筛选的投影；`active_memory` 是记忆片段，内容里带 `ref`（`apps/core-go/internal/core/prompt_context_assembler.go:415-447`、`apps/core-go/internal/core/provider_context.go:1359-1378`）。`active_memory:ctx_*` 是用于非 create 记忆操作的冻结目标引用，不能直接删除（`apps/core-go/internal/core/active_memory.go:36-66,83-135`）。普通对话里更细的 scene/schedule/presence refs 可能可按 surface 省略（`apps/core-go/internal/core/provider_context.go:385-404,670-707`）。
- 对话总结异步生成，最近 24 条消息暂不进总结；发送给模型的原始历史最多 8 条（`apps/core-go/internal/core/conversation_summary.go:146-203`、`apps/core-go/internal/core/provider_context.go:222-272`），因此存在上下文连续性空档风险。
- 人格时区从 `identity.timezone` 读取；当前时间会按该时区格式化（`apps/core-go/internal/core/life_context.go:91-105`、`apps/core-go/internal/core/intelligence.go:541-560`）。已有日程分支返回值缺少 `timezone`，但 workflow 用它计算当地午夜（`apps/core-go/internal/core/workflow_ops.go:775-776`、`apps/core-go/internal/workflow/workflow.go:498-512`）。
- Life Context 从有效 Event 或当前日程读取，活动 run 单独读取。临时 scene event 默认两小时；活动可能继续处于 `in_progress/deferred`，形成用户描述的分裂（`apps/core-go/internal/core/life_context.go:161-212`、`apps/core-go/internal/core/life_activity_capabilities.go:190-200`、`apps/core-go/internal/core/effective_life.go:239-265`）。
- Tool 调用有模型生成的 call ID 和 Core 派生的业务 operation ID；部分业务能力还要求资源 ID。用户无法确定失败能力，不能把失败直接归因到其中任何一种 ID（`apps/core-go/internal/core/adk_conversation_runtime.go:109-143`、`apps/core-go/internal/core/tool_execution.go:71-87`）。`intention.decide` 等存在 provider schema 与运行时条件必填不一致（`apps/core-go/internal/core/intention_capabilities.go:130-184`）。
- 用户澄清：日程、活动、场景应当搭配；事件到期且摇光未明确延期时，事件就结束。购物事件的结束不代表一定买到了东西；购买结果与事件终态必须分开。
- 用户澄清：摇光自己的时间（日程、她发送聊天时）以显式设定时区为主；没有显式设定时沿用初始化时识别的时区。用户消息按用户发送当时的时区呈现，不随后来的设备/设置时区变化而改写。
- ADK 可以在一轮对话内多次物理调用模型和工具，诊断中心按物理 model run 显示 Prompt/Response；聊天浏览器边界只显示最终公开的消息流（`apps/core-go/internal/core/eino_model_runtime.go:72-164`、`apps/web/src/views/DiagnosticsView.vue:146`、`apps/core-go/internal/httpapi/browser/ndjson.go:526-633`）。

## Requirements

- R1：修复 Visual Identity 自动审查的 `commit_review.observations` 参数契约，使有界的数组、单条文本或结构化观察对象都能规范化为同一内部表示；参数形态错误不应终止整个 ADK 运行，而应产生可纠正的 Tool 结果。最终仍须只提交一次有效审查。
- R2：审视并精简模型可见的 Runtime Context 字段。保留记忆更新、安全归因真正需要的 opaque ref，省略普通对话无专门消费价值的细粒度来源引用，同时保留运行所需的语义与记忆来源信息。
- R3：当前时间、人格显式时区或初始化识别时区、当地日程的日界计算必须一致；摇光消息按摇光时区、用户消息按发送时捕获的用户时区显示；其他诊断时间应明确标注显示时区。
- R4：日程、场景、活动须遵守统一的通用事件生命周期。事件有效时其场景与活动共同生效；到期且无明确延期时共同结束，再按当前日程解析；事件结束和业务结果成功（例如买到衣服）是不同事实。
- R5：逐项审视模型可用查询/新增/更新能力的输入及输出，把可由 Core 安全推导的标识和上下文从模型参数移走；条件必填与 schema 一致，新增输出能为后续更新提供可复用的安全目标引用；保留更新目标不唯一时所必需的选择信息。可纠正的参数错误必须给模型一次有界恢复机会，不当作依赖故障；查询输出须保留答案，变更输出尽可能简短但足以支持后续动作。
- R6：明确总结的触发窗口、异步完成与原始上下文的拼接规则；历史消息不能因为两个窗口错位而悄然丢失。
- R7：诊断展示需让一轮 ADK 执行中的多次物理模型请求及 Tool 回合可以分辨；每条 Response 应对应自己的 Prompt，而聊天中的公开消息仍只呈现最终可见内容。

## Deliverable map

| 子任务 | 拥有要求 | 独立验收边界 |
| --- | --- | --- |
| `09-27-visual-identity-review-contract` | R1 / AC1 | `commit_review` 形态归一化及失败恢复 |
| `09-27-context-summary-continuity` | R2、R6 / AC2、AC6 | Provider Runtime Context 与历史覆盖 |
| `09-27-life-event-timezone` | R3、R4 / AC3、AC4 | 当地时间、事件终止、场景/活动/日程 |
| `09-27-capability-contract` | R5 / AC5 | Provider Tool 参数与结果闭环 |
| `09-27-adk-diagnostics-display` | R7 / AC7 | Owner 诊断模型运行展示 |

父任务只负责跨子任务要求和最终联合验收，不直接修改产品代码。

## Acceptance criteria

- AC1：重放 `observations` 为非数组的现场模式时，Visual Identity 不再因“value must be an array”直接终止；数组、单条文本及可解释对象形式都产生同一有界规范化结果。不可解析值给模型可纠正的失败结果；一次 run 至多提交一次有效审查。
- AC2：正常对话的 Runtime Context 仅带需要的语义字段和确有消费者的安全 `ctx_*` 引用；可省的细粒度来源 ref 不进入普通对话；记忆更新与结构化因果归因仍可工作。
- AC3：按摇光生效时区进入新的一天时，日程选择及下一次生成时刻一致；已设定/仅初始化识别两种来源均有测试。固定 UTC instant 下，摇光消息用其生效时区，用户历史消息用发送时区显示；跨设备/旅行后旧消息不漂移。
- AC4：临时事件有效时场景与活动一致；静默后到期且无明确延期时，当前状态、详情界面和后续对话都进入新日程，不再声称仍在旧事件。明确延期可延长事件；结束购物事件可没有购入物，不得凭事件结束就改衣橱。
- AC5：模型侧工具 schema 与条件必填一致，可推导参数减少，更新目标可唯一确定时不强求模型传内部 ID；新增输出与后续更新输入的安全目标字段一致；失败信息能指出可恢复的选择或缺参原因。每项修改有相应验证。
- AC6：总结尚未生成、生成中及已完成时，预算允许的范围内不会出现因固定窗口错位造成的历史空档；裁剪发生时有明确预算依据。
- AC7：诊断页面能够把同一 ADK 执行的多轮 Prompt、Response 和 Tool 关联正确展示，不会把中间 Response 误作最终回答或重复“认知判断”。

## Constraints and out of scope

- 保留现有未提交改动；本任务不得覆盖或顺手提交与本任务无关的用户工作。
- 不把 Core 内部认知内容或未脱敏 Tool trace 直接暴露到公开聊天流。
- 不为解决一次缺参错误而取消身份校验、幂等或资源目标唯一性约束。

## Verification limits

- 第 5 点无现场错误样本；只能以确证的 schema/运行时错位与可重现错误验收，不能声称已定位原始失败。
- 诊断页的历史 model-run 可能没有稳定轮次字段；旧记录须有诚实的兼容展示，新记录通过轮次标识和关联 trace 验证。
- 第 3 点的偏差在多处出现，现场 timestamp 未保留；固定 UTC instant 的测试必须覆盖上下文、日程、详情、两种作者消息和诊断页面。
