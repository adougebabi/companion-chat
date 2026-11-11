# 运行入口盘点：MeroMero-first T01

核对版本：`12b05faa`（2026-11-10）；按当前源码静态盘点。起始基线为 `ffa808bc`，本页覆盖原先只按起始基线列出的 30 项。实际配置、部署中启用状态及真实请求结果须分别以运行时读取或专门验收为准。本次不读取业务数据库、不发送模型请求，也不恢复已删除的 `docs/llm-agent-prompts.md`。

## 盘点口径

- 当前 `formal_agents.go` 注册 **31 个**正式 Agent ID。`FormalAgentRunInput.Variant` 非空时优先作为任务变体，否则使用 `SchemaName`，见 `apps/core-go/internal/core/formal_agents.go:205`、`:255`。所以一个 ID 或 schema 不能当成唯一请求；Writer 的 `conversation_reply_text` 是输出 schema，不是其实际的三个基础 variant。
- 表中 **有上游**表示发现非测试生产代码的静态调用链，不表示服务已经部署、功能开关开启、模型支持或结果已被验收。**未确认**表示找到注册/包装但本次未找到非测试上游，不等于已删除或停用。
- 所有相对源码路径以 `apps/core-go/internal/` 为根。表中 `core/` 为核心领域，`workflow/` 为持久工作流。首个路径对应实际 caller 或任务构造；Prompt 详细 Slot 与协议另见 [提示词组成清单](../llm-prompt-composition.md)。
- **G**：默认 `generic_llm`；**R**：默认 `conversation_reply`；**V**：独立 `vision_understanding`。可配置的文本任务按 variant → Agent → 默认用途解析；私聊主认知、WakeUp、NPC 和表达提取从 G 改为 R。已冻结的同任务 assignment 优先；视觉和 embedding 不被文本覆盖。具体代码 `core/agent_model_routing.go:114`、`core/diagnostics.go:179`、`core/provider.go:137`。这不是“所有 Agent 已使用 MeroMero”的证明。
- **S**：typed task 不安装 Agent 工具，调用与输出均经过正式 Runner；外层仍可能写数据库、发布或启动任务。**A**：有作用域限定的原生工具或继续决策；可能多次物理请求。两者都可能触发下文所列修复或传输重试，S 不承诺恰好一次模型请求。
- 处理方式：除行内明确写明迁移或未确认外，其余入口按现有职责保留；“归 Image/Moment”表示复用现有插件边界，不意味着重新创建运行时。
- `MandatoryTools` 只是已安装工具经 Kev 过滤后保留的交集，不授予工具；实际可见集合取决于 typed task、surface、插件开关及权限，见 `core/formal_agents.go:73`。表格中的工具为源码声明或专用工具，不代表整份运行时目录。
- **版本口径**：Writer 使用 `fact-only-reply.v4`–`v8` 并计算协议正文 hash；累计摘要使用 `conversation-summary.prompt.v2`，片段使用 `conversation-segment.prompt.v1`。其他下列入口未发现逐项独立的固定 Prompt 版本/hash，按各行构造函数与代码版本定位；Provider 通用 `go-provider-v1`（reflection 为 `reflection.v2`）是粗粒度诊断标签，不能冒充请求正文 hash，见 `core/diagnostics.go:674`。动态人格、来源和工具使完整请求无法由静态版本推得。

## 正式 Agent 与实际任务

