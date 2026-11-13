# M3 实施进度

用户授权将 M2 剩余真实模型和媒体验收暂缓，继续 M3。M2 保持 BLOCKED，既有证据见 `meromero-m2-review-20261113.md`。M3 从提交 `0a53dd91a03eedc48e18b7ba227832edd0e5f3b1` 继续，复用原 T07～T10，未重拆原任务。

| 当前节点 | 原需求 | 状态 | 结果和剩余事项 |
|---|---|---|---|
| M3-01 连续无事 WakeUp 退避 | T07 无事退避、新事件、Goal/日程义务、既有调度 | VERIFIED | 正式 ProcessWakeUp 结算按 20、40、60、60 分钟退避，首次触发保持十分钟。消息、新 native fact、手工触发重置，同 cycle 回放不累加。Redis 丢键和 supervisor 恢复遵循持久 due。实现提交 `6a96cef6` 的干净源码复验 33 个顶层测试通过，退出码 0、无 SKIP。 |
| T07 其他原验收 | 主动/静默、并发和消息节奏 | PARTIAL | 继承已有消息及受控主动/静默证据。真实模型自主正反场景和其余多端流式矩阵仍未验收。 |
| M3-02 权威称呼输入与自主选择约束 | T08 称呼受真实关系和角色选择约束 | VERIFIED | 修复 fact-only 关系投影遗漏正式 addressing，表达参考协议明确结合真实关系与当前人格选择。提交 `99dcd67f` 的干净源码 46 个顶层回归通过，另有 1 项私有录制回放 SKIP；真实模型语义效果仍待验收。 |
| T08 其他原验收 | 表达学习闭环 | PARTIAL | 继承候选启停闭环证据。任意注入/日志排除、真实模型关系称呼语义和 UI 到运行闭环尚未完成或未验收。 |
| M3-03 可定位 legacy 摘要来源纠错屏障 | T09 旧来源及派生失效 | VERIFIED | 修复首次全 unknown 的摘要来源负向屏障，提交 `ae9023ce` 的干净源码复验 8 个核心顶层测试及 1 个迁移测试通过，退出码 0、无 SKIP。 |
| M3-04 在途纠错发布保护证据 | T09 后续输入/在途请求 | VERIFIED | direct/Writer × revise/forget 四场景及正常发布/追发对照在提交 `b0d47d68` 的干净源码通过，3 个顶层测试、退出码 0、无 SKIP。沿用既有 generation fence，本节点仅增加测试。 |
| T09 其他原验收 | 记忆治理 | PARTIAL | 完全不可解析来源、正确吸收纠错的重建、其余主链在途与锁序矩阵仍未验收。 |
| M3-05 Goal 领域验收恢复 | T10 成功条件、日程/活动、衣柜和依赖保护 | IMPLEMENTED | 修正隔离测试的 DB 时间前置，原四组失败与 handoff 实际进入评估，24 个直接受影响顶层回归通过，等待提交后汇总复验。 |
| T10 其他原验收 | 领域保护 | PARTIAL | 继承既有其他领域证据，真实模型、真实渲染及完整领域矩阵仍未验收。 |

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

M3-02 提交后受检 `99dcd67f836478ebeb3be5e90cf678db9f3aa86d`，无源码覆盖，46 个顶层测试通过、退出码 0；另有 1 项 SKIP。包含表达撤销、在途停用/ABA、direct 与 fact-only/Writer 及原始历史保留回归。证据 `t08-postcommit.log`。父 T08 保持 PARTIAL。

更正 M3-02 提交后扩展回归统计：`TestFactOnlyActualActorWireReplay` 缺少显式私有录制输入而 SKIP，未用于本节点验收。实际为 46 个顶层 PASS、1 个 SKIP、0 个 FAIL。此前提交和 AEL 摘要中的“无 SKIP”统计有误，保留原日志并以此更正。该私有回放与本节点新增关系视图和正式候选启停测试独立。

## M3-03 可定位 legacy 摘要来源的负向纠错屏障

`t09-legacy-red.log` 在原提交加新增用例上真实复现：首次来源全部 `legacy/unknown`，没有任何早期 typed binding，Owner 修订后旧 runtime summary 仍可读。正式修复在既有 `summaryCorrectionGuardSQL` 中，仅将同 owner/conversation 可解析的 `conversation_summary:<id>` 引用用于负向纠错屏障。派生 Memory 在读取中同样检测仍被当前 snapshot 保留的 legacy summary 引用。原 typed liveness、来源状态、fingerprint 和历史正文均未被改写。既有迁移 Apply 每次重装这些函数，本节点沿用该机制，没有新增数据回填。

`t09-legacy-green.log` 初轮 6 个顶层回归通过。扩展派生读取与迁移重跑时出现两项局部测试错误，保留在 `t09-legacy-expanded.log`、`t09-migration.log`。派生 fixture 错误地假定创建 revision 为 1，已改为使用正式创建回执中的真实 revision；迁移测试把已删除函数 OID 常量缓存，已改为参数化 `to_regprocedure` 在每次查询重新解析。没有修改生产逻辑迎合这两项测试错误。

