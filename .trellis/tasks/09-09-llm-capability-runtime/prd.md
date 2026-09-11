# LLM Capability Runtime 全量重构

状态：in_progress。Inventory、迁移映射、技术设计与执行计划已经收敛；用户已要求在专用 worktree 中开始实现，任务已通过 `task.py start` 激活。

## 目标与用户价值

把当前 Go 项目中真正暴露给 Main LLM 的全部本地 Tool，一次性迁移到统一、可扩展的 Capability Runtime。Main LLM 只决定“做什么”，Capability 负责“怎么做”，Context Resolver 提供执行所需知识/状态，Runtime 只做通用编排。迁移后新增一个本地 Capability 原则上只需实现、定义上下文需求并注册，不修改 MainAgent、具体名称分发、PromptManager 或逐 Tool Schema 分支。

## 用户已确认的事实与约束

- U01 已有 PoC 验证 Thick `image_generate` 可收敛为以完整行为意图为边界的 Thin Tool；本任务不再论证 Thin Tool 是否可行。
- U02 当前 Main LLM Tool 预计少于 10 个，但真实清单、参数、执行器、上下文与 Prompt 依赖必须以代码扫描为准，不能根据需求示例推测。
- U03 迁移采用“基础设施建立 → 全量迁移 → 一次性切换 → 清理旧链 → 全量验证”；最终不得长期保留 Old Tool / Legacy Adapter / Compat Executor / 双 Schema Builder 双轨。
- U04 Thin 表示只暴露 Main LLM 做行为决策时应决定的字段，不等于所有能力只能有 `intent string`；Query 或天然需要 `target`、`scope` 等决策字段的能力可以保留必要参数。
- U05 Capability Definition 的说明只回答“能做什么”；全局行为原则属于 Agent Policy，执行细节与业务参数构造属于 Capability Implementation。
- U06 全局 Policy 需要保证：当回复声称已执行、正在执行或将立即执行会改变系统状态的行为，且存在相应能力时，必须真实调用该能力；不得把逐 Tool 触发规则重新塞回各 description。
- U07 MainAgent / Runtime 不得根据 `image`、`scene`、`social`、`memory` 等具体业务能力名称做 `if` / `switch` 路由。
- U08 Capability 声明执行所需上下文，Runtime / ContextResolver 统一解析；避免 Capability 到处直接拉全局状态，也避免退化成遍地 `map[string]any` 与 type assertion。
- U09 复用当前项目的 LLM SDK、依赖管理与领域服务；不为“架构感”机械照搬 Java 或引入大量无意义 interface。
- U10 所有潜在 IO 必须沿调用链传播 `context.Context`；多 Tool Call 的执行顺序先保持当前语义，不默认并发状态修改能力。
- U11 实现与验证必须位于新 worktree `/private/tmp/local-ai-companion-llm-capability-runtime`、分支 `codex/llm-capability-runtime`；不得在 `master` 当前检出中开发。
- U12 普通 conversation 保持一次 Main LLM cognition；CapabilityResult 不在同一轮触发第二次 Main LLM continuation。Image/Schedule 的 capability-local structured planner 只负责 HOW，不生成第二份 MainAgent 回复。

## 仓库确认事实

- C01 生产 Registry 实际注册 11 个 Tool，而非少于 10 个；普通 conversation 暴露 10 个，WakeUp/daily review 暴露全部 11 个，native cognition 暴露 8 个。权威清单和源码锚点见 `research/current-tool-inventory.md`。
- C02 项目已有 `CapabilityManifest`、`CapabilityRegistry`、`CapabilityExecutor`、`ToolCallV1/ToolResultV1` 与统一 OpenAI-compatible renderer，但尚无 `ContextSlot`、`RequiredContext`、`ContextResolver` 或 executor-facing `CapabilityContext`。
- C03 当前 conversation 只有一次 structured cognition Provider 调用；ToolResults 写入 frozen action，但不会构造成 `role=tool` 消息并在同一轮触发第二次 Main LLM continuation。
- C04 当前多 Tool Call 按 Provider slice 顺序串行扫描；immediate calls 先执行，deferred output calls 在 assistant/Moment 获得持久 ID 后按原相对顺序结算。`parallel` 目前只是 manifest metadata。
- C05 只有图片调用把 cognition-time scene/state/visual identity/appearance 冻结进 arguments；其余 stateful executors 多数通过 `App` 直接读取 live PostgreSQL authority。`ContextProjection` 是 eager cognition read model，不是统一 executor context。
- C06 现存具体名称耦合至少包括：conversation catalog 排除 `moment.publish`、native catalog 排除三项、image preflight switch、image context binder、reply action detector、`capability.request → conversation.reply` 兼容改写。
- C07 当前全部 11 项 schema 经生产 renderer 序列化为 7,857 bytes；普通 conversation catalog 为 7,564 bytes；最大单项为 `schedule.replan` 1,514 bytes、`memory_event` 1,375 bytes、`scene_event` 1,099 bytes。
- C08 Core/BFF 基线 test/race/vet/build/gofmt 均通过；DB/live-provider 测试按环境变量 opt-in，受限 sandbox 的本地监听失败属于环境限制，不是仓库基线失败。
- C09 MCP 不在当前 Go 产品运行时或 Main LLM Tool 主链中。
- C10 capability 名称、call/provider/workflow/idempotency ID 与 target kind 已写入 frozen actions/intent/replay边界；本次保留现有 11 个稳定名称，通过 surface metadata 修正暴露范围，不做无价值重命名。