| Agent ID / 实际 variant | 领域、当前 caller 与状态 | 模型/运行 | Prompt 来源、主要输入 Slot | 输出、工具及真实副作用/发布 |
| --- | --- | --- | --- | --- |
| `conversation_cognition` / `conversation_turn_response` | 私聊；`core/agent_result_adapter.go:304` → `core/cognition_agent.go:171`，恢复 `core/reply_continuation.go:144`；有上游 | R·A | 主认知协议、有效人格、会话状态、来源记忆与历史、当前输入；`cognition_agent.go` | 认知输出与授权原生工具；`conversation.reply` 等真实发布由 Core 执行。缺省暂保留 fact-only + Writer；direct 可显式选择，待 T06 发布核验及真实模型验收 |
| `conversation_reply` / `private_reply`、`proactive`、`moment` | 私聊/主动/动态；`core/reply_failure_policy.go:181` → `core/reply_provider.go:227`；有上游 | R·A | Writer 协议及 v4–v8、人格、对应场景事实/真实回执、按需记忆历史和当前输入；`reply_context.go`、`reply_expression.go` | 正文草稿和私有 `reply.image.request` / `persona.detail`；外层校验后唯一发布；保留场景表达，主私聊迁移 T04 |
| `conversation_reply` / `{private_reply,proactive,moment}.style_repair`、`*.correction` | 同一 Writer 的错误/格式修复；`core/reply_style.go:207`、`core/reply_action_consistency.go:79`、`core/reply_failure_policy.go:184`、`core/reply_provider.go:240`；条件触发 | R·A；继承冻结绑定或 Agent/默认配置 | 原场景资料、未发布候选及相应校验反馈；`reply_provider.go:166` | 修复候选仍须校验，已有真实效果不得重做；保留。修复后缀不在 `agentModelVariants` 基础列表中，不能当成独立可配置槽位 |
| `reply_action_check` / `reply_action_consistency` | 动作核验；`core/reply_action_consistency.go:33`；有上游 | G·S | 失败换装的真实回执、状态与未发布候选；无人格扮演 | 内部一致性结论，无工具；外层决定是否修复/发布；媒体一致性范围未覆盖，T06 补齐 |
| `expression_learning` / `expression_learning_v1` | 表达候选；`core/expression_learning_extract.go:171` → `core/expression_learning_task.go:87`；有上游 | R·S | 已发送人类消息原文、局部来源引用；不加载生活状态 | schema + 原文连续性/来源校验；外层保存候选，Owner 审核后可供回复参考；保留，T08 验证闭环 |
| `wake_up` / `wake_up_response` | 主动生活/交流；`core/agent_result_adapter.go:965`，调度 `workflow/workflow.go:941`；有上游 | R·A | WakeUp 协议、人格、生活状态/记忆、历史资料及真实触发事件 | 认知决定及授权工具；可以静默，发布走真实工具；保留，T07 验证节奏 |
| `takeover_judge` / `takeover_judge_response` | 接管判断；`core/conversation_runtime.go:116`；非测试生产上游未确认 | G·S | 接管规则、本轮输入/候选、控制与关系事实 | 内部判断，无工具；保留注册，不擅自激活 |
| `takeover_reply` / `takeover_reply_response` | 接管表达；`core/conversation_runtime.go:130`；非测试生产上游未确认 | G·A | caller 指定人格、上下文及 messages；可带授权工具 | 内部 turn contract；保留注册，不擅自激活 |
| `wardrobe_preparation` / `wardrobe_preparation_v1` | 初始衣柜；`workflow/wardrobe.go:50` → `core/wardrobe_preparation.go:315`；有上游 | G·S | 分阶段准备/选择规则、身份偏好、当地时间、身体/实际穿戴及衣物目录 | schema 校验库存和成员引用；外层登记与独立 wear 才实际生效；保留 |
| `initialization` / `initialization_response` | 初始化及 Studio 定义准备；`core/app.go:380`、`core/studio_foundation.go:283` → `core/model_tasks.go:68`；有上游 | G·S | 初始化规则、Owner 原文、预览字段及类型 | 初始化结构和业务校验；外层预览、授权、持久化；保留 |
| `persona_compilation` / `persona_compilation_response` | 人格编译；`core/persona_compilation.go:251`，上游为 persona 持久化及反思；有上游 | G·S | profile、共享身份、规则与必要校验反馈 | Working Persona 候选，校验后外层保存；保留 |
| `media_prompt` / `media_prompt_text` | 图片/头像；`core/media.go:194` → `core/model_tasks.go:130`；头像及重试 `core/model_tasks.go:200`；有上游 | G·S | 冻结拍摄意图、主体、视觉/穿着事实、尺寸；重试含既有 prompt 与质量反馈 | 渲染文本；外层提交真实渲染，不直接发布；归 Image 插件 |
| `media_prompt` / `media_subject_parse` | 媒体主体规划/Studio 图片预览回退；`core/studio_media.go:70` → `core/media_capture_plan.go:451`；有条件上游 | G·S | 冻结意图 packet、明确约束及必要反馈；`core/media_capture_plan.go:438` | 结构化主体/拍摄方案，经 `mediaCapturePlanSchema` 和 Core 校验；不直接渲染；归 Image |
| `media_prompt` / `media_prompt_normalized` | 媒体规范化；`core/model_tasks.go:97` → `core/media_prompt_branches.go:249` → `core/media_capture_plan.go:451`；有上游 | G·S | 规范化 packet、意图/主体/尺寸与校验反馈 | `mediaNormalizationSchema` 结构化结果；同次纠错保留绑定，外层决定渲染；归 Image |
| `media_quality` / `media_quality_acceptance_response` | 媒体审核；`core/media_quality.go:211` → `core/model_tasks.go:274`；有上游 | V·S | 媒体概念、实际 renderer prompt、候选及必要参考真图 | 质量结论；Core 根据结果决定重试、停止或审核，非真实交付；归 Image |
| `visual_identity_vision` / `visual_identity_vision_response` | 视觉观察；`core/model_tasks.go:322`；非测试生产上游未确认 | V·S | 身份快照、视图约束与真图 | 视觉观察结构，无直接发布；保留注册，不擅自激活 |
| `visual_identity_patch` / `visual_identity_patch_response` | 视觉修正；`core/model_tasks.go:344`；非测试生产上游未确认 | G·S | candidate、snapshot、constraints、vision findings | accept/regenerate 候选，无直接发布；保留注册，不擅自激活 |
| `visual_identity` / `visual_identity_agent_response` | 视觉身份；`core/visual_identity.go:1152` → `core/visual_identity_agent.go:200`；有上游 | G·A；视觉阶段可冻结 V | 身份快照、阶段状态、真实审核/历史及图片 | 专用 generate/review/finalize 工具；事务控制候选及身份生效；归 Image |
| `conversation_summary` / `conversation_summary_v1` | 累计摘要；`core/runtime_summary.go:163` → `core/model_tasks.go:375`；有上游 | G·S | 旧摘要、Actor 更正、指定源消息；`conversation-summary.prompt.v2` | `conversationSummaryProviderSchema` 及业务校验；外层保存摘要；保留，纠错见 T09 |
| `conversation_summary` / `conversation_segment_v1` | 片段摘要；`core/conversation_segment.go:300` → `core/model_tasks.go:425`；有上游 | G·S | 参与者、时间范围与指定源消息；`conversation-segment.prompt.v1` | `conversationSegmentProviderSchema` 及版本/范围校验；外层保存片段；保留 |
| `conversation_summary` / `conversation_daily_memory_v1` | 每日对话记忆；`core/conversation_daily_memory.go:261` → `core/model_tasks.go:445`；有上游 | G·S | 本地日期、时区、segments | `conversationDailyEpisodeProviderSchema`；外层来源复核并保存 memory；保留，纠错见 T09 |
| `schedule_generation` / `schedule_response` | 日程；`core/schedule_generation.go:66` → `core/model_tasks.go:535`；有上游 | G·S | 身份、生活、目标、承诺、世界规则、时区及地点候选 | 日程 schema 和领域校验；外层保存计划，不等于到场；保留 |
| `native_cognition` / `native_cognition_response` | 后台生活；`core/agent_result_adapter.go:1296` → `core/model_tasks.go:651`；有上游 | G·A | 人格、生活事实与记忆、本次事件 | 内部认知和作用域内原生工具；真实效果经 Core；保留 |
| `native_cognition` / `owner_life_request_decision` | Owner 生活请求；`core/owner_life_requests.go:426`；有上游 | G·A | 原始请求、冻结目标、授权状态和必要事实 | 决策与限定工具；Core 核验授权/目标并结算；保留 |
| `native_cognition` / `npc_opportunity_decision` | NPC 相遇机会；`core/world_opportunity_cognition.go:32`；有上游 | G·A | 人格、生活与记忆、公开角色/地点/真实机会 | 内部决策及限定的 encounter 工具；保留 |
| `studio_assistant` / `studio_assistant_v1` | Studio；`workflow/studio.go:42` → `core/studio_runtime.go:230`；有上游 | G·S | 隔离的定义 body、创作要求及 allowed references | 结构化定义建议，外层管理草稿；角色定义准备另走 initialization；保留 |
| `studio_text_preview` / `studio_preview_text` | Studio 文本预览；`workflow/studio.go:42` → `core/studio_runtime.go:275`；有上游 | R·S | 虚构预览协议、隔离定义及请求快照 | 文本样本；外层管理预览，不写生活世界；保留 |
| `npc_response` / `npc_response_v1` | NPC 遭遇；`core/world_encounter_service.go:42`；有上游 | R·S | NPC 自身定义、世界规则、已知遭遇、公开来意 | 结构化 NPC 决定；外层校验并控制互动生效；保留 |
| `npc_response` / `npc_moment_response_v1` | NPC 动态互动；`core/moment_npc_interactions.go:132`；有上游 | R·S | NPC 私有定义、世界、已接受遭遇、公开动态正文 | comment/react/pass 等结构化结果；外层控制发布；归 Moment |
| `daily_review` / `daily_review_response` | 每日复核；`core/agent_result_adapter.go:1476` → `core/model_tasks.go:678`；有上游 | G·A | 人格、生活状态与记忆、local_date | 内部日审及 autonomy 工具；真实效果由 Core；保留 |
| `diary_material` / `diary_material_v1` | 日记选材；`workflow/diary.go:42` → `core/diary_runtime.go:231`；有上游 | G·S | 本地日期、时区、真实 source_window | write/source_refs/reason schema 与来源校验；外层保存选材；保留 |
| `diary_expression` / `diary_expression_text` | 日记表达；`workflow/diary.go:42` → `core/diary_runtime.go:302`；有上游 | R·S | 人格、日期、as_of 和选定真实素材 | 自然正文；外层保存，不直接改变生活事实；保留 |
| `persistent_switch` / `persistent_switch_assessment` | Owner 人格切换；`core/owner_persona_execution.go:89` → `core/model_tasks.go:702`；有上游 | G·S | 切换规则、人格、生活/记忆/历史、本轮输入和候选 | keep/switch 判断；Owner 工作流复检版本后决定生效；旧包装上游未确认，保留有效路径 |
| `reflection` / `reflection_proposal_v2` | 反思；`core/reflection_runtime_v2.go:167` → `core/model_tasks.go:751`；有上游 | G·S | 人格、范围内证据、生活状态和记忆 | 反思 proposal；外层校验及持久状态结算；保留 |
| `schedule_replan` / `schedule_replan_plan` | 日程重排；`core/schedule_capability.go:74` → `core/model_tasks.go:774`；有上游 | G·S | 冻结意图、现有日程、生活和世界/地点事实 | 替换日程候选及原领域校验；外层 Capability 执行；保留 |
| `goal_planner` / `goal_planner_v1` | Goal 规划；`core/goal_planner.go:532` → `:301`；有上游 | G·A | 规划协议、run/mode、目标集快照及 query/commit 回执 | `goal_planner.query/commit`；实际提交受事务、依赖和原成功条件约束；保留 |
| `goal_evaluation` / `goal_evaluation_v1` | Goal 评估；`core/goal_evaluation_runtime.go:478`、纠错 `:500` → `core/goal_evaluation_task.go:262`；有上游 | G·A | 人格、选定目标/criteria、来源、审查和已接受提交 | 专用 evaluation/object/plan submit 工具；Core 校验来源/成功条件；保留 |
| `virtual_activity_result` / `virtual_activity_result` | 虚拟活动；`core/life_activity_capabilities.go:493` → `core/model_tasks.go:500`；有上游 | G·S | 冻结请求、外貌/生活、近期结果和时间窗口 | 活动结果结构；外层结算购物/理发等真实状态；保留 |
| `world_event_generation` / `world_event_generation_v1` | 世界事件；`workflow/world_events.go:74` → `core/world_event_generation.go:303`；有上游 | G·S | 世界规则/时区、真实地点与角色、近期事件 | 事件候选或 no-event；独立 Kev 准入及 Core 事务后才生效；保留 |

