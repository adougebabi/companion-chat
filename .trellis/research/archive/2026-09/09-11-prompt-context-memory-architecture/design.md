# Prompt Context Assembly 与 Memory System 目标设计

状态：planning  
基线分支：`codex/llm-capability-runtime` (`6cbc18b`)  
工作分支：`codex/prompt-context-memory`  
Role 决策：B（真实本地模型两轮、36 次调用）

## 1. 设计结论

本次不重写现有 durable Memory authority，也不把 `ContextProjection` 扩成新的巨型 PromptManager。目标是补齐两层明确的中间结构：

1. `WorkingMemoryResolver`：从受权的 Recent、Active、Long-term Retrieval 与 Summary 候选中，按语义优先级和分区预算选择当前 cognition 可见内容。
2. `PromptContextAssembler`：接收已经准备好的 fragments/messages、真实 capability catalog 和 response schema，形成单 system、独立 Runtime Context、真实 recent roles、独立 current input，并对完整 Provider input 执行统一预算与 trace。

```text
Authoritative stores (can grow)
  ├── Conversation Messages
  ├── Cognition Facts / Frozen Actions / Outcomes
  ├── Life / Scene / State / Moment facts
  ├── Active Memory authority
  └── Long-term Memory authority
              │
              ▼
      RawHistoryReader / source refs
              │
       ┌──────┼──────────────┐
       ▼      ▼              ▼
    Recent  Active      Long-term Retrieval
       │      │              │
       └──────┼──── Summary ─┘
              ▼
      WorkingMemoryResolver
      (bounded selection)
              ▼
      PromptContextAssembler
      (roles + total budget)
              ▼
      Provider request / trace
              ▼
         Main cognition
```

核心不变量：

- Storage growth 与 Main input growth 解耦。
- Conversation、Raw History、Active Memory、Long-term Memory、Summary、Working Memory 是不同职责；可以共享 PostgreSQL，但不能共享语义。
- Summary/Working Memory 是 projection/cache，永不覆盖 Raw History。
- LLM 负责自然语言语义；Core 负责 schema、scope、authorization、time validation、CAS、idempotency、ranking arithmetic、budget、transaction 与 workflow。
- Durable Memory 继续只有 `episodic | semantic | relationship | autobiographical` 四类；Working Memory 不是 durable type。
- Capability 继续使用 canonical `CapabilityDefinition -> Registry -> ContextResolver -> Runtime`。本任务不恢复 Manifest/Executor/名称 switch。

## 2. 证据基线与 Role 决策

完整盘点见：

- `research/current-architecture-inventory.md`
- `research/prompt-role-baseline.md`
- `research/query-continuation-conflict.md`
- `research/role-experiment-results.md`

当前生产 conversation 是一条 system + 一条巨型 user document；current input 在 `current_message.content` 和顶层 `text` 中重复，Recent role 只是文档表格字段。四个 Main surface 又分别手写 request envelope。Memory 有局部 12-result/2400-rune 上限，但整个 input 没有总预算。

真实 mlx-serve 返回：

```text
loaded model: huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX
active context_length: 65,536
model metadata max: 262,144
```

预算必须以当前服务真正开放的 65,536 为准，不能使用 262,144 的模型理论上限。

两轮受控实验中：

- A 两轮均生成非法工具名 `media.image_generate`，并两轮都捏造 23:30 截止时间；工具名准确率 4/6。
- B/C 工具名均为 6/6；短期 Fact/Persona/Instruction 边界均通过。
- B 将动态事实保留在独立 Runtime user message，C 将其提升到 system；C 的短测通过不抵消长期 rule/fact 污染风险。
- B 平均 input 约 1056 tokens、output 133 tokens、latency 4.17s；C 相近；A output 更长且约 8.36s。延迟受缓存/顺序影响，仅作方向性证据。

最终采用 B：

```text
SYSTEM
  # 运行协议            stable rules
  # 人格设定            stable Core Persona

USER
  [RUNTIME CONTEXT]     dynamic facts, never commands

USER / ASSISTANT
  recent real messages  chronological roles

USER
  current input         exactly once, always last

TOOLS
  canonical thin CapabilityDefinitions
```

