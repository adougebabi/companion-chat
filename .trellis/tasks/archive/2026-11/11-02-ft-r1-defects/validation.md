# D1–D6 修复与验收记录

## 范围和执行约束
依据 `/workspace/ft-runs/20261102-r1/REPORT.md` 与 `logs/`。变更留在工作树；不执行 Git 提交、索引写入或钩子安装，不修改任何 `.env` 或图片，不访问 `/workspace/accept`。

## 按缺陷的根因、改动和测试

### D1：人格失败与聊天恢复
根因：编译 Activity 重试耗尽后已有失败账本，但聊天入口和已受理消息的观察流没有检查该准备状态，始终等待调度屏障。浏览器内部管道还会把入口错误转换成流协议错误。
修复：准备中/失败均在接收前 HTTP 409 返回固定中文说明和 `persona_preparation_pending` / `persona_preparation_failed`；不持久化未受理消息。已受理但未开始的消息持久化明确失败并在观察流立即返回，保留原身份恢复。保留既有人格编译三次重试。聊天页提供原恢复接口入口，保留草稿和请求身份，跨页面/重挂载后只读恢复受理结果。
主要文件：`apps/core-go/internal/core/{persona_preparation,agent_result_adapter,durable_turn,mutations}.go`；`internal/httpapi/{browser_backend,server}.go`、`internal/httpapi/browser/{routes,ndjson}.go`；`apps/web/src/react-app/pages/ChatPage.tsx`、`shared/PersonaPreparationFeedback.tsx`、`state/conversation.ts`、`v3/{chat-feedback,domain-presentation}.mjs`。
测试：`TestPostgresPersonaPreparationRejectsChatBeforeAcceptanceAndRecovers`；`TestPostgresPersonaFailureClosesAlreadyAcceptedObserverAndHistory`；`TestPersonaPreparationWorkflowExhaustsThreeAttemptsAsExplicitFailure`；`TestPersonaAdmissionFailureRemainsHTTPConflictWithoutStartingStream`；`TestPersonaChatErrorsKeepReadableReasonWithoutCompilerDetails`；`TestPersonaPreparationErrorsCrossBrowserHTTPAndNDJSONWithRecoveryCopy`；`apps/web/test/ft-r1-defects.test.mjs` 中五项 D1 DOM/请求回归。

### D2：长任务阻塞聊天及并行一致性
根因：图片/换装 Owner 请求与私聊共享逻辑锁、认知前序检查和 interaction Worker 容量。
修复：Owner 请求采用独立有序通道，新执行使用 lifecycle Worker，保留旧队列注册和 Temporal 历史兼容。Tool 与最终提交检查原 inbox claim，保留原始目标、事务锁及 CAS。反思扫描停在最早未完成事实之前，长任务结算后安排新反思，避免并行完成的水位越过未完成证据。
主要文件：`core/{cognition,owner_request_claim,owner_life_requests,tool_execution_ledger,workflow_ops,reflection_sources}.go`；`workflow/{cognition_lanes,workflow}.go`。
测试：`TestPostgresOwnerRequestLaneAllowsChatDuringSlowImageAndOutfitAssessment`（含图片、换装、同类排序、重复执行与反思证据）；`TestPostgresOwnerRequestLaneFencesStaleClaimBeforeToolAndSettlement`；`TestPostgresOwnerRequestLaneKeepsOriginalCASAfterConcurrentWardrobeChange`；`TestCognitionDispatchSeparatesOwnerRequestCapacityAndKeepsHistoryRegistrations`。

### D3：Studio 单次超时进入死信
根因：Activity 与外层意图只有一次执行；领域 attempt 已失败，直接增加 Temporal 次数只会立即收到“须显式重试”。
修复：对瞬时超时/连接/限流等失败创建独立、稳定 ID 的后继领域 attempt，每轮最多三次，退避 15 秒、60 秒。原失败记录不重开，复用已验证阶段；重复观察不重复排队，保护已完成/丢弃/已委托媒体任务。手动恢复可开启新一轮。
主要文件：`core/{studio_retry,studio_jobs}.go`、`workflow/workflow.go`。
测试：`TestStudioAutomaticRetryOnlyRetriesTransientFailures`；`TestStudioTimeoutCreatesNewAttemptsWithBackoffThenExhausts`；`TestStudioAutomaticRetryReusesValidatedStageAndPreservesDiscard`；`TestStudioWorkflowLeavesFailedAttemptRecoveryToDomainReconciliation`；既有媒体测试新增保护断言。

