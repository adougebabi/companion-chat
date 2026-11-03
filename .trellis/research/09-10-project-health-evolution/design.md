# Project Health Evolution Design

状态：in_progress。本文建立在`09-09-llm-capability-runtime`已独立修复、真实环境复验并由
用户确认的direct Capability Runtime fixed point之上。本文描述后继目标架构，不把尚未
实现的Project Health/Evolution闭环包装成已交付事实。

## 1. 设计结论

系统需要两个边界清晰、通过事实和引用连接的平面：

```text
Action plane
  observation -> cognition -> frozen decision -> Capability -> ActionOutcome

Evolution plane
  evidence window + outcomes -> ReflectionProposal -> policy/CAS -> revisions

Shared authority
  ContextProjection + ContextReferenceIndex + domain repositories
```

Capability Runtime 是 action plane 的唯一执行模型，负责把一个已经形成的行为决定
可靠地准备、冻结、执行、结算和回放。Reflection 不是第二个 MainAgent，也不是另一套
Tool dispatcher；它是异步 evolution coordinator，产生语义演化 proposal，再通过窄领域
service 在一个 Unit of Work 中提交 revision 与 watermark。

两者只通过以下稳定对象连接：

- `ContextSnapshot`：决定时看到的权威状态；
- `ContextReferenceIndex`：Provider-safe ref 与内部实体/revision 的映射；
- `DecisionInfluence`：模型声明哪些 context refs 实际影响了决定；
- `CapabilityInvocation` / `CapabilityResult`：想做什么与执行边界结果；
- `ActionOutcome`：期望、实际结果和 Goal/Intention 关联；
- evidence refs/revisions：Reflection 的唯一事实来源。

### 1.1 任务拆分决定

本次不再创建相互独立的 Memory/Affect/Goal/Reflection child task。它们共同修改
`ContextProjection`、Provider response schema、structured-turn transaction、ActionOutcome、
Reflection watermark和同一组migration/replay边界，拆成并行child会产生双协议与重复迁移。
现有`09-09-llm-capability-runtime`不是本任务内部的普通第一批，而是必须先关闭的独立
前置任务：先修复、验证、复审并报告，再由用户确认该任务已处理完。此门禁前不得运行
本任务的`task.py start`、不得编写后继闭环代码、不得把两个任务的migration合并。
门禁通过后，本任务内部再使用S02-S11可独立验收的纵向批次和回滚点推进。

## 2. 不变量与分层状态

### 2.1 状态层级

| 层 | 含义 | 更新所有者 | 变化速度 | Provider authority |
| --- | --- | --- | --- | --- |
| Identity/Core Persona anchors | 初始化或 Owner 设定的身份、背景、价值与硬约束 | Owner/governance | 极慢/人工 | hard constraint |
| Personality baseline | 初始化人格基线与各 personality profile | Owner/governance | 极慢 | hard baseline |
| Personality/Behavior evolution overlay | Reflection 接受的 trait/表达策略微调 | Reflection + Core policy | 慢 | evidence-backed overlay |
| Developing Self | 偏好、习惯、敏感点、自我认知、能力认识 | Reflection | 中慢 | soft evidence |
| Affect profile | PAD baseline、decay、momentum/regulation 参数与 emotional summary | Reflection + Core reducer policy | 中慢 | evolving state policy |
| Current State | 当前 PAD、mood、momentum、drive pressure/conflicts | cognition/semantic event + Core reducer | 快 | current fact |
| Life Context | Scene、Activity、Location、Presence、Schedule source | Event/Schedule/Capability authority | 快 | current fact |
| Memory | Working/Episodic/Semantic/Relationship/Autobiographical revisions | explicit cognition + Reflection | 不同速度 | evidence-backed fact |
| Goal/Intention | desired outcome 与可执行计划 | cognition/Reflection + autonomy governance | 中等 | motivation/plan |
| ActionOutcome | Capability/Action 的真实结算 | Runtime/workflow | append/revision | observed fact |