mlx-serve 的“最多一条且必须 leading system”约束继续保留。不同稳定 system fragments 先合并，动态 relationship/state/memory 不再提升进 system。

## 3. 组件边界

### 3.1 RawHistoryReader

`RawHistory` 在第一版是逻辑 authority，不新增一张复制所有领域事件的巨大 event-sourcing 表。

理由：当前系统已经存在多个真正拥有事实的 authority：

- `conversation_messages` 拥有用户/助手原文；
- `cognition_inbox` 拥有 persona-sequenced observation；
- `cognition_frozen_actions` / `cognition_action_outcomes` 拥有 Capability invocation/result；
- Life/State/Moment/Reflection 各自拥有事实或 revision/governance。

再复制一份完整 payload 会制造双重 SOT 和跨模块事务扩散。第一版增加一个只读组合接口，将权威行规范化为统一 source envelope：

```go
type RawHistoryEvent struct {
    SourceRef     string
    Kind          RawHistoryKind
    FluctlightID  string
    ConversationID string
    ActorID       string
    OccurredAt    time.Time
    Sequence      int64
    Content       map[string]any
    Authority     string
}

type RawHistoryReader interface {
    Recent(ctx, RawHistoryQuery) ([]RawHistoryEvent, error)
    Search(ctx, RawHistorySearchQuery) ([]RawHistoryEvent, error)
    ReadSources(ctx, []string) ([]RawHistoryEvent, error)
}
```

Adapters只能读取 owning tables，不反向写领域。`SourceRef` 使用明确前缀，例如 `message:<id>`、`fact:<id>`、`outcome:<id>`、`memory:<id>@<revision>`。Reflection 的裸 `sequence:N` 仍只在其 frozen persona/window scope 内解析；新 projection 不把它当全局 ID。

为了让 conversation 与 cognition 可稳定关联，migration 为新消息增加 nullable `turn_id`、`source_fact_id`、`correlation_id`，并为新写入填充。新 turn 将 user message、claimed cognition fact、workflow intent 与 outbox 在一个短 PostgreSQL transaction 内提交；Provider 调用仍在 transaction 外。这样既保留“Provider 失败时用户原文仍 durable”，又消除消息成功而 fact enqueue 失败的裂缝。

新增数据库唯一性：

- `conversation_messages(conversation_id, sequence)`；
- `cognition_inbox(fluctlight_id, sequence)`。

Migration 在建索引前检测历史重复；发现重复时 fail closed，不猜测修复。

已有 mutable lifecycle/status 字段不伪装成 append-only payload。Raw History 的“发生事实”来自原始 message/fact/revision/outcome；当前 projection/status 可以更新，但不能删除原 observation。Diagnostics 与 outbox delivery state不是 Raw History SOT。

### 3.2 Active Memory authority

Active Memory 使用独立 authority，不复用 `memories.status='active'`。后者表示 durable Memory 当前有效，语义完全不同。

最小当前表：

```text
active_memories
  id
  owner_fluctlight_id
  conversation_id?
  kind: future_event | commitment | temporary_context
  content
  status: active | completed | expired | superseded
  confidence / importance
  actor_refs[]
  source_fact_id
  evidence_refs[]
  original_time_expression?
  valid_from? / valid_until?
  time_precision: exact | part_of_day | date | range | unknown
  timezone
  last_relevant_at
  revision
  canonical_key
  superseded_by_active_memory_id?
  created_at / updated_at / closed_at?
```

审计表 `active_memory_revisions` 保存完整 snapshot、operation、expected/resulting revision、evidence、request digest、idempotency、policy version 和时间。生产写入只能经过：

```go
applyActiveMemoryCommandTx(ctx, tx, PreparedActiveMemoryMutation)
```

操作：

```text
create | confirm | revise | complete | expire | supersede
```

不引入复杂 rollback UI；若后续需要 owner governance，再沿现有 durable Memory compensation 模式扩展。当前 migration 与 API 不把 Active Memory 暴露成 Long-term Memory。

