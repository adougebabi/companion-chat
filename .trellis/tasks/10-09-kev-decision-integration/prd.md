# 摇光 Kev 主动生效接入

## 目标与用户价值

按 source-decision-analysis.md 完成 Kev 在四类场景、七个决策点的真实接入：减少不必要自动运行、预披露相关能力、判断已声明人格切换条件、控制可选上下文体积。用户已授权创建任务，并于 2026-10-09 指示“开始实施”；来源文档现作为正式需求依据。

用户已在完整规划摘要之后批准实施，任务已切换为 in_progress。代码实现和本地门禁已完成；正式环境验收由用户执行。

## 背景与证据

- 来源原文快照 source-decision-analysis.md 是完整需求和验收清单；本 PRD 的摘要不删减其约束。
- 基线 master / 63be0df；入口证据与当前行为见 research/entry-mapping.md。
- 当前 Go Core 与 Vue Web 已有原生 Eino loop、独立 Tool、动态 settings、加密 secrets、Owner 诊断；没有 Kev 调用/应用闭环。
- 锁定 Eino v0.7.37；隔离试验证明当前模型调用选项允许 wire schema 动态增长，无需整体框架升级。生产流式与人格 gate 尚需专项实现验收。
- 附件所述 82 用例结果仅是探索背景，不作为准确率或本仓库接入证据。完整 benchmark/replay 文件在 Downloads 定向搜索未找到。

## 范围与需求

| ID | 要求 | 来源 |
| --- | --- | --- |
| K1 | 使用用户已有 Kev runtime 的 POST /v1/systemone state/questions/answers；有界 HTTP、取消和协议校验；固定已测模型，不下载/升级 | §1、2、4、11 |
| K2 | 动态全局与七点开关；disabled 真正零 Kev HTTP/额外准备；每点 use_original 延迟调用旧实现；中途关闭重查配置版本，结果留痕但不生效；父取消直接退出 | §2、3 |
| K3 | 默认 choice_argmax 合法 yes/no 直接采用，unclear 回原路径；可选显式 guarded；问题/状态/策略版本化，完整概率不冒充 confidence | §4 |
| K4 | WakeUp、反思、完成检测、规划四点真实 gate；硬资格和强制事项先处理；no 不推进原水位/last_run；独立延后计数和有界恢复；完成提交后再规划 | §5 |
| K5 | Tool 多选、依赖闭合、权限隔离；漏选后可独立 discovery/load，下一模型轮 schema 真增长、真实执行并反馈；补载不被旧 no 再否决 | §6 |
| K6 | 只判已声明人格规则；yes 经原唯一合法提交、刷新人格再生成；no 保持当前人格，后续不能覆盖；版本/冷却/优先级/冲突仍由 Core；同运行最多一次持久切换；A 副作用未提交前决定 | §7 |
| K7 | 可选上下文按真实候选 ID 筛选；required、活动目标目录、承诺、事实及 Tool 协议保留；不删除源数据；关闭恢复原装配；沿用总结/硬预算 | §8 |
| K8 | 每次实际 question 可追溯；请求/题目/决策关联、完整 raw/概率、策略和实际动作分开；可靠留痕先于采用；DB 失败有界持久 spool，两者失败回原路径告警；权限隔离/保留期 | §9 |
| K9 | 现有设置界面可改开关和 endpoint、查看生效状态；诊断中心过滤、详情、稳定倒序、JSON 导出、显式时区；不将采用率称准确率 | §10 |
| K10 | 用户本地交付配置七点及全局启用，choice 直接生效；按实际调用进程验证地址；服务缺失时启动/运行降级；手动测试连接无业务副作用 | §3、11 |
| K11 | 完成 T1–T10、入口映射、操作说明和 kev-integration-report.md；如实区分 mock、回放、真实模型及未执行项；原已知错例不改 expected | §12–14 |

## 验收标准

- AC1（K1–K3）：协议合法/非法、部分题失败、超时、父取消、低概率合法 choice、guarded 均有固定用例；全局/逐项关闭 HTTP=0、旧模型调用/原 payload 等价；中途关闭不采用旧结果。
- AC2（K4）：四点 yes/no/fallback/disabled 断言原执行体是否运行；no 不消耗水位/cycle/执行重试；达到默认三次延后后下一合格机会走原路径；容量/权限/睡眠/并发/revision 不被绕过。
- AC3（K5）：真实原生 loop 中遗漏→发现→下轮 wire schema 增加→真实 Tool 结果→继续决策；多能力、依赖、Actor 隔离、无授权拒绝、独立 Tool 调用均通过。
- AC4（K6）：整批副作用放行前 gate；被拒 A 无消息/购买/日程写；B 再生成无孤立 ToolCall/假成功；no 无偷偷切换；失败/版本 stale/取消/流式/批次均测试；原合法提交路径唯一。
- AC5（K7）：required/活动目标目录/承诺/Tool 配对和预算保留；只影响当次可选集合；关闭后由权威来源恢复；无长期数据删除。
- AC6（K8–K9）：逐题 raw/输入/概率/策略/实际动作可查可导出；Owner 权限及 Actor scope；started_at DESC,id DESC 稳定分页；DB/spool 失败不采用未留痕结果；保留期显式且删除有界。
- AC7（K10）：提供可实际应用的本地全启用配置和操作入口；不只修改示例；明确网络命名空间/模型 revision 可得性。关闭 Kev 时无需其服务或模型文件。
- AC8（K11）：T1–T10 覆盖矩阵、各命令退出码/数量/skip、真实 Kev 四场景 E2E 证据或诚实 NOT_RUN 原因；后端/前端回归；报告分工程接入、真实模型行为、业务待观察。

