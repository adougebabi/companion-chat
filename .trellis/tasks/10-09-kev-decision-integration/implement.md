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


## 2026-10-10 Goal 首次原生提交 nil map panic

正式反馈：App.executeGoalEvaluationSubmissionTx 报 assignment to entry in nil map。本轮确认首次请求 result={}，goalSubmissionRecords 将缺失 submissions 序列化为 JSON null 再解码；encoding/json 会把已 make 的 map 重置 nil。随后成功领域 apply/savepoint 后 records[key]=... panic，外层 Tool 事务回滚，无法完成任何首提。属于本轮专用 Tool 改造遗漏的初始化契约，不是模型判断或提示词问题。

修复只有 helper 解码后的三行 nil map 初始化；缺失、null 和 nil result 返回可写空 journal，已存 digest/output 保留；不全局改 JSON 工具、证据/CAS、暂停、事务或 Tool 选择，不新增迁移。新 TestGoalSubmissionRecordsAlwaysReturnsWritableJournal 对真实 helper 结果执行与提交路径相同的 map 写入：未修复时 nil result/missing submissions/JSON null 三组确切 panic RED exit1，stored record通过；修复后四组GREEN exit0。真实 PostgreSQL首提+重放既有用例继续保留，无隔离DB时SKIP，未写正式数据或部署。主线程接续并完成最终验证；没有把中断的实现代理过程计为完整独立审计。

本轮最终本地验证：Go全仓race 1445 PASS / 496 SKIP / 0 FAIL（含子用例），Go vet/build与diff check通过；新增首提journal回归四组实际GREEN，原始缺失/null/nil三组RED已确认。未修改前端/API，无需重新生成合同或前端回归；真实PG/模型/部署仍用户验收，未安装DB、未写正式数据、未提交。


## 2026-10-10 重复 current_facts_stale 与后台 Agent 插队

现场只读确认 turn_dcd23c98-5a81-47f0-b994-c1a1e4cab636 14:12:22.567—14:13:52.994，单轮无native Tool，settlement expected facts_gen_27724 actual27752。14:13:53启动GoalEvaluation在错误之后，不可作为本次根因。natural assistant INSERT被generation trigger豁免，输入inbox/userMessage在快照之前，不是自身receipt漏衔接。现有generation行只有计数/时间，无法事后精确归因这28个版本。

确定缺口两类：1. semantic Intention每消费processed fact只更新trigger_cursor_sequence，仍被fluctlight_intentions任意UPDATE触发facts推进；同值consolidated summary UPDATE也非必要事实变更。2. ProcessConversationSummaryIntent/processRuntimeSummary、ProcessConversationDailyMemoryIntent没有logical lease，独立Temporal Activity可以在chat持lease期间快照/LLM/提交；daily-memory真正写Memory authority。dispatcher优先级不阻止已启动Agent。不能仅凭代码缺口断言现场28次就是这些写入。

实施边界：复用现有per-Fluctlight durable logical lease补后台summary/daily-memory两个Core入口，覆盖snapshot→LLM→commit，保留同owner重入、不同Fluc并发、lease fencing/取消，不移动handleTurn即时接受新输入/supersede事务。新输入应立即终止旧turn，而不是被旧Agent锁挡住。Metadata cursor只在所有其余语义字段不变时跳过facts bump；真实Intention status/trigger/body/revision变化仍推进。Summary真实内容/来源/状态/日期分类变化保持现有CAS，仅避免完全同值consolidation UPDATE。inner-state已有独立revision但仍是事实权威，本轮不移除它的generation保护。

新增下一additive migration（0059，前head0058）在历史installer和0058之后执行：cursor-only排除、generation每次实际bump的有界来源journal（Fluc/generation/table/op/entityId可选/txid/time，禁止存业务payload；每Fluc最多256或512行），同事务记录并裁剪。现有one-arg bump API兼容，direct/owner/child/message/fact/schedule/fluctlight trigger准确记录来源；保持0058 embedding trigger移除及head-rerun不恢复旧过滤。发生currentFactsMismatch时把expected→actual区间来源按table/op/count放现有Owner诊断payload，bounded read，日志失败不覆盖原CAS错误，窗口不完整要明确不可全归因。没有第二Agent loop、自动忽略锁冲突或盲目重跑已提交业务。

