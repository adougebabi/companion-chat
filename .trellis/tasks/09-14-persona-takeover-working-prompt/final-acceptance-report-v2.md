# 多重人格接管、Working Persona 与 Prompt 收敛：最终验收报告（复查版）

> 验收日期：2026-09-15  
> 验收对象：`master@7b26364` 之上的当前未提交工作区  
> 任务：`09-14-persona-takeover-working-prompt`  
> 设计：[`design.md`](design.md)  
> 当前交付摘要：[`docs/persona-takeover-delivery-report.md`](../../../docs/persona-takeover-delivery-report.md)

## 1. 最终结论

**范围内验收通过，任务可以标记为 `completed`。**

当前 Go Runtime 已实现并通过确定性测试验证：A 候选先冻结、Judge 只做一次布尔仲裁、只执行最终 winner、B 不递归仲裁、持久 active 与本轮 reply owner 分离、Working Persona 与 Judge control view 分离、纯查询续接与 takeover 互斥、以及 typed takeover rule 的 fail-closed 契约。前次复验发现的 F-02 权限前置不足、F-03 旧 prose 误执行、F-05 Overlay 错误静默回退和 F-04 恢复证明不完整均已收口；本轮还修复了 Overlay 失败路径中 nil baseline 导致的 panic。

“通过”只覆盖当前代码、固定 Provider 和测试数据库能够证明的范围。真实 Provider、真实模型 Judge 行为、真实 Token/缓存/延迟和 deterministic 时间窗求值仍按本报告第 7 节标记为环境未验证或明确未实现，没有用模拟数据代替这些证据。

## 2. 与设计的逐项核对

| 设计要求 | 当前实现与证据 | 结论 |
|---|---|---|
| A → Judge → B，winner-only execution | `turn_takeover.go` 阶段机；`turn_takeover_chain_test.go`、`turn_chain_budget_test.go` 验证 A rejected 后无执行，只有 winner 进入 Prepare/Execute/结算 | 通过 |
| Candidate 在 Judge 前无副作用 | A/B 都先走 `normalizeTurnDecision`；candidate validator 不调用 Execute、Preflight、Prepare、数据库、网络或 planner；非法 A/B 在 Judge 前失败 | 通过 |
| Judge 看到 A 的真实待发送文本 | `resolveCanonicalVisibleReply` 只计算一次并冻结；Judge、preview、结算读取同一 canonical | 通过 |
| reply 文本权威唯一 | 根级 `visible_text` 优先；reply-only fallback 合法；来源冲突 `visible_text_source_conflict` fail closed；消息仍走 `conversation_messages` | 通过（以 request.md 的 path-b 修订为准） |
| B 不改变持久 active | E1–E5 授权脊；takeover B schema 无 `personality_decision`，结算/恢复/覆盖均拒绝未授权 persistent switch | 通过 |
| B 的 profile-scoped 读写 | `takeover_scope.go` 重建 reply-owner projection；关系、目标、意图、记忆、引用和 Overlay 使用冻结 owner；下一轮回到 active | 通过 |
| Judge 输入收敛 | Judge 只收到有限 system 协议和 user 数据包；无 Tools、完整 Main Prompt、完整 profiles 或完整记忆；`takeover_rules` 不进 Main | 通过 |
| B Overlay 可靠进入 Judge | accepted Overlay 合并到 B control view；clone/load/compose 失败返回稳定错误码、跳过 Judge、`judge_degraded` 保留 A；无 Overlay 记录才使用 baseline | 通过 |
| typed takeover rule | 初始化与 Runtime 都要求 `id/kind/version/condition/target_profile_id`；目标必须是声明 profile ID；profile name/prose mention 不能授权 target | 通过 |
| forced_activation 迁移边界 | 原 prose 原样保留；无完整 typed migration 只记录 `unclassified`；固定 typed fixture 可执行；Runtime 不按关键词/正则猜测 | 通过 |
| dense multi card | 真实 `dense_multi_card.txt` prose 路径 0 条 executable + 诊断；`dense_multi_typed_migration.json` 路径 1 条 executable；真实 Provider 产出未运行 | 通过（fixture），live 未验证 |
| persistent switch | 只在授权 `cognitive_assessment` 场景保留 Main 的 persistent switch；WakeUp/Reflection 不新增切换入口；deterministic 时间窗只分类诊断 | 通过；deterministic evaluator 未实现 |
| pure QUERY 边界 | 结果依赖型 `query_continuation` 整轮跳过 takeover；QUERY + ACTION mixed 为 final、可仲裁但不伪造查询结果；B continuation 超预算受控失败 | 通过 |
| 调用预算 | 普通 1 次 A；Judge 保留 A 为 A+Judge；接管为 A+Judge+B；零重试，Judge 独立记账 | 通过（Fake Provider） |
| Working Persona 收敛 | 唯一 System Persona 出口；roster 只保留 `{id,name}`；life_profile allowlist 为 `preferences`、`character_constraints`；描述性 traits 保真 | 通过 |
| 恢复与阶段 | `a_frozen → arbitration_decided → b_frozen → winner_ready → executing → settled`；冻结 rule proof 包含 version/digest；恢复不重新 Judge/选规则 | 通过 |

