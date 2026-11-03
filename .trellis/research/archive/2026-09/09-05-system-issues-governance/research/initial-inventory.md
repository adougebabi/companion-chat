# 初始现状盘点（2026-09-05）

本记录来自只读代码检索，未修改生产代码。

## Workflow 清单

当前生产代码注册 11 个 Temporal workflow，分布在三个 task queue：

| intent type | workflow | queue | activity/行为 |
| --- | --- | --- | --- |
| `visual_identity.initialize` | `VisualIdentityWorkflow` | `lifecycle` | `ProcessVisualIdentityActivity`，未完成时 `ContinueAsNew` |
| `wake_up.current` | `WakeUpWorkflow` | `lifecycle` | `ProcessWakeUpActivity`，按 cadence `Sleep`/`ContinueAsNew` |
| `daily_review.current_day` | `DailyReviewWorkflow` | `lifecycle` | `ProcessDailyReviewActivity`，等待本地午夜 |
| `schedule.current_day` | `CurrentDayScheduleWorkflow` | `lifecycle` | `EnsureCurrentDayScheduleActivity` |
| `reflection.run` | `ReflectionWorkflow` | `lifecycle` | `ProcessReflectionActivity`，一次性 evidence window |
| `memory.embedding` | `MemoryEmbeddingWorkflow` | `lifecycle` | `ProcessMemoryEmbeddingActivity` |
| `platform.control` | `PlatformControlWorkflow` | `lifecycle` | 仅监听 `stop` signal，生产 body 未执行 control activity |
| `media.generation` | `MediaWorkflow` | `media` | `ProcessMediaActivity`，最多一次 quality retry |
| `autonomy.action` | `AutonomyActionWorkflow` | `interaction` | `ProcessAutonomyActionActivity` |
| `capability.action` | `CapabilityActionWorkflow` | `interaction` | `ProcessCapabilityActionActivity` |
| `cognition.processing` | `CognitionProcessingWorkflow` | `interaction` | `ProcessCognitionActivity` |

注册和 queue 并发见 `apps/core-go/internal/workflow/workflow.go:124-668`；intent 映射和 Temporal start 见 `apps/core-go/internal/workflow/workflow.go:870-943`。Worker 每秒运行 dispatch/reconcile/outbox/consumer，见 `apps/core-go/cmd/worker/main.go:100-140`。BFF 通过 Core 提供 workflow list/status/history/commands，见 `apps/gateway-go/internal/bff/routes.go:299-324,718-740`。

## Googleapis CSS

唯一确认的运行时远程 CSS 是：

```css
@import url('https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700&display=swap');
```

位置：`apps/web/src/styles/globals.css:1-6`。入口链为 `apps/web/index.html` → `apps/web/src/main.ts` → `globals.css`/`app.css`。当前同时出现 `Geist`、`Geist Variable`、`Noto Sans SC` 三组字体名，且没有本地字体文件或 `@font-face`。已移除 Google Fonts import，并将 `--font-sans` 与 `app.css` 的中文系统字体回退链统一；`pnpm --filter @fluctlight/web test`（32/32）和 `pnpm --filter @fluctlight/web build` 均通过。

## Provider queue / Redis

- Provider generated LLM queue 是每个进程独立的 in-process priority/FIFO heap，见 `apps/core-go/internal/core/provider_queue.go:25-269`；API 与 Worker 各自有一套，不能提供集群级顺序/并发。
- priority 映射集中在 `apps/core-go/internal/core/diagnostics.go:74-125`，`media_prompt` 与 `reflection`/`wake_up` 等共享 generic generated queue。
- Redis 当前只用于 Streams/outbox/consumer groups，见 `apps/core-go/internal/platform/redis_pipeline.go:17-286`；`infra/redis/redis.conf:1-5` 未配置 `notify-keyspace-events Ex`。
- `platform_workflow_intents`、Temporal history 和 diagnostic model run 仍是 durable 状态，Redis ZSET 不能直接取代其 lease/ack/recovery 语义。

## Media diagnostics

