# FO-B 独立模型配置与权威回复上下文 技术设计

## 整体设计依据

依父任务 ../10-13-fluctlight-fact-only/design.md 执行；共同跨子任务合同不可被局部替换。

## 本子任务技术边界

新增reply/vision严格配置reader模仿kev_decisions.go:26-62，以system_settings revision固定run。provider.go/diagnostics.go/settings.go/operations.go/browser.routes与SettingsView/KevSettings/control-center同步支持conversation_reply/vision_understanding独立绑定；身份与Redis queue class拆开。新ReplyContextBuilder刷新BuildContextProjectionFor，复用当前编译WorkingPersona与纯Composer/budget；provider_context增加独立surface的required与history/summary规则。Qwen只引用真实typed事实，推断/摘要保持不确定，当前拒绝直接读真实history；不沿用旧ActionRealizationInstruction。UI显示完整性/最新结果，测试视觉仅显式触发；API变化改生成源再generate。

## 兼容与失败

T01/04/05/06/07：旧配置、恶意指导、不同人格、错误摘要/内部ID、Actor/状态/物品的context快照。T17：中途保存不会改变run schema/模型。FO-08：无嵌套队列锁死，required超限调用数0，预算/时延可观察。

## 证据

- ../10-13-fluctlight-fact-only/research/settings-context.md
- ../10-13-fluctlight-fact-only/research/external-docs.md


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