## 非注册调用、辅助服务和恢复

| 入口或阶段 | caller、配置、输入及输出 | 状态与处理 |
| --- | --- | --- |
| 会话记忆查询 embedding | `core/memory_retrieval.go:448` → `core/model_tasks.go:781` → `core/provider.go:1190`；每次选 `embedding` binding，输入为检索文本，输出向量供查询 | 生产源码有上游；独立 embedding，不计为 FormalAgent 或角色文案；保留 |
| 持久 memory embedding | `core/memory_embedding.go:54`、`:119` → `core/model_tasks.go:792` → `core/provider.go:1201`；可按已冻结 endpoint/model 恢复，输入记忆文本，输出向量 | 生产源码有上游；与查询 embedding 的绑定策略不同；保留 |
| Owner 显式视觉探针 | `httpapi/kev.go:43`、`httpapi/browser_backend.go:239` → `core/vision_probe.go:15`、`:34`；独立视觉绑定，发送生成的真实双色图及问题，核对两个颜色 | 是诊断入口，可产生真实模型 I/O；不是业务 Agent，不代表真实生图能力；保留 |
| 日程规划裸 Provider 后备 | `core/schedule_capability.go:72`、`:84`；`runner == nil` 时直接 `StructuredWithSchema`，输入重排事实和规则，输出替代日程 DTO | 正常 `core/app.go:144` 装配 runner 并走正式 Agent；后备实现存在，正常部署启用未确认；保留并标明 |
| Kev 业务决策 | `core/kev_decisions.go:59`、`:79` → `ai/decision/decision.go:427`、`:456`；独立 settings/secret 与 `{state,questions}` HTTP 协议，输出选择/准入并记录诊断 | 独立于文本模型路由；包括 context/tools selection、人格 gate、自动/媒体 gate、世界事件准入；保留 |
| Kev 连接诊断 | `httpapi/kev.go:30` → `core/kev_diagnostics.go:100`、`:116`；Owner 显式发起 tools.select 测试 | 真实外部 I/O 诊断，不当作业务决策或正式 Agent；保留 |
| Writer 格式、风格、动作纠错 | `core/reply_failure_policy.go:181`、`core/reply_style.go:207`、`core/reply_action_consistency.go:79` → `core/reply_provider.go:227` | 使用已冻结的任务上下文及规则；属于 Writer variant 的额外物理请求，不是新 Agent；保留 |
| 原生循环纠错与传输重试 | `core/eino_model_runtime.go:150`、`:172`、`:190`、`:1090`；`core/provider_upstream_retry.go:111` | 包括上游重试、Kev gate 再生成、发布遗漏纠正和最终结构校验修复；沿既有 assignment/作用域，不把它们计为新的 Agent；保留真实失败证据 |
| 领域任务的校验重试/恢复 | `core/persona_compilation.go:251`、`:255`；`core/goal_evaluation_task.go:302`；`core/reply_continuation.go:144`；`core/workflow_model_snapshot.go:38` | 人格/Goal 可带真实反馈重跑；回复、Diary/Studio 等按各自已存阶段绑定恢复。不同后台任务跨进程快照尚需 T02 验证 |
| 底层文本流 API | `core/provider.go:1142` → `core/eino_model_runtime.go:1337` | 已有实现但本次未找到非测试上游；不能据注册/实现声称正在生产使用 |

