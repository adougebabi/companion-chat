# 设计：唯一原生 Agent 循环与独立 Tool 执行

## Architecture and boundaries

复用现有 `internal/ai/agent`、`internal/ai/model`、Core CapabilityRegistry 和领域服务。采用显式 Agent 定义/装配注册与 Tool 注册，不新建插件平台。新增普通 Tool 只新增定义与 executor/领域服务绑定；新增 Agent 只新增自身输入、prompt/context、model/tools/output 绑定。通用 loop 与 executor 不接收 Agent 名称来切业务分支。

正式路径：业务入口 → 对应 Agent 输入装配 → Eino ChatModelAgent/Runner → 正式 Tool executor → 领域依赖/短事务 → 原生 tool result → 下一次模型决策 → 最终输出契约 → 调用方接收。

直接路径：已授权的应用调用方 → 正式 Tool invocation（业务输入+资源+operation ID）→ 同一个 executor/领域服务 → 同一结果序列化。Eino adapter 只附加真实 native call ID 并转交执行；直接调用不制造 model call。App 可作生产 composition root，但独立 Tool 最小装配只接入所需服务，不启动 Main/worker 全流程。

## Agent definitions

每个完整任务定义持有名字、任务 prompt/版本、typed input/context assembler、模型角色、工具集合、final decoder 和运行保护。继续复用现有 prompt 与 context budget/slot 组件；任务相关输入在各自边界校验。research/inventory.md 中 16 类完整任务全部迁移；视觉身份完整任务有独立总入口及循环证据，vision/patch 既有职责与 prompt 不丢失。无工具任务也通过同一 Runner，允许一轮自然完成。Embedding 不纳入决策 Agent。

保留每次物理模型调用的 Provider queue lease、diagnostics、预算与请求身份。移除 schema allowlist 的运行时旁路；final schema 只用于最终 assistant，无工具的中间结构不得提前终结循环。不缓存前轮正文作为最终校验失败兜底。

Eino v0.7.37 原生 MaxIterations 保护可沿用默认 20 或显式 agent 配置；删除 <=2 规则、deferred-stop、按写工具类型超限成功。超限、取消、依赖错误准确返回 error 并附带实际 partial outcome。默认工具按序执行可保留以保证读后写/写后读；同轮多个 ToolCall 各自保留 identity 与结果，不能去重掉不同操作或抹掉中间发布。

流式入口消费原生模型 stream，工具调用增量先汇聚成正式请求再执行；最终结构化内容仅在完整结束后校验。既有 NDJSON 仍传递已提交 action_result/message/media，不能把中间候选或 reasoning 发布。若正文需要投影，使用正式 typed contract，不引入通用文本解析兜底。

## Tool contract and operation identity

正式 invocation 区分：

- 授权主体与目标资源（owner/Fluctlight、conversation/asset/profile 等确实需要的 ID）；
- 工具业务参数、必要证据来源与版本/CAS；
- 稳定 operation ID（由业务命令/任务产生并持久化，用于重试）；
- 可选 run ID、真实 native tool-call ID、Provider request ID 等诊断关联。

source fact 可为实际业务证据，不能要求必须存在一次认知轮次。直接写记忆等路径用正式命令来源和授权证据，不伪造 cognition_inbox/persona 快照。资源确属某 Fluctlight 时仍需目标 ID；不要求其人格语义或 surface 才能执行无关工具。Agent 工具装配可按职责选集，Tool 执行本身不设来源场景门禁。

结果区分 completed、真实 accepted（任务 ID）、业务 rejected/failed；运行异常保留 Go error/cause，业务失败则序列化给模型继续决策。返回应包含真实资源/版本/结果及 replay disposition；不能把 queued 写成 completed。参数解析与结果序列化只维护一份。

幂等在现有领域 command/outcome ledger 上收敛：scope+tool+operation ID，绑定规范化 payload digest；相同操作返回既有结果，不同 payload 冲突。若已有表无法表达独立操作，则只允许必要的局部增量迁移与隔离测试，绝不生产迁移或另建第二运行平台。模型不同 call ID 不能导致同一业务命令重做；业务新操作也不能因参数恰好相同被错误合并。

