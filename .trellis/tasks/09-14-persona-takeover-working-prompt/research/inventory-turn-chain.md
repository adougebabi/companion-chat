# Turn 执行链盘点（为 Turn Takeover 服务）

> 只读盘点，未修改任何代码。所有结论均带 `文件:行号`，并区分「读到」与「推断」。
> 项目：`apps/core-go/internal/core`。核心交互入口：`mutations.go` 的 `handleTurn`。

> **[2026-09-14 状态标注]** 本文件是**代码事实记录**。其中的「建议 / 推断」部分不是架构权威：本文档层级为 **request > 经评审的 `design.md` > `implement.md` > research 旧建议**。以下条目已被评审否决，**标 `superseded`，不再作为实施依据**：
> - `superseded` §4 第 4 条「接管时向正在进行的 A 模型调用发取消」——目标流程中 A 已生成完毕，不存在「正在进行的 A」。
> - `superseded` §4 第 7 条「B 接管需同时覆盖 `query_continuation` 子状态」——QUERY continuation 与 takeover **互斥**（`design.md` §4.7）。
> - `superseded`「最自然的插入点」一节中「用 Judge（轻量 LLM）生成 B 的 `decision` + `capability_invocations`」——Judge **只返回布尔**，B 是一次独立的完整主生成（`design.md` §4.4）。
> - `superseded` §6 与 18e 的措辞矛盾——已按 F12 结论修正为：`settleDeferredCapabilitiesTx`（`capability_runtime.go:366`）**在结算事务内**（`mutations.go:1157` ⊂ `:1122`），但内置 deferred 能力**无网络 IO**（`builtin_capabilities.go:123-136`、`:243-295`），真正外部执行在事务提交后由 outbox/工作流完成；详见 `design.md` §4.10。
> - `superseded` recent 消息 role 表把 recent 全写成 `user`——实际 `recentPromptFragments`（`provider_context.go:133-135`、`:164`）**保留** `role=user/assistant`。

---

## 1. 端到端调用链表（R01）

下表从「用户消息进入 Core」到「私聊送达用户」逐步骤列出。列：步骤 → `文件:行号` 函数 → 输入/输出 → 是否数据库事务内 → 外部副作用。

