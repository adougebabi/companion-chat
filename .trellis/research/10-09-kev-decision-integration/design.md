# Kev 接入技术设计

## 1. 现有权威与设计边界

入口见 research/entry-mapping.md。使用 internal/ai 中的协议客户端与纯策略类型；internal/core 的小型应用服务负责 settings、快照、记录和领域适配；已有 capability、workflow、browser 边界保持职责。不将 *App 注入 capability 实现，不添加总控 Agent、第二 dispatcher 或手工 Tool continuation。

最新 structured-turn-contract/cognitive-runtime 优先于历史 docs 中 winner-only/双生成说明。所有已经提交的 Tool receipt、消息和事实不可回滚；本次 gate 只控制尚未释放的候选 batch。直接 Kev 触发的合法 persona 命令必须使用独立 Tool 身份，不伪造模型 ToolCall ID。

## 2. 协议与服务

建议 internal/ai/decision 包：KevClient、DecisionSpec、Question、ValidatedAnswer、PolicyOutcome；只有 HTTP/System One，绝不套用 Chat Completions。Core DecisionService 依赖窄 SettingsReader/DecisionRepository/CandidateBuilder/Clock/Permit 接口，业务适配传入 original thunk。

顺序：父 ctx 检查→读取开关→硬资格/最小快照→获取有界 permit→请求 pending 可选记录→子调用→解析/逐题校验→持久保存实际输出与策略→父取消检查→重读开关/config revision及相关状态→应用原领域边界→独立更新实际 application 状态。disabled 在额外候选构建前退出。存储与网络不处于业务事务；fallback 只在需要时调用一次。

响应大小有限；JSON 必需字段、题 ID/type、允许 choice、有限 [0,1] 分布、合计容差、最大项一致性均检查。结构级错误整批 fallback，独立题错误逐题 fallback；组合仍做依赖闭合。confidence 与选中概率分别保存；策略不覆盖 raw。父 ctx 已取消绝不 fallback。

## 3. 动态配置与预算

Owner settings 增加 kev 配置键，含全局、七点 flags、endpoint、模型标识/revision（未知为 unknown）、timeout 1500ms、selection_budget 3000ms、max_concurrency 1、max_batch_questions 8、retry 0、strategy choice_argmax、guarded 可选阈值、max_deferrals 3、延后时间、retention_days 7、请求/响应/spool 字节上限。

凭据只从 setting_secrets 固定 purpose 解析，遵循既有 AEAD/write-only/明确清除规则。endpoint 禁止携带 userinfo/query 凭据。runtime_settings 增加服务器 revision，原 upsert 自增；浏览器不能提交版本伪造有效状态。call 捕获配置快照，应用前再次查询，关闭记录 discarded_after_disable；其他版本/策略变化保守 original/config_changed。

同进程共享 semaphore 覆盖 Core/Worker 服务实例内所有点；跨实例服务总并发不能宣称已经受此限制。必要时用短事务 durable permit+租约共享现有 PostgreSQL，不持有事务/Actor 锁跨 HTTP；排队属于总预算。熔断/预算耗尽走原路径，关闭绝无周期健康调用。

## 4. 可靠记录与 migration

新增下一可用 additive migration（当前预计 0056_kev_decisions，实施前重查 head）：

- runtime_settings.revision。
- kev_decision_requests：请求 ID、config/spec/model 版本、实际请求/raw响应、HTTP/status/error/usage/latency、started/completed 时间/字节数。
- kev_decisions：decision/request/question 关联、Actor/Agent/trace/run/event/candidate/rule/goal、raw answer、分布/choice/confidence/margin、policy outcome、application status、fallback/rule reason、前后集合与实际 action refs。
- kev_deferrals：候选点/identity/revision 对应的 deferred_until/count；必要 permit lease 独立。

request/question 唯一关联，索引采用 started_at DESC,id DESC 与 scoped filter。模型数据可靠提交后才返回可采用 outcome；业务提交后独立更新 actual，不混淆模型 yes 与硬规则拒绝。pending/未知 application 崩溃恢复只标未知，不重放副作用。