## 功能要求

- R01 形成完整的当前架构 Inventory：所有真实暴露给 Main LLM 的 Tool、用途、参数、description 复杂度、Schema 来源、执行器、Context 依赖、Prompt 依赖、Tool loop/result 回传路径，以及与 Capability 明显耦合的 response format。
- R02 逐 Tool 输出迁移映射：新 Capability 名称与稳定性、类型、Thin Input、Required Context、内部 Planner/Executor；逐字段说明保留、下沉实现、改由 Context 提供或删除的理由。
- R03 建立项目风格一致的 Capability 核心概念与通用执行链，至少覆盖 Definition、Invocation、Result、Registry、ContextSlot、ContextResolver、CapabilityContext、Runtime 和 Definition 到 provider Tool Schema 的统一渲染边界；确切类型和 package 以代码证据决定。
- R04 Capability Type 至少评估 ACTION / QUERY / INTERNAL，用于日志、审计和未来策略边界，不得成为具体能力或主执行路由的 switch。
- R05 Registry 统一负责注册和按稳定名称查找；重复名与未知名必须有确定行为；新增本地 Capability 不要求修改主执行链。
- R06 Capability Result 能以统一、provider-safe 的结构持久化，并在后续符合产品语义的 cognition 中提供给 LLM，同时允许差异化业务 payload；Runtime 不理解业务结果结构。同一 conversation turn 不做 ToolResult continuation。
- R07 Capability 错误至少可区分 not found、invalid arguments、context resolve failed、execution failed，并使用 Go `error`、`errors.Is/As`、`%w` 风格而非复杂异常层级。
- R08 Capability Runtime 日志至少包含 capability、call ID、type、intent（按安全策略裁剪）、required context、duration、成功/失败；普通日志不得输出完整 Prompt 或敏感上下文。
- R09 将现有全部本地 Tool 迁移并切换 Main Agent 主流程；迁移完成后删除可删除的旧 registry、dispatcher、schema builder、旧 Tool 定义、兼容 adapter 和重复 Prompt 规则。
- R10 增加简洁的 Capability 架构规范文档，覆盖设计目标、核心对象、执行流、Thin/description/context 原则、新增能力步骤和用户指定的禁止事项。
- R11 若 MCP 尚未属于当前本地 Tool 主链，本任务不迁移 MCP；设计不得阻断未来把 Local/MCP/HTTP/Workflow 作为能力来源或 executor，但当前不实现复杂 Provider/Discovery 系统。
- R12 对 Tool schema 序列化大小做重构前后对比；项目能直接取得 token usage 时同时记录 inputTokens，否则不新增 tokenizer，仅报告 bytes/chars。
- R13 保留当前 11 个稳定 capability 名称和已发布的持久 identity；使用 Definition 的通用 surface metadata 决定 conversation/WakeUp/autonomy/native-cognition 可见性，调用点不得维护名称排除列表。
- R14 Image 与 Schedule 的实现细节由 capability-local structured planner/assembler 生成，内部 planner schema 不进入 Main LLM Tool catalog；planner 失败必须返回 capability execution failure，不允许回退旧厚 schema 或语义启发式。
- R15 对本轮必需的状态改变 Capability，执行失败必须 fail closed：不得持久化一条可能声称该状态已经改变的 assistant reply；可选内部能力失败继续以结构化结果记录，不得伪造成功。
- R16 与 Capability 直接耦合的 response format 只保留一个 root `tool_calls` sidecar 作为 provider codec 兼容输入，删除嵌套 `response_plan.tool_calls` 重复定义；native 与 sidecar 最终进入同一 Invocation/Runtime，不形成双执行链。

