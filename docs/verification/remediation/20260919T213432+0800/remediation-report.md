# 摇光第五阶段验收失败后的定点整改报告

本报告对应整改工作树 `codex/yaoguang-adk-phase2` 的当前未提交快照。它是独立整改证据，不覆盖历史第五阶段报告，也不把历史报告中的 PASS 继承为本次结论。

## 总结

| 门禁 | 状态 | 依据 |
|---|---|---|
| G0 工具可信度 | PASS（工具门禁）；整体清零门禁 FAIL | `evidence/tests/G0-self-tests.txt`、`evidence/commands/G0-gate.txt`、`evidence/commands/G0-clear-gate.txt` |
| G1 模型/Prompt/Task 边界 | PASS（可执行本地范围） | `evidence/code/F01-provider-boundary.txt`、`evidence/code/G1-task-prompt-contract.txt`、`evidence/tests/G1-task-boundary.txt` |
| G2 残留与运行风险 | PASS | `evidence/tests/R01-after-fix.txt`、`evidence/tests/R02-after-fix.txt`、`evidence/tests/F02-browser-compatibility.txt` |
| G3 隔离业务回归 | 部分 PASS，整体 BLOCKED | `evidence/tests/G3-handle-turn-db.txt`、`evidence/runtime/G3-temporal-active-workflow.txt`、`evidence/runtime/G3-browser-smoke.txt` |

整体不能标记为“本地整改全部清零”：原始 F-03 联合业务回归、原始 F-06 的完整快照/成本证据、R2-02 真实卡 typed migration、R2-04 同卡性能对照、无 BFF 浏览器业务 smoke 和真实 Provider 仍为 BLOCKED。`--require-clear` 明确返回非零。

## 二审问题身份映射

第二次外部复核指出，上一版整改报告把原始 F-03～F-06 重新赋予了新含义。本报告恢复原问题身份；新增发现使用 `R2-*` 编号，不覆盖原编号。

| 原问题 | 本报告身份 | 当前结论 |
|---|---|---|
| F-01 Task/模型支撑耦合 | F-01 + G1 Task 边界 | PASS（本地可验证范围） |
| F-02 BFF/字段迁移兼容残留 | F-02 | PASS |
| F-03 关键业务、数据库、后台、浏览器、模型联合回归缺口 | F-03 | BLOCKED：数据库 HandleTurn 与 Temporal 已有实证，但完整业务叶子集合、浏览器和真实 Provider 仍缺环境/证据 |
| F-04 验收 CSV 列错位 | F-04 | PASS：标准 CSV writer/reader 与回读门禁 |
| F-05 测试选择器漏执行、父 PASS 掩盖子 SKIP | F-05 | PASS：固定预期集合、原始 JSON 事件、conditional skip 和进程退出码门禁 |
| F-06 Prompt/Task/人格/提交/运行/快照证据不足 | F-06 | BLOCKED：离线 Prompt/Task 检查已通过，完整同卡成本与全部业务快照仍待补 |
| 二审 R2-01 Judge 前参数级能力授权 | R2-01 | PASS：冻结 scope/relationship 参数验证与 Judge 前回归 |
| 二审 R2-02 真实卡 typed takeover migration | R2-02 | BLOCKED：固定 fixture 通过，live Provider 未配置 |
| 二审 R2-03 B Overlay 失败语义 | R2-03 | PASS：既有架构契约明确允许 `judge_degraded` 保留 A；失败不使用 baseline、不调用 Judge |
| 二审 R2-04 同卡完整 Prompt 成本对照 | R2-04 | BLOCKED：离线字节/估算预算已测，真实缓存/延迟/计费未测 |
| 二审 R2-05 报告口径冲突 | R2-05 | PASS：本报告与矩阵改为独立整改口径，历史报告保留为历史文件 |

## F/R 逐项结论

### F-01：模型运行支撑不再反向构造 App — PASS

之前的正式 Provider、队列、Eino 模型路径使用 `&App{DB: ...}` 进入审计和生命周期写入。现已抽取窄接口 `ProviderRuntimeSupport`，由 `NewApp` 组合根装配；Provider/queued model/diagnostics 只依赖该支撑，未增加第二套队列或审计表。

证据：

- `evidence/code/F01-provider-boundary.txt`
- `evidence/tests/G1-task-boundary.txt`
- `evidence/commands/G3-core-full.txt`、`G3-core-race.txt`、`G3-core-vet.txt`

### F-02：迁移兼容残留 — PASS

删除浏览器 `NewServer → New` 兼容别名，固定 Core snake_case → Browser camelCase 映射；删除 `actorId/actor_id`、`sessionToken/session_token`、`setupAvailable/setup_available` 及诊断/provider DTO 的 snake/camel 双猜测。合法 DTO 转换仍保留。

证据：`evidence/code/F02-browser-compatibility.txt`、`evidence/tests/F02-browser-compatibility.txt`。

### F-03：关键业务联合回归缺口 — BLOCKED

