# T12 全量集成、切换与旧系统删除

## Goal

执行完整 capability matrix e2e/failure/security/backup 验收，一次性切换，并删除旧 server/web/test/SQLite/jobs/routes/dependencies/docs/CI，确保仓库只剩一套生产实现。

## Requirements

- 父任务与依赖门禁：T01B and T02-T11 completed, merged and parent integration review approved。
- 实施范围：全 capability matrix e2e/failure/security/backup 验收、一次性切换、旧 server/web/test/SQLite/jobs/routes/dependencies/docs/CI 删除、唯一生产实现确认。
- 遵守父任务 `READ_FIRST.md`、`decisions.md`、`design.md`、capability inventory 和 assigned code-specs；不得静默改变父决策。
- 旧实现/旧测试只读冻结；不兼容、不双写、不分批切换。
- 本 child 在 start 前必须由父任务补齐 child-specific brief、exact decisions/manifests/owned paths/commands/handoff，并通过 no-history dry run。

## Final Acceptance Criteria

- [ ] 所有前置 child 完成、合并，且每个 T03-T11 handoff 明确记录 `acceptance_owner=T12`、`acceptance=pending`、实现证据、剩余风险、T12 coverage IDs 和排除范围。
- [ ] T12 child-specific brief/manifests/owned paths/final commands/rollback 已由父任务批准，dry run 无文档歧义。
- [ ] 所有 assigned specs 的完整 contract/error matrix/test union 进入最终验收并通过；T03-T11 的局部实现证据不能替代 T12 重跑。
- [ ] 新系统 architecture/contract/real-PostgreSQL/failure/security/redaction/backup/restore/upgrade tests 通过，且完整 Compose readiness、Required capability matrix 和跨模块 e2e 通过。
- [ ] `REQUIRED_CLEANUP` 的旧 server/web/test/SQLite/jobs/routes/dependencies/docs/CI 删除证明通过，仓库只剩一套生产实现。
- [ ] `EXCLUDED_FUTURE_OR_RESERVED` 与 `EXCLUDED_PLACEHOLDER_ONLY` 不生成正向验收用例；仅通过 scope guard 证明未暴露、未越界实现或误标记为 delivered。
- [ ] 冻结旧实现保持不变直到 pre-cutover gates 通过；然后仅执行一次 product cutover，不做分批交付或新旧混跑。
- [ ] 最终 handoff 记录完整 validation matrix/results、cutover evidence、deletion manifest、post-cutover smoke/readiness、rollback/restore point 和剩余运维限制；没有后续 child。

## Planning State

Child exists in `planning` but is not executable yet. The parent program outline is not implementation approval.
