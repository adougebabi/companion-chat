import {sourceHref,sourceReturn} from '../components/source-navigation.js';
import {mountCalendarReading,localDate,reconcileReadingRecord} from '../components/calendar-reading.js';
import { memoryClient } from '../services.js';
import { asyncPage,instant } from '../components/async-page.js';
import { CursorPager,bindCursorPager } from '../components/cursor-pager.js';
import { mountCursorRecords } from '../components/cursor-records.js';
import { Field,Tabs,SearchForm,DiffView } from '../components/forms.js';
import { JsonDetails } from '../components/records.js';
import { esc,badge,drawer,toast,requireOwner } from '../ui.js';
import { get } from '../store.js';

const types={'':'全部',episodic:'经历',semantic:'知识',relationship:'关系',autobiographical:'自我'};
const statuses={'':'全部状态',active:'有效',superseded:'已替代',deprecated:'已废弃',forgotten:'已忘却'};
const memoryHref=(fid,id)=>`#/c/${encodeURIComponent(fid)}/growth/memory/${encodeURIComponent(id)}`;
// Unsaved form data only; no second copy of authoritative memory state.
const drafts=new Map();
const draftKey=memory=>JSON.stringify([get('session')?.actorId,memory.owner_fluctlight_id,memory.id]);

export function renderRealMemoryList(el,ctx) {
  const aboutOwner=ctx.query.aboutOwner==='true';
  const type=Object.hasOwn(types,ctx.query.type)?ctx.query.type:'',status=aboutOwner?'active':Object.hasOwn(statuses,ctx.query.status)?ctx.query.status:'active',q=ctx.query.q||'',zone=get('characters',ctx.charId)?.timezone;
  const params={type,status,q,...(aboutOwner?{aboutOwner:'true'}:{}),month:ctx.query.month||'',date:ctx.query.date||''};
  const link=patch=>`#/c/${ctx.charId}/growth?${new URLSearchParams({...params,...patch})}`;
  return mountCalendarReading(el,{scope:[ctx.charId,'memory',type,status,q,aboutOwner],zone,initialDate:params.date,initialMonth:params.month,onView:view=>{Object.assign(params,view);ctx.raw=ctx.path+'?'+new URLSearchParams(params);history.replaceState(null,'','#'+ctx.raw);el.querySelectorAll('[data-memory-type]').forEach(a=>a.href=link({type:a.dataset.memoryType}));el.querySelectorAll('[data-memory-status]').forEach(a=>a.href=link({status:a.dataset.memoryStatus}));},read:cursor=>memoryClient.list(ctx.charId,{type,status,q,aboutOwner,cursor,limit:30}),dateOf:m=>localDate(m.created_at,zone),label:'记忆',
   toolbar:`<div class="stack">${aboutOwner?'<h2>关于你的记忆</h2><p class="small muted">只读取提及当前所有者且来源仍有效的当前记忆。</p>':''}${Tabs({value:type,items:Object.entries(types).map(([value,label])=>({value,label,href:link({type:value}),attrs:{'data-memory-type':value}}))})}${aboutOwner?'':Tabs({value:status,items:Object.entries(statuses).map(([value,label])=>({value,label,href:link({status:value}),attrs:{'data-memory-status':value}}))})}${SearchForm({value:q,placeholder:'搜索记忆正文',attrs:{'data-memory-search':true},inputAttrs:{'data-test':'mem-search'}})}<div class="row"><a href="#/c/${ctx.charId}/growth/reflection">反思与成长</a><a href="#/c/${ctx.charId}/growth/self">成长认识</a></div></div>`,
   bindToolbar:box=>{box.querySelector('[data-memory-search]').onsubmit=e=>{e.preventDefault();ctx.navigate(link({q:e.target.q.value}));};},
   render:(m,date)=>`<article class="reading-paper" data-test="mem-${esc(m.id)}"><div class="row"><span class="paper-date grow">${date?'记于 '+esc(date):'记忆时间未记录'}</span>${badge(statuses[m.status]||m.status,'plain')}</div><p class="small muted">${esc(types[m.type]||m.type)} · 第 ${esc(m.revision)} 版 · 来源 ${esc(m.provenance_status)}</p><p class="paper-body">${esc(m.content)}</p><a class="btn" href="${esc(sourceHref(memoryHref(ctx.charId,m.id),ctx,aboutOwner?'返回关于你的记忆':'返回她的记忆'))}">阅读与来源</a></article>`});
}

