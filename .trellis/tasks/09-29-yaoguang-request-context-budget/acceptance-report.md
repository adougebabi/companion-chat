# 摇光完整请求上下文与预算修复：Go 验收记录

记录日期：2026-09-29。此报告只声明本机已执行的 Go/静态证据。真实 LLM、mlx-serve、ComfyUI 和用户长对话回放由用户后续验证；本报告不把 Mock 当成实际显存问题已解决。

## 结论及边界

现有 Eino 原生循环保留。每次物理 Generate/Stream 的模型输入、Tool Schema、结果和输出结构约束继续由同一 `estimatePromptWireInput` 策略检查；预算预留现在映射到 Eino 发送的 `max_completion_tokens`。可配置的 `prompt-budget.v2` 为已确认 16384-token 窗口提供 4096 输出预留、512 安全余量、11776 最大输入；普通会话首轮可选内容以 8000 为软目标，必需内容可超过软目标但不能超过硬限。`prompt-budget.v1` 及现存角色设置不被静默改写。

计数仍是保守启发式，`count_mode=estimated`，并非匹配 mlx-serve tokenizer/模板的精确硬上限。Role 的 `context_window_tokens` 必须配置为实际服务窗口；仓库没有可靠方法自动从 mlx-serve 推断。16K 的 GPU prefill 是否可用需要用户实测。

## 根因：已证实与待实测

| 结论 | 证据 | 状态 |
| --- | --- | --- |
| 物理调用前已有完整输入估算，但缺少实际 tokenizer/模板计数 | `internal/core/eino_model_runtime.go:176`、`prompt_context_assembler.go:196` | 已证实；仍为估算 |
| 输出预留原先未落到实际 Eino 请求；三个模型构造点均为 `MaxCompletionTokens: 0` | 修改前 `internal/core/eino_model_runtime.go` 的直接/ADK/流式构造；Eino factory 仅在值大于零时设置字段 | 已修复并用 HTTP Mock 验证字段 |
| 数据库预算约束只允许 v1 与 4096 余量，无法保存 16K/512 配置 | 修改前 `internal/migrations/runner.go` 的 `ck_model_roles_prompt_budget` | 已加幂等约束升级及测试 |
| 常规 Prompt 可注入多份 episode summary；单纯只取最新一份会让旧的未解决事项失去自动连续性 | `internal/core/conversation_summary.go:544`、`provider_context.go:85` | 保留现有来源校验与 2048-rune 总预算；未宣称完成单份累计摘要 |
| 已有按 Surface 精简 Runtime 投影、summary/raw 覆盖和状态替换机制，不能再叠加一套同类实现 | `provider_context.go:499`、`prompt_context_assembler.go:308`、`runtime_context_refresh.go:147` | 复用并沿此处收敛 |
| mlx-serve 16K 请求在真实负载下的 tokenizer 偏差与显存拒绝比例 | 需要部署环境 Provider usage/回放 | 未执行，由用户验证 |

## 请求链路

原链路和修复后链路均为：浏览器 durable turn → `ProcessCognitionInbox` → `RunConversationCognitionAgent` → `BuildContextProjectionFor` → `assembleProjectionPromptForSurface` → `RunADKStructuredTask` → `Provider.AgentAssembledCompletion` → Eino `RunADKLoop` → `queuedToolCallingChatModel.Generate/Stream` → Eino OpenAI-compatible serialization。工具续接由 Eino 保存 assistant ToolCall 与匹配结果；Core 执行独立 `ExecuteTool`，成功变更后 `runtimeContextRefresh.prepare` 仅在出站复制消息上替换 System/Runtime Slot，随后再次走物理预算检查。

修复前：预算采用估算值、输出预留未发往 Provider、16K/512 配置被数据库拒绝。修复后：相同预算版本贯穿组装、物理检查、数据库约束与 Provider 输出上限；诊断标记估算模式，并记录实际 Provider prompt/completion usage（可得时）。Eino 仍拥有循环，不新增手写模型—工具流程。

## 修改职责

- `prompt_context_assembler.go`、`provider_context.go`：v2 安全余量、角色预算解析、会话首轮可选内容软目标、估算标记；默认/未知任务 Surface 不再恢复宽泛 Runtime 与记忆对象，统一走现有精简投影。
- `eino_model_runtime.go`、`provider.go`、`diagnostics.go`、`mutations.go`、`agent_run_record.go`：三个 Eino 入口的输出上限、每轮预算细项与 usage 对照；区分巨大当前输入、必要工具结果、输出预留冲突与一般输入超限，诊断 payload 与实际请求一致。
- `adk_conversation_runtime.go`：衣柜列表的模型视图最多 12 件，长描述有截断标记；真实 ID/cursor 可由 `wardrobe.inspect` 继续读取，原始 Tool 收据不变。
- `intention_capabilities.go`：`intention.inspect` 列表由最多 20 条、`has_more` 却无后续读取方式，改为 10 条一页、显式 `cursor`/`next_cursor`；保留真实意愿 ID、状态和独立 Tool 入口。
- `capability_prompt_policy.go`：WakeUp 明确要求无新事实时不写 `memory_event`；数据库现有精确 canonical 去重与周期幂等保持不变。
- `settings.go`、`migrations/runner.go`：v2 配置持久化、现有 v1-only 约束的幂等升级和设置中的真实安全余量。
- 相应 `*_test.go` 与 `.trellis/spec/backend/fluctlight-provider-contract.md`：配置边界、三次工具续接后的预算拒绝、真实 cursor、输出上限与规范同步。
- 工作区原有的引用编码、时间格式、人格协议与投影修改保持原样并参与回归；本报告不将它们归功于本次新增改动。

