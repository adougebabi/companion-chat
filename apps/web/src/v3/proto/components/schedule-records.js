import {esc,badge} from './controls.js';
import {JsonDetails} from './records.js';
import {stateLabel} from '../../domain-presentation.mjs';
import {planWindow} from '../../presentation-time.mjs';
import {formatZonedRange} from '../../../lib/fluctlight-display.ts';

export const planWindowLabel=window=>({current:'当前计划时段',past:'历史计划时段',future:'尚未到时段',unknown:'时间待核对'})[window];
export function SchedulePlanItem(item,zone,now=Date.now()){
 const window=planWindow(item.start_at,item.end_at,now);
 return `<div class="lrow planfact schedule-slot ${window==='current'?'is-current':window==='past'?'is-past':''}" data-schedule-slot="${esc(item.id)}"><div class="small"><b>${esc(formatZonedRange(item.start_at,item.end_at,zone))}</b><div data-window-label>${esc(planWindowLabel(window))}</div></div><div><span class="schedule-cell-label">计划</span><b>${esc(item.activity)}</b> ${item.status?badge(stateLabel(item.status)):''}<div class="small muted">${esc(item.location || '地点未记录')}</div><details class="schedule-item-options"><summary>计划操作与来源</summary><div class="row"><button class="btn ghost sm" data-edit="${esc(item.id)}">建议调整</button><button class="btn ghost sm" data-cancel="${esc(item.id)}">建议取消</button></div>${JsonDetails(item,'原始时间与来源')}</details></div><div><span class="schedule-cell-label">实际</span>${item.action_status?badge(stateLabel(item.action_status)):'<span class="small muted" aria-label="尚无执行结果" title="尚无执行结果">—</span>'}${item.intention_id?`<details><summary class="small muted">关联意向</summary><p class="small">${esc(item.intention_id)}</p></details>`:''}</div></div>`;
}
export function refreshPlanWindows(root,items,now=Date.now()){
 const byId=new Map(items.map(item=>[item.id,item]));
 root.querySelectorAll('[data-schedule-slot]').forEach(row=>{
  const item=byId.get(row.dataset.scheduleSlot);if(!item)return;
  const window=planWindow(item.start_at,item.end_at,now);
  row.classList.toggle('is-current',window==='current');row.classList.toggle('is-past',window==='past');
  row.querySelector('[data-window-label]').textContent=planWindowLabel(window);
 });
}
