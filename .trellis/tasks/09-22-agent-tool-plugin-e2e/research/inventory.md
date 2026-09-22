# 当前工作树能力基线与迁移清单

## 证据边界

2026-09-22，HEAD 见 baseline-head.txt，既有 52 项工作树变更见 baseline-status.txt。此文件是产品能力期望基线，测试不得从变更后的注册集合反向生成期望清单。局部详细报告见 agents.md、tools.md、tests-preflight.md。下列清单结合主线程对实际装配及正式调用点的补充读取，不把历史报告当代码事实。逐工具参数/事务内部细节必须在修改前继续完整读取。

## 全部当前注册业务 Tool（18）

统一装配 `apps/core-go/internal/core/builtin_capabilities.go:71-123`；所有下列名字均需独立真实测试和 Eino adapter 证据，internal 标签不豁免业务能力。以下相对路径以 `apps/core-go/internal/core/` 为根。

| Tool | 实现 / 依赖与真实结果边界 | 当前独立执行障碍 |
|---|---|---|
| conversation.reply | builtin_capabilities.go:152-174；mutations.go 发布/消息事务、action result 通道 | Execute 仅 deferred，外层创建消息，deferred executor 无真实写入 |
| moment.publish | builtin_capabilities.go:176-196；现有 Moment 发布/settlement | Execute 仅 deferred，需外层 binding 与发布 |
| media.image.generate | builtin_capabilities.go:198-335；MediaService、durable intent、outbox、Temporal、ComfyUI、对象存储 | target binding 在外层；直接调用 deferred；已有异步契约需准确区分 accepted 与完成 |
| visual_identity.initialize | builtin_capabilities.go:338-380；MediaService、visual_identity.go session/attempt/intent | 要求 wake_up_ SourceFactID，要求 persona/visual slots，runtime 拒绝自事务 |
| scene_event | builtin_capabilities.go:383-411；native_capabilities.go:12，LifeContextService/事件投影 | transactional 被通用 runtime 拒绝，Prepare 依赖冻结上下文 |
| presence_event | builtin_capabilities.go:414-442；native_capabilities.go:53，LifeContextService | 同上，须改为明确资源与业务输入 |
| schedule.replan | builtin_capabilities.go:445-564、schedule_capability.go；SchedulePlanner/Service、真实 Provider | Prepare 内模型计划，外层事务 apply；不能在 DB 事务中调用模型 |
| memory_event | builtin_capabilities.go:566-596、memory_intelligence.go:55-138；MemoryService/lifecycle | 需要 CorePersona/MemoryScope、source fact；普通 apply 明确 caller_transaction_required |
| active_memory_event | builtin_capabilities.go:598-627、active_memory.go:98-106,204-236；MemoryService | 从 cognition_inbox 获取来源时间，直接 apply 拒绝；独立业务来源需显式建模 |
| affect_event | builtin_capabilities.go:661-679、affect_capability.go；affect reducer 与 DB event/state | runtime 要求外层事务；保留服务器数值 reducer |
| memory.recall | builtin_capabilities.go:629-658、memory_recall_capability.go:17-90；MemoryRecallService | 查询可真实执行，但装配依赖 App/MemoryScope；只按 conversation surface 暴露 |
| relationship.lookup | builtin_capabilities.go:681-726、relationship_capability.go；关系查询 | 查询可真实执行；需解除与上下文候选阶段的耦合，保留授权范围 |
| capability.request | builtin_capabilities.go:729-745、capability_requests.go:35-120；request ledger | transactional runtime 拒绝普通调用；当前默认幂等键依赖 call ID |
| persona.takeover | persona_action_capabilities.go:12-50；turn_takeover.go:537 与后续提交 | 只返回 awaiting_domain_commit，internal/policy 不等于纯框架控制 |
| persona.switch | persona_action_capabilities.go:12-50；mutations.go:861 与持久人格提交 | 同上，必须实际独立提交人格切换 |

视觉完整 Agent 新增并固定登记以下 3 项业务 Tool；它们只出现在专用 `visual_identity` surface，不进入 conversation/WakeUp/autonomy/native/reflection catalog：

| Tool | 实现 / 真实结果边界 |
|---|---|
| visual_identity.generate_candidate | `visual_identity_tools.go`；冻结角色卡 prompt，创建真实 `media_intents` 与 `platform_workflow_intents`，返回 `accepted`，最终边界为候选 asset ready。 |
| visual_identity.commit_review | `visual_identity_tools.go`；保存真实图片观察/决策，保留 rejected 历史资产，或以 CAS 晋升 canonical 并创建 character-sheet media intent。 |
| visual_identity.finalize | `visual_identity_tools.go`；仅在 character-sheet media intent 和实际 ready asset 都存在时提交 revision/profile/session/attempt/timeline 完成事实。 |

