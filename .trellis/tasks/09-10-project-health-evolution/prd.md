# Project Health Evolution

状态：in_progress；前置`09-09-llm-capability-runtime`已完成独立整改、真实环境复验，
并由用户确认可作为本任务fixed point。本文定义 Memory、Life Context、
Goal/Intention、Affect 与 Reflection 如何形成可观察、可审计、可回放的认知与行动闭环。
本任务只在 `/Users/vinson/Documents/project/个人/local-ai-companion-llm-capability-runtime` 的
`codex/llm-capability-runtime` 分支规划和实施；不得在 `master` 工作区写入。

规划评审：用户已于2026-09-10明确批准按当前`prd.md`、`design.md`和`implement.md`继续
实现，包括严格裁剪的Recent ActionOutcome Provider投影；后续整体Prompt重构不属于本任务。

数据兼容决策：用户于2026-09-11明确接受完全抛弃旧业务数据并按新authority重新开始。
本任务不再为旧Runtime、旧active payload或旧Life Context row保留兼容/repair路线；部署必须
使用全新数据库或由Owner显式重建数据库。migration不得静默迁移、猜测或改写旧语义数据。

## 目标与用户价值

让 Fluctlight 的 Memory、当前场景/生活上下文、Goal/Intention 与 Affect 不再只是
详情页中的字段或模型可见的背景文本，而是真正参与后续注意、欲望、决策、Capability
选择和自主行动；同时让 Reflection 消费真实事实与行动结果，可靠地更新可演化状态，
并让更新后的状态再次影响未来行为。

本任务中的“闭环”必须满足以下可追踪因果链：

```text
authoritative state / memory / life context / goal
  -> bounded provider context with resolvable references
  -> model-owned semantic decision with validated influence references
  -> frozen action and real Capability outcome
  -> recent-outcome context plus reflection evidence
  -> CAS-governed evolution commit
  -> later cognition observes the new revision and can act differently
```

只把字段读进 Prompt、只增加数据库表、只得到 `reflection status=applied`，或只在测试
中断言 JSON 包含某个 key，均不构成闭环完成。

## 已确认事实

- F01 `ContextProjection` 已读取 Current State、Life Context、Memory、Relationship、
  Drive/Preference/Trigger slots、Goal 和 Intention，并由 `compactCognitionContext`
  发送给 conversation、WakeUp、daily review 和 native cognition。
  证据：`apps/core-go/internal/core/intelligence.go:33-69,119-242`、
  `apps/core-go/internal/core/provider_context.go:18-88`。
- F02 Provider-facing Goal/Intention 投影删除 `id`、`goal_id`、`intention_id`，但
  Reflection 的 update/complete/pause 又要求数据库 ID，因此模型可以看到目标，却无法
  稳定引用并持续更新同一个目标或意图。
  证据：`provider_context.go:226-276`、`workflow_ops.go:1125-1217`。
- F03 conversation/daily-review response schema 没有“本次决定受哪些 Memory、Goal、
  Intention、Affect revision、scene fact 影响”的结构化字段。现有系统无法区分“字段被
  放进 Prompt”和“字段实际参与了本次决定”。证据：`provider_schemas.go:151-203`。
- F04 当前即时 appraisal 会通过 `persistCognitiveStagesTx` 更新 inner state，但 reducer
  使用 `0..1` clamp 处理 PAD；领域契约要求 PAD 与 emotional momentum 使用 `-1..1`。
  默认 PAD 为零，因此负向事件可能被直接压到零，不能形成可靠负向情绪状态。
  证据：`cognition_growth.go:44-157,159-216`、`affect_capability.go:147-184`、
  `CONTEXT.md:38-41`。
- F05 `affect_event` 具有独立 reducer 和事务，但当前 Capability 在 assistant caller-owned
  transaction 之前自行提交；Memory 也采用相同的独立事务模式。若随后 assistant/claims
  结算失败，会出现状态/记忆已变而本轮失败的分裂提交。
  证据：`mutations.go:704-866`、`affect_capability.go:188-247`、
  `memory_intelligence.go:90-147`。
- F06 Reflection 当前候选只覆盖 Memory、Relationship、Goal、Intention、Developing Self
  及 typed slots；没有 emotional summary、Affect baseline/regulation recalibration 或
  Current State reconciliation。证据：`provider_schemas.go:255-344`、
  `provider_prompts.go:16-24`。
