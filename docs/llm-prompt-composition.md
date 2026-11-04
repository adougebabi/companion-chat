# LLM 提示词组成清单

更新：2026-10-30。代码权威是 `apps/core-go/internal/core/formal_agents.go`
与各 typed task，规范在 [Prompt Composition Contract](../.ael/knowledge/spec/legacy-9b89c41b4be449b963dcba95.md)。

## 统一构造

所有正式 Agent 共享一个前置 `# 运行协议` system 边界。已有任务协议不叠加
第二套协议；相同规则去重；没有人格资料的分析任务不加入空人格段。
结构化业务数据统一 JSON，稳定任务资料可单独放在运行时事实之前；实际用户
输入保持原文一次。原生 Tools、ToolCall/ToolResult、response schema 和图片
保持原生结构。传输层不根据用户文本形状转换 JSON/YAML/TOON。`ComposeTaskMessages` 只对
调用方显式标记的 `promptTaskData` 进行资料提取/编码，Owner 的 JSON 描述仍是原文。

每个任务按下表选择资料。统一构造并不表示所有任务共享整套摇光状态、记忆
和私聊；共享 Agent ID 的不同子任务仍各有独立资料范围。

## 记忆与历史数量

表中 P 为该任务协议，D 为所需生活事实，M 为记忆，S 为累计摘要，H 为近期对话。

- D：按任务选择主体/时间/当前身体与穿着/情绪/场景/活动/地点/关系/目标/日程/
  近期结果；当前事实不被人格偏好或旧聊天覆盖。
- M：活动记忆查询最多 30 条；长期检索私聊 6 条、WakeUp/DailyReview 8 条、
  其他操作 12 条；常驻记忆来自已发布且有效的快照。这些是既有语义查询契约，
  没有改成 token 配额。
- S：0 或 1 条已持久化累计摘要，附内部源覆盖范围；摘要文本产品长度设置保持
  原样。只有实际选入且有效的覆盖才能代替原始消息。
- H：有会话时通常读取最近 200 条候选，必要时回补窗口前真实触发 user；排除
  当前输入和已覆盖完整交互组。最终数量可变，并非固定 10 句。私聊保护最新
  两个完整交互组，不表示只发送两组。正文/顺序/角色/发送时间保持。
- 同来源不同陈述不合并；半组摘要覆盖保留整个交互组。WakeUp/Moment 的 H
  是 `historical_conversation` 数据，不模拟当前用户发来旧请求。
- 表中“0”表示不追加通用私聊；Goal、日记、摘要的显式证据可包含源消息。

## 30 个正式 Agent

表内文件均在 `apps/core-go/internal/core/`；这些文件与函数可用于定位具体入口。

