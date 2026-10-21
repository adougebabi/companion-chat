import {diaryClient} from '../services.js';
import * as store from '../store.js';
import {asyncPage,instant,requestReceipt} from '../components/async-page.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {CursorPager,bindCursorPager} from '../components/cursor-pager.js';
import {Field,Tabs,Toggle} from '../components/forms.js';
import {JsonDetails} from '../components/records.js';
import {ConflictPanel} from '../components/revisions.js';
import {beginDraftSubmission,settleDraftSubmission} from '../components/draft-submission.js';
import {esc,badge,confirm} from '../ui.js';

const labels={pending:'等待日终或执行',running:'正在生成',ready:'已生成',no_content:'本次没有日记',failed:'生成失败',inactive:'角色未处于运行状态'};
const entryHref=(fid,id)=>`#/c/${encodeURIComponent(fid)}/life/diary/${encodeURIComponent(id)}`;
const drafts=new Map();store.subscribe(()=>{if(!store.get('session'))drafts.clear();});

export function renderRealDiaryList(el,ctx){
 const cursors=[''];let at=0;const favorite=ctx.query.favorite==='true';
 return asyncPage(el,()=>diaryClient.list(ctx.charId,cursors[at],favorite),(page,reload,alive)=>{
  el.innerHTML=`<div class="stack diary-list"><p class="small muted">${badge('角色主观记录','plain')} 日记保留她自己的表达，事实来源和你的注记分别记录。</p><div class="row">${Tabs({value:favorite?'favorite':'all',items:[{value:'all',label:'全部',href:`#/c/${ctx.charId}/life/diary`},{value:'favorite',label:'收藏',href:`#/c/${ctx.charId}/life/diary?favorite=true`}]})}<button class="btn" data-reload>重新读取</button></div>${page.items.map(d=>`<a class="card tint-sun" style="display:block" href="${entryHref(ctx.charId,d.id)}"><div class="row"><b class="grow">${esc(d.date)}</b>${d.annotation.favorite?badge('已收藏'):''}${badge(labels[d.status]||d.status)}</div><p>${esc(d.body?.slice(0,200)||'尚无正文，打开查看生产状态与来源。')}</p><span class="small muted">${esc(d.timezone)} · 打开 →</span></a>`).join('')||`<div class="statebox"><h2>${favorite?'还没有收藏的日记':'还没有角色日记'}</h2><p>日终处理使用真实交流和生活素材；也可以在下方选择已结束的本地日期发起生成。没有素材和生成失败分别显示。</p></div>`}${CursorPager({more:!!page.next_cursor,previous:at>0,label:'日记'})}<div class="grid g2"><form class="card stack" data-generate><h3>回顾一个已结束的本地日期</h3>${Field({label:'日期',name:'localDate',type:'date',attrs:{required:true}})}${Field({label:'IANA 时区',name:'timezone',value:page.timezone,attrs:{required:true}})}<p class="small muted">自动生成在日程接受时登记，于相应日期结束后执行。这里只受理任务，不预填正文；重复提交返回同一日记。</p><button class="btn primary">发起日记生成</button><p data-feedback role="alert"></p></form><div class="card"><h3>与你的共同经历</h3><p>真实交流属于远程共同经历；只有明确的双方参与证据才能标同场。</p><a class="btn" href="#/c/${ctx.charId}/life/shared">共同经历时间线</a></div></div></div>`;
  el.querySelector('[data-reload]').onclick=reload;bindCursorPager(el,{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});
  el.querySelector('[data-generate]').onsubmit=async event=>{event.preventDefault();const form=event.target,button=form.querySelector('button');button.disabled=true;try{const result=await diaryClient.generate(ctx.charId,{localDate:form.localDate.value,timezone:form.timezone.value.trim(),reason:'Owner从日记页请求回顾已结束日期'});if(alive())ctx.navigate(entryHref(ctx.charId,result.diary_id));}catch(error){if(alive()){form.querySelector('[data-feedback]').textContent=error.message;button.disabled=false;}}};
 });
}

