# 已核实的运行时路径（2026-09-27）

本文件只记录源码证据；用户原始现场日志不可得。已有未提交改动属于进入本任务前的工作区状态。

## 视觉身份：随机种子已替换，本次故障是审查参数

- `apps/core-go/internal/core/media.go:106-141`：媒体 worker 在 POST `/prompt` 前选择 workflow、替换占位符并记录提交诊断。
- `apps/core-go/internal/core/media.go:499-607`：一次调用共享同一个 `crypto/rand` seed；整个 `{{seed}}` 值保持整数类型，嵌入字符串时转十进制文本。
- `apps/core-go/internal/core/media_test.go:76-126` 已验证函数级替换及两次随机值不同。尚无提交失败再试的持久 seed 语义测试。
- `provider_prompt` 不是 ComfyUI 最终 JSON；最终工作流在 `media.comfyui.prompt_submitted` 诊断的 `request_payload` 中。
- 用户提供现场错误：`visual_identity.commit_review` 的 `observations` 在 Prepare 阶段未通过数组类型校验，外层报 `visual_identity_activity_failed · agent_run_failed ... capability_prepare_failed ... field "observations": value must be an array`。没有完整 ToolCall 参数，非数组的具体 JSON 类型未知。
- `apps/core-go/internal/core/visual_identity_tools.go:121-140` 要求 `observations` 为有界字符串数组；`apps/core-go/internal/core/visual_identity_agent.go:64-72` 只要求“bounded observations”，旧 `provider_schemas.go:662-669` Vision response 则声明对象。旧 schema 可能影响模型形态，但尚未证实为现场原因。
- `apps/core-go/internal/core/capability_core.go:1368-1387` 在 Prepare 时做输入 schema 校验；`tool_execution.go:210-216` 对普通参数错误默认 retryable，`adk_conversation_runtime.go:216-223` 因此终止 ToolNode。
- 已运行临时 Go overlay 测试 `go test -overlay=<temporary overlay> ./internal/core -run '^TestVisualIdentityObjectObservationsRegression$' -count=1`；对象型 `observations` 稳定产生 `invalid capability arguments: field "observations": value must be an array`。测试文件只在系统临时目录，仓库未修改。

## 上下文、引用及总结

- `apps/core-go/internal/core/prompt_context_assembler.go:415-447`：Runtime Facts、Active/Resident/Retrieved Memory、conversation summaries 进 `[RUNTIME CONTEXT]`，近期原始对话作为独立消息。
- `apps/core-go/internal/core/active_memory.go:36-66,83-135`：`active_memory:ctx_*` 是非 create 更新的 `target_ref`，Core 反解并校验 revision；不能无替代地删除。
- `apps/core-go/internal/core/provider_context.go:385-404,670-707`：普通对话 surface 的允许 ref 类型与 life context 嵌套 ref 的实际过滤不完全一致，可在保留 life_context ref 的前提下核实细粒度 refs。
- `apps/core-go/internal/core/conversation_summary.go:71-95,146-203`：summary 异步触发、保留最新 24 条、旧消息达到 6000 估算 token 或 20 assistant turn 或 40 条才排队。
- `apps/core-go/internal/core/provider_context.go:222-272`：raw recent 历史硬截最后 8 条，随后仍受 token budget 限制。与 summary 的 24 条保留区之间存在无需预算触发的空档风险。

## 时区与生活状态

