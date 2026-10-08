# Fluctlight 目标闭环实施报告（2026-10-08，待真实验收）

基线为 master / ee1e468，初始工作区干净；实施分支为 codex/fluctlight-goal-closed-loop。完整需求快照保存在 .trellis/tasks/10-07-fluctlight-goal-closed-loop/source-requirements.md。用户已明确：真实场景由用户验收，技术边界修复与验证由主代理完成。真实场景未反馈前，整体不标为完成。

## 范围状态

| 范围 | 当前结果 |
| --- | --- |
| G00 | 生产入口、原始缺陷及基线差异已核实；14 项原始失败有独立 ee1e468 对照证据，后续全包验证以最新日志为准 |
| G01–G03 | 统一动作门禁、durable Attempt、同步／查询／异步／未知结果分类、有限恢复、标准身份及版本已实现并验证 |
| G04–G07 | Stage／Commitment、真实来源 journal、正式 Evaluation Task／Workflow、逐标准 CAS、关系确认及有条件后续目标已接通；真实语义体验待用户 |
| G08–G10 | Actor 时区 Review、原因与阈值、动态上下文、四个原生 Goal Tools、Owner Core／HTTP／BFF／客户端／GoalPanel 已实现；实际界面体验待用户 |
| G11 | 增量迁移至 0053、只读预览／reviewed digest 应用、分批中断恢复及对账已实现；无生产数据变更 |
| G12 | 领域、真实隔离 PostgreSQL、原生 Agent／脚本 Provider、实际 PG／Redis／Temporal／生产 Worker 分层验证；live/media SKIP 不计通过 |
| G13 | 源码、迁移、契约、操作说明及证据齐备；最后交付整理和用户真实验收未结束，整体保持进行中 |

## 执行与证据权威

到期认领先写 Attempt，再装配快照和调用 Provider。新副作用和 Goal 生命周期事务共用 Life 锁；暂停、取消、硬期限、窗口、人格、权限及预算均在实际 Tool 事务中复核。实际结果保留，晚到成功／失败不复活终止目标。未知副作用按原操作核对，有限次数后停止自动核对，不能盲目购买或发送。

同步 ACTION 的真实 receipt 可以结算行动；重复消息抑制、QUERY/no_op 不冒充业务成功；已接收异步任务保持等待直到权威结果。聚合先等待仍未知的副作用，再结算最终状态。任何策略／权限拒绝都优先于同轮普通临时失败，失败码进入 Attempt 与退避治理，不能被成功 sibling 或调用顺序掩盖。

Goal 完成只经过统一的 ApplyGoalEvaluation／事务提交。成功标准有稳定 ID 与 criteria_version；标准修订和完成混合请求拒绝，旧领取版本不能覆盖新标准。来源重新读取真实消息、Outcome、物品、活动和 Actor 事实；模型自述、未发布草稿、引用、假设、转述和无效物品不能证明业务完成。全部 verdict 都遵守实例／人格／Goal 绑定；撤销来源可以解释回退，不能支持成功。

Evaluation 必须覆盖每个领取的 Goal；计划不能替代评估并消费来源，全 unknown 不能伪记 no_change。Provider 失败／畸形输出保留来源和 retry 状态。六目标批次保留 remainder 和来源水位，领取 revision 限制晚到提交。子对象只能来自冻结快照；窗口内发生的真实证据可在窗口结束后结算承诺。阶段和承诺证据建立持久关联，消费 journal 后不会丢失；它们的完成不自动增加父目标进度。

关系确认必须属于关系 Goal，使用满足关系标准的双方同语境真实消息，并通过冻结关系 ID／revision 的现有 writer。表达、开放信号、沉默、拒绝和双方确认分别判定。只有合法未解决动机可以创建后续 candidate；重复请求和不同投递身份不能重复阶段、关系、后续 Goal、Resolution 或 outbox。

## 原生认知、Review 与 Owner

conversation.reply 已接入 NativeCognition 的正式目录。到期触发和实际认知执行先读取现有 Owner 私聊权威，再构建一致快照；不在此处创建新会话。排队期间私聊作用域发生变化时，依据持久化 Intention／Goal／Attempt 身份绑定当前可见引用，保留版本、状态、人格和父链接验证。Tool 使用原有授权、预算、重复抑制与发布事务；最终认知 DTO 只记录状态，不重复发布正文。

DailyReview 和 Reflection 使用统一的 Goal Review 队列。Actor IANA 当地日期计算 UTC 周期，跨 DST 不假设固定 24 小时。晚到来源在提交事务中增加 Review revision 并重排，旧提交不能覆盖新证据。无机会、未到时机、睡眠、外部等待、拒绝、系统评估失败、错失可行机会和尝试无效分开。阈值仅累计连续 missed／ineffective 周期，要求实际周期证据与具体下一步／策略；中性等待不计惩罚、不制造动作。软期限产生可见复核，不自动取消或完成。

