import {reflectionClient} from '../services.js';
import {asyncPage,requestReceipt,instant} from '../components/async-page.js';
import {CursorPager,bindCursorPager} from '../components/cursor-pager.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {JsonDetails} from '../components/records.js';
import {Field} from '../components/forms.js';
import {stateLabel as domainStateLabel} from '../../domain-presentation.mjs';
import {esc,badge,drawer} from '../ui.js';

const href=(fid,id)=>`#/c/${encodeURIComponent(fid)}/growth/reflection/${encodeURIComponent(id)}`;
const stateLabel={applied:'已应用',no_change:'无变化',proposed:'历史提案'};
const receiptOptions={title:'反思反馈',labels:{completed:'反馈已处理',failed:'反馈处理失败'},note:'反馈由实际认知任务处理。认同不强制成长，不同看法不直接更正事实；关闭仅停止观察。'};

export function renderRealReflections(el,ctx){
  const cursors=[''];let at=0;
  return asyncPage(el,()=>reflectionClient.list(ctx.charId,cursors[at]),(page,reload)=>{
    el.innerHTML=`<div class="stack"><div class="card"><div class="row"><h3 class="grow">最近的反思进度</h3><button class="btn" data-reload>重新读取</button></div>${page.window?`<p>${badge(domainStateLabel(page.window.status))}</p>${JsonDetails(page.window,'窗口与领域版本')}`:'<p class="small muted">尚无已持久化的反思窗口。继续交流或生活后，系统按真实证据触发。</p>'}${page.latest_task?`<p class="small muted">最近一次处理：${esc(domainStateLabel(page.latest_task.status))} · 已尝试 ${esc(page.latest_task.attempt_count)} 次</p>${JsonDetails(page.latest_task,'最近任务状态')}`:'<p class="small muted">尚无反思任务记录。</p>'}<p class="small muted">她会结合新的交流与经历进行反思；一次反思也可能没有新的变化。</p></div><div class="card"><div class="list">${page.items.map(r=>`<a class="lrow click" href="${href(ctx.charId,r.id)}"><div class="grow"><b>${esc(r.title)}</b><p>${esc(r.summary ?? '旧记录没有摘要')}</p><div class="small muted">${esc(instant(r.at))} · ${esc(r.schema_version)}</div></div>${badge(stateLabel[r.status]||r.status)}</a>`).join('')||'<p class="small muted">当前没有反思提案。可以继续交流和生活；窗口或任务状态会说明是否尚待处理。</p>'}</div></div>${CursorPager({more:!!page.next_cursor,previous:at>0,label:'反思'})}<div class="row"><a class="btn" href="#/c/${ctx.charId}/growth">记忆库</a><a class="btn" href="#/c/${ctx.charId}/growth/self">自我认知</a></div></div>`;
    el.querySelector('[data-reload]').onclick=reload;bindCursorPager(el,{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});
  });
}

