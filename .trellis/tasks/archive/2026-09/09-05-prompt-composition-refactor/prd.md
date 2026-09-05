# 提示词构成重构

## Goal

在不改变各业务 workflow、Provider role、结构化 schema、Tool Call 和 `media_prompt` 专属协议的前提下，重构普通 LLM 调用的 prompt composition：

- `system` 只承载固定运行协议、当前操作规则和过滤后的 Core Persona；
- 动态事实、记忆、目标、意图、最近对话和当前时间进入独立的动态上下文 message；
- 过滤数据库/状态机内部元数据，保留模型理解和证据引用真正需要的语义字段；
- 列表型上下文优先使用稳定、可转义的 TOON，嵌套结构继续使用 YAML-like 格式；
- 保持 Provider 边界只有一个位于首位的 `system` message。

`media_prompt`、`media_quality_acceptance`、Visual Identity 媒体 prompt 的特殊协议暂不套用普通 persona/system 重构，只保持现有 media contract，并为后续单独决策保留清晰边界。

## Confirmed decisions

- `# 运行协议` 是 system 的固定规范区，包含语言、约束优先级、context 绑定、claims/evidence、Tool Call、禁止伪造事实和禁止越权修改人格层级等规则。
- `# 人格设定` 只放初始化/Owner-governed 的 Core Persona 语义层：`identity`、`personality`、`behavioral_policy`、`life_profile`。
- `schema_version`、persona/claim/message/goal 等内部 `id`、revision、created/updated timestamps、状态机/持久化字段、transport metadata 不进入模型 prompt。
- `evidence_refs` 是证据锚定语义，不按普通数据库 ID 处理；在需要证据引用的 memory/developing-self/reflection 区块中保留短而稳定的引用。
- Memory 的 `created_at` 是模型理解记忆新旧程度的语义时间，按规范保留；原始数据库审计时间、revision、FK 不保留。
- `life_context.current_time` 和 canonical `timezone` 是动态上下文必需事实；Core 内部原始 RFC3339 `instant` 不进入 prompt。
- `developing_self`、`current_state`、memory、goals、intentions、recent messages 不进入固定 system persona 区。
- Memory、Developing Self claims、goals、intentions、recent messages 等同构列表显式使用 TOON；嵌套 relationships、slots、tool results、complex context fallback 使用 YAML-like。
- 仅当 TOON 的 header/cell 安全转义且 rows 同构时才使用 TOON；否则必须可预测地 fallback 到 YAML，不得生成歧义文本。
- 所有 role-specific operation instructions 归入 `# 运行协议` 下的 operation rules 区，而不是伪装成 Core Persona 字段。

## Confirmed repository facts

- Provider 边界当前由 `prependSystemMessage` 合并所有 system，并保证只有一个首位 system；见 `apps/core-go/internal/core/provider_language.go:30-55`。
- 目前 system 来源分散于 `provider_prompts.go`、初始化/日程/Visual Identity 调用方、context authority wrapper、中文语言 wrapper 和 media 特例；见 `apps/core-go/internal/core/provider.go:147-160`、`app.go:255-264`、`schedule_generation.go:18-43`、`provider.go:297-316`。
- `compactCognitionContext` 已经只保留 canonical `core_persona`、`developing_self`、`current_state` 和非空 evidence collections，但最后的全局 `stripProviderMetadata` 会删除 `evidence_refs` 与 memory `created_at`；见 `apps/core-go/internal/core/provider_context.go:24-71,542-595`。
- Core Persona 的初始化 schema 要求 `identity`、`personality`、`behavioral_policy`、`life_profile`；见 `apps/core-go/internal/core/provider_schemas.go:187-231`。
- `life_context.current_time` 已由 Core 按 IANA timezone 注入，原始 `instant` 仅用于 Core/replay；见 `apps/core-go/internal/core/intelligence.go:216-239`。
- 当前 formatter 以 `role != media_prompt` 作为 TOON 开关，并对 JSON-like string 做 opportunistic TOON/YAML 转换；见 `apps/core-go/internal/core/provider_prompt_format.go:22-35,106-193`。
- `recent_messages` 当前被压成多行 scalar string，而不是表格；memory/goals/intentions 只有满足动态同构条件时才自动 TOON；见 `apps/core-go/internal/core/provider_context.go:73-113,178-214,229-265`。
- `media_prompt` 同时承载普通媒体 prompt 生成和质量验收，且质量验收是 multimodal `[]any`，不会经过字符串 formatter；见 `apps/core-go/internal/core/media_quality.go:124-157`。

