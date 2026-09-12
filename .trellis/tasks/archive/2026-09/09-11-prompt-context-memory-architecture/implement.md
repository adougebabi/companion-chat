# Implementation Plan

状态：in_progress；S12已通过（`check-go-projections.sh`获用户明确环境豁免），S13最终报告已完成；当前等待Phase 3.4提交计划确认。  
执行目录：`/Users/vinson/Documents/project/个人/local-ai-companion`  
分支：`codex/prompt-context-memory`（基于 `codex/llm-capability-runtime`）

## Task Shape

本任务不拆 parent/child。虽然交付包含多个子系统，但 Raw source linkage、Active/Long-term provenance、Working Memory、Prompt Budget、B-layout roles 与 QUERY continuation 共用同一 migration、ContextProjection、frozen replay 和 conversation cutover；拆为并行 child 会制造不可独立验收的中间状态。实现仍按下列小阶段推进，每阶段有独立测试和 rollback point。

## Strict Execution Rules

- S01→S11严格串行；当前阶段的业务代码、精准测试、targeted formatting/diff check和evidence未完成前，不进入下一阶段。
- 当前阶段只可修改该阶段明确列出的 **Stage file allowlist**。所有阶段额外允许更新当前任务目录 `.trellis/tasks/09-11-prompt-context-memory-architecture/` 内的checklist与evidence。
- “Likely file”不是隐式授权；若实现要求触碰allowlist外文件，立即停止，先修订本计划并取得用户范围确认。
- S01→S11禁止运行：`go test ./...`、任何`-race ./...`、全仓vet/build、Docker Compose、Live Provider、全量PostgreSQL/Temporal smoke或跨阶段验收。
- S01→S11只运行直接覆盖当前阶段新增/修改符号的`go test <exact package> -run '<exact test names>' -count=1`；格式与diff检查也只指向当前阶段修改文件。
- 阶段外既有失败只记录在`implementation-evidence.md`，不得扩散修改。
- 只有S01→S11全部完成并打勾后，才进入S12并运行本文件末尾的Required Validation Commands。
- `.trellis/tasks/09-08-project-health-evolution/` 永远不在任何阶段allowlist中：不改写、不格式化、不删除、不stage。

## Completed Planning Evidence

- [x] S00.1 Inventory：Prompt、Conversation/Raw facts、Memory、Reflection、Wake-up/Workflow、Capability、Provider配置与trace。
- [x] S00.2 Prompt/Role baseline：确认一条巨型user document、current input重复、recent role非transport role、四条Main builder、无总input budget。
- [x] S00.3 A/B/C真实模型实验：两轮36次调用；选择B；记录input/output token、bytes、latency、raw代表样本与限制。
- [x] S00.4 用户批准pure-QUERY-only单次continuation例外。
- [x] S00.5 完成PRD初版、研究记录和target design。

## Ordered Checklist

### S01. Baseline gate and exact-code read

- Stage file allowlist（write）：仅当前任务目录内的`implement.md`、`implementation-evidence.md`；生产代码全部只读。
- [x] 确认分支、HEAD、dirty paths；保留无关未跟踪`.trellis/tasks/09-08-project-health-evolution/`，不修改、不stage。
- [x] 加载`trellis-before-dev`并完整读取即将修改的Go文件、migration、相关测试以及backend directory/error/debug/quality guidelines。
- [x] 不运行任何测试或全量验证；复用已记录的Capability分支验收和本任务规划期只读/Role实验基线，记录当前HEAD与文件hash作为pre-change baseline。
- [x] 冻结当前Capability schema stats、response schema source hash、典型system/runtime/recent/current已知指标与不可得项，作为迁移前对照。
- [x] Rollback point: 只含task/research artifacts；生产树零改动。

### S02. Raw History source contract and turn atomicity

Stage file allowlist:

- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`
- `apps/core-go/internal/migrations/prompt_context_memory_postgres_test.go` (new; author in S02, execute only in S12)
- `apps/core-go/internal/core/repository.go`
- `apps/core-go/internal/core/cognition.go`
- `apps/core-go/internal/core/mutations.go`
- `apps/core-go/internal/core/raw_history.go` (new)
- `apps/core-go/internal/core/raw_history_test.go` (new)
- `apps/core-go/internal/core/conversation_delivery_regression_test.go`

- [x] 新增`RawHistoryEvent`、query/search/read-source ports和PostgreSQL adapters，组合Conversation message、Cognition fact、frozen action/outcome等已有authority，不复制成第二个全量event table。
- [x] Migration添加message `turn_id/source_fact_id/correlation_id`、conversation FTS和两类sequence unique indexes；历史不可验证link保持NULL。
- [x] 重构新turn短事务，使user message、claimed `conversation.turn` fact、workflow intent与outbox原子提交；Provider仍在事务外。
- [x] Assistant message绑定同一turn/source/correlation；不改变现有failed-provider时user/fact durable和retry identity。
- [x] 添加pagination、scope、source-link、atomic rollback、duplicate sequence migration fail-closed和1000+ raw rows测试；纯单测通过，PostgreSQL cases延迟到S12。
- [x] Rollback point: revert source columns/reader/atomic enqueue together；additive migration尚未被后续Active/Summary依赖。

### S03. Active Memory authority

Stage file allowlist:

- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`
- `apps/core-go/internal/core/active_memory.go` (new)
- `apps/core-go/internal/core/active_memory_lifecycle.go` (new)
- `apps/core-go/internal/core/active_memory_test.go` (new)
- `apps/core-go/internal/core/context_reference.go`
- `apps/core-go/internal/core/context_reference_test.go`
- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/provider_context_test.go`
- `apps/core-go/internal/core/reflection_v2.go`
- `apps/core-go/internal/core/reflection_memory.go`
- `apps/core-go/internal/core/reflection_runtime_v2.go`
- `apps/core-go/internal/core/reflection_memory_test.go`
- `apps/core-go/internal/core/provider_schemas.go` (S03 extension approved 2026-09-11)
- `apps/core-go/internal/core/provider_schemas_test.go` (new; S03 extension approved 2026-09-11)
- `apps/core-go/internal/core/builtin_capabilities.go`
- `apps/core-go/internal/core/capability_core_test.go`
- `apps/core-go/internal/core/tool_contract_test.go`

- [x] 添加`active_memories`与`active_memory_revisions`，闭合kind/status/operation/time precision约束、FK/index、active exact identity和idempotency。
- [x] 实现唯一`applyActiveMemoryCommandTx` authority：create/confirm/revise/complete/expire/supersede、expected revision、full snapshots、request digest和replay。
- [x] 实现hard filters、read-time expiry和ranking components；过期项先排除，选中/淘汰原因可trace。
- [x] 扩展opaque ContextReference支持Active Memory，Provider只见bounded semantic fields。
- [x] 实现`active_memory_event` direct Thin Capability与built-in注册；runtime绑定scope/time/evidence/idempotency，Provider输入无ID/revision/scope。
- [x] Reflection schema增加closed active candidates并与proposal/watermark/Long-term candidates同事务应用。
- [x] 证明Main cognition可同轮记录“明早7点航班”且无第二个Memory LLM；异步Reflection是兜底而非唯一来源。
- [x] 添加time-zone、unknown precision、expire/complete/supersede、CAS/replay、optional failure与跨Recent测试。
- [x] Rollback point: Active authority尚未进入production prompt时可整体回滚，不触碰existing durable Memory。

### S04. Conversation Summary projection

Stage file allowlist:

- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`
- `apps/core-go/internal/core/conversation_summary.go` (new)
- `apps/core-go/internal/core/conversation_summary_test.go` (new)
- `apps/core-go/internal/core/mutations.go`
- `apps/core-go/internal/core/cognition.go`
- `apps/core-go/internal/core/workflow_ops.go`
- `apps/core-go/internal/core/provider.go`
- `apps/core-go/internal/core/provider_schemas.go`
- `apps/core-go/internal/core/provider_schemas_test.go` (new)
- `apps/core-go/internal/workflow/runtime.go`
- `apps/core-go/internal/workflow/workflow.go`
- `apps/core-go/internal/workflow/workflow_test.go`

- [x] 添加source-bounded `conversation_summaries` projection、revision/supersede语义、source digest与Provider provenance。
- [x] 在assistant settlement后仅按old-chunk estimated size/message count生成稳定`conversation.summary` workflow intent；保留Recent reserve。
- [x] 使用现有Temporal runtime和reflection role读取原始message range并生成strict summary；不输入旧summary。
- [x] 确保Provider/result commit crash用稳定request/workflow ID恢复，失败不丢Raw/Recent。
- [x] 实现summary query/ranking与opaque ref；只有matching conversation且在summary budget内才进入Working Memory。
- [x] 添加multi-chunk、rebuild、source drift、provider failure/retry、no-recursive-summary和Raw rows unchanged测试。
- [x] 更新Temporal registry/version/replay tests；不引入第二个scheduler/queue runtime。
- [x] Rollback point: disable/remove summary intent consumer；additive summaries保持闲置，不影响Raw/Recent。