| # | 步骤 | `文件:行号` | 函数/动作 | 事务内？ | 外部副作用 |
|---|------|------------|-----------|----------|-----------|
| 1 | HTTP 入口 | `httpapi/server.go:746` `Server.turn`；`:757` 调 `app.StreamTurn` | 接收 `POST /internal/conversations/{id}/turn`，鉴权后调 `StreamTurn` | 否 | HTTP NDJSON 流（响应体） |
| 2 | 流式包装 | `mutations.go:1378` `StreamTurn` → `:1403` 调 `handleTurn` | 把 NDJSON 回调（`onActionResult`/`onChunk`）包成 `turnCallbacks` | 否 | — |
| 3 | 落盘用户事实 | `mutations.go:527` `withTransaction` → `:559` `enqueueTurnFactTx`（`cognition.go:224`） | 入参 `actorID/conversationID/text/idempotency` → 写 `conversation_messages`(user) + `cognition_inbox` + `platform_workflow_intents`('cognition.processing') + outbox `cognition.fact.created`（`cognition.go:309-315`） | **是** | 把更早的 pending/claimed inbox 置 `failed`/`superseded_by_newer_turn`（`cognition.go:265-270`，同事务内） |
| 4 | 取消竞争 | `mutations.go:607` `cancelSupersededCognitionFacts` → `cognition.go:321` | 对上一步返回的 `supersededInboxIDs` 写 Redis 取消键 | 否 | **Redis 写入**（外部） |
| 5 | 立即回显用户帧 | `mutations.go:640` `emitUserFrame` | 通过 `callbacks.onActionResult` 把 user message 推给客户端 | 否 | **HTTP 流推送**（外部） |
| 6 | 抢占/重放检查 | `mutations.go:650` `recoverFrozenTurnAfterAssistant`（`cognition.go:1262`） | 若已存在 `assistant:turnID` 消息，则重放既有 frozen action 的结算 | 否（内部再开事务） | 可能触发下游结算副作用（见 16） |
| 7 | 读取已冻结候选 | `mutations.go:692` `LoadFrozenTurn`（`cognition.go:345`） | inboxID → `frozenTurn`（payload/status） | 否 | 无 |
| 8 | 组装投影 + 主 LLM 评测 | `mutations.go:711` `buildTurnProjection`；`:763` `assembleProjectionPrompt`；`:770` `Provider.StructuredAssembledWithToolsSchema("cognitive_assessment", ...)` | 用户文本 → 模型结构化输出（可见文本 + tool_calls = capability 调用） | 否 | **LLM 调用**（外部，最大副作用之一） |
| 9 | 占用/抢占再校验 | `mutations.go:643/772/777/885` `cognitionFactSuperseded`（`cognition.go:334`） | 检查 inbox 是否被标 `superseded_by_newer_turn` | 否 | 读 Redis/DB |
| 10 | 归一化决策 | `mutations.go:822` `normalizeResponsePlan`；`:838` `resolveCapabilityAction`；`:877` `normalizeCompositeAction` | 模型原始输出 → `decision`/`action`/`composite`/`responseMode` | 否（纯内存） | 无 |
| 11 | 冻结候选 | `mutations.go:888` `PersistTurnDecision`（`cognition.go:398`） | 把 `decision` + `capability_invocations` 写入 `cognition_frozen_actions`(status='frozen')、`cognition_assessments`、`cognition_decision_proposals` | **是**（`:446` `withTransaction`） | 仅 DB；冻结点本身无外部副作用 |
| 12 | 准备（Prepare）能力调用 | `mutations.go:925` `prepareCapabilityInvocations`（`capability_runtime.go:168`）→ `:191` `runtime.Prepare` | 为每个 invocation 跑 Prepare（补全参数/上下文快照；`conversation.reply` 的 Prepare 为空操作，`image.generate` 等可能预检） | 否（每个 invocation 可能读/写外部） | **部分能力有外部调用**（见副作用清单） |
| 13 | 持久化已准备调用 | `mutations.go:933` `persistFrozenCapabilityInvocations`（`cognition.go:484`） | `UPDATE cognition_frozen_actions SET payload=jsonb_set(payload,'{capability_invocations}',...)` | **是**（单条 `Exec`，非 `withTransaction`，但仍是事务语句） | 仅 DB |
| 14 | 规划/执行纯查询能力 | `mutations.go:1012` `planCapabilitiesForTransaction`（`capability_runtime.go:55`）→ `executeCapabilities`（`:63`） | `deferTransactional=true`：`pure_query`（如 `memory.recall`/`relationship.lookup`）真正执行；事务型/异步型返回 `deferred` | 否（执行本身在 tx 外） | **查询型能力的外部调用**（如记忆召回） |
| 15 | 持久化能力结果 | `mutations.go:1023` `PersistCapabilityResults`（`cognition.go:470`） | `UPDATE ... SET payload=jsonb_set(payload,'{capability_results}',...)` | **是**（单条 `Exec`） | 仅 DB |
| 16 | QUERY continuation（仅首轮特例） | `mutations.go:1043-1105`，`validatePureQueryContinuation`(`query_continuation.go:20`)、`queryContinuationMessages`(`:92`)、`Provider.StructuredQueryContinuation`(`:1061`)、`continuationVisibleText`(`:111`) | 无可见文本 + 1–2 个 pure query 时，最多一次**不带工具**的 LLM 续写 → 产出可见文本，持久化进 frozen payload | 否（模型调用在 tx 外） | **LLM 续写**（外部） |
| 17 | 选定可见文本 | `mutations.go:1107-1111` | `visible = decision.visible_text` 或 `replyTextFromCapabilityInvocations`（取 `conversation.reply` 参数里的 text） | 否 | 无 |
| 18 | **结算事务（核心副作用窗口）** | `mutations.go:1122` `withTransaction` → 内部： | | **是** | |
| 18a | 版本门控 | `mutations.go:1123` `requireCognitionAuthorityRevisionsTx` | 校验 `ContextRevision/CurrentStateRevision/LifeContextRevision` | 是 | 乐观并发控制 |
| 18b | 人格演化提案 | `mutations.go:1126` `applyPersonalityDecisionPlanTx`（`personality_runtime.go:227`） | `personalityDecisionPlan` → `UPDATE/INSERT fluctlight_personality_runtime` | 是 | **根级状态副作用**（人格切换） |
| 18c | 认知/情绪/状态提案 | `mutations.go:1129` `applyFrozenCognitiveStagesTx`（`cognition_growth.go:223`）→ `persistCognitiveStagesTx`（`cognition_growth.go:180`） | 写 `cognition_appraisals`(accepted)、`cognition_focus_cycles`、`cognition_internal_dynamics`、`fluctlight_state_revisions`（emotion/mood/state 增量） | 是 | **根级状态副作用**（appraisal/情绪/演化） |
| 18d | 写私聊消息行（真正“发送”） | `mutations.go:1149` `INSERT conversation_messages(kind='assistant', text=visible)` | 这就是 DM 的持久落库 | 是 | 仅 DB（网络投递见 20/21） |
| 18e | 执行延迟能力/外部工具 | `mutations.go:1157` `settleDeferredCapabilitiesTx`（`capability_runtime.go:366`） | 事务型走 `ExecuteTransactional`(带 savepoint，`capability_core.go:1649`)；deferred-output 走 `ExecuteDeferred`(`:1583`)；包括 `memory_event`/`affect_event`/`media.image.generate`/`scene`/`presence`/`schedule.replan` 等 | 是（**在结算事务内**） | 仅 DB 行写入 + intent 登记；**无网络 IO**（外部执行在提交后由 outbox/工作流完成） |
| 18f | 持久化 claim | `mutations.go:1179` `persistClaimsTx`（`intelligence.go:943`） | 写 cognition claim 行 | 是 | 仅 DB |
| 18g | 完成冻结动作 | `mutations.go:1182` `completeTurnCognitionTx`（`cognition.go:583`） | 置 frozen action=`completed`、inbox=`processed`、建 `autonomy.result` 事实、写 outbox `autonomy.result`（`cognition.go:687`）、关系互动记录(`:659-668`) | 是 | DB + outbox 事件 |
| 19 | 触发反思 | `mutations.go:1203` `scheduleReflectionTrigger` | 安排 quiet-period reflection intent | 否 | Redis/DB 计划 |
| 20 | 流回显助手帧 | `mutations.go:1204-1212` `emitAssistantFrame`/`onChunk` | 把 assistant message 推给客户端 | 否 | **HTTP 流推送**（外部，同步送达通道之一） |
| 21 | 异步投递扇出 | `platform/redis_pipeline.go:122`（publisher 读 `platform_outbox_events`）→ `:184` `NewEventConsumer` → `:244` `process`（消费组 `bff-notifications` 等） | outbox 事件 → Redis Stream → 下游（推送/WebSocket 扇出） | 否（独立进程/连接） | **网络投递**（外部，异步送达通道） |

