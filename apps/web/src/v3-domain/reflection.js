import {requestDecisionText} from '../v3/proto/components/request-failure.js';
import {errorMessage} from '../v3/domain-presentation.mjs';
import {ReflectionProgress} from '../v3/proto/components/reflection-progress.js';
import {reflectionClient} from '../v3/proto/services.js';
import {asyncPage,requestReceipt,instant} from '../v3/proto/components/async-page.js';
import {CursorPager,bindCursorPager} from '../v3/proto/components/cursor-pager.js';
import {mountCursorRecords} from '../v3/proto/components/cursor-records.js';
import {scoreText} from '../approved-ui/memory-record-summary.mjs';
import {RawRecordDownload} from '../v3/proto/components/records.js';
import {Field} from '../v3/proto/components/forms.js';
import {stateLabel as domainStateLabel} from '../v3/domain-presentation.mjs';
import {isInternalId} from '../v3/internal-id.mjs';
import {userInstant} from '../v3/presentation-time.mjs';
import {esc,badge,drawer} from '../v3/proto/ui.js';

const growthNavigation=(fid,current)=>`<nav class="reading-module-nav" aria-label="记忆成长">${[['memory','记忆',`#/c/${fid}/growth`],['reflection','反思',`#/c/${fid}/growth/reflection`],['self','成长认识',`#/c/${fid}/growth/self`],['snapshot','人格快照',`#/c/${fid}/growth/self?view=snapshot`]].map(([key,label,href])=>`<a href="${href}" ${current===key?'aria-current="page"':''}>${label}</a>`).join('')}</nav>`;

const href=(fid,id)=>`#/c/${encodeURIComponent(fid)}/growth/reflection/${encodeURIComponent(id)}`;
const stateLabel={applied:'已应用',no_change:'无变化',proposed:'历史提案'};
const receiptOptions={title:'反思反馈',labels:{completed:'反馈已处理',failed:'反馈处理失败'},note:'反馈由实际认知任务处理。认同不强制成长，不同看法不直接更正事实；关闭仅停止观察。'};

