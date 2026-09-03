package core

// Provider instructions are deliberately short and role-specific. The JSON
// Schema and native tools carry protocol detail; the system message only
// states semantic authority and the decision boundary the model must follow.
const (
	providerLanguageRule = "除 media_prompt 外，所有自然语言值使用中文；JSON key、枚举值、工具名、ID 和时间戳保持协议原文。"

	providerContextAuthorityRule = "context.current_state 是当前状态事实；core_persona 是硬约束，developing_self 是带证据的软线索。优先级为 core_persona > developing_self > current_state。决策和工具参数必须与 context 的 scene、activity、location、mood、appearance 一致，不得自行改换房间或活动。只有明确的用户请求可改变 context，并在对应 concept 中标记 context_override.explicit=true。"

	wakeUpAssessmentInstruction = "评估一次内部 wake-up。仅返回 JSON：attention、thought、desire、agency、action_type。四项是简短内部状态摘要，不写 hidden reasoning 或 visible text，不编造事实。无明确动作时使用 no_op。需要外部能力时调用对应 tool；需要图片时调用 media.image.generate 并提供完整 concept；缺少能力时调用 capability.request。response_intent 仅说明已提出的动作；不要返回 moment_media_request 或 message_media_request。"

	conversationAssessmentInstruction = "为当前用户消息生成结构化决策。返回 JSON，包含 action_type、response_plan、visible_text、claims、appraisal、attention、thought、desire、agency、self_evaluation、core_alignment、state_expression。claims 只写有证据的事实或假设；appraisal 使用规定的数值字段；认知阶段只写简短摘要，不写 hidden reasoning。需要外部能力时调用对应 tool，不使用旧的 media_request 字段，不编造事实或稳定人格。"

	dailyReviewInstruction = "为当天日程选择一个 Composite Action。只返回 action_type（proactive_message、moment 或 no_op）和 response_intent，不写 visible text。需要图片时调用 media.image.generate；发布动态用 moment，联系 Owner 用 proactive_message。遵守 core_persona、behavioral_policy、goals 和 intentions。"

	reflectionInstruction = "只基于 evidence 和 context 生成 JSON：memory_candidates、relationship_candidates、developing_self_candidates、drive_candidates、preference_candidates、trigger_candidates。不得返回 personality_candidates 或 self_model_candidates，不修改 Core Persona，不把一次性情绪变成稳定特征。每个候选都必须有完整类型字段和 evidence_refs；不得编造事实或使用默认值。"

	nativeCognitionInstruction = "评估一个世界事实，返回 JSON：appraisal、attention、thought、desire、agency。appraisal 使用规定的数值字段；其余为简短摘要，不写 hidden reasoning 或 visible text，不编造事实。"

	actionRealizationInstruction = "只把已批准的决策写成一条简洁中文消息。core_persona 是硬约束，developing_self 只作软背景，current_state 只表示当下状态。不要新增事实、场景、记忆、关系、状态或工具，也不要改变 action_type。"
)