`effective_persona` 由 Core 在读取时组合：

```text
immutable Core Persona baseline
  + active personality-profile baseline
  + accepted, non-expired evolution overlays
  + Developing Self soft context
```

overlay 不能删除或重解释 anchor，也不能被序列化回初始化基线。每个 overlay 记录
profile、field path、semantic direction、requested/applied delta、evidence window、policy
version、revision、cooldown 与 rollback link。

### 2.2 数值所有权

- PAD 与 PAD/momentum delta：`-1..1`。
- Mood intensity、Drive pressure、Goal progress、importance、urgency、confidence：`0..1`。
- LLM 只拥有 semantic direction、strength、confidence、reason/evidence。
- Core versioned reducer 计算 requested/applied numeric delta、clamp、decay 和冲突处理。
- Reflection 不能返回并直接写入 raw PAD、raw personality trait delta、raw relationship
  metrics 或 raw Goal progress。

## 3. ContextReference：让状态可被真正引用

### 3.1 内部引用索引

`ContextProjection` 增加内部索引，不把数据库实体 ID 当成模型 mutation command：

```go
type ContextReference struct {
    Ref        string
    Kind       ContextReferenceKind
    EntityID   string
    Revision   int
    Scope      string
    Snapshot   json.RawMessage
}

type ContextReferenceIndex struct {
    ByRef map[string]ContextReference
}
```

`Ref` 是稳定、bounded、opaque token，例如 `goal:g_2f8c...`。Provider 只看到 `ref`、
semantic fields 和必要 revision；冻结的内部 snapshot 保存完整 ref->entity mapping。
apply 阶段只解析当前 frozen index 中的 ref，不能用 description/content 模糊匹配，也不能
接受 Provider 提供的任意数据库 ID。

需要 ref 的对象至少包括：

- Memory/revision；
- Relationship/profile-target state；
- Goal/Intention；
- active Scene/Life Event/Schedule item；
- Current State/Affect profile revision；
- ActionOutcome；
- Developing Self/personality-policy overlay。

### 3.2 Provider 投影

Provider-facing semantic object保留：

```text
ref
type/status
semantic content
relevant bounded numbers
target actor ref
deadline/expiry when behaviorally relevant
revision when CAS awareness is useful
```

删除/隐藏：owner database ID、row primary key、raw provenance、idempotency key、workflow/
provider ID、storage status、embedding metadata。`stripProviderContextMetadata` 不能再无差别
删掉 Goal/Intention/Memory 的可解析 ref。

### 3.3 DecisionInfluence

conversation、WakeUp、daily review 和 native cognition 的 structured response 增加：

```go
type DecisionInfluence struct {
    Ref        string  `json:"ref"`
    Role       string  `json:"role"` // grounds|motivates|constrains|conflicts|satisfies
    Confidence float64 `json:"confidence"`
    Note       string  `json:"note"`
}
```

Core 校验：

- ref 必须存在于本轮 frozen `ContextReferenceIndex`；
- role、confidence、note 大小必须 bounded；
- 同一 ref/role 去重；
- Goal/Intention lifecycle proposal 和 proactive action 必须引用其服务的 Goal/Intention；
- 普通直接回复/no-op 可以没有 influence；
- Core 不从文本或字段差异反推 influences。

validated influences进入 assessment、frozen action、outcome 和 Reflection evidence。由此测试
可以证明“某个状态参与了决定”，而不是只证明它出现在 Prompt 中。

## 4. Action plane 与原子提交

### 4.1 Capability 生命周期

Runtime 使用三个通用阶段，不按具体 Capability 名称分支：

```text
validate Provider Arguments
  -> resolve declared ContextSlots
  -> Prepare outside transaction (only when needed)
  -> persist/freeze canonical invocation + prepared payload
  -> ApplyTx / bind durable target inside caller transaction
  -> persist CapabilityResult + ActionOutcome
  -> commit
  -> visible callback / asynchronous worker
```

Provider Arguments 与 Runtime-owned PreparedPayload 是两个物理字段：

