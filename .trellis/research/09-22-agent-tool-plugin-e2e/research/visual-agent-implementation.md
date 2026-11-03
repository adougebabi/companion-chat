# Visual Identity 完整 Agent 实现记录

## 实现边界

本切片把原先由 `ProcessVisualIdentity` 外层 checkpoint 选择 `RunVisualIdentityVisionTask` / `RunVisualIdentityPatchTask` / canonical 保存的生产路径，迁移为一个正式完整 Agent。Temporal 仍只持有稳定 `session_id` 并重复调用 `ProcessVisualIdentity`；媒体生成仍由既有 `MediaWorkflow`、ComfyUI、对象存储和 `media_intents` 负责。没有新增 runtime、执行协议、工作流平台或数据库迁移。

正式 typed 入口：

```go
RunVisualIdentityAgent(ctx, VisualIdentityAgentInput{SessionID string})
    -> VisualIdentityAgentOutput
```

输出显式区分：

- `status=waiting, accepted=true`：候选图或 character sheet 已有真实 durable media intent，但最终资产尚未完成；
- `status=awaiting_review, accepted=false`：渲染配置无效或三轮自动再生成耗尽；
- `status=completed, accepted=true`：character-sheet ready asset 已提交到 profile/revision/session/attempt/timeline；
- `status=failed`：持久化 session 的 terminal failure，或底层依赖以 Go error 失败。

完整 Agent 注册为 `FormalAgentVisualIdentity`，使用现有 `RunFormalAgent` / Eino Runner、`visual_identity_vision` 模型绑定、专用 `visual_identity` Tool surface 和 `visual_identity_agent_response` 最终 schema。旧 vision/patch typed task 仍作为既有 16 项任务保留，但生产 `ProcessVisualIdentity` 不再调用它们，也不再在外层根据阶段选择下一次模型任务。

每个可执行 checkpoint 使用 `visual_identity_agent:<session_id>:attempt-<n>:<action_required>` 作为 stable Agent `OperationID`/run identity。同一个 durable 状态重入命中同一个 `agent_runs` 记录；候选 media 完成后 action 从 generate 变为 review、下一轮 attempt number 变化、character sheet 完成后 action 变为 finalize，因此后续合法 checkpoint 使用新的 run identity，不与旧输入冲突。pending 状态不调用模型。Agent Tool ledger 行持久关联 `agent_id=visual_identity` 与该 run ID。

## 独立 Tool 与事务

固定新增 3 项 Tool：

1. `visual_identity.generate_candidate`
   - 输入只有模型语义 `reason=initial|regenerate`；session/Fluctlight/Owner 由 `DirectToolTarget` 与 `ToolExecutionRequest` 显式绑定。
   - 同一短事务冻结角色卡 prompt、写 attempt、`media_intents`、`platform_workflow_intents` 和 timeline。
   - 返回 `accepted` 和真实 media intent / workflow task ID，不把 queued 冒充 completed。
2. `visual_identity.commit_review`
   - 输入包含 `decision`、`identity_match`、`confidence`、bounded observations、missing sections、summary 和 feedback。
   - 只有 media intent completed 且实际 ready asset 归属当前 Fluctlight 时才能提交。
   - regenerate 保留旧 asset/attempt 并创建带原 identity snapshot 和 previous review 的下一 attempt；三轮耗尽进入 `awaiting_review`。
   - accepted 在同一事务保存 vision/patch/decision，以 CAS 晋升 canonical revision，并创建 character-sheet media intent。
3. `visual_identity.finalize`
   - 只有 character-sheet media intent completed 且实际 ready asset 存在时完成。
   - 同一事务更新 profile/revision/session/attempt、ActionOutcome 与 character-ready/completed timeline。

三个 Tool 的直接调用和 Agent 原生 ToolCall 都进入当前 `App.ExecuteTool`；使用正式参数校验、窄服务接口、`TransactionalCapability` / `DirectToolCapability`、stable operation ledger 和 payload digest 冲突检测。Capability 不保存 `*App` service locator。

## 真实图片进入模型

当候选 media intent 已完成时，typed Agent 入口从 `media_assets` 核验 owner/status，通过既有 S3/MinIO client 读取实际对象 bytes，并构造 OpenAI-compatible：

```json
{"type":"image_url","image_url":{"url":"data:image/png;base64,..."}}
```

