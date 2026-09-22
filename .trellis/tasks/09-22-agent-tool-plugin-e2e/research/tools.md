# Tool contract 局部核查

> 范围声明：这只是对 `apps/core-go/internal/core/tool_contract.go` 中定义的 4 个 capability，以及其直接执行/交付路径的局部静态核查。没有扩展盘点其他文件中定义的 built-in capability，也没有做运行时或数据库实验。

## 契约内的正式名称

`tool_contract.go` 内明确定义了以下 4 个名称：

| 正式名称 | 定义位置 | 契约类型 / 表面 | 直接执行是否真实提交 |
| --- | --- | --- | --- |
| `conversation.reply` | `tool_contract.go:196-218` | action；conversation / wake-up / autonomy | **否。** `Execute` 只返回 `deferred`（`builtin_capabilities.go:158-160`）。`ExecuteDeferredTx` 只验证 text 与 binding 并返回 completed，本身没有 DB write（`:161-174`）；可见消息由外层 Main turn 事务中的消息/行为 settlement 一起提交。 |
| `moment.publish` | `tool_contract.go:221-242` | action；wake-up / autonomy | **否。** `Execute` 只返回 `deferred`（`builtin_capabilities.go:180-182`）；`ExecuteDeferredTx` 只验证 moment binding/text 并产生 completed result，不自己写库（`:183-196`）。真正 moment 行的持久化由调用者的 settlement transaction 承担。 |
| `media.image.generate` | `tool_contract.go:245-278` | action；conversation / wake-up / autonomy / native-cognition | **直接 `Execute` 否，绑定后的 deferred transaction 会真实创建持久 intent。** `Execute` 只返回 `output_target_pending`（`builtin_capabilities.go:257-268`）；`ExecuteDeferredTx` 在调用者事务内执行 `createMediaIntentTargetTx`（`:281-335`，写入点 `:332`）。这个成功边界是 durable media intent，不是已生成最终图片；外部执行在 commit 后由 outbox/workflow worker 驱动（`capability_runtime.go:717-723`）。 |
| `visual_identity.initialize` | `tool_contract.go:281-300` | internal type；wake-up / native-cognition | **公开 runtime 直达路径不会自提交，会转入 caller-owned transaction。** 实现同时提供 `Execute` 与 `ExecuteTx`（`builtin_capabilities.go:344-380`），但它被声明为 `TransactionalCapability`；普通 `CapabilityRuntime.Execute` 对事务型 capability 直接返回 `caller_transaction_required`（`capability_core.go:1252-1258`）。正常提交路径是 `ExecuteTransactional` 调用 `ExecuteTx`（`:1511-1547`），后者调用 `EnsureVisualIdentityInitializationWithPersonaTx`（`builtin_capabilities.go:363-380`，写入点 `:376`）。 |

## Provider 可见性的局部风险

Provider tool catalog 的硬门是 `!definition.InternalOnly && definition.SupportsSurface(surface)`（`tool_contract.go:147-158`），渲染时直接使用 definition name（`:161-182`）。`visual_identity.initialize` 虽然 `Type` 是 internal，但这个定义在 `tool_contract.go:281-300` **没有显式设置 `InternalOnly: true`**。仅根据本次局部静态路径，不能把“`Type: internal` 自动隐藏”当作已证明事实；如果没有其他层补齐 `InternalOnly`，它在支持的 surface 上会通过这个 catalog 过滤。

## Main 代执行的位置与真实边界

Main 本身是语义决策/工具调用产生者，不是 capability 的直接 DB executor：

1. conversation surface 从 registry 取 catalog：`mutations.go:777-780`。
2. Main 运行并接收 definitions：`mutations.go:793-804`。
3. Main 返回的 formal tool calls 被取出为 `capabilityInvocations`：`mutations.go:814-820`，随后统一 normalize：`:822-835`。
4. 准备好的 invocation 先持久化作为 crash/replay boundary，注释明确禁止在此之前执行：`mutations.go:1028-1032`。
5. 真正的“代执行/落库”发生在 Main turn 的 settlement transaction 内：no-op/tool-only 路径在 `mutations.go:1061-1077`，有 assistant message binding 的路径在 `mutations.go:1435-1455`。两者都调用 `settleDeferredCapabilitiesTx` (`capability_runtime.go:711-724`)。
6. settlement 中，transactional capability 经 savepoint 调 `ExecuteTransactional`（`capability_runtime.go:786-804`）；deferred output 经具体 binding 调 `ExecuteDeferred`（`:805-811`）。底层分别转调实现的 `ExecuteTx` (`capability_core.go:1547`) 与 `ExecuteDeferredTx` (`capability_core.go:1481`)。

因此，对“直接执行是否真实提交”最准确的结论是：`ExecuteCapabilities` 是 planning/query 入口，注释明确表示它不自提交 transactional capability（`capability_runtime.go:122-126`）；三个可见输出工具的直接 `Execute` 都只产生 deferred result，只有外层 Main/action 拥有的 settlement transaction 才会把绑定输出、原生修改或 durable intent 一起提交。Capability runtime 也明确与 MainAgent 解耦（`capability_core.go:1224-1227`）。
