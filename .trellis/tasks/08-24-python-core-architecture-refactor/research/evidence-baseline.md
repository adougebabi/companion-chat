# Evidence Baseline

## Scope

本文件记录 2026-08-24 重构规划开始时的代码库与历史证据。它不是目标架构设计；当共享工作树继续变化时，应以引用的符号和当时 commit 为核查入口。

取证期间工作树中的 daily-plan 修改被其他并行工作提交并归档，最终代码基线为 `6e52536`。本规划任务没有修改产品代码。

## Current Runtime

```text
Vue browser
  -> Node.js / Express :4178
       -> browser DTO and SSE transport
       -> application/domain flows
       -> SQLite + migrations + repositories
       -> in-process durable-job worker
       -> MTPLX / ComfyUI / h3 adapters
```

- CLI/runtime composition: `server/index.js:273`, `server/runtime/runtime.js:1408`.
- HTTP/static/SSE boundary: `server/http/app.js:294`, `server/http/route-registry.js:6`.
- SQLite startup and migration: `server/runtime/startup.js:112`, `server/runtime/sqlite-runtime.js:243`.
- Worker lifecycle: `server/runtime/worker-runtime.js:118`, `server/runtime/job-dispatcher.js:208`.
- Production providers: `server/infrastructure/production-media-providers.js:156`.

## Boundary Findings

- Node 已经是有效的浏览器防腐层：聚合 bootstrap / persona detail，转换 DTO，隐藏 provider secret，保持 `{error}`，翻译 SSE，并代理媒体。
- 浏览器最敏感契约是 SSE token 顺序、唯一 terminal event、abort 传播、`done.messages`、conversation/activity cursor、generation/jobs 状态与媒体 URL。
- Python 核心若直接暴露数据库行或 provider chunk，会把内部迁移扩散到浏览器；Node BFF 应保留现有外部契约并只调用稳定内部 API。
- 当前 `activity comment` 和 `appendConversationMessage` 已存在服务端/客户端 envelope 漂移，应在冻结迁移契约前裁决；出处为 `server/application/activity-service.js:435`、`web/src/stores/activities.ts:162`、`server/application/conversation-service.js:81`、`web/src/api/conversations.ts:15`。

## AI And Fluctlight Core

- 聊天链路：`server/application/chat-service.js:118` -> `server/application/chat-turn-flow.js:458` -> `server/application/structured-turn.js:127` -> `server/application/flow-executor.js:124` -> `server/infrastructure/conversation-commit-adapter.js:106`.
- 核心状态包含 identity/foundation、life blueprint/state、relationship evolution、affect event/snapshot、interaction appraisal、memory consolidation candidate、self-model claim、agency intention。
- 纯 domain 目前很薄；prompt 组合和多个领域读模型仍直接集中在 `server/runtime/runtime.js:358`。
- capability commit 假设同步 SQLite writer；这使“Python 核心 + PostgreSQL”更适合成为新的事务所有者，而不是让 Node 与 Python 同时直接写同一领域表。
- `摇光` / `companion` / `persona` 是旧词。用户已决定 clean-start 系统统一使用 `Fluctlight` / `Fluctlight instance`、identity core、life world、relationship、presence，且不提供命名兼容别名；依据 `CONTEXT.md:5`。

## Jobs And Workflow Semantics

- 真实链路包括：`daily_plan -> timeline_candidate -> proactive_message -> media submit -> media poll/compensation/quality retry`。
- 另外有 pending event、deferred chat batch、activity decision、relationship evolution；后两类部分 handler 当前无生产 enqueue / evaluator，属于 dormant capability。
- 通用机制支持 durable run-after、priority、lease、attempt、retry/backoff、restart recovery 和 guarded settlement，但不支持依赖图、续租、通用取消、人工重试或持久可观测性。
- h3 最大 15 分钟与默认 60 秒租约冲突；stale worker 可继续执行外部副作用。
- `companion_jobs` 没有数据库级稳定幂等键，当前 lookup-before-insert 只在单进程单并发假设下相对安全。
- Redis Streams 的 PEL 不能替代 `run_after`、冻结 decision、业务唯一键、补偿任务和审计状态；若引入 Streams，业务事务与发布之间需要 PostgreSQL outbox，消费者需要幂等 inbox/ledger。

## Data And Media

- SQLite 有 19 个 migration 和约 36 张业务表，JSON TEXT 是重要兼容面。
- PostgreSQL 迁移风险包括同步 API 全面异步化、SQLite 方言、非法 JSON 清洗、TEXT 时间语义、整数布尔、删除顺序及多连接并发下缺失唯一约束。
- 长期权威状态必须在 PostgreSQL；Redis 只能保存可重建缓存、短租约和事件传输状态，不应成为 identity、messages、life state 或 affect snapshot 的唯一来源。
- 媒体迁移源既有 ComfyUI remote locator，也有 h3 local path。迁移对象存储需要补充 checksum / MIME / size / object version，并为旧 provider 保留过渡 adapter。
- 当前 asset 引用没有完全规范化，persona 删除未完整检查 serialized message attachment；对象存储垃圾回收前必须先修复所有权模型。

## Historical Refactor Decisions

### 2026-08-17

- Session: `01a00f9a-b6f8-7511-8c59-3806df02c820`.
- 结论：Node 足够，先修复单行状态与任务编排，不先换语言或拆服务。
- 当时合理性：产品单用户本地运行，瓶颈主要是外部推理；缺的是规范化表、事务、lease、retry 与幂等。

### 2026-08-20

- Session: `01a01d27-f9fa-71c1-9258-abf9c6f06986`.
- 结论：先把 Node 固定成模块化 control plane，通过 ports/contracts 为未来 Python / Go Worker 留边界；拿到 workload profile 后再复议。
- 归档设计：`.trellis/tasks/archive/2026-08/08-20-fluctlight-architecture-performance-modernization/design.md:122`.

### Changed Evidence

- 单行状态和旧巨型入口已经清理，说明前两次延后换语言换来了有价值的领域和契约基线。
- 后台任务数量、链路和长任务显著增长，已经出现足以重新评估 Worker / 工作流基础设施的正确性证据。
- 仍不应把语言迁移当作性能项目；它是职责所有权、生态适配和扩展成本项目。

## Planning Implications

1. 先决定部署产品形态，再选工作流技术；本地单机与托管多租户对基础设施容忍度完全不同。
2. Python 应成为核心领域事务的唯一写入者，Node BFF 不应与其共享写库职责。
3. 浏览器、Node BFF 与 Python Core 的契约可以重新设计并一起切换；现有契约只作为行为证据，不作为兼容目标。
4. 用户已选择 clean start；新系统不导入 PostgreSQL 业务数据、工作流状态或对象存储对象，不实现旧 SQLite / media / job 兼容层。
5. Redis Streams 是事件传输机制，不是工作流状态机或权威 job ledger 的自动替代品。
