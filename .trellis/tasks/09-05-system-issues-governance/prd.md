# 系统问题治理（第一阶段，不含提示词重构）

## Goal

在不改变提示词构成协议的前提下，先治理当前项目中的工作流可见性、外部字体依赖、媒体提示词诊断、LLM 调度以及反思/唤醒触发问题。所有改动在现有 Go Core、Go BFF、Vue 客户端、PostgreSQL、Redis 和 Temporal 架构内完成，并保留可恢复性与幂等性。

提示词构成重构（此前讨论中的第 4 项）明确排除在本任务之外，待本任务完成后另行确认格式、协议和兼容策略。

## Scope and work items

本任务只有一个 Trellis 任务，内部包含以下五个按顺序执行的工作项：

1. **现状盘点**
   - 形成当前所有 Temporal workflow、intent 类型、task queue、主要生产者、触发条件、管理入口和前端观测入口的清单。
   - 调查 `apps/web/src/styles/globals.css` 中 Googleapis CSS 的用途、最终字体命中情况、构建产物行为和部署访问风险。

2. **Googleapis CSS 处理**
   - 根据盘点结果移除不必要的外部 CSS，或将确有必要的字体资源改为项目可控的本地/自托管方案。
   - 不引入未经确认的字体授权或新的远程运行时依赖。

3. **诊断中心媒体提示词独立展示**
   - 将“提交媒体生成请求所使用的提示词”从普通 LLM model run 展示中独立为媒体提示词模块。
   - 默认覆盖当前真实存在的图片媒体链路，以及同一媒体诊断模型可表达的质量检查/媒体生成阶段；不虚构当前尚未注册的 video/audio capability。
   - 按发起时间倒序展示最近 20 条。
   - 保留 prompt 脱敏、owner-only 权限和现有诊断保留策略。
   - 让媒体模型调用与 ComfyUI 实际提交事件具备稳定、可查询的关联关系；不得依赖当前不稳定的 `provider:<hash(messages)>` 与 `media:<intentID>` 并列展示。

4. **LLM 权重调度**
   - 使用 Redis Sorted Set 表达生成类 LLM 等待任务的优先级与 FIFO 顺序，同时保留 PostgreSQL diagnostic model run 和现有调用返回语义。
   - 当前 priority 映射（reply/autonomy_reply=100、cognitive_assessment/native_cognition/daily_review/schedule_generation=90、media_prompt/media_quality_acceptance=80、reflection/wake_up=70、initialization=60、其他=50）作为默认业务权重来源，不另造一套配置。
   - 支持原子 claim、processing lease、超时/失败重入、取消、幂等和进程/worker 重启后的恢复；不能使用裸 `ZPOPMIN/ZPOPMAX` 造成任务丢失。
   - Redis 作为调度层或加速索引，PostgreSQL/现有 durable intent 与诊断记录仍是事实和恢复依据；除非后续明确批准扩大架构边界，不迁移 Temporal intent dispatcher。

5. **反思与唤醒触发**
   - 当前仓库没有独立的反思 delay 配置；第一阶段沿用 `product.wakeup.interval_seconds` 作为用户消息后反思的 delay 来源，并在后续提示词/设置重构时再单独确认是否拆出配置。
   - 按当前配置和现有反思语义，为用户消息后的反思调度建立 Redis TTL key，并监听过期事件。
   - 反思 key 过期后最多触发一次对应反思；唤醒执行一次后重新设置下一次 key，形成连续 cadence。
   - 保留现有 reflection evidence window、watermark、state-revision CAS、稳定 workflow ID 和 `cognition_wakeups` 唯一约束。
   - Redis 过期事件只作为低延迟触发提示；必须有启动修复/定期补偿扫描和 PostgreSQL/Temporal 幂等校验，避免 Pub/Sub 事件丢失导致业务永久丢失。

## Confirmed repository facts