### D4：用户名称被模型拆解
根因：模型 foundation 替换了包含用户 name 的 identity。
修复：在编译与持久化前恢复非空用户 name 原文，保留 nickname 等独立字段，幂等重放返回相同结果。
主要文件：`core/app.go`、`core/creation_identity.go`。
测试：`TestCreationIdentityPreservesOwnerNameOverModelInterpretation`；`TestLLMDefinedActivationPersistsVerbatimOwnerNameAndReplays`，含 `FT-青岚`。

### D5：衣柜生命周期诊断无法持久化
根因：中文/模型 reason 文本被用作仅允许机器标识的 ReasonCode。
修复：各生产分支返回固定 reason_code，衣柜 Activity 只投影该机器字段；旧结果使用固定默认值，原中文说明保留。
主要文件：`core/wardrobe_preparation.go`、`workflow/wardrobe.go`。
测试：`TestWardrobeLifecycleUsesMachineReasonAndPreservesHumanExplanation`；`TestWardrobeInactivePreparationPersistsValidNoopDiagnostic`；既有正式衣柜 Agent 测试新增实际结果断言。

### D6：关闭视觉理解的说明及图片文字
根因：视觉理解默认关闭是独立可选配置，但仅暴露机器原因；负向提示未统一禁字。
修复：新旧时间线都提供可读配置/人工评审说明，保留候选未验证事实；前端提供设置和人工评审路径。统一图片提交边界为正向/负向补充禁字、伪字、字母、数字、标题、字幕、签名、徽标与水印约束；保护 Flux 正负共用编码器、原始图结构/图片引用/大整数 seed。同步提示词组成清单。
主要文件：`core/{vision_capability,visual_identity,image_text_policy,media,media_prompt_branches}.go`；`ai/prompt/prompts.go`；`apps/web/src/v3/proto/components/visual-identity-records.js`；`docs/llm-prompt-composition.md`。
测试：`TestVisionDisabledExplainsConfigurationAndPreservesUnverifiedCandidate`；`TestImageSubmissionForbidsTextInPositiveAndNegativeProviderInputs`；`TestImageSubmissionPreservesPositiveTextWhenFluxSharesNegativeConditioning`；`TestImageNegativePromptPlaceholderResolvesWithoutLosingOriginalExclusions`；`TestMediaPromptPhysicalRequestsRequireTextFreeImages`；`D6 disabled vision explains preserved unreviewed candidate and offers actual settings and manual review entries`。

## 旧断言变更：逐项理由
以下针对现行 JSON 提示词和共享世界契约修正过时检查；未降低业务期望。

1. `TestMediaQualityFinalProviderRequestKeepsImageAndReadableText`：原始 diagnostic 字符串查找冻结概念键，改为 JSON 解码 diagnostic 消息并与真实 modelText 完全相等；原图片脱敏检查保留。理由：键存在于 JSON 编码字符串内，原查找没有解码。
2. 同测试：原拒绝 modelText 以 `{` 开头、笼统包含 frozen_media_concept，改为要求有效 JSON、非空冻结概念和 `media_kind=image`；全部隐私、图片原生传输与嵌套转义排除检查保留。理由：当前契约明确要求 JSON。
3. `TestInitializationTextToStoredPortraitToFormalRequest`：旧 TOON `hair_length: short/long` 字符串断言改为实际 Runtime Context JSON 中 `appearance.body_fields.hair_length` 在剪发前精确为 long、之后为 short；旧描述排除及白衬衫检查保留。
4. `TestFormalWakeUpFinalProviderRequestUsesCurrentSharedBodyAndWearing`：旧短发/非长发/白衬衫文本检查改为 JSON 中精确短发，以及唯一实际穿着项为白衬衫。
5. 同 WakeUp 测试：`schedule_ref:`、`wake_up_id:`、`context_revision:` 的旧 TOON 私有字段禁令改为检查对应三个 JSON 键；禁令本身不变。
6. 同 WakeUp 测试：笼统“阅读”/schedule_status 文本检查改为 JSON 中精确 `life_context.activity=阅读`、`trigger.schedule_status=accepted`。
7. `TestScheduleGenerationFinalProviderRequestOmitsStorageIDs`：旧目标/意图文本包含及拒绝 JSON 前缀，改为有效 JSON，精确验证唯一目标/意图、日期和时区；私有字段禁令不变。
8. `TestCompiledWorkingPersonaNativeDetailChain`：全 System 搜索 portrait/source_refs/stable_preferences，改为解析实际人格 JSON，要求恰好 profile_id/portrait_text 两个键及精确值，并在该人格内排除 source_refs/stable_preferences。理由：共享协议合法提到 source_refs，不能等同于人格数据泄漏。
9. `TestWorldNPCPersistenceIsolationEncounterAndSource`：空白激活后的候选列表从“必须为空”改为“恰好只有同 Owner 既有世界且未绑定”，增加实际 WorldSnapshot 为空的检查。理由：共享世界契约允许候选共享，候选不等于新增/绑定。
10. 同世界测试：NPC 读取后的空列表检查同样改为精确既有候选、不新增、不绑定、无实际 world snapshot；继续验证 GET 不产生副作用。

