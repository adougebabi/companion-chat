# 2026-10-03 正式入口与缺口定位

## 证据性质
四个只读探索子代理并行定位，主代理抽查了诊断 SQL、前端排序、Clock、投影 Actor、购入提交、摘要选择、检索有效性、物理请求预算与媒体完成快照。未运行业务测试或访问生产服务。以下是代码定位，不是用户故障的全部根因；搜索未命中和历史报告不证明功能不存在/已通过。

## 正式运行与获取使用
- `apps/core-go/internal/core/formal_agents.go:17,61,164`：formalAgentRegistry、RunFormalAgent、RunADKStructuredTask；conversation_cognition / wake_up / schedule_generation / native_cognition / daily_review / reflection / schedule_replan / virtual_activity_result。
- `builtin_capabilities.go:133`（本节未写全路径的文件均在 apps/core-go/internal/core）：intention.inspect/decide、schedule.inspect/edit/replan、intention.schedule、life.activity.start/advance、wardrobe.inspect/wear/outfit.save、media.image.generate 均已注册。
- `intention_capabilities.go:281`：create 同事务登记 Goal/Intention，不要求库存 ID，按 goal/action 复用开放项。`scheduled_activity_capability.go:177`：保存 intention_id 和 action_plan。
- `life_activity_capabilities.go:36`：virtual_shopping 以需求开始，accepted/in_progress；`life_activity_resolution.go:91,251`：elapsed result 后才授予 item。grantVirtualPurchaseItemTx 先校验 completed Event，稳定 ID 由 actor/event/primary 派生，写 wardrobe item 与 revision。
- `wardrobe_capabilities.go:260`：独立 wear 要求本 Actor 已存在 available ID。`life_activity_capabilities_test.go:176`：购买后 worn=0。
- `apps/core-go/internal/migrations/effective_life.go:44`：现有物品聚合为 wardrobe，未定位到正式普通物品使用状态；grant 当前要求 category/slot/description、单 primary，需补非服装和套装成员契约。
- `adk_conversation_runtime.go:221,278`：真实 Tool receipt 立即回填下一原生模型输入。`life_activity_resolution.go:36`：长动作 deferred 保持 pending，并延后 advance；同一 Loop 不虚构未来时间。
- `tool_execution_ledger.go:73`、`tool_authorization.go:13`：变更事务内统一授权和 receipt；当前授权策略未读睡眠。`agent_result_adapter.go:678`、`wakeup.go:622`、`autonomy_policy.go:43`：周期主要判断模式/预算/cooldown/quiet hours；仍需有效睡眠和到期事件先后关系实测。
- `effective_life.go:160`、`builtin_capabilities.go:300`、`media.go:331`：权威 body/worn 快照已有；完成锁 revision 并标 stale。最后可信 workflow 文本/参考图约束及 snapshot 一致性需补验，不可将 stale 标记直接认定为渲染错误。

## 时间、Actor、摘要和预算
- `app.go:55-69`：已有 App.Clock / now；`intelligence.go:173` 的 projectionAt 仍直接 time.Now；schedule_capability.go:244,385 与 life_activity_capabilities.go:126,378,403 等也直接读取壁钟。
- `message_time.go:17-50`：IANA/offset 校验和历史 unknown 已有。`intelligence.go:558-580`、`prompt_context_assembler.go:542-581` 仍使用带空格或 MST 格式；API 多处 RFC3339Nano，不是固定毫秒数字偏移。
- `intelligence.go:500-518`、`provider_context.go:596-610`：Actor 投影仅 ref/id/type/display；`apps/core-go/internal/migrations/runner.go:301` claims 为 content/repetition_key，无属性级 Actor fact 契约。
- `memory_retrieval.go:358-368`：source links 有效性已有，但 legacy_unknown 仍可正常检索；`memory_episode_invalidation.go:12-65`、`apps/core-go/internal/migrations/context_generation.go:78-135`：修订失效与 generation/Resident 联动可复用。`developing_self.go:124-138` 只按 status/expiry 读取，需贯通事实纠正派生失效。
- `conversation_summary.go:18-27,71-96,129-205,343-382,406-497,544-639`：已有 bounded episode chunk、保留 24 messages、source digest、会话锁、revision CAS、投影预算；不等同单份覆盖旧历史的累计摘要，亦未满足定时 5—10 分钟策略。
- `eino_model_runtime.go:75-83,124-131,184-241`：每次 Generate/Stream 前计完整 messages/tools/schema，含 Tool result 和多模态；count_mode=estimated。`runtime_context_refresh.go:147-235` 及 `adk_conversation_runtime.go:221-225` 替换动态上下文，复用而非新建预算器。
- `apps/core-go/internal/migrations/memory_provenance.go:103-144`：0037 只恢复真实可追溯来源，未知项 legacy_unknown；尚无本轮污染的 scoped dry-run/apply/rollback 工具证据。