- F07 Reflection 可以创建 Goal/Intention 或修改部分状态，但 Goal update SQL 不写回
  `progress/importance/urgency/deadline`，Intention update 不写回 trigger/preferred time/
  expiration/confidence，Memory 主要只有新增路径。候选被判为 no-op 或被过滤时，返回
  结果也没有逐类 applied/rejected/deferred 统计。
  证据：`workflow_ops.go:1042-1243`。
- F08 Goal/Intention 当前进入普通 cognition 和 daily review，但生产代码没有完整的
  “typed trigger due -> re-assessment -> frozen Capability action -> outcome -> goal progress”
  执行链。Goal/Intention 因而主要是 Prompt 背景和 Reflection 写入对象。
  证据：`autonomy.go:1-242,245-313` 以及对 `fluctlight_intentions` 的生产引用扫描。
- F09 Scene/Event 已按 `active Event > accepted Schedule > pending` 解析为当前 Life Context，
  并会进入后续 cognition；但缺少结构化 influence provenance，无法证明某次决定因场景
  改变而改变。证据：`detail.go:381-426`、`provider_context.go:18-88`。
- F10 Memory 已有 owner/visibility hard filter、FTS/词法/可选 embedding 排名和预算；
  但 background projection 使用空 query 时主要按重要性、情绪显著性、置信度和时间
  排序，且没有独立的 operation-aware retrieval plan。证据：
  `intelligence.go:424-561`。
- F11 conversation 和 autonomy action result 会成为 `autonomy.result` evidence 并创建
  Reflection intent，但最近 Capability/action outcome 不进入普通 `ContextProjection`；
  下一轮 cognition 可能看不到刚才的真实成功、失败或异步结果。
  证据：`cognition.go:417-466`、`cognition_growth.go:294-318`、
  `intelligence.go:33-69,119-242`。
- F12 Capability Runtime 重构曾在第二轮复验中发现公开 Schema、PreparedPayload、Image
  context、migration/replay、Schedule 幂等、事务、ContextResolver、错误码、OutputSchema
  和日志等 P0/P1/P2。上述问题已在独立`09-09-llm-capability-runtime`任务中全部关闭，
  并通过确定性门禁与 disposable 真实环境复验；用户已确认其可作为本任务 fixed point。
  历史问题与逐项证据分别见`../09-09-llm-capability-runtime/second-review-remediation.md`和
  `../09-09-llm-capability-runtime/remediation-evidence.md`。本任务只负责防止这些缺陷回归，
  不重新建立兼容双轨。
- F13 普通 conversation 必须保持一次 Main LLM cognition；Capability-local planner 只
  负责 HOW，不得生成第二份 MainAgent 回复或同轮 `role=tool` continuation。
- F14 历史产品决定要求 Reflection 根据场景、聊天和行动结果总结并迭代情绪、思考/
  对话方式、Relationship、Goal、Developing Self；WakeUp 只判断当前是否需要主动做事，
  不直接承担 Reflection 或 Current State mutation。

## 功能要求

### R00. 先收口 Capability Runtime

- 本任务是严格后继任务，不与`09-09-llm-capability-runtime`并行实施。在增加任何领域闭环
  代码前，必须先在原任务上下文关闭F12的全部已知P0/P1/P2；不得在错误的Schema、双来源PreparedPayload、
  不安全 replay 或分裂事务上继续叠加状态演化。
- 原任务必须独立完成：全部确定性门禁通过，真实PostgreSQL migration/replay已验证，
  Temporal active-history通过saved-history replay或明确drain/version gate，未运行项如实列出，
  再由主session重新审查并确认达到可提交/可合并状态。仅“测试全绿”或“direct主干存在”不够。
- 原任务未达到上述完成判据时，本任务保持planning/blocked；不得运行本任务的
  `task.py start`，不得新增ContextReference、Outcome、Affect、Goal/Intention或Reflection V2代码，
  也不得把两个任务的migration混成一个revision。
- 最终只有一个 direct Capability Runtime。Project Health/Evolution 使用该 Runtime 的
  Definition、ContextSlot、Invocation、PreparedPayload、Result、事务和 replay 边界，
  不建立第二套 Tool/Effect dispatcher。