其他旧测试修复均未改变既有断言：两个编译 Provider 假响应迁移至 portrait_text schema；迁移测试把带参数的多语句 Exec 拆开；Life Context/日程夹具从实际 JSON 取权威引用和精确时间；衣柜恢复夹具把三个竞争角色放在同一数据库并增加同库计数断言。

## 数据库、配置与边界
- 无新增数据库迁移或强制配置变更。只在独立临时 PostgreSQL 应用既有迁移用于测试。
- 自动视觉检验需要在产品设置中显式启用视觉理解并配置独立模型；既有跳过评审不会自动重试。
- 已进入 dead letter 的历史 Studio 作业通过既有手动重试入口恢复，不自动重写历史。
- D2 不绕过模型服务共享物理并发配置；若 generated_concurrency=1，正在进行的模型 HTTP 调用仍占唯一槽。并发模型回归使用既有配置值 2 验证独立通道。
- 没有使用真实模型、图片服务或真实 Temporal 历史回放进行在线验收。物理请求回归使用受控 HTTP 服务；禁字提示不能保证外部模型绝不生成字符。未改动 token 上限或正文裁剪。

## 最终检查结果
- `pnpm install --store-dir /tmp/pnpm-store`：通过。使用已有 Node 24.12.0 / pnpm 11.0.7；只把 Corepack 缓存与命令包装放在 `/tmp`，无项目配置修改。
- `pnpm build`：通过，包含产品夹具隔离和 React 生产图检查。
- `pnpm --filter @fluctlight/web lint`：通过。
- `pnpm typecheck`：通过。
- `pnpm test`：883 项通过（web 840、browser-client 40、core-client 3），0 失败、0 跳过。
- `go test -json -timeout 20m ./...`（在 apps/core-go，使用独立 PostgreSQL）：3,428 项测试/子测试通过，47 项既有跳过，0 失败；Core 包耗时 459.472 秒。
- 新增人格/并发/Studio/衣柜路径针对性 `-race`：通过。
- browser `go vet ./...`、所有变更 Go 文件格式与 `git diff --check`：通过。
- 最终日志：`/tmp/ft-r1-{js,build,typecheck,web-lint}-verified.log`、`/tmp/ft-r1-go-verified.jsonl`、`/tmp/ft-r1-test-summary.json`。

Go 既有跳过项逐项记录如下；未新增 skip，未减少断言来绕过失败。

| 测试 | 既有跳过原因 |
| --- | --- |