export function renderRealReflection(el,ctx){
  const id=ctx.params.refId;
  return asyncPage(el,()=>reflectionClient.read(ctx.charId,id),(r,reload,alive)=>{
    const changed=r.dispositions.filter(d=>d.disposition==='accepted');
    const link=ref=>{const entry=r.references.find(row=>row.ref===ref);if(entry?.kind==='memory')return `<a href="#/c/${ctx.charId}/growth/memory/${encodeURIComponent(entry.entity_id)}">查看记忆 ${esc(entry.entity_id)}</a>`;if(entry?.kind==='goal')return `<a href="#/c/${ctx.charId}/life/goal/${encodeURIComponent(entry.entity_id)}">查看目标 ${esc(entry.entity_id)}</a>`;return esc(ref||'没有目标引用');};
    el.innerHTML=`<div class="stack"><div class="card"><div class="row"><h3 class="grow">${esc(r.title)}</h3>${badge(stateLabel[r.status]||r.status)}<button class="btn" data-reload>重新读取</button></div><p class="memory-content">${esc(r.summary ?? '旧记录没有摘要')}</p><p class="small muted">证据序列 ${esc(r.from_sequence)}—${esc(r.to_sequence)} · ${esc(instant(r.at))} · ${esc(r.schema_version)}</p><p class="small muted">${changed.length?`有 ${changed.length} 个候选被接受，具体领域与版本见下方。`:'没有候选被接受不等于运行失败；以真实处置原因和结果为准。'}</p>${JsonDetails(r.result,'实际应用结果与领域版本')}</div><div class="card"><h3>候选评估与应用</h3>${r.dispositions.map(d=>`<div class="lrow"><div class="grow"><b>${esc(d.domain)}</b><p>${link(d.changed_ref||d.target_ref)}</p><div class="small muted">${esc(d.reason_code)} · 版本 ${esc(d.base_revision)} → ${esc(d.resulting_revision)}</div>${JsonDetails(d,'候选身份与来源')}</div>${badge(d.disposition)}</div>`).join('')||'<p class="small muted">本次没有候选处置记录；请同时核对格式版本和原始提案。</p>'}${JsonDetails(r.proposal,'完整 V2 提案')}</div><div class="card"><h3>输入窗口与来源</h3>${r.source_window_status==='available'?JsonDetails(r.source_window,'实际输入窗口、事实与结果'):'<p class="small muted">旧记录没有可读取的窗口快照，不能补造来源。</p>'}${JsonDetails(r.references,'范围内的引用与版本')}</div><div class="card"><div class="row"><h3 class="grow">你的反馈</h3><button class="btn" data-feedback="disagree">有不同看法</button><button class="btn" data-feedback="agree">认同</button></div><div data-feedback-list></div></div><a class="btn" href="#/c/${ctx.charId}/growth/reflection">返回反思时间线</a></div>`;
    const feedbacks=mountCursorRecords(el.querySelector('[data-feedback-list]'),{alive,read:cursor=>reflectionClient.feedback(ctx.charId,id,cursor),label:'反馈',empty:'尚无反馈。可以表达认同或不同看法，由她在真实认知中考虑。',render:request=>`<div class="lrow"><div class="grow"><p>${esc(request.payload?.text)}</p><div class="small muted">${esc(request.payload?.stance||'未标注态度')} · ${esc(instant(request.created_at))}</div><p>${esc(request.decision_reason||'')}</p></div>${badge(request.status)}<button class="btn sm" data-request="${esc(request.id)}">查看处理</button></div>`,bind:(box,page)=>box.querySelectorAll('[data-request]').forEach(button=>button.onclick=()=>requestReceipt(ctx.charId,{request:page.items.find(row=>row.id===button.dataset.request)},()=>feedbacks.reload({reset:true}),receiptOptions))});
    el.querySelector('[data-reload]').onclick=reload;
    el.querySelectorAll('[data-feedback]').forEach(button=>button.onclick=()=>feedbackDialog(ctx.charId,id,button.dataset.feedback,()=>feedbacks.reload({reset:true})));
    return()=>feedbacks.dispose();
  });
}

function feedbackDialog(fid,id,stance,onAccepted){
  const d=drawer({title:stance==='agree'?'认同这次反思':'有不同看法',body:`<p class="small muted">反馈进入真实认知处理，不直接改写人格、记忆或已发生历史。</p>${Field({label:'想告诉她',name:'feedback',type:'textarea',attrs:{required:true,maxlength:2000}})}<p role="alert" data-error></p>`,footer:'<button class="btn" data-close>取消</button><button class="btn primary" data-submit>提交反馈</button>'});
  d.el.querySelector('[data-close]').onclick=()=>d.close();
  d.el.querySelector('[data-submit]').onclick=async event=>{const field=d.el.querySelector('[name=feedback]');if(!field.reportValidity())return;event.target.disabled=true;try{const request=await reflectionClient.submit(fid,id,field.value,stance);d.close();await onAccepted();requestReceipt(fid,request,onAccepted,receiptOptions);}catch(error){if(d.el.isConnected){d.el.querySelector('[data-error]').textContent=error.message;event.target.disabled=false;}}};
}