验证：真实入口受控并发测试先RED：chat持同Fluc lease时后台summary/daily不得执行snapshot/Provider/commit；释放后执行；同owner重入与不同owner不阻塞；取消/lost lease保留fence。PG真实generation tests验证cursor-only及同值consolidation不推进、真实语义/来源更改推进、原CAS仍拒绝真实变化；empty→0059、0058→0059、head-rerun；journal同事务rollback/owner scope/retention与诊断区间。无隔离DB按用户约定SKIP，不安装DB、不写正式数据。纯入口与SQL契约/有界诊断解析测试应本地可运行。真实生产来源未追溯之处如实报告，升级须应用新migration。

本轮最终本地验证：Go全仓race 1453 PASS / 499 SKIP / 0 FAIL（含子用例），Go vet/build/diff通过。后台入口work并发/重入/不同Owner/取消/fence纯回归、版本区间分类和SQL契约已实际运行。真实PG迁移/trigger/journal/跨进程与诊断聚合用例按用户约定SKIP；无安装/正式数据写入/部署。原公共API/前端无改动，未重跑前端；独立核验无确定findings。必须先应用0059再更新Core/Worker。未提交。


## 2026-10-10 Agent 白名单、工具目录、私聊与视觉身份修复

用户明确：私聊仅 conversation.reply 原生 Tool 发送，禁止 final xxx_text/分析代发；每个 Agent 独立必备白名单，只有其余可选工具参与 Kev；新增查询全部授权 Tool 的兜底。FormalAgentDefinition.MandatoryTools 已显式定义21个Agent；conversation_cognition/wake_up的reply/moment/image常驻，其余Agent按职责定义，tool-free保持空。新 capability.catalog 返回当前Agent原安装目录name/purpose/parameters，discover补载下一物理轮真实schema，二者不入Kev候选、关闭/版本变化/设置错误时仍可用。总注册48个Tool，普通Agent无法查询或调用目标评估私有提交工具。

移除 natural final 发布旁路和 visible_text schema；私聊在真实reply提交前 required Tool/no final grammar，media alone不解锁，final-only拒绝而不代发。VisualIdentity不是Kev漏选；按stage只暴露对应私有Tool，durable progress前forced/no final grammar，pending媒体零LLM，native_tools_v2 checkpoint保留旧失败历史与真实媒体IDs。现场0059 provenance已证实29783→29804的21次变化全部为视觉聚合UPDATE；ensure/refresh两处四业务字段同值guard停止无意义写入，真实变化仍推进facts。本轮无新迁移，head仍0059。

独立check修正恢复工具误入候选、settings-read fallback丢工具、tool-free Agent泄露utilities；主线程补物理请求disabled/version/settings-error三组同类回退回归。完整Go race/vet/build/gofmt/diff通过；最终全仓race JSON计数1468PASS/501SKIP/0FAIL（包括主线程追加的物理请求回退三组回归）。实际HTTP/Eino Generate/Stream测试已跑；PG/Redis/Temporal/真实Provider opt-in未配置跳过，不称生产验收。未改Web/API合同，无前端重跑；没有部署、安装DB、写正式数据或提交。服务507内存不足独立于本修复，不能称已解决。

本轮收尾同步了旧PG正向私聊夹具：显式ReplyText脚本产生原生reply与真实结果后final，SSE发送ToolCall后final，Goal closure/planner与fault sensitivity保留产品断言；移除成本报告中obsolete natural_final成功路径。夹具相关PG执行仍SKIP，历史剩余成本数字需隔离DB重生成，不能作为当前版本实测。


## 2026-10-11 目标完成观感与自治停滞修复边界

现场小说目标27条根修订完整历史未见completed→active，标准版本1；当前active0。真实聊天已提供“人物本身？应该是”和随后《灯塔》推荐理由，但最新评估仍反复仅依据旧《斯通纳》/泛小说兴趣。确认代码缺口：本次未提供的candidate也被全目标rejected，processed消息缺后续恢复池与对话邻域。修复来源覆盖/有界真实对话语境/历史候选恢复，保持原标准、CAS及模型语义判断，不强制完成。

短靴目标active24，无关联attempt，仅多次文字plan；全实例recent_outcomes错误归因可能诱导不存在的获取失败。确认plan无执行交接，Kev周期输入也缺active目标和执行义务。修复向现有自治cognition/WakeUp的持久交接及门控上下文，保留LLM通过真实intention/life工具执行和qualify链；不把prose转动作、不使用购物关键词、不新增Agent loop、不催逼关系目标。已有Intention、暂停、睡眠、依赖、权限预算与幂等须保留。目标执行结果与普通消息证据分离。