| Agent ID | 当前提示词组成 | 通用原始聊天 | 入口/构造位置 |
| --- | --- | --- | --- |
| world_event_generation | 世界规则/时区/真实地点与 NPC/实际在场角色/近期已发生事件；只提议或 no_event，独立 Kev 审批后才可入可执行库 | 0 | world_event_generation.go |
| conversation_cognition | 私聊认知协议 + Working Persona + 私聊 D/M/S + H + 当前输入；legacy 与 fact-only 投影不同 | H | cognition_agent.go；conversation_runtime.go |
| conversation_reply | 私聊表达协议 + Working Persona + 独立事实视图/能力边界/实际 Tool facts + M/S + H + 当前输入 | H | reply_context.go；reply_provider.go |
| reply_action_check | 换装未生效时核验未发送正文；实际动作回执 + 当前状态 + 候选正文，无人格扮演，无工具 | 0 | reply_action_consistency.go |
| wake_up | 专属完整唤醒协议 + Working Persona + D/M/S + 历史资料 H + 本次唤醒事件 | H，作为历史数据 | agent_result_adapter.go；capability_prompt_policy.go |
| takeover_judge | Judge 协议 + 本轮 user/candidate/接管规则/控制与关系事实 + 可选 recent references | 无全量 H/M | turn_takeover.go；conversation_runtime.go |
| takeover_reply | 接管任务规则 + caller 提供的选定人格、上下文及 messages + 授权 Tools | 取决于 caller | conversation_runtime.go |
| wardrobe_preparation | 分阶段准备/选衣规则 + 穿搭所需身份/性格/偏好 + 本地时间/当前情境/实际身体与穿戴引用 + 唯一衣物目录/方案成员引用 | 0 | wardrobe_preparation.go；wardrobe_preparation_prompt.go |
| initialization | 初始化解析协议（显式完整姓名包含前缀/连字符，不拆为昵称） + Owner 描述一次 + 预览字段、类型和输出结构提示；自然语言分析规范化后、持久化前校正明确姓名，保留 JSON 导入字段 | 0 | app.go；model_tasks.go；initialization_name.go |
| persona_compilation | 人格编译协议 + 选定 profile 和共享身份；必要时校验反馈 | 0 | persona_compilation.go |
| media_prompt | 媒体任务规则 + 冻结意图/主体/拍摄方案/视觉事实；重试附此前 prompt 和质量反馈 | 0 | model_tasks.go；media_capture_plan.go；media_prompt_branches.go |
| media_quality | 质量检查协议 + 冻结媒体概念/实际 renderer prompt/重试情况 + 图片；角色设定图和头像的检查同时携带已确认参考图与待审候选图，只有真实通过才自动接受 | 0 | media_quality.go；model_tasks.go |
| visual_identity_vision | 视觉检查协议 + identity snapshot/视图要求/renderer 约束 + 图片 | 0 | model_tasks.go |
| visual_identity_patch | 修正决策协议 + candidate/snapshot/constraints/vision findings | 0 | model_tasks.go |
| visual_identity | 视觉身份流程规则 + 身份快照/任务状态/尝试历史/前次审查的真实观察、评分与修正建议 + 必要图片；参考图与设定图共用角色卡模板，头像使用单人肖像模板，重新生成携带对应评审反馈 | 0；尝试历史是任务历史 | visual_identity_agent.go；visual_identity.go |
| conversation_summary | 累计：旧摘要+Actor 更正+源消息；片段：真实参与者/说话人别名与名称+时间范围+源消息，显式 JSON 合同；daily：日期/时区+segments | 指定源范围，非通用 H | conversation_summary.go；model_tasks.go |
| schedule_generation | 排程规则 + 含已声明住所的 identity/life/goals/intentions/outcomes/承诺 + 共享世界规则/时区/真实地点候选 | 0 | model_tasks.go；schedule_places.go |
| native_cognition | 认知协议 + Working Persona + D/M + 本次事件/事实 | 0，无 S | model_tasks.go |
| studio_assistant | 定义编辑协议 + 隔离的定义 body/创作要求/允许引用（移除 job 身份） | 0，无生活 M | studio_runtime.go |
| studio_text_preview | 虚构示例协议 + 隔离的定义/请求 snapshot | 0，无生活 M | studio_runtime.go |
| npc_response | NPC 规则 + NPC 私有定义/world/机会/公开来意；动态评论分支为已接受遇见+公开 post | 0 | world_encounter_response.go；moment_npc_interactions.go |
| daily_review | 日复核规则 + Working Persona + D/M + local_date，不自动附加私聊 | 0 | model_tasks.go |
| diary_material | 选材规则 + 本地日期/时区 + source_window | 0；材料可含当天消息证据 | diary_runtime.go |
| diary_expression | 日记表达协议 + Working Persona + 日期/时区/as_of + 选定真实材料 | 0 | diary_runtime.go |
| persistent_switch | 切换判定规则 + Working Persona/切换规则 + D/M/S/H + user_text/candidate_reply/response_intent | 可带 H | model_tasks.go |
| reflection | 反思规则 + Working Persona + D/M + 明确范围的 evidence | 0，无 S | model_tasks.go |
| schedule_replan | 重排规则 + 冻结 intent + 当前日程/生活事实/已声明住所 + 共享世界规则/时区/地点候选 | 0 | model_tasks.go；schedule_capability.go |
| goal_planner | Planner 协议 + run/mode + goal-set snapshot；后续 query/commit 原生结果 | 0 | goal_planner.go |
| goal_evaluation | 评估协议 + Working Persona + stable 目标/criteria/actor refs + 专属事实/记忆/其他目标背景 + 当前目标状态/sources/reviews/accepted submissions；重复正文通过 evidence_payloads 复用 | 0；sources 包含消息证据 | goal_evaluation_task.go；goal_evaluation_wire.go |
| virtual_activity_result | 活动结果规则 + 请求/当前 appearance/life/近期结果/开始和最早结束时间 | 0 | model_tasks.go |

