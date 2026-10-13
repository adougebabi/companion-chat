# Reply runtime inventory（只读代码核验）

范围：`apps/core-go` 产品代码与相关测试；未读取 README、CONTEXT、架构或 design 等奠基文档，未运行真实模型请求，未切换分支。以下均为代码事实/接口选项，不替主代理做方案取舍。路径相对于仓库根。

## 1. 私聊入口与最终认知合同

- `apps/core-go/internal/core/durable_turn.go:14` `AcceptTurn`：“commits the user message and cognition intent without running a model in the API process”。调用 `handleTurn(... acceptOnly: true)`；`StreamDurableTurn:27` 只观察已提交状态，断开 HTTP 不取消 Worker/Provider。
- `apps/core-go/internal/core/mutations.go:566` `HandleTurn` 直接调用 `handleTurn`；`:572` `HandleActorTurn` 为 internal/group-chat 入口，因此相同函数不天然保证仅私聊，若模式只覆盖现有私聊必须在适当入口/授权资源类型处限定。
- `apps/core-go/internal/core/cognition.go:35` `ProcessCognitionInbox` claim 已提交 inbox，再通过 HandleTurn 重入同一 turn（消息/inbox 幂等）。
- `apps/core-go/internal/core/agent_result_adapter.go:111` `handleTurn`：用户消息/inbox 在事务中接受；`:228` 取消旧认知事实与抢占 lifecycle；`:287` 调 `RunConversationCognitionAgent`；`:291` 获取 committed trace；`:299` 从 receipt 加载已提交 assistant 消息；`:371` 才结算认知。
- `apps/core-go/internal/core/cognition_agent.go:43` `RunConversationCognitionAgent` 自建 ContextProjection、工具目录与 prompt；`:87` 使用 `cognitiveTurnResponseSchema`；`:107` 在有 conversation 时加入 `conversationPublicationPhysicalRequestPolicy`；`:134` `RunFormalAgent(FormalAgentConversationCognition)`。
- `apps/core-go/internal/core/formal_agents.go:70` ConversationCognition 的 role/scenario 均为 `cognitive_assessment`，强制目录包含 reply/moment/image；`:176` 进入 `RunADKStructuredTask`。当前该 seam 无 Cydonia 专属回复任务/role。
- `apps/core-go/internal/core/provider_schemas.go:217` 最终 schema 必填 `action_type,response_intent,influences`；`:227` 属性包括 response_plan/personality/output_preference/core_alignment/state_expression/claims/appraisal/attention/thought/desire/agency/self_evaluation/evidence_refs 等。`:241` action_type 为 reply。接入回复子任务后仍应完成此原认知合同。
- `apps/core-go/internal/core/conversation_publication_policy.go:19` 在真实 reply receipt 前强制 tool_choice 并省略最终 JSON format；`:26` 纠正轮只允许 conversation.reply；`:33` 已提交 reply 后恢复 allowed，进入最终 DTO。

## 2. 工具立即执行、回填、冻结与事务

