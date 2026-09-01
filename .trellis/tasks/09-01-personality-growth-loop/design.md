# 技术设计：完整人格成长闭环

## 1. 目标与边界

本任务把上一轮的 `wake_up.current` 从“定期生成内在摘要”扩展为统一的认知与成长运行时。对话、生活事件和定期唤醒都进入同一条事实流水线；差别只在于触发事实的来源，不再为 Wake-up 维护一套旁路的每日 Review 语义。

产品层不限制人格可以选择的 Action 类型。Agency 可以从当前已安装、已 preflight 的 Capability manifest 中选择任意可执行能力，或选择无外部副作用的内部动作。Go Core 仍保留不可绕过的技术不变量：来源/所有权、能力参数 schema、Owner 授权、硬安全、资源边界、稳定幂等键、CAS、lease、取消和审计。这些是系统正确性边界，不是人格偏好的 Action allowlist。

## 2. 目标架构

```text
World / Human / Timer
        ↓
Experience fact（cognition inbox，单调 sequence）
        ↓
LLM Appraisal
        ↓
Core Internal Dynamics reducer（PAD/mood/momentum/regulation/drives/conflicts）
        ↓
LLM Attention → Thought → Desire → Agency
        ↓
Core freeze（Capability Action / no-op，稳定 action id）
        ↓
Temporal execution → Result fact（成功/失败/反馈/预期偏差）
        ↓
Reflection evidence window（watermark + state revision CAS）
        ↓
Self Model / Personality / Drives / Preferences / Trigger bias revisions
        ↓
下一次 ContextProjection 的 attention 输入
```

### 2.1 领域存储

- 扩展 `cognition_inbox` payload/event type，支持 `conversation.turn`、`life.*`、`internal.wake_up` 和 `autonomy.result`，所有来源共用 Fluctlight sequence、causation、correlation、idempotency。
- 新增结构化 cognition ledger（可拆成表或统一 JSON projection，但必须具备事实 ID、阶段、revision、evidence refs、model/prompt/policy version、status、错误码和时间边界）：
  - `cognition_appraisals`
  - `cognition_internal_dynamics`
  - `cognition_focus_cycles`（attention/thought/desire/agency）
  - `cognition_action_results`
- 新增人格成长投影/修订：
  - `fluctlight_drive_slots` / `fluctlight_drive_revisions`
  - `fluctlight_preference_slots` / `fluctlight_preference_revisions`
  - `fluctlight_trigger_preferences`（影响未来唤醒/注意候选，不是关键词监听器）
- Drive/Preference slot 使用统一 envelope：`slot_id`、稳定 `key`、`kind`、`label`、`description`、`value`、`value_schema`、`confidence`、`evidence_refs`、`source_window`、`revision`、`status`、`provenance`、`decay_policy` 和 `update_policy`。Slot key 是实例范围内唯一的，停用使用状态变更或 supersede，不删除历史。
- 保留现有 `cognition_wakeups` 作为 wake-up 入口摘要；它通过 fact ID 关联完整阶段 ledger，不重复保存另一套事实。

### 2.2 Provider 交互

- 新增/扩展结构化 Provider role schema：
  - `cognitive_assessment`：Experience → Appraisal/Attention/Thought/Desire/Agency/Action proposal。
  - `reflection`：Evidence window → memory/relationship/self-model/personality/drive-slot/preference-slot/trigger candidates。
- Provider 只能返回语义候选；不接受未经 Core policy 验证的 PAD、drive 或关系原始 delta。
- 所有 prompt 都带有真实的 bounded evidence/context，不从可见回复文本反推人格事实。
- 非法 schema、外部 evidence、越界数值和未知 candidate 必须 fail closed，不合成默认中性人格。

### 2.3 Core 状态转移

- Internal Dynamics reducer 读取当前 state + elapsed time + 已验证 appraisal/result，按显式 policy 计算 bounded requested/applied delta。
- 每次状态变化只推进一次 revision，并在 `fluctlight_state_revisions`/新 revision 表中记录 previous/resulting state、requested/applied delta、evidence refs、policy/model version、reason 和 idempotency key。
- Drives 不使用固定语义集合。每个 Drive slot 的运行时值必须通过 `value_schema` 声明，Core 对可用于动力学计算的数值 pressure 统一执行 `0..1` 边界、衰减和更新策略；尚未有 reducer 的新 slot 可以保留并展示，但不能被隐式当作另一个已知 drive。
- Preferences 不使用固定字段白名单。Preference slot 可以是有 schema 的 scalar、categorical、set 或 bounded object；每次变更都以证据绑定的 claim/revision 表示，不直接覆盖 Foundation，下一轮 projection 读取当前 active slots。
- 模型可以提出新 slot 或更新现有 slot，但不能直接写库；Core 先验证 key、kind、value_schema、大小、证据和 owner，再在同一 Reflection transaction 中创建/更新 slot 与 revision。

### 2.4 Action / Result

- Agency 产出 capability action proposal 或 no-op；Capability registry 是可扩展的 Action 边界，不再在 wake-up 代码中写死 action type 分支。
- Action 先冻结，后由 `CapabilityActionWorkflow` 或兼容的自治 workflow 执行；结果写回 action、cognition result fact 和 wake-up/interaction correlation。
- 结果包括 `status`、bounded output/error、completed_at、provider request ID、expected-vs-observed summary 和 feedback evidence refs。
- 重试使用相同 action/provider/workflow IDs；过期 lease、取消和重复 settlement 不得产生第二个副作用。

