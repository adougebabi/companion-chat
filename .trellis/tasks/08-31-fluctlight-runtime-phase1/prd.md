# Fluctlight Runtime phase 1: cognitive contract and transport/tool-call seam

## Goal

收敛 Go-only Fluctlight Runtime 的第一阶段边界，使后续的 Memory、Reflection、Autonomy、Life World 以及 image/video/audio capability 可以在同一条可恢复的认知链路上扩展，而不再通过自然语言返回值猜测语义或反复更换传输协议。

本任务以已经合并到 `master` 的 `codex/go-core-worker-closure` 结果为基线，在独立工作树 `codex/fluctlight-runtime-phase1` 中推进。第一阶段优先完成合同、数据流和最小可验证 seam；不重新引入 Claude Agent SDK/HITL，也不做第四次全量框架重写。

## Confirmed facts

- 当前 Go Core/BFF/Worker、Temporal、Redis Streams、PostgreSQL outbox/inbox 已形成平台骨架；有效运行时代码位于 `apps/core-go` 与 `apps/gateway-go`。
- 交互路径当前由 `apps/core-go/internal/core/mutations.go:206-422` 在 API 进程中直接完成 assessment、realization 和消息提交；`apps/core-go/internal/core/cognition.go:20-44` 的 Worker 路径主要是 replay/no-op。
- 浏览器 turn 当前是 `POST` + `application/x-ndjson`：`packages/browser-client/src/index.ts:113-118`、`.trellis/spec/backend/fluctlight-bff-contract.md:19-42`。Core/BFF 已有增量 NDJSON translator；SSE 目前只在旧规范文字中出现。
- Go `Structured`/`StreamText` 目前允许宽松 JSON/单次整段可见文本，业务层在 `mutations.go:294-327` 通过 `action_type`、`visual_concept` 等字段猜测 action；没有统一的 native `tool_call` canonical boundary。
- Memory 已有 revision/forget/embedding 的局部实现，但没有 retrieve/hybrid rank/prompt injection；Reflection 没有稳定 producer；Autonomy Daily Review 绕过 freeze/policy；Media 主要只有 ComfyUI image。
- Node 历史提交 `09a13ca` 曾实现 ComfyUI 与 h3 provider registry；h3/video/audio 不是本阶段的核心实现目标，但应由后续 capability plugin contract 承载。
- Fluctlight 是一个持续存在的个体。内部 Memory、Reflection、affect、relationship、self-model 和 autonomy 属于其原生能力；外部 LLM、embedding、image/video/audio/search 等属于可替换 capability plugin。插件可用不等于允许任意数据库写入；领域状态仍由 Runtime owner 管理。

## Requirements

### R1. 明确三类边界

定义并记录：

1. Fluctlight 原生能力：cognition、context、affect/drive、Memory、Reflection/evolution、Relationship、Goal/Intention、Autonomy、Life World、self-model。
2. 外部 capability plugin：LLM structured/streaming、embedding、image/video/audio、search、notification 等。
3. Runtime 基础设施：ordered mailbox、transaction/UoW、revision/CAS、outbox/inbox/effect ledger、Temporal、scheduler、diagnostics、replay/evaluation。

插件不得直接拥有 Fluctlight 领域表、绕过 policy/revision/freeze，或自行创建第二套 workflow/state source。第一阶段不引入人工审批；插件安装并通过 capability preflight 后即可被 Runtime 调用，但仍需遵守资源授权、schema、超时、预算、幂等和取消合同。

### R2. 固定交互认知链路

第一阶段的 canonical flow 为：

```text
CognitionFact
  → ContextSnapshot
  → typed Assessment
  → DecisionProposal
  → deterministic validation/policy/revision check
  → FrozenAction
  → Effect/Plugin invocation
  → Outcome settlement/reconciliation
  → visible projection
```

每次 provider 请求和外部效果都必须能通过持久化 ID 重建来源、输入快照、state revision、policy/schema/prompt/provider version 与最终状态。不得从可见自然语言、关键字或缺失字段推断新的语义效果。

### R3. tool_call 作为 provider-to-runtime 控制协议

恢复并统一 native tool call，但限定其职责：

