import {errorMessage} from '../../domain-presentation.mjs';
import {MissingWorld,EffectSummary,worldRecordLabel,definitionCondition} from './world-presentation.js';
import * as store from '../store.js';
import {mockWorld} from '../services.js';
import {esc,requireOwner,confirm,toast} from '../ui.js';
import {Field} from './forms.js';
import {JsonDetails} from './records.js';
import {mountCursorRecords} from './cursor-records.js';
import {instant} from './async-page.js';
import {studioDraftStorage} from './studio-draft-storage.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {zonedInput,zonedInputInstant} from '../../zoned-input.mjs';
const statusLabels={queued:'已排队，尚未应用',applied:'已应用',filtered:'条件拦截，未应用',failed:'执行失败'};
export function EventDefinitionSummary(d={}){const condition=definitionCondition(d);return `<p>${esc(d.description||'尚无描述')}</p><p class="small muted">${esc(d.trigger?.mode==='automatic'?'自动检查':'受控触发')} · 间隔 ${esc(d.trigger?.interval_minutes??'未配置')} 分钟 · 冷却 ${esc(d.trigger?.cooldown_minutes??'未配置')} 分钟 · 每次自动检查选中概率 ${esc(condition.probability)} · 持续 ${esc(d.duration_minutes??'未配置')} 分钟</p><p class="small muted">${esc(condition.day)} ${esc(condition.time)} · 时间条件按角色时区判断 · ${d.condition?.require_presence?'要求角色实际在场':'环境效果独立判断'} · ${d.condition?.place_ids?.length?'限定已有地点':'整个应用世界'}</p>${EffectSummary(d.effects)}`;}
export function mountWorldEventRuntime(el,ctx,{alive:parentAlive=()=>true}={}){
 const fid=ctx.charId,actor=store.get('session')?.actorId,world=store.get('characters',fid)?.worldId,zone=store.get('characters',fid)?.timezone;
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
 function runRecord(r){return `<div class="card"><a href="${link(selected,r.id)}" data-open-run="${esc(r.id)}"><b>${esc(statusLabels[r.status]||r.status)}</b> · ${esc(instant(r.due_at,zone))}</a><p class="small muted">原执行 ${esc(r.id)} · 模板 v${esc(r.template_revision)} · 执行 v${esc(r.revision)}</p>${r.status==='filtered'?`<p>拦截结果 ${esc(r.result?.reason||r.error_code||'详见服务端判定记录')}</p>`:''}${r.error_code?`<p class="banner err">${esc(errorMessage({message:r.error_code},'事件执行未完成，请展开来源记录核对原因。'))}</p>`:''}<p>恢复状态 ${esc(worldRecordLabel(r.recovery_status))}</p></div>`;}
 function paintTemplate(){
  if(!alive()||!template)return;
  records?.dispose();const draft=triggerDraft(),t=template,root=el.querySelector('[data-template-detail]');let pending=null;try{pending=command().pending();}catch(e){errorText=e.message;}
  conflict=!!pending?.rejection;
  root.innerHTML=`<div class="card stack"><h3>${esc(t.name)} · v${esc(t.revision)}</h3><p>${t.enabled?'已启用':'已关闭'} · 来源 <a href="#/studio/edit/${encodeURIComponent(t.source_resource_id)}">查看创作来源 ${esc(t.source_version)}</a></p>${EventDefinitionSummary(t.definition)}${JsonDetails(t.definition,'完整运行定义')}<p class="small muted">下次检查 ${esc(instant(t.next_check_at,zone))} · 最近触发 ${esc(instant(t.last_triggered_at,zone))}</p>${t.error_code?`<p class="banner err">${esc(errorMessage({message:t.error_code},'模板检查未完成，请重新读取并核对来源。'))}</p>`:''}<label class="toggle"><input data-domain-ui="field" type="checkbox" data-enabled ${t.enabled?'checked':''} ${busy||!writable()?'disabled':''}> 启用运行模板</label><p class="small muted">开关只修改运行设置。关闭页面不会取消已受理执行；排队收据不能代表效果已应用。</p><form data-trigger class="stack">${Field({label:'计划触发时刻（可留空）',name:'scheduledAt',type:'datetime-local',value:zonedInput(draft.scheduledAt,'Asia/Shanghai'),hint:'按北京时间填写；留空时按当前时刻触发'})}<p class="small muted" data-trigger-time>${draft.scheduledAt?esc(instant(draft.scheduledAt)):'留空时按当前时刻触发'}</p>${Field({label:'本次触发原因',name:'reason',value:draft.reason||'',attrs:{required:true,maxlength:500}})}<button data-domain-ui="button" class="btn primary" ${busy||!writable()||!t.enabled?'disabled':''}>提交受控触发</button></form>${pending?`<div class="banner warn"><p>原提交结果未知，请以原内容和幂等身份重试。</p>${JsonDetails(pending.value,'原提交')}<button data-domain-ui="button" class="btn" data-original ${busy||!writable()?'disabled':''}>重试原提交</button></div>`:''}<p data-command-error class="banner err" role="alert" ${errorText?'':'hidden'}>${esc(errorText)}</p>${conflict?'<button data-domain-ui="button" class="btn" data-rebase>保留原操作，明确改用当前版本重试</button>':''}<button data-domain-ui="button" class="btn" data-refresh-template>重读模板与执行</button></div><div class="card"><h3>实际执行历史</h3><div data-run-list></div></div><div data-run-detail></div>`;
  root.querySelector('[data-enabled]').onchange=e=>void submit({operation:'toggle',template:selected,body:{enabled:e.target.checked,expectedRevision:t.revision,reason:'所有者调整事件模板开关'}});
  const readScheduled=f=>f.scheduledAt.value?zonedInputInstant(f.scheduledAt.value,'Asia/Shanghai',draft.scheduledAt||''):'';
  root.querySelector('[data-trigger]').oninput=e=>{const f=e.currentTarget;try{const scheduledAt=readScheduled(f);f.scheduledAt.setCustomValidity('');studioDraftStorage().setItem(commandKey()+':draft',JSON.stringify({scheduledAt,reason:f.reason.value}));root.querySelector('[data-trigger-time]').textContent=scheduledAt?instant(scheduledAt):'留空时按当前时刻触发';}catch(error){f.scheduledAt.setCustomValidity(errorMessage(error));}};
  root.querySelector('[data-trigger]').onsubmit=e=>{e.preventDefault();const f=e.target;if(!f.reportValidity())return;try{const scheduledAt=readScheduled(f);void submit({operation:'trigger',template:selected,body:{expectedRevision:t.revision,reason:f.reason.value,...(scheduledAt?{scheduledAt}:{})}});}catch(error){f.scheduledAt.setCustomValidity(errorMessage(error));f.reportValidity();}};
  root.querySelector('[data-original]')?.addEventListener('click',()=>void submit(pending.value,true));
  root.querySelector('[data-rebase]')?.addEventListener('click',async()=>{
   const tid=selected,version=epoch,original=command(),record=original.pending();if(!record?.rejection)return;
   busy=true;let next=null;
   const same=()=>alive()&&selected===tid&&epoch===version;
   try{
    if(!(await confirm({title:'重设操作版本',text:'原提交已明确被拒绝。读取最新模板与原执行后，按新版本提交相同操作。'}))||!same())return;
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
   box.innerHTML=`<div class="card stack"><h3>${esc(statusLabels[r.status]||r.status)}</h3><p class="small muted">${esc(r.id)} · v${esc(r.revision)} · 原窗口 ${esc(instant(r.due_at,zone))}—${esc(instant(r.expires_at,zone))}</p>${r.status==='queued'?'<p class="banner info">执行已排队，尚无已应用结果。关闭页面后仍会继续处理。</p>':''}${r.error_code?`<p class="banner err">${esc(errorMessage({message:r.error_code},'事件执行未完成，请展开来源记录核对原因。'))}</p>`:''}${EffectSummary(r.result)}${JsonDetails(r.result,'实际判定与效果证据')}<p>恢复状态 ${esc(worldRecordLabel(r.recovery_status))}</p>${JsonDetails(r.recovery_result,'恢复结果与证据')}${r.life_event_id?`<a href="#/c/${encodeURIComponent(fid)}/world/event/${encodeURIComponent(r.life_event_id)}">实际生活事件 ${esc(r.life_event_id)}</a>`:''}${JsonDetails(r,'完整执行来源')}<div class="row"><button data-domain-ui="button" class="btn" data-query>重新查询原执行</button>${r.status==='failed'?`<button data-domain-ui="button" class="btn" data-retry-execution ${!writable()||busy||!template.enabled||r.template_revision!==template.revision||Date.parse(r.expires_at)<=Date.now()?'disabled':''}>重试原窗口失败执行</button>`:''}</div><p class="small muted">只有失败且原执行窗口仍有效、原模板版本仍启用时才可重试。过滤结果不会当作失败重放。</p></div>`;
   box.querySelector('[data-query]').onclick=()=>void readRun();box.querySelector('[data-retry-execution]')?.addEventListener('click',()=>void submit({operation:'retry',template:tid,run:id,body:{expectedRevision:template.revision,expectedRunRevision:r.revision,reason:'所有者重试原窗口内失败的事件执行'}}));
  }catch(error){if(!alive()||version!==detailsEpoch)return;box.innerHTML=`<p role="alert" class="banner err">${esc(errorMessage(error))}</p><button data-domain-ui="button" class="btn" data-query>重新查询</button>`;box.querySelector('[data-query]').onclick=()=>void readRun();}
 }
 async function select(id,run=''){
  const version=++epoch;detailsEpoch++;selected=id;runId=run;errorText='';conflict=false;records?.dispose();updateURL();
  el.querySelector('[data-template-detail]').innerHTML='<p role="status">正在读取运行模板…</p>';
  try{const t=await mockWorld.template(fid,id);if(!alive()||version!==epoch)return;template=t;paintTemplate();}catch(error){if(alive()&&version===epoch){el.querySelector('[data-template-detail]').innerHTML=`<p class="banner err" role="alert">${esc(errorMessage(error))}</p><button data-domain-ui="button" class="btn" data-retry-detail>重新读取</button>`;el.querySelector('[data-retry-detail]').onclick=()=>void select(id,run);}}
 }
 async function load(){
  const version=++epoch;el.innerHTML='<div class="statebox" role="status">正在读取运行模板…</div>';
  try{const page=await mockWorld.templates(fid);if(!alive()||version!==epoch)return;
   el.innerHTML=`<div class="grid g2"><div class="card stack"><h3>世界事件模板</h3><p class="small muted">应用来源、启用状态和实际执行分别记录。环境变化不等于角色已经移动或结识人物。</p>${page.items.map(t=>`<a class="lrow" href="${link(t.id)}" data-select-template="${esc(t.id)}"><span class="grow">${esc(t.name)}</span><span>${t.enabled?'启用':'关闭'} · v${esc(t.revision)}</span></a>`).join('')||'<p class="small muted">尚无已应用模板。请在创作中心保存、发布并应用到当前世界。</p>'}<a data-domain-ui="button" class="btn" href="#/studio?type=${encodeURIComponent('事件模板')}">前往创作中心管理事件模板</a><div class="lockrow">永久性伤害不可启用。</div></div><div class="stack" data-template-detail></div></div>`;
   el.querySelectorAll('[data-select-template]').forEach(a=>a.onclick=e=>{e.preventDefault();if(busy)return;void select(a.dataset.selectTemplate);});if(selected)void select(selected,runId);
  }catch(error){if(alive()&&version===epoch){el.innerHTML=`<div class="statebox"><p role="alert">${esc(errorMessage(error))}</p><button data-domain-ui="button" class="btn" data-load-retry>重新读取</button></div>`;el.querySelector('[data-load-retry]').onclick=()=>void load();}}
 }
 void load();return()=>{disposed=true;epoch++;detailsEpoch++;records?.dispose();};
}
