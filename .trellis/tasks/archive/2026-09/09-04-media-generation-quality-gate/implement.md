# 生图后 LLM 质量验收与提示词修订：实施计划

## 前置检查

- [ ] 用户审阅并批准 `prd.md`、`design.md`、`implement.md`。
- [ ] 修改前搜索 `media_intents`、`media_assets`、`media_references`、`provider_job_id`、`publishMediaAsset`、`ProcessMediaActivity` 的全部生产者与消费者。
- [ ] 只处理当前 Go Core/Worker；不恢复历史 Node/SQLite 运行时，不新增浏览器到 Provider 的旁路。
- [ ] 保留工作区中其他任务的未提交改动，不将其纳入本任务提交。

## 实施顺序

### 1. 固化 C 阶段契约

- 在 `apps/core-go/internal/core/provider_prompts.go` 增加 C 阶段系统指令，明确硬一致性范围、审美排除、结构化输出和 retry guidance 边界。
- 在 `apps/core-go/internal/core/provider_schemas.go` 增加 `media_quality_acceptance_response` JSON Schema。
- 新增 `MediaQualityAcceptanceV1` 归一化/验证 helper，限制 verdict、schema version、violations 数量/长度、severity 和 guidance。
- 在 `provider_prompts_test.go`、新建质量契约测试中覆盖 schema、输出边界和禁止主观审美判断。

### 2. 扩展持久化质量状态

- 在 `apps/core-go/internal/migrations/runner.go` 的初始 DDL 与兼容 SQL 增加 `provider_prompt`、`quality_retry_count`、`quality_retry_guidance`、`quality_verdict`、候选摘要和检查时间字段。
- 扩展 `mediaIntent` 读取结构和相关 SQL；默认值兼容已有 intent。
- 增加状态转换 helper：保存候选 verdict、准备一次 quality retry、标记 terminal reject；全部使用 intent/provider job 条件更新。
- 为字段默认值、重复 retry、stale worker 和旧 intent 写 PostgreSQL 集成回归。

### 3. 建立候选图片 C 输入边界

- 在 `apps/core-go/internal/core/media.go` 或同层新增图片 MIME/大小/空字节校验和 data URL 构造 helper。
- 使用内存候选调用 `Provider.StructuredWithSchema(..., "media_quality_acceptance_response", ...)`；user message 包含冻结概念摘要、当前 provider prompt 和候选图片。
- 不调用 Go BFF media route，不把 ComfyUI URL/路径或凭据发送给 LLM。
- 修改 `redactDiagnostic`，去除 `image_url` data URL 和过长二进制值；增加 redaction 单元测试。

### 4. 将 C 接入 Media Activity

- 在 `ProcessMediaIntent` 中把 C 插入 `downloadComfy` 之后、`PutObject`/ready asset 之前。
- B 首次/质量 retry 都持久化精确 `provider_prompt`；retry 将 C guidance 作为有界结构化输入传给 B。
- `pass`/`skipped` 只在 verdict durable 后继续现有上传、asset 写入、目标发布和完成。
- `retry` 第一次只保存 guidance、递增 quality count、清空当前 provider job/verdict 并返回 `quality_retry`；同一个 MediaWorkflow 再执行一次 B→Provider→C，不上传候选、不创建新占位项。
- `reject` 或 retry 耗尽只失败媒体目标；正文/动态保留，不提交候选 asset。
- 复用现有 heartbeat、Provider job ID、对象 key、publish 幂等和 ready replay。

### 5. 区分内容终态与基础设施失败

- 在 Core/Workflow 边界增加可识别的 `quality_retry`、质量失败和完成结果。
- 修改 `ProcessMediaActivity`/`MediaWorkflow`：首次 quality retry 继续同一 workflow 且最多一次；质量 reject/耗尽以完成的失败结果结束，不触发 Temporal Activity retry；Provider/DB/S3 error 仍走现有 retry/failure 投影；C skipped 作为成功交付。
- 增加 workflow replay、重复 poll、Worker restart、stale lease 和重复完成测试。

