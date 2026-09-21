# 摇光第八阶段：Eino Native 收敛、全量契约测试与故障定位

## 交付结论

本阶段已在当前工作树 `HEAD=50db5325e24252f1f44db5f9f8c4efbb92e4b466` 上完成可执行的 Eino Native 收敛、ADK 事件/结果投影、模型/工具协议负例、surface/InternalOnly 权限门禁、run_id 诊断关联和确定性契约门禁。

本地确定性质量门禁通过。数据库、真实 Provider、Temporal/Redis/S3/Compose 外部链路因当前环境未提供必要端点或凭据，按要求标记为 `BLOCKED`/conditional；没有将这些条件用例写成 PASS，也没有声称已经定位线上真实故障。

详细可回读材料位于 [phase8-eino-native-evidence](phase8-eino-native-evidence/)，压缩包为 [phase8-eino-native-evidence.zip](phase8-eino-native-evidence.zip)。旧的 `phase8-eino-audit.*` 文件保持原样，未被覆盖。

## 基线与范围

- 当前分支：`master`。
- 当前 HEAD：`50db5325e24252f1f44db5f9f8c4efbb92e4b466`。
- 历史审计基线 `a237b611...` 未被重置；当前工作树的既有审计文件和 WakeUp 修复未被覆盖。
- Go module 声明：`1.25.4`；本机实际测试 toolchain：`go1.26.3 darwin/arm64`（见 `commands/dependency-versions.txt`）。
- Eino：`v0.7.37`；ADK 来自主模块。
- OpenAI model extension：`v0.1.13`。
- 未升级依赖、Go 或扩展组件；未 fork/复制框架源码；未迁移 Temporal；未添加默认 retry/failover/模型降级。

## P8-01 原生协议清理

完成内容：

- `providerMessagesToEino` 使用严格的 `einoToolCalls`，要求正式 ID、名称、对象 arguments，拒绝冲突/重复/缺失身份和非对象参数。
- `normalizeEinoNativeToolCallsIndependently` 只从 Eino `schema.Message.ToolCalls` 建立领域 invocation；坏 sibling 产生有界错误但不抹掉合法 sibling。
- 移除 Provider sidecar 的缺失 ID 派生路径：删除 `normalizeProviderToolCallsWithDerivedIDs` 和 `derivedProviderToolCallID` 的执行使用；固定 Task 的 structured JSON 仍按严格 DTO/Schema 解析，但不会制造 ToolCall 执行身份。
- ADK completion 的 `tool_calls` 只来自 Eino typed message；structured/reasoning sidecar 中的 `tool_calls` 被忽略/删除为执行权威，仅可作为受限诊断来源。

保留内容：固定 Task 的严格结构化输出、冻结记录领域解码、非 ADK 合法 DTO 映射；这些不再拥有 ADK 工具执行权。

关键测试：

- `TestEinoNativeToolCallsRequireFormalIdentityAndObjectArguments`
- `TestEinoADKNativeToolCallsDoNotReadStructuredSidecar`
- `TestEinoNativeToolCallNormalizationKeepsValidSiblingAndRejectsMalformedSibling`
- `TestNormalizeProviderToolCallsRejectsMissingSidecarIDs`

## P8-02 ADK 事件与结果投影

完成内容：

- `RunADKLoop` 保留官方 `adk.NewChatModelAgent`、`adk.NewRunner`、`AgentEvent` 消费和 Core trace 投影；删除通过吞掉 `ErrExceedMaxIterations` 把上限错误转成成功的 `ToolOnlyTermination` 分支。
- `queuedToolCallingChatModel` 继续负责已存在的 deferred/rejected round 边界，保持纯查询结果可继续回到模型、写/输出 deferred 不产生无意义第二次物理请求；这不是第二个通用模型循环，也不执行工具或事务。
- 工具事实、工具结果、最终文本、模型/工具错误、取消、空最终消息和迭代上限仍分别识别；没有回退旧 assistant 文本。
- Conversation、WakeUp、Takeover Reply 的正式 Eino Runner/Tool result pairing 和 WakeUp trace 保留测试。

关键测试：

- `TestRunADKLoopExecutesCapabilityAndFeedsResultBack`
- `TestRunADKLoopStopsLoopAtTwoGenerations`
- `TestRunADKLoopDoesNotReuseTextBeforeFinalEmptyAssistant`
- `TestProviderConversationUsesADKLoopAndReturnsCanonicalTrace`
- `TestProviderWakeUpUsesADKLoopAndPreservesTrace`
- `TestProviderTakeoverReplyUsesADKLoopAndPreservesTrace`