范围：GoalEvaluation sources/admission/persistence；accepted plan→existing cognition交接；Kev WakeUp bounded goal context；目标级attempt/outcome投影与必要prompt；对应回归。后端必须有本地可跑源覆盖/语义投影/交接policy tests，PG完整虚拟生活闭环无隔离DB诚实SKIP。正式环境只读，不修status、不部署、不安装DB。前端展示修正仅在明确必要时收敛，不扩张重构。


## 2026-10-11 目标完成观感、证据连续性与自治交接

现场只读：林岚人格晴岚的“根据对方真实阅读偏好推荐一本小说”当前active、progress0、criteria_version1、revision27。完整27条根修订为create/progressed/no_change，未见根completed→active，不能确认用户此前看到完成是持久根状态。实际聊天10-10 09:41“人物本身？应该是”与10:20《灯塔》推荐及理由存在，但最新评估仍仅依据旧《斯通纳》/泛小说兴趣。短靴目标active revision24，没有关联Attempt，多次text plan，且真实context_review_required阻塞；最近Planner失败query_results_missing。

确认并修复：移除所有未引用candidate整批rejected；有界历史恢复包括processed/rejected，关联/未关联各32并公平轮转，部分成功根提交保留实际offering历史；真实消息±1邻域原子准入/私有fingerprint+CAS，policyv5使旧memo失效。原标准与模型语义判断不变，不按关键词自动完成。

已接受plan保存稳定execution_handoff，既有周期WakeUp修复旧plan并在Kev前提供执行义务，当前真实Goalref绑定、candidate资格确认、已有dispatch去重；成功周期最多3次重新考虑/10m，失败睡眠延期不耗次数，后续评估保留marker、终态清除。全局失败不冒充目标失败，execution.last_result与last_evidence分离。

Planner新增实际query阶段：两条不同成功query前query-only forced/no final grammar；commit后必须后续物理轮snapshot。正常RepairGoalPlanning只为旧query_missing上下文复核提供版本化正常新请求，不重置旧失败/已提交记录，不把评估revision变化当新恢复机会，保留所有权限、budget、backoff、复核和CAS。

最终独立核验：Go全仓race JSON1486PASS/508SKIP/0FAIL（含子用例），vet/build/gofmt/diff通过；实际HTTP/Eino Generate/Stream已运行。PG源恢复、40候选保留、未主动用户消息的虚拟生活闭环、Planner恢复等因未配置隔离DB明确SKIP，SQL/参数只作静态核验。无新增migration，head仍0059；没有安装服务、写正式业务数据、提交或部署。未改Web/API结构，新增execution字段属既有open record，无前端回归。真实业务效果由用户验收，不能称正式Goal已完成或购物已执行。


## 2026-10-11 深夜空输出/格式失败排查

只读现场：22:15conversation_cognition单轮reasoning-only193tokens、content空、tool_calls[]，adk_final_message_empty；22:45Wake同类。22:55Wake315原输出及单次final repair均半截JSON，usage1403/1352，repair仍tool_count37；21:41等507明确Macprefill内存不足。应用reserve40960，不能把193或1403直接归因为额度耗尽；没有finish_reason旧诊断不能定论。

确认bug：repair清空definitions却保留Kev选择/人格门控，queuedGenerate/Stream会重新装tools。修复为显式tool-free final correction隔离Kev工具/人格+business policy，无工具无工具选择，修复调用关闭thinking并保留原格式/引用校验；不从reasoning发消息，不制造no_op，不加无界重试/自动重放业务。普通初始full38目录可由合法Kevfallback恢复，不据tool_count直接判错或删注册工具。

补齐物理response finish_reason、完整tokenusage（含可用reasoning/cache细分）和Stream terminal-only事件，提供真实空输出类型与model_output诊断分类；既有empty失败保持失败，不推断length，不冒充已发消息。仅runtime/diagnostics/测试最小范围，不改生产模型/配置/服务，不提高预算，不重写普通业务提示词。受控Generate/Stream实HTTP验证 repair零tools/零schema budget、原生副作用不重放，stop/length/unknown reasoning-only与terminalusage可追溯；PG不配置按约定SKIP。


## 2026-10-11 深夜 Agent 空输出与 final repair 重新注入工具

