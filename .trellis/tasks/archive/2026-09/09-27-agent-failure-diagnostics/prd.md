# Agent失败诊断闭环

## Goal

让诊断中心关联物理模型、工具与逻辑Agent失败及安全原因

## Confirmed facts

- `agent_runs.error_detail` 保存逻辑运行失败的底层错误，但现有 Model Runs 只读物理 `diagnostic_model_runs`；模型成功返回 ToolCall、Tool 后续失败时卡片仍显示“已完成”。见父任务 `research/confirmed-paths.md`。
- 既有失败运行重放时 `agent_run_failed: <原始错误>` 可直接从 `agent_runs` 返回，而不会新增物理模型请求。
- `adk.run.termination` / `agent.run.termination` 不进入仅查询 `lifecycle.%` 的生命周期页；系统事件默认全局最近 20 条，容易看不到相应失败。

## Requirements

- D1：诊断中心展示逻辑 Agent run 的明确终态、失败阶段、稳定错误码和经脱敏/限长的原因，并保留各物理模型行本来的状态。
- D2：同一 correlation/run 下连接各次物理请求、Tool 摘要和 Agent 终态；Tool 关联不能只靠测试插入的手工 `model_call_id`。
- D3：旧记录缺关联信息时诚实标“关联未知”，不把物理 completed 误解释为 Agent 成功。
- D4：取消、超时、失败的持久化及 Owner 查询仍受原诊断权限、保留、脱敏边界保护；诊断写失败不改变业务结果。

## Acceptance criteria

- [ ] 回放“模型 completed → Tool invalid_arguments/依赖错误 → Agent failed”时，页面同时显示三层状态与具体失败步骤；`agent_run_failed` 有可用的安全原因。
- [ ] 至少两次物理模型调用及 Tool result能够挂在正确的逻辑 run 和轮次；旧记录不伪造轮次或最终回复。
- [ ] 取消/超时路径最终诊断可见；Owner 以外不可读取，原始参数、密钥、隐藏推理不泄露。
- [ ] 浏览器接口、生成客户端与页面刷新/过滤均通过对应测试。

## Out of scope

- 不把中间模型回复加入公开聊天流；不将物理 Model Run 的 completed 强行改成 failed。
- 不重新修复当前已解决的 Visual Identity `observations` 参数形态错误。
