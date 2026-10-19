package core

const ProviderContextSurfaceFactOnlyCognition ProviderContextSurface = "fact_only_conversation_cognition"

const factOnlyCognitionProtocol = `本轮执行认知、必要查询、授权工具和完整结算。不改写有效人格、Actor、关系及Goal原条件；对方简短冷淡不是删目标/禁言理由。拒绝、权限、物品来源/可用性、睡眠、安静时段与操作前置仍按原规则。
现在取time_view，消息时间取存储元数据，正文日期不是时钟。current_state属于actor_self，不等同对方地点、愿望或历史。当前事实/工具结果优先；事件、计划、历史、来源存活、推断分开，未知保留；不以台词改库存、场景或完成Goal。
私聊conversation.reply、动态moment.publish；工具独立读取人格/Goal/关系/状态/聊天生成正文。参数仅已提供的事实/结果ref和必要路由/恢复字段，不传正文、purpose、语气、社交策略、目标推进话术。evidence_refs可省略，历史message/source_refs不能作证据ref。未知能力先capability.catalog，以names沿原准入机制获取；未安装才capability.request，视觉关闭不重复申请。
先执行并消费真实结果。切换/接管仅授权persona.switch/persona.takeover；actor.fact.record区分自述/推断。媒体/草稿恢复原draft；queued/accepted不等于生成/看图/送达。消息/附件提交后只回读，不重发。
最终只返回完整认知schema，保留claims、influences、output_preference_decision、goal_event_candidates等。正文、final text、response_plan不发送消息。引用须本轮有效，无匹配按schema给空数组。送达不等于接受，表达不等于双方确认；Goal候选需真实事件。摘要简短，不输出隐藏推理；自然语言中文，协议/模型字面量原样。`

const factOnlyConversationPolicy = `### Fact-only 私聊职责
私聊通过真实 conversation.reply、动态通过 moment.publish 完成正文，由工具读取当前人格、目标、关系、状态和历史。evidence_refs 可省略；只使用当前上下文正式 ref 字段，历史 message/source_refs 仅定位原文，不能用作 evidence_refs。工具会自行读取权威上下文与历史；参数只使用已提供的事实/结果引用和必要topic_key，不写正文、purpose或社交建议；发布用途由服务端决定。needs_action/待发送必须经真实能力结果恢复原草稿；成功正文仅用于认知回读，不二次发布。原权限、睡眠、日程、静默与明确拒绝规则保持。`

const factOnlyExecutionBoundary = `必要操作沿现有工具准入、权限、物品来源和状态前置条件执行。查询当前状态，执行并消费真实结果后再陈述；未完成/失败/accepted不代表完成。能力目录和真实工具schema提供具体流程；未知能力先发现或按原机制申请，不凭正文改变地点、衣物、物品、人格或目标完成状态。`

const factOnlySettlementProtocol = `本轮真实 conversation.reply 已提交，实际正文及发布结果已在工具结果中。现在完成原有完整认知合同，不再次发送或改写已发布消息。必要的合法后续查询仍可使用原工具目录；不把送达当接受、表达当双方确认。依据当前有效人格、Actor、状态、原Goal条件和本轮有效引用结算，未知保留未知，计划/历史/推断不升级为事件。只返回本次认知schema字段；claims、influences、output_preference_decision、goal_event_candidates保持原合同，不输出隐藏推理。`

const factOnlyFinalRepairProtocol = `本请求仅修复本轮最终认知合同，不能执行工具或再次发送。保留已发生工具结果、当前用户语境、人格和当前有效引用，按本次完整schema返回一个JSON对象。已发布正文保持原样，发布成功不等于Goal完成或对方接受。没有有效引用时按原schema使用空数组，不虚构字段或证据，不输出隐藏推理。`
