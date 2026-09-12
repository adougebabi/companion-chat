# Prompt Context Assembly 与 Memory System 最终实施报告

日期：2026-09-12  
分支：`codex/prompt-context-memory`  
基线：`codex/llm-capability-runtime` @ `6cbc18b`  
Migration head：`0032_prompt_context_memory`

## 1. 旧架构分析

重构前的根因是“持久存储增长”和“每轮 Main cognition 输入增长”没有明确解耦：

- Conversation Main 由一条 system 加一条巨型 user document 组成。
- Current input 同时出现在 `current_message.content` 和顶层 `text`，语义重复。
- Recent role 只是 user document 中的字段，不是 Provider transport role。
- Conversation、native cognition、daily review、wake-up 和 Reflection 分别手写 Main request。
- Recent 只有固定条数边界，Memory 只有局部 rune budget；System、Tools、Schema、Current Input 没有统一 input hard cap。
- 已有 durable Memory 具有完整 lifecycle/授权/检索基础，但缺少独立 Active Memory、source-bounded Summary、Working Memory 和显式 recall。
- QUERY taxonomy 已存在，但生产和契约绝对禁止同轮 `role=tool` continuation。

因此旧系统看似“有记忆”，但没有一个能证明最终 Provider wire 始终受控的总预算 authority。

## 2. Prompt / Role Baseline

基线组织为：

```text
system
  operation/runtime rules + dynamic persona/relationship context

user
  {
    current_message,
    text,                 // duplicates current_message.content
    context: {
      recent_messages[]  // role is only a data field
      memories[]
      state/relationship/...
    }
  }
```

基线存在三个直接风险：动态 facts 被提升为 system authority，current input 重复，历史对话的真实 role/顺序丢失。

## 3. A/B/C 真实模型实验与推荐

实验使用当前本地 mlx-serve 真实模型：

```text
model: huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX
active context_length: 65,536
model metadata maximum: 262,144
```

两轮、36 次调用只改变 role/message organization，规则、事实、persona、tools 与 user input 保持一致。

| 方案 | 组织 | 真实结果 |
| --- | --- | --- |
| A | 旧巨型 user document | 两轮都生成非法工具名，工具准确率 4/6，两轮捏造 23:30 deadline |
| B | stable system + Runtime user + real recent roles + final current user | 工具 6/6，Fact/Persona/Instruction 边界通过 |
| C | dynamic Runtime Context 进 system | 短测 6/6，但长期 rule/fact authority 污染风险更高 |

最终选择 B。A 平均延迟约 8.36s，B 约 4.17s；延迟受调用顺序和 cache 影响，仅作方向性证据。

## 4. 最终 Memory Architecture

```text
Owning authoritative stores (unbounded growth allowed)
  ├─ Conversation Messages
  ├─ Cognition Facts
  ├─ Frozen Actions / Outcomes
  ├─ Life / Scene / State / Moment facts
  ├─ Active Memory authority
  └─ Long-term Memory authority
              │
              ▼
       RawHistoryReader / source refs
              │
      ┌───────├──────────────┐
      ▼       ▼              ▼
   Recent   Active       Automatic Long-term Retrieval
      │       │              │
      └───────┼── Summary ─────┘
              ▼
      WorkingMemoryResolver
              ▼
      PromptContextAssembler
              ▼
        bounded Provider wire
```

关键语义：

- Raw History：发生过什么，是对 owning authority 的逻辑读视图，不复制为第二张总事件表。
- Recent Context：当前可见的近期完整 message/turn，退出 prompt 不代表删除。
- Active Memory：当前仍对近期行为有影响的时限事实，不是 durable `memories.status=active`。
- Long-term Memory：仍由原有四类 durable authority 统一写入。
- Conversation Summary：直接绑定 Raw message range/refs/digest 的 projection，不递归覆盖。
- Working Memory：单次 cognition 实际看到的 bounded read model，不是持久 Memory type。

## 5. Raw History 与 Turn Atomicity

`RawHistoryReader` 组合：

