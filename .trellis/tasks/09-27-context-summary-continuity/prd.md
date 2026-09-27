# 运行时上下文与对话总结连续性

## Goal

让模型看到必要且连续的当前事实与对话历史，避免无用引用占据上下文，以及摘要/原文窗口错位造成的遗忘。

## Confirmed facts

- `active_memory:ctx_*` 用于更新目标反解和 CAS；直接删除会破坏生命周期（`apps/core-go/internal/core/active_memory.go:36-66,83-135`）。
- 普通对话允许的引用类型与 `life_context` 嵌套 ref 的过滤不完全一致（`apps/core-go/internal/core/provider_context.go:385-404,670-707`）。
- Summary 旧消息 chunk 最多 40 条、保留最新 24 条；raw recent 硬截最后 8 条（`apps/core-go/internal/core/conversation_summary.go:71-95,146-203`、`apps/core-go/internal/core/provider_context.go:222-272`）。

## Requirements

- R2.1：Runtime Context 的字段按 surface 和真实消费者收敛；保留记忆目标 ref、life context 因果锚点及内部去重所需 source refs，普通对话不无端暴露细粒度来源 ref。
- R2.2：模型可见字段的含义可辨，历史总结明确标注历史时间语义，不与当前生活状态混淆。
- R6.1：summary 异步完成前后，未被已提交 summary 覆盖的消息有机会按预算进入 raw history；不能先固定截 8 条再形成无预算依据的空档。
- R6.2：无法发送全部候选历史时，选择完整 turn 并留下可检查的预算裁剪原因。

## Acceptance criteria

- [ ] 真实有效的 nested schedule/scene/presence ref 不违反 ordinary conversation surface 策略；记忆 revise/complete 仍能用 ref。
- [ ] summary 未生成、生成中、已提交三个阶段，在预算足够时 summary 覆盖后紧接 raw history，当前消息只发送一次。
- [ ] 预算不足时不拆半个 turn，并能从 trace 分辨预算裁剪与 summary 尚未覆盖。

## Out of scope

- 不承诺无限历史，也不删除所有 opaque refs 或降低 owner/conversation/revision 校验。