## P8-03 工具适配与权限

完成内容：

- `ADKCapabilityTool` 仍是唯一薄 Eino 工具适配器，保留 `Info`、`compose.GetToolCallID(ctx)`、正式 call ID 和 invoker bridge；不在 Agent 外重复调用 ToolsNode。
- Candidate gate 增加 `InternalOnly` fail-closed 检查。
- Native Cognition 和 Daily Review 在 Prepare/副作用前复用 canonical surface、owner/context snapshot、candidate validation；不因共用 Registry 获得 persona policy 权限。
- 新增独立期望矩阵，锁定 15 个实际注册 capability（13 direct + 2 policy）、Conversation/WakeUp/Autonomy/Native Cognition/Reflection catalog 关系、模型可见 Eino ToolInfo 数量和 ADK 三 schema allowlist。
- 对 `visual_identity.initialize` 按当前代码实际的 `InternalOnly=false`/surface 归属进行锁定测试，没有为了填满矩阵扩大权限。
- 生产扫描门禁确认非 `internal/capability` 的生产代码没有调用第二套 `capability.NewCapabilityRegistry`/`NewCapabilityRuntime`；App composition root 使用 Core Registry/Runtime。`internal/capability` 仍提供 Core 所需共享值对象/接口，未被当作第二条生产执行入口。

关键测试：

- `TestPhase8ProductionCapabilityMatrixIsExplicitAndStable`
- `TestPhase8ModelVisibleCapabilitiesHaveRealEinoAdapters`
- `TestPhase8CandidateGateRejectsPolicyOnlyCapabilities`
- `TestPhase8CapabilityExecutionHasOneProductionRegistryAuthority`
- 现有 `TestADKCapabilityDefinitionsFailClosedBySurfaceAndVisibility`

## P8-04 Trace 与故障定位

完成内容：

- 复用 `diagnostic_events`、`diagnostic_model_runs`、Provider attempt/request identity 和 `DiagnosticsExportFiltered`，没有新增观测平台或 Runtime。
- 物理 ADK Generate/Stream 输入记录 `adk.model.input`，包括 bounded message count、formal tool-result IDs 和 assistant/tool pair match count；输出和终止分别记录 `adk.model.output`、`adk.run.termination`。
- Tool invoker 记录 `adk.tool.requested`、`adk.tool.rejected`、`adk.tool.dispatched`、`adk.tool.result`，只保留 call ID、capability、surface、status、error code 和 arguments digest，不保存原始参数/推理。
- 每个 model-run metrics 在缺少独立 workflow run 时继承同一 parent correlation 作为 `run_id`，不制造随机关联。
- `DiagnosticsExportFiltered` 对 owner-authorized `run_id` 过滤事件和 model-run；普通 ModelRuns API 仍不暴露详细 metrics/prompt 字段。

可导出链路：

```text
run_id/correlation
  → physical model call / attempt / model_run
  → native tool_call_id/name/arguments_digest
  → requested / rejected / dispatched / result
  → next input pair count
  → model output / termination
  → existing freeze/settlement/lifecycle correlation
```

受控成功、工具失败、模型解析失败 trace 已放入 evidence；没有真实线上 run，因此它们明确标注为 controlled fixture。

“工具成功后最终模型解析失败”由真实 Eino Runner 受控测试
`TestADKToolSuccessThenStructuredParseFailureKeepsTrace` 产生：工具调用和
completed result 保留在 trace，后续 malformed structured response 被归类为
`adk_structured_response_invalid`，不是 Tool failed。

## P8-05 全量契约门禁

完成内容：

- `phase8_contract_matrix_test.go` 维护独立期望关系，生产 catalog 仅作为被审查对象；实际注册项、surface、InternalOnly、Agent/Task allowlist 漂移会使测试失败。
- `infra/acceptance/phase8-required-tests.json` 登记必跑叶子测试；`run-phase8-contract-gates.sh` 使用原始 Go JSON events、真实退出码和既有 evidence checker，拒绝父 PASS 掩盖子 SKIP、零匹配和 pipeline 吞错。
- CI 的 Go job 显式运行 Phase 8 selector；完整 `go test -race ./...` 仍保留。
- 每个正式 ADK Agent 的基础套件、固定 Task 不进入 ADK、native ToolCall 负例和模型可见 adapter matrix 均有确定性证据。
- 写能力的 PostgreSQL rollback/idempotency/最终发布集成仍受环境条件约束，未伪造通过。

