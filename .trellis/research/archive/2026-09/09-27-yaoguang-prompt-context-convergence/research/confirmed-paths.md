# 已确认路径（2026-09-27）

本次只读审计基于当前 `master`，不把用户今天已提交的总结连续性修复当成待重新实现的缺陷。以下行号是规划锚点；代码变更前仍需读取将修改的完整函数和相关规格。

## 地点与场景

- `ContextProjection` 只有摇光所属的一份 `LifeContext`，并嵌入 `CurrentState`；`current_speaker` 只有身份，没有用户地点：`apps/core-go/internal/core/intelligence.go:32-84,178-195,335-355,478-535`。
- Event 的 scene/activity/location 按 `fluctlight_id` 写入，优先于 Schedule；`scene_event` 没有目标 Actor 输入：`apps/core-go/internal/core/native_capabilities.go:12-32,103-116,163-174`、`apps/core-go/internal/core/life_context.go:168-192`。
- Runtime 协议说“除 actor_user 明确要求外禁止变更场景”，又要求 `context_override.explicit=true`，但 `scene_event` schema 不接受该字段；它也没有说明用户自述地点与摇光地点的主语区别：`apps/core-go/internal/core/provider_prompt_composer.go:9-29`、`apps/core-go/internal/core/native_capabilities.go:20-32`。
- 可疑但未由现场记录证明：若漂移时有 `scene_inferred`，属于错误 Tool 写入权威状态；否则更可能是模型对未标主语的地点和历史事实产生指代漂移。实施前应按 correlation 检查首次漂移 turn 的 `scene_event` 与 `life_events`。

## 对话总结和历史

- assistant 消息提交后尝试 enqueue；保留最近 24 条、单块最多 40 条，达到 20 个 assistant 回合、约 6000 tokens 或 40 条才生成，块结束于 assistant：`apps/core-go/internal/core/tool_publication.go:144-147`、`apps/core-go/internal/core/conversation_summary.go:17-28,71-95,146-203`。
- 70 个正常 user/assistant 回合约为 140 条消息；短消息下两条 40-message summary 覆盖 1–80，81–116 未达到下一块阈值，117–140 是最新 24 条。因此只有两条不自动表示 worker 失败。若 raw budget 容不下 81–116，仍会出现预算型历史缺口。
- 今天的连续性修复已把 raw candidate 增至最多 200，提供 summary 落选时的 raw fallback，并在最终 wire 只按已入选 summary 对完整 raw turn 去重：`apps/core-go/internal/core/intelligence.go:258-270`、`apps/core-go/internal/core/working_memory.go:65-106`、`apps/core-go/internal/core/prompt_context_assembler.go:307-376`。不得重复改旧的固定 8-message 窗口。
- `historical_conversation` 是 `conversation_summaries[]` 单项的 `time_semantics` 值，不是另一组独立摘要：`apps/core-go/internal/core/provider_context.go:974-987`。
- 当前摘要 prompt 只自由文本要求保留时间；输入是 UTC `occurred_at`，输出 schema 只有自由文本 `summary`，无对话时间段、阶段结束和未完线索契约：`apps/core-go/internal/core/conversation_summary.go:30,332-340`、`apps/core-go/internal/core/provider_schemas.go:8-12`。
- summary settle 后只发布 ready 事件，没有自动 enqueue 后续已达阈值的块；需验证 backlog/catch-up：`apps/core-go/internal/core/conversation_summary.go:406-484`。

## Runtime Context 与重复结果