空白人格首版由 `working_persona_persistence.go` 本地合成。共享特质被合并到
profile 后，完全相同的输出行仅保留一次；同名但不同取值的事实均保留，原始
Core Persona 不改写。`r4_remaining_regression_test.go` 覆盖该去重边界。

初始化使用 `json_object` 输出格式；该请求不向模型发送完整 JSON Schema，因此
初始化协议中的规范 JSON 形状显式列出第三步的身份、共同性格、表达和生活资料
字段。外貌说明保留描述原文；共同性格与 0–1 表达参数使用数字，回应延迟为
秒，文本表达使用字符串。描述未支持的字段可省略或为 null，不能为了填满预览
编造事实。模型漏掉字段时，
Core 延续空值和空容器兜底，让用户在预览中补充。初始衣物的规范位置是
`core_persona.life_profile.appearance.wardrobe_items`；预览编辑此来源并保留
已有草稿的原始结构。模型将衣物数组放在 `life_profile.wardrobe_items` 时，
Core 仅在规范位置缺省或为 null 时移入该位置；两处均有值时保留规范值，并把
上一级的冲突值放在可审阅扩展中。`TestInitializationPreviewPhysicalRequestAndMissingFields`
核对实际 Provider HTTP 请求及缺省、零和 false 的保留，另有别名和冲突测试；
受控响应不等于真实模型的语义提取质量。

`takeover_judge`、`takeover_reply` 是已注册兼容入口，本轮没有发现非测试生产
调用方；保留注册和构造覆盖，不据此宣称正在生产运行，也不删除。

`conversation_reply` 另有 Moments 分支：Moment 协议 + Working Persona +
生活/关系/目标/能力/真实工具事实 + 检索记忆/S + 按既有会话范围取得的 H。
H 现在明确放在历史数据块。Moment 不是私聊续写，不能复制父 Agent 草稿或
把私聊全文作为公开动态。

私聊 Writer 输出通过 `parseReplyDraft` / `extractReplyBody` 提取正文，
`ToolPublicationService.PublishConversationReplyTx` 对原生 Tool 和准备草稿再次
使用同一解析器。中英文回复段、重复段、围栏和缺失段均不能把认知、事务记录、
时间或工具元数据写成用户可见消息。JSON 畸形、额外外壳字段或标题标记不完整时，
回退到清理后的可恢复正文并正常发布；不能直接保存原始外壳，也不能提取嵌套私有
字段。清理后确实没有正文才拒绝发布，媒体参数及原生工具授权仍按原规则校验。
原始候选只保留在
既有私有诊断路径。该防线不改变人格、输入协议或 token 设置，
`reply_body_integration_test.go` 覆盖真实 Provider HTTP 传输与 PostgreSQL 发布。

私聊表达默认协议为 `fact-only-reply.v6`：角色是具体的人及该角色本人，不得
自称 AI、助手、模型或人工智能；此身份约束明确覆盖 `blank_slate`（白板）
及人格资料为空的情况，未知经历仍保持未知。人格原文及当前输入不改写。
v4/v5 的原协议、hash 和冻结草稿恢复保持原样；v5/v6 继续使用相同的聊天
媒介、文体修复与只读人格详情工具规则，设置页保留旧版本选项。
聊天正文解析后的统一输出清洗 `stripReplyMarkdownHeadings` 去掉行首 1–6 个
`#` 加空白（或空标题）的 ATX 标记与合法尾标记，保留标题文字。普通正文、
话题标签、行内代码、缩进代码及反引号/波浪线围栏内的内容保持原样；围栏
按实际标记类型与长度闭合，不因代码内较短或异类围栏提前结束。动态场景
沿用其原有标题规则；聊天 Writer、原生 Tool、准备草稿及重复回复投影均清洗。
`reply_markdown_test.go` 覆盖清洗与共享解析路径；`reply_identity_test.go`
核对空白/已定义人格的 system 约束、实际 Provider HTTP 请求、无输出 token
上限及旧协议 hash。受控 Provider 验证传输和解码，不代表真实模型行为验收。