## P8-06 回归、清理与证据

已执行并记录：

| 门禁 | 结果 |
|---|---|
| `go -C apps/core-go test -mod=readonly -count=1 ./...` | PASS |
| `go -C apps/core-go test -mod=readonly -race -count=1 ./...` | PASS |
| `go -C apps/core-go vet ./...` | PASS |
| `go -C apps/core-go build ./...` | PASS |
| Go `gofmt -l` 检查 | PASS |
| `pnpm test` | PASS（browser-client 12、Web 47） |
| `pnpm typecheck && pnpm build` | PASS |
| OpenAPI/Compose/reference/legacy guards | PASS |
| `pnpm generate` + generated diff | PASS |
| `run-phase8-contract-gates.sh` | PASS；DB 条件测试显式 SKIP |

本地环境缺失：

- `GO_CORE_TEST_DATABASE_URL`
- `FLUCTLIGHT_LIVE_PROVIDER_URL`
- `FLUCTLIGHT_LIVE_PROVIDER_MODEL`
- `FLUCTLIGHT_LIVE_PROVIDER_TEST`
- `REDIS_URL`
- `TEMPORAL_ADDRESS`
- `S3_ENDPOINT`

因此数据库-backed HandleTurn/settlement、真实 Provider、Temporal/Redis/S3/Compose live compatibility 在矩阵中为 BLOCKED，不计入 PASS。

本次额外核对确认 Docker `29.4.0` 和 Compose `v5.1.2` 可调用；尝试启动独立端口 PostgreSQL 容器时，`postgres:16-alpine` 镜像拉取在本地 Docker 引擎中持续无进展，未启动容器、未使用项目命名卷，也未清理任何共享卷。该实际阻塞已写入 `commands/environment.txt`。

规范同步：本阶段新增的原生 Eino ToolCall 权威边界已写入
`.trellis/spec/backend/fluctlight-provider-contract.md`；ADK run/tool/input
诊断事件和 `run_id` 导出契约已写入
`.trellis/spec/backend/fluctlight-diagnostics-contract.md`。

## 证据文件

- 计划与设计：[.trellis/tasks/09-21-phase8-eino-native/prd.md](.trellis/tasks/09-21-phase8-eino-native/prd.md)、[design.md](.trellis/tasks/09-21-phase8-eino-native/design.md)、[implement.md](.trellis/tasks/09-21-phase8-eino-native/implement.md)。
- 矩阵：[phase8-eino-native-evidence/acceptance-matrix.csv](phase8-eino-native-evidence/acceptance-matrix.csv)。
- Agent/Task/Tool/阶段对账：[phase8-eino-native-evidence/contract-coverage.csv](phase8-eino-native-evidence/contract-coverage.csv)。
- 原始测试事件：[phase8-eino-native-evidence/commands/p8-required-go-test.jsonl](phase8-eino-native-evidence/commands/p8-required-go-test.jsonl)。
- 执行对账：[phase8-eino-native-evidence/executions.json](phase8-eino-native-evidence/executions.json)。
- 受控 trace：[phase8-eino-native-evidence/evidence/traces](phase8-eino-native-evidence/evidence/traces)。
- 关键连续摘录：[phase8-eino-native-evidence/evidence/code](phase8-eino-native-evidence/evidence/code)。
- 关键实际源码/测试快照：[phase8-eino-native-evidence/evidence/code/source](phase8-eino-native-evidence/evidence/code/source)。
- 可复用 gate：[infra/acceptance/run-phase8-contract-gates.sh](infra/acceptance/run-phase8-contract-gates.sh)。

`manifest.json` 记录本快照的 HEAD、工作树指纹、依赖版本、文件大小和 SHA-256；报告自身的哈希也以 manifest 中记录为准。压缩包重新解压后检查报告、矩阵、执行事件、trace 和 hash 一致性。

当前 evidence manifest 的 SHA-256 以 `phase8-eino-native-evidence/manifest.sha256` 为准。Evidence bundle 的最终 SHA-256 在交付摘要中给出；它不被写回 bundle 内部，避免自引用哈希。
