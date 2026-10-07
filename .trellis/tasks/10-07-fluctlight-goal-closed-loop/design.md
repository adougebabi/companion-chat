# 目标闭环技术设计

## 权威与调用边界
Go Core 保留 Goal/Intention 领域权威与 persistGoalAuthorityTx 单 SQL writer。补充领域命令服务，所有 Goal lifecycle、标准修订与 Evaluation 提交由此收口；Owner、Goal Tool、Reflection、购物适配器只提交命令/真实来源。HTTP/BFF/client/UI 不复制状态机。Projection 派生，不创建第二套生命周期。
复用平台短事务、outbox/workflow intents、Worker/Temporal、现有 Eino/Task/Tool registry。需要实际工具的独立任务用原生 Agent loop；证据语义分类与评估可用结构化 Task。禁止在事务内 Provider I/O；独立 Tool 不依赖 Agent 隐式预处理。

## 执行资格、许可与 Attempt
关联 Goal 的每次新动作，读取当前 Goal/Intention、Actor/profile、窗口/硬期限、expected revision、Attempt、权限/预算/睡眠/日程，在生命周期与 Tool 副作用共用的锁顺序下申请并记录许可。只约束有 Goal 关联的操作；独立 Tool 保留现有授权，不恢复全局 surface-stage gate。新建/资格提升/到期投递/Worker 恢复/活动启动共用规则。
明确“开始”是许可确认与能力开始审计的事务边界；外部系统不是数据库原子事务。异步实际 dispatch 前重验许可；安全可取消的请求取消，未知或不可撤销副作用进入核对，真实结果始终保留。paused 允许记录事实与待结算，cancelled/abandoned 不因迟到成功复活。
Executor 认领 due 时创建或复用非终态 Attempt，记录 logical_operation_id、attempt_id、认领/开始时间、冻结版本、等待对象/截止、failure class、next_attempt_at 与预算。终态 update 已有 Attempt，重复 digest 重放，无不同 payload 覆盖。
所有 Agent 出口统一结算：首 Tool 前 Provider failure、校验失败、取消/超时、no_op、QUERY、同步 ACTION、async accepted/terminal。同步按真实 receipt 终结行动；QUERY/no_op 不伪记业务成功；async 必须有 durable operation/callback/timeout。Agent 失败后不重放整轮；新 Attempt 用新 run，稳定业务操作身份保留，已提交 effects 先读 receipt/核对。
重试配置包括类别、base/max delay、jitter、max attempts；领域状态唯一决定 eligibility，workflow 只执行它。拒绝/权限/硬期限不自动重试。next_attempt_at 参与 trigger，过去的 due_at 不绕过退避。崩溃后的无主 Attempt 根据真实 receipt/operation reconcile，不盲目重做未知副作用。

## 标准、Stage 与 Commitment
Goal 保留原目的/动机/成功定义，新增 stable criterion IDs、criteria_version、completion mode、current_stage、deadline_policy。现有文本 criteria 可保留边界投影，但权威判定绑定 ID/版本；标准修订和完成提交混合请求明确拒绝。修订使旧评估失效并排队重评；终态普通编辑禁止改成功定义，显式重开/修订保留旧结论。
新增 GoalStage 与 GoalCommitment 逻辑聚合，各具版本/标准与生命周期；父 Goal/可选 Stage 关联需同实例、Actor/profile 一致。Stage 存进入/退出/完成依据、策略、依赖和 skip reason；Commitment 存预期结果/窗口/机会/阻碍。默认一个当前推进阶段，不建 DAG，不铺未来剧情。跨日承诺不清空；简单目标可直接 Intention。
完成逻辑仅结构化必要/可选与 all/alternative 的当前需求，不做通用表达式。逐项 satisfied/not_satisfied/unknown 及正反证据为主事实，数字 progress 为有依据的显示投影，绝不按次数或子目标数量累加。

## 统一证据与 durable Evaluation
证据引用绑定 source_type/source_id/source_version、发生/入库时刻、subject/object、实例/profile/会话、Goal/可选 Stage/Commitment、候选状态与理由。真实源来自终态 Outcome、正式已提交 message/turn、库存/生活活动；读取来源确认真实性，不存模型自述当事实。用户转述/引用/假设/未来计划与 assistant 草稿不可证明事件发生，Actor 与会话语境独立校验。
正式消息/Outcome/领域事实提交时，同事务记录可恢复的评估请求或来源 journal+平台 intent；不能仅在进程内启动。候选关联读取作用域内聚焦目标，由语义 Task 输出候选，Review 以入库 journal/未处理记录补偿迟到事件，不能只按 occurred_at 游标。用户消息可先候选，assistant 完成发布后覆盖新的真实来源；不伪造 Outcome/Attempt。
Evaluation 独立状态 pending/processing/succeeded/failed/retry，输出 goal/criteria/strategy version、来源集合/水位、逐条判定、本次 impact、blocker/wait/next step/replan。确定性库存由真实 ID/版本验证；开放语义只作候选，不确定为 unknown/needs_evidence。失败保持 pending/retry 可见，不伪记 no_change。
CAS 提交验证来源仍有效、身份/父级/范围/标准版本与 current Goal state；原子写判定、revision、阶段变化、Resolution/outbox/后续待处理。冲突重排，重复来源不重复进度/阶段/后继目标。Goal 必要标准直接满足即可结束冗余未开始计划；Stage/Commitment 与 Goal 分别评估。
旧购物 completeScheduledGoalTx、Reflection ApplyGoalProgress、通用 Outcome 都进入该服务；旧 API 如保留仅作薄适配，生产不运行两套完成规则。证据撤销活动目标重评，终态生成需复核记录而不改历史。

