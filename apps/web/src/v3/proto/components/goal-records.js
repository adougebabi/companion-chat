import {esc,badge} from './controls.js';
import {JsonDetails,RawRecordDownload} from './records.js';
import {userInstant} from '../../presentation-time.mjs';
import {goalRecordSummary} from '../../../approved-ui/goal-record-presentation.mjs';
export function GoalSummary(goal,{profiles=[],zone}={}){
 const title=goal.desired_outcome||goal.description||goal.title||'尚未补充目标内容';
 const criteria=Array.isArray(goal.success_criteria)?goal.success_criteria.filter(v=>typeof v==='string'):typeof goal.success_criteria==='string'?[goal.success_criteria]:[];
 const profile=profiles.find(p=>p.id===goal.profile_id);
 return `<div class="stack goal-summary"><b>${esc(title)}</b>${goal.motivation?`<p>${esc(goal.motivation)}</p>`:''}${criteria.length?`<div><span class="small muted">完成条件</span><ul>${criteria.map(v=>`<li>${esc(v)}</li>`).join('')}</ul></div>`:''}${goal.deadline?`<p class="small muted">期望完成时间：${esc(userInstant(goal.deadline,zone))}</p>`:''}${goal.profile_id?`<p class="small muted">所属人格：${esc(profile?.name||'人格名称尚未提供')}</p>`:''}${JsonDetails(goal,'目标完整资料与来源')}</div>`;
}
export function GoalRecordFacts(facts) {
 return facts.filter(([,value])=>value).map(([label,value])=>
  `<p class="small muted">${esc(label)}：${esc(value)}</p>`).join('');
}
export function GoalRecords(values,{kind,goalNames,downloadTitle='原始目标与推进记录（诊断）'}={}){
 const rows=Array.isArray(values)?values:values==null?[]:[values];
 return rows.filter(row=>row!=null&&row!==''&&(typeof row!=='object'||Object.keys(row).length)).map(row=>{
  const {title,status,facts}=goalRecordSummary(row,kind,goalNames);
  return `<div data-domain-ui="item" class="lrow"><div class="grow">
    <div class="row"><b class="grow">${esc(title)}</b>${status?badge(status,'plain'):''}</div>
    ${GoalRecordFacts(facts)}${RawRecordDownload(row,downloadTitle)}
   </div></div>`;
 }).join('');
}