## 同一受控输入的估算对比

`go test ./internal/core -run 'TestConversationSoftInputTargetLeavesRoomWithoutDroppingRequiredInput|TestModelFacingWardrobeListKeepsRealContinuationAndCanonicalReceipt' -count=1 -v` 输出：

| 受控样本 | 对照 | 本次路径 | 说明 |
| --- | ---: | ---: | --- |
| 完整组装估算，无/有会话首轮 8K 软目标 | 11081 | 6066 | 同一协议、当前输入、当前短发状态、Tool Schema、输出约束及四份较早摘要；较早可选摘要按完整单元减少，当前状态和输入保留。分项：system 617、tools 143、schema 28、current 46、摘要 10148→5074。 |
| 衣柜查询的原始收据/模型视图估算 | 4502 | 3508 | 同一 15 件记录；模型拿到前 12 件和真实 `next_cursor=item-11`，描述变短有标记，原始 15 件收据不变。 |

以上数字是测试构造样本、现有启发式估算，并非用户实际部署日志或真实 tokenizer 数量。静态 Schema 与当前输入未被禁用或缩短。它们不能单独证明所有实际普通会话均达到 8K。

## 自动化证据

- `go test ./...`（`apps/core-go`）：通过，覆盖 Go Core 各包与现有集成测试包。单独以 `-v` 执行 `TestPostgresProviderRolePromptBudgetRoundTripAndValidation`，因 `GO_CORE_TEST_DATABASE_URL` 未设置而 **SKIP**；数据库 v2 round-trip 尚未实际验证，不能把全量 Go 通过视为该项通过。
- `TestProviderADKChecksThirdToolContinuationBeforeHTTP`：HTTP Mock 收到三次模型请求和三次 Tool 执行；第三次工具结果过大，第四次请求在本地拦截，`errors.Is(ErrPromptRequiredBudgetExceeded)` 成立。
- `TestPromptBudgetConfigurationUsesOutputReserveAndSafetyMargin`：16384/11776/4096/512 边界通过，11777 输入配置拒绝。
- `TestProviderFormalAgentStreamingUsesTheSameRunnerStream`、`TestStreamWithEinoAggregatesChunksAndPropagatesCallback`、`TestDirectEinoRequestUsesBudgetedCompletionLimit`：HTTP Mock 分别验证 ADK 流、直接流和直接生成的 `max_completion_tokens` 与配置预留一致。
- `TestModelFacingWardrobeListKeepsRealContinuationAndCanonicalReceipt`：真实 cursor、截断标记及原始收据不变。
- `TestPromptContextMemoryMigrationAddsValidatedModelPromptBudgets`：v2 约束与旧约束升级 SQL 静态校验。
- `TestDefaultSurfaceCannotRestoreBroadRuntimeDump`：默认 Surface 不再注入 Persona、原文历史与记忆存储元数据，当前状态仍在。
- `TestPromptAssemblerPressureDoesNotScaleWithStores`：1000 条原文、100 条检索记忆及 30 条活动记忆的受控输入在 v2 16K 策略下保持 8000 估算 token 软目标以内，原输入集合未被修改；这是构造数据，不是真实 30 轮回放。
- `TestIntentionToolSchemasAllowUniqueTargetInference`：分页游标 schema 通过；`TestIntentionInspectListCanReadEveryPageByCursor` 因 `GO_CORE_TEST_DATABASE_URL` 未设置而 **SKIP**，数据库两页实测留待有隔离库的环境。
- `git diff --check`：通过。真实 tokenizer、真实 Provider、ComfyUI、30 轮真实对话回放：未执行，按用户要求留待其验证。

## 未完成验收及风险

1. 完整请求计数没有模型 tokenizer/聊天模板的精确实现。估算和 512 安全余量不能被表述为严格的实际 token 上限；需要用用户的 Provider usage 数据校准。
2. 摘要仍是来源可核验的多个 episode，不是单份涵盖全部更早历史的累计摘要。若只保留最新 episode 会损失跨天未解决事项，因此没有这样做；跨 episode 的持续事项是否稳定进入 Prompt 尚需用户回放验证。
3. 检索记忆对不同来源 ref 但语义相同的内容尚无可证明安全的去重；只凭文本相似合并会误删两次真实事件。本次保留来源优先机制。
4. 除衣柜列表及已有定界工具外，其余大型工具结果若没有可授权分页/读取方式，最终物理预算会明确拒绝该轮，而不会静默截断或重复执行工具。
5. WakeUp 协议已禁止无新事实时写重复记忆，但没有对跨周期同义改写增加可证明安全的代码级语义去重；现有精确 canonical key 和同周期幂等仍生效。
6. 尚未做真实用户样本的完整分项前后对比，也未证明所有角色都把实际 16K 窗口写入 `model_roles`。已检索的 Go Core 模型入口都经 `generateWithEino` 或 `streamWithEino`；未发现其外的 Go Core Eino Provider 发送点，但部署插件或外部调用不在本机证据范围。
7. Token 降低不保证 LLM 与 ComfyUI 并行时永不缺显存；Provider/GPU 内存拒绝仍需作为实际 Provider 错误处理与实测。

本报告是阶段性 Go 验收记录。上述未完成项存在时，不将专项任务标记为完全完成。

代码目前保持未提交：任务开始前工作区已有用户的协议、引用编码、投影与测试改动，且与本次改动位于同一批文件。未替用户将混合差异提交或推送；Trellis 任务保持 `in_progress` 供用户验证后继续收尾。
