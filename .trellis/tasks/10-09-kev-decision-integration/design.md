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
