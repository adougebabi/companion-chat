# Kev 入口核验（2026-10-09）

基线：master / 63be0df。三个只读探子分别核验调度与目标、Agent/人格/上下文、配置与诊断；主线程点验 loop.go、eino_model_runtime.go、personality_runtime.go、persona_action_service.go、cognition_agent.go 及本地依赖源码。技术设计由主线程负责。

## 七点真实入口

| 点 | 候选/资格 | 原执行或装配 | 接入及回退边界 |
| --- | --- | --- | --- |
| runtime.wakeup | redis_triggers.go:322 ReleaseDueWakeUpIntents；agent_result_adapter.go:740 ProcessWakeUp，827 睡眠检查 | agent_result_adapter.go:862 RunFormalAgent WakeUp | 拆 deterministic preflight；在 EnsureDirectConversation（814）前判断；no 保留同一 cycle，由 workflow.go:260 WakeUpWorkflow durable timer 恢复 |
| runtime.reflection | workflow_ops.go:521 ProcessReflection；564 claimReflectionWindow；573 真实证据；reflection_window_v2.go:21 lease | reflection_runtime_v2.go:172 RunReflectionProposalTask | 非空真实证据后、完整 projection 前；no 复用 setReflectionWindowIdle（62），不推进 watermark；workflow.go:629 durable 延后 |
| goal.completion_check | goal_sources.go:20 queueGoalEvaluationTx；goal_evaluation_runtime.go:41 claim；goal_assessment_memo.go:263 eligibility | goal_evaluation_runtime.go:354 RunGoalEvaluationTask；363 commit | 337-347 eligible 子集处逐 Goal gate；retry/available_at 保留 source；Kev 延后不消耗 attempt_count；workflow.go:649 已支持 deferred |
| goal.replenish_plan | goal_set.go:634 requestGoalPlanningTx；goal_planner.go:342 ProcessGoalPlanningIntent，421 policy/capacity | goal_planner.go:462 RunGoalPlannerAgent；501 event 消费 | 452 claim 后、462 执行前；manual owner_request 旁路；独立 deferral_count；workflow.go:679 已支持 deferred |
| tools.select | capability/registry.go:115 Catalog(surface)；cognition_agent.go:83 候选 | ai/agent/loop.go:247 RunADKLoop；core/eino_model_runtime.go:668 generateWithADK | 原始授权集与 run-local 可见集分离；注册完整执行对象；物理请求发送可见 schema；新增真实 discovery/load capability |
| persona.switch | personality_runtime.go:83 preparePersonalityDecision：规则、冷却、profile；声明来源，不猜人格 | persona_action_service.go:201 applyPersonaActionTx；235 CAS 写入 | batch 放行前判断；经同一 ExecuteTool/领域事务提交；拒绝旧 A 未执行 batch，刷新 B； no 屏蔽后续同规则切换，返回真实业务拒绝 |
| context.select | provider_context.go:185 workingMemoryInputFromProjectionForSurface；216 required classification | provider_context.go:121 ResolveWorkingMemory；prompt_context_assembler.go:296 whole-wire budget | 只筛可选 fragments；required 和活动目标目录保留；原 projection/index 不变；关闭重建原装配 |

表内 Go 相对路径以 apps/core-go/internal/core 为根；workflow.go 属于 internal/workflow，ai/agent/loop.go 属于 internal/ai/agent。

## 共用基础

- settings.go:20/59 ReadSettings/UpdateSettings，107 允许键，123 upsert，252 AES-GCM；runtime_settings / setting_secrets（migrations/runner.go:307）。当前无配置 revision。
- diagnostics.go:667 RecordDiagnosticEvent 为 best-effort；679 persistDiagnosticEvent 返回错误；515 PersistModelRunLifecycle 是可靠写入参考。Kev 采用结果需要独立可靠记录，不改变既有诊断 best-effort 契约。
- operations.go:1505 查询、1560 filter、2043 export；diagnostic_page.go:13 snapshot cursor；现有 Owner-only 权限。
- migrations/runner.go:15 head=0055_goal_planner_cadence，增量 migration 需同时进入 empty/head/upgrade 分支。
- browser/routes.go:299 settings，328 diagnostics；browser_backend.go:164 分派；packages/browser-client/scripts/generate-openapi.mjs；生成物须通过 pnpm generate 同步。
- apps/web/src/views/SettingsView.vue:60/215；DiagnosticsView.vue:187 timezone、214 raw 展示；stores/control-center.ts:190 loader、978 settings 保存；app/navigation.ts:3/15 sections。
- worker/main.go:140 retention 当前固定 30 天/10000；operations.go:2128 prune；Kev 新增自己的 7 天显式策略和有界批量删除，避免扩大本次范围去重建所有既有诊断。
- Compose core/worker 不存在持久 decision spool 卷；须添加有界持久卷及幂等导入，不使用内存替代。

## 框架兼容核验及纠正

锁定 Eino v0.7.37、OpenAI component v0.1.13、ACL OpenAI v0.1.17。旧版没有 BeforeModelRewriteState/ToolInfos，但不能据此断言动态 schema 不可实现。

主线程完整读取 components/model/option.go：旧版本 model.WithTools 是调用参数；ACL openai/chat_model.go:560/640 用 options.Tools 覆盖固定工具并同步 callback。react.go:249 固定 executor table 不阻止 model adapter 每次请求使用可见子集。

隔离试验：research/eino-v0737-compatibility-test.go.txt，使用原生 adk.ChatModelAgent/Runner + 项目锁定 OpenAI component + 本地假 HTTP 服务。测试结果：

```
TestNativeLoopCallTimeToolsGrow PASS
wire schema: [[discover] [discover hidden] [discover hidden]]
actual tool executions: 2
TestNativeBatchGateBlocksAllTools PASS
AfterChatModel returns gate error: actual tool executions 0
```

因此采用现有模型适配层的 request model.WithTools 与完整授权 executor catalog，不升级框架、不创建另一套 loop。试验仅证明 Generate 下实际 wire schema 增长和错误拦截；未证明生产 persona 再生成、SSE 流式、配置中途关闭等。T5/T7 必须进一步覆盖，不把试验当作工程接入完成。

Context7 已先 library 再 docs 查询 /cloudwego/eino，官方当前文档证实调用选项和不可变 WithTools，但不提供 v0.7.37 索引；版本结论以本地锁定源码和试验为准。查询 URL：https://github.com/cloudwego/eino/blob/main/_autodocs/api-reference/component-model.md 。新版 middleware 文档仅作参考。

## 规划期基线与限制

- go -C apps/core-go test ./internal/ai/agent ./internal/workflow -count=1：两个 package PASS，exit 0。
- 首次默认沙箱试验无法监听本地端口；首次 workflow 编译缓存受限。已在批准的沙箱外重跑并通过，属于环境权限问题，不是产品失败。
- pnpm typecheck：执行结果见 baseline.md。
- rg --files /Users/vinson/Downloads -g '*kev*' -g '*benchmark*'：无匹配。仅此目录范围，不宣称所有磁盘均无完整回放文件。
- 尚未调用真实 Kev、读取/修改生产配置、连接共享数据库或进行部署。
- docs/persona-takeover-architecture.md 和 docs/capability-architecture.md 包含早期 winner/frozen 设计；最新 structured-turn-contract.md / cognitive-runtime 明确已替代。不得因旧说明恢复第二套 loop/延迟 Tool dispatcher。
