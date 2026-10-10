# Kev 实施计划

## 阶段门与责任

本轮是同一集成交付，采用一个任务管理七个点和联合验收，不因工作流划分而提前宣布局部完成。规划 gate 后 task.py start。主线程负责设计取舍、生产代码修改与最终验证；子代理只承担独立只读探索/核验，遵守本仓 AGENTS.md 的 clean fork/单轮/等待规则。

## 有序执行清单

- [x] 保存来源，确认以完整实施文档为正式需求。
- [x] 三路只读核验真实入口，主线程读取权威架构/规范并点验关键代码。
- [x] Context7 官方文档 + 锁定依赖源码核验 Eino v0.7.37。
- [x] 隔离 native-loop/真实 OpenAI adapter fake HTTP 试验：wire schema 补载、整批 Tool 拦截。
- [x] 完成 PRD、design、实施计划和真实 spec/research manifests。
- [x] 提交完整规划摘要，用户批准后已执行 task.py start。

### I1 基线与框架高风险验证（先于大范围接入）

- [ ] 读取 trellis-before-dev 和本次目标层全部 relevant specs。
- [ ] 记录当前 commit/status、后端与浏览器基线；隔离数据库/服务能力清单。
- [ ] 为现有 queued model/loop 添 production seam 与行为测试：Generate/Stream wire 可见 schema、真实 discovery feedback、原 tool IDs/receipts、不共享状态。
- [ ] persona 整批 pre-admission→合法 switch→B 生成：A 未执行 batch 无任何副作用，流式不泄漏，no 能阻止后续覆盖；不能另建 loop。
- [ ] 若桥接无法满足原 loop/批次边界，停在该技术点修订设计；不得用静态 tool-name prompt 或 fake success 交付。

### I2 共享判定与记录

- [ ] 有界 System One client、版本化 specs、严格逐题 parser/策略、父取消。
- [ ] additive migration、settings revision/allowlist/secret、DecisionRepository。
- [ ] 可靠 raw-before-adopt、实际 application 更新、bounded fsync spool/volumes/幂等导入/容量告警。
- [ ] 同实例/进程 permit、选择总预算、熔断和精确 original fallback。
- [ ] disabled 无额外准备/HTTP；response 后 recheck config/state。

### I3 七点真实适配

- [ ] WakeUp preflight→gate→原 body；Reflection evidence→gate→原 proposal；各自 durable deferred workflow。
- [ ] Goal completion 逐 Goal 分支、exact subset/remainder/source 保留；Planner 自动 gate/manual bypass；deferral 不耗 attempt budget。
- [ ] 最大三次 no 后下一合格 original；not_before durable timer；完成提交后 latest state 规划。
- [ ] tools.select 与真实独立 discovery/load；authorized executor/visible schema/依赖状态同步；explicit load 不重否决。
- [ ] persona.switch 完整规则/优先级/no authority/CAS/once-per-run，与 I1 validated seam 连接。
- [ ] context.select 仅 optional；required/活动目标 catalog/承诺/ref pairing；关闭从源恢复。

### I4 管理和诊断

- [ ] Owner settings typed controls、生效 config revision、secret write-only、手动 protocol test。
- [ ] Kev list/detail/export DTO/filter/scoped SQL/stable cursor；Actor 过滤不能授予权限。
- [ ] Core/browser generators+generated clients，Web section/state epoch/raw详情/关联/时区。
- [ ] retention 七天动态配置+真实 bounded cleanup；spool 记录查询/导入限制说明。

### I5 验收、配置、报告

- [ ] T1–T10 逐项执行，覆盖 AC1–AC8，不仅 parser tests。
- [ ] 用户本地配置全启用实际通过 settings 机制应用/核验；地址从 Core/Worker 网络核验；不改远程生产。
- [ ] 完整 replay 文件若找到真实沿用；找不到仅固定 fixtures 明示限制；保留已知错例 expected。
- [ ] 真实 Kev 四场景 E2E 串行、隔离 Actor/存储、测试外部适配，服务不可达明确 NOT_RUN。
- [ ] 全后端+Web checks、迁移 empty/previous/head、原 workflow history 兼容、UI production/mobile+console。
- [ ] kev-integration-report.md：入口共用表、配置/关闭/endpoint/诊断操作、命令退出码和数量、mock/replay/live区分、未验证/失败归因。
- [ ] 独立只读核验与主线程最终验证；更新真实 spec 契约；按项目流程处理 commit/finish，不推送或部署未授权环境。

## 验收映射