**关键结论（读到）**：用户消息在步骤 3 的**短事务**内落盘并被冻结为 `cognition_inbox`+`conversation_messages`；真正的“发送”是步骤 18d 把可见文本写进 `conversation_messages(assistant)`（事务提交即“已送达”持久层），网络层投递由步骤 20（同步流）与 21（outbox→Redis→consumer，异步）两条独立通道完成。模型调用发生在事务**外**（步骤 8、16）。

---

## 2. 候选与冻结的生命周期

- **模型输出被解析成什么结构**：`Provider.StructuredAssembledWithToolsSchema` 返回 `completion.Structured`（map）+ `completion.ToolCalls`（[]`CapabilityInvocation`）。见 `mutations.go:780-784`：`decision = completion.Structured`；`capabilityInvocations = completion.ToolCalls`（`CapabilityInvocation` 结构见 `capability_core.go:896`）。
- **在哪里被冻结**：`PersistTurnDecision`（`cognition.go:398`），内部 `withTransaction`（`:446`）向三张表插入：
  - `cognition_assessments`（`cognition.go:455`）
  - `cognition_decision_proposals`（`cognition.go:458`）
  - `cognition_frozen_actions`（status='frozen'，`cognition.go:461`，`ON CONFLICT DO NOTHING`）。
- **冻结记录类型与字段**：Go 结构 `frozenTurn`（`cognition.go:13-21`）：`ID / InboxID / ActionType / Payload / StateRev / Status / ErrorCode`。`Payload` 由 `cognition.go:414` 构造，含：
  - `capability_runtime_version`（常量 `CapabilityRuntimePayloadVersion`，用于负载校验 `cognition.go:365`）
  - `turn_id`、`conversation_id`
  - `decision`（含 `context_projection`、`personality_transition`(`personalityDecisionPlan`)、`composite_action`(`CompositeActionV1`)、`response_plan`、`visible_text`、`appraisal`、`cognitive_state_transition`、`context_reference_version/index`、`influences`、`goal_refs`、`intention_refs`）
  - `capability_invocations`（[]`CapabilityInvocation`）、`capability_results`（[]`CapabilityResult`）、`capability_context_snapshot`
  - 另有 `state_revision`（从 `fluctlight_inner_states` 读，`cognition.go:448`）作为 `StateRev`。
- **冻结后到执行之间的窗口（读到，重要）**：`PersistTurnDecision` 在 `:888` 提交（独立事务）。之后步骤 12–15 在事务外做 Prepare/规划，步骤 18 才在另一事务内执行副作用。若进程在 `:888` 之后、`:1122` 之前崩溃，既有 `recoverFrozenTurnAfterAssistant`（`cognition.go:1262`，由 `mutations.go:652` 在重放路径触发）或下一次 `ProcessCognitionInbox` 重入 `handleTurn` 时会通过 `LoadFrozenTurn`（`:692`/`cognition.go:345`）读到 status='frozen' 的候选并重跑 18 步结算。**这就是“未执行提案”阶段**——候选已持久但 side effect 尚未发生，是 Turn Takeover 介入的物理窗口。

---

## 3. 副作用清单（拒绝 A 候选必须拦住的落点）

逐条带 `文件:行号`：

