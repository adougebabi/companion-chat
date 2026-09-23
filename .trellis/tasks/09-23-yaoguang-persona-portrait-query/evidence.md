# 实施与验收证据（2026-09-23）

## 实际接线

| 能力 | 文件与符号 | 正式调用者 | 实施前 | 本次修改 | 对应测试 |
| --- | --- | --- | --- | --- | --- |
| 文本→结构化人格 | `app.go: AnalyzeDescription`, `model_tasks.go: RunInitializationTask`, `app.go: prepareInitializationResponse` | Core/BFF 初始化分析路由 | 生效 | 保留，画像以已校验的激活 JSON 为来源 | `TestInitializationTextToStoredPortraitToFormalRequest` |
| 来源与激活 | `app.go: persistInitializationAnalysisSource`, `CreateFluctlight` | 创建路由 | 生效 | 验证分析归属/最新/未用后，在事务外编译所有 profile，再与 foundation 一同提交 | 同上；HTTP `TestPostgresInitializationAnalysisAuthoritySerializesLatestSourceAndReplay` |
| 稳定画像 | `persona_compilation.go: CompileWorkingPersona`, `working_persona_persistence.go` | 创建、正式 Foundation 更新、稳定覆盖层 Reflection、维护命令 | 无模型编译/持久化 | 新 formal Agent、来源引用/遗漏诊断、规则与预算版本、单次语义修复、DB 产物 | `TestPersonaCompilerUsesOneBoundedSemanticRepair`, `TestFoundationAcceptancePublishesMatchingPortraitOrKeepsOldVersion` |
| 有效人格/覆盖层 | `evolution_overlay.go: readEffectivePersonaProjection`, `portraitOverlayRevision`; `reflection_runtime_v2.go` | ContextProjection、Reflection | 生效，但总 revision 混入非人格演化 | 画像只绑定实际有效的人格覆盖层修订；稳定覆盖层发布同事务写画像 | `TestProcessReflectionMemoryCandidateUsesOpaqueRefLifecycleAuthority`, `TestPersonaToolComposesAcceptedOverlayAndRejectsCorruption` |
| 正式 Prompt | `provider_context.go: assembleProjectionPromptForSurface`; `working_persona.go: systemPersonaForProjectionWithWorking` | Main、WakeUp、受影响 formal Agent | 运行时确定性投影 | 读取保存画像并校验；只保留标识 roster，缺失/过期明确失败 | `TestCompiledWorkingPersonaNativeDetailChain`, `TestWakeUpConversationReplyCreatesAndDeliversPrivateMessage`, `TestFormalMainRejectsMissingOrStalePortraitBeforeProvider` |
| 接管后发言 | `persona_action_service.go`, `tool_profile_scope.go` | Main 原生 Eino Loop 中的 `persona.takeover`/`persona.switch` | Tool 回传完整目标 profile | 回传目标的已保存画像，后续 Tool 使用目标 profile 和来源/覆盖层版本 | `TestTakeoverDetailQueryUsesNewSpeakingProfile`, `TestNativePersonaTakeoverFeedsScopedToolsReplyAndFinal` |
| 人格详情 Tool | `persona_detail_capability.go: personaDetailCapability`, `tool_execution.go: ExecuteTool` | 独立调用；Main/WakeUp capability catalog | 无注册 Tool；内部 HTTP detail 仅供 Owner 页面 | 同一只读实现提供 list/read、2048 字符分段、权限/版本/出处 | `TestPersonaDetailIndependentToolReadsCanonicalSource`, `TestFormalToolEinoAdapterE2E` |
| 原生查询续接 | `adk_conversation_runtime.go: ExecuteWithID`, `internal/ai/agent/loop.go` | Eino ChatModelAgent | 现有 Loop | 注册新 Tool、保留读者身份与 Trace、实际结果回填 | `TestCompiledWorkingPersonaNativeDetailChain`, `TestPersonaDetailTraceSurvivesModelContinuationFailure` |
| 存量准备 | `working_persona_backfill.go`, `cmd/persona-backfill`, API/Worker 启动门禁 | 运维明确调用 | 无 | 范围预览、幂等跳过、失败报告、规则/预算变更预检；服务前要求准备完成 | `TestWorkingPersonaBackfillPreviewApplyAndReplay` |

## 数据职责

- `fluctlights.core_persona`、accepted Foundation 和有效稳定 Persona 覆盖层是权威来源；`fluctlight_initialization_sources` 保留原文与分析投影。Owner 可以合法编辑分析预览，激活 digest 记录最终提交内容，因此分析投影 digest 不强制与激活 JSON 相等。
- `fluctlight_working_personas` 是派生画像，按 `fluctlight_id + profile_id` 保存事实、来源引用与遗漏诊断；`source_hash` 是过滤后的编译输入摘要，不是原文摘要。普通 Prompt 只渲染画像类别文本。
- Prompt diagnostics 记录 profile、来源 revision/hash 前缀、覆盖层 revision、规则版本、预算和 `cache_hit=false`；当前实现按保存产物读取，无画像缓存，也不在诊断中写入完整私人档案。
- 情绪、当前场景/日程/衣着、关系进展和记忆仍由原 ContextProjection、Prompt Composer 与各领域来源读取；画像不声称这些状态已经发生。`persona.detail` 读取版本化设定，`memory.recall` 读取运行记忆。
- 初始化若无 profile 资料，blank-slate 产生最小来源引用身份画像；正式稳定更新改由编译 Agent。编译失败不发布新 Foundation/overlay，已有一致版本保留。