### S05. Working Memory and total Prompt Budget

Stage file allowlist:

- `apps/core-go/internal/core/working_memory.go` (new)
- `apps/core-go/internal/core/working_memory_test.go` (new)
- `apps/core-go/internal/core/prompt_context_assembler.go` (new)
- `apps/core-go/internal/core/prompt_context_assembler_test.go` (new)
- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/provider_context_test.go`
- `apps/core-go/internal/core/provider_prompt_composer.go`
- `apps/core-go/internal/core/provider_prompt_composer_test.go`
- `apps/core-go/internal/core/provider.go`
- `apps/core-go/internal/core/provider_test.go` (new)
- `apps/core-go/internal/core/settings.go`
- `apps/core-go/internal/core/operations.go`
- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`

- [x] 实现conservative estimator、`PromptFragment`、`WorkingMemoryResolver`、sectioned budgets和deterministic whole-item selection。
- [x] `model_roles`增加context window/max input/policy version；现有`token_budget`继续只表示output reserve。
- [x] 默认按本地实际模型配置65536 context/49152 max input/4096 output/4096 safety；验证预算算式。
- [x] 保护System、Tools/Schema、Current Input；Required总量超限typed fail，Optional按优先级整项drop。
- [x] Recent按估算budget从新到旧选择完整message/turn再恢复chronological order；不尾截断。
- [x] Active/Long-term/Summary交叉dedupe，不删除未选sources。
- [x] 添加1000 Raw +100 Long-term +30 Active压力fixture，断言完整Provider input不超hard budget。
- [x] Rollback point: resolver/assembler仍未切换production callers，可删除新实现且不影响storage。

### S06. B-layout Context Assembly cutover

Stage file allowlist:

- `apps/core-go/internal/core/mutations.go`
- `apps/core-go/internal/core/cognition_growth.go`
- `apps/core-go/internal/core/autonomy.go`
- `apps/core-go/internal/core/wakeup.go`
- `apps/core-go/internal/core/reflection_runtime_v2.go`
- `apps/core-go/internal/core/provider.go`
- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/provider_prompt_composer.go`
- `apps/core-go/internal/core/provider_prompts.go`
- `apps/core-go/internal/core/provider_context_test.go`
- `apps/core-go/internal/core/provider_prompt_composer_test.go`
- `apps/core-go/internal/core/provider_prompts_test.go`
- `apps/core-go/internal/core/life_context_e2e_test.go`
- `apps/core-go/internal/core/provider_live_tool_test.go`

- [x] 将conversation、native cognition、daily review、wake-up迁移到唯一assembler；Reflection/Summary复用role-specific policy，media_prompt维持显式旁路。
- [x] System只含stable runtime rules + filtered Core Persona；移除dynamic actor/relationship system section。
- [x] Runtime Context作为独立delimited user message；Current Input成为最后一条user message且恰好一次。
- [x] Recent user/assistant使用真实transport roles，保留bounded sender/time语义；不伪造tool history。
- [x] Tools继续只经`RenderCapabilityTools`，response schema与Tools参与总预算。
- [x] 删除巨型`{current_message,text,context}` production envelope、current双重渲染与Recent TOON production path。
- [x] 更新single-leading-system、fact/rule boundary、group actor、unsafe quoted history和full request capture tests。
- [x] 新增选定B的full production schema opt-in fixture，但本阶段不运行；Live Provider执行严格留到S12。A/C只保留在experiment harness。
- [x] Rollback point: B-layout cutover作为独立commit可恢复；storage/memory authority不回滚。

### S07. Automatic Retrieval and Memory selection

Stage file allowlist:

- `apps/core-go/internal/core/memory_retrieval.go`
- `apps/core-go/internal/core/memory_retrieval_test.go`
- `apps/core-go/internal/core/intelligence.go`
- `apps/core-go/internal/core/intelligence_test.go`
- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/provider_context_test.go`

- [x] Query cue加入bounded recent topic/messages、selected Active、Current State，同时保留goals/intentions/outcomes/hypotheses。
- [x] 修复`created_at DESC LIMIT 200`先截断blind spot，使授权后FTS relevance可召回更老结果。
- [x] 保持owner/visibility/actor/conversation/type/status/time在candidate limit/ranking/vector前。
- [x] Embedding egress只有显式field-level policy开启；未开启/失败时trace诚实标记lexical fallback。
- [x] 扩展selected/dropped score component trace，不暴露给Provider/browser。
- [x] 添加old-but-relevant、irrelevant bulk、scope-before-limit、embedding failure和ABA current-lineage tests。
- [x] Rollback point: retrieval ranking独立commit；可恢复旧ranking但保留Active/Prompt budget。