- 原 `09-09-llm-capability-runtime` 任务继续作为 Runtime cutover 的权威设计与缺陷记录；
  本任务负责跨领域闭环和最终组合验收，两者的依赖必须在实施顺序中显式体现。

### R01. 统一的 Provider-safe 引用与因果证明

- 为 Memory、Relationship、Goal、Intention、Life Event/Scene、State revision 和近期
  Action Outcome 提供 bounded、opaque、可解析的 `context_ref`；不要求模型复制数据库
  元数据，也不能在 apply 时猜目标对象。
- cognition decision 增加结构化 `influences`：模型明确选择实际影响本轮 decision 的
  refs，并给出 bounded semantic note/confidence。Core 只校验引用属于当前 frozen
  projection，不用规则替模型推断“什么影响了它”。
- 空 influences 对简单回复/no-op 可以合法，但凡创建/推进 Goal/Intention、主动行动、
  Reflection 状态变更或声称受记忆/情绪/场景影响，都必须有合法引用。
- frozen action、Capability Invocation/Result、Action Outcome 和 Reflection revision
  保留同一因果链上的 refs、source fact、state revision 与 idempotency identity。

### R02. Memory 从写入到行为消费闭环

- 保留 explicit `memory_event` 作为普通 cognition 的显式 Memory 写入能力，并将
  Provider Arguments 与 runtime-owned evidence/profile/provenance/idempotency 分离。
- Reflection 支持 create、confirm、revise、merge/supersede、forget/deprecate 等受治理
  生命周期；不能只能追加近似重复 Memory。
- retrieval 在硬授权之后，根据 operation/context 构造 bounded query plan：conversation
  使用当前消息/主题，WakeUp/daily review 使用当前 scene、active goals/intentions、recent
  outcomes 与 unresolved items；没有语义 query 时采用明确的 salience/recency policy，
  不伪装成语义搜索。
- Provider 只能看到 bounded semantic payload 与 `context_ref`；Memory 被 cognition 引用
  后，引用进入 frozen decision/outcome/reflection evidence。
- Memory revision、embedding rebuild intent 与 outbox 保持原子；Reflection 不能把可见
  assistant prose 当作事实来源。

### R03. Life Context / Scene 驱动决策但不制造行为

- 当前 Scene、Activity、Location、Presence、Schedule source、event ref 和 revision 进入
  authoritative Current State slot，并可被 cognition/Capability planner 引用。
- Scene 只作为事实约束与 decision input；Core 不根据字符串或场景名自动选择行为。
  场景变化仍必须由 `scene_event` 或权威 Schedule/Event 产生。
- Image/Schedule/自主行动以及普通回复使用同一 frozen Life Context revision；决策后发生
  的 live 变化通过 CAS/re-assessment 显式处理，不能悄悄混入旧 action。
- 测试必须证明同一 observation 在不同 frozen scene/context 下可以形成不同的结构化
  decision/influence chain；不以概率性 live-model 结果作为唯一验收。

### R04. Affect 与 Drive 的即时闭环

- 修正 canonical numeric contract：PAD 和 emotional momentum 为 `-1..1`，其他 intensity/
  pressure/confidence/progress 为 `0..1`；旧错误范围数据必须有可重复 migration/reconciliation。
- 一次 observation 的 appraisal、Affect/Drive transition、frozen decision、事务型 native
  Capability effects、assistant claims/output facts属于一个 structured-turn commit boundary；
  任一必需环节失败时不得留下部分状态。
- LLM 只输出 semantic appraisal/event/direction/strength/confidence/evidence；Core policy
  根据版本化 reducer 计算 numeric delta、decay、momentum、regulation 和 drive pressure。
- Current State 的新 revision 必须进入本轮 frozen action和下一轮 context；决策的
  influences 可引用它。`state_expression` 只是可见表达，不得反向成为状态事实。
- Wall-time decay、momentum 与 regulation 必须按真实 elapsed time 执行，且结果不依赖
  Worker tick 频率。

### R05. Goal / Intention / Action / Outcome 闭环

- Goal/Intention 在 Provider context 中保留 opaque refs 以及真正用于决策的 lifecycle
  字段；Reflection 与 cognition 用 refs 而不是数据库 ID/描述匹配更新对象。
