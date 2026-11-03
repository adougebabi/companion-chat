# Persona 分层与防漂移技术设计

## 1. 设计目标与边界

本任务只修改现有 Go Core、Go BFF 和 Vue Persona 页面中与 Persona 直接相关的契约，不重构项目基础设施、Worker、Temporal、Life World、Memory、Relationship 或前端导航。

新实例采用全新的 Persona 分层协议。旧实例、旧 `personality` 自动演进、旧 `provenance.self_model` 和旧 evolution 数据不做迁移、回放或兼容适配；验收从迁移后新建的 Fluctlight 开始。

保留现有“描述分析 → 预览 → 激活”和“白纸创建”交互，只替换分析结果、激活 payload、运行时投影和 Reflection candidate 的语义。

核心不变量：

```text
Core Persona      = hard constraint / owner-governed
Developing Self   = evidence-backed soft context / reflection-governed
Current State     = transient runtime snapshot / cognition-governed
```

优先级固定为：

```text
Core Persona > Developing Self > Current State
```

## 2. 领域模型

### 2.1 Core Persona

在 `fluctlights` 聚合中增加 canonical `core_persona jsonb`，结构保持现有页面字段的形状：

```json
{
  "schema_version": 1,
  "identity": {},
  "personality": {},
  "behavioral_policy": {},
  "life_profile": {}
}
```

其中：

- `identity` 的身份锚点、核心价值、世界观属于 Core Persona；
- `personality` 的稳定基线属于 Core Persona；
- `behavioral_policy` 的长期表达和边界属于 Core Persona；
- `life_profile.character_constraints` 属于 Core Persona；
- `life_profile.preferences`、`life_habits` 等不是 Core Persona 的自动写入目标。

现有 `identity`、`personality`、`behavioral_policy`、`life_profile` 列保留为新实例的兼容投影，以免现有页面和内部读取点全部重写。新实例创建和 Foundation governance 在一个事务中写入 canonical `core_persona` 与这些投影；Reflection 不得更新这些列。

Core Persona 的人工变更继续复用 `fluctlight_foundation_revisions` 与现有 Foundation route。Revision 记录需包含 canonical Core Persona 快照或等价的完整 before/after，避免只记录无法重放的局部旧字段。

### 2.2 Developing Self

增加 `fluctlight_developing_self_claims`，一条 claim 一行：

```text
id
fluctlight_id
category
claim
value jsonb
confidence numeric
evidence_refs jsonb
provenance jsonb
status
expires_at nullable
revision
superseded_by nullable
created_at
updated_at
```

首版允许的 category 为有限集合：

```text
preference | habit | sensitivity | emotion_pattern |
self_perception | capability | interest
```

明确禁止 `identity`、`core_value`、`behavioral_policy` 等把 Core Persona 当作自动目标的类别。

增加 `fluctlight_developing_self_revisions`，用于记录 accepted、rejected、conflict、rollback、forgotten 等结果。为支持被拒绝且尚未有 claim 的候选，`claim_id` 可为空；记录至少包含 candidate、before/after、evidence_refs、confidence、provenance、source_window、reason_code、status、created_at。

新 claim 的最小要求：

- `confidence` 在 `0..1`；
- evidence 至少引用两个不同的事实/sequence，或一个明确的 `owner_defined` 初始化来源；
- evidence 必须属于当前 Reflection window 且归属同一 Fluctlight；
- 同一事实的 fact ID 与 `sequence:N` 形式必须解析为同一证据，不能重复计数；
- 重复 claim 没有新证据时是 deterministic no-op；
- 每次变更都递增 claim revision 并写 revision ledger；
- Core 冲突写入 `status=conflict`，不创建/修改 active claim。

现有 `drive_slots`、`preference_slots`、`trigger_preferences` 保持原表和运行逻辑，在上下文语义上归入 Developing Self；如能关联来源，使用其现有 provenance 或新增 source metadata 记录 `source_claim_id`/`source_revision`。不在本任务中合并这些表。

