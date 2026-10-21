import { lifeClient } from '../services.js';
import { asyncPage,requestReceipt } from '../components/async-page.js';
import { Field } from '../components/forms.js';
import {displayTimezone} from '../../presentation-time.mjs';
import {zonedInput,zonedInputInstant} from '../../zoned-input.mjs';
import {SchedulePlanItem,refreshPlanWindows} from '../components/schedule-records.js';
import { esc,badge,drawer,toast } from '../ui.js';

export async function suggestRealSchedule(fid,{detail,item=null,placeId='',title='',reload=()=>{}}={}) {
  try {
    detail=detail || await lifeClient.detail(fid);
    if(!detail.schedule)throw new Error('今天的初始日程尚未就绪，请在运行诊断查看日程初始化任务，或在对话中与她安排。');
    const zone=displayTimezone(detail.schedule.timezone,detail.context?.timezone),originalTimes={startAt:item?.start_at,endAt:item?.end_at};
    const d=drawer({title:item?'建议调整日程':'建议一项日程',body:`<p class="small muted">${badge('行动请求')} 由她决定是否接受，接受并执行后才会形成新计划。</p><form class="stack" data-form id="schedule-suggestion-form">${Field({label:'做什么',name:'activity',value:item?.activity || title,attrs:{required:true}})}<section class="stack"><div class="row"><b class="grow">时间安排</b><span class="small muted">${esc(zone)}</span></div><div class="configuration-fields">${Field({label:'开始时间',name:'startLocal',type:'datetime-local',value:zonedInput(item?.start_at,zone),attrs:{required:true}})}${Field({label:'结束时间',name:'endLocal',type:'datetime-local',value:zonedInput(item?.end_at,zone),attrs:{required:true}})}</div><details class="schedule-offset"><summary>时区与精确时间</summary><p class="small muted">上方时间使用 ${esc(zone)}。跨时区或夏令时有歧义时，可以直接输入带偏移的完整时间。</p><label class="toggle"><input type="checkbox" data-offset-mode>直接输入带时区偏移的时间</label><div class="stack" data-offset-fields hidden>${Field({label:'完整开始时间',name:'startAt',value:item?.start_at||'',hint:'例如 2026-10-21T15:00:00+08:00',attrs:{required:true,disabled:true}})}${Field({label:'完整结束时间',name:'endAt',value:item?.end_at||'',attrs:{required:true,disabled:true}})}</div></details></section>${Field({label:'地点描述',name:'location',value:item?.location || ''})}<details class="schedule-location" ${item?.place_id||placeId?'open':''}><summary>场景与精确地点（可选）</summary><div class="stack">${Field({label:'场景',name:'scene',value:item?.scene || ''})}${Field({label:'地点 ID',name:'placeId',value:item?.place_id || placeId,hint:'填写运行世界中的地点 ID；仅填写地点描述不会改变实际在场状态。'})}</div></details>${Field({label:'建议原因',name:'reason',type:'textarea',attrs:{required:true,maxlength:500}})}<p data-error role="alert"></p></form>`,footer:'<button class="btn" data-close>关闭</button><button class="btn primary" type="submit" form="schedule-suggestion-form" data-save>发出建议</button>'});
    d.el.querySelector('[data-close]').onclick=()=>d.close();
    const form=d.el.querySelector('form'),save=d.el.querySelector('[data-save]');let busy=false;
    d.el.querySelector('[data-offset-mode]').onchange=event=>{
      const exact=event.target.checked;d.el.querySelector('[data-offset-fields]').hidden=!exact;
      for(const [local,raw,original] of [['startLocal','startAt',originalTimes.startAt],['endLocal','endAt',originalTimes.endAt]]){
        if(exact&&form.elements[local].value)try{form.elements[raw].value=zonedInputInstant(form.elements[local].value,zone,original);}catch{}
        if(!exact&&/[+-]\d{2}:\d{2}$|Z$/.test(form.elements[raw].value)&&Number.isFinite(Date.parse(form.elements[raw].value))){form.elements[local].value=zonedInput(form.elements[raw].value,zone);originalTimes[raw]=form.elements[raw].value;}
        form.elements[local].disabled=exact;form.elements[raw].disabled=!exact;
      }
    };
    form.onsubmit=async event=>{
      event.preventDefault();if(busy || !form.reportValidity())return;
      d.el.querySelector('[data-error]').textContent='';
      try {
        const exact=d.el.querySelector('[data-offset-mode]').checked,values=Object.fromEntries(new FormData(form));
        const body={activity:values.activity,scene:values.scene,location:values.location,placeId:values.placeId,reason:values.reason,
          startAt:exact?values.startAt:zonedInputInstant(values.startLocal,zone,originalTimes.startAt),endAt:exact?values.endAt:zonedInputInstant(values.endLocal,zone,originalTimes.endAt)};
        if(!/[+-]\d{2}:\d{2}$|Z$/.test(body.startAt)||!/[+-]\d{2}:\d{2}$|Z$/.test(body.endAt)||!Number.isFinite(Date.parse(body.startAt))||!Number.isFinite(Date.parse(body.endAt)))throw new Error('完整时间需要有效日期和明确的时区偏移');
        if(Date.parse(body.endAt)<=Date.parse(body.startAt)){form.elements[exact?'endAt':'endLocal'].focus();throw new Error('结束时间需要晚于开始时间');}
        busy=true;save.disabled=true;save.textContent='正在提交…';
        const result=await lifeClient.scheduleItem(fid,detail,item,body,item?'update':'create');
        if(d.el.isConnected){d.close();requestReceipt(fid,result,reload);}
      } catch(error) {if(d.el.isConnected)d.el.querySelector('[data-error]').textContent=`${error.message}${error.status===409?'；草稿已保留，请关闭后重新读取日程并确认。':''}`;}
      finally {busy=false;if(d.el.isConnected){save.disabled=false;save.textContent='发出建议';}}
    };
  } catch(error) {toast(error.message,'err');}
}