export function renderRealMemory(el,ctx) {
  const id=ctx.params.memId;
  return asyncPage(el,()=>memoryClient.read(ctx.charId,id),(memory,reload,alive)=>{
    reconcileReadingRecord(ctx.charId,'memory',memory,(scope,row)=>(!scope[2]||scope[2]===row.type)&&(!scope[3]||scope[3]===row.status)&&(!scope[4]||row.content.toLocaleLowerCase().includes(scope[4].toLocaleLowerCase()))&&(!scope[5]||(!['pending','invalid'].includes(row.provenance_status)&&row.actor_refs?.includes(get('session')?.actorId))));
    let disposed=false;
    el.innerHTML=`<div class="stack"><div class="reading-paper"><div class="row"><span class="grow small muted">${esc(types[memory.type])}</span>${badge(statuses[memory.status])}<span class="small muted">第 ${esc(memory.revision)} 版</span></div><p class="memory-content" data-test="memory-content">${esc(memory.content)}</p><p class="small muted">来源状态 ${esc(memory.provenance_status)} · 可见范围 ${esc(memory.visibility)} · 发生时间 ${esc(instant(memory.occurred_at))}</p>${memory.provenance_status==='invalid'||memory.provenance_status==='pending'?'<p class="banner warn">来源尚未有效，不能将这条记忆当作当前已确认事实。</p>':''}<div class="row">${memory.status==='active'?'<button class="btn accent" data-edit data-test="mem-edit">所有者修改…</button><button class="btn" data-forget>忘却…</button>':''}<button class="btn" data-reload>重新读取</button></div><div data-editor></div>${JsonDetails({actor_refs:memory.actor_refs,event_refs:memory.event_refs,personality_perspectives:memory.personality_perspectives,confidence:memory.confidence,importance:memory.importance,emotional_significance:memory.emotional_significance,supersedes_memory_id:memory.supersedes_memory_id,superseded_by_memory_id:memory.superseded_by_memory_id},'范围、人格理解与谱系')}</div><div class="card"><h3>来源</h3><div data-sources role="status">正在读取来源…</div></div><div class="card"><h3>修订记录</h3><div data-revisions role="status">正在读取修订…</div></div><div class="row"><a class="btn" href="${esc(sourceReturn(ctx,`#/c/${ctx.charId}/growth`))}">${esc(ctx.query.returnLabel||'返回记忆库')}</a><a class="btn" href="#/c/${ctx.charId}/people/owner">关于你</a></div></div>`;
    const current=()=>!disposed&&alive();
    const sourceBox=el.querySelector('[data-sources]'),revisionBox=el.querySelector('[data-revisions]');
    function sourceLink(source){if(source.source_kind==='message'&&source.conversation_id)return `<a href="#/c/${ctx.charId}/chat?${new URLSearchParams({conversation:source.conversation_id,message:source.source_id})}">查看原消息</a>`;return `<span class="small muted">${esc(source.source_ref)}</span>`;}
    let sourceRevision=memory.revision;
    const sourceLabel=document.createElement('p');sourceLabel.className='small muted';sourceLabel.textContent=`第 ${sourceRevision} 版的来源`;sourceBox.before(sourceLabel);
    const sources=mountCursorRecords(sourceBox,{alive:current,label:'来源',empty:'本版没有可追溯来源，保留其未知状态。',read:cursor=>memoryClient.sources(ctx.charId,id,sourceRevision,cursor),render:source=>`<div class="lrow"><div class="grow">${sourceLink(source)}<div class="small muted">${esc(source.source_kind)} · ${esc(instant(source.occurred_at))}${source.superseded_by_ref?' · 已被 '+esc(source.superseded_by_ref)+' 替代':''}</div></div>${badge(source.live?'来源有效':'来源失效或未知')}</div>`});
    const revisions=mountCursorRecords(revisionBox,{alive:current,label:'修订',empty:'尚无修订记录。',read:cursor=>memoryClient.revisions(ctx.charId,id,cursor),render:r=>`<div class="lrow" data-test="rev"><b>v${esc(r.revision)}</b><div class="grow"><p>${esc(r.content)}</p><div class="small muted">${esc(r.operation)} · ${esc(r.semantic_reason)} · ${esc(instant(r.created_at))}</div>${JsonDetails(r,'修订来源与处置')}</div><button class="btn sm" data-source-revision="${esc(r.revision)}">第 ${esc(r.revision)} 版来源</button>${r.status==='active'&&r.revision<memory.revision?`<button class="btn sm" data-rollback="${esc(r.revision)}">补偿恢复至此版…</button>`:''}</div>`,bind:box=>{
      box.querySelectorAll('[data-rollback]').forEach(button=>button.onclick=()=>governMemory(memory,'rollback',Number(button.dataset.rollback),reload));
      box.querySelectorAll('[data-source-revision]').forEach(button=>button.onclick=()=>{sourceRevision=Number(button.dataset.sourceRevision);sourceLabel.textContent=`第 ${sourceRevision} 版的来源`;void sources.reload({reset:true});});
    }});
    el.querySelector('[data-reload]').onclick=reload;
    el.querySelector('[data-edit]')?.addEventListener('click',()=>editMemory(el.querySelector('[data-editor]'),memory,reload));
    if(drafts.has(draftKey(memory)))editMemory(el.querySelector('[data-editor]'),memory,reload);
    el.querySelector('[data-forget]')?.addEventListener('click',()=>governMemory(memory,'forget',null,reload));
    return()=>{disposed=true;sources.dispose();revisions.dispose();};
  });
}

