import {requestDecisionText,RequestDecisionSource} from '../v3/proto/components/request-failure.js';
import {errorMessage} from '../v3/domain-presentation.mjs';
import {ReflectionProgress} from '../v3/proto/components/reflection-progress.js';
import {reflectionClient} from '../v3/proto/services.js';
import {asyncPage,requestReceipt,instant} from '../v3/proto/components/async-page.js';
import {CursorPager,bindCursorPager} from '../v3/proto/components/cursor-pager.js';
import {mountCursorRecords} from '../v3/proto/components/cursor-records.js';
import {JsonDetails} from '../v3/proto/components/records.js';
import {Field} from '../v3/proto/components/forms.js';
import {stateLabel as domainStateLabel} from '../v3/domain-presentation.mjs';
import {esc,badge,drawer} from '../v3/proto/ui.js';

const growthNavigation=(fid,current)=>`<nav class="reading-module-nav" aria-label="记忆成长">${[['memory','记忆',`#/c/${fid}/growth`],['reflection','反思',`#/c/${fid}/growth/reflection`],['self','成长认识',`#/c/${fid}/growth/self`],['snapshot','人格快照',`#/c/${fid}/growth/self?view=snapshot`]].map(([key,label,href])=>`<a href="${href}" ${current===key?'aria-current="page"':''}>${label}</a>`).join('')}</nav>`;

const href=(fid,id)=>`#/c/${encodeURIComponent(fid)}/growth/reflection/${encodeURIComponent(id)}`;
const stateLabel={applied:'已应用',no_change:'无变化',proposed:'历史提案'};
const receiptOptions={title:'反思反馈',labels:{completed:'反馈已处理',failed:'反馈处理失败'},note:'反馈由实际认知任务处理。认同不强制成长，不同看法不直接更正事实；关闭仅停止观察。'};