| 文档测试 | 重点 | PRD |
| --- | --- | --- |
| T1 | global/per-point/off/unavailable，原请求/Tool/prompt/调用轨迹等价，Kev HTTP=0 | AC1 |
| T2 | 全协议矩阵、partial、低概率 choice、guarded、取消 | AC1 |
| T3 | 七点真实 yes/no/fallback/disabled 应用 | AC1–AC5 |
| T4 | 水位/cycle/retry不消耗，延后兜底，manual/forced/sleep/capacity/concurrency | AC2 |
| T5 | 原生 loop actual wire 增长/补载/真实结果，授权/多Actor/独立Tools/affected Agents | AC3 |
| T6 | required/catalog/承诺/ref pairing/budget/source不变/恢复 | AC5 |
| T7 | A batch无副作用、B、no、stale、失败、stream、once | AC4 |
| T8 | config disable races，DB/spool failure，cancel/circuit/budget，逐题导出/权限/order | AC6 |
| T9 | 固定协议回放与真实 Kev active 四场景；准确率和 adapter 正确性分开 | AC7–AC8 |
| T10 | 后端/集成/前端/迁移/浏览器回归和明确 baseline/new/env 区分 | AC8 |

## 验证命令

从仓库根执行，实际环境权限按测试需要处理：

```
go -C apps/core-go test ./internal/ai/agent ./internal/workflow -count=1
go -C apps/core-go test ./internal/core -run 'Test(Kev|GoalPlanner|GoalAssessment|GoalEvaluation|Persona|ProviderADK|RuntimeRecovery|ContextReference)' -count=1
go -C apps/core-go test -race ./... -count=1
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
```

Go真实数据库测试需项目既有隔离 fixture；无 DB 的 SKIP 不视 PASS。显式 migrate 只针对任务自建 disposable DB，empty→next head、0055→next head、head rerun/rollback 均测。可用时运行 infra/compose/run-platform-smoke.sh 与 infra/acceptance/run-go-live-provider-smoke.sh 相关 suites；不得复用含真实数据的 Compose volume 作为隔离环境。实施后补可直接运行的 Kev E2E 命令与参数，不虚构当前未实现 CLI。

## 风险/回滚点

I1 是 schema/人格关键风险点；I2 是可靠记录与 config race；I3 是 Temporal history/lease/cursor；I4 是 API/client parity/Owner隔离。每段完成后跑 owning tests，然后集成全量。失败先保持原路径；rollback 全局关闭，保留原 Tool receipts、审计和新增 schema。未发模型请求恢复原 catalog/context，已发请求和已提交副作用不撤回、不重放。

## 当前验收约定（用户 2026-10-09 更新）

以本地代码测试、类型检查、构建与静态质量通过为交付门禁。正式环境和真实 Kev E2E 用户自行执行；交付 migration、设置操作、验收命令与 NOT_RUN 清单。不要为本任务再安装/启动 Docker 或数据库。

## 本轮交付状态

七点实现、设置/诊断/配置命令已交付。两轮只读核验提出的取消、spool、权限/批次问题由主线程修复；新增原生 Generate/Stream 回归发现并修复丢弃提案的内层 callback 泄漏。最终本地 race/vet/build/generate/typecheck/test/build 均通过；证据与正式操作见 docs/kev-integration-report.md。正式数据库/模型/部署验收按用户要求留待其执行。产品代码尚未提交，等待 Phase 3.4 的一次提交确认；未运行 archive/finish，不改变其他任务。

## 正式反馈修复交付

App URL/computed 路由回归 4 RED→GREEN；无新证据 candidate 入队回归 RED→GREEN。0057 PostgreSQL 逻辑运行租约在 6 个入口快照前接入，withTransaction 提交 fence；重入、跨 owner、取消、过期释放回归通过。独立只读核验未发现确定的新 lease/deadlock/时序问题。Go race 1370 PASS/490 SKIP/0 FAIL，Web/client 98 PASS，类型/vet/build 通过。真实数据库验收仍由用户执行。当前分支 codex/kev-runtime-recovery，产品改动未提交。


## Goal Evaluation 完成未落库反馈修复（2026-10-09）

用户提供的 response 中 goal:2/3/4 的 review 均引用 stage:1.1；goal:2 在描述无现有阶段时还使用 adjust/object_ref。引用按目标编号绑定，旧全局 enum 会放行跨目标选择，hydration 随后拒绝整批，导致 goal:1 的 completed 也未提交。用户看到物理 response，没有领域提交错误展示；未查询正式数据库，不将推断错误码当作生产实测。

本轮 schema 改为按 Goal 判别并限制对象/标准所有权及 create/adjust 操作；wire/coverage 可纠正一次完整输出，替换仍校验；wire 与耗尽 final-contract 错误 terminal。诊断独立展示目标评估提交状态/error/result，成功结果包含实际 goal_outcomes。原批次原子性、CAS、证据与 paused settlement 规则保留；无新 migration，head 仍 0057_logical_agent_leases。