读取时硬过滤：

```sql
status = 'active'
AND (valid_from IS NULL OR valid_from <= now)
AND (valid_until IS NULL OR valid_until > now)
```

因此到期事实会立即离开 Working Memory，即使异步 cleanup 尚未将 row 标记为 `expired`。Wake-up/daily review/reflection 复用既有 durable workflows，批量把 due rows 通过同一写 authority 关闭并追加 revision。

### 3.3 Active Memory 的 LLM 输入与实时产生

新增 direct Thin ACTION capability：

```text
active_memory_event
  operation: create | confirm | revise | complete | supersede
  target_ref?               opaque, non-create operations
  kind?
  content?
  confidence
  importance?
  original_time_expression?
  valid_from?
  valid_until?
  time_precision?
```

Core 从 frozen context 绑定 owner、conversation、timezone、source fact、evidence、idempotency 和 expected revision；Provider 不提供数据库 ID、scope、revision 或 canonical key。

时间语义由 LLM 基于 Runtime Context 的真实 `current_time + timezone + source occurred_at` 提议绝对 ISO 时间；Core只做格式、offset、顺序、范围与合理 horizon 校验，不用 regex/关键词自行解释“明天/晚上”。原表达总是保留。无法可靠解析时，模型输出 `time_precision=unknown` 且时间为空，Core不猜。

该 capability 使用 `optional_internal`：它是非阻塞 observation，不应因候选失败让普通聊天失败。用户明确要求“记住”且助手声称长期记住时仍使用现有 `memory_event` 的 `required_for_visible_claim`，不能用 optional Active write伪造成功。

实时路径不增加第二个 Memory LLM：Main cognition 在同一次响应中提出 capability。异步兜底复用 Reflection，新增 closed `active_memory_candidates[]`，允许 create/confirm/revise/complete/expire/supersede。Reflection 与 Active/Long-term candidates 在同一 caller-owned transaction 应用；Provider failure不影响已提交 conversation。

### 3.4 Active Memory ranking

Active candidates先做 owner/conversation/status/time hard filters，再计算确定性 score。Core不解释自然语言，只组合已有信号：

```text
score =
  critical-window boost
  + temporal urgency
  + importance
  + lexical/vector relevance (if authorized)
  + last-relevant recency
  + confidence tie-break
```

临近/正在发生的有绝对时间事实优先；过期值在 ranking 前排除。`last_relevant_at` 只在一个 Active Memory 真正被选入 Working Memory 或被显式 confirm/revise 时更新，且更新有节流，避免每轮无意义写放大。

Ranking 输出每项 component、selected/dropped 和 budget reason 到 Core-only trace；Provider只看到 opaque ref、content、semantic time、confidence/importance 和状态语义。

### 3.5 Long-term Memory

现有 `memories + memory_revisions + memory_governance + memory_embeddings` 保持唯一 durable Memory authority，继续复用：

- 四类 durable type；
- create/confirm/revise/merge/supersede/deprecate/forget/rollback；
- canonical exact duplicate、CAS、idempotency、full snapshots；
- owner/visibility/actor/conversation hard filters；
- FTS、versioned embeddings、opaque refs、reflection operations。

本任务只做三个必要增强：

1. retrieval cue加入 bounded recent topic、Active Memory 与 Current State，而不是只依赖 current input；
2. lexical/FTS candidate选择按授权后的相关性查询，不再先按 `created_at DESC LIMIT 200` 让更老的相关 Memory永久失去候选资格；
3. trace记录每个 selected/dropped item及 score components。

ABA 通过现有显式 lineage保证：A1 superseded → B superseded → A2 active。Working Memory只读 authoritative active row。Core不从相似度猜 supersede；Main/Reflection 必须对检索到的旧 ref 提出 revise/merge/supersede。测试使用受控 Provider proposals验证完整链与最终唯一注入。

### 3.6 Conversation Summary

Summary 使用有 source boundary 的 chunk projection，不递归 summary：

