# 人格分层与防漂移

## Goal

建立清晰、可审计且不会因日常聊天/反思而漂移的人格状态模型，将稳定的 Core Persona、带证据的 Developing Self，以及高度动态的 Current State 分开管理，确保后续对话在保持核心身份一致的前提下仍能形成真实的自我认知和即时状态变化。

用户价值：人格可以持续成长，但“成长”不再等同于“改写人格”；用户能够理解某个自我认知从何而来、可信度多高、由谁/什么过程产生，并能区分长期特征和当前情绪。

## Background / Confirmed Facts

- 当前运行时已经具备 Foundation、Reflection、ContextProjection、Inner State、Memory、Relationship 和 typed slots 等能力，但它们没有形成严格的三层所有权边界。
- Core 当前将 `identity`、`personality`、`behavioral_policy`、`life_profile` 存在 `fluctlights` 的 JSONB 字段中，并通过 Foundation revision 进行人工治理。
- Reflection 当前可以直接更新 `fluctlights.personality`，并将 self model 写入 `provenance.self_model`；这会把“可演进观察”直接提升为基础人格事实，是人格漂移的主要风险点之一。
- `fluctlight_evolution_revisions` 已记录部分演进审计，但当前 personality 的 `update_policy`（`max_delta`、`cooldown_seconds`、`minimum_confidence`、`evidence_window_events`）没有完整参与实际更新决策。
- Self model 当前存在两个潜在存储位置：运行时代码使用 `provenance.self_model`，数据库兼容迁移另有独立 `fluctlights.self_model` 列；新实现选择 Developing Self claim 表作为唯一权威来源，不读取旧数据。
- `ContextProjection` 是 cognition、chat、wake-up、reflection、media 等流程共享的上下文入口，当前已同时暴露 personality、self model、inner state、life context、memories、relationships 和 hypotheses，但没有明确层级优先级和只读/可写契约。
- Current State 主要存放于 `fluctlight_inner_states` 及其 revision/audit 表；Detail 读取目前遗漏 `momentum`、`regulation`、`conflicts`，属于现有上下文契约缺口。
- 当前已有 evidence-backed `cognition_claims`、memory 和 typed slots 机制，可作为 Developing Self 的证据与过期/置信度语义参考，但不能未经边界定义直接视为 Core Persona。
- 现有系统已有 foundation、evolution、state 三类 revision 概念；新模型不能把它们混为一个无语义的 revision 数字。

## Requirements

### R1. 分层语义与所有权

- 系统必须明确提供三层语义：
  - **Core Persona**：稳定身份、核心价值、长期行为原则和不可越权边界；普通聊天、Current State 和自动 Reflection 不得直接修改。
  - **Developing Self**：对“我是什么样的人”的可发展假设；每条记录必须携带 evidence、confidence、provenance，并支持不确定、冲突、被替代或过期等状态。
  - **Current State**：当前情绪、精力、兴趣、疲惫、烦躁、即时关注等短期状态；可快速变化，不得自动沉淀为 Core Persona。
- 层之间必须有明确的写入权限、读取语义和冲突处理规则。
- Core Persona 是显式、自动不可写的稳定语义层；新实例以 `fluctlights.core_persona` 作为 canonical 快照，现有四个 Foundation 字段作为同实例页面投影，不能作为 Reflection 自动演进的写入目标。
- 首版字段归类固定为：`identity` 身份锚点/核心价值/世界观、全部现有 `personality` 稳定基线、稳定 `behavioral_policy`、`life_profile.character_constraints` → Core Persona；偏好/习惯/情绪反应/自我解释 → Developing Self；`inner_state`、PAD/mood/momentum/regulation、resolved context、Presence、当前 life event → Current State；memory、relationship、goal、intention、schedule → 支持领域。
- Core Persona 使用 JSONB 快照 + Foundation revision 审计；Developing Self 使用逐条 claim 表 + revision ledger；Current State 继续使用 `fluctlight_inner_states` 快照和状态审计。
- 初始配置中用户明确给出的偏好可以作为高置信度 `owner_defined` Developing Self seed，但不升级为 Core Persona。
- Developing Self 是带 confidence/provenance 的软上下文；Core Persona 约束优先，冲突必须记录，自动 promotion 被禁止。
- 候选写入、结构化行为决策和工具行为由 Core 硬校验；ContextProjection 明确三层 authority/priority；第一阶段不对每条消息增加二次 LLM 风格审查。
- 不迁移、不回放、不转换旧实例或旧人格演进数据；新实例从新的 Core Persona、Developing Self 和 Current State 开始。
- 保留现有自然语言分析→预览→激活和 blank-slate 入口，但初始化/Reflection contract 改为 `core_persona`、`developing_self.claims` 和 `developing_self_candidates`；Current State 由系统中性初始化。

### R2. 防止自动人格漂移

- Reflection 产生的观察必须先进入 Developing Self 或其他合适的演进槽位，不能直接覆盖 Core Persona。
- 单次或短时间窗口内的情绪/反馈不能改变 Core Persona 的身份、核心价值、成熟度、独立性、理性等稳定约束。
- 生成上下文必须能够区分“稳定行为约束”“带置信度的自我假设”和“当前状态”，避免模型将自我认知或情绪误读为永久人格。
- 当 Developing Self 与 Core Persona 看似冲突时，系统应保留冲突证据并拒绝静默覆盖；需要有明确的解释/治理路径。

