# 第二阶段实施计划：ADK 对话运行接入

## A. 前置门禁

- [x] 核对第一阶段提交 `89764bc`、`d82daf8` 和当前 HEAD `de2d73e`。
- [x] Core/Gateway 全量 test、race、vet 通过；生产源码无旧 Chat/Embedding HTTP 入口。
- [x] 记录第二阶段修改前基线，当前 worktree 无无关改动。

## B. ConversationRuntime 与入口迁移

- [x] 新增窄 `ConversationRuntime`，把 Main ADK 运行、query continuation、takeover Judge/reply 的应用接线从 `HandleTurn`/Provider facade 中移出。
- [x] 迁移普通对话、冻结恢复和接管回复入口到 runtime；鉴权、幂等、projection、freeze/settlement/publish 仍由 Core 负责。
- [x] 删除应用层旧 Main/query/takeover task facade；query continuation 与 Judge 保持一次、无工具的专用边界。

## C. Capability 与人格动作

- [x] 统一 CapabilityRegistry → ADK tool adapter → CapabilityRuntime；适配器构造期要求正式 tool-call ID，保留 source/ID/result/permission/prepare/deferred/transaction 记录。
- [x] 将当轮 takeover 与持久 profile switch 接到明确的 internal-only policy capability 入口；不改变 Judge、授权和生效规则。
- [x] 保留 A 废弃候选、B 权限、persistent switch gate、query continuation 互斥和并发运行隔离；policy action 不进入任何模型 catalog。

## D. Prompt/message/failure correctness

- [x] 每次 ADK call 使用第一阶段 Composer 重新构建 stage/persona/tools/history；严格保持 system/current/tool pair 与预算。
- [x] 加入模型正式 tool-call ID 保真、消息配对、unknown/malformed tool、tool failure、iteration cap、no-final fail-closed；取消沿用 Eino/queue context。
- [x] ADK 内每次 Generate/Stream 创建独立 queue/diagnostic/provider-attempt identity，并保留父 correlation。

## E. 全范围验证与清理

- [x] 增加真实 Provider/业务入口 ADK 闭环测试（Fake model/tool），覆盖 single publish、query、takeover、persistent switch、failure/iteration cap。
- [x] 清理旧控制流、旧解析/临时接线/无效 facade 和未引用代码；更新阶段二 migration 文档及 backend specs。
- [x] 运行最后一轮 `gofmt`、Core/Gateway test、相关 race、vet、`go mod tidy -diff`、旧路径/入口扫描。
- [x] 阶段二 work commit 已准备；归档与 session journal 在提交后由 Trellis 收尾完成。

## 验证命令

```bash
go -C apps/core-go test -mod=readonly ./...
go -C apps/core-go test -mod=readonly -race ./internal/core
go -C apps/core-go vet ./...
go -C apps/gateway-go test -mod=readonly ./...
go -C apps/gateway-go test -mod=readonly -race ./internal/bff
go -C apps/gateway-go vet ./...
go -C apps/core-go mod tidy -diff
rg -n --glob '*.go' --glob '!**/*_test.go' 'chat/completions|/embeddings|for .*Tool|HandleTurn\(' apps/core-go/internal/core
```

## 当前实现证据

- Main 入口：`mutations.go` 仅准备 projection/鉴权/冻结上下文，然后调用 `ConversationRuntime.RunMain`。
- Query continuation：`ConversationRuntime.RunQueryContinuation` 直接进入无工具的 `StructuredQueryContinuation`，没有 ADK capability context。
- Takeover Judge：`turn_takeover.go` 通过 `RunTakeoverJudge` 执行一次专用、不可递归的 Judge task。
- Takeover B：`RunTakeoverReply` 使用新的 reply-owner projection、新的 request-scoped bridge 和 `takeover_reply_response` ADK schema；B overwrite 只写入 B 的 invocation/result trace。
- ADK identity：`compose.GetToolCallID(ctx)` 是唯一 tool-call ID 来源；Eino ToolMessage 同时保留 `tool_call_id` 与 tool name。
- Policy actions：`persona.takeover` / `persona.switch` 注册在 CapabilityRegistry，但标记 `InternalOnly`，不能从 Conversation/WakeUp/Autonomy/Reflection model catalog 取得；确定性 policy invocation 的 `Metadata.Source` 固定为 `policy`。
- 旧控制流：`RunMainConversationTask`、`RunTakeoverReplyTask`、`RunTakeoverJudgeTask` 已删除；生产扫描只保留业务 `HandleTurn` 入口和明确的 query boundary。
