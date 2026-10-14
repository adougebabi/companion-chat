# 两处调用链、修改落点和证据可用性

基线 d696c44；2026-10-14 只读源码研究。以下路径相对 `apps/core-go/internal/core/`。主线程已抽查 Goal 过滤、参数发布、provenance 定义和 final-repair 关键片段。未测试、访问用户数据库或远端服务。

## 实际调用链

| 阶段 | 文件:行 / 符号 | 来源与当前行为 |
| --- | --- | --- |
| 私聊 run/schema | private_reply_run.go:43 / withPrivateReplyRun；:102 privateReplyRegistry | direct ownership、配置和 Writer binding 固定；legacy 原 registry，Fact-only run-local 替换 |
| 认知入口 | cognition_agent.go:45 / RunConversationCognitionAgent | BuildContextProjectionFor → cognitive_assessment → Composer → formal cognition |
| 认知 WorkingMemory | provider_context.go:214 / workingMemoryInputFromProjectionForSurface | FactOnlyCognition 重映射 ConversationMain，仍用通用 runtime/记忆/历史 |
| 真实历史/身份 | intelligence.go:275、:347、:529 | 最多 200 候选，真实 author/time snapshot；自 Actor 名与 profile 分开 |
| 认知历史渲染 | provider_context.go:334 / recentPromptFragmentsForSurface；:388 currentInputRecentMessageIndex | 真实角色，生成 sender/time 前缀；严格当前输入去重；缺 turn_id 时 sequence-pair 回退 |
| 原生工具 | adk_conversation_runtime.go:130；native_tool_contract.go:26 | 实际 native/Provider 身份、当前 registry/refs → ExecuteTool → invocation/result trace |
| Fact-only 参数 | fact_only_reply_capability.go:19 / Definition | optional evidence_refs、draft_ref、action_result_refs、topic_key、purpose；closed schema，无 draft_id |
| evidence/草稿 | fact_only_reply_capability.go:167、:264、:332 | 缓存/已发布优先；offered+live scope/entity/revision/snapshot 验证；无效 refs 在 lease 前拒绝 |
| Writer 数据输入 | fact_only_reply_capability.go:348 / writerTaskInput | 当前编译人格、刷新投影、真实输入/结果/摘要；未把 purpose/topic_key 传 Writer |
| Writer Composer | reply_context.go:70 / buildFactOnlyReplyContext | 通用 compact 后覆盖 Goals；history 只有 role/content；Memory 丢部分来源语义 |
| Writer 调用 | reply_provider.go:125 / RunConversationReplyTask | 独立 assignment/队列、formal Writer、实际 reply.image.request，返回 draft |
| 发布/去重 | fact_only_reply_capability.go:60 / ExecuteDirectTx；tool_publication.go:155 | purpose/topic_key 用作 publication 元数据；主动抑制要求两者非空，并检查 inbound sequence/window |
| 媒体恢复 | reply_media.go:145 / replyExecutionScope | 保存 source/scope/当前输入/purpose/topic_key；同 draft continuation，无新用户轮次 |
| 工具/格式分离 | eino_model_request_policy.go:68 / physicalRequestOptions | effective tools 非空即不发 response_format；对应实际预算 |
| 合同/修复 | eino_model_runtime.go:965 | 完整 ResponseSchema/refs 校验；至多一次无工具 repair；实际配对保留，失败不撤销 Tool 提交 |
| 修复隔离 | eino_model_request_policy.go:158 / finalRepairContext；:169 finalRepairModelOptions | mask Kev/业务 policy/refresh；删除 tools/choice/parallel，thinking=false，原 schema 保留 |
| 实际诊断 | eino_model_runtime.go:109、:208、:342、:400、:437；diagnostics.go:525 | 独立物理 ID、SDK messages/response 表示、估算、实际 usage/latency，非完整 wire bytes |
| 共享 Moments | fact_only_moment_capability.go:132 | 同 writerTaskInput/buildFactOnlyReplyContext，但 feed scene/无私聊图片工具；需按 Scene/Surface 隔离 |

## 已证实缺口与已有保护

- replySemanticValue (`reply_context.go:28`) 删 last_evaluation/ref 元数据，但保留 next_step/current_stage.strategy；`:83` 重投影 projection.Goals，绕开通用 profile filter。
- Writer `:123` 每条缺 turn_id 消息为一组，保护最后两组不保证触发 user；未携带 sender/time。
- Writer `:155` 只给 content/confidence/通用 uncertainty，按内容 digest 去重；跨事件同文和来源语义需要更精确边界。
- `reply_context_test.go` 已有 `TestFactOnlyReplyHistoryIsBoundedAndUsesExistingSummary`、`TestFactOnlyReplySummaryReplacesCoveredWholeTurn`、`TestFactOnlyReplyContextUsesAuthorityAndOriginalHistory`、`TestFactOnlyRequiredGoalOverflowIsExplicit`，未覆盖完整策略/profile/来源/时间矩阵。
- `tool_final_schema_separation_test.go:19` 的 `TestToolFinalSchemaSeparationPhysicalRequests` 覆盖 Generate/Stream、valid/repair/invalid_repair、一次 reply、后续 optional read。坏 final 是合法 JSON 的字段类型错误，publication 为模拟 invoker。
- `conversation_reply_turn_test.go:9` 验证持久化一次发布/replay，但用 legacy text；`reply_evidence_recovery_test.go:15` 验证无效 ref 不占 lease 和实际失败续接。需要组合 Fact-only 真发布与语法坏 JSON。

## 事故证据可用性

| 数据 | 可用性 | 约束 |
| --- | --- | --- |
| as_of、两条 sender/time、夜间标题、purpose、部分 Goal/Memory/关系观察 | 附件转述/摘录 | 已报告材料，不扩写成完整事故消息 |
| 附件提到的两份粘贴文本 | 未提供/未读取 | 不猜其余字段 |
| 三次完整认知请求/响应、tools/choice/format/sampling | 仓库未找到匹配原件 | unavailable，不能认定故障具体根因 |
| 完整 Cydonia 原事故请求/响应 | 仓库未找到匹配原件 | unavailable，不能宣称原始 A/B 已执行 |
| SDK messages/工具/usage 诊断 | 代码有记录能力，未读取真实记录 | reconstructed SDK 不等于原 HTTP bytes |
| Markdown 时间标题生成源 | 未证实 | 不删除正文；sender/time renderer 另有已证实来源 |
| 新 fixture/旧新 wire 对照 | 待实施 | document_excerpt/synthetic/missing 明确标识，mock publication 不算真实发布 |

## 原数据限制

关系是否陈旧、承诺日期/文本是否误记、关系标签是否被原话实际支持、历史标题来源，以及上游 completed Goal 配额和 dependency read-model/admission 差异，只记录来源/限制，不自动修写，不绕过最终 admission。
