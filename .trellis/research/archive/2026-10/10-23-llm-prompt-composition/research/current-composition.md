# 当前 LLM 提示词构成与已验证问题

基线：2026-10-23，分支 `codex/fluctlight-functional-recovery`，起始 HEAD `c11c121b`。本文件是规划阶段静态代码盘点，尚未表示实现完成或真实 Provider 验收。

## 共同数据与历史定义

- P：固定运行协议及任务规则；需要扮演/判断人格的任务另有当前 Working Persona，分析任务只读其必要资料。
- D：按当前代码 surface 投影的当前主体、时间、身体/穿着、情绪/驱动力、生活场景/活动/地点、关系、目标、日程、意图和近期结果。不同 surface 字段不同，不能理解为每个任务都发送全部。
- M：活动记忆、常驻记忆、检索记忆。活动记忆查询最多 30 条；长期检索 Conversation 6 条、WakeUp/DailyReview 8 条、其他 12 条；常驻记忆读取已发布且有效的快照。是既有查询契约，不是 token 配额。
- S：0 或 1 条已持久化累计对话摘要，有源消息覆盖范围。摘要内容设置默认 2048 rune（可配置 512–4096），与模型 prompt/output token 限制不同，本任务不修改。
- H：有 conversation_id 时通常读取最近 200 条候选消息，必要时回补窗口前真实触发 user；排除当前输入/当前已提交回复，再按实际选入摘要或记忆的源覆盖去除整个已覆盖对话组。最终数量可变，绝非固定 10 句。Fact-only 标记最近两个完整交互组 required；这不是只保留两组。原始正文、顺序、发送者时间与时区保持。
- WakeUp 的 H 是标记为历史的动态数据，不作为当下 user/assistant 轮次。NativeCognition/Reflection 禁止通用 H/S；其任务证据可能包含真实历史消息，这是有范围的证据而非追加私聊。
- 私聊 Writer 与 cognition 的事实权限不同：Writer 不接受父 agent 草稿、执行策略或自由附加 facts，不承担目标提交/人格切换。

出处：`apps/core-go/internal/core/intelligence.go:274`、`provider_context.go:316`、`runtime_summary.go:241`、`fact_only_history.go:48`、`reply_context.go:79`、`reply_context.go:169`、`active_memory.go:17`、`memory_retrieval.go:114`、`resident_memory.go:28`。用户已确认 H/S 是此前优化成果，允许有理由调整；本任务选择保留该机制并修正错误去重。

## 28 个正式 Agent

所有下列代码路径均以 `apps/core-go/internal/core/` 为前缀。注册权威在 `formal_agents.go:74`；实际输入由 caller 构造，`RunFormalAgent` 不统一业务输入。

