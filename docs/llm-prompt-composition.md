# LLM 提示词组成清单

更新：2026-10-23。代码权威是 `apps/core-go/internal/core/formal_agents.go`
与各 typed task，规范在 [Prompt Composition Contract](../.trellis/spec/backend/prompt-composition-contract.md)。

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

## 28 个正式 Agent

表内文件均在 `apps/core-go/internal/core/`；这些文件与函数可用于定位具体入口。

| Agent ID | 当前提示词组成 | 通用原始聊天 | 入口/构造位置 |
| --- | --- | --- | --- |
| conversation_cognition | 私聊认知协议 + Working Persona + 私聊 D/M/S + H + 当前输入；legacy 与 fact-only 投影不同 | H | cognition_agent.go；conversation_runtime.go |
| conversation_reply | 私聊表达协议 + Working Persona + 独立事实视图/能力边界/实际 Tool facts + M/S + H + 当前输入 | H | reply_context.go；reply_provider.go |
| wake_up | 专属完整唤醒协议 + Working Persona + D/M/S + 历史资料 H + 本次唤醒事件 | H，作为历史数据 | agent_result_adapter.go；capability_prompt_policy.go |
| takeover_judge | Judge 协议 + 本轮 user/candidate/接管规则/控制与关系事实 + 可选 recent references | 无全量 H/M | turn_takeover.go；conversation_runtime.go |
| takeover_reply | 接管任务规则 + caller 提供的选定人格、上下文及 messages + 授权 Tools | 取决于 caller | conversation_runtime.go |
| wardrobe_preparation | 服装准备规则 + 人格/气候/当前情境/已有衣物/候选穿搭 | 0 | wardrobe_preparation.go |
| initialization | 初始化解析协议 + Owner 描述 + 字段语义/输出结构提示 | 0 | app.go；model_tasks.go |
| persona_compilation | 人格编译协议 + 选定 profile 和共享身份；必要时校验反馈 | 0 | persona_compilation.go |
| media_prompt | 媒体任务规则 + 冻结意图/主体/拍摄方案/视觉事实；重试附此前 prompt 和质量反馈 | 0 | model_tasks.go；media_capture_plan.go；media_prompt_branches.go |
| media_quality | 质量检查协议 + 冻结媒体概念/实际 renderer prompt/重试情况 + 图片 | 0 | media_quality.go；model_tasks.go |
| visual_identity_vision | 视觉检查协议 + identity snapshot/视图要求/renderer 约束 + 图片 | 0 | model_tasks.go |
| visual_identity_patch | 修正决策协议 + candidate/snapshot/constraints/vision findings | 0 | model_tasks.go |
| visual_identity | 视觉身份流程规则 + 身份快照/任务状态/尝试历史/前次审查 + 必要图片 | 0；尝试历史是任务历史 | visual_identity_agent.go |
| conversation_summary | 累计：旧摘要+Actor 更正+源消息；片段：时间范围+源消息；daily：日期/时区+segments | 指定源范围，非通用 H | conversation_summary.go；model_tasks.go |
| schedule_generation | 排程规则 + 当前 identity/life/goals/intentions/outcomes/承诺 | 0 | model_tasks.go |
| native_cognition | 认知协议 + Working Persona + D/M + 本次事件/事实 | 0，无 S | model_tasks.go |
| studio_assistant | 定义编辑协议 + 隔离的定义 body/创作要求/允许引用（移除 job 身份） | 0，无生活 M | studio_runtime.go |
| studio_text_preview | 虚构示例协议 + 隔离的定义/请求 snapshot | 0，无生活 M | studio_runtime.go |
| npc_response | NPC 规则 + NPC 私有定义/world/机会/公开来意；动态评论分支为已接受遇见+公开 post | 0 | world_encounter_response.go；moment_npc_interactions.go |
| daily_review | 日复核规则 + Working Persona + D/M + local_date，不自动附加私聊 | 0 | model_tasks.go |
| diary_material | 选材规则 + 本地日期/时区 + source_window | 0；材料可含当天消息证据 | diary_runtime.go |
| diary_expression | 日记表达协议 + Working Persona + 日期/时区/as_of + 选定真实材料 | 0 | diary_runtime.go |
| persistent_switch | 切换判定规则 + Working Persona/切换规则 + D/M/S/H + user_text/candidate_reply/response_intent | 可带 H | model_tasks.go |
| reflection | 反思规则 + Working Persona + D/M + 明确范围的 evidence | 0，无 S | model_tasks.go |
| schedule_replan | 重排规则 + 冻结 intent + 当前日程/生活事实 | 0 | model_tasks.go；schedule_capability.go |
| goal_planner | Planner 协议 + run/mode + goal-set snapshot；后续 query/commit 原生结果 | 0 | goal_planner.go |
| goal_evaluation | 评估协议 + Working Persona + stable 目标/criteria/actor refs + 专属事实/记忆/其他目标背景 + 当前目标状态/sources/reviews/accepted submissions；重复正文通过 evidence_payloads 复用 | 0；sources 包含消息证据 | goal_evaluation_task.go；goal_evaluation_wire.go |
| virtual_activity_result | 活动结果规则 + 请求/当前 appearance/life/近期结果/开始和最早结束时间 | 0 | model_tasks.go |

`takeover_judge`、`takeover_reply` 是已注册兼容入口，本轮没有发现非测试生产
调用方；保留注册和构造覆盖，不据此宣称正在生产运行，也不删除。

`conversation_reply` 另有 Moments 分支：Moment 协议 + Working Persona +
生活/关系/目标/能力/真实工具事实 + 检索记忆/S + 按既有会话范围取得的 H。
H 现在明确放在历史数据块。Moment 不是私聊续写，不能复制父 Agent 草稿或
把私聊全文作为公开动态。

## 共享 Agent 子任务与辅助调用

| 入口 | 数据范围 | 不可改变的边界 |
| --- | --- | --- |
| media_prompt 的主体解析/分支规范化/最终 prompt | 本阶段冻结意图、拍摄关系、可见主体及必要视觉字段；校验失败反馈 | 不加载无关 persona/history；真实图像继续用 image content part |
| conversation_summary 的累计/片段/每日模式 | 旧摘要与未覆盖消息 / 精确片段源 / 本地日与 segments | 不将通用聊天窗口额外叠加到待总结来源 |
| npc_response 的相遇/动态模式 | NPC 定义、世界规则、真实历史相遇、公开来意或动态正文 | 原始 target/CAS 在 Core，不冒充 Human 或 Fluctlight |
| Kev context.select/tools.select | 当前任务输入与本批可选上下文/工具候选 | 保留 `{state,questions}` 外部协议；state 为 JSON |
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

- `prompt_composition_contract_test.go`：28 Agent 协议边界、原文与图片保真、
  必要引用、字段白名单、源事件独立、schema/领域数据保护、日程去重。
- `working_memory_test.go` 与 `prompt_context_assembler_test.go`：同来源不同
  事实、完整对话组、摘要覆盖和大输入保留。
- ordinary structured/stream、Writer、native 第三次工具结果进入第四轮的
  实际 HTTP 请求测试：没有 max_tokens/max_completion_tokens，输入完整。
- `provider_prompt_cache_prefix_test.go`：稳定事实前缀与变化后的动态后缀。

任何提示词精简都不能通过 token 上限、按估算截断或预算拒绝实现。
