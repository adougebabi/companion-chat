# Phase 8 执行计划

## 实施前门禁

在 `task.py start` 后、任何生产代码改动前：

1. 重新记录 `git rev-parse HEAD`、`git status --short`、当前分支、模块版本和既有未提交文件的 SHA-256；绝不 `reset`、`checkout` 或覆盖 `phase8-eino-audit.*`。
2. 按 `trellis-before-dev` 读取 backend 相关规范，至少包括 structured turn、provider、diagnostics、persistence、autonomy、workflow、quality 和 debug observability 指引。
3. 运行一次只读基线：核心 focused tests、`go test -mod=readonly ./...`、Web tests/typecheck/build、`go vet`/build/gofmt 和现有静态 guard；把退出码、SKIP 原因和环境变量缺失记录到任务 evidence，而不是把历史审计文件当成通过证据。
4. 固化“实际存在项”扫描结果和独立期望矩阵夹具；矩阵变更必须在后续代码/测试同一快照中更新。

## P8-01 原生协议清理

目标文件/符号：

- `apps/core-go/internal/core/eino_model_runtime.go`：`generateWithADK`、`einoToolCalls`、ADK response conversion；
- `apps/core-go/internal/core/provider.go`：`completeWithToolsSchemaMode`、`normalizeProviderToolCalls*` 调用方；
- `apps/core-go/internal/core/tool_contract.go` 及对应 parser tests；
- 相关 `eino_adk_runtime_test.go`、`provider_tool_call_diagnostics_test.go`、`tool_contract_test.go`。

顺序：

1. 把 ADK completion 和 fixed-task structured completion 的解析模式明确分开，确认 ADK 不从正文/reasoning/sidecar 获取执行 calls。
2. 删除或收窄 ADK 路径中的派生 ID、名称/参数 fallback 和重复 normalize；固定 Task 合法 DTO/非 ADK 原生映射保留并写清入口。
3. 对缺失/冲突 ID、畸形 JSON、未知/未授权工具、native 与 sidecar 冲突、有效 sibling + 坏 sibling 建立 fail-closed 反例。
4. 运行 P8-01 focused tests，确认不影响固定 Task 严格解析和历史冻结记录读取。

回滚点：若 fixed Task 的合法结构化输出受影响，只回退 parser 分流改动，保留已通过的 ADK fail-closed 测试；不恢复正文猜测执行路径。

## P8-02 ADK 事件与结果投影

目标文件/符号：

- `apps/core-go/internal/ai/agent/loop.go`：`RunADKLoop`、`ADKLoopResult`、trace 收集；
- `apps/core-go/internal/core/eino_model_runtime.go`：trace filtering/merge、tool-only termination；
- `apps/core-go/internal/core/adk_conversation_runtime.go`、`conversation_runtime.go`、`wakeup.go`、`turn_takeover.go` 的 completion 消费；
- `eino_adk_runtime_test.go`、`wakeup_test.go`、`wakeup_intents_test.go`、conversation/takeover regression tests。

顺序：

1. 以 Runner/AgentEvent 为唯一 loop authority，审查 `RunADKLoop` 是否有任何第二次 Tool/Model 调用或错误成功化。
2. 将 model/tool/final/deferred/error/cancel/limit 状态分离，禁止最后非空文本回退；同 call ID 冲突必须报错，不取第一条。
3. 固化 `ReturnDirectly` 仅适用于已有终止契约；验证多工具同轮、tool-only + empty/error、WakeUp trace 合并只结算一次。
4. 复跑 Conversation Main、Takeover Reply、WakeUp 的真实 Runner + controllable model 测试。

回滚点：如果正常 WakeUp action-only 或既有 deferred settlement 行为变化，回退终止判定变更，保留事件来源/失败语义测试并重新定位业务边界。

## P8-03 工具适配、Registry 与权限

目标文件/符号：

- `apps/core-go/internal/ai/agent/loop.go`：`ADKCapabilityTool`/`CapabilityToolInfos`；
- `apps/core-go/internal/core/adk_conversation_runtime.go`：`appADKCapabilityInvoker.ExecuteWithID` 与 canonical checks；
- `apps/core-go/internal/core/capability_runtime.go`、`capability_core.go`、`tool_contract.go`、`builtin_capabilities.go` 及各 capability 定义；
- `apps/core-go/internal/capability/registry.go`、`capability.go`：仅在全仓引用核对后收敛重复可执行实现；
- `model_tasks.go`、`cognition_growth.go`、`autonomy.go` 的 fixed-task candidate/freeze 接线。

顺序：

1. 从 App 初始化和所有直接调用方确认 Core Registry 是唯一生产权威；将 `internal/capability` 中真正仍需的值对象/接口与未使用的独立执行实现分开处理，不删除合法共享类型。
2. 提取/复用 canonical surface、InternalOnly、owner/context/candidate validation，使 ADK、Native Cognition、Daily Review 等所有模型 Tool 入口在副作用前使用同一闸门；策略能力保持 policy-only。
3. 用当前实际策略锁定 `visual_identity.initialize` 的可见性/来源规则，并补“可见但不可执行/不可见且不可执行”中适用的反例，不擅自扩大权限。
4. 为每个真实 Tool 通过 Eino `Info`/`InvokableRun` 验证 schema、参数、ContextSlot、call ID、结果序列化、error/cancel；写 Tool 增加 Prepare、拒绝、rollback、idempotency、deferred/async、正式提交测试。
5. 对多工具顺序、同 call ID 参数漂移、Tool result 不重复执行和 Stream close/cancel 做回归。