动态发布在 `publishInvocationMomentTx` 复用同一正文解析器，识别“发布动态”
协议标题并保留普通正文回退；没有实际附件时删除 `[自拍照：…]` 占位文本，
清理后没有正文则拒绝发布。用户建议携带的素材仍须通过原有权限和可读取校验，
已提交请求继续返回原正文。`moment_body_integration_test.go` 覆盖原生工具、
独立 Writer、用户建议、实际附件关联与重复请求。

R7 换装回归明确阶段职责：fact-only `conversation_cognition` 自主决定是否接受
本人生活动作；接受且可执行时先发现、查询和执行，再交 `conversation.reply`。
未知能力名先空参数查询 catalog，不能猜名字。目录加载和库存查询不算执行。
Writer 的当前事实视图增加 `action_execution_boundary`：当前为表达阶段、
其工具不是角色完整目录、文字聊天不禁用本人生活动作、叙述不能改写状态。
当前人格原文、旧 Writer protocol/hash 和原生工具授权仍使用原契约。
Owner 换装子任务即使目标看似已满足，也由真实 wear Tool 返回 no_change；
仅有 accepted 文本或目录回执不能结算为换装成功。
私聊 Writer 的 `tool_facts.current_action_result` 还直接读取本轮实际原生动作
回执，不依赖父模型选填 evidence_refs。按 owner/角色/会话/source 配对，
保留 handled 的 warning_code、disposition 与 effect_applied/mutation_committed=false。
普通查询、参数、父推理、原始错误 detail 不进入这条通道。本轮未结算的失败不能
指望历史 ActionOutcome 数据库读取自动补齐。
R8 增加专用 `moment.inspect`：只读本人已发布动态，真实结果以
`published_moments`（正文/发布时间/是否还有下一页）交给 Writer。实际完成的
`moment.publish` 回执补充 published_text；accepted/failed 草稿不冒充发布正文。
不由用户关键词推断发布/承诺，不把原聊天删除或改写成新事实。
本轮存在未生效的 wardrobe.wear 时，reply_action_check 使用已配置的认知模型
核验待发布正文；无冲突直接发布，有冲突只允许原 Writer 做一次无工具纠正，
再核验仍失败或核验服务出错则不发布。已有草稿失败栅栏阻止重做；父循环在
下一次物理模型调用前立即结束。成功、no_change、纯讨论不增加此检查。

## 共享 Agent 子任务与辅助调用

| 入口 | 数据范围 | 不可改变的边界 |
| --- | --- | --- |
| media_prompt 的主体解析/分支规范化/最终 prompt | 本阶段冻结意图、拍摄关系、可见主体及必要视觉字段；校验失败反馈 | 不加载无关 persona/history；真实图像继续用 image content part |
| conversation_summary 的累计/片段/每日模式 | 旧摘要与未覆盖消息 / 精确片段源 / 本地日与 segments | 不将通用聊天窗口额外叠加到待总结来源 |
| npc_response 的相遇/动态模式 | NPC 定义、世界规则、真实历史相遇、公开来意或动态正文 | 原始 target/CAS 在 Core，不冒充 Human 或 Fluctlight |
| Kev context.select/tools.select | 当前任务输入与本批可选上下文/工具候选 | 保留 `{state,questions}` 外部协议；state 为 JSON；context.select 异步执行，整阶段最多等待 250ms，超时保留原上下文 |
| Kev persona.switch | profile/rule 条件、提议、实际 Tool arguments、当前输入与语义状态 | event identity 在 Core；实际授权切换仍走工具 |
| Kev 自动流程/媒体 gate | 当前候选流程/意图/约束/拍摄方案 | 不增加无关私聊；不改远端 API 或回退语义 |
| native_cognition 的 NPC 相遇机会判断 | Working Persona + native 生活事实/记忆 + 公开角色/已认识状态/地点/真实机会与有效时间 | `world_opportunity_cognition.go`；仅安装 world.npc.encounter，保留 exact opportunity id/revision，其余来源身份由 Core 绑定 |
| Owner life request | 本次请求所需字段、原始意图、冻结目标与真实状态 | `_request_id`/CAS 留服务端；必要 item/intention/asset target 保留 |
| ADK/Writer/persona/media correction | 原任务上下文、必要真实结果和校验/质量反馈 | 保留工具隔离、原生 transcript、旧草稿 contract；不重做已提交效果 |
| vision probe | 诊断规则、测试文字和图片 | 非业务人格任务 |
| embedding | 文本向量输入 | 不使用 chat 运行协议 |

