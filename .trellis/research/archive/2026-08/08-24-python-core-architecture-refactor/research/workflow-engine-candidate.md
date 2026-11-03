# Historical Workflow Candidate: DBOS Transact Python (Rejected)

## Status

Superseded historical evidence. T01 completed with `FAIL`; DBOS is rejected and must not be implemented. The authoritative result is `dbos-runtime-gate-report.md`, and Temporal T01B is the current conditional path. The sections below preserve the original evaluation assumptions and must not be read as future instructions.

## Why It Fits The Deployment Decision

- 目标部署是个人本地 / NAS，自包含 Docker Compose 优先。
- PostgreSQL 已经是候选领域权威数据库，因此工作流状态复用 PostgreSQL 不会新增一套专用集群。
- Python Core 与 Python Worker 可以共享同一 modular-monolith 代码库，由不同进程启动 API 与队列消费。
- 当前实际需求不是单纯 cron，而是持久延迟、step 恢复、重试、队列并发、冻结决策、长媒体任务和人工恢复。

## Confirmed From Current Official Documentation

Context7 library: `/dbos-inc/dbos-transact-py`, source reputation High, fetched 2026-08-24.

- `@DBOS.workflow` 提供自动 checkpoint 和故障恢复，并可配置 recovery attempts / isolation level。
- `@DBOS.step` 提供持久步骤、重试次数和 timeout。
- `SetWorkflowID(event_id)` 为一次 workflow start 提供稳定 ID 与 exactly-once 启动语义。
- `Queue` 支持后台排队执行 workflow / step。
- `@DBOS.scheduled(cron)` 支持周期调度。
- `DBOS.sleep()` 是可跨中断恢复的 durable timer。
- `DBOSClient` 可查询 workflow，并提供 pause / resume / restart 等管理入口；文档示例还展示了从指定步骤 fork 进行恢复。
- production config 直接连接 PostgreSQL system/application database，由 Python 应用调用 `dbos.launch()`；文档未要求额外部署独立 DBOS workflow server。
- production config 支持日志级别和 OTLP tracing。

## Important Non-Guarantees

- workflow ID 去重不等于任意外部副作用天然 exactly-once。ComfyUI / h3 / 对象存储调用在“外部动作成功但 checkpoint 尚未持久化”的窗口仍应使用 provider request key、幂等对象 key或补偿记录。
- DBOS workflow 状态不能代替领域事实。pending event、daily plan、timeline decision、message generation 等用户可见或可审计状态仍应写入业务 schema。
- DBOS 不能替代 PostgreSQL outbox。领域事务提交后要发布 Redis Streams 事件时，仍需 outbox publisher 与幂等 consumer。
- Redis Streams 不应再承载同一份 workflow 编排状态，否则会形成两个互相竞争的任务权威源。

## Prototype Gates

- 使用一个 frozen decision + delayed delivery workflow 验证重启恢复与稳定 workflow ID。
- 使用一个最长 15 分钟的 h3 fake provider 验证 timeout、取消、进程退出、恢复及不会重复外部 submit。
- 验证 queue concurrency / rate limit 能按 provider 分组限制，而不是只有全局单并发。
- 验证 DBOS system schema 与业务 schema 的 migration、备份和版本升级流程适合 Docker Compose / NAS。
- 验证官方 API/client 或 thin wrapper 能支撑 list/get/pause/resume/cancel/restart/fork-from-step，不要求用户操作数据库。
- 按 `t01-dbos-runtime-gate-brief.md` 的量化阈值测量空闲/峰值内存、CPU、PostgreSQL 连接数和 readiness/recovery 时间。

## Rejection Conditions

- 不能可靠表达 provider 分组限流或长任务取消。
- 升级要求额外云控制面或无法自托管恢复。
- workflow 历史与业务 transaction 无法形成可验证的一致性边界。
- 任一 `t01-dbos-runtime-gate-brief.md` 量化资源阈值失败。
