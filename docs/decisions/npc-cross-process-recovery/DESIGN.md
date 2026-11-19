# NPC 持久响应跨进程恢复验收

2026-11-19。补齐现有持久 checkpoint 的验收覆盖，不改变生产人格、策略、模型入口或租约时限。原 T12 完整范围保持待确认。

## 场景与身份

父进程仅初始化一次隔离 PostgreSQL 和正式世界/NPC/来源。两个不同 PID 使用同一数据库，私有 manifest 保存完整原 ToolExecutionRequest 或原 inbox、来源、机会和虚拟时钟。第二进程不重新 seed，不换 operation/inbox，不替换 NPC 模型为内存 closure。

受控 loopback HTTP 服务通过真实 Provider 请求解析响应，分别计数 NPC 响应与 Moment Writer。每个场景 NPC 请求始终一次；Moment Writer 另计一次。前置旧 inbox 仅用既有 no-effect fixture 正式消费，不冒充所测 NPC HTTP。

## 中断边界

机会分支通过真实 Prepare 持久 ready npc_response。在现有 service.commit 依赖接口直接 os.Exit(73)，此时响应事务已提交，业务事务未提交，进程退出使事务锁和未提交工作回滚。仅测试替换 commit 回调，不增加生产钩子。新进程恢复 accept、decline 或来源取消后的同一请求，并重放；回执、机会版本、encounter、关系、记忆和 outbox 不重复，来源与隐私归属不丢。

Moment 分支通过真实 ProcessCognitionInbox，在 comment INSERT 注入数据库异常。函数返回错误并执行 claim/逻辑租约释放后，进程退出74。父核对 response 已持久、interaction/inbox 仍 pending、claim 已释放、公共效果无；移除测试 trigger，新进程处理原 inbox 并重放。评论与 outbox 一次，或原结识 life event 被正式取消后 suppressed/npc_moment_source_changed、无新评论。原响应保留，社交资料不变。

## 安全边界与限制

子进程只接受 127.0.0.1 且 lac_core_ 前缀的一次性数据库；manifest 0600、仓库外保存，子进程输出移除数据库 URL/测试口令。不连接业务数据库，不读取真实私人业务内容，不使用或上传图片。

机会用例是业务提交前真实进程退出。Moment 用例是数据库故障返回后的跨进程恢复，不能声称覆盖绕过 defer 的硬杀：那种情况可能等待既有3分钟逻辑租约及10分钟 inbox claim 过期，本轮不改变该政策。测试证明技术持久化、身份与去重，未调用真实模型，不证明 NPC 自然语言质量或原任务的全部语义验收。

## 验证

五个子场景：机会 accept、decline、来源取消；Moment comment 恢复、结识来源取消。父断言不同 PID、HTTP 计数、持久响应字节一致和最终效果；子进程重放前后比对完整相关业务行（包含身份、revision、comment 和 outbox ID）。另跑 race、vet 和相关既有 NPC 场景。