```go
type CapabilityInvocation struct {
    Arguments       json.RawMessage // immutable Provider-owned input
    PreparedPayload json.RawMessage // Runtime/Capability-owned, schema-versioned
    ContextSnapshot map[string]any
    // existing identity/provenance fields
}
```

`Arguments` 严格执行 Definition 的 `additionalProperties`、type、enum、required、anyOf、
bounds 与 size。Provider 即使伪造 `prepared_concept`、schedule replacement、revision 或
idempotency 字段，也只能得到 `invalid_arguments`。

### 4.2 Transaction-aware apply

Capability 保持 direct registration，并按真实差异实现可选 seam：

```go
type CapabilityPreparer interface {
    Prepare(context.Context, CapabilityInvocation, CapabilityContext) (PreparedPayload, error)
}

type TransactionalCapability interface {
    ApplyTx(context.Context, pgx.Tx, CapabilityInvocation,
        CapabilityContext, *OutputBindingV1) (CapabilityResult, error)
}
```

- planner/provider/storage read 等 I/O 在 `Prepare`；
- native state mutation、Memory、Affect、Scene、Schedule accept、durable media intent、
  output binding 与 result/outcome persistence 在 caller-owned transaction；
- read-only query 可在 freeze 前执行，但其 result也在 commit 中持久化；
- asynchronous renderer/Provider job在 durable intent commit 后由 Worker 执行；
- Runtime 根据接口/Definition metadata 选择通用阶段，绝不 switch concrete name。

对于 interactive turn，appraisal transition、personality decision、transactional Capability、
assistant/claims、frozen action/result/outcome 在同一事务提交。可见 callback 仅在 commit 后。
对于 external async Capability，`durable intent created` 是首阶段成功；final asset/result 以
后续 Outcome revision 和 inbox fact 回流，不把两者混为同一成功状态。

### 4.3 ActionOutcome authority

扩展现有 `cognition_action_results` 或建立等价的单一 authority（实现阶段以 migration
兼容性决定），规范化为：

```go
type ActionOutcome struct {
    OutcomeID       string
    ActionID        string
    CallID          string
    CapabilityName  string
    Status          string // pending|completed|failed|cancelled|suppressed|unknown
    SuccessBoundary string
    Expected        map[string]any
    Observed        map[string]any
    ErrorCode       string
    GoalRefs        []string
    IntentionRefs   []string
    EvidenceRefs    []string
    Revision        int
    OccurredAt      time.Time
}
```

唯一 `(action_id, call_id)` 身份防止重试重复。async completion 更新同一 outcome revision，
并追加一个按 Fluctlight sequence 排序的 inbox fact。最近相关 outcomes 进入新
`SlotRecentOutcomes` 和 `ContextProjection.RecentOutcomes`；Reflection 读取窗口内完整事实。

Provider-facing Recent Outcomes 不复用上述完整 authority DTO。Core 在
`compactCognitionContext`边界创建独立 allowlist projection，只允许：opaque refs、Capability、
status、success boundary、occurred-at、bounded error code、Goal/Intention refs，以及少量
枚举/数值型 observed signals（首批仅`delivery_status`、`target_kind`等明确白名单）。以下字段
即使存在于内部 outcome 也必须在 Provider 序列化前物理删除：原始 user/assistant 文本、raw
expected/observed、内部 ContextReferenceIndex、entity/database ID、完整 evidence/provenance、
Provider/workflow/idempotency 标识。测试直接检查最终 Provider request，不以日志脱敏替代。

本任务只在现有 sectioned dynamic prompt 中增加最小 Recent Outcomes section 和一句 authority
规则，不重新组织完整系统 Prompt；整体提示词由后续独立设计统一处理。

## 5. Memory 闭环

### 5.1 写入与演化

普通 cognition 保留 `memory_event`，其 InputSchema 只包含模型真正决定的 content、type、
confidence、importance、optional emotional significance。profile、visibility、actor scope、
evidence binding、provenance、idempotency 与 embedding 均由 Context/Runtime 提供。

