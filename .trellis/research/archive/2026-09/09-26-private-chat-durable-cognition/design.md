# Design

## Boundary

`App.AcceptTurn` 复用 `handleTurn` 现有短事务接受逻辑。浏览器 `StreamTurn` 在接受后仅轮询持久 inbox 与已提交助手消息并产生可见 NDJSON；只有 Worker 的 `ProcessCognitionInbox` 运行 Agent。旧内部同步 `HandleTurn` 入口保留，避免其他非浏览器调用方意外改变执行方式。

## Durable state and transport

用户消息通过 `source_fact_id` 关联 inbox，助手消息由相同 `turn_id`/`source_fact_id` 关联。读历史时附加用户消息的处理状态、安全错误码与重试身份。观察流断开不触碰 Worker；终态由 inbox 和已提交回复共同决定。已提交回复优先展示，即使后续认知投影降级。失败且无回复才提供重试。

Inbox payload 同时保留 speaker `actor_id` 与 Owner `authorization_actor_id`，Worker 在两者不同的会话走 `HandleActorTurn`。旧 payload 缺少授权者时从 Fluctlight 所有权记录恢复，避免异步化后把另一位 Fluctlight 当作 Owner。

明确取消命令在验证当前会话参与者和 turn 所有权后标记 inbox 终态，设置 Provider 取消标志，并请求对应 workflow 取消。Worker 的最终提交仍需检查 inbox 状态，防止取消后落库。运行中的取消保持 `cancel_requested`，历史 `turnRetryable=false`，旧 workflow 终止后才允许同身份重开；新身份仍按现有抢占规则处理。

## WakeUp

已提交回复的降级返回和没有回复的失败终态都重新安排 followups；监督器在没有仍运行的认知时修复遗留 `superseded` WakeUp。间隔仍由 `wakeUpAfterCognitionDelay` 计算。

## Compatibility

添加可选历史字段和取消 API；更新 checked OpenAPI 与生成客户端。无新表迁移；既有 inbox 状态及关联键直接可用。部署后监督器修复旧 `superseded` 记录。