## 验收标准

- AC01 Inventory 表逐项覆盖代码中所有 Main LLM 可见 Tool，且每项具有可核验的源码锚点；不存在仅依据需求文本猜出的 Tool。
- AC02 每个现有 Tool 都有唯一的迁移去向和字段级边界判断；不适合单 `intent` 的能力明确保留必要参数并记录原因。
- AC03 provider tools 由 Capability Definition 经一个通用渲染入口产生；行为型 Thin Schema 测试证明没有重新暴露相机、workflow、数据库 key 等实现参数。
- AC04 Registry 测试覆盖注册、查找、重复名、未知名；ContextResolver 测试覆盖声明、正确加载与缺失 Slot；Runtime 测试覆盖 invocation → registry → context → capability → result。
- AC05 Dummy Capability 仅通过实现/注册即可被主执行链发现、渲染并调用，测试过程中无需修改 MainAgent、名称 switch、PromptManager 或 Schema switch。
- AC06 全量搜索确认 MainAgent 与通用 Runtime 不含按具体 Capability 名称分支；如存在不可删除处，最终报告逐处给出文件锚点和保留理由。
- AC07 当前全部本地 Tool 已迁移到单一 Capability 执行链，旧执行链、v1 runtime codec 与临时 compat/adapter 已删除；只保留线性数据库 migration 中不可删除的一次性结构转换，仓库不存在长期双轨。
- AC08 全局 Policy 只新增通用“语言声明必须对应真实状态改变”规则；Capability description 保持简短，未为行为回归测试加入逐 Tool 触发提示。
- AC09 行为回归至少覆盖普通聊天不强制调用、查看当前形象触发图片能力、明确场景改变真实执行、社交发布请求可调用相应能力；以确定性单元/集成测试和项目现有 Agent 测试能力为准，模型概率性结果不得伪装成确定通过。
- AC10 `go test ./...` 及仓库已有相关 lint/build/test 通过；若存在基线失败，须在改动前记录，并在最终报告区分 existing failure 与 regression。
- AC11 所有可能 IO 的实现保持调用方 `context.Context`，没有以 `context.Background()` 截断取消链；多调用顺序与原有语义一致或有明确、已测试的变更决策。
- AC12 架构文档明确列出十项禁止事项及新增 Capability 的最短路径；最终报告含重构前后架构、迁移后清单、删除项、Prompt 变化、测试、schema/token 对比和已知问题。
- AC13 五项演进检验均可回答“是”：新增 `appearance_change` 不改 MainAgent；Image 底层参数增长不扩大 Main LLM Schema；Social 实现替换不影响边界；未来 MCP 能沿同一 invocation/runtime 模型接入；未来 Discovery 可在 registry/definition 边界演进而不重写业务 Capability。
- AC14 `master` 工作区未承载本任务代码改动；所有任务产物、实现与验证均来自 `codex/llm-capability-runtime` worktree。
- AC15 Image/Schedule 内部 planner 的失败不会调用旧厚 Tool、不会生成 heuristic fallback、不会伪造成功；普通 Main LLM catalog 永远看不到内部 planner schema。
- AC16 普通 conversation 的确定性测试证明每轮最多一次 Main LLM cognition；Capability 内部 planner 调用不会进入 MainAgent reply flow；必需状态改变失败时没有不实 assistant reply 被持久化。
- AC17 Provider 测试证明只有一个 structured Tool Call sidecar，native calls 仍优先，任一形态只被执行一次；response format 其余非 Capability 字段未被无范围重构。

## 明确不在范围内

- 不实现 MCP 迁移、Lazy Capability Discovery、Embedding/Vector Search、Capability Search、Event Bus、Workflow Engine、新 DI 框架、复杂泛型框架、Policy DSL 或通用 Plugin Framework。
- 不无范围重构所有 response format；只处理或记录与 Capability/Tool 行为控制直接耦合的部分。
- 不以渐进式单 Tool 长期兼容作为交付终态，不把临时 migration bridge 当作最终架构。
- 不重新验证 PoC 已证明的 Thin Tool 可行性；只需要将原则落实为项目可维护架构与回归保障。

## 规划证据与权威文档

- 当前代码清单与源码锚点：`research/current-tool-inventory.md`。
- Prompt、Runtime、schema 大小和测试基线：`research/prompt-runtime-baseline.md`。
- 适用项目规范：`research/spec-constraints.md`。
- 技术边界和逐 Tool 迁移映射：`design.md`。
- 有序实施、回滚与验证计划：`implement.md`。