主库不可写时使用有界 fsync append spool（独立记录 codec、rotation、容量限制、受保护权限），Core/Worker 配置各自持久卷。导入基于 request/decision ID 幂等，只导入诊断而不执行业务。磁盘和数据库均失败时 original+安全告警。request/raw 只含真正提交的必要状态，排除 headers/credentials；不改变既有 Owner 诊断的原文保留策略。Kev 单独采用七天/行数/字节上限，cleanup bounded CTE，先 decisions 后 requests；domain audit 保留。

## 5. 四个调度点

WakeUp 抽确定性 preflight，含 cancellation/cycle/epoch/enabled/active/replay/sleep；Kev 在 EnsureDirectConversation 前调用，避免 no 已产生业务写。yes/original 进入原 RunFormalAgent。no 保留当前 cycle、last_run 和水位，WakeUpWorkflow 支持 not_before durable timer/CAN，不借 next-due completion 偷增 cycle。

Reflection 在 lease claim、真实证据过滤后调用，no 释放 lease 且保留 watermark，ReflectionWorkflow 持久延后；零证据继续原无模型路径。网络不在 DB transaction。

Goal Evaluation 在 memo/eligibility 后逐 Goal 筛选；仅 yes/original 的 exact offered subset 进入同一 RunGoalEvaluationTask。no 子集保持原 source 未消费，重新排期，不耗原 execution attempt。混合批 commit 不丢 deferred remainder。

Planner 在现有 policy/capacity/trigger/claim 后 gate；Owner 手动与原强制语义旁路；no 保存同 run/source，释放 claim、设置 available_at，独立计数。既有 deferred workflows 读取实际 not_before。最大连续延后后下一合格机会 original，仍先过硬门禁。完成检查实际提交后才产生 replenishment candidate，现有事务链路保持。

## 6. Tool 可见集与原生 loop

维持 original authorized definitions、required definitions、visible definitions、explicitly loaded definitions 四个 request-scoped 集合。新增最小 discovery/load capability，独立查询授权 registry；不以 capability.request 冒充加载。required 由真实 Agent 输出/查询需求建立受保护策略，并用固定库存测试；不猜 capability 分组或能力名称。

executor table 注册完整授权工具，可见 schema 单独送往物理请求。当前 v0.7.37 没有 rewrite ToolInfos middleware，采用既有 queuedToolCallingChatModel 在 Generate/Stream 调用的最后 model.WithTools(visible infos)，不新增另一组工具对象或更改全局单例。底层 OpenAI component 实际支持 options.Tools 覆盖；research 隔离试验已验证 Generate native loop 动态增长。公开候选仅包含授权范围；隐藏工具的任意直接调用仍需 admission 检查。

discovery 成功只扩张本 run visible 集，dependency/context slots 补齐；旧 no 不撤销显式加载项。每次物理发送重读开关，disabled 恢复 original 集合，保留合法历史与已提交结果。budget/diagnostic Tool schema 必须同步真正 wire 集，parallel_tool_calls 和 ref maps 也对应有效集合。T5 覆盖 streaming、并发 isolation 与关闭恢复。

## 7. 人格 batch gate

仅评估实际已声明的 persistent switching 规则；typed 时间/标志规则由确定性程序处理，不能从 prose 猜分类。纯语义条件用 Kev；需要候选提案的规则在完整响应可知后、返回给原生 tools node/外部可见发布前处理。

在现有模型适配/Agent hook 边界增加 request-scoped BatchAdmission：先判断整个尚未执行 batch，再开放任意副作用。yes 通过既有 ExecuteTool→personaActionService→applyPersonalityDecisionPlanTx 提交；把 actual receipt 用于内部权威更新而非制造 model tool success。成功后刷新 Working Persona/Actor/profile/ref/context，旧 A batch 只留诊断且不进入执行 trace/graph消息；同一物理模型适配边界生成 B，让原生 Runner 消费 B 的真实返回。每个物理 A/B 请求独立 queue/id/diagnostic/budget，不重启整 run，不恢复旧 outer takeover coordinator。

