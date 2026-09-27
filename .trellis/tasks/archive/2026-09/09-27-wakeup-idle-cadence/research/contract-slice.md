# Wake-up Workflow 规格摘录与本次变更

- `.trellis/spec/backend/fluctlight-workflow-contract.md:441-531`：`wake_up.current` 的 PG `next_attempt_at`、status、cycle是权威时钟；Redis仅为低延迟 hint；due release用 expected status/due/cycle CAS，重复expiry/多Worker应收敛。单次 `WakeUpWorkflow`只执行一个 cycle。主动可见消息必须由 `conversation.reply`真实 Tool提交。
- 同文件 `:475-480,543,576-578` 仍把聊天后的 clock规定为 `interval + 10m`，与用户确认的 user message `t0+10m`、`t0+30m`、随后每30m冲突；实施必须同步该合同和旧40m测试。
- 同文件 `:39-64`：Workflow/Activity/Provider恢复必须稳定ID、有限重试、协作取消和可重放历史；取消后的旧执行不得提交领域结果。不能增加第二调度运行时。
- `.trellis/spec/backend/fluctlight-diagnostics-contract.md:81-90`：到期且无进展的Wake-up需要有稳定cycle correlation的dedup overdue诊断。