## 必要标识例外

“去掉数据库字段”不能破坏可执行协议；以下字段有实际消费者，明确保留：

| 字段类别 | 必要性 |
| --- | --- |
| 目标评估 `goal_ref/criterion_ref/object_ref/eN` | 选择冻结对象和证据，由 Core 恢复真实 ID/version |
| persona profile/rule selectors | 选择已声明且获授权的人格/规则 |
| Owner 请求的 item/intention/asset IDs | 对应工具必须精确匹配用户选择的目标，不能猜测或改对象 |
| 初始穿搭的 registered_items.id / item_refs / wear_item_refs | 方案和当前穿戴引用唯一衣物目录，选衣结果由 Core 校验并交给真实 wardrobe.wear；不发送方案自身 ID 或库存来源流水 |
| 日程 place_ref/place_id、新地点局部 key；事件 place_id/npc_id/target_actor_ref | 显式选择授权世界实体或冻结的实际在场角色；Core 校验/绑定后才登记、排程或执行，不能猜运行时 ID |
| Studio body/allowed_references 中的资源 ID/version | 创作定义的合法引用词表；job 自身 id/version 不发送 |
| Goal Planner run/goal/promise selectors、revision/facts_revision | 当前 query/commit 的实际 CAS 与目标协议；调度 lease/retry knobs 已去除 |
| NPC 相遇机会 id/revision | world.npc.encounter 的精确选择与 CAS；来源事件/presence/profile 身份不进入模型 |
| 视觉候选 asset 引用与原生 ToolCall ID | 绑定真实候选或匹配原生工具结果 |

新增例外必须指出消费者并补测试，不能把完整 DB 行作为“方便上下文”。

## 本次样本验证

用户附件经本地重建后走实际 assembler：消息 content 合计从 **57,020** 降到
**24,447** Unicode 字符，减少 **57.1%**；仍为 4 条初始消息、32 个独立证据
事件，8 份重复正文使用共享 payload。去重保留 source ref、主体、时间、有效性
与成功证明能力。主因是移除历史 catalog 的无关工具契约、选中目标重复和日程
派生表示，并复用完全相同的证据正文。

这是基于用户提供 prompt 的重建输入实验，未调用 Provider，未重放原始数据库
快照，也不证明模型效果、实际 tokenizer 数、缓存命中或延迟。私人原始数据
仅保存在附件和本次临时目录，不作为仓库 fixture。

## 验证入口

- `prompt_composition_contract_test.go`：29 Agent 协议边界、原文与图片保真、
  必要引用、字段白名单、源事件独立、schema/领域数据保护、日程去重。
- `working_memory_test.go` 与 `prompt_context_assembler_test.go`：同来源不同
  事实、完整对话组、摘要覆盖和大输入保留。
- ordinary structured/stream、Writer、native 第三次工具结果进入第四轮的
  实际 HTTP 请求测试：没有 max_tokens/max_completion_tokens，输入完整。
- `provider_prompt_cache_prefix_test.go`：稳定事实前缀与变化后的动态后缀。
- `r7_live_wear_test.go`：显式启用本地真实模型与一次性 PostgreSQL，从自然语言
  请求进入生产 handleTurn，模型自主发现和执行；核对实际穿着、其他衣物保留、
  原生回执和重载读回。含动态复合、不可用、只讨论、SSE 与 Owner no_change。
  不能用 scripted Provider 或回复包含衣物名称替代此验收；还需审阅真实正文语义。

任何提示词精简都不能通过 token 上限、按估算截断或预算拒绝实现。

## 图片禁字与可选视觉审核（2026-11-02）