1. **能力 Execute 的实际实现**
   - 事务型/延迟输出能力：`settleDeferredCapabilitiesTx`（`capability_runtime.go:366`）→ `runtime.ExecuteTransactional`（`capability_core.go:1649`，带 savepoint）或 `runtime.ExecuteDeferred`（`capability_core.go:1583`）。
   - 纯查询/即时能力：`executeCapabilities`（`capability_runtime.go:63`）→ `runtime.Execute`（`capability_core.go:1381`）或 `executeExclusiveCapabilityCanonical`（`:272`，持 `pg_advisory_lock`）。
   - 各能力的 `Execute`/`ExecuteDeferredTx`：`builtin_capabilities.go`（`conversationReplyCapability` `:120`、`:123`；`memoryEventCapability` `:538`；`affectEventCapability` `:627`；`imageGenerateCapability` `:219`；`sceneEventCapability` `:355`；`presenceEventCapability` `:386`；`scheduleReplanCapability` `:490` 等）。

2. **根级状态提案的提交点**
   - 人格演化：`applyPersonalityDecisionPlanTx`（`personality_runtime.go:227`）→ `fluctlight_personality_runtime`（切换 active_profile_id，含 revision CAS，`personality_runtime.go:236/244`）。
   - Appraisal / 情绪 / 内部动力学 / 状态演化：`applyFrozenCognitiveStagesTx`（`cognition_growth.go:223`）→ `persistCognitiveStagesTx`（`cognition_growth.go:180` 附近）→ 写 `cognition_appraisals`(status='accepted')、`cognition_focus_cycles`、`cognition_internal_dynamics`、`fluctlight_state_revisions`（state pad/mood/momentum/regulation/drives/conflicts 增量，`cognition_growth.go:204-217`）。
   - 关系变更：`completeTurnCognitionTx` → `recordRelationshipInteractionTx`（`cognition.go:665`，在结算事务内）。
   - 记忆（active/episodic）：由能力 `memory_event`/`active_memory_event` 在 `settleDeferredCapabilitiesTx` 内执行（属能力副作用，见 1）。

3. **事件/监听器/consumer 是否会在冻结阶段就触发（读到）**
   - 冻结本身（步骤 11）**只写 DB，不触发任何外部监听**。
   - 但 `enqueueTurnFactTx`（`cognition.go:309-315`）在**步骤 3 的短事务**里就写了 outbox `cognition.fact.created` 与 `platform_workflow_intents('cognition.processing')`。后者由 Temporal Dispatcher（`workflow/workflow.go:1401` `DispatchOnce`、`:601` `ProcessCognitionActivity` → `:606` `ProcessCognitionInbox`）消费并重新进入 `handleTurn`。**即：冻结前已有 consumer 在跑；冻结动作不会额外触发监听器，但 cognition 流程整体是被 workflow intent 驱动的。**
   - 真正的“副作用型”监听（bff-notifications 等）在步骤 21 消费 `autonomy.result`/`cognition.fact.created` 等 outbox 时触发，**都在候选已执行之后**。

4. **自动 Tool Runner / 参数补全 / 冻结回调是否可能在执行前调用（读到）**
   - Prepare 阶段（步骤 12，`prepareCapabilityInvocations` `capability_runtime.go:168` → `runtime.Prepare` `capability_core.go:1502`）**在冻结之后、结算之前**运行，会做参数补全与上下文快照（`normalizeCapabilityInvocationMetadata` `capability_runtime.go:238`）。这是“执行前”唯一会动外部的地方（部分能力 Prepare 有外部预检，如 `image.generate`）。
   - 没有独立的“冻结回调自动执行器”在冻结瞬间触发能力。`conversation.reply` 的 `Execute`（`builtin_capabilities.go:120`）只返回 `deferred`，真正不做事，仅 `ExecuteDeferredTx`（`:123`）校验文本——**不会二次改写文本，也不会提前发送**（见第 4 节）。

**结论（推断+读到）**：要“拒绝 A 的全部候选副作用”，物理上需要在步骤 18 的结算事务**前/内**拦住：18b 人格演化、18c appraisal/情绪/状态、18d 私聊消息行、18e 能力执行、18g 完成与 outbox。最干净的是在**步骤 11 之后、步骤 12/18 之前**让接管生效（见第 8 节）。

---

## 4. `conversation.reply` 的真实实现

