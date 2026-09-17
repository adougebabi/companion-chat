# Turn Takeover 交付与当前验收报告

任务：`09-14-persona-takeover-working-prompt`  
当前复核：2026-09-15  
权威设计：`.trellis/tasks/09-14-persona-takeover-working-prompt/design.md`  
当前验收凭证：`.trellis/tasks/09-14-persona-takeover-working-prompt/final-acceptance-report-v2.md`

本文记录原任务的确定性实现与 Fake/fixture 验收事实。`final-acceptance-report.md` 是第一轮混合报告，`final-acceptance-recheck-report.md` 是第二轮整改前的复验记录；二者不再作为当前状态凭证。真实 LLM 与生产 `HandleTurn` 的后续复验见 [`live-llm-acceptance-report.md`](live-llm-acceptance-report.md)，不能用本文原先的 Fake Provider 结论替代。

## 1. 实现范围

生产实现集中在 `apps/core-go/internal/core/`，核心职责如下：

| 模块 | 当前职责 |
|---|---|
| `turn_takeover.go` | A → Judge → B 仲裁点、阶段机、B 生成、QUERY 互斥、Overlay 错误降级 |
| `turn_decision.go` / `visible_output.go` | 单一候选归一化与 canonical visible text 冻结 |
| `capability_runtime.go` / `capability_core.go` | Judge 前结构、权限、冻结 snapshot 和 capability candidate 校验 |
| `persistent_switch_gate.go` | E1–E5 持久切换授权脊；takeover B 无权改变 active profile |
| `persona_switch_rules.go` | typed `takeover_rules[]` 归一化、稳定 ID、诊断；旧 prose 不做执行推断 |
| `working_persona.go` / `takeover_scope.go` | Working Persona、Judge control view 和 reply-owner 作用域重建 |
| `provider_prompt_composer.go` / `provider_context.go` | 唯一 System Persona 出口、roster 收敛和 Runtime Context 去重 |
| `app.go` / `provider_schemas.go` | 初始化 typed rule schema、边界校验和迁移提示 |

没有新增 Agent 框架、数据库表或并行发送通道；阶段状态保存在既有 frozen payload 的 `turn_stage` 中。

## 2. 当前运行契约

1. A 先生成未执行 Candidate。Candidate 通过同一个 `normalizeTurnDecision` 和 `validateCandidateCapabilityInvocations` 后，才允许进入仲裁。
2. Judge 只判断 Runtime 预先选定的唯一 takeover rule 是否触发；它不选目标、不执行能力、不改写回复，也不能创建 rule ID。
3. Judge `true` 时 A 标为 rejected/superseded，A 的消息、能力、状态、关系、记忆和演化提案均不结算；B 使用 reply-owner 作用域重新生成且不再仲裁。
4. 只有 winner 进入 Prepare、Execute、事务结算和 outbox 投递。阶段 CAS、版本门和恢复路径不重判已持久化 verdict。
5. `active_profile_id` 是持久主导人格，`reply_owner_profile_id` 是本轮发言人格。Takeover 只改变后者。
6. 可见文本由 `resolveCanonicalVisibleReply` 计算一次并冻结：根级 `visible_text` 优先，根字段为空时允许 `conversation.reply.text` fallback；两个来源冲突时 `visible_text_source_conflict` fail closed。最终发送仍走既有 `conversation_messages` 管线。
7. 需要可见结果的 ACTION（例如 `media.image.generate` 或 Memory mutation）必须和 `conversation.reply` 在同一 Main cognition 中返回；ACTION 参数不能替代可见文本。多工具 Provider 请求携带 `parallel_tool_calls=true`，单工具请求省略该字段；ACTION-only 结果在结算前以 `cognition_visible_text_missing` fail closed。
8. Judge 前 candidate validator 使用 Core 冻结的 ContextSnapshot；内置 `relationship.lookup` 只从冻结 alias 和授权 actor 集合解析参数，不查询数据库，不调用 Preflight/Prepare/Execute。
9. B 的 Overlay clone/load/compose 失败返回稳定错误码，跳过 Judge 并走 `judge_degraded` 保留 A；明确无 Overlay 记录才使用声明 baseline。
10. WakeUp Provider 的 native/structured tool-call 归一化失败持久化 `tool_call_invalid` 与有界 shape 诊断；Worker 及 activity lifecycle `safe_cause` 只输出稳定 `error_code`/allowlist `error_reason`，不输出模型控制的调用内容。
11. 混合 `media.image.generate` + `conversation.reply` 在同一 Main cognition 中绑定同一个 assistant message；图片 intent、媒体 workflow、outbox 与 Core stream assistant frame 均有回归覆盖。Core 错误帧使用 `payload.code`，BFF/Web 保留旧 `payload.error` 兼容。
12. 流开始前 BFF HTTP 错误由生成的 `BrowserClient.turn()` 解析为 `BrowserApiError`；Pinia 使用稳定 code，避免状态码错误退化为 `turn_failed`。

