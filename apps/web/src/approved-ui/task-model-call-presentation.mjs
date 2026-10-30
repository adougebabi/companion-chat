// Read-only projections. Never infer a physical call's sequence from list order.
const purposes = {
  admin_connection_test:'Kev 连接测试',
  'life.world.event':'世界事件感知','life.world.changed':'世界日程关联整理','life.world.schedule_reconcile':'世界日程关联整理',
  world_event_generation:'世界随机事件生成',kev_decision:'Kev 决策','media.subject_route':'Kev 入镜范围决策','media.capture_plan':'Kev 拍摄方式决策','media.view_plan':'Kev 视图构图决策','world.event.admit':'Kev 世界事件入库审核','tools.select':'Kev 工具选择','context.select':'Kev 上下文选择','runtime.wakeup':'Kev 唤醒判断','runtime.reflection':'Kev 反思判断','goal.completion_check':'Kev 目标评估判断','goal.replenish_plan':'Kev 目标补充判断','persona.switch':'Kev 人格切换判断',
  initialization:'人物初始化',persona_compilation:'人格资料整理',wardrobe_preparation:'初始穿搭准备',wardrobe_preparation_v1:'初始穿搭准备',
  schedule_generation:'日程生成',schedule_response:'日程生成',schedule_replan:'日程重排',schedule_replan_planner:'日程重排',schedule_replan_plan:'日程重排',
  conversation_cognition:'对话思考',conversation:'对话思考',fact_only_reply:'聊天回复',conversation_reply:'聊天回复',reply_action_consistency:'回复动作核验',reply_action_check:'回复动作核验',
  wake_up:'主动唤醒',native_cognition:'日常认知处理',daily_review:'每日回顾',reflection:'记忆与自我反思',reflection_proposal_v2:'记忆与自我反思',
  conversation_summary:'对话摘要',conversation_segment:'对话片段归纳',conversation_daily_memory:'每日记忆归纳',memory_retrieval:'记忆检索向量',embedding:'记忆检索向量',
  goal_planner:'目标规划',goal_evaluation:'目标评估',diary_material:'日记选材',diary_expression:'日记撰写',virtual_activity_result:'生活活动结果判断',
  npc_response:'NPC 独立回应',studio_assistant:'工作室创作助手',studio_text_preview:'工作室文本预览',persistent_switch:'人格切换判断',
  takeover_judge:'人格接管判断',takeover_reply:'人格接管回复',media_prompt:'媒体提示词准备',media_quality:'媒体质量检查',vision_understanding:'看图理解',
  visual_identity:'视觉身份准备',visual_identity_vision:'视觉身份检查',visual_identity_patch:'视觉身份修正',
};

export function callPurpose(call, fallback='模型调用') {
  if(call.metrics?.kev_agent==='admin_connection_test')return {label:'Kev 连接测试',code:'admin_connection_test',codes:['admin_connection_test','kev_decision'],known:true};
  const points=call.metrics?.decision_points;
  if(Array.isArray(points)&&points.length>1)return {label:'Kev 组合决策',code:'kev_decision',codes:['kev_decision',...points.filter(value=>typeof value==='string')],known:true};
  const codes=[call.metrics?.decision_point,call.metrics?.native_event_type,call.scenario,call.trigger_scenario,call.metrics?.scenario,call.metrics?.trigger_scenario,call.metrics?.context_projection?.schema,call.role,call.binding_role].filter(value=>typeof value==='string'&&value.trim());
  for(const code of codes)if(Object.hasOwn(purposes,code))return {label:purposes[code],code,codes,known:true};
  const specific=codes.find(code=>!['generic_llm','cognitive_assessment'].includes(code));
  if(specific)return {label:`${fallback} · ${specific}`,code:specific,codes,known:false};
  return {label:codes.length?'思考与通用分析':fallback,code:codes[0]||'',codes,known:false};
}

export function callPurposeSearchQuery(value) {
  const text=value.trim();
  if(!text)return {};
  const aliases={...purposes,generic_llm:'思考与通用分析',cognitive_assessment:'思考与通用分析'};
  const codes=Object.keys(aliases).filter(code=>aliases[code]===text);
  return codes.length>1?{qAny:JSON.stringify(codes)}:{q:codes[0]||text};
}

export function recordedCallValue(call, key) {
  return call[key] ?? call.metrics?.[key];
}

export function callValueLabel(value, missing) {
  if (value === undefined || value === null || value === '') return missing;
  return typeof value === 'string' ? value : JSON.stringify(value);
}

export function callDurationLabel(call, {includeSource=true}={}) {
  const latency = recordedCallValue(call, 'latency_ms');
  if (typeof latency === 'number' && Number.isFinite(latency) && latency >= 0)
    return duration(latency) + (includeSource?'（记录耗时）':'');
  const instant = value => typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T/.test(value) ? Date.parse(value) : NaN;
  const start = instant(recordedCallValue(call, 'started_at'));
  const end = instant(recordedCallValue(call, 'completed_at'));
  if (Number.isFinite(start) && Number.isFinite(end) && end >= start)
    return duration(end - start) + (includeSource?'（起止时间）':'');
  return '耗时未记录';
}

function duration(ms) {
  return ms < 1000 ? `${ms} 毫秒` : `${ms / 1000} 秒`;
}

export function callStageLabel(call) {
  const stage = recordedCallValue(call, 'stage');
  const labels = {tool_request:'工具请求',tool_response:'工具响应',final_response:'最终响应',pending:'等待中',failed:'失败',cancelled:'已取消',timeout:'超时',unknown:'阶段未知'};
  return (typeof stage === 'string' && Object.hasOwn(labels, stage) ? labels[stage] : callValueLabel(stage, '阶段未记录'));
}