Owner 使用认证身份、expected revision、reason 和 idempotency 治理目标；浏览器不能注入 actor／source／status／progress／actionPlan 或强制完成。当前／结束目标、修订和证据均有有界倒序分页。证据按 source.recorded_at 与 link.id 排序，游标绑定 Owner 实例、Goal 和 evidence 集合，不能混用修订游标。GoalPanel 使用 scope／selection epoch 丢弃旧响应；409 保留草稿、冻结目标与标准版本，只有显式重读才采用新版本。

## Worker 恢复与迁移

真实联合测试使用任务自有 PostgreSQL、Redis 和 Temporal，以及生产 StartWorkers／Dispatcher。正式消息产生真实来源，脚本 Provider 首次 HTTP 503，目标保持 active／评估 retry；停止并重启 Workers 后原 workflow 收敛为一条 Resolution、一条消息、零虚假 Attempt、两次评估。真实 Owner HTTP 暂停／恢复及无 session 拒绝、Redis outbox 与重复消费也在同一联合测试中验证。

race 联合验收暴露了显式 Stop 与 context shutdown 重复停止 SDK Worker 的竞争；生产停止边界现在使用 sync.Once，保留原任务队列、注册、运行方式和部署版本。并发停止及真实重启均有回归。

0050–0053 保留旧状态、描述、标准和历史。goal-reconcile 默认只读、批量有界；apply 必须提供已审阅 digest，CAS 后记录／重放批次。无法重新验证的历史完成只增复核标记；活动／暂停目标只排规划和证据评估，未知操作仅报告，不重发物理操作。多批次中断重启不复活 cancelled／abandoned。部署先备份，在副本验证迁移；新 schema 写入后优先停 Worker 并前向修复。完整操作见 fluctlight-goal-closed-loop-operations.md；本轮没有部署、推送、现实支付或生产迁移。

## 验证证据

验收矩阵为 46 项：41 verified_scripted_or_domain，5 pending_user_acceptance。源码／结构／脚本 Provider 的通过不等于真实模型语义或媒体质量通过。

- 最终全包检查：research/d-final-go-all-with-worker-race.jsonl（含真实联合设施）：1842 PASS events，1712 leaf PASS，0 FAIL，24 live/media SKIP；15 包 PASS，其余 11 包无测试文件。父 PASS 不计入 leaf。
- 真实 Worker restart race：research/d-joint-worker-stop-race-fixed.jsonl，1 PASS／0 FAIL／0 SKIP；停止边界并发：d-worker-stop-concurrency.jsonl，1 PASS。
- 三条 E2E：research/d-final-three-e2e-identities.jsonl，10 PASS events／0 FAIL／0 SKIP。实际 Goal、标准版本、revision、request／evaluation ID 和证据引用保存在 research/d-three-e2e-evidence-identities.json。
- 原生实际短期表达、旧 due 私聊作用域重绑及门禁：d-native-contact-scope-rebinding.jsonl，19 PASS events／0 FAIL／0 SKIP。
- 人格共享／私有作用域、初始化有界规划、中性 Review：d-persona-initialization-neutral-review-fixed.jsonl，6 PASS events／0 FAIL／0 SKIP。
- Core client 2/2、browser client 17/17、Web 68/68；类型检查和 Web build 通过。Core OpenAPI 路由清单检查已补齐 Goal 路由并通过；内部和浏览器客户端均同步生成。

保留所有先前失败日志，包括测试夹具遗漏、作用域接线和 Worker 清理竞争。它们不是通过证据；每项最终以对应修复后的日志为准。24 个 live/media 测试无配置，SKIP 不计通过。数据库来自任务自有一次性集群和随机子库，没有访问用户生产数据；秘密连接信息未写入报告或日志。

原生多轮测试保留 49152 物理输入限制和真实 ToolCall／ToolResult 协议；详细预算与调用记录在 research/d-final-turn-path-cost-report.json 测量证据中，不通过提高上限解决超预算。一次既有陈旧权限测试的实际行为是保留真实晚到消息、拒绝原陈旧效果和旧权限结算；报告没有将该失败账本包装成成功恢复。

## 用户真实验收与最终状态

待用户验收：C04 普通无关聊天、C05 阶段机会识别、C11 国外／异地事实一致性、C12 人格表达风格、D08 实际浏览器治理体验。C12 的私有作用域与共享连续性、D08 的领域／HTTP／客户端／异步 UI 技术边界已有验证。验收步骤在父任务 research/user-live-acceptance.md。真实媒体质量亦由实际环境验证，脚本输出不作为像素质量证据。

B／C／D 和整体仍保持 in_progress。只有最终门禁和对应用户真实验收均符合要求，才归档完成任务；当前不会将未反馈项、SKIP 或缺少配置记为通过。