```text
conversation_summaries
  id
  owner_fluctlight_id
  conversation_id
  from_sequence / to_sequence
  source_message_refs[]
  source_digest
  summary
  status: active | superseded
  revision
  supersedes_summary_id?
  provider/model/prompt/schema provenance
  created_at
```

每个 summary 必须直接读取 `[from_sequence,to_sequence]` 的 Raw `conversation_messages`。输入永远不包含旧 summary。重建会从同一 Raw range生成新 revision/superseding row；Raw message不变。

生成策略：

- assistant settlement 后只用确定性计数/估算判断“是否存在一个可总结的旧 chunk”；
- 保留最新 Recent reserve，不总结当前未稳定 turn；
- 每个 chunk以 source token/bytes 和最大 message count 双重限制；
- 写稳定 `conversation.summary` workflow intent；
- 使用现有 Temporal runtime 与 `reflection` model role执行严格 summary schema；
- 默认约每 20 个已完成 turn或旧区间达到约 6000 estimated tokens才生成，不是每条消息一个 LLM；
- 同一 source range/digest 幂等；Provider/result commit crash按稳定 request ID恢复。

Summary 不包含工具内部参数、隐藏 reasoning、数据库元数据或未经证据支持的新事实。选入 prompt时带 opaque summary ref、source time/range 和明确 `summary_projection` 标签。

## 4. Working Memory

`WorkingMemoryResolver` 接收已经授权、已经有来源的候选，不执行 SQL/embedding/LLM：

```go
type WorkingMemoryInput struct {
    CurrentFacts      []PromptFragment
    ActiveCandidates  []RankedActiveMemory
    RecentMessages    []RecentMessage
    RetrievedMemories []RankedMemory
    Summaries         []ConversationSummary
}

type WorkingMemory struct {
    RuntimeFacts      []PromptFragment
    Active            []PromptMemoryItem
    Recent            []ProviderMessage
    Retrieved         []PromptMemoryItem
    Summaries         []PromptSummary
    Trace             WorkingMemoryTrace
}
```

选择规则：

- Recent 从最新完整消息/turn向后选择，再恢复 chronological order；尽量不拆 user/assistant pair。
- 一条旧 message 超过 Recent 分区时整条淘汰，由 Summary/Raw recall补位；不做字符串尾截断。
- Active、Retrieved、Summary 都按完整 item/chunk选择；未选中不删除。
- 同一 source/ref在 Active、Retrieved、Summary、Recent重叠时只保留最高 authority/priority 的一个投影。
- Current user input不属于 Working Memory候选；它由 Prompt assembler独立保护。

## 5. Prompt Fragment 与 Assembler

最小 fragment：

```go
type PromptFragment struct {
    Kind            PromptFragmentKind
    Priority        int
    Required        bool
    Content         any
    EstimatedTokens int
    SourceRefs      []string
}
```

Assembler职责仅为：

1. 接收 stable operation rules/Core Persona、Working Memory、Current Input、canonical Tools 与 response schema；
2. 估算每个完整 fragment/message和固定 envelope成本；
3. 按 policy选择，超限则丢弃低优先级完整 fragment；
4. 形成实验 B 的 messages；
5. 输出可审计 trace。

它不执行：

- Memory/Raw History SQL；
- embedding或 ranking语义；
- Active Memory extraction；
- Persona/relationship/state mutation；
- Capability business validation；
- Scene/Schedule/Tool-specific assembly。

现有 `composeProviderMessages` 的稳定 system合并、Core Persona过滤与安全 TOON/YAML renderer 会下沉/复用为 assembler renderer；不保留第二套生产 formatter。

## 6. Prompt Budget

### 6.1 Model/role配置

`model_roles` 增加：

```text
context_window_tokens
max_input_tokens
prompt_budget_policy_version
```

现有 `token_budget` 继续表示 output `max_tokens`，不再被误称为 input budget。角色配置验证：

```text
max_input_tokens + token_budget + safety_margin <= context_window_tokens
```

当前模型默认：

