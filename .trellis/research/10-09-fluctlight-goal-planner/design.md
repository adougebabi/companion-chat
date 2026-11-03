# GoalPlanner 增量设计
基线 HEAD 71f6066febe068e8eae028e97e4fe409747bf149，工作区干净。用户明确授权本轮实际实施，不再次请求阶段批准。
复用 GoalAuthority、persistGoalAuthorityTx、Life advisory lock、GoalEvaluation 与 Intention 执行资格。新增集合策略/版本、有效顺序、依赖、初始来源映射、持久规划请求和 run。所有入口在统一存储边界校验容量，数据库触发器作跨进程兜底；旧超额保留且禁止增量激活。
独立 FormalAgentGoalPlanner 使用 Eino 原生 loop 与专属 query/commit Tool，无外部行动权限。短事务读取快照与CAS提交，模型调用不持锁；人工关闭/事实变更/失效租约拒绝迟交。请求合并按序列水位保留在途新事件，失败退避。
背景沿用 actor_facts，关系沿用 relationships/revisions；配置不转换为互动证据。可见Actor作用域校验。Goal依赖独立于derived来源，AND完成资格接现有启动门禁。
扩展现有GoalPanel与治理UI，BFF显式字段映射，更新生成合同。目标、初始输入、稳定人格各有权威。
迁移 additive，保留所有ID/证据/日程/终态；原始人格只作历史来源，混合文本不关键词删除。已超额实例需Owner治理。旧程序回滚可能绕过新语义，优先前向修复。

## 运行频率增量修复边界
已核实：memo去重在工作流claim后；Provider失败和成功memo skip不同。线上一个评估Goal显示goal_assessment_coverage_missing；实例5/5，Planner同run多次后retry_exhausted；WakeUp507原文明确内存不足，但GoalEvaluation失败HTTP原文尚未读到。不得归因所有失败为507。
最小变更在领域enqueue/claim与正式AcceptSchedule：自动补目标满额不产生模型工作；日程按实例/IANA本地日稳定source触发一次；普通模型愿望仅积累线索，不能成为每轮自动规划；Owner明确请求保留。已经排队的旧Planner必须同样检查容量，晚到事件与私有事件不吞掉；恢复不越过这些门禁。
GoalEvaluation对缺覆盖的有效JSON需在当前task内提供一次明确修正反馈；修正仍失败的固定契约错误终止该请求，保留诊断和失败记录，不伪造成功memo。临时Provider失败仍有界退避。相关事实、新证据、初始首次评估、Owner强制和真实日程/到期变化保持原证据链。按既有语义memo避免自身hint/纯时钟导致新增模型调用。
暂不重构通用Provider队列、人格或执行系统；不操作线上目标、Provider设置或部署。
