# 执行计划

- [x] 对照各 surface ref 消费者、`surfaceAllowsEntityRef`、Provider schema 和 frozen index。
- [x] 修复所有 Provider ref 的 kind-aware 过滤；普通对话仅保留 life-context/Memory/Active Memory 需要的引用，内部 Summary source refs 用于去重。
- [x] 在预算选择前保留最多 200 条 raw 候选，移除固定 8 条硬截；只有选中的 Summary 覆盖的消息会去重。
- [x] 补充 Summary 有/无及预算落选、完整 turn 预算连续性、重复当前输入和有效 nested ref 的回归。
- [x] Go Core 全包、vet 与隔离 PostgreSQL Summary/Active Memory 用例通过。
- [x] Trellis check 两轮复核：补上最终 Prompt 总预算落选 Summary 时的 raw fallback，并补足 section_cap/source_invalid trace 断言；纯 Go、vet 与隔离 PostgreSQL 用例通过。