### S08. `memory.recall` Thin QUERY

Stage file allowlist:

- `apps/core-go/internal/core/memory_recall_capability.go` (new)
- `apps/core-go/internal/core/memory_recall_capability_test.go` (new)
- `apps/core-go/internal/core/builtin_capabilities.go`
- `apps/core-go/internal/core/capability_core_test.go`
- `apps/core-go/internal/core/tool_contract_test.go`
- `apps/core-go/internal/core/memory_retrieval.go`
- `apps/core-go/internal/core/raw_history.go`
- `apps/core-go/internal/core/conversation_summary.go`
- `docs/capability-schema-report.md`

- [x] 注册conversation-surface `memory.recall`：intent-only、read-only、pure query、`SlotMemoryScope`、bounded output。
- [x] 注入窄`MemoryRecallService`，不持有`*App`；frozen scope只提供authorization，按intent执行新的deep query。
- [x] 搜索Active、Long-term、older authorized messages与Summary；限制12项/3072 estimated tokens。
- [x] 把raw结果映射为opaque ref + bounded semantics；不泄露DB IDs、visibility、revision、evidence、plan IDs或internal score。
- [x] 证明Automatic Top-K与explicit recall结果范围不同，relationship query等既有pure query行为不回归。
- [x] 更新built-in count、surface catalog和schema bytes/chars报告。
- [x] Rollback point: capability本身可独立移除；不需要改Capability core/codec/Registry。

### S09. Generic pure-QUERY continuation coordinator

Stage file allowlist:

- `apps/core-go/internal/core/query_continuation.go` (new)
- `apps/core-go/internal/core/query_continuation_test.go` (new)
- `apps/core-go/internal/core/mutations.go`
- `apps/core-go/internal/core/cognition.go`
- `apps/core-go/internal/core/provider.go`
- `apps/core-go/internal/core/provider_schemas.go`
- `apps/core-go/internal/core/provider_schemas_test.go` (new)
- `apps/core-go/internal/core/capability_core_test.go`
- `apps/core-go/internal/core/project_health_architecture_guard_test.go`

- [x] Response schema加入required `response_mode=final|query_continuation`并迁移normalization。
- [x] 仅当no visible + 1..2 invocations + all metadata-classified pure_query时冻结continuation；ACTION/mixed拒绝。
- [x] Prepare/execute query在business transaction外，持久化results与continuation phase/digest/request ID。
- [x] 由canonical Invocation构造assistant tool-call + `role=tool` results；sidecar/native共享同一serialization。
- [x] 第二次Provider调用复用frozen B context，不发送Tools，只接受visible-text-only closed schema。
- [x] 实现requested/queries_completed/provider_completed replay；query/result/provider/settlement crash不重复。
- [x] Continuation query失败显式失败；不得无结果生成答案。
- [x] 替换blanket no-tool-role guards为dedicated coordinator allowlist；禁止concrete capability name、third call、新tools/new mutations。
- [x] 添加final reply、pure query、mixed/action、two-query limit、malformed continuation、retry/supersede/concurrent turn测试。
- [x] Rollback point: coordinator独立commit；回滚后`memory.recall`仍可作为later-cognition query，Automatic Retrieval不受影响。

### S10. Observability and diagnostics

Stage file allowlist:

