# 连续唤醒计时与取消去重

## Goal

按用户消息后10/30分钟及后续30分钟调度唤醒并封住取消竞态

## Confirmed facts

- 当前默认首次 Wake-up 在认知结算后 `interval_seconds + 10m`（约 40m），后续按完成时间再隔 30m；没有相对最后一条用户消息的 10/30 分钟阶段。
- 自动 due已有单 Redis key、PostgreSQL expected-status/due CAS、cycle唯一性。用户新消息会 preempt pending/running，但最终取消检查发生在获取 lifecycle lock之前，存在仍提交 wake fact 的竞态。详见父任务 `research/confirmed-paths.md`。
- 现有 workflow spec与测试明确锁定旧 40m行为，实施时需同步。

## Requirements

- W1：以最后一条**成功接收的用户消息**为静默基点 `t0`，分别在 `t0+10m`、`t0+30m` 触发；此后静默时默认每 30m一轮，下一轮不因前一轮 Provider 耗时或主动消息后移。
- W2：任意新用户消息使旧 idle epoch与尚未提交的 wake-up失效，并从新 `t0`重排；wake-up 自己的 `conversation.reply` 不重置 idle epoch。
- W3：同一 due/cycle无论 Redis、PostgreSQL sweep、Worker retry还是重放，都至多提交一次 fact和可见结果；最终持锁提交前必须重查 supersession。
- W4：保留现有 enabled/暂停/退役、owner 可配置递归间隔、重试、诊断与 Temporal 历史兼容；默认递归间隔为 30m。

## Acceptance criteria

- [ ] 无新聊天时，`t0+10m`、`t0+30m`、`t0+60m`、`t0+90m`各有一个可辨的到期节点；主动消息不改这些绝对节点。
- [ ] 新用户消息在 pending、running、最终检查与锁内提交之间到达，旧节点都不会提交新的 wake fact、reflection intent、outbox或下一轮旧时钟；已在新消息前提交的独立 Tool effect保留审计事实。
- [ ] Redis/PG重复释放和多 Worker只收敛成一个 cycle；慢 Provider使节点迟到时不并发产生两个同主体 Wake-up，迟到原因可诊断且后续相位仍锚定 `t0`。
- [ ] Worker重启、禁用/恢复和旧 Temporal history仍能恢复；默认设置符合 10/30/每30分钟。

## Out of scope

- 不把打开聊天页面当成聊天活动；只有成功接收的用户消息重置计时。
- 不撤销用户消息到达前已独立提交的 Tool effect；本次去重针对自动 due，不改变手动 HTTP命令的幂等契约。
