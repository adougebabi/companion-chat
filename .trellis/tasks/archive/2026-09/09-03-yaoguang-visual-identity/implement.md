# 实施计划：摇光 Visual Identity

本任务的数据库、workflow、媒体、API/BFF 和 Vue 时间轴相互依赖，先完成一体化 vertical slice；不拆成并行 child task，以免在 session/attempt/asset 契约尚未稳定时产生跨分支冲突。实现阶段在独立工作树中进行，主工作树已有的 `.trellis/spec/backend/*.md` 未提交改动保持不动。

## 0. 工作树与上下文

- [ ] 从当前基线创建独立工作树和 `codex/yaoguang-visual-identity` 分支。
- [ ] 在目标工作树运行 `trellis-before-dev`，读取 backend/frontend package/layer 指南及 cross-layer/code-reuse thinking guides。
- [ ] 复核本 PRD/design 与现有 `master` 差异；确认不覆盖用户已有改动。

## 1. 数据与领域模型

- [ ] 在 `apps/core-go/internal/migrations/runner.go` 添加 visual identity aggregate/revision/session/attempt/timeline 表、索引、唯一约束和兼容列；补迁移 runner 测试。
- [ ] 新增 Visual Identity domain types、状态枚举、timeline stage 常量和 bounded JSON validators。
- [ ] 实现 `chestCupToLoRAWeight` adapter（A/B/C/D 归一化、显式 if/else、有限数与 `[-10,10]` 校验、adapter version），并为边界/不支持值添加单测。
- [ ] 实现创建/读取/快照/CAS/revision/outbox helper，确保 accepted canonical 才更新正式 visual identity。

## 2. Context 与 provider contracts

- [ ] 将 `VisualIdentity` 安全快照加入 `ContextProjection`、compact provider context 和 media context binding；修复 `life_profile.appearance` 与 identity appearance 的层级缺口。
- [ ] 扩展 provider role 白名单、配置/预检和角色显示：`visual_identity_vision`、`visual_identity_patch`。
- [ ] 增加 seed/review/vision structured schemas；支持受限的 multimodal image content 输入，不把 Provider 原始响应带入 browser。
- [ ] 保持现有 `media_prompt`/`media.image.generate` 兼容；新增 concept envelope 字段只做结构校验和快照绑定，不加入语义关键词推断。

## 3. Durable workflow 与触发器

- [ ] 新增 lifecycle queue 的 `VisualIdentityWorkflow`、activities、durable wait/retry/continue-as-new 所需输入和 registry 映射。
- [ ] 为初始化实现事务内的 profile/session/attempt/timeline + `visual_identity.initialize` intent；不在 HTTP 请求内调用 Provider/Temporal。
- [ ] 为 Wake-up 增加缺失状态投影、可见提醒的 LLM action contract，以及同次 wake-up 幂等复用/排队 helper；已有 active/running session 不重复排队。
- [ ] 实现 attempt 阶段：seed → media intent → media ready → vision → patch decision → regenerate（最多 3 次）→ canonical → character sheet。
- [ ] 复用 `MediaWorkflow`/`media_intents`，保存 provider job IDs 并在恢复时只轮询既有任务；过期 attempt/cancel 不得提交 canonical。

## 4. Scene Image 与媒体绑定

- [ ] 在现有 scene/image concept 绑定中读取同一 visual identity snapshot、胸部语义值、解析 weight、adapter version。
- [ ] 未初始化或 mapping invalid 时返回 `identity_pending`/`renderer_config_pending`，不提交 provider job；已有旧 MediaIntent 保持兼容。
- [ ] 为 visual identity candidate/canonical/character-sheet 资产建立安全 media references，并保证 BFF 只暴露授权 asset ID。

## 5. Core HTTP、BFF 与 browser client

- [ ] 在 Core detail/read model 增加 visual identity summary、session/attempt/timeline safe projection；必要时增加内部只读 route。
- [ ] 在 Go BFF 添加授权后的 Fluctlight visual identity projection route/DTO，并更新 route inventory、OpenAPI artifact、generated browser client 和 parity tests。
- [ ] 明确不提供独立初始化 POST 产品入口；timeline/read 使用已有 Fluctlight detail 语义，workflow IDs/asset IDs 仅以安全投影形式出现。

## 6. Vue 时间轴

- [ ] 在 `InstancesView`/`InstanceDetailsDialog`（或最终确定的 Fluctlight detail owner）增加 visual-generation timeline：stage、attempt、时间、状态、safe summary、缩略图。
- [ ] 从 BFF `/api/media/:assetId` 读取图片；为 queued/running/pending/failed/accepted/character-sheet 提供稳定尺寸、可访问标签和错误状态。
- [ ] 不复用 transport `retry()` 作为“不是自己”重生成；由后端 patch decision 驱动 attempt 追加，刷新后从 server projection 恢复历史。
- [ ] 若聊天 `media` 事件承载可见候选引用，补齐 conversations store 的 `media` merge；不让晚到 queued placeholder 覆盖 ready projection。

## 7. 测试与验证

- [ ] Go unit tests：adapter、schema、snapshot/CAS、attempt state machine、timeline projection、Scene Image binding。
- [ ] Go integration tests（可用现有 disposable Postgres/Temporal harness）：初始化事务、Wake-up 同次排队、media job reuse、vision/patch retry、三次上限、canonical/character-sheet、取消/重启。
- [ ] BFF/API contract tests：授权、错误映射、OpenAPI/generated client、无 provider/MinIO/prompt 泄露。
- [ ] Web tests：timeline rendering、刷新恢复、媒体事件 merge、旧 queued → ready projection、稳定媒体盒子和错误态。
- [ ] 运行最小验证：
  - `GOCACHE="$PWD/.gocache" go test ./apps/core-go/internal/core ./apps/core-go/internal/workflow ./apps/core-go/internal/migrations`
  - `GOCACHE="$PWD/.gocache" go test ./apps/gateway-go/internal/bff`
  - `pnpm --filter web test`（按仓库实际脚本调整）
  - `pnpm --filter web build`
  - 生成 client/OpenAPI diff 检查及相关 `infra/acceptance` 检查
- [ ] 运行完整 Trellis quality check：spec compliance、lint/type-check、cross-layer data-flow、media/workflow recovery、security/redaction。

## 8. 风险与回滚点

- [ ] 迁移必须 additive、可在旧库启动；任何 schema 失败阻断 readiness，不删旧媒体数据。
- [ ] Provider role 未配置或用户未填 ComfyUI JSON 时保持 pending/retry；绝不写 heuristic fallback prompt。
- [ ] workflow registry/replay 改动前保存 deterministic replay 测试；若历史无法回放，回滚 Worker build 而非放弃 active history。
- [ ] Timeline projection 只读，canonical revision 通过 CAS；失败时可重放最近稳定 attempt。
- [ ] 在完成前运行 `trellis-check`，并按 `trellis-update-spec` 记录本次新增的可执行契约。

## 9. 完成门槛

- [ ] PRD convergence pass 已完成，`design.md` 与本计划和实际代码一致。
- [ ] 所有验收标准有测试或明确的运行证据。
- [ ] 目标工作树无未解释的失败/调试残留；主工作树原有改动未被覆盖。
- [ ] 用户确认可以结束后，再运行 `task.py archive`/finish-work 流程并提醒提交。
