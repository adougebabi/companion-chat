# 设计

PostgreSQL `wake_up.current`继续是唯一权威时钟，Redis只作为低延迟提示。时钟 payload/version显式保存 `idle_epoch`（最后已接收 user message ID/时间）、phase (`first_10m`, `second_30m`, `recurring`)与绝对 `next_due_at`。用户消息的接受事务或同一可靠后置边界更新 epoch并 supersede旧 cycle；认知结算只能补偿 re-arm，不得把 `t0` 改成结算时间。Wake-up主动回复不更新 epoch。当前 `interval_seconds`保留为 Owner可配后续递归间隔，默认 1800s；前两个节点固定 600/1800s。

到期释放仍经现有 PG CAS和单 Redis key。每次 settlement在 lifecycle advisory lock内核对最新 idle epoch、intent status/cycle与 cancellation marker；不匹配则记录 superseded，不提交 wake fact/child intent/outbox/next clock。Provider/Tool运行期间继续 cooperative cancel；已经独立提交的 Tool receipt不能回滚，只阻止后续未提交效果。重复自动 due共享 epoch+phase+cycle稳定 identity。t+10运行若跨过t+30，序列化同主体执行；完成/取消后立即处理已到期的 t+30（带 late trace），不并发启动第二轮，也不把后续相位改为完成时间+30m。

旧 persisted clocks与 Temporal history通过 payload version分支保持可回放：旧 history使用旧解释，新启动的 cycle使用 absolute idle phase；迁移/repair读取最近 user message作为 epoch，无历史消息维持既有 activation clock。启停/暂停遵循既有设置与治理规则；每种抑制结果明确下一次 due策略。实际实现不得添加第二延迟队列或仅靠 Redis定时器。

预计修改 `wakeup.go`、`redis_triggers.go`、`cognition.go`/消息接受路径、`agent_result_adapter.go`、`lifecycle_preemption.go`、Worker reconcile/Workflow input及必要迁移、设置映射与测试。同步 `.trellis/spec/backend/fluctlight-workflow-contract.md` 中旧 `interval+10m` 合同；如设置 DTO变化，同步 Web/BFF。
