import {esc,badge} from './controls.js';
import {JsonDetails} from './records.js';
import {stateLabel} from '../../domain-presentation.mjs';
import {planWindow} from '../../presentation-time.mjs';
import {formatZonedTime,formatZonedRange} from '../../../lib/fluctlight-display.ts';

// Both endpoints retain the full public date format; the layout owns wrapping.
export function ScheduleTimeRange(start,end,zone){
 const valid=typeof start==='string'&&typeof end==='string'&&Number.isFinite(Date.parse(start))&&Number.isFinite(Date.parse(end));
 if(!valid)return '<span class="schedule-range">时间未设定</span>';
 return `<span class="schedule-range" aria-label="${esc(formatZonedRange(start,end,zone))}"><time datetime="${esc(start)}">${esc(formatZonedTime(start,zone))}</time><span class="schedule-range-end"><span class="schedule-range-separator" aria-hidden="true">至</span><time datetime="${esc(end)}">${esc(formatZonedTime(end,zone))}</time></span></span>`;
}

function ScheduleSummaryRange(start,end,zone){
 if(!Number.isFinite(Date.parse(start))||!Number.isFinite(Date.parse(end)))return ScheduleTimeRange(start,end,zone);
 const day=new Intl.DateTimeFormat('zh-CN',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit'}),time=new Intl.DateTimeFormat('zh-CN',{timeZone:'Asia/Shanghai',hour:'2-digit',minute:'2-digit',hour12:false}),from=new Date(start),to=new Date(end),same=day.format(from)===day.format(to),label=date=>same?time.format(date):day.format(date)+' '+time.format(date),full=formatZonedRange(start,end,zone);
 return `<span class="schedule-range" title="${esc(full)}" aria-label="${esc(full)}"><time datetime="${esc(start)}">${esc(label(from))}</time><span aria-hidden="true"> — </span><time datetime="${esc(end)}">${esc(label(to))}</time></span>`;
}

export const planWindowLabel=window=>({current:'当前计划时段',past:'历史计划时段',future:'尚未到时段',unknown:'时间待核对'})[window];
export function SchedulePlanItem(item,zone,now=Date.now()){
 const window=planWindow(item.start_at,item.end_at,now);
 return `<div data-domain-ui="item" class="lrow planfact schedule-slot ${window==='current'?'is-current':window==='past'?'is-past':''}" data-schedule-slot="${esc(item.id)}"><div class="small"><b>${ScheduleTimeRange(item.start_at,item.end_at,zone)}</b><div data-window-label>${esc(planWindowLabel(window))}</div></div><div><span class="schedule-cell-label">计划</span><b>${esc(item.activity)}</b> ${item.status?badge(stateLabel(item.status)):''}<div class="small muted">${esc(item.location || '地点未记录')}</div><details data-domain-ui="disclosure" class="schedule-item-options"><summary>计划操作与来源</summary><div class="row"><button data-domain-ui="button" class="btn ghost sm" data-edit="${esc(item.id)}">建议调整</button><button data-domain-ui="button" class="btn ghost sm" data-cancel="${esc(item.id)}">建议取消</button></div>${JsonDetails(item,'原始时间与来源')}</details></div><div><span class="schedule-cell-label">实际</span>${item.action_status?badge(stateLabel(item.action_status)):'<span class="small muted" aria-label="尚无执行结果" title="尚无执行结果">—</span>'}${item.intention_id?`<details data-domain-ui="disclosure"><summary class="small muted">关联意向</summary><p class="small">${esc(item.intention_id)}</p></details>`:''}</div></div>`;
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

export function ScheduleTimelineItem(item,zone,{expanded=false,editable=true}={}){
 const window=planWindow(item.start_at,item.end_at);
 const promise=item.promise_request_id;
 const canEdit=editable&&window==='future'&&!['in_progress','completed'].includes(item.action_status);
 return `<details data-domain-ui="disclosure" class="schedule-timeline-item ${window==='current'?'is-current':''}" data-schedule-expand="${esc(item.id)}" ${expanded?'open':''}><summary><span class="schedule-time">${ScheduleSummaryRange(item.start_at,item.end_at,zone)}</span><span class="grow"><span class="schedule-item-heading"><b>${esc(item.activity)}</b>${item.status?badge(stateLabel(item.status)):badge('计划状态未知','plain')}</span><span class="small muted">${esc(item.location||item.scene||'地点未记录')}</span></span><span aria-hidden="true">⌄</span></summary><div class="schedule-timeline-body" data-schedule-slot="${esc(item.id)}"><p class="small muted" data-window-label>${esc(planWindowLabel(window))}</p><div class="row">${item.status?badge(stateLabel(item.status)):badge('计划状态未知')}${item.action_status?badge(stateLabel(item.action_status)):badge(promise?'实际履行另行记录':'执行未知','plain')}</div><p>${esc(item.scene||'场景未记录')}</p>${promise?`<p class="small muted">${item.promise_schedule_valid===false?'这项约定已取消、完成或改变时段；此处保留原安排记录。':'这是已接受约定的时间安排，排入不代表执行或履约。'}</p><button data-domain-ui="button" class="btn" data-promise-id="${esc(promise)}">查看原约定与证据</button>`:''}${item.place_id?`<a data-domain-ui="button" class="btn ghost sm" href="${esc('#/c/'+item.charId+'/world/place/'+encodeURIComponent(item.place_id))}">查看地点</a>`:''}${canEdit?`<div class="row"><button data-domain-ui="button" class="btn" data-edit="${esc(item.id)}">建议调整</button><button data-domain-ui="button" class="btn ghost" data-cancel="${esc(item.id)}">建议取消</button></div>`:''}${JsonDetails(item,'安排来源与原记录')}</div></details>`;
}