function sourceRows(fid,entry){
 const wanted=new Set(entry.refs);
 return (entry.source_snapshot?.sources||[]).filter(s=>wanted.has(s.ref)).map(source=>{
  const id=source.ref.replace(/^[^:]+:/,'');
  const link=source.kind==='remote_communication'?`#/c/${encodeURIComponent(fid)}/chat?${new URLSearchParams({conversation:source.conversation_id,message:id})}`:`#/c/${encodeURIComponent(fid)}/world/event/${encodeURIComponent(id)}`;
  return `<div class="lrow"><a class="grow" href="${esc(link)}">${esc(source.kind==='remote_communication'?'原交流消息':'已确认生活事件')} ${esc(id)}</a>${source.kind==='remote_communication'?badge('远程交流','plain'):''}${JsonDetails(source,'查看原来源快照')}</div>`;
 }).join('')||'<p class="small muted">还没有已提交正文的引用来源；生成尝试和没有产出的原因见下方。</p>';
}
export function renderRealDiaryEntry(el,ctx){
 const fid=ctx.charId,id=ctx.params.entryId,key=JSON.stringify([store.get('session')?.actorId,fid,id]);
 return asyncPage(el,()=>diaryClient.read(fid,id),(entry,reload,alive)=>{
  if(!drafts.has(key))drafts.set(key,{note:entry.annotation.note,favorite:entry.annotation.favorite,base:entry.annotation.revision,reply:'',dirty:false,pending:{}});
  const draft=drafts.get(key);
  if(!draft.dirty&&!draft.pending.note&&entry.annotation.revision>=draft.base)Object.assign(draft,{note:entry.annotation.note,favorite:entry.annotation.favorite,base:entry.annotation.revision});
  const conflict=draft.base<entry.annotation.revision;
  el.innerHTML=`<div class="stack"><div class="card tint-sun"><div class="row diary-entry-head"><h2 class="grow">${esc(entry.date)}</h2>${badge('角色主观记录','plain')}${badge(labels[entry.status]||entry.status)}<button class="btn" data-reload>重新读取</button></div><p class="small muted">${esc(entry.timezone)} · 正文来源：独立人物表达 · v${esc(entry.revision)}</p>${entry.body?`<p class="memory-content">${esc(entry.body)}</p>`:`<p>${entry.status==='no_content'?'这次没有足够或适合的素材形成日记。可以继续交流、生活，之后回顾相应日期。':'尚无已提交正文。请查看实际任务和状态，受理不代表已经生成。'}</p>`}${entry.last_error_code?`<p class="banner err" role="alert">${esc(entry.last_error_code)}</p>`:''}${['failed','inactive'].includes(entry.status)?'<button class="btn" data-retry>按当前状态重新生成</button>':''}<p data-generation-feedback role="alert"></p></div><div class="card"><h3>素材与来源</h3>${sourceRows(fid,entry)}${JsonDetails(entry.persona_provenance,'本次表达所用人格版本')}${JsonDetails(entry.model_provenance,'模型来源与关联标识')}</div><div class="card"><h3>生成尝试与实际任务</h3>${(entry.attempts||[]).map(a=>`<div class="lrow"><div class="grow"><b>尝试 ${esc(a.attempt)} · ${esc(labels[a.status]||a.status)}</b><p class="small muted">${esc(instant(a.created_at))} · ${esc(a.error_code||a.plan?.reason||'')}</p></div>${a.task_id?`<a class="btn sm" href="#/diag/task/${encodeURIComponent(a.task_id)}">实际任务</a>`:''}</div>`).join('')}${entry.has_more_attempts?'<p class="small muted">显示最近20次尝试；更早执行记录仍保留在任务诊断中。</p>':''}</div><div class="card"><form class="stack" data-note><h3>你的收藏与注记</h3><p class="small muted">这些字段属于你，不修改她的主观正文。</p>${conflict?ConflictPanel({baseVersion:draft.base,currentVersion:entry.annotation.revision}):''}${Toggle({label:'收藏这篇日记',name:'favorite',checked:draft.favorite})}${Field({label:'你的注记',name:'note',type:'textarea',value:draft.note,attrs:{maxlength:5000}})}<div class="row"><button type="button" class="btn" data-reset>撤销草稿</button><button class="btn primary" ${conflict||draft.pending.note?'disabled':''}>保存注记</button></div><p role="alert" data-note-error></p>${conflict?JsonDetails(entry.annotation,'最新已保存注记（草稿保留）'):''}</form></div><div class="card"><h3>你的回应</h3><div data-replies></div><form class="stack" data-reply>${Field({label:'想告诉她',name:'text',type:'textarea',value:draft.reply,attrs:{required:true,maxlength:2000,disabled:!entry.body}})}<p class="small muted">回应形成有日记来源的真实认知请求；她自主决定表达，结果在会话和请求详情回读。回应不改写原日记。</p><button class="btn" type="button" data-clear-reply>清空草稿</button><button class="btn primary" ${entry.body&&!draft.pending.reply?'':'disabled'}>发出回应</button><p role="alert" data-reply-error></p></form></div><a class="btn" href="#/c/${fid}/life/diary">回日记列表</a></div>`;
  const replies=mountCursorRecords(el.querySelector('[data-replies]'),{alive,read:cursor=>diaryClient.replies(fid,id,cursor),label:'回应',empty:'尚无回应。',render:r=>`<div class="lrow"><div class="grow"><p>${esc(r.payload.text)}</p><p class="small muted">${esc(instant(r.created_at))} · ${esc(r.decision_reason||'')}</p></div>${badge(r.status)}<button class="btn sm" data-request="${esc(r.id)}">查看处理</button></div>`,bind:(box,page)=>box.querySelectorAll('[data-request]').forEach(button=>button.onclick=()=>requestReceipt(fid,{request:page.items.find(r=>r.id===button.dataset.request)},()=>replies.reload({reset:true}),{title:'日记回应'}))});
  el.querySelector('[data-reload]').onclick=reload;
  el.querySelector('[data-retry]')?.addEventListener('click',async event=>{event.target.disabled=true;try{await diaryClient.generate(fid,{localDate:entry.date,timezone:entry.timezone,expectedRevision:entry.revision,reason:'Owner在失败或未运行后明确重试日记'});if(alive())await reload();}catch(error){if(alive()){el.querySelector('[data-generation-feedback]').textContent=error.message;event.target.disabled=false;}}});
  const note=el.querySelector('[data-note]');note.oninput=note.onchange=()=>{draft.note=note.note.value;draft.favorite=note.favorite.checked;draft.dirty=true;};
  el.querySelector('[data-reset]').onclick=()=>{draft.dirty=false;void reload();};
  el.querySelector('[data-keep]')?.addEventListener('click',()=>{el.querySelector('[data-note-error]').textContent='注记草稿已保留。';});
  el.querySelector('[data-refresh]')?.addEventListener('click',async()=>{if(await confirm({title:'核对最新注记后采用新基线',text:'保留当前草稿。请核对上方最新已保存注记，后续保存会基于它的版本。',identity:'所有者修改'})&&alive()){draft.base=entry.annotation.revision;void reload();}});
  const reply=el.querySelector('[data-reply]');reply.oninput=()=>draft.reply=reply.text.value;
  el.querySelector('[data-clear-reply]').onclick=()=>{draft.reply='';reply.text.value='';};
  const settled=(channel,error)=>{
   if(!alive())return;
   if(channel==='note'){
    note.querySelector('.primary').disabled=false;
    note.querySelector('[data-note-error]').textContent=error?.message||(draft.dirty?'本次提交已保存，期间的新草稿保留。':'注记已保存');
    if(error?.status===409||(!error&&!draft.dirty))void reload();
   }else{
    reply.querySelector('.primary').disabled=!entry.body;
    reply.text.value=draft.reply;
    reply.querySelector('[data-reply-error]').textContent=error?.message||'本次回应已受理；新的未提交草稿保留。';
    if(!error)void replies.reload({reset:true});
   }
  };
  draft.onSettled=settled;
  note.onsubmit=async event=>{
   event.preventDefault();if(draft.pending.note)return;
   const submission=beginDraftSubmission(draft,'note',{favorite:draft.favorite,note:draft.note});
   note.querySelector('.primary').disabled=true;
   try{
    const saved=await diaryClient.annotate(fid,id,{...submission.value,expectedRevision:draft.base});
    if(drafts.get(key)!==draft)return;
    const result=settleDraftSubmission(draft,submission,{favorite:draft.favorite,note:draft.note});
    if(result){draft.base=Math.max(draft.base,saved.revision);draft.dirty=!result.unchanged;draft.onSettled?.('note');}
   }catch(error){if(drafts.get(key)!==draft)return;settleDraftSubmission(draft,submission,null);draft.dirty=true;draft.onSettled?.('note',error);}
  };
  reply.onsubmit=async event=>{
   event.preventDefault();if(draft.pending.reply)return;
   const submission=beginDraftSubmission(draft,'reply',draft.reply);
   reply.querySelector('.primary').disabled=true;
   try{
    const result=await diaryClient.reply(fid,id,submission.value,entry.revision);
    if(drafts.get(key)!==draft)return;
    const status=settleDraftSubmission(draft,submission,draft.reply);if(status?.unchanged)draft.reply='';
    draft.onSettled?.('reply');
    if(alive())requestReceipt(fid,result,()=>replies.reload({reset:true}),{title:'日记回应'});
   }catch(error){if(drafts.get(key)!==draft)return;settleDraftSubmission(draft,submission,null);draft.onSettled?.('reply',error);}
  };
  return()=>{replies.dispose();if(draft.onSettled===settled)delete draft.onSettled;};
 });
}
export function renderRealShared(el,ctx){
 const cursors=[''];let at=0;
 return asyncPage(el,()=>diaryClient.shared(ctx.charId,cursors[at]),(page,reload)=>{
  el.innerHTML=`<p class="small muted">按实际参与消息记录共同交流。远程交流不表示在同一地点；日记、记忆和独自生活事件不会自动成为双方同场经历。</p><div class="card chain">${page.items.map(r=>`<div class="lrow"><b class="small">${esc(instant(r.occurred_at))}</b><a class="grow" href="${esc(r.href)}">${esc(r.title)}</a>${badge('远程交流','plain')}</div>`).join('')||'<p class="small muted">尚无可读取的双方交流记录。可以从对话开始真实交流。</p>'}</div>${CursorPager({more:!!page.next_cursor,previous:at>0,label:'共同经历'})}<a class="btn" href="#/c/${ctx.charId}/chat">进入对话</a>`;
  bindCursorPager(el,{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});
 });
}
