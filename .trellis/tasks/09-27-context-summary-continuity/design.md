# 设计

Provider-facing Runtime Context 继续由 `compactCognitionContextForSurface` 统一构造。按 ref 的 expected kind 执行 surface allowlist；普通对话保留必要的 `life_context.ref`、`memory/active_memory` 目标 ref，过滤没有该 surface 独立消费的嵌套 Event/Schedule/Presence ref。`PromptFragment.SourceRefs` 仍留在 Core 内用于去重。若通用 influences/claim grounding 依赖细粒度来源，优先映射到有效的 life-context 锚点，不接受模型伪造原始 ID。

会话历史先读取最多 200 条原始 user/assistant 候选，不在预算选择前硬截 8 条。已验证 Summary 的原始 `source_message_refs` 只放在 Core 内部 `PromptFragment.SourceRefs`，不进入模型内容。Working Memory 先按 Summary section cap 选 Summary，同时用独立的 seen 集选择完整原文回合作为 fallback；它不提前删掉 Summary 覆盖的 raw。最终 Prompt assembler 以 Summary 优先顺序决定哪些 Summary 真正进入 wire，再只对已入选 Summary 覆盖的完整 raw turn 去重。若 Summary 在 section 或总预算落选，raw 仍可竞争预算。近期 section cap 为 8192 tokens，且一旦一个较新的完整 turn 因预算不合适，更旧 turn 记录 contiguity 原因。最终 trace 明确 budget/dedupe 原因；部分覆盖的 turn 保留完整原文，宁可局部重复也不丢失未覆盖的消息。历史 Summary 标注 `historical_conversation`，不当作当前场景权威。

Tool 和 Context 子任务共享 opaque target 契约；先完成本子任务测试，再在父任务联合检查各 update Tool。