export function renderRealSchedule(el,ctx) {
  return asyncPage(el,()=>lifeClient.detail(ctx.charId),(detail,reload,alive)=>{
    const schedule=detail.schedule,zone=displayTimezone(schedule?.timezone,detail.context?.timezone);
    el.innerHTML=`<div class="row" style="margin-bottom:12px"><span class="grow small muted">${esc(schedule?.local_date || detail.context?.local_date || '')} · ${esc(zone || '')} · 计划与实际分开记录</span><button class="btn" data-reload>重新读取</button><button class="btn primary" data-suggest>建议一项日程</button></div><div class="card"><div class="planfact schedule-head small muted"><b>时间</b><b>计划</b><b>实际</b></div>${schedule?`<div class="list">${schedule.items.map(item=>SchedulePlanItem(item,zone)).join('')}</div>`:'<div class="statebox"><h2>今天尚无已接受日程</h2><p>初始日程由真实任务生成，等待不代表已经发生。</p><a class="btn" href="#/diag">查看初始化任务</a><a class="btn" href="#/c/'+ctx.charId+'/chat">与她安排日程</a></div>'}</div><p class="small muted">当前时段仅表示计划时间，实际发生的事情单独记录。经过时段不代表已经完成；“—”表示尚无执行结果。</p><p data-error role="alert"></p>`;
    el.querySelector('[data-reload]').onclick=reload;
    el.querySelector('[data-suggest]').onclick=()=>suggestRealSchedule(ctx.charId,{detail,reload});
    el.querySelectorAll('[data-edit]').forEach(button=>button.onclick=()=>suggestRealSchedule(ctx.charId,{detail,item:schedule.items.find(i=>i.id===button.dataset.edit),reload}));
    el.querySelectorAll('[data-cancel]').forEach(button=>button.onclick=()=>{
      const item=schedule.items.find(i=>i.id===button.dataset.cancel);
      const d=drawer({title:'建议取消日程',body:`<p>你正在建议取消「${esc(item.activity)}」，由她决定是否接受。历史记录会保留。</p><form class="stack" id="cancel-schedule-form">${Field({label:'建议原因',name:'reason',type:'textarea',attrs:{required:true,maxlength:500}})}<p data-error role="alert"></p></form>`,footer:'<button class="btn" data-close>关闭</button><button class="btn primary" type="submit" form="cancel-schedule-form" data-save>发出取消建议</button>'});
      d.el.querySelector('[data-close]').onclick=()=>d.close();let busy=false;
      d.el.querySelector('form').onsubmit=async event=>{event.preventDefault();if(busy||!event.target.reportValidity())return;const reason=event.target.elements.reason.value.trim();if(!reason){d.el.querySelector('[data-error]').textContent='请填写建议原因';return;}busy=true;const save=d.el.querySelector('[data-save]');save.disabled=true;try{const result=await lifeClient.scheduleItem(ctx.charId,detail,item,{reason},'cancel');if(d.el.isConnected){d.close();requestReceipt(ctx.charId,result,reload);}}catch(error){if(d.el.isConnected)d.el.querySelector('[data-error]').textContent=error.message;}finally{busy=false;if(d.el.isConnected)save.disabled=false;}};
    });
    const refreshWindows=()=>{if(alive()&&!document.hidden&&schedule)refreshPlanWindows(el,schedule.items);};
    const timer=setInterval(refreshWindows,30000);document.addEventListener('visibilitychange',refreshWindows);
    return()=>{clearInterval(timer);document.removeEventListener('visibilitychange',refreshWindows);};
  });
}