- **参数结构（读到）**：`conversationReplyCapability` 定义在 `tool_contract.go:191`（`conversationReplyCapabilityDefinition`）：`Name:"conversation.reply"`、`Version:"v1"`、`Type:CapabilityTypeAction`、`RequiredContext:[]SlotCurrentLife`、`FailurePolicy:FailurePolicyRequiredForVisibleClaim`、`IsDeferredOutput()=true`、`TargetKinds:["conversation_message"]`、`OutputRole:"conversation_message"`、`ConcurrencyClass:"exclusive"`（`tool_contract.go:196-212`）。
- **执行时是否还有一次模型改写（读到：否）**：
  - `Execute`（`builtin_capabilities.go:120`）直接 `return ... Status:"deferred"`，**不调用任何模型**。
  - `ExecuteDeferredTx`（`:123`）仅 `json.Unmarshal` 参数、校验 `text` 非空且 ≤32000 字符，返回 `Status:"completed"`。它是**纯校验器**，不产生也不改写文本、不发起网络发送。
  - 因此：**最终可见文本在步骤 8 的主 `cognitive_assessment` 调用时即由模型写入 `conversation.reply` 的 `arguments.text`**，由 `replyTextFromCapabilityInvocations`（`capability_runtime.go:581`）或 `decision.visible_text`（`mutations.go:830/864/1107`）读取，最终写进 `conversation_messages`（`mutations.go:1149`）。**无二次模型改写。**
- **最终文本确定点（读到）**：模型 `cognitive_assessment` 响应 → `mutations.go:780` `decision`；可见文本取 `normalizeVisibleReply(decision.visible_text)` 或 `replyTextFromCapabilityInvocations(...)`（`mutations.go:830,864,1107`）。
- **发送管线（读到）**：`conversation.reply` 本身**不做网络发送**。发送是步骤 18d 把 `visible` 写进 `conversation_messages(assistant)`（事务提交即持久送达），之后由：
  - 同步：`emitAssistantFrame`（`mutations.go:1212`）经 NDJSON 流推给当前连接客户端；
  - 异步：outbox 事件（步骤 3 的 `cognition.fact.created`、步骤 18g 的 `autonomy.result`）经 `platform/redis_pipeline.go:122` 发布到 Redis Stream，由 `EventConsumer.process`（`:244`，消费组 `bff-notifications` 等）投递到客户端/推送。
- **是否唯一（读到：在交互 Turn 链内唯一）**：`conversation.reply` 是交互 Turn 中唯一 `OutputRole:"conversation_message"` 的能力（`tool_contract.go:212`；`hasConversationReplyCapability`/`replyTextFromCapabilityInvocations` 均按 `OutputRole=="conversation_message"` 判定，`capability_runtime.go:569/581`）。**不存在 `visible_reply` 旁路**（全仓 grep `visible_reply` 无结果）。注：自治/唤醒链（`autonomy.go`/`wakeup.go`）也会写 `conversation_messages`，但不在本 Turn 链范围内。

---

## 5. QUERY continuation 的真实实现

- **触发条件（读到）**：
  - 主调用返回 `structuredFallback && 无 conversation.reply/deferred 输出` 时 `responseMode` 被推导为 `query_continuation`：`normalizeConversationResponseMode`（`query_continuation.go:38`，`:43` 判断 `validatePureQueryContinuation(...)==nil` 且 `visibleText==""`）。
  - 显式契约守卫：`mutations.go:852-858`，要求 `visibleCandidate==""` 且 `validatePureQueryContinuation(...)` 通过，否则报 `query_continuation_contract_invalid`。
- **精确判断函数与行号（读到）**：`validatePureQueryContinuation`（`query_continuation.go:20`）：要求 `registry!=nil`、调用数 **1–2**（`query_continuation.go:21`），且每个调用的 `classifyCapabilityExecution` 必须为 `CapabilityExecutionPureQuery`（`:31`）。pure query 能力示例：`memory.recall`（`memory_recall_capability_test.go:28`）、`relationship.lookup`。
- **调用次数上限（读到）**：**最多 1 次** continuation LLM 调用（不带回填工具）。`mutations.go:1061` 与 `:1088` 各调用一次 `Provider.StructuredQueryContinuation`，但受 `continuationState.Phase` 状态机控制（`query_continuation.go:62-74`）：`requested → queries_completed → provider_completed`。重放时若已是 `provider_completed` 直接取 `VisibleText`（`:1103-1104`），不会再调模型；`queries_completed` 分支也只调一次。即：无论重放几次，模型续写**至多执行一次**（结果持久化在 frozen payload 的 `query_continuation.VisibleText`）。
- **消息如何组装（读到）**：`queryContinuationMessages`（`query_continuation.go:92`）：以 `BaseMessages`（主调用请求消息，`mutations.go:768` `cloneMapSlice(assembly.Messages)`）为基底，追加 `assistant`(tool_calls) + 每个 `tool`(result) 消息（`:104-107`）。
- **结果如何回到同一发送管线（读到）**：`continuationVisibleText`（`query_continuation.go:111`）从 `completion.Structured["visible_text"]` 取文本 → 赋给 `visible`（`mutations.go:1068/1092/1104`）→ 与正常路径汇合到步骤 17/18d 同一 `conversation_messages` 写入。状态用 `persistQueryContinuationState`（`query_continuation.go:78` → `UPDATE cognition_frozen_actions payload#>>'{query_continuation}'`）持久化。
- **静态守卫/测试（读到）**：`query_continuation.go:54` `queryContinuationStateFromValue` 校验 `SchemaVersion`/`RequestDigest`/`BaseMessages` 长度；`mutations.go:906` 校验 `RequestDigest` 一致性（防重放篡改）。测试见 `provider_live_tool_test.go:399`（`liveProviderMessage` 带 `query_continuation_response` schema）。