物理传输汇合在 `ai/model/eino_runtime.go:42` 的 Chat model factory、`core/eino_model_runtime.go:920` 的普通 Generate、`:1030` 的 ADK、`:1337` 的 Stream 和 `core/provider.go:1201` 的 embedding。Kev 自行使用 HTTP，不经过 Eino。图片渲染的 `core/media.go:1001` 是 ComfyUI `/prompt`，是独立渲染能力，不是文本 LLM；模型列表 GET 和 renderer `/system_stats` 也不是推理。测试 fixture 和 `_test.go` 中的直接 HTTP 不计为生产入口。

## 工具发现、独立调用与发布

1. `core/app.go:151` → `core/builtin_capabilities.go:79` 建立 Core 注册；`core/domain_plugins.go:27` 声明 Image/Moment 并复用该 registry。私有 Writer `reply.image.request` 由 Writer 构造，不是全局注册。
2. `core/kev_tools.go:60` 的 `capability.catalog` 按作用域区分可加载、禁用或不可用；`:152` 的 `capability.discover` 发现授权 schema。模型的原生定义和绑定在 `core/native_tool_contract.go:16`、`:26`、`:47`；不可把目录可见当作执行授权。
3. Agent 调用从 `core/adk_conversation_runtime.go:339` 进入 `core/tool_execution.go:108` 的 `App.ExecuteTool`；独立调用也使用该正式入口。它核对资源所有者、私有调用、真实 Provider/ToolCall 身份和任务作用域；写入边界与 policy 校验分别见 `core/tool_execution_ledger.go:74`、`core/tool_authorization.go:13`。
4. `conversation.reply`、`moment.publish`、`media.image.generate` 的 direct 入口分别见 `core/builtin_capabilities.go:240`、`:286`、`:477`。实际消息发布在 `core/tool_publication.go:72`，动态发布与 outbox 在 `:324`，图片创建异步 intent 在 `:378`、`:438`。`accepted` 图片意图不等于生成、质量通过或 delivered。
5. Image 声明包含 `media.image.generate`、`reply.image.request` 和 visual identity 专用能力；Moment 声明 `moment.publish`、`moment.inspect` 与 NPC 动态交互，见 `core/domain_plugins.go:27`。插件健康目前返回 `not_checked`，后续 T05/T11 处理。

