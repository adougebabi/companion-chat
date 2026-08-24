# 修复摇光详情、聊天输入、安排生成、调试展示与分组新增

## Goal

修复摇光实例在详情、聊天、生活计划、调试和分组管理中的可见行为回归，让用户看到正确的当前外观、发送后立即清空输入、持续生成并展示生活安排、以可读链路理解人格涌现，并能发现和创建联系人分组。

本轮优先级：先恢复“新实例自动生成后续生活安排”的后端生产链路。其他四项保留在父任务的集成验收中，但不得阻塞该优先修复的启动和验证。

## Background and confirmed facts

- 详情页 [PersonaDetail.vue:49] 使用 `Object.values(state.appearance).join(' · ')`。后端 life-state fallback 可以把 blueprint 的字符串 `visualBaseline` 原样放进 `state.appearance`，所以字符串会被按字符逐个拼接。
- 聊天发送链路在 [useComposer.ts:76-101] 只有等待完整 SSE `done` 后才清空 draft；长回复或 provider 延迟期间，用户会看到已发送文本仍留在输入框。现有逻辑还需要保留失败重试和避免覆盖发送期间的新草稿。
- 新 persona 创建时只产生当天 `ready` daily plan 和全天 `baseline_idle`，maintenance `daily_plan` job 只同步已有计划，没有跨本地日创建下一份计划的生产链路。详情接口/组件目前只展示显式 `schedule`，不读取 daily-plan/timeline slots。
- 调试页 [DebugView.vue:122-126] 将四类 emergence 数据直接 `JSON.stringify` 到 `<pre>`，前端没有按交互或阶段组织的可读链路展示。
- 分组创建 API 和 Pinia action 仍存在；Vue 迁移后显式“新增分组”按钮/向导消失，只剩通过 `window.prompt` 输入 `+` 的隐藏路径。创建成功后也没有自动选中新组。
- 既有契约要求：persona 始终属于有效 group；debug 数据必须 persona-scoped、redacted、bounded；聊天遵循 Enter 发送、Shift+Enter 换行；ready daily plan 是当天状态权威来源；不得用默认人格或关键词回退伪造 emergence 结果。

## Requirements

### R1. 当前外观渲染

- `state.appearance` 为字符串时，详情页完整显示该字符串，不得逐字符插入 `·`。
- `state.appearance` 为对象时，继续以稳定、可读的字段值展示；空值显示已有空状态。
- 在 state projection 或 API 边界增加必要的类型/归一化保护，避免同类合法 JSON 值再次触发字符级渲染。

### R2. 发送后输入清理

- Enter 和发送按钮都在发送被接受后立即清空当前 draft/textarea。
- Shift+Enter、IME composing、空白输入和重复发送语义保持现有约定。
- 发送失败时，在用户未修改草稿且仍处于同一 persona 的情况下恢复原文；用户已开始输入的新草稿不得被旧请求完成回调覆盖或清空。

### R3. 生活安排和关系变化

- **P0 自动安排生成（本轮先做）**：daily-plan 维护链路必须按 persona 本地日期幂等地产生后续日期的 ready plan、baseline slot 和 generation job，不能让新 persona 只停留在创建日计划。
- 计划生成后，非 baseline timeline slot 必须按现有 idempotency 规则产生 candidate job；重复 tick/重启不能重复创建。
- 关系变化必须继续遵守现有 evidence/evaluator 约束；没有关系证据时不伪造变化。若当前生产聊天链路尚未提交 evidence，应补上可审计的接入或明确显示“暂无关系变化”的状态。
- 详情页的“最近安排”需要展示产品确认范围内的计划来源（见 Open question）；显式用户日程的优先级和现有状态来源契约不能被破坏。

### R4. 人格涌现调试展示

- 将 appraisal、memory consolidation、self-model、agency intention 从原始 JSON 改为按交互/证据关联组织的纵向步骤链。
- 每一步至少展示阶段名、核心摘要、状态、置信度（如有）、来源/证据引用和错误信息（如有）；缺失阶段显示明确的未产生状态。
- 保留 bounded/redacted 原始详情的折叠查看能力；遇到后端截断字符串时显示“数据已截断”，不能按对象访问导致页面异常。
- 展示必须保持 persona-scoped，不扩大 debug API 暴露范围。

### R5. 分组新增入口

- 在联系人页或详情页提供可见、可访问的“新增分组”入口，调用现有创建 API，并在成功后刷新 groups/persona counts。
- 创建成功后自动选中新组，或至少给出明确的当前分组反馈；默认组不可删除，persona 归属必须始终满足后端约束。
- 不再要求用户记忆 `window.prompt` 中的特殊 `+` 指令才能创建分组。

## Acceptance Criteria

- [ ] 字符串型 `visualBaseline` 在实例详情中按完整文本显示；对象型 appearance 和空值回归测试通过。
- [ ] Enter/点击发送在 SSE 尚未结束时输入框已清空；失败恢复原文，新草稿和 persona 切换不被旧回调覆盖；Shift+Enter/IME/重复发送行为保持不变。
- [ ] 将时钟推进到新 persona 创建日后的至少两个本地日，每日存在唯一 ready daily plan、baseline slot 和对应 job；重复执行保持幂等。
- [ ] 含非 baseline 计划项时，timeline slot、candidate job 和后续 life-event/proactive 链路按现有契约可审计地产生；无证据时不产生虚假的关系变化。
- [ ] 调试页不再以单一 `<pre>` 作为人格涌现主视图；用户能按交互看到四阶段链路、状态/置信度/证据/错误，且截断数据和空阶段有明确提示。
- [ ] 联系人/详情页面存在可见新增分组操作；创建、刷新计数、自动选中新组和 persona 归属均可验证。
- [ ] 相关后端测试、前端 typecheck/build 通过，且没有违反 debug 脱敏、persona scope、Enter/Shift+Enter 和 group 完整性契约。

## Open question

- “最近安排”是否应把 AI 生成的 daily-plan/timeline slots 与用户显式创建的 schedule 一并展示？已确认：是，按时间合并，标注来源（AI 计划/用户安排）；本轮先确保后端计划生成，详情消费端随后接入。

## Task map

- Child task: `自动生成后续生活安排`（P0，先实现和验收跨本地日 daily-plan 生成、幂等和 worker/job 链路）。
- Parent follow-up: 外观渲染、聊天 draft 清理、涌现链路可视化、分组新增入口，以及“最近安排”对 daily-plan/timeline 的展示整合。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
