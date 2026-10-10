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


## 2026-10-10 Goal 完成判定与 Kev 请求负载修复

用户授权两项一起处理。Goal wire 增加 criterion_quote/optional_improvement，负面缺口须来自冻结原标准；父 Goal/Stage/Commitment 纯 preflight 与事务复用语义校验，完成标志双向一致，判断错误使用既有一次完整替换纠正。policy 升为 goal.evaluation.v3，旧 memo 在下次评估不再复用。实际作者陈述与 quotation 区分；active 与 paused 规则保留。字面引用校验不能证明任意模型语义正确，不把 mock 说成真实模型验收。

Kev context/tool 每个 batch 只带当前候选内容，保留当前任务输入和协议；新增明确批次 state builder，原 Decide 兼容。共享 stage、Service budget、单请求和父取消有不同原因，覆盖 HTTP headers/response body 等待。没有改 timeout 默认值、正式配置、全局 token 预算或推理服务；逐 Goal completion gate 仍为原逐项请求。无法承诺所有请求<1秒，剩余大 currentInput/服务排队须实际测量。

最终产品代码无公共API/迁移变更，head仍0057_logical_agent_leases。本地 Go race 1396 PASS/490 SKIP/0 FAIL（含子用例），Web/client98 PASS，generate/typecheck/vet/build/diff通过。独立核验指出主线程新增测试身份缺失，已补完整身份并实际断言active completed/progress1及paused保持。未操作正式业务数据、未部署、未提交。


## 2026-10-10 Conversation 输出与结算反馈

正式只读检查：09:48模型返回reply侧车但无native ToolCall/visible_text；09:42模型有自然回复而后结算current_facts_stale；08:52服务507明确内存不足。未发现Kev移除conversation.reply或required current_state；输出最终schema缺口早于Kev存在。修复：无已提交消息/已接受媒体时加强本地final输出校验，复用一次tool-free repair；已提交输出不要求重复。post-run诊断在通用分类回退时保留publication/settlement外层码及内层cause。事实冲突补期望/实际generation和settlement/tool_receipt边界，不关闭CAS。

主线程曾怀疑acting_profile_id UPDATE引起事实推进；核对trigger UPDATE OF text,attachment_refs后否定，已撤掉相关publication/profile改动。不可将它记录为已确认根因。09:42摘要物理模型在冲突之后启动，也不能认定为该次根因。实际并发写入/部署trigger仍未实测；新版本诊断为后续定位提供值，不能宣称所有current_facts_stale已修复。

本轮无新migration/API/正式设置改动，无部署或生产数据写入。最终测试结果另附；数据库验证按用户本地验收约定明确SKIP。当前代码未提交。


## 2026-10-10 WakeUp 全量重写与事实版本缓存修复

用户要求修复WakePrompt并整体优化固定部分，不追加堆叠；随后追加facts_gen23339→23378。实现显式Surface专用固定协议，统一任务/权威/决策/Tool/结果；动态人格独立。修Runtime段落LF（所有surface）；Wake历史消息移入historical_conversation数据而非物理chatrole，Goal去旧评估reason/内部错误，摘要非权威，原来源不变。普通chat/其他任务保留原协议与顺序。新增固定全文review文件docs/wake-up-fixed-prompt.md。按旧多人格规则组合口径3316runes→1676（约49.5%减少），单人格2384→1676（约29.7%），排除动态人格，不把字数称token或实测延迟。

Wake action枚举收紧，持久化按实际receipts，不保留幽灵message。独立review发现duplicate_suppressed仍可能被计新消息和重置idle到旧时间，主线程已同时排除动作分类和clock输入并补回归。

事实版本核查未发现投影/准备阶段ensure-on-read写入。已证明memory_embeddings异步缓存更新会推进facts，WorkingPersona相同upsert也会推进。新增0058_semantic_fact_generation（从0057升级），移除缓存trigger，真实memory/source/summary/state变化仍追踪，CAS不放宽；旧迁移不改，新纠正SQL在installer之后执行防headrerun装回。相同人格写入变为无UPDATE。无法追溯39次全部来源，后台summary或真实写入仍会合法冲突。

最终本地Go race1422 PASS/494 SKIP/0 FAIL（含子用例），Web/client98 PASS；generate/typecheck/vet/build/diff通过。新增PG empty→head/0057→head/headrerun及cache-fact边界test因无隔离DB跳过，真实模型/部署由用户验收。没有安装DB或写正式数据；本轮未提交。


## 2026-10-10 专用 Goal Evaluation Tool 改造

用户批准本轮采用专用评估 Tool，并明确普通聊天/WakeUp 不提供修改目标状态 Tool。现场只读诊断 request142 显示根 goal:1/4 完成证据有效，但 goal:1 的 Stage 以助手转述作为 information 成功证据，触发 goal_judgment_self_report_not_business_fact；原全批事务回滚两个根目标。