- conversation 和 wake-up 共享 `compactCognitionContextForSurface`；当前状态包含 inner_state、life_context、appearance 和 active_activities：`apps/core-go/internal/core/provider_context.go:481-550,595-615`。
- 最近 12 条 `cognition_action_outcomes` 未按 capability/status 去重，每次 action 还可能有 aggregate 与每个 Tool 的独立 outcome；provider compactor保留 capability/status/error/observed：`apps/core-go/internal/core/intelligence.go:250-253`、`apps/core-go/internal/core/action_outcome.go:83-212,593-626`、`apps/core-go/internal/core/provider_context.go:882-907`。这直接解释重复的 `affect_event`、`conversation.reply` 和失败状态。
- 同轮 Tool result、刷新后的 current_state、已发布的 assistant recent history可能再表达同一事实；当前 model-facing Tool result保留完整 output：`apps/core-go/internal/core/adk_conversation_runtime.go:206-245`、`apps/core-go/internal/core/provider_context.go:276-308`。
- Runtime Context 使用 YAML 主体，并仅将同构 scalar 对象数组渲染为 TOON 表；换 serializer 不能消除重复来源：`apps/core-go/internal/core/prompt_context_assembler.go:453-485`、`apps/core-go/internal/ai/prompt/format.go:95-140,247-267`。
- runtime facts 第一层 section cap 为 6144 tokens，顶层 fact 原子入选且不标 Required；最终 wire 把已入选 facts 当 required。Tool continuation 保留全部 Tool transcript，超预算会失败：`apps/core-go/internal/core/working_memory.go:65-67,132-166`、`apps/core-go/internal/core/prompt_context_assembler.go:277-292`、`apps/core-go/internal/core/eino_model_runtime.go:167-218`。

## 媒体提示词

- `media.image.generate` 只接受自由文本 `intent`，Prepare 冻结 appearance/current_state 等 context_binding：`apps/core-go/internal/core/tool_contract.go:248-260`、`apps/core-go/internal/core/builtin_capabilities.go:288-345`。
- 当前 `MediaPromptInstruction` 固定“写真”和“画面不是普通自拍”，缺少第一人称/手持/镜前/第三人称的拍摄物理规则；camera/capture 等虽在 concept allowlist 中，上游没有结构化写入：`apps/core-go/internal/ai/prompt/prompts.go:9-44`、`apps/core-go/internal/core/provider_context.go:1986-2000`。
- media compactor 原样透传 appearance；effective-life 会补未知 body_fields，metadata stripping 后可留下空对象。current_state 的 drive compactor还保留零压力 drive 的完整 description/direction/key/label：`apps/core-go/internal/core/effective_life.go:160-190`、`apps/core-go/internal/core/provider_context.go:1468-1541,1931-1983`。
- 初次 media prompt 出站已被格式化为 YAML；`media_quality_acceptance` 的多模态 text part仍是外层 JSON 包内层 JSON 字符串，retry也有 JSON-string-in-YAML。相同 raw JSON 多模态入口还有 Visual Identity vision/formal agent：`apps/core-go/internal/core/media_quality.go:137-149,247-255`、`apps/core-go/internal/ai/prompt/format.go:20-31`、`apps/core-go/internal/core/model_tasks.go:146-157`、`apps/core-go/internal/core/visual_identity_agent.go:152-175`。

## agent_run_failed 与诊断

- `agent_run_failed` 可由已失败 durable run 的重放守卫产生；原始底层错误存在 `agent_runs.error_detail`，而后续重试可不再产生物理模型调用：`apps/core-go/internal/core/agent_run_record.go:82-114`。
- 物理 model run可成功返回 ToolCall（status completed），随后 Tool/ADK 失败；诊断中心的 Model Runs 查询 `diagnostic_model_runs`，不投影逻辑 `agent_runs`。所以“模型已完成”与“Agent 失败”在现有页面可同时成立：`apps/core-go/internal/core/eino_model_runtime.go:81-103`、`apps/core-go/internal/core/adk_conversation_runtime.go:178-212`、`apps/core-go/internal/core/diagnostic_model_runs_filter.go:24-127`、`apps/web/src/views/DiagnosticsView.vue:178-189`。
- 系统事件默认只取全局最近 20 条，生命周期分区仅查 `lifecycle.%`；ADK/Agent termination不在其中：`apps/web/src/stores/control-center.ts:214`、`apps/core-go/internal/core/operations.go:1519-1533,1621-1622`。
- 已有的 Visual Identity `observations` 参数形态错误当前代码已改为可纠正、非 retryable Tool 结果，不应重复修复：`apps/core-go/internal/core/tool_execution.go:281`、`apps/core-go/internal/core/adk_conversation_runtime.go:212`。
- 待端到端证实：真实 ADK Tool callback 的 `model_call_id` 是否与物理 model run相同；取消/超时时 lifecycle failure是否因沿用已取消 ctx 丢失：`apps/core-go/internal/core/eino_model_runtime.go:85,253`、`apps/core-go/internal/core/adk_conversation_runtime.go:248`、`apps/core-go/internal/core/lifecycle_diagnostics.go:316`。

