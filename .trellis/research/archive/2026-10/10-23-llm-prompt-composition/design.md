# LLM 提示词统一构造设计

## 决策与最小行为差距

当前 PromptComposer 已存在，但 caller 仍有独立消息拼接、混合格式、通用 projection 过宽和递归字段清理。目标是在现有 composer/typed task 边界收敛，不重建 LLM runtime、不改变前端或模型配置。

统一结构是逻辑契约，不要求每个 Agent 收到所有数据或相同数量的 message：

1. **运行协议**：一次声明共用权威边界、一次声明任务规则；需要人格的任务加完整受信任 Working Persona，分析任务不追加空人格段。
2. **任务动态数据**：同一种确定性 JSON 编码。可按缓存稳定性拆 stable/task 与 current/runtime 两段，但每类事实仅一个 owner，不能同时出现在两段和当前输入中。
3. **按需历史**：私聊保留原始 user/assistant roles、正文、顺序；WakeUp/Moment 需要的旧聊天明确作为历史资料。默认不附带通用私聊，summary/diary/goal 的源消息属于显式任务材料。
4. **当前输入**：真实用户文本原样出现一次。机器 task payload 使用 typed JSON，不通过“看起来像 JSON”去重排用户正文。
5. **原生协议附加物**：工具 schema/ToolCall/ToolResult、response schema、图片仍走 Provider 原生结构；它们不是混用业务数据格式的理由，也不能重复塞回历史证据。

使用 JSON 的理由：已有 Tools、证据绑定和 Provider 结构均为 JSON；可直接校验/比较和稳定转义，消除当前按列表形状从 YAML 自动切 TOON 的行为。不宣称 JSON 本身一定更短，收益来自语义选择和去重。

## 现有边界内的实现

保留 `PromptComposer` 纯渲染职责。调用方负责从已授权领域快照产生任务专属模型视图；renderer 不读 DB、不做相关性推断、不重新判断权限。给 task/surface 建立明确的协议模块、动态字段和 history policy 清单，formal registry 覆盖测试确保新增 Agent 必须选择构造契约；共用 Agent 的媒体/summary/NPC 子任务单独列 case。

`ComposeAssembly` 与 `ComposeTaskMessages` 收敛到同一个结构化编码入口，迁移 raw 双消息 caller。媒体仅编码 text part，图片保留；Kev 仅统一 state 的语义选择与 JSON，维持外部 `{state,questions}` API。修复请求使用明确的 validation/quality feedback，不复用父 agent 自由草稿来指挥 Writer。

## 最小必要字段与引用

- 对当前状态、Actor、日程、目标、memory、Studio、NPC、媒体等分别建立白名单。纯 DB bookkeeping（row id、revision、version、digest、workflow/request/operation identity、纯审计时间、lease/retry 元数据）留 Core/诊断。
- 业务发生时间、有效区间、deadline、timezone、unknown/false、不同主体、证据有效性必须保留。
- 必需实体选择使用已有授权 opaque refs；原 ID/version/CAS 在 Core 绑定并校验。确需模型选择的业务标识或外部协议字段是显式例外，需在视图注释和测试说明用途。
- 不按任意 `_id`/`_ref` 后缀递归删除/改名。不改写自然语言、用户 JSON、领域正文或 schema 的 property 名。对象发现同样只遍历已声明实体节点。
- Native ToolResult 的真实 JSON 不经一般 formatter 修改。若某 Tool 要向模型精简返回值，使用该 Tool 明确的模型投影并保持 receipt 业务语义。

## 去重与样本修复

- GoalEvaluation 独立 surface/profile：冻结目标/criteria/阶段/承诺由 stable+current 专属 packet 提供；去掉重复的普通 runtime goals，对非选中目标的依赖背景以专属依赖视图承接。
- 历史 catalog evidence 保留实际查询到的工具名、加载/可用状态和 admission；不把执行工具参数说明误作目标达成事实。确实需要完整结构的领域数据仍保留原义，不能再次经过任意字段改写。catalog 当前原生调用可继续返回完整有效 schema。
- 完全相同的 evidence data 只编码一次，source envelope 通过短 payload_ref 指向它；每个 eN、发生时间、valid/can_support_success、goal refs 保持独立。不同有效期、主体、含义或 schema 变化不能错误合并。仅在确有重复时建立共享正文，避免无收益间接层。
- schedule 仅发送一个规范时间对和时区，当前 item 引用 items 中唯一记录；当前事实优先于计划。保留真实发生时间与发送者原时区，不统一成错误的主体时区。
- 只按精确内容及完整语义身份去重，不使用模糊文本匹配；同来源不同事实保持。共用规则按模块装配去重，不能用全文字符串删除信任边界。

