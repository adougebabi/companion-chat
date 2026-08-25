# T04 Fluctlight 基础与内部状态

## Goal

实现 Fluctlight lifecycle、identity/personality/behavioral policy、revision/governance，以及 affect/mood/momentum/drives/goals/intentions 数值状态与历史。

## Requirements

- 父任务与依赖门禁：T03 completed and merged。
- 实施范围：fluctlights/inner_state modules：identity、personality、behavioral policy、revision/governance、affect/PAD/mood/momentum/regulation、drives、Goals/Intentions core state 与 numeric policy。
- 遵守父任务 `READ_FIRST.md`、`decisions.md`、`design.md`、capability inventory 和 assigned code-specs；不得静默改变父决策。
- 旧实现/旧测试只读冻结；不兼容、不双写、不分批切换。
- 本 child 在 start 前必须由父任务补齐 child-specific brief、exact decisions/manifests/owned paths/commands/handoff，并通过 no-history dry run。

## Implementation Evidence (Not Acceptance)

- [ ] 所有前置 child 状态、D038 carry-forward risk、child brief、owned/forbidden paths、implementation-check commands、T12 coverage IDs 和 handoff 模板已记录。
- [x] 已完成实现授权文档和 no-history dry run；该结果只证明 implementation readiness，不证明 child acceptance。
- [x] 已记录 focused implementation evidence，未将其标记为 product PASS；real-PG/runtime、cross-module、failure/security 和 full-product acceptance 均 pending T12。
- [x] 未修改冻结旧实现，未触发产品分批交付或 cutover。
- [x] 已在 `research/t04-fluctlight-foundation-inner-state-report.md` 提供 implementation handoff，列出 deferred gates、T12 coverage IDs、`acceptance_owner=T12` 和 `acceptance=pending`。

## Planning State

The parent program outline was expanded into a child-specific brief and no-history dry run under parent decision D038. The Owner explicitly authorized implementation while T03 remains `in_progress`; this exception does not mark T03/T04 PASS or production readiness. Docker, Compose, long-running process, real-PostgreSQL, full-stack and final functional acceptance are T12-owned and remain pending.
