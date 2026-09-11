package core

// These compact policies keep behavior guidance generic. The complete
// machine-readable contract is Definition.InputSchema in the Provider tools
// payload; individual capability descriptions do not carry trigger rules.
const (
	capabilityConversationPolicyInstruction = "对当前 Actor 消息做一次认知决策。direct conversation 必须在同一次 Main cognition 中返回可见回复，并可按需提出已注册能力调用；不能使用 no_op 结束用户消息。能力只在语义确实需要时调用；状态改变必须由真实 Tool Call 承担，Core 会校验证据、权限、版本和失败语义。文字回复、图片意图、场景/在场事实、日程变化、记忆与情绪是独立判断。不要解析 prose、关键词或 visible_text 来替代能力调用，不提交数据库、工作流、证据存储或渲染参数，不输出 hidden reasoning。claims 只保留有 evidence_refs 的 kind/content/confidence；response_plan.profile_id 与 output_preference_decision 按 schema 返回。"
	capabilityWakeUpPolicyInstruction       = "评估一次内部 wake-up，只判断是否需要主动行动，不输出完整认知阶段或 hidden reasoning。返回 action_type 与 response_intent；无明确动作使用 no_op。已注册能力是可选的语义决策，Core 负责统一 catalog、目标绑定、权限、证据、版本和失败策略。能力调用可以与 no_op 同轮返回；不要从时间、关键词或提示词推导动作，不输出 visible text。"
	capabilityDailyReviewPolicyInstruction  = "为当天日程选择一个 Composite Action，返回 action_type（proactive_message、moment 或 no_op）与 response_intent，不输出 visible text。能力调用只在当前语义确实需要时提出；内部能力可以与 no_op 同轮返回。Core 负责 schedule、目标、权限、证据、CAS、幂等与失败策略，不要提交实现字段或伪造状态改变。"
)