```text
context_window_tokens = 65,536
max_input_tokens      = 49,152
output token_budget   = 4,096
safety margin         = 4,096
unused headroom       = 8,192
```

模型更换时必须重新配置/预检；不得根据 model name猜 context window。`/models` 有 metadata时可以显示/建议，但持久化 role assignment才是运行权威。

### 6.2 初始分区策略

以下为 soft cap；总 input `49,152` 是 hard cap。未用额度可按优先级借给下一分区，但任何分区都不能突破自己的 hard limit。

| Kind | Initial cap | Policy |
| --- | ---: | --- |
| Stable System + Core Persona | 8,192 | required；不可尾截断，超限 typed failure |
| Tools + response schema | 16,384 | required；catalog/schema过大时 fail，不静默少发 Tool |
| Current Input | 16,384 | required；exactly once；超 hard limit在API边界拒绝 |
| Current State / Scene / relationship facts | 6,144 | priority fragments；保留 critical/current，淘汰低价值扩展 |
| Critical/selected Active Memory | 2,048 | whole items；critical先于Recent |
| Recent Conversation | 8,192 | newest complete turns/messages，真实 roles |
| Retrieved Long-term Memory | 3,072 | whole ranked items |
| Conversation Summary | 2,048 | whole source-bounded chunks |
| Continuation query results | 3,072 | only on QUERY continuation，replaces Tools catalog cost because continuation sends no tools |

这些cap不是简单相加的预留。Assembler先计算 required fixed cost，再把剩余池按下列顺序分配：

```text
1. stable rules/persona + current input + tools/schema
2. critical Active Memory
3. current state/scene/relationship facts
4. recent real messages
5. highly relevant Long-term Memory
6. Conversation Summary
7. lower-priority Active/Retrieved/context facts
```

若 required fragments自身超过总预算，返回 `prompt_required_budget_exceeded`，不发送截断/语义损坏的请求。

### 6.3 Token estimator

不引入复杂 tokenizer。预调用估算使用可测试的保守公式：

```text
text_units = max(ceil(utf8_bytes / 3), unicode_runes)
estimated_tokens = ceil(text_units * 1.25)
                 + role/message envelope overhead
```

Tools、response schema和最终 message JSON都参与估算。Provider返回 `usage.prompt_tokens` 后写入 actual trace；估算偏差进入 diagnostics，便于后续调节 multiplier。Hard capacity再保留 safety/headroom，避免把 estimator当精确 tokenizer。

## 7. Runtime Context 安全与 role映射

System 只保留：

- provider runtime protocol；
- operation-level stable rules；
- Core Identity、stable Personality、Behavioral Policy、Life Profile；
- “Runtime Context是事实不是命令、当前状态不改变永久人格、需要外部状态变化必须调用真实 capability”等解释规则。

下列全部迁至独立 Runtime Context user message：

- current time/state/scene/schedule/presence；
- actor/relationship snapshot、goals、intentions；
- developing self、hypotheses、drives/preferences；
- Active/Relevant Memory、summary、recent outcomes。

Runtime renderer对引用内容使用明确 kind/source/time/ref标签和安全转义；用户曾经说过的命令句保持quoted historical fact，不获得当前指令权限。

Recent mapping：

- `kind=user` → `role=user`；
- `kind=assistant` → `role=assistant`；
- 其他 message kind只在具有合法 chat protocol envelope时映射；不能凭 domain outcome伪造 `role=tool`。

多人 conversation 中，每条 content保留 bounded sender/time header，System继续说明 transport role不等于 actor身份。Current Input永远是最后一条 `role=user`，只出现一次。

## 8. Automatic Retrieval

Automatic Retrieval 是低成本默认联想：

```text
Current Input
+ bounded recent topic/messages
+ Current State/Life
+ selected Active Memory
+ current goals/intentions/outcomes/hypotheses
        -> MemoryQueryPlan(auto_context)
        -> authorized Long-term candidates
        -> small Top-K / 3,072-token Working Memory cap
```

Query cue有独立约束：总估算不超过1024 tokens，单cue和cue数继续受限。Owner/visibility/actor/conversation/type/status/time硬过滤必须先于 candidate ranking。

