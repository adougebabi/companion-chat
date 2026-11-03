# Prompt 构成现状分析（2026-09-06）

## 已确认事实

- `ContextProjection` 是内部完整投影，包含 schema/version、Fluctlight/Conversation/Fact ID、各层 revision、当前用户输入和语义层；定义见 `apps/core-go/internal/core/intelligence.go:30-63`。
- `BuildContextProjection` 会通过 `annotateLifeContextClock` 注入按 IANA timezone 格式化的 `life_context.current_time`/`timezone`；Core 内部 `instant` 只用于解析/replay，见 `apps/core-go/internal/core/intelligence.go:112-239`、`apps/core-go/internal/core/detail.go:333-359`。
- `compactCognitionContext` 已经构造 canonical `core_persona`、`developing_self`、`current_state` 和非空列表；但全局 `stripProviderMetadata` 会删除所有 evidence refs、created_at 和 ID-like string，见 `apps/core-go/internal/core/provider_context.go:24-71,542-595`。
- Core Persona 初始化 schema 要求 `identity`、`personality`、`behavioral_policy`、`life_profile`；schema_version 是可选的结构字段，见 `apps/core-go/internal/core/provider_schemas.go:187-231`。默认 Core Persona 还含 identity/timezone、人格维度、behavioral policy、life profile，见 `apps/core-go/internal/core/app.go:442-475`。
- Provider boundary 当前通过 `prependSystemMessage` 将多条 system 合并成一个首位 system；调用方仍可在多个位置追加 operation/context/language 指令，见 `apps/core-go/internal/core/provider_language.go:30-55`、`apps/core-go/internal/core/provider.go:147-160`。
- 普通 role 当前按 `role != media_prompt` 二分 TOON/YAML；TOON 只对同构、scalar cell 的数组生效，见 `apps/core-go/internal/core/provider_prompt_format.go:22-35,106-193,232-284`。
- `recent_messages` 当前被压成 UTC 多行 scalar string，因此不会进入 TOON；memory、goals、intentions 仅在运行时形状满足时偶然 TOON，见 `apps/core-go/internal/core/provider_context.go:73-113,178-265`。
- `media_prompt` 同时用于 media prompt 生成和 media quality acceptance；后者是 multimodal content，不经过字符串 formatter，见 `apps/core-go/internal/core/media_quality.go:124-157`。

## 当前 prompt 调用来源

- 初始化：`apps/core-go/internal/core/app.go:255-264`，调用方提供多个 system，user 是 owner description。
- 普通 cognition：`apps/core-go/internal/core/mutations.go:356-399`，user payload 包含当前输入和 compact context。
- 回复 realization：`apps/core-go/internal/core/mutations.go:503-525`，user payload 包含 response plan、context projection、tool results。
- Daily review：`apps/core-go/internal/core/autonomy.go:54-71`。
- Wake-up：`apps/core-go/internal/core/wakeup.go:254-279`。
- Native cognition：`apps/core-go/internal/core/cognition_growth.go:257-260`。
- Reflection：`apps/core-go/internal/core/workflow_ops.go:352-364`。
- Schedule generation：`apps/core-go/internal/core/schedule_generation.go:18-43`。
- Media prompt/quality/Visual Identity：`apps/core-go/internal/core/media.go:85-100`、`apps/core-go/internal/core/media_quality.go:124-157`、`apps/core-go/internal/core/provider.go:297-316`。

## 设计决策依据

- system 采用两个顶层 heading：`# 运行协议` 和 `# 人格设定`。operation-specific rules 仍保留，但作为运行协议内部内容，不再冒充人格。
- Core Persona 只传四个语义组；schema/version、实例/claim/message/goal IDs、revision、数据库审计时间、FK、状态机和自动演化控制字段不传。
- `evidence_refs` 是 grounding 语义，不按普通 storage ID 删除；memory `created_at` 作为记忆新旧语义时间保留。
- dynamic context 通过简单 `#` headings 组织；context/current state 等嵌套结构使用 YAML-like，memory/developing-self/goals/intentions/recent messages 使用安全 TOON。
- TOON delimiter 或 row shape 不安全时回退 YAML；不得修改自然语言中的 ID-like 文本。
- media_prompt 和 multimodal media quality 保持现有特例，普通 composer 不处理。

## 实现结果与验证

- 新增 `apps/core-go/internal/core/provider_prompt_composer.go`，普通非媒体 role 统一生成单个首位 system：`# 运行协议`（含 operation rules）+ `# 人格设定`（过滤后的 Core Persona）。
- `compactCognitionContext` 不再输出顶层 schema_version；recent messages 改为带 role/time/content 的语义行；memory 保留 created_at/evidence_refs，Developing Self 保留 evidence_refs 和 bounded provenance_source；新增 scoped metadata filter，避免改写自然语言中的 ID-like 文本。
- 动态 JSON payload 按 `# 当前上下文`、`# Developing Self`、`# 当前状态`、`# 记忆`、`# 当前目标`、`# 当前意图`、`# 最近对话`、`# 本次用户输入` 等标题渲染；context/current state 使用 YAML-like，列表在同构安全时使用 TOON。
- TOON cell 对逗号、竖线、括号、换行和引号进行安全引用；普通非 media role 由 composer 处理，`media_prompt` 继续旧的英文/YAML/multimodal path。
- Provider `completeWithToolsSchema` 与 `StreamText` 已接入 composer；初始化、schedule、cognition、realization、daily review、wake-up、native cognition、reflection 均保留原 schema/tools/streaming 语义。
- 通过 `go test ./...`、`go vet ./...`、Gateway `go test ./...`、Browser Client typecheck/test、Web typecheck/build/test，以及 disposable Compose smoke（所有长驻服务 healthy，migrate/minio-init/cutover 成功退出）验证。