修正后 `t09-legacy-corrected.log` 的 8 个顶层核心测试、`t09-migration-corrected.log` 的 1 个迁移测试均退出码 0，无 SKIP。覆盖旧摘要、派生记忆、原始历史保留、其他 owner/conversation/时间窗、已知 typed/legacy-confirm 路径、daily 模型前后纠错、resident 及迁移升级/重跑数据保全。独立只读审阅未发现确定阻断问题。

本节点保留未知属性，不将解析出 ID 的旧来源提升为真实支持证据。无法解析的 opaque 来源和首次全 unknown 的 `memory:<id>` 链仍未解决，正确吸收纠错后的摘要重建及主链 in-flight/锁序尚未验收。父 T09 不能因此标完成。

M3-03 提交后证据 `t09-postcommit.log` 与 `t09-postcommit-migration.log`，受检 `ae9023ce81eb9e08d28e14539a228d4d18c1778a`，无源码覆盖。8 个核心与 1 个迁移顶层测试通过、退出码 0，无 SKIP。下一节点补核真实纠错发生在 direct/Writer 模型请求在途时的发布保护，不扩大生产实现。

## M3-04 在途记忆纠错的正式发布保护

新增 `memory_correction_inflight_reply_test.go`，真实受理会话并创建可追溯旧记忆，在正式 Provider 请求已经消费旧记忆后、响应返回前，经正式 Owner revise/forget 事务提交更正或撤回。direct 两场景命中 `current_facts_stale`，Writer 两场景保留 `reply_context_stale` handled 回执；四场景均无 assistant 消息、无 completed 回复回执、inbox 未被视为 processed。没有更改生产实现。

首次运行 `t09-inflight.log` 失败且超时。新增测试错误查询不存在的 `tool_executions.status` 列，同时受控主模型在 handled 后无限返回同一工具调用。已改用正式回执 `result.status`，并令受控模型在已观察 stale receipt 后以 HTTP 400 结束续轮。`t09-inflight-corrected.log` 四场景通过，顶层 1 项 PASS、退出码 0、无 SKIP。Writer 的最终 HTTP 400 是局部夹具的停止响应，通过依据是先于它已持久化的真实 stale 拒绝、无正式消息和无成功回执。此证据没有验证真实模型如何在拒绝后自行恢复，也没有证明所有锁序无死锁。

M3-04 提交后 `b0d47d683b12127d7a3cd62702c2c3811f99ee08`，`t09-inflight-postcommit.log` 运行 3 个顶层测试含四种在途纠错，以及正常原子发布/回滚和 Writer 追发，退出码 0，无 SKIP。T09 完全无法解析来源的旧记忆兼容规则已向用户提出选择，答复前保留现有业务策略。正确吸收纠错的摘要重建和完整锁序矩阵仍未完成。继续 M3 内原 T10 已满足依赖的领域失败核对。

## M3-05 Goal 领域验收恢复

干净提交 `723d2cfd` 在 `t10-domain-red.log` 中复现原四组领域失败、handoff 和 cadence 准入失败。根因为隔离夹具仅推进请求合并窗口，却没有推进数据库拥有的最近活动后 30 分钟静默期及 evaluation cycle。`App.Clock` 的业务时间推进不能替代 DB cadence 就绪。

仅修改测试。已有 `advanceGoalEvaluationCycleForTest` 补齐 `last_activity_at`，并实际查询 available_at、evaluation_next_at 与 quiet-until 三项均就绪。领域 fixture 和必要首次评估调用显式使用它，没有改通用 pending request 读取、请求理由或生产 admission。关系场景新增 succeeded 和两次实际模型调用断言，原接受、引用、拒绝和并发 revision fencing 成功标准完整保留。

`t10-domain-green.log` 在四组领域及 handoff 通过后暴露 cadence 测试跨人格分支缺少真实新活动。补入正式记录的真实消息来源，保留一秒前不可 claim 及未来 merge time 断言，`t10-domain-corrected.log` 6 个顶层测试全部通过。

`t10-affected.log` 扩展到 24 个直接使用受影响夹具的测试时发现 6 个首次准入夹具缺口，`t10-affected-baseline.log` 在同环境干净源码复现。补齐这些场景的显式时间前置后，`t10-affected-corrected.log` 保留 memo 用例的后续失败。该用例过去期待重复来源跳过 model 后仍返回 memo 选择结果，而当前生产规则先返回 `awaiting_new_activity`。测试现在分别验证原 duplicate_source_delivery 提前静默，以及另一次正式 `tool_evaluation` 内部机会实际进入 durable memo 选择且仍不重跑模型，原请求理由保留。生产行为未修改。

`t10-affected-final.log` 24 个顶层测试全部通过、退出码 0、无 SKIP。真实隔离 PostgreSQL 下保持 Goal 原 criteria，实际获取与穿着/使用分离、染发不改发长、真实接受和关系 revision、依赖、已完成/撤回/批次/失败重试与 memo 都有实际断言。Provider 为受控夹具，媒体最终 workflow 是文本装配检查，无真实生图或外部购买。本节点恢复原领域保护证据，不能代表 T10 完整矩阵或真实模型通过。
