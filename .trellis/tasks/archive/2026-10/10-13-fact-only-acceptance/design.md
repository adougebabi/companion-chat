# FO-F 诊断与联合验收交付 技术设计

## 整体设计依据

依父任务 ../10-13-fluctlight-fact-only/design.md 执行；共同跨子任务合同不可被局部替换。

## 本子任务技术边界

复用queued Eino物理model_run/correlation/Owner-only诊断页，增加mode/config/prompt/parent/draft/vision-step状态和safe metrics，raw请求按现权限保留且不包含assignment.Secret/Authorization。settings近期状态从model runs安全projection，UI区分内部草稿与实际message。三条10次业务链fixture复用既有Goal/persona，不重跑direct、不引入AI Judge，按实际已发回复推进并记录真实Goal完成依据。PG/Redis/S3/Temporal使用disposable环境；CPU/digest/stable DB/私有备份需实查。远端push构建与真实部署回滚另记层级。

## 兼容与失败

T20诊断查看权限、secret红线、时间倒序与实际消息/内部草稿分开；T01–T20联合检查，包括10–20次稳定聊天及三条真实persona/Goal场景、双镜像CI、registry部署回滚与旧版本消费兼容。不可将SKIP/mock/file存在算真实完成。

## 证据

- ../10-13-fluctlight-fact-only/research/reply-runtime.md
- ../10-13-fluctlight-fact-only/research/settings-context.md
- ../10-13-fluctlight-fact-only/research/vision-media.md
- ../10-13-fluctlight-fact-only/research/release-baseline.md
- ../10-13-fluctlight-fact-only/research/external-docs.md


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
