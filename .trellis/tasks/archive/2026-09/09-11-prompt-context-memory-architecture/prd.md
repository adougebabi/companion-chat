# Prompt Context 与 Memory 架构重构

## Goal

在现有 Go Core 与 Thin Capability Runtime 之上重构 Prompt Context Assembly 和 Memory System，使 Raw History 与 Memory Store 可以持续增长，而每次 Main LLM Cognition 的输入仍保持有限、稳定、可预测，并只包含与当前认知相关的信息。

最终系统必须明确区分：

- Raw History：实际发生过什么，是 append-oriented Source of Truth。
- Recent Context：刚刚发生了什么，是受预算约束的近期投影。
- Active Memory：当前仍然有效、会影响近期行为的临时事实。
- Long-term Memory：跨较长时间仍值得保留、但按需检索的事实与经历。
- Working Memory：当前这一次 cognition 实际意识到什么，是有硬预算的 prompt read model。

## Background

现有 Tool → Thin Capability 架构已经完成或正在收尾。本任务承接 `codex/llm-capability-runtime`，兼容其 canonical `CapabilityDefinition`、Registry、统一 dispatch 与 ACTION 单次 Main Cognition 语义，但不重新设计 Capability 核心。代码盘点已经确认生产路径不存在旧 `CapabilityManifest`，也不存在既有 QUERY continuation；用户已批准本任务新增一个受限的 pure-QUERY-only continuation orchestration seam。

本任务要解决的根因不是“缺少短期/长期记忆”这一分类问题，而是存储增长与 cognition context 增长被错误绑定。`Not in Prompt != Forgotten`：没有进入当前 prompt 的事实仍可以存在于 Raw History、Active Memory 或 Long-term Memory 中，并在需要时被选择或召回。

用户提供的原始需求位于 `/Users/vinson/.codex/attachments/355bb63a-8106-4a0f-95ad-f68779372a65/pasted-text.txt`；本 PRD 是其任务内、可验收的要求投影，不以 summary 取代原始要求。

## Confirmed Repository Facts

- 当前 Main conversation 在 `apps/core-go/internal/core/mutations.go:722-729` 手工发送一条 system 和一条包含 `current_message`、重复 `text`、`context` 的 user message；recent role 仅作为该 user 文档内的数据字段，不是 Provider transport role。
- `apps/core-go/internal/core/provider_prompt_composer.go:42-97` 已统一最终 formatter，但 conversation、native cognition、daily review、wake-up 仍有四个独立 Main request builder；尚无统一的 budget/fragment/role assembly authority。
- 当前只有固定 12 条 recent message 和 2400-rune Memory 预算；System、Persona、State、Schedule、Tools、Response Schema、Current Input 与 Output Reserve 没有统一 input budget。
- Memory 已有四类 durable type、单写 authority、revision/governance、CAS、idempotency、merge/supersede/rollback、硬授权过滤、FTS/embedding 基础与 Automatic Retrieval；但没有独立 Active Memory tier、自动 expiry/promotion、Conversation Summary、显式 `memory.recall` 或全 prompt budget。
- 当前 `cognition_inbox` 是最接近 persona Raw History 的事实流，但同时承担 queue 状态且会原位更新；Conversation、Capability、Life、Moment、Reflection 分属不同 authority，尚无统一可遍历的 immutable Raw History ledger。
- `CapabilityTypeQuery` 与 pure-query execution class 已存在，但 `.trellis/spec/backend/fluctlight-cognitive-runtime.md:73-78`、`structured-turn-contract.md:467-475`、`capability_core_test.go:1020-1028` 和生产代码都明确禁止同轮 `role=tool` continuation。

## Approved Decisions

### D1. Pure-QUERY-only continuation

用户于 2026-09-11 批准为显式主动回想增加一个新的、通用的 conversation orchestration seam：

- 第一次 Main cognition 只有在无法形成最终可见回复且只产生 pure QUERY invocation 时，才能进入 continuation。
- QUERY 在数据库业务事务外执行并先持久化 bounded result；随后最多进行一次不再暴露 Tools 的 continuation。
- Continuation 只生成最终可见回复，不新增 appraisal、状态变化、claims 或 capability mutation。
- ACTION 或 QUERY+ACTION mixed batch 继续保持一次 Main cognition，不允许 continuation。
- 资格判断必须依据 metadata-driven execution class，不能针对 `memory.recall` 写名称分支。
- `CapabilityDefinition`、Registry、codec、通用 Runtime dispatch 不因该例外重新设计。
- 现有 blanket static guards 与契约必须同步改为验证上述受控边界。

