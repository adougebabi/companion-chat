# C：Review、上下文与 Owner 技术设计

## 完整契约
必须结合父任务 ../10-07-fluctlight-goal-closed-loop/design.md 阅读；其完整技术设计、跨层契约和取舍属于本子任务设计的一部分，不在这里建立第二份不同设计。

## 负责内容与入口
周期Review与停滞原因、软硬期限；通用execution/关键Runtime Facts；Goal查询/命令/重评Tool；Owner正式API、typed client、目标详情/历史/治理UI。
当前定位：goal_execution.go / autonomy.go / provider_context.go / reflection_runtime_v2.go / agent_result_adapter.go / httpapi/server.go / httpapi/browser/routes.go / OpenAPI与browser-client / InstanceDetailsDialog.vue / GovernanceView.vue / control-center.ts；详细file:line见父 research/implementation-map.md。主代理实施前完整阅读待改代码，检索结果不能代替代码。

## 本批关键约束
Review唯一周期/修订语义不重复计数；Owner actor/source审计；详情只读治理写入；请求增加记录完整物理预算，不注入全历史。
Goal/Intention持久化、统一领域命令、权限和现有短事务/outbox沿用父设计；不在HTTP/UI或Projection另建writer，不在Provider工作期间持锁。

## 依赖与部署
依赖 B 统一证据/评价/推进。本批schema变更使用新增线性migration；不改已发布历史，不访问未知生产库。回滚/前向修复形状与父设计一致，迁移检查和对应回归随本批完成。