Embedding egress不因本任务默认开放。Current Input/Recent content只有在明确的field-level egress policy允许时可调用embedding role；否则trace必须写 `lexical_salience`。有授权时复用versioned embedding与exact vector search；HNSW仍等待benchmark。

## 9. `memory.recall`

### 9.1 Capability定义

```text
Name              memory.recall
Version           v1
Type              query
Surfaces          conversation
Input             { intent: string[1..2000] }
RequiredContext   memory_scope
SideEffectClass   read_only
Concurrency       parallel
SuccessBoundary   query_result_available
FailurePolicy     optional_internal
```

Capability持有窄 `MemoryRecallService`，不持有 `*App`，不修改 generic Runtime。Frozen `MemoryScope`只提供 owner/viewer/conversation/profile authorization；执行时用 `intent`新建更深、仍有硬上限的 query plan，不能只返回自动检索时已冻结的 Top-K。

搜索源第一版包括：

1. Active Memory current projection；
2. authorized active Long-term Memory；
3. authorized older Raw conversation messages与Conversation Summary。

Raw history检索增加 conversation message FTS/search seam；assistant prose标记为 `conversation_record`，不提升为 confirmed fact。所有结果先授权后排名，返回上限12项/约3072 tokens。

Provider-safe item：

```text
ref                 opaque
source_kind         active_memory | long_term_memory | conversation_record | summary_projection
content
speaker? / occurred_at?
confidence? / importance?
```

Raw DB ID、visibility、conversation FK、revision、evidence IDs、query plan ID与内部score不进入Provider tool result；Core trace保留完整映射。

### 9.2 Pure-QUERY-only continuation

用户已批准新增一次受限例外。第一 Main响应必须显式输出：

```text
response_mode: final | query_continuation
```

`final` 保持现状：同一 Main cognition必须给visible text。`query_continuation` 必须满足：

- visible text为空；
- 至少一个且最多两个 invocation；
- 所有 invocation经 metadata分类均为 `pure_query`；
- 不含 ACTION/deferred/transactional mutation；
- first response仍通过closed appraisal/decision validation。

流程：

```text
First Main cognition
  -> validate/freeze query continuation decision
  -> Prepare and persist query invocations
  -> execute pure queries outside business transaction
  -> persist bounded results + continuation state
  -> one continuation Provider call, tools omitted
  -> continuation schema returns visible_text only
  -> assistant/state settlement transaction
```

Continuation messages采用标准协议：原始已冻结的B-layout messages，追加一条 canonical assistant tool-call envelope和对应 `role=tool` results。Sidecar-originated call也从 canonical Invocation确定性序列化，不重新解释语义。

Continuation不发送 tools，不能产生第三轮调用；response schema只允许 `visible_text`，不能返回 appraisal、claims、state/capability changes。

Frozen action保存：

```text
continuation.version
continuation.status: requested | queries_completed | provider_completed
continuation.query_call_ids[]
continuation.result_digest
continuation.provider_request_id
continuation.visible_text?
```

Retry优先重放：已完成query不重跑；已有visible output不重调Provider。Query失败时整个依赖型turn显式失败；不能让第二次模型在无结果时编造答案。

原 blanket guards改成：

- ordinary final/ACTION/mixed path仍恰好一次 Main cognition；
- 只有 dedicated generic pure-query coordinator可出现一次 `role=tool`；
- coordinator无capability-name switch、无tools catalog、无第二批invocation。

## 10. Daily Consolidation 与 Reflection

现有 Reflection继续是 Long-term Memory consolidation authority，不增加每句话一个模型调用。

Reflection input增加：

- bounded Active Memory refs；
- source-bounded Raw History evidence；
- selected current Long-term refs；
- Conversation Summary refs仅作为导航，不作为原始证据替代。

Reflection可在一个proposal中：

- create/confirm/revise/merge/supersede/deprecate Long-term Memory；
- create/confirm/revise/complete/expire/supersede Active Memory；
- 用同一Raw evidence产生一个Long-term candidate并关闭对应Active Memory，实现选择性promotion。

