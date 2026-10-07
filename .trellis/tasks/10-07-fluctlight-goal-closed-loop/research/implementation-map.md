# G00 初步实现映射

2026-10-07，master / ee1e468，初始工作区干净。三路只读检索并抽查关键代码；除基线测试外均为静态分析，G00 仍需实施前补故障复现。

## 已有底座与缺口
| 范围 | 可复用/缺口 | 精确入口 |
| --- | --- | --- |
| G01 | 已有 lifecycle CAS、typed trigger、Tool 授权；缺统一关联 Goal 事务许可，领域 Freeze 未接生产 | goal_intention.go:148/634/658；intention_runtime.go:93；scheduled_intention_runtime.go:83；tool_execution.go:172；life_activity_capabilities.go:180 |
| G02 | current_attempt_id、终态结算、稳定 operation ID/异步恢复可复用；Attempt 到终态才 insert；首 Tool 前失败只标 inbox failed；任意 invocation 就 pending | evolution_persistence.go:114/155；agent_result_adapter.go:497/1066/1104；action_outcome.go:280；workflow/workflow.go:1201/1445 |
| G03 | revision 与 criteria index 校验已有；Reflection 先按旧标准完成，再覆盖新标准；缺稳定标准 ID/版本/逐项状态 | goal_intention.go:42/237；reflection_v2.go:75；reflection_domains_v2.go:63–87；provider_schemas.go:381 |
| G04 | shared NULL profile scope 已有；Stage 仅 read model，无 GoalStage/GoalCommitment aggregate | goal_execution.go:11；goal_intention.go:298；autonomy.go:39 |
| G05 | 正式消息/turn fact/Outcome/库存 ID 已有；conversation settlement 未带 goal/intention causality；缺无绑定事件候选评估 | agent_result_adapter.go:182/539/593/613；tool_publication.go:64/176；cognition.go:387/1008 |
| G06 | 单 SQL writer、outbox、workflow intents、CAS 可复用；缺 durable Evaluation/统一语义提交；Goal revision 无独立 outbox | evolution_persistence.go:12–57；outbox.go:8；life_activity_resolution.go:130/202/220；reflection_domains_v2.go:14 |
| G07 | intention.inspect/decide/schedule、正式 Task/Agent registry 已有；缺受触发约束的当前阶段规划/等待/Resolution | builtin_capabilities.go:138；intention_capabilities.go:33/143；formal_agents.go:61 |
| G08 | Daily Review 本地日/锁/stable fact、Reflection 水位已有；缺 GoalReview/停滞原因/策略/soft-hard deadline | agent_result_adapter.go:1189/1197/1224/1298；reflection_runtime_v2.go:136；provider_schemas.go:263 |
| G09 | ContextProjection/Runtime Facts/预算已有；execution 只读 activity、running/in_progress 映射错误；DailyReview/Reflection Goal 可被淘汰；无 Goal Tool | goal_execution.go:13/35；intelligence.go:255/397；provider_context.go:189/193/523/887；ai/prompt/slots.go:18 |
| G10 | Owner session/CSRF/归属/CAS/store reload 可复用；缺目标 API/client/UI/分页历史；actor_id 固定实例；终态计入活跃数 | httpapi/server.go:90；httpapi/browser/routes.go:601/988/1042；repository.go:126；detail.go:276；evolution_persistence.go:56；InstanceDetailsDialog.vue:137/473/496；GovernanceView.vue:126；control-center.ts:457 |
| G11 | embedded linear migration/Runner 单事务锁可复用；head 0049，首增量预计 0050；缺存量标准回填/审核、Attempt 对账 | migrations/runner.go:15/49；cmd/migrate/main.go:13 |
| G12 | isolated helper 随机 DB、native Agent/Tool 测试已有；新增缺口待回归；CI validation 当前禁用，不能作通过证据 | project_health_transaction_integration_test.go:84；httpapi/server_test.go:311；.github/workflows/fluctlight-ci.yml:23/26/55 |
| G13 | 旧报告格式参考；本任务最终报告待实现后生成 | docs/fluctlight-goal-life-consistency-report.md |

无另行说明的 Go 入口相对 apps/core-go/internal/core；workflow/HTTP 路径相对 apps/core-go/internal；Vue/store 相对 apps/web/src。实施前主代理完整读即将修改的代码。

## 当前枚举、权威与事务
Goal: candidate/active/paused/completed/abandoned/cancelled。Intention: candidate/qualified/due/in_progress/paused/completed/expired/cancelled。Attempt 只有 succeeded/failed/cancelled/suppressed；执行期只有 current_attempt_id。Outcome 有 pending/终态。Goal SQL 写集中 persistGoalAuthorityTx，但 GoalComplete/ApplyGoalProgress/购物/Reflection 的完成编排未唯一。
Tool effect/receipt/outbox 同短事务，Provider 事务外，mutation identity=实例+capability+operation_id；查询新鲜读。conversation.turn 与用户消息同事务，assistant PublishConversationReplyTx 幂等发布。Goal revision 无 outbox。平台 workflow intents/Worker 与 outbox 可接评估，Redis 非权威。

## 可复用测试
TestGoalIntentionStageS07；TestIntentionTriggerProductionFlowCreatesDueFactAndSettlesFromOutcome；TestFormalDueNativeAgentStartsActivityAndResultSettlesAttempt；TestFormalDueStartSurvivesAgentFailureAndSettlesOnce；TestFormalDueActivityResultPreservesPausedOrCancelledIntention；TestFormalDueActivityResultCannotSettleSupersededAttempt；TestDueIntentionActivityResultSettlesFrozenAttempt；TestProcessReflectionV2AdvancesGoalOnlyFromBoundCompletedOutcome；TestScheduledAcquisitionGoalClockIndependentWearAndFinalWorkflow；TestInitializedSharedRelationshipGoalCanCreateAndPauseNextStepWithoutDuplicatingGoal；TestConcurrentDailyReviewUsesOneMainProviderCall；TestBrowserScheduleWriteCannotCreateExecutableActionLink。

## 实际证据与安全环境
主代理复跑 go test -json 领域 S07 和五个 workflow timer/deferral/retry-budget 测试；命令/逐场景记录 baseline-tests.json，日志 baseline-1.jsonl/baseline-2.jsonl。4 个领域叶子场景、5 个 workflow 用例 PASS，父级 PASS 不重复计数。未验证数据库。探子另报若干数据库命令父级 ok，但无子场景 SKIP 记录，故不纳入通过证据。
docker info --format '{{.ServerVersion}}' 返回无法连接 /Users/vinson/.orbstack/run/docker.sock。建议一次性 pgvector/pgvector:pg16、随机本机端口/凭据、tmpfs，只对该集群设置 GO_CORE_TEST_DATABASE_URL；helper 创建/删除随机库，需要 CREATE/DROP DATABASE，禁止连开发/生产库。
Go 验证：模块内 go test -json ./...、go test -race ./internal/core ./internal/workflow、go vet ./...、go build ./...。Web/client 验证：根目录 pnpm --filter @fluctlight/browser-client generate/typecheck/test；pnpm --filter @fluctlight/web lint/typecheck/test/build；infra/acceptance/check-core-openapi.sh。真实 Provider/媒体另验，不混计。
