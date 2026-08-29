# T08 Life World、Schedule 与 Autonomy

## Goal

实现 Context/Event/Schedule/timezone、full-day planning/replan、Goals/Intentions workflow、pending/deferred/proactive actions、budget/quiet-hours/pause/cancel 和 reconciliation。

## Requirements

- 父任务与依赖门禁：T07 completed and merged。
- 实施范围：life_world module、Context/Event/full-day Schedule/timezone/replan、Goals/Intentions workflows、pending/deferred/proactive actions、budget/quiet-hours/pause/cancel/reconciliation。
- 遵守父任务 `READ_FIRST.md`、`decisions.md`、`design.md`、capability inventory 和 assigned code-specs；不得静默改变父决策。
- 旧实现/旧测试只读冻结；不兼容、不双写、不分批切换。
- 本 child 在 start 前必须由父任务补齐 child-specific brief、exact decisions/manifests/owned paths/commands/handoff，并通过 no-history dry run。

## Implementation Evidence (Not Acceptance)

- [ ] 前置 child 已完成并合并；child brief、owned/forbidden paths、implementation-check commands、T12 coverage IDs 和 handoff 模板已由父任务批准。
- [ ] 本 child 只记录实现所需的格式化、类型、导入、接口/生成物和最小本地检查；这些结果不构成 child PASS、production readiness 或 cutover authorization。
- [ ] Functional contract、real-PostgreSQL、workflow/restart/DST/failure、security/redaction、cross-module autonomy 和 full-product validation 全部由 T12 重新执行；本 child 不为这些场景生成 acceptance 结果。
- [ ] Future-only、reserved、placeholder-only 功能不在本 child 中做正向验证；`pending/deferred` 只有在 Required lifecycle 中才由 T12 验收。
- [ ] 冻结旧实现未修改，未触发产品分批交付或 cutover。
- [ ] 完成后提供包含 changed paths、implementation evidence、remaining risks、contract/schema artifacts、`acceptance_owner=T12` 和 `acceptance=pending` 的明确 handoff。

## Planning State

Child exists in `planning` but is not executable yet. The parent program outline is not implementation approval.
