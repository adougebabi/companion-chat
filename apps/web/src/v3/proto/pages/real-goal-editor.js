import {errorMessage} from '../../domain-presentation.mjs';
import { randomId } from '../../../random-id.mjs';
import { lifeClient } from '../services.js';
import { Field } from '../components/forms.js';
import {GoalRecords as RecordDetails} from '../components/goal-records.js';
import {stateLabel} from '../../domain-presentation.mjs';
import {JsonDetails} from '../components/records.js';
import {userInstant} from '../../presentation-time.mjs';
import {zonedInput,zonedInputInstant} from '../../zoned-input.mjs';
import { ConflictPanel } from '../components/revisions.js';
import { esc,badge,toast,requireOwner,confirm } from '../ui.js';

export function renderRealGoalEditor(el,ctx) {
  let alive=true,id=ctx.params.goalId || null,current,draft,busy=false,conflict=false,accepted=null;
  let editOpen=!id;
  let history={items:[],next_cursor:''},evidence={items:[],next_cursor:''},key=randomId();
  const fromGoal=g=>({title:g?.desired_outcome || '',why:g?.motivation || '',criteria:(g?.success_criteria || []).join('\n'),due:g?.deadline || '',deadlinePolicy:g?.deadline_policy || 'soft',scope:g?.scope || 'general',profileId:g?.profile_id || '',targetActorId:g?.target_actor_id || '',ownerProtected:!!g?.owner_protected,candidateOnly:g?.status==='candidate',reason:'',version:g?.revision ?? 0});
  async function read() {
    el.innerHTML='<div class="statebox" role="status">正在读取目标…</div>';
    try{if(id){[current,history,evidence]=await Promise.all([lifeClient.goal(ctx.charId,id),lifeClient.goalHistory(ctx.charId,id),lifeClient.goalEvidence(ctx.charId,id)]);}draft=fromGoal(current);paint();}
    catch(error){if(alive)el.innerHTML=`<div class="banner err" role="alert">${esc(errorMessage(error))}</div><a class="btn" href="#/c/${ctx.charId}/life/goals">返回目标列表</a>`;}
  }
  function paint() {
    if(!alive)return;
    el.innerHTML=`<div class="row" style="margin-bottom:12px"><a class="btn" href="#/c/${ctx.charId}/life/goals">目标列表</a><a class="btn" href="#/c/${ctx.charId}/life/planning">独立规划</a></div>${current?`<section class="card goal-reading"><div class="row"><h2 class="grow">${esc(current.desired_outcome)}</h2>${badge(stateLabel(current.status))}</div>${current.success_criteria?.length?`<h3>怎样算完成</h3><ul>${current.success_criteria.map(value=>`<li>${esc(value)}</li>`).join('')}</ul>`:''}${current.deadline?`<p class="small muted">期望完成时间：${esc(userInstant(current.deadline))}</p>`:''}<details><summary>动机与说明</summary><p>${esc(current.motivation||'尚未补充说明')}</p></details></section>`:''}<div class="grid g2 goal-detail-layout"><details class="goal-edit-panel" ${editOpen||conflict?'open':''}><summary>${id?'编辑目标':'填写新目标'}</summary><form class="card stack" data-form><h2>${id?'修改目标资料':'新建目标'}</h2>${conflict?ConflictPanel({baseVersion:draft.version,currentVersion:current.revision}):''}${Field({label:'标题',name:'title',value:draft.title,attrs:{required:true,maxlength:4000}})}${Field({label:'动机与描述',name:'why',value:draft.why,type:'textarea',attrs:{required:true}})}${Field({label:'完成条件（每行一项）',name:'criteria',value:draft.criteria,type:'textarea',attrs:{required:true}})}${Field({label:'截止时间（可留空）',name:'due',type:'datetime-local',value:zonedInput(draft.due,'Asia/Shanghai'),hint:'按北京时间填写'})}<p class="small muted" data-deadline-preview>${draft.due?esc(userInstant(draft.due)):'未设置截止时间'}</p>${Field({label:'截止策略',name:'deadlinePolicy',value:draft.deadlinePolicy,type:'select',options:[{value:'soft',label:'软期限'},{value:'hard',label:'硬期限'}]})}<details><summary>范围与保护</summary>${Field({label:'作用范围',name:'scope',value:draft.scope,type:'select',options:[{value:'general',label:'一般目标'},{value:'relationship',label:'关系目标'}]})}${Field({label:'人格 ID',name:'profileId',value:draft.profileId})}${Field({label:'相关 Actor ID',name:'targetActorId',value:draft.targetActorId})}<label class="toggle"><input name="ownerProtected" type="checkbox" ${draft.ownerProtected?'checked':''}> 所有者保护目标</label>${!id?`<label class="toggle"><input name="candidateOnly" type="checkbox" ${draft.candidateOnly?'checked':''}> 先作为候选</label>`:''}</details>${Field({label:'修改原因',name:'reason',value:draft.reason,type:'textarea',attrs:{required:true}})}<div class="row"><button class="btn primary" ${busy?'disabled':''}>保存目标</button><button type="button" class="btn" data-revert>撤销更改</button></div><p data-error role="alert"></p></form></details><div class="stack"><div class="card"><h3>状态与来源</h3>${badge(stateLabel(current?.status,'尚未提交'))}<p class="small muted">${id?`资料版本 ${esc(current.revision)}`:'创建后的状态由领域服务决定，不自动宣称完成。'}</p>${id?`<div class="row">${['暂停','恢复','取消','放弃','重新评估'].map((label,index)=>`<button class="btn sm" data-command="${['pause','resume','cancel','abandon','reassess'][index]}">${label}</button>`).join('')}</div>`:''}${current?JsonDetails(current,'完整目标与来源'):''}</div><div class="card"><h3>证据与修订</h3><p class="small muted">目标是否完成取决于证据评估。此处不提供直接标记完成。</p><h4>证据</h4>${RecordDetails(evidence.items)}${evidence.next_cursor?'<button class="btn sm" data-more="evidence">加载更多证据</button>':''}<h4>修订</h4>${RecordDetails(history.items)}${history.next_cursor?'<button class="btn sm" data-more="history">加载更多修订</button>':''}</div></div></div>`;
    const form=el.querySelector('form'),panel=el.querySelector('.goal-edit-panel');panel.ontoggle=()=>{editOpen=panel.open;};
    const capture=()=>{for(const input of form.querySelectorAll('[name]')){if(input.name==='due'){try{draft.due=input.value?zonedInputInstant(input.value,'Asia/Shanghai',draft.due):'';input.setCustomValidity('');}catch(error){input.setCustomValidity(errorMessage(error));}}else draft[input.name]=input.type==='checkbox'?input.checked:input.value;}el.querySelector('[data-deadline-preview]').textContent=draft.due?userInstant(draft.due):'未设置截止时间';};
    form.oninput=form.onchange=capture;
    form.onsubmit=async event=>{event.preventDefault();capture();if(!form.reportValidity()||busy || !(await requireOwner()))return;busy=true;
      try {
        if(!accepted){const body={idempotencyKey:key,reason:draft.reason,desiredOutcome:draft.title,motivation:draft.why,successCriteria:draft.criteria.split('\n').map(t=>t.trim()).filter(Boolean),deadlinePolicy:draft.deadlinePolicy,scope:draft.scope,ownerProtected:draft.ownerProtected,...(draft.profileId?{profileId:draft.profileId}:{}),...(draft.targetActorId?{targetActorId:draft.targetActorId}:{}),...(draft.due?{deadline:draft.due}:{clearDeadline:true}),...(id?{expectedRevision:draft.version}:{candidateOnly:draft.candidateOnly})};accepted=await lifeClient.saveGoal(ctx.charId,id,body);id=accepted.goal_id;}
        const saved=await lifeClient.goal(ctx.charId,id);if(!alive)return;
        if(saved.desired_outcome!==draft.title || saved.motivation!==draft.why)throw new Error('命令已受理，但读回内容尚未确认一致。请重新读取。');
        current=saved;draft=fromGoal(saved);accepted=null;conflict=false;key=randomId();toast('目标已保存并读回','ok');
        if(!ctx.params.goalId){ctx.navigate(`#/c/${ctx.charId}/life/goal/${id}`);return;}
        [history,evidence]=await Promise.all([lifeClient.goalHistory(ctx.charId,id),lifeClient.goalEvidence(ctx.charId,id)]);paint();
      } catch(error) {if(!alive)return;if(error.status===409&&id){current=await lifeClient.goal(ctx.charId,id);conflict=true;paint();}el.querySelector('[data-error]').textContent=errorMessage(error);}finally{busy=false;}
    };
    el.querySelector('[data-revert]').onclick=()=>{draft=fromGoal(current);conflict=false;key=randomId();paint();};
    el.querySelector('[data-keep]')?.addEventListener('click',()=>toast('草稿已保留'));
    el.querySelector('[data-refresh]')?.addEventListener('click',()=>{draft.version=current.revision;conflict=false;key=randomId();paint();});
    el.querySelectorAll('[data-command]').forEach(button=>button.onclick=async()=>{capture();if(busy || !(await requireOwner()))return;if(!draft.reason.trim()){editOpen=true;panel.open=true;form.elements.reason.focus();return toast('请在编辑目标中填写本次操作原因','err');}if(!(await confirm({title:'调整目标',text:esc(draft.title)})))return;busy=true;try{await lifeClient.goalCommand(ctx.charId,id,button.dataset.command,current.revision,draft.reason);await read();}catch(error){if(alive)el.querySelector('[data-error]').textContent=errorMessage(error);}finally{busy=false;}});
    el.querySelectorAll('[data-more]').forEach(button=>button.onclick=async()=>{capture();button.disabled=true;try{const kind=button.dataset.more,old=kind==='history'?history:evidence,next=await (kind==='history'?lifeClient.goalHistory(ctx.charId,id,old.next_cursor):lifeClient.goalEvidence(ctx.charId,id,old.next_cursor));const merged={items:[...old.items,...next.items],next_cursor:next.next_cursor};if(kind==='history')history=merged;else evidence=merged;paint();}catch(error){if(alive)el.querySelector('[data-error]').textContent=errorMessage(error);button.disabled=false;}});
  }
  void read();return()=>{alive=false;};
}
