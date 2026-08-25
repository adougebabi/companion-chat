# T11 Backup、Restore、Upgrade 与运维

## Goal

实现 PostgreSQL+object+.env backup/restore manifest、完整性校验、migration/Temporal active-workflow upgrade、cleanup、NAS operator docs 和恢复演练。

## Requirements

- 父任务与依赖门禁：T10 completed and merged。
- 实施范围：PostgreSQL+object+.env backup/restore manifest、integrity、Alembic/Temporal active-workflow upgrade、diagnostics/media cleanup、NAS operator docs 与 recovery drills。
- 遵守父任务 `READ_FIRST.md`、`decisions.md`、`design.md`、capability inventory 和 assigned code-specs；不得静默改变父决策。
- 旧实现/旧测试只读冻结；不兼容、不双写、不分批切换。
- 本 child 在 start 前必须由父任务补齐 child-specific brief、exact decisions/manifests/owned paths/commands/handoff，并通过 no-history dry run。
- 实现 against-restored Temporal default+visibility databases 的恢复路径和 active-workflow resume seams；最终 active workflow 恢复验收由 T12 执行。不要求固定时长 soak 或资源/磁盘研究。

## Implementation Evidence (Not Acceptance)

- [ ] 前置 child 已完成并合并；child brief、owned/forbidden paths、implementation-check commands、T12 coverage IDs 和 handoff 模板已由父任务批准。
- [ ] 本 child 只记录 backup/restore/upgrade procedures、operator docs、recovery seams 和最小本地检查；这些结果不构成 child PASS、production readiness 或 cutover authorization。
- [ ] Empty-deployment restore、PostgreSQL/object/.env integrity、previous-release migration、restored Temporal Server/Worker active-workflow resume 和 backup/restore security 全部由 T12 重新执行；固定时长/resource soak 不属于验收。
- [ ] Future-only、reserved、placeholder-only 运维功能不在本 child 中做正向验证；相关 deferred risk 和 T12 coverage ID 必须进入 handoff。
- [ ] 冻结旧实现未修改，未触发产品分批交付或 cutover。
- [ ] 完成后提供包含 changed paths、implementation evidence、remaining risks、contract/schema artifacts、`acceptance_owner=T12` 和 `acceptance=pending` 的明确 handoff。

## Planning State

Child exists in `planning` but is not executable yet. The parent program outline is not implementation approval.
