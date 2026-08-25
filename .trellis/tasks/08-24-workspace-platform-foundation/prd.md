# T02 工作区与平台基础

## Goal

建立 pnpm/uv monorepo、FastAPI/Fastify/Vue skeleton、PostgreSQL/Redis/MinIO Compose、Alembic、Unit of Work、outbox/inbox、OpenAPI generated clients 和共享测试基础。

## Requirements

- 父任务与依赖门禁：T01/T01B 已归档；父 D020/D036 接受 Temporal core 并移除资源/soak blocker。本 child 仍需显式 start。
- 实施范围：pnpm/uv workspace、apps skeleton、FastAPI/Fastify/Vue transport foundation、PostgreSQL/Redis/MinIO Compose、Alembic/Unit of Work/outbox/inbox、generated clients、health 与 shared test harness。
- 遵守父任务 `READ_FIRST.md`、`decisions.md`、`design.md`、capability inventory 和 assigned code-specs；不得静默改变父决策。
- 旧实现/旧测试只读冻结；不兼容、不双写、不分批切换。
- 本 child 在 start 前必须由父任务补齐 child-specific brief、exact decisions/manifests/owned paths/commands/handoff，并通过 no-history dry run。
- 严格执行父 `research/t02-platform-foundation-brief.md`，实现平台 skeleton、final Temporal topology、PG/Redis/MinIO、generated clients 和 platform tests，不实现产品 domain。
- 补齐 Temporal live reset/restart/history-point 和完整 management authorization/audit；资源/运行时长不作为门禁。

## Acceptance Criteria

- [ ] 所有前置 child 完成、合并并通过父任务检查。
- [ ] Child-specific brief/manifests/owned paths/validation 已由父任务批准，dry run 无文档歧义。
- [ ] 仅本 child 拥有的 spec 条款/scenarios 及直接受本 child 修改影响的 shared contracts targeted regression 通过。
- [ ] 只运行 child brief 明确列出的必要 unit/architecture/contract/real-PG/failure/redaction 类别；不重跑前置任务或全系统 suite。
- [ ] 未修改冻结旧实现，未触发产品分批交付或 cutover。
- [ ] 完成后提供可供 T03+ domain/product implementation 使用的明确 handoff。
- [ ] `research/t02-platform-foundation-report.md` 按模板完成并判定 PASS。

## Planning State

Child-level planning is complete. Consult `task.json.status`; implementation requires explicit `in_progress` and exclusive writer.
