# Agent / Tool 验收矩阵

状态：实现与非真实模型检查已收敛，**完整双真实 E2E 未通过验收，等待用户串行实测**。用户明确要求资源优先、LLM 一个个运行；本轮没有继续生成式 live 请求。受控模型证据不能替代真实 Agent 行。

## 受测版本与共用证据

- HEAD：`32276a1c20ad68462b966c45c16ab6126230b297`；分支 `codex/agent-tool-plugin-e2e`，工作树未提交。
- 全部 Go 源文件聚合 SHA-256：`5fc22c87583cd99e240835ad12e5cfb0dec52762117f9b3b05dbc011ccecb831`；逐文件哈希、命令和退出码见各 `meta.json`。
- `research/runs/final-source-go-db`：退出 0，1217 passing test events，0 fail；25 skip 事件为无测试包和 opt-in live 用例，单列，不能用作双 E2E PASS。
- `research/runs/final-source-race`：退出 0，93 passing events，0 skip；覆盖原生循环、Tool adapter、故障注入、人格、恢复、授权及预算。
- `research/runs/final-source-vet`、`final-source-build`：退出 0。
- `research/frontend-and-runner-checks.md`：前端 typecheck/test/build 通过；最终 runner/verifier 17 个测试通过，另补 scope-label 检查。
- 所有历史失败尝试保留，包括 `integration-go-db-02`、`adapters-full-failures-01` 和旧 live 请求；不以成功记录覆盖失败。

## 全部 18 个业务 Tool

每行的 Eino 适配证据是固定基线 `TestFormalToolEinoAdapterE2E/<名称>`，包含：无 Agent 的正式直接执行、独立 SQL/产物断言、真实原生 ToolCall 与物理 request ID、下一模型输入中的准确回执、重复语义；`dependency_failure` 和 `foreign_owner_rejected` 子项验证缺失业务依赖与真实资源所有权拒绝。18 行使用受控 HTTP Provider + 真实隔离 PostgreSQL，共 55 个 passing events。产品特有业务拒绝/参数冲突/生命周期检查由下列独立用例补充。

| Tool | 独立正式边界 case ID | 最终非生成式结果 | 仍待真实验收 |
|---|---|---|---|
| `conversation.reply` | `TestExecuteToolConversationReplyPublishesReplaysAndRejectsPayloadConflict`; `TestExecuteToolConversationReplyRejectsUnownedTargetWithoutProduct` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `moment.publish` | `TestExecuteToolMomentPublishCommitsOutboxAndReplays` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `media.image.generate` | `TestExecuteToolImageGenerateAcceptsDurableTaskAndRejectsConflict`; `TestExecuteToolImageGenerateRejectsInvalidTargetWithoutIntent`; `TestExecuteToolImageGenerateDependencyFailureCreatesNoIntent` | DB + ADAPTER PASS | 真实生成/评审闭环由 visual_identity live 行补齐；accepted 不等于图片完成 |
| `visual_identity.initialize` | `TestIndependentToolE2EVisualIdentityInitialize` | DB + ADAPTER PASS | 真实生成/评审闭环由 visual_identity live 行补齐；accepted 不等于图片完成 |
| `visual_identity.generate_candidate` | `TestVisualIdentityGenerateCandidateToolCommitsDurableIntentAndReplays` | DB + ADAPTER PASS | 真实生成/评审闭环由 visual_identity live 行补齐；accepted 不等于图片完成 |
| `visual_identity.commit_review` | `TestVisualIdentityCommitReviewToolPreservesRejectedAssetAndCreatesNextAttempt` | DB + ADAPTER PASS | 真实生成/评审闭环由 visual_identity live 行补齐；accepted 不等于图片完成 |
| `visual_identity.finalize` | `TestVisualIdentityFinalizeToolCommitsCanonicalCharacterSheetAndCompletion` | DB + ADAPTER PASS | 真实生成/评审闭环由 visual_identity live 行补齐；accepted 不等于图片完成 |
| `scene_event` | `TestIndependentToolE2ESceneEvent` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `presence_event` | `TestIndependentToolE2EPresenceEvent` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `schedule.replan` | `TestIndependentToolE2EScheduleReplan` | CONTROLLED ADAPTER PASS；独立 live 用例未运行 | 真实日程模型计划与提交 |
| `memory_event` | `TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `active_memory_event` | `TestPostgresDirectActiveMemoryToolCommitsRejectsReplaysAndPersistsAudit` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `affect_event` | `TestIndependentToolE2EAffectEvent` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `memory.recall` | `TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `relationship.lookup` | `TestIndependentToolE2ERelationshipLookup` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `capability.request` | `TestIndependentToolE2ECapabilityRequest` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `persona.takeover` | `TestPostgresDirectPersonaToolsCommitRejectReplayConflictAndAudit` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |
| `persona.switch` | `TestPostgresDirectPersonaToolsCommitRejectReplayConflictAndAudit` | DB + ADAPTER PASS | 无额外模型依赖；完整严格 tools 套件仍待执行 |

