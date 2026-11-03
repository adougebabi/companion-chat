# Conversation Tool 契约审计

`apps/core-go/internal/core/phase8_contract_matrix_test.go:33` 固定了 26 个 conversation surface Tools；本次逐项对照模型输入与 Core 注入字段。Actor、Fluctlight、Conversation、OperationID、证据、当前 profile 和多数 CAS revision 已由 Core 绑定，不应新增模型参数。

## 可确认的错位与修复范围

| Tool | 原有差距 | 本次处理 |
| --- | --- | --- |
| `memory_event` | create 返回数据库 `memory_id`，revise 要 opaque `target_ref`；缺目标/失效目标在 Prepare 可中断 ADK | create/revise 返回可复用 `target_ref`；通用 omit-field 元数据从模型结果隐藏 `memory_id`/revision；缺/失效目标是非 retryable Tool 错误 |
| `active_memory_event` | create 不返回可复用 ref；失效目标在 Prepare 可中断 ADK | 活跃结果返回 `target_ref`；目标不存在/快照无效是非 retryable Tool 错误 |
| `intention.decide` / `intention.inspect` | 多个 operation 可能在运行时要求 `intention_id` | 当前 profile 恰有一个未结意图时由 Core 绑定；0/多个候选分别返回 not_found / selection_required；模型可先 `inspect(list)` |
| `life.activity.advance` | 单一当前活动仍必须传 `activity_id` | 当前 profile 恰有一个进行中/延期活动时由 Core 绑定；多个候选要求明确 ID |
| `wardrobe.inspect`、`persona.detail` | `detail` 类 operation 有运行时目标要求，但 flat schema 不表达 | 保持紧凑 schema，在 Tool 描述和非 retryable 错误中说明所需目标；不猜多候选 |
| `life.activity.start` | shopping/haircut/hair_dye 各有语义字段条件 | 保持紧凑 schema，在 Tool 描述中列出条件；已有效果校验不变 |
| `persona.switch` / `persona.takeover` | 决策需声明 profile/rule/trigger | 保留显式目标用于 stale/规则校验，描述说明条件；不让 Core 猜切换目标 |

`relationship.lookup` 的目标 Actor、`wardrobe.wear` 的 item_ids、`wardrobe.outfit.save` 的 item_ids/outfit 选择是业务对象选择，不可从 Fluctlight 身份唯一推断。其他 conversation Tools 的模型输入没有强制内部 revision、owner、Provider request、operation ID。新增/更新 Tool 定义是静态代码注册，不存在运行时 CRUD。

## 通用错误与模型结果

普通 `ErrInvalidArguments` 原经 `tool_execution.go` 映射为 retryable `capability_prepare_failed`，导致 `adk_conversation_runtime.go` 抛错中断整轮。现在区分 schema/缺参（非 retryable、安全字段/类型反馈）和依赖/数据库故障（保留 retryable）。`ToolExecutionReceipt` 仍记录完整 operation、native call、execution call、authority revision；模型只收到 business status/error/output。此投影不改变 ToolCall ID 关联。对于 `memory_event`，后续操作使用 `target_ref`，模型不再收到 raw memory ID/revision。

展开多个 `oneOf` 会使 conversation Tool schema 超过 `DefaultPromptBudgetPolicy.ToolsSchemaTokensCap`；`TestConversationCapabilityCatalogFitsDefaultPromptBudget` 因此是本次验收门槛。当前做法用 Core 唯一目标绑定、紧凑描述与可纠正反馈，避免通过调大预算掩盖膨胀。完整 Core DB 测试的两个 final-contract 测试在未修改代码的 HEAD 基线 worktree 上同样失败；另有直接读测试主库的测试因主库未迁移而失败。针对本子任务的隔离 PostgreSQL 用例均通过。
