import { ReadingFields } from './reading-fields.js';
import { readingFields } from '../../../approved-ui/reading-fields.mjs';
import {errorMessage} from '../../domain-presentation.mjs';
import {isInternalId} from '../../internal-id.mjs';
import {userInstant} from '../../presentation-time.mjs';
import {MissingWorld,worldRecordLabel,definitionCondition} from './world-presentation.js';
import {eventEffectLabels} from './event-definition.js';
import * as store from '../store.js';
import {mockWorld} from '../services.js';
import {esc,requireOwner,confirm,toast} from '../ui.js';
import {Field} from './forms.js';
import {RawRecordDownload} from './records.js';
import {mountCursorRecords} from './cursor-records.js';
import {studioDraftStorage} from './studio-draft-storage.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {zonedInput,zonedInputInstant} from '../../zoned-input.mjs';
const statusLabels={queued:'已排队，尚未应用',applied:'已应用',filtered:'条件拦截，未应用',failed:'执行失败'};
const filterReasons={
 execution_window_expired:'执行窗口已结束',
 world_or_character_inactive:'世界或角色当前未处于运行状态',
 template_disabled_or_changed:'模板已停用或运行设置已改变',
 template_changed_or_disabled:'模板已停用或运行设置已改变',
 cooldown_or_active_effect:'仍在冷却，或上次效果仍然有效',
 confirmed_character_presence_required:'需要确认角色实际在场',
 confirmed_presence_required:'需要确认角色实际在场',
 character_target_required:'需要指定角色',
 probability_not_selected:'本次自动检查未选中',
 day_condition_not_met:'日期条件未满足',
 time_condition_not_met:'时段条件未满足',
 place_condition_not_met:'地点条件未满足',
};
const templateStatus=enabled=>enabled===true?'启用':enabled===false?'停用':'';
const runStatus=status=>Object.hasOwn(statusLabels,status)?statusLabels[status]:'执行状态待确认';
const download=record=>RawRecordDownload(record,'原始世界事件记录（诊断）');
const displayTime=value=>typeof value==='string'&&Number.isFinite(Date.parse(value))?userInstant(value):'';
const fact=(label,value)=>value?`${label} ${value}`:'';
const timeFact=(label,value)=>fact(label,displayTime(value));
function line(...facts){
 const text=facts.filter(Boolean).map(esc).join(' · ');
 return text?`<p class="small muted">${text}</p>`:'';
}
const minutes=(label,value)=>Number.isFinite(value)?`${label} ${value} 分钟`:'';
function readableText(value){
 if(typeof value!=='string')return '';
 return value.trim().replace(/[a-z0-9_-]+/gi,token=>
  isInternalId(token)||/^(?:generated_template|life_event|npc)_[a-z0-9_-]+$/i.test(token)?'':token).trim();
}
function eventName(t){
 const name=t.name?.trim();
 return (name!==t.id&&name!==t.template_id&&name!==t.source_resource_id&&readableText(name))||'未命名的事件模板';
}
const bodyEffectShape = readingFields("body_part severity impact");

