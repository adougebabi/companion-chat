# Research: T01 action and producer/consumer evidence

- Query: 当前83路由UI动作与Mock生产消费闭环、旧53项/WP/E映射建议。
- Scope: internal
- Date: 2026-10-15

## Findings

完整机器可读证据见 `t01-action-evidence.json`，包含 470 个建议动作/闭环条目、410 处事件/控件/导航源代码观察和旧53项逐项映射。此文件仅是研究输入，唯一权威台账仍是 `docs/v3-page-acceptance.md`。

文件路径与用途、代码符号及行号、相关规格、任务书WP/E映射均存于JSON的source_basis/actions/source_observations。外部资料未使用；无第三方API问题需要查文档。

## Caveats / Not Found

- 本研究是供主线程合并docs/v3-page-acceptance.md的非权威建议；没有修改台账，没有实际浏览器或真实业务执行；所有status均非通过。
- UI-01/UI-02及人格服务由其他代理并发修改，profile相关定位是本次读取快照，需合并后重核。
- 83路由数字来自保留routes.json与当前动态注册机制，动作数量不等于独立最终Feature数量；重复自然入口应合并、独立副作用应保留。
- 后端证据仅可定位route/dispatch，不穷尽Core实现；尤其surfaceRoutes泛化转发不能证明领域处理/任务执行/消费者齐全。
- 当前主聊天send/flows、媒体、日程、约定、动态建议等硬编码c1/w1/u1；快速切角色与其他Actor需E19专项验证。
- 新建角色仍未启动worldId=null，与E01开始生活要求存在缺口。
- 聊天图页仍提到灰色附件按钮但UI-01已移除；WP03/E05附件与最新UI要求冲突须主线程按确认历史裁定。
- 通话入口仍为placeholder，数据备份仍禁用/说明；当前P2标签不自动等于范围排除。
- 当前GeneratedImage始终叠加小遥头像，风景/物品无人物路径需要E15验证。
- 旧53项映射是功能语义候选；DET06旧备注所指GOV02/03与实际表编号不完全相同，引用以具体行标题/操作复核。
- 大型蓝图/总稿/Quasar仅在任务书的83、102、110、182行定位来源与裁定责任，本轮未完整读取外部原稿，不做语义裁决。
- 所有纯取消/撤销/分页/筛选/抽屉也保存在observations；它们可映射同一业务动作的验收分支，无须机械创建互不相干的Feature。
