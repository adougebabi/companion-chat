import {errorMessage} from '../../domain-presentation.mjs';
import {lifeClient} from '../services.js';
import {drawer,esc,badge} from '../ui.js';
import {Field} from './forms.js';
import {zonedInput,zonedInputInstant} from '../../zoned-input.mjs';

const labels={queued:'正在等待规划',running:'正在规划安排',completed:'已读取处理结果',failed:'这次安排未完成',conflict:'资料已经变化，原安排保持'};
export function observeScheduleRequest(fid,request,onCompleted=()=>{}){
 let alive=true,timer,reads=0,inFlight=false,notified=false;
 const d=drawer({title:'日程安排进度',body:'<div data-schedule-request></div>',footer:'<button class="btn" data-refresh>刷新进度</button><button class="btn" data-close>关闭</button>',onClose:()=>{alive=false;clearTimeout(timer);document.removeEventListener('visibilitychange',visible);}});
 const current=()=>alive&&d.el.isConnected;
 function paint(error=''){
  if(!current())return;const result=request.result||{};
  d.body.querySelector('[data-schedule-request]').innerHTML=`<h3>${esc(labels[request.status]||'状态尚未确认')}</h3><p>日程日期：${esc(request.local_date)}</p>${request.status==='completed'?`<p>${result.disposition==='preserved'?'这一天已有日程，原安排已保留。':result.disposition==='already_scheduled'?'这条约定已有安排，已定位原条目。':result.disposition==='scheduled'?'已读回约定关联的真实日程条目；执行与履约仍单独记录。':'日程已生成并读回。'}</p><a class="btn" href="#/c/${encodeURIComponent(fid)}/life/schedule?date=${encodeURIComponent(result.local_date||request.local_date)}">查看这一天的日程</a>`:request.status==='failed'||request.status==='conflict'?`<p class="banner warn">${esc(request.error_code==='schedule_scope_changed'?'角色、人格或世界已经变化，请核对最新资料后重新安排。':request.error_code==='schedule_task_stopped'?'原任务已停止，已保存的结果仍保留。':request.error_code==='schedule_request_date_elapsed'?'目标日期已经过去，没有补造历史日程。':'本次处理没有形成新的安排。可查看原任务原因，再决定是否重新请求。')}</p>`:'<p class="small muted">正在自动跟进原请求。关闭此处不会取消任务，也不会重复生成。</p>'}${error?`<p class="banner err" role="alert">更新失败，保留上次状态：${esc(error)}</p>`:''}<details class="mgmt"><summary>原请求与运行记录</summary><p>${esc(request.request_id)}</p><p>日程原时区：${esc(request.timezone)}</p><a href="#/diag/task/${encodeURIComponent(request.task_id)}">查看关联任务</a>${request.error_code?`<p>${esc(request.error_code)}</p>`:''}</details>`;
 }
 async function read(){
  clearTimeout(timer);if(!current()||inFlight||document.hidden)return;inFlight=true;
  try{const next=await lifeClient.scheduleRequest(fid,request.request_id);if(!current())return;request=next;paint();if(!['queued','running'].includes(request.status)&&!notified){notified=true;void onCompleted(request);}}
  catch(error){if(current())paint(errorMessage(error));}
  finally{inFlight=false;if(current()&&!document.hidden&&['queued','running'].includes(request.status)&&++reads<100)timer=setTimeout(read,3000);}
 }
 function visible(){if(!document.hidden)void read();else clearTimeout(timer)}
 document.addEventListener('visibilitychange',visible);d.el.querySelector('[data-close]').onclick=()=>d.close();d.el.querySelector('[data-refresh]').onclick=()=>{reads=0;void read();};paint();if(['queued','running'].includes(request.status))void read();else if(!notified){notified=true;void onCompleted(request);}return d;
}