媒体 workflow 会产生两类可见提示词：

1. `media_prompt` Provider LLM 生成的英文 `provider_prompt`，写入 `media_intents.provider_prompt` 并记录 `diagnostic_model_runs`；
2. 实际提交到 ComfyUI 的 prompt，记录为 `diagnostic_events`，event type 为 `media.comfyui.prompt_submitted`，correlation 为 `media:<intentID>`。

当前两者 correlation 不同，也没有 `media_intent_id` 可直接 join。`diagnostic_model_runs.role` 写入路径同时使用 binding role，导致 `media_prompt` 可能落库为 `generic_llm`；Web 条件 `run.role === 'media_prompt'` 见 `apps/web/src/views/DiagnosticsView.vue:96-103`，存在最终提示词不显示的风险。媒体 provider/quality prompt 仍须经过现有 redaction，不能泄露图片 data URL。

## Reflection / wake-up

- Reflection intent 来源包括普通 cognition、native fact、capability/action result 和 wake-up settlement；见 `apps/core-go/internal/core/cognition.go:296-323`、`native_capabilities.go:225-241`、`wakeup.go:517-616`。
- `ReflectionWorkflow` 使用 evidence window、watermark、running claim 和 state revision CAS，见 `apps/core-go/internal/core/workflow_ops.go:285-423`；不能按 Fluctlight 粗暴合并所有 reflection source。
- 当前没有独立的反思 delay setting；第一阶段若需要延迟用户 turn 的 reflection，唯一现有 cadence 配置是 `product.wakeup.interval_seconds`，因此实现复用该值并记录为后续配置拆分决策。
- Wake-up 当前是长期 `WakeUpWorkflow`，按 `product.wakeup.interval_seconds`（默认 1800）执行 `Sleep`/`ContinueAsNew`，见 `apps/core-go/internal/workflow/workflow.go:157-195`、`apps/core-go/internal/core/wakeup.go:19-65`。
- Redis keyevent 是非持久 Pub/Sub；实现 TTL listener 必须保留 startup repair 和 periodic due scan，listener 只做低延迟 nudge。

## 第一阶段实现验证

- Google Fonts import 已移除，`--font-sans` 改为与现有 `Noto Sans SC`/PingFang/Hiragino/system-ui 回退链一致；Web 32 项静态测试、typecheck 和 Vite production build 通过。
- 新增 `/internal/diagnostics/media-prompts` 与 `/api/diagnostics/media-prompts`，Core 以 `media_intents.created_at DESC, id DESC` 限制最近 20 条，并按 `media:<intentID>` 关联 provider model run 和 ComfyUI submission event；BFF/Browser Client/Vue 独立展示。
- `diagnostic_model_runs.role` 现在保存语义 role，`binding_role` 保存 `generic_llm`/`embedding`；媒体 prompt provider 调用显式使用 `media:<intentID>` correlation。
- Provider generated queue 增加 Redis ZSET pending/processing、Lua 原子 claim/release/requeue、lease renewal、job TTL 和 Redis 不可用回退；API/Worker 均接入可选 Redis client，embedding 仍独立。
- Worker 增加 Redis expiration listener、startup wake-up hint repair 和每秒 provider queue lease reconciliation；用户 turn reflection intent 复用 `product.wakeup.interval_seconds` 作为 delay，并合并为每 turn 一个 delayed reflection intent。
- `go test ./...`（Core）、`go test ./...`（BFF）、Browser Client typecheck/test 均通过。需要本地监听或 Docker 的 Redis/Compose integration smoke 在当前沙箱被禁止绑定端口，未冒充通过。
- 后续获得 Docker 权限后，`FLUCTLIGHT_ENV_FILE=infra/compose/fluctlight.local.env infra/compose/run-platform-smoke.sh` 已成功完成：bind source 检查、Core/Worker/Temporal/PostgreSQL/Redis/MinIO/BFF/Web 健康检查，以及 migrate/minio-init/cutover 成功退出；脚本使用独立临时 project 并已清理容器/卷。
