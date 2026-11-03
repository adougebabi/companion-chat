# 技术设计：摇光 Visual Identity

## 1. 边界与目标

本任务在现有 Go Core + Go Worker + Go BFF + Vue 3 架构内增加一条视觉身份领域链路。Temporal 仍是唯一 durable workflow runtime；PostgreSQL 仍是业务状态、时间轴和幂等边界；MinIO/S3 仍是私有图片资产存储；浏览器只通过 BFF 读取脱敏后的投影和媒体代理。

产品入口只有两个：

1. `CreateFluctlight` 在初始化事务中提交 Visual Identity 初始化 intent；
2. `WakeUp` 读取到没有有效 visual identity 时，让摇光通过既有认知/行动链告知 Owner“需要创建自己”，并在同一事务幂等提交初始化 intent。

两条入口调用同一个 Core helper，不新增按钮/API 作为启动入口，也不从普通文本做关键词解析。

## 2. 持久化模型

新增 PostgreSQL 表（迁移 runner 中按 additive/clean-start 方式加入）：

- `fluctlight_visual_identities`：每个 Fluctlight 一个当前聚合，保存 `id`、`fluctlight_id`、`status`、`current_revision`、`identity_snapshot`、`renderer_constraints`、`canonical_asset_id`、`character_sheet_asset_id`、`adapter_version`、`active_session_id`、时间戳。
- `fluctlight_visual_identity_revisions`：只追加的 accepted canonical revision，保存 before/after snapshot、canonical/character-sheet asset IDs、evidence/source、CAS 基线、idempotency key。
- `fluctlight_visual_identity_sessions`：一次初始化/再收敛会话，保存 trigger (`initialization|wakeup`)、workflow ID、状态、`max_attempts=3`、当前 attempt、source wake-up/foundation fact 和时间戳。
- `fluctlight_visual_identity_attempts`：每轮候选，保存 attempt number、状态、seed prompt、输入快照、renderer constraint 快照、media intent/asset IDs、vision JSON、patch JSON、decision、feedback、错误和时间戳。
- `fluctlight_visual_identity_timeline`：面向时间轴的 append-only 阶段事件；每行含 session/attempt、stage、status、safe summary、asset IDs、correlation/workflow ID、occurred_at。原始 prompt、Provider 响应、推理内容和凭据不进入 browser projection。

所有表都带 `fluctlight_id` 作用域；session/attempt/timeline 使用稳定主键和唯一约束防止重放重复。历史候选 asset 永不被 canonical 提升覆盖或删除。

## 3. Visual identity slot 与 ContextProjection

新增 `VisualIdentitySnapshot`/读取 helper，作为对外 `visual_identity` slot 的专用实现。`BuildContextProjection` 增加 `VisualIdentity` 字段，包含当前聚合快照、revision、状态和 renderer constraints；`compactCognitionContext` 与 `media_context` 使用同一快照。

视觉身份是稳定外观约束，优先级高于 transient current state，但不改变 life Event > Schedule > pending 的 scene authority。普通 Scene Image 只读取当前已 accepted/active snapshot；没有 canonical 时明确返回 `identity_pending`，不推断默认外貌。

## 4. Renderer constraint 与胸部 LoRA adapter

身份语义保存为 `appearance.chest_cup`（先支持归一化后的 A/B/C/D；不支持的值返回配置错误）。Core 内新增纯函数 adapter，例如 `chestCupToLoRAWeight(cup) (float64, adapterVersion, error)`，用显式 if/else 映射；A/B 的具体数值作为代码中的待验证常量，不在本任务中假定业务正确性。

写入/冻结时同时保存：

```json
{
  "chest_cup": "B",
  "chest_lora_weight": -3,
  "adapter_version": "chest-cup-adapter.v1"
}
```

weight 必须是有限数且在 `[-10,10]`。每个 attempt 的 renderer snapshot 包含原始语义值、解析值和 adapter 版本；accepted 时才写入正式 revision。Scene Image 和 Visual Identity media concept 都从该 snapshot 传递 weight，ComfyUI 节点 JSON 仍由用户提供。

## 5. Temporal workflow 与阶段

新增 `VisualIdentityWorkflow`，注册在 `lifecycle` queue，稳定 workflow ID 由 session ID 派生；dispatcher 新增 `visual_identity.initialize` intent type。工作流只保存小型 ID/状态，不把图片二进制或完整 Provider 输出放入 history。

每个 attempt 的确定性阶段为：