### D2. Stage-by-stage isolation and final-only full verification

用户于 2026-09-11 批准开始实现，并规定：

- S01 至 S11 必须严格串行；当前阶段未完成前不得开始下一阶段。
- 每个阶段只能修改 `implement.md` 为该阶段列出的 file allowlist，以及当前任务目录内的计划/evidence 文件。
- 发现必须修改 allowlist 外文件时必须停止，不得顺手修改；先更新计划并重新取得范围确认。
- S01 至 S11 只运行直接覆盖当前阶段新增/修改符号的精确测试，不运行 `./...`、`-race ./...`、Docker Compose、Live Provider 或跨阶段验收。
- 既有失败若不属于当前阶段直接修改文件，只记录到 evidence，不扩散修复。
- 只有 S01 至 S11 的核心功能与阶段单测全部完成并打勾后，S12 才能执行 full test/race/vet/build、PostgreSQL migration、Temporal、Live Provider 与 Docker smoke。
- 无关未跟踪目录 `.trellis/tasks/09-08-project-health-evolution/` 始终禁止修改、删除、格式化或 stage。

## Requirements

### R1. Raw History 与投影边界

- User/assistant message、capability invocation/result、scene change、post、wake-up、reflection 及重要 state event 必须保留可追溯的权威记录。
- Raw History 原则上 append-oriented；summary、memory、working context 只能作为可重建 projection/cache，不能覆盖或成为唯一事实来源。
- Conversation Store 与 Memory Store 必须在语义和代码职责上分离；保存每句聊天不等于为每句聊天创建 Memory Unit。
- Memory 与 prompt 中的条目必须保留足以回溯 source event / evidence 的标识。

### R2. Recent Context

- Recent Conversation、近期 capability result 和重要 event 的选择必须优先受 token budget 约束，不能只依赖固定消息条数。
- Recent Context 淘汰只影响当前 prompt 投影，不删除 Raw History。
- 在 provider/model 协议允许时，近期真实聊天应保留原始 `user` / `assistant` / `tool` role、顺序和说话者语义，而不是统一拼成一个大文本块。

### R3. Active Memory 的实时性与生命周期

- “明天赶飞机”“今晚早点睡”“本周等待结果”“尚未完成的约定”等仍有行为意义的事实，在离开 Recent Context 后仍必须可被识别、保存、排序并进入 Working Memory。
- Active Memory candidate 必须在相关 conversation/event 后及时产生；不能只依赖凌晨 consolidation。
- 提取默认不得在每条消息后额外调用一次大型 LLM、不得令正常聊天成本翻倍。应优先复用当前 structured output、reflection、event、cron 或异步 chunk 机制。
- Active Memory 必须表达当前状态以及必要的 lifecycle、importance、source/evidence 和时间语义；具体字段服从现有数据库与代码风格，不机械照抄概念模型。
- 生命周期至少能区分仍有效、已完成/过期和已被后续事实取代；不是所有 Active Memory 都应 promotion 为 Long-term Memory。
- 可解析的“今天/明天/晚上/下周”等时间表达必须同时保留原文和基于事件时间、用户/角色时区解析出的时间锚点；信息不足时不得猜测精确时间。
- Active Memory 即使总体数量增长，也只能按 temporal urgency、importance、current relevance、last relevant time 等信号在预算内选择，不能全量注入 prompt。

### R4. Long-term Memory 与 consolidation

- Long-term Memory 只保存跨较长时间仍有价值的事实、经历、关系认知、自我认知、稳定偏好、重要人物/目标和重复模式。
- Long-term Memory 默认不全量进入 prompt；必须经过 Automatic Retrieval 或显式 recall。
- 当前同一概念的有效认知必须支持 create/update/merge/supersede（必要时 decay/archive），避免 ABA 场景中同时注入互相矛盾的旧事实；完整变化链仍保留在 Raw History/evidence 中。
- 类型体系优先兼容并整理现有模型，可按 Episodic、Semantic、Relationship、Self 等用途支持 retrieval/consolidation/ranking，但不得为分类引入复杂继承或大量 memory type。
- Daily Consolidation 应复用现有 cron/reflection 等机制，维护去重、可检索的 Memory Projection；不得把每日聊天压成永久注入 prompt 的大段 summary。

### R5. Conversation Summary

- Recent Conversation 超预算时可以生成 Conversation Summary，但 summary 只是有来源范围的 working projection/cache。
- Summary 必须保留 source event IDs 或等价 provenance，并支持从 Raw History 周期性重建或按 chunk/topic 重建。
- 不得把 `old summary + new messages -> overwrite old summary` 作为唯一事实链，避免递归漂移和 ABA 信息丢失。