export function renderRealReflections(el,ctx){
 const cursors=[''];let at=0;
 return asyncPage(el,()=>reflectionClient.list(ctx.charId,cursors[at]),(initial,reload,alive)=>{
  let page=initial,timer,reads=0,busy=false,failure='';const taskId=page.latest_task?.intent_id;
  el.innerHTML=`<div class="stack reading-workspace"><h1 class="reading-business-title">反思与成长</h1>${growthNavigation(ctx.charId,'reflection')}<section class="stack" data-reflection-results></section><details data-domain-ui="disclosure" class="card" data-progress><summary>这次反思的处理进度</summary><div data-reflection-progress></div></details><div data-reflection-pager></div><p data-domain-ui="alert" data-observe-error role="status"></p><div class="row"><button data-domain-ui="button" data-domain-variant="ghost" data-domain-size="default" data-reload>重新读取</button><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${ctx.charId}/growth/self">查看成长认识</a><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${ctx.charId}/growth">她的记忆</a></div></div>`;
  function paint(){if(!alive())return;
   el.querySelector('[data-reflection-results]').innerHTML=page.items.map(r=>`<a data-domain-ui="card" class="card reflection-result" href="${href(ctx.charId,r.id)}"><div class="row"><h3 class="grow">${esc(r.title)}</h3>${badge(stateLabel[r.status]||r.status,'plain')}</div><p>${esc(r.summary??'这条旧记录没有摘要。')}</p><span class="small muted">${esc(instant(r.at))}</span></a>`).join('')||'<div data-domain-ui="empty" class="statebox"><h3>还没有反思结果</h3><p>她会结合新的交流和经历反思；一次反思也可能没有新的变化。</p></div>';
   const progress=el.querySelector('[data-reflection-progress]'),opened=[...progress.querySelectorAll('details')].map((n,i)=>n.open?i:-1);
   progress.innerHTML=ReflectionProgress(page);el.querySelector('[data-progress]').open ||= !!page.latest_task?.error_code||['failed','dead_letter','superseded'].includes(page.latest_task?.status);
   progress.querySelectorAll('details').forEach((n,i)=>{n.open=opened.includes(i);});
   el.querySelector('[data-reflection-pager]').innerHTML=CursorPager({more:!!page.next_cursor,previous:at>0,label:'反思'});
   bindCursorPager(el.querySelector('[data-reflection-pager]'),{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});
   el.querySelector('[data-observe-error]').textContent=failure;
  }
  const pending=()=>['pending','queued','dispatched','running','retry','retry_wait'].includes(page.latest_task?.status)||page.window?.status==='running';
  async function query(){if(!alive()||document.hidden||busy||reads>=60||!pending())return;busy=true;try{const next=await reflectionClient.list(ctx.charId,cursors[at]);if(!alive())return;if(taskId&&next.latest_task?.intent_id!==taskId){failure='已有处理已结束或有新的反思任务，可重新读取。';reads=60;}else{page=next;failure='';}paint();}catch(error){if(alive()){failure='更新失败，已读取的反思保留：'+errorMessage(error);paint();}}finally{busy=false;reads++;schedule();}}
  function schedule(){clearTimeout(timer);if(alive()&&!document.hidden&&pending()&&reads<60)timer=setTimeout(query,3000);}
  const visibility=()=>{clearTimeout(timer);if(!document.hidden)void query();};document.addEventListener('visibilitychange',visibility);
  el.querySelector('[data-reload]').onclick=reload;paint();schedule();
  return()=>{clearTimeout(timer);document.removeEventListener('visibilitychange',visibility);};
 },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/growth`,backLabel:'返回记忆成长'});
}

const changeDomains={
  active_memory:['近期记忆','active_memory_candidates'],
  memory:['记忆','memory_candidates'],
  relationship:['关系观察','relationship_observations'],
  goal:['目标','goal_candidates'],
  intention:['行动意图','intention_candidates'],
  affect_profile:['情绪调整','affect_recalibration_candidates'],
  drive:['动机','drive_candidates'],
  preference:['偏好','preference_candidates'],
  trigger:['触发规律','trigger_candidates'],
  developing_self:['成长认识','developing_self_candidates'],
  personality_overlay:['人格调整','personality_evolution_candidates'],
  behavior_policy_overlay:['行为方式调整','behavior_policy_evolution_candidates'],
};
const dispositions={accepted:'已采纳',rejected:'未采纳',deferred:'暂缓采纳',no_change:'维持原状'};
const reasons={
  domain_reducer_unavailable:'当前无法应用这类提议',
  confidence_below_threshold:'现有依据不足以支持这项变化',
  cross_window_evidence_required:'还需要其他时期的依据',
  cooldown_active:'距离上次调整时间较短',
  semantic_no_change:'提议与现状一致',
  exact_duplicate:'已有相同内容',
  reflection_memory_source_stale:'相关记忆的依据已发生变化',
  field_forbidden:'这项设定不可通过反思调整',
  field_not_allowlisted:'这项设定超出反思可调整的范围',
};
const sourceKinds={
  'conversation.turn':'对话交流',
  'life.owner_request.submitted':'用户提交的请求',
  'autonomy.result':'自主行动结果',
  'internal.wake_up':'日常唤醒',
};
const operations={
  create:'新增',update:'更新',revise:'修订',merge:'合并',confirm:'确认',complete:'完成',
  expire:'到期',deprecate:'停用',pause:'暂停',resume:'恢复',cancel:'取消',no_change:'维持原状',
};
const directions={
  increase:'增强',strengthen:'增强',improve:'改善',positive:'改善',toward:'增强',
  decrease:'减弱',weaken:'减弱',negative:'减弱',away:'减弱',maintain:'维持',
};
const candidateTextFields={
  content:'内容',observation:'关系观察',desired_outcome:'期望结果',success_criteria:'完成标准',
  motivation:'动机',action_intent:'行动意图',expected_outcome:'预期结果',capability_constraints:'能力约束',
  semantic_value:'拟调整内容',claim:'认识内容',
  original_time_expression:'原时间表述',semantic_reason:'提议原因',
  dominant_patterns:'主要情绪规律',triggers:'情绪触发因素',recovery_patterns:'恢复方式',conflicts:'情绪冲突',
};
const candidateTimes={valid_from:'生效时间',valid_until:'有效期至',preferred_time:'期望行动时间',expiration:'截止时间'};
const changeSubjects={
  'baseline.pleasure':'愉悦程度','baseline.arousal':'唤醒程度','baseline.dominance':'掌控感',
  'traits.openness':'开放性','traits.conscientiousness':'尽责性','traits.extraversion':'外向性',
  'traits.agreeableness':'亲和性','traits.emotional_stability':'情绪稳定性','expression.warmth':'表达温暖度',
  'expression.initiative':'表达主动性','expression.playfulness':'表达活泼度',
  'communication.tone':'交流语气','response.style':'回应方式','initiative.mode':'主动沟通方式',
  'conflict.approach':'冲突处理方式','support.style':'支持方式',
};
const selfCategories={
  preference:'偏好',habit:'习惯',sensitivity:'敏感点',emotion_pattern:'情绪模式',
  self_perception:'自我认识',capability:'能力',interest:'兴趣',
};
const labelFor=(labels,key)=>Object.hasOwn(labels,key)?labels[key]:'';
const rows=value=>Array.isArray(value)?value:[];

function reflectionText(value){
  if(typeof value!=='string')return '';
  const text=value.trim();
  const machineValue=/^(?:[\w.-]+\.v\d+|v\d+|(?:reflection_proposal|evolution_candidate|goal|intent|outcome)_[\w-]+|sequence:\d+(?:-\d+)?|fact:[\w:-]+|[\w-]+:ctx_[\w-]+)$/i;
  return isInternalId(text)||machineValue.test(text)?'':text;
}

function reflectionTime(value){
  return typeof value==='string'&&Number.isFinite(Date.parse(value))?userInstant(value):'';
}

function reflectionFacts(facts){
  const present=facts.filter(([,value])=>value);
  return present.length?'<dl class="record-facts">'+present.map(([label,value])=>
    `<div><dt>${esc(label)}</dt><dd>${esc(value)}</dd></div>`).join('')+'</dl>':'';
}

function reflectionSources(window){
  const evidence=rows(window?.evidence).filter(Boolean);
  const byKind=new Map();
  for(const item of evidence){
    const label=labelFor(sourceKinds,item.kind)||'其他反思素材';
    byKind.set(label,(byKind.get(label)||0)+1);
  }
  const times=evidence.map(item=>item.occurred_at).filter(value=>typeof value==='string'&&Number.isFinite(Date.parse(value)))
    .sort((a,b)=>Date.parse(a)-Date.parse(b));
  const start=reflectionTime(times[0]),end=reflectionTime(times.at(-1));
  const outcomes=rows(window?.outcomes).filter(Boolean).length;
  const facts=reflectionFacts([
    ['来源',[...byKind].map(([label,count])=>`${label} ${count} 条`).join('、')],
    ['素材发生时间',start===end?start:[start,end].filter(Boolean).join(' 至 ')],
    ['关联行动结果',outcomes?`${outcomes} 条`:''],
  ]);
  return facts?`<div data-domain-ui="card" class="card"><h3>反思来源</h3>${facts}</div>`:'';
}

function reflectionChanges(record,charId){
  const proposal=record.proposal;
  const candidates=new Map();
  const references=new Map(rows(record.references).filter(Boolean).map(entry=>[entry.ref,entry]));
  const items=[];
  for(const [domain,[label,field]] of Object.entries(changeDomains)){
    const group=new Map();
    const offset=domain==='affect_profile'?1:0;
    rows(proposal?.[field]).forEach((candidate,index)=>{
      if(candidate&&typeof candidate==='object')group.set(index+offset,{candidate,label});
    });
    if(domain==='affect_profile'&&proposal?.emotional_summary){
      group.set(0,{candidate:proposal.emotional_summary,label:'情绪总结'});
    }
    candidates.set(domain,group);
  }

  function paint(candidate,label,decision){
    const trigger=candidate?.typed_trigger;
    const facts=reflectionFacts([
      ['处理结果',labelFor(dispositions,decision?.disposition)],
      ['处理原因',labelFor(reasons,decision?.reason_code)],
      ['提议操作',labelFor(operations,candidate?.operation)],
      ['调整对象',labelFor(changeSubjects,candidate?.field_path||candidate?.target)],
      ['认识类别',labelFor(selfCategories,candidate?.category)],
      ['变化方向',labelFor(directions,candidate?.direction)],
      ['重要程度',scoreText(candidate?.importance)],
      ['情感意义',scoreText(candidate?.emotional_significance)],
      ...Object.entries(candidateTextFields).map(([key,fieldLabel])=>{
        const value=candidate?.[key];
        return [fieldLabel,Array.isArray(value)?value.map(reflectionText).filter(Boolean).join('、'):reflectionText(value)];
      }),
      ...Object.entries(candidateTimes).map(([key,fieldLabel])=>[fieldLabel,reflectionTime(candidate?.[key])]),
      ['触发条件',labelFor({time:'到达指定时间',event:'相关事件发生',semantic:'符合语义条件'},trigger?.type)],
      ['触发事件',labelFor(sourceKinds,trigger?.event_type)],
      ['触发时间',reflectionTime(trigger?.due_at)],
      ['目标进展',candidate?.complete===true?'提议标记为完成':''],
    ]);
    const reference=references.get(decision?.changed_ref)||references.get(decision?.target_ref)||references.get(candidate?.target_ref);
    const target=labelFor({memory:['growth/memory','查看记忆'],goal:['life/goal','查看目标']},reference?.kind);
    const link=target&&reference.entity_id
      ?`<a href="#/c/${encodeURIComponent(charId)}/${target[0]}/${encodeURIComponent(reference.entity_id)}">${target[1]}</a>`:'';
    if(facts||link)items.push(`<div data-domain-ui="item" class="lrow"><div class="grow"><b>${label}</b>${facts}${link}</div></div>`);
  }

  for(const decision of rows(record.dispositions).filter(Boolean)){
    const group=candidates.get(decision.domain);
    const match=group?.get(decision.candidate_index);
    group?.delete(decision.candidate_index);
    paint(match?.candidate,match?.label||labelFor(changeDomains,decision.domain)?.[0]||'其他变化',decision);
  }
  for(const group of candidates.values()){
    for(const {candidate,label} of group.values())paint(candidate,label);
  }
  return items.length?`<div data-domain-ui="card" class="card"><h3>提议的变化</h3>${items.join('')}</div>`:'';
}

export function renderRealReflection(el,ctx){
  const id=ctx.params.refId;
  return asyncPage(el,()=>reflectionClient.read(ctx.charId,id),(r,reload,alive)=>{
    const title=reflectionText(r.title);
    const summary=reflectionText(r.summary)||reflectionText(r.proposal?.summary);
    const body=reflectionText(r.body);
    const recordedAt=reflectionTime(r.at)||reflectionTime(r.created_at);
    const status=r.status||r.state||r.result?.status;
    const statusLabel=status?(labelFor(stateLabel,status)||domainStateLabel(status)):'';
    el.innerHTML=`<div class="stack reading-workspace">
      ${growthNavigation(ctx.charId,'reflection')}
      <h1 class="reading-business-title">${esc(title&&!/^反思窗口 \d+—\d+$/.test(title)?title:'反思详情')}</h1>
      <div data-domain-ui="card" class="card">
        <div class="row">${statusLabel?badge(statusLabel):''}<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-reload>重新读取</button></div>
        ${[summary,body!==summary?body:''].filter(Boolean).map(text=>`<p class="memory-content">${esc(text)}</p>`).join('')}
        ${reflectionFacts([['反思时间',recordedAt]])}
      </div>
      ${reflectionSources(r.source_window)}
      ${reflectionChanges(r,ctx.charId)}
      <div>${RawRecordDownload(r,'原始反思记录（诊断）')}</div>
      <div data-domain-ui="card" class="card"><div class="row"><h3 class="grow">你的反馈</h3><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-feedback="disagree">有不同看法</button><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-feedback="agree">认同</button></div><div data-feedback-list></div></div>
      <a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${encodeURIComponent(ctx.charId)}/growth/reflection">返回反思时间线</a>
    </div>`;
    const feedbacks=mountCursorRecords(el.querySelector('[data-feedback-list]'),{
      alive,read:cursor=>reflectionClient.feedback(ctx.charId,id,cursor),label:'反馈',
      empty:'尚无反馈。可以表达认同或不同看法，由她在真实认知中考虑。',
      render:request=>{
        const text=reflectionText(request.payload?.text);
        const metadata=[labelFor({agree:'认同',disagree:'有不同看法'},request.payload?.stance),reflectionTime(request.created_at)].filter(Boolean).join(' · ');
        const decision=requestDecisionText(request);
        return `<div data-domain-ui="item" class="lrow"><div class="grow">
          ${text?`<p>${esc(text)}</p>`:''}
          ${metadata?`<div class="small muted">${esc(metadata)}</div>`:''}
          ${decision?`<p>${esc(decision)}</p>`:''}
          ${RawRecordDownload(request,'原始反馈记录（诊断）')}
        </div>${request.status?badge(domainStateLabel(request.status)):''}<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="sm" data-request="${esc(request.id)}">查看处理</button></div>`;
      },
      bind:(box,page)=>box.querySelectorAll('[data-request]').forEach(button=>button.onclick=()=>requestReceipt(ctx.charId,{request:page.items.find(row=>row.id===button.dataset.request)},()=>feedbacks.reload({reset:true}),receiptOptions)),
    });
    el.querySelector('[data-reload]').onclick=reload;
    el.querySelectorAll('[data-feedback]').forEach(button=>button.onclick=()=>feedbackDialog(ctx.charId,id,button.dataset.feedback,()=>feedbacks.reload({reset:true})));
    return()=>feedbacks.dispose();
  },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/growth`,backLabel:'返回记忆成长'});
}

function feedbackDialog(fid,id,stance,onAccepted){
  const d=drawer({title:stance==='agree'?'认同这次反思':'有不同看法',body:`<p class="small muted">反馈进入真实认知处理，不直接改写人格、记忆或已发生历史。</p>${Field({label:'想告诉她',name:'feedback',type:'textarea',attrs:{required:true,maxlength:2000}})}<p data-domain-ui="alert" role="alert" data-error></p>`,footer:'<button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-close>取消</button><button data-domain-ui="button" data-domain-variant="default" data-domain-size="default" data-submit>提交反馈</button>'});
  d.el.querySelector('[data-close]').onclick=()=>d.close();
  d.el.querySelector('[data-submit]').onclick=async event=>{const field=d.el.querySelector('[name=feedback]');if(!field.reportValidity())return;event.target.disabled=true;try{const request=await reflectionClient.submit(fid,id,field.value,stance);d.close();await onAccepted();requestReceipt(fid,request,onAccepted,receiptOptions);}catch(error){if(d.el.isConnected){d.el.querySelector('[data-error]').textContent=errorMessage(error);event.target.disabled=false;}}};
}