回滚点：Registry 收敛若触及共享类型，分步提交并保持 Core App 构造不变；不得恢复第二个生产 Runtime，只保留必要的类型依赖。

## P8-04 Trace 与只读导出

目标文件/符号：

- `apps/core-go/internal/core/diagnostics.go`、`lifecycle_diagnostics.go`、`operations.go`；
- `apps/core-go/internal/migrations/runner.go`（仅在现有事件/metrics 无法表达时评估最小迁移）；
- `apps/core-go/internal/httpapi/domain.go`、`httpapi/browser_backend.go`、browser routes；
- provider request/model-run helpers、ADK invoker/loop 和对应 diagnostics tests。

顺序：

1. 先用现有 correlation/provider attempt/model-run/event 表示 run → physical call 链，确认是否无需 schema 变更。
2. 在同一 context 写入脱敏、bounded 的 ADK/tool stage events 和 model-run metrics：请求、授权、进入/未调度、结果/序列化、next-input evidence、终止、settlement/publish。
3. 扩展 `DiagnosticsExportFiltered`/最小开发查询按 `run_id` 或 correlation 汇总，保持 owner auth、filters、redaction、retention 和现有浏览器契约。
4. 用受控模型注入成功、工具失败、模型解析失败、未知工具/调度失败、合法 direct return，验证最后失败阶段可区分。

回滚点：诊断写入异常不得阻塞业务主路径；保留现有 persistence failure 记录和 bounded fallback。若需要迁移，先停在可回退的 migration/reader 双验证点，不改变旧领域表。

## P8-05 全量 Agent/Task/Tool 契约门禁

新增/更新测试和 evidence 工具位置优先放在：

- `apps/core-go/internal/core/*contract*_test.go`、`eino_adk_runtime_test.go`、各固定 Task/Capability 集成测试；
- `infra/acceptance/` 的 Go JSON event collector、矩阵验证器、selector 清单和自测；
- 任务本地 evidence 目录，避免将运行产物混入源码目录。

顺序：

1. 固化独立 `agent/task/tool/surface` 期望矩阵，扫描生产入口生成实际清单并做差集检查。
2. 为每个 ADK Agent 运行基础套件：无工具、单/多回合、同轮多工具、unknown/unauthorized、bad args、model/tool error、cancel、limit、终止型结果。
3. 为每个模型可见 Tool 运行 adapter/permission/context/result 套件；每类写能力至少一个隔离 PostgreSQL/事务集成用例，覆盖 Prepare、拒绝、回滚、幂等、异步受理和提交。
4. 对每个允许 Agent/阶段 × Tool 关系执行一次真实接线；对禁止关系证明模型不可见且伪造调用无副作用；策略能力单独测试。
5. 登记每个固定 Task 的输入、Prompt/Schema、解析和失败；Embedding 单独验证向量维度/契约，不增加 Agent 回合。
6. 接入零匹配、父 PASS 掩盖子 SKIP、pipeline 非零、缺失目标、未覆盖生产入口和秘密泄漏自测；保留原始 JSONL 与完整叶子测试名。

## P8-06 回归、报告和打包

顺序：

1. 在最后一次源码修改后运行完整 Go/Web/静态/相关集成/race/vet/build/gofmt 门禁；按环境将 DB、Temporal、真实 provider 标记 conditional/blocking。
2. 运行普通对话、query continuation、takeover/persistent switch、WakeUp、Reflection、Native Cognition/Daily Review、submission/outbox、cancel/idempotency/stream/API/Worker 回归。
3. 生成 Conversation/WakeUp 成功 trace、工具失败 trace、模型解析失败 trace 和按 run_id 可读导出；脱敏并扫描密钥/私人数据。
4. 生成 `phase8-eino-native-report.md`、矩阵、commands、原始 test events、manifest（HEAD、未提交变更指纹、依赖版本、大小、SHA-256）和 `phase8-eino-native-evidence.zip`。
5. 重新解压 zip，验证报告存在、相对引用可达、hash 一致、矩阵/事件可回读；任何失败保持非通过状态并列明阻塞。

## 建议验证命令（按实际项目环境执行）

```bash
git rev-parse HEAD
git status --short
go -C apps/core-go test -mod=readonly -count=1 ./...
go -C apps/core-go test -mod=readonly -race -count=1 ./...
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
test -z "$(find apps/core-go -name '*.go' -print0 | xargs -0 gofmt -l)"
pnpm test
pnpm typecheck
pnpm build
./infra/acceptance/check-core-openapi.sh
./infra/acceptance/check-compose-bind-sources.sh
./infra/acceptance/go-core-reference-guard.sh
./infra/acceptance/legacy-scope-guard.sh
node --test infra/acceptance/verify-phase5-evidence.test.mjs
```

针对本阶段新增的 selector、数据库集成和受控故障注入命令必须另存完整 stdout/stderr、退出码和原始 JSON events；不能以父测试 PASS 或命令摘要代替叶子证据。

## 最后检查与提交前阻塞

- 最后一次源码改动后，重新运行受影响 package 和全量矩阵；证据快照、报告 hash、zip 内容必须来自同一工作树。
- 任何现有行为变化都要说明是删除已证实重复调用/回填，还是保留产品语义；无理由增加模型回合、改变人格规则、开放 InternalOnly 或吞掉错误都视为阻塞。
- 没有隔离数据库/真实 provider 时，不将 conditional/blocked 写成 PASS；若关键验收因此未完成，报告本地非通过并保留已完成证据。
- 只有通过 `trellis-check`、`trellis-update-spec`、最终质量门禁和用户确认后，才进入提交/finish-work 阶段。
