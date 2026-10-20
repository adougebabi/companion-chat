import { lifeClient } from '../services.js';
import { asyncPage,requestReceipt } from '../components/async-page.js';
import { Field } from '../components/forms.js';
import {displayTimezone} from '../../presentation-time.mjs';
import {SchedulePlanItem,refreshPlanWindows} from '../components/schedule-records.js';
import { esc,badge,drawer,toast } from '../ui.js';

export async function suggestRealSchedule(fid,{detail,item=null,placeId='',title='',reload=()=>{}}={}) {
  try {
    detail=detail || await lifeClient.detail(fid);
    if(!detail.schedule)throw new Error('今天的初始日程尚未就绪，请在运行诊断查看日程初始化任务，或在对话中与她安排。');
    const d=drawer({title:item?'建议调整日程':'建议一项日程',body:`<p class="small muted">${badge('行动请求')} 她接受并执行后才会形成新计划。时区 ${esc(displayTimezone(detail.schedule.timezone,detail.context?.timezone))}。时间须包含明确偏移，例如 2026-10-17T15:00:00+08:00。</p><form class="stack" data-form>${Field({label:'做什么',name:'activity',value:item?.activity || title,attrs:{required:true}})}${Field({label:'开始时间',name:'startAt',value:item?.start_at || '',attrs:{required:true}})}${Field({label:'结束时间',name:'endAt',value:item?.end_at || '',attrs:{required:true}})}${Field({label:'场景',name:'scene',value:item?.scene || ''})}${Field({label:'地点描述',name:'location',value:item?.location || ''})}${Field({label:'地点 ID（可留空）',name:'placeId',value:item?.place_id || placeId,hint:'只接受当前运行世界的稳定地点，文本同名不会自动建立在场。'})}${Field({label:'建议原因',name:'reason',type:'textarea',attrs:{required:true,maxlength:500}})}<p data-error role="alert"></p></form>`,footer:'<button class="btn" data-close>关闭</button><button class="btn primary" data-save>发出建议</button>'});
    d.el.querySelector('[data-close]').onclick=()=>d.close();
    let busy=false;
    d.el.querySelector('[data-save]').onclick=async()=>{
      const form=d.el.querySelector('form');if(busy || !form.reportValidity())return;
      busy=true;const body=Object.fromEntries(new FormData(form));
      try {const result=await lifeClient.scheduleItem(fid,detail,item,body,item?'update':'create');d.close();requestReceipt(fid,result,reload);}
      catch(error) {if(d.el.isConnected)d.el.querySelector('[data-error]').textContent=`${error.message}${error.status===409?'；草稿已保留，请关闭后重新读取日程并确认。':''}`;}
      finally {busy=false;}
    };
  } catch(error) {toast(error.message,'err');}
}

export function renderRealSchedule(el,ctx) {
  return asyncPage(el,()=>lifeClient.detail(ctx.charId),(detail,reload,alive)=>{
    const schedule=detail.schedule,zone=displayTimezone(schedule?.timezone,detail.context?.timezone);
    el.innerHTML=`<div class="row" style="margin-bottom:12px"><span class="grow small muted">${esc(schedule?.local_date || detail.context?.local_date || '')} · ${esc(zone || '')} · 计划与实际分开记录</span><button class="btn" data-reload>重新读取</button><button class="btn primary" data-suggest>建议一项日程</button></div><div class="card"><div class="planfact schedule-head small muted"><b>时间</b><b>计划</b><b>实际</b></div>${schedule?`<div class="list">${schedule.items.map(item=>SchedulePlanItem(item,zone)).join('')}</div>`:'<div class="statebox"><h2>今天尚无已接受日程</h2><p>初始日程由真实任务生成，等待不代表已经发生。</p><a class="btn" href="#/diag">查看初始化任务</a><a class="btn" href="#/c/'+ctx.charId+'/chat">与她安排日程</a></div>'}</div><p class="small muted">当前计划时段按时钟标记，不代表正在执行。历史时段保留原计划，经过时段不代表已经完成。“—”表示尚无执行结果。调整使用日程版本与生活版本双重校验。</p><p data-error role="alert"></p>`;
    el.querySelector('[data-reload]').onclick=reload;
    el.querySelector('[data-suggest]').onclick=()=>suggestRealSchedule(ctx.charId,{detail,reload});
    el.querySelectorAll('[data-edit]').forEach(button=>button.onclick=()=>suggestRealSchedule(ctx.charId,{detail,item:schedule.items.find(i=>i.id===button.dataset.edit),reload}));
    el.querySelectorAll('[data-cancel]').forEach(button=>button.onclick=async()=>{
      const reason=prompt('建议取消的原因');if(!reason?.trim())return;
      button.disabled=true;
      try {const item=schedule.items.find(i=>i.id===button.dataset.cancel);requestReceipt(ctx.charId,await lifeClient.scheduleItem(ctx.charId,detail,item,{reason},'cancel'),reload);}
      catch(error){el.querySelector('[data-error]').textContent=error.message;}
      finally{button.disabled=false;}
    });
    const refreshWindows=()=>{if(alive()&&!document.hidden&&schedule)refreshPlanWindows(el,schedule.items);};
    const timer=setInterval(refreshWindows,30000);document.addEventListener('visibilitychange',refreshWindows);
    return()=>{clearInterval(timer);document.removeEventListener('visibilitychange',refreshWindows);};
  });
}