---

## 6. 事务与恢复

- **事务边界（读到）**：
  - 短事务①：`enqueueTurnFactTx`（`mutations.go:527`）。
  - 冻结事务：`PersistTurnDecision`（`cognition.go:446`）。
  - 单语句事务：`persistFrozenCapabilityInvocations`（`cognition.go:484`）、`PersistCapabilityResults`（`:470`）、`persistQueryContinuationState`（由 `UPDATE` 直接执行）。
  - 结算事务：`mutations.go:952`（no_op）与 `:1122`（reply），以及 `recoverFrozenTurnAfterAssistant` 内 `:1302`。
  - **模型调用、Prepare 均在上述事务之外**。能力执行分两类：`pure_query` 类（记忆召回 / 关系查询）在事务外真正执行；事务型与 deferred-output 能力在结算事务**内**运行 `settleDeferredCapabilitiesTx`，但按契约只做本行数据写入与 intent 登记，**不做网络 IO** —— 真正的外部执行在事务提交后由 outbox/工作流完成（`design.md` §4.10，F12）。
- **幂等键机制（读到）**：
  - `conversation_messages.idempotency_key`（`user:`/`assistant:`+turnID），唯一索引（`migrations/runner.go:2169` `uq_conversation_messages_turn_kind`），保证同轮用户/助手消息只写一次（`mutations.go:537/1135` 先查后插）。
  - `cognition_inbox.idempotency_key`（= 入参 idempotency，`cognition.go:309`），`enqueueTurnFactTx` 用 `FOR UPDATE` 查重并复用既有 inbox（`:230-261`）。
  - `cognition_frozen_actions` 的 `frozenID = "frozen_"+stableDigest(inboxID)`（`cognition.go:404`），**确定性**、`ON CONFLICT DO NOTHING`（`:461`）——同一 inbox 再冻结不会覆盖。
  - outbox 用 `idempotency_key` + `ON CONFLICT DO NOTHING`（`outbox.go:9`）。
  - Provider 取消键：`WithProviderCancellationKey(ctx, inboxID)`（`mutations.go:608`）+ Redis `fluctlight:cognition:cancel:<inboxID>`（`cognition.go:331`）。
- **崩溃恢复入口（读到）**：
  - `ProcessCognitionInbox`（`cognition.go:28`）→ `HandleTurn`（幂等：按 inboxID 重入，`LoadFrozenTurn` 读 status）。
  - `recoverFrozenTurnAfterAssistant`（`cognition.go:1262`）：若 `assistant:turnID` 已存在而 frozen 仍 `frozen`，重跑 18 步结算。
  - `claimCognitionInbox`/`releaseCognitionClaim`（`cognition.go:102/148`）：10 分钟租约，避免并发 worker 重复执行。
  - 既有冻结决策被复用：`handleTurn` `:703-744` 直接读 `frozen.Payload` 的 decision/invocations，不再调模型。
- **并发串行化与版本控制（读到）**：
  - `cognition_inbox` 同 fluctlight 的 `earlierPending` 检查（`cognition.go:130`）：有更早 pending/claimed 则 `ErrConflict`，保证按序列顺序处理。
  - `enqueueTurnFactTx` 把同会话更早的 `conversation.turn` inbox 置 `superseded_by_newer_turn`（`cognition.go:265-270`）。
  - 结算前 `requireCognitionAuthorityRevisionsTx`（`mutations.go:1123`/`cognition.go:583` 调用）校验 `ContextRevision/CurrentStateRevision/LifeContextRevision`，版本不匹配则失败（如 `life_context_test.go:414` 验证 `life_context_stale`）。
  - 能力级并发：`ConcurrencyClass=="exclusive"` 走 `pg_advisory_lock`（`capability_runtime.go:272-292`）。
  - WakeUp/Reflection 通过 `platform_workflow_intents`（带 `status` 状态机与 `superseded`）串行化，且 `enqueueQuietPeriodReflectionIntentTx`（`cognition.go:700`）会把旧 reflection intent 置 `superseded`。

---

## 7. 执行资格校验

模型输出的 capability 调用在执行前依次过以下校验（逐条 `文件:行号`，读到）：

