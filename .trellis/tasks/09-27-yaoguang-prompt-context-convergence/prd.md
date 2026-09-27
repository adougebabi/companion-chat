# 摇光提示词与运行时上下文收敛

## Goal

让摇光在多轮对话中保持地点与视角一致，用简洁、时间明确且连续的历史理解用户；让媒体生成使用有效语义与明确拍摄视角；让失败在诊断中心可见并可定位。

## Reported behavior

- 用户与摇光设定为异地，场景变换后对话曾从用户家漂移到摇光家。
- 约 70 多轮对话后，`conversation_summaries` 仅两条，前文遗忘；`historical_conversation` 涵盖更多对话但与前者重复，摘要像流水账，缺少日期或时间段与结束方式。
- 认知判断的 Runtime Context 约 7000 多字符，含重复的历史当前状态、重复的 `affect_event` / `conversation.reply` 结果和失败状态。
- 媒体提示词中 `context_binding.appearance.body_fields` 仍显冗长，休息等生活需求含 description/direction/key/label 多余字段；图片常以第三人称视角生成，缺少自拍、镜前、手持相机等明确视角。
- `media_quality_acceptance` 及其他可能的提示词仍以换行 JSON 传入。
- 用户常见 `agent_run_failed`，但诊断中心未显示相应错误。
- 用户今天已改总结逻辑但尚未验证；此次以当前代码为基线。
- 当前 wake-up 与期望的聊天后 10/30 分钟静默节点不一致，用户还观察到应取消的唤醒或重复触发风险。

## Confirmed facts

- `historical_conversation` 是 `conversation_summaries[]` 的 `time_semantics` 标签，不是独立的第二组摘要。今天的提交已修复旧的固定 8 条 raw 历史窗口和最终 wire 去重；这部分应先用 70+ 回合场景验证，不重复实现。证据见 `research/confirmed-paths.md` 的“对话总结和历史”。
- 摘要仍按最多 40 条消息、20 个 assistant 回合或约 6000 tokens 触发，保留最新 24 条；70 个短的完整回合只有两条 summary 与当前阈值相符。摘要生成缺少阶段时间范围、结束状态的结构化契约；异步积压后的追赶也未闭环。
- 唯一结构化 Life Context 属于摇光，用户地点没有独立当前事实。`scene_event` 总是修改摇光场景，现有运行协议的地点主语和 `context_override.explicit` 可执行性不一致。
- 重复的 `affect_event`、`conversation.reply` 主要来自未筛选的 `recent_outcomes`，同轮 Tool 回执与刷新后的当前状态又可能重复；Runtime Context 当前已采用 YAML + 部分 TOON 表格。
- 初次 media prompt 最终会格式化为 YAML；`media_quality_acceptance` 的多模态文本部分仍为嵌套 JSON，另两个 Visual Identity 多模态入口也有相同格式问题。当前 media instruction 明写“不是普通自拍”。
- 物理 model run 成功与逻辑 Agent/Tool 失败是不同状态；诊断中心目前主要展示前者，完整底层错误存在 `agent_runs.error_detail`，却未投影到模型运行页。
- 当前 wake-up 默认首次是认知结算后 `interval_seconds + 10 分钟`（默认约 40 分钟），后续为每次 wake-up 完成后再隔默认 30 分钟；现有自动队列已有 Redis/PG 双重释放去重，但最终提交持锁后未重查取消状态。证据见 `research/confirmed-paths.md`。
- 当前摘要和 typed Memory 是不同边界。Summary 有精确原文 refs/digest；typed Memory 尚不接受 Summary 作为 frozen evidence source。直接把自由文本 Summary 当成稳定偏好或关系事实会失真。

## Requirements

- R1：把当前场景/地点明确绑定到摇光；用户自述自己的地点不能修改摇光的 Life Context。需要跨轮持续的用户地点须有独立归属，不复用 `scene_event`。
- R2：连续聊天停止后形成有起止时间的阶段摘要；跨天后把前一天的阶段摘要归并为一份有来源的日记忆，旧阶段摘要从活动 Prompt 中退出。日记忆先作为 episodic conversation memory，不自动把摘要推断升级为稳定偏好或关系事实。记录核心变化、结束状态和未完线索；补齐异步追赶与 70+ 回合覆盖验证，不重做今天已修复的 raw fallback/wire 去重。
- R3：按 surface 删除 Runtime Context 中已由当前事实或近期消息表达的重复 outcome 和 Tool 回执，精简无效 appearance/drive 字段；保留对记忆更新、证据归因、工具决策真正必要的安全引用和失败信号。
- R4：媒体意图明确区分第一人称、第三人称、手持自拍、镜前自拍等拍摄关系，并贯穿冻结 concept、生成提示词和质量验收；未指定视角时恢复现有媒体规格中保守的自拍视角默认规则，不被当前“写真/不是普通自拍”提示词强制成第三人称。媒体 context binding 应以紧凑 TOON 形式表达有效语义。
- R5：`media_quality_acceptance` 与其他多模态结构化文本输入不得无必要地嵌套/转义 JSON；普通 Tool 协议所需 JSON 不在此要求之内。
- R6：在诊断中心明确关联物理模型、Tool 与逻辑 Agent run 的不同状态，并显示可安全呈现的 `agent_run_failed` 具体失败原因；验证取消/超时与真实 Tool 关联路径。
- R7：以最后一条用户消息被成功接收的时间为静默基点，在 t+10 分钟、t+30 分钟触发 wake-up，此后持续静默时每 30 分钟触发一次；wake-up 自己的主动消息不重置时钟。新用户消息取消尚未提交的旧 wake-up、重新计时；重复自动触发只执行一次。

