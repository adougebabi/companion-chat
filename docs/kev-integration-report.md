> 2026-10-10 最新交付：schema head 为 `0058_semantic_fact_generation`，须先迁移再升级 Core/Worker。WakeUp 固定协议、历史输入和真实动作已重写；embedding 缓存不再推进事实版本。最新证据见文末。

> 2026-10-09 正式反馈修复已追加：当前 schema head 为 `0057_logical_agent_leases`。部署请先迁移并同时升级 Core/Worker；设置和诊断路由已修复。下面首次交付证据保留，最新结果见文末。

# Kev 主动接入交付报告

日期：2026-10-09。开发分支：`codex/kev-decision-integration`。基线：`63be0df`。

本轮完成七个决策点的代码接入、动态设置、独立能力发现、可靠逐题记录、诊断界面和本地验证。按用户最后确认“本地代码过了就行，我去正式环境验收”，正式环境迁移、真实 Kev 推理、运行效果及浏览器真实服务验收由用户执行；本报告不把受控模型/跳过测试当作真实模型通过。

## 交付与入口

| 决策点 | 实际入口与共用边界 |
| --- | --- |
| runtime.wakeup | ProcessWakeUp 硬资格/睡眠后、EnsureDirectConversation 前；原 WakeUp Agent 执行体共用；durable 延后保留 cycle；人工执行可打断延后 |
| runtime.reflection | ProcessReflection 非空真实证据后；no 释放 window lease、不推进 watermark；原 ReflectionProposalTask 共用 |
| goal.completion_check | Goal eligibility/memo 后逐 Goal gate；保留 deferred remainder 和 sources；原 GoalEvaluationTask/commit 共用；人工复核旁路 |
| goal.replenish_plan | Planner claim/capacity/policy 后；Owner 请求旁路；原 GoalPlannerAgent/commit 共用；独立 deferral 不耗执行重试 |
| tools.select | Prompt assembly 中形成 request-local visible set；原授权 executor catalog 保留；每次 Generate/Stream 的实际 schema 与预算对齐 |
| persona.switch | 原已安装 persona.switch 且有 refresh 的 Agent 中，完整 A 响应释放前 gate；原 ExecuteTool/persona domain 提交；整批 A 丢弃后 B 生成 |
| context.select | WorkingMemory resolver 前，仅 optional fragments；required facts、活动目标目录及协议历史保留；原预算/总结共用 |

`internal/ai/decision` 使用 `POST /v1/systemone`。合法 choice 默认直接采用；unclear/异常/预算耗尽回原路径，父请求取消不启动 fallback。旧模型不为 fallback 提前执行。配置撤销后仍保存原始 yes/no 和概率，将实际应用记录为 original/discarded。

新增独立 Tool `capability.discover`：当前 Agent 内只公开其原安装集合，先校验整次 load 再扩张；独立调用报告授权 catalog，不声称修改不存在的 Agent。补载后的 Tool schema 在下轮真实 HTTP 请求增长，保留 Eino v0.7.37 与原生 Runner。

人格接管的内层模型回调曾让丢弃 A 的调用进入运行结果。现已只对 gated proposal 的内层 callback 做隔离；外层原生 loop 收到 B，A 的原输出仍保留在 Owner 模型诊断中。Generate/Stream 回归均断言 A 不执行且不进入结果 ToolCalls。

## 设置和诊断操作

设置 → Kev 决策，可保存全局/七点开关、System One endpoint、超时、阶段预算、choice/guarded 策略、最大延后和保留天数。凭据只写入加密 `setting_secrets`，留空不覆盖已有值。显示已保存配置版本；动态修改不需要重新编译前端。

诊断中心 → Kev 决策，可过滤 Actor、Agent、决策点、策略、调用/应用状态和时间。每个 question 保留 request ID、raw response/answer、完整分布、策略、配置/状态版本及实际动作关联。列表按 `started_at DESC,id DESC` 排序，snapshot cursor 分页；显示选定时区。导出使用同一过滤和 snapshot，超过 10000 条时明确 `complete=false` 并保留 next_cursor。

手动“测试连接”只执行协议问题，不执行 Agent 或业务动作；即使全局关闭也能由 Owner 显式测试。正常关闭状态不发 Kev 健康探测或 shadow 请求。

关闭全局：取消“启用 Kev 判定”并保存。单点关闭：只取消对应点并保存。已经发送的请求/已提交事实不回滚、不重放。可选上下文与工具集由原来源恢复。

## 迁移与持久日志