function editMemory(box,memory,reload) {
  const key=draftKey(memory),draft=drafts.get(key)||{content:memory.content,reason:'',baseRevision:memory.revision};drafts.set(key,draft);
  const needsReview=()=>draft.baseRevision!==memory.revision;
  box.innerHTML=`<form class="card tint-accent stack">${needsReview()?`<p class="banner warn">已重新读取第 ${esc(memory.revision)} 版，保留了第 ${esc(draft.baseRevision)} 版上的草稿。请核对当前内容与草稿，再确认使用新基线。</p><button class="btn" type="button" data-rebase>已核对，使用第 ${esc(memory.revision)} 版作为基线</button>`:''}${Field({label:'修改为',name:'content',value:draft.content,type:'textarea',attrs:{required:true,maxlength:4096,'data-test':'mem-draft'}})}${Field({label:'修订说明',name:'reason',value:draft.reason,attrs:{required:true,maxlength:150}})}<div data-diff></div><p class="banner err" role="alert" data-error hidden></p><div class="row"><button class="btn" type="button" data-cancel>取消</button><button class="btn accent" data-test="mem-save" ${needsReview()?'disabled':''}>保存修改</button></div></form>`;
  const form=box.querySelector('form');
  const paint=()=>{draft.content=form.elements.content.value;draft.reason=form.elements.reason.value;box.querySelector('[data-diff]').innerHTML=DiffView([{field:'当前有效版本 → 未提交草稿',before:memory.content,after:draft.content}],{attrs:{'data-test':'mem-diff'}});};paint();form.oninput=paint;
  box.querySelector('[data-rebase]')?.addEventListener('click',()=>{draft.baseRevision=memory.revision;editMemory(box,memory,reload);});
  box.querySelector('[data-cancel]').onclick=()=>{drafts.delete(key);box.innerHTML='';};
  if(memory.status!=='active'){form.querySelector('[data-test=mem-save]').disabled=true;box.insertAdjacentHTML('afterbegin','<p class="banner warn">当前记忆已不处于有效状态，草稿仍保留。请先核对其生命周期和恢复记录。</p>');}
  form.onsubmit=async event=>{event.preventDefault();if(needsReview()||memory.status!=='active'||!(await requireOwner()))return;const button=form.querySelector('[data-test=mem-save]');button.disabled=true;try{await memoryClient.revise(memory.owner_fluctlight_id,memory,draft.content,draft.reason);drafts.delete(key);toast('记忆已修订并读回；原消息保留','ok');await reload();}catch(error){const message=box.querySelector('[data-error]');message.hidden=false;message.textContent=error.status===409?'记忆版本已变化，草稿已保留。重新读取并核对后再保存。':error.message;button.disabled=false;}};
}

function governMemory(memory,operation,target,reload) {
  let base=memory;
  const label=operation==='forget'?'忘却记忆':`补偿恢复至第 ${target} 版`;
  const d=drawer({title:label,body:`<p data-current-content>${esc(memory.content)}</p><p class="small muted">新增一条治理修订，保留原始历史。回滚不会撤销其他领域的历史效果。</p>${Field({label:'操作说明',name:'reason',attrs:{required:true,maxlength:150}})}<p role="alert" data-error></p><button class="btn" data-read-current hidden>读取最新版本后重新确认</button>`,footer:'<button class="btn" data-close>取消</button><button class="btn accent" data-confirm>确认</button>'});
  d.el.querySelector('[data-close]').onclick=()=>d.close();
  d.el.querySelector('[data-read-current]').onclick=async event=>{event.target.disabled=true;try{const latest=await memoryClient.read(memory.owner_fluctlight_id,memory.id);if(!d.el.isConnected)return;base=latest;d.el.querySelector('[data-current-content]').textContent=latest.content;d.el.querySelector('[data-error]').textContent=`已读取第 ${latest.revision} 版，操作说明仍保留，请核对后再次确认。`;event.target.hidden=true;}catch(error){if(d.el.isConnected)d.el.querySelector('[data-error]').textContent=error.message;}finally{event.target.disabled=false;}};
  d.el.querySelector('[data-confirm]').onclick=async event=>{if(!(await requireOwner()))return;const reason=d.el.querySelector('[name=reason]');if(!reason.reportValidity())return;event.target.disabled=true;try{if(operation==='forget')await memoryClient.forget(memory.owner_fluctlight_id,base,reason.value);else await memoryClient.rollback(memory.owner_fluctlight_id,base,target,reason.value);d.close();toast('治理修订已保存并读回','ok');await reload();}catch(error){if(d.el.isConnected){d.el.querySelector('[data-error]').textContent=error.status===409?'版本或谱系已变化，说明已保留，请重新读取后核对。':error.message;if(error.status===409)d.el.querySelector('[data-read-current]').hidden=false;event.target.disabled=false;}}};
}