1. **结构/编号校验**：`CapabilityInvocation` 由 `completion.ToolCalls` 构造；`normalizeCapabilityInvocationMetadata`（`capability_runtime.go:238`）补全 `CallID/SourceFactID/ProviderRequestID/SchemaVersion`。
2. **工具存在性**：`executeCapabilities` 中 `registry.Definition(invocation.CapabilityName)`（`:87`）与 `registry.LookupCapability`（`:96`）——缺失返回 `capability_not_found`（`:89/98`）。
3. **执行类校验**：`classifyCapabilityExecution(capability, definition)`（`capability_core.go:58`，调用点 `capability_runtime.go:105`）——非法类返回 `capability_execution_class_invalid`。
4. **参数合法性（Prepare 阶段）**：`runtime.Prepare`（`capability_core.go:1502`）做参数解码/补全；`invocation.Validate(definition)` 在 `validateCapabilityInvocationsForPersistence`（`capability_runtime.go:205-218`）中调用；`conversation.reply` 文本长度 ≤32000（`builtin_capabilities.go:132`）。
5. **权限/业务约束**：
   - `FailurePolicy` 必须为 `FailurePolicyRequiredForVisibleClaim` 或 `OptionalInternal`（`capability_core.go:883`）；`RequiredContext` 必须声明（`capability_core.go:886`）。
   - 占用性门控：`requireCognitionAuthorityRevisionsTx`（结算前，`:1123`）。
   - pure-query 特例：`validatePureQueryContinuation`（`query_continuation.go:20`）。
   - 必填能力失败拦截：`requiredCapabilityFailureCanonical`（`capability_runtime.go:294`），`status=="failed"||"rejected"` 则整体失败（`:313`）。
6. **冻结期决策完整性**：`validateFrozenDecisionInfluences`（`cognition.go:415`、`:719`）、`validateExecutableCapabilityPayload`（`cognition.go:364`，校验 `capability_runtime_version`、无 `tool_calls`/`tool_results` 双权威字段）。

**是否已有“拒绝/接管”状态机（读到）**：
- `cognition_frozen_actions` 的状态机只有 **`frozen` / `completed` / `failed`** 三种（`cognition.go:461` 写 frozen、`:638` 写 completed、`:780` 写 failed）。**不存在 `rejected` 或 `superseded` 状态**（grep `status='superseded'/'rejected'` 在 `cognition_frozen_actions` 上无匹配；`:703` 的 superseded 是 `platform_workflow_intents` 的 reflection intent，非 frozen action）。
- `CapabilityResult.Status` **可以有 `rejected`**（`capability_runtime.go:313`、`capability_core.go:547/1735`），但这是“能力结果”级别，不是“候选 action”级别。
- “抢占/接管”在现有代码里是**在 inbox 层**建模的：更早的 turn 被置 `failed` + `error_code='superseded_by_newer_turn'`（`cognition.go:267`），由 `cognitionFactSuperseded`（`cognition.go:334`）在多处（`:643/772/777/885/1000/1072/1086/1096/1215`）拦下旧 turn 的继续。

> 结论：**没有可直接复用的“frozen action 被 rejected/superseded”状态**。接管机制需要新增状态/标记，或复用 inbox 级 `superseded_by_newer_turn` 语义。

---

## 8. 接入点建议（Turn Takeover）

**目标**：A 人格候选冻结后、发送前，由轻量 Judge 判断是否让 B 接管；接管后**拒绝 A 的全部候选副作用**（含根级 appraisal/人格演化），仅执行 B 的候选。

### 最自然的插入点（读到推断）

**位置：`mutations.go` 步骤 11（`PersistTurnDecision`，`:888`）之后、步骤 12（`prepareCapabilityInvocations`，`:925`）之前。**

理由：
- 此时 A 候选已冻结（status='frozen'，`cognition_frozen_actions` 已提交），但 **18b/18c/18d/18e/18g 的全部副作用尚未发生**——正是“未执行提案”窗口（见第 2、3 节）。
- 在此插入 Judge：
  - 若**不接管**：原流程不变（A 的副作用照常执行）。
  - 若**接管**：**[superseded — 见文首标注]** 用 Judge（轻量 LLM，可复用 `assembleProjectionPrompt`+`StructuredAssembledWithToolsSchema`，参考 `mutations.go:763/770`）生成 B 的 `decision` + `capability_invocations`，然后**覆盖既有 frozen action 的 payload**（见下），再继续步骤 12→18，结算时执行的是 B 的候选。A 的副作用因从未被读取执行而天然被“拒绝”。
    > 替代设计：Judge **只返回布尔**，B 是一次独立的完整主生成（`design.md` §4.4/§5）；覆盖 payload 时必须按 `turn_stage` 条件 UPDATE、重置持久切换授权、重算每条 invocation 的 `ContextSnapshot`（`design.md` §4.8）。
- 该点复用现有机制即可，无需在 18 步事务内插入复杂分支。

### 可复用的现有机制

