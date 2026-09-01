# 完整人格成长闭环

## Goal

把上一轮新增的定期 `Wake-up` 从“能产生内在思考记录”扩展为完整、可审计、会影响未来行为的人格成长闭环。摇光必须能够从 World experience 经过 appraisal、内部动力学、注意、思考、欲望、agency、action/result、reflection，最终更新 self model、drives 和 preferences，并把更新后的模型用于下一次 wake-up。

## Background and confirmed facts

- 当前运行主线是 Go Core、Go Worker、Go BFF、Vue Web、PostgreSQL/pgvector、Redis Streams、Temporal 和 MinIO；Go Core 是领域事实与业务写入的唯一入口。
- 上一轮已实现 `wake_up.current` 长生命周期 Temporal workflow，默认每 30 分钟运行一次，使用 `Sleep + ContinueAsNew`，周期范围由 `product.wakeup` 的 300–86400 秒约束。
- 上一轮已新增 `cognition_wakeups`，写入 `attention`、`thought`、`desire`、`agency`、`internal_dynamics`、动作和结果，并为每次唤醒创建 `internal.wake_up` cognition fact 与 `reflection.run` intent。
- 现有 Reflection 已经可以基于 cognition evidence 产生并应用 memory、relationship、self-model、personality candidates；self model 位于 `fluctlights.provenance.self_model`，人格演进有 revision、evidence 和 outbox 记录。
- 现有 `fluctlight_inner_states` 已持久化 PAD、mood、momentum、regulation、drives、conflicts，并有 `fluctlight_state_revisions` 审计表；但 Reflection 尚未拥有 drive/preference recalibration 的结构化候选协议。
- 现有自治 Action 支持 `proactive_message`、`moment`、`media_request`；上一轮的 `CapabilityExecutor`/`CapabilityManifest` 已预留插件化 slot seam，但当前模型只能看到已安装能力。
- 当前 Web 已可编辑 `product.wakeup` 并在治理详情展示唤醒摘要；暂未提供独立的成长时间线或 drive/preference 编辑页面。

## Requirements

1. **完整阶段事实**：每次用户交互、生活事件或定期唤醒都必须形成有序、可追溯的 Experience；需要新增或扩展结构化 Appraisal、Internal Dynamics、Attention、Thought、Desire、Agency、Action、Result 记录，不能只保存自由文本。
2. **LLM 与 Core 职责分离**：Provider 负责语义 appraisal、attention、thought、desire、agency、reflection proposal；Go Core 负责事实、证据授权、数值边界、时间衰减、CAS、幂等、执行恢复和硬安全不变量。产品层不再额外限制 Action 类型，禁止用关键词、闲置时长、固定阈值或默认人格替代模型判断。
3. **内部动力学转移**：将 appraisal/Result 转换为有 revision 的 PAD、mood、momentum、regulation、drive、conflict 状态；模型不能直接提交未经验证的数值 delta，Core 必须记录 requested/applied delta、policy/model version、evidence refs 和 source fact。
4. **注意与欲望回路**：下一次交互或 wake-up 的上下文必须读取当前 drives/preferences/self model，并让它们成为 attention、thought、desire、agency 的输入；不得把上一轮状态当作静态展示字段。
5. **人格成长候选**：Reflection 必须支持并验证 drive recalibration、preference revision、future trigger/attention bias 等候选类型，并以证据窗口、置信度、revision/CAS 和幂等键落库。无效候选不能制造替代值。
6. **任意 Drive/Preference slot**：Drive 和 Preference 不使用固定字段白名单，而是通过 slot registry/projection 扩展。模型可以提出新的稳定 key、名称、语义说明和初始值；Core 验证 key/类型/范围/版本/证据/所有者，保留 slot 的 provenance、revision、decay/update policy、status 和 supersede 链，并让 Attention/Thought/Desire/Agency 读取当前 slot 集合。
7. **开放行动与结果回流**：Agency 可以选择能力目录中任何已注册、可执行的 Action，不再按 `proactive_message` / `moment` 等产品类型做额外限制；Core 仍必须执行能力 manifest、Owner 授权、硬安全、资源上限、稳定 ID 和幂等边界。Action 冻结后持久化执行结果、失败原因、Owner/关系反馈和预期偏差；Result 必须能成为下一次 Reflection 的证据。重试复用稳定 Action/Provider ID，不得重复副作用。
8. **与 Wake-up 配合**：定期 wake-up 必须运行同一套阶段协议，而不是旁路一套“每日 review”逻辑；周期唤醒产生的内部事实应和对话/生活事实共用 cognition inbox、reflection watermark 和演进审计。
9. **可观测与治理**：实例详情/诊断应能区分阶段、来源、证据、当前 revision、候选状态和动作结果；暂停自治时仍允许内部成长，但阻止未授权外显动作。
10. **兼容与迁移**：新增表/列使用新的有序 migration head，不修改已发布 migration；更新 Go Worker registry、dispatcher、生成契约（如有）和相关 smoke fixture。
11. **能力缺口 tool call**：新增内置 `capability.request` tool slot。当摇光判断需要当前不存在的能力时，必须通过 tool call 提交能力 key、名称、用途、理由、期望输入/输出、side-effect 级别、优先级和 evidence refs，写入需求池；不得用自由文本宣称能力已经执行。
12. **需求池人工治理与插件预留**：Owner 页面可查看全局需求池、来源 Fluctlight、重复请求、证据和审核状态，执行 reviewing/accepted/rejected/fulfilled 状态变更。插件补充由人手工完成，接入后通过现有 `CapabilityExecutor`/`CapabilityManifest` 注册和 preflight；运行时不自动安装代码或凭据。

