import {LifeNavigation} from '../components/life-navigation.js';
import { lifeClient } from '../services.js';
import { asyncPage,requestReceipt } from '../components/async-page.js';
import { Field,Tabs } from '../components/forms.js';
import {IntentionList} from '../components/cards.js';
import {mountPromiseCatalogue,promiseForm,openPromise} from '../components/promise-catalogue.js';
import {scheduleRequestForm,observeScheduleRequest} from '../components/schedule-request.js';
import {displayTimezone} from '../../presentation-time.mjs';
import {zonedInput,zonedInputInstant} from '../../zoned-input.mjs';
import {SchedulePlanItem,ScheduleTimelineItem,refreshPlanWindows} from '../components/schedule-records.js';
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

const scheduleViews=new Map();
const nextDate=(value,days)=>{const date=new Date(value+'T12:00:00Z');date.setUTCDate(date.getUTCDate()+days);return date.toISOString().slice(0,10);};
export function renderRealSchedule(el,ctx) {
  const key=JSON.stringify([ctx.store.get('session')?.actorId,ctx.charId]);
  const view=scheduleViews.get(key)||{section:'today',expanded:new Set()};scheduleViews.set(key,view);
  return asyncPage(el,async()=>{
    const detail=await lifeClient.detail(ctx.charId);let calendar;
    try{calendar=await lifeClient.schedules(ctx.charId,ctx.query.date?{from:ctx.query.date,to:ctx.query.date}:{});}
    catch(error){if(![404,405].includes(error.status))throw error;const date=detail.schedule?.local_date||detail.context?.local_date;calendar={timezone:displayTimezone(detail.schedule?.timezone,detail.context?.timezone),today:date,from:date,to:date,days:[{local_date:date,schedule:detail.schedule,request:null}],context_revision:detail.context?.context_revision,unavailable:true};}
    return {detail,calendar};
  },({detail,calendar},reload,alive)=>{
    const selected=calendar.days[0],schedule=selected?.schedule,zone=calendar.timezone;
    const editable=calendar.from===calendar.today;
    const habits=detail.core_persona?.personality_system?.profiles?.find(p=>p.id===detail.personality_runtime?.active_profile_id)?.habits;
    el.innerHTML=`<div class="stack schedule-workspace"><div class="row"><div class="grow"><h2 class="page-title">她的日程</h2><p class="small muted">${esc(zone)} · 计划与实际分别记录</p></div><button class="btn" data-reload>刷新</button></div>${LifeNavigation(ctx)}${Tabs({value:view.section,attribute:'schedule-section',attrs:{'aria-label':'日程分区'},items:[['today','当天安排'],['upcoming','接下来几天'],['intentions','意向'],['promises','约定'],['habits','生活倾向']].map(([value,label])=>({value,label}))})}<form class="row schedule-date-query" data-date-query>${Field({label:'查看日期',name:'date',type:'date',value:calendar.from||'',attrs:{required:true,disabled:calendar.unavailable}})}<button class="btn" ${calendar.unavailable?'disabled':''}>查找</button><button type="button" class="btn ghost" data-today>回到今天</button><span class="spacer"></span><button type="button" class="btn primary" data-generate ${calendar.unavailable?'disabled':''}>生成日程</button>${editable?'<button type="button" class="btn" data-suggest>建议一项日程</button>':''}</form>${calendar.unavailable?'<p class="banner info">当前服务尚未升级日程接口，先显示已读取的今天。跨日查阅与生成需要运行本次 Core 构建。</p>':''}${selected?.request?`<p class="small muted">最近的安排请求：${esc(({queued:'等待规划',running:'正在规划',completed:'已完成处理',failed:'未完成',conflict:'资料变化'})[selected.request.status]||'状态待核对')} <button class="btn ghost sm" data-original-request>查看进度与结果</button></p>`:''}<section data-section="today"><h3>${esc(calendar.from||'当前日期')}</h3><div class="schedule-timeline">${schedule?schedule.items.map(item=>ScheduleTimelineItem({...item,charId:ctx.charId},zone,{expanded:view.expanded.has(item.id),editable})).join(''):'<div class="statebox"><h3>这一天尚无已接受日程</h3><p>可请求为缺失日期规划；已有安排会受到保护。</p></div>'}</div></section><section data-section="upcoming" hidden><div data-upcoming></div></section><section data-section="intentions" hidden><h3>她的意向</h3>${IntentionList(detail.intentions||[],{fid:ctx.charId,goals:detail.goals||[],profiles:detail.core_persona?.personality_system?.profiles||[]})}<p class="small muted">意向仍需实际执行，不能据此视为已完成。</p></section><section data-section="promises" hidden><div class="row"><h3 class="grow">我们的约定</h3><button class="btn primary" data-propose>和她约定</button></div><div data-promises></div></section><section data-section="habits" hidden><h3>生活倾向</h3><p class="reading-prose">${esc(typeof habits==='string'&&habits.trim()?habits:'当前人格尚未记录生活倾向。')}</p><p class="small muted">这些是生活中的软倾向，具体安排仍以实际日程为准。</p></section><p data-error role="alert"></p></div>`;
    const promises=mountPromiseCatalogue(el.querySelector('[data-promises]'),{fid:ctx.charId,alive,onSaved:reload});
    el.querySelector('[data-propose]').onclick=()=>promiseForm(ctx.charId,{onSaved:envelope=>{if(alive())requestReceipt(ctx.charId,envelope,reload);}});
    el.querySelectorAll('[data-schedule-expand]').forEach(row=>row.addEventListener('toggle',()=>{if(row.open)view.expanded.add(row.dataset.scheduleExpand);else view.expanded.delete(row.dataset.scheduleExpand);}));
    let upcomingLoaded=false;
    async function show(section){
      view.section=section;el.querySelectorAll('[data-section]').forEach(box=>box.hidden=box.dataset.section!==section);
      el.querySelectorAll('[data-schedule-section]').forEach(button=>{button.classList.toggle('on',button.dataset.scheduleSection===section);button.setAttribute('aria-pressed',String(button.dataset.scheduleSection===section));});
      if(section==='upcoming'&&!upcomingLoaded){const box=el.querySelector('[data-upcoming]');if(calendar.unavailable){box.innerHTML='<p>跨日服务尚未升级，当前不能读取接下来几天。</p>';return;}box.innerHTML='<p role="status">正在读取接下来的安排…</p>';try{const page=await lifeClient.schedules(ctx.charId,{from:nextDate(calendar.from,1),to:nextDate(calendar.from,7)});if(!alive())return;upcomingLoaded=true;box.innerHTML=page.days.map(day=>`<section class="schedule-day"><h3>${esc(day.local_date)}</h3>${day.schedule?day.schedule.items.map(item=>ScheduleTimelineItem({...item,charId:ctx.charId},zone,{editable:false})).join(''):'<p class="small muted">这一天尚无已接受日程。</p>'}</section>`).join('');box.querySelectorAll('[data-promise-id]').forEach(button=>button.onclick=()=>openPromise(ctx.charId,button.dataset.promiseId,reload,alive));}catch(error){if(alive())box.innerHTML=`<p class="banner err" role="alert">${esc(error.message)}</p><button class="btn" data-upcoming-retry>重新读取</button>`;box.querySelector('[data-upcoming-retry]')?.addEventListener('click',()=>void show('upcoming'));}}
    }
    el.querySelectorAll('[data-schedule-section]').forEach(button=>button.onclick=()=>void show(button.dataset.scheduleSection));void show(view.section);
    el.querySelector('[data-date-query]').onsubmit=event=>{event.preventDefault();if(event.target.reportValidity())ctx.navigate(`#/c/${encodeURIComponent(ctx.charId)}/life/schedule?date=${event.target.elements.date.value}`);};
    el.querySelector('[data-today]').onclick=()=>ctx.navigate(`#/c/${encodeURIComponent(ctx.charId)}/life/schedule`);
    el.querySelector('[data-generate]').onclick=()=>scheduleRequestForm(ctx.charId,{calendar,schedule,onCompleted:reload});
    el.querySelector('[data-original-request]')?.addEventListener('click',()=>observeScheduleRequest(ctx.charId,selected.request,reload));
    el.querySelectorAll('[data-promise-id]').forEach(button=>button.onclick=()=>openPromise(ctx.charId,button.dataset.promiseId,reload,alive));
    el.querySelector('[data-reload]').onclick=reload;
    el.querySelector('[data-suggest]')?.addEventListener('click',()=>suggestRealSchedule(ctx.charId,{detail:{...detail,schedule},reload}));
    el.querySelectorAll('[data-edit]').forEach(button=>button.onclick=()=>suggestRealSchedule(ctx.charId,{detail,item:schedule.items.find(i=>i.id===button.dataset.edit),reload}));
    el.querySelectorAll('[data-cancel]').forEach(button=>button.onclick=()=>{
      const item=schedule.items.find(i=>i.id===button.dataset.cancel);
      const d=drawer({title:'建议取消日程',body:`<p>你正在建议取消「${esc(item.activity)}」，由她决定是否接受。历史记录会保留。</p><form class="stack" id="cancel-schedule-form">${Field({label:'建议原因',name:'reason',type:'textarea',attrs:{required:true,maxlength:500}})}<p data-error role="alert"></p></form>`,footer:'<button class="btn" data-close>关闭</button><button class="btn primary" type="submit" form="cancel-schedule-form" data-save>发出取消建议</button>'});
      d.el.querySelector('[data-close]').onclick=()=>d.close();let busy=false;
      d.el.querySelector('form').onsubmit=async event=>{event.preventDefault();if(busy||!event.target.reportValidity())return;const reason=event.target.elements.reason.value.trim();if(!reason){d.el.querySelector('[data-error]').textContent='请填写建议原因';return;}busy=true;const save=d.el.querySelector('[data-save]');save.disabled=true;try{const result=await lifeClient.scheduleItem(ctx.charId,detail,item,{reason},'cancel');if(d.el.isConnected){d.close();requestReceipt(ctx.charId,result,reload);}}catch(error){if(d.el.isConnected)d.el.querySelector('[data-error]').textContent=error.message;}finally{busy=false;if(d.el.isConnected)save.disabled=false;}};
    });
    const refreshWindows=()=>{if(alive()&&!document.hidden&&schedule)refreshPlanWindows(el,schedule.items);};
    const timer=setInterval(refreshWindows,30000);document.addEventListener('visibilitychange',refreshWindows);
    return()=>{promises.dispose();clearInterval(timer);document.removeEventListener('visibilitychange',refreshWindows);};
  });
}