### R6. Automatic Retrieval 与显式 recall

- 每轮可执行一个低成本、小 Top-K、小 token budget 的 Automatic Retrieval，作为默认的“潜意识联想”。
- Retrieval query 不得只取最后一句 user message；应在有限 query budget 内综合 current input、recent context/topic、current state 和 active context。
- 优先复用项目已有 metadata、keyword/FTS、embedding/vector 和 database search 能力；本任务不因检索立即引入大型向量基础设施。
- 提供或迁移 Thin QUERY Capability `memory.recall`，让 Main LLM 能以 `intent` 主动搜索更老或更深的 Memory/Raw History。
- `memory.recall` 只有在没有 result 无法完成当前回复时，才允许触发获批的 pure-QUERY-only 单次 result-dependent continuation；不得恢复所有工具的 `LLM -> Tool -> LLM`，ACTION 和 mixed batch 仍不 continuation。

### R7. Working Memory 与 Prompt Budget

- Main Cognition 不得直接 `loadAllMemories()`；必须经过 resolver/selection 形成 Working Memory。
- 对 System、Runtime Context、Active Memory、Retrieved Long-term Memory、Conversation Summary、Recent Conversation、Tools、Current Input 与 Output Reserve 建立明确、配置化的总预算和分区预算/保底。
- 即使存在 1000 条 Raw History、100 条 Long-term Memory、30 条 Active Memory，最终 input 仍不得超过配置预算。
- 超预算时按语义优先级选择完整 fragment/message，不允许从整段字符串尾部粗暴截断导致 role、事实或结构损坏。
- 默认优先保障 Core Identity/Policy、Current Input、Critical Active Memory、Current State/Scene，再权衡 Recent Conversation、Highly Relevant Long-term Memory、Summary 和低优先级候选；最终顺序由代码证据与实验确定。
- Token estimator 可以在 provider 无 usage 数据时采用可解释、可测试的保守估算；不得仅为第一版引入复杂 tokenizer。

### R8. Context Assembly 职责

- 建立明确但“薄”的 `ContextAssembler` / `PromptContextBuilder`（名称按代码风格确定），只负责收集已经准备好的 fragments、按预算选择并形成 messages。
- Assembler 不承担 memory extraction/retrieval、persona mutation、tool business logic 或 scene business logic，不得演化为新的巨型 `PromptManager`。
- 如引入 Prompt Fragment，其最小信息应支持 kind、priority、content/message、estimated tokens 与 source/provenance；不得为抽象本身建立复杂框架。
- 完成迁移后只保留一套生产 Context Assembly 权威路径；删除或替换旧的并行拼装逻辑。

### R9. Rule / Fact 与 message role 边界

- System 必须保持小、稳定、高约束、低变化，只包含 stable identity/persona、global policy、capability usage principles 与 context interpretation rules。
- 当前情绪、场景、关系 snapshot、Active/Relevant Memory、summary 和临时状态属于 Runtime Context facts，不得伪装成 System rule 或永久人格。
- Runtime Context 与真实 Current User Message 必须成为可辨识的独立逻辑单元；即使 provider 只支持基础 roles，也必须使用明确边界，不能继续混成一个含 context/history/current input 的巨大 user 字符串。
- 真实模型实验最终选择 B：`System = Stable Rules`、独立 user message 承载 `Runtime Context = Dynamic Facts`、`Recent Messages = Real Conversation roles`、最后一条 `Current User Message = Real Current Input`。

### R10. A/B/C Role Experiment

- 先记录当前生产 Prompt Baseline：system/user/history/tools/response format 的真实组织方式，不能凭印象描述。
- 使用当前实际本地 model/API 对 A（现状 baseline）、B（Runtime Context 独立 user message + recent real roles）、C（Runtime Context 置于 system，作为对照）进行可重复实验。
- 三组实验必须使用完全相同的规则、事实、persona、tool schema 和 user input，只改变 role/message organization，不得为某组单独调 prompt。
- 至少评估 Runtime Fact Understanding、Rule/Fact Boundary、Persona Stability（含“甜妹化”漂移）、Conversation Coherence、Instruction Leakage、Tool Decision Stability、input/output tokens 或 chars/bytes/message count、latency。
- 输出逐组原始/汇总证据和对比表，如实保留失败，并按当前实际模型行为推荐 A、B 或 C。

### R11. Observability

- 每次 cognition 的 trace 至少能解释 system、tools、recent、active、retrieved、summary 和 total input 的估算/实际 token 使用与选中数量。
- Trace 能定位哪些 memory 被选中、主要 ranking 原因，以及哪些候选因预算被淘汰。
- 第一版使用现有日志/trace 体系，不要求新增复杂可视化平台。