```text
session_created
→ seed_requested / seed_ready
→ image_requested
→ image_queued / image_running / image_ready
→ vision_requested / vision_ready
→ patch_requested / patch_ready
→ accepted
或 regenerate → next attempt
→ character_sheet_requested / character_sheet_ready
→ completed
```

媒体实际提交仍复用 `media_intents` + `MediaWorkflow`（media queue）。Visual Identity workflow 通过短 Activity 查询 media intent/asset 状态和 durable sleep 等待，不重复提交 Provider job；每轮 provider request/job ID 都从 attempt 稳定 ID 派生。

`visual_identity_patch(stage=seed)` 生成首轮纯文本 seed prompt；`visual_identity_vision` 接收 BFF/Storage 可读的图片内容并输出结构化观察；`visual_identity_patch(stage=review)` 接收 visual identity snapshot、vision、上一轮 patch 和 feedback，输出 `accepted|regenerate`、下一轮 patch 及可选 renderer patch。三者均保存结构化结果和 schema version。

`accepted` 在短事务中 CAS 更新当前聚合、追加 revision 和 canonical asset 引用，然后创建 character-sheet media intent；character sheet ready 后再写入 `character_sheet_asset_id` 并完成 session。达到三次 attempt 且仍 regenerate 时进入 `awaiting_review`，不继续生成。

## 6. 初始化与 Wake-up 触发

`CreateFluctlight` 的现有创建事务在 Foundation、inner state、conversation、schedule intent 旁边插入：

- visual identity 聚合初始行（`missing`/`initializing`）；
- session + attempt-1/timeline `session_created`；
- 一个稳定的 `visual_identity.initialize` lifecycle intent。

事务提交前不调用 Provider/Temporal。若请求重放，按 Fluctlight/session 唯一键返回已有记录。

Wake-up 活动读取 visual identity 状态并把 `visual_identity_missing` 作为结构化 context 输入。若没有 active canonical，动作实现阶段要求模型生成“我需要创建自己的视觉身份”的可见表达；同一个 wake-up 事务调用共享 helper，以 wake-up ID 做幂等键提交/复用 session 和 intent。若已有 running/accepted session，不重复排队。

## 7. 浏览器契约与时间轴

新增 browser-safe 读取投影（建议挂在 Fluctlight detail，必要时提供 `GET /api/fluctlights/:id/visual-identity` 只读别名）返回：当前 snapshot、revision、status、renderer constraints、session summary、timeline stages 和已授权 asset IDs。原始 vision/patch 只返回 bounded summary，不返回内部推理或 Provider payload。

`visual_identity.initialize` 没有独立公共 POST 入口；“再次生成”由 patch 的 `regenerate` 决策驱动。若后续要人工干预，使用现有治理/工作流审计边界扩展，不在本 MVP 添加第三个产品入口。

前端在 Fluctlight detail/聊天关联区域增加 `visual-generation-timeline`：按 timeline stage 渲染时间、attempt、状态、缩略图和 safe summary；候选图按 asset ID 走 `/api/media/:assetId`。刷新重新读取 detail projection；不通过浏览器自行判断“是不是自己”，不复用 transport retry。

## 8. 兼容性与失败策略

- 旧 Fluctlight 没有 visual identity 行：读取时返回 `missing`，首次符合初始化/wake-up 条件时惰性补建；不阻断既有聊天。
- 旧 MediaIntent 没有 visual snapshot：Scene Image 继续旧路径；新的 Visual Identity intent 必须带完整快照，否则 terminal migration/configuration failure，禁止猜测。
- provider role 未配置、ComfyUI workflow JSON 缺失、vision/patch schema 无效：timeline 记录 bounded failure/pending，Temporal 按策略 retry；不提交 heuristic fallback prompt。
- cancellation/worker restart/provider retry 使用稳定 session/attempt/media IDs，已完成阶段只读恢复；过期 attempt 不能提交 canonical。
- 所有跨表写入使用短事务和 CAS；outbox 事件包含 fluctlight/session/attempt/workflow/correlation IDs。

## 9. 主要测试面

- adapter 边界、罩杯归一化、`[-10,10]`、快照/版本和不支持值。
- 初始化事务的 session/intent 幂等；Wake-up 缺失检查、可见提醒和同次排队。
- workflow timer/retry/replay、media job reuse、三次 attempt 上限、accepted/regenerate、canonical/character-sheet CAS。
- ContextProjection/media context/Scene Image 同一 visual identity snapshot；未初始化 pending。
- BFF 授权、safe timeline projection、asset proxy 和 Vue refresh/late media 状态；不泄露 provider/MinIO/prompt。
