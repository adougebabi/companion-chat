# 后端模块边界与运行时拆分

## Goal

在同一个后端现代化任务中完成二维模块架构的一次性最终迁移：拆除旧的单体 `server.js` 层级混合，建立清晰的纵向技术层和横向领域能力模块，同时保持对外 API、SQLite 数据和 worker 行为兼容。迁移期间可以使用临时 facade/adapter，任务完成时必须删除所有遗留单体层和重复路径。

## Scope

- 从纯契约/解析函数开始，随后拆 storage/repository、LLM stream、media provider、job queue/dispatcher、domain service 和 HTTP routes。
- 保留一份 SQLite connection、现有 migration、`/api/companion/...` API、环境变量、data directory 和 worker lease/retry 语义。
- 迁移期间允许 `server.js` 暂时作为 composition root/facade，允许旧 hooks 作为对照；完成前必须删除旧单体实现、旧 hooks 暴露和重复入口。

## Architecture Direction

Use a two-dimensional modular monolith: vertical technical layers (transport, application, domain, ports/contracts, infrastructure/runtime) crossed with horizontal capabilities (identity, memory, life world, relationship, presence, capabilities, conversation, media, activity/proactive). The first problem is layer/dependency confusion; a 摇光实例 is not assumed to be an independent process or database.

The first boundaries are `Ports / Contracts`, `ChatTurnUseCase + CapabilityDispatcher`, a pure `LifeStateResolver`, and `MediaProviderPort + settlement`. Only after these are verified should `createEvent()`, `contextFor()`, activity/proactive, identity, memory, relationship, and presence be moved.

Use cases must be composed as typed pipelines registered with a generic runner, not as one bespoke orchestrator per feature. The runner handles shared execution concerns; horizontal modules provide typed capabilities, facts, projections, effect intents, and step implementations.

Every step returns `facts`, `projections`, `effects`, and `presentation`; it cannot directly call a provider, write a job row, write SSE, or open a database transaction. This keeps side-effect execution generic and removes per-feature transaction/job special cases.

Prompt context is another shared pipeline: horizontal modules emit typed `ContextFragment` values, `ContextBudgeter` selects them under the existing prompt-optimization policy, and `PromptSerializer` produces LLM messages. No domain module owns the complete prompt string.

## Requirements

- 纯模块不得导入 database；repository 不返回 browser-specific DTO；provider 不执行领域决策；route 不直接写 SQL。
- 保留事务、causation ID、幂等、租约、重试和重启恢复语义。
- `runMediaJob` 改为注册式 dispatcher，避免新的 job type 继续扩张中心分支。
- 迁移每个边界后保留完整 API test 和 temporary database 验证。
- 与现有 prompt optimization、shared-scene、media_event 未提交改动明确整合，不重复实现相同契约。

## Acceptance Criteria

- [ ] 目标目录和依赖方向与父任务 `design.md` 一致，核心模块可单独导入测试。
- [ ] `server.js` 只保留 composition/startup 和薄路由注册，不再承担所有 repository SQL 和 provider 实现。
- [ ] `npm test`、`node --check server.js`、temporary `DATA_DIR` API/worker 检查通过。
- [ ] API payload、SSE events、database rows、job lease/retry 和 debug gating 与重构前兼容。
- [ ] provider failure、malformed input、expired lease、restart recovery 有回归覆盖。
- [ ] `streamPersonaChat()`、`createEvent()`、`contextFor()` 和 media worker 的跨层职责有映射记录，且第一批边界不是机械按行号拆分。
- [ ] 迁移结束前删除临时 facade、旧 `companionTestHooks`、重复 job dispatcher、旧 provider 路径和遗留单体模块；全仓库扫描确认没有生产代码引用它们。
- [ ] 聊天、生活、媒体、主动行为和关系流程均通过统一 pipeline/step registry 执行，不再各自复制事务、job、retry 或 provider orchestration。
- [ ] 迁移期间有 contract fixtures、pipeline tests、normalized replay/dry-run 对照；删除旧层后再次运行完整 API/worker 测试。
- [ ] flow/step/effect correlation、脱敏结构化日志、SQLite lease/retry 恢复和上一构建回滚路径有验证。

## Dependencies And Out Of Scope

- 依赖 native tool child 定义 capability dispatcher 的调用边界；依赖 parent contract review。
- 在父任务确定“自主 AI 实体/关系/生活运行时”的领域边界前，本子任务只做架构方案讨论和边界草图，不启动文件拆分实现；实现阶段允许分批施工，但最终必须一次性切换并清理遗留层。
- 不更换数据库、队列、HTTP 框架或前端技术栈；不在本子任务中做 Fluctlight 文案改名。