## 对话与记忆

保留用户已认可的记忆/摘要优先机制，不引入“固定 10 句”或另一条数配额。只在实际选入的记忆/摘要完整覆盖整组消息时省略该组原文；半组覆盖保留整组，最新必要交互仍受保护。summary 当前事实权威不超过 authoritative current state，旧聊天不是当前用户命令。保留 current-input 精确排除、真实触发 user 回补、Actor/时间/权限和已发送正文。

`daily_review`、Moment 和其他非私聊任务从默认允许 H 改为显式任务需要时才取历史资料；其必要的事件/消息证据由任务源范围提供，不通过原始 role transcript 误造当前对话。反思与原生认知原有不带通用 H/S 的边界保留。

## Token 与兼容性

本次绝不新增、恢复、提高、降低或换名设置 prompt/context 或 response/output token 上限；不增加 section token target、自动按估算截断、预算拒绝或压缩回退。估算仅作诊断。无关字段删除、相同内容复用、源覆盖和任务相关性选择不依赖 token 数值。

Provider 原生模型窗口的真实失败仍按现有错误上报；不借此修改模型 settings。DB 兼容 token 列、业务查询分页、既有 summary 产品长度/最大 Tool 参数/schema 和迭代次数不属于本次限制调整。Planner 的初始分页/详情查询契约保留，移除无必要元数据时必须保持 CAS/ref 绑定，不把 raw ID 删除当成功。

旧 durable Writer 草稿的 contract/version/hash 仍遵循冻结版本恢复；新渲染格式若改变协议 hash，仅用于新任务并保留旧 contract 解码/续接所需兼容。不得把旧草稿静默标成新协议。无需 DB migration 或浏览器接口修改。

## 文件边界

- 公共构造：`internal/ai/prompt/format.go`、Core `provider_prompt_composer.go`、`provider_prompt_format.go`、`prompt_context_assembler.go`、`prompt_slots.go`、`formal_agents.go`（若需契约元数据）。
- 数据视图：`provider_context.go`、`fact_only_context_views.go`、`fact_only_actor_wire.go`、`working_memory.go`、必要的 time/summary/history helper。
- 样本根因：`goal_evaluation_task.go`、`goal_evaluation_source_input.go`、`goal_evaluation_wire.go` 与对应 tests。
- 入口迁移：`model_tasks.go`、`reply_context.go`、`diary_runtime.go`、`studio_runtime.go`、`world_encounter_response.go`、`moment_npc_interactions.go`、`goal_planner.go`、`media_*`、`visual_identity_agent.go`、`persona_compilation.go`、`turn_takeover.go`、`owner_life_requests.go`、`kev_*`、`vision_probe.go`；只改 prompt view/composition 必要部分。
- 次级调用：`eino_model_runtime.go` 中 repair/correction 消息构造，保留循环、Tools 和副作用边界。
- 防回退：AGENTS.md 的非托管段、新 `prompt-composition-contract.md`、backend index，修正文档中已废弃预算要求，发布 `docs/llm-prompt-composition.md`。

不会编辑 `apps/web`、其他 session 的任务记录、数据库事实、Provider endpoint/模型/timeout 或业务目标政策。子代理只做探索与独立核验，产品代码和最终验证由主会话负责。

## 风险和回退

最大风险是误删证据语义、断开 opaque ref、改变 Writer 冻结 contract 或把历史当当前输入。使用独立语义 fixture 与真实序列化请求测试。按公共 renderer、Goal surface、其他入口三块落地，每块可单独回退提示词实现，始终保留无 token 上限规则。真实模型风格/成功率没有离线保证，报告中区分本地传输回归与 live Provider 证据。