## 推进、Review、关系与 Resolution
推进仅在激活/结果/阶段或承诺终止/阻碍解除/相关机会/Review 调整时运行，批次有限转换，持久事件承接下一轮。活动目标始终可解释为下一步、已安排、执行中、等待/阻碍及条件、带错误的待规划或暂停/结束。普通聊天不强制规划，不硬编码恋爱关键词。
终结 Resolution 记录原标准版本/真实依据/残留动机；只有合法未解决上层动机生成后续候选，遵守现有自治授权与幂等。关系确认使用现有 Actor 关系权威入口与同事务/可靠事件，表达不等于接受，不写用户现实状态或 Core Persona。
Review 先补偿未评证据，再记录周期/Actor IANA、UTC 窗口、处理水位、原因/影响、机会依据/可行替代、策略、next step/next review 与 revision。幂等 key 绑定 Goal+周期+策略；新增迟到证据可产生可追溯修订但不重复计数。无机会/等待/睡眠/拒绝/系统故障与无效尝试/可行机会未动作分别处理，停滞阈值可配置/查看/重置，不加焦虑或亲密度。
历史期限默认 soft，超期触发复核；hard 禁止新开始，迟到证据按真实发生时刻判窗口。Actor 当地日界线使用 IANA 和下一日日期计算再转 UTC，DST 不用固定24小时。交换仍用带时区 RFC3339；沿用仓库固定毫秒+00:00 formatter，不为新功能另造无时区或不一致时间格式。

## 上下文、Tool 与 Owner
readGoalExecutionStateWith 扩展通用 Attempt/Outcome/Evidence/Evaluation/Review/Stage/Commitment，映射 in_progress，提供有效目的/标准/最近结果/待评/阻碍/下一步/时机/来源 ID。详细历史按需查询。Conversation/WakeUp/Reflection/DailyReview 同权威 compactor，关键当前状态预算保障，完成状态覆盖旧人格愿望；来源 hash 不混入派生 execution。
扩展 registry 的目标查询/命令/重评能力，明确 side effect/permission/idempotency/errors，模型读取实际 receipt 续轮。查询只读；Goal completion 不能由无证据命令强制设置。新增正式 Task/Agent 的调用只在事件/治理触发并合并退避，记录额外调用与完整请求预算变化。
Owner 使用现有 session/CSRF/归属与 Actor/profile 校验，补目标 list/detail/create/edit/lifecycle/reassess/history 路由。变更带 expected revision、reason、idempotency，source/actor 区分 Owner 与模型。分页实际时间 DESC+ID DESC；typed OpenAPI/client DTO 同步。UI 沿现有详情只读、治理写入模式，提供当前/历史目标和证据/标准/尝试/评价/复盘/修订/诊断关联，冲突保存草稿并提示刷新。保留 browser schedule intentionId/actionPlan 注入拒绝。

## 数据迁移与部署
当前 head 0049；首个增量预计0050，以实际 HEAD 确认。不得改 digest-frozen 已发布迁移。沿 embedded Runner 单事务 schema+ledger，新增字段/聚合表/请求记录/索引与组合外键、唯一业务幂等约束；Evaluation/Review 可用既有 append-only 记录承载，表数量不是目标。
存量稳定标准 ID/版本可确定回填，保留描述/状态/证据/关联/历史；不可验证完成只标 review_required，不伪造历史阶段。旧长期目标入有界规划候选队列，SQL 不调模型。永久 due/pending 修复先看 receipt/operation，再决定等待/结算/重排，取消目标不复活。工具支持 dry-run、批量水位、暂停/重入与审计。
迁移前备份并在一次性库验证 upgrade/head rerun/失败回滚。真实部署提供操作说明，不在本轮自动迁移用户生产库。新记录写入后回退旧二进制不安全；优先停 Worker/保持数据/前向修复。完整回滚需备份恢复与业务写入停止，不声称 DROP 新表即可无损回滚。

## 取舍与仍待实施验证
选择领域事件驱动补偿而非每日强制规划；选择明确类型而非递归 Goal；选择语义候选+服务端校验而非关键词；选择单逻辑提交而非单 SQL writer 即视为闭环完成。真实模型语义仍需 live 场景，结构校验不能保证所有理解正确。
隔离 DB/Worker/真实 Provider/媒体能力尚未验证；Docker daemon 当前不可连接。环境失败标 BLOCKED、给重跑方法，可实现部分继续。产品行为不因环境而缩小。