变更边界：真实行为位于 RunGoalEvaluationTask 的输出协议和 ProcessGoalEvaluationIntent 的批量提交，不在自然语言摘要。增加仅 Goal Evaluation session 可执行的三个原生能力 goal.evaluation.submit / goal.object.submit / goal.plan.submit；复用 Eino、ExecuteTool、ApplyGoalEvaluation、原证据校验、短事务和 Tool receipt。根目标含 due review/关系确认原子提交；Stage/Commitment 和 plan 分别提交。最终 summary 不再作为状态写入 DTO。普通模型目录移除 goal.decide，Owner 直接治理和独立 Planner 保留。

利用既有 goal_evaluation_requests.snapshot/result 保存协议标记、冻结 refs、逐项 accepted digest/result、Core 管理的版本链、partial 覆盖与 memo；无新表或迁移。请求行锁和 claim_revision fence 在每次提交校验，能力自有授权接口在 receipt replay 之前检查私有 context session、FormalAgent、run/claim、surface、scope、实际 native/provider identity。可纠正领域拒绝先回滚 PostgreSQL savepoint，再作为真实 Tool 失败反馈；数据库和取消错误保持失败。接受的相同 payload 重放已有结果，改变 payload 冲突。子对象证据独立保留链接。

同请求重试维持 refs 和已接受结果；RunID 包含 claim_revision，未处理根目标才继续提交。部分成功 memo 可从 retry/failed 请求读取，不把更新旧请求的时间冒充新评估时间。只有全部根目标已覆盖且无 deferred 目标才消费相应源版本；未处理证据保留。过期快照保留成功兄弟并为未处理目标另排新快照。评估 Tool acknowledgement 不成为完成证据。policy=goal.evaluation.v4。权限、来源/CAS、暂停 ready_for_settlement 和已有业务完成条件不放宽。

本轮不部署、不操作正式业务数据、不安装数据库，不重写 Planner/普通聊天/WakeUp 提示词，不引入第二 Agent loop 或 DTO 伪 Tool。验证覆盖原生 HTTP rejection→后续根完成、私有权限/封闭参数、摘要无权限、持久化兄弟隔离/重放/partial retry/memo/冻结 refs（无隔离 DB 时 SKIP）；原 Goal fixture 仅在测试 Provider 中改为真实 native ToolCall 脚本，生产无兼容 fallback。


## 2026-10-10 Goal Evaluation 仍不完成：原生调用提前结束

只读正式诊断确认新版已上线：13:07 request goal_review_event_146_533c5ba8368075db8f6ef201546bd71a 的 response 是 {"summary":"正在评估 4 个目标。"}，tool_calls=[]；领域 retry/goal_evaluation_native_submission_missing，evaluated_goals=[]、submission_errors=[]，四个目标未提交。此次没有进入任何 Tool，不能归因于上轮 Stage 证据回滚。

代码缺口：执行请求仍带最终 summary JSON grammar，ToolChoice 默认 auto，所以 summary-only 是合法的原生 Runner 终态。修复只在 typed GoalEvaluation 安装逐物理请求策略：DB 中冻结根目标未全部提交时，required native Tool + 去最终response_format；覆盖后 auto + 原summaryschema；保持可选object/plan。Generate/Stream同一Runner、每轮真实身份/header/思考关闭、有效预算同步。独立核验补工具自由的 final repair隔离和不可原快照修正的stale错误退出，保留fresh-request恢复。测试夹具改为能按三种私有Tool目录识别无final格式的执行阶段，不在生产解释final DTO为Tool。

真实HTTP/Eino首个RED：TestGoalEvaluationPhysicalRequestPolicyRequiresNativeRootsBeforeSummary exit1，Generate/Stream×0/partial coverage四组均观察auto+summarygrammar并提前结束；GREEN四组PASS，实际断言canonical闭合Tool schema、tool_choice required、无response_format、实际stream=true、拒绝反馈不推进覆盖、最终auto/schema恢复以及headers真实一致。另一个fixture路由RED观察called=0/content={}，修复后GREEN。生产数据库policy与typedTask阶段断言已补，无隔离数据库时明确SKIP。正式部署和真实模型语义仍由用户验收，未写正式业务数据。

升级Core/Worker后对卡住目标手动发起一次复核，诊断应先出现 goal.evaluation.submit 的原生ToolCall，再出现实际ToolResult；最终summary不会再作为状态依据。原先已耗尽重试的failed请求不由代码伪装成功或自动改状态。本轮无新migration，head仍0058_semantic_fact_generation。