- Goal 支持 create/update/pause/resume/complete/abandon/cancel；update 必须处理 importance、
  urgency、progress、deadline、scope、target 和 evidence，并写 revision/governance history。
- Intention 支持 create/update/qualify/pause/resume/complete/expire/cancel；必须有 typed
  trigger、preferred time、expiration、confidence、goal ref、target/Capability intent 和
  revision。不得把任意自然语言变成代码条件或 keyword listener。
- time/event/semantic trigger 进入现有 durable inbox/Temporal workflow：到期只产生事实，
  仍由一次 cognition 结合当前状态决定并冻结 Capability action；Core 不直接推断语义动作。
- frozen decision 记录其服务的 Goal/Intention refs；真实 Capability/Action outcome 回写
  Intention status 和 Goal progress。失败、取消、拒绝、duplicate_suppressed 不能计为成功。
- Goal progress 由有证据的 outcome/reflection proposal 和 Core policy更新；不得根据
  assistant 说“完成了”或仅根据模型期望直接增加。

### R06. Action Outcome 回流

- 定义统一、bounded 的 `ActionOutcome`：action/call ref、Capability、status、expected、
  observed、error code、goal/intention refs、evidence、occurred_at、revision。
- immediate、deferred、async media、proactive message、Moment、scene、schedule、Memory 和
  Affect 的最终结算都产生或更新同一个 outcome authority；重试不得产生第二个 outcome。
- 最近相关 outcomes 作为独立 ContextSlot/Projection section 进入下一次 cognition；完整
  outcome 进入 Reflection evidence。异步结果只通过已排序 inbox fact 回流。
- 用户已授权将 Recent ActionOutcome 的严格裁剪摘要发送给当前配置的 LLM Provider。允许字段
  仅限 opaque outcome/context/Goal/Intention refs、Capability 名称、status、success boundary、
  occurred-at、bounded error code，以及白名单化结果信号（例如`delivery_status`、
  `target_kind`）；禁止发送原始 user/assistant 文本、raw expected/observed、内部
  ContextReferenceIndex/entity/database ID、provenance、完整 evidence payload 和 Provider/
  workflow/idempotency 内部标识。
- outcome 和 visible delivery 的成功边界必须与每个 Capability 的 Definition 一致，
  不能把 durable intent created 误写成 final asset ready。

### R07. Reflection 成为真正的演化事务

- Reflection 输入包含 bounded evidence window、当前可演化状态快照、opaque refs、最近
  outcomes 和 expected revisions；不得要求模型复制 runtime-owned profile ID、provenance、
  idempotency key 或完整关系数值快照。
- Reflection model-owned 输出至少覆盖：Memory lifecycle candidates、Relationship semantic
  observations、Goal/Intention lifecycle candidates、emotional summary/Affect-profile recalibration
  candidates、Drive/Preference/Trigger candidates、Developing Self candidates，以及本窗口
  的 bounded summary。
- Relationship/PAD/Drive/Goal progress 的 numeric delta 由 Core versioned policy计算；模型
  只提供语义方向、强度、confidence 与 evidence refs。
- prepare/validate 阶段在事务外完成 Provider I/O；最终 apply 在一个 caller-owned
  transaction 内完成 proposal、各领域 mutation、revision history 和 watermark。malformed
  schema、unknown/foreign evidence/ref使整个proposal invalid且不推进watermark；CAS stale或
  domain apply失败使事务整体rollback；schema合法但证据阈值/cooldown不足的候选必须记录
  deferred/rejected disposition，不能静默删除后伪装applied。
- `applied` 结果必须返回各领域 `applied/rejected/deferred/no_change` 计数、变更 refs、
  新 revisions 和 reason codes；空候选只能是显式 `no_change`，不能表现成含糊成功。
- Reflection 的变更必须在下一次 `ContextProjection` 中可见，并能被 `influences` 引用；
  这是闭环验收的最后一跳。

### R08. Evolution governance 与可观察性

- Core Persona anchors 继续遵守明确治理；可演化层的自动变更必须 evidence-backed、
  revisioned、auditable、idempotent、可回滚，并带 model/prompt/policy version。
