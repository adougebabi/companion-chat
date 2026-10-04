# 历史任务与沿用边界

主代理完整阅读 README.md、CONTEXT.md、docs/capability-architecture.md、docs/persona-takeover-architecture.md，以及下列四份设计与治理衣柜 PRD。文档若与当前代码不同，以当前 formal registry/调用链和测试为准；特别是 README 和较早 capability 文档关于两轮/deferred 的描述已被 09-22 的正式工具方案替代。

| 任务 | 元数据状态 | 可复用成果与限制 |
|---|---|---|
| .trellis/tasks/09-27-yaoguang-runtime-consistency | planning | design.md、research/confirmed-paths.md:21-47；1e68eab 涉及时间/有效活动/ADK 诊断。研究症状不代表当前仍未修；T01 这轮统一参考时区替代其按作者显示的旧方案，用户时区保持显式未知。 |
| .trellis/tasks/09-29-yaoguang-request-context-budget | in_progress | design.md、acceptance-report.md:43-71；4680d55/7415910 已有 wire budget/精简结果；报告明确累计摘要、tokenizer、live 与回放未完成。 |
| .trellis/tasks/09-22-agent-tool-plugin-e2e | in_progress | design.md、research/delivery.md、acceptance-matrix.md；5a0c407 独立 Tool、真实 receipt、原生多轮 Loop；报告真实 Agent/视觉未完成，不能沿用其单测数字作为本轮通过证据。 |
| .trellis/tasks/archive/2026-09/09-27-goal-schedule-hairdye-closure | completed | design.md；36355bd typed intention→schedule→due activity/result；不证明购买非服装与事实纠正的完整闭环。 |
| .trellis/tasks/archive/2026-10/10-01-governance-wardrobe-edit | completed | aea34ce、5b0dbc6、5f75004 治理衣柜 CRUD/路由/visual prompt；prd.md 仍 TBD，不能当本轮验收证据。 |

不更改旧任务状态、不批量链接为子任务、不重复创建其平台能力。本任务以集成修复直接承担代码与证据交付，内部按切片实施；同一事实/时间/结果契约需要共同验收，不拆成可独立宣称完成的 T00—T10 任务。

本轮子代理仅探索；实施方案取舍、代码修改、最终验证由主代理负责（用户 AGENTS.md 的明确约束）。Trellis 默认 implement/check 代理建议在此按用户约束采用主线程实现与最终验证，必要时只派只读独立核验。
