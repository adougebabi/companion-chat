# `commit_review.observations` 现场错误与回归

用户提供的现场错误为：`visual_identity_activity_failed · agent_run_failed: adk_run: [NodeRunError] failed to invoke tool[name:visual_identity.commit_review ...]: tool execution capability_prepare_failed: invalid capability arguments: field "observations": value must be an array`。原始 ToolCall JSON 未保存，因此只能断定字段非数组，不能断定它具体是对象还是字符串。

旧 schema `visual_identity_tools.go:121-140` 只允许字符串数组；旧 Vision response schema `provider_schemas.go:662-669` 使用对象，Agent 指令仅写 bounded observations。合成对象输入在真实 `CapabilityInvocation.Validate` 上得到与现场相同错误；在隔离 PostgreSQL 的真实 Tool 边界，正式回归测试先红再绿。另一测试证明一次无效调用加一次修正调用曾被 `validateVisualIdentityAgentToolProgress` 误判为“两次审查”。

最终实现：Tool schema 接受数组、单条字符串、平坦对象；执行边界把它们严格归一成最多 24 条、每条最多 500 rune 的字符串数组；对象键排序，非文本/嵌套/空值失败。通用 Tool Prepare 错误分类使不可解析参数返回非 retryable Tool 结果。进度只统计一次未 replay 的 `completed|accepted` 审查。受控 PostgreSQL + MinIO + 假 Provider 测试把 `observations=42` 的失败结果送入下一次模型请求，再提交有效审查并写入 canonical 资产一次。