现有 `cognition_claims` 继续是短期假设，不直接属于 Developing Self；只有 Reflection 在证据窗口内提炼后，才创建 Developing Self claim。

### 2.3 Current State

继续使用 `fluctlight_inner_states` 作为 Current State 的权威快照。`inner_state` 必须完整读取：

```text
pad
mood
momentum
regulation
drives
conflicts
revision
last_updated_at
```

`current_state` 是 ContextProjection/Detail 的语义包装，组合当前 inner state 与 resolved context/Presence；不新增第二份状态事实，也不允许 JSON 页面覆盖。

本任务不重新设计 wall-time decay 或 Life World 状态机，但修复 Detail/ContextProjection 遗漏 `momentum`、`regulation`、`conflicts` 的契约缺口。

## 3. 初始化数据流

### 3.1 LLM-defined

保留现有分析和激活 endpoint，但新的初始化 provider schema 要求：

```json
{
  "core_persona": {
    "identity": {},
    "personality": {},
    "behavioral_policy": {},
    "life_profile": {}
  },
  "developing_self": {
    "claims": []
  },
  "initial_goals": [],
  "initial_intentions": []
}
```

初始化提示词明确：稳定身份/价值/边界进入 `core_persona`；偏好、习惯、情绪反应和自我解释进入 `developing_self.claims`；Current State 不由模型生成。

前端仍然展示 JSON 预览并等待用户激活。激活时 Core 对象、Developing Self seeds 和初始目标在同一个创建事务写入；每个 owner-defined seed 的 provenance 指向本次初始化 request/description，而不是伪造对话事实。

### 3.2 Blank slate

保留白纸创建：

- `core_persona` 使用最小安全默认值并包含实例名称；
- `developing_self.claims` 为空；
- `current_state` 使用现有中性默认值；
- 不接受非空的 LLM foundation 或 arbitrary personality input。

## 4. Reflection 数据流

Reflection provider schema 改为要求：

```text
memory_candidates
relationship_candidates
developing_self_candidates
drive_candidates
preference_candidates
trigger_candidates
```

不再生成或接受 `personality_candidates`、`self_model_candidates`。Reflection prompt 明确：

- 不能修改 Core Persona；
- 不能把单次情绪/反馈当作稳定人格；
- Developing Self candidate 必须说明 claim、category、confidence、evidence_refs、provenance；
- Core 冲突必须返回 bounded conflict candidate，由 Core 记录拒绝。

`ProcessReflection` 在同一个短事务中完成：

1. claim/lease 当前 evidence watermark；
2. 验证 candidate schema、category、evidence ownership、distinct evidence 和 confidence；
3. 应用 Developing Self claim 或写 rejected/conflict revision；
4. 应用其他已存在的 memory/relationship/typed slot candidate；
5. 推进 watermark。

Reflection 不再调用任何更新 `fluctlights.personality` 或 `provenance.self_model` 的函数。旧的 `applyPersonalityCandidateTx` / `applySelfModelCandidateTx` 路径从新 Reflection 流程移除或变成不可达的显式错误。

## 5. ContextProjection 与生成

`ContextProjection` 增加 canonical 字段：

```go
CorePersona    map[string]any
DevelopingSelf []map[string]any
CurrentState   map[string]any
```

保留 `Identity`、`Personality`、`BehavioralPolicy` 等字段作为从 Core Persona 派生的内部兼容视图，避免无关 cognition/life-world 代码同时重写。

投影内容带有 authority/metadata：

```json
{
  "core_persona": {"authority": "hard_constraint", "data": {}},
  "developing_self": {"authority": "soft_context", "claims": []},
  "current_state": {"authority": "transient_state", "data": {}}
}
```

聊天 assessment、wake-up、daily review、media context、Reflection 和 realization 复用同一分层投影。Prompt 指示 Core Persona 是约束，不得被 Developing Self 或 Current State 覆盖。