现场只读最新20条：23:05Wake成功，22:15conversation/22:45Wake adk_final_message_empty；22:55Wake315原final及一次repair均cut-mid-string JSON。私聊真实completion193、prompt16191、reserve40960；Wake两轮1403/1352、prompt14981/15032；旧日志缺finish_reason，不能断言额度耗尽。更早21:41等507明确Macprefill内存保护，与CAS独立。该批未见current_facts_stale。

确定bug为repair继承Kev选择状态：definitions=nil在下一物理请求被kevModelOptions重新绑定tools；repair实际tool_count37。实际HTTP Generate/Stream RED→GREEN验证。修复隔离Kev tools/persona、business policy和refresh，wire无tools/tool_choice/parallel，repair-only thinking=false，流式repair保持stream；原始task thinking/gates与Tool-only私聊不变，effect仅一次。

补ResponseMeta finish_reason/availableusage与typedempty model_output分类；bounded2MiB观察在既有queueRecv中，终态一次聚合不会覆盖正文，无新pump，取消/error原样保留；WithTools共享实际physical response metadata。termination事件仅counts/reason/usage，rawreasoning仍属Owner model-run。SDK独立usage帧前置聚合可能丢reasoning细分；无法获得时unknown/省略，不新增SSE parser或推断预算耗尽。

最终独立检查Go race1570PASS/511SKIP/0FAIL（含子用例），vet/build/gofmt/diff通过。实际HTTP/SSE工具隔离、stop/length/unknown、combined/split终态、nativeidentity和队列取消已运行。PG/其他service opt-in仍SKIP。无公共API/schema改动，无前端重跑，无migration，head0059。没有修改正式模型服务或设置、安装DB、写业务数据、提交/部署。507及真实模型提前停止的语义原因未被本修复证明解决；新诊断只能为后续真实失败提供终止依据。


## 2026-10-12 32K后原生回复遗漏与重试身份漂移

现场15:42privateRun turn_8d87... actualprompt15882/completion746/total16628、finishstop，正文47字但Tools0，conversation_native_output_missing；不再是16K length，15:36/15:19有真实成功记录。普通首轮必须保留相关读取/媒体Tool自由顺序；模型尝试final却仍未publication时，在既有queued native model边界允许最多一次原生输出纠正，exact conversation.reply工具选择，由模型生成真实ToolCall交原Eino执行。不得从draft text/分析构造Tool入参或后台代发，不新Agent/manual tool loop，不修reasoning-only/length/content_filter，不重放已提交effect；仍不调用时fail。此次新行为替换旧final-only立即失败的测试合同，最终DTO repair继续tool-free。

重试确定缺口：captureSenderTime冻结3字段，但syncServerTurnState重建RetryTurn漏senderTimezone/senderUtcOffsetMinutes/senderSentAt，handleTurn对同幂等消息snapshot.equal(existingTime)强校验导致409后恢复原failed。补原字段、不recapture，增加loading可见性和actualerrorcode。same turn/message/key preserved，no-mutation failure正常freshattempt路径保留；partialmutation防重放不放开。

修改边界：conversation native-publication策略及queued model correction和HTTP/SSE回归；Web retry store/View及交付回归；不写正式消息、不代点重试、不调整模型服务/生产配置、不修改budget。真实32K与应用128K预算仍需Owner同步，GPU400是独立资源失败。无需新migration；本地Go和Web/client typecheck/test/build按改动完整验证。


## 2026-10-12 32K后漏原生回复与重试无效

现场只读15:42私聊turn_8d87ca4a-7069-418c-9a71-9666786f34c3 actualprompt15882/completion746/total16628、finishstop，正文47字、ToolCalls0，conversation_native_output_missing。15:36/15:19有完成记录；15:31GPU400（15666token需1570MB仅537MB+106MB可回收）另属资源错误。此最新失败不是16K length；应用仍预算131072/40960，未代改Owner设置。

接受新协议：未publication且非空stop/unknown final，既有queued native boundary最多一次exact conversation.reply纠正，保留初始read/media顺序、模型自主Tool参数、真实Eino执行/Provider身份；不从正文代发，不修empty/length/filter，已回复不纠正，拒绝参数走既有native feedback，最后DTO repair仍无工具。实际HTTP/SSE RED→GREEN，模型忽略纠正/错误Tool均failclosed，已提交effect不重放。

