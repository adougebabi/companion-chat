# 技术设计：提示词构成重构

## 1. 设计目标与边界

本任务只改变 Provider-facing prompt 的组织和过滤方式，不改变业务决策、schema、Tool Call、workflow、Provider role/binding 或媒体生成协议。普通非 `media_prompt` role 使用新的集中式 composer；media generation、media quality acceptance、Visual Identity media prompt 保留现有专属路径。

最终普通请求的形态：

```text
system
  # 运行协议
    common protocol + operation rules
  # 人格设定
    filtered Core Persona semantic fields

user
  # 当前上下文
    context + current local time/timezone
  # Developing Self
    TOON when tabular, YAML fallback otherwise
  # 当前状态
    nested YAML-like semantic state
  # 记忆
    TOON when tabular, semantic evidence/time preserved
  # 当前目标与意图
    TOON when tabular
  # 最近对话
    TOON rows with role/kind/time/content
  # 本次用户输入
    operation-owned input
```

仍由 Provider boundary 保证最多一个首位 system，但不再依赖调用方多次 prepend 的隐式顺序。

## 2. 集中式 system composer

新增内部纯函数/值对象（名称可按现有命名调整）：

```text
composeProviderSystem(role, operationInstruction, corePersona, options)
  -> one system message string
composeProviderDynamicDocument(role, semanticPayload, sections, options)
  -> one user/context message string
```

`composeProviderSystem` 的责任：

1. 生成固定中文运行协议；
2. 将 role-specific operation instruction 放在运行协议区的 operation rules 中；
3. 渲染过滤后的 Core Persona；
4. 合并 Visual Identity/media 特例时遵守显式 role boundary，不把 media 特例混入普通 composer；
5. 只返回一个 system message，不在 formatter 中重复注入 language/context rules。

建议的固定 protocol 文本以用户已确认版本为基线，Tool Call、evidence、no-fabrication 和 context override 规则属于运行协议，不得放入 Core Persona。

## 3. Core Persona 过滤

`renderCorePersonaForProvider` 只读取 canonical `core_persona` envelope 的语义组：

- `identity`：name、role/occupation、residence、timezone、background、biography、core_values、worldview、notes；移除 identity `id` 和内部审计字段；
- `personality`：人格维度和稳定表达倾向；`update_policy` 仅在确有模型决策用途时保留可读语义，否则留在 Core；本任务默认不发送自动演化数值控制参数；
- `behavioral_policy`：response style、directness、initiative、silence tolerance、refusal style 等语义规则；
- `life_profile`：appearance、social background、preferences、life habits、recurring commitments、relationship seeds、character constraints；移除 workflow/renderer/storage metadata。

不会把 `schema_version`、persona ID、revision、created/updated timestamps、persistence status 或 provenance DB envelope 传入 system。

初始化调用尚未拥有 Core Persona 时，system 仍输出固定运行协议；`# 人格设定` 可省略或使用明确的“本次调用正在生成 Core Persona”空状态，不能伪造人格字段。

## 4. 动态 context composer

### 4.1 Section ownership

- `# 当前上下文`：`scene`、`activity`、`location`、`mood`、`appearance`、`life_context.current_time`、`timezone`、必要的 context revision-free facts；原始 `instant` 不发送。
- `# Developing Self`：category、claim、value、confidence、evidence_refs、必要的 bounded provenance summary、expiry semantics（若模型需要判断有效性）；删除 claim ID/revision/FK/audit timestamps。
- `# 当前状态`：PAD/mood/momentum/drives/conflicts/regulation 的语义值和 life context；删除 revision、decay control、storage timestamps。
- `# 记忆`：type、content、confidence、importance、emotional significance、created_at（语义时间）、evidence_refs；删除 memory ID/status/revision/visibility/FK/source event IDs。
- `# 当前目标与意图`：goals 的 description/importance/urgency/progress/state；intentions 的 goal/action/confidence/preferred_time/deadline/state；不发送 ID/revision/audit fields。
- `# 最近对话`：role/kind/time/content，保留顺序；当前 user input 已作为操作输入时从 recent history 去重。
- `# 本次用户输入`：由调用方拥有的当前操作输入，不能重复写入 recent history。

### 4.2 TOON/YAML selection

新增明确的 section-aware rendering，而不是只依赖 `role != media_prompt`：

- memory/developing-self/goals/intentions/recent messages 使用 TOON table renderer；
- table rows 必须同构且 cell 经过 delimiter-safe escaping；
- nested object、nested arrays、multimodal parts、非同构 rows、单行/空列表 fallback 到 YAML-like；
- TOON header 和 cell 的 comma/pipe/bracket/newline/quote 处理必须测试；
- system 文本永远不经过 TOON/YAML formatter；
- `media_prompt` 仍使用现有普通 YAML/media path。

建议将当前 generic opportunistic TOON 保留为底层 helper，但增加显式 section field order 和安全 escaping，避免 map iteration/optional field 顺序导致同一语义在请求间漂移。

## 5. 调用方兼容策略

短期不要求所有业务调用方一次性重写为新值对象。Provider boundary 可兼容解析当前调用方的 JSON payload：

1. 解析现有 user payload 中的 `context.core_persona`/`core_persona`；
2. 将 Core Persona 提升到 system；
3. 从动态 context 中删除被提升的 persona，避免重复；
4. 将现有 operation instructions 作为 composer 的 operation rules 输入；
5. 对无法解析的普通文本保持原样，不进行自然语言猜测。

之后逐步将 mutations/autonomy/wakeup/reflection 等调用方迁移到显式 section builder。初始化、日程生成等没有既有 Core Persona 的调用保留 operation-specific system rules，但不伪造人格。

## 6. Metadata 与 evidence policy

当前 `stripProviderMetadata` 的全局删除规则需要按字段作用域拆分：

- storage/coordination metadata：删除；
- semantic time/evidence references：保留；
- provenance：只保留 bounded semantic source/method（不保留内部表 ID、凭据或完整审计对象）；
- free text 中的 ID-like token：不得无差别删除自然语言内容；仅对结构化字段按白名单过滤。

这是本次重构的重点兼容风险。必须用“应保留字段”和“应删除字段”双向测试，而不是只测试不会泄露 ID。

## 7. Media boundary

`media_prompt` 继续：

- 使用英文 media prompt instruction；
- 不注入普通中文输出规则；
- 不使用普通 cognition TOON policy；
- 保留 Visual Identity system injection、context binding preamble、quality acceptance multimodal input；
- 不把 `media_quality_acceptance` 的 image data URL 放入普通动态 prompt 或 browser diagnostics。

## 8. 主要风险与回滚

- system 内容顺序变化影响严格 chat template：保留 single leading system contract，并增加精确字符串/顺序测试。
- Core Persona 被重复放入 system 和 user：composer 提升后删除动态副本，并测试只出现一次。
- evidence refs 被过度过滤：用 provider spec 的保留字段测试锁定行为。
- TOON 分隔符歧义：转义失败直接 YAML fallback，不输出可能误解析的 TOON。
- 老调用方 payload 无法解析：保持原文本作为 bounded fallback，不丢弃业务输入；feature flag/role gate 可独立回退到旧 formatter。
- media prompt 误套普通 composer：对 `media_prompt`、`media_quality_acceptance`、Visual Identity fixture 做 negative tests。