实现时优先使用旧版 AfterChatModel 对完整消息作 gate；如模型事件在 hook 前可向下游泄漏，则在现有 queued model 出口先完成可取消的有界流式缓冲与 admission。不能以简单 per-tool middleware 替代整批检查。当前隔离 gate 仅证明阻止全部 Tool 执行，不证明 B 再生成和 SSE 时序；先完成 T7 的生产桥接验证再继续其他接入。

no 记录 run/rule 对应拒绝权威，后续 persona.switch 相同候选不能被 Main 覆盖；若出现调用，返回真实 not_applied 业务拒绝，不能伪造 Tool 成功。fallback 走现有 Main 工具判定路径，未使用旧已不在生产链路的 assessment helper。多 yes 采用现有规则优先级；无唯一胜者 original，不比较跨题概率。一次 run 最多一持久切换；stale 采用当前状态重建原路径，不提交旧结果；如果 A 先前轮已有合法 commits，仅丢本批未执行动作，绝不回滚既有事实。

## 8. 上下文选择

原权威 projection/ref index 只读保留。workingMemoryInputFromProjectionForSurface 产生 candidates 后、required reserve/ResolveWorkingMemory 前筛 optional。required 的真实事实、承诺、active goal compact catalog、正在进行协议/结果永不让 Kev 排除。候选摘要包含 stable real ID，选中再从既有来源读取，无模型生成数据。

准备顺序固定：最小权威快照/goal catalog→Tool 选择→工具依赖字段→optional context→原 loop。过滤后继续原 WorkingMemory 和 whole-wire budget；yes 不是无限预算授权，fallback 也重新过预算。原摘要 producer/coverage 和历史压缩不变。不将 Kev logs 再注入运行 context。关闭按原来源重新组装，不使用旧持久裁剪表。

## 9. API/UI 与本地配置

Owner-only typed settings section 显示全局/七点、endpoint、版本/生效状态和手动无副作用 test connection，secret write-only。新增 scoped Kev list/detail/export，延用 snapshot cursor/Owner auth/CSRF、安全错误映射；始终按 started_at DESC,id DESC，不按异步 insert 顺序。

更新 browser/Core DTO、两个 OpenAPI generator 与生成客户端；Web 独立 section/store epoch/cursor；安全文本/raw JSON 展示，不用 v-html。展示 call_status/policy_outcome/application_status 及理由，时间 RFC3339 UTC→明确 zone；指标仅调用/采用/拒绝/回退/延迟。

交付独立本地配置应用入口，经原 settings 权限写入全启用七点/choice_argmax；不是只在 example 中启用。不得迁移自动覆盖已有 Owner settings 或远程环境。127.0.0.1 仅当前进程网络；NAS/Docker endpoint 由实际可达性核验决定。没有真实服务仍正常启动/回退。模型 revision unknown 如实记录。

## 10. 兼容与恢复

Temporal workflow 添加 deferred 分支用项目既有 history/version 规则隔离旧历史；原 completed/noop 结果保持兼容。新 schema 仅显式 migrate 升级，API/Worker readiness 不自动迁移。回滚先全局关闭并验证零调用，保留新增审计/迁移和既有 Tool facts，不数据库降级或重放业务。若技术验收不能满足附件范围，更新计划并报告具体冲突，不用空实现通过。

## 反馈修复：0057 逻辑运行协调

旧 physical queue 契约不变。增加 PostgreSQL logical_agent_leases（Fluctlight PK，owner token，kind，expires_at）：3 分钟有效，15 秒心跳，获取前不建立语义快照；同 actor 嵌套继承 token；withTransaction 提交前短事务锁及 token/expiry fence；释放不得清理继任者。覆盖 conversation/native cognition、WakeUp/Reflection、GoalEvaluation/Planner。Goal Evaluation 等待发生在 claim 前，原 attempt budget 保持。不同 Fluctlight 可并行，独立事实更新仍必须通过 CAS 校验。

