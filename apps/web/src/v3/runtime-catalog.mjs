export function normalizeRuntimeCatalog(value){
 return {...value,agents:Array.isArray(value.agents)?value.agents:[],capabilities:(Array.isArray(value.capabilities)?value.capabilities:[]).map(c=>({...c,name:c.name??c.Name,version:c.version??c.Version,type:c.type??c.Type,description:c.description??c.Description,sourceContract:c}))};
}

const agentLabels={
 'conversation-cognition':'对话认知','conversation-reply':'交流表达','conversation-summary':'对话总结','daily-review':'日常回顾','diary-expression':'日记表达','diary-material':'日记素材整理','goal-evaluation':'目标评估','goal-planner':'目标规划',initialization:'人物初始化','media-prompt':'图像提示整理','media-quality':'图像质量检查','native-cognition':'原生认知','npc-response':'NPC 回应','persistent-switch':'持久人格切换','persona-compilation':'工作人格准备',reflection:'反思成长','schedule-generation':'日程生成','schedule-replan':'日程调整','studio-assistant':'创作助手','studio-text-preview':'创作文本预览','takeover-judge':'人格接管判断','takeover-reply':'接管后的表达','virtual-activity-result':'虚拟活动结果','visual-identity':'视觉身份准备','visual-identity-patch':'视觉身份修正','visual-identity-vision':'视觉一致性检验','wake-up':'日常唤醒',
};
const toolLabels={
 active_memory_event:'记录临时事实','actor.fact.record':'记录与修正人物事实','actor.inspect':'查询人物事实',affect_event:'记录情绪事件','appearance.style':'调整临时发型','capability.catalog':'查询可用能力','capability.discover':'读取能力参数','capability.request':'提出新能力请求','conversation.reply':'发送私聊消息','goal.decide':'创建与管理目标','goal.evaluate':'请求目标评估','goal.evaluation.submit':'提交目标评估结论','goal.inspect':'查询目标与证据','goal.object.submit':'提交阶段或承诺','goal.plan.submit':'提交当前行动计划','goal.review':'请求目标回顾','goal_planner.commit':'提交独立规划结果','goal_planner.query':'读取独立规划依据','habit.decide':'修订日常习惯','habit.inspect':'查询日常习惯','intention.decide':'创建与管理意向','intention.inspect':'查询意向','intention.schedule':'安排意向行动','item.use':'使用或停止使用物品','life.activity.advance':'推进当前活动','life.activity.start':'开始生活活动','media.image.generate':'提出图片生成','memory.recall':'检索记忆来源',memory_event:'记录与修订记忆','moment.publish':'发布生活动态','persona.detail':'查询人格与有效状态','persona.switch':'切换已声明人格','persona.takeover':'按规则接管人格',presence_event:'记录临时交流状态','relationship.lookup':'查询人物关系',scene_event:'记录场景变化','schedule.edit':'编辑日程','schedule.inspect':'查询日程','schedule.replan':'重新规划日程','visual_identity.commit_review':'提交视觉身份评审','visual_identity.finalize':'完成视觉身份准备','visual_identity.generate_candidate':'生成视觉候选','visual_identity.initialize':'初始化视觉身份','wardrobe.borrow':'借入物品','wardrobe.inspect':'查询衣柜与穿着','wardrobe.outfit.save':'保存穿搭组合','wardrobe.return':'归还物品','wardrobe.wear':'穿着与脱下服饰','world.npc.encounter':'记录 NPC 实际接触',
};
export function catalogLabel(kind,name,description=''){
 const labels=kind==='agent'?agentLabels:toolLabels,key=kind==='agent'?String(name||'').replace(/^fluctlight-/,''):name;
 if(Object.hasOwn(labels,key))return labels[key];
 return typeof description==='string'&&/[\u3400-\u9fff]/u.test(description)?description.split(/[。\n]/)[0].slice(0,80):name||'已注册能力';
}
