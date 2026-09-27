# 执行计划

1. 先用当前代码重放 70+/140-message短与长消息覆盖场景，记录今天修复已满足与仍缺失的部分；测试旧 Summary/raw行为作为兼容基线。
2. 引入 source-bounded阶段窗口、Core-owned时间范围、结构化摘要 schema及静默 10分钟 durable debounce；重复 timer与新聊天边界有 deterministic identity。
3. 增加 backlog drain和旧固定块过渡读取；保持 Summary原文refs/digest/revision校验及当前 wire/raw dedupe。
4. 扩 frozen Memory evidence支持 conversation summary及失效传播；新增每日 consolidation intent，Core-owned当地日窗口、same-conversation scope与 episodic Memory写入。
5. 日记忆与阶段退役原子提交/可恢复，测试失败回退、原文更正、旧版本、跨午夜/DST、权限隔离、重复归并和检索预算。
6. 运行 Go Core/Worker/迁移/Workflow测试，受控模型摘要质量 fixture；同步 Memory/Workflow specs，最后与 Wake-up和认知上下文子任务联合测试。

依赖：`09-27-context-summary-continuity`的已提交去重修复是基线；`09-27-wakeup-idle-cadence`先定义 idle epoch。回滚点分别为阶段摘要生产、日记忆来源链接和活动摘要退役；任一步失败时保留旧 active Summary可读。

验证命令：`go -C apps/core-go test ./internal/core ./internal/workflow/... ./internal/migrations/...`；隔离PostgreSQL/Temporal测试覆盖重试、事务、旧历史与失效传播，70+回合fixture核对最终Provider wire。
