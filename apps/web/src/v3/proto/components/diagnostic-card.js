import {esc,badge} from './controls.js';
import {JsonDetails} from './records.js';
import {userInstant} from '../../presentation-time.mjs';
import {stateLabel,domainName} from '../../domain-presentation.mjs';
export function DiagnosticCard(record){
 const d=record.data||{},correlation=d.correlation_id||d.scope?.correlation_id;
 const title=d.model_id?[domainName(d.role,'模型调用'),d.model_id].join(' · '):domainName(d.event_type||d.intent_type||d.agent_id||d.action||d.decision_point||d.boundary,({audit:'所有者操作记录','agent-runs':'Agent 运行记录','workflow-intents':'后台任务'})[record.kind]||'系统运行记录');
 const status=d.status||d.payload?.status||d.call_status||d.details?.status;
 const reason=d.failure_code||d.error_code||d.reason_code;
 return `<article class="card diagnostic-card"><div class="row"><b class="grow">${esc(title)}</b>${badge(stateLabel(status,d.details?.error?'命令执行失败':'已记录'),'plain')}</div><p class="small muted">${esc(userInstant(record.occurred_at))}</p>${d.source==='termination_event'?'<p class="small muted">此条是执行结束时记录的状态；开始时间尚未记录。</p>':''}${record.kind==='model-runs'?`<p class="small muted">${esc(domainName(d.stage,'模型处理阶段'))}${Number.isInteger(d.round_count)?` · 已记录 ${d.round_count} 次调用`:' · 调用次数尚未记录'}</p>`:''}${reason?`<p class="small">${esc(stateLabel(reason,'本次执行有待核对的原因，详见下方记录。'))}</p>`:''}${JsonDetails({recordId:record.id,originalTime:record.occurred_at,kind:record.kind,status,reason,stage:d.stage,logicalRunId:d.logical_run_id||d.metrics?.run_id},'原始状态、时间与关联标识')}<div class="row"><a class="btn sm" href="#/diag/record/${encodeURIComponent(record.kind)}/${encodeURIComponent(record.ref)}">完整记录与来源</a>${correlation?`<a class="btn ghost sm" href="#/diag/events?correlationId=${encodeURIComponent(correlation)}">相关事件</a>`:''}${d.workflow_id?`<a class="btn ghost sm" href="#/diag/workflow/${encodeURIComponent(d.workflow_id)}">执行状态与恢复</a>`:''}${d.intent_id?`<a class="btn ghost sm" href="#/diag/task/${encodeURIComponent(d.intent_id)}">任务详情</a>`:''}</div></article>`;
}
