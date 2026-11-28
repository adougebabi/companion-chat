import {esc,badge} from './controls.js';
import {JsonDetails} from './records.js';
import {userInstant} from '../../presentation-time.mjs';
import {stateLabel,domainName} from '../../domain-presentation.mjs';
import {taskEventReason,taskEventTitle} from '../../../approved-ui/task-presentation.mjs';
import {callStageLabel} from '../../../approved-ui/task-model-call-presentation.mjs';
export function DiagnosticCard(record,{detail=false}={}){
 const d=record.data||{},correlation=d.correlation_id||d.scope?.correlation_id;
 const title=d.event_type?taskEventTitle({...d,status:d.payload?.status||d.status}):d.model_id?[domainName(d.role,'模型调用'),d.model_id].join(' · '):domainName(d.event_type||d.intent_type||d.agent_id||d.action||d.decision_point||d.boundary,({audit:'所有者操作记录','agent-runs':'Agent 运行记录','workflow-intents':'后台任务'})[record.kind]||'系统运行记录');
 const status=d.status||d.payload?.status||d.call_status||d.details?.status;
 const reason=d.failure_code||d.error_code||d.reason_code||d.last_error||d.payload?.reason_code||d.payload?.safe_cause;
 const failed=['failed','dead_letter','error','timeout','timed_out'].includes(status);
 const explanation=taskEventReason({status,reason_code:reason,safe_cause:d.safe_cause||d.payload?.safe_cause,reason:d.reason||(failed?d.last_error:undefined)});
 const budget=d.metrics?.prompt_budget||{},budgetText=[['estimated_input_tokens','估算输入']].filter(([key])=>Number.isFinite(budget[key])).map(([key,label])=>label+' '+budget[key]).join(' · ');
 let deliverySummary='';
 if(d.event_type==='reply.media_delivery.checked'){
  const check=d.payload||{},reasons={invalid_claim_quote:'校验器引用的片段不在本次候选里，回复未发布。',delivery_reference_missing:'校验器认为回复声称图片已发送，但没有找到匹配的图片交付记录。',delivery_reference_invalid:'引用的图片交付记录无法核对，回复未发布。',delivery_subject_mismatch:'回复所指图片与实际交付的图片不一致，回复未发布。',delivery_scope_mismatch:'图片发送的会话或动态范围不匹配，回复未发布。',unresolved_delivery_language:'回复仍包含无法核实的图片交付表述，尚未发布。'};
  const passed=check.status==='passed',ignored=Array.isArray(check.ignored_claims)?check.ignored_claims:[];
  const summary=passed?(ignored.length?'检查通过：已排除校验器误提取的非交付表述。':'检查通过：没有未核实的图片交付声明。'):(reasons[check.reason_code]||'图片发送状态检查未通过，回复未发布。');
  deliverySummary=`<section aria-label="图片发送检查结论"><p>${esc(summary)}</p>${check.rejected_claim?`<p>被拦内容：</p><blockquote>${esc(check.rejected_claim)}</blockquote>`:''}${Array.isArray(check.committed_deliveries)?`<p>同一会话内可核对的图片交付记录：${check.committed_deliveries.length} 条。</p>`:''}${check.check_correlation_id?`<a data-domain-ui="button" data-domain-variant="outline" data-domain-size="sm" href="#/diag/model-runs?correlationId=${encodeURIComponent(check.check_correlation_id)}">查看这次校验的输入与判断</a>`:''}</section>`;
 }
 return `<article data-domain-ui="card" class="card diagnostic-card"><div class="row"><b class="grow">${esc(title)}</b>${badge(stateLabel(status,d.details?.error?'命令执行失败':'已记录'),'plain')}</div><p class="small muted">${esc(userInstant(record.occurred_at))}</p>${record.kind==='events'?'<p class="small muted">事件发生时的状态快照</p>':''}${d.source==='termination_event'?'<p class="small muted">此条是执行结束时记录的状态；开始时间尚未记录。</p>':''}${record.kind==='model-runs'?`<p class="small muted">${esc(callStageLabel(d))}${Number.isInteger(d.round_count)?` · 已记录 ${d.round_count} 次调用`:' · 调用次数尚未记录'}</p>`:''}${deliverySummary}${!deliverySummary&&explanation?`<p class="small">${esc(explanation)}</p>`:''}${budgetText?`<p class="small diagnostic-budget">${esc(domainName(d.binding_role||d.role,'本次绑定'))} · ${esc(budgetText)}</p>`:''}${record.kind==='lifecycle'&&status==='cancelled'?'<p class="small muted">这是一次执行尝试的中断记录，不表示用户手动取消；当前消息结果以回合状态为准。</p>':''}${JsonDetails({recordId:record.id,originalTime:record.occurred_at,kind:record.kind,status,reason,stage:d.stage,logicalRunId:d.logical_run_id||d.metrics?.run_id},'原始状态、时间与关联标识')}<div class="row">${detail?'':`<a data-domain-ui="button" data-domain-variant="outline" data-domain-size="sm" href="#/diag/record/${encodeURIComponent(record.kind)}/${encodeURIComponent(record.ref)}">完整记录与来源</a>`}${correlation?`<a data-domain-ui="button" data-domain-variant="ghost" data-domain-size="sm" href="#/diag/events?correlationId=${encodeURIComponent(correlation)}">相关事件</a>`:''}${d.workflow_id?`<a data-domain-ui="button" data-domain-variant="ghost" data-domain-size="sm" href="#/diag/workflow/${encodeURIComponent(d.workflow_id)}">执行状态与恢复</a>`:''}${d.intent_id?`<a data-domain-ui="button" data-domain-variant="ghost" data-domain-size="sm" href="#/diag/task/${encodeURIComponent(d.intent_id)}">任务详情</a>`:''}</div></article>`;
}