- `tool_call` 只表达模型对 Runtime/capability 的结构化意图；不直接执行数据库或外部副作用。
- Runtime 在一个 canonical boundary 将 OpenAI-compatible/native sidecar/旧兼容格式归一化为 typed `ToolCallV1`，严格校验 name、arguments、schema version、call id、source fact 和 capability manifest。
- 通过 capability registry 找到插件，由 Runtime 执行 pre-validation、授权/资源范围、幂等、超时、重试、取消和结果分类；执行结果以 typed `ToolResultV1` 回注模型并持久化。
- 对没有原生 tool-call 能力的 provider，只允许同一 schema 的结构化 JSON sidecar；禁止解析自然语言 marker 或“返回数据后再猜 action”。
- 内部 Memory/Reflection/affect/evolution 仍由 Runtime native service/authority port 拥有，不把数据库操作暴露成任意工具；可对模型暴露受限的意图能力，但最终由领域服务提交。
- 不引入人工审批/HITL 状态；若能力不可用、输入非法、revision 冲突或资源不满足，返回确定性的拒绝/失败/延期结果。

### R4. 传输协议决策

- 交互 turn 继续使用 `POST` + `application/x-ndjson`，不为“看起来像 agent”而切换 SSE。NDJSON 适合带请求体的 POST、任意 JSON frame、现有 BFF translator、AbortSignal 和 Core→BFF→Browser 取消传播。
- 第一阶段修复真实增量传播：Provider chunk → Core frame → BFF frame → Browser，而不是等完整 realization 后伪造一个 token 帧。
- Fluctlight 的主动联系由 Runtime 产生 durable cognition/effect/message 事实；它不要求把交互 turn 改成 SSE。客户端可以通过现有查询/刷新获得主动消息。
- 保留未来单独增加 `GET` SSE subscription 的可能性，用于低延迟 server-push 的主动消息、workflow/media progress 或 diagnostics；该通道不取代 turn command，也不共享第二份状态。
- 同步清理/修正仍声称浏览器为 SSE 的旧结构化 turn 规范文字，避免合同漂移。

### R5. 可恢复与可观测合同

第一阶段必须定义 action/fact/plugin outcome 的状态和幂等语义，至少区分：

```text
proposed, validated, rejected, frozen, realizing,
completed, failed_retryable, failed_terminal,
result_unknown, cancelled, deferred, stuck
```

并建立 `fact_id`、`action_id`、`tool_call_id`、`provider_request_id`、`workflow_id`、`correlation_id`、`causation_id` 的关联规则。重试不得隐式重新 assessment 或重复提交外部 job；取消不得把“结果未知”伪装成失败。

## Acceptance Criteria

- [x] `design.md` 明确 native capability、external plugin、runtime infrastructure 的 owner/依赖方向，且不引入 HITL/审批。
- [x] `design.md` 固化 `CognitionFact → ContextSnapshot → Assessment → DecisionProposal → FrozenAction → Effect → Settlement` 的 canonical data flow，以及每个阶段的持久化 ID、版本和重试边界。
- [x] `design.md` 固化 `ToolCallV1`/`ToolResultV1` 的 provider-normalization、capability manifest、执行阶段和错误分类；自然语言 marker 不再是生产语义入口。
- [x] `design.md` 固化 transport 选择：turn 使用 POST NDJSON；SSE 仅作为未来 server-push subscription 的独立可选通道，并列出不切换 SSE 的理由。
- [x] `implement.md` 给出不超过一个阶段的最小实现顺序、风险文件、回滚点和验证命令；第一阶段代码范围不扩展到完整 Memory/Reflection/Autonomy 实现。
- [ ] 在实现阶段加入至少一组协议测试：native tool call 与 JSON sidecar 归一化、unknown/invalid tool 拒绝、tool result 回注、真实 chunk-to-NDJSON 增量传播、Abort/cancel、唯一 terminal frame、幂等重放。
- [x] 在实现前完成一次 PRD 收敛审阅，并由用户确认“tool_call 仅承载外部 capability/受限 native intent，内部 Memory/Reflection 继续由 Runtime owner 管理”这一范围。

## Out of scope for phase 1

- Claude Agent SDK、人工审批、HITL、多个 Fluctlight handoff。
- 直接引入 LangGraph/AutoGen/Letta 作为第二个生产状态运行时。
- 完整 Memory hybrid retrieval、Reflection candidate apply、Autonomy policy、Life World timezone/replan、video/audio provider 的全部实现；这些在后续阶段沿用本阶段合同。
- 将浏览器 turn 改成 SSE，或把 SSE 和 NDJSON 同时作为同一个 turn 的双重事实通道。

## Confirmed decisions

- 用户确认第一阶段追加 `tool_call` 处理。
- 用户确认交互 turn 继续使用 POST NDJSON，不因存在主动联系设定而切换 SSE。
- 用户确认 Memory、Reflection、affect、evolution 等属于 Fluctlight 原生能力；插件化是通用 capability slot，不是把固定业务逻辑搬进插件，也不引入人工审批。
- Native tool call 只作为 Runtime 与外部 capability/受限 native intent 的结构化控制面；所有实际状态修改仍通过领域 authority port。