| `TestTemporalRuntimeRealControlRecovery` | temporal_runtime_integration_test.go:22: task-owned FLUCTLIGHT_TEMPORAL_TEST_ADDRESS is required |
| `TestGoalJointPostgresRedisTemporalWorkerRestart` | goal_joint_test.go:36: task-owned PostgreSQL, Redis and Temporal endpoints are required |
| `TestBackgroundStartupExistingKeyAndMissingKeyReleaseOnce` | background_trigger_integration_test.go:39: GO_CORE_TEST_REDIS_URL is not set |
| `TestBackgroundReflectionLastChatThirtyMinutesAndReplayDoesNotReopen` | background_trigger_integration_test.go:76: GO_CORE_TEST_REDIS_URL is not set |
| `TestBackgroundStartupCannotResurrectWakeWhileCognitionIsPending` | background_trigger_integration_test.go:127: GO_CORE_TEST_REDIS_URL is not set |
| `TestBackgroundNewCognitionCancelsRunningProviderAndRejectsLateWatermark` | background_trigger_integration_test.go:232: GO_CORE_TEST_REDIS_URL is not set |
| `TestDeliveryMemoryOwnerCommandFixture` | delivery_memory_live_test.go:15: explicit isolated delivery fixture only |
| `TestFactOnlyActualActorWireReplay` | fact_only_actor_wire_test.go:65: private wire capture is not configured |
| `TestFormalAgentE2E` | formal_agent_e2e_test.go:18: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to run real FormalAgent E2E |
| `TestIndependentToolE2EScheduleReplan` | independent_tools_e2e_test.go:214: FLUCTLIGHT_LIVE_PROVIDER_TEST=1 is required for the final serial schedule.replan live E2E |
| `TestMediaPlanProbe` | media_capture_plan_test.go:473: set FLUCTLIGHT_MEDIA_KEV_URL for real isolated Kev single-point JSON report |
| `TestMediaNormalizationProbe` | media_probe_test.go:22: set FLUCTLIGHT_MEDIA_PROBE=1 for the independent normalization probe |
| `TestLiveStreamTurnFormalAgentNDJSON` | production_stream_live_e2e_test.go:78: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestGoalPromptPrivateSampleReplay` | prompt_composition_contract_test.go:215: private prompt replay not configured |
| `TestLiveHandleTurnUsesRealProviderForPostCognitionPersonalityAssessment` | provider_live_e2e_test.go:144: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveHandleTurnOrdinaryChatHasNoFixedPlanningOrMemoryPass` | provider_live_e2e_test.go:202: live Provider and disposable PostgreSQL are required |
| `TestLiveHandleTurnReadsCompletedHaircutAfterReload` | provider_live_e2e_test.go:247: live Provider and disposable PostgreSQL are required |
| `TestLiveHandleTurnReadsChangedWearingAfterReload` | provider_live_e2e_test.go:311: live Provider and disposable PostgreSQL are required |
| `TestLiveHandleTurnDoesNotTreatUnexecutedWishAsPurchase` | provider_live_e2e_test.go:373: live Provider and disposable PostgreSQL are required |
| `TestLiveHandleTurnRetrievesAndCorrectsNonResidentMemory` | provider_live_e2e_test.go:411: live Provider and disposable PostgreSQL are required |
| `TestLiveHandleTurnRealToolCallsReachDurableMediaAndReply` | provider_live_e2e_test.go:495: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveWakeUpRealToolCallsReachDurableActionAndReflection` | provider_live_e2e_test.go:603: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveRegressionMultiPersonalityDayNightContract` | provider_live_regression_test.go:20: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveRegressionConversationImageAndReplySameTurn` | provider_live_regression_test.go:55: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveRegressionWakeUpProactiveImageAndReplyBundle` | provider_live_regression_test.go:69: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveProviderDenseSingleInitialization` | provider_live_tool_test.go:19: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveProviderDenseMultiInitialization` | provider_live_tool_test.go:23: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveProviderDenseExternalInitialization` | provider_live_tool_test.go:30: external initialization card and expectations paths are required |
| `TestLiveProviderRecognizesImageGenerationIntent` | provider_live_tool_test.go:200: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveProviderRoleOrganization` | provider_live_tool_test.go:290: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveProviderActiveMemory` | provider_live_tool_test.go:335: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveProviderComplexMultiPersonalityInitialization` | provider_live_tool_test.go:374: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLiveProviderPersonalityDecision` | provider_live_tool_test.go:415: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestR4ProvidedDiagnosticsReplay` | r4_output_contract_test.go:97: local r4 diagnostic replay not requested |
| `TestR5ProvidedWardrobeDiagnosticsPreserveEmptyArrays` | r5_wardrobe_contract_test.go:146: local user diagnostic replay not requested |
| `TestR6ProvidedOutfitDiagnosticsExplainZeroNativeTools` | r6_wear_contract_test.go:214: local supplied diagnostic replay not requested |
| `TestR7LiveConversationWear` | r7_live_wear_test.go:65: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestR7LiveOwnerOutfitAlreadyWorn` | r7_live_wear_test.go:216: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestR8LiveSegmentAndMomentRecall` | r8_recovery_test.go:244: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestR8LiveCatalogueShopping` | r8_recovery_test.go:311: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestR9LiveShoppingDurableResolutionAndSelectedWear` | r9_live_test.go:12: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestR9LiveSingleInitializationAndInitialSchedule` | r9_live_test.go:76: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider |
| `TestLivePrivateChatExpressionAblation` | reply_expression_ablation_test.go:168: real model replay not opted in |
| `TestLivePrivateChatMultiRoundAndVoices` | reply_expression_ablation_test.go:214: real model replay not opted in |
| `TestVisualIdentityLiveE2EPreflight` | visual_identity_live_e2e_test.go:44: set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to preflight real Visual Identity E2E |
| `TestVisualIdentityAgentReviewsRealObjectImageAndSavesCanonical` | visual_identity_tools_test.go:688: isolated Visual Identity object-storage test configuration is not set |
| `TestWardrobePreparationPromptPrivateCapture` | wardrobe_preparation_prompt_test.go:164: private wardrobe capture not configured |