Schema head：`0056_kev_decisions`，支持从 `0055_goal_planner_cadence` 增量升级，新增 `kev_decisions`、`kev_deferrals` 与 settings revision。迁移不覆盖已有 Owner 的 Kev 设置，也不启用未知生产地址。

DB 审计写入失败时，先落 bounded fsync journal，再采用结果；两种存储都失败则 original + 告警。Core/Worker 分别使用 `fluctlight_kev_core` / `fluctlight_kev_worker` named volumes。Core 每 30 秒维护自身 journal，Worker 复用已有维护 tick。跨进程文件锁、尾部半条写入隔离、按 recorded_at 幂等导入防止旧记录覆盖新应用结果；导入不重放业务动作。活动 journal 限制 32 MiB，最多另保留一份 bounded incomplete-tail 隔离文件。

## 正式环境启用

先使用包含本次代码的新 Core/Worker/Web 镜像，并按现有备份流程保护数据库。执行项目的显式 migrate 服务，确认 head 为 0056；再启用配置。不要把 Mac 的 loopback 当成 NAS/容器的 Kev 地址。

已有 Compose 私有环境配置时，可运行：

```sh
docker compose --env-file infra/compose/fluctlight.env -f infra/compose/fluctlight.compose.yml run --rm migrate

docker compose --env-file infra/compose/fluctlight.env -f infra/compose/fluctlight.compose.yml run --rm core /usr/local/bin/fluctlight-kev-config-go --endpoint http://MAC_LAN_ADDRESS:8010/v1/systemone
```

将 MAC_LAN_ADDRESS 替换为 Core/Worker 实际可达、受访问控制的地址。第二条是实际写入原 settings 服务的配置命令，会启用全局与七点，使用 choice_argmax；没有修改生产配置的自动启动脚本。配置凭据使用 Owner 设置入口。CLI 使用现有部署环境变量，不读取额外云密钥，不下载/升级权重。

同主机原生 Go 进程可以在对应环境变量就绪后运行：

```sh
go -C apps/core-go run ./cmd/kev-config --endpoint http://127.0.0.1:8010/v1/systemone
```

一键关闭也可在同一命令加 `--disable`；endpoint 仍需明确提供。命令已纳入 Core Docker 镜像构建。

正式验收建议依次检查：

1. 关闭全局，确认业务仍能运行、Kev HTTP 为零；逐项关闭只改变对应点。
2. 测试连接，核实真实 runtime/模型 revision；当前配置 revision 未知时记录 unknown。
3. 隔离测试 Actor 上覆盖自动运行、Tool 选择/发现、人格接管、上下文筛选四类 active 行为；从诊断中同时核对模型判断和真实动作。
4. 查看连续 no 的延后恢复、人工立即执行、目标证据/容量、开关中途关闭与上下文恢复。
5. 对隔离测试存储注入诊断故障，核实 spool/恢复和未留痕不采用；避免对真实用户发送消息、支付或发布动态。

项目现有独立 Tool / Agent 验收命令可按已有部署参数使用 `infra/acceptance/run-go-live-provider-smoke.sh --suite all`。它是既有 Provider/Tool 验收，不等同于七点真实 Kev 四场景验收。

## 本地验证证据

| 命令 | 结果 | exit |
| --- | --- | --- |
| go -C apps/core-go test -race -json ./... -count=1 | 1363 个 PASS、489 个 SKIP、0 FAIL（含子用例事件） | 0 |
| go -C apps/core-go vet ./... | 通过 | 0 |
| go -C apps/core-go build ./... | 全命令编译通过，含 kev-config | 0 |
| pnpm generate | Core/browser OpenAPI 与客户端同步 | 0 |
| pnpm typecheck | 三个工作区包通过 | 0 |
| pnpm test | Browser client 20 + Core client 2 + Web 72，共 94 PASS、0 FAIL | 0 |
| pnpm build | Web production build 通过 | 0 |
| git diff --check | 通过 | 0 |

Go SKIP 来自未配置隔离 PostgreSQL/真实 Provider 等 opt-in 环境，不能算验证通过。受控 HTTP/SSE 测试走项目锁定 OpenAI component、生产模型适配器和原生 Runner；它们验证工程分支/协议，不证明真实 Kev 的判断正确率。

前端 build 有 minified chunk 超过 500 kB 的提示，不影响构建退出码；本轮没有扩展为打包优化任务。

## 未执行与实测限制

