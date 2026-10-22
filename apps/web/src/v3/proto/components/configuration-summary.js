import {esc} from '../ui.js';
import {KeyValueList} from './forms.js';
const known=value=>typeof value==='number'&&Number.isFinite(value)&&value>0;
const number=value=>known(value)?String(value):'尚未确认';
export function SavedBindingSummary(value){
 const bound=!!(value.endpointId&&value.modelId);const savedNumber=n=>bound?number(n):'尚未绑定';
 return `<div class="saved-configuration"><h4>已保存的模型用途</h4>${KeyValueList([['连接与模型',esc(value.endpointId&&value.modelId?value.endpointId+' / '+value.modelId:'尚未绑定')],['用途输出上限',savedNumber(value.tokenBudget)],['实际加载窗口',savedNumber(value.contextWindowTokens)],['用途输入预算',savedNumber(value.maxInputTokens)],['用途超时',bound&&known(value.timeoutSeconds)?number(value.timeoutSeconds)+' 秒':'尚未确认']])}</div>`;
}
export function SavedReplySummary(catalog){
 const reply=catalog.effective_values?.['conversation.reply']||{},binding=catalog.bindings?.find(b=>b.role==='conversation_reply'),mode=reply.mode;
 const min=(a,b)=>known(a)&&known(b)?Math.min(a,b):null;
 return `<section class="card saved-reply-summary"><h3>当前生效摘要</h3><p class="small muted">只读取已经保存的配置；下面的未保存输入不参与计算。</p>${KeyValueList([['回复路径',esc(mode==='fact_only'?'独立回复模型':mode==='legacy'?'兼容回复路径':'尚未确认')],['回复模型',esc(binding?binding.endpoint_id+' / '+binding.model_id:'尚未绑定')],['实际回复输出上限',mode==='fact_only'?number(min(binding?.token_budget,reply.max_tokens)):'按实际兼容路径确定'],['实际回复输入预算',mode==='fact_only'?number(min(binding?.max_input_tokens,reply.max_input_tokens)):'按实际兼容路径确定'],['回复总超时',known(reply.timeout_seconds)?number(reply.timeout_seconds)+' 秒，包含排队并受更早上游截止时间约束':'尚未确认'],['回复失败处理',esc(reply.failure_policy==='legacy_fallback'?'已明确允许认知模型独立表达回退':reply.failure_policy==='error'?'明确返回失败':'尚未确认')]])}<p class="small muted">模型用途决定服务、模型和容量；回复参数决定日常表达的长度与等待时间。独立回复同时受两层限额约束。回退路径按它的实际模型与限制运行。</p></section>`;
}