export function scheduleRequestForm(fid,{calendar,schedule,promise=null,onCompleted=()=>{},localDate=calendar.from}={}){
 const zone=calendar.timezone,inputZone='Asia/Shanghai';
 if(!promise&&schedule){const d=drawer({title:'这一天已有日程记录',body:`<p>${esc(localDate)} 已有日程记录，其中包含 ${esc(schedule.items?.length??0)} 项安排。生成操作会保留整份已有日程；没有约定条目也不代表没有日程记录。</p><p>请返回当天安排查看；需要调整今天的安排时，可以使用“建议一项日程”。</p>`,footer:'<button class="btn primary" data-close>返回当天安排</button>'});d.el.querySelector('[data-close]').onclick=()=>d.close();return d;}
 const d=drawer({title:promise?'为约定安排时间':'生成这一天的日程',body:`<p class="small muted">${badge('安排请求')} ${promise?'只安排已经接受的约定；排入不代表执行或履约。':'只为缺少日程的日期规划，不覆盖已有安排。'}</p><form class="stack" id="owner-schedule-form">${Field({label:'日期',name:'localDate',type:'date',value:localDate,attrs:{required:true}})}<p class="small muted">日程归档时区：${esc(zone)}。开始与结束时间按北京时间填写。</p>${promise?`${Field({label:'开始时间',name:'startLocal',type:'datetime-local',value:zonedInput(promise.window_start,inputZone),attrs:{required:true}})}${Field({label:'结束时间',name:'endLocal',type:'datetime-local',value:'',attrs:{required:true}})}<p class="small muted">选择约定希望时段内的未来时间；当前填写尚未提交。</p>`:''}${Field({label:'安排说明',name:'reason',type:'textarea',attrs:{required:true,maxlength:500}})}<div data-original></div><p role="alert" data-error></p></form>`,footer:'<button class="btn" data-close>取消</button><button class="btn primary" form="owner-schedule-form" data-submit>请求安排</button>'});
 const form=d.el.querySelector('form'),save=d.el.querySelector('[data-submit]');let busy=false;
 d.el.querySelector('[data-close]').onclick=()=>d.close();
 let pendingDate=localDate;
 function showOriginal(){
  const pending=lifeClient.schedulePending(fid,pendingDate,promise?.request_id||'');
  save.disabled=busy||!!pending;form.querySelectorAll('input,textarea,select').forEach(control=>control.disabled=busy||!!pending);
  if(!pending){d.el.querySelector('[data-original]').innerHTML='';return false;}
  form.elements.localDate.value=pending.value.localDate;form.elements.reason.value=pending.value.reason;
  if(promise){form.elements.startLocal.value=zonedInput(pending.value.startAt,inputZone);form.elements.endLocal.value=zonedInput(pending.value.endAt,inputZone);}
  d.el.querySelector('[data-original]').innerHTML='<p class="banner warn">原提交的回执尚未确认。先核对原提交，再继续编辑。</p><button type="button" class="btn" data-original-query>查询或重试原提交</button>';
  d.el.querySelector('[data-original-query]').onclick=async()=>{if(busy)return;busy=true;showOriginal();try{const request=await lifeClient.retrySchedule(fid,pendingDate,promise?.request_id||'');if(d.el.isConnected){d.close();observeScheduleRequest(fid,request,onCompleted);}}catch(error){if(d.el.isConnected)d.el.querySelector('[data-error]').textContent=errorMessage(error);}finally{busy=false;if(d.el.isConnected)showOriginal();}};
  return true;
 }
 showOriginal();
 form.onsubmit=async event=>{
  event.preventDefault();if(busy||!form.reportValidity())return;pendingDate=form.elements.localDate.value;if(showOriginal())return;
  if(!form.elements.reason.value.trim()){d.el.querySelector('[data-error]').textContent='请填写安排说明';form.elements.reason.focus();return;}
  busy=true;save.disabled=true;form.querySelectorAll('input,textarea,select').forEach(control=>control.disabled=true);
  try{
   const date=form.elements.localDate.value;
   // Date edits read their own schedule/version before a command is captured.
   const latest=await lifeClient.schedules(fid,{from:date,to:date});
   if(!d.el.isConnected)return;
   if(latest.timezone!==zone)throw new Error('角色时区已经变化，请保留输入并重新打开安排表单。');
   const actual=latest.days[0]?.schedule;
   if(!promise&&actual)throw new Error('这一天已有安排，原日程保持。请关闭后查看，或提出调整建议。');
   if(promise&&Date.parse(zonedInputInstant(form.elements.endLocal.value,inputZone))<=Date.parse(zonedInputInstant(form.elements.startLocal.value,inputZone))){form.elements.endLocal.focus();throw new Error('结束时间需要晚于开始时间');}
   const body={localDate:date,expectedRevision:actual?.revision||0,expectedLifeContextRevision:latest.context_revision,reason:form.elements.reason.value.trim(),...(promise?{promiseId:promise.request_id,expectedPromiseRevision:promise.revision,startAt:zonedInputInstant(form.elements.startLocal.value,inputZone),endAt:zonedInputInstant(form.elements.endLocal.value,inputZone)}:{})};
   const request=await lifeClient.submitSchedule(fid,body);if(d.el.isConnected){d.close();observeScheduleRequest(fid,request,onCompleted);}
  }catch(error){if(d.el.isConnected)d.el.querySelector('[data-error]').textContent=errorMessage(error);}
  finally{busy=false;if(d.el.isConnected)showOriginal();}
 };
 return d;
}