- 未执行正式 PostgreSQL empty/0055→0056 升级、真实数据库业务分支、Redis/Temporal/Compose 联合环境验收。
- 未调用真实 Kev，也未核验 NAS/Core/Worker 到 Mac 的连通性、已测权重 revision 或真实模型业务效果。
- Downloads 定向搜索未发现完整 kev_fluctlight_results(1).json / benchmark；未使用完整 82 用例回放，也没有修改其已知误判 expected。
- 未在正式服务上进行浏览器/移动视口验收。
- 并发配置限制同一个 DecisionService 的在途调用，不宣称它是多个进程/实例的 Kev 服务总并发限额。

以上由用户按本轮验收约定在正式环境检查。当前交付结论是代码实现和本地检查通过；真实模型行为与业务效果仍待正式验收。

## 正式反馈修复：设置入口、调用顺序和 Goal Evaluation 频率

基线 c4cd44e，分支 codex/kev-runtime-recovery。

1. Kev 设置/诊断不能进入：App.vue 两组手写白名单未包含 kev/kev-decisions。已改为从导航声明派生 guard，URL、刷新、侧栏选择共用。4 个实际 App parser/computed 回归先失败、修复后通过。
2. 多轮调用交错：物理请求队列原本每次 Generate/Stream 释放，因此 Goal Evaluation 能在同一摇光的 WakeUp/cognition 轮次间进入。新增 PostgreSQL logical_agent_leases，从快照/claim 前保护到最终提交。租约 3 分钟、15 秒心跳、失效取消、token 提交 fence 和拥有者限定释放；同一摇光的嵌套 Agent 继承租约。不同摇光仍可并行，物理队列仍逐次释放，避免 Tool 内部调用模型自锁。current_facts_stale 的真实事实校验保留。
3. 高频：enqueueTurnGoalCandidatesTx 原本即使没有插入任何新 evidence link 也创建评估。现仅新增 committed link 的 Goal 入队；真实 outcome 显式关联 Goal，source remainder 只查看 active/paused 关联来源/目标版本，避免无关待处理来源驱动无限续排。重复 link/no message 不再入队。goal.evaluate 的模型说明明确只因新相关证据/标准变化请求，并禁止在当前 Agent 内轮询等待自己的后台评估。

当前节奏仍为事件驱动：pending 合并窗口 2 秒，非 terminal 失败 1 分钟退避，最多 5 次评估尝试。没有一个“每 N 分钟检查所有目标”的固定频率。原 deferred 缺 not_before 时工作流每 30 秒查看；现在 retry 返回实际 available_at，不在 1 分钟退避期间无效轮询。因忙碌逻辑运行等待时还未 claim，不消耗评估尝试。保留真实新证据、Owner 强制复核、到期 review、目标标准/权威变化。

最新本地结果：Go test -race ./... 1370 PASS、490 SKIP、0 FAIL（含子用例）；Web/client 98 PASS；typecheck、vet、build、production build 和 diff check 全通过。TestPostgresLogicalAgentRunCoordinatesIndependentApps 使用任务隔离数据库验收，当前因未配置 GO_CORE_TEST_DATABASE_URL 跳过，不能算真实数据库验证通过。

上线须先执行包含本次代码的 migrate 服务，确认 0057_logical_agent_leases，再同时升级 Core 与 Worker；仅更新 Web 无法修复跨进程逻辑协调。操作命令沿用上文既有 Compose 方式。人工/外部事实在运行期间发生真实变化，原 CAS 仍会拒绝过期结果；本次消除的是这几类同一摇光逻辑运行相互交错造成的冲突。


## Goal Evaluation 完成未落库反馈修复（2026-10-09）

用户提供的 response 中 goal:2/3/4 的 review 均引用 stage:1.1；goal:2 在描述无现有阶段时还使用 adjust/object_ref。引用按目标编号绑定，旧全局 enum 会放行跨目标选择，hydration 随后拒绝整批，导致 goal:1 的 completed 也未提交。用户看到物理 response，没有领域提交错误展示；未查询正式数据库，不将推断错误码当作生产实测。

本轮 schema 改为按 Goal 判别并限制对象/标准所有权及 create/adjust 操作；wire/coverage 可纠正一次完整输出，替换仍校验；wire 与耗尽 final-contract 错误 terminal。诊断独立展示目标评估提交状态/error/result，成功结果包含实际 goal_outcomes。原批次原子性、CAS、证据与 paused settlement 规则保留；无新 migration，head 仍 0057_logical_agent_leases。

