import {requestDecisionText,RequestDecisionSource} from './request-failure.js';
import {scheduleRequestForm} from './schedule-request.js';
import {zonedInput,zonedInputInstant} from '../../zoned-input.mjs';
import {lifeClient} from '../services.js';
import {get,subscribe} from '../store.js';
import {drawer,esc,badge,toast} from '../ui.js';
import {Field} from './forms.js';
import {JsonDetails} from './records.js';
import {IntentionRow} from './cards.js';
import {asyncPage,instant,requestReceipt} from './async-page.js';
import {mountCursorRecords} from './cursor-records.js';
import {governanceCommand} from './governance-command.js';
import {beginDraftSubmission,settleDraftSubmission} from './draft-submission.js';

const drafts=new Map();subscribe(()=>{if(!get('session'))drafts.clear();});
const labels={pending:'已提出，等待判断',processing:'正在判断',awaiting_goal:'等待目标规划',accepted:'已接受，履行待证据',fulfilled:'已履行，有评估证据',missed:'期限已过，尚无履约证据',cancelled:'已取消',paused:'已暂停',blocked:'暂时受阻',deferred:'暂缓决定',rejected:'未接受',failed:'处理失败',legacy_unlinked:'历史提议，执行关联未建立'};
const readOnly=fid=>get('characters',fid)?.status==='已退休';
function canChange(p){return !p.legacy_unlinked&&!['fulfilled','cancelled'].includes(p.lifecycle);}

export function mountPromiseCatalogue(box,{fid,alive,onSaved}){
 box.innerHTML='<h3>约定</h3><div data-promises></div><p class="small muted">提出、接受、实际执行和履约分别记录。履行以约定窗口内的事实及评估为依据。</p>';
 return mountCursorRecords(box.querySelector('[data-promises]'),{alive,label:'约定',read:cursor=>lifeClient.promises(fid,cursor),empty:'还没有约定提议。可通过“和她约定”提出内容及履行期限。',render:p=>`<div class="lrow"><div class="grow"><b>${esc(p.title)}</b><div class="small muted">${esc(labels[p.lifecycle]||p.lifecycle)}${p.window_end?' · '+esc(instant(p.window_end,p.timezone)):''}</div></div><button class="btn sm" data-promise-id="${esc(p.request_id)}">查看约定</button></div>`,bind:list=>list.querySelectorAll('[data-promise-id]').forEach(button=>button.onclick=()=>openPromise(fid,button.dataset.promiseId,onSaved,alive))});
}
export function openPromise(fid,id,onSaved=()=>{},observe=()=>true){
 let dispose;
 const d=drawer({title:'约定与履行证据',body:'<div data-promise-detail></div>',footer:'<button class="btn" data-close>关闭</button>',onClose:()=>dispose?.()});
 d.el.querySelector('[data-close]').onclick=()=>d.close();
 dispose=asyncPage(d.el.querySelector('[data-promise-detail]'),()=>lifeClient.readPromise(fid,id),(p,reload,alive)=>{
  const writable=!readOnly(fid)&&canChange(p),scheduled=(p.scheduled_items||[]).find(item=>item.valid);
  d.el.querySelector('[data-promise-detail]').innerHTML=`<h3>${esc(p.title)}</h3><p>${badge(labels[p.lifecycle]||p.lifecycle)}</p>${p.legacy_unlinked?'<p class="banner warn">这条历史提议没有完整的履行关联。保留原请求及实际结果，不补造约定或履约历史。</p>':`<p>${esc(instant(p.window_start,p.timezone))} — ${esc(instant(p.window_end,p.timezone))}</p><p class="small muted">约定 v${esc(p.revision)} · 期限内发生的真实证据可以稍后入库并由评估确认；到点不会自动成功。</p>`}<p>${esc(requestDecisionText(p.request||{}))}</p>${RequestDecisionSource(p.request)}<div class="row"><button class="btn" data-reload>重新读取</button>${p.goal_id?`<a class="btn" href="#/c/${encodeURIComponent(fid)}/life/goal/${encodeURIComponent(p.goal_id)}">查看目标与证据</a>`:''}<a class="btn" href="#/diag/request/${encodeURIComponent(id)}">原请求与任务</a>${scheduled?`<a class="btn" href="#/c/${encodeURIComponent(fid)}/life/schedule?date=${encodeURIComponent(scheduled.local_date)}">查看已关联日程</a>`:writable&&p.lifecycle==='accepted'&&['candidate','qualified','due'].includes(p.intention?.status)?'<button class="btn" data-arrange>为约定安排时间</button>':''}${writable&&p.intention_id&&!['completed','expired','cancelled','in_progress'].includes(p.intention?.status)?'<button class="btn" data-reschedule>请求改期</button>':''}${writable&&!p.intention_id&&['awaiting_goal','deferred','failed'].includes(p.lifecycle)?'<button class="btn" data-retry>重新请求处理</button>':''}${writable?'<button class="btn danger" data-cancel>取消我的参与</button>':''}</div>${p.intention?IntentionRow(p.intention,{fid}):''}${p.commitment?JsonDetails(p.commitment,'约定的执行窗口与完成条件'):''}${JsonDetails(p.evaluations||[],'履行评估与实际证据')}${(p.planning||[]).filter(run=>run.task_id).map(run=>`<p><a href="#/diag/task/${encodeURIComponent(run.task_id)}">目标规划任务 · ${esc(run.run_status)}</a></p>`).join('')}${JsonDetails(p.planning||[],'Planner请求与处理结果')}<div data-promise-history></div>`;
  d.el.querySelector('[data-reload]').onclick=reload;
  d.el.querySelector('[data-arrange]')?.addEventListener('click',async event=>{const button=event.currentTarget;button.disabled=true;try{const calendar=await lifeClient.schedules(fid);if(!alive())return;const startDate=zonedInput(p.window_start,calendar.timezone).slice(0,10);scheduleRequestForm(fid,{calendar,promise:p,localDate:startDate>calendar.today?startDate:calendar.today,onCompleted:()=>{if(alive()){void reload();void onSaved();}}});}catch(error){if(alive())toast([404,405].includes(error.status)?'当前服务尚未升级约定安排接口，需要运行本次 Core 构建。':error.message,'err');}finally{if(alive())button.disabled=false;}});
  d.el.querySelector('[data-reschedule]')?.addEventListener('click',()=>promiseForm(fid,{promise:p,onSaved:async envelope=>{if(alive()&&observe())requestReceipt(fid,envelope,()=>{void reload();void onSaved();});}}));
  for(const operation of ['cancel','retry'])d.el.querySelector(`[data-${operation}]`)?.addEventListener('click',()=>governanceCommand({scope:[fid,id,'promise',operation],title:operation==='cancel'?'取消这次约定的参与':'重新请求处理约定',revision:p.revision,maxReason:500,note:operation==='cancel'?'撤回你的参与，停止关联意向并保留已发生事实与历史。不会取消她的其他目标，也不撤销已经完成的外部效果。':'重新提交真实处理任务。她仍可拒绝或暂缓，等待规划不等于接受。',snapshot:p,read:()=>lifeClient.readPromise(fid,id),execute:body=>lifeClient.promiseCommand(fid,id,operation,body),successMessage:operation==='cancel'?'已取消参与，历史保留':'处理请求已记录，结果待查询',onSaved:async()=>{await reload();await onSaved();}}));
  const history=mountCursorRecords(d.el.querySelector('[data-promise-history]'),{alive,label:'约定历史',read:cursor=>lifeClient.promiseHistory(fid,id,cursor),render:h=>`<div class="card"><b>${esc(h.operation)}</b> · ${esc(instant(h.occurred_at,p.timezone))}<p>${esc(h.reason)}</p>${JsonDetails({source_ref:h.source_ref,before:h.before_value,after:h.after_value},'范围与版本变更')}</div>`,empty:p.legacy_unlinked?'旧提议保留原请求结果，没有补造修订历史。':'尚无可读取的修订历史。'});
  return()=>history.dispose();
 });return d;
}