- `apps/core-go/internal/core/tool_contract.go:196` reply v1：输入必填 `text`，可选 topic_key/purpose；RequiredContext **只有 CurrentLife**。`:217` 输出 text/target_kind/target_ref/replayed；`:220` SideEffectClass external_async，但 SuccessBoundary 是 `visible_output_committed`、无 CompletionBoundary、SupportsCancel false。`:221` **ModelResultOmitFields 隐藏 text,target_ref,replayed**。
- `apps/core-go/internal/core/builtin_capabilities.go:235` reply implementation；`:241` 普通 Execute 返回 executeToolRequired；`:245` `ExecuteDirectTx` 解析 text，调用 PublicationService，completed 输出实际消息 ref。
- `apps/core-go/internal/core/adk_conversation_runtime.go:96` `ExecuteWithID`：校验真实 native call/provider identity；`:158` operationID 来自 run root + model sequence + tool index + capability（native ID 只诊断关联）；`:180` 仅给所需 slots 的投影快照；`:190` 支持 capability 独立 NativeDefinition/BindNativeArguments；`:263` **直接调用 App.ExecuteTool**；`:289` AppendResult；`:307` serialized business result 返回 Runner。
- `apps/core-go/internal/core/adk_conversation_runtime.go:357` `modelFacingToolResultForContext` 支持 NativeOutput + context refs；`:384` modelFacingToolResult 按 ModelResultOmitFields 精简，因此当前 Qwen 确能得到 completed 状态，但**看不到实际已发送正文**，对“结果回填原认知”要求可能不足。
- `apps/core-go/internal/core/eino_model_runtime.go:930` 调 RunADKLoop；`:943` 无 authoritative reply 则 final contract invalid；`:950` 验证最终 schema，必要时 tool-free repair（已提交 effects 不再执行）；`:992` 明确 final DTO 与 executed calls 分离。
- `apps/core-go/internal/core/agent_result_adapter.go:14` 关键原文：“Tool calls are audit records here: the ADK bridge has already executed and committed them before they entered the trace.” `committedAgentOutcome:24` 核验一调用一结果。`:543` 认知事务应用 FrozenCognitiveStages、claims、assessment/proposal、goal candidates，存 agent_result 与 outcomes，**不重发 reply**。`:314` final 失败发生在已提交 reply 后，保留 visible message 并记录 degraded/failed fact。
- `apps/core-go/internal/core/tool_execution.go:99` `ExecuteTool` 授权资源→resolver→执行分类→读已有 receipt→Prepare→提交 mutation。`:218` 非 pure_query 才读 receipt；`:242` DirectToolCapability 进入 executeToolMutation；`:266` pure query 调 runtime.Execute；transactional/external async 调 runtime.ExecuteTransactional。
- `apps/core-go/internal/core/capability_core.go:53` 分类以 CompletionBoundary 判断 ExternalAsyncIntent；DirectToolCapability 优先 TransactionalMutation；因此 reply 虽标 external_async 仍属于 transaction mutation，不能仅看 SideEffectClass 推断延期。
- `apps/core-go/internal/capability/capability.go:1448` Prepare 在执行事务外，`:1469` 有 ContextSnapshot 时 SnapshotContextResolver 覆盖 resolver；`:1484` 支持 CapabilityPreparer 冻结 PreparedPayload；`:1495` 无 preparer 也补空 frozen payload。
- `apps/core-go/internal/core/app_capability_context.go:21` resolver 仅加载 requested slots、context cancel guards；`:10` 明确“bounded execution snapshot, not authority”；`:109` CurrentLife 走一致 life/schedule snapshot。回复子模型若需要 persona/history/事实/appearance 等，现有 reply 仅 CurrentLife 不够，须新增 capability-owned 输入/context service 或扩充 slots；不能假设工具能看到主 prompt 全文。
- `apps/core-go/internal/core/tool_execution_ledger.go:41` receipt replay key 是 fluctlight+capability+operation，digest 含授权/主体/conversation/target/evidence/profile/**arguments**；不同参数重用 operation 返回 operation_id_conflict。`:72` 业务状态与 receipt 同一 commit；`:77` REPEATABLE READ，lifecycle advisory lock→life lock→operation lock；`:86` WakeUp current guard；`:164` ledger INSERT。
- `apps/core-go/internal/core/tool_publication.go:66` reply publication：owner 校验、life lock/revision 校验、稳定 message/idempotency；`:132` 每 conversation turn 只允许一个 assistant，同文 replay，异文 ErrReplyAlreadyPublished；`:152` autonomy topic+purpose+inbound sequence+时间窗去重，awake guard；`:193` 插入 assistant；`:219` quiet reflection，`:228` runtime summary/segment intent。
- reply publication **没有专属 conversation.reply outbox event**；其 durable message 本身可被 observer 读到，后续 intent/ledger 同事务。Moment 对照有 `tool_publication.go:311` moment.published outbox。因此“发送成功”边界现为产品可见消息提交，不是外部网络 ack。
- `apps/core-go/internal/core/tool_publication.go:256` turn source 仅 pending/claimed 可发布；已 superseded/failed 无法晚发。独立 Cydonia 较慢时必须保持生成后 publication 的该检查，而不是生成前一次检查就当授权。

## 3. 主动私聊与旧冻结链

- `apps/core-go/internal/core/agent_result_adapter.go:712` `ProcessWakeUp`；`:868` wakeUp schema；`:881` FormalAgentWakeUp，`:884` AuthorizationPolicy autonomy + 当前 conversation + wake source/action/operation；工具内即时执行；`:911` normalize/freeze influences；`:917` 实际 action 从 receipt 派生；`:921` cancellation 抢占；`:925` persistCommittedWakeUp。
- `apps/core-go/internal/core/provider_schemas.go:268` WakeUp schema 必填 action_type/response_intent/evidence_refs/influences；response_intent 是内部诊断，静默必须 no_op，不可发给用户。
- `apps/core-go/internal/core/workflow_ops.go:81` 仍有 `processFrozenToolAction`（AutonomyAction/CapabilityAction）；`:121` materialize publication invocation，`:155` 顺序 ExecuteTool，`:184` retryable 时保存 trace，`:200` settle。`cognition.go:463` LoadFrozenTurn/PersistTurnDecision 等也仍存在。**不要把这个兼容/冻结 seam 当当前生产私聊 Main 的必经边界。**
- `apps/core-go/internal/core/composite_actions.go:139` deferred binding；`:189` splitDeferredOutputCapabilities 以 IsDeferredOutput 分组，这是已有 composite seam，不等同于 native reply 延后执行。
- 未在检索的核心 runtime/provider/model-task 文件发现明确名为 legacy/Qwen/Cydonia 的回复策略分支；模式选择位置需主代理结合配置/历史文档定位，不能从本检索宣称整个仓库不存在。

## 4. 最小新接口候选（主代理选择）

1. **reply capability 的 NativeDefinition + BindNativeArguments**（`native_tool_contract.go:13`，ADK bridge `:190`）为 fact-only 原生输入建立单独合同，保留 canonical v1 text 供 legacy/direct caller。注意“将事实生成 text”需要独立模型调用与上下文，这不是同步 schema bind 的自然职责。
2. **CapabilityPreparer / 专属 ReplyGenerationTask service**（runtime Prepare `capability.go:1484`）在事务外生成正文、冻结实际输出，然后复用 ExecuteDirectTx 提交。适合保持 message+receipt 原子；注意 ExecuteTool 在 Prepare 前就做 ledger digest/replay，canonical input 与 prepared generated text 的一致性及生成失败重试必须明确。调用模型不要放在 ExecuteDirectTx 持有 lifecycle/life/operation 锁时。
3. **appADKCapabilityInvoker 的按 request 模式 reply adapter**（`:263`）在调用 ExecuteTool 前执行独立子任务，再把生成后 text 交 canonical publication。保留当前 loop/最终 schema；但 adapter 应承接稳定业务 operation、context cancel/authority guard，避免每次模型重试再造不同正文导致 ledger conflict。更容易按 surface/request 模式隔离 legacy，却可能将能力业务逻辑放在 ADK bridge。
4. 回填正文候选：reply 的 NativeOutput 或新模式专用 ModelResultOmitFields/receipt projection（`:357`/`:384`），canonical receipt 保留 full text。只回填 completed 并不让 Qwen知道独立 Cydonia 说了什么。

## 5. 媒体异步 seams 与限制

- `apps/core-go/internal/core/tool_contract.go:250` image：SuccessBoundary durable_media_intent_created，CompletionBoundary final_media_asset_ready，OutcomeReferenceField media_intent_id，SupportsCancel true；被模型回填的是 accepted/pending，不能声称图已就绪。
- `apps/core-go/internal/core/tool_publication.go:322` ValidateMediaTargetTx；`:377` CreateMediaIntentTx，为 conversation/message/moment 现有 target seam。
- `apps/core-go/internal/core/media.go:36` ProcessMediaIntent 重试/ready asset replay；`:999` publishMediaAsset：message target 更新 attachment_refs；`:1022` conversation target生成独立 media_reference 消息“图片已生成。”，稳定 media intent idempotency；Moment 附件分支也已有。
- `apps/core-go/internal/core/media.go:456` markMediaIntentCompleted →`:501` settleActionOutcomeByExternalRefTx；`action_outcome.go:496` 原子 outcome pending/unknown→terminal；`:592` 创建 processed autonomy.result fact，`:596` reflection intent。
- 这是持久后台结果/反思续接，**未在本主题代码发现恢复同一个 Qwen ADK Runner、携带原对话会话继续原 final cognition 的持久 checkpoint seam**。ADK deferred result 测试仅证明同一进程 Runner 收到 deferred 仍可发下一轮模型，不证明未来 media完成时能恢复原 Runner。
- `agent_result_adapter.go:69` 从 committed image receipts 取 media_intent_id；`:605` 存 agent_result 用于读取与 replay。可作为异步 handoff 的现有身份来源，但当前 media 已完成路径只发布资产和 outcome/reflection。

## 6. 已有回归测试抓手（未执行）

- `eino_adk_runtime_test.go:416` TestRunADKLoopExecutesCapabilityAndFeedsResultBack；`:859` TestProviderADKLoopContinuesAfterDeferredResultsAndPreservesEveryCall；`:642` cancellation 不伪造 final。
- `tool_publication_test.go:52` publishes/replays/payload conflict；`:108` unowned target；`:260` no_op control不发布。
- `conversation_delivery_regression_test.go:17` 消息先 durable、无 reply不能 complete；`:271` reply 后没有 appraisal仍提交消息；`:429` superseded不能晚发。
- `conversation_mixed_media_reply_test.go:48` native image+reply双效果；`:93` stream assistant frame；`:130` reply 后 final contract failure 保留 Provider response。

## 文档一致性提醒

未阅读奠基文档，不能做 docs-vs-code 完整比对；但代码内部 `conversation_runtime.go:68` “trace consumed by existing frozen settlement path” 的历史注释，与当前 `agent_result_adapter.go:14/543` “effects already committed / audit-only trace / final cognition settlement” 用语有潜在歧义。以生产 caller代码为准。当前实际私聊与WakeUp确实都已经在 native loop内提交 reply；不需先修复一个虚构的“全部 action都 deferred”的前提。
