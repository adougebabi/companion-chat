# 实施计划：Eino 基础层整体重构与 ADK 对话接入

## Phase A — 基线与依赖

- [x] 记录两个 Go module 的基线 `go test -mod=readonly ./...`、`go vet`、关键 `-race` 包结果。
- [x] 锁定 Eino、Eino ADK、`eino-ext` OpenAI ChatModel/Embedding 版本；验证 Go 1.25.4 API 能构建。
- [x] 添加最小 Fake Eino ChatModel/Embedder/Tool 测试夹具，支持 tool call、tool result、结构化、多模态、stream 和错误/cancel。

## Phase B — 基础支撑和 Prompt Composer

- [x] 新增 Eino model config/factory 与 per-call support：assignment 映射、队列/Redis lease、timeout/cancel、usage/diagnostics、capability preflight、provider extra fields。
- [x] 将 Eino `schema.Message`/ToolCalls/StreamReader/Embedder 适配到现有 bounded completion/result 合同；删除 raw model POST/response parser 的生产使用。
- [x] 新增显式 Prompt Slot/Fragment/Composer；迁移 Working Persona、WorkingMemory、预算、recent turn grouping、current input dedup、B scope rebuild 和 trace。
- [x] 为每个模型入口建立 Task 函数和最小契约测试，不让 Task 捕获 `*core.App`。

## Phase C — ADK 对话运行时

- [x] 实现 request-scoped ADK `ChatModelAgent`/`Runner` 工厂，配置严格的 MaxIterations、无隐式 retry/failover 和 bounded checkpoint。
- [x] 实现 CapabilityDefinition → Eino ToolInfo/窄执行适配；执行调用现有 CapabilityRuntime 和冻结 identity，不直接写 DB。
- [x] 将 Main `HandleTurn` 的一次模型调用、tool call、工具结果回填、继续生成和 canonical visible output 接入 ADK；保留现有 auth/idempotency/freeze/settlement/commit 后 delivery。
- [x] 将 query continuation、takeover Judge/B、persistent switch、recovery/replay 接入显式 task/runtime policy；确保无递归循环、无 A 候选泄漏、B 无持久切换权限。

## Phase D — 全量任务迁移与旧实现清除

- [x] 迁移 WakeUp、Reflection、Daily Review/native cognition、Summary、Schedule、Media Prompt/Quality、Visual Identity Vision/Patch、Initialization。
- [x] 迁移 durable/query Embedding 与 retrieval fallback；维持 pinned assignment、dimension/vector validation 和独立 queue。
- [x] 迁移/验证现有 StreamTask；保持 settlement 后 Core NDJSON 语义，不将 ADK stream 直接暴露到 BFF。
- [x] 删除旧 Provider HTTP、旧 SSE parser、旧 generic loop、旧 Prompt Composer 调用、伪 tool/畸形修复/旧 fallback/双读逻辑；更新配置和架构文档、迁移清单。

## Phase E — 验证与收尾

- [x] 添加/更新测试：ADK normal turn、tool result next input、query budget、takeover/persistent switch、candidate rejection、failure/cancel/iteration cap、single publish、Prompt Slot budget/isolation、Reflection context、text/structured/multimodal/embedding/stream、legacy format fail-closed。
- [x] 运行 `gofmt`、`go test -mod=readonly ./...`（两个 module）、相关 `go test -race`、`go vet`、旧符号/旧 HTTP/旧循环全仓扫描。
- [ ] 运行 Trellis quality check；更新 backend provider/queue/structured-turn/diagnostics specs，记录 Eino/ADK 边界和真实 provider 未验证项。
- [ ] 提交单一可回滚 commit，归档 Trellis 任务并输出迁移清单、调用路径、删除清单、配置变化、测试证据和外部阻塞。

## 关键验证命令

```bash
cd apps/core-go && gofmt -w internal/core/*.go && go test -mod=readonly ./...
cd apps/core-go && go test -mod=readonly -race ./internal/core
cd apps/core-go && go vet ./...
cd apps/gateway-go && go test -mod=readonly ./... && go vet ./...
rg -n --glob '*.go' 'chat/completions|/embeddings|StreamText\(|for .*Tool|legacy|fallback|reasoning_content' apps/core-go/internal/core
```

## 风险与回滚点

- Eino/ADK API 版本漂移：先用最小 adapter contract 编译锁定；任何不支持能力在 factory/preflight 明确失败。
- ADK 自动 loop 破坏现有单次 cognition/事务：runtime policy 与 Fake model 测试必须在迁移 Main 前通过。
- Provider-specific payload/usage/tool ID 丢失：保留旧诊断/normalization tests，逐字段比较 Eino adapter 结果。
- 多模块大范围迁移产生交叉回归：按 Task/入口分批提交；每批保留可运行测试和 `git diff --check`。
- 真实 provider/ComfyUI/计费无凭据：只报告未验证，不降低 Fake/契约测试的失败严格度。