## 3. 本轮实际修复

- 在 `capability_core.go` / `capability_runtime.go` 增加 `CapabilityCandidateValidator` seam。Judge 前验证 capability 存在性、schema 参数、surface、metadata owner、output target kind、冻结 snapshot identity/RequiredContext，并执行纯参数级 hook。
- 为内置 `relationship.lookup` 增加冻结 scope 校验：`target_actor_id` 只能解析冻结 alias 和 `authorized_actor_ids`，foreign actor、unknown alias、空授权集合均 fail closed；Judge 前不查数据库。
- 将 `projectTakeoverControlView` 改为显式返回错误。Overlay projection clone、读取或合成失败不会再静默使用旧 baseline，Judge 请求数为零并保留 A。
- 将 `takeover_rules[]` 固化为初始化与 Runtime 的 typed contract；`forced_activation` 只作迁移证据。新增真实 dense prose 与固定 typed migration 双路径 fixture 和断言。
- 收紧 pending takeover 恢复：`rule_id`、声明 target、`rule_version=persona-switch-rules.v1`、`rule_content_digest` 均必须存在且匹配，缺失/错误时 fail closed。
- 修正 Evolution baseline 在 nil/空 map 下的可写形状，避免 Overlay 失败路径 panic；新增回归测试。
- 同步架构文档、交付报告、执行计划、F-01 测试注释；第一轮报告和旧第二轮复验报告已明确标为历史文件。

## 4. 质量门禁

以下命令在当前工作区重新执行，均退出码 0：

```bash
test -z "$(gofmt -l $(rg --files apps/core-go -g '*.go'))"
git diff --check HEAD
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
```

带测试数据库的完整包测试：

```bash
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go -C apps/core-go test -count=1 ./...
```

结果：退出码 0。

| 包 | 结果 | 用时 |
|---|---:|---:|
| `internal/core` | PASS | 156.624s |
| `internal/httpapi` | PASS | 3.134s |
| `internal/migrations` | PASS | 31.467s |
| `internal/platform` | PASS | 2.168s |
| `internal/workflow` | PASS | 2.594s |
| `internal/config` | PASS | 0.409s |
| `cmd/*` | 无测试文件 | — |

Race Detector：

```bash
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go -C apps/core-go test -race -count=1 ./...
```

结果：退出码 0。

| 包 | 结果 | 用时 |
|---|---:|---:|
| `internal/core` | PASS | 176.068s |
| `internal/httpapi` | PASS | 3.340s |
| `internal/migrations` | PASS | 31.865s |
| `internal/platform` | PASS | 2.150s |
| `internal/workflow` | PASS | 3.947s |
| `internal/config` | PASS | 1.566s |

专项复核还通过了：

- `TestTakeoverControlViewFailureDoesNotFallBackToBaseline`
- `TestJudgeSkipsProviderWhenTakeoverControlViewFails`
- `TestTakeoverControlViewCompositionFailureIsNotAnEmptyOverlay`
- `TestTakeoverControlViewCloneFailureIsNotAnEmptyOverlay`
- `TestEvolutionBaselineStillNormalizesStructuredTraits`
- `TestRecoveryFromPendingTakeoverRequiresFrozenRuleProof`
- `TestPersonaSwitchRulesNormalizeTheRealMultiProfileCard`
- `TestDeclaredRulesReachTheProductionTurn`
- F-02 candidate validator、foreign actor、snapshot identity 和禁止执行路径测试