| Agent ID | 目前提示词组成 | 通用原始聊天 | 入口/构造位置 |
| --- | --- | --- | --- |
| conversation_cognition | P + Working Persona + 私聊 D/M/S + H + 当前输入；legacy 与 fact-only 投影不同 | H | cognition_agent.go:99；conversation_runtime.go:89 |
| conversation_reply | 私聊表达协议 + Working Persona + 独立事实视图/能力边界/实际 Tool facts + M/S + H + 当前输入 | H | reply_context.go:76；reply_provider.go:173 |
| wake_up | 专属完整唤醒协议 + Working Persona + D/M/S + 历史资料 H + 本次唤醒事件 | H，作为历史数据 | agent_result_adapter.go:908；capability_prompt_policy.go:41 |
| takeover_judge | Judge 协议 + 本轮 user/candidate/接管规则/控制与关系事实 + 可选 recent references | 无全量 H/M | turn_takeover.go:211、275；conversation_runtime.go:116 |
| takeover_reply | 接管任务规则 + caller 提供的选定人格、上下文及 messages + 授权 Tools | 取决于 caller | conversation_runtime.go:130 |
| wardrobe_preparation | 服装准备规则 + 人格/气候/当前情境/已有衣物/候选穿搭 | 0 | wardrobe_preparation.go:13、294 |
| initialization | 初始化解析协议 + Owner 描述 + 字段语义/输出结构提示 | 0 | app.go:487；model_tasks.go:56 |
| persona_compilation | 人格编译协议 + 选定 profile 和共享身份；必要时校验反馈 | 0 | persona_compilation.go:53、240 |
| media_prompt | 媒体任务规则 + 冻结意图/主体/拍摄方案/视觉事实；重试附此前 prompt 和质量反馈 | 0 | model_tasks.go:111；media_capture_plan.go:438；media_prompt_branches.go:205 |
| media_quality | 质量检查协议 + 冻结媒体概念/实际 renderer prompt/重试情况 + 图片 | 0 | media_quality.go:132；model_tasks.go:188 |
| visual_identity_vision | 视觉检查协议 + identity snapshot/视图要求/renderer 约束 + 图片 | 0 | model_tasks.go:219 |
| visual_identity_patch | 修正决策协议 + candidate/snapshot/constraints/vision findings | 0 | model_tasks.go:252 |
| visual_identity | 视觉身份流程规则 + 身份快照/任务状态/尝试历史/前次审查 + 必要图片 | 0；尝试历史是任务历史 | visual_identity_agent.go:92、225 |
| conversation_summary | 累计：旧摘要+Actor 更正+源消息；片段：时间范围+源消息；daily：日期/时区+segments | 指定源范围，非通用 H | conversation_summary.go:30；model_tasks.go:276、313、330 |
| schedule_generation | 排程规则 + 当前 identity/life/goals/intentions/outcomes/承诺 | 0 | model_tasks.go:417、432 |
| native_cognition | 认知协议 + Working Persona + D/M + 本次事件/事实 | 0，无 S | model_tasks.go:509 |
| studio_assistant | 定义编辑协议 + 隔离的资源 snapshot/Owner request/允许引用 | 0，无生活 M | studio_runtime.go:162、196 |
| studio_text_preview | 虚构示例协议 + 隔离的定义/请求 snapshot | 0，无生活 M | studio_runtime.go:228 |
| npc_response | NPC 规则 + NPC 私有定义/world/机会/公开来意；动态评论分支为已接受遇见+公开 post | 0 | world_encounter_response.go:73；moment_npc_interactions.go:122 |
| daily_review | 日复核规则 + Working Persona + D/M/S + local_date，若 projection 有会话则现策略允许 H | 可带 H | model_tasks.go:546 |
| diary_material | 选材规则 + 本地日期/时区 + source_window | 0；材料可含当天消息证据 | diary_runtime.go:202 |
| diary_expression | 日记表达协议 + Working Persona + 日期/时区/as_of + 选定真实材料 | 0 | diary_runtime.go:261 |
| persistent_switch | 切换判定规则 + Working Persona/切换规则 + D/M/S/H + user_text/candidate_reply/response_intent | 可带 H | model_tasks.go:575 |
| reflection | 反思规则 + Working Persona + D/M + 明确范围的 evidence | 0，无 S | model_tasks.go:593、625 |
| schedule_replan | 重排规则 + 冻结 intent + 当前日程/生活事实 | 0 | model_tasks.go:639；schedule_capability.go:116、193 |
| goal_planner | Planner 协议 + run/mode + goal-set snapshot；后续 query/commit 原生结果 | 0 | goal_planner.go:251、293、729 |
| goal_evaluation | 评估协议 + Working Persona + stable 目标/criteria/actor refs + Reflection D/M + 当前目标状态/sources/reviews/accepted submissions | 0；sources 包含消息证据 | goal_evaluation_task.go:226；goal_evaluation_wire.go:280、365 |
| virtual_activity_result | 活动结果规则 + 请求/当前 appearance/life/近期结果/开始和最早结束时间 | 0 | model_tasks.go:388、398 |

`takeover_judge`、`takeover_reply` 已注册且有实现，但本轮检索未找到非测试生产调用方；保留契约与测试覆盖，不宣称正在生产使用，也不删除。

`conversation_reply` 还承担 Moments 分支：Moment 协议 + Working Persona + 生活/关系/目标/能力/Tool facts + retrieved memory/S + 若 projection 有则提供 H。现状仍把这些 H 作为 role messages；新设计要明确历史资料语义和任务选择，避免把发动态变成继续私聊。

## 非独立注册 Agent 的路径

| 路径 | 目前输入 | 组成治理边界 |
| --- | --- | --- |
| Kev context.select/tools.select | 当前输入 + 本批可选上下文或工具候选 + 问题 | `kev_decisions.go:285`、`kev_tools.go:313`；只发本批语义材料 |
| Kev persona.switch | active_profile/conditions/proposal/actions/event_id/current_input/current_state | `kev_persona.go:69`；当前状态整包需语义白名单 |
| Kev 自动流程 gate | 有资格执行的候选流程 state + should-run question | `kev_decisions.go:215`；保留退避/恢复语义 |
| Kev 媒体分支/拍摄决策 | 意图/显式约束/已接受方案/拍摄声明/指代 | `media_capture_plan.go:252、332`；question_version 等内部字段需审查 |
| Owner life request | projection + `{event_type,kind,request}` + 真实工具 | `owner_life_requests.go:362`；内部 request identity 交 Core |
| ADK final repair | 初始上下文 + 实际原生工具 transcript + 校验反馈 | `eino_model_runtime.go:1104`；只修 DTO、不得重新执行业务工具 |
| publication correction | 原上下文 + 要求真实 conversation.reply 的纠正信息 | `eino_model_runtime.go:1739`；与 tool-free repair 的授权语义不同 |
| Writer/persona/media 局部 correction | 原任务材料 + 校验/质量反馈 | 保留现有次数/冻结模型/隔离语义，统一反馈数据表达 |
| vision probe | 诊断协议 + 文字 + 图片 | `vision_probe.go:33`；不是业务 persona 任务 |
| embedding | 文本向量输入 | `model_tasks.go:648`；无 chat prompt，不纳入运行协议组装 |