`media_prompt` 的普通、质量重试和分支规范化指令明确要求最终图片无文字、
伪文字、字母、数字、标题、标签、字幕、签名、徽标和水印。Visual Identity
使用无文字角色卡模板，只投影外观与穿着，不额外拼接姓名、年龄元数据；
原始外观正文保持原样。模板和媒体补充指令不要求资料、简介、性格关键词或
签名分区，不预设领结、校徽。保留三视图、六种表情、服装/实际配饰、细节和
色卡。`visual_identity_vision`、`visual_identity_patch` 与生产 `visual_identity`
Agent 一致拒绝可见文字、伪文字、字母、数字、签名、徽标、水印，包括不可读
或零散字符；缺少文字和资料/签名分区不构成缺陷。视觉检查必须记录字符缺陷
并降低 identity_match，修正与生产审核即使人脸一致也必须选择 regenerate。
所有生产出图入口（聊天、角色视觉身份与 Studio）
共用 `submitComfyPrompt`：在提交时保留原正向描述并追加固定禁字协议；沿
ComfyUI `negative`/`negative_prompt` 声明边找到负向文本编码输入，保留原质量
排除项并追加禁字负向词。直接声明的 `negative_prompt` 同样处理，可选
`{{negative_prompt}}` 占位符解析为固定禁字负向词。无负向通道的模型保留其
原图结构，通过正向协议禁字；不创建虚构的负向节点。该固定输出政策不推断
主体、场景、构图，不改写用户输入、冻结意图、原生图片或数字渲染参数。
最终物理请求完整记录在原诊断 `request_payload`，缓存的模型 prompt 保持原样。

`vision.understanding` 继续默认关闭并独立绑定视觉模型。关闭时不发送图片
理解请求，候选图片保留为尚未检验、等待显式人工评审。返回摘要和时间线投影
说明设置入口及独立模型要求；开启设置不会自动重试此前跳过的审核。机器原因
`vision_disabled` 和既有权限保持原契约，无迁移或强制启用。

`TestMediaPromptPhysicalRequestsRequireTextFreeImages` 核对实际模型 HTTP JSON
的普通、重试与规范化协议及无输出 token 上限。
`TestImageSubmissionForbidsTextInPositiveAndNegativeProviderInputs` 通过真实
ComfyUI 提交函数检查三类入口的物理 HTTP 请求、SDXL 多文本负向编码、组合
条件、原始排除项、真实参考图片文件名和大整数 seed 保真；不等于已验证实图
质量或保证图像模型绝不产生字符。

`visual_identity_text_policy_test.go` 覆盖不同快照结构的姓名/年龄投影排除、
原始外观与源快照保真、无预设配饰和信息分区，以及 seed/character_sheet、
生产 Agent 多模态审核和两个 typed review task 的实际 Provider HTTP JSON。
typed task 使用一次性 PostgreSQL 数据库，其余测试无需数据库；受控响应只
验证协议传输和结果契约，不代表已完成真实模型识字或实图质量验收。

## 初始衣柜任务输入（2026-10-30）

`wardrobe_preparation` 的生成阶段只准备初始库存和 1–4 套方案，选衣阶段只
返回已登记衣物引用；每次只发送本阶段的规则。生成权限
`starter_items_allowed` 仅在生成阶段出现。既有四字段输出 schema 和独立
`wardrobe.wear` 执行、可用性/主体覆盖校验、版本栅栏、恢复语义保持原契约。
协议明确 `outer` 不能代替 `upper_body`；主体覆盖需要上衣与下装或既有
`full_body`。本地时钟从真实 instant/timezone 转换；月份、时区、职业及方案
名称不能证明天气、所在场景或当前活动。

输入按任务字段白名单构造。衣物详情只发送一次，方案与当前穿戴仅引用成员；
失效/缺失方案成员显式保留为不可用引用，目录外当前穿戴不能变成可选库存。
Core 保留库存/人格/Life 版本、来源流水和运行身份，不把完整数据库行发给模型。
身体事实取当前状态；共享和所选 profile 的通用 preferences、character_constraints
作为不透明业务资料保留，避免丢失存放其中的面料禁忌等约束。Foundation 只恢复
明确的 style_preferences 和 daily_outfit_preferences，不恢复旧伤情、旧库存或
旧穿着。旧卡的原始 system_prompt（无则 image_prompt）保留一份为旧风格参考
数据，不能成为运行协议；其余开场白、互动规则和媒体参数不再整包注入。

