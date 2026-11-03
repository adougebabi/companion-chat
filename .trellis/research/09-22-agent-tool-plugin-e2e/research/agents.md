# Agent / Eino runtime 局部核查

## 范围与限制

本次是紧凑只读核查。逐行读取了以下 8 个文件：

- `apps/core-go/internal/ai/agent/loop.go`
- `apps/core-go/internal/core/eino_model_runtime.go`
- `apps/core-go/internal/core/provider_prompts.go`
- `apps/core-go/internal/core/provider_prompt_composer.go`
- `apps/core-go/internal/core/prompt_slots.go`
- `apps/core-go/internal/core/provider.go`（仅 Provider Eino 调用与输出归一化相关段）
- `apps/core-go/internal/core/model_tasks.go`（任务入口段）
- `apps/core-go/internal/core/adk_conversation_runtime.go`（ADK 上下文、allowlist 与统一入口段）

其余文件只用 `rg` 定位调用点，没有阅读实现。因此下面对“调用方”给出的是静态命中，不保证覆盖反射、接口注入或生成代码；`provider_prompts.go` 中转引自 `internal/ai/prompt` 的常量也未越界展开原文。

## 结论

Core 的正式 ADK 入口是 `App.RunADKStructuredTask`，它只允许三种 schema：`conversation_turn_response`、`takeoverReplySchemaName`、`wake_up_response`。入口验证 capability 上下文、工具定义和已组装 prompt 后，把 request-scoped invoker/trace 放进 context，再调用 `Provider.StructuredAssembledWithToolsSchema`。Provider 只有在 context 含 ADK invoker 且 schema 命中 allowlist 时，才从普通单次 Eino `Generate` 切到 `generateWithADK`。关键位置：`adk_conversation_runtime.go:241-259,262-309`，`provider.go:323-328,358-363`，`eino_model_runtime.go:433-439,498-572`。

ADK 的模型—工具循环是明确受限的两轮循环：生产调用固定 `MaxIterations: 2`；底层默认值也是 2，并拒绝任何大于 2 的配置；工具顺序执行；生产未启用 streaming。只有“最终 assistant 工具调用全部不是只读 query”时，达到最大轮次可作为合法 action-only 终止。关键位置：`loop.go:38-55,135-160,164-172,176-236`，`eino_model_runtime.go:551-572`。

## 完整提示词集合（在本核查边界内）

### Core 统一别名

`provider_prompts.go:8-15` 是任务提示词的 Core 出口，共 7 项：

1. `providerLanguageRule = prompt.ProviderLanguageRule`
2. `mediaPromptInstruction = prompt.MediaPromptInstruction`
3. `mediaQualityAcceptanceInstruction = prompt.MediaQualityAcceptanceInstruction`
4. `providerContextAuthorityRule = prompt.ProviderContextAuthorityRule`
5. `reflectionV2Instruction = prompt.ReflectionV2Instruction`
6. `nativeCognitionInstruction = prompt.NativeCognitionInstruction`
7. `actionRealizationInstruction = prompt.ActionRealizationInstruction`

这些值由 `internal/ai/prompt` 提供，本次按限定范围没有读取其定义文本。因此这里的“完整”是完整标识符集合，不是对跨包常量原文的复制。

### Composer 内嵌运行协议

`provider_prompt_composer.go:9-22` 的 `providerRuntimeProtocol` 有 7 条硬规则：中文自然语言；权威优先级 `core_persona > developing_self > current_state`；场景/context 强绑定和显式 override；Actor 语义不等于 transport role；认知摘要、证据、真实 Tool Call 与不伪造成功；模型负责多人格判断但只通过 `personality_decision` 提出持久切换；context reference 必须逐字来自 runtime context。

`provider_prompt_composer.go:24-31` 的 `providerInitializationRuntimeProtocol` 也有 7 条：中文字段；初始化是信息解析而非扮演/聊天；只结构化 Owner 描述；多人格各自资料和切换关系但不在初始化时判定主导人格；固定 Actor 语义；不把瞬时状态写入 Core Persona；只返回初始化 schema JSON。

`renderProviderSystem` 始终生成一个系统消息，顺序为“运行协议 → operation_rules → Actor 与关系上下文 → 人格设定”；初始化角色使用初始化协议，其余角色使用通用协议。无 persona 时分别注入初始化边界或“不得自行补充固定人格事实”。见 `provider_prompt_composer.go:157-188`。

### 已定位的正式任务与规则组合

`model_tasks.go` 暴露以下 typed task：

