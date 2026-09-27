# 地点归属与认知上下文精简

## Goal

明确双方地点主语并删除认知上下文中的重复状态与结果

## Confirmed facts

- `current_state.life_context` 是摇光的场景/地点，`current_speaker` 只含 Actor 身份；`scene_event` 总是写摇光的 `life_events`，而运行协议没有明确“用户说我回家了”与“请摇光回家”的区别。证据见父任务 `research/confirmed-paths.md`。
- 最近 12 条 action outcomes 未按状态/能力去重，导致完成的 `affect_event`、`conversation.reply` 与失败行反复进入 Runtime Context；同轮 Tool receipt/刷新状态/历史回复也可重复。
- Runtime Context 已采用 YAML + 部分 TOON；体积问题主要来自重复数据与冗余字段，单纯换格式无法解决。

## Requirements

- C1：摇光当前 Life Context 明确带 `actor_self` 归属；用户的第一人称地点陈述不得授权 `scene_event` 改变摇光地点。只有明确以摇光为主体的变更才能调用该能力。
- C2：对话中已明确且仍有效的用户地点以独立 Actor 归属表达或从已授权的用户上下文读取，不能塞进摇光 Life Context；缺失时保持未知。
- C3：conversation/wake-up 的 recent outcomes 和 Tool continuation 只保留尚未被当前状态/已发布消息吸收的决策信号，尤其保留仍相关的失败与异步结果。
- C4：精简 appearance/drive 元数据并保证关键当前事实不会因 section cap 被静默丢弃；保留必要 opaque refs、CAS 与来源安全。

## Acceptance criteria

- [ ] 异地多轮 fixture 中，“我回家了”不写摇光场景，“请你回家”只有真实 Tool 提交后才改变摇光地点；无来源时不臆造用户当前位置。
- [ ] 连续调用 `affect_event`、`conversation.reply` 和一次失败 Tool 后，当前 mood/已发布回复不以多份 outcome/receipt/history重复输入，最新可行动失败仍可见。
- [ ] conversation 与 wake-up 的 Runtime Context 保留有效 current state、actor 归属与必要引用；减重前后 token/字段对比可检查，超预算关键事实有明确失败或裁剪原因。
- [ ] 原生 ToolCall/ToolResult 仍配对，记忆目标引用、工具授权和当前事实刷新不退化。

## Out of scope

- 不以 Go 关键词/正则猜用户地点或 scene 主语；不把用户地点写进 Presence 的 scene/location 列。
- 不重做现有 `09-27-context-summary-continuity` 的 raw-history/wire-dedupe 修复。
