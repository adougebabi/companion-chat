# 内部上下文容量与可用性修复

本轮继续在 `codex/fact-only-reply-vision` 修复内部容量硬拦截。合法引用、冻结上下文与 Tool 准备数据超过经验阈值时保留完整数据继续执行，并留下模型前诊断。

## 事故依据与原因

用户提供的日志显示，2026-10-14 03:50:30 UTC（11:50:30 +08），`ProcessCognitionActivity` 在工作流 `go:cognition:inbox_6b441b2405510a34ded150964922afb9:retry:1`、Run `01a13888-78e6-7e99-b1f8-50c395550ac1` 返回 `context_reference_index_too_large`。

原实现把 128 条引用和 128 KiB 索引编码总量作为硬门槛，使用同一个错误。仅凭该日志无法判断实际命中条数还是字节量，也没有实际容量数据可供排查。这些门槛早于本任务已存在（引入记录 `6cbc18b`），前四笔 Fact-only 提交保留了它们；本次修正这个可用性取舍。

同一路径还存在单快照 32 KiB 拒绝、slot/完整冻结快照 128 KiB 静默丢弃、prepared envelope 128 KiB 拒绝。只删除入口报错或把上限改大，仍会在后续步骤丢上下文或失败。

## 当前行为

| 数据/路径 | 原行为 | 修复后 |
| --- | --- | --- |
| 内部引用条数 >128 | 构建/校验失败；重复注册也可能提前失败 | 完整保留；相同引用注册幂等 |
| 单快照 >32 KiB / 完整索引 >128 KiB | 快照或索引校验失败 | 验证真实性后继续，记录容量告警 |
| Active Memory 注册 | 三处按索引剩余条数裁剪 | 保留所有已选条目及引用 |
| Core/canonical capability slot 与完整 snapshot >128 KiB | slot 遗失或 copy 返回 nil | 两套 Snapshot/resolver 均完整传递 |
| 内部 prepared data/envelope >128 KiB | 编码、解码或 invocation validation 拒绝 | 完整通过两套 codec 和 canonical validation |
| Tool frozen snapshot / resulting-state snapshot | 同类容量拒绝 | 保留 JSON 与身份校验，移除容量拒绝 |
| ActionOutcome 内部引用 | >128 KiB 拒绝；24 条模型引用加一条 Core 状态也失败 | 合并集合无经验条数/字节门槛，完整结算 |

128 条、32 KiB 与 128 KiB 现在是告警参考值，超过时不会据此截断或拒绝。未新增容量开关、数据库迁移、依赖、清洗 Agent 或自动重试机制。

保留所有权、Actor/profile/conversation 作用域、版本、hash、实际引用成员、JSON 对象/封闭 envelope 与 Tool 准入检查。模型提供的 `influences` 仍按原合同最多 24 项；Core 合并后的实际状态引用不再重复套用这个模型输出限制。外部模型 `Arguments` 的独立 64 KiB/schema 校验未改变。真实模型上下文配置与物理请求预算也未修改，容量告警不等于 Provider 已接受请求。

## 排查入口

在 Worker JSON 日志中检索 `context.reference.capacity_warning`，或消息：

```text
context reference capacity advisory; continuing with complete context
```

日志在 Owner 诊断写入之前输出，因此即使没有 ModelRun 或诊断数据库不可用，也有可见证据。三个阶段分别是 `context_projection`、`prompt_assembly`、`reflection_context`。

| 字段 | 含义 |
| --- | --- |
| event_type / stage / decision | 容量告警、发生阶段、`continue_complete_context` |
| fluctlight_id / source_fact_id / correlation_id / as_of | 实例、入站来源、关联身份与投影时钟 |
| reference_count / encoded_bytes | 实际引用条数、完整索引的 JSON 编码字节数 |
| snapshot_bytes / largest_snapshot_bytes | 快照字节总和、最大单快照字节数 |
| by_kind | 每种 kind 的 count 与 snapshot_bytes |
| largest_snapshots | 前五个最大条目的 ref、kind、entity_id、bytes；按大小再按 ref 排序 |
| advisory_thresholds / advisory_thresholds_exceeded | 建议阈值及本次超过的维度 |

Owner 诊断使用相同事件类型，payload 包含 `stage`、`decision`、`as_of`、`advisory_thresholds` 和完整 `capacity` 报告。事件与 Worker 告警均只含元数据，不含快照正文、用户文字、凭据或 Prompt。既有 sink 是独立上下文、最多两秒的 best-effort 同步写入：错误不替换业务结果，但不能保证数据库异常时没有额外延迟。

## 回归与验证

先在原实现确定复现四项失败：180 条引用、64×4 KiB 聚合快照、40 KiB 单快照、128 条时重复注册。独立核验又定位到结算残留：合法的 24 条 influences 加 Core 实际结果引用变为 25 条，返回 `action_outcome_context_references_too_large`；该场景同样先 RED 再 GREEN。

`context_reference_capacity_test.go` 的八个顶层回归覆盖上述行为，以及大索引的 Active Memory/完整投影冻结、两套 slot/resolver、两套 prepared codec 与 canonical invocation validation、无诊断 DB 的 worker 告警、不打印正文，以及越权/hash 篡改和模型 >24 influences 仍拒绝。ActionOutcome 通过真实 freeze/causality/build 链路验证 25 条合并引用及超过 128 KiB 的保留结果；这是纯本地回归，不是实际 PostgreSQL 结算证明。

最终本地检查：

- `go test -json ./...` 通过；1772 个 PASS 事件、531 个 SKIP 事件，包含父测试与子测试，不能当独立验收案例数。16 个包通过，14 个包没有测试文件，无 FAIL。
- `go test -race ./internal/core ./internal/workflow`、`go vet ./...`、`go build ./...` 通过。
- `pnpm typecheck`、`pnpm test`、`pnpm build` 通过；Browser Client 21、Core Client 2、Web 105 项全通过。构建保留原有 >500 kB chunk 提示。
- 测试关闭真实 Provider，测试数据库未配置；数据库/真实服务条件化用例仍 SKIP。真实模型、生产 PostgreSQL、实际 Temporal workflow 重试与恢复均未执行。

完整命令、计数和日志摘要见任务的 `verification/capacity-check-summary.json`。前端测试首次因沙箱 `listen EPERM` 未启动，取得本地 IPC 权限后的同一命令通过；不是产品测试失败。

## 生效与后续验证

本修复当前为本地未提交改动。代码需进入实际运行的 Core/Worker 构建才会生效；仅重新运行旧镜像会保留原硬门槛。此次没有 push、部署、修改真实业务数据或操作该工作流，不能据本地测试宣称用户环境已经恢复。

换用修复构建后，可沿同一 `source_fact_id`/correlation 检查容量告警及后续真实 ModelRun、Tool receipt、结算结果。日志中的失败 run 是否需要重试，应依据当前 inbox/发布 receipt 状态决定；本报告不自动重开旧轮次或重发消息。

完整数据继续后，CPU、内存与模型输入成本仍可能增加；新增元数据用于后续定位与优化，不再把经验值当作不可用的理由。