## Scope

1. 新增集中式普通 prompt composer，明确区分：固定协议、operation rules、Core Persona、动态 context sections。
2. 将 Core Persona 从普通 cognition context 的动态层提升到 system 的 `# 人格设定`，同时保留 legacy projection/replay 兼容输入。
3. 重构动态 context 文档的 heading 和 section ownership，确保当前时间位于 `# 当前上下文`/`life_context`。
4. 增加 section-aware TOON formatter：memory、developing-self、goals、intentions、recent messages 优先使用 TOON；嵌套数据 fallback YAML。
5. 收窄 metadata stripping：删除内部 ID/revision/status/FK/transport 字段，但保留语义 `evidence_refs`、memory `created_at`、当前时间和必要 provenance 摘要。
6. 统一普通 role 的 system composition 顺序，并保留 `media_prompt`/media quality/Visual Identity 的现有特殊路径。
7. 更新 provider/context/prompt tests，覆盖初始化、conversation、realization、daily review、wake-up、native cognition、reflection 和 media 边界。

## Out of scope

- 不改变 `media_prompt` 的英文输出协议、Visual Identity system instruction、media quality multimodal payload 或 media provider prompt master。
- 不改变 Provider role/binding、workflow、Tool Call schema、结构化 response schema、业务决策和 reflection policy。
- 不把数据库内部所有 evidence ID 删除；经过协议认可的 evidence references 继续保留。
- 不把所有对象强行转换成 TOON；嵌套对象、multimodal content 和不安全 cell 使用 YAML/原生 transport。
- 不实现新的 UI、诊断、Redis 或 workflow 功能。

## Acceptance Criteria

- [x] 普通非-media Provider 请求最终恰好拥有一个首位 system message；system 顶层只包含 `# 运行协议` 和 `# 人格设定`，operation-specific rules 位于运行协议区。
- [x] `# 人格设定` 只包含 Core Persona 的语义字段：identity/personality/behavioral_policy/life_profile；prompt 中不存在 schema_version、实例/claim/message/goal id、revision、数据库 timestamps、FK 或 transport metadata。
- [x] 动态 user/context message 明确包含 `context.scene/activity/location/mood/appearance`、`life_context.current_time` 和 `timezone`；原始 Core instant 不出现。
- [x] Developing Self、memory、goals、intentions、recent messages 以 section headings 分隔，并在同构列表时稳定输出 TOON；嵌套或不安全数据可预测地 fallback YAML。
- [x] Memory/developing-self 的 evidence references 和 memory creation time 按语义规范保留；无关持久化状态、revision、visibility、FK、source/event IDs 不泄漏。
- [x] Core Persona、Developing Self、Current State 的 authority/priority 语义保持 `core_persona > developing_self > current_state`；reflection/realization 不会因为 formatter 重构写入 Core Persona。
- [x] 初始化、认知评估、回复实现、日评、唤醒、原生 cognition、反思的现有 operation instruction、schema、tools、streaming 和 structured response 行为保持兼容。
- [x] `media_prompt`、`media_quality_acceptance`、Visual Identity media prompt 保持现有英文/YAML/multimodal 特例，不被普通 composer 误改。
- [x] 现有 Provider/context/format tests 扩展到实际 memory/goals/intention/recent-message、多行、TOON delimiter escaping、metadata/evidence preservation、current-time 和单 system 顺序；Core/BFF/Web 无接口回归。
