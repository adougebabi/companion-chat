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

## 2026-10-08 实际后台队列持续占用修复

最新用户规则与最小修改边界见 `research/background-trigger-contract.md`。唤醒为实际最后聊天后10分钟及后续每10分钟，启动无Redis key才补一次；反思为最后聊天后30分钟且仅新未处理证据；新认知入队取消同实例排队/执行中的唤醒与反思并阻止晚到提交；检查/静默回执不驱动Goal来源水位自循环。保留真实结果、独立Goal标准writer、已有Agent/Tool/短事务/outbox/Temporal。用户运行环境只读，本地修复不等于已部署/真实验收完成。

## 2026-10-08 Provider语义边界、稳定前缀和Goal重复评估

新增用户要求见 `research/goal-provider-semantic-cache-contract.md`：去掉模型输入/输出的内部ID与版本，改冻结短引用及Core绑定；同数据评估JSON/TOON，选择实测适合的编码；同Agent稳定协议/人格/定义置前，动态事实后置；成功判断的相同证据持久去重，新证据/约束/标准/复核或显式Owner重评仍触发。完整CAS/作用域/证据/续批门禁保留。真实KV命中和模型语义需实际环境指标，不将脚本估算当验收。