该内容块与 session、attempt、identity、renderer constraints、历史 asset 摘要一起成为正式 Agent 的初始 user message。图片不是文件路径、asset ID JSON 或文本化附件；Eino adapter 将真实 image content block 送进后续模型调用。`commit_review` 只能在这一路径下由完整 Agent 调用。

异步媒体不能在 PostgreSQL 事务或一次模型调用里阻塞等待。候选生成 Tool 返回后，Agent 给出 durable waiting；Temporal resume 同一个 typed Agent 入口，入口加载 ready asset 并把真实图片送到下一次正式模型决策。外层不选择 vision/patch Agent，也不执行模型输出。

## 验证记录

机器可读命令/状态证据：`research/runs/visual-agent-controlled.jsonl`。

不触发真实 LLM 的验证：

- `go test ./internal/core -run '^(TestVisualIdentity|...Phase8 visual selectors...)' -count=1`：PASS。
- 隔离 PostgreSQL：`TestVisualIdentityGenerateCandidateToolCommitsDurableIntentAndReplays`：PASS。
- 隔离 PostgreSQL：`TestVisualIdentityCommitReviewToolPreservesRejectedAssetAndCreatesNextAttempt`：PASS。
- 隔离 PostgreSQL：`TestVisualIdentityFinalizeToolCommitsCanonicalCharacterSheetAndCompletion`：PASS。
- 受控 Eino/httptest Provider：`TestVisualIdentityAgentUsesFormalRunnerAndToolReceiptForCandidateGeneration`：PASS；两次模型调用，第二次请求含真实已提交 Tool receipt。此项是 controlled regression，不是 live Provider 证据。
- 受控 Eino/隔离 PostgreSQL：`TestVisualIdentityAgentPreservesAcceptedGenerationWhenFinalModelFails`：PASS；候选生成 Tool 已提交后最终模型输出非法，运行准确失败，但 media intent 保留；同 session resume 直接返回 durable waiting，模型和 Tool 都没有重跑。
- `TestVisualIdentityAgentMessagesCarryRealImageContentBlock`：PASS；断言正式消息为多模态 image content block，并断言 3 个视觉 Tool 不泄漏到其他 surface。
- `TestVisualIdentityAgentCheckpointIdentityIsStableAndAdvancesWithDurableState`：PASS；同状态 ID 稳定，action/attempt 推进后 ID 改变。受控生成与失败恢复测试还独立查 `agent_runs` 及关联 `tool_executions`。
- `TestProcessVisualIdentityReportsDurableProviderConfigurationWait`：PASS；当前 `generic_llm` binding 缺失时生产入口返回明确 `waiting/provider_config_pending`，不创建 media intent，也不把配置缺失变成完成。
- 初始数据库/受控集合合并运行：6/6 PASS，exit 0；随后新增并单独通过提交后模型失败恢复用例与一次性 MinIO 真实对象用例。

`TestVisualIdentityAgentReviewsRealObjectImageAndSavesCanonical`：PASS，exit 0。测试使用隔离 PostgreSQL、一次性 MinIO 容器和专用 bucket 中的真实 PNG、正式 Eino Tool loop 与受控 Provider 响应；首个模型请求包含从对象存储读取的准确 base64 bytes，随后保存 canonical revision 与 character-sheet intent。容器在命令退出时由 trap 停止，复核无残留。该项仍是受控模型回归，且媒体完成状态由测试夹具提交，不属于真实 ComfyUI E2E。

整包 `go test ./internal/core -count=1` 当前仍有并行旧路径清理切片的静态断言失败；视觉新增的 service-locator 与固定 Tool inventory 失败已修复，视觉 selector 全部通过。未把整包当前失败报告成通过。

## 未执行与剩余验收

根据主线程 2026-09-22 的资源优先指令，本切片没有新增任何真实 LLM 调用。以下不计为通过：

- 完整 Agent 的真实 Provider 多模态 decision；
- 真实 ComfyUI 候选图和 character sheet 完成；
- 完整 Temporal + MediaWorkflow + MinIO + live Provider 的端到端完成。

后续串行验收必须使用任务隔离 PostgreSQL、专用 MinIO bucket、真实 ComfyUI 和真实 Provider，逐阶段等待同一 media intent 的最终 asset，再重复调用 `ProcessVisualIdentity` 直到 `status=completed`。不得以受控 Provider、手工置为 completed 的 media intent、queued/accepted 或本报告中的 MinIO 适配回归替代真实媒体成功。