本地最终验证：Go race 1374 PASS、490 SKIP、0 FAIL（含子用例）；Go vet/build、pnpm generate/typecheck/test/build、git diff --check 通过；Web/client 共 98 PASS。独立只读核验无 findings。SKIP/真实数据库/真实模型/正式部署仍由用户验收。当前 codex/goal-evaluation-output-recovery 未提交。


## 部署环境继续失败：覆盖数量与 thinking（2026-10-09）

用户授权浏览器检查环境。实际诊断中 goal_review_event_120 和 goal_review_event_122 提交 failed/goal_assessment_coverage_missing；四目标输入只返回 goal:1，纠正后仍缺其余目标。确认已上线提交诊断，并非仅显示未刷新。Reasoning sidecar较长且中断，但没有 finish_reason/usage，不宣称已证实 token 耗尽。

补修：evaluations minItems/maxItems 等于 offered count，plans 最大数量同 count；Goal Evaluation 关闭 thinking，实际 direct/ADK HTTP 明确发送 enable_thinking=false，避免省略字段导致服务器默认启用。保留总 output reserve、其他 Agent策略、所有权/CAS/批次事务；无 migration。Schema 数量回归 RED→GREEN，HTTP/ADK 修复回归通过。生产入口 thinking 断言补入数据库集成测试，未配置隔离数据库时仍 SKIP。

本地 Go race 1378 PASS/490 SKIP/0 FAIL（含子用例），vet/build/typecheck 通过；正式状态仅只读检查，未部署、未写业务数据。最新补修未提交。

补修最终检查：Web/client 98 PASS，production build 通过；git diff --check 通过。独立核验未发现产品正确性缺陷，提出 production call-site 测试缺口，已在现有 PostgreSQL 集成测试补断言（本机 SKIP）。


## 2026-10-10 Goal 完成判定与 Kev 请求负载修复

用户授权两项一起处理。Goal wire 增加 criterion_quote/optional_improvement，负面缺口须来自冻结原标准；父 Goal/Stage/Commitment 纯 preflight 与事务复用语义校验，完成标志双向一致，判断错误使用既有一次完整替换纠正。policy 升为 goal.evaluation.v3，旧 memo 在下次评估不再复用。实际作者陈述与 quotation 区分；active 与 paused 规则保留。字面引用校验不能证明任意模型语义正确，不把 mock 说成真实模型验收。

Kev context/tool 每个 batch 只带当前候选内容，保留当前任务输入和协议；新增明确批次 state builder，原 Decide 兼容。共享 stage、Service budget、单请求和父取消有不同原因，覆盖 HTTP headers/response body 等待。没有改 timeout 默认值、正式配置、全局 token 预算或推理服务；逐 Goal completion gate 仍为原逐项请求。无法承诺所有请求<1秒，剩余大 currentInput/服务排队须实际测量。

最终产品代码无公共API/迁移变更，head仍0057_logical_agent_leases。本地 Go race 1396 PASS/490 SKIP/0 FAIL（含子用例），Web/client98 PASS，generate/typecheck/vet/build/diff通过。独立核验指出主线程新增测试身份缺失，已补完整身份并实际断言active completed/progress1及paused保持。未操作正式业务数据、未部署、未提交。


## 2026-10-10 Conversation 输出与结算反馈

正式只读检查：09:48模型返回reply侧车但无native ToolCall/visible_text；09:42模型有自然回复而后结算current_facts_stale；08:52服务507明确内存不足。未发现Kev移除conversation.reply或required current_state；输出最终schema缺口早于Kev存在。修复：无已提交消息/已接受媒体时加强本地final输出校验，复用一次tool-free repair；已提交输出不要求重复。post-run诊断在通用分类回退时保留publication/settlement外层码及内层cause。事实冲突补期望/实际generation和settlement/tool_receipt边界，不关闭CAS。

主线程曾怀疑acting_profile_id UPDATE引起事实推进；核对trigger UPDATE OF text,attachment_refs后否定，已撤掉相关publication/profile改动。不可将它记录为已确认根因。09:42摘要物理模型在冲突之后启动，也不能认定为该次根因。实际并发写入/部署trigger仍未实测；新版本诊断为后续定位提供值，不能宣称所有current_facts_stale已修复。

本轮无新migration/API/正式设置改动，无部署或生产数据写入。最终测试结果另附；数据库验证按用户本地验收约定明确SKIP。当前代码未提交。

