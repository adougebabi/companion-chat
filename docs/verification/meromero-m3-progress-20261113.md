# M3 实施进度

用户授权将 M2 剩余真实模型和媒体验收暂缓，继续 M3。M2 保持 BLOCKED，既有证据见 `meromero-m2-review-20261113.md`。M3 从提交 `0a53dd91a03eedc48e18b7ba227832edd0e5f3b1` 继续，复用原 T07～T10，未重拆原任务。

| 当前节点 | 原需求 | 状态 | 结果和剩余事项 |
|---|---|---|---|
| M3-01 连续无事 WakeUp 退避 | T07 无事退避、新事件、Goal/日程义务、既有调度 | VERIFIED | 正式 ProcessWakeUp 结算按 20、40、60、60 分钟退避，首次触发保持十分钟。消息、新 native fact、手工触发重置，同 cycle 回放不累加。Redis 丢键和 supervisor 恢复遵循持久 due。实现提交 `6a96cef6` 的干净源码复验 33 个顶层测试通过，退出码 0、无 SKIP。 |
| T07 其他原验收 | 主动/静默、并发和消息节奏 | PARTIAL | 继承已有消息及受控主动/静默证据。真实模型自主正反场景和其余多端流式矩阵仍未验收。 |
| M3-02 权威称呼输入与自主选择约束 | T08 称呼受真实关系和角色选择约束 | VERIFIED | 修复 fact-only 关系投影遗漏正式 addressing，表达参考协议明确结合真实关系与当前人格选择。提交 `99dcd67f` 的干净源码 46 个顶层回归通过，无 SKIP；真实模型语义效果仍待验收。 |
| T08 其他原验收 | 表达学习闭环 | PARTIAL | 继承候选启停闭环证据。任意注入/日志排除、真实模型关系称呼语义和 UI 到运行闭环尚未完成或未验收。 |
| T09、T10 | 记忆治理及领域保护 | PARTIAL | 保留原台账缺口，尚未开始本轮修改。Goal cadence 基线失败仍未修复，不能宣称领域回归整套通过。 |

## M3-01 实现与证据

- 正式入口为 `persistCommittedWakeUp → updateWakeUpNextDueTx`，字段写入原 WakeUp intent payload，沿用同一 `next_attempt_at`。连续无事策略选择 20→40→60 分钟上限，原十分钟保留为基础检查间隔。正常已提交效果、失败、睡眠和待执行 Goal 义务不增加空闲 streak。
- 有效日程未来 start/end、活动 not_before/active_until、持久 pending Goal handoff 的较晚就绪时间限制退避。结算仍执行现有 authority revision 核验。本节点没有证明所有跨日或并发更新组合。
- `t07-red.log` 是新增测试使用错误的注入字段导致的编译失败。修正为已有 `App.Clock` 后，`t07-red-corrected.log` 在干净基线加测试上复现连续 idle 仅十分钟，以及丢 Redis key 提前释放 future due 两个缺口。
- `t07-green-initial.log` 保留两项测试问题。回放需模拟 Activity 以相同 cycle 进入 started，已有 startup 测试需明确设置 overdue 才应立即释放。修正仅本节点测试，不改变业务断言。
- `t07-green-expanded.log` 在新实现上复现 Goal handoff 最终评估 `deferred/evaluation_cadence`。`t07-baseline-goal.log` 在同 HEAD 干净生产源码、同隔离环境及同测试输入复现。关联 AEL `goal-handoff-assessment-cadence-baseline`，本节点未更改 Goal 规则，该失败仍属 M3/T10 待处理事项。
- `t07-green-focused.log` 运行 26 个顶层测试，退出码 0，无 SKIP；`t07-recovery.log` 另验 supervisor 的 ensure/audit 恢复入口，1 个顶层测试通过。命令、overlay 与受检 HEAD 在日志头。环境为 task-owned 隔离 PostgreSQL 和 Redis，模型为受控 Provider，未调用真实模型、媒体、购买或外部发布。
- 四处用户未暂存修改保持原 SHA-256，未加入本节点提交。所有测试使用干净 HEAD 导出加明确本节点源码覆盖，未消费这些草稿。

提交后证据为 `meromero-m3-evidence/t07-postcommit.log`，受检完整提交 `6a96cef6fb0c0f5f2a9aa54189ac2349d1e3bfeb`，无源码覆盖，33 个顶层测试通过，退出码 0，无 SKIP。包含真实消息发布和 direct/fact-only 追发回归。独立只读审阅未发现确定阻断问题。此节点只证明原持久调度的退避及所列边界，Provider 为受控夹具。

## 停止与恢复边界

M3-01 的局部验证不能标记父 T07 或 M3 完成。下一节点按原 T08 继续，M4 未授权。与当前节点无关的非阻断问题只记录关联，不扩展实现。

## M3-02 权威称呼输入与选择约束

`expression_relationship_context_test.go` 在修复前对 cognition/Writer 均报告正式嵌套称呼丢失，见 `t08-address-red.log`。`factOnlyContextFacts` 补充正式 `role.addressing`，保留既有 legacy 字段与当前人格/当前对象过滤。表达参考沿用既有可选资料协议，新增结合真实关系、已有称呼及当前人格自主选择的明确规则，关系未知保持未知。

`t08-address-green.log` 的 10 个顶层测试实际通过，退出码 0，无 SKIP。隔离 PostgreSQL 经正式 Owner 提取、启用、开关与停用入口，以及正式 HandleTurn 的物理请求，核对关系称呼与规则可见、停用不注入候选，核心人格及关系语义不被学习流程改写。当前人格与其他对象隔离另有纯视图回归。模型为受控 Provider，此证据证明运行输入完整性，尚不能证明真实模型对所有关系含义称呼做出正确判断。

M3-02 提交后受检 `99dcd67f836478ebeb3be5e90cf678db9f3aa86d`，无源码覆盖，46 个顶层测试通过、退出码 0、无 SKIP。包含表达撤销、在途停用/ABA、direct 与 fact-only/Writer 及原始历史保留回归。证据 `t08-postcommit.log`。父 T08 保持 PARTIAL。