### 6. 诊断与兼容投影

- 写入 `media.quality.acceptance` persona-scoped diagnostic event，保存 verdict、候选摘要、有限违例、重试次数和 skipped 原因。
- 扩展现有 debug summary（如已有媒体诊断投影）显示 C 状态，但不返回图片 data URL、Provider URL、路径、凭据或完整内部 JSON。
- 对旧 job/intent 使用默认 quality state；没有冻结概念的旧 job 仍遵循现有安全失败契约，不在 C 阶段重新创作概念。
- 同步 `.trellis/spec/backend/media-prompt-contract.md`、`fluctlight-media-contract.md`、`debug-observability.md` 中的 C 状态、fail-open 和一次质量重试契约。

## 测试与验证命令

### 单元测试

- `cd apps/core-go && GOCACHE="$PWD/.gocache" go test ./internal/core`
- 覆盖：C schema/normalizer、verdict 状态转换、data URL redaction、C 多模态消息结构、retry guidance 约束、幂等摘要判断。

### Go 全量检查

- `cd apps/core-go && GOCACHE="$PWD/.gocache" go vet ./...`
- `cd apps/core-go && GOCACHE="$PWD/.gocache" go test ./...`
- `git diff --check`

### PostgreSQL/Provider 集成

- 在 disposable PostgreSQL + MinIO/ComfyUI fixture 下验证：
  1. `pass` 只产生一个 ready asset/reference/目标附件；
  2. C 基础设施错误产生 `skipped` 并仍交付；
  3. 首次 `retry` 只产生一次质量继任执行，不产生第二个可见占位；
  4. 第二次 `retry`、`reject` 不写入/关联候选 asset，正文保留；
  5. Provider job ID、Worker crash/restart、stale lease、重复 poll 不会二次提交或绕过 C；
  6. 诊断只保存摘要，绝不保存 data URL。

## 高风险文件与回滚点

| 文件/区域 | 风险 | 回滚点 |
| --- | --- | --- |
| `apps/core-go/internal/core/media.go` | ready/publish 边界和 Provider job replay | 保留下载后 C 前的旧 candidate 流程；确保新状态字段可默认回退。 |
| `apps/core-go/internal/core/provider.go` / `provider_schemas.go` | 多模态消息和严格 schema 兼容性 | 只增加 role schema 调用，不改变现有文本 role payload。 |
| `apps/core-go/internal/migrations/runner.go` | 旧数据库兼容与默认值 | 所有字段 `ADD COLUMN IF NOT EXISTS`，不删除既有列。 |
| `apps/core-go/internal/workflow/workflow.go` | 内容终态被 Temporal 误重试 | 先补终态回归，再切换 Activity 返回分支。 |
| `apps/core-go/internal/core/diagnostics.go` | base64/凭据泄露 | 先补 redaction 测试，再允许 C message 进入 Provider diagnostics。 |
| `apps/core-go/internal/core/detail.go` 与 BFF DTO | 内部验收 JSON 泄露到浏览器 | 默认不扩展公开 DTO；若扩展，仅使用 bounded summary。 |

## 完成门槛

- [ ] 代码只在 C LLM 明确 `retry/reject` 时阻断或重生成；C 基础设施故障 `skipped` 直接交付。
- [ ] 主动取消/父上下文取消不会被误判为 C 基础设施故障，候选不会因 fail-open 继续发送。
- [ ] 图片候选在 C 允许交付前不会进入 ready/reference/用户可见消息。
- [ ] 质量重试最多一次，复用冻结概念和目标，不产生第二个可见占位。
- [ ] 失败、诊断、恢复、幂等和旧 job 行为均有测试证据。
- [ ] `go vet ./...`、`go test ./...`、`git diff --check` 通过。
- [ ] 用户确认最终结果后再归档任务；视频 C 阶段另行规划。