function effectSummary(effects){
 if(!Array.isArray(effects)||!effects.length)return '';
 return `<ul>${effects.map(e=>{
  const kind=Object.hasOwn(eventEffectLabels,e.kind)?eventEffectLabels[e.kind]:'世界变化';
  const scope=['world','place','character'].includes(e.scope)?worldRecordLabel(e.scope):'';
  const value=e.value==='open'||e.value==='closed'?worldRecordLabel(e.value):readableText(e.value)||readableText(e.description);
  const details=[...new Set([value,readableText(e.body_effect?.description)].filter(Boolean))].join(' · ');
  return `<li>${esc([kind,scope].filter(Boolean).join(' · '))}${details?`：${esc(details)}`:''}${ReadingFields(e.body_effect,bodyEffectShape)}</li>`;
 }).join('')}</ul>`;
}
export function EventDefinitionSummary(d){
 if(!d)return '';
 const {trigger,condition:c}=d;
 const condition=definitionCondition(d);
 const hasTime=/^\d{2}:\d{2}$/.test(c?.start)&&/^\d{2}:\d{2}$/.test(c?.end);
 const category={environment:'环境变化',visitor:'人物到访',shop:'商店变化',encounter:'偶遇',temporary_state:'临时状态'};
 const intensity={low:'低',medium:'中'};
 return `${line(readableText(d.description))}
  ${line(Object.hasOwn(category,d.category)&&category[d.category],Object.hasOwn(intensity,d.intensity)&&fact('强度',intensity[d.intensity]))}
  ${line(trigger?.mode==='automatic'?'自动检查':trigger?.mode==='manual'?'受控触发':'',
   minutes('间隔',trigger?.interval_minutes),minutes('冷却',trigger?.cooldown_minutes),
   Number.isFinite(trigger?.probability)&&fact('每次自动检查选中概率',condition.probability),minutes('持续',d.duration_minutes))}
  ${line(['daily','weekday','weekend'].includes(c?.day)&&condition.day,hasTime&&condition.time,hasTime&&'时间条件按角色时区判断',
   typeof c?.require_presence==='boolean'&&(c.require_presence?'要求角色实际在场':'环境效果独立判断'),
   Array.isArray(c?.place_ids)&&(c.place_ids.length?'限定已有地点':'整个应用世界'))}
  ${effectSummary(d.effects)}`;
}
function runOutcome(r){
 const reason=r.result?.reason;
 const filter=Object.hasOwn(filterReasons,reason)?filterReasons[reason]
  :typeof reason==='string'&&!/^[a-z0-9]+(?:[_.:-][a-z0-9_-]+)+$/i.test(reason.trim())?readableText(reason):'';
 return `${r.status==='filtered'?line(fact('拦截原因',filter)):''}
  ${r.error_code?`<p data-domain-ui="alert" class="banner err">${esc(errorMessage({message:r.error_code},'事件执行未完成，可下载原始记录核对原因。'))}</p>`:''}
  ${['not_required','pending','completed','failed','recovered','scheduled'].includes(r.recovery_status)
   ?line(fact('恢复状态',r.recovery_status==='completed'?'已恢复':worldRecordLabel(r.recovery_status))):''}`;
}
function pendingSummary(pending){
 const {operation,body}=pending.value;
 const action=operation==='toggle'?(body?.enabled?'启用运行模板':'停用运行模板')
  :operation==='trigger'?'受控触发':operation==='retry'?'重试原执行':'';
 return line(action,timeFact('计划触发',body?.scheduledAt),fact('原因',readableText(body?.reason)));
}
export function mountWorldEventRuntime(el,ctx,{alive:parentAlive=()=>true}={}){
 const fid=ctx.charId,actor=store.get('session')?.actorId,world=store.get('characters',fid)?.worldId;
 let disposed=false,epoch=0,detailsEpoch=0,records=null,selected=ctx.query.template||'',runId=ctx.query.run||'',template=null,busy=false,errorText='',conflict=false;
 const alive=()=>!disposed&&parentAlive()&&el.isConnected&&store.get('session')?.actorId===actor&&store.get('characters',fid)?.worldId===world;
 if(!world){el.innerHTML=MissingWorld(fid);return()=>{disposed=true;};}
 const writable=()=>store.get('characters',fid)?.status!=='已退休';
 const commandKey=()=>`fluctlight-world-event:${actor}:${fid}:${world}:${selected}`;
 const triggerDraft=()=>JSON.parse(studioDraftStorage().getItem(commandKey()+':draft')||'{}');
 const command=()=>persistentCommand({key:commandKey(),storage:()=>studioDraftStorage(),execute:async(value,key)=>{
  if(!alive()||!writable())throw new Error('角色范围已切换或角色已退休，请回到原范围核对请求');
  return mockWorld.eventCommand(fid,value.template,value.operation,{...value.body,idempotencyKey:key},value.run);
 }});
 const link=(tid,rid='')=>`#/c/${encodeURIComponent(fid)}/world/settings?template=${encodeURIComponent(tid)}${rid?`&run=${encodeURIComponent(rid)}`:''}`;
 const updateURL=()=>window.history.replaceState(null,'',link(selected,runId));
 function runRecord(r){
  const due=displayTime(r.due_at);
  return `<div data-domain-ui="card" class="card">
   <a href="${link(selected,r.id)}" data-open-run="${esc(r.id)}"><b>${esc(runStatus(r.status))}</b>${due?' · '+esc(due):''}</a>
   ${runOutcome(r)}
  </div>`;
 }
 function paintTemplate(){
  if(!alive()||!template)return;
  records?.dispose();const draft=triggerDraft(),t=template,root=el.querySelector('[data-template-detail]');let pending=null;try{pending=command().pending();}catch(e){errorText=e.message;}
  conflict=!!pending?.rejection;
  const source=t.source_resource_id?`<a href="#/studio/edit/${encodeURIComponent(t.source_resource_id)}">查看创作来源</a>`:'';
  root.innerHTML=`<div data-domain-ui="card" class="card stack">
   <h3>${esc(eventName(t))}</h3>
   ${line(templateStatus(t.enabled))}
   ${source}
   ${EventDefinitionSummary(t.definition)}
   ${line(timeFact('下次检查',t.next_check_at),timeFact('最近检查',t.last_checked_at),timeFact('最近触发',t.last_triggered_at))}
   ${t.error_code?`<p data-domain-ui="alert" class="banner err">${esc(errorMessage({message:t.error_code},'模板检查未完成，可下载原始记录核对原因。'))}</p>`:''}
   ${download(t)}
   <label class="toggle"><input data-domain-ui="field" type="checkbox" data-enabled ${t.enabled?'checked':''} ${busy||!writable()?'disabled':''}> 启用运行模板</label>
   <p class="small muted">开关只修改运行设置。关闭页面不会取消已受理执行；排队收据不能代表效果已应用。</p>
   <form data-trigger class="stack">
    ${Field({label:'计划触发时刻（可留空）',name:'scheduledAt',type:'datetime-local',value:zonedInput(draft.scheduledAt,'Asia/Shanghai'),hint:'按北京时间填写；留空时按当前时刻触发'})}
    <p class="small muted" data-trigger-time>${draft.scheduledAt?esc(displayTime(draft.scheduledAt)):'留空时按当前时刻触发'}</p>
    ${Field({label:'本次触发原因',name:'reason',value:draft.reason||'',attrs:{required:true,maxlength:500}})}
    <button data-domain-ui="button" data-domain-variant="default" data-domain-size="default" ${busy||!writable()||!t.enabled?'disabled':''}>提交受控触发</button>
   </form>
   ${pending?`<div data-domain-ui="alert" class="banner warn">
    <p>${conflict?'原提交已被拒绝，请按最新设置重试。':'原提交结果未知，请重试同一笔提交。'}</p>
    ${pendingSummary(pending)}
    ${download(pending)}
    <button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-original ${busy||!writable()?'disabled':''}>重试原提交</button>
   </div>`:''}
   <p data-domain-ui="alert" data-command-error class="banner err" role="alert" ${errorText?'':'hidden'}>${esc(errorText)}</p>
   ${conflict?'<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-rebase>保留原操作，按最新设置重试</button>':''}
   <button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-refresh-template>重读模板与执行</button>
  </div><div data-domain-ui="card" class="card"><h3>实际执行历史</h3><div data-run-list></div></div><div data-run-detail></div>`;
  root.querySelector('[data-enabled]').onchange=e=>void submit({operation:'toggle',template:selected,body:{enabled:e.target.checked,expectedRevision:t.revision,reason:'所有者调整事件模板开关'}});
  const readScheduled=f=>f.scheduledAt.value?zonedInputInstant(f.scheduledAt.value,'Asia/Shanghai',draft.scheduledAt||''):'';
  root.querySelector('[data-trigger]').oninput=e=>{const f=e.currentTarget;try{const scheduledAt=readScheduled(f);f.scheduledAt.setCustomValidity('');studioDraftStorage().setItem(commandKey()+':draft',JSON.stringify({scheduledAt,reason:f.reason.value}));root.querySelector('[data-trigger-time]').textContent=scheduledAt?displayTime(scheduledAt):'留空时按当前时刻触发';}catch(error){f.scheduledAt.setCustomValidity(errorMessage(error));}};
  root.querySelector('[data-trigger]').onsubmit=e=>{e.preventDefault();const f=e.target;if(!f.reportValidity())return;try{const scheduledAt=readScheduled(f);void submit({operation:'trigger',template:selected,body:{expectedRevision:t.revision,reason:f.reason.value,...(scheduledAt?{scheduledAt}:{})}});}catch(error){f.scheduledAt.setCustomValidity(errorMessage(error));f.reportValidity();}};
  root.querySelector('[data-original]')?.addEventListener('click',()=>void submit(pending.value,true));
  root.querySelector('[data-rebase]')?.addEventListener('click',async()=>{
   const tid=selected,version=epoch,original=command(),record=original.pending();if(!record?.rejection)return;
   busy=true;let next=null;
   const same=()=>alive()&&selected===tid&&epoch===version;
   try{
    if(!(await confirm({title:'按最新设置重试',text:'原提交已明确被拒绝。读取最新模板与原执行后，重新提交相同操作。'}))||!same())return;
    const latest=await mockWorld.template(fid,tid);if(!same())return;
    const run=record.value.run?await mockWorld.eventRun(fid,tid,record.value.run):null;if(!same())return;
    record.value.body.expectedRevision=latest.revision;if(run)record.value.body.expectedRunRevision=run.revision;
    original.releaseRejected();next=record.value;conflict=false;
   }catch(error){if(same())errorText=errorMessage(error);}
   finally{busy=false;if(same()){if(next)void submit(next);else paintTemplate();}}
  });
  root.querySelector('[data-refresh-template]').onclick=()=>void select(selected,runId);
  records=mountCursorRecords(root.querySelector('[data-run-list]'),{alive,read:cursor=>mockWorld.eventRuns(fid,selected,cursor),render:runRecord,label:'执行记录',bind:box=>box.querySelectorAll('[data-open-run]').forEach(a=>a.onclick=e=>{e.preventDefault();runId=a.dataset.openRun;updateURL();void readRun();})});
  if(runId)void readRun();
 }
 async function submit(value,retry=false){
  const tid=selected,original=command();
  if(busy||value.template!==tid||!alive()||!writable()||!(await requireOwner())||!alive()||selected!==tid)return;
  busy=true;errorText='';paintTemplate();
  try{const r=retry?await original.retry():await original.submit(value);if(!alive())return;conflict=false;if(value.operation!=='toggle'){runId=r.id;updateURL();}template=await mockWorld.template(fid,selected);if(!alive())return;toast(value.operation==='toggle'?'运行开关已保存并读回':'请求已受理，请查询实际执行结果','ok');}
  catch(error){if(alive()){errorText=errorMessage(error);conflict=!!command().pending()?.rejection;}}
  finally{busy=false;if(alive())paintTemplate();}
 }
 async function readRun(){
  const version=++detailsEpoch,tid=selected,id=runId,box=el.querySelector('[data-run-detail]');if(!box||!id)return;
  box.innerHTML='<p role="status">正在查询原执行…</p>';
  try{const r=await mockWorld.eventRun(fid,tid,id);if(!alive()||version!==detailsEpoch||tid!==selected)return;
   const recovery=r.recovery_result;
   box.innerHTML=`<div data-domain-ui="card" class="card stack">
    <h3>${esc(runStatus(r.status))}</h3>
    ${line(timeFact('窗口开始',r.due_at),timeFact('窗口截止',r.expires_at))}
    ${line(timeFact(r.status==='applied'?'实际开始':'处理时间',r.result?.actual_start_at||r.started_at),
     timeFact('实际结束',r.result?.actual_end_at||r.end_at))}
    ${r.status==='queued'?'<p data-domain-ui="alert" class="banner info">执行已排队，尚无已应用结果。关闭页面后仍会继续处理。</p>':''}
    ${runOutcome(r)}
    ${effectSummary(r.result?.effects)}
    ${line(recovery?.status==='already_resolved'&&'原效果已解除',
     timeFact('恢复时间',recovery?.start_at),timeFact('最近恢复尝试',recovery?.last_attempt_at))}
    ${recovery?.error_code?'<p data-domain-ui="alert" class="banner err">上次恢复尝试失败，可下载原始记录核对原因。</p>':''}
    ${r.life_event_id?`<a href="#/c/${encodeURIComponent(fid)}/world/event/${encodeURIComponent(r.life_event_id)}">查看实际生活事件</a>`:''}
    ${download(r)}
    <div class="row">
     <button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-query>重新查询原执行</button>
     ${r.status==='failed'?`<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-retry-execution ${!writable()||busy||!template.enabled||r.template_revision!==template.revision||Date.parse(r.expires_at)<=Date.now()?'disabled':''}>重试原窗口失败执行</button>`:''}
    </div>
    <p class="small muted">只有失败且原执行窗口仍有效、原模板仍启用且运行设置未变更时才可重试。过滤结果不会当作失败重放。</p>
   </div>`;
   box.querySelector('[data-query]').onclick=()=>void readRun();box.querySelector('[data-retry-execution]')?.addEventListener('click',()=>void submit({operation:'retry',template:tid,run:id,body:{expectedRevision:template.revision,expectedRunRevision:r.revision,reason:'所有者重试原窗口内失败的事件执行'}}));
  }catch(error){if(!alive()||version!==detailsEpoch)return;box.innerHTML=`<p data-domain-ui="alert" role="alert" class="banner err">${esc(errorMessage(error))}</p><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-query>重新查询</button>`;box.querySelector('[data-query]').onclick=()=>void readRun();}
 }
 async function select(id,run=''){
  const version=++epoch;detailsEpoch++;selected=id;runId=run;errorText='';conflict=false;records?.dispose();updateURL();
  el.querySelector('[data-template-detail]').innerHTML='<p role="status" data-domain-ui="skeleton">正在读取运行模板…</p>';
  try{const t=await mockWorld.template(fid,id);if(!alive()||version!==epoch)return;template=t;paintTemplate();}catch(error){if(alive()&&version===epoch){el.querySelector('[data-template-detail]').innerHTML=`<p data-domain-ui="alert" class="banner err" role="alert">${esc(errorMessage(error))}</p><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-retry-detail>重新读取</button>`;el.querySelector('[data-retry-detail]').onclick=()=>void select(id,run);}}
 }
 async function load(){
  const version=++epoch;el.innerHTML='<div class="statebox" role="status" data-domain-ui="skeleton">正在读取运行模板…</div>';
  try{const page=await mockWorld.templates(fid);if(!alive()||version!==epoch)return;
   const templates=page.items.map(t=>`<a data-domain-ui="item" class="lrow" href="${link(t.id)}" data-select-template="${esc(t.id)}">
    <span class="grow">${esc(eventName(t))}</span><span>${templateStatus(t.enabled)}</span>
   </a>`).join('');
   el.innerHTML=`<div class="grid g2"><div data-domain-ui="card" class="card stack">
    <h3>世界事件模板</h3>
    <p class="small muted">应用来源、启用状态和实际执行分别记录。环境变化不等于角色已经移动或结识人物。</p>
    ${templates||'<p class="small muted">尚无已应用模板。请在创作中心保存、发布并应用到当前世界。</p>'}
    <a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/studio?type=${encodeURIComponent('事件模板')}">前往创作中心管理事件模板</a>
    <div class="lockrow">永久性伤害不可启用。</div>
   </div><div class="stack" data-template-detail></div></div>`;
   el.querySelectorAll('[data-select-template]').forEach(a=>a.onclick=e=>{e.preventDefault();if(busy)return;void select(a.dataset.selectTemplate);});if(selected)void select(selected,runId);
  }catch(error){if(alive()&&version===epoch){el.innerHTML=`<div data-domain-ui="empty" class="statebox"><p role="alert">${esc(errorMessage(error))}</p><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-load-retry>重新读取</button></div>`;el.querySelector('[data-load-retry]').onclick=()=>void load();}}
 }
 void load();return()=>{disposed=true;epoch++;detailsEpoch++;records?.dispose();};
}