Reflection 的 `MemoryCandidateV2`：

```text
operation: create|confirm|revise|merge|supersede|deprecate
target_ref / merge_refs (when required)
type/content/confidence/importance/emotional_significance
evidence_refs
semantic_reason
```

Core 把 refs 解析成 current revisions，执行 owner/scope/evidence/CAS 校验，写 Memory revision、
supersede link、embedding rebuild intent、outbox 和 evolution result。模型不能提供 revision
primary key、profile ID、provenance 或 idempotency key。

### 5.2 Retrieval plan

硬授权/visibility/conversation/Actor 过滤始终先于 ranking。查询材料按 surface 构造：

- conversation：当前消息、当前话题、未解决问题；
- WakeUp/daily review：active scene、Goals/Intentions、recent Outcomes、unresolved items；
- Reflection：本窗口 facts、candidate target refs 和当前 active Memory；
- Capability planner：其 RequiredContext 与 intent。

拼接已存在 semantic fields 是 query construction，不是 Go semantic inference。FTS/lexical、
embedding 和 salience/recency 分数有明确权重、budget 和诊断。空 semantic query 被标记为
`salience_recent`，不能宣称 semantic match。

## 6. Life Context / Scene 闭环

Life Context projection增加 event/schedule/presence ref 与各自 revision/source：

```text
source=event|schedule|pending
scene/activity/location
event_ref or schedule_item_ref
presence_ref
context_revision
effective_at/expires_at
```

解析优先级保持 `active confirmed Event > active inferred Event > accepted Schedule item >
pending`。Scene 不直接产生行动；模型通过 `DecisionInfluence(role=constrains|grounds)` 表明
它影响了当前 decision。Image/Schedule planner、reply、WakeUp 和 autonomy 均使用冻结版本。
若 apply 前 authoritative revision 改变，按照 Definition policy re-assess/defer/fail，不用
live value静默替换 snapshot。

### 6.1 Conversation delivery 与 clean-start authority

direct conversation的user row先在短事务提交，再进入Provider cognition；stream必须立即发送该
authoritative row，不能只依赖浏览器内存中的optimistic bubble。assistant row与structured-turn
effects在同一事务提交后，stream发送authoritative assistant row和terminal completed。Browser将
stream事实与history结果按message id/sequence单调合并，旧history快照不能回退已经观察到的消息。

普通conversation没有“成功但不回复”的产品状态。Main cognition缺少visible text或
`conversation.reply`时返回显式可重试失败，不提交completed frozen action，也不合成默认回复或
发起第二次Main LLM调用。

用户已选择clean-start而非旧数据repair。0030对非空旧业务数据库fail closed，部署方显式重建；
新authority表从第一行开始要求稳定idempotency、canonical digest和stored result。Event、Presence、
Schedule允许同一事务内暂时写入占位result，但`DEFERRABLE INITIALLY DEFERRED` constraint trigger在
COMMIT检查最终row可完整replay；检查失败时整笔mutation/fact/outbox/outcome回滚。

## 7. Affect / Drive 闭环

### 7.1 即时状态

统一 appraisal 与 `affect_event` 到一个 versioned reducer API：

```go
ReduceAffect(current, profile, semanticSignals, elapsed) -> StateTransition
```

它负责：

- lazy decay toward Affect baseline；
- PAD `-1..1` 与 intensity/pressure `0..1` clamp；
- momentum、mood label/source/intensity、regulation；
- built-in Drives 与 typed Drive slots 的 pressure/conflict transition；
- requested/applied delta、policy version、evidence 和 resulting revision。

Provider semantic appraisal 不直接携带 raw delta。`state_expression` 只描述显示方式，不参与
reducer。一个 source fact只产生一个 state revision；appraisal 和同轮 `affect_event` 不能
各自重复解释同一语义事件，Runtime 必须按 source/call identity去重或明确组合。

### 7.2 Reflection emotion profile

Reflection 返回：

