# Fluctlight Intelligence P1: context, self-evaluation, memory and evolution

## Goal

在 Go-only Fluctlight Runtime 上一次性补齐 P1 智能内核：让 Fluctlight
能够围绕当前上下文做出有依据的表达，自主判断哪些内容应该说、哪些内容
只是临时假设，并从经过证据和治理的经历中形成可回滚的长期变化。P1 必须
形成一条完整、可重放、可停止的闭环，而不是再增加一层近似相同的 prompt。

```text
CognitionFact
  → ContextProjection
  → expression/semantic assessment
  → self-evaluation gate
  → ResponsePlan / native capability candidates
  → deterministic validation + revision gate
  → FrozenAction
  → optional capability effect
  → realization/projection
  → evidence window
  → reflection candidate
  → governed Memory/Relationship/Self-model revision
```

本任务在独立工作树 `codex/fluctlight-runtime-phase1` 中完成，基于已合并
Go Worker closure 的 `master` 和第一阶段 `tool_call`/NDJSON seam。用户明确
要求本 session 完成 P1 闭环，避免在多次重构/上下文切换中遗漏语义接线。

## Confirmed facts and decisions

- 有效运行时是 `apps/core-go` + `apps/gateway-go`；Temporal、Redis Streams、
  PostgreSQL outbox/inbox 和 Go `CapabilityRegistry` 已存在。
- 交互 turn 保持 `POST` + `application/x-ndjson`。主动联系由 Runtime 产生
  durable message/event；SSE 仅是未来独立的 server-push projection，不是
  P1 turn 协议。
- `tool_call` 是模型到 Runtime 的结构化控制面。外部能力和 Fluctlight
  native capability 都可以占用 slot，但插件不能拥有第二套事实源、revision
  或 Agent Loop。
- 不引入 Claude Agent SDK、人工审批/HITL、多个 Fluctlight handoff 或 P2
  Harness；没有人工审批并不取消 Runtime 的 schema、evidence、owner、
  revision、budget、timeout、idempotency 和 cancellation 约束。
- 未经确认的 Fluctlight 自述默认是可衰减的 Hypothesis，不得直接成为长期
  Memory、Personality 或 Life World fact。Fluctlight 必须在表达前自行判断
  是否应该说，而不是等待用户纠正。
- 场景记录只是 native capability slot 的一个例子；P1 需要预留并接通
  `scene_event`/`presence_event`，但视觉身份/头像/图生图暂不属于 P1。

## Requirements

### R1. Unified capability slots

建立统一的 capability catalog/registry，区分：

1. Fluctlight-native slots：`scene_event`、`presence_event`、
   `memory_event`、`relationship_signal`、`self_evaluation`、
   `reflection_candidate` 等；
2. External slots：LLM structured/streaming、embedding、image/video/audio、
   search、notification 等；
3. Runtime infrastructure：mailbox、Context Projection、transaction/UoW、
   revision/CAS、outbox/inbox/effect ledger、Temporal、diagnostics。

所有 slot 使用版本化 manifest、typed input/output、side-effect/concurrency
分类、幂等和错误合同。slot 实现可以替换，但 canonical authority、revision
ledger、evidence、Context resolver 和 cognition ordering 只能有一个 owner。

### R2. Context Projection

实现唯一的 Fluctlight context projection，供 cognition、realization、
Memory retrieval、Reflection、scene plugin 和未来 autonomy 共用，至少包含：

- 当前用户意图和 source fact；
- Identity/Personality/behavioral policy projection；
- inner-state/affect/drive；
- resolved Event > Schedule > pending context；
- Presence overlay；
- authorized relevant Memory；
- Relationship projection；
- active Hypothesis（带 confidence/expiry/evidence）；
- available capability manifests。

每个注入项保留 source、revision、confidence 和 evidence refs；不得用完整
conversation history 或多个模块各自拼接的 current scene 代替 projection。