共同补充：`TestIndependentToolReceiptFailureRollsBackMemoryAndAffect` 证明 receipt 写入故障回滚同一 Tool 的业务/outbox，恢复故障后正常提交；`TestIndependentToolE2ERejectsFalseSuccessWithoutWrite` 的隔离子进程跳过真实写入时必须失败，恢复后通过。

## 全部 17 个正式 Agent

每行均已提供正式入口和 live 用例，以下状态指**最终代码**。旧 16 行 live 尝试中 media_quality 曾通过，其余多因 GPU 内存不足失败；随机秘密链曾有两轮查询但最终答案错误，保留为 FAIL，不计作通过。完整视觉 live 未执行。历史详情见 `research/formal-agent-e2e.md`。

| Agent ID | 正式真实 case ID | 最终 live 状态 |
|---|---|---|
| `conversation_cognition` | `TestFormalAgentE2E/conversation_cognition` | NOT_RUN / 用户串行验收待执行 |
| `wake_up` | `TestFormalAgentE2E/wake_up` | NOT_RUN / 用户串行验收待执行 |
| `takeover_judge` | `TestFormalAgentE2E/takeover_judge` | NOT_RUN / 用户串行验收待执行 |
| `takeover_reply` | `TestFormalAgentE2E/takeover_reply` | NOT_RUN / 用户串行验收待执行 |
| `initialization` | `TestFormalAgentE2E/initialization` | NOT_RUN / 用户串行验收待执行 |
| `media_prompt` | `TestFormalAgentE2E/media_prompt` | NOT_RUN / 用户串行验收待执行 |
| `media_quality` | `TestFormalAgentE2E/media_quality` | NOT_RUN / 用户串行验收待执行 |
| `visual_identity_vision` | `TestFormalAgentE2E/visual_identity_vision` | NOT_RUN / 用户串行验收待执行 |
| `visual_identity_patch` | `TestFormalAgentE2E/visual_identity_patch` | NOT_RUN / 用户串行验收待执行 |
| `visual_identity` | `TestFormalAgentE2E/visual_identity` | NOT_RUN / 用户串行验收待执行 |
| `conversation_summary` | `TestFormalAgentE2E/conversation_summary` | NOT_RUN / 用户串行验收待执行 |
| `schedule_generation` | `TestFormalAgentE2E/schedule_generation` | NOT_RUN / 用户串行验收待执行 |
| `native_cognition` | `TestFormalAgentE2E/native_cognition` | NOT_RUN / 用户串行验收待执行 |
| `daily_review` | `TestFormalAgentE2E/daily_review` | NOT_RUN / 用户串行验收待执行 |
| `persistent_switch` | `TestFormalAgentE2E/persistent_switch` | NOT_RUN / 用户串行验收待执行 |
| `reflection` | `TestFormalAgentE2E/reflection` | NOT_RUN / 用户串行验收待执行 |
| `schedule_replan` | `TestFormalAgentE2E/schedule_replan` | NOT_RUN / 用户串行验收待执行 |