## 追加：静默摘要、日记忆与 Wake-up（用户 2026-09-27 补充）

- 当前摘要只在 assistant 消息提交后 enqueue，不含静默 timer 或当地日界触发：`apps/core-go/internal/core/tool_publication.go:120-149`、`apps/core-go/internal/core/conversation_summary.go:17-28,146-203`。日归并需新增可恢复的 durable intent，而非直接依赖下一条聊天消息。
- Summary row 保存精确 message refs/digest 与不可变 revision；Provider 输出仅自由文本。它是历史投影，不等同长期事实：`apps/core-go/internal/migrations/runner.go:2209-2240`、`apps/core-go/internal/core/provider_schemas.go:8-12`。
- 当前 typed Memory provenance 未支持 `conversation_summary` frozen source；只支持 `memory` 和 `outcome`，且来源存在不证明摘要语义正确：`apps/core-go/internal/core/memory_provenance.go:38-40,114-172`。日归并默认应是带来源的 `episodic` memory，不自动升级为 semantic/relationship fact；summary source revision/fingerprint与 raw 失效要向日记忆传播。
- `conversation_summaries` 检索只选 `status='active'`；“清空”应是日记忆成功提交后让已覆盖的阶段摘要退出活动检索，同时保留可审计 row、source refs/digest，防止先清后写造成历史空洞：`apps/core-go/internal/core/conversation_summary.go:550-607`。需要显式 consolidated/retired 状态及目标 Memory ID，不能物理删除。
- 日界应按冻结的摇光有效 IANA 时区计算半开 UTC 范围，不使用服务器/执行时日期，也不能假设一天恒为 24 小时。跨 conversation 归并需独立授权；默认每 conversation、每 local day 一条日记忆，以免权限范围扩散：`apps/core-go/internal/core/memory_lifecycle.go:327-359`。
- 当前 Wake-up 默认 interval=1800s；聊天认知结算后调用 `wakeUpAfterCognitionDelay(interval)`，再加 10 分钟，故首次默认约 40 分钟。每次 Wake-up 完成后再按 interval 排下一次：`apps/core-go/internal/core/wakeup.go:17-35,456-480`、`apps/core-go/internal/core/redis_triggers.go:17-18,67-114`。与用户要的 t+10、超过 t+30 静默不符。
- 现有单一 Redis key + PostgreSQL due CAS + cycle/唯一约束已能收敛大部分自动重复触发：`apps/core-go/internal/core/redis_triggers.go:50-65,175-195,241-315`、`apps/core-go/internal/migrations/runner.go:266,313`。用户消息会 preempt pending/running；但 `ProcessWakeUp` 最后一次取消检查在取得 lifecycle advisory lock 前，锁后未重查，存在新聊天刚好介入时仍提交 wake fact/下周期的竞态：`apps/core-go/internal/core/agent_result_adapter.go:792-795,834-872`。可参考 Reflection 的锁后重查：`apps/core-go/internal/core/reflection_runtime_v2.go:356-367`。
- 现有设置只有 `{enabled, interval_seconds}`，旧 contract 与测试明确写“chat 后 interval + 10m”；实施新时序需同步配置、后端/前端 fallback、测试与规格：`apps/core-go/internal/core/wakeup.go:23-35`、`.trellis/spec/backend/fluctlight-workflow-contract.md:462-480`、`apps/core-go/internal/core/redis_triggers_test.go:56-84`。