export function renderRealReflections(el,ctx){
 const cursors=[''];let at=0;
 return asyncPage(el,()=>reflectionClient.list(ctx.charId,cursors[at]),(initial,reload,alive)=>{
  let page=initial,timer,reads=0,busy=false,failure='';const taskId=page.latest_task?.intent_id;
  el.innerHTML=`<div class="stack reading-workspace"><h1 class="reading-business-title">反思与成长</h1>${growthNavigation(ctx.charId,'reflection')}<section class="stack" data-reflection-results></section><details data-domain-ui="disclosure" class="card" data-progress><summary>这次反思的处理进度</summary><div data-reflection-progress></div></details><div data-reflection-pager></div><p data-domain-ui="alert" data-observe-error role="status"></p><div class="row"><button data-domain-ui="button" class="btn ghost" data-reload>重新读取</button><a data-domain-ui="button" class="btn" href="#/c/${ctx.charId}/growth/self">查看成长认识</a><a data-domain-ui="button" class="btn" href="#/c/${ctx.charId}/growth">她的记忆</a></div></div>`;
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

export function renderRealReflection(el,ctx){
  const id=ctx.params.refId;
  return asyncPage(el,()=>reflectionClient.read(ctx.charId,id),(r,reload,alive)=>{
    const changed=r.dispositions.filter(d=>d.disposition==='accepted');
    const link=ref=>{const entry=r.references.find(row=>row.ref===ref);if(entry?.kind==='memory')return `<a href="#/c/${ctx.charId}/growth/memory/${encodeURIComponent(entry.entity_id)}">查看记忆 ${esc(entry.entity_id)}</a>`;if(entry?.kind==='goal')return `<a href="#/c/${ctx.charId}/life/goal/${encodeURIComponent(entry.entity_id)}">查看目标 ${esc(entry.entity_id)}</a>`;return esc(ref||'没有目标引用');};
    el.innerHTML=`<div class="stack reading-workspace">${growthNavigation(ctx.charId,'reflection')}<h1 class="reading-business-title">${esc(r.title)}</h1><div data-domain-ui="card" class="card"><div class="row">${badge(stateLabel[r.status]||r.status)}<button data-domain-ui="button" class="btn" data-reload>重新读取</button></div><p class="memory-content">${esc(r.summary ?? '这条旧记录没有摘要')}</p><p class="small muted">证据序列 ${esc(r.from_sequence)}—${esc(r.to_sequence)} · ${esc(instant(r.at))} · ${esc(r.schema_version)}</p><p class="small muted">${changed.length?`有 ${changed.length} 个候选被接受，具体领域与版本见下方。`:'这次没有形成新的改变，具体原因保留在下方过程记录中。'}</p>${JsonDetails(r.result,'已保存的变化与来源')}</div><div data-domain-ui="card" class="card"><h3>这次反思带来的变化</h3>${r.dispositions.map(d=>`<div data-domain-ui="item" class="lrow"><div class="grow"><b>${esc(d.domain)}</b><p>${link(d.changed_ref||d.target_ref)}</p><div class="small muted">${esc(d.reason_code)} · 版本 ${esc(d.base_revision)} → ${esc(d.resulting_revision)}</div>${JsonDetails(d,'候选身份与来源')}</div>${badge(d.disposition)}</div>`).join('')||'<p class="small muted">本次没有候选处置记录；请同时核对格式版本和原始提案。</p>'}${JsonDetails(r.proposal,'完整 V2 提案')}</div><div data-domain-ui="card" class="card"><h3>输入窗口与来源</h3>${r.source_window_status==='available'?JsonDetails(r.source_window,'实际输入窗口、事实与结果'):'<p class="small muted">旧记录没有可读取的窗口快照，不能补造来源。</p>'}${JsonDetails(r.references,'范围内的引用与版本')}</div><div data-domain-ui="card" class="card"><div class="row"><h3 class="grow">你的反馈</h3><button data-domain-ui="button" class="btn" data-feedback="disagree">有不同看法</button><button data-domain-ui="button" class="btn" data-feedback="agree">认同</button></div><div data-feedback-list></div></div><a data-domain-ui="button" class="btn" href="#/c/${ctx.charId}/growth/reflection">返回反思时间线</a></div>`;
    const feedbacks=mountCursorRecords(el.querySelector('[data-feedback-list]'),{alive,read:cursor=>reflectionClient.feedback(ctx.charId,id,cursor),label:'反馈',empty:'尚无反馈。可以表达认同或不同看法，由她在真实认知中考虑。',render:request=>`<div data-domain-ui="item" class="lrow"><div class="grow"><p>${esc(request.payload?.text)}</p><div class="small muted">${esc(({agree:'认同',disagree:'有不同看法'})[request.payload?.stance]||'未标注态度')} · ${esc(instant(request.created_at))}</div><p>${esc(requestDecisionText(request))}</p>${RequestDecisionSource(request)}</div>${badge(domainStateLabel(request.status))}<button data-domain-ui="button" class="btn sm" data-request="${esc(request.id)}">查看处理</button></div>`,bind:(box,page)=>box.querySelectorAll('[data-request]').forEach(button=>button.onclick=()=>requestReceipt(ctx.charId,{request:page.items.find(row=>row.id===button.dataset.request)},()=>feedbacks.reload({reset:true}),receiptOptions))});
    el.querySelector('[data-reload]').onclick=reload;
    el.querySelectorAll('[data-feedback]').forEach(button=>button.onclick=()=>feedbackDialog(ctx.charId,id,button.dataset.feedback,()=>feedbacks.reload({reset:true})));
    return()=>feedbacks.dispose();
  },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/growth`,backLabel:'返回记忆成长'});
}

function feedbackDialog(fid,id,stance,onAccepted){
  const d=drawer({title:stance==='agree'?'认同这次反思':'有不同看法',body:`<p class="small muted">反馈进入真实认知处理，不直接改写人格、记忆或已发生历史。</p>${Field({label:'想告诉她',name:'feedback',type:'textarea',attrs:{required:true,maxlength:2000}})}<p data-domain-ui="alert" role="alert" data-error></p>`,footer:'<button data-domain-ui="button" class="btn" data-close>取消</button><button data-domain-ui="button" class="btn primary" data-submit>提交反馈</button>'});
  d.el.querySelector('[data-close]').onclick=()=>d.close();
  d.el.querySelector('[data-submit]').onclick=async event=>{const field=d.el.querySelector('[name=feedback]');if(!field.reportValidity())return;event.target.disabled=true;try{const request=await reflectionClient.submit(fid,id,field.value,stance);d.close();await onAccepted();requestReceipt(fid,request,onAccepted,receiptOptions);}catch(error){if(d.el.isConnected){d.el.querySelector('[data-error]').textContent=errorMessage(error);event.target.disabled=false;}}};
}