原始 F-03 不是 typed takeover migration。当前已经取得的可复核范围是：Disposable PostgreSQL 下正式 `HandleTurn` 工具回填、单条 assistant 提交通过；Disposable Compose 下 Core/Worker/Redis/Temporal/MinIO 健康和 `PlatformControlWorkflow` 通过。完整 F-03 仍缺：B 被拒/接管后的全量副作用、WakeUp 主动/无动作生命周期、Outbox/重试/取消/并发隔离和真实浏览器链路。

对应新增问题 R2-02 的真实卡结论见下节。

### R2-02：真实卡 typed takeover migration — BLOCKED

Runtime 只消费显式 `takeover_rules[]`，旧 `forced_activation` prose 不再由 Go 关键词/正则推断；真实卡与固定 typed migration fixture 的本地归一化测试通过。但本轮无授权 live Provider，不能证明 `dense_multi_card.txt` 经真实初始化模型产出 typed rule，因此不勾选完成。

证据：`evidence/tests/F03-typed-migration.txt`。

### F-04：验收 CSV/证据工具 — PASS

原始 F-04 的 CSV 列错位已由标准 writer/reader 和矩阵回读门禁覆盖；证据路径必须属于本次 evidence 目录。详见 G0 证据。

### F-05：测试选择器与执行完整性 — PASS

原始 F-05 已由固定预期集合、完整 Go JSON 事件、父/子测试路径比对、conditional skip 原因和 `PIPESTATUS` 退出码校验覆盖。单个条件性测试可以 SKIP，但不会被父测试 PASS 掩盖，也不会进入 clear gate。

### R2-01：Judge 前参数级能力授权 — PASS

`relationship.lookup.target_actor_id` 使用冻结 relationship scope 做无副作用参数验证；候选 surface、身份、target kind、required ContextSlot 均在 Judge 前验证。纯 hook 测试不进入 DB/网络 Prepare。

### R2-03：B Overlay 失败语义 — PASS

既有架构契约 [`docs/persona-takeover-architecture.md:64`] 明确规定：Overlay clone/load/compose 失败记录稳定错误码、跳过 Judge 并走 `judge_degraded` 保留 A；只有明确“无 Overlay 记录”才使用 baseline。当前实现符合这一既有契约，而不是在本轮新增“失败即发布 A”的降级语义。新增正式 `HandleTurn` 回归证明：失败只提交 A 一次、Judge 调用数为 0、active/reply owner 保持 A、失败码进入冻结 takeover 记录，且不使用 stale baseline。

证据：`evidence/tests/F04-F05-db.txt`、`evidence/tests/R2-03-overlay-production.txt`。

### F-06：Prompt/Task/人格/提交/运行/快照证据 — BLOCKED

离线部分已经通过：Task 全入口迁移、Prompt Slot/预算、tools/schema 估算、ADK 调用次数和快照 hash 均有固定输入证据。仍未完成的是原始 F-06 要求的完整联合快照：同一真实复杂卡的 full request、Working Persona、Judge 前后成本、人格/提交/运行全链路和真实缓存/延迟。它们不因为真实 Provider 缺失而把离线检查一并标成未执行；离线与 live 现在分开记录。

证据：`evidence/code/G1-task-prompt-contract.txt`、`evidence/tests/G1-task-boundary.txt`、`evidence/tests/R2-04-offline-prompt.txt`、`evidence/commands/G3-core-full.txt`。

### R2-04：同卡完整 Prompt 成本对照 — BLOCKED

固定 Prompt/Task 输入下的消息、tools、schema、估算 token、output reserve 和裁剪结果不需要真实推理，已由现有 Prompt assembler 测试覆盖；离线测试还记录了 `system/tools/schema/required/max_input` 的实际边界。真实 tokenizer/chat-template、Provider usage、缓存命中和延迟没有配置，保持 BLOCKED，不把它们升级为新的架构前置条件。

### R2-05：报告口径冲突 — PASS

本次矩阵恢复原始 F-01～F-06 定义，typed takeover migration、Overlay 语义和性能成本使用 `R2-*` 独立编号；历史报告不被覆盖，仅作为历史材料保留。

### R-01：Stream 生命周期 — PASS

旧路径在返回 `StreamReader` 后立即取消 queue lease；新增 wrapper 让 lease、watcher 和诊断状态持续到物理 EOF/Close，外部取消主动关闭 inner reader。并发上限为 1 的 lease holding、取消关闭和 race 覆盖通过。

证据：`evidence/tests/R01-before-fix.txt`、`evidence/tests/R01-after-fix.txt`、`evidence/code/R01-stream-lifecycle.txt`。

### R-02：最终回复回退旧候选 — PASS

`RunADKLoop` 只把最后一个 assistant event 作为最终事件；后续空 assistant 不再回退前一轮文本。另对中间轮的 `conversation.reply` 做输出源收敛：当最终 assistant 已给出 root visible text 时，中间 deferred reply 不再制造第二个 visible-text source；纯查询等非输出 capability 仍保留。正式 DB `HandleTurn` 工具回填测试通过。