所有targets使用frozen opaque refs，所有候选先全量closed-schema验证，再与proposal/watermark/CAS同事务应用。普通航班到期只expire Active；严重延误等重要经历可由Reflection创建episodic Long-term。不是所有Active都promotion。

## 11. Observability

扩展现有 Provider diagnostics，不创建独立可视化系统。每次请求记录：

```text
context_window_tokens
max_input_tokens
output_reserve_tokens
estimated_total_input_tokens
actual_prompt_tokens?
actual_completion_tokens?
latency_ms

system_tokens
runtime_tokens
active_count / active_tokens
recent_count / recent_tokens
retrieved_count / retrieved_tokens
summary_count / summary_tokens
tool_tokens
response_schema_tokens
current_input_tokens

selected_refs[] {kind, ref, score_components, reason}
dropped_refs[]  {kind, ref, reason: section_cap|total_cap|deduplicated|expired|unauthorized}
continuation_phase?
```

正常日志只保留bounded counts/digests；详细ref/score trace留在persona-scoped gated diagnostics。API/browser仍看不到raw prompt、hidden reasoning、credentials或Core-only IDs。

## 12. Persistence 与 Migration

新增线性 additive migration（预计 `0032_prompt_context_memory`）：

- `conversation_messages`: nullable source linkage columns + sequence unique index + FTS document/index；
- `cognition_inbox`: sequence unique index；
- `active_memories` + `active_memory_revisions` 与约束/index；
- `conversation_summaries` 与source range/digest/index；
- `model_roles`: context window/input budget/policy version；
- diagnostics所需JSON trace/usage列（优先复用现有表字段，只有无法表达时新增）。

Migration不重写durable Memory历史，不把旧chat批量变成Memory，不删除旧facts。历史message没有可靠link时列保持NULL；RawHistoryReader仍可用稳定`message:<id>`读取。

Migration preflight检测：

- duplicate conversation/persona sequence；
- invalid existing role budgets；
- malformed Active/Summary rows（rerun路径）；
- index/constraint identity。

失败则schema与ledger同事务回滚。

## 13. 生产路径迁移

按一个cutover迁移四个Main request builder：conversation、native cognition、daily review、wake-up。Reflection/summary使用同一assembler但自己的surface/budget policy。Media prompt保持显式旁路，不混入普通composer。

删除/替换：

- conversation的`current_message + text + context`巨型user envelope；
- current input双重渲染；
- Recent作为单user文档内TOON table的生产路径；
- dynamic actor/relationship context提升进system；
-固定12条作为Recent唯一边界；
-全局只有2400-rune Memory局部budget却无总budget的假bounded状态；
-四个call-site手写Main message assembly；
-绝对禁止所有`role=tool`的blanket guard（替换为pure-query exception guard）。

保留：

-一个leading system；
-safe TOON/YAML runtime facts renderer；
-canonical Tools独立payload；
-strict response schema/native call precedence；
-full frozen ContextProjection用于replay；
-Memory lifecycle/retrieval hard authorization；
-Capability Runtime core。

不长期保留feature flag双路径。Role A/C只存在于opt-in experiment test builder，不进入生产assembly。

## 14. Failure、并发与恢复

- Required prompt fragments超预算：调用前typed failure，无Provider请求。
- Optional fragment超预算：整项drop，trace原因；source不删除。
- Active write失败：optional structured result，普通reply可提交；显式长期“记住”仍依赖required `memory_event`。
- Summary Provider失败：workflow retry；Raw History与Recent不受影响。
- Embedding失败：沿现有规则退回authorized lexical/FTS，不丢Memory。
- Active到期：read-time立即排除；background close幂等。
- Continuation crash：依照frozen phase从query result或visible output恢复，不重复query/Provider。
- Later turn supersedes claimed continuation turn：旧turn completion仍受现有inbox claim/status锁约束，不能提交迟到assistant。
- 外部Provider/embedding/Temporal/Redis I/O均不进入PostgreSQL business transaction。