| 任务入口 | role / scenario | operation instruction | 输入上下文 | schema / 输出 |
|---|---|---|---|---|
| `RunInitializationTask` | `initialization` / `initialization` | `initializationAnalysisMessages`，最终由 initialization runtime protocol 统一包裹 | Owner `Description` | `map[string]any`，Provider role schema |
| `RunMediaPromptTask` | `media_prompt` / `media_prompt` | `mediaPromptInstruction` | 冻结后的 `mediaIntent` | 文本 |
| `RunMediaQualityTask` | `media_prompt` / `media_quality_acceptance` | `mediaQualityAcceptanceInstruction` 由 `mediaQualityMessages` 产生 | intent + content type + bytes | `media_quality_acceptance_response` → `mediaQualityAcceptance` |
| `RunVisualIdentityVisionTask` | `visual_identity_vision` | `visualIdentityVisionTaskInstruction` | asset id、身份快照、渲染约束、可选图像 | `visual_identity_vision_response` structured object |
| `RunVisualIdentityPatchTask` | `visual_identity_patch` | `visualIdentityPatchTaskInstruction` | asset id、身份快照、约束、vision 观察 | `visual_identity_patch_response` structured object |
| `RunConversationSummaryTask` | `reflection` / `conversation_summary` | `conversationSummaryInstruction` | source messages | `conversation_summary_v1` |
| `RunScheduleGenerationTask` | `cognitive_assessment` | `scheduleGenerationTaskInstruction`，截断重试时追加完整紧凑输出提醒 | date、timezone、identity、life profile | `schedule_response` |
| `RunNativeCognitionTask` | `cognitive_assessment` / `native_cognition` | `providerContextAuthorityRule + nativeCognitionInstruction` | Native Cognition projection + event type + compact fact + capability catalog | `native_cognition_response` + `ProviderCompletion` + projection/diagnostics |
| `RunDailyReviewTask` | `cognitive_assessment` / `daily_review` | `providerContextAuthorityRule + capabilityDailyReviewPolicyInstruction` | Daily Review projection + local date + autonomy capability catalog | `daily_review_response` + completion/projection/diagnostics |
| `RunPersistentSwitchTask` | `cognitive_assessment` | `providerContextAuthorityRule + persistentSwitchAssessmentInstruction` | Persistent Switch projection + user text + candidate reply + response intent | persistent-switch assessment schema，无 tools |
| `RunReflectionProposalTask` | `reflection` / `reflection` | `providerContextAuthorityRule + reflectionV2Instruction` | Reflection projection + compact evidence | `reflection_proposal_v2`，无 tools |
| `RunScheduleReplanTask` | `cognitive_assessment` / `schedule_replan_planner` | 字面规则：只返回完整替换日程，保留已完成历史，使用给定 timezone/revision | intent、compact schedule、current life、agency、timezone | `schedule_replan_plan` |
| `RunEmbeddingTask` | embedding 通道 | 无聊天 system prompt | text | model id + vector |

表中入口与字段见 `model_tasks.go:22-180,183-287`。两个 visual identity instruction 的完整字面量在 `model_tasks.go:92,119`；schedule generation 字面量在 `model_tasks.go:166-176`；schedule replan 字面量在 `model_tasks.go:282-287`。`conversationSummaryInstruction`、daily-review/persistent-switch policy 常量定义在范围外，本次只记录实际组合名。

## Prompt 正式形态与上下文

普通任务入口 `PromptComposer.ComposeTaskMessages` 委托 `composeTaskMessages`（`prompt_slots.go:77-83`）。除 `media_prompt` 外，composer 会：

1. 收集所有 system fragments 为 `operationRules`，去重基础 language/context authority 规则；
2. 从 system 或 JSON payload 抽出 `actor_relationship_context`；
3. 从非 system JSON 抽出并合并 `core_persona`，从动态 payload 中删除其副本；
4. 把动态 JSON 转成分节 YAML/TOON 文档；
5. 产出恰好一个 system message，再追加原顺序非 system 消息。

见 `provider_prompt_composer.go:43-98`。assembled prompt 必须至少两条、首条唯一 system、末条 user、中间只能是 user/assistant 且内容非空；见 `provider_prompt_composer.go:100-125`。动态上下文可呈现 `context/context_projection`，以及 developing self、schedule、current state、memories、goals、intentions、recent outcomes/messages、relationships、hypotheses、drive/preference/trigger slots、visual identity、presence；操作输入还包括 current message/text/current user text/event type/fact/evidence/response plan/capability results/local date。见 `provider_prompt_composer.go:493-549`。

显式 slot composer 的输入是 `System`、`CurrentInput`、有序 slots、tools 和 response format；输出是 Eino `[]*schema.Message` 与 assembly trace。它排序、按 token budget 选择 fragments，然后复用 `AssemblePromptContext` 作为唯一预算/选择权威，最后仅转换一次 Eino message。见 `prompt_slots.go:38-49,85-170`。

## ADK 正式入口、上下文与输出

### 入口与上下文

