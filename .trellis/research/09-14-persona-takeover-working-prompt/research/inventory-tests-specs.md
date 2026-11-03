# 测试基础设施 / 规范 / 环境 盘点（Turn Takeover 改造前）

> 范围：测试基础设施、静态守卫、项目规范、可运行性与环境、新增测试落点。
> 全部为只读盘点，未修改任何代码文件。
> 标注说明：`[读]` = 从代码/文档中直接读到；`[推]` = 基于读到内容的合理推断。
> 每条结论尽量带 `文件:行号`；规范文档引章节标题。

> **[2026-09-14 状态标注]** 本文件是**代码事实记录**，层级为 **request > 经评审的 `design.md` > `implement.md` > research 旧建议**：其中的「建议落点」不是架构权威，实施以 `design.md` / `implement.md` 为准。补充事实（R1）：
> - 确定性预算守卫**已存在**且不依赖 live provider：`TestConversationCapabilityCatalogFitsDefaultPromptBudget`（`provider_live_tool_test.go:34-47`）；同文件内 `TestLiveProviderDense*Initialization`（`:17-23`）才需要 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1`（`:443-447`）。两类测试必须分开报告。
> - 多头卡的接管语义断言（`dense_multi_expectations.json` 的 `id=forced`）**只在 live provider 路径校验**，本地为 skip → 报告中标注「未验证」。

---

## 0. 结论速览（最重要）

- **有可用的“Fake Provider”，但它是 HTTP 传输层 fake，不是独立的 Provider 接口替身。** 真实 `ProviderClient`（`app.Provider`，由 `ProviderClient{DB, HTTP}` 构造）在测试中通过注入 `http.Client{Transport: projectHealthRoundTripFunc(...)}` 来返回固定 JSON 响应，从而不开真实 LLM 跑完整链路（`conversation_delivery_regression_test.go:34-64` 等）。
- **没有 Fake Judge、没有可控 Clock、没有统一的 `newTestApp`/`testApp` 封装。** 全仓库搜索 `Judge`/`FakeJudge` 仅有 media 提示词里一句 “Do not judge beauty”（无关）；`nowFn`/`Clock`/`FixedNow` 等关键词无匹配，`time.Now()` 在生产与测试中直接调用。
- **最接近“Spy Executor”的，是几个测试用 capability 类型**（`nativeReplayTestCapability` 等带 `atomic.Int32` 计数器），不是通用 spy 框架。
- **引入 Turn Takeover 必须同步改动的静态守卫集中在两处**：`capability_core_test.go:1026`（`TestCapabilityRuntimeStaticGuardsPreserveActionSingleCognitionAndGenericQueryContinuation`，对 `mutations.go` 中 `StructuredAssembledWithToolsSchema(` 与 `StructuredQueryContinuation(` 的调用次数做精确计数）与 `project_health_architecture_guard_test.go`（role=tool / 遗留符号扫描）。这两个守卫的“单次认知”“最多两次主生成”“查询续调用与接管互斥”边界会直接被新契约打破。

---

## 1. 测试基础设施清单（Fake / Stub / Spy / Clock）

### 1.1 隔离 Postgres 仓储（实质上的“集成测试基座”）
- **`isolatedCoreTestRepository(t)`** — `project_health_transaction_integration_test.go:126-179`
  - 用法：`ctx, repository := isolatedCoreTestRepository(t)`。
  - 行为：读环境变量 `GO_CORE_TEST_DATABASE_URL`；为空则 `t.Skip`。用 `pgxpool` 以 `postgres` 库创建临时数据库 `lac_core_<digest>`，跑 `migrations.New(pool).Apply(ctx)`，返回 `*PostgresRepository`。
  - 覆盖：任何需要真实数据库（对话、认知 inbox、人格运行时、能力事务、状态修订）的测试。
  - 不覆盖：没有该环境变量时整套 DB 相关测试直接 skip，等于在 CI 默认环境下不执行。
  - 配套：`seedLifeContextFluctlight(...)` (`life_context_test.go:19`) 注入最小 Fluctlight 行。

### 1.2 HTTP 传输层 Fake（实质上的“Fake Provider”）
- **`projectHealthRoundTripFunc`** — `project_health_transaction_integration_test.go:37-41`
  - 类型：`type projectHealthRoundTripFunc func(*http.Request) (*http.Response, error)`，实现 `RoundTrip`。
  - 用法：在 `http.Client{Transport: projectHealthRoundTripFunc(func(req){...})}` 中返回手写 `map[string]any` 的 `choices[].message.content`（结构化 JSON 或 tool_calls）。
  - 覆盖：Provider 的 `/chat/completions` 网络边界（结构化解析、tool_calls 解析、超时/取消语义）。用例见 `conversation_delivery_regression_test.go:34-64, 123-233, 235-298`（直接聊天链路 + 投递顺序 + reply tool）。
  - 不覆盖：真实的 token 计费、真实模型行为、真实流式 chunk；它只校验“请求确实发出 / 响应被解析”，语义由固定 JSON 决定。
- **`embeddingHTTPResponse(...)`** — `memory_lifecycle_test.go:621`：把字符串包成 `*http.Response`（供 embedding/provider 传输测试复用）。
- **真实客户端挂载点**：`app.Provider = &ProviderClient{DB: repository, HTTP: providerHTTP}`（`conversation_delivery_regression_test.go:57, 146, 256`）。即“Fake Provider”= 真实 `ProviderClient` + 假 `HTTP.Transport`，没有独立的 Provider 接口替身类型。

### 1.3 真实 Provider 的“live”路径（需外部密钥，默认 skip）
- **`liveProviderConfig(t)`** — `provider_live_tool_test.go:443-454`：要求 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1` + `FLUCTLIGHT_LIVE_PROVIDER_URL` + `FLUCTLIGHT_LIVE_PROVIDER_MODEL`，否则 `t.Skip`。
- **`liveProviderMessage` / `privateLiveProviderMessage`** — `provider_live_tool_test.go:456, 123`：直接打真实 endpoint（3 分钟超时）。初始化语义覆盖率测试（`TestLiveProviderDenseSingleInitialization` 等）走这条路径。

### 1.4 Spy 性质的 Executor（测试用 capability 类型）
- **`nativeReplayTestCapability`** — `project_health_transaction_integration_test.go:43-124`：带 `prepareCalls/txCalls atomic.Int32`、`failFirst atomic.Bool`，用于断言“事务性能力在 caller-owned 事务内执行、且只执行一次、可重试一次”。这是最接近“Spy Executor”的设施。
- **`autonomyMutationTestCapability` / `autonomyFailureTestCapability`** — `project_health_transaction_integration_test.go:49-87`：在 caller 事务内写入 `runtime_settings` 或故意失败，用于自治能力事务边界测试。
- **`testCapability`** — `tool_contract_test.go:10-47`：一个最小 query 能力，用于 normalize/registry 单测。
- 注意：以上都**不是通用 spy 框架**，是具体 domain 能力的“可计数/可失败”替身；没有 `SpyExecutor`/`spyExecutor` 统一定义（已搜索确认）。

### 1.5 可控时钟（Clock）
- **不存在。** `[读]` 搜索 `nowFn|NowFn|Clock|FixedNow|func timeNow|fixedClock` 在 `apps/core-go`（含 `internal/core`）无匹配。`[读]` 生产代码直接用 `time.Now().UTC()`（如 `app.go:1421`、`app.go:1122` 附近默认填充；`conversation_delivery_regression_test.go:332` 用 `time.Now().UTC()` 作结算时间）。`[读]` `go.mod` 仅把 `github.com/facebookgo/clock` 列为 **indirect**（Temporal 传递依赖），未被任何测试用作受控时钟。
- `[推]` 这意味着“冻结候选时间/接管预算/超时”相关测试若想确定性，需要新建 Clock 抽象或注入 `now func()`；目前没有复用点。

### 1.6 统一测试 App 构造器
- **不存在** `newTestApp`/`testApp`/`newTestRuntime` 命名 helper。`[读]` 各测试手动 `app := &App{DB: repository}` 然后分别挂载 `app.Provider` / `app.ContextResolver = NewAppContextResolver(app)` / `app.Capabilities = app.capabilityRegistry()` / `app.Runtime = NewCapabilityRuntime(...)`（如 `conversation_delivery_regression_test.go:36-64`）。`[推]` 新增 takeover 的“确定性 Runtime 测试”建议在此处补一个 `newTestApp(t, repository, fakeHTTP)` 收敛构造逻辑。

### 1.7 Fixture / 数据
- **`testdata/`** — `provider_live_tool_test.go:18,21` 用 `testdata/initialization/dense_single_card.txt` 与 `..._expectations.json`（仅 live 路径）。
- **内联 fixture**：`conversation_tool_response_fixture_test.go:14-31` 用一段真实 cognition 响应（affect/memory/image 但无 reply）做边界 fixture。
- **`ToolCallV1` fixture 结构** — `capability_codec_fixture_test.go:8`：承载 tool_calls 编解码 fixture。
- 通用 helper：`mustCapabilityRegistry(...)`、`NewStaticContextResolver(map[ContextSlot]ContextLoader{...})`、`NewCapabilityRuntime(registry, resolver)`、`conversationReplyCapabilityDefinition()`、`cognitiveTurnResponseSchema()`、`DefaultPromptBudgetPolicy(...)`、`AssemblePromptContext(...)`、`EstimatePromptTokens(...)`、`ResolveWorkingMemory(...)`（均被多个 *_test.go 复用）。

---

## 2. 现有相关测试清单（按主题分组）

> 标注：`[契约级]` = 断言精确字段/枚举/计数/边界；`[松散]` = 仅断言关键词出现/对象非空/日志包含某串。

### 2.1 Prompt 组装 / 布局 / 预算
- `TestPromptAssemblerBuildsBLayoutAndCurrentInputExactlyOnce` — `prompt_context_assembler_test.go:23` `[契约级]`：断言 `AssemblePromptContext` 产出 5 条消息且顺序 `system/user(RUNTIME CONTEXT)/user/assistant/user`，`current` 仅出现 1 次且为末条；`EstimatedInputTokens <= MaxInputTokens`，tools=1，responseFormat 非空。**直接覆盖 B-layout 与“当前输入唯一一次”。**
- `TestPromptAssemblerFailsWhenRequiredWireSectionsExceedCaps` — `:59` `[契约级]`：断言 `CurrentInputTokensCap=8` 触发 `ErrPromptRequiredBudgetExceeded`。
- `TestPromptAssemblerTotalCapNeverSplitsRecentTurn` — `:68` `[契约级]`：断言总 cap 下整轮 recent turn 被丢弃（reason=`total_cap`），不半截选取。
- `TestPromptAssemblerPressureDoesNotScaleWithStores` — `:88` `[契约级]`：断言 1000 条 recent + 100 memory + 30 active 下 token 不超 `defaultMaxInputTokens` 且不修改入参切片。
- `TestPromptBudgetConfigurationUsesOutputReserveAndSafetyMargin` — `:119` `[契约级]`：断言 `validatePromptBudgetConfiguration` 容量公式与未知 policy 错误码。
- `TestPromptEstimatorHandlesMultimodalImages` / `TestPromptEstimatorUsesConservativeUTF8Formula` — `:131 / :10` `[契约级]`：token 估算上限。
- `TestConversationCapabilityCatalogFitsDefaultPromptBudget` — `provider_live_tool_test.go:34` `[契约级]`：断言 conversation 目录 tools+schema token 不超 `ToolsSchemaTokensCap` 与 `MaxInputTokens`（与“Prompt 收敛/Working Persona”相关）。
- `TestCompactProviderFactRemovesTransportMetadata`、`TestCompactResponsePlanForProviderRemovesProtocolFields`、`TestCompactCapabilityResultsForProviderKeepsOnlyOutcome` 等 — `provider_context_test.go:443,400,424` `[契约级]`：provider 投影只保留语义层、剔除 DB/传输元数据。
- `TestProviderPromptInstructionsStayCompactAndPreserveContracts` — `provider_prompts_test.go:8` `[契约级/关键词]`：断言每条提示词含 `must` 关键词列表（如 media-quality 必须含 “Do not judge beauty”）。
- `TestQuotedHistoricalInstructionCannotBecomeSystemRule` — `provider_context_test.go:637` `[契约级]`：被引用的历史指令不得变成系统规则（与“Prompt 收敛/防注入”相关）。

### 2.2 人格 / Overlay / active_profile_id
- `personality_runtime_test.go`：`TestInitialPersonalityProfileIDUsesDeclaredActiveProfile` `:5`、`TestInitializationDefaultProfileRemainsValidForMultiProfileSeeds` `:34` `[契约级]`：断言 `active_profile_id` 取值与回退规则（default 为虚拟未选 profile）。
- `evolution_overlay_test.go`：`TestEvolutionOverlayStageS09` `:11`、`TestReflectionOverlayEvidenceCountsHistoricalWindowsNotFacts` `:114` `[契约级]`：overlay 只计历史窗口、不把事实当窗口。
- `provider_context_test.go`：`TestCompactMemoriesUsesActiveProfilePerspectiveWithoutDuplicatingMemory` `:520`、`TestSelectActiveProfileRelationshipsPrefersDominantProfileAndSharedFallback` `:538`、`TestProviderMetadataKeepsPersonalityProfileIdentifiers` `:563`、`TestProviderMetadataStripsRawActiveMemoryIdentifiers` `:575` `[契约级]`：**均围绕 active_profile 视角与 persona 标识符**——新增 `reply_owner_profile_id` 后这些投影测试是必须回归的点。
- `initialization_contract_test.go`：`TestInitializationCompletesMissingProfileFieldsButPreservesProfileIdentity` `:486`、`TestInitializationModeMatchesDeclaredProfileCardinality` `:510`、`TestInitializationPreservesDenseCharacterCardSemanticOwners` `:542`、`TestNormalizeInitializationProfilesKeepsKnownFieldsAndMovesUnknowns` `:628` `[契约级]`：profile 身份不可被归一化抹掉。

### 2.3 Capability 执行与事务 / 单次认知 / 调用次数
- `capability_transaction_integration_test.go`：`TestTransactionalMemoryAndAffectRollbackWithCallerOwnedPostgresTransaction` `:12` `[契约级, 需 DB]`：断言内存/情感事件在 caller 事务内、rollback 后不落库。
- `project_health_transaction_integration_test.go`：`TestStructuredTurnSettlementRollsBackStateAssistantAndOutcomeTogether` `:316`、`TestProjectHealthNativeMutationsUseCallerOwnedTransaction` `:464`、`TestNativeCognitionRequiredFailureReplaysFrozenDecisionWithoutSecondProviderCall` `:602` **[关键]`[契约级]`：断言“原生认知失败后重放冻结决策，**不再发起第二次 Provider 调用**”（直接定义“单次认知、不二次生成”的契约）；`TestAffectEventSerializesIdempotencySourceAndFrozenProfile` `:812`、`TestAppraisalAndAffectEventCommitOneRevisionVisibleToNextProjection` `:941`、`TestNativeCognitionCycleGuardBoundsCapabilityProducedLifeFacts` `:1096`、`TestAutonomyCapabilityActionRollsBackTransactionalSiblingOnRequiredFailure` `:1145`。
- `capability_core_test.go`：`TestCapabilityRuntimeResolvesDeclaredContextAndExecutesOnce` `:273`、`TestInteractiveCapabilityPlanDefersNativeMutationUntilCallerTransaction` `:685`、`TestRuntimeRequiresFrozenCallerTransactionForTransactionalCapabilities` `:720`、`TestRequiredCapabilityFailureFailsClosedForMissingOrDeferredResult` `:782`、`TestProviderCannotSmuggleCapabilityPreparedDataThroughArguments` `:876`、`TestPreparedPayloadEnvelopeRejectsUnknownFieldsAndWrongVersion` `:890`、`TestCompositeActionPersistsOnlyCapabilityCallIDs` `:937`、`TestResponsePlanSchemaHasNoNestedCapabilitySidecar` `:926`、`TestBuiltinRegistryContainsExactlyThirteenDirectCapabilities` `:586` **[关键]`[契约级]`：能力目录数量与内容固定。
- **`TestCapabilityRuntimeStaticGuardsPreserveActionSingleCognitionAndGenericQueryContinuation`** — `capability_core_test.go:1026-1047` `[静态守卫, 见 §3]`。

### 2.4 投递 / 帧顺序 / reply
- `conversation_delivery_regression_test.go`：
  - `TestDirectConversationMessageIsDurableDuringCognitionAndNoReplyCannotComplete` `:14` **[关键]`[契约级, 需 DB]`：断言在 Provider 返回前 user 消息 / workflow intent / outbox 已原子提交；当认知无可见文本（`cognition_visible_text_missing`）时不提交 assistant、inbox 保持 pending、frozen 不 completed。
  - `TestDirectConversationStreamsCommittedUserBeforeProviderAndAssistantAfterCommit` `:123` `[契约级]`：断言帧顺序 `user → token → assistant`，且 assistant 在 Provider 完成后才落库。
  - `TestDirectConversationReplyToolWithoutAppraisalCommitsBothMessages` `:235` `[契约级]`：通过 `conversation.reply` tool 回复成功提交 user+assistant，且不产生 state revision。
  - `TestSupersededConversationTurnCannotCommitLateAssistantOrReviveInbox` `:300` `[契约级]`：被新 turn 取代的旧 cognition 不能再提交迟到 assistant 或复活 inbox。
  - `TestDirectConversationSourceFactConflictRollsBackNewUserMessage` `:357` `[契约级]`：源事实冲突回滚。

### 2.5 Query continuation（与 takeover 互斥）
- `query_continuation_test.go`：
  - `TestQueryContinuationAcceptsOnlyOneOrTwoGenericPureQueries` `:8` `[契约级]`：`validatePureQueryContinuation` 只接受 1–2 个纯 QUERY（memory.recall/relationship.lookup），拒绝 0/3/动作/混合。
  - `TestNativeToolOnlyFallbackNormalizesOnlyPureQueriesToContinuation` `:26` `[契约级]`：`normalizeConversationResponseMode` 的 fallback 规则矩阵（本任务“QUERY 与 takeover 互斥”的核心对照）。
  - `TestQueryContinuationStateAndToolMessagesAreReplayStable` `:55`、`TestQueryContinuationMessagesAllowBLayoutAssistantHistory` `:87` `[契约级]`：续调用消息结构可重放、不泄漏 visibility/evidence_refs、复用 B-layout 历史。
  - `TestQueryContinuationResponseSchemaIsVisibleTextOnlyAndResponseModeRequired` `:113` `[契约级]`：`query_continuation_response_schema` 仅含 `visible_text`；主 schema 的 `response_mode` 枚举只有 `final|query_continuation`。
  - `TestContinuationVisibleTextRejectsMutationFields` `:129` `[契约级]`：续调用可见文本不得携带 mutation 字段。
- `conversation_tool_response_fixture_test.go:13` `[契约级]`：`response_mode=final` 无 reply 但带其他 tool 时仍要求同一 Main 认知产出可见回复，否则 `cognition_visible_text_missing`；`action_type` 枚举为 `[reply]`。

### 2.6 架构守卫（详见 §3）
- `project_health_architecture_guard_test.go` 全文件（7 个守卫）。
- `capability_core_test.go:1026` 上文已列。
- `capability_core_test.go:18` `TestCapabilityArchitectureGuardHasNoLegacyRuntimeSymbols`、`TestCapabilityRuntimeLogDoesNotRecordIntentPlaintext` `:406`。

### 2.7 诊断（与 Judge 计费/预算相关）
- `diagnostics_test.go`：`TestProviderUsageAndWireBudgetDiagnosticsNormalizeActuals` `:45`、`TestModelRunPersistenceDoesNotReturnGhostID` `:107`、`TestProviderAttemptIdentityIsStableWithinInvocationAndDistinctAcrossRetries` `:140`、`TestProviderModelRunLifecycleCannotRegressFromTerminalToQueued` `:208`、`TestPostgresProviderTimeoutPersistsOneTypedTerminalState` `:273`、`TestProviderProvenanceRoleParameterHasOnePostgresType` `:309` `[契约级]`：**这些是 Judge 单独计费/identity/诊断落库的直接复用点**——新增 Judge 调用应新增 role 或复用 provenance role 分类，并扩展这些测试。

---

## 3. 静态守卫清单（引入新契约时必须同步更新）

> 所有守卫位于 `package core` 生产代码扫描（`walkProductionGo` 排除 `_test.go` 与 `migrations`）。

1. **`TestCapabilityRuntimeStaticGuardsPreserveActionSingleCognitionAndGenericQueryContinuation`** — `capability_core_test.go:1026-1047`
   - 断言内容（**[读]** 逐字）：
     - `mutations.go` 中 `StructuredAssembledWithToolsSchema(` 恰好 **1** 次（= 一次 Main 生成）；`StructuredQueryContinuation(` 恰好 **2** 次（= 查询续调用站点数）。
     - 不得含 `invocation.CapabilityName ==`；不得含 `toolOnlyCognitionAppraisal`；必须含 `decision["cognitive_state_transition"] = "not_proposed"`。
     - `mutations.go / wakeup.go / workflow_ops.go / capability_runtime.go` 中不得含 `switch invocation.CapabilityName` / `switch call.Name`。
   - **引入 takeover 必须改**：新增 Judge（独立 Provider 调用）+ B 主生成会使 `StructuredAssembledWithToolsSchema(` 计数变成 2，`StructuredQueryContinuation(` 计数也可能变；A 被拒时根级状态提案逻辑需保留 `not_proposed`。需把“Judge 单独计、主生成至多两次、query/takeover 互斥”编码进此守卫或新增守卫。**

2. **`TestProjectHealthArchitectureGuardAllowsToolRoleOnlyInQueryContinuation`** — `project_health_architecture_guard_test.go:21-42`
   - 断言：生产代码中 `role: "tool"`（不区分大小写）**只允许**出现在 `query_continuation.go`；并扫描 `mutations.go`/`composite_actions.go` 不得含 `decision["effects"]`、`decision["decision"]`、`mapValue(decision["action"])`、`resolveDecisionAction`。
   - **引入 takeover 必须改**：若新握手把 Judge 判定结果以 `role`/消息形式注入主 prompt，需确保不违反 “tool role 仅 query_continuation” 的扫描，或在白名单里显式新增位置。

3. **`TestProjectHealthArchitectureGuardHasNoReflectionV1ProductionSurface`** — `project_health_architecture_guard_test.go:12-19`：禁止遗留 Reflection v1 符号（正则）。`[推]` takeover 不引入 Reflection，通常不受影响，但新增文件若误用这些符号会被拦。

4. **`TestProjectHealthArchitectureGuardHasNoScheduleSemanticSplitter`** — `:44-54`：禁止 Go 侧 schedule 语义启发式。与 takeover 无关但属同批守卫。

5. **`TestExecutableReplayRejectsMissingWrongOrDualAuthorityVersion`** — `:56-77`：可执行能力 payload 的版本/字段权威校验（拒绝缺 version、错误版本、带 `tool_calls`、嵌套 `decision.tool_calls`、`capability_invocations` 非数组）。`[推]` 若 takeover 改动 frozen decision 结构（如新增 `reply_owner_profile_id`），需确认不破坏此 replay 校验，必要时同步。

6. **`TestActiveFrozenReplayRequiresCurrentContextReferenceAuthority`** — `:79-89`：冻结 replay 必须有 `context_reference` 权威；否则 `frozen_context_reference_version_invalid`。`[推]` takeover 不改此权威，但 B 接管后 persona 投影若改变需保证 reference 版本一致。

7. **`TestAffectStaticGuardPreservesBipolarAndUnitClampOwnership`** — `:91-105`：affect reducer 的 bipolar/unit clamp 归属。与 takeover 无关。

8. **`TestCapabilityArchitectureGuardHasNoLegacyRuntimeSymbols`**（`capability_core_test.go:18`）、**`TestCapabilityRuntimeLogDoesNotRecordIntentPlaintext`**（`:406`）、**`TestBuiltinRegistryContainsExactlyThirteenDirectCapabilities`**（`:586`）：能力目录/日志/遗留符号的契约守卫。`[推]` takeover 若新增 capability（例如 Judge 不新增 capability，仅为 provider 调用）通常不碰；但若引入新的 direct capability 数量会变，需更新 13 的计数。

---

## 4. 相关规范硬约束摘要（逐份，引原文/章节）

> 这些文档**尚未**包含 takeover 条款（takeover 是新任务，见 `prd.md`/`request.md`）。下方为 takeover 直接相关的现有硬约束，新增契约时必须与之对齐且不得破坏。

### 4.1 单次认知 / 禁止第三方生成 / 调用次数上限
- **`fluctlight-cognitive-runtime.md` §5 Good/Base/Bad Cases（:228-234）**：`[读]` “Base: Main needs one read-only recall result, returns no text plus one pure QUERY, and the bounded continuation produces only the final answer; **no state or Capability mutation schema is available in the second call**.” 以及 Bad 条：“use `memory.recall` name matching instead of execution metadata, permit ACTION ToolResults to continue, **or call a third model turn**.” —— 明确**禁止第三次模型轮次**。
- **`fluctlight-cognitive-runtime.md` §3 Contracts / §6 Tests Required（:244-252）**：`[读]` “Final/ACTION/mixed tests assert one `conversation_turn_response`, **zero `role=tool` messages, zero `action_realization` calls, and frozen retry without another Main request**.” “Query-continuation tests assert only 1–2 generic pure queries qualify; **the second request reuses frozen B-layout history** … and replays … without repeating work.”
- **`structured-turn-contract.md` §2 Signatures（:74-87）**：`[读]` “The only same-turn continuation is generic and result-dependent: `response_mode=query_continuation`, no visible text, one or two metadata-classified pure QUERY invocations … then **at most one no-tools Provider call** whose closed response contains only `visible_text`.” —— **主生成至多两次（A + 续调用 或 A + takeover B）**，与 `request.md:R06` 一致。
- `[推]` Turn Takeover 的 “主生成/合成合计最多两次、Judge 单独统计、QUERY 与 takeover 互斥” 实际是把上述“单次认知 + 至多一次续调用”约束扩展为“A 主生成 + 可选 Judge + 可选 B 主生成，且不与 query_continuation 叠加”。现有 `cognitiveTurnResponseSchema` 的 `response_mode` 枚举（见 `query_continuation_test.go:113-126`）目前只有 `final|query_continuation`，takeover 不应新增 response_mode，而是 B 以 `final` 重生成。

### 4.2 Prompt 布局 / Working Persona 收敛
- **`structured-turn-contract.md` §3 Contracts（:55-67）**：`[读]` “Visible text may stream from provider `content`; structured controls are accumulated and validated before any side effect is applied.” “The model-facing system prompt contains only short behavioral guidance; it **must not duplicate JSON schema bounds, dispatcher internals, or legacy marker syntax**.”
- **`request.md`（任务契约, :208）**：`[读]` “A 的正常 Main Prompt 只包含必要 Shared Identity 与 Working Persona A；**不能同时包含完整 B、完整 profiles 数组、全部接管规则和另一份冲突 effective_persona。B 只有实际生成时才加载其 Working Persona**。” —— 这是“Working Persona 收敛”的硬要求，**与 `provider_prompts_test.go:8` / `TestCompactProviderFactRemovesTransportMetadata` 等 prompt 边界测试互为印证**。
- `fluctlight-cognitive-runtime.md` §Foundation Expression Context（:89-107）：frozen `persona_profile` 随 CognitionFact 与 FrozenAction 走，realization 只读冻结副本，不复读可变人格状态——takeover 的 B 生成必须同样基于冻结投影。

### 4.3 Persona 层职责 / active_profile_id
- **`persona-layer-contract.md` §3 Contracts（:39-40）**：`[读]` “`core_persona.personality_system` is the explicit multi-personality contract … `fluctlight_personality_runtime.active_profile_id` is the current dominant profile.” “Personality switching is a cognition decision (`keep|switch`) … **Realization receives the frozen profile and cannot select another profile**.”
- **`persona-layer-contract.md`（:150）**：`[读]` Provider 序列化把语义 Core Persona 组 + 活动 `personality_system` profile 抬进系统 `# 人格设定` 段；不暴露持久化 ID/修订/时间戳，但保留语义 profile 标识。
- `[推/读]` `request.md:R04`（:22）要求显式区分持久 `active_profile_id` 与当前 turn 的 `reply_owner_profile_id`，且 takeover 后 `active` 不变、消息归属 B、下一轮仍按 persistent profile 构建——这会在 `personality_runtime.go`（active_profile_id 持久化路径 :236-253）、`provider_context.go`（active 投影 :183-299, :538-550）、`provider_context_test.go`（:520, :538, :563）处新增回归点。

### 4.4 Provider 边界 / 模型角色
- **`fluctlight-provider-contract.md`**：端点/模型角色（endpoint/model_roles）、capability preflight、structured/stream/embedding、预算、provenance、失败边界。Judge 作为“额外模型调用、独立计时计费”（`request.md:R05, :24`）需在 provider 角色/预算/provenance 体系中落位；现有 `diagnostics_test.go`（§2.7）已对 provenance role、attempt identity、model run lifecycle 有契约级断言，`request.md:R05` 的“有限控制投影、数据隔离、明确预算与异常降级”应复用/扩展这些。
- **`structured-turn-contract.md` §3（:63-67）**：`[读]` “Native tool calls and parsed provider sidecars normalize into the same `CapabilityInvocation`; **no active marker adapter or second execution path is advertised**.” —— 接管判定结果若经由 provider 返回，须符合“单一执行路径”约束。

### 4.5 诊断 / 隔离失败
- **`fluctlight-diagnostics-contract.md`（:51, :61, :105, :130, :217）**：`[读]` “Prompt diagnostic arrays are capped at 64 entries and recursively …”；`[读]` “Diagnostic sink errors cannot recursively emit into the same sink … non-recursion, and no business rollback.” `[推]` Judge 的诊断/预算落库必须遵守“不递归写、不业务回滚、bounded”原则，可复用 `diagnostics_test.go` 的 persist 测试。

### 4.6 禁止递归 / 不虚构
- `fluctlight-cognitive-runtime.md` §6 Tests Required（:241）：`[读]` “Negative architecture tests scan Go Core and Go BFF production paths for newly introduced semantic regex/keyword dictionaries and require explicit review for any natural-language matching.” —— 接管条件若用规则而非关键词匹配，需避免触发此项；接管规则来自角色卡/授权配置，不得默认 “B 更热情” 等（见 `request.md:123`）。

---

## 5. 构建与测试命令（项目实际使用）

> 来源：`package.json`、` .github/workflows/fluctlight-ci.yml`、`.trellis/workflow.md`（推断/已读）。

- **单元/集成测试（带 race）**：`cd apps/core-go && go test -race ./...`（CI `fluctlight-ci.yml:38`）。gateway-go 同法（`:42`）。
- **Vet**：`go -C apps/core-go vet ./...`（`:46-47`）。
- **Build**：`go -C apps/core-go build ./...`（`:51`）。
- **格式检查**：`test -z "$(find apps/core-go apps/gateway-go -name '*.go' -print0 | xargs -0 gofmt -l)"`（`:55`）→ 需 `gofmt` 干净。
- **前端/全量（workspace）**：`pnpm -r build` / `pnpm -r test` / `pnpm -r typecheck`（`package.json:11-13`）；`pnpm generate` 重新生成 OpenAPI 客户端。
- **验收守卫脚本**：`infra/acceptance/go-core-reference-guard.sh`、`legacy-scope-guard.sh`、`check-compose-bind-sources.sh`、`check-core-openapi.sh`（CI `:58-100`）。
- **无 Makefile**：`[读]` 根目录无 Makefile，命令直接走 `go`/`pnpm`/CI。
- **Compose 冒烟**（需完整外部依赖）：`infra/compose/run-platform-smoke.sh`，env 见 `fluctlight.env.example`（含 POSTGRES_*, REDIS_URL, S3_*, TEMPORAL_*）。仅在 `codex/go-build` 分支 push 时跑（`:127-160`）。

---

## 6. 已知环境阻塞

- **PostgreSQL（必需 for 集成测试）**：`[读]` `isolatedCoreTestRepository` 需要 `GO_CORE_TEST_DATABASE_URL`；未设置时所有 DB 集成测试（含全部 `conversation_delivery_regression_test.go`、`*_transaction_integration_test.go`、`life_context_test.go`、`memory_lifecycle_test.go`、`intention_runtime_test.go`、`reflection_*_test.go` 等）直接 `t.Skip`。CI 的 `go test -race ./...` **不**自动提供该变量，故默认 CI 只跑内存/纯函数单测，DB 路径跳过。
- **真实 Provider（live 测试）**：`[读]` `liveProviderConfig` 需要 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1` + URL + MODEL，否则 skip；默认 CI 不跑。
- **Temporal / Redis / MinIO / S3**：`[读]` `go.mod` 依赖 Temporal SDK、minio-go、go-redis；但 core-go 单测中 Redis/Temporal/MinIO 路径（`redis_triggers_test.go`、`life_context_test.go` 中某些）大多在 `GO_CORE_TEST_DATABASE_URL` 缺失时整体 skip，或仅用 `miniredis`（indirect）做内存替身。`[推]` Redis/Temporal/MinIO 仅在 Compose 冒烟（`run-platform-smoke.sh`）与 worker 进程里真正需要，不是 `go test ./...` 的硬门槛。
- **已知失败**：`[读/推]` 未运行测试，未发现文档化的已知失败。CI 默认跳过 DB/live 路径，因此“默认绿色”不代表集成测试全过；需本地设置 `GO_CORE_TEST_DATABASE_URL` 才能验证 takeover 的链路级测试。
- **`gofmt` / vet**：新增文件若未格式化会令 CI `Verify Go formatting` 失败（`:55`），且 `go vet` 为独立门。

---

## 7. 新增测试的落点建议

> 目标：确定性 Runtime 测试（Fake Provider/Judge/Spy Executor）+ Persona/Prompt 契约测试。

### 7.1 确定性 Runtime 测试（不开真实 LLM、不开 DB 也能跑的部分）
- **落点包**：`apps/core-go/internal/core/`（同 `package core`，复用 `isolatedCoreTestRepository`、`projectHealthRoundTripFunc`、`NewCapabilityRuntime`、`NewAppContextResolver`、`mustCapabilityRegistry`）。
- **新增 Fake Judge**：`[推]` 目前无 Judge 设施，建议新增 `fakeJudgeTransport` 或在现有 `projectHealthRoundTripFunc` 模式上扩展：用一个可脚本化的路由，对 `cognitive_assessment`（A 主生成）、`judge`（Judge 角色）、B 主生成分别返回固定 JSON。建议把这些 fake 抽成 `_test.go` 里的 `fakeProviderRouter`（按 request role / schema 路由响应），避免散落闭包。
- **新增可控 Clock**：`[推]` 新建 `type nowFunc func() time.Time` 注入点（生产代码目前直接 `time.Now()`），或引进 `github.com/facebookgo/clock` 已在依赖中但未被使用——可借此让“Judge 超时/预算/冻结时间”确定性。
- **收敛 `newTestApp`**：`[推]` 把 `conversation_delivery_regression_test.go:36-64` 的 `&App{DB, Provider, ContextResolver, Capabilities, Runtime}` 构造封装成 `newTestApp(t, repository, httpTransport)`，供 takeover 测试复用。
- **复用 Spy 能力**：直接复用 `nativeReplayTestCapability`（`project_health_transaction_integration_test.go:43`）等已有计数型能力，断言“A 被拒时其根级状态提案（affect/memory/goal）均不执行/不落库（在 DB 集成测试中）”。

### 7.2 Persona / Prompt 契约测试
- **`reply_owner_profile_id` 区分**：`[推]` 在 `personality_runtime_test.go` 与 `provider_context_test.go` 附近新增断言：takeover 后 `fluctlight_personality_runtime.active_profile_id` 不变，但本轮消息/冻结决策的 `reply_owner_profile_id`（或等价字段）= B；下一轮仍用 persistent profile 构建。对照 `persona-layer-contract.md:39-40` 与 `provider_context.go:183-299`。
- **Working Persona 收敛**：`[推]` 扩展 `provider_prompts_test.go:8` 与 `provider_context_test.go:520,538,563`：断言 A 主 prompt 不含完整 B profile / 完整 profiles 数组 / 全部接管规则 / 第二份 effective_persona；B 仅在生成时加载其 Working Persona（对齐 `request.md:208`）。
- **调用次数上限 / 互斥**：`[推]` 在 `capability_core_test.go:1026` 的静态守卫中把“主生成至多两次 + Judge 单独 + query/takeover 互斥”编码进去（见 §3 第 1 条）；并新增一个运行时断言（仿 `TestNativeCognitionRequiredFailureReplaysFrozenDecisionWithoutSecondProviderCall` `:602`）证明“takeover 后 B 若需第三次生成则受控失败、不伪造答案、不反向仲裁”。
- **Judge 计费/诊断隔离**：`[推]` 在 `diagnostics_test.go`（§2.7）新增 Judge role 的 provenance/attempt identity/预算断言，复用既有 provenance role 单 Postgres 类型测试（`:309`）。
- **数据隔离 / 提示注入**：`[推]` 复用 `provider_context_test.go:637` `TestQuotedHistoricalInstructionCannotBecomeSystemRule` 的模式，新增断言 A 未发送内容与命中接管条件不会作为系统规则注入 B 的 prompt（对齐 `request.md:159` “Takeover Context” 边界）。
- **forced_activation 分类**：`[推]` 据 `request.md:R07/:194` 把 `forced_activation` 逐条区分为“确定性持续切换 / 语义持续切换 / 本轮接管”，新增迁移对照测试，确保不自动全转 takeover、不改角色卡持续性语义（相关生产代码在 `app.go:657,874` 的 `forced_activation` 字段校验处）。

### 7.3 不必改动 / 注意
- `[推]` 架构守卫 `TestProjectHealthArchitectureGuardHasNoReflectionV1ProductionSurface`、affect clamp 守卫等与 takeover 无关，但若新增生产文件误用遗留符号会被拦，新文件命名/写法需自检。
- `[推]` `TestBuiltinRegistryContainsExactlyThirteenDirectCapabilities`（`capability_core_test.go:586`）仅在真正新增 direct capability 时才需更新；Judge 作为 provider 调用而非 capability，不应触发。

---

## 附：搜索过但“不存在”的设施（明确记录）
- Fake Judge / `FakeJudge` / `fakeJudge` / `judgeProvider`：全仓库无匹配（`[读]` grep 仅命中无关 media 文案）。
- 可控时钟：`nowFn`/`NowFn`/`Clock`/`FixedNow`/`fixedClock`/`func timeNow` 在 `apps/core-go` 无测试用匹配（`[读]`）。
- 统一测试 App 构造器：`newTestApp`/`testApp`/`newTestRuntime` 无匹配（`[读]`）。
- 通用 Spy 框架：`SpyExecutor`/`spyExecutor` 无匹配（`[读]`）；仅存在 domain 级计数能力替身。
- 真实 Provider 的默认离线替身：`[读]` 没有 `interface Provider` 的独立 fake 实现；fake 仅在 HTTP transport 层。