### R3. Adaptive cognition and ResponsePlan

普通无副作用聊天走单次 cognition provider call，输出 visible draft、
structured `ResponsePlan`、self-evaluation 和可选 tool/native candidates。
仅当出现外部 effect、scene/memory/relationship/self-model candidate 或高
不确定性时，才进入有限的 `assessment → freeze → effect → realization`
路径。不得把近似相同的原始 prompt 再交给第二个模型重新决策。

`ResponsePlan` 至少拥有：

```text
schema_version, source_fact_id, answer_mode,
approved_claims, uncertain_claims, omitted_claims,
response_outline, tone, tool_calls, native_candidates,
self_evaluation, context_revision
```

realization 只能把已冻结 plan 编译为自然语言，不得新增事实、场景、Memory、
Personality、关系或 capability effect。最多允许一次有界重写；之后必须
显式 `accepted`、`uncertain`、`omitted` 或 `deferred`。

### R4. Self-evaluation and anti-loop

每个拟表达 claim 必须标记为 `confirmed_fact`、`observed_fact`、
`supported_hypothesis`、`uncertain_hypothesis` 或 `unsupported_self_claim`。
Self-evaluation 检查相关性、evidence ownership、当前情境一致性、人格/状态
一致性、重复强化和表达必要性。无证据内容默认省略或降级为不确定表达，不
依赖用户先纠正。

实现有界语义重复保护：相同 normalized claim/topic/action 在没有新 evidence
时不提升 confidence、不再次写 Memory/Scene、不持续注入 Context。被判断为
错误的内容保留 `superseded/rejected/expired` provenance，后续 projection
不得把它当作事实复活。

### R5. Native scene/presence slots

接通 `scene_event` 和 `presence_event` capability：

- plugin/model 只产生带 `source_fact_id`、`evidence_refs`、confidence、
  temporal bounds、idempotency key 的 candidate；
- Life World authority 决定 confirmed Event、decaying Hypothesis、Presence
  overlay 或 no-op；
- Presence 只覆盖 `user_presence/current_task`，不得伪造 scene/activity/
  location/Event；
- accepted scene changes 进入 Fluctlight ordered cognition stream；
- 相同 candidate 在没有新 evidence 时幂等 no-op；
- Scene、Schedule、Memory、Relationship 和 Self-evaluation 共享同一
  Context Projection。

### R6. Memory authority and retrieval

完成 Fluctlight-owned Memory 的：

```text
record → revise → forget → retrieve → embed/reindex
```

Memory record 必须保留 typed type、content、actor/conversation/event/evidence
refs、visibility、confidence、importance、revision 和 provenance。新增 Memory
必须同一 authority transaction 写 initial revision，并通过 outbox 请求
embedding；embedding 失败不能丢失 Memory。

`retrieve(MemoryQuery)` 必须先做 owner/visibility/Actor/Conversation hard
filter，再做 FTS/vector/hybrid rank、bounded rerank、recency/importance/
confidence 加权和 token budget，输出带来源的 `MemoryContext`。普通 cognition
必须把授权相关 Memory 注入 Context Projection；不得把全部 Memory 或未经
授权的候选送入 provider。

### R7. Reflection and self-evolution

Reflection 必须由真实 cognition completion/周期事实稳定触发，按 Fluctlight
有序 evidence window claim watermark，调用 reflection capability/plugin 产出
typed candidates，并校验证据属于当前 window 和 owner。

候选至少支持：

- Memory consolidation；
- Relationship trend；
- recurring preference/behavior tendency；
- Self-model claim；
- Personality tendency（慢速）；
- Goal/Intention 长期变化。

candidate 不能直接写 canonical projection。必须经 evidence、consistency、
revision/CAS 和 governance policy 后应用；每次变化保留旧 revision、source
window、provider/prompt/schema version、evidence refs 和 rollback 路径。

### R8. Evolution speeds and correction

明确三种变化速度：

