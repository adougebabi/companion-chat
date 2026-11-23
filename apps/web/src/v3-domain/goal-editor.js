import {errorMessage,errorRecord} from '../v3/domain-presentation.mjs';
import { randomId } from '../random-id.mjs';
import { lifeClient } from '../v3/proto/services.js';
import { Field } from '../v3/proto/components/forms.js';
import {GoalRecordFacts,GoalRecords} from '../v3/proto/components/goal-records.js';
import {stateLabel} from '../v3/domain-presentation.mjs';
import {RawRecordDownload} from '../v3/proto/components/records.js';
import {userInstant} from '../v3/presentation-time.mjs';
import {zonedInput,zonedInputInstant} from '../v3/zoned-input.mjs';
import {goalExecutionFacts,goalRecordStatus,goalRecordText} from '../approved-ui/goal-record-presentation.mjs';
import { esc,badge,toast,requireOwner,confirm } from '../v3/proto/ui.js';

function goalPlanSummary(goal) {
  const stages=Array.isArray(goal.stages)?goal.stages:[];
  const commitments=Array.isArray(goal.commitments)?goal.commitments:[];
  const stageName=id=>goalRecordText(stages.find(stage=>stage?.id===id)?.purpose);
  const criteria=value=>Array.isArray(value)?value.map(item=>goalRecordText(item?.text??item)).filter(Boolean).join('；'):'';
  const section=(title,rows)=>rows.length?`<h4>${title}</h4>${rows.map(facts=>`<div data-domain-ui="item">${GoalRecordFacts(facts)}</div>`).join('')}`:'';
  return section('阶段与推进条件',stages.filter(Boolean).map(stage=>[
    ['阶段',goalRecordText(stage.purpose)],['状态',goalRecordStatus(stage.status)],
    ['推进方式',goalRecordText(stage.strategy)],['开始依据',goalRecordText(stage.entry_basis)],
    ['结束依据',goalRecordText(stage.exit_basis)],['完成条件',criteria(stage.criteria)],
  ]))+section('行动承诺',commitments.filter(Boolean).map(commitment=>[
    ['期望结果',goalRecordText(commitment.expected_result)],['状态',goalRecordStatus(commitment.status)],
    ['所属阶段',stageName(commitment.stage_id)],
    ['窗口开始',commitment.window_start&&userInstant(commitment.window_start)],
    ['窗口结束',commitment.window_end&&userInstant(commitment.window_end)],
    ['推进机会',goalRecordText(commitment.opportunity_condition)],['当前阻碍',goalRecordText(commitment.blocker)],
    ['完成条件',criteria(commitment.criteria)],
  ]));
}