- `conversation_messages` → `message:<id>`
- `cognition_inbox` → `fact:<id>`
- `cognition_action_outcomes` → `outcome:<id>`

Reader 仅读，保留 `Authority`、occurred time、sequence 和 source ref。Recent 默认 50、上限 200；source re-read 上限 64；conversation search 第一版只检索 message FTS。

新 turn 在一个短事务内提交：

```text
user message
+ claimed conversation.turn cognition fact
+ stable workflow intent
+ outbox
```

Provider 仍在事务外。Assistant message 与同一 turn/source/correlation 绑定。Migration 为 conversation/persona sequence 增加唯一性，历史重复值 fail closed，不猜测修复。

## 6. Active Memory Lifecycle

独立表：

```text
active_memories
active_memory_revisions
active_memory_commands
```

唯一生产写 authority：

```go
applyActiveMemoryCommandTx(ctx, tx, PreparedActiveMemoryMutation)
```

操作：`create | confirm | revise | complete | expire | supersede`  
类型：`future_event | commitment | temporary_context`  
状态：`active | completed | expired | superseded`

读时必须满足：

```sql
status = 'active'
AND (valid_from IS NULL OR valid_from <= now)
AND (valid_until IS NULL OR valid_until > now)
```

因此过期项在 cleanup 写 revision 之前已立即退出 Working Memory。

S12 真实模型反馈促使 schema 进一步收紧：每个 create 要求 `original_time_expression` 和 `time_precision`；`future_event` 还必须有非空原表达、非 unknown precision 和 `valid_until`。`valid_from` 是事实开始影响行为的时间，不能机械当作事件发生时间。

## 7. Long-term Retrieval 与 `memory.recall`

Durable Memory 仍只有：

```text
episodic | semantic | relationship | autobiographical
```

`memory_event`、Reflection 和 Owner governance 继续共用 `memory.lifecycle.v2` 唯一 authority。检索先应用 owner/visibility/viewer/conversation/type/status/time 硬过滤，再进行 FTS/词法/vector/ranking。更旧但相关的 Memory 不再被 `created_at DESC LIMIT 200` 永久挡在候选外。

Automatic Retrieval query cue 组合：

- current input；
- 最近六条消息/近期 topic；
- current life/state/scene；
- selected Active Memory；
- goals/intentions/outcomes/hypotheses。

Cue 最多 32 条、单条 1000 runes、总查询不超 1024 estimated tokens。Projection cues 默认 `AllowEmbedding=false`，不伪称 semantic/vector retrieval。

`memory.recall/v1`：

```text
surface: conversation only
input: {intent: string[1..1000]}
type: query
side effect: read_only
concurrency: parallel
required context: memory_scope
sources: Active + Long-term + authorized older messages + Summary
output: <= 12 whole items, <= 3072 estimated tokens
```

返回值只包含 opaque ref、source kind 和 bounded semantics，不暴露 DB ID、revision、visibility、evidence ID、query plan 或内部 score。

## 8. Working Memory 与 Prompt Budget

Token estimator：

```text
text_units = max(ceil(utf8_bytes / 3), unicode_runes)
estimated_tokens = ceil(text_units * 1.25)
```

Message envelope、Tools、response schema 和最终 JSON wire 都计入。

默认 role budget：

| 字段 | 值 |
| --- | ---: |
| context window | 65,536 |
| max input | 49,152 |
| output reserve / `max_tokens` | 4,096 |
| safety margin | 4,096 |
| unused headroom | 8,192 |
| policy | `prompt-budget.v1` |

Working Memory section caps：

| Section | Cap |
| --- | ---: |
| Runtime facts | 6,144 |
| Active Memory | 2,048 |
| Recent | 8,192 |
| Retrieved Long-term | 3,072 |
| Summary | 2,048 |

Required caps：System 8,192，Current Input 16,384，Tools + response schema 16,384。Required wire 超限在 Provider 调用前返回 `prompt_required_budget_exceeded`；optional 内容以完整 fragment/turn 为单位丢弃，不做字符串尾截断。