## Acceptance Criteria

- [ ] 一个定期 wake-up 能写入 Experience、结构化 Appraisal、Internal Dynamics、Attention、Thought、Desire、Agency 和 Result 记录，并创建唯一 Reflection intent。
- [ ] 一个有效 Reflection 能在满足证据门槛时同时更新 self model，并创建或更新至少一个任意 Drive/Preference slot，写入对应 evolution/state revision；下一次 wake-up 的 context 能读到更新后的 slot。
- [ ] Agency 可以选择并执行任意满足能力 manifest 和硬安全不变量的 Action；其 Result 能进入后续 Reflection evidence；重试、重复 dispatch 或 Worker 重启不会产生重复副作用或状态 revision。
- [ ] 无效/缺字段/越界/外部 evidence 的模型输出 fail closed，不会产生默认人格、默认 drive 或伪造的语义状态。
- [ ] 暂停自治不会停止内部成长；被暂停或未授权的外显动作只记录 blocked/deferred 结果。
- [ ] 定期 wake-up 使用 Temporal durable timer + Continue-As-New；不存在进程内 ticker、Redis 延迟队列或第二 workflow runtime。
- [ ] Web 可配置 wake-up cadence，并能查看阶段摘要、成长变化和动作结果；未接入的底层能力不得被描述为已完成 UI 功能。
- [ ] Drive/Preference slot 不依赖重新发布代码即可新增、更新、停用和 supersede；slot 结构、值类型、版本和证据可在诊断/治理视图中追踪。
- [ ] 缺失能力只能通过 `capability.request` tool call 产生需求；需求池支持 owner-scoped evidence、全局去重/聚合和审核状态；接入插件后可追踪 fulfilled capability/version。
- [ ] Go Core、Gateway、Web、migration、fixture 和全栈 smoke 所需检查通过，文档与 code-spec 同步。

## Out of scope

- 本任务不引入第二个 LLM、第二个 workflow runtime、新的数据库或新的消息队列。
- 本任务不允许直接把自然语言思考写成事实；所有成长仍须通过结构化模型输出与证据验证。
- 首版不承诺跨多个 Fluctlight 的社会人格合并，也不把 Owner 未授权的基础设施、身份或安全边界纳入自主行动。

## Resolved product decisions

- **开放行动空间**：不增加产品层的 Action 类型限制。Wake-up 和交互中的 Agency 可以选择能力目录里任何经能力契约和硬安全校验可执行的行为，并进入同一套冻结、执行、结果回流和反思流程。暂停、授权、幂等、资源上限和禁止破坏性操作仍属于 Core 的工程/安全不变量，不是人格偏好的额外限制。
- **开放 Drive/Preference 空间**：不把 Drive/Preference 固定为 `social`、`exploration`、`rest` 等字段。它们通过 slot registry 管理；模型可以提出新 slot，Core 只约束 slot 的结构、版本、证据、资源边界和生命周期，不限制语义名称的集合。
- **Typed slot 值模型**：Drive slot 使用 `pressure` envelope（`pressure`、`salience`、`direction`，数值由 Core 约束在 `0..1`）；Preference slot 使用声明式 `scalar`、`categorical`、`set` 或 `bounded_object` schema。模型可以创建新 slot，但不能绕过 slot schema、证据和 revision 边界直接写入。
- **能力缺口由 tool call 提交**：模型不能调用不存在的能力，也不能用自然语言假装成功；它必须调用 `capability.request`，由 Core 写入需求池，Owner 人工评估并通过插件 seam 补充实现。
- **需求池聚合范围**：需求池按稳定 `capability_key` 做全局聚合，同时保留每个 Fluctlight 的原始 request、source fact、证据、理由和审核历史；插件接入是系统级能力，但 fulfilled 不能抹掉来源人格的需求事实。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