`wardrobe_preparation_prompt_test.go` 覆盖字段投影、未知/false/zero/大整数与
领域字段保真、失效成员、两阶段实际 Provider HTTP JSON 和无 token 上限。
`GO_CORE_WARDROBE_PROMPT_CAPTURE` 可显式指定本地私人 request/response 文本，
重建消息并比较字符数、核验原选择的主体覆盖；不入库、不调用模型，不能据此
宣称真实模型选择质量、token 数或延迟已验证。

## R9 传输与工具进度补充

`EnableThinking=false` 在两条 Eino 及 legacy 请求中均显式传 false；初始化、
初始日程、片段摘要不再依赖网关的默认推理开关。输入输出仍无应用 token 上限。
诊断保留完整原生调用序列与 tool_call_id/name，不再只展示前 64 条消息。

私聊重复目录加载先进入已选只读查询阶段，真实读取结果后才可表达；重复相同
事实或无效果回执收敛到真实 Writer。执行侧同样校验阶段，模型忽略工具列表
不能执行其他动作。已接受的真实动作和媒体续写继续保留。未知名字返回原生
handled 回执，不猜工具、不伪造成功。此处未改人格和 Writer 的表达提示词。

新增 `promise.propose` 能力使用实际当前 Owner 消息原文，父认知只能提交明确
时间窗口，不能改写提议或声称接受。Writer 事实视图读取真实提议状态；已接受
约定由原 Goal/Intention/日程流程关联。独立购物由真实到期结果完成入库，只有
开始时选择的购后穿戴才执行独立 wear。正式 Agent 仍为 29 个。

## R10 初始化的世界来源

前端初始化仍使用一次分析请求。选择复用已有生活世界时，人物描述只补充所选
世界的已保存配置（背景、规则等），不把原角色身份、人物、运行位置、事件或
生活历史加入初始化输入。激活后由持久命令复制世界和地点配置，再显式选用；
该过程不调用模型，也不复制运行事实。没有修改 Agent、Writer、Kev 或模型
修复提示词，没有恢复输入输出 Token 上限。本轮验证为 UI、命令恢复和独立
PostgreSQL 配置复制，没有运行模型效果回归。

## R11 媒体主体引用执行

媒体工具将 `visible_subjects[].actor_id` 中既有的 `actor_self` / `self`
绑定到当前调用的任务角色，与当前事实中的 Actor 语义一致。只处理该声明字段，
不改写用户正文、媒体意图或其他人物引用。确定的人物权限拒绝作为非重试工具
结果交回原生循环，依赖故障仍保持真实失败。本次修复未修改提示词、模型入口
或任何输入输出 Token 控制；静态上下文回归不代表实际媒体生成成功。

## 共享世界与事件（2026-10-30）

世界提议不加载任一角色的 Working Persona 或私聊。实际在场角色使用本次冻结的
aN 引用，Core 保留绑定；人物 ID、世界运行和调度元数据不进入提议提示词。
Kev 使用 world.event.admit 的独立 JSON state，只携带候选涉及的地点/NPC/在场
角色及必要世界规则、时间和近期事实。No/unclear/disabled/异常不等于允许入库。

NPC 独立回应仍保留该 NPC 的共享已发生互动，新增 participant_name 和
is_current_character，防止把与甲的经历当作与乙的关系。scene_event 的稳定地点
身份贯穿准备、执行和结果，实际 NPC 在场仍以确认事件或有效已开始活动为准。
输入输出继续不设应用 token 上限，诊断估算不参与截断或拒绝。

### D7–D10 execution boundaries (2026-11-02)
- Owner-request automatic attempts keep retry ordinal/timing/cause in Core, out
  of the real Provider messages. A stale wardrobe request gets a new assessment
  with the same original target and a freshly frozen execution CAS; original
  Owner text and payload remain unchanged. Physical controlled HTTP regression:
  `TestOwnerImageAssessmentTimeoutRetriesThenCreatesExactlyOneImageIntent`.
- World event input is unchanged. Incomplete worlds stop before model/Kev I/O;
  each world/hour owns one staged result and a 10-minute execution deadline.
  New-world baseline configuration is materialized transactionally, without an
  additional model entry or fabricated Life facts. No token limits were added.
