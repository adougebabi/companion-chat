# 执行计划

1. 建 deterministic clock tests：t0+10/30/60/90、聊天重置、主动回复不重置、慢 Provider跨节点、重复释放与 restart；保存旧 history replay fixture。
2. 在单一 PG时钟中加入 idle epoch/phase和绝对 due计算，重用 Redis hint与PG CAS；迁移/repair现有 clock，保留 Owner interval兼容。
3. 新用户消息处 supersede旧 epoch；在 Wake-up锁内 settlement重查 marker/intent/epoch；分别测试 pending、running和检查→提交竞态。
4. 检查暂停/退役/禁用/恢复及过期诊断；更新 backend workflow spec、设置展示和旧 40m测试。
5. 运行 Go Core/Worker/Workflow测试、PG竞争测试和 Temporal history replay；必要时用受控时钟做一次真实队列路径验证。

依赖：阶段摘要可复用这里确立的 user idle epoch，但摘要子任务仍有独立 durable intent，不能把 Summary工作塞进 Wake-up Tool或复用其业务 cycle。回滚点是 additive payload/version与旧 clock解释分支。

验证命令：`go -C apps/core-go test ./internal/core ./internal/workflow/... ./internal/migrations/...`；有Temporal历史变化时运行仓库现有历史回放测试及隔离Worker/PG竞争测试。