- 生产 workflow runtime 是 Temporal，当前注册 11 个 workflow，分布在 `lifecycle`、`media`、`interaction` 三个 task queue；注册和映射见 `apps/core-go/internal/workflow/workflow.go:124-668`、`apps/core-go/internal/workflow/workflow.go:870-921`。
- `platform_workflow_intents` 是 durable workflow start ledger，Worker 每秒 dispatch/reconcile；见 `apps/core-go/cmd/worker/main.go:100-140` 和 `apps/core-go/internal/workflow/workflow.go:798-943`。
- Core API 与 Worker 当前各自拥有独立的进程内 Provider priority/FIFO queue；见 `apps/core-go/internal/core/provider_queue.go:25-269`、`apps/core-go/internal/core/app.go:47-56`。
- Redis 当前用于 Streams/outbox/consumer groups，尚未配置 keyspace expiration notification，也没有 Redis ZSET/provider lease；见 `apps/core-go/internal/platform/redis_pipeline.go:17-286`、`infra/redis/redis.conf:1-5`。
- Provider 的语义 role 与 binding role 在诊断记录路径存在混淆风险；`media_prompt` 当前绑定到 `generic_llm`，但诊断 UI 用 `run.role === 'media_prompt'` 展示最终提示词；见 `apps/core-go/internal/core/diagnostics.go:67-72,235-273`、`apps/web/src/views/DiagnosticsView.vue:96-103`。
- ComfyUI 实际提交提示词是 `diagnostic_events` 的 `media.comfyui.prompt_submitted` 事件，correlation 为 `media:<intentID>`；媒体 Prompt LLM model run 通常使用 `provider:<hash(messages)>`，目前没有 `media_intent_id` 直接关联字段；见 `apps/core-go/internal/core/media.go:61-138`、`apps/core-go/internal/core/provider.go:133-156`。
- 唯一确认的运行时外部 CSS 是 `apps/web/src/styles/globals.css:1-6` 的 `https://fonts.googleapis.com/...Geist...`；当前代码同时声明 `Geist`、`Geist Variable` 和 `Noto Sans SC`，没有本地字体文件。
- 反思当前由多个 cognition/native/action/wakeup producer 插入一次性 `reflection.run` intent；其 evidence window 与 CAS 不能被按 Fluctlight 粗暴合并；见 `apps/core-go/internal/core/cognition.go:296-323`、`apps/core-go/internal/core/native_capabilities.go:225-241`、`apps/core-go/internal/core/wakeup.go:517-616`、`apps/core-go/internal/core/workflow_ops.go:285-423`。
- 唤醒当前由长期 `WakeUpWorkflow` 的 `workflow.Sleep`/`ContinueAsNew` 驱动，默认 1800 秒；见 `apps/core-go/internal/workflow/workflow.go:157-195`、`apps/core-go/internal/core/wakeup.go:19-65`。

## Acceptance Criteria

- [ ] 现状盘点产出可审阅的 workflow 清单，覆盖 workflow symbol、intent type、queue、activity、生产者、触发条件、管理入口和前端观测入口；同时给出 Googleapis CSS 的用途和构建/运行时验证结果。
- [ ] 生产构建或部署验证不再依赖未批准的 Googleapis 运行时 CSS；字体回退链稳定，中文和英文界面无明显布局回归。
- [ ] 诊断中心保留普通 LLM model runs，同时新增独立媒体提示词区块；仅显示最近 20 条、按发起时间倒序，并能显示脱敏后的 provider prompt/实际提交 prompt 及其关联信息。
- [ ] 媒体诊断链路中，语义 role、binding role、scenario 和 media intent/correlation 关系可被 Core 查询、BFF 转换和 Vue 展示一致消费；不得破坏现有 owner-only、redaction 和 retention 行为。
- [ ] 同一 Redis-backed LLM queue 在多进程场景下按现有 priority 与 FIFO 规则原子取任务；worker 崩溃、Redis 短暂不可用、provider 超时、取消和重试均有可验证的恢复路径，且 diagnostic model run 最终状态不悬挂。
- [ ] Redis 过期事件触发反思/唤醒时最多产生一个有效 workflow/intent；listener 断线、Redis 重启或事件丢失后，补偿扫描能恢复 due work；现有 evidence window、watermark、CAS、cycle 唯一约束和 workflow management 不回归。
- [ ] Go Core、Go BFF、Vue 客户端相关 lint/type/test/build 及必要的 Compose smoke 均通过；变更后的跨层 payload、路由和诊断展示有针对性测试。

## Out of scope

- 提示词构成第 4 项：固定 system 协议、人格区块、TOON/YAML/Markdown 编排及 `media_prompt` 例外格式。
- 新增 video/audio capability、完整的视频/音频生成产品闭环；当前媒体诊断只覆盖实际存在的能力和可查询阶段。
- 将 PostgreSQL durable ledger、Temporal workflow history 或现有 reflection evidence authority 迁移到 Redis。
- 引入新的外部字体、远程 CDN 或未审查的第三方 Redis 调度库。

## Confirmed technical decision

- **Redis 的权威边界（用户已确认）**：Redis ZSET 和 key expiration listener 只做低延迟调度/唤醒提示，PostgreSQL/Temporal 继续作为事实来源、幂等和恢复依据。本任务不把 Redis 变成唯一队列或唯一触发事实，也不迁移 Temporal intent dispatcher。
