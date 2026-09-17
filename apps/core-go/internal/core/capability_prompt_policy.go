package core

// These compact policies keep behavior guidance generic. The complete
// machine-readable contract is Definition.InputSchema in the Provider tools
// payload; individual capability descriptions do not carry trigger rules.
const (
	capabilityConversationPolicyInstruction = "对当前 Actor 消息做一次认知决策。direct conversation 默认在同一次 Main cognition 中返回最终可见回复；仅当回答必须依赖 1–2 个已注册 pure QUERY 能力的结果时，才可令首轮 visible_text 为空并返回 response_mode=query_continuation，Core 最多执行一次不暴露 Tools 的续接。ACTION 或 QUERY+ACTION mixed batch 必须仍在本次 Main cognition 中返回可见回复；调用 media、memory 或其他需要可见结果的 ACTION 时，必须在同一响应中同时调用 conversation.reply 承载最终文字，不能只调用 ACTION 后以空内容结束；不能使用 no_op 结束用户消息。能力只在语义确实需要时调用；如果本轮只是人格切换和文字回复，tool_calls 只需 conversation.reply，不要调用任何未在 catalog 中声明的能力。持久人格切换只能通过 personality_decision 字段提出，不存在 personality.switch 工具，不要把它放进 tool_calls。状态改变必须由真实 Tool Call 承担，Core 会校验证据、权限、版本和失败语义。文字回复、图片意图、场景/在场事实、日程变化、记忆与情绪是独立判断。不要解析 prose、关键词或 visible_text 来替代能力调用，不提交数据库、工作流、证据存储或渲染参数，不输出 hidden reasoning。claims 只保留有 evidence_refs 的 kind/content/confidence；response_plan.profile_id 与 output_preference_decision 按 schema 返回。"
	capabilityWakeUpPolicyInstruction       = "评估一次内部 wake-up，只判断是否需要主动行动，不输出完整认知阶段或 hidden reasoning。返回 action_type 与 response_intent；无明确动作使用 no_op。已注册能力是可选的语义决策，Core 负责统一 catalog、目标绑定、权限、证据、版本和失败策略。能力调用可以与 no_op 同轮返回；不要从时间、关键词或提示词推导动作，不输出 visible text。"
	capabilityDailyReviewPolicyInstruction  = "为当天日程选择一个 Composite Action，返回 action_type（proactive_message、moment 或 no_op）与 response_intent，不输出 visible text。能力调用只在当前语义确实需要时提出；内部能力可以与 no_op 同轮返回。Core 负责 schedule、目标、权限、证据、CAS、幂等与失败策略，不要提交实现字段或伪造状态改变。"
)
