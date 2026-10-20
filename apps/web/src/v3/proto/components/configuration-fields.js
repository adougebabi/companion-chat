const f=(key,label,type='text',extra={})=>({key,label,type,...extra});
export function conversationReplyDisplayValue(value){
 const next=value&&typeof value==='object'&&!Array.isArray(value)?{...value}:{};
 if(next.mode==null||String(next.mode).trim()==='')next.mode='fact_only';
 return next;
}
export const configurationFields={
 'conversation.reply':[
  f('mode','回复模式','select',{options:[{value:'legacy',label:'兼容模式'},{value:'fact_only',label:'Fact-only · 独立回复模型'}]}),f('expression_version','表达协议','select',{options:['fact-only-reply.v5','fact-only-reply.v4']}),f('model_ref','模型绑定角色','select',{options:['conversation_reply']}),
  f('temperature','温度','number',{min:0,max:2,step:'any'}),f('top_p','Top P','number',{min:0.01,max:1,step:'any'}),f('max_tokens','回复输出上限','number',{min:1,max:32768}),f('max_input_tokens','回复输入预算','number',{min:1024,max:131072}),f('timeout_seconds','总超时（秒，含排队）','number',{min:1,max:3600}),f('concurrency','回复并发','number',{min:1,max:8}),f('retry_limit','可恢复错误重试次数','number',{min:0,max:2}),f('max_action_handoffs','媒体动作回交上限','number',{min:1,max:2}),f('failure_policy','回复失败策略','select',{options:[{value:'error',label:'明确返回失败'},{value:'legacy_fallback',label:'显式由认知模型独立表达'}]}),
 ],
 'vision.understanding':[f('enabled','启用视觉理解','boolean'),f('model_ref','独立视觉模型绑定','select',{options:['vision_understanding']})],
 kev:[f('enabled','启用 KEV','boolean'),f('media_enabled','启用媒体方案补充','boolean'),f('endpoint','服务地址','url'),f('points','启用的决策点','boolean-map'),f('timeout_ms','请求超时（毫秒）','number',{min:50,max:60000}),f('budget_ms','阶段总预算（毫秒）','number',{min:50,max:120000}),f('concurrency','并发','number',{min:1,max:16}),f('batch_size','批次大小','number',{min:1,max:8}),f('response_bytes','响应字节上限','number',{min:1024,max:4194304}),f('request_bytes','请求字节上限','number',{min:1024,max:1048576}),f('strategy','选择策略','select',{options:['choice_argmax','choice_guarded']}),f('min_probability','最低概率','number',{min:0,max:1,step:'any'}),f('min_margin','最低候选差距','number',{min:0,max:1,step:'any'}),f('max_deferrals','最大连续延期次数','number',{min:1,max:100}),f('deferral_seconds','每次延期（秒）','number',{min:1,max:86400}),f('retention_days','决策记录保留（天）','number',{min:1,max:90}),f('model','模型标识'),f('model_revision','模型版本')],
 'media.comfyui':[f('baseUrl','ComfyUI 地址','url'),f('generation_timeout_seconds','单次媒体处理上限（秒；受工作流总期限约束）','number',{min:30,max:1200}),f('workflow','默认 API 工作流','workflow'),f('visual_identity_workflow','视觉身份工作流覆盖（空白即跟随默认）','workflow',{optional:true}),f('visual_identity_workflows','视觉身份分阶段覆盖（JSON；空白清除）','json',{optional:true}),f('media_workflows','人物／风景／物品等分支覆盖（JSON；空白清除）','json',{optional:true})],
 'product.autonomy':[f('mode','全局自治模式','select',{options:[{value:'active',label:'启用'},{value:'paused',label:'暂停自治'},{value:'muted',label:'静默模式'}]}),f('allowed_actions','允许的行动类型','json'),f('confirm_action_types','需要明确用户指令的行动类型','json'),f('budget_remaining','授予行动预算（不是模型 Token 或虚拟货币）','number',{min:0}),f('concurrency_limit','行动并发上限','number',{min:1}),f('cooldown_seconds','新行动冷却（秒）','number',{min:0}),f('outreach_paused','暂停主动联系','boolean'),f('notifications_muted','静音通知，保留通知记录','boolean'),f('quiet_hours','安静时段（JSON，含 IANA 时区）','json')],
 'product.wakeup':[f('enabled','启用定时唤醒','boolean'),f('interval_seconds','唤醒间隔（秒）','number',{min:1})],
 'llm.queue':[f('generated_concurrency','生成模型并发','number',{min:1,max:8}),f('embedding_concurrency','Embedding 并发','number',{min:1,max:8})],
 'product.summary':[f('interval_seconds','摘要维护间隔（秒）','number',{min:300,max:600}),f('max_runes','摘要字符预算','number',{min:512,max:4096})],
 'product.goal_retry':[f('max_attempts','意向重试上限','number',{min:1,max:20}),f('base_seconds','首次重试等待（秒）','number',{min:30,max:3600}),f('max_seconds','重试间隔上限（秒）','number',{min:30,max:86400})],
 'product.notifications':[f('messages','新消息通知','boolean'),f('goals','目标与请求结果提醒','boolean'),f('media','媒体结果通知','boolean')],
};
export const configurationTitles={'conversation.reply':'私聊回复与 Fact-only','vision.understanding':'视觉理解',kev:'KEV 选择与连接','media.comfyui':'媒体工作流','product.autonomy':'全局运行与自治','product.wakeup':'定时唤醒','llm.queue':'模型队列','product.summary':'上下文摘要','product.goal_retry':'意向失败重试','product.notifications':'通知分类偏好'};
