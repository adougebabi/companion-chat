# Summary/Raw coverage 与 Runtime Context ref 审计

- 当前总结 intent 在助手消息提交后异步排队；最新 24 条不进总结，旧 chunk 达 20 助手回合、约 6000 token 或 40 条才生成。此前 `recentPromptFragments` 在 token 预算前把 raw 固定截成最后 8 条，造成确定的 24/8 空档。
- `BuildContextProjectionFor` 最多读 200 条有权限的 raw history。现在它保留 `turn_id`；`recentPromptFragments` 只去掉真正位于末尾、与当前输入相同的 user message，不误删更早的同文本消息，也不预截 8 条。
- `retrieveConversationSummaries` 只返回源 refs/digest 重验通过且符合 summary 预算的 active projection。`compactSummaryForSurface` 仍只向模型送历史文本与时间语义；其 `source_message_refs` 仅作为 Core 内部 `PromptFragment.SourceRefs`。
- Working Memory 先选择符合 Summary section cap 的 Summary，但用独立 seen 集保留 Summary 覆盖的 raw fallback；最终 Prompt assembler 才以真正入选 wire 的 Summary message refs 去重完整 raw turn。未完成、失效、section cap 或总预算落选的 Summary 不会抢先移除 raw。Recent 默认 cap 从实现中的 1500 改为规范的 8192 tokens；一旦完整较新 turn 因 cap 落选，更旧 turn 标记 `recent_contiguity_excluded`。部分覆盖的 turn 保留完整原文。
- 最终 Prompt assembler 仍有整体预算；`working_memory` 和 `prompt_budget` trace 说明因 section/total cap 裁剪或因真正入选 Summary 去重，而 `history_coverage` 记录 fetched raw、候选 raw 与检索到的最新 summary 序号。诊断中的原始 `message:<id>` SourceRefs 转为稳定 `message:diag_<digest>`，不持久化 Core 原始 ID。无限历史不在本任务范围内。
- Ordinary Conversation/Takeover surface 只接受 frozen index 中 kind 匹配的 `life_context`、Memory、Active Memory refs；有效但属于 nested Scene/Schedule/Presence 的 token 被滤除。WakeUp 等允许 surface 仍可见对应类型。内部引用继续支持记忆 target CAS 与 source 去重。

验证：`go test ./... -count=1`、`go vet ./...`，以及隔离 PostgreSQL 的 Conversation Summary 和 Active Memory 用例通过。固定 64 条消息的测试验证 Summary 覆盖 1–40 后，在预算足够时最终 wire 选中 41–64；Summary section/total 预算落选时 raw 1–64 仍有资格进入选择。默认/收紧预算均有明确 trace。
