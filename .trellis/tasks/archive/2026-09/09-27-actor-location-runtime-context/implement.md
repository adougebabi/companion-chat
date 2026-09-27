# 执行计划

1. 读取将修改的函数与已有场景/引用/预算测试；取得一条异地场景受控 fixture，区分 `scene_event` 错写与纯语言漂移。
2. 加 Actor 归属及 scene Tool 主语契约，测试“我回家了”与“请你回家”；用户地点只从授权来源表达。
3. 压缩 recent outcomes、model-facing Tool receipts 和同轮已提交回复的 outbound 重复；保留失败/异步结果与 Tool 配对。
4. 压缩 current-state appearance/drive，保证 critical facts 可诊断地入选；对 conversation/wake-up 输出记录前后 token/字段数量。
5. 运行 Go Core 单元/集成测试和受控 Agent fixture；复核 Life Context、Memory target refs、CAS、Tool 更新与多轮刷新，更新相关 backend specs。

依赖：实施前确认已提交的 `09-27-context-summary-continuity` 行为，避免改其 raw fallback/最终 wire 去重。回滚点按场景归属和上下文减重分开保留，以便判断模型语义和预算回归。

验证命令：`go -C apps/core-go test ./internal/core ./internal/ai/...`；若改迁移或持久用户位置，再运行隔离PostgreSQL集成测试和 `go -C apps/core-go test ./internal/migrations/...`。