export function renderRealGoalEditor(el,ctx) {
  let alive=true,id=ctx.params.goalId || null,current,draft,busy=false,conflict=false,accepted=null;
  let editOpen=!id;
  let history={items:[],next_cursor:''},evidence={items:[],next_cursor:''},key=randomId();
  const fromGoal=g=>({title:g?.desired_outcome || '',why:g?.motivation || '',criteria:(g?.success_criteria || []).join('\n'),due:g?.deadline || '',deadlinePolicy:g?.deadline_policy || 'soft',scope:g?.scope || 'general',profileId:'',targetActorId:'',ownerProtected:!!g?.owner_protected,candidateOnly:g?.status==='candidate',reason:'',version:g?.revision ?? 0});
  async function read() {
    el.innerHTML='<div class="statebox" role="status" data-domain-ui="skeleton">正在读取目标…</div>';
    try{if(id){[current,history,evidence]=await Promise.all([lifeClient.goal(ctx.charId,id),lifeClient.goalHistory(ctx.charId,id),lifeClient.goalEvidence(ctx.charId,id)]);}draft=fromGoal(current);paint();}
    catch(error){if(alive){el.innerHTML=`<div data-domain-ui="alert" data-feedback-kind="error" class="banner err"><h2>目标资料暂未读到</h2><p>${esc(errorMessage(error))}</p>${RawRecordDownload(errorRecord(error),'原始目标读取错误记录（诊断）')}<button data-domain-ui="button" type="button" data-domain-variant="outline" data-domain-size="default" data-read-retry>重新读取原目标</button></div><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${ctx.charId}/life/goals">返回目标列表</a>`;el.querySelector('[data-read-retry]').onclick=()=>void read();}}
  }
  function showFailure(cause){const host=el.querySelector('[data-error]');if(!alive||!host)return;host.hidden=false;host.setAttribute('data-feedback-kind',accepted?'action-required':cause.status===409?'warning':'error');host.textContent=errorMessage(cause);}
  function paint() {
    if(!alive)return;
    el.innerHTML=`<div class="row [margin-bottom:12px]"><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${ctx.charId}/life/goals">目标列表</a><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${ctx.charId}/life/planning">独立规划</a></div>${current?`<section data-domain-ui="card" class="card goal-reading"><div class="row"><h2 class="grow">${esc(current.desired_outcome)}</h2>${badge(stateLabel(current.status))}</div>${current.success_criteria?.length?`<h3>怎样算完成</h3><ul>${current.success_criteria.map(value=>`<li>${esc(value)}</li>`).join('')}</ul>`:''}${current.deadline?`<p class="small muted">期望完成时间：${esc(userInstant(current.deadline))}</p>`:''}${current.motivation?`<details data-domain-ui="disclosure"><summary>动机与说明</summary><p>${esc(current.motivation)}</p></details>`:''}</section>`:''}
      <div class="grid g2 goal-detail-layout"><details data-domain-ui="disclosure" class="goal-edit-panel" ${editOpen||conflict?'open':''}><summary>${id?'编辑目标':'填写新目标'}</summary>
      <form data-domain-ui="card" class="card stack" data-form><h2>${id?'修改目标资料':'新建目标'}</h2>${conflict?`<div data-domain-ui="alert" class="banner err" role="alert" data-test="conflict">目标已被更新，草稿仍然保留。</div><div class="row"><button data-domain-ui="button" type="button" data-domain-variant="outline" data-domain-size="default" data-keep>保留草稿</button><button data-domain-ui="button" type="button" data-domain-variant="outline" data-domain-size="default" data-refresh>采用最新目标并保留草稿</button></div>`:''}${Field({label:'标题',name:'title',value:draft.title,attrs:{required:true,maxlength:4000}})}${Field({label:'动机与描述',name:'why',value:draft.why,type:'textarea',attrs:{required:true}})}${Field({label:'完成条件（每行一项）',name:'criteria',value:draft.criteria,type:'textarea',attrs:{required:true}})}${Field({label:'截止时间（可留空）',name:'due',type:'datetime-local',value:zonedInput(draft.due,'Asia/Shanghai'),hint:'按北京时间填写'})}<p class="small muted" data-deadline-preview>${draft.due?esc(userInstant(draft.due)):''}</p>${Field({label:'截止策略',name:'deadlinePolicy',value:draft.deadlinePolicy,type:'select',options:[{value:'soft',label:'软期限'},{value:'hard',label:'硬期限'}]})}
      <details data-domain-ui="disclosure"><summary>范围与保护</summary>${Field({label:'作用范围',name:'scope',value:draft.scope,type:'select',options:[{value:'general',label:'一般目标'},{value:'relationship',label:'关系目标'}]})}${!id?Field({label:'所属人格标识（选填）',name:'profileId',value:draft.profileId})+Field({label:'关联对象标识（选填）',name:'targetActorId',value:draft.targetActorId}):''}<label class="toggle"><input data-domain-ui="field" name="ownerProtected" type="checkbox" ${draft.ownerProtected?'checked':''}> 所有者保护目标</label>${!id?`<label class="toggle"><input data-domain-ui="field" name="candidateOnly" type="checkbox" ${draft.candidateOnly?'checked':''}> 先作为候选</label>`:''}</details>${Field({label:'修改原因',name:'reason',value:draft.reason,type:'textarea',attrs:{required:true}})}<div class="row"><button data-domain-ui="button" data-domain-variant="default" data-domain-size="default" ${busy?'disabled':''}>保存目标</button><button data-domain-ui="button" type="button" data-domain-variant="outline" data-domain-size="default" data-revert>撤销更改</button></div><p data-domain-ui="alert" data-feedback-kind="information" data-error hidden></p></form></details>
      <div class="stack"><div data-domain-ui="card" class="card"><h3>状态与来源</h3>${badge(stateLabel(current?.status,'尚未提交'))}${current?GoalRecordFacts(goalExecutionFacts(current.execution))+goalPlanSummary(current):''}${id?`<div class="row">${['暂停','恢复','取消','放弃','重新评估'].map((label,index)=>`<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="sm" data-command="${['pause','resume','cancel','abandon','reassess'][index]}">${label}</button>`).join('')}</div>`:''}${current?RawRecordDownload(current,'原始目标与推进记录（诊断）'):''}</div>
      <div data-domain-ui="card" class="card"><h3>证据与修订</h3><p class="small muted">目标是否完成取决于证据评估。此处不提供直接标记完成。</p><h4>证据</h4>${GoalRecords(evidence.items,{kind:'evidence',downloadTitle:'原始目标证据记录（诊断）'})}${evidence.next_cursor?'<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="sm" data-more="evidence">加载更多证据</button>':''}
      <h4>修订</h4>${GoalRecords(history.items,{kind:'history',downloadTitle:'原始目标修订记录（诊断）'})}${history.next_cursor?'<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="sm" data-more="history">加载更多修订</button>':''}</div></div></div>`;
    const form=el.querySelector('form'),panel=el.querySelector('.goal-edit-panel');panel.ontoggle=()=>{editOpen=panel.open;};
    const capture=()=>{for(const input of form.querySelectorAll('[name]')){if(input.name==='due'){try{draft.due=input.value?zonedInputInstant(input.value,'Asia/Shanghai',draft.due):'';input.setCustomValidity('');}catch(error){input.setCustomValidity(errorMessage(error));}}else draft[input.name]=input.type==='checkbox'?input.checked:input.value;}el.querySelector('[data-deadline-preview]').textContent=draft.due?userInstant(draft.due):'';};
    form.oninput=form.onchange=capture;
    form.onsubmit=async event=>{event.preventDefault();capture();if(!form.reportValidity()||busy || !(await requireOwner()))return;busy=true;const sending=form.querySelector('[data-domain-variant=default]');sending.disabled=true;sending.textContent=accepted?'正在读取原目标结果…':'正在保存目标…';
      try {
        if(!accepted){const body={idempotencyKey:key,reason:draft.reason,desiredOutcome:draft.title,motivation:draft.why,successCriteria:draft.criteria.split('\n').map(t=>t.trim()).filter(Boolean),deadlinePolicy:draft.deadlinePolicy,scope:draft.scope,ownerProtected:draft.ownerProtected,...(draft.profileId?{profileId:draft.profileId}:{}),...(draft.targetActorId?{targetActorId:draft.targetActorId}:{}),...(draft.due?{deadline:draft.due}:{clearDeadline:true}),...(id?{expectedRevision:draft.version}:{candidateOnly:draft.candidateOnly})};accepted=await lifeClient.saveGoal(ctx.charId,id,body);id=accepted.goal_id;}
        const saved=await lifeClient.goal(ctx.charId,id);if(!alive)return;
        if(saved.desired_outcome!==draft.title || saved.motivation!==draft.why)throw new Error('命令已受理，但读回内容尚未确认一致。请重新读取。');
        current=saved;draft=fromGoal(saved);accepted=null;conflict=false;key=randomId();toast('目标已保存并读回','ok');
        if(!ctx.params.goalId){ctx.navigate(`#/c/${ctx.charId}/life/goal/${id}`);return;}
        [history,evidence]=await Promise.all([lifeClient.goalHistory(ctx.charId,id),lifeClient.goalEvidence(ctx.charId,id)]);paint();
      } catch(error) {if(!alive)return;if(error.status===409&&id){try{current=await lifeClient.goal(ctx.charId,id);if(!alive)return;conflict=true;paint();}catch(readError){if(!alive)return;const host=el.querySelector('[data-error]');host.hidden=false;host.setAttribute('data-feedback-kind','warning');host.textContent='版本已变化，但最新目标资料尚未读到；当前修改仍保留。';host.insertAdjacentHTML('afterend',`<div data-latest-recovery>${RawRecordDownload(errorRecord(readError),'原始目标读取错误记录（诊断）')}<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" type="button" data-retry-latest>重新读取最新目标版本</button></div>`);el.querySelector('[data-retry-latest]').onclick=async event=>{const retryButton=event.currentTarget;retryButton.disabled=true;try{const latest=await lifeClient.goal(ctx.charId,id);if(!alive)return;current=latest;conflict=true;paint();}catch(cause){if(alive){host.textContent='最新版本仍未读到，当前修改与原请求保持。';retryButton.disabled=false;}}};return;}}const host=el.querySelector('[data-error]');showFailure(error);}finally{busy=false;if(alive){const send=el.querySelector('form [data-domain-variant=default]');if(send){send.disabled=false;send.textContent=accepted?'重新读取原目标结果':'保存目标';}}}
    };
    el.querySelector('[data-revert]').onclick=()=>{draft=fromGoal(current);conflict=false;key=randomId();paint();};
    el.querySelector('[data-keep]')?.addEventListener('click',()=>toast('草稿已保留'));
    el.querySelector('[data-refresh]')?.addEventListener('click',()=>{draft.version=current.revision;conflict=false;key=randomId();paint();});
    el.querySelectorAll('[data-command]').forEach(button=>button.onclick=async()=>{capture();if(busy || !(await requireOwner()))return;if(!draft.reason.trim()){editOpen=true;panel.open=true;form.elements.reason.focus();return toast('请在编辑目标中填写本次操作原因','err');}if(!(await confirm({title:'调整目标',text:esc(draft.title)})))return;busy=true;try{await lifeClient.goalCommand(ctx.charId,id,button.dataset.command,current.revision,draft.reason);await read();}catch(error){if(alive)showFailure(error);}finally{busy=false;}});
    el.querySelectorAll('[data-more]').forEach(button=>button.onclick=async()=>{capture();button.disabled=true;try{const kind=button.dataset.more,old=kind==='history'?history:evidence,next=await (kind==='history'?lifeClient.goalHistory(ctx.charId,id,old.next_cursor):lifeClient.goalEvidence(ctx.charId,id,old.next_cursor));const merged={items:[...old.items,...next.items],next_cursor:next.next_cursor};if(kind==='history')history=merged;else evidence=merged;paint();}catch(error){if(alive)showFailure(error);button.disabled=false;}});
  }
  void read();return()=>{alive=false;};
}
