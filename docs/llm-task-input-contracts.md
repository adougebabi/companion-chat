# Agent 与任务变体输入契约

本表记录当前代码的任务边界，供组装、诊断和验收对照；它不是新的工具授权，
也不表示真实模型语义效果已通过。文件名除特别注明外均在
`apps/core-go/internal/core/`。必需列指任务必须具备的业务资料，集合允许为空
的场景由原有领域校验决定；不应自动输入列不禁止调用方明确提供该任务有权
消费的来源证据。`无`表示该任务不自动加载或安装，不表示人物没有这种资料。

人格、历史、状态列依次描述：人格来源；真实聊天历史范围；当前状态范围。
所有 `surface` 工具都以调用方实际安装集合为上限，再受插件、Kev 和权限约束；
注册表的 `MandatoryTools` 只是已安装工具收窄时的保留交集，不授予新能力。
结果为内部候选时，只有对应 Core 校验和真实工具回执才能产生业务效果。

| Agent | 变体 | 必需输入 | 可选输入 | 不应自动输入 | 来源 | 人格；真实历史；当前状态 | 工具 | 输出与消费方 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `conversation_cognition` | `conversation_turn_response:fact_only` | 授权主体、运行身份、本次输入和作用域 | 关系、记忆、摘要、已授权动作事实 | 无关会话、伪造回执、父文本作为事实 | `cognition_agent.go` 的授权投影与当前输入 | 当前 Working Persona；授权会话完整组；按事实视图选择当前状态 | 实际安装的 conversation surface | 内部认知与真实工具回执交 Core；回复工具才发布 |
| `conversation_cognition` | `conversation_turn_response:legacy` | 授权主体、本次输入和作用域 | 范围内关系、记忆、摘要 | 无关会话及未经授权状态 | `cognition_agent.go`、`provider_context.go` 的一般投影 | 当前 Working Persona；授权会话历史；任务范围内状态 | 实际安装的 conversation surface | 内部认知；实际发布依真实工具 |
| `conversation_cognition` | `conversation_turn_response:direct` | 新主私聊的授权输入、范围及本轮 decision | 冻结的已审核表达建议、已完成工具结果 | 把文字或计划当完成回执 | `cognition_agent.go`、`direct_main_reply.go` | 当前 Working Persona；授权会话历史；私聊事实视图 | 实际 conversation surface，含授权的 `conversation.reply` | 自拟正文与 decision 经 Core 同事务校验和结算 |
| `conversation_reply` | `private_reply` | 有效 Working Persona、当前输入和授权投影 | 范围内记忆、摘要、真实动作回执、已审核表达 | 父模型推理、工具参数、代写正文或策略 | `reply_context.go`、`fact_only_reply_capability.go` 的独立投影 | 当前 Working Persona；授权会话历史；相关事实与能力状态 | 条件安装 `reply.image.request`、`persona.detail` | 未发布草稿交外层核验和发布 |
| `conversation_reply` | `proactive` | 已授权主动事件及有效人格 | 相关记忆、历史、冻结表达建议、真实回执 | 将旧消息冒充新用户输入、父模型拟文 | `reply_context.go`、`reply_provider.go` | 当前 Working Persona；范围内真实历史；相关当前状态 | 按私聊 Writer 条件安装的私有工具 | 未发布主动消息草稿交外层核验 |
| `conversation_reply` | `moment` | 已授权动态任务及有效人格 | Owner 明确原文、相关事实与历史 | 父模型代写、未授权私聊、虚构附件 | `fact_only_moment_capability.go`、`reply_context.go` | 当前 Working Persona；范围内历史仅作资料；相关事实 | 无 | 动态正文候选交发布流程 |
| `conversation_reply` | `private_reply.style_repair` | 原私聊上下文、未发布候选、修复规则 | 真实媒体请求状态 | 重新执行已有媒体效果 | `reply_style.go`、`reply_action_consistency.go` | 原有效人格；原授权历史；原真实状态 | 无 | 修复草稿须重新校验 |
| `conversation_reply` | `proactive.style_repair` | 原主动事件、未发布候选、修复规则 | 真实媒体请求状态 | 把历史当新消息、重做已有媒体效果 | `reply_style.go`、`reply_provider.go` | 原有效人格；原授权历史；原真实状态 | 无 | 修复草稿须重新校验 |
| `conversation_reply` | `moment.style_repair` | 原动态任务、未发布候选、修复规则 | 真实动作状态 | 重做发布或媒体效果 | `reply_style.go`、`reply_provider.go` | 原有效人格；原授权历史；原相关状态 | 无 | 修复草稿须重新校验 |
| `conversation_reply` | `private_reply.correction` | 原私聊上下文和格式反馈 | 已有候选或媒体结果，视实际路径 | 伪造已交付结果 | `reply_failure_policy.go`、`reply_provider.go` | 原有效人格；原授权历史；原相关状态 | 按 `replyWriterDefinitions`；已有候选或媒体结果时关闭 | 格式修正草稿交正常校验 |
| `conversation_reply` | `proactive.correction` | 原主动事件和格式反馈 | 已有候选或媒体结果，视实际路径 | 把历史当当前用户命令 | `reply_failure_policy.go`、`reply_provider.go` | 原有效人格；原授权历史；原相关状态 | 按 `replyWriterDefinitions`；已有候选或媒体结果时关闭 | 格式修正草稿交正常校验 |
| `conversation_reply` | `moment.correction` | 原动态任务和格式反馈 | 已有候选或动作结果 | 重做发布 | `reply_failure_policy.go`、`reply_provider.go` | 原有效人格；原授权历史；原相关状态 | 无 | 格式修正草稿交正常校验 |
| `reply_action_check` | `reply_action_consistency` | 未生效换装回执、当前状态、未发布候选 | 无 | 通用人格、记忆和聊天 | `reply_action_consistency.go` 的真实回执与候选 | 无；无；当前动作相关状态 | 无 | 内部一致性判断，外层决定修复或停止 |
| `reply_action_check` | `direct_media_delivery_check_v1` | 未发布私聊或无当前附件的动态候选、当前输入、同作用域已提交的私聊媒体交付；旧消息后补附件的交付时间为未知 | 已授权会话历史 | 原始数据库身份、其他会话资料、媒体执行或发布权限 | `direct_media_delivery_check.go` 的候选与 Core 交付投影 | 无；仅用于解释指代的历史；无完整生活状态 | 无 | 内部提取声明和局部引用；Core 重新核验具体交付并决定发布。历史私聊交付不能证明新动态已有附件 |
| `expression_learning` | `expression_learning_v1` | Owner 选取的已发送 Human 消息原文与局部引用 | 适用情境 | 通用人格、关系、生活、其他聊天 | `expression_learning_extract.go`、`expression_learning_task.go` | 无；仅指定源消息；无 | 无 | 原文摘录候选交 Core 校验和 Owner 审核 |
| `wake_up` | `wake_up_response` | 真实周期事件、授权角色与任务范围 | 目标交接、记忆、摘要和范围内历史 | 将历史当新用户输入、无依据行动结果 | `agent_result_adapter.go` 的唤醒事件和投影 | 当前 Working Persona；范围内历史作为历史；生活与任务状态 | 实际安装的 WakeUp surface | 内部认知或静默；真实动作由工具产生 |
| `takeover_judge` | `takeover_judge_response` | caller 提供的本轮输入、候选和规则 | recent references、关系与控制事实 | 全量人格 roster、无关记忆和工具 | `turn_takeover.go`、`conversation_runtime.go`；生产上游未确认 | 无自动人格；仅 caller 提供的参考；仅 caller 提供的事实 | 无 | 内部接管判断，不直接切换或发布 |
| `takeover_reply` | `takeover_reply_response` | caller 提供的 messages 和输出契约 | 人格、历史、状态及 definitions 由 caller 提供 | 自动构造未授权投影 | `conversation_runtime.go`；生产上游未确认 | 取决于 caller；取决于 caller；取决于 caller | caller 安装且通过 conversation 授权的工具 | 内部 turn contract；效果依真实工具 |
| `wardrobe_preparation` | `wardrobe_preparation_v1:generate` | 已选角色资料、当前身体、库存与方案 | 本地时间、实际场景、声明的穿搭偏好 | 混合社交策略的旧卡全文、旧库存和通用聊天 | `wardrobe_preparation.go`、`wardrobe_preparation_prompt.go` | 相关身份性格和偏好；无；实际身体和穿戴引用 | 无 | 库存/方案候选交 Core 校验登记，不执行换衣 |
| `wardrobe_preparation` | `wardrobe_preparation_v1:select` | 真实库存、方案、当前身体和穿戴引用 | 本地时间、实际场景、声明的穿搭偏好 | 旧卡混合正文、生成阶段权限、通用聊天 | `wardrobe_preparation.go`、`wardrobe_preparation_prompt.go` | 相关身份性格和偏好；无；实际身体和穿戴 | 无 | 选中引用交 Core 校验，独立 wear 才生效 |
| `initialization` | `initialization_response` | Owner 原始描述、预览字段与类型 | 所选 Studio 定义 | 现有角色生活、对话和当前人格选择 | `model_tasks.go`、`studio_foundation.go` | 输入中的初始描述；无；无自动运行状态 | 无 | 初始化结构交预览、校验和授权流程 |
| `persona_compilation` | `persona_compilation_response` | Core Persona、指定 profile、共享身份 | 有效 overlay、稳定习惯、真实校验反馈 | 其他 profile、当前情绪/穿着/场景和聊天 | `persona_compilation.go` | 指定人格资料；无；不加载瞬时状态 | 无 | Working Persona 候选经校验后保存 |
| `media_prompt` | `media_subject_parse` | 原始图像意图和明确约束 | 指代绑定、已有方案、校验反馈 | 自动加载本人外貌、人格、图片或聊天 | `media_capture_plan.go`、`studio_media.go` | 无；无；无自动生活状态 | 无 | 主体/拍摄方案交 Core 校验并冻结 |
| `media_prompt` | `media_prompt_normalized` | 冻结意图、拍摄方案、明确约束 | 尺寸、获准主体/视觉资料、校验错误 | 无关人格聊天、非可见主体资料 | `media_prompt_branches.go`、`media_capture_plan.go` | 无自动人格；无；仅获准可见主体状态 | 无 | 结构化 prompt 交 Core 校验与 renderer；质量重试反馈缺口见独立 bug |
| `media_prompt` | `media_prompt_text:media` | 冻结媒体概念及约束 | 可见主体资料、此前 prompt 和真实质量反馈 | 无关人格和聊天 | `model_tasks.go`、`media_quality.go` | 无自动人格；无；仅 concept 冻结的相关状态 | 无 | 渲染提示词交媒体流程 |
| `media_prompt` | `media_prompt_text:avatar` | 冻结角色设定与已确认外貌 | 视觉偏好、此前 prompt 和审核反馈 | 其他聊天和未授权主体状态 | `generated_avatar.go`、`model_tasks.go` | 选定角色的身份、性格和偏好；无；已确认外貌 | 无 | 英文头像提示词交媒体流程 |
| `media_quality` | `media_quality_acceptance_response` | 冻结概念、实际 renderer prompt、候选真图 | 已确认参考图、重试情况 | 通用人格和聊天 | `media_quality.go`、`model_tasks.go` | 无；无；冻结的相关视觉事实 | 无 | 内部视觉 verdict 交 Core 决定重试、停止或审核 |
| `visual_identity_vision` | `visual_identity_vision_response` | 身份快照、视图约束、候选真图 | renderer constraints | 通用人格、聊天和生活查询 | `model_tasks.go`；生产上游未确认 | 无；无；视觉快照 | 无 | 内部视觉观察；生产消费方未确认 |
| `visual_identity_patch` | `visual_identity_patch_response` | candidate、身份快照、constraints、视觉观察 | 无 | 通用聊天和生活查询 | `model_tasks.go`；生产上游未确认 | 无；无；视觉快照 | 无 | 内部 accept/regenerate 候选；生产消费方未确认 |
| `visual_identity` | `visual_identity_agent_response` | 持久 session、身份快照、当前阶段 | 尝试历史、真实审核与候选图片 | 通用聊天和无关人格 | `visual_identity_agent.go` 的持久状态 | 视觉身份资料；无；当前任务阶段 | 按阶段仅安装 generate、commit_review 或 finalize 对应工具 | 内部阶段结果和真实工具回执交 Core |
| `conversation_summary` | `conversation_summary_v1` | 指定未覆盖消息窗口 | 旧摘要、Actor 更正 | 通用额外聊天、人格和附件本体 | `conversation_summary.go`、`runtime_summary.go` | 无；仅指定来源消息；无 | 无 | 累计摘要经来源/版本校验后保存 |
| `conversation_summary` | `conversation_segment_v1` | 指定源消息、参与者、时间范围 | 角色显示名 | 其他聊天、人格和生活状态 | `conversation_segment.go`、`model_tasks.go` | 无；仅指定完整消息组；无 | 无 | 片段摘要经范围校验保存 |
| `conversation_summary` | `conversation_daily_memory_v1` | 关闭后的当地日期、时区、有效 segments | 无 | 额外原始聊天、人格和生活状态 | `conversation_daily_memory.go` 的已校验片段 | 无；仅片段来源；无 | 无 | episode 经来源复核后保存为私有记忆 |
| `schedule_generation` | `schedule_response` | 当地日期/时区、身份、生活、当前状态、目标与地点上下文 | 承诺、住所及共享世界资料 | 原始聊天、推定住所或已完成动作 | `schedule_generation.go`、`model_tasks.go` | 稳定身份/偏好；无；当前生活和内在状态 | 无 | 日程候选经领域校验后接收，计划不证明到场 |
| `native_cognition` | `native_cognition_response` | 授权投影、本次事件及事实 | 相关记忆、事件专属规则 | 通用原始聊天、未证实动作 | `agent_result_adapter.go`、`model_tasks.go` | 当前 Working Persona；无自动聊天；相关生活状态 | 实际 native-cognition surface | 内部认知与真实回执交 Core 结算 |
| `native_cognition` | `owner_life_request_decision` | 已认证 Owner 的冻结请求和目标 | 所需物品/地点及真实状态 | 其他目标、CAS 和运行身份、无关聊天 | `owner_life_requests.go` 的请求视图 | 按该请求提供；无自动聊天；所需生活事实 | 私有 catalog/检查工具与该请求类型的动作 | 内部决策和真实回执交 Core，accepted 不等于完成 |
| `native_cognition` | `npc_opportunity_decision` | 有效机会、版本、地点、同场证据和时间 | NPC 公开角色及已认识状态、相关记忆 | NPC 私有定义、虚构对话或 Owner 同场 | `world_opportunity_cognition.go` | 当前 Working Persona；无自动聊天；相关生活状态 | 仅 `world.npc.encounter` | 内部决策；真实 contact/accept 回执才建立认识 |
| `studio_assistant` | `studio_assistant_v1` | 隔离的资源 kind、name、body 和创作指令 | 允许引用、purpose | 角色运行生活、记忆、聊天和未授权资源 | `studio_runtime.go`、`prompt_semantic_views.go` | 定义资料而非运行人格；无；无 | 无 | 定义建议交 Studio 草稿，不写生活 |
| `studio_text_preview` | `studio_preview_text` | 隔离定义和预览请求 | 允许引用 | 真实生活、记忆和聊天 | `studio_runtime.go` | 虚构定义；无；无 | 无 | 虚构文本预览交 Studio |
| `npc_response` | `npc_response_v1` | NPC 自身定义、世界规则、公开来意和机会 | 该 NPC 的真实既往相遇 | Fluctlight 私有人格/记忆及虚构 Human 同场 | `world_encounter_response.go` | 该 NPC 定义；无通用聊天；NPC 当前活动 | 无 | 内部 accept/decline，外层复核后处理相遇 |
| `npc_response` | `npc_moment_response_v1` | NPC 自身定义、已接受相遇、公开动态正文 | 世界规则和公开互动资料 | Fluctlight 私有聊天、图片观察和虚构同场 | `moment_npc_interactions.go` | 该 NPC 定义；无通用聊天；已确认相遇事实 | 无 | comment/react/pass 候选交外层复核发布 |
| `daily_review` | `daily_review_response` | 当地日期、已接受日程、授权投影 | 相关生活和记忆 | 通用原始聊天、未执行结果 | `agent_result_adapter.go`、`model_tasks.go` | 当前 Working Persona；无自动聊天；相关生活状态 | 实际 autonomy surface | 内部日审和真实回执交 Core |
| `diary_material` | `diary_material_v1` | 关闭的本地日期、时区和真实 source window | 来源消息或已确认事件 | 人格、额外聊天、未发生经历 | `diary_runtime.go`、`diary_sources.go` | 无；仅指定当天来源；无自动状态 | 无 | 来源引用候选经 Core 校验保存 |
| `diary_expression` | `diary_expression_text` | Working Persona、日期/时区、as_of、选定真实素材 | 无 | 额外聊天、选材模型的草稿、虚构经历 | `diary_runtime.go` 的已选材料 | 当前 Working Persona；仅选定材料；无额外生活状态 | 无 | 日记正文经来源复核后保存 |
| `persistent_switch` | `persistent_switch_assessment` | 已声明切换规则、目标请求及授权投影 | 当前状态、记忆、摘要和范围内历史 | 无关人格、未声明目标和工具 | `owner_persona_execution.go`、`model_tasks.go` | 当前 Working Persona；范围内历史；相关状态 | 无 | keep/switch 判断交 Core 复核后生效 |
| `reflection` | `reflection_proposal_v2` | 本次来源窗口及证据引用 | 相关人格、记忆、生活状态 | 公共当前穿着、进行中活动和额外聊天 | `reflection_runtime_v2.go`、`model_tasks.go` | 当前 Working Persona；仅明确来源证据；相关非衣柜状态 | 无 | 内部多域候选交 Core 校验与结算 |
| `schedule_replan` | `schedule_replan_plan` | 冻结意图、当前日程、生活、时区和地点上下文 | 精确目标编辑、Owner 请求、既有承诺 | 通用聊天、虚构地点或已执行事实 | `schedule_capability.go`、`model_tasks.go` | 相关身份/偏好；无；当前日程及生活 | 无 | 替代计划交 Capability 复核并提交 |
| `goal_planner` | `goal_planner_v1` | run/mode、目标集快照 | query 返回的目标、承诺、人格和关系资料 | 全量私聊、虚构进度或证据 | `goal_planner.go` 的冻结快照和真实 query 回执 | 初始人格摘要，query 可取完整资料；无自动聊天；按 query 提供 | `goal_planner.query`、`goal_planner.commit`，按模式及权限 | 建议或真实提交结果交 Core 读回 |
| `goal_evaluation` | `goal_evaluation_v1` | 冻结目标、原 criteria、授权引用和当前目标状态 | 来源证据、review、已接受提交及纠错反馈 | 未提供引用、通用聊天、虚构成功 | `goal_evaluation_task.go`、`goal_evaluation_wire.go` | 当前 Working Persona；仅指定消息证据；相关状态 | evaluation、object、plan 的专用 submit 工具 | 真实提交回执交 Core 校验覆盖 |
| `virtual_activity_result` | `virtual_activity_result` | 已启动且到期活动、冻结请求、开始/最早结束时间 | 外貌、生活和近期结果 | 人格聊天、虚构现实支付或未到期完成 | `model_tasks.go`、`life_activity_capabilities.go` | 无；无；活动相关外貌和生活 | 无 | 活动结果候选交 Capability 校验结算 |
| `world_event_generation` | `world_event_generation_v1` | 世界规则/时区、真实地点与 NPC、当前时间及在场角色 | 近期已发生事件 | 任一角色私有人格/聊天、推定同场 | `world_event_generation.go` 的世界快照 | 无；无；世界事件与实际在场 | 无 | no-event 或候选，经 Kev 准入及 Core 复核后生效 |

`media_prompt_normalized` 的质量重试没有看到上一轮反馈的静态缺口已单独登记在
`media-normalization-quality-retry-feedback`，本表不把期望写成现有行为。

## 非正式 Agent 的模型入口

- Kev 的 context/tools 选择、人格及自动/媒体 gate 使用各自候选和 `{state,questions}`，
  不自动获取完整私聊；它们不授予超出安装和授权范围的能力。
- embedding 只接收指定检索文本或待持久化记忆文本，输出向量给检索或存储；不使用聊天人格。
- 视觉探针是 Owner 显式发起的诊断图片与问题，不代表业务图片已通过审核。
- 结构化最终纠错、Writer 纠错、人格/媒体校验重试复用原任务的冻结范围和真实反馈；
  工具权限以该具体纠错路径为准，不能统一假定所有纠错都有或都没有工具。