## 9. B-layout Prompt Context Assembly

五个生产 Main caller 全部使用：

```text
assembleProjectionPrompt
  -> StructuredAssembledWithToolsSchema
```

包括 Conversation、Native Cognition、Daily Review、Wake-up 和 Reflection。

最终 messages：

```text
SYSTEM
  stable runtime rules + filtered Core Persona

USER
  [RUNTIME CONTEXT]
  dynamic facts/state/Active/Retrieved/Summary
  [/RUNTIME CONTEXT]

USER / ASSISTANT
  selected recent real transport roles

USER
  current input exactly once and last
```

Assembler 只负责选择、格式、role 与 budget。它不执行 Memory SQL/embedding/ranking、不解析时间、不修改 Persona/场景/关系，不执行 Capability business logic。

## 10. Pure-QUERY-only Continuation

默认 direct conversation 是一次 Main cognition 并同轮返回可见文本。唯一例外：

```text
first Main cognition
  -> no visible text
  -> 1–2 registry-classified pure QUERY invocations
  -> freeze/prepare/persist
  -> execute read-only queries outside business transaction
  -> persist bounded results
  -> append canonical assistant tool_calls + role=tool results
  -> at most one no-tools Provider continuation
  -> closed {visible_text} only
  -> assistant settlement
```

Frozen phase：

```text
requested -> queries_completed -> provider_completed
```

Retry 复用已完成 query result 与稳定 Provider request identity。Provider 前后均检查 superseded fact，旧 turn 不能提交迟到 assistant。Continuation 不发 Tools，不能输出 appraisal/claims/state/capability mutations。ACTION 和 QUERY+ACTION mixed batch 仍必须在第一次 Main 返回可见文本。

真实 mlx native tool call 不总是同时返回 structured sidecar。因此增加一个通用 transport normalization：只有 Provider adapter 明确标记 `StructuredFallback`、无可见文本、且 1–2 个调用都是 pure QUERY 时，缺失 mode 才归一为 `query_continuation`。普通 schema omission、ACTION、mixed 和 visible fallback 不会触发。

## 11. Conversation Summary 与 Workflow

Summary 保留最新 24 条 message。旧区间在下列任一条件达成时生成 intent：

- 20 个已完成 assistant turn；
- 约 6000 estimated tokens；
- 40 条 message。

Chunk 必须在 assistant boundary 结束。`conversation.summary` 使用稳定 intent/workflow identity，只在 Temporal `lifecycle` queue 注册。Provider 输入只读原始 message range，settlement 重读连续 sequence、有序 refs 和 digest。S12 修复了把有序 source refs 误用通用 set helper 排序的问题。

## 12. Diagnostics、Migration 与删除/替换清单

`diagnostic_model_runs` 新增：

```text
fluctlight_id
metrics
estimated_input_tokens
actual_prompt_tokens
actual_completion_tokens
latency_ms
```

Metrics 记录 system/runtime/recent/current/tools/schema token/count、Provider usage、estimator delta、latency、continuation phase 和 bounded selected/dropped reasons。Array 上限 64，credential/raw prompt/raw response/reasoning/perception/appraisal/image data 继续脱敏或移除。Diagnostics 写失败不影响业务。

Migration `0032_prompt_context_memory` 添加：

- message turn/source/correlation linkage + FTS；
- conversation/persona sequence unique preflight/index；
- Active Memory authority；
- Conversation Summary projection；
- model role context/input budget；
- prompt diagnostic metrics。

被删除或退出生产 Main 路径的逻辑：

- `{current_message,text,context}` 巨型 user envelope；
- Current Input 双重渲染；
- Recent 作为一个 user document 内 table；
- dynamic relationship/state/memory 提升进 system；
- 固定 12 条作为 Recent 唯一边界；
- 四/五个 Main caller 手写 assembly；
- 全生产树绝对禁止 `role=tool` 的 blanket guard，替换为仅允许 dedicated generic pure-query coordinator。

