# T10 完整 Web 产品与 Control Center

## Goal

完成 Vue contacts/groups/create/detail/settings/governance/Moments/Diagnostics/Control Center UX、响应式与可访问性，并覆盖 capability inventory 全部浏览器场景。

## Requirements

- 父任务与依赖门禁：T09 completed and merged。
- 实施范围：完整 Vue contacts/groups/create/detail/settings/governance/Moments/Diagnostics/Control Center、generated browser client、responsive/accessibility 和供 T12 使用的 capability inventory browser scenario seams。
- 遵守父任务 `READ_FIRST.md`、`decisions.md`、`design.md`、capability inventory 和 assigned code-specs；不得静默改变父决策。
- 旧实现/旧测试只读冻结；不兼容、不双写、不分批切换。
- 本 child 在 start 前必须由父任务补齐 child-specific brief、exact decisions/manifests/owned paths/commands/handoff，并通过 no-history dry run。

## Implementation Evidence (Not Acceptance)

- [ ] 前置 child 已完成并合并；child brief、owned/forbidden paths、implementation-check commands、T12 coverage IDs 和 handoff 模板已由父任务批准。
- [ ] 本 child 只记录 Web/Control Center 页面、generated browser client、a11y seams、fixtures 和最小本地检查；这些结果不构成 child PASS、production readiness 或 cutover authorization。
- [ ] Complete browser flows、responsive/accessibility、Diagnostics/Control Center、auth/config/media integration 和 capability matrix 全部由 T12 重新执行；本 child 不为这些场景生成 acceptance 结果。
- [ ] Future-only、reserved、placeholder-only 页面/功能不在本 child 中做正向验证；相关 deferred risk 和 T12 coverage ID 必须进入 handoff。
- [ ] 冻结旧实现未修改，未触发产品分批交付或 cutover。
- [ ] 完成后提供包含 changed paths、implementation evidence、remaining risks、contract/schema artifacts、`acceptance_owner=T12` 和 `acceptance=pending` 的明确 handoff。

## Planning State

Child exists in `planning` but is not executable yet. The parent program outline is not implementation approval.