### R12. Delivery、迁移与验证顺序

- 严格按照 Inventory → Prompt/Role Baseline → Role Experiment → Memory Architecture → Context Budget → Implementation → Migration → Verification 推进。
- 迁移必须与现有 Thin Capability Runtime 兼容，不重新设计 canonical `CapabilityDefinition`、Registry、codec、统一 dispatch 或 Runtime 核心；获批的 pure-QUERY-only continuation 只在 conversation orchestration 建立受限例外。
- S01 至 S11 按 `implement.md` 的阶段 file allowlist 串行修改并只运行精准测试；全量验证严格延迟到 S12。
- 至少运行 `go test ./...` 和仓库已有相关测试；如果真实模型实验依赖本地服务/模型不可用，必须报告具体环境阻塞和已经完成的 deterministic harness 结果，不得伪造结论。
- 最终报告必须包含：旧架构分析、baseline、实验结果、推荐 role 结构、新 Memory Architecture、新 Prompt Assembly、预算、Active lifecycle、Long-term retrieval、`memory.recall`、被删除/替换的旧逻辑、测试结果、风险与未解决问题。

## Acceptance Criteria

- [x] AC1 — Raw History 可以持续增长，且不存在把完整历史或全部 memory 注入 Main Prompt 的生产路径；大小受控测试证明 prompt 不随 store 线性增长。
- [x] AC2 — “用户明天早上 7 点赶飞机”离开 Recent Context 后仍存在于 Active Memory，并可在当晚相关对话的 Working Memory 中被选中。
- [x] AC3 — 航班事件完成、过期或被取代后不再占用 Active Context；Raw History/provenance 仍保留，且只有值得保留时才进入 Long-term Memory。
- [x] AC4 — 大量 Long-term Memory 中只有与当前 query 相关且通过 hard scope/visibility 过滤的部分进入 prompt；无关 memory 不进入。
- [x] AC5 — ABA 测试（喜欢 A → 不喜欢 A → 重新喜欢 A）只注入当前有效事实，同时保留完整历史来源。
- [x] AC6 — 多轮 summary/consolidation 不删除 Raw History；summary 具有 source boundary，并可从 Raw History 重建或做一致性校验。
- [x] AC7 — 1000 Raw History + 100 Long-term + 30 Active 的测试输入不突破固定、配置化的总 Prompt Budget，且 trace 解释保留/淘汰结果。
- [x] AC8 — Runtime Context 中的“用户明早赶飞机”和“摇光当前疲惫”被组织为动态 facts，而不是永久 persona 或 system commands。
- [x] AC9 — Recent Conversation 在所选 provider role 方案下尽可能保留真实 user/assistant/tool roles，且 Runtime Context 与 Current User Message 明确分离。
- [x] AC10 — A/B/C 实验在当前本地模型/API 上以同内容、仅变 role 的方式完成，并给出 Fact Recall、Persona Stability、Conversation Coherence、Instruction Leakage、Tool Behavior、Tokens/size 和 Latency 的真实对比与推荐。
- [x] AC11 — Automatic Retrieval 与 `memory.recall` 并存；前者每轮低成本、后者为显式 Thin QUERY，且不破坏 ACTION 默认单次 Main Cognition 原则。
- [x] AC12 — Memory observation 在事件后及时产生而不要求每条消息增加一个大型 LLM 调用；相关异步、失败、幂等和重启路径有测试或明确验证证据。
- [x] AC13 — 生产代码只有一套 Prompt Context Assembly 权威路径；旧的重复拼装、全量 memory/history 注入或 rule/fact 混合路径已删除或迁移。
- [x] AC14 — `go test ./...` 与项目相关质量检查通过；所有不可运行项均明确列出原因和影响。
- [x] AC15 — 最终报告逐项明确回答原始需求的十个问题，并包含要求的十三类交付信息，不隐藏实验失败。

## Out of Scope

- 重写 canonical `CapabilityDefinition`、Registry、codec、统一 dispatch 或 Capability Runtime 核心。获批的 pure-QUERY-only continuation 仅属于 conversation orchestration 例外，不属于本条排除范围。
- 引入复杂 Knowledge Graph、Agent OS、新 Workflow Engine、复杂 Memory DSL、几十种 Memory Type 或大型向量数据库/索引基础设施。
- 将 summary 提升为权威事实存储，或为了演示效果删除、覆盖 Raw History。
- 仅为实验结果更好看而修改某一组 prompt 内容、规则或 tool schema。