1. **冻结/重载机制**：`LoadFrozenTurn`（`cognition.go:345`）与 `frozen.Payload` 结构已支持“读回决策并重跑”。接管只需改 `payload` 内容。
2. **in-place payload 更新先例**：`persistFrozenCapabilityInvocations`（`cognition.go:484`）、`PersistCapabilityResults`（`:470`）、`persistQueryContinuationState`（`query_continuation.go:78`）都用 `UPDATE ... SET payload=jsonb_set(payload,'{...}',...)`。**新增一个“覆盖决策”的 `UPDATE`（如 `UPDATE cognition_frozen_actions SET payload=jsonb_set(payload,'{decision,capability_invocations,capability_results,capability_context_snapshot}',...)`)即可复用同一模式**，且同样受 `WHERE id=$1 AND status='frozen'` 约束（与 `cognition.go:474/488` 一致，保证只在未执行前可改）。
3. **幂等/确定性 ID**：`frozenID="frozen_"+stableDigest(inboxID)` 是确定性的（`cognition.go:404`），但 `PersistTurnDecision` 用 `ON CONFLICT DO NOTHING`（`:461`）——**再调一次不会覆盖 A**。因此接管应走“UPDATE 覆盖 payload”而非“再 freeze”，或先 `FailTurnCognition`(A) 再 freeze(B 用不同 inbox 派生 ID)。推荐前者（同 frozenID 覆盖）。
4. **[superseded — 见文首标注]** **抢占/取消语义**：`cognitionFactSuperseded` + Redis 取消键（`cognition.go:331/334`）可改造成“A 被 B 接管”标记，向正在进行的 A 模型调用发取消（避免 A 的长任务浪费），与现有 `WithProviderCancellationKey`（`mutations.go:608`）一致。
   > 替代设计：仲裁发生在 A **生成完毕并已冻结**之后，不存在「正在进行的 A」，因此**不新增取消语义**（保留既有 `cognitionFactSuperseded` / `superseded_by_newer_turn` 用于真正的并发 turn 抢占）。
5. **版本门控**：B 的 decision 仍须过 `requireCognitionAuthorityRevisionsTx`（`:1123`）与 `validateFrozenDecisionInfluences`（`:719`），复用现有并发/一致性防线。
6. **拒绝 A 副作用的具体拦点**（物理上必须保证 A 不经过）：
   - 根级：18b `applyPersonalityDecisionPlanTx`（`personality_runtime.go:227`）、18c `applyFrozenCognitiveStagesTx`（`cognition_growth.go:223`）、18g 内 `recordRelationshipInteractionTx`（`cognition.go:665`）——只要 payload 已被 B 覆盖，这些函数读到的就是 B 的 `decision`/`personality_transition`，A 的提案不会落库。
   - 能力：18e `settleDeferredCapabilitiesTx`（`capability_runtime.go:366`）只执行 payload 里的 `capability_invocations`，覆盖后即只执行 B 的。
   - 私聊消息：18d `conversation_messages` 写入用 `visible`，覆盖后写的是 B 的文本。
   - 结论：**因为整个结算事务以 frozen payload 为唯一事实源，覆盖 payload 即“拒绝 A 全部副作用”，无需逐点打补丁。**

### 注意的约束（读到）

- `PersistTurnDecision` 的 `personalityDecisionPlanFromValue` 校验（`cognition.go:406-411`）与 `validateFrozenDecisionInfluences`（`:415`）会在 B 覆盖后再次执行，B 的 payload 必须满足同样的格式约束。
- 若 Judge 需要 “B 额外发一次主 LLM”，应放在步骤 11 之后、12 之前（事务外、便宜），且复用 `assembleProjectionPrompt`（`mutations.go:763`）以减少重复代码。
- **[superseded — 见文首标注]** QUERY continuation 特例（第 5 节）下，B 接管需同时覆盖 `query_continuation` 子状态或将其置空重算，避免 A 的 `VisibleText` 残留。
  > 替代设计：`responseMode == "query_continuation"` 时**整个跳过仲裁块**（QUERY continuation 与 takeover 互斥，`design.md` §4.7/F06），因此不需要覆盖 continuation 子状态。

---

## 附：关键事实速查

- 唯一 DM 发送工具：`conversation.reply`（`tool_contract.go:191`），无 `visible_reply` 旁路（grep 无结果）。
- `conversation.reply` 无二次模型改写（`builtin_capabilities.go:120/123` 仅返回 deferred/校验）。
- 冻结状态机仅 `frozen/completed/failed`，**无 rejected/superseded**（`cognition.go:461/638/780`）。
- 抢占在 inbox 层建模：`superseded_by_newer_turn`（`cognition.go:267`）+ `cognitionFactSuperseded`（`cognition.go:334`）。
- 模型调用在事务外；结算事务以 `cognition_frozen_actions.payload` 为唯一事实源。
- 候选冻结后有“未执行”窗口，崩溃由 `recoverFrozenTurnAfterAssistant`（`cognition.go:1262`）重放。