## 3. Prompt 与数据收敛

- Main 的唯一 System Persona 由 shared identity、当前 Working Persona 和授权场景的 persistent switch 组成；`takeover_rules` 只进 Judge control view。
- 非激活 profile 的 roster 只保留 `{id,name}`。完整 B 不进入 A Prompt，完整 A 不进入 B Prompt。
- Working Persona 只保留当前发言人格的必要行为语义；`life_profile` 当前 allowlist 为 `preferences` 与 `character_constraints`。描述性 traits 原词保留，非对象形状改为描述性载体，不臆造数值。
- Runtime Context 删除 `core_persona`、`effective_persona`、`personality_system` 等重复人格副本；历史消息保留真实 role，并附发言人格归属。
- 未授权场景的 response schema 不含 `personality_decision`；B、Judge 和 QUERY continuation 不能提出持久切换。

## 4. 规则迁移与真实卡边界

Runtime 只消费一套 typed takeover 语义。新的初始化输出放在 `personality_system.takeover_rules[]`；为兼容迁移中间态，`forced_activation` 下只有本身带齐同一组 typed 字段的对象才会映射到该规范语义。每条可执行条目必须包含：

```json
{
  "id": "stable_rule_id",
  "kind": "turn_takeover",
  "version": "turn-takeover.v1",
  "condition": "…",
  "target_profile_id": "declared_profile_id"
}
```

`source_profile_id`、`priority`、`cooldown_seconds`、`enabled` 和 `evidence_refs` 可选，但若出现必须通过初始化与 Runtime 的类型/引用校验。`target_profile_id` 与 `source_profile_id` 必须是声明的稳定 profile ID；profile display name 或 condition prose 不能替代它们。

| 来源 | 当前处置 | 证据 |
|---|---|---|
| `switching.rules[]` | 保留为授权 Main 的 persistent switch；机器可读时间窗只分类、生成诊断，不实现 deterministic evaluator | `persona_switch_rules_test.go`、`persistent_switch_gate_test.go` |
| `forced_activation` 历史 prose / 迁移中间态 | 原文保留为迁移证据；没有完整 typed 元数据时输出 `unclassified`/bounded diagnostic；只有显式完整 typed 对象才映射到同一规范语义，不按关键词、正则或 profile name 推断执行语义 | `TestUnclassifiedForcedActivationKeepsRawValueAndIsReported`、`TestForcedActivationRuleIDIsStructuralNotTextual`、dense card 双路径测试 |
| `takeover_rules[]` | 接管唯一正式执行入口，`takeover:<id>` 贯穿初始化、归一化、Judge 引用、冻结和恢复 | `TestDeclaredRulesReachTheProductionTurn`、初始化校验测试 |

`dense_multi_card.txt` 与 `dense_multi_expectations.json` 继续验证「公开质疑 → 星火接管」原始 prose 被保留。真实 prose 归一化为 0 条 executable takeover，并记录诊断；固定 `dense_multi_typed_migration.json` 代表授权迁移 Provider 输出，同一语义归一化为 1 条显式 typed rule。真实初始化是否从该卡产出 typed 字段仍需显式启用对应 live 测试；固定 fixture 不替代 live 结论。

## 5. 测试证据

本轮最终报告中的命令和退出码以同一工作区重新执行的结果为准：

```bash
test -z "$(gofmt -l $(rg --files apps/core-go -g '*.go'))"
git diff --check HEAD
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go -C apps/core-go test -count=1 ./...
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go -C apps/core-go test -race -count=1 ./...
```

覆盖范围包括：

- A/B winner-only 副作用隔离、五阶段恢复、执行阶段崩溃恢复和调用预算；
- Judge 前 capability surface、metadata owner、冻结 snapshot、参数级授权与 relationship foreign actor fail closed；
- F-01 root/fallback/conflict 文本契约；
- typed rule schema、真实 dense prose 与固定 typed migration 双路径；
- persistent active 与 turn reply owner 分离、B 无持久切换权限；
- B accepted Overlay happy path 以及 clone/load/compose 失败降级；
- Working Persona allowlist、描述性 traits 保真、roster 收敛和 Runtime Context 去重；
- pure QUERY 与 takeover 互斥、QUERY + ACTION mixed 候选边界；
- WakeUp `tool_call_invalid` 有界诊断、native/structured mixed media + reply durability 和 Core stream delivery；
- `go vet`、构建、格式、完整包测试和 Race Detector。

