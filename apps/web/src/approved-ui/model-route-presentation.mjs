import { callPurpose } from './task-model-call-presentation.mjs';

const agentNames = {
  conversation_cognition: '对话理解',
  conversation_reply: '聊天回复',
  expression_learning: '表达学习',
};
const variantNames = {
  conversation_turn_response: '回合回应',
  private_reply: '私聊回复',
  proactive: '主动私聊',
  moment: '动态',
  conversation_summary_v1: '对话摘要',
  diary_material_v1: '日记选材',
  diary_expression_v1: '日记撰写',
  media_prompt_normalized: '媒体提示词整理',
  reflection_proposal_v2: '记忆与自我反思',
  direct_media_delivery_check_v1: '媒体发送核验',
  expression_learning_v1: '表达学习',
  wake_up_response: '主动唤醒',
  takeover_judge_response: '人格接管判断',
  takeover_reply_response: '人格接管回复',
  initialization_response: '人物初始化',
  persona_compilation_response: '人格资料整理',
  media_prompt_text: '媒体提示词撰写',
  media_subject_parse: '媒体主体解析',
  visual_identity_patch_response: '视觉身份修正',
  conversation_segment_v1: '对话片段归纳',
  conversation_daily_memory_v1: '每日记忆归纳',
  native_cognition_response: '日常认知处理',
  owner_life_request_decision: '生活请求判断',
  npc_opportunity_decision: '互动时机判断',
  studio_assistant_v1: '工作室创作助手',
  studio_preview_text: '工作室文本预览',
  npc_response_v1: '人物独立回应',
  npc_moment_response_v1: '人物动态回应',
  daily_review_response: '每日回顾',
  diary_expression_text: '日记撰写',
  persistent_switch_assessment: '人格切换判断',
  goal_planner_v1: '目标规划',
  goal_evaluation_v1: '目标评估',
  world_event_generation_v1: '世界随机事件生成',
};

const lookup = (names, code) => Object.hasOwn(names, code) ? names[code] : undefined;

function knownPurpose(code) {
  const purpose = callPurpose({ scenario: code });
  return purpose.known ? purpose.label : undefined;
}

export function modelRouteLabel(route) {
  const agent = lookup(agentNames, route.agent_id) || lookup(variantNames, route.agent_id) || knownPurpose(route.agent_id) || '其他任务';
  if (!route.variant) return `${agent} · 全部任务`;
  const variant = lookup(variantNames, route.variant) || knownPurpose(route.variant);
  if (!variant) return `${agent} · 其他变体`;
  return route.agent_id === 'conversation_cognition' ? `${agent} · ${variant}` : variant;
}

const businessGroups = [
  ['对话与交流', ['conversation_cognition', 'conversation_reply', 'conversation_summary', 'npc_response', 'reply_action_check', 'reply_action_consistency', 'expression_learning']],
  ['记忆与日记', ['diary_material', 'diary_expression', 'reflection', 'daily_review', 'memory_retrieval', 'embedding']],
  ['人物与生活', ['initialization', 'persona_compilation', 'wardrobe_preparation', 'schedule_generation', 'schedule_replan', 'native_cognition', 'wake_up', 'goal_planner', 'goal_evaluation', 'virtual_activity_result', 'world_event_generation', 'persistent_switch', 'takeover_judge', 'takeover_reply']],
  ['媒体与创作', ['media_prompt', 'media_quality', 'vision_understanding', 'visual_identity', 'visual_identity_vision', 'visual_identity_patch', 'studio_assistant', 'studio_text_preview']],
];

export function modelRouteGroups(routes) {
  const groups = new Map([...businessGroups.map(([label]) => [label, []]), ['其他任务', []]]);
  for (const route of routes) {
    if (!route.writable || !route.key) continue;
    const group = businessGroups.find(([, agents]) => agents.includes(({diary_material_v1:'diary_material',media_prompt_normalized:'media_prompt'})[route.agent_id] || route.agent_id))?.[0] || '其他任务';
    groups.get(group).push({ key: route.key, label: modelRouteLabel(route) });
  }
  return [...groups].filter(([, options]) => options.length).map(([label, options]) => ({ label, options }));
}
