# T04 Fluctlight 基础与内部状态

## Goal

实现 Fluctlight lifecycle、identity/personality/behavioral policy、revision/governance，以及 affect/mood/momentum/drives/goals/intentions 数值状态与历史。

## Requirements

- 父任务与依赖门禁：T03 completed and merged。
- 实施范围：fluctlights/inner_state modules：identity、personality、behavioral policy、revision/governance、affect/PAD/mood/momentum/regulation、drives、Goals/Intentions core state 与 numeric policy。
- 遵守父任务 `READ_FIRST.md`、`decisions.md`、`design.md`、capability inventory 和 assigned code-specs；不得静默改变父决策。
- 旧实现/旧测试只读冻结；不兼容、不双写、不分批切换。
- 本 child 在 start 前必须由父任务补齐 child-specific brief、exact decisions/manifests/owned paths/commands/handoff，并通过 no-history dry run。

## Acceptance Criteria

- [ ] 所有前置 child 完成、合并并通过父任务检查（D038 明确将 T03 未完成状态作为 carry-forward risk）。
- [x] Child-specific brief/manifests/owned paths/validation 已由父任务批准，dry run 无文档歧义。
- [x] 仅本 child 拥有的 spec 条款/scenarios 及直接受本 child 修改影响的 shared contracts targeted regression 通过。
- [x] 已运行 child brief 允许的 focused unit/architecture/contract 类别，未重跑前置任务或全系统 suite；real-PG/runtime 类别按 Owner 指令 deferred。
- [x] 未修改冻结旧实现，未触发产品分批交付或 cutover。
- [x] 已在 `research/t04-fluctlight-foundation-inner-state-report.md` 提供可供 cognitive runtime orchestration and user-facing flows 使用的明确 handoff，并列出 deferred gates。

## Planning State

The parent program outline was expanded into a child-specific brief and no-history dry run under parent decision D038. The Owner explicitly authorized implementation while T03 remains `in_progress`; this exception does not mark T03 complete or merged. Docker, Compose, long-running process, and full-stack runtime acceptance are explicitly deferred for this session and remain open acceptance items.