上述 3 项与原 15 项一起构成固定 18 项产品 Tool 清单。新增项必须通过独立 `App.ExecuteTool` 数据库/产物/重放测试与正式 Eino adapter 测试，不能从当前 registry 动态反推期望集合。

## 完整任务提示词与正式入口（当前 16 类）

| 任务 | 当前入口 / prompt 来源 | 上下文与输出去向 |
|---|---|---|
| conversation cognition | conversation_runtime.go:90 RunMain；mutations.go:785 的 prompt | 对话 projection → 回复/工具/认知结果；当前在外层冻结结算 |
| wake-up | wakeup.go:625-637 | 身份/生活/记忆 projection → 后台行为、follow-up |
| takeover judge | conversation_runtime.go:120；turn_takeover.go:294,768 | 人格规则/轮次输入 → takeover 判定 |
| takeover reply | conversation_runtime.go:130；turn_takeover.go:946 | 选定人格上下文 → 接管回复/工具结果 |
| initialization | model_tasks.go:29；initializationAnalysisMessages | Owner 描述 → 初始化身份/人格 |
| media prompt | model_tasks.go:48；prompt.MediaPromptInstruction | intent/concept → 媒体生成 prompt |
| media quality | model_tasks.go:66；prompt.MediaQualityAcceptanceInstruction | 图片 bytes+intent → pass/retry/reject |
| visual vision | model_tasks.go:94；visualIdentityVisionTaskInstruction | 真实 image content+identity → bounded observations |
| visual patch | model_tasks.go:121；visualIdentityPatchTaskInstruction | identity+vision → accepted/regenerate+patch |
| conversation summary | model_tasks.go:140；conversationSummaryInstruction | 原始消息 → summary projection |
| schedule generation | model_tasks.go:168；scheduleGenerationTaskInstruction | 日期/时区/身份/生活 → 完整日程 |
| native cognition | model_tasks.go:192；NativeCognitionInstruction | 世界事实+projection → appraisal/attention/thought/desire/agency 与工具 |
| daily review | model_tasks.go:213；capabilityDailyReviewPolicyInstruction | 日期+schedule+projection → 自主行为/工具 |
| persistent switch | model_tasks.go:237；persistentSwitchAssessmentInstruction | persona rules+对话+候选输出 → keep/switch |
| reflection | model_tasks.go:259；ReflectionV2Instruction | evidence+projection → 反思候选/领域应用 |
| schedule replan | model_tasks.go:277；内嵌 replan instruction | intent+schedule/life/agency → 替换计划 |

以上每类有正式 Agent 定义、独立输入及运行证据；没有业务必要时可以工具集合为空并一次决策结束。视觉身份完整任务另提供自身正式入口，负责生成/看图/评审/保存闭环，持久工作流只承载进度恢复。不得仅保留两个模型 Task 让外层代决策。Embedding 是向量模型基础设施，不是完整任务提示词；LanguageRule/ContextAuthorityRule/RuntimeProtocol/Slots 是公共片段。ActionRealizationInstruction 当前搜索只见定义与别名，没有生产调用，后续如确认有效调用才纳入迁移，不能仅凭名字删除。

## 已确认必须替换的机制

- loop.go:146-151 限制两轮；eino_model_runtime.go:527-532 deferred-stop；:551-572 写工具超限当成功。
- adk_conversation_runtime.go:241-259 schema allowlist；model_tasks.go 单次 Provider 旁路；conversation_runtime.go:112-128 自研 query continuation/judge 旁路。
- adk_conversation_runtime.go:83-112 用 native call ID 做唯一 replay 身份；:135 默认 settlement_pending；真实业务幂等必须独立于模型 ID。
- eino_model_runtime.go:580-624 删除中间 reply tool calls/trace；真实提交后不得抹除。
- capability_core.go:1252-1258 caller_transaction_required，capability_runtime.go:228-232 deferred，:711-811 代执行；mutations.go:1061-1077,1435-1455 settlement。
- visual_identity.initialize 的 wake_up_ 门禁；persona.* 只有 deferred；memory/active memory 对认知来源和人格快照的隐藏依赖。
- structured-turn-contract 与 cognitive-runtime 中“只允许 QUERY 再调用一次”“写操作后不续接”“只能外层结算”的规范必须替换。

## 测试缺口

现有 controlled ADK/httptest 不算真实 Agent E2E；live durable 使用 Mock ComfyUI；persona E2E 无一次性环境生命周期。当前没有全部 15 Tool 的无 Agent 独立 E2E，也没有上述每类 Agent 的正式独立循环证据。完整验收必须包括真实媒体产物、随机秘密记忆检索、生产流式、故障后提交事实、多调用 identity 和两项破坏性检验。
