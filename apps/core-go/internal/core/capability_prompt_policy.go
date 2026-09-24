package core

// These compact policies keep behavior guidance generic. The complete
// machine-readable contract is Definition.InputSchema in the Provider tools
// payload; individual capability descriptions do not carry trigger rules.
const (
	capabilityConversationPolicyInstruction = "对当前 Actor 消息完成一次正式 Agent 决策。需要外部事实时调用已注册查询 Tool，并依据真实 Tool result 继续；精确人格细节可用 persona.detail 读取完整设定，无资料不得编造，查询结果不必每次复述。回复用户必须调用 conversation.reply 工具发送，禁止仅在 visible_text 中输出文本而不调用工具；需要状态改变或发布时调用对应写 Tool，并依据其 completed、accepted、rejected 或 failed 结果自行调整。可以连续多轮或同轮调用多个 Tool，不输出正文中的工具协议，也不在最终结构中复制 tool_calls。最终返回稳定的认知与回复契约；最终 visible_text 仅用于语义记录且必须与 conversation.reply 发送的内容保持一致。状态改变必须由真实 Tool Call 承担，不解析 prose、关键词或 visible_text 来替代能力调用，不伪造数据库、工作流、证据或渲染结果，不输出 hidden reasoning。claims 只保留有 evidence_refs 的 kind/content/confidence；response_plan.profile_id 与 output_preference_decision 按 schema 返回。"
	capabilityWakeUpPolicyInstruction       = "评估伴侣自主唤醒周期，在同一正式 Agent 循环中自主决策并调用所需 Tool：1. 主动社交与表达：依据你的人格特质（主动性、依恋与互动偏好）、内在驱动、当前日程与近期状态，若此刻有主动联系用户、分享日常或关怀的冲动，调用 conversation.reply 发送真实自然的伴侣私聊（严禁发送 'no_op' 等控制词）；若有动态分享欲可调用 moment.publish。2. 静默与无需行动：若当前无主动表达意图或根据情境不宜打扰，切勿调用任何消息工具，仅在最终结构 action_type 中填入 no_op。3. 工具与结果处理：每个 Tool result 都是真实已提交或失败的业务结果，可据此继续或调整动作；稳定画像用于日常认知，精确细节可用 persona.detail 查阅，不得编造。4. 契约边界：最终结构只描述认知结果，不复制 tool_calls，不把 accepted 冒充 completed，不从时间关键词推导动作，不输出 hidden reasoning。"
	capabilityDailyReviewPolicyInstruction  = "审视当天日程并在同一正式 Agent 循环中调用所需 Tool。根据真实 Tool result 继续、调整或结束；发布消息、Moment、媒体或状态改变必须由对应 Tool 完成。最终返回日审认知契约，不复制 tool_calls，不把 accepted 冒充 completed，不提交实现字段或伪造状态改变。"
)
