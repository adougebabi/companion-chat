import {autonomyClient} from '../services.js';
import * as store from '../store.js';
import {asyncPage} from '../components/async-page.js';
import {Field} from '../components/forms.js';
import {AutonomyFields} from '../components/autonomy-fields.js';
import {JsonDetails} from '../components/records.js';
import {ConflictPanel} from '../components/revisions.js';
import {beginDraftSubmission,settleDraftSubmission} from '../components/draft-submission.js';
import {esc,badge,confirm} from '../ui.js';

const drafts=new Map();
store.subscribe(()=>{if(!store.get('session'))drafts.clear();});
const fieldsFrom=p=>({policySource:p.policy_source,mode:p.mode,allowedActions:[...p.allowed_actions],budgetRemaining:p.budget_remaining,quietHours:{...p.quiet_hours},concurrencyLimit:p.concurrency_limit,cooldownSeconds:p.cooldown_seconds,outreachPaused:p.outreach_paused,notificationsMuted:p.notifications_muted,confirmActionTypes:[...p.confirm_action_types],reason:''});

export function renderRealAutonomy(el,ctx){
 const fid=ctx.charId,key=JSON.stringify([store.get('session')?.actorId,fid]);
 return asyncPage(el,()=>autonomyClient.read(fid),(saved,reload,alive)=>{
  if(!drafts.has(key))drafts.set(key,{values:fieldsFrom(saved),base:saved.revision,dirty:false,pending:{}});
  const draft=drafts.get(key);
  if(!draft.dirty&&!draft.pending.policy&&saved.revision>=draft.base)Object.assign(draft,{values:fieldsFrom(saved),base:saved.revision});
  const v=draft.values,conflict=draft.base<saved.revision;
  el.innerHTML=`<div class="grid g2 autonomy-page" style="align-items:start"><form class="card stack" data-policy><h3>${esc(store.get('characters',fid)?.name||'该角色')}的自治与触达 ${badge(saved.policy_source==='inherit'?'继承全局':'角色覆盖')}</h3>${conflict?ConflictPanel({baseVersion:draft.base,currentVersion:saved.revision}):''}${Field({label:'策略来源',name:'policySource',type:'select',options:[{value:'inherit',label:'继承全局策略'},{value:'override',label:'此角色独立覆盖'}],value:v.policySource})}<div class="stack autonomy-fields" data-config-fields>${AutonomyFields(v)}</div>${Field({label:'剩余自治行动预算（次）',name:'budgetRemaining',type:'number',value:v.budgetRemaining,attrs:{min:0,max:1000000,step:1,required:true}})}<p class="small muted">每个新的自治运行首次执行外部行动扣 1 次；同一运行后续工具复用预留。此处不是每日预算、模型 Token 或虚拟货币。仅修改本值时调整实际余额。</p>${Field({label:'修改原因',name:'reason',type:'textarea',value:v.reason,attrs:{required:true,maxlength:1000}})}<div class="row"><button type="button" class="btn" data-reset>撤销草稿</button><button class="btn primary" ${conflict||draft.pending.policy?'disabled':''}>保存并生效</button><button type="button" class="btn" data-reload>重新读取</button></div><p role="alert" data-feedback></p></form><div class="stack"><div class="card"><h3>当前执行边界</h3><p>静音只改变新通知的提醒计数，记录保留。暂停主动联系只阻止主动消息；暂停自治阻止新外部行动，明确交流和内部生活仍可继续。</p><p>要求确认的类别会阻止自主执行；通过具体业务的 Owner 指令执行。保存策略不会重放已经拒绝或失败的操作，恢复后按当前事实重新判断。</p><p>已提交的事实和已成功的副作用保留。未提交的工具会在实际写入前重新核对策略。</p>${JsonDetails(saved,'服务端实际有效策略与版本')}${JsonDetails(saved.global_policy,'全局策略')}${Object.keys(saved.legacy_preferences||{}).length?JsonDetails(saved.legacy_preferences,'旧版偏好记录（保留原始来源）'):''}</div><a class="btn" href="#/diag/pending?charId=${encodeURIComponent(fid)}">该角色待处理事项</a></div></div>`;
  const form=el.querySelector('[data-policy]');
  function update(){const f=form.elements;Object.assign(v,{policySource:f.policySource.value,mode:f.mode.value,outreachPaused:f.outreachPaused.checked,notificationsMuted:f.notificationsMuted.checked,allowedActions:[...form.querySelectorAll('[data-allowed]:checked')].map(i=>i.dataset.allowed),confirmActionTypes:[...form.querySelectorAll('[data-confirm]:checked')].map(i=>i.dataset.confirm),concurrencyLimit:Number(f.concurrencyLimit.value),cooldownSeconds:Number(f.cooldownSeconds.value),budgetRemaining:Number(f.budgetRemaining.value),quietHours:f.quietEnabled.checked?{start:f.quietStart.value,end:f.quietEnd.value,timezone:f.quietTimezone.value.trim()}:{},reason:f.reason.value});draft.dirty=true;disableInherited();}
  function disableInherited(){el.querySelectorAll('[data-config-fields] input,[data-config-fields] select').forEach(input=>input.disabled=v.policySource==='inherit');}
  form.addEventListener('input',update);form.addEventListener('change',update);disableInherited();
  el.querySelector('[data-reset]').onclick=()=>{Object.assign(draft,{values:fieldsFrom(saved),dirty:false});void reload();};el.querySelector('[data-reload]').onclick=reload;
  el.querySelector('[data-keep]')?.addEventListener('click',()=>{el.querySelector('[data-feedback]').textContent='草稿已保留。';});
  el.querySelector('[data-refresh]')?.addEventListener('click',async()=>{if(await confirm({title:'采用最新版本作为保存基线',text:'草稿字段保留。请先核对右侧最新实际策略和余额；下一次保存会按新版本提交。',identity:'所有者修改'})&&alive()){draft.base=saved.revision;void reload();}});
  const settled=error=>{
   if(!alive())return;
   el.querySelector('[data-feedback]').textContent=error?.message||(draft.dirty?'本次提交已保存，期间编辑的新草稿保留。':'策略已保存');
   form.querySelector('.primary').disabled=false;
   if(error?.status===409||(!error&&!draft.dirty))void reload();
  };
  draft.onSettled=settled;
  form.onsubmit=async event=>{
   event.preventDefault();if(draft.pending.policy)return;update();
   const submission=beginDraftSubmission(draft,'policy',draft.values),body={...submission.value,expectedRevision:draft.base};
   if(body.budgetRemaining===saved.budget_remaining)delete body.budgetRemaining;
   if(body.policySource==='inherit')for(const name of ['mode','allowedActions','quietHours','concurrencyLimit','cooldownSeconds','outreachPaused','notificationsMuted','confirmActionTypes'])delete body[name];
   form.querySelector('.primary').disabled=true;
   try{
    const response=await autonomyClient.save(fid,body);if(drafts.get(key)!==draft)return;
    const result=settleDraftSubmission(draft,submission,draft.values);if(result){draft.base=Math.max(draft.base,response.revision);draft.dirty=!result.unchanged;draft.onSettled?.();}
   }catch(error){if(drafts.get(key)!==draft)return;settleDraftSubmission(draft,submission,null);draft.dirty=true;draft.onSettled?.(error);}
  };
  return()=>{if(draft.onSettled===settled)delete draft.onSettled;};
 });
}
