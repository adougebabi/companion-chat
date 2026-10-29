import {autonomyModes} from './autonomy-modes.js';
const f=(key,label,type='text',extra={})=>({key,label,type,...extra});
export function conversationReplyDisplayValue(value){
 const next=value&&typeof value==='object'&&!Array.isArray(value)?{...value}:{};
 if(next.mode==null||String(next.mode).trim()==='')next.mode='fact_only';
 return next;
}
export const configurationFields={
 'conversation.reply':[
  f('mode','回复模式','select',{options:[{value:'legacy',label:'兼容模式'},{value:'fact_only',label:'独立回复模型'}]}),f('expression_version','表达版本','select',{options:[{value:'fact-only-reply.v5',label:'自然交流 · 当前协议'},{value:'fact-only-reply.v4',label:'自然交流 · 兼容协议'}]}),f('model_ref','模型绑定角色','select',{options:[{value:'conversation_reply',label:'交流表达'}]}),
  f('temperature','表达温度','number',{min:0,max:2,step:'any'}),f('top_p','Top P','number',{min:0.01,max:1,step:'any'}),f('timeout_seconds','总超时（秒，含排队）','number',{min:1,max:3600}),f('concurrency','同时回复数量','number',{min:1,max:8}),f('retry_limit','可恢复错误重试次数','number',{min:0,max:2}),f('max_action_handoffs','媒体动作回交上限','number',{min:1,max:2}),f('failure_policy','回复失败策略','select',{options:[{value:'error',label:'明确返回失败'},{value:'legacy_fallback',label:'显式由认知模型独立表达'}]}),
 ],
 'vision.understanding':[f('enabled','启用视觉理解','boolean'),f('model_ref','独立视觉模型绑定','select',{options:[{value:'vision_understanding',label:'独立视觉理解'}]})],
 kev:[f('enabled','启用辅助决策服务','boolean'),f('media_enabled','启用媒体方案补充','boolean'),f('endpoint','服务地址','url'),f('points','启用的决策点','boolean-map'),f('timeout_ms','请求超时（毫秒）','number',{min:50,max:60000}),f('budget_ms','阶段总预算（毫秒）','number',{min:50,max:120000}),f('concurrency','并发','number',{min:1,max:16}),f('batch_size','批次大小','number',{min:1,max:8}),f('response_bytes','响应字节上限','number',{min:1024,max:4194304}),f('request_bytes','请求字节上限','number',{min:1024,max:1048576}),f('strategy','选择策略','select',{options:[{value:'choice_argmax',label:'按最高分选择'},{value:'choice_guarded',label:'按置信条件选择'}]}),f('min_probability','最低置信概率','number',{min:0,max:1,step:'any'}),f('min_margin','领先候选最小差值','number',{min:0,max:1,step:'any'}),f('max_deferrals','最大连续延期次数','number',{min:1,max:100}),f('deferral_seconds','每次延期（秒）','number',{min:1,max:86400}),f('retention_days','决策记录保留（天）','number',{min:1,max:90}),f('model','模型标识'),f('model_revision','模型版本')],
 'media.comfyui':[f('baseUrl','ComfyUI 地址','url'),f('generation_timeout_seconds','单次媒体处理上限（秒；受工作流总期限约束）','number',{min:30,max:1200}),f('workflow','默认 API 工作流','workflow'),f('visual_identity_workflow','视觉身份工作流覆盖（空白即跟随默认）','workflow',{optional:true}),f('visual_identity_workflows','视觉身份分阶段覆盖（JSON；空白清除）','json',{optional:true}),f('media_workflows','人物／风景／物品等分支覆盖（JSON；空白清除）','json',{optional:true})],
 'product.autonomy':[f('mode','全局自治模式','select',{options:autonomyModes}),f('allowed_actions','允许的行动类型','action-types'),f('confirm_action_types','需要明确用户指令的行动类型','action-types'),f('budget_remaining','授予行动预算（不是模型 Token 或虚拟货币）','number',{min:0}),f('concurrency_limit','行动并发上限','number',{min:1}),f('cooldown_seconds','新行动冷却（秒）','number',{min:0}),f('outreach_paused','暂停主动联系','boolean'),f('notifications_muted','静音通知，保留通知记录','boolean'),f('quiet_hours','安静时段','quiet-hours')],
 'product.wakeup':[f('enabled','启用定时唤醒','boolean'),f('interval_seconds','唤醒间隔（秒）','number',{min:1})],
 'llm.queue':[f('generated_concurrency','生成模型并发','number',{min:1,max:8}),f('embedding_concurrency','记忆检索并发','number',{min:1,max:8})],
 'product.summary':[f('interval_seconds','摘要间隔（秒）','number',{min:300,max:600,hint:'300–600 秒'}),f('max_runes','摘要最大字符数','number',{min:512,max:4096,hint:'512–4096 字符'})],
 'product.goal_retry':[f('max_attempts','意向重试上限','number',{min:1,max:20}),f('base_seconds','首次重试等待（秒）','number',{min:30,max:3600}),f('max_seconds','重试间隔上限（秒）','number',{min:30,max:86400})],
 'product.notifications':[f('messages','新消息通知','boolean'),f('goals','目标与请求结果提醒','boolean'),f('media','媒体结果通知','boolean')],
};
export const configurationTitles={'conversation.reply':'私聊回复与 Fact-only','vision.understanding':'视觉理解',kev:'KEV 选择与连接','media.comfyui':'媒体工作流','product.autonomy':'全局运行与自治','product.wakeup':'定时唤醒','llm.queue':'模型队列','product.summary':'上下文摘要','product.goal_retry':'意向失败重试','product.notifications':'通知分类偏好'};

export const pointLabels={'context.select':'选择上下文','goal.completion_check':'检查目标完成条件','goal.replenish_plan':'补充目标规划','media.capture_plan':'准备图片拍摄方案','media.subject_route':'选择图片主题','media.view_plan':'规划画面视角','persona.switch':'判断人格切换','runtime.reflection':'判断反思时机','runtime.wakeup':'判断日常唤醒','tools.select':'选择可用工具'};
export const configurationPoints=Object.keys(pointLabels);
export function configurationChoiceLabel(key){return Object.hasOwn(pointLabels,key)?pointLabels[key]:key;}

export const autonomyActionChoices=[['proactive_message','主动联系'],['moment','发布动态'],['capability','外部行动与生成']];