- `ADKStructuredTaskInput`：`Role`、`Scenario`、组装后的 `Prompt`、`Definitions`、`SchemaName`、`EnableThinking`、可选 `Capability`。见 `adk_conversation_runtime.go:262-272`。
- `RunADKStructuredTask`：校验 provider、schema allowlist、工具与 capability context；创建 request-scoped trace；注入 invoker 和 scenario；验证 assembled messages；调用 Provider。见 `adk_conversation_runtime.go:279-309`。
- `WithADKCapabilityInvoker` 只把 `{Invoker, Trace}` 放进 context，取出时 invoker 必须非 nil。见 `adk_conversation_runtime.go:16-37`。
- Provider 构造 `EinoModelCall`，携带 assignment、role/scenario/priority、diagnostic/correlation IDs、messages、definitions、JSON/schema、thinking 和 provider request ID。见 `eino_model_runtime.go:36-53`、`provider.go:358-363`。
- `generateWithEino` 的分流条件是“context 有 ADK capability + schema allowlist”；否则执行单次 Eino Generate。见 `eino_model_runtime.go:433-495`。

### 工具边界

- 工具只来自已安装 `CapabilityDefinition`；定义必须有 name，input schema 被转换成 Eino JSON Schema。见 `loop.go:57-76,112-133`。
- 有工具定义时 invoker 必须实现保留 identity 的 `ExecuteWithID`；每次调用必须从 Eino context 取得真实 tool-call ID，空 ID 直接失败。见 `loop.go:31-36,59-67,93-109`。
- Core bridge 的 invoker 是 request-scoped CapabilityRuntime closure，负责 authorization、context snapshot、idempotency 和 settlement；工具 adapter 不接触 App/repository/transaction。见 `adk_conversation_runtime.go:231-239`。

### 输出

- `RunADKLoop` 返回 `ADKLoopResult`：最终 assistant message、所有输出 messages、去重后的 tool calls、tool-role results、event iteration count。见 `loop.go:49-55,173-229`。
- 必须有 final assistant message；其 content 与 tool calls 不能同时为空。见 `loop.go:229-236`。
- `generateWithADK` 会移除仅作为中间实现细节、且不与 root visible text 同轮的 `conversation.reply`，保留其他中间 capability calls；随后按最终 calls 过滤 trace。见 `eino_model_runtime.go:580-624`。
- Provider 只从 Eino typed native ToolCalls 接受执行请求，保留真实 ID；structured Content 只作为 DTO，解析后会删除其中的 `tool_calls`。最终 `ProviderCompletion` 至少包含 `Text`、native `ToolCalls`、`DoneSeen=true`，可带归一化 `Structured`。见 `provider.go:379-399,400-447`。
- `RunADKStructuredTask` 输出 `{Completion, Trace}`，不负责 freeze、settle、publish 或 transaction；这些由领域调用方完成。见 `adk_conversation_runtime.go:274-280,305-309`。

## Loop 限制与终止细节

1. `Model` 必填；agent name/description 必填；至少一条 message；nil message 拒绝。`loop.go:136-145,165-170`。
2. `MaxIterations <= 0` 归一到 2；`> 2` 拒绝。生产固定 2。`loop.go:146-151`，`eino_model_runtime.go:553-556`。
3. tools `ExecuteSequentially: true`。`loop.go:153-160`。
4. event error 默认传播并保留 partial result；只有 `ErrExceedMaxIterations` 且最后 assistant 有 tool calls、且 `ToolOnlyTermination` 判真时吞掉并结束。`loop.go:176-197`。
5. `ToolOnlyTermination` 对每个 call 查 definition；未知工具或 read-only query 都使其返回 false；只有全是非只读 action 才允许 action-only 终止。`eino_model_runtime.go:556-570`。
6. queued wrapper 对 deferred/rejected action round 可本地提前投影；conversation/takeover 还要求 root visible text，wake-up 可 action-only。`eino_model_runtime.go:55-73,75-81,527-532`。
7. 每个物理 model call 独立排队并分配 `correlation:adk:N`、request ID 与 provider attempt identity，避免在整个 agent loop 期间占住一个 queue lease。`eino_model_runtime.go:75-107`，`provider.go:323-328`。
8. Tool call 按非空 ID 去重；无 ID 的 call 不会被 map 去重。`loop.go:173-219`。

## `rg` 定位到的领域调用方

- ADK allowlist 三个主要 prompt 组装点：conversation main `mutations.go:785`，wake-up `wakeup.go:625`，takeover reply `turn_takeover.go:946`。
- 初始化：`app.go:354`。
- media prompt：`media.go:88`；media quality：`media_quality.go:155`。
- summary：`conversation_summary.go:266`。
- persistent switch：`persistent_switch_assessment.go:44`。
- daily review：`autonomy.go:82`。
- native cognition：`cognition_growth.go:363`。
- schedule generation：`schedule_generation.go:24,34`。
- visual identity vision/patch：`visual_identity.go:892,921`。
- reflection：`reflection_runtime_v2.go:168`。
- embedding：`memory_retrieval.go:419`。
- schedule replan 通过 planner interface：`schedule_capability.go:61,67`。

这些调用方只做了 `rg` 定位，没有读取其具体 freeze/settlement 行为。