- 快速：affect、attention、临时意图、短期 hypothesis；
- 中速：Memory、Relationship、recurring preference、goal priority；
- 慢速：Personality、Identity biography、Self-model、长期 behavioral policy。

一次表达不能直接改变慢速状态。用户未纠正之前，系统自身也必须通过
self-evaluation、evidence window、重复/冲突检查阻止奇怪回答成为新设定。

### R9. Durable outcomes and replay

统一 fact/action/capability/reflection 状态和关联 ID：

```text
proposed, validated, rejected, frozen, realizing,
completed, failed_retryable, failed_terminal,
result_unknown, cancelled, deferred, stuck
```

重试必须复用 fact/action/provider request；外部结果未知时进入
`result_unknown` 并对账，不得盲目二次提交。Provider、tool、scene、Memory、
Reflection 的结果都必须可按 correlation/causation/fact/action/workflow ID
重建。

## Acceptance Criteria

- [ ] 普通聊天默认走单次 cognition fast path；需要 effect/native candidate
  才进入有限两阶段路径，realization 不重新决定语义。
- [ ] `ResponsePlanV1`、`SelfEvaluationV1` 和 claim 分类具有严格 schema、
  source/evidence/repetition 字段；无证据自述不会写入长期 Memory/Personality。
- [ ] Context Projection 是 Chat、Memory、Reflection、Scene、Media 的唯一
  current-context 来源，包含 source/revision/confidence/expiry。
- [ ] `scene_event`/`presence_event` 作为可替换 slot 接入 ordered cognition，
  有 authority、时间边界、幂等和重复 no-op 测试。
- [ ] Memory record/revise/forget/retrieve/embed 闭环可运行：授权过滤先于
  ranking，Memory 进入 prompt，embedding/provider 失败不丢事实，revision 可
  回滚。
- [ ] Reflection 有真实 producer、window claim/CAS、evidence ownership、
  candidate ledger 和至少 Memory/Relationship/Self-model 一条 apply path。
- [ ] 自进化按快/中/慢速策略工作，旧 revision 可审计和回滚，未来 cognition
  能消费新 projection。
- [ ] 重复 claim/topic 在无新 evidence 时不会持续升权、重复写入或触发场景/
  记忆循环；被拒绝/过期候选不会被 projection 复活。
- [ ] Provider/native sidecar/tool call、tool result、scene/memory/reflection
  outcome 和 action lifecycle 均有幂等、重放、取消、失败分类测试。
- [ ] Core/BFF 继续使用 POST NDJSON，真实 token chunk 增量、abort、唯一
  terminal frame 和 hidden payload redaction 通过；不引入 turn SSE。
- [ ] `go test`、`go vet`、`go build`、race 和可用的前端生成/类型检查通过；
  受环境阻断的检查必须明确记录，不得伪报通过。

## Out of scope

- 视觉身份/头像/图生图/AppearanceProfile；只在后续 capability backlog
  预留，不纳入本 P1 的实现和验收。
- 人工审批/HITL、Claude Agent SDK、多个 Fluctlight handoff、P2 Harness、
  独立 SSE server-push subscription。
- 用 LangGraph/AutoGen/Letta 替换 Temporal 或引入第二套生产状态源。
- 把完整 transcript、Markdown/MemFS 或 provider conversation state 作为
  Fluctlight canonical Memory/Personality 来源。

## Completion boundary

P1 只有在以下行为链同时成立时才算闭环：

```text
fact
  → context projection
  → self-evaluated response/capability candidate
  → typed validation + freeze
  → visible response/effect
  → evidence window
  → reflection candidate
  → governed revision
  → future context consumes the revision
```

## Validation note

The Go Core/Gateway test, vet, build, race and diff gates pass for this P1
implementation. Browser workspace checks remain environment-blocked until Node
dependencies can be installed in the phase worktree; no browser contract or
generated artifact was changed in P1.
