# D：迁移与联合验收 技术设计

## 完整契约
必须结合父任务 ../10-07-fluctlight-goal-closed-loop/design.md 阅读；其完整技术设计、跨层契约和取舍属于本子任务设计的一部分，不在这里建立第二份不同设计。

## 负责内容与入口
完成存量修复对账与旧入口清理；upgrade/重启/Worker故障；46矩阵与三E2E分层；真实Provider/媒体证据；操作说明和最终报告。
当前定位：migrations/runner.go / isolatedCoreTestRepository / infra/acceptance / core与workflow与httpapi测试 / web测试 / docs报告；详细file:line见父 research/implementation-map.md。主代理实施前完整阅读待改代码，检索结果不能代替代码。

## 本批关键约束
保留存量原文/标准/状态/历史；可中断回填无模型SQL/伪证据；生产仅说明不自动写库；新数据后优先前向修复。
Goal/Intention持久化、统一领域命令、权限和现有短事务/outbox沿用父设计；不在HTTP/UI或Projection另建writer，不在Provider工作期间持锁。

## 依赖与部署
依赖 A/B/C生产接线完成，迁移/测试从首批就开始。本批schema变更使用新增线性migration；不改已发布历史，不访问未知生产库。回滚/前向修复形状与父设计一致，迁移检查和对应回归随本批完成。