证据：`evidence/tests/R02-before-fix.txt`、`evidence/tests/R02-after-fix.txt`、`evidence/tests/G3-handle-turn-db.txt`、`evidence/code/R02-final-reply-authority.txt`。

### R2-06：Life Context surface authority — PASS

此前广义 `compactCognitionContext` 能保留 Life Context opaque ref，但
Conversation/WakeUp/Daily Review 使用的 surface-specific 投影会丢掉 `ref`、事件/日程/存在
关联 ref 和 authority revision，导致模型请求无法携带与冻结决策对应的 Life Context authority。
现已让 `compactLifeContextForSurface` 使用 Core-owned `ContextReferenceIndex` 校验并保留
合法 opaque refs 与 `life_ctx_[32 hex]` revision，同时继续拒绝原始数据库 ID。四项定点回归验证
了模型可见上下文、不同场景导致不同 planner 结果，以及决策后 Life Context 变更时的 stale
拒绝。

证据：`evidence/code/L1-life-context-authority.txt`、`evidence/tests/L1-life-context-authority.txt`、`evidence/tests/R2-focused-regressions.txt`

### R2-07：ADK deferred action round 收束 — PASS

动作/输出 capability 返回 `deferred` 或 `rejected` 时，第二次物理 Provider 请求不能被 ADK
无条件触发。queued ADK model 现在只在紧邻的 tool result 全部为 `deferred`/`rejected` 且满足
surface 的 visible-text policy 时复用上一轮 assistant proposal；completed pure-query 结果仍
继续一次正常 Provider cognition，Conversation 在第一轮没有 visible text 时仍允许补生成最终
回复，WakeUp 可直接收束 action-only 回合。回归测试同时确认中间 deferred reply 不会覆盖非输出
capability 或最终 reply。

证据：`evidence/code/R2-06-adk-deferred-round.txt`、`evidence/tests/R2-06-adk-deferred-round.txt`、`evidence/tests/R2-focused-regressions.txt`

## G1 Task/Prompt 迁移

普通模型入口已迁移到 typed Task：

| Task | 业务输入 | Task 内负责 | 调用方保留 |
|---|---|---|---|
| Media Prompt | frozen media intent | media prompt instruction、concept/retry slot、Text 调用 | intent 持久化与 provider job |
| Media Quality | intent + candidate bytes | 多模态消息、schema、结果 normalize | quality 状态与 retry |
| Visual Identity Vision/Patch | identity snapshot、constraints、candidate image | 固定 system fragment、multimodal schema、结构化解析 | timeline/asset 事务 |
| Conversation Summary | typed source messages | summary instruction、schema、response decode | source identity 与 settlement |
| Schedule Generation/Replan | typed schedule facts | schedule instruction、schema、retry reminder | 日期连续性、事务接收 |
| Native Cognition/Daily Review/Reflection/Persistent Switch | projection + domain facts | surface、operation rules、capability catalog、schema、Prompt assembly、Provider 调用 | domain freeze/prepare/settlement |
| Initialization | Owner description | canonical initialization instructions/schema | initialization semantic validation |

ADK lower-level bridge 现在只接收 `PromptAssemblyResult`，不再暴露独立 raw `Messages`/`Schema` 字段；主对话、接管回复和 WakeUp 继续使用同一 ADK/Eino loop 与 request-scoped capability trace。

## G3 隔离回归

- Disposable PostgreSQL：真实 migration head `0033_initialization_source`，正式 `HandleTurn` + Fake Provider 工具回填并只提交一个 assistant，通过。
- Disposable Compose：PostgreSQL、Redis、Temporal、MinIO、Core、Worker、Web 健康检查通过；`PlatformControlWorkflow` 完成，history length、namespace、task queue、deployment `fluctlight:platform-v1` 均通过；资源由唯一项目名和 `down -v` 清理。
- Browser actor smoke：不得伪造通过。第二次 Compose build 遇到 Docker registry `alpine:3.22` TLS handshake timeout，脚本已修复 null ID 假绿问题并将本次状态标为 BLOCKED。
- Live Provider/Embedding：环境未提供，BLOCKED。

## 反例检查

- Provider/queue/diagnostic 路径没有 `&App{DB: ...}`；旧 wrapper 名称不再出现在生产调用方。
- 所有普通模型入口都使用具体 Task；不是只迁移 media/reflection 示例。
- G0 不能用父测试 PASS 覆盖子测试 SKIP，不能通过日志管道隐藏非零退出码，不能用仓库外证据路径冒充本次证据。
- 最后一轮 Go full/race/vet/gofmt、browser tests、Compose active workflow 和 DB HandleTurn 证据均来自本整改工作树的最后源码快照。

本轮追加的 Life Context 与 ADK 定点回归均已通过；完整 Core 测试仍需按下列口径阅读：临时
PostgreSQL 缺少部分测试表会造成 schema 环境失败，另有 persistent switch/native cognition/
reflection 等既有领域测试失败。这些结果没有被本轮定点 PASS 覆盖，也没有被报告为“全量通过”。
