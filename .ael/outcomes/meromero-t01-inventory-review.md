# T01 运行入口盘点：静态模型审查

- 受检提交：`8d4d82bd`；受检清单：`docs/verification/meromero-agent-inventory-20261109.md`。
- 方法：模型审查，结合源码检索、两个独立只读复核和本地一次性覆盖脚本；不是人工审批、真实 Provider 测试或生产流量观测。
- 覆盖：正式 Agent 注册、任务变体和可配置模型路由；Prompt 版本与实际 Writer 修复变体；Provider、embedding、视觉探针和 Kev 旁路；`ExecuteTool`、插件、发布边界和后台工作流。
- 自动复核：提交后核对 31 个不同正式 Agent ID、36 个显式变体和一个表达学习符号变体，全部在清单中；132 个完整 `file:line` 均存在；8 个非测试 `ExecuteTool` caller 均有对应记录。日志 `/private/tmp/fluctlight-t01-inventory-committed.log`，脚本 `/private/tmp/fluctlight-t01-coverage.py`。
- 发现与处理：独立审查发现原复查命令没有覆盖 Kev、独立 Tool 和后台 worker，已在提交前补充；首次自动核对发现一处同文件第二行使用缩写 `:171` 而无法机械匹配，已改为完整文件引用并复核。
- 限制：四个已注册入口未发现非测试生产上游，清单按“未确认”记录；动态 Prompt 不一定有独立内容版本/hash，不能用粗粒度诊断标签替代。未查询业务数据库或部署配置，未运行真实模型、图片或 UI 场景；这些是 T02–T12 的后续工作。
- 结论：在上述静态源码范围内，T01 入口清单通过审查。它不代表后续实现或全链路验收完成。
