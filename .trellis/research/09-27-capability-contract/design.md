# 设计

保持 Core Tool ledger、授权、idempotency、CAS 的权威。Provider-visible schema 必须受默认 Tools/schema token cap 约束；多个 operation 的 `oneOf` 展开会超预算，因此保留紧凑 schema，在 Tool 描述中说明真正需要的语义字段，并把 schema/缺参错误作为可纠正 Tool 结果。只有目标唯一且属于本轮授权 scope 时 Core 才自动解析，0 个返回 not_found，多个返回 needs_selection，模型通过已有 list/current-context 选择目标。

写 Tool 的内部 `CapabilityResult`/receipt 保持审计数据；回传给模型的通用投影去掉 operation、native/execution call 与 authority metadata，仅保留业务状态、错误和必要输出。`memory_event`/`active_memory_event` 的新增结果提供可在对应 lifecycle update 中直接使用的 opaque target_ref；Tool 定义的通用 omit-field 元数据让记忆原始 ID/revision 留在内部 receipt，避免通用执行边界按业务名分流。`ErrInvalidArguments` 等 schema/缺参错误在 Prepare 边界转为非 retryable、安全可读的结果，让 ADK 在既有请求期限内纠正；数据库/依赖/授权错误仍终止。native call ID 缺失由 adapter/diagnostic 层处理，不向业务 schema 新增 id。

与 `context-summary-continuity` 的 ref surface 调整联合验证，不把仍被更新 Tool 使用的 `active_memory:ctx_*` 删掉。