## 15. Rollout / Rollback

Rollout顺序：

1. additive schema与read ports；
2. Active/Summary authority但尚不注入生产prompt；
3. Working Memory/Budget deterministic tests；
4. B-layout assembler全surface cutover；
5. Automatic Retrieval增强；
6. `memory.recall` pure query；
7. guarded continuation；
8. diagnostics与full integration；
9. 删除旧assembly路径。

每一步在同一分支内保持可测试。若B-layout/预算回归，回滚代码commit即可；additive表保持闲置，不删除用户历史。Continuation最后启用，便于单独回滚为“query结果只供later cognition”而不影响Automatic Retrieval或Memory authority。

### 15.1 Strict stage boundary

实现严格按 `implement.md` 的 S01→S12 顺序推进。每一阶段有固定 file allowlist；除当前任务目录内的 evidence/checklist 外，主会话不得修改当前阶段allowlist之外的文件。若真实依赖超出allowlist，停止当前阶段并先修订计划、取得范围确认，不能以“顺手修复”扩大变更。

S01至S11只允许运行当前阶段新增/修改符号的精确单元或定向集成测试，以及针对当前文件的格式/diff检查。全仓`./...`、race、Compose、Live Provider、完整PostgreSQL/Temporal验收全部只在S12触发。阶段外既有失败记录到implementation evidence，不在当前阶段修复。

## 16. Test Matrix

### Raw / Recent / Summary

- 1000+消息/事实可分页，Raw行不因summary或prompt淘汰删除。
- user message + claimed fact + workflow intent/outbox原子提交。
- sequence uniqueness与migration duplicate fail-closed。
- summary只读取Raw source range，不读取旧summary；重建保留sources/digest并supersede projection。
- Recent按token选择完整真实roles、current input去重。

### Active Memory

- 航班事实在离开Recent后仍selected。
- valid_until后立即不进prompt，cleanup追加expired revision。
- complete/supersede/CAS/idempotency/concurrency。
- ambiguous time不生成假精确anchor。
- observation无额外Main/Memory Provider call。

### Long-term Retrieval

- owner/visibility/actor/conversation过滤先于candidate limit。
-更老但相关Memory可越过原recent-200 blind spot。
- irrelevant Memory不进入prompt。
- ABA lineage只注入A2，A1/B仍可审计。
- embedding disabled/failed有诚实lexical trace。

### Prompt Budget / Roles

- 1000 Raw + 100 Long-term + 30 Active不突破49152 estimated tokens。
- System/Tools/Schema/Current protected；optional whole-fragment淘汰。
- B messages: one system、one Runtime user、real recent roles、final current user once。
- dynamic relationship/state/memory不在system。
- A/B/C opt-in live harness保留真实结果；生产full schema复验B。

### Recall / Continuation

- `memory.recall` Definition/Registry/ContextScope/provider-safe output。
- automatic Top-K与explicit deep recall不同。
- pure-query-only最多一次continuation，tools为空。
- ACTION/mixed/final reply路径仍一次Main。
- query失败、continuation Provider失败、crash/retry/superseded turn无重复。
- continuation不能产出或执行新capability/state/claim。

### Gates

- Core `go test ./... -count=1`、`-race`、`go vet`、`go build`。
- Gateway受影响时同样运行四项。
- migration empty→head、rerun、malformed rollback与真实PostgreSQL。
- Temporal summary/continuation相关workflow replay/versioning测试。
- `gofmt`、`git diff --check`、架构guard。
- live local Provider B-layout实验与最终full production request行为。

## 17. 明确不做

- 不引入Knowledge Graph、Memory DSL、几十种type、HNSW、LLM reranker或第二个vector store。
- 不建立第二个workflow engine或Redis delayed queue。
- 不把assistant prose当权威事实或自动Long-term Memory。
- 不让Core用关键词理解“航班/面试/偏好”的语义；关键词/FTS仅用于检索候选。
- 不让Prompt assembler执行retrieval、extraction、persona mutation或tool business logic。
- 不把Role A/C生产路径作为长期兼容开关。
