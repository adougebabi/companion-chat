import {configurationTestSummary} from '../../configuration-feedback.mjs';
import {esc} from '../ui.js';
import {KeyValueList} from './forms.js';
const known=value=>typeof value==='number'&&Number.isFinite(value)&&value>0;
const number=value=>known(value)?String(value):'尚未确认';
const summaryGrid=entries=>`<dl class="saved-summary-grid">${entries.map(([label,value])=>`<div><dt>${esc(label)}</dt><dd>${value}</dd></div>`).join('')}</dl>`;
export function SavedBindingSummary(value){
 const bound=!!(value.endpointId&&value.modelId);const savedNumber=n=>bound?number(n):'尚未绑定';
 return `<div class="saved-configuration"><h4>已保存的模型用途</h4>${KeyValueList([['连接与模型',esc(value.endpointId&&value.modelId?value.endpointId+' / '+value.modelId:'尚未绑定')],['用途输出上限',savedNumber(value.tokenBudget)],['实际加载窗口',savedNumber(value.contextWindowTokens)],['用途输入预算',savedNumber(value.maxInputTokens)],['用途超时',bound&&known(value.timeoutSeconds)?number(value.timeoutSeconds)+' 秒':'尚未确认']])}</div>`;
}
export function SavedReplySummary(catalog){
 const reply=catalog.effective_values?.['conversation.reply']||{},binding=catalog.bindings?.find(b=>b.role==='conversation_reply'),mode=reply.mode;
 const min=(a,b)=>known(a)&&known(b)?Math.min(a,b):null;
 const recent=configurationTestSummary(catalog.effective_values?.['ai.capability_status']);
 return `<section class="card saved-reply-summary"><div class="saved-summary-heading"><h3>已保存的生效摘要</h3><span class="small muted">未保存修改不参与计算</span></div>${recent.state==='failed'?'<p class="banner warn" role="status">最近能力调用存在失败记录，请核对连接测试和诊断详情。配置摘要只说明已保存值，不代表运行正常。</p>':''}${summaryGrid([['回复路径',esc(mode==='fact_only'?'独立回复模型':mode==='legacy'?'兼容回复路径':'尚未确认')],['回复模型',esc(binding?binding.endpoint_id+' / '+binding.model_id:'尚未绑定')],['实际输出上限',mode==='fact_only'?number(min(binding?.token_budget,reply.max_tokens)):'按实际兼容路径确定'],['实际输入预算',mode==='fact_only'?number(min(binding?.max_input_tokens,reply.max_input_tokens)):'按实际兼容路径确定'],['回复总超时',known(reply.timeout_seconds)?number(reply.timeout_seconds)+' 秒（含排队）':'尚未确认'],['回复失败处理',esc(reply.failure_policy==='legacy_fallback'?'已明确允许认知模型独立表达回退':reply.failure_policy==='error'?'明确返回失败':'尚未确认')]])}<details class="saved-summary-explanation"><summary>限额与回复路径说明</summary><p class="small muted">总超时包含排队，并受更早上游截止时间约束。模型用途决定服务、模型和容量；回复参数决定日常表达的长度与等待时间。独立回复同时受两层限额约束。回退路径按它的实际模型与限制运行。</p></details></section>`;
}

export function ManagementNavigation(current='settings'){
 return `<nav class="management-navigation" aria-label="管理中心分区">${[['settings','配置中心','#/settings'],['diag','运行诊断','#/diag'],['data','数据与恢复','#/data']].map(([key,label,href])=>`<a class="tab ${key===current?'on':''}" href="${href}" ${key===current?'aria-current="page"':''}>${label}</a>`).join('')}</nav>`;
}
export function SettingsHeading(title){return `<header class="settings-heading"><h1>${esc(title)}</h1></header>`;}
