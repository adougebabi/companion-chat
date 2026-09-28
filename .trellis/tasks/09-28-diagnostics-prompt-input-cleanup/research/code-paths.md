# 2026-09-28 只读排查

## 诊断记录与脱敏

- `diagnostic_model_runs` 仅有 `metrics jsonb`，没有独立的 sequence/stage 列（`apps/core-go/internal/migrations/runner.go:338`）。ADK 的 `queuedToolCallingChatModel.Generate/Stream` 会写 `model_call_id` 与递增 sequence（`apps/core-go/internal/core/eino_model_runtime.go:72-140`），但非 ADK 的 `completeWithToolsSchemaMode` 不经此路径（`apps/core-go/internal/core/provider.go:329-372`）。
- `decorateModelRunRounds` 只处理带 `model_call_id` 的行；没有 ID 时直接返回，留下 `stage=unknown` 和空 sequence（`apps/core-go/internal/core/diagnostic_model_runs_filter.go:88,113-125`）。Vue 将空 sequence 显示成“轮次未知（旧记录）”（`apps/web/src/views/DiagnosticsView.vue:74-80`）。既有 ADK 和真正空 metrics 的旧记录测试位于 `apps/core-go/internal/core/diagnostics_test.go:320,475`。
- `redactDiagnostic` 将 `arguments`、`reasoning_content` 和凭证名键替换为 `[REDACTED]`，图片 data URL 替换为 `[REDACTED_IMAGE_DATA]`（`apps/core-go/internal/core/diagnostics.go:261-319`）。模型诊断在写入时已执行脱敏，读取时再次执行（同文件 `:505-514`；`diagnostic_model_runs_filter.go:81-88`）；已替换旧值无法恢复。
- 多模态 Eino 消息将内容存于 `UserInputMultiContent`（`eino_model_runtime.go:1058-1085`），但 `einoMessageRaw` 只读 `Content`（同文件 `:890-906`）。故目前的 model-run 诊断可能漏掉实际发送的文字和图片占位；应通过真实 HTTP wire 与诊断记录对照。
- Provider 绑定来自 `ProviderClient.assignment`，`endpoint_id` 和 `model_id` 均已存储并进入 Browser DTO；页面只显示绑定角色与 model ID，未使用 endpoint ID（`provider.go:100-137`、`diagnostics.go:516-553`、`apps/web/src/views/DiagnosticsView.vue:206`）。第 2 点展示样式暂不改。

## 模型输入

- Visual Identity 的 system 文案和 payload 在 `visual_identity_agent.go:64-72,152-176`。payload 含多种内部 ID、完整 `identity_snapshot`、约束、历史和图片。`refreshVisualIdentityRendererConstraints` 用完整 persona identity/life_profile 覆盖窄快照，导致 `background`/`background_story` 进入模型输入（`visual_identity.go:1019-1029,1091-1126`）。Tool 目标由 Core 绑定，schema 不要求模型回传这些 ID（`visual_identity_tools.go:105-159`）。保留真实图片和 review 语义。
- 计划生成在 `schedule_generation.go:18-58` 装入原始 goals/intentions/outcomes 等 DTO；初始化 goal ID 和 `foundation:<fluctlight_id>` 在 `app.go:2026,2055,2093-2096` 生成。goal 的 `description/desired_outcome`、intention 的 `action/action_intent` 在存储时同值（`evolution_persistence.go:36-47,84-95`）；response schema 不接收内部 ID（`provider_schemas.go:318-331`）。
- `mediaQualityMessages` 明确输入为 media kind、frozen concept、provider prompt、重试次数与真实图片（`media_quality.go:132-151`）。其 text 先被序列化为 JSON，再由公共 formatter 转为 YAML/TOON（`model_tasks.go:122-127`、`internal/ai/prompt/format.go:20-68`）。Visual Identity 和 schedule 也有类似中间 JSON；诊断页面 `JSON.stringify` 的外壳不等于 Provider message text 为 JSON。媒体 concept 的 visual snapshot 缺视觉字段 allowlist，可能带入背景故事（`provider_context.go:1815-1832`）。
- Wake-up 的 `schedule_ref` 是 Core 生成的合法 opaque token（`context_reference.go:106,372-410`），通过共享 life-context map、surface allowlist 进入 prompt（`intelligence.go:335-386`、`provider_context.go:766-785`）；独立 schedule fact 还会重复输出该 ref 与日程语义（`provider_context.go:1012-1031`）。Tool 路由使用内部冻结 index，不依赖模型看到该 token（`capability_core.go:313-346`）。现有测试反而断言 wake-up 保留嵌套 ref（`provider_context_test.go:960-1005`），需按新要求更新。

## 超时

- `provider.go:342` 创建一次 `context.WithTimeout(runCtx, assignment.Timeout)`，`generateWithEino` 和 `RunADKLoop` 的所有模型、队列、Tool 往返共用它（`eino_model_runtime.go:632-709`）。生产 Agent 没有五轮固定上限（`formal_agents.go:61-81`、`internal/ai/agent/loop.go:248-270`）。
- 设置页空值 60 秒（`apps/web/src/views/SettingsView.vue:26,54`）；数据库和后端默认 120 秒（`migrations/runner.go:249`、`provider.go:127-135`）；初始化最少 600 秒（`provider.go:80-97`）；生产 HTTP client 每次物理请求 900 秒（`app.go:117-130`）。所有非 embedding 角色通常共用 `generic_llm` 配置（`diagnostics.go:196-201`、`provider.go:100-137`）。本地无法读取实际持久值：Docker 不可用、无数据库连接变量。诊断工作流列表的 5 秒 deadline 属于另一路径，不应混改（`.trellis/tasks/09-26-workflow-diagnostics-polling-timeout/design.md:7`）。
- 可复现接缝：扩展 `eino_adk_runtime_test.go:828-879` 的受控多轮 Provider；各轮低于超时、累计高于超时，当前实现应在共享 deadline 失败。另用 fake model 记录每轮 `ctx.Deadline()`，避免计时抖动。

## 现有任务约束

- `.trellis/tasks/09-27-adk-diagnostics-display/design.md` 已定义物理 sequence/stage 与真正旧记录兼容，不能另起一套字段语义。
- `.trellis/tasks/09-27-visual-identity-review-contract/design.md` 要求保留真实图片、专用 `commit_review`、纠错回合和业务校验；原始 Tool 参数诊断仅存安全摘要。
- `.trellis/spec/backend/fluctlight-diagnostics-contract.md:54-55,85` 禁止保存隐藏推理和凭证，初始化普通 model-run 仅存元数据；这些边界与“全部原样展示”有冲突，需用户确认。

## 实施后验证补记

用户已明确选择仅隐藏图片；相应诊断与 Provider 规范已更新。最初只读排查时 Docker 不可用，实施后启动了 OrbStack，在一次性 pgvector PostgreSQL 容器中运行完整 Go Core 数据库测试集以及新增的最终 Provider HTTP wire、Visual Identity 评审续传、诊断写入/查询/导出回归，全部通过；容器已停止并自动删除。真实外部 Provider、ComfyUI/MinIO、Temporal 与手工 Owner 页面验收仍不在该隔离测试环境中。
