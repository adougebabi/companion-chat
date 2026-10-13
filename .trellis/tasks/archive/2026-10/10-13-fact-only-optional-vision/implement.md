# FO-E 可选视觉理解全入口治理 执行计划

父任务最终方案已获用户明确批准，可按顺序 start 本子任务。

依赖：依赖FO-B独立vision binding/配置；可与私聊独立验证，联合接FO-D媒体。

## 顺序

- [ ] 实现统一capability resolver与独立vision physical路由，显式test验证实际支持图片。
- [ ] media quality gate/skip与未校验metadata，视频与生成配置完全独立。
- [ ] VisualIdentity文字/视觉stage拆分，optional缺失awaiting_review终止poll、保留旧有效资产。
- [ ] direct Tool/后台/catalog/discover/physical兜底及验收completion条件统一，来源分清vision/manual。
- [ ] 执行全入口调用计数与实际image payload、未配置/伪模型/取消/旧资产与初始化PG/workflow回归。

## 检查与交付门

按父 implement.md 的命令与本范围定向验证执行，新增DB集成通过真实registry/ADK/事务路径；SKIP与未执行单独记录。完成本子任务后独立功能commit，更新spec及父验收矩阵，父联合验收前不宣告整体可用。任何新增配置/状态保持additive；回滚点遵循父design的stable镜像ref、配置恢复与新协议任务隔离。


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
