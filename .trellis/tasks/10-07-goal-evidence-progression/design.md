# B：真实证据与阶段推进 技术设计

## 完整契约
必须结合父任务 ../10-07-fluctlight-goal-closed-loop/design.md 阅读；其完整技术设计、跨层契约和取舍属于本子任务设计的一部分，不在这里建立第二份不同设计。

## 负责内容与入口
独立 Stage/Commitment；真实来源引用与普通对话候选；durable Evaluation、CAS逐标准提交；统一购物/Reflection完成；下一步/Resolution/有条件后续目标。
当前定位：goal_execution.go / tool_publication.go / agent_result_adapter.go / reflection_domains_v2.go / life_activity_resolution.go / builtin_capabilities.go / formal_agents.go；详细file:line见父 research/implementation-map.md。主代理实施前完整阅读待改代码，检索结果不能代替代码。

## 本批关键约束
源提交与评估请求同事务，入库水位补迟到；父级同实例/Actor/profile约束；每项聚合标准版本一致，移除旧独立完成编排。
Goal/Intention持久化、统一领域命令、权限和现有短事务/outbox沿用父设计；不在HTTP/UI或Projection另建writer，不在Provider工作期间持锁。

## 依赖与部署
依赖 A 执行许可、Attempt与标准契约。本批schema变更使用新增线性migration；不改已发布历史，不访问未知生产库。回滚/前向修复形状与父设计一致，迁移检查和对应回归随本批完成。