## 补编译与切换

在 `apps/core-go` 目录先运行迁移，再使用当前版本二进制：

```text
go run ./cmd/migrate
go run ./cmd/persona-backfill --owner <Owner actor ID> --fluctlights <ID,ID>
go run ./cmd/persona-backfill --owner <Owner actor ID> --fluctlights <ID,ID> --apply
```

明确全范围时才使用 `--all`。预览不会调用模型或写业务数据；`--apply` 对缺失/过期项逐 profile 编译，当前项跳过。运行时规则版本或 `runtime_settings.working_persona_budget` 变更后先执行补编译。API 与 Worker 的 `VerifyWorkingPersonasReady` 在启动前拒绝未准备完成的存量。未对生产全库执行本命令。

`persona.detail` 独立调用通过 `App.ExecuteTool`：Core 绑定 `AuthorizationActorID`、`FluctlightID`、可选的合法 `WorkingProfileID`、`OperationID`，模型参数只含 `{"operation":"list"}` 或 `{"operation":"read","section_id":"life_profile","cursor":0}`。目录列出当前发言 profile 可访问的分区；读取返回实际 profile、Foundation/覆盖层版本、真实内容及 `has_more/next_cursor`。`source_text`、digest、provenance 等初始化管理字段按现有模型出站过滤规则剔除。

## 实际样例与预算

受控测试来源：`life_profile.preferences.drink = "喜欢咖啡，但不喜欢甜咖啡"`；所选 profile 另有只在完整资料中的私密过去经历。编译产物保留 `stable_preferences: ["喜欢咖啡，但不喜欢甜咖啡"]`，诊断指出细节仍在完整设定。用户当前消息未提“咖啡”，首轮正式模型请求仍含该偏好；首轮不含完整经历，原生 `persona.detail(read, "profile.secrets")` 后下一轮请求才含真实查询结果。

数字来自 `TestCompiledWorkingPersonaNativeDetailChain` 的受控 HTTP Provider，Token 栏为 `EstimatePromptTokens` 的约 1.25 Token/字符估算，**不是实际 Provider usage**：

| 项目 | 字符 | UTF-8 字节 | 估算 Token |
| --- | ---: | ---: | ---: |
| 同来源旧确定性 Working Persona | 245 | 267 | 307 |
| 保存后精简画像 | 182 | 214 | 228 |
| 初轮完整请求 | 19,273 | 21,201 | 24,092 |
| 查询后续接完整请求 | 20,392 | 22,342 | 25,490 |
| 两轮累计输入 | 39,665 | 43,543 | 49,582 |
| 续接中的 Tool result | 923 | 945 | 1,154 |
| 一次性编译请求 profile A | 2,247 | 3,035 | 2,809 |
| 一次性编译请求 profile B | 2,204 | 2,970 | 2,755 |

初轮拆分：协议 1,891 字符/3,575 字节/约 2,364 Token；画像和 profile roster 541/619/约 677；动态上下文 969/1,067/约 1,212；当前输入 64/132/约 80；tools 9,579/9,579/约 11,974；response schema 6,068/6,068/约 7,585；该受控夹具无历史与记忆。各槽位按自身序列化统计，因此不应相加当成完整 wire Token 总数。

现有相同受控 turn cost fixture 的提交前/本次结果见 `internal/core/testdata/turn_path_cost_report.json`：无 Tool 首轮从 19,891 字符、22,039 字节、约 24,864 Token 到 19,400/21,248/约 24,250；一次 Tool 场景累计输入从约 51,204 到约 49,977 Token。它是另一固定场景，不能把差异全部归因于画像。新查询场景的第二轮增加已如上单列。

## 执行记录与验收状态

- `go test ./...`、`go vet ./...`（`apps/core-go`）：通过。
- 使用隔离 Docker `pgvector/pgvector:pg16` PostgreSQL 的新增 Core 链路/独立 Tool/接管/补编译/更新、正式 WakeUp 请求测试，以及 `internal/httpapi` 初始化来源重放测试：通过。
- `GO_CORE_TEST_DATABASE_URL=... go test ./internal/migrations -count=1`：通过。
- 数据库全量 `go test ./internal/core -count=1`：仍有 11 个失败；已在原始提交 `c966c31` 的独立工作树中逐个复跑，同 11 个在改动前也失败。包含固定日期 Life Context、诊断表未建立、现有流式错误/Provider 诊断断言；详见本次终端记录，未宣称全量 DB 套件通过。
- 真实 Provider `TestFormalAgentE2E`：**BLOCKED / 未验收**。`FLUCTLIGHT_LIVE_PROVIDER_URL`、`FLUCTLIGHT_LIVE_PROVIDER_MODEL`、`FLUCTLIGHT_LIVE_PROVIDER_API_KEY` 均未提供；命令返回 SKIP，仅记录为缺少验收条件。编译保真、多场景行为稳定性与真实 tokenizer/usage 尚不能宣称通过。

## 旧路径清理与限制

正式 Prompt 不再调用旧 `systemPersonaForProjection` 确定性全量投影；旧投影仅供治理/测试中的明确用途。接管 Tool 的完整目标 profile 回执改为已保存画像；`personality_system` 常驻内容收敛为标识 roster/授权切换区域，共享人格机制进入编译输入。原始完整档案及 Owner detail 页面保留。受控测试证明协议链和版本一致性；真实 Provider 与长期行为样本仍待验收。
