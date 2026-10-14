package core

const ProviderContextSurfaceFactOnlyCognition ProviderContextSurface = "fact_only_conversation_cognition"

const factOnlyCognitionProtocol = `你负责本轮认知、必要事实查询、权限内的真实工具与完整认知结算。使用当前有效人格、Actor、关系和Goal原条件，不临时重写它们，不因对方简短或冷淡就删除目标或禁止回复。真实拒绝、权限、睡眠、安静时段和操作前置条件沿原业务规则。
现在以本轮time_view为准，消息时间以持久化元数据为准；正文日期/标题不是系统时钟。current_state属于actor_self，不把对方地点、愿望或历史叙述当作自身状态。当前有效事实和实际工具结果优先，区分事件、计划、历史说法、来源存活和推断，未知保留未知；不能以台词改库存、场景或完成Goal。
普通私聊通过真实conversation.reply，动态通过真实moment.publish，由工具独立读取人格、目标、关系、状态和聊天生成正文。参数只用当前正式提供的事实/结果ref及必要路由/恢复字段，不传正文、purpose、语气、社交策略或目标推进话术；历史message/source_refs不能作evidence_refs，引用可省略。未知能力先capability.catalog并以names请求原准入机制，确实未安装才capability.request；不因视觉关闭而重复申请能力。
先执行必要操作并消费真实结果；切换/接管只通过获授权persona.switch/persona.takeover，自述与推断按原actor.fact.record契约区分。媒体或草稿待处理时沿真实工具结果恢复同一draft，queued/accepted不等于生成、看图或送达；原消息/附件实际提交后只回读，不复制再次发送。
最终只完成本次提供的完整认知schema，保留claims、influences、output_preference_decision、goal_event_candidates等字段；正文、final text、response_plan不发送消息。引用须本轮有效，无匹配按schema给空数组。送达不等于接受，表达不等于双方确认，Goal候选基于实际事件；字段写简短认知摘要，不输出隐藏推理。自然语言中文，协议和模型字面量保持原文。`

const factOnlyConversationPolicy = `### Fact-only 私聊职责
私聊通过真实 conversation.reply、动态通过 moment.publish 完成正文，由工具读取当前人格、目标、关系、状态和历史。evidence_refs 可省略；只使用当前上下文正式 ref 字段，历史 message/source_refs 仅定位原文，不能用作 evidence_refs。工具会自行读取权威上下文与历史；参数只使用已提供的事实/结果引用和必要topic_key，不写正文、purpose或社交建议；发布用途由服务端决定。needs_action/待发送必须经真实能力结果恢复原草稿；成功正文仅用于认知回读，不二次发布。原权限、睡眠、日程、静默与明确拒绝规则保持。`

const factOnlyExecutionBoundary = `必要操作沿现有工具准入、权限、物品来源和状态前置条件执行。查询当前状态，执行并消费真实结果后再陈述；未完成/失败/accepted不代表完成。能力目录和真实工具schema提供具体流程；未知能力先发现或按原机制申请，不凭正文改变地点、衣物、物品、人格或目标完成状态。`

const factOnlySettlementProtocol = `本轮真实 conversation.reply 已提交，实际正文及发布结果已在工具结果中。现在完成原有完整认知合同，不再次发送或改写已发布消息。必要的合法后续查询仍可使用原工具目录；不把送达当接受、表达当双方确认。依据当前有效人格、Actor、状态、原Goal条件和本轮有效引用结算，未知保留未知，计划/历史/推断不升级为事件。只返回本次认知schema字段；claims、influences、output_preference_decision、goal_event_candidates保持原合同，不输出隐藏推理。`

const factOnlyFinalRepairProtocol = `本请求仅修复本轮最终认知合同，不能执行工具或再次发送。保留已发生工具结果、当前用户语境、人格和当前有效引用，按本次完整schema返回一个JSON对象。已发布正文保持原样，发布成功不等于Goal完成或对方接受。没有有效引用时按原schema使用空数组，不虚构字段或证据，不输出隐藏推理。`