## 5. 规则迁移验收

正式执行语义只有一套，新的初始化输出入口是 `takeover_rules[]`；为兼容迁移中间态，`forced_activation` 下只有带齐同一组 typed 字段的对象才会映射到该规范语义：

```json
{
  "id": "stable_rule_id",
  "kind": "turn_takeover",
  "version": "turn-takeover.v1",
  "condition": "…",
  "target_profile_id": "declared_profile_id"
}
```

`dense_multi_card.txt` 中的原文仍保留：暮光/星火的差异、公开质疑时星火接管以及安全确认后暮光重新主导。Runtime 对这段 prose 不做自然语言执行推断；它会保留诊断，得到 0 条可执行 takeover。固定 `dense_multi_typed_migration.json` 提供了经授权迁移后的 typed 输出，测试确认得到 `takeover:dense-public-challenge`、target `spark`、source `twilight`、正确 declaration version。真实 Provider 是否在初始化时生成同样字段，必须用 live 环境另测。

`switching.rules[]` 继续支持授权 Main 提出的 persistent switch；`persistent_deterministic` 只分类、生成稳定 ID 和诊断，不实现时间求值器。没有新增 WakeUp/Reflection 切换入口。

## 6. 性能与成本证据

`apps/core-go/internal/core/testdata/turn_path_cost_report.json` 是变更后 Fake Provider wire capture：

| 路径 | 物理请求 | 估算输入 Token | 字符 | 字节 |
|---|---:|---:|---:|---:|
| `plain_no_judge` | 1 | 23,439 | 18,751 | 20,367 |
| `judge_keeps_a` | 2 | 25,379 | 20,303 | 22,543 |
| `judge_takeover_b` | 3 | 48,342 | 38,673 | 43,049 |
| `pure_query` | 2 | 30,343 | 24,274 | 27,524 |

Judge 请求为 1,940 估算 Token / 2,176 bytes，约为主请求输入的 8.3%；接管路径包含被丢弃 A 的输入/输出成本。估算约按 1.25 tokens/rune，输出侧是脚本值。

`life_profile_before_after_report.json` 是 dense-shaped synthetic fixture 的局部 allowlist 对照：8 个 key、753 bytes → 2 个 key、174 bytes；Working Persona 990 → 432 bytes。它证明裁剪方向和当前序列化结果，不能替代同一真实卡的 full-request before/after。

## 7. 未验证与未实现

**环境未验证**：

- `FLUCTLIGHT_LIVE_PROVIDER_TEST=1` 未配置，8 个 `TestLiveProvider*` 用例均按预期 SKIP；
- 真实 Provider 是否从 `dense_multi_card.txt` 生成 typed `takeover_rules[]`；
- 真实模型 Judge 的误报/漏报、抗注入和多轮风格保持；
- 真实 Token 计费、缓存命中、首条可见延迟、总耗时；
- 同一真实卡的 baseline/current full request、Working Persona、Judge 三组前后性能；
- 未配置 `GO_CORE_TEST_DATABASE_URL` 时冻结/结算集成链路不会被声称为真实执行。

**明确未实现**：

- deterministic 时间窗 Runtime evaluator；当前只分类、保留稳定 ID 和诊断。

Fake Provider、固定 migration fixture 和 synthetic bytes 报告不会被当作以上 live 证据。

## 8. 文档与状态

当前口径已同步到：

- `.trellis/tasks/09-14-persona-takeover-working-prompt/design.md`
- `.trellis/tasks/09-14-persona-takeover-working-prompt/implement.md`
- `docs/persona-takeover-architecture.md`
- `docs/persona-takeover-delivery-report.md`
- `apps/core-go/internal/core/turn_takeover_f01_test.go`

`final-acceptance-report.md` 与 `final-acceptance-recheck-report.md` 已标记为历史报告；本文件是唯一当前验收凭证。任务状态在本轮验证完成后更新为 `completed`。
