import {randomId} from '../random-id.mjs';
import {errorMessage} from '../v3/domain-presentation.mjs';
import {get} from '../v3/proto/store.js';
import {studioDraftStorage} from '../v3/proto/components/studio-draft-storage.js';
import {mountGoalOrdering} from '../v3/proto/components/goal-ordering.js';
import { lifeClient } from '../v3/proto/services.js';
import { asyncPage } from '../v3/proto/components/async-page.js';
import { Field } from '../v3/proto/components/forms.js';
import {GoalRecords as RecordDetails} from '../v3/proto/components/goal-records.js';
import {stateLabel} from '../v3/domain-presentation.mjs';
import { esc,badge,requireOwner,toast,confirm,drawer } from '../v3/proto/ui.js';

// Only the newest mounted editor owns draft fields. Older requests may still
// checkpoint their original command's acknowledgement into that editor.
const editors=new Map();
function acknowledgedFields(value,changes){
  const out={};
  for(const [input,field]of [['autoPlanningEnabled','auto_planning_enabled'],['orderingMode','ordering_mode'],['maxActiveGoals','max_active_goals']])if(input in changes)out[field]=value[field];
  if(changes.recoveryPolicy)for(const field of Object.keys(changes.recoveryPolicy))out[field]=value[field];
  if(changes.dependencies)out.dependencies=Object.fromEntries(Object.keys(changes.dependencies).sort().map(id=>[id,(value.dependencies||[]).filter(d=>d.goal_id===id).map(d=>d.prerequisite_id).sort()]));
  if(changes.order)out.order=(value.goals||[]).filter(g=>g.status==='active').map(g=>g.id);
  if(changes.changes)out.goals=changes.changes.map(c=>{const g=value.goals?.find(g=>g.id===c.id);return g?{id:g.id,status:g.status,revision:g.revision}:null;});
  if(changes.sourceReviews)out.source_reviews=value.source_reviews;
  if(changes.reviewedGoalIds)out.reviewed=changes.reviewedGoalIds.map(id=>value.goals?.find(g=>g.id===id)?.context_review_required);
  return out;
}
export function renderRealGoalPlanning(el,ctx) {
  const slot=`fluctlight-studio:${get('session')?.actorId}:goal-planning:${ctx.charId}`,storage=studioDraftStorage();let draft;try{draft=JSON.parse(storage.getItem(slot)||'null');}catch{}
  const owner=get('session')?.actorId,editor={draft};editors.set(slot,editor);
  const persist=()=>{
    if(get('session')?.actorId!==owner)return;
    if(editors.get(slot)===editor){editor.draft=draft;storage.setItem(slot,JSON.stringify(draft));editor.refreshLabels?.();return;}
    let latest=editors.get(slot)?.draft;try{latest||=JSON.parse(storage.getItem(slot)||'null');}catch{}
    if(!latest)return;
    for(const name of ['pending','planPending'])if(draft?.[name]&&latest[name]&&draft[name].key===latest[name].key&&draft[name].accepted)Object.assign(latest[name],draft[name]);
    storage.setItem(slot,JSON.stringify(latest));editors.get(slot)?.refreshLabels?.();
  };
  const dispose=asyncPage(el,async()=>{const [state,history]=await Promise.all([lifeClient.goalSet(ctx.charId),lifeClient.planningHistory(ctx.charId)]);return {state,history};},({state,history},reload,alive)=>{
    const dependencies=Object.fromEntries(state.goals.map(g=>[g.id,state.dependencies.filter(d=>d.goal_id===g.id).map(d=>d.prerequisite_id)]));
    draft||={base:state.revision,facts:state.facts_revision,policy:{auto:state.auto_planning_enabled,orderingMode:state.ordering_mode,max:state.max_active_goals,recoveryPolicy:JSON.stringify(Object.fromEntries(['merge_window_seconds','retry_backoff_seconds','max_attempts','lease_seconds','candidate_limit'].map(k=>[k,state[k]])),null,2),reason:''},dependencies,order:state.goals.map(g=>g.id),selected:'',review:{sourceReviews:'[]',reviewedGoalIds:'',reason:''}};
    editor.draft=draft;
    const recoveryLabel=()=>draft.pending?.accepted?'只读恢复已保存提交结果':draft.planPending?.accepted?'只读恢复规划任务历史':'核对或重试原提交';
    const scopedAlive=()=>alive()&&get('session')?.actorId===owner;
    const baseStale=draft.base!==state.revision||draft.facts!==state.facts_revision;
    el.innerHTML=`<div class="row [margin-bottom:12px]"><a data-domain-ui="button" class="btn" href="#/c/${ctx.charId}/life/goals">目标列表</a><button data-domain-ui="button" class="btn" data-reload>重新读取</button></div>${baseStale?'<p data-domain-ui="alert" class="banner warn">策略版本已变化，原草稿已保留。请核对最新策略后采用新基线。</p><button data-domain-ui="button" class="btn" data-policy-rebase>读取并采用最新基线，保留草稿</button>':''}${draft.pending||draft.planPending?'<p data-domain-ui="alert" class="banner warn">上次提交尚待核对，原提交和新草稿均已保留。</p><button data-domain-ui="button" class="btn" data-command-recover>'+recoveryLabel()+'</button>':''}<div class="grid g2 items-start"><div data-domain-ui="card" class="card"><h3>独立规划</h3><p class="small muted">规划可产生候选、保留现有目标或无须新增；不会强制每次凑足固定数量。已受理不代表规划完成。</p><form class="stack" data-policy><label class="toggle"><input data-domain-ui="field" type="checkbox" name="auto" ${state.auto_planning_enabled?'checked':''}> 自动规划</label>${Field({label:'排序方式',name:'orderingMode',type:'select',value:state.ordering_mode,options:[{value:'automatic',label:'自动排序'},{value:'manual',label:'手动排序'}]})}${Field({label:'活跃目标上限',name:'max',type:'number',value:state.max_active_goals,attrs:{min:1,max:5,required:true}})}<div data-goal-ordering></div><details data-domain-ui="disclosure"><summary>依赖与恢复策略</summary>${Field({label:'规划恢复策略（JSON）',name:'recoveryPolicy',type:'textarea',value:JSON.stringify(Object.fromEntries(['merge_window_seconds','retry_backoff_seconds','max_attempts','lease_seconds','candidate_limit'].map(k=>[k,state[k]])),null,2),hint:'修改前核对当前保存值；留空表示保留。'})}</details>${Field({label:'修改原因',name:'reason',type:'textarea',attrs:{required:true}})}<button data-domain-ui="button" class="btn primary">保存策略</button></form><p class="small muted">目标集 v${esc(state.revision)} · 当前活跃 ${esc(state.active_count)}${state.capacity_violation?' · 已超出当前上限，请先调整集合':''}</p><button data-domain-ui="button" class="btn" data-plan>${draft.planPending?.accepted?'只读恢复规划任务历史':'立即规划'}</button><p data-domain-ui="alert" data-error role="alert"></p><h3>规划任务与历史</h3>${RecordDetails(history)}</div><div data-domain-ui="card" class="card"><h3>候选与来源审查</h3>${state.goals.filter(g=>g.status==='candidate').map(g=>`<div data-domain-ui="item" class="lrow"><span class="grow"><a href="#/c/${ctx.charId}/life/goal/${esc(g.id)}">${esc(g.desired_outcome || g.description)}</a><div class="small muted">${esc(g.motivation)}</div></span>${badge(stateLabel(g.status))}<button data-domain-ui="button" class="btn sm" data-accept="${esc(g.id)}">采纳</button><button data-domain-ui="button" class="btn sm" data-reject="${esc(g.id)}">拒绝</button></div>`).join('')||'<p class="small muted">当前规划窗口暂无候选，可查看历史中的判断结果。</p>'}<h3>初始化来源</h3>${RecordDetails(state.source_reviews || [])}<p class="small muted">来源审查与候选生命周期分别记录，不能通过删除目标消除来源。</p><details data-domain-ui="disclosure"><summary>审查命令</summary><form class="stack" data-review>${Field({label:'来源裁定（JSON 数组）',name:'sourceReviews',type:'textarea',value:'[]'})}${Field({label:'已核对目标 ID（每行一项）',name:'reviewedGoalIds',type:'textarea'})}${Field({label:'审查原因',name:'reason',attrs:{required:true}})}<button data-domain-ui="button" class="btn">提交审查</button></form></details></div></div>`;
    editor.refreshLabels=()=>{if(!scopedAlive())return;const recover=el.querySelector('[data-command-recover]');if(recover)recover.textContent=recoveryLabel();const planButton=el.querySelector('[data-plan]');if(planButton)planButton.textContent=draft.planPending?.accepted?'只读恢复规划任务历史':'立即规划';};
    const ordering=mountGoalOrdering(el.querySelector('[data-goal-ordering]'),{goals:state.goals,dependencies:draft.dependencies,mode:draft.policy.orderingMode,initialOrder:draft.order,initialSelected:draft.selected,onChange:value=>{Object.assign(draft,value);persist();}});
    const policy=el.querySelector('[data-policy]'),review=el.querySelector('[data-review]');
    const fill=(form,values)=>{for(const [name,value]of Object.entries(values)){const input=form.elements[name];if(input){if(input.type==='checkbox')input.checked=value;else input.value=value;}}};fill(policy,draft.policy);fill(review,draft.review);
    const capture=(form,keys)=>Object.fromEntries(keys.map(name=>[name,form.elements[name].type==='checkbox'?form.elements[name].checked:form.elements[name].value]));
    policy.addEventListener('input',()=>{draft.policy=capture(policy,['auto','orderingMode','max','recoveryPolicy','reason']);persist();});policy.addEventListener('change',()=>{draft.policy=capture(policy,['auto','orderingMode','max','recoveryPolicy','reason']);persist();});review.oninput=()=>{draft.review=capture(review,['sourceReviews','reviewedGoalIds','reason']);persist();};
    el.querySelector('[data-policy-rebase]')?.addEventListener('click',async()=>{if(await confirm({title:'采用最新策略版本',identity:'草稿操作',text:'保留当前排序、前置条件和表单草稿，以最新读取版本作为下一次保存的基线。'})&&alive()){draft.base=state.revision;draft.facts=state.facts_revision;persist();void reload();}});
    el.querySelector('[name=orderingMode]').addEventListener('change',event=>ordering.setMode(event.target.value));
    const error=message=>{if(scopedAlive())el.querySelector('[data-error]').textContent=message;};
    el.querySelector('[data-reload]').onclick=reload;
    let commandBusy=false;
    async function change(changes,reason,channel='decision',recover=false) {
      if(commandBusy)return;
      if((draft.pending||draft.planPending)&&!recover){error('请先核对或重试原提交，新草稿已保留。');return;}
      commandBusy=true;
      if(!(await requireOwner())||!scopedAlive()){commandBusy=false;return;}
      if(!recover&&channel!=='decision'&&baseStale){error('请先核对并采用最新策略版本，草稿已保留。');commandBusy=false;return;}
      const snapshot=()=>JSON.stringify({policy:draft.policy,review:draft.review,dependencies:draft.dependencies,order:draft.order});
      if(!recover){draft.pending={base:channel==='decision'?{revision:state.revision,facts_revision:state.facts_revision}:{revision:draft.base,facts_revision:draft.facts},changes,reason,channel,key:randomId(),snapshot:snapshot(),accepted:false,attempts:0};persist();}
      const pending=draft.pending;
      // Another mounted editor can receive this original request's late receipt.
      let saved;try{saved=JSON.parse(storage.getItem(slot)||'null');}catch{}
      if(saved?.pending?.key===pending.key&&saved.pending.accepted)Object.assign(pending,saved.pending);
      const button=pending.channel==='policy'?policy.querySelector('.primary'):pending.channel==='review'?review.querySelector('button'):null;
      if(button)button.disabled=true;
      try{
        if(!pending.accepted){pending.attempts++;persist();const receipt=await lifeClient.updateGoalSet(ctx.charId,pending.base,pending.changes,pending.reason,pending.key);if(receipt?.commit_succeeded!==true||!Number.isSafeInteger(receipt.revision)||receipt.revision<=pending.base.revision)throw Error('提交回执尚未完整读到，原提交身份已保留。');pending.accepted=true;pending.receiptRevision=receipt.revision;pending.acknowledged=acknowledgedFields(receipt,pending.changes);persist();}
        const latest=await lifeClient.goalSet(ctx.charId);if(!scopedAlive())return;
        if(!Number.isSafeInteger(latest.revision)||latest.revision<(pending.receiptRevision||pending.base.revision+1)||typeof latest.facts_revision!=='string')throw Error('最新目标集版本尚未完整读到，原提交已保留。');
        if(latest.revision===pending.receiptRevision&&JSON.stringify(acknowledgedFields(latest,pending.changes))!==JSON.stringify(pending.acknowledged))throw Error('已保存版本的提交字段尚未完整读回，原回执已保留。');
        if(snapshot()===pending.snapshot&&pending.channel==='policy'){draft.policy={auto:latest.auto_planning_enabled,orderingMode:latest.ordering_mode,max:latest.max_active_goals,recoveryPolicy:JSON.stringify(Object.fromEntries(['merge_window_seconds','retry_backoff_seconds','max_attempts','lease_seconds','candidate_limit'].map(k=>[k,latest[k]])),null,2),reason:''};draft.dependencies=Object.fromEntries(latest.goals.map(g=>[g.id,latest.dependencies.filter(d=>d.goal_id===g.id).map(d=>d.prerequisite_id)]));draft.order=latest.goals.map(g=>g.id);}
        else if(snapshot()===pending.snapshot&&pending.channel==='review')draft.review={sourceReviews:'[]',reviewedGoalIds:'',reason:''};
        // Candidate/source commands must not silently rebase a separate policy draft.
        if(pending.channel==='policy'){draft.base=latest.revision;draft.facts=latest.facts_revision;}
        if(pending.channel==='decision'&&draft.decision?.reason===pending.reason)delete draft.decision;
        delete draft.pending;persist();await reload();toast('本次提交已保存并读回，期间的新草稿保留','ok');
      }catch(e){
        const accepted=pending.accepted;
        if(!accepted&&pending.attempts===1&&[400,403,409,422].includes(e.status)){delete draft.pending;persist();}
        error((accepted?'本次提交已保存，最新状态读取失败：':'')+errorMessage(e)+(e.status===409?'；版本已变化，输入已保留，请重新读取后确认。':''));
        if(scopedAlive()&&draft.pending&&!el.querySelector('[data-command-recover]')){const retry=document.createElement('button');retry.type='button';retry.dataset.domainUi='button';retry.dataset.commandRecover='';retry.className='btn';retry.textContent=recoveryLabel();retry.onclick=()=>change(null,null,null,true);el.querySelector('[data-error]').after(retry);}
        if(button?.isConnected)button.disabled=false;
      }finally{commandBusy=false;}
    }
    el.querySelector('[data-command-recover]')?.addEventListener('click',()=>draft.planPending?plan():change(null,null,null,true));
    el.querySelector('[data-policy]').onsubmit=event=>{event.preventDefault();const f=event.target;try{const changes={autoPlanningEnabled:f.elements.auto.checked,orderingMode:f.elements.orderingMode.value,maxActiveGoals:Number(f.elements.max.value),dependencies:JSON.parse(f.elements.dependencies.value),...(f.elements.orderingMode.value==='manual'?{order:f.elements.order.value.split('\n').map(v=>v.trim()).filter(id=>state.goals.some(g=>g.id===id&&g.status==='active'))}:{}),...(f.elements.recoveryPolicy.value.trim() && f.elements.recoveryPolicy.value.trim()!=='{}'?{recoveryPolicy:JSON.parse(f.elements.recoveryPolicy.value)}:{})};void change(changes,f.elements.reason.value,'policy');}catch(e){error(e instanceof SyntaxError?'目标依赖或恢复策略不是有效 JSON，请检查后再保存。':errorMessage(e));}};
    el.querySelector('[data-review]').onsubmit=event=>{event.preventDefault();const f=event.target;try{void change({sourceReviews:JSON.parse(f.elements.sourceReviews.value),reviewedGoalIds:f.elements.reviewedGoalIds.value.split('\n').map(v=>v.trim()).filter(Boolean)},f.elements.reason.value,'review');}catch(e){error(e instanceof SyntaxError?'来源裁定不是有效 JSON，请检查后再提交。':errorMessage(e));}};
    const decide=(id,accept)=>{if(draft.pending||draft.planPending){error('请先核对或重试原提交，新草稿已保留。');return;}const goal=state.goals.find(g=>g.id===id);const d=drawer({title:accept?'采纳候选目标':'拒绝候选目标',body:`<p>${esc(goal.desired_outcome||goal.description)}</p><form data-decision>${Field({label:'原因',name:'reason',type:'textarea',attrs:{required:true}})}</form><p data-domain-ui="alert" data-decision-error role="alert"></p>`,footer:'<button data-domain-ui="button" class="btn" data-close>取消</button><button data-domain-ui="button" class="btn primary" data-submit>提交判断</button>'});const decisionForm=d.el.querySelector('form');if(draft.decision?.id===id&&draft.decision.accept===accept)decisionForm.elements.reason.value=draft.decision.reason;decisionForm.oninput=()=>{draft.decision={id,accept,reason:decisionForm.elements.reason.value};persist();};d.el.querySelector('[data-close]').onclick=()=>{delete draft.decision;persist();d.close();};d.el.querySelector('[data-submit]').onclick=async()=>{const form=d.el.querySelector('form');if(!form.reportValidity())return;d.close();await change({changes:[{id,operation:accept?'resume':'cancel',expected_revision:goal.revision}]},form.reason.value);};};
    el.querySelectorAll('[data-accept]').forEach(b=>b.onclick=()=>decide(b.dataset.accept,true));
    el.querySelectorAll('[data-reject]').forEach(b=>b.onclick=()=>decide(b.dataset.reject,false));
    async function plan(){
      if(commandBusy)return;
      if(draft.pending){error('请先核对或重试原提交，新草稿已保留。');return;}
      if(!(await requireOwner())||!scopedAlive())return;
      commandBusy=true;const button=el.querySelector('[data-plan]');button.disabled=true;
      draft.planPending||={key:randomId(),accepted:false};const pending=draft.planPending;let saved;try{saved=JSON.parse(storage.getItem(slot)||'null');}catch{}if(saved?.planPending?.key===pending.key&&saved.planPending.accepted)Object.assign(pending,saved.planPending);persist();
      try{if(!pending.accepted){const receipt=await lifeClient.plan(ctx.charId,pending.key);if(receipt?.idempotency_key!==pending.key)throw Error('规划回执身份尚未完整读到，原提交身份已保留。');pending.accepted=true;persist();}await lifeClient.planningHistory(ctx.charId);if(!scopedAlive())return;delete draft.planPending;persist();await reload();toast('规划请求已受理，正在重新读取任务历史；受理不代表完成');}
      catch(e){if(button.isConnected&&pending.accepted)button.textContent='只读恢复规划任务历史';error((pending.accepted?'规划请求已受理，任务历史读取失败：':'规划请求结果尚待核对：')+errorMessage(e));if(button.isConnected)button.disabled=false;}
      finally{commandBusy=false;}
    }
    el.querySelector('[data-plan]').onclick=plan;
  });
  return()=>{dispose();if(editors.get(slot)===editor)editors.delete(slot);};
}
