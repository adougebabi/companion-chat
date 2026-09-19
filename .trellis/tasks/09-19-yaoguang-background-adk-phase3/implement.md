# 第三阶段实施计划：后台主动行为共享 ADK 接入

## A. 前置核验

- [x] 读取第三阶段需求、前两阶段 task/migration 记录和 backend specs。
- [x] 核实第二阶段提交 `7a47b66`、归档 `bcf83ad`、journal `26aaca2`、当前分支/worktree 和第二阶段 ADK 闭环测试。
- [x] 完成 WakeUp、Daily Review、Native Cognition、Reflection、Worker/Activity/Temporal/Redis 真实入口扫描。
- [x] 形成分类：仅 WakeUp 进入共享 ADK；Daily Review/Native Cognition/Reflection/Summary/Schedule/Media/Visual/Embedding 维持单次 Task 或固定异步。

## B. 共享 ADK 最小提取

- [x] 将 Conversation 专用 ADK loop/bridge 提取为 surface-aware structured-task boundary。
- [x] 保持正式 tool-call ID、ToolMessage 配对、max two generations、per-call queue/diagnostics/attempt identity 和 cancellation 语义。
- [x] 将 definition/Registry equality 校验改为传入 surface 参数，拒绝 InternalOnly、未知、重复或跨 surface 工具。
- [x] 保持 Conversation Main/B/Judge/query 所有阶段二回归通过。

## C. WakeUp 迁移

- [x] `ProcessWakeUp` 使用共享 ADK structured-task boundary；保留 wake-up schema、scenario、Composer、Projection 和 existing post-processing。
- [x] WakeUp invoker 使用 `CapabilitySurfaceWakeUp`、wakeID/actionID/correlation；只允许 WakeUp catalog。
- [x] 纯查询结果若被模型请求可进入下一次 ADK 输入；mutation/deferred/external 保持 deferred，不在 callback 内提交。
- [x] 保留 no-op、policy rejection、frozen action、intent/outbox、next clock、Reflection/WakeUp hint、replay/idempotency。
- [x] 失败、取消、超限不会变成 no-op 成功或发布中间消息。

## D. 非迁移任务守卫

- [x] 增加 schema/classification guard，证明 Daily Review、Native Cognition、Reflection 不进入 ADK loop。
- [x] 保持 Daily Review advisory lock/Continue-As-New，Native Cognition cycle guard/frozen replay，Reflection no-tools/tool rejection。
- [x] 不改 Summary、Schedule、Media、Visual Identity、Embedding 和 action worker 的固定异步职责。

## E. 测试与文档

- [x] Provider-level Fake ADK WakeUp：无工具单次终止、正式工具调用、工具结果配对、第二次输入、最终 structured result。
- [x] WakeUp surface catalog/unauthorized/internal capability fail-closed 测试。
- [x] WakeUp tool failure、model failure、cancel、iteration cap、deferred side-effect/no-publish 测试。
- [x] 有 `GO_CORE_TEST_DATABASE_URL` 时从真实 `ProcessWakeUp` 入口验证冻结/intent/发布；无数据库时明确记录 skip。
- [x] 更新第三阶段 migration/acceptance 文档和 provider/queue/diagnostics/autonomy specs（仅新增实际 contract）。
- [x] 运行 Core/Gateway 全量 test、race、vet、gofmt、mod tidy、diff check、旧路径扫描。

## 验证命令

```bash
go -C apps/core-go test -mod=readonly ./...
go -C apps/core-go test -mod=readonly -race ./internal/core
go -C apps/core-go vet ./...
go -C apps/gateway-go test -mod=readonly ./...
go -C apps/gateway-go test -mod=readonly -race ./internal/bff
go -C apps/gateway-go vet ./...
go -C apps/core-go mod tidy -diff
gofmt -l apps/core-go/internal/core apps/gateway-go
git diff --check
rg -n --glob '*.go' --glob '!**/*_test.go' \
  'RunWakeUpTask|RunBackground|BackgroundAgent|for .*Tool|RunADKConversation|RunADKLoop|wake_up_response|daily_review_response|native_cognition_response|reflection_proposal_v2' \
  apps/core-go/internal/core apps/core-go/internal/workflow
```

## 风险与回滚点

- 风险最高的文件：`adk_conversation_runtime.go`、`eino_model_runtime.go`、`provider.go`、`conversation_runtime.go`、`wakeup.go`。
- 若通用提取破坏 Conversation，先修复抽象并重跑阶段二测试；不能恢复旧 Provider/旧循环作为 fallback。
- 若 WakeUp 真实入口无法在无数据库环境验证，保留 provider-level evidence，并记录 PostgreSQL 集成阻塞，不伪造通过。