```text
emotional_summary
  dominant_patterns[]
  triggers[]
  recovery_patterns[]
  conflicts[]
  evidence_refs[]

affect_recalibration_candidates[]
  target: baseline|decay|regulation|drive
  direction
  strength
  confidence
  evidence_refs[]
```

Core policy只在跨事件阈值、cooldown 与 confidence 达标时更新 revisioned Affect profile。
它不回写过去，也不突然重置当前 mood；下一次 lazy decay/appraisal 读取新 profile，使变化
真实影响未来情绪和决策。

## 8. Goal / Intention / Action 闭环

### 8.1 Goal

Goal 不再只是 description + 数值。保留现有字段并增加/明确：

```text
ref
desired_outcome
success_criteria[]
motivation
source/scope/target_actor_ref
importance/urgency/progress/deadline
status/revision/evidence_refs
```

LLM 可以提出 semantic progress assessment，但 Core 只接受绑定真实 Outcome/evidence 的
变更。Goal lifecycle 完整支持 candidate/active/paused/completed/abandoned/cancelled，并有
append-only governance/revision。

### 8.2 Intention

Intention 是 Goal 到行动的桥：

```text
ref / goal_ref
action_intent
expected_outcome
capability_constraints (optional installed capability class/name)
typed_trigger
preferred_time / expiration
confidence/status/revision/evidence_refs
```

typed trigger仅允许：

- `time`：Temporal durable timer到期；
- `event`：权威 event/inbox type/ref；
- `semantic`：一个新事实到来时重新交给 LLM判断，绝不是 keyword matcher。

trigger 到期只创建 `agency.intention_due` cognition fact。该事实与当前 projection 一起进入
一次 cognition，模型决定 no-op/defer 或产生 Capability Invocation，并用 influence 引用
Intention/Goal。Core执行权限、预算、quiet hours、revision、expiration和Capability preflight，
再冻结动作。

### 8.3 Outcome settlement

- frozen action保存 Goal/Intention refs；
- completed/suppressed/failed/cancelled outcome先确定 Intention 的机械 lifecycle 状态；
- `duplicate_suppressed` 不等价于目标达成；
- Goal progress/complete 由 Reflection 或专门的语义 outcome assessment提出，并要求引用
  Outcome + Goal success criteria；
- Core policy计算 bounded progress delta并写 revision；
- 重试复用同一 Action/Outcome/Intention attempt identity。

## 9. Reflection V2

### 9.1 输入与输出

输入：

```text
EvidenceWindow {
  processed facts + appraisals
  ActionOutcomes
  current Memory/Relationship/Goal/Intention refs
  current Affect/DevelopingSelf/effective-persona revisions
  reference index
  from_sequence/to_sequence/base revisions
}
```

输出 `ReflectionProposalV2`：

```text
summary
memory_candidates
relationship_observations
goal_candidates
intention_candidates
emotional_summary
affect_recalibration_candidates
drive/preference/trigger candidates
developing_self_candidates
personality_evolution_candidates
behavior_policy_evolution_candidates
```

Personality/Behavior candidates使用 field path + semantic direction/strength/confidence/evidence，
不能提供最终 baseline value。Identity/Core Persona anchor paths不在 schema 中。

### 9.2 Prepare 与 commit

```text
claim evidence window
  -> build frozen EvolutionContext/reference index
  -> Provider call outside tx
  -> strict schema + all-ref validation
  -> compile domain EvolutionPlan with expected revisions
  -> begin tx and recheck window/state/entity revisions
  -> apply valid commands and record explicit policy dispositions
  -> persist proposal/result/revisions/watermark
  -> commit
```

错误语义：

- malformed schema、unknown/foreign evidence/ref：整个 proposal invalid，watermark 不推进；
- state/entity/window CAS stale：整个 apply rollback，重新计划或显式 deferred；
- schema-valid 但证据阈值/cooldown/policy未满足：该 candidate记录 `deferred` 或 `rejected`，
  其他 candidate可以同事务提交；