## 必需跨对象场景

| 场景 | case / 强制 gate | 类型 | 状态 |
|---|---|---|---|
| 三次决策、业务失败后继续 | `TestRunADKLoopContinuesAcrossBusinessFailureForThreeModelDecisions` | controlled | PASS |
| 同轮多调用关联 | `TestRunADKLoopPreservesSameRoundMultipleCallAssociation` | controlled | PASS |
| 写后查询可见 | `TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay`、正式 adapter SQL | real PostgreSQL | PASS |
| 提交后模型错误/非法 final | `TestRunADKLoopToolSuccessThenModelErrorPreservesPartialResult`、`TestCommittedReplySurvivesInvalidFinalAndFailedRunDoesNotReplay`、视觉对应 case | controlled + real PostgreSQL | PASS |
| 取消后收据、运行幂等 | `TestAgentRunRecordRetainsCommittedToolAfterCancellationAndPreventsReplay` | real PostgreSQL | PASS |
| timeout/cancel | `TestInitializationProviderErrorsDistinguishTimeoutAndCancellation`、两项 PostgreSQL terminal-state cases | controlled + real PostgreSQL | PASS |
| 超限、空 final 不能复用旧文本 | `TestRunADKLoopMaxIterationsReturnsErrorWithPartialFacts`、`TestRunADKLoopDoesNotReuseTextBeforeFinalEmptyAssistant` | controlled | PASS |
| 作用域隔离 | `TestContextReferenceIndexContainsOnlyCurrentProviderScope`、18 Tool foreign-owner 子项、人格 scope matrix | controlled + real PostgreSQL | PASS |
| Provider 原生流式 + 已提交 NDJSON | `TestProviderFormalAgentStreamingUsesTheSameRunnerStream`、`TestDirectConversationStreamsCommittedUserBeforeProviderAndAssistantAfterCommit` | controlled SSE + real PostgreSQL | PASS |
| 真实生产流式入口 | `TestLiveStreamTurnFormalAgentNDJSON`（完整 agents/all 强制） | real Provider + PostgreSQL | NOT_RUN |
| 不可猜随机秘密，两轮 recall/三次决策 | `TestFormalAgentE2E/conversation_cognition` | real Provider | 最终 NOT_RUN；历史错误答案 FAIL 保留 |
| 真实图片内容块与保存 | `TestVisualIdentityAgentReviewsRealObjectImageAndSavesCanonical` | controlled Provider + real MinIO/PostgreSQL | PASS |
| 真实图片生成与看图完成 | `TestFormalAgentE2E/visual_identity` | real Provider/ComfyUI/S3 | NOT_RUN |
| 断开回填必须失败，恢复后通过 | `TestFormalAgentE2ERejectsBrokenToolResultFeedback` | test-only subprocess mutation | PASS |
| 假写成功必须失败，恢复后通过 | `TestIndependentToolE2ERejectsFalseSuccessWithoutWrite` | test-only subprocess mutation | PASS |
| 缺项/零匹配/SKIP/BLOCKED/失败 | runner/verifier 固定用例与反例 | Node fake-shell | PASS |

完整 agents/all 强制运行脚本中的固定 controlled cross-case 清单、真实生产 StreamTurn gate 和全部 17 Agent 行；完整 tools/all 强制固定 18 Tool adapter 与独立用例。指定 Tool 命令也必须包含指定 adapter，结果标签明确为该 Tool，不冒充整个双套件。

## 实测与边界

串行命令、私有配置和清理说明见 `research/user-live-verification.md`。视觉非生成式依赖预检 `visual-dependencies-preflight-host` 已通过（ComfyUI system_stats + 随机空 bucket），没有调用 LLM/生图。视觉 live 验证生产 handler 链，不单独证明 Temporal transport/history/worker recovery。

只有最后真实 Tool/Agent 两套完整套件及必需 gate 均通过后，才允许标记整个任务完成。当前保持 `in_progress`，不归档、不宣布双 E2E 验收通过。