本地最终验证：Go race 1374 PASS、490 SKIP、0 FAIL（含子用例）；Go vet/build、pnpm generate/typecheck/test/build、git diff --check 通过；Web/client 共 98 PASS。独立只读核验无 findings。SKIP/真实数据库/真实模型/正式部署仍由用户验收。当前 codex/goal-evaluation-output-recovery 未提交。

正式验收：升级 Core/Worker/Web 后，在诊断打开 Goal Evaluation，分别核对模型 response 与“目标评估提交”。合法输出成功提交时，在 result.goal_outcomes 查看实际 status；引用错误经纠正仍无效时应显示明确失败/error_code。使用新的相关证据或 Owner 明确复核重新触发历史失败目标；本轮未手工改写正式目标状态。


## 部署环境继续失败：覆盖数量与 thinking（2026-10-09）

用户授权浏览器检查环境。实际诊断中 goal_review_event_120 和 goal_review_event_122 提交 failed/goal_assessment_coverage_missing；四目标输入只返回 goal:1，纠正后仍缺其余目标。确认已上线提交诊断，并非仅显示未刷新。Reasoning sidecar较长且中断，但没有 finish_reason/usage，不宣称已证实 token 耗尽。

补修：evaluations minItems/maxItems 等于 offered count，plans 最大数量同 count；Goal Evaluation 关闭 thinking，实际 direct/ADK HTTP 明确发送 enable_thinking=false，避免省略字段导致服务器默认启用。保留总 output reserve、其他 Agent策略、所有权/CAS/批次事务；无 migration。Schema 数量回归 RED→GREEN，HTTP/ADK 修复回归通过。生产入口 thinking 断言补入数据库集成测试，未配置隔离数据库时仍 SKIP。

本地 Go race 1378 PASS/490 SKIP/0 FAIL（含子用例），vet/build/typecheck 通过；正式状态仅只读检查，未部署、未写业务数据。最新补修未提交。

补修最终检查：Web/client 98 PASS，production build 通过；git diff --check 通过。独立核验未发现产品正确性缺陷，提出 production call-site 测试缺口，已在现有 PostgreSQL 集成测试补断言（本机 SKIP）。


## 2026-10-10 Goal 完成判定与 Kev 请求负载修复

用户授权两项一起处理。Goal wire 增加 criterion_quote/optional_improvement，负面缺口须来自冻结原标准；父 Goal/Stage/Commitment 纯 preflight 与事务复用语义校验，完成标志双向一致，判断错误使用既有一次完整替换纠正。policy 升为 goal.evaluation.v3，旧 memo 在下次评估不再复用。实际作者陈述与 quotation 区分；active 与 paused 规则保留。字面引用校验不能证明任意模型语义正确，不把 mock 说成真实模型验收。

Kev context/tool 每个 batch 只带当前候选内容，保留当前任务输入和协议；新增明确批次 state builder，原 Decide 兼容。共享 stage、Service budget、单请求和父取消有不同原因，覆盖 HTTP headers/response body 等待。没有改 timeout 默认值、正式配置、全局 token 预算或推理服务；逐 Goal completion gate 仍为原逐项请求。无法承诺所有请求<1秒，剩余大 currentInput/服务排队须实际测量。

最终产品代码无公共API/迁移变更，head仍0057_logical_agent_leases。本地 Go race 1396 PASS/490 SKIP/0 FAIL（含子用例），Web/client98 PASS，generate/typecheck/vet/build/diff通过。独立核验指出主线程新增测试身份缺失，已补完整身份并实际断言active completed/progress1及paused保持。未操作正式业务数据、未部署、未提交。

正式验收：升级 Core/Worker 后对原推书目标发起一次 Owner 复核，检查 goal.evaluation.v3、提交 succeeded、goal_outcomes 实际状态。负面判断的 criterion_quote 与可选改进可在物理模型 response 查看。Kev 对超过8题请求检查每批 state候选数与题数一致，按唯一request_id比较实际usage/duration；同一请求逐题诊断行不是多次HTTP。若只剩真实原标准却仍被误判，保留该实测错例继续语义验收；本轮不能承诺字面quote就能消除全部误判。


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

上线验收：按既有部署迁移服务执行至0058，确认ledger后升级Core/Worker。Wake模型Prompt应只有一份专用固定协议、清晰LF边界和historical_conversation记录；周期false不得自动重放旧问题。查真实Tool receipts与action一致，duplicate_suppressed不算新发送。Embedding重试期间的新settlement冲突不应仅由cache状态引起；真实用户/记忆/状态变更仍允许current_facts_stale。发生新冲突保留expected/actual、边界和correlation，继续定位其他实际写入。无需手工改目标/事实generation。