重试确定代码缺口：首次captureSenderTime冻结字段，failed-history重建漏3字段，后端同message snapshot.equal核验会409。修复preserve timezone/offset/sentAt（含0/null语义），原user/key/turn/actor/attachments不变；在途按钮持续禁用可见，409保留拒绝+旧失败。History与enqueue partialmutation挡板一致，已有effect显示agent_partial_committed/不可直接重试，不再永远显示正在结算；无新public字段、无移除防重放。

最终独立核验：Go全仓race1593PASS/513SKIP/0FAIL（含子用例），Web83+browser-client20+core-client2=105PASS/0FAIL；generate/typecheck/test/build/vet/gofmt/diff通过。实际ChatView SSR和HTTP/SSE已跑，真实PGdurable retry/partialHistory因无隔离DB明确SKIP，真实Provider与生产点击重试未执行。无migration、head0059，未安装DB、写正式业务数据、提交或部署；生产GPU资源与预算同步不在本次写入。


## 2026-10-12 turn_1d71 审计与引用契约修复边界

只读持久诊断：17:01:44.125→17:09:08.592八轮，reply第一轮completed，actor.fact.record两次fabricated source_message_id失败，goal.evaluate把ctxref当goal_id并猜revision0失败，goal.inspect同样ref失败，list完整DB行返回rawGoalID/revision，后evaluate真实ID25提交queued，final rawID填goal_event_candidates.goal_ref直到settlement才报scope_invalid。FormalAgent完成但结算失败，已发送回复和排队请求不可回滚/重放。约7分24秒主要模型轮次，不能把工具数据库处理称分钟级。

已核实cr_226d1f25ffa9来自“2026-10-12上午问候/关心是否影响工作”旧summary，不能直接证明本轮退缩/间接承认；observed_fact/confirmed_fact语义标签混用。此为语义审计，不新增关键词判断或自动纠正关系事实。

修复范围：native Goal inspect(detail)/evaluate用当前goal_ref，由typed bridge绑定真实ID+冻结revision，canonical/directOwner rawAPI保留且CAS不放宽；模型Tool result仅已提供scope的语义DTO+refs，完整receipt内部保留，不静默从rawfinalID查DB授权。Actor已有当前inbound自动source binding，native schema不再索要不可得source_message_id并强制assertion_type，继续当前source author/invalidation验证和inference uncertain，不添加新历史来源写权限或新ContextReferenceKind。catalog/discovery返回同样native schema，全部物理续轮有效schema一致。

final goal_event_candidates共享scope/kind/count校验前移到final-contract，在单次无工具DTO repair中由模型改合法ref；invalid持续fail，已提交reply/evaluation一次且结算校验保留。提供明确字段/允许refs纠正反馈。必要提示词只改引用/推断说明，不堆叠强制情感/推进规则，不限制固定轮数或回复后自动结束。

只改Core协议/adapter/source validator/相关回归和必要现有夹具；不操作user-owned docs/audit_screenshots，不写正式业务数据、不部署、不安装DB、不新增迁移。测试要覆盖真实HTTP/Eino ref→canonical frozenID、queryDTO不含rawIDs、错误Goalref进入repair、Actor省略sourceID真实绑定+assertion/inference、cross-profile/stale/unknown隔离、持久reply+queued effect保留。真实PG无隔离DB诚实SKIP。


独立只读检查确认并由主线程补修两个缺口：参数提前拒绝缺少trace/result及原生ID冲突校验，现保留原模型参数身份、物理来源和非重试失败回执；同ID变参数拒绝，相同失败回执重放不增trace。native inspect保留cursor/include_closed和分页回传，避免当前offered超过10条时无法继续查询。绑定要求positive冻结revision，避免revision0查询绕过。旧重复Tool测试补真实物理模型身份和operationRoot，未放宽生产权限。

引用语义问题仅作审计保留：未引入语义裁判，也未改动业务提示词；不能宣称“退缩/间接承认”的事实判断已由本轮修复保证正确。全量最终验证结果见报告。

最终主线程全仓Go race1601PASS/514SKIP/0FAIL（含子用例），vet/build/gofmt/diff通过；实际HTTP/SSE引用绑定与tool-free repair已执行，PG明确SKIP。没有Web/publicAPI变更，未重跑前端或生成；未安装DB、写正式数据、提交/部署。报告同步docs/kev-integration-report.md，规范同步structured-turn-contract。


## 2026-10-12 最新私聊与 WakeUp 空 Provider 错误