### R3. Evidence / confidence / provenance

- Developing Self 的每次新增、更新、降级、替代或拒绝都必须保留证据引用、来源类型、来源窗口/时间、置信度和变更审计。
- evidence 必须按不同事实/事件去重，不能通过同一事实的不同引用形式绕过最低证据门槛。
- confidence 与 provenance 必须参与读取和更新决策，而不是只作为展示字段。
- 自动演进规则至少应能表达最小证据窗口、最小置信度、重复 claim no-op、冲突和 revision/CAS；自然语言语义仍由模型产生，边界和持久化正确性由 Core 校验。

### R4. Current State 生命周期

- Current State 必须有时间语义（更新时间、必要时有效期/衰减），并保留状态变更审计；本任务不重新设计现有 wall-time decay。
- Current State 的更新应影响当前认知和回复，但默认不产生长期人格修订。
- 生活事件、Presence、日程、PAD/mood 等实时信息必须继续作为当前生活/状态输入，不得被误写为 Developing Self 或 Core Persona。

### R5. 现有 Persona 表面与运行时边界

- 新 Fluctlight 仍保留现有 Foundation 四字段（`identity`、`personality`、`behavioral_policy`、`life_profile`）的页面展示/编辑形式；它们在新模型中作为 `core_persona` 的同实例投影，不要求旧实例或旧 API 继续兼容。
- Developing Self 只有一个 canonical source：新增 `developing_self_claims` 与 revision ledger；不读取、不迁移、不回放旧 `provenance.self_model` 或旧 evolution 数据。
- `ContextProjection`、chat、wake-up、reflection、media context 和 Detail API 必须使用一致的三层读取结果。
- Foundation、Developing Self/evolution、Current State 的修改/回滚接口和 revision 语义必须分离：Core Persona 复用 Foundation revision；Developing Self 使用 claim 级 rollback/forget；Current State 只读。
- 现有自然语言分析→预览→激活和 blank-slate 入口保留，但 provider/activation contract 改为 `core_persona` + `developing_self.claims`；Current State 由系统初始化为中性快照。
- Persona-only vertical slice 包含现有 Web 创建预览、详情和治理页面的最小增量；现有字段保持当前形式，新分层字段先只读 JSON 展示，后续再做专门 UI 优化。
- Developing Self 作为带 confidence/provenance 的软上下文参与回复；候选写入、结构化行为决策和工具行为由 Core 硬校验，第一阶段不增加逐消息二次 LLM 风格审查。
- 现有 `drive_slots`、`preference_slots`、`trigger_preferences` 保持独立存储但语义上归入 Developing Self；`cognition_claims` 继续作为短期假设，不直接属于 Developing Self。

### R6. 可观测性与治理

- 被接受、被拒绝、因证据不足/冲突/cooldown/版本冲突而跳过的演进候选，都应有可诊断记录。
- 详情页至少能区分三层内容；Developing Self 应展示证据、置信度和来源，Current State 应展示更新时间/有效性，Core Persona 应显示稳定/受治理保护状态。
- 自动人格演进必须支持审计和回滚，且不能破坏已有 Foundation revision 历史。

## Acceptance Criteria

- [ ] 一次普通聊天或 Reflection 即使产生“更甜美/更依赖认可”等观察，也不会直接修改 Core Persona 的稳定字段或行为边界。
- [ ] Developing Self 中的每条有效记录都能查询到 evidence、confidence、provenance、来源窗口和当前状态；重复引用同一事实不能伪造多条独立证据。
- [ ] Current State 可以在多轮对话中快速变化并被下一次 cognition/read context 读取，但不会自动写入 Core Persona。
- [ ] 生成上下文以明确的三层结构（或等价的兼容投影）提供 Core Persona、Developing Self、Current State，并定义优先级：Core Persona 约束 > Developing Self 假设 > Current State 即时表达。
- [ ] Reflection 与 Foundation governance 的并发/版本冲突不会以旧投影静默覆盖新的 Core Persona；冲突可审计、可重试或可拒绝。
- [ ] Developing Self 只有一个运行时权威来源；新实例不再产生 `provenance.self_model` 与独立 `self_model` 的双写分叉。
- [ ] Detail/API/UI 能分别展示三层数据及其审计信息；Current State 读取包含数据库已有的 `momentum`、`regulation`、`conflicts` 字段。
- [ ] 自动演进候选被应用、拒绝或跳过的原因可在诊断/治理面查看，并支持对已应用的 Developing Self 演进进行回滚。
- [ ] 现有聊天、wake-up、media context、memory、life event、schedule 和 Foundation revision 回归测试保持通过，并新增覆盖人格漂移防护的测试。

## Out of Scope for the Initial Slice

- 不在第一阶段重新设计整个生活世界、日程编排、关系模型或媒体生成策略。
- 不进行整个项目或 Core/Worker 架构重构；仅在现有 Persona 相关边界内增量修改。
- 不把所有现有 `personality` 数值维度一次性改造成复杂心理学模型；先解决层级边界、证据链、写入权限和上下文投影。
- 不要求自动推断出的 Developing Self 立即成为用户可编辑的自然语言人格提示词；先保证结构化、可审计和可回滚。

## Notes

- `prd.md` 记录需求、约束和验收标准；复杂任务还需要 `design.md` 与 `implement.md`。
- 规划已审核并通过 `task.py start`；实现和质量检查正在按 `implement.md` 执行。