- `apps/core-go/internal/core/diagnostics.go`
- `apps/core-go/internal/core/diagnostics_test.go` (new)
- `apps/core-go/internal/core/provider.go`
- `apps/core-go/internal/core/provider_test.go` (new)
- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`
- `apps/core-go/internal/core/prompt_context_assembler.go` (S10 extension approved 2026-09-12)
- `apps/core-go/internal/core/provider_context.go` (S10 extension approved 2026-09-12)
- `apps/core-go/internal/core/mutations.go` (S10 extension approved 2026-09-12)
- `apps/core-go/internal/core/cognition_growth.go` (S10 extension approved 2026-09-12)
- `apps/core-go/internal/core/autonomy.go` (S10 extension approved 2026-09-12)
- `apps/core-go/internal/core/wakeup.go` (S10 extension approved 2026-09-12)
- `apps/core-go/internal/core/reflection_runtime_v2.go` (S10 extension approved 2026-09-12)

- [x] Persist estimated/actual total input/output、latency、context/budget policy和per-section counts/tokens。
- [x] Persistpersona-scoped selected/dropped refs/reasons/score components和continuation phase；普通日志只写bounded counts/digests。
- [x] 将Provider `usage.prompt_tokens/completion_tokens`规范化并与estimator偏差关联。
- [x] 证明Tools/response schema参与trace，credentials/hidden reasoning/raw Core IDs不进入普通API/browser。
- [x] 添加redaction、retention/prune和diagnostics-failure-nonfatal tests。

### S11. One-authority cleanup and contract updates

- Stage file allowlist: only production files already modified in S02-S10, `README.md`, `docs/capability-architecture.md`, and `.trellis/spec/backend/{fluctlight-memory-contract,fluctlight-provider-contract,fluctlight-cognitive-runtime,structured-turn-contract,fluctlight-persistence-contract,fluctlight-workflow-contract,fluctlight-diagnostics-contract}.md`.
- `apps/core-go/internal/core/capability_prompt_policy.go` (S11 extension approved 2026-09-12)
- `apps/core-go/internal/core/query_continuation_test.go` (S11 regression-test extension approved 2026-09-12)
- `.trellis/spec/backend/shared-scene-contract.md` (S11 stale-contract extension approved 2026-09-12)
- `.trellis/spec/backend/error-handling.md` (S11 stale-contract extension approved 2026-09-12)
- [x] 删除旧production prompt assembly/relationship-system/current duplication/recent table路径；`rg` guard禁止回归。
- [x] 确认没有第二个Memory SQL authority、第二个Capability runtime、第二个workflow engine或legacy role feature flag。
- [x] 更新README、`docs/capability-architecture.md`及backend Memory/Provider/Cognitive/Structured Turn/Persistence/Workflow/Diagnostics contracts。
- [x] 将“绝对零continuation”改为“direct conversation默认一次；唯一generic pure-query例外”，并明确ACTION/mixed保持一次。
- [x] 记录migration、default budget、Role experiment、schema stats和删除清单。
- [x] PRD acceptance逐项映射到tests/evidence；未验证项保持未完成。

### S12. Full verification

- Stage file allowlist（write）：无。S12只执行验证；若发现失败，记录evidence并退回拥有该文件的S02-S11阶段，在该阶段allowlist内修复后重新进入S12。
- [x] Focused unit/integration tests按每阶段即时运行。
- [x] Core full test/race/vet/build。
- [x] Gateway full test/race/vet/build（若API/settings contract变化则必须）。
- [x] Real PostgreSQL empty→head、previous-head→head、rerun、duplicate/malformed rollback、Active/Summary/Raw provenance transactions。
- [x] Temporal conversation-summary workflow fresh/restart/replay/versioning/duplicate dispatch。
- [x] Local live Provider：B full schema、Active flight extraction、fact/persona/instruction boundary、three tool decisions、pure-query continuation。
- [x] `gofmt -l`、`git diff --check`、architecture guards、generated Core OpenAPI checks。
- [x] 检查worktree只包含本任务改动，无关`.trellis/tasks/09-08-project-health-evolution/`未被触碰。
- [x] `check-go-projections.sh`：用户于2026-09-12明确批准环境性豁免；本任务无BFF/API projection shape变更，且独立smoke不创建已认证Moment/Proactive业务fixture。

### S13. Final report and Trellis finish

- [x] 生成最终implementation evidence与用户要求的13类报告。
- [x] 明确回答Raw growth、flight Active、expiry、Long-term on-demand、fixed budget、Conversation/Memory分离、System动态污染、real roles、Runtime/current分离、最佳Role十个问题。
- [ ] 如有真实实验失败，保留原始结果与影响，不为了证明成功调prompt或隐藏。
- [x] 执行`trellis-check`全量质量门禁；根据实现学习更新spec；按Trellis流程完成工作提交，并进入归档/journal收尾。

## Acceptance Mapping

| PRD | Primary implementation/test stages |
| --- | --- |
| AC1 Raw growth without prompt growth | S02, S05, S12 |
| AC2 flight survives Recent | S03, S05, S12 |
| AC3 flight expiry | S03, S12 |
| AC4 relevant Long-term only | S07, S12 |
| AC5 ABA | S03, S07, S12 |
| AC6 Summary drift/rebuild | S04, S12 |
| AC7 total Prompt Budget | S05, S06, S10, S12 |
| AC8 Rule/Fact boundary | S06, live role tests |
| AC9 real roles/current input | S06 |
| AC10 A/B/C real experiment | S00, S06, S12 |
| AC11 Automatic + recall/continuation | S07-S09 |
| AC12 timely, bounded observation | S03, S04, Reflection tests |
| AC13 one assembly authority | S06, S11 |
| AC14 full quality gate | S12 |
| AC15 final report | S13 |

## Risk and Rollback Matrix

| Risk | Prevention | Rollback boundary |
| --- | --- | --- |
| Migration blocks existing data due duplicate sequence | Preflight/read-only duplicate query, real PostgreSQL malformed fixtures | Revert S02 migration/code before later schema depends on it |
| Active Memory becomes a second Long-term store | Separate closed kinds/status, no embedding/promotion by default, existing durable Memory remains sole long-term authority | Disable Active injection/capability; preserve rows for audit |
| Main model omits Active candidate | Same-call thin capability plus existing Reflection fallback; live flight fixture | Keep Raw/Recent behavior; diagnose candidate omission, no heuristic extraction |
| Summary drift | Every build reads only Raw source range; digest and refs mandatory | Stop summary intents; prompts fall back to Recent/Raw recall |
| Estimator undercounts actual tokens | 1.25 multiplier, message overhead, 8192 unused headroom, actual usage diagnostics | Lower max input/config without schema rollback |
| B layout hurts model/tool behavior | Two-run A/B/C evidence plus full production-schema live gate | Revert S06 commit; storage remains compatible |
| Older relevant Memory still crowded out | Authorization-aware FTS selection before candidate limit, exact vector only if allowed | Revert ranking independently |
| Recall leaks raw IDs/evidence | Provider-safe mapper and output-schema/redaction tests | Disable capability catalog entry |
| Continuation duplicates model/query/effects on crash | Frozen phase/result digest/request ID and replay tests | Revert S09; recall remains later-cognition only |
| Continuation expands to ACTION | Generic execution-class validation and production-tree guards | Guard failure blocks merge |
| Dynamic facts return to system | One assembler + section/role tests + production `rg` guard | Revert offending renderer change |
| External I/O enters transaction | Existing UoW contract plus transaction-boundary tests | Fail review; move I/O before/after tx |

## Required Validation Commands

**S12 only. Running any command in this section during S01-S11 violates the task boundary.**

Final commands:

```bash
GOCACHE=/tmp/lac-prompt-memory-core-test go -C apps/core-go test ./... -count=1
GOCACHE=/tmp/lac-prompt-memory-core-race go -C apps/core-go test -race ./... -count=1
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
GOCACHE=/tmp/lac-prompt-memory-gateway-test go -C apps/gateway-go test ./... -count=1
GOCACHE=/tmp/lac-prompt-memory-gateway-race go -C apps/gateway-go test -race ./... -count=1
go -C apps/gateway-go vet ./...
go -C apps/gateway-go build ./...
test -z "$(find apps/core-go apps/gateway-go -name '*.go' -print0 | xargs -0 gofmt -l)"
git diff --check
```

Focused tests will use exact `-run` names created in S02-S10. Real PostgreSQL tests use an isolated disposable database via `GO_CORE_TEST_DATABASE_URL`; never point destructive migration fixtures at a user database.

Local live Provider gate:

```bash
FLUCTLIGHT_LIVE_PROVIDER_TEST=1 \
FLUCTLIGHT_LIVE_PROVIDER_URL=http://127.0.0.1:11234/v1 \
FLUCTLIGHT_LIVE_PROVIDER_MODEL=huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX \
GOCACHE=/tmp/lac-prompt-memory-live-provider \
go -C apps/core-go test ./internal/core -run 'TestLiveProvider(RoleOrganization|ActiveMemory|RecallContinuation)' -count=1 -v
```

Compose/acceptance commands, after creating an isolated env from the example:

```bash
docker compose --env-file infra/compose/fluctlight.env -f infra/compose/fluctlight.compose.yml up -d --build
FLUCTLIGHT_ENV_FILE=infra/compose/fluctlight.env ./infra/compose/run-platform-smoke.sh
./infra/acceptance/check-compose-bind-sources.sh
./infra/acceptance/check-core-openapi.sh
./infra/acceptance/check-go-projections.sh
```

Do not create `infra/compose/fluctlight.env` by copying over an existing user file; inspect first and use an isolated task-specific env/project name.