## 不在范围内

新的总控 Agent、替换/复制 Eino loop、系统级调度/事务重建、GoalPlanner 重写、人格设定重写、主模型替换、第二推理服务、云 fallback、微调/下载/升级权重、未经授权的远程生产配置/部署，以及真实用户外部副作用测试。

## 已确定的约束

同一候选/请求的原路径只有一个实现；fallback 不预跑旧语义模型。所有权限、所有权、合法状态转换、目标容量和 CAS 保留。Kev 只作选择，不作摘要。模型结论不等于领域成功。真实 LLM 验收串行、使用隔离 Actor/存储和可验证外部适配。

## 风险与待核验项

生产流式 persona gate、多实例访问同一 Kev 的总并发、实际 Core/Worker endpoint 可达性、已测模型 revision 和完整 benchmark 文件均需实施期核验。不允许把缺失证据标成完成；技术验证失败需更新设计，不静默缩小范围。范围决策已明确，无产品偏好问题待回答。

## 2026-10-09 验收范围调整

用户明确：“本地代码过了就行，我去正式环境验收。” 本轮交付门禁调整为本地单元/受控集成、类型、构建和代码质量检查；不继续启动 Docker/安装数据库/连接正式环境来获取真实服务验收。真实 Kev、正式数据库迁移、网络可达性和业务使用效果由用户在正式环境验收。仍实现既定功能、隔离测试和可运行验收指引，报告如实标注未执行事项。此调整不授权缩减七点实现范围。

## 正式验收反馈：设置入口、模型交错和评估噪声

2026-10-09 用户报告 Kev 设置无法进入；WakeUp/cognition 与 Goal Evaluation 的多轮调用交错导致 current_facts_stale；Goal Evaluation 近乎持续触发。本轮在现有任务中直接修复：导航声明成为 URL/computed 唯一来源；同一 Fluctlight 逻辑运行从快照前到提交后互斥；证据 candidate 只有新增 committed link 才入队；续排限定关联目标来源；保留人工/真实结果/review 和原 CAS。不放宽过期事实校验、不把物理模型队列占有到整个 Agent、不引入固定轮数限制。本地验收继续执行；生产数据库跨进程检查由用户自行运行。


## Goal Evaluation 完成未落库反馈修复（2026-10-09）

用户提供的 response 中 goal:2/3/4 的 review 均引用 stage:1.1；goal:2 在描述无现有阶段时还使用 adjust/object_ref。引用按目标编号绑定，旧全局 enum 会放行跨目标选择，hydration 随后拒绝整批，导致 goal:1 的 completed 也未提交。用户看到物理 response，没有领域提交错误展示；未查询正式数据库，不将推断错误码当作生产实测。

本轮 schema 改为按 Goal 判别并限制对象/标准所有权及 create/adjust 操作；wire/coverage 可纠正一次完整输出，替换仍校验；wire 与耗尽 final-contract 错误 terminal。诊断独立展示目标评估提交状态/error/result，成功结果包含实际 goal_outcomes。原批次原子性、CAS、证据与 paused settlement 规则保留；无新 migration，head 仍 0057_logical_agent_leases。

本地最终验证：Go race 1374 PASS、490 SKIP、0 FAIL（含子用例）；Go vet/build、pnpm generate/typecheck/test/build、git diff --check 通过；Web/client 共 98 PASS。独立只读核验无 findings。SKIP/真实数据库/真实模型/正式部署仍由用户验收。当前 codex/goal-evaluation-output-recovery 未提交。


## 部署环境继续失败：覆盖数量与 thinking（2026-10-09）

用户授权浏览器检查环境。实际诊断中 goal_review_event_120 和 goal_review_event_122 提交 failed/goal_assessment_coverage_missing；四目标输入只返回 goal:1，纠正后仍缺其余目标。确认已上线提交诊断，并非仅显示未刷新。Reasoning sidecar较长且中断，但没有 finish_reason/usage，不宣称已证实 token 耗尽。

补修：evaluations minItems/maxItems 等于 offered count，plans 最大数量同 count；Goal Evaluation 关闭 thinking，实际 direct/ADK HTTP 明确发送 enable_thinking=false，避免省略字段导致服务器默认启用。保留总 output reserve、其他 Agent策略、所有权/CAS/批次事务；无 migration。Schema 数量回归 RED→GREEN，HTTP/ADK 修复回归通过。生产入口 thinking 断言补入数据库集成测试，未配置隔离数据库时仍 SKIP。

本地 Go race 1378 PASS/490 SKIP/0 FAIL（含子用例），vet/build/typecheck 通过；正式状态仅只读检查，未部署、未写业务数据。最新补修未提交。