保留的旧 helper 只服务非 Main compatibility/历史测试；静态 guard 证明五个 Main caller 不调用它们。

## 13. Verification、风险与未解决项

通过：

- Core full test / race / vet / build；
- Gateway full test / race / vet / build；
- 隔离 PostgreSQL empty→head、previous→head、rerun、malformed rollback；
- 数据库启用的完整 Core 包与完整 migrations 包；
- Active/Raw/Summary/provider-role/retrieval 真实 PostgreSQL 用例；
- Summary Temporal workflow 注册/校验/重放契约；
- 真实本地 Provider B-layout、Active flight、Fact/Persona/Instruction boundary、Recall continuation；
- 隔离 Compose PostgreSQL/Redis/MinIO/Temporal/Core/Worker/BFF/Web smoke；
- Core OpenAPI、Compose bind sources、gofmt、git diff、architecture guards。

用户于 2026-09-12 明确豁免 `check-go-projections.sh`：该脚本需要已认证 Cookie Jar 以及预先存在的 completed Moment/Proactive 业务数据；本任务未修改 BFF/API projection shape，且不得复用无关现有栈的用户数据。

保留风险：

- Live Provider latency 有明显波动；一次 Role case 在 180s 无响应，相同 fixture 的独立与最终组合运行均通过。
- Token estimator 是保守近似，需继续根据 actual usage delta 校准；8192 headroom 继续保留。
- Conversation Raw Search 第一版只检索 message FTS，不将 cognition fact/outcome 全文搜索伪装成已实现。
- Browser/BFF 不暴露 model role 新 budget 字段；Core 保留默认/旧值，本任务没有扩大 API scope。

## 原始需求十个问题的直接回答

1. **Raw History 能否持续增长而 prompt 不线性增长？**  
   能。Raw 保留在 owning tables，Working Memory 只选 bounded whole items/turns，最终 wire 受 49,152 hard cap。

2. **“明早7点赶飞机”离开 Recent 后会不会丢？**  
   不会。同一 Main cognition 可提出 `active_memory_event/create`，Active retrieval 在预算内选入 Working Memory。真实模型 fixture 已通过。

3. **航班完成/过期后会怎样？**  
   `valid_until` 读时立即排除，cleanup 再幂等写 `expire` revision。Raw source 保留，只有 Reflection 判断值得时才创建 durable Memory。

4. **Long-term Memory 是否仍然全量进 prompt？**  
   不是。必须经过授权优先的 Automatic Retrieval 或显式 `memory.recall`，然后再受分区/总预算筛选。

5. **如何保证固定 Prompt Budget？**  
   持久 model role capacity，对 System/Current/Tools/Schema/optional sections 估算，最后对 Provider wire 再估算；required overflow 在网络前失败。

6. **Conversation Store 和 Memory Store 是否仍混在一起？**  
   否。Conversation message 是原文 authority，Active 是时限 authority，durable Memory 是长期语义 authority，Summary/Working Memory 是 projection/read model。

7. **动态事实还会污染 System/Persona 吗？**  
   不会进生产 Main system。System 只有 stable protocol/Core Persona；relationship/state/scene/memory/summary 都在 delimited Runtime Context user message。

8. **Recent Conversation 是否保留真实 roles？**  
   是。选中的近期消息以真实 `user`/`assistant` transport roles 按时间顺序发送。`role=tool` 只出现在 dedicated continuation。

9. **Runtime Context 与 Current User Message 是否分离？**  
   是。Runtime Context 是独立 delimited user message；Current Input 恰好一次、永远在最后一条 user message。

10. **A/B/C 中最佳组织是什么？**  
    B。真实模型工具决策与 Fact/Persona/Instruction boundary 证据支持 B，它又避免 C 将动态 facts 提升到 system authority 的长期风险。

## 最终状态

- S01–S12 核心实现与验证完成。
- `check-go-projections.sh` 获得用户明确环境性豁免。
- 无关 `.trellis/tasks/09-08-project-health-evolution/` 未被触碰或 stage。
- 无 push/merge/reset/checkout。