## Commit and publication changes

工具执行拥有完整局部业务事务：命令幂等/领域写入/结果/outbox 同事务，提交后才返回已完成。需要 Provider 的 schedule/media prepare 在事务外执行，apply 再做 CAS，不持锁等待模型或媒体。

- 回复/动态：提取复用现有发布服务，消息/Moment 行和必要 outbox 由 Tool 提交；外层只传递已提交事件，不再次发布最终 assistant 候选。无显式 reply Tool 的自然最终回复由 Agent 的输出适配调用同一正式发布服务一次，不让外层执行工具协议。
- 媒体：保留现有 durable intent/MediaWorkflow/ComfyUI 及对象存储；直接 Tool 自己处理合法目标并受理，真实任务 ID 与产物状态可追踪。既有异步受理能力明确 accepted，完成验证跟踪同任务最终资产；不把同步承诺批量改为提交任务。
- scene/presence/affect/memory/active memory/capability request：复用领域 reducer/lifecycle，在工具拥有的短事务中提交，保留业务授权、CAS、有效证据及 outbox。
- persona switch/takeover：提取现有领域变更与审计，工具返回实际已应用/拒绝结果；判定 prompt 仍归对应 Agent，不让模型直接控制数值/权限。
- visual identity：初始化解除 wake_up_ 门禁；生成、真实看图、评审和保存通过正式 Agent 与业务工具闭环。复用 visualIdentityImageContent 当前真实图片路径及 canonical CAS；Temporal 继续管理 durable intent/重启恢复，不再作为模型续接的第二实现。

必要行为变化：工具已提交即为事实，后续 Agent 失败不再整轮原子回滚；失败运行必须保留消息/记忆/媒体已提交资源与 outcome，调用方返回失败并准确呈现已有结果，不能谎称整个任务成功或不存在副作用。重试必须恢复命令/任务状态，禁止重跑整轮重做已提交操作。

## Removal and compatibility

按 producer/consumer 迁移后删除 query_continuation、第二轮 no-tools 特例、caller settlement 执行器、只生成 deferred 的业务实现、隐藏场景检查、action-only 超限成功、过滤中间执行事实逻辑、正文/sidecar tool fallback。保留有效事务、outbox、CAS、权限、取消、人格/视觉等产品能力。旧测试按产品契约迁移，记录旧断言与新证据对应，不按旧实现限制维持兼容。

规范更新原文件 structured-turn-contract.md、fluctlight-cognitive-runtime.md、visual-identity-contract.md 及受影响 memory/autonomy/provider/persistence 章节；明确 superseded 条款，不能留下互相冲突的有效规则。

## Evidence and safety

期望集合使用已确认产品清单作固定基线，新增提取能力显式登记；不可用当前 registry 自动生成 expected。每个 Tool/Agent 行记录 case ID、真实/受控类型、command/exit、commit+working-tree digest、run ID、request/call/operation ID、结果/下一输入/实际产物证据。敏感请求与图片可仅保留安全摘要、哈希和受控本地路径；诊断不能泄露密钥或用户隐私。

隔离 PostgreSQL（每用例临时库）、Redis、MinIO、Temporal 复用已有 Compose 方式；从 `.env` 与 `fluctlight.local.env` 安全读取既有模型/媒体配置，不能打印值。测试只落隔离库/专用测试账号。真实外部服务失效仅阻塞相关验收行，继续其他实现与回归。

## Trade-offs and rollback

不引入跨模型和数据库的全局事务；用局部提交和可恢复 outcome 明确事实。串行 ToolCall 执行可保留，正确身份与结果比并行性能优先。若需要新增局部表，保持与当前数据兼容、无破坏升级。

任务作为一个集成交付直接承担代码修改，不拆成独立结案的框架/工具/Agent 子任务，因为端到端验收依赖共同切换。按切片提交/验证，不留生产双路开关。回退仅精确撤销本任务 hunk/提交，不 reset 用户工作树、不回退已发生外部效果。
