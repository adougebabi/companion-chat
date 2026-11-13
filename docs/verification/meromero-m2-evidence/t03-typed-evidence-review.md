# 原 T03 其余已选证据类型

基线 `fc70c31b`。实际受控 Writer 请求确认，通用递归处理丢失场景与情绪来源，
并改写世界设定和日程调整策略的业务 JSON；Outcome、ScheduleItem 和
AffectProfile 的 Core 数据也进入了 Writer。红测见 `t03-typed-red.log`。

修复依据生产 snapshot 的类型构造字段视图，并在 ToolFacts 组装保留该视图。
场景和生活来源、状态、日程及承诺语义保持完整；世界的背景、规则、氛围和
效果业务值按原值提供，具体 Core 身份由已知外壳隔离或映射为人物别名。
Outcome 复用既有真实状态视图；动作计划和 Core 数值情绪策略继续由原
消费者处理。依据为 `life_context.go`、`world_context.go`、
`world_event_context.go`、`action_outcome.go` 和 `affect_profile.go`。

干净源码加本项改动的 44 个顶层原 T03 与直接相关用例通过，退出 0、无 SKIP，
见 `t03-typed-regression.log`。含四处原有草稿的工作区五个顶层场景也通过，
见 `t03-typed-working.log`。真实模型验证沿现有环境阻塞记录保留。

提交 `8e24ad0d` 的干净源码复验十个关键顶层用例通过，退出 0、无 SKIP，见
`t03-typed-postcommit.log`。

本项覆盖已选证据处理。Moment 默认运行背景仍使用原有组装路径，原 T03 和
M2 的完整验收继续保持未完成。
