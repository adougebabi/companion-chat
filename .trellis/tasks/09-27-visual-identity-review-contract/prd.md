# 视觉身份审查工具参数与失败恢复

## Goal

让自动视觉审查在模型给出有界但不同形态的 `observations` 时仍能提交或得到可纠正反馈，不因一次参数形态错误把完整 ADK run 判为失败。

## Confirmed facts

- 用户现场错误：`visual_identity_activity_failed · agent_run_failed: adk_run: ... visual_identity.commit_review ... capability_prepare_failed: invalid capability arguments: field "observations": value must be an array`。原始 ToolCall 参数未保留，不能断言非数组具体是对象、字符串还是 null。
- `visual_identity.commit_review` 要求 `observations` 为 1–24 条字符串数组（`apps/core-go/internal/core/visual_identity_tools.go:121-140`）；Agent 指令只说 bounded observations，未写明数组（`apps/core-go/internal/core/visual_identity_agent.go:64-72`）。旧 Vision response schema 接受对象（`apps/core-go/internal/core/provider_schemas.go:662-669`），这是可能的形态来源，不是已证实的现场原因。
- `CapabilityRuntime.Prepare` 先运行 schema 校验（`apps/core-go/internal/core/capability_core.go:1368-1387`）。`tool_execution.go:210-216` 把普通参数错误默认标为 retryable，`adk_conversation_runtime.go:216-223` 因而终止 ADK ToolNode，而不让模型纠正。
- 临时 Go overlay 回归测试以对象 `observations` 调用真实 `CapabilityInvocation.Validate`，稳定得到同一 `field "observations": value must be an array` 错误；仓库产品代码未改。

## Requirements

- R1.1：`commit_review` 的模型输入契约与可接受的有界观察语义一致；允许合理的非数组形态经严格校验后规范化为单一持久表示，不接受空值、过长或不可解释的嵌套内容。
- R1.2：提示词明确期望的 `observations` 格式；参数错误返回安全、可纠正的 Tool 结果，在既有请求期限内允许同一 Agent 重新提交。一次 run 至多有一次成功提交，不能重复推进视觉身份状态。
- R1.3：保留真实图片审核、`accepted` 与 `missing_sections` 一致性检查、候选资产授权和 durable session 幂等保护。

## Acceptance criteria

- [ ] 对象、单条字符串与字符串数组等有界观察输入均可按明确规则规范化；此前的对象型回归重放不再报 `value must be an array`。
- [ ] 无效/不可解释输入产生非 retryable、可纠正 Tool 结果；测试覆盖先失败后纠正，最终恰好一次持久审查。
- [ ] 接受与缺失项目矛盾仍被拒绝，跨 session/资产不合规及重复提交仍有原来的授权/幂等保障。

## Out of scope

- 不修改 `{{seed}}` 替换、ComfyUI 工作流或媒体 intent 重试策略；它们与本次现场错误无关。