非 Agent 的正式 `ExecuteTool` 调用方也要计入：`core/kev_persona.go:149` 提交经 Kev 选择的人格切换；`core/scheduled_intention_runtime.go:99`、`core/scheduled_intention_runtime.go:171` 处理已调度意图；`core/owner_visual_review.go:45` 提交 Owner 视觉审核；`core/wardrobe_preparation.go:517` 执行已选择衣物；`core/activity_resolution_lifecycle.go:96` 处理活动后的实际换衣；`core/workflow_ops.go:175` 从持久工作流执行能力。它们与 `core/adk_conversation_runtime.go:339` 的模型工具调用复用同一授权及回执边界，但各自的业务入口、作用域和触发条件不同。

基础工具还包含 Goal、scene/presence、memory/recall、affect、promise、Actor、persona、衣柜/物品、habit/intention、日程/活动、appearance 和 relationship 等，完整实例注册及追加入口以 `core/builtin_capabilities.go:131`–`:161` 为准；Goal Planner、Goal Evaluation 和视觉身份还有各自专用工具。不能把 registry 中出现的能力都视为任意 Agent 实际拿到的工具。

## 后台执行与重新核查

Worker 入口 `cmd/worker/main.go:27`；dispatcher、回复续接、outbox 和事件消费者的循环在 `:157`、`:188`，定期 intents 补齐在 `:265` 起。Temporal 工作流和 activity 注册在 `workflow/workflow.go:1333`，`DispatchOnce` 在 `:2347`。这些路径覆盖唤醒、日审、日程、衣柜、Studio、日记、反思、摘要、Goal、embedding、图片和续接；静态注册不证明实际 worker 正在运行。