- Diagnostics 能沿 correlation/source/window/action/context refs 查看：输入 evidence、模型
  proposal、验证/拒绝原因、实际 domain changes、watermark 和后续消费。
- 普通日志只记录 refs、计数、revision、duration、status 和 digest，不输出原始 Memory、
  Prompt、reflection payload 或 intent 明文。

### R09. 一次 cognition 与无启发式约束

- 普通 conversation 每轮最多一次 Main LLM cognition；不引入同轮 ToolResult continuation
  或第二份 MainAgent 回复。WakeUp 只做 action assessment，Reflection 始终异步。
- 不允许用 regex、keyword、固定文案、message length 或 scene name 在 Go 中推断 Memory、
  emotion、Goal、Relationship meaning 或 semantic action。
- Provider invalid/unavailable 时 fail closed、retry、defer 或 explicit no-op；不得合成默认
  情绪、默认 Goal、默认 Memory、默认人格或默认主动消息。
- direct conversation不是允许`no_op`成功的surface：user message提交后，本次Main cognition
  必须提供可见assistant文本；若没有，turn显式失败并保留同一idempotency retry，不能把
  cognition标为completed后只显示user message双勾。
- user message一经事务提交，Core必须在Provider调用前发送authoritative message frame；assistant
  message提交后也必须发送authoritative frame。Browser对stream与history采用单调merge，较旧的
  history响应不得擦除optimistic、queued或已stream确认的消息。

### R10. Migration、replay 与兼容终态

- 两个任务使用严格顺序的独立migration revision。Capability v2 active payload migration
  归`09-09`任务所有并先完成验证；本任务后续只在更高revision增加ContextReference、Outcome、
  Affect profile、Goal/Intention和Reflection V2字段。不得用一个revision同时表示两次cutover。
- active Capability v2 row migration与completed-audit保留规则只适用于前置0026 cutover；
  malformed/dual authority阻止该cutover。0030/0031不迁移任何active/completed业务row。
- completed audit history 可保留历史 shape但不得进入active executable decoder；Runtime不保留
  v1 fallback、missing-context空降级、live reprojection或old/new effect双轨。
- 对 active schedule/media/autonomy/intention workflow 提供 drain/replay/version gate，稳定
  call/provider/workflow/idempotency/ref identity 不变。
- 2026-09-11 clean-start决策覆盖本节此前的旧业务数据兼容要求：0030及后续新authority只接受
  空业务数据库；检测到旧Actor/Fluctlight/conversation/Life/Cognition数据时阻止升级并要求显式
  重建，绝不自动`TRUNCATE`或伪造repair metadata。新schema直接使用NOT NULL replay identity、
  closed stored result和提交级deferred constraints。

## 验收标准

- AC00 前置任务`09-09-llm-capability-runtime`已独立完成最终复验：全部已知P0/P1/P2关闭，
  Capability migration拥有新revision并在真实PostgreSQL验证，active workflow具备Temporal
  replay/drain/version证据，架构与全量Go门禁通过；在此之前AC01-AC14均不得开始验收。
- AC01 本任务从AC00确认的Capability fixed point开始，并持续回归单一Runtime、strict
  Input/Output schema、PreparedPayload分离、canonical replay、Schedule idempotency、
  caller-owned transaction和ContextResolver边界；新增闭环不得重新引入任何已关闭缺陷。
- AC02 一个确定性 conversation fixture 证明 Memory、Scene、Affect revision、Goal、
  Intention 和 recent Outcome 可通过合法 refs 进入 decision `influences`，foreign/stale ref
  在 freeze 前被拒绝。
- AC03 Memory e2e 证明 create -> retrieve -> cited decision -> outcome -> reflection
  confirm/revise/merge/supersede -> later retrieval；owner/visibility 过滤先于 ranking。
- AC04 Scene e2e 证明 event/schedule source priority、frozen context revision、scene-aware
  decision/planner 和 stale context conflict；场景不触发代码启发式行为。
- AC05 Affect e2e 证明负向 PAD 可低于 0、正负 clamp 正确、elapsed-time decay/momentum、
  appraisal与 assistant/native effects 原子提交、后续 decision 引用新 state revision。
- AC06 Goal/Intention e2e 证明 create -> typed trigger fact -> re-assessment -> frozen
  Capability -> real outcome -> Intention settlement -> Goal progress/completion；失败/取消不
  增加 progress，重试不重复 action。