新增证据 link 的 RowsAffected 是 conversation_candidate 入队依据，重复/无来源不建请求；真实 outcome 为明确关联 Goal 建 candidate link；source-remainder 只看 active/paused 关联 link/goal revision，保留源存储，不用任意 owner/profile 待处理 source 无条件重排。retry 返回实际 not_before，避免 30 秒无效轮询。goal.evaluate 描述声明后台评估等待当前 Agent 结束，不能在当前 run 内轮询等待自己的任务。


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


## 2026-10-10 原生评估提前返回 summary 修复

现场 goal_review_event_146 在13:07 返回 summary-only/tool_calls=[]；见 research/goal-native-premature-final.md。实施仅调整专用 GoalEvaluation 的逐物理请求 Tool选择/输出格式：缺持久化root提交时 required native Tool 且不启用最终summary grammar，全覆盖才恢复final格式。普通Agent保持原行为；仍单Eino Runner，Source/CAS/paused/独立提交不变。须受控真实HTTP测试先RED再GREEN，不重放正式写入，不新增迁移。


## 2026-10-10 重复 current_facts_stale 与后台 Agent 插队

现场只读确认 turn_dcd23c98-5a81-47f0-b994-c1a1e4cab636 14:12:22.567—14:13:52.994，单轮无native Tool，settlement expected facts_gen_27724 actual27752。14:13:53启动GoalEvaluation在错误之后，不可作为本次根因。natural assistant INSERT被generation trigger豁免，输入inbox/userMessage在快照之前，不是自身receipt漏衔接。现有generation行只有计数/时间，无法事后精确归因这28个版本。

确定缺口两类：1. semantic Intention每消费processed fact只更新trigger_cursor_sequence，仍被fluctlight_intentions任意UPDATE触发facts推进；同值consolidated summary UPDATE也非必要事实变更。2. ProcessConversationSummaryIntent/processRuntimeSummary、ProcessConversationDailyMemoryIntent没有logical lease，独立Temporal Activity可以在chat持lease期间快照/LLM/提交；daily-memory真正写Memory authority。dispatcher优先级不阻止已启动Agent。不能仅凭代码缺口断言现场28次就是这些写入。

实施边界：复用现有per-Fluctlight durable logical lease补后台summary/daily-memory两个Core入口，覆盖snapshot→LLM→commit，保留同owner重入、不同Fluc并发、lease fencing/取消，不移动handleTurn即时接受新输入/supersede事务。新输入应立即终止旧turn，而不是被旧Agent锁挡住。Metadata cursor只在所有其余语义字段不变时跳过facts bump；真实Intention status/trigger/body/revision变化仍推进。Summary真实内容/来源/状态/日期分类变化保持现有CAS，仅避免完全同值consolidation UPDATE。inner-state已有独立revision但仍是事实权威，本轮不移除它的generation保护。

新增下一additive migration（0059，前head0058）在历史installer和0058之后执行：cursor-only排除、generation每次实际bump的有界来源journal（Fluc/generation/table/op/entityId可选/txid/time，禁止存业务payload；每Fluc最多256或512行），同事务记录并裁剪。现有one-arg bump API兼容，direct/owner/child/message/fact/schedule/fluctlight trigger准确记录来源；保持0058 embedding trigger移除及head-rerun不恢复旧过滤。发生currentFactsMismatch时把expected→actual区间来源按table/op/count放现有Owner诊断payload，bounded read，日志失败不覆盖原CAS错误，窗口不完整要明确不可全归因。没有第二Agent loop、自动忽略锁冲突或盲目重跑已提交业务。

验证：真实入口受控并发测试先RED：chat持同Fluc lease时后台summary/daily不得执行snapshot/Provider/commit；释放后执行；同owner重入与不同owner不阻塞；取消/lost lease保留fence。PG真实generation tests验证cursor-only及同值consolidation不推进、真实语义/来源更改推进、原CAS仍拒绝真实变化；empty→0059、0058→0059、head-rerun；journal同事务rollback/owner scope/retention与诊断区间。无隔离DB按用户约定SKIP，不安装DB、不写正式数据。纯入口与SQL契约/有界诊断解析测试应本地可运行。真实生产来源未追溯之处如实报告，升级须应用新migration。