## 6. 成本与性能证据边界

`apps/core-go/internal/core/testdata/turn_path_cost_report.json` 是 Fake Provider wire capture，当前四条路径为：

| 路径 | 物理请求 | 估算输入 Token | 字符 | 字节 |
|---|---:|---:|---:|---:|
| plain_no_judge | 1 | 23,593 | 18,874 | 20,584 |
| judge_keeps_a | 2 | 25,533 | 20,426 | 22,760 |
| judge_takeover_b | 3 | 48,650 | 38,919 | 43,483 |
| pure_query | 2 | 30,617 | 24,493 | 27,931 |

估算约按 1.25 tokens/rune，Judge 请求本身为 1,940 估算 Token / 2,176 bytes；接管路径包含被丢弃 A 输出。启用多工具并行提示后，主请求 wire 增加了该控制字段，报告中的数字已由测试重新生成。`life_profile_before_after_report.json` 只比较 dense-shaped synthetic fixture 的局部 allowlist：life_profile 8 个 key/753 bytes → 2 个 key/174 bytes，Working Persona 990 → 432 bytes。

这些数字证明当前 wire 预算、调用序列和裁剪方向；它们不证明真实模型 Token 计费、缓存命中、首条可见延迟、总耗时，也不满足同一真实卡 full request 前后性能对照。真实 Provider 指标需在 live 配置后另行验收。

## 7. 未实现与环境未验证

**明确未实现**：deterministic 时间窗 Runtime evaluator；本版只分类、生成稳定 ID 和诊断。

**本地确定性回归已验证**：Fake Provider + PostgreSQL 已覆盖 native/structured mixed media + reply 的 capability normalization、同一 assistant message 结算、图片 intent/workflow/outbox 持久化和 Core stream 帧顺序。真实 Provider、人格初始化和同一轮切换后的再次认知证据见 `docs/live-llm-acceptance-report.md`。

**仍未验证**：真实模型 Judge 误报/漏报、抗注入表现；真实 Token/缓存/完整延迟基线；真实 ComfyUI/Media Worker 最终图片资产；WakeUp/媒体 Worker 的生产端到端链路；deterministic 时间窗求值器（尚未实现）。已运行的真实 Provider smoke 与 disposable PostgreSQL `HandleTurn` 链路见 `docs/live-llm-acceptance-report.md`。

这些项目不会被 Fake Provider、固定 migration fixture 或合成字节报告标记为通过。

## 8. §20 八问结论

| 问题 | 当前结论 |
|---|---|
| A 是否只是未执行提案，B 接管后 A 的状态变化是否不提交？ | **通过**：winner-only 执行、A rejected candidate 无事实残留和恢复测试覆盖。 |
| 私聊是否只有一条正式发送管线？ | **通过**：canonical visible text 统一冻结，最终仍经 `conversation_messages`；reply-only 是受控 fallback，冲突 fail closed。 |
| 持久 active 与本轮 reply owner 是否分离，B 是否禁止再仲裁？ | **通过**：`active_profile_id` 与 `reply_owner_profile_id` 分离，B 无持久切换授权且不调用 Judge。 |
| Judge 是否只看到所需的小上下文？ | **通过**：control view、候选文本和必要事实独立组装，无 Tools/完整 Main Prompt/完整 profiles。 |
| Main Prompt 是否只有当前发言人格的 Working Persona？ | **通过**：唯一 System Persona 出口、roster `{id,name}` 和 life_profile allowlist。 |
| pure QUERY 与 takeover 是否互斥并有测试？ | **通过**：结果依赖型 continuation 跳过仲裁；mixed final 可仲裁但不伪造查询结果；B continuation 超预算受控失败。 |
| 全请求大小、调用次数和真实延迟是否如实测量？ | **部分通过**：Fake wire 的请求序列/估算大小已测；真实延迟、缓存、计费和同一真实卡 full-request before/after 未验证。 |
| 是否能通过数据配置扩展 profile/rule 而不用角色名或工具名分支？ | **通过**：typed rule 与声明 profile 驱动，静态守卫禁止具体 capability/name 分派。 |

## 9. 验收状态

本轮格式、静态检查、完整测试、竞态测试和文档同步均已完成；原任务的确定性状态保持 `completed`。真实 LLM 复验由 `docs/live-llm-acceptance-report.md` 单独记录，仍未覆盖的项目不能被本报告的 Fake/fixture 结果替代。