export function promiseForm(fid,{promise=null,onSaved=()=>{}}={}){
 if(readOnly(fid))return;
 const key=JSON.stringify([get('session')?.actorId,fid,'promise',promise?.request_id||'new']);
 if(!drafts.has(key))drafts.set(key,{title:promise?.title||'',startsAt:promise?.window_start||'',dueAt:promise?.window_end||'',reason:'',pending:{}});
 const draft=drafts.get(key),value=()=>({title:draft.title,dueAt:draft.dueAt,reason:draft.reason,...(draft.startsAt?{startsAt:draft.startsAt}:{})});
 const d=drawer({title:promise?'请求约定改期':'和她约定',body:`<p>填写最早开始时间（可留空）与最迟履行期限，按北京时间填写。提议需要她判断；受理不等于接受或履行。</p><form class="stack">${!promise?Field({label:'约定内容',name:'title',value:draft.title,attrs:{required:true,maxlength:2000}}):`<p>${esc(promise.title)}</p>`}${Field({label:'最早开始（可选）',name:'startsAt',type:'datetime-local',value:zonedInput(draft.startsAt,'Asia/Shanghai'),hint:'北京时间，可留空'})}${Field({label:'最迟履行期限',name:'dueAt',type:'datetime-local',value:zonedInput(draft.dueAt,'Asia/Shanghai'),attrs:{required:true},hint:'北京时间'})}${!promise?Field({label:'提出原因',name:'reason',value:draft.reason,type:'textarea',attrs:{required:true,maxlength:500}}):''}<p class="small muted" data-time-preview></p><p role="alert" data-error></p><div data-original></div><button class="btn" type="button" data-retry-original hidden>查询或重试原提交</button></form>`,footer:`<button class="btn" data-close>关闭</button><button class="btn primary" data-submit>${promise?'核对新期限并填写理由':'提出约定'}</button>`});
 d.el.querySelector('[data-close]').onclick=()=>d.close();
 const paintTime=()=>{d.el.querySelector('[data-time-preview]').textContent=`${draft.startsAt?instant(draft.startsAt)+' — ':''}${draft.dueAt?instant(draft.dueAt):'请选择履行期限'}`;};
 for(const field of ['title','startsAt','dueAt','reason']){const input=d.el.querySelector(`[name=${field}]`);if(input)input.oninput=()=>{if(['startsAt','dueAt'].includes(field)){try{draft[field]=input.value?zonedInputInstant(input.value,'Asia/Shanghai',draft[field]):'';input.setCustomValidity('');}catch(error){input.setCustomValidity(error.message);}}else draft[field]=input.value;paintTime();};}paintTime();
 const submit=d.el.querySelector('[data-submit]'),errorBox=d.el.querySelector('[data-error]');
 const retryOriginal=d.el.querySelector('[data-retry-original]');
 const settled=(error,saved,unchanged)=>{if(!d.el.isConnected)return;submit.disabled=!!draft.pending.proposal;retryOriginal.disabled=!!draft.pending.proposal;
  if(!promise){try{const original=lifeClient.pendingPromise(fid);retryOriginal.hidden=!original;submit.disabled ||= !!original;d.el.querySelector('[data-original]').innerHTML=original?`<p class="banner warn">原提交的受理结果尚未确认。编辑中的新草稿保持；先查询或重试原提交，使用相同提交身份，不重复创建。</p>${JsonDetails(original.value,'原提交内容')}`:'';}catch(storageError){submit.disabled=true;errorBox.textContent=storageError.message;}}
  if(error)errorBox.textContent=error.message;else if(saved){if(unchanged)d.close();else errorBox.textContent='原提议已受理，处理期间的新草稿已保留。';}};draft.onSettled=settled;settled();
 async function submitProposal(submission,execute){
  draft.onSettled?.();
  try{const result=await execute();if(drafts.get(key)!==draft)return;const ack=settleDraftSubmission(draft,submission,value());if(!ack)return;if(ack.unchanged)drafts.delete(key);draft.onSettled?.(null,result,ack.unchanged);toast('约定提议已受理，结果待查询');try{await onSaved(result);}catch(error){toast(`原提议已受理，读取结果失败：${error.message}`,'err');}}
  catch(error){if(drafts.get(key)===draft){settleDraftSubmission(draft,submission,null);draft.onSettled?.(error);}}
 }
 retryOriginal.onclick=async()=>{if(draft.pending.proposal)return;try{const original=lifeClient.pendingPromise(fid);if(!original)throw new Error('原提交已经结算，请重新读取列表。');await submitProposal(beginDraftSubmission(draft,'proposal',original.value),()=>lifeClient.retryPromise(fid));}catch(error){errorBox.textContent=error.message;}};
 submit.onclick=async()=>{
  if(draft.pending.proposal||!d.el.querySelector('form').reportValidity())return;
  const end=Date.parse(draft.dueAt),start=draft.startsAt?Date.parse(draft.startsAt):null,offset=/(?:Z|[+-]\d{2}:\d{2})$/;
  if(!Number.isFinite(end)||!offset.test(draft.dueAt)||end<=Date.now()||(draft.startsAt&&(!Number.isFinite(start)||!offset.test(draft.startsAt)||end<=start))){errorBox.textContent='请选择北京时间的未来期限，结束时间须晚于开始。';return;}
  if(promise){
   const target={dueAt:draft.dueAt,...(draft.startsAt?{startsAt:draft.startsAt}:{})};d.close();
   governanceCommand({scope:[fid,promise.request_id,'promise-reschedule',target],title:'提出约定改期',revision:promise.revision,maxReason:500,note:'仅请求调整这条约定的履行窗口。她可拒绝或暂缓；实际接受后才改动原执行期限。已完成、已取消或正在执行的意向不能从这里改期。',target,snapshot:promise,read:()=>lifeClient.readPromise(fid,promise.request_id),execute:(body,target)=>lifeClient.reschedulePromise(fid,promise.request_id,{...target,...body}),successMessage:'改期请求已受理，实际结果待查询',onSaved});return;
  }
  const submission=beginDraftSubmission(draft,'proposal',value());await submitProposal(submission,()=>lifeClient.promise(fid,submission.value));
 };
 return d;
}