- domain I/O/apply failure：整个事务 rollback；
- 空 proposal：提交 `no_change` disposition并推进合法窗口。

Reflection outcome返回每类 applied/rejected/deferred/no_change 计数、changed refs、新 revision、
reason codes、window与proposal ID。不能只返回 `status=applied`。

## 10. Personality / Behavioral Policy 慢演化

新 overlay authority按 profile 和字段保存：

```text
field_path
semantic_direction
requested_delta/applied_delta or categorical replacement
before/after effective value
confidence/evidence_refs/source_window
policy_version/status/revision/superseded_by/cooldown_until
```

允许字段是 Personality trait 与 Behavioral Policy 表达字段；Identity、Owner、安全、权限、
provider、infrastructure、persona profile identity、hard character constraints 禁止自动修改。
数值字段使用 baseline update policy；categorical text更新需要多窗口一致证据和更高阈值。
rollback追加反向 revision，不删除历史。Provider context明确标注 baseline/overlay/effective，
但普通 cognition只接收 bounded effective view与必要 provenance summary。

## 11. Persistence 与 migration

迁移严格分成两个顺序revision集合。前置`09-09`任务先创建并验证Capability v2 cutover
revision，不能继续复用旧`0025` head；该任务完成后，本任务才创建更高的Project Health/
Evolution revision。两个任务的ledger状态、active预检和rollback证据彼此独立。

前置Capability migration覆盖：

- active Capability v1->v2 canonical envelope/result conversion；
- `PreparedPayload` 与 Arguments 分离；

本任务的线性migration已覆盖：

- ContextReference/DecisionInfluence frozen payload；
- ActionOutcome authority和唯一约束；
- Goal success criteria、完整 lifecycle/revision/governance；
- Intention typed trigger/expected outcome/attempt identity；
- Affect profile/emotional summary与 PAD reconciliation；
- Personality/Behavior evolution overlays/revisions；
- Reflection proposal/result V2与逐类 disposition。

`0031_evolution_authority`只在fresh schema或无业务authority的0030 head上增加上述状态。
任何既存业务row直接阻止cutover并要求显式重建；0031没有active-valid、active-malformed或
completed-history转换分支。Temporal active history仍需独立saved-history replay或部署前
drain/build-version gate；Runtime不保留永久compat codec。

## 12. 测试策略

### 12.1 确定性闭环 fixtures

Fake Provider 根据输入 Context refs返回预定义语义结果，用于证明数据流，而不是模拟模型
智能。每条 e2e都检查：source -> provider context -> influence -> freeze -> mutation/outcome ->
reflection -> next projection。

关键场景：

1. 同一消息在不同 Scene snapshot下产生不同 influence/Capability plan；
2. 一条 Memory被检索、引用、在 Reflection中修订，并在下一轮以新 revision出现；
3. 负向 appraisal让 PAD低于零，经过 elapsed-time decay后仍保留 momentum；
4. Goal创建 Intention，time trigger产生 fact，Capability成功后 Intention结算，Reflection
   根据 Outcome推进 Goal；
5. Capability失败/assistant transaction失败不留下 Memory/Affect/Goal partial commit；
6. Reflection stale/foreign ref不推进 watermark；policy deferred有明确 disposition；
7. Personality overlay满足多窗口策略后改变 effective policy，rollback后恢复前一版本；
8. async media只在 final result fact到达后产生 final Outcome revision。

### 12.2 架构与环境门禁

- Definition/Input/Output/ref schema consistency与 `additionalProperties` 全语义验证；
- no concrete capability-name branch、no legacy adapter、no semantic keyword inference；
- transaction/replay/crash-window/idempotency tests；
- real PostgreSQL migration/rollback/replay/CAS；
- Redis expiry duplicate/reconnect与PG fallback；
- Temporal intention timer/history/version/drain；
- opt-in live Provider只验证 contract compatibility，不作为确定性行为唯一证据；
- Core/Gateway test/race/vet/build、gofmt、diff-check。