## Deliverable map

| 子任务 | 要求 | 独立验收 |
| --- | --- | --- |
| `09-27-agent-failure-diagnostics` | R6 | 逻辑 Agent/Tool失败可见且和物理模型调用区分 |
| `09-27-actor-location-runtime-context` | R1、R3 | 地点主语与认知上下文去重 |
| `09-27-media-capture-prompt-format` | R4、R5 | 拍摄关系与媒体输入格式 |
| `09-27-wakeup-idle-cadence` | R7 | 绝对静默节点、取消与去重 |
| `09-27-conversation-segment-daily-memory` | R2 | 阶段摘要、日记忆、来源连续性 |

父任务只负责源要求、依赖顺序和最终联合验收，不直接修改产品代码。

## Acceptance criteria

- [ ] “我回家了”与“请你回家”在异地场景的回归中归属不同；前者不写摇光 `scene_inferred`，后者若确实执行则只更新摇光；无依据时不臆造用户当前位置。
- [ ] 70+ 回合短/长消息场景能说明每条摘要的原文覆盖和下一段 raw 入口；已覆盖完整 turn 不双重输入，未覆盖 turn 若因预算落选有可诊断原因。
- [ ] 摘要输出包含真实对话起止时间、核心事件或关系变化、阶段如何结束及未完事项，不把 `completed_at` 当作对话结束时间。
- [ ] 静默产生的阶段摘要在跨天后归并为单份日记忆；已归并阶段摘要不再重复进入运行时上下文，原始来源与日记忆的追溯关系仍可核查。
- [ ] 日记忆创建失败时阶段摘要继续可用；原文更正或阶段摘要 revision 变化时，依赖它的日记忆不能继续冒充已验证来源；归并不会自动创建未经证实的长期偏好/关系记忆。
- [ ] 连续多轮的已完成 `affect_event`、`conversation.reply` 不反复占据认知上下文；同轮已发布回复文本不在 Tool 结果和 recent history中三重出现；必要的失败/异步结果仍可见。
- [ ] 媒体 context binding 的未知身体字段、零意义 drive 和元数据不进入提示词；手持、镜前、第一人称和第三人称分别有明确且物理一致的输出测试。
- [ ] `media_quality_acceptance` 和相同类型的多模态 text part采用可读结构化文本，避免 JSON 字符串嵌 JSON 字符串；初次 media prompt 使用紧凑 TOON。
- [ ] 在同一 correlation 中，诊断可同时显示“模型调用 completed”与“Tool/Agent failed”及安全原因，避免把逻辑运行失败呈现为整体正常。
- [ ] 以最后一条已接收用户消息为 t0，静默时恰在 t+10、t+30、t+60、t+90 分钟等节点各至多提交一轮 wake-up；主动消息不推迟下一节点。任一节点前有新用户消息则旧节点失效，以新 t0 重排；重复 Redis/PG 释放不产生重复 fact、Tool effect 或公开消息。

## Constraints

- 尊重今天已改但尚未验证的摘要逻辑，以及现有 `09-27-context-summary-continuity` 任务的所有权，先验证再决定是否补改。
- 不牺牲必要的记忆更新目标、诊断关联、权限边界或历史连续性来缩短提示词。
- 不把原生 Tool message JSON 改成 TOON；它是协议消息。现有 Visual Identity 参数形态错误的修复不重复纳入。
- “清空摘要”指归并成功后退出活动检索，保留 provenance 与审计记录；不物理删除来源行。日归并按冻结的有效本地时区计算日界，避免 UTC 日期或执行日期造成错日。
- 日记忆先限定在同一会话及其权限范围内归并当天的阶段摘要；不在本次改动中自动跨会话合并，避免改变记忆可见范围。

## Decisions

- 2026-09-27 用户确认：wake-up 使用最后一条已接收用户消息为静默基点，固定 t+10、t+30，其后每 30 分钟；新用户消息重置，wake-up 主动消息不重置。
- 2026-09-27 用户确认：摘要采用静默时间段与当地日归并两级；诊断中心需直接显示逻辑 Agent/Tool 失败原因。
- 媒体未指定视角时按已有媒体规格恢复保守自摄默认，同时尊重显式拍摄要求；这是与用户所述“现在总像第三人称，不像自己拍”一致的现有规格收敛。