本轮最终验证：Go race 1403 PASS / 491 SKIP / 0 FAIL（含子用例），vet/build通过；pnpm generate/typecheck/test/build通过，Web/client98 PASS，diff check通过。HTTP回复纠正测试实际运行；新增数据库诊断测试因未配置隔离PG跳过。


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


本轮最终本地验收（2026-10-10）：全仓 Go race 完整执行，并在最后恢复路径修改后重跑受影响 Core race；合并当前结果 1427 PASS / 495 SKIP / 0 FAIL（含子用例）。pnpm generate/typecheck/test/build 通过，Web 76 + browser-client20 + core-client2 = 98 PASS。Go vet/build 和 diff 检查通过。测试所需本机 httptest/tsx IPC 监听在沙箱外运行；未连接正式数据。新 PostgreSQL 原生持久化、partial retry、重复提交、摘要无提交、deferred review 迁移等用例因未配置隔离 DB 而 SKIP，真实模型/部署仍由用户验收。独立权限核验未发现绕过；提交核验发现 stale replacement 遗失 deferred reviews，已修正目标合并、活动目标筛选及限定 ended review supersede，并补回归。最后增加“全部根已提交后只恢复结算、零新模型调用”的恢复路径。代码未提交、未部署。


## 2026-10-10 原生评估提前返回 summary 修复

现场 goal_review_event_146 在13:07 返回 summary-only/tool_calls=[]；见 research/goal-native-premature-final.md。实施仅调整专用 GoalEvaluation 的逐物理请求 Tool选择/输出格式：缺持久化root提交时 required native Tool 且不启用最终summary grammar，全覆盖才恢复final格式。普通Agent保持原行为；仍单Eino Runner，Source/CAS/paused/独立提交不变。须受控真实HTTP测试先RED再GREEN，不重放正式写入，不新增迁移。


## 2026-10-10 Goal Evaluation 仍不完成：原生调用提前结束

只读正式诊断确认新版已上线：13:07 request goal_review_event_146_533c5ba8368075db8f6ef201546bd71a 的 response 是 {"summary":"正在评估 4 个目标。"}，tool_calls=[]；领域 retry/goal_evaluation_native_submission_missing，evaluated_goals=[]、submission_errors=[]，四个目标未提交。此次没有进入任何 Tool，不能归因于上轮 Stage 证据回滚。

代码缺口：执行请求仍带最终 summary JSON grammar，ToolChoice 默认 auto，所以 summary-only 是合法的原生 Runner 终态。修复只在 typed GoalEvaluation 安装逐物理请求策略：DB 中冻结根目标未全部提交时，required native Tool + 去最终response_format；覆盖后 auto + 原summaryschema；保持可选object/plan。Generate/Stream同一Runner、每轮真实身份/header/思考关闭、有效预算同步。独立核验补工具自由的 final repair隔离和不可原快照修正的stale错误退出，保留fresh-request恢复。测试夹具改为能按三种私有Tool目录识别无final格式的执行阶段，不在生产解释final DTO为Tool。

真实HTTP/Eino首个RED：TestGoalEvaluationPhysicalRequestPolicyRequiresNativeRootsBeforeSummary exit1，Generate/Stream×0/partial coverage四组均观察auto+summarygrammar并提前结束；GREEN四组PASS，实际断言canonical闭合Tool schema、tool_choice required、无response_format、实际stream=true、拒绝反馈不推进覆盖、最终auto/schema恢复以及headers真实一致。另一个fixture路由RED观察called=0/content={}，修复后GREEN。生产数据库policy与typedTask阶段断言已补，无隔离数据库时明确SKIP。正式部署和真实模型语义仍由用户验收，未写正式业务数据。

升级Core/Worker后对卡住目标手动发起一次复核，诊断应先出现 goal.evaluation.submit 的原生ToolCall，再出现实际ToolResult；最终summary不会再作为状态依据。原先已耗尽重试的failed请求不由代码伪装成功或自动改状态。本轮无新migration，head仍0058_semantic_fact_generation。


本轮最终本地门禁：Go race 全仓 1440 PASS / 496 SKIP / 0 FAIL（含子用例），Go vet/build、pnpm generate/typecheck/test/build、git diff --check 全部 exit0；Web/client仍98 PASS。新增的实际HTTP/Eino Generate/Stream请求阶段测试已运行，PG production policy/Task用例因无隔离DB跳过；真实模型行为/部署由用户验收。check阶段局部修复了stale退出和tool-free repair隔离，但代理核验未完成完整终态，主线程收敛后完成夹具路由RED→GREEN、补针对性回归并执行最终全仓门禁；不记录成完整独立审计。当前改动未提交、未部署，无新migration。