`response_plan` 增加可选的 `core_alignment` 与 `state_expression` 摘要；它们只用于结构化审计，不授予模型写入语义状态的权限。

冻结 action 时保存完整分层 projection。realization 只读取冻结的 `response_plan`、冻结 projection 和 tool results，不在重试时重新读取可能已变化的 Persona。这样 Reflection 在 assessment 和 realization 之间运行，也不会混用两个版本的人格。

## 6. API / BFF / Web

### Core/BFF

- 初始化分析/激活 endpoint 路径保持不变，body 改为新 contract；不实现旧实例 migration。
- Detail 增加 `core_persona`、`developing_self`、`current_state` 和 bounded revision/conflict 数据。
- Foundation revision route 继续作为 Core Persona 的 owner governance；应用层把现有字段 patch 映射到 canonical `core_persona`。
- 新增 Developing Self 查询、claim rollback、claim forget 的 Core/BFF route，并同步浏览器 client/OpenAPI artifact。
- 所有 BFF 只做校验、映射、脱敏和错误转换，不实现 Persona 语义。

### Web

- `InstancesView` 继续使用现有创建模式；预览读取 `core_persona` 与 `developing_self`，白纸按钮保留。
- `InstanceDetailsDialog` 保留现有 identity/personality/policy/life_profile 字段展示，新增三个只读 JSON 区块：Core Persona、Developing Self、Current State。
- `GovernanceView` 保留既有 Foundation 表单/治理；新增 Developing Self revision/rollback/forget 的最小操作和 JSON 审计展示。
- 新字段全部通过 Vue 文本绑定渲染；不使用 `v-html`；详情 dialog 保持现有单一滚动区域和移动端 bottom-sheet 约束。
- Pinia 只保存服务端 snapshot；mutation 成功后重新加载 detail，不在前端推断 confidence、evidence 或层级。

## 7. 并发、审计与回滚

- Core Persona revision 使用 Foundation 当前 revision CAS；Reflection 不拥有 Core Persona 写权限。
- Developing Self claim 使用独立 claim revision CAS，并以 evidence watermark + inner-state/base context revision 保护 Reflection stale work。
- 任何 accepted/rejected/conflict/rollback/forget 都写 revision ledger；拒绝原因使用稳定 reason code，不解析错误文本。
- 初始 owner-defined seed 的证据来源为初始化 request，不回放旧聊天。
- 旧实例不迁移；新建实例是唯一验收路径。部署回滚只回滚代码，不执行破坏性 down migration；新增表/列保持未使用即可。

## 8. 关键风险与缓解

| 风险 | 缓解 |
| --- | --- |
| 旧 `personality` 路径仍被 Reflection 调用 | 删除/禁用旧 candidate schema 和 apply seam；增加架构/回归测试确保 personality 未变 |
| Core Persona 与旧四字段双写分叉 | canonical Core Persona + 同事务投影更新；所有 Foundation 写入集中到一个 mapper |
| 同一证据以不同 ID 形式重复计数 | 在 claim applier 前解析到 canonical fact/sequence 并去重 |
| Reflection 与 Foundation 并发 | Foundation revision CAS + Reflection 读取时的 base revision 检查 |
| realization 读取新旧人格混用 | frozen action 使用 assessment 时的完整 projection |
| 新 JSON 展示出现 `[object Object]` | 统一 bounded `JSON.stringify` display helper，并对 null/array/object 做显式处理 |
| 过度扩大任务范围 | 不修改 Life World、Memory、Relationship、Worker/Temporal 和整体 UI 架构 |

## 9. 验收证据

至少覆盖：新建 LLM-defined/blank-slate、Reflection claim 新增/更新/拒绝/conflict、Core Persona 不变、证据去重、ContextProjection 三层优先级、frozen realization、Detail/BFF/Web JSON 展示、claim rollback/forget，以及现有 chat/wake-up/media/状态读取回归。