- `apps/core-go/internal/core/intelligence.go:171-194,541-560`：当前 UTC instant 按 `identity.timezone` 转换后进入 Life Context；`apps/core-go/internal/core/life_context.go:91-105` 缺值回退 `Asia/Shanghai`。
- `apps/core-go/internal/core/workflow_ops.go:775-776`：`EnsureCurrentDaySchedule` 已有日程分支漏回 `timezone`；`apps/core-go/internal/workflow/workflow.go:498-519` 因而可能退化为从当前起睡 24 小时，而不是到当地午夜。
- `apps/core-go/internal/core/life_context.go:161-212`：有效 Event 优先于当前 Schedule；过期后自动回到日程。
- `apps/core-go/internal/core/life_capability_plans.go:10-16,61-108`：scene event 默认两小时。
- `apps/core-go/internal/core/life_activity_capabilities.go:190-200`、`apps/core-go/internal/core/effective_life.go:239-265`：活动 run 独立存储和读取，不随 Event 过期或日程结束自动结算。
- `apps/core-go/internal/core/detail.go:100-113` 同时返回 context 与 active activities；`apps/web/src/components/instances/InstanceDetailsDialog.vue:401-420` 当前只展示 context 的 scene/activity。
- `apps/core-go/internal/core/app.go:1196,1493`：LLM 初始化未识别出时区时持久化 `null`；blank 初始化默认 `Asia/Shanghai`。Web 初始化不采集浏览器 IANA 时区；运行期缺值退上海。
- `apps/core-go/internal/migrations/runner.go:226`、`apps/core-go/internal/core/model.go:58`：消息表和 DTO 只有 `created_at` UTC instant，没有发送者时区快照。
- `apps/web/src/views/ChatView.vue:146-159`：消息时间使用未指定 `timeZone` 的 `toLocaleTimeString`，旧消息会随查看时设备时区变化。`apps/web/src/stores/conversations.ts:253-263,557-571,606` 的乐观、排队和正式 turn 也未捕获时区。
- `apps/core-go/internal/httpapi/browser/routes.go:232-242,1755`、`apps/core-go/internal/core/agent_result_adapter.go:198-234`：turn 请求未传时区，Core 消息 INSERT 使用数据库 `created_at` 默认值；需沿写入、流、历史、生成 client 全链补字段。
- `apps/web/src/stores/control-center.ts:650-651` 将 `datetime-local` 用浏览器当前时区转成 ISO，但 `apps/web/src/views/GovernanceView.vue:18-20,117` 按人格时区展示；两时区不同时事件输入即偏移。
- `apps/core-go/internal/core/life_activity_capabilities.go:314-338` 当前要求虚拟购物 `completed` 必有 `acquired_item`；`life_activity_resolution.go:89-100` 已完成购物直接写衣橱，与用户明确的“活动结束不一定购入”冲突。
- 最近合入的 `09-27-goal-schedule-hairdye-closure` 任务（`.trellis/tasks/archive/2026-09/09-27-goal-schedule-hairdye-closure/design.md`）确立：未来虚拟活动经 Goal/Intention + accepted Schedule 关联事项启动，`IntentionTriggerWorkflow` 在 `not_before` 后推进结果；Provider 故障不得伪造完成，旧日程版本需安全回放。通用事件收敛不得破坏此路径。

## Tool 与 ADK 诊断

- `apps/core-go/internal/core/cognition_agent.go:83-125`：conversation agent 使用 conversation surface 能力目录。
- `apps/core-go/internal/core/intention_capabilities.go:130-184`：`intention.decide` schema 要 `operation,reason`，但非 create 运行时还要求 `intention_id`，精确错误 `intention_id_required`。用户未留日志，不能断言就是此项。
- `apps/core-go/internal/core/memory_intelligence.go:20-95`：`memory_event.revise` 要 `target_ref`；create 返回数据库 `memory_id`，不能直接作为 revise 的 opaque target_ref。`active_memory_event` create 也未返回可复用 ref。
- `apps/core-go/internal/core/adk_conversation_runtime.go:94-143`：native ToolCall ID 与业务 OperationID 分开，前者缺失会报 `adk_tool_call_id_required`，不能通过给业务 Tool 加 `id` 解决。
- `apps/core-go/internal/core/eino_model_runtime.go:72-116`：每个物理模型请求有独立 row/request 和递增 sequence，共用父 correlation。
- `apps/web/src/views/DiagnosticsView.vue:146`：诊断中心把同一 scenario 的每条 model run 平铺为相同标题，不区分 Tool 中间回合和最终回复；每条 row 自身的 Prompt/Response 已独立配对。
- `apps/core-go/internal/httpapi/browser/ndjson.go:526-633`：公开聊天流不透出 appraisal/reasoning/raw Tool trace；诊断视图才是 Owner 查看 model run 的位置。