Kev 的外部协议为 `{state:string,questions:...}`，见 `internal/ai/decision/decision.go:100、387`，不是 chat completions。统一它的语义投影与 JSON 编码，不修改远端 API envelope。

## 用户样本的定量证据

样本仅位于用户附件；不复制完整个人数据到仓库。四条消息 content 合计 **57,020 Unicode 字符**，system 3,352，stable 864，runtime 10,075，current 42,729。`sources` 41,987 字符，32 条 source 中有 29 条 outcome、3 条 message。

- 5 条 `capability.catalog` sources 共 **28,570** 字符，占全部 content 约 **50.1%**；其中包含重复工具 purpose 和完整 parameters schema。
- runtime 的 schedule 占 6,607 字符；current_item 与 items 重复，同一时间有多个显示派生值。
- stable goal 定义与 runtime goals 又重复；runtime goals 本段占 973 字符。
- 15 条 source 的 `data` 与其他条目完全相同，重复 payload 合计 10,183 字符。事件 ref、时间、validity 仍不同；不能合并整个事件。
- 上述重复量互有重叠，不能直接相加，也不是优化后的节省承诺。

已验证根因：

1. `goal_evaluation_task.go:250` 把专用 Goal 输入放入通用 Reflection projection，`provider_context.go:662` 再注入 runtime goals。
2. `goal_evaluation_source_input.go:29` 对 catalog 无专用结果投影，整套工具定义进入历史 evidence。
3. `goal_evaluation_wire.go:432` 递归按 `_id/_ref/_ids/_refs` 改写，误入 `parameters.properties`。样本已有 `evidence_refs: []` 代替 schema、`required:item_ids` 而 properties 无 item_ids 的损坏。
4. `provider_prompt_composer.go:194` system 使用自然语言+YAML；`prompt_context_assembler.go:457` stable 是 JSON，`:548` runtime 是 YAML/TOON；current 按原文透传。`internal/ai/prompt/format.go:291` 根据列表形状自动切 TOON。
5. `working_memory.go:237` 根据任一 source ref 相交丢整条 fragment，可能丢同来源不同陈述；fact-only 专有去重比通用路径更谨慎。
6. Generic task JSON 没有 context/context_projection 时，`provider_prompt_composer.go:630` 直接渲染整包。Studio、NPC、Planner 等仍有数据库元数据入口，需逐任务审查而不是全局删键。

Planner 特别边界：`goal_planner.go:729` 初始摘要每 status 最多 5 个目标，desired_outcome/motivation 各到 300 rune，完整信息通过 query 获取。revision/facts_revision/run identity 参与 commit/CAS，不能直接删而不迁移服务端绑定。任务计数/分页不是 token 限制；新投影不能引入更多有损裁剪。

## 回归与无 token 上限基线

权威行为在 `.trellis/spec/backend/fluctlight-provider-contract.md:3` 的 2026-10-23 amendment；移除提交为 `bea62002d1257b1038cba1c3ac35903d9a6d4cdb`。

- `prompt_context_assembler_test.go:465`：ordinary structured + stream 实际 HTTP，完整大输入，max_tokens/max_completion_tokens 均不存在。
- `eino_adk_runtime_test.go:908`：完整第三次 Tool result 到达第四轮，无 cap；`:174` direct Eino 无 cap。
- `provider_budget_automatic_test.go:5`：legacy token settings 不改变配置有效性；`reply_provider_test.go:41` Writer 实际请求无 cap。
- `goal_evaluation_source_input_test.go:50`：大型真实 domain evidence 全文和推荐保留。
- `provider_context_test.go:764`：64 条历史候选，摘要覆盖 40 条后留下未覆盖 24 条；token 字段不剪裁。
- `prompt_context_assembler_test.go:295`：摘要只覆盖部分对话时保留整个 turn。
- `provider_prompt_cache_prefix_test.go:9`：稳定前缀独立于时钟、状态和证据变化。
- `provider_prompt_composer_test.go:382`：普通正文中类似 ID 的文字保持原文。
- `goal_evaluation_wire_test.go:149、249、273`：冻结引用恢复、未知/错误/重复 ref 拒绝、跨 profile 权限。

需改表示断言的旧测试：`internal/ai/prompt/format_test.go`、`core/provider_prompt_format_test.go` 的 YAML/TOON 选择，`media_context_test.go:205`、`provider_prompt_cache_prefix_test.go` 的特定 block 数量。保留其语义与稳定性断言。

旧预算条款仍在 provider 81/179/617/659/708 行、fact-only reply 56 行、memory 318/354/388/392 行、diagnostics 240 行、structured-turn 445 行等；实现阶段清理冲突的现行陈述，持久化兼容列单独说明，不改其存储。