## 诊断排序与分页
- `diagnostic_model_runs_filter.go:42-62`：内部 round 正序合理；外部列表先 queued/running，再 priority/queued_at，终态 completed_at，最后 created_at/id，与需求固定创建时间倒序不符。
- `agent_run_diagnostics.go:27-35,82-94,145-150`：Agent 和 termination event 各 LIMIT 后 Go 合并，按 started_at 字符串，无 source-aware 稳定 tie-break。
- `apps/core-go/internal/httpapi/domain.go:618-645`、`browser/routes.go:1232-1271`：只有 limit/correlation filter，无 cursor。
- `apps/web/src/stores/control-center.ts:230-245`：每次覆盖各 20 条；`apps/web/src/views/DiagnosticsView.vue:38-72` 明确 aFailed 优先，组时间取 agents[0]/runs[0]；`:112-116` toLocaleString 未满足统一时间输出。

## 既有回归接缝（尚未运行）
- TestVirtualShoppingActivityRequiresElapsedResultAndReusesPurchasedItem；TestIntentionIndependentToolPersistsAcrossDaysAndDoesNotConflatePlanWithResult；TestIntentionSchedule*。
- TestFormalMainWardrobeQueryResultCanLeadToPendingIntention；TestFormalWakeUpAdvancesElapsedHaircutWithoutSendingMessage；TestFormalDueNativeAgentStartsActivityAndResultSettlesAttempt。
- TestPostgresActiveMemoryFactCorrectionInvalidatesRecallResidentAndLateReflection；TestOwnerMemoryCorrectionInvalidatesOnlyAffectedEpisodeAndOldWorkerResult；TestResidentMemoryPublishesWholeGenerationAndSkipsUnchangedReplay。
- TestPostgresConversationSummaryRebuildSupersedesAndOverlapCASFailsClosed；TestPhysicalEinoContinuationBudgetCountsToolResultAndMultimodalImage；TestRuntimeContextRefreshPreservesRawADKHistoryAndMultimodalInput；TestPostgresMemoryProvenanceUpgradeRecoversOnlyRealSourcesAndReruns。
- `project_health_transaction_integration_test.go:82-134`：isolatedCoreTestRepository 使用 GO_CORE_TEST_DATABASE_URL 建临时数据库，缺变量会 Skip；验收不能把 Skip 当通过。
- `formal_tool_adapter_e2e_test.go:72-83`：TestFormalToolEinoAdapterE2E 原生 Eino + controlled HTTP + real PG；`formal_agent_e2e_test.go:12-40`：TestFormalAgentE2E live Agent 入口。
- `infra/acceptance/run-go-live-provider-smoke.sh:8-49,53-97,132-150,168-223,364-455,691-708,764-808`：tools/agents/all 固定矩阵，缺配置/零匹配/SKIP/BLOCKED/FAIL 均失败，保留 baseline。
- `provider_live_e2e_test.go:488-596`：受控 503 ComfyUI 边界不是实际生图成功。`visual_identity_live_e2e_test.go:39-67`：真实 ComfyUI + S3 资产验证。
- 后端排序新增测试复用 `diagnostics_test.go:401-456,490-556`；DTO `browser/dto_test.go:47-106`；Web `apps/web/test/detail-display.test.mjs:116-120` 现仅存在性断言。