本次复检范围：`formal_agents.go` 与 typed task 的非测试 caller；`RunFormalAgent`、`runFormalStructuredTask`、`StructuredWithSchema`、`completeWithToolsSchema`、`Generate`、`Stream`、`EmbedStrings`、`NewChatModel`、`NewEmbedder`；Capability、Kev 和 worker 的生产代码。可按下面命令复查，排除 `_test.go` 并单独审阅匹配上下文：

```sh
rg -n --glob '*.go' --glob '!**/*_test.go' '(RunFormalAgent\(|runFormalStructuredTask\(|StructuredWithSchema\(|completeWithToolsSchema\(|RunEmbeddingTask\(|RunFrozenEmbeddingTask\()' apps/core-go/internal
rg -n --glob '*.go' --glob '!**/*_test.go' '(\.Generate\(|\.Stream\(|\.EmbedStrings\(|NewChatModel\(|NewEmbedder\(|chat/completions|/embeddings)' apps/core-go
rg -n '(retryUpstreamModelCall|regenerate|publicationCorrection|final-repair)' apps/core-go/internal/core/eino_model_runtime.go apps/core-go/internal/core/provider_upstream_retry.go
rg -n --glob '*.go' --glob '!**/*_test.go' '\.ExecuteTool\(' apps/core-go/internal apps/core-go/cmd
rg -n '(NewRequestWithContext|\.Do\()' apps/core-go/internal/ai/decision/decision.go
rg -n '(DispatchOnce|DispatchReplyContinuations|ReconcileOnce|PublishOnce|ConsumeOnce|RegisterWorkflow|RegisterActivity)' apps/core-go/cmd/worker/main.go apps/core-go/internal/workflow/workflow.go
```

当前未知项：未查询真实业务库的 model ID、绑定/凭据版本、配置开关或运行中 worker；未证明全部后台任务都具有跨进程配置快照，也未执行真实模型、渲染或 UI 端到端验收。这些属于 T02–T12 的实现或验证，不把源码盘点当作完成证明。
