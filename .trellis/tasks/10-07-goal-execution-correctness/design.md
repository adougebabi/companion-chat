# A：执行资格与 Attempt 正确性 技术设计

## 完整契约
必须结合父任务 ../10-07-fluctlight-goal-closed-loop/design.md 阅读；其完整技术设计、跨层契约和取舍属于本子任务设计的一部分，不在这里建立第二份不同设计。

## 负责内容与入口
统一关联执行许可；认领即持久化 Attempt；首 Tool 前失败与同步/异步完整结算；有界分类退避和未知副作用核对；稳定标准 ID/版本、混合修改拒绝。
当前定位：goal_intention.go / evolution_persistence.go / intention_runtime.go / scheduled_intention_runtime.go / agent_result_adapter.go / action_outcome.go / life_activity_capabilities.go / workflow/workflow.go；详细file:line见父 research/implementation-map.md。主代理实施前完整阅读待改代码，检索结果不能代替代码。

## 本批关键约束
初次schema从0049接续；begin/settle Attempt共用记录，终态重放兼容；旧标准保留ID/版本映射。
Goal/Intention持久化、统一领域命令、权限和现有短事务/outbox沿用父设计；不在HTTP/UI或Projection另建writer，不在Provider工作期间持锁。

## 依赖与部署
无前置子任务。本批schema变更使用新增线性migration；不改已发布历史，不访问未知生产库。回滚/前向修复形状与父设计一致，迁移检查和对应回归随本批完成。
