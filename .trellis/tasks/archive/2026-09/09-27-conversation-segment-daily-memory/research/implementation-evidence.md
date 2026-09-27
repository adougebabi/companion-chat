# 实施与验收记录

## 已实现

- 用户消息接受和 assistant 发布后维护 `conversation.segment` PostgreSQL intent；最后用户消息后 10 分钟静默触发，跨当地日的已完成回合立即封段。新用户活动让旧 idle intent 失效；静默阶段至多 40 条消息、约 6000 估算 tokens，并在每次结算后追赶剩余完整回合。
- 新阶段摘要使用闭合结构 `{summary, ending_state, open_threads, core_events}`。Core 从已验证的原始消息计算起止时间、时区及当地日期；Prompt 不再把摘要完成时间、存储 sequence 或 `historical_conversation` 标签当作聊天时间展示。
- `conversation.daily_memory` 按会话、当地日期及冻结时区，在次日当地午夜后 10 分钟运行。它复核阶段 source refs/digest/revision，调用既有 `applyMemoryCommandTx` 创建或修订 `episodic` Memory。Memory、来源链接、阶段摘要的 `consolidated` 状态同事务提交；失败回滚后阶段仍 active。
- 迁移 `0042_conversation_daily_memory` 增加阶段时间字段、来源状态、日记忆索引和 `conversation_summary` frozen evidence kind。原文更正会失效阶段和依赖的日记忆 provenance；返回早先 digest 时可重开同一 intent 并生成新摘要修订。旧同日固定块从原文补时间，跨日固定块从原文重建。
- 最终 Prompt 仅在日记忆确实入选时用其内部来源 refs 去重近期原文；落选时原文保留。旧 `conversation.summary` Temporal history 保持可回放，晚到旧结果不能覆盖新的阶段摘要。
- Segment/Daily Workflow 注册于原有 `lifecycle` 队列和管理映射。终止失败按 5 分钟、连续失败后 30 分钟退避；同 Workflow ID 旧 Run 的终态不会结算一个刚重开的 PostgreSQL intent。

## 已验证

- 隔离 PostgreSQL 升级至 `0042`，迁移重跑成功；新增数据库回归覆盖 10 分钟 debounce、新消息 supersede、连续跨午夜用户消息、跨午夜完整回合、持续跨日封段、DST 23/25 小时、冻结的不同时区 day intent、140 条消息完整追赶、旧固定块迁移、日记忆写入失败回滚、原文更正/恢复早先 digest、日记忆 revision 和来源失效。
- 新 Workflow deterministic Activity/ID reuse 测试、Prompt 选中后原文去重及预算落选回退测试通过。
- `go vet`、Web/客户端生成、类型检查、单测、构建通过。隔离 PostgreSQL 上完整 Core、Workflow、migrations、Core HTTP、Browser HTTP 套件通过；新增末轮代码另跑定向数据库回归及全 Go 无数据库回归。

## 现场验收边界

- 单测使用结构化假 Provider，尚未用真实模型评估摘要措辞质量，也未在实际 Temporal 服务上演练进程重启和跨版本 replay；数据库权威、事务和 deterministic Workflow 测试已覆盖对应代码边界。