只读现场21:13:30 turn_ae1bd803-843e-40d4-9cc7-35dd225aae13使用
LM Studio100.80.75.9:1234 cydonia-24b-v4.3-heretic-v2-i1，约70ms失败，
无Response/Tools，cause仅provider request failed。用户21:19:04服务日志确认
收到原生请求（max_completion_tokens40960）；用户加载32k，预算不匹配
不能直接认定为这次拒绝原因。此前11234 refused/reset属于另一服务。

研究固定版本源码并用内存RoundTripper复现：SDK遇到缺失/错误类型message
的error对象，RequestError保存真实HTTP状态/body，但内层半成品APIError为空；
Eino model/openai0.1.13 errors.As转换选择内层APIError，丢失外层全部详情。
Generate/初始Stream均可精确生成空cause。正式原始响应已不可从旧诊断恢复，
不将本地HTTP400当成真实LMStudio状态或拒绝原因。

仅RoundTripper+新helper/tests修复：实际HTTP>=400在SDK之前变Core typed
HTTP错误；最多读4097bytes、显示4096、关闭body一次，脱敏凭据与图片，
保留readErr Unwrap。成功/重定向/原transport错误透明，无重试/配置变化。
独立核验提出3xx重定向和truncated escaped credential两个缺口，主线程
已修正并加回归。真正SDK/Eino→queued→ADK RED→GREEN，失败仅一次请求、
无fake final，正常JSON/SSE透明。最终检查结果另附。

Worker日志WakeUp431在13:20UTC失败后下一activity cancelled，源码可解释为
failed周期保存并结束intent后，current-check将完成intent误标superseded_by_cognition；
该顺序可独立产生cancelled，并不证明用户取消/新认知。未查真实DB/history，
故这是源码支持路径，不将其写作生产已证实唯一原因。防effect重放原意保留，
本轮未修改取消语义或失败周期重试策略。

无生产模型调用/数据写入/预算配置改动/部署/数据库安装/迁移或提交。

最终主线程验证：Go全仓race1629PASS/514SKIP/0FAIL（含子用例），vet/build/gofmt/diff通过；日志/tmp/provider-http-final-race.jsonl、/tmp/provider-http-final-race.stderr、/tmp/provider-http-final-vet.log、/tmp/provider-http-final-build.log。PG与其他opt-in514项明确SKIP，无Web/API变更所以未重跑前端。最终实际HTTP错误保留与重定向/escaped-secret边界已测，正式拒绝原因仍未知，未提交或部署。


## 2026-10-12 LM Studio HTTP400 无参工具 parameters.properties 修复

用户Worker日志13:44:18UTC（21:44:18CST）提供真实HTTP400：
path=[36,function,parameters,properties]，expectedobject/receivedundefined。
同轮请求tools[36]为capability.catalog。来源objectSchema(nil,nil,false)
生成properties:null；Eino typedjsonschema.Properties指针omitempty随后省略字段。
这次拒绝原因已确认是工具参数schema缺失，不能归因32k预算。

最小产品补丁仅provider_schemas.go：nil properties归一化非nil空map，
发送properties:{}，保持additionalProperties:false及required/权限/选择。
新增no_arg_tool_schema_test.go，固定SDK/Eino queuedGenerate/Stream实际
HTTP请求RED→GREEN：48注册roots、38安装目录fallback（disabled/version/settings）、
2→3工具discovery后续轮次；catalog{}允许、额外参数拒绝、catalogschema一致、
原定义无变异。没有移除catalog或修改Kev候选/白名单。

正式只读Kev记录21:44:17–18配置version6显示33可选工具均unavailable/request_failed，
use_original/original_used；33optional+3mandatory+2utility=38。用户决定Kev稍后自行测试，
本轮不继续排查或调整服务。当前设置version7不能反推旧版本endpoint。

独立只读检查无findings；完整Go race1642PASS/513SKIP/0FAIL（含子用例），
vet/build/gofmt/diff通过。证据/tmp/no-arg-check-race-unsandboxed.jsonl、
/tmp/no-arg-check-race-unsandboxed.stderr、/tmp/no-arg-check-vet.log、
/tmp/no-arg-check-build.log；RED证据/tmp/no-arg-tool-red.log。
服务/PG/真实Provider显式SKIP，不冒称正式已恢复。无公共API/Web变更，无前端重跑；
未写正式数据、改设置/预算、部署、安装DB、增加migration或提交。
