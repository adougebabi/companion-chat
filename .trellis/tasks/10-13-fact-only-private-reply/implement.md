# FO-C Fact-only 原生私聊闭环 执行计划

父任务最终方案已获用户明确批准，可按顺序 start 本子任务。

依赖：依赖FO-B配置、独立Provider/ReplyContext与FO-A前置修复。

## 顺序

- [ ] 扩scope snapshot/单一catalog与reply服务依赖，合法direct caller可跑、不依赖隐藏Main。
- [ ] 实现pre-transaction生成+draft checkpoint复用+PreparedPayload，先恢复logical已发布结果。
- [ ] 复用DirectTx/ledger/sourceguard，fact_only real text/状态回填，final cognition保持audit-only。
- [ ] Main/Writer源协议更新与cache invalidation，physical policy保留silence/no_op与tool/final separation。
- [ ] 真实PG/测试发送集成覆盖原生工具回填、取消/重试/认知失败、多表单profile与legacy。

## 检查与交付门

按父 implement.md 的命令与本范围定向验证执行，新增DB集成通过真实registry/ADK/事务路径；SKIP与未执行单独记录。完成本子任务后独立功能commit，更新spec及父验收矩阵，父联合验收前不宣告整体可用。任何新增配置/状态保持additive；回滚点遵循父design的stable镜像ref、配置恢复与新协议任务隔离。


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