### 2.5 Reflection 与未来 Trigger

- Reflection 使用同一 evidence watermark 和 `state_revision` CAS，扩展候选验证/应用：drive-slot recalibration、preference-slot revision、attention priority、future trigger preference。
- 每类慢变量都需要 evidence window 和置信度门槛；应用失败时 proposal、candidate、watermark 和 revision 全部回滚。
- Trigger preference 只能作为后续 wake-up 的结构化上下文/优先级输入；不生成自由文本关键词匹配器，不根据“多久没收到消息”直接发起行为。
- Wake-up workflow 每次重新读取 cadence 和 projection；人格成长不依赖某个特定 Web 页面存在。

### 2.6 能力缺口与插件请求

- 在 `CapabilityExecutor` registry 中新增一个始终可用的 native slot：`capability.request`。它不是外部能力，而是一个把缺口写入需求池的领域工具。
- Tool 参数采用严格 schema：`capability_key`、`title`、`description`、`rationale`、`desired_contract`、`side_effect_class`、`priority`、`evidence_refs`、`idempotency_key`。参数只表达需求，不包含凭据、任意代码或可直接执行的 provider 地址。
- `capability.request` executor 校验 source fact/Fluctlight ownership、字段长度、key/name 格式、evidence refs 和幂等键，在一个事务中写入 `capability_requests`、source link/outbox，并返回 `ToolResultV1{status: proposed, request_id}`。
- 需求状态为 `proposed → reviewing → accepted | rejected → fulfilled`，支持 `cancelled`；Owner 评估和插件接入是人工操作，模型不能批准自己的需求，也不能自动安装插件。
- 需求池按稳定 `capability_key` 聚合多个 Fluctlight 的 request，但每个来源事实、证据和提出者仍保留。Owner 可以从全局池查看来源与聚合计数，并按 request 单独记录备注/状态。
- 现有 `CapabilityManifest`/`CapabilityExecutor` 是插件 seam。人工接入插件后，注册 executor、完成 preflight、通过 manifest/tool contract 测试，再将对应需求标记为 `fulfilled`；未 fulfilled 的需求不会出现在可执行能力 catalog 中。
- Conversation 和 Wake-up 的 assessment 都使用 `StructuredWithTools`，把 `capability.request` 与已安装能力一起提供给模型。缺失能力请求本身可作为 internal result fact 进入 Reflection，但不会伪造成功的 external action。

## 3. 跨层数据流

1. Worker/HTTP 将外部、内部或 timer source 写入 cognition inbox，并以稳定 idempotency key 关联 workflow intent。
2. Cognition activity claim fact，调用 assessment，Core reducer 提交 appraisal 与 internal dynamics revision。
3. Focus cycle 使用更新后的 projection 生成 attention/thought/desire/agency；Core 验证并冻结开放 Capability action。
4. Temporal 执行 action；结果以 action result fact 回到 cognition inbox，并触发 reflection intent。
5. Reflection 读取 bounded evidence window，应用成长 candidates，推进 self-model/drive/preference/trigger revision。
6. 下一次 projection 读取这些 revision；旧事实保持不可变，浏览器只读安全摘要。
7. 当模型需要不存在的能力时，调用 `capability.request`；Core 写入全局聚合需求池，BFF/Web 提供 owner-only review surface，人工接入插件后再回写 fulfilled。

## 4. 兼容、迁移与回滚

- 使用新 migration head，禁止改写已发布 SQL；旧的 `cognition_wakeups` 行和 reflection proposal 必须可读取。
- 旧 `daily_review.current_day` 保留为生活世界 Schedule-ready fact 的兼容入口，但其生成的 Experience 必须进入新的统一阶段协议；不能产生第二套人格状态。
- 旧自治 Action (`proactive_message`、`moment`、`media_request`) 通过 generic action adapter 继续执行；新增 capability 不改变已有 Browser DTO。
- Workflow history 使用显式 versioning/replay 测试；新阶段字段采用可选 envelope 或 Continue-As-New 边界，避免旧历史无法重放。
- 回滚顺序：停止新 wake-up dispatch → 保留已提交事实/结果 → Worker 回退到兼容 workflow build → 迁移不删除新表，读取路径继续容忍空 projection。

## 5. 可观测性与治理

- 每个阶段、revision、action、result 和 reflection proposal 使用同一 correlation ID；诊断只展示 bounded summary、status、evidence refs、版本和错误码。
- Web 治理显示最近 wake-up、成长变化和 action/result 状态；不展示完整 prompt、凭据或隐藏 reasoning。
- Owner 可以暂停外显动作；暂停不停止内部 Experience、Internal Dynamics、Reflection 和 self-model/drive/preference 成长。

## 6. 关键取舍

- 统一 cognition ledger 会增加迁移和 projection 复杂度，但避免 wake-up、对话和生活事件各自拥有不可互认的“人格状态”。
- 不使用 Temporal Schedule API，而使用已有 domain intent + long-lived workflow，是为了保持 PostgreSQL 事实、重试和恢复的单一权威。
- 开放 Action 空间通过 Capability manifest 扩展，而不是在应用层硬编码白名单；这样满足“人格不受产品类型限制”，同时保留安装能力和硬安全边界。
- 能力缺口用 native tool 记录而不是自由文本 marker 或自动安装流程，既让摇光能表达“我需要什么”，又让 Owner 保有插件代码/凭据/部署的明确控制权。