- AC07 Reflection e2e 证明一个 window 能在单事务内产生至少 Memory、Goal/Intention、
  emotional summary/Affect-profile recalibration 中的真实变化，watermark 与各 revision 同 commit；stale/
  invalid proposal 不推进 watermark。
- AC08 Reflection 的 `applied/no_change/rejected/deferred` 结果可解释到逐类计数、refs、
  revisions 与 reason code；不再存在“显示 applied 但无法判断改了什么”。
- AC09 action outcome 在下一次 cognition 与 Reflection 中均可见，且 asynchronous completion
  通过 inbox 顺序回流；最终资产与 durable intent 的成功语义不混淆。
- AC09a Provider request fixture 证明 Recent ActionOutcome 只包含 R06 allowlist；带文本、raw
  expected/observed、entity ID、完整 reference index/provenance/evidence 的 fixture 必须在序列化
  前被删除，不能只依赖日志脱敏。
- AC10 普通 conversation、WakeUp、daily review、native cognition 和 Reflection 的 fake
  Provider tests 均证明一次 Main LLM/role call约束；无 `role=tool` continuation 或 hidden
  second reply。
- AC11 architecture guard 证明 Main flow/Runtime 不按具体 Capability 名称路由，生产代码
  不新增 semantic regex/keyword fallback，Reflection 不暴露 runtime-owned mutation metadata。
- AC12 pure/unit/integration 测试、Core/Gateway test-race-vet-build、gofmt、diff-check 通过；
  真实 PostgreSQL migration/replay、Redis trigger、Temporal history 和 opt-in live Provider
  分别报告已运行结果，未配置项不得伪装通过。
- AC13 `master` 工作区没有本任务写入；所有文档、代码、测试和验证都来自专用 worktree。
- AC14 Personality/Behavioral Policy evolution e2e 证明：单窗口或低置信度候选被defer，
  跨窗口证据可在max-delta/cooldown内创建profile-scoped overlay，Core Persona baseline未变，
  下一次cognition读取effective persona，rollback以新revision恢复且不删除历史。

## 明确不在范围内

- MCP、Capability Discovery、插件自动安装、Vector DB 替换、Event Bus、新 DI 框架、Policy
  DSL、群聊产品能力和多用户权限模型。
- 依据用户可见文本反推内部事实，或要求 live Provider 概率性表现成为 CI 唯一门槛。
- 通过提高 Prompt 长度、逐 Tool 触发描述或硬编码场景/情绪/关系词典制造“看起来更像闭环”。
- 本任务不重写整体提示词结构、语气或长篇 operation instruction。当前只增加最小的
  Recent Outcome section/authority 说明；用户后续会单独重新设计整体提示词。
- 无证据地让 Reflection 直接改 Identity anchor、Owner 权限、安全规则或基础设施配置。

## 已收敛的设计边界

- D01 Identity 与人工设定的 Core Persona anchors 保持人类治理；Reflection 不得自动
  改名、改背景锚点、改 Owner/权限/安全设定，也不得把一次状态直接固化为人格。
- D02 Personality 与 Behavioral Policy 允许自动慢演化，但只通过独立 evolution overlay
  生效；每次修改必须来自跨事件证据并满足限幅、置信度、冷却、CAS、审计和回滚策略。
- D03 Scene 是权威事实和行为约束，不是代码触发器；Goal/Intention 是模型动机与行动计划，
  但实际外部动作仍必须经过当前 cognition、Capability、权限、预算与 frozen-action 边界。
- D04 Reflection 不直接重写当前 PAD/mood。即时 cognition/`affect_event` 负责当前情绪；
  Reflection 生成 emotional summary，并更新慢速 Affect baseline、regulation、decay 与
  emotion pattern，使后续 lazy decay、appraisal 和 decision真实改变，同时避免同一 evidence
  被重复计入即时状态。
- D05 Recent ActionOutcome 可以进入当前 LLM 上下文，但必须采用 R06 的字段级 allowlist，
  由 Core 在 Provider 序列化边界构造；内部完整 outcome authority 与 Provider-safe DTO 是两个
  物理对象。整体 Prompt 重写留给后续独立任务，本任务不得借机扩写全局 Prompt。
