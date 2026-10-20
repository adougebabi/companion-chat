import {esc,badge} from './controls.js';
import {JsonDetails} from './records.js';
import {userInstant} from '../../presentation-time.mjs';
import {domainName,stateLabel} from '../../domain-presentation.mjs';
export function GoalSummary(goal,{profiles=[],zone}={}){
 const title=goal.desired_outcome||goal.description||goal.title||'尚未补充目标内容';
 const criteria=Array.isArray(goal.success_criteria)?goal.success_criteria.filter(v=>typeof v==='string'):typeof goal.success_criteria==='string'?[goal.success_criteria]:[];
 const profile=profiles.find(p=>p.id===goal.profile_id);
 return `<div class="stack goal-summary"><b>${esc(title)}</b>${goal.motivation?`<p>${esc(goal.motivation)}</p>`:''}${criteria.length?`<div><span class="small muted">完成条件</span><ul>${criteria.map(v=>`<li>${esc(v)}</li>`).join('')}</ul></div>`:''}${goal.deadline?`<p class="small muted">期望完成时间：${esc(userInstant(goal.deadline,zone))}</p>`:''}${goal.profile_id?`<p class="small muted">所属人格：${esc(profile?.name||'人格名称尚未提供')}</p>`:''}${JsonDetails(goal,'目标完整资料与来源')}</div>`;
}
export function GoalRecords(values,{empty='尚无记录',fallback='目标记录'}={}){
 const rows=Array.isArray(values)?values:[];
 return rows.length?rows.map(row=>{const reason=row.reason||row.decision_reason,readable=typeof reason==='string'&&/[\u3400-\u9fff]/u.test(reason)?reason:'';const title=readable||row.desired_outcome||row.description||domainName(row.operation||row.event_type||reason,fallback);return `<div class="lrow"><div class="grow"><div class="row"><b class="grow">${esc(title)}</b>${row.status?badge(stateLabel(row.status),'plain'):''}</div>${row.created_at||row.occurred_at?`<p class="small muted">${esc(userInstant(row.created_at||row.occurred_at))}</p>`:''}${JsonDetails(row,'完整记录与来源')}</div></div>`;}).join(''):`<p class="small muted">${esc(empty)}</p>`;
}
